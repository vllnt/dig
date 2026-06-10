package store

import (
	"os"
	"path/filepath"
	"testing"
)

// Rung 2 — label union: two views label the SAME path; both merge clean.
func TestLadderLabelUnion(t *testing.T) {
	root, st := setupViewKB(t, 1)

	mustV(t)(st.CreateView("a"))
	mustV(t)(st.ProposeView("a", []ViewOp{{From: "area0/file0.txt", Labels: []string{"alpha"}}}))
	mustV(t)(st.StageView("a"))

	mustV(t)(st.CreateView("b"))
	mustV(t)(st.ProposeView("b", []ViewOp{{From: "area0/file0.txt", Labels: []string{"beta"}}}))
	mustV(t)(st.StageView("b"))

	if v, _, err := st.MergeView(root, "a", nil); err != nil || v.State != StateMerged {
		t.Fatalf("a: %v %+v", err, v)
	}
	if v, _, err := st.MergeView(root, "b", nil); err != nil || v.State != StateMerged {
		t.Fatalf("b should union labels, got: %v %+v", err, v)
	}
	head, _ := st.Head()
	e, _ := head.Lookup("area0/file0.txt")
	if len(e.Labels) != 2 {
		t.Fatalf("labels should union to 2: %+v", e)
	}
}

// Rung 2 — blob follow: head moved the file; a label-only op finds it at the
// new path instead of conflicting.
func TestLadderLabelFollowsMovedFile(t *testing.T) {
	root, st := setupViewKB(t, 1)

	mustV(t)(st.CreateView("mover"))
	mustV(t)(st.ProposeView("mover", []ViewOp{{From: "area0/file0.txt", To: "moved/f.txt"}}))
	mustV(t)(st.StageView("mover"))

	mustV(t)(st.CreateView("labeler"))
	mustV(t)(st.ProposeView("labeler", []ViewOp{{From: "area0/file0.txt", Labels: []string{"tagged"}}}))
	mustV(t)(st.StageView("labeler"))

	if v, _, err := st.MergeView(root, "mover", nil); err != nil || v.State != StateMerged {
		t.Fatalf("mover: %v %+v", err, v)
	}
	v, _, err := st.MergeView(root, "labeler", nil)
	if err != nil || v.State != StateMerged {
		t.Fatalf("labeler should follow the moved blob: %v %+v", err, v)
	}
	head, _ := st.Head()
	e, ok := head.Lookup("moved/f.txt")
	if !ok || len(e.Labels) != 1 || e.Labels[0] != "tagged" {
		t.Fatalf("label did not follow the file: %+v", e)
	}
}

// Rung 2 — same-target noop: head already moved the file exactly where the
// view wanted; the view merges as a no-op union.
func TestLadderSameTargetNoop(t *testing.T) {
	root, st := setupViewKB(t, 1)

	for _, name := range []string{"x", "y"} {
		mustV(t)(st.CreateView(name))
		mustV(t)(st.ProposeView(name, []ViewOp{{From: "area0/file0.txt", To: "same/place.txt"}}))
		mustV(t)(st.StageView(name))
	}
	if v, _, err := st.MergeView(root, "x", nil); err != nil || v.State != StateMerged {
		t.Fatalf("x: %v %+v", err, v)
	}
	v, _, err := st.MergeView(root, "y", nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.State != StateMerged {
		t.Fatalf("agreeing move should merge as noop, got %s (%s)", v.State, v.Conflict)
	}
}

// Rung 3 — precedence. Setup: the entry at head carries rule provenance
// ("weak" placed it) and its content was touched since the view's base.
// An incoming op attributed to an EARLIER policy rule ("strong") wins and
// applies; an incoming op from a LATER rule drops — head stands. Either way
// no escalation: policy resolved it deterministically.
func TestLadderRulePrecedence(t *testing.T) {
	rankFn := func(rule string) int {
		switch rule {
		case "strong":
			return 0
		case "weak":
			return 1
		case "weakest":
			return 2
		}
		return -1
	}

	scenario := func(t *testing.T, incomingRule string) (*View, *Store, string) {
		t.Helper()
		root, st := setupViewKB(t, 1)
		head, _ := st.Head()

		// Provenance: a previous (weak-rule) op placed the entry.
		entries := append([]Entry{}, head.Entries...)
		entries[0].Rule = "weak"
		if _, err := st.Commit("org", KindMutate, entries); err != nil {
			t.Fatal(err)
		}
		// View forks here.
		mustV(t)(st.CreateView("v"))
		mustV(t)(st.ProposeView("v", []ViewOp{{From: "area0/file0.txt", To: "claimed/f.txt", Rule: incomingRule}}))
		mustV(t)(st.StageView("v"))

		// Touch: content changes at the same path (an observe commit), so the
		// path is "changed since base" when the view merges.
		entries2 := append([]Entry{}, entries...)
		entries2[0].Blob = HashBytes([]byte("edited"))
		if err := os.WriteFile(filepath.Join(root, "area0/file0.txt"), []byte("edited"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := st.Commit("scan", KindObserve, entries2); err != nil {
			t.Fatal(err)
		}

		v, _, err := st.MergeView(root, "v", rankFn)
		if err != nil {
			t.Fatal(err)
		}
		return v, st, root
	}

	t.Run("stronger incoming rule wins and applies", func(t *testing.T) {
		v, st, _ := scenario(t, "strong")
		if v.State != StateMerged {
			t.Fatalf("strong rule should auto-win: %s (%s)", v.State, v.Conflict)
		}
		head, _ := st.Head()
		if _, ok := head.Lookup("claimed/f.txt"); !ok {
			t.Fatal("winning op not applied")
		}
	})

	t.Run("weaker incoming rule drops, head stands", func(t *testing.T) {
		v, st, _ := scenario(t, "weakest")
		if v.State != StateMerged {
			t.Fatalf("weaker rule should resolve by dropping, not escalate: %s", v.State)
		}
		head, _ := st.Head()
		if _, ok := head.Lookup("area0/file0.txt"); !ok {
			t.Fatal("head placement should stand when incoming rule is weaker")
		}
		if _, ok := head.Lookup("claimed/f.txt"); ok {
			t.Fatal("losing op must not apply")
		}
	})

	t.Run("unattributed conflict escalates", func(t *testing.T) {
		v, _, _ := scenario(t, "") // no rule on the incoming op
		if v.State != StateEscalated {
			t.Fatalf("rule-less conflict must go to a human: %s", v.State)
		}
	})
}

// Rung 4 + subtree isolation: a view with one clean op and one conflicted op
// PARTIALLY merges — the clean op lands, only the conflicted op is held.
// A conflict on one subtree never blocks the other.
func TestLadderPartialMergeHoldsOnlyConflicts(t *testing.T) {
	root, st := setupViewKB(t, 2) // area0, area1

	// Winner takes area0's file first.
	mustV(t)(st.CreateView("winner"))
	mustV(t)(st.ProposeView("winner", []ViewOp{{From: "area0/file0.txt", To: "won/f.txt"}}))
	mustV(t)(st.StageView("winner"))
	if v, _, err := st.MergeView(root, "winner", nil); err != nil || v.State != StateMerged {
		t.Fatalf("winner: %v %+v", err, v)
	}

	// Worker proposed BOTH a conflicting move (area0 → elsewhere) and a clean
	// move (area1) from the old base.
	mustV(t)(st.CreateView("worker"))
	mustV(t)(st.ProposeView("worker", []ViewOp{
		{From: "area0/file0.txt", To: "other/f.txt"}, // conflicts with winner
		{From: "area1/file1.txt", To: "fine/f1.txt"}, // clean — different subtree
	}))
	// Staging validates against the view's base... the view was created after
	// winner merged, so area0/file0.txt is gone from its base → stage fails.
	// That failure is correct; rebuild the realistic scenario: create the view
	// BEFORE the winner merges. Start over with fresh names.
	mustV(t)(st.AbortView("worker"))

	mustV(t)(st.CreateView("worker2"))
	// base now = post-winner head; conflict comes from a second mover.
	mustV(t)(st.ProposeView("worker2", []ViewOp{
		{From: "won/f.txt", To: "mine/f.txt"},        // will conflict with mover3
		{From: "area1/file1.txt", To: "fine/f1.txt"}, // clean
	}))
	mustV(t)(st.StageView("worker2"))

	mustV(t)(st.CreateView("mover3"))
	mustV(t)(st.ProposeView("mover3", []ViewOp{{From: "won/f.txt", To: "theirs/f.txt"}}))
	mustV(t)(st.StageView("mover3"))
	if v, _, err := st.MergeView(root, "mover3", nil); err != nil || v.State != StateMerged {
		t.Fatalf("mover3: %v %+v", err, v)
	}

	// worker2 now merges: area1 op clean → lands; won/f.txt op conflicted → held.
	v, m, err := st.MergeView(root, "worker2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.State != StateEscalated {
		t.Fatalf("worker2 should be ESCALATED with remainder, got %s", v.State)
	}
	if m == nil {
		t.Fatal("partial merge should still commit the clean op")
	}
	if len(v.Ops) != 1 || v.Ops[0].From != "won/f.txt" {
		t.Fatalf("remainder should hold only the conflicted op: %+v", v.Ops)
	}
	head, _ := st.Head()
	if _, ok := head.Lookup("fine/f1.txt"); !ok {
		t.Fatal("clean subtree op was blocked by an unrelated conflict")
	}
	if _, err := os.Stat(filepath.Join(root, "fine/f1.txt")); err != nil {
		t.Fatal("clean op not applied on disk")
	}

	// Resolution: --mine force-applies the held op against fresh state...
	// but theirs/f.txt occupies nothing relevant — mine moves theirs/f.txt?
	// No: held op is {won/f.txt → mine/f.txt} and won/f.txt no longer exists
	// (mover3 moved it) — resolve --mine re-merges; the op's From is gone and
	// the blob now lives at theirs/f.txt; a MOVE op falls to remainder again →
	// still ESCALATED. The human's real choice here is --theirs.
	rv, _, err := st.ResolveView(root, "worker2", "theirs")
	if err != nil {
		t.Fatal(err)
	}
	if rv.State != StateAborted {
		t.Fatalf("resolve --theirs should abort the remainder: %s", rv.State)
	}
}

// Resolution POV: --mine applies held ops when they are applicable against
// fresh state (target freed up).
func TestResolveMineApplies(t *testing.T) {
	root, st := setupViewKB(t, 2)

	// v1 moves file0 → spot.txt and merges.
	mustV(t)(st.CreateView("v1"))
	mustV(t)(st.ProposeView("v1", []ViewOp{{From: "area0/file0.txt", To: "spot.txt"}}))
	mustV(t)(st.StageView("v1"))
	if v, _, err := st.MergeView(root, "v1", nil); err != nil || v.State != StateMerged {
		t.Fatalf("v1: %v %+v", err, v)
	}

	// v2 (older base unaware) wants file1 → spot.txt: target occupied → held.
	mustV(t)(st.CreateView("v2"))
	mustV(t)(st.ProposeView("v2", []ViewOp{{From: "area1/file1.txt", To: "spot.txt"}}))
	mustV(t)(st.StageView("v2"))
	v, _, err := st.MergeView(root, "v2", nil)
	if err != nil || v.State != StateEscalated {
		t.Fatalf("v2 should escalate on occupied target: %v %+v", err, v)
	}

	// Human frees the spot by moving it via a third view, then resolves mine.
	mustV(t)(st.CreateView("v3"))
	mustV(t)(st.ProposeView("v3", []ViewOp{{From: "spot.txt", To: "archive/spot.txt"}}))
	mustV(t)(st.StageView("v3"))
	if v, _, err := st.MergeView(root, "v3", nil); err != nil || v.State != StateMerged {
		t.Fatalf("v3: %v %+v", err, v)
	}

	rv, m, err := st.ResolveView(root, "v2", "mine")
	if err != nil {
		t.Fatal(err)
	}
	if rv.State != StateMerged || m == nil {
		t.Fatalf("resolve --mine should merge now: %+v", rv)
	}
	head, _ := st.Head()
	if _, ok := head.Lookup("spot.txt"); !ok {
		t.Fatal("resolved op not applied")
	}
}

// Contract POV: resolving a non-escalated view fails; bad accept fails.
func TestResolveGuards(t *testing.T) {
	root, st := setupViewKB(t, 1)
	mustV(t)(st.CreateView("v"))
	if _, _, err := st.ResolveView(root, "v", "mine"); err == nil {
		t.Fatal("resolving a DRAFT view must fail")
	}
	if _, _, err := st.ResolveView(root, "v", "bananas"); err == nil {
		t.Fatal("unknown accept value must fail")
	}
}
