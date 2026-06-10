package dedup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/organize"
	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/scan"
	"github.com/vllnt/dig/internal/store"
)

var (
	older = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
)

// setupKB writes files with controlled mtimes, scans, returns the pieces.
func setupKB(t *testing.T, files map[string]string, mtimes map[string]time.Time) (kb.KB, *store.Store, *store.Manifest) {
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
		if mt, ok := mtimes[rel]; ok {
			if err := os.Chtimes(p, mt, mt); err != nil {
				t.Fatal(err)
			}
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
		b, _ := os.ReadFile(p)
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPlanKeepOldest(t *testing.T) {
	_, _, head := setupKB(t,
		map[string]string{"old/a.txt": "same", "new/b.txt": "same", "unique.txt": "solo"},
		map[string]time.Time{"old/a.txt": older, "new/b.txt": newer},
	)
	plan, err := BuildPlan(head, policy.DedupPolicy{}) // default keep-oldest
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Sets) != 1 || plan.Sets[0].Keep != "old/a.txt" {
		t.Fatalf("keep-oldest should keep old/a.txt: %+v", plan.Sets)
	}
	if len(plan.Sets[0].Remove) != 1 || plan.Sets[0].Remove[0] != "new/b.txt" {
		t.Fatalf("remove set wrong: %+v", plan.Sets[0])
	}
}

func TestPlanKeepNewest(t *testing.T) {
	_, _, head := setupKB(t,
		map[string]string{"old/a.txt": "same", "new/b.txt": "same"},
		map[string]time.Time{"old/a.txt": older, "new/b.txt": newer},
	)
	plan, err := BuildPlan(head, policy.DedupPolicy{Strategy: "keep-newest"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Sets) != 1 || plan.Sets[0].Keep != "new/b.txt" {
		t.Fatalf("keep-newest should keep new/b.txt: %+v", plan.Sets)
	}
}

// Escalation POV: ambiguity (mtime tie) must conflict, never guess.
func TestPlanTieEscalates(t *testing.T) {
	_, _, head := setupKB(t,
		map[string]string{"x/a.txt": "same", "y/b.txt": "same"},
		map[string]time.Time{"x/a.txt": older, "y/b.txt": older},
	)
	plan, err := BuildPlan(head, policy.DedupPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Sets) != 0 || len(plan.Conflicts) != 1 {
		t.Fatalf("tie must escalate: sets=%+v conflicts=%+v", plan.Sets, plan.Conflicts)
	}
}

// Data-integrity POV: apply removes duplicates but the canonical copy and all
// unique files survive; dedup never deletes the last copy.
func TestApplyNeverDeletesLastCopy(t *testing.T) {
	k, st, head := setupKB(t,
		map[string]string{"old/a.txt": "same", "new/b.txt": "same", "unique.txt": "solo"},
		map[string]time.Time{"old/a.txt": older, "new/b.txt": newer},
	)
	plan, _ := BuildPlan(head, policy.DedupPolicy{})
	m, err := Apply(k.Root, st, head, plan)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != store.KindMutate {
		t.Fatalf("dedup commit must be KindMutate, got %q", m.Kind)
	}
	ds := diskState(t, k.Root)
	if ds["old/a.txt"] != "same" || ds["unique.txt"] != "solo" {
		t.Fatalf("canonical or unique file lost: %+v", ds)
	}
	if _, gone := ds["new/b.txt"]; gone {
		t.Fatal("duplicate should be removed")
	}
	if len(m.Entries) != 2 {
		t.Fatalf("manifest should have 2 entries, got %d", len(m.Entries))
	}
}

// Data-integrity POV: undo after dedup restores every removed copy
// byte-identically (blob store is the safety net).
func TestUndoRestoresRemovedDuplicates(t *testing.T) {
	k, st, head := setupKB(t,
		map[string]string{"old/a.txt": "same", "new/b.txt": "same", "c/third.txt": "same"},
		map[string]time.Time{"old/a.txt": older, "new/b.txt": newer, "c/third.txt": newer.Add(time.Hour)},
	)
	before := diskState(t, k.Root)

	plan, _ := BuildPlan(head, policy.DedupPolicy{})
	if _, err := Apply(k.Root, st, head, plan); err != nil {
		t.Fatal(err)
	}
	if len(diskState(t, k.Root)) != 1 {
		t.Fatal("expected only canonical copy on disk")
	}

	undone, parent, err := st.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if err := organize.Revert(k.Root, st, undone, parent); err != nil {
		t.Fatal(err)
	}
	after := diskState(t, k.Root)
	if len(after) != len(before) {
		t.Fatalf("undo file count: want %d got %d", len(before), len(after))
	}
	for p, c := range before {
		if after[p] != c {
			t.Fatalf("undo not byte-identical at %s", p)
		}
	}
}

// Idempotency POV: a second dedup over the collapsed manifest is a no-op.
func TestDedupIdempotent(t *testing.T) {
	k, st, head := setupKB(t,
		map[string]string{"old/a.txt": "same", "new/b.txt": "same"},
		map[string]time.Time{"old/a.txt": older, "new/b.txt": newer},
	)
	plan, _ := BuildPlan(head, policy.DedupPolicy{})
	m, err := Apply(k.Root, st, head, plan)
	if err != nil {
		t.Fatal(err)
	}
	plan2, err := BuildPlan(m, policy.DedupPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if !plan2.Empty() || len(plan2.Conflicts) != 0 {
		t.Fatalf("second dedup must be a no-op: %+v", plan2)
	}
}

// Safety POV: if the canonical copy vanished from disk between plan and
// apply, dedup refuses rather than deleting the remaining copies.
func TestApplyRefusesWhenCanonicalMissing(t *testing.T) {
	k, st, head := setupKB(t,
		map[string]string{"old/a.txt": "same", "new/b.txt": "same"},
		map[string]time.Time{"old/a.txt": older, "new/b.txt": newer},
	)
	plan, _ := BuildPlan(head, policy.DedupPolicy{})
	if err := os.Remove(filepath.Join(k.Root, "old/a.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(k.Root, st, head, plan); err == nil {
		t.Fatal("apply must refuse when the canonical copy is missing")
	}
	// The other copy must still exist.
	if ds := diskState(t, k.Root); ds["new/b.txt"] != "same" {
		t.Fatal("refusal must leave remaining copies untouched")
	}
}
