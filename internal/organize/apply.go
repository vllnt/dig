package organize

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/store"
)

// Apply executes the plan on disk and commits it as ONE journaled mutation
// manifest. Disk ops happen first; the commit records the resulting tree, so
// a crash mid-apply is recovered by the next scan (disk = source of truth).
// Unsorted files are labeled (manifest-only — no disk effect).
func Apply(kbRoot string, st *store.Store, head *store.Manifest, plan *Plan) (*store.Manifest, error) {
	if head == nil {
		return nil, fmt.Errorf("nothing to organize: no manifest — run 'dig scan' first")
	}

	// Disk moves. Two-phase via temp names to survive swap chains (a→b while
	// b→c): first stage every source aside, then settle into targets.
	type staged struct{ tmp, to string }
	var moves []staged
	for _, op := range plan.Ops {
		if op.Kind != OpMove {
			continue
		}
		src := filepath.Join(kbRoot, filepath.FromSlash(op.From))
		tmp := src + ".dig-moving"
		if err := os.Rename(src, tmp); err != nil {
			return nil, fmt.Errorf("stage %s: %w", op.From, err)
		}
		moves = append(moves, staged{tmp: tmp, to: filepath.Join(kbRoot, filepath.FromSlash(op.To))})
	}
	for _, mv := range moves {
		if err := os.MkdirAll(filepath.Dir(mv.to), 0o755); err != nil {
			return nil, fmt.Errorf("create target dir: %w", err)
		}
		if err := os.Rename(mv.tmp, mv.to); err != nil {
			return nil, fmt.Errorf("settle %s: %w", mv.to, err)
		}
	}
	pruneEmptyDirs(kbRoot)

	// New manifest entries = head entries with plan ops applied.
	byPath := map[string]*Op{}
	for i := range plan.Ops {
		byPath[plan.Ops[i].From] = &plan.Ops[i]
	}
	unsorted := map[string]bool{}
	for _, p := range plan.Unsorted {
		unsorted[p] = true
	}

	entries := make([]store.Entry, 0, len(head.Entries))
	for _, e := range head.Entries {
		ne := e
		if op, ok := byPath[e.Path]; ok {
			if op.Kind == OpMove {
				ne.Path = op.To
			}
			ne.Labels = append(append([]string{}, e.Labels...), op.Labels...)
			ne.Rule = op.Rule // provenance — precedence resolution keys off this
		} else if unsorted[e.Path] {
			ne.Labels = append(append([]string{}, e.Labels...), policy.UnsortedLabel)
		}
		entries = append(entries, ne)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	return st.Commit("org", store.KindMutate, entries)
}

// Revert reverses the disk effects of an undone mutation manifest: every blob
// that lives at a different path in the parent moves back; entries the parent
// has but disk lost (e.g. duplicates a dedup removed) are restored from the
// blob store. Labels are manifest-only and revert with the head pointer itself.
func Revert(kbRoot string, st *store.Store, undone, parent *store.Manifest) error {
	if parent == nil {
		return nil
	}
	// Index parent paths by blob to find where each file should return to.
	parentPath := map[string]string{} // blob → path (first occurrence)
	for _, e := range parent.Entries {
		if _, ok := parentPath[e.Blob]; !ok {
			parentPath[e.Blob] = e.Path
		}
	}
	// Stage-then-settle, mirroring Apply, for reverse swap chains.
	type staged struct{ tmp, to string }
	var moves []staged
	for _, e := range undone.Entries {
		want, ok := parentPath[e.Blob]
		if !ok || want == e.Path {
			continue
		}
		src := filepath.Join(kbRoot, filepath.FromSlash(e.Path))
		if _, err := os.Stat(src); err != nil {
			continue // disk moved on since; next scan observes reality
		}
		tmp := src + ".dig-moving"
		if err := os.Rename(src, tmp); err != nil {
			return fmt.Errorf("stage revert %s: %w", e.Path, err)
		}
		moves = append(moves, staged{tmp: tmp, to: filepath.Join(kbRoot, filepath.FromSlash(want))})
	}
	for _, mv := range moves {
		if err := os.MkdirAll(filepath.Dir(mv.to), 0o755); err != nil {
			return err
		}
		if err := os.Rename(mv.tmp, mv.to); err != nil {
			return fmt.Errorf("settle revert %s: %w", mv.to, err)
		}
	}

	// Restore pass: parent entries missing on disk (e.g. duplicates a dedup
	// removed) come back from the blob store — content never dies there.
	for _, e := range parent.Entries {
		dst := filepath.Join(kbRoot, filepath.FromSlash(e.Path))
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		rc, err := st.Blobs().Get(e.Blob)
		if err != nil {
			return fmt.Errorf("restore %s: blob %s: %w", e.Path, e.Blob, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			_ = rc.Close()
			return err
		}
		f, err := os.Create(dst)
		if err != nil {
			_ = rc.Close()
			return fmt.Errorf("restore %s: %w", e.Path, err)
		}
		if _, err := io.Copy(f, rc); err != nil {
			_ = f.Close()
			_ = rc.Close()
			return fmt.Errorf("restore %s: %w", e.Path, err)
		}
		_ = f.Close()
		_ = rc.Close()
		_ = os.Chtimes(dst, e.ModTime, e.ModTime)
	}
	pruneEmptyDirs(kbRoot)
	return nil
}

// pruneEmptyDirs removes directories left empty by moves, bottom-up. Best
// effort — an unremovable dir is not an error. Never touches .dig.
func pruneEmptyDirs(kbRoot string) {
	var dirs []string
	_ = filepath.WalkDir(kbRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil //nolint:nilerr // best-effort walk
		}
		if d.Name() == ".dig" {
			return filepath.SkipDir
		}
		if p != kbRoot {
			dirs = append(dirs, p)
		}
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) }) // deepest first
	for _, d := range dirs {
		_ = os.Remove(d) // fails on non-empty — exactly what we want
	}
}
