package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// setupViewKB seeds a store + disk with n files under distinct subtrees.
func setupViewKB(t *testing.T, n int) (string, *Store) {
	t.Helper()
	root := t.TempDir()
	st, err := Open(filepath.Join(root, ".dig"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	var entries []Entry
	for i := 0; i < n; i++ {
		rel := fmt.Sprintf("area%d/file%d.txt", i, i)
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		content := []byte(fmt.Sprintf("content-%d", i))
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
		h := HashBytes(content)
		if err := st.Blobs().Put(h, bytes.NewReader(content)); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, Entry{Path: rel, Blob: h})
	}
	if _, err := st.Commit("scan", KindObserve, entries); err != nil {
		t.Fatal(err)
	}
	return root, st
}

// mustV returns an unwrapper for view-returning setup calls — multi-value
// returns can only be forwarded as a call's sole arguments, hence the curry.
func mustV(t *testing.T) func(*View, error) *View {
	return func(v *View, err error) *View {
		t.Helper()
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
		return v
	}
}

// State-machine POV: legal lifecycle works; illegal transitions are rejected;
// terminal states stay terminal.
func TestViewStateMachine(t *testing.T) {
	root, st := setupViewKB(t, 1)

	v, err := st.CreateView("w")
	if err != nil || v.State != StateDraft {
		t.Fatalf("create: %v %+v", err, v)
	}
	// DRAFT → MERGED is illegal (never commit unvalidated work).
	if _, _, err := st.MergeView(root, "w", nil); err == nil {
		t.Fatal("DRAFT → MERGED must be rejected")
	}
	// DRAFT → STAGED is illegal (must propose first).
	if _, err := st.StageView("w"); err == nil {
		t.Fatal("DRAFT → STAGED must be rejected")
	}
	if _, err := st.ProposeView("w", []ViewOp{{From: "area0/file0.txt", To: "moved/file0.txt"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.StageView("w"); err != nil {
		t.Fatal(err)
	}
	v, m, err := st.MergeView(root, "w", nil)
	if err != nil || v.State != StateMerged {
		t.Fatalf("merge: %v %+v", err, v)
	}
	if m.Kind != KindMutate || m.CreatedBy != "merge/w" {
		t.Fatalf("merge manifest wrong: %+v", m)
	}
	// MERGED is terminal.
	if _, err := st.AbortView("w"); err == nil {
		t.Fatal("MERGED → ABORTED must be rejected (terminal)")
	}
}

// Validation POV: staging rejects unknown paths and root escapes.
func TestStageValidation(t *testing.T) {
	_, st := setupViewKB(t, 1)
	mustV(t)(st.CreateView("bad1"))
	mustV(t)(st.ProposeView("bad1", []ViewOp{{From: "nope.txt", To: "x.txt"}}))
	if _, err := st.StageView("bad1"); err == nil {
		t.Fatal("staging an op on a path missing from base must fail")
	}
	mustV(t)(st.CreateView("bad2"))
	mustV(t)(st.ProposeView("bad2", []ViewOp{{From: "area0/file0.txt", To: "../escape.txt"}}))
	if _, err := st.StageView("bad2"); err == nil {
		t.Fatal("staging a root-escaping target must fail")
	}
}

// THE concurrency test: N workers on disjoint subtrees, all merging
// concurrently. Every merge must land, no lost ops, no torn state.
func TestConcurrentDisjointMergesAllLand(t *testing.T) {
	const n = 8
	root, st := setupViewKB(t, n)

	// Each worker opens a view on the SAME base, proposes a move inside its
	// own subtree, stages, merges — all in parallel.
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("worker%d", i)
			if _, err := st.CreateView(name); err != nil {
				errs[i] = err
				return
			}
			op := ViewOp{
				From:   fmt.Sprintf("area%d/file%d.txt", i, i),
				To:     fmt.Sprintf("area%d/renamed%d.txt", i, i),
				Labels: []string{fmt.Sprintf("w%d", i)},
			}
			if _, err := st.ProposeView(name, []ViewOp{op}); err != nil {
				errs[i] = err
				return
			}
			if _, err := st.StageView(name); err != nil {
				errs[i] = err
				return
			}
			v, _, err := st.MergeView(root, name, nil)
			if err != nil {
				errs[i] = err
				return
			}
			if v.State != StateMerged {
				errs[i] = fmt.Errorf("worker %d not merged: %s (%s)", i, v.State, v.Conflict)
			}
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
	}

	// Final manifest holds every worker's change — no lost updates.
	head, err := st.Head()
	if err != nil {
		t.Fatal(err)
	}
	if len(head.Entries) != n {
		t.Fatalf("entry count changed: want %d got %d", n, len(head.Entries))
	}
	for i := 0; i < n; i++ {
		e, ok := head.Lookup(fmt.Sprintf("area%d/renamed%d.txt", i, i))
		if !ok {
			t.Fatalf("worker %d's move lost", i)
		}
		if len(e.Labels) != 1 || e.Labels[0] != fmt.Sprintf("w%d", i) {
			t.Fatalf("worker %d's label lost: %+v", i, e)
		}
		// Disk reflects the merge.
		if _, err := os.Stat(filepath.Join(root, fmt.Sprintf("area%d/renamed%d.txt", i, i))); err != nil {
			t.Fatalf("worker %d's disk move missing", i)
		}
	}
	// History chain is intact: n merges + 1 scan.
	hist, _ := st.History()
	if len(hist) != n+1 {
		t.Fatalf("history length: want %d got %d", n+1, len(hist))
	}
}

// Conflict POV: two views moving the SAME file differently — first merge
// wins; the second's move is held (ESCALATED); head and disk untouched by
// the loser.
func TestOverlappingMergeConflicts(t *testing.T) {
	root, st := setupViewKB(t, 1)

	mustV(t)(st.CreateView("first"))
	mustV(t)(st.ProposeView("first", []ViewOp{{From: "area0/file0.txt", To: "a/one.txt"}}))
	mustV(t)(st.StageView("first"))

	mustV(t)(st.CreateView("second"))
	mustV(t)(st.ProposeView("second", []ViewOp{{From: "area0/file0.txt", To: "b/two.txt"}}))
	mustV(t)(st.StageView("second"))

	if v, _, err := st.MergeView(root, "first", nil); err != nil || v.State != StateMerged {
		t.Fatalf("first merge: %v %+v", err, v)
	}
	headBefore, _ := st.Head()

	v, _, err := st.MergeView(root, "second", nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.State != StateEscalated {
		t.Fatalf("second merge must ESCALATE its held move, got %s", v.State)
	}
	if len(v.Ops) != 1 {
		t.Fatalf("remainder should hold exactly the conflicted op: %+v", v.Ops)
	}
	headAfter, _ := st.Head()
	if headAfter.ID != headBefore.ID {
		t.Fatal("conflicting merge advanced head")
	}
	if _, err := os.Stat(filepath.Join(root, "a/one.txt")); err != nil {
		t.Fatal("winner's disk state disturbed by conflicting merge")
	}
	// ESCALATED can be aborted (= accept theirs).
	if _, err := st.AbortView("second"); err != nil {
		t.Fatal(err)
	}
}

// Abort POV: aborting a staged view leaves head and disk untouched.
func TestAbortTouchesNothing(t *testing.T) {
	root, st := setupViewKB(t, 1)
	headBefore, _ := st.Head()

	mustV(t)(st.CreateView("w"))
	mustV(t)(st.ProposeView("w", []ViewOp{{From: "area0/file0.txt", To: "x/y.txt"}}))
	mustV(t)(st.StageView("w"))
	if _, err := st.AbortView("w"); err != nil {
		t.Fatal(err)
	}
	headAfter, _ := st.Head()
	if headAfter.ID != headBefore.ID {
		t.Fatal("abort advanced head")
	}
	if _, err := os.Stat(filepath.Join(root, "area0/file0.txt")); err != nil {
		t.Fatal("abort touched disk")
	}
}
