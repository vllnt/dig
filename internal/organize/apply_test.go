package organize

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/scan"
	"github.com/vllnt/dig/internal/store"
)

// setupKB builds a real KB on disk with files, scans it, and returns the
// pieces. No mocks — real store, real filesystem.
func setupKB(t *testing.T, files map[string]string) (kb.KB, *store.Store, *store.Manifest) {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	k, err := kb.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	dig, _ := k.EnsureDig()
	st, err := store.Open(dig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	entries, err := scan.Walk(k, st, false)
	if err != nil {
		t.Fatal(err)
	}
	head, err := st.Commit("scan", store.KindObserve, entries)
	if err != nil {
		t.Fatal(err)
	}
	return k, st, head
}

// diskState maps every file (excluding .dig) to its content.
func diskState(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == kb.DigDir {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestApplyMovesOnDiskAndCommitsMutate(t *testing.T) {
	k, st, head := setupKB(t, map[string]string{
		"inbox/a.pdf": "pdf-a",
		"keep.txt":    "text",
	})
	rs := rules(t, `
[[rule]]
name  = "pdfs"
match = { ext = ["pdf"] }
into  = "docs"
label = ["doc"]`)

	plan, err := BuildPlan(k.Root, head, rs)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Apply(k.Root, st, head, plan)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != store.KindMutate {
		t.Fatalf("org commit must be KindMutate, got %q", m.Kind)
	}
	disk := diskState(t, k.Root)
	if disk["docs/a.pdf"] != "pdf-a" {
		t.Fatalf("file not moved on disk: %+v", disk)
	}
	if _, stale := disk["inbox/a.pdf"]; stale {
		t.Fatal("source file still present after move")
	}
	// keep.txt untouched and labeled unsorted in the manifest.
	if disk["keep.txt"] != "text" {
		t.Fatal("unmatched file was touched")
	}
	e, ok := m.Lookup("keep.txt")
	if !ok || len(e.Labels) != 1 || e.Labels[0] != "unsorted" {
		t.Fatalf("unmatched file should carry unsorted label: %+v", e)
	}
}

// Data-integrity POV: org → revert restores the disk byte-identically.
func TestRevertRestoresDiskByteIdentical(t *testing.T) {
	k, st, head := setupKB(t, map[string]string{
		"inbox/a.pdf": "pdf-a",
		"inbox/b.pdf": "pdf-b",
		"notes/n.txt": "note",
		"media/p.jpg": "jpeg",
	})
	before := diskState(t, k.Root)

	rs := rules(t, `
[[rule]]
name  = "pdfs"
match = { ext = ["pdf"] }
into  = "docs/{year}"
[[rule]]
name  = "imgs"
match = { mime = ["image/*"] }
into  = "gallery"`)

	plan, err := BuildPlan(k.Root, head, rs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(k.Root, st, head, plan); err != nil {
		t.Fatal(err)
	}
	if len(diskState(t, k.Root)) != len(before) {
		t.Fatal("apply lost or duplicated files")
	}

	undone, parent, err := st.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if err := Revert(k.Root, st, undone, parent); err != nil {
		t.Fatal(err)
	}
	after := diskState(t, k.Root)
	if len(after) != len(before) {
		t.Fatalf("revert changed file count: before=%d after=%d", len(before), len(after))
	}
	for p, c := range before {
		if after[p] != c {
			t.Fatalf("revert not byte-identical at %s", p)
		}
	}
}

// State POV: empty plan (already organized) applies as a label-only or no-op
// without disturbing disk.
func TestApplyIdempotentSecondRun(t *testing.T) {
	k, st, head := setupKB(t, map[string]string{"inbox/a.pdf": "pdf-a"})
	rs := rules(t, `
[[rule]]
name  = "pdfs"
match = { ext = ["pdf"] }
into  = "docs"`)

	plan, _ := BuildPlan(k.Root, head, rs)
	m, err := Apply(k.Root, st, head, plan)
	if err != nil {
		t.Fatal(err)
	}
	// Second pass over the new head: nothing to do.
	plan2, err := BuildPlan(k.Root, m, rs)
	if err != nil {
		t.Fatal(err)
	}
	if !plan2.Empty() || len(plan2.Unsorted) != 0 {
		t.Fatalf("second org run should be a no-op, got %+v", plan2)
	}
}
