package drift

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bntvllnt/dig/internal/kb"
	"github.com/bntvllnt/dig/internal/policy"
	"github.com/bntvllnt/dig/internal/store"
)

func setupKB(t *testing.T, files map[string]string) (kb.KB, *store.Store, *store.Manifest) {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		write(t, root, rel, content)
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
	entries, err := DiskEntries(k, st)
	if err != nil {
		t.Fatal(err)
	}
	head, err := st.Commit("scan", store.KindObserve, entries)
	if err != nil {
		t.Fatal(err)
	}
	return k, st, head
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRules(t *testing.T, src string) ([]policy.CompiledRule, policy.DedupPolicy) {
	t.Helper()
	p, err := policy.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := p.Compile()
	if err != nil {
		t.Fatal(err)
	}
	return rules, p.Dedup
}

const testPolicy = `
[[rule]]
name  = "pdfs"
match = { ext = ["pdf"] }
into  = "docs"
label = ["doc"]`

// Diff POV: every change class detected; renames found by content identity.
func TestDiffDetectsAllClasses(t *testing.T) {
	k, st, head := setupKB(t, map[string]string{
		"keep.txt":   "stay",
		"gone.txt":   "delete me",
		"edit.txt":   "v1",
		"moveme.txt": "movable",
	})
	// Human acts: delete, edit, rename, add.
	if err := os.Remove(filepath.Join(k.Root, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	write(t, k.Root, "edit.txt", "v2")
	if err := os.Rename(filepath.Join(k.Root, "moveme.txt"), filepath.Join(k.Root, "moved.txt")); err != nil {
		t.Fatal(err)
	}
	write(t, k.Root, "fresh.txt", "new")

	current, err := DiskEntries(k, st)
	if err != nil {
		t.Fatal(err)
	}
	changes, _ := Diff(head, current)

	got := map[string]string{} // kind → path (one each in this scenario)
	for _, c := range changes {
		got[c.Kind] = c.Path
	}
	if got[Removed] != "gone.txt" || got[Modified] != "edit.txt" ||
		got[Renamed] != "moved.txt" || got[Added] != "fresh.txt" {
		t.Fatalf("diff classes wrong: %+v", changes)
	}
	for _, c := range changes {
		if c.Kind == Renamed && c.From != "moveme.txt" {
			t.Fatalf("rename source wrong: %+v", c)
		}
	}
}

// Coexistence POV: labels are dig metadata — they survive human renames and
// edits the human's tools know nothing about.
func TestDiffPreservesLabelsAcrossRenameAndEdit(t *testing.T) {
	k, st, head := setupKB(t, map[string]string{"a.txt": "alpha", "b.txt": "beta"})
	// Simulate labels assigned by a previous org.
	entries := append([]store.Entry{}, head.Entries...)
	for i := range entries {
		entries[i].Labels = []string{"keepme"}
	}
	labeled, err := st.Commit("org", store.KindMutate, entries)
	if err != nil {
		t.Fatal(err)
	}
	// Human renames one, edits the other.
	if err := os.Rename(filepath.Join(k.Root, "a.txt"), filepath.Join(k.Root, "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	write(t, k.Root, "b.txt", "beta v2")

	current, _ := DiskEntries(k, st)
	_, absorbed := Diff(labeled, current)
	for _, e := range absorbed {
		if !hasLabel(e.Labels, "keepme") {
			t.Fatalf("labels lost through human edit on %s: %+v", e.Path, e)
		}
		// The renamed file is additionally pinned (human placement).
		if e.Path == "renamed.txt" && !hasLabel(e.Labels, policy.PinnedLabel) {
			t.Fatalf("renamed file should be pinned: %+v", e)
		}
	}
}

func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if l == want {
			return true
		}
	}
	return false
}

// THE coexistence test: a human-moved file that now violates policy is
// escalated, never moved back.
func TestReconcileEscalatesHumanMovesNeverOverwrites(t *testing.T) {
	k, st, _ := setupKB(t, map[string]string{"inbox/a.pdf": "pdf"})
	rules, dpol := mustRules(t, testPolicy)

	// First reconcile: files a.pdf under docs/ per policy (auto — it's drift,
	// not a human decision).
	sum, err := Reconcile(k, st, rules, dpol, false, ModeOneShot)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Applied) != 1 || len(sum.Escalated) != 0 {
		t.Fatalf("initial reconcile should auto-file: %+v", sum)
	}

	// Human decides: this pdf belongs in special/.
	if err := os.MkdirAll(filepath.Join(k.Root, "special"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(k.Root, "docs/a.pdf"), filepath.Join(k.Root, "special/a.pdf")); err != nil {
		t.Fatal(err)
	}

	sum, err = Reconcile(k, st, rules, dpol, false, ModeOneShot)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Applied) != 0 || len(sum.Escalated) != 1 {
		t.Fatalf("human move must escalate, not apply: %+v", sum)
	}
	// The file stays where the human put it.
	if _, err := os.Stat(filepath.Join(k.Root, "special/a.pdf")); err != nil {
		t.Fatal("reconcile moved a human-placed file — forbidden")
	}
}

// New files are drift, not intent — they get filed automatically.
func TestReconcileAutoFilesNewFiles(t *testing.T) {
	k, st, _ := setupKB(t, map[string]string{"keep.txt": "x"})
	rules, dpol := mustRules(t, testPolicy)

	if _, err := Reconcile(k, st, rules, dpol, false, ModeOneShot); err != nil {
		t.Fatal(err)
	}
	write(t, k.Root, "dropped.pdf", "new pdf")

	sum, err := Reconcile(k, st, rules, dpol, false, ModeOneShot)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Applied) != 1 || sum.Applied[0].To != "docs/dropped.pdf" {
		t.Fatalf("new file should be auto-filed: %+v", sum)
	}
	if _, err := os.Stat(filepath.Join(k.Root, "docs/dropped.pdf")); err != nil {
		t.Fatal("auto-filed file not on disk at target")
	}
}

// Idempotency POV: a converged KB reconciles to a no-op with no new commits.
func TestReconcileIdempotent(t *testing.T) {
	k, st, _ := setupKB(t, map[string]string{"inbox/a.pdf": "pdf"})
	rules, dpol := mustRules(t, testPolicy)

	if _, err := Reconcile(k, st, rules, dpol, false, ModeOneShot); err != nil {
		t.Fatal(err)
	}
	before, _ := st.History()

	sum, err := Reconcile(k, st, rules, dpol, false, ModeOneShot)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Absorbed)+len(sum.Applied)+len(sum.Escalated)+len(sum.Collapsed) != 0 {
		t.Fatalf("second reconcile should be a no-op: %+v", sum)
	}
	after, _ := st.History()
	if len(after) != len(before) {
		t.Fatal("no-op reconcile must not create empty commits")
	}
}

// Dry-run POV: full computation, zero commits, zero disk changes.
func TestReconcileDryRunTouchesNothing(t *testing.T) {
	k, st, _ := setupKB(t, map[string]string{"inbox/a.pdf": "pdf", "dup1.txt": "same", "dup2.txt": "same"})
	rules, dpol := mustRules(t, testPolicy)

	histBefore, _ := st.History()
	sum, err := Reconcile(k, st, rules, dpol, true, ModeOneShot)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Applied) == 0 {
		t.Fatalf("dry-run should report would-apply ops: %+v", sum)
	}
	histAfter, _ := st.History()
	if len(histAfter) != len(histBefore) {
		t.Fatal("dry-run committed something")
	}
	if _, err := os.Stat(filepath.Join(k.Root, "inbox/a.pdf")); err != nil {
		t.Fatal("dry-run moved a file")
	}
}

// Report POV: all drift classes appear in one report, computed off live disk.
func TestBuildReportClasses(t *testing.T) {
	k, st, _ := setupKB(t, map[string]string{
		"inbox/a.pdf": "pdf",
		"x.txt":       "same",
		"y.txt":       "same",
	})
	rules, dpol := mustRules(t, testPolicy)
	write(t, k.Root, "added.md", "note") // external add

	rep, err := BuildReport(k, st, rules, dpol)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Clean() {
		t.Fatal("report should show drift")
	}
	if len(rep.Changes) != 1 || rep.Changes[0].Kind != Added {
		t.Fatalf("external add missing: %+v", rep.Changes)
	}
	if len(rep.PolicyOps) != 1 { // a.pdf → docs
		t.Fatalf("policy op missing: %+v", rep.PolicyOps)
	}
	if len(rep.Unsorted) == 0 {
		t.Fatal("unsorted files missing from report")
	}
	// x/y .txt same content — dup tie (same mtime second) or set; either way reported.
	if len(rep.DupSets)+len(rep.DupConflicts) == 0 {
		t.Fatal("duplicates missing from report")
	}
}
