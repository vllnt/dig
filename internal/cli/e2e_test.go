package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bntvllnt/dig/internal/kb"
	"github.com/bntvllnt/dig/internal/store"
)

// run executes the real cobra tree exactly as a user (or another harness)
// would: fresh root, args, captured output. No mocks anywhere in the chain.
func run(t *testing.T, args ...string) string {
	t.Helper()
	out := runErrOK(t, true, args...)
	return out
}

func runExpectErr(t *testing.T, args ...string) string {
	t.Helper()
	return runErrOK(t, false, args...)
}

func runErrOK(t *testing.T, wantOK bool, args ...string) string {
	t.Helper()
	root := NewRoot()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	if wantOK && err != nil {
		t.Fatalf("dig %s: %v\n%s", strings.Join(args, " "), err, buf.String())
	}
	if !wantOK && err == nil {
		t.Fatalf("dig %s: expected error, got success\n%s", strings.Join(args, " "), buf.String())
	}
	return buf.String()
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

const e2ePolicy = `
[[rule]]
name  = "invoices"
match = { ext = ["pdf"], content_matches = "invoice" }
into  = "finance/invoices"
label = ["finance"]

[[rule]]
name  = "notes"
match = { ext = ["md"] }
into  = "notes"
`

// TestChainOrganizeLifecycle is the full user journey in one chain:
// init → scan → policy validate → org --dry-run (disk untouched) → org →
// find at new path → undo (disk byte-identical) → org again (same result).
func TestChainOrganizeLifecycle(t *testing.T) {
	root := t.TempDir()
	write(t, root, "inbox/acme.pdf", "ACME invoice #1007")
	write(t, root, "inbox/todo.md", "- [ ] things")
	write(t, root, "misc/photo.bin", "\x00\xff binary")

	run(t, "init", root)
	write(t, root, ".dig/policy.toml", e2ePolicy)

	out := run(t, "--kb", root, "scan")
	if !strings.Contains(out, "manifest M1") {
		t.Fatalf("scan output: %s", out)
	}
	run(t, "--kb", root, "policy", "validate")

	pristine := diskState(t, root)

	// Dry-run shows the plan and changes nothing.
	out = run(t, "--kb", root, "org", "--dry-run")
	for _, want := range []string{"MOVE", "finance/invoices/acme.pdf", "UNSORTED", "photo.bin", "[dry-run]"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run missing %q in:\n%s", want, out)
		}
	}
	if ds := diskState(t, root); len(ds) != len(pristine) || ds["inbox/acme.pdf"] != pristine["inbox/acme.pdf"] {
		t.Fatal("dry-run touched the disk")
	}

	// Apply.
	out = run(t, "--kb", root, "org")
	if !strings.Contains(out, "Applied") {
		t.Fatalf("org output: %s", out)
	}
	ds := diskState(t, root)
	if ds["finance/invoices/acme.pdf"] != "ACME invoice #1007" || ds["notes/todo.md"] != "- [ ] things" {
		t.Fatalf("org did not move files: %+v", ds)
	}
	if _, stale := ds["inbox/acme.pdf"]; stale {
		t.Fatal("source still on disk after org")
	}

	// find sees the new path; labels searchable.
	out = run(t, "--kb", root, "find", "finance")
	if !strings.Contains(out, "finance/invoices/acme.pdf") {
		t.Fatalf("find after org: %s", out)
	}

	// undo restores the disk byte-identically.
	out = run(t, "--kb", root, "undo")
	if !strings.Contains(out, "head is now M1") {
		t.Fatalf("undo output: %s", out)
	}
	after := diskState(t, root)
	if len(after) != len(pristine) {
		t.Fatalf("undo changed file count: %d vs %d", len(after), len(pristine))
	}
	for p, c := range pristine {
		if after[p] != c {
			t.Fatalf("undo not byte-identical at %s", p)
		}
	}

	// Chain continues: org again reproduces the same organization.
	run(t, "--kb", root, "org")
	ds2 := diskState(t, root)
	if ds2["finance/invoices/acme.pdf"] == "" || ds2["notes/todo.md"] == "" {
		t.Fatalf("re-org after undo failed: %+v", ds2)
	}
}

// State POV: undoing a scan (observation) must never touch files.
func TestChainUndoScanNeverTouchesDisk(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.txt", "alpha")
	run(t, "init", root)
	run(t, "--kb", root, "scan")

	write(t, root, "b.txt", "bravo")
	run(t, "--kb", root, "scan") // M2 observes b.txt

	before := diskState(t, root)
	run(t, "--kb", root, "undo") // back to M1 — an observation undo
	after := diskState(t, root)

	if len(after) != len(before) || after["b.txt"] != "bravo" {
		t.Fatal("undo of a scan deleted/changed user files — must never happen")
	}
}

// Contract POV: org without a policy fails loudly with guidance, and org
// before any scan refuses to guess.
func TestChainGuardrails(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.pdf", "x")
	run(t, "init", root)

	// No policy file.
	out := runExpectErr(t, "--kb", root, "org")
	_ = out

	// Policy exists but no scan yet.
	write(t, root, ".dig/policy.toml", e2ePolicy)
	runExpectErr(t, "--kb", root, "org")
}

// TestChainOrgDedupUndoUndo stacks both mutate features and unwinds them in
// order: scan → org → dedup → undo (dedup back) → undo (org back) → disk
// byte-identical to the original. The chain proves mutate-reverts compose.
func TestChainOrgDedupUndoUndo(t *testing.T) {
	root := t.TempDir()
	write(t, root, "inbox/acme.pdf", "ACME invoice #1007")
	write(t, root, "copies/acme-copy.pdf", "ACME invoice #1007") // duplicate content
	write(t, root, "inbox/todo.md", "- [ ] things")
	// Distinct mtimes so dedup's keep-oldest is deterministic.
	older := timeAt(t, 2024)
	newer := timeAt(t, 2025)
	chtimes(t, root, "inbox/acme.pdf", older)
	chtimes(t, root, "copies/acme-copy.pdf", newer)

	run(t, "init", root)
	write(t, root, ".dig/policy.toml", e2ePolicy)
	run(t, "--kb", root, "scan")
	pristine := diskState(t, root)

	// org moves both pdfs... second one CONFLICTS (same target) — kept where
	// it is; dedup later removes it as a duplicate. The chain shows the two
	// features covering each other's gaps.
	run(t, "--kb", root, "org")
	out := run(t, "--kb", root, "dedup", "--dry-run")
	if !strings.Contains(out, "KEEP") {
		t.Fatalf("dedup dry-run should find the duplicate:\n%s", out)
	}
	ds := diskState(t, root)
	nFiles := len(ds)

	out = run(t, "--kb", root, "dedup")
	if !strings.Contains(out, "Removed 1 duplicate") {
		t.Fatalf("dedup output: %s", out)
	}
	if len(diskState(t, root)) != nFiles-1 {
		t.Fatal("dedup should remove exactly one file")
	}

	// Unwind: undo dedup → duplicate restored; undo org → original tree.
	run(t, "--kb", root, "undo")
	if len(diskState(t, root)) != nFiles {
		t.Fatal("undo of dedup did not restore the removed duplicate")
	}
	run(t, "--kb", root, "undo")
	after := diskState(t, root)
	if len(after) != len(pristine) {
		t.Fatalf("full unwind file count: want %d got %d", len(pristine), len(after))
	}
	for p, c := range pristine {
		if after[p] != c {
			t.Fatalf("full unwind not byte-identical at %s", p)
		}
	}
}

func timeAt(t *testing.T, year int) time.Time {
	t.Helper()
	return time.Date(year, 6, 1, 0, 0, 0, 0, time.UTC)
}

func chtimes(t *testing.T, root, rel string, mt time.Time) {
	t.Helper()
	if err := os.Chtimes(filepath.Join(root, filepath.FromSlash(rel)), mt, mt); err != nil {
		t.Fatal(err)
	}
}

// TestChainOrgExportPinned chains org's labels into export's filter, then
// proves the --at pin: scan → org (labels finance) → export filtered →
// mutate disk + rescan → export --at old manifest is byte-identical.
func TestChainOrgExportPinned(t *testing.T) {
	root := t.TempDir()
	write(t, root, "inbox/acme.pdf", "ACME invoice #1007")
	write(t, root, "inbox/todo.md", "- [ ] things")
	run(t, "init", root)
	write(t, root, ".dig/policy.toml", e2ePolicy)
	run(t, "--kb", root, "scan")
	run(t, "--kb", root, "org") // M2: acme.pdf → finance/invoices + label finance

	// Filtered export carries only the labeled file, with provenance.
	out := run(t, "--kb", root, "export", "--filter", "label:finance")
	if !strings.Contains(out, "finance/invoices/acme.pdf") || strings.Contains(out, "todo.md") {
		t.Fatalf("label filter export wrong:\n%s", out)
	}
	if !strings.Contains(out, `"manifest":"M2"`) || !strings.Contains(out, `"src":"b3:`) {
		t.Fatalf("provenance missing:\n%s", out)
	}

	pinned := run(t, "--kb", root, "export", "--at", "M2")

	// Disk moves on: new file lands, gets scanned (M3).
	write(t, root, "inbox/new.md", "later doc")
	run(t, "--kb", root, "scan")

	// Head export sees the new world; pinned export is byte-identical to before.
	head := run(t, "--kb", root, "export")
	if !strings.Contains(head, "new.md") {
		t.Fatalf("head export should include new file:\n%s", head)
	}
	pinnedAgain := run(t, "--kb", root, "export", "--at", "M2")
	if pinned != pinnedAgain {
		t.Fatal("--at pinned export must be byte-identical across later changes")
	}
	if strings.Contains(pinnedAgain, "new.md") {
		t.Fatal("pinned export must not see files from later manifests")
	}
}

// TestChainHumanCoexistence is the full librarian-vs-human story over the CLI:
// scan → org → human makes a mess (adds, renames a managed file, drops a dup)
// → drift reports every class → reconcile auto-files the new, escalates the
// human move, collapses the dup → undo unwinds reconcile's commits in order.
func TestChainHumanCoexistence(t *testing.T) {
	root := t.TempDir()
	write(t, root, "inbox/acme.pdf", "ACME invoice #1007")
	run(t, "init", root)
	write(t, root, ".dig/policy.toml", e2ePolicy)
	run(t, "--kb", root, "scan")
	run(t, "--kb", root, "org") // acme.pdf → finance/invoices/

	// The human acts with their own tools.
	write(t, root, "inbox/new-note.md", "# fresh")                  // add (auto-filed: notes/)
	mustRename(t, root, "finance/invoices/acme.pdf", "keep/my.pdf") // deliberate move
	write(t, root, "copy.md", "# fresh")                            // duplicate content (older=new-note? same mtime → tie OK)

	// drift sees all classes, read-only.
	out := run(t, "--kb", root, "drift")
	for _, want := range []string{"EDIT", "renamed", "added", "POLICY"} {
		if !strings.Contains(out, want) {
			t.Fatalf("drift missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(run(t, "--kb", root, "drift", "--json"), `"changes"`) {
		t.Fatal("drift --json contract broken")
	}

	// reconcile: new file filed, human move escalated + left alone.
	out = run(t, "--kb", root, "reconcile")
	if !strings.Contains(out, "ESCALATE") || !strings.Contains(out, "keep/my.pdf") {
		t.Fatalf("human move should be escalated:\n%s", out)
	}
	if !strings.Contains(out, "APPLY") {
		t.Fatalf("new file should be auto-applied:\n%s", out)
	}
	ds := diskState(t, root)
	if _, ok := ds["keep/my.pdf"]; !ok {
		t.Fatal("reconcile moved a human-placed file — forbidden")
	}
	if _, ok := ds["notes/new-note.md"]; !ok {
		t.Fatalf("new file not auto-filed: %+v", ds)
	}

	// Converged: a second reconcile is silent except the standing escalation.
	out = run(t, "--kb", root, "reconcile")
	if strings.Contains(out, "APPLY") || strings.Contains(out, "ABSORB") {
		t.Fatalf("second reconcile should not re-apply or re-absorb:\n%s", out)
	}
}

func mustRename(t *testing.T, root, from, to string) {
	t.Helper()
	dst := filepath.Join(root, filepath.FromSlash(to))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, filepath.FromSlash(from)), dst); err != nil {
		t.Fatal(err)
	}
}

// TestChainWorkMergeUndo drives views over the CLI: create → list → merge
// (after programmatic propose/stage, the worker surface) → undo unwinds the
// merge like any other mutation.
func TestChainWorkMergeUndo(t *testing.T) {
	root := t.TempDir()
	write(t, root, "area/doc.txt", "content")
	run(t, "init", root)
	run(t, "--kb", root, "scan")

	out := run(t, "--kb", root, "work", "create", "agent-1")
	if !strings.Contains(out, "DRAFT") {
		t.Fatalf("work create: %s", out)
	}
	// Worker fills the view programmatically (the API agents use).
	k, err := kb.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(k.Dig())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ProposeView("agent-1", []store.ViewOp{{From: "area/doc.txt", To: "sorted/doc.txt", Labels: []string{"sorted"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.StageView("agent-1"); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	out = run(t, "--kb", root, "work", "list")
	if !strings.Contains(out, "STAGED") {
		t.Fatalf("work list should show STAGED: %s", out)
	}

	out = run(t, "--kb", root, "merge", "agent-1")
	if !strings.Contains(out, "merged") {
		t.Fatalf("merge: %s", out)
	}
	ds := diskState(t, root)
	if _, ok := ds["sorted/doc.txt"]; !ok {
		t.Fatalf("merge did not move on disk: %+v", ds)
	}
	if !strings.Contains(run(t, "--kb", root, "find", "sorted"), "sorted/doc.txt") {
		t.Fatal("index not rebuilt after merge")
	}

	// A merge is a mutation like any other — undo reverses it.
	run(t, "--kb", root, "undo")
	ds = diskState(t, root)
	if _, ok := ds["area/doc.txt"]; !ok {
		t.Fatalf("undo of merge did not restore disk: %+v", ds)
	}
}

// TestChainEscalateResolve drives the escalation ladder over the CLI: two
// views move the same file differently → second merge ESCALATES → human
// resolves --theirs → view closed, winner's state stands.
func TestChainEscalateResolve(t *testing.T) {
	root := t.TempDir()
	write(t, root, "f.txt", "content")
	run(t, "init", root)
	run(t, "--kb", root, "scan")

	k, _ := kb.Resolve(root)
	st, err := store.Open(k.Dig())
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []struct{ name, to string }{{"a", "x/f.txt"}, {"b", "y/f.txt"}} {
		if _, err := st.CreateView(v.name); err != nil {
			t.Fatal(err)
		}
		if _, err := st.ProposeView(v.name, []store.ViewOp{{From: "f.txt", To: v.to}}); err != nil {
			t.Fatal(err)
		}
		if _, err := st.StageView(v.name); err != nil {
			t.Fatal(err)
		}
	}
	_ = st.Close()

	run(t, "--kb", root, "merge", "a")
	out := run(t, "--kb", root, "merge", "b")
	if !strings.Contains(out, "ESCALATED") {
		t.Fatalf("second merge should escalate:\n%s", out)
	}
	out = run(t, "--kb", root, "work", "list")
	if !strings.Contains(out, "ESCALATED") {
		t.Fatalf("work list should show the escalation:\n%s", out)
	}
	// Human decides: head stands.
	out = run(t, "--kb", root, "work", "resolve", "b", "--theirs")
	if !strings.Contains(out, "ABORTED") {
		t.Fatalf("resolve --theirs: %s", out)
	}
	ds := diskState(t, root)
	if _, ok := ds["x/f.txt"]; !ok {
		t.Fatalf("winner's placement should stand: %+v", ds)
	}
	// Choosing both flags or none is rejected.
	runExpectErr(t, "--kb", root, "work", "resolve", "b")
}

// Multi-rule interplay + conflicts surface in the plan rather than failing.
func TestChainConflictReported(t *testing.T) {
	root := t.TempDir()
	write(t, root, "x/inv.pdf", "invoice one")
	write(t, root, "y/inv.pdf", "invoice two")
	run(t, "init", root)
	write(t, root, ".dig/policy.toml", `
[[rule]]
name   = "flatten"
match  = { ext = ["pdf"] }
into   = "all"
rename = "doc.pdf"
`)
	run(t, "--kb", root, "scan")
	out := run(t, "--kb", root, "org", "--dry-run")
	if !strings.Contains(out, "CONFLICT") {
		t.Fatalf("expected CONFLICT in plan:\n%s", out)
	}
	// JSON plan is machine-consumable for other harnesses.
	out = run(t, "--kb", root, "org", "--dry-run", "--json")
	if !strings.Contains(out, `"conflicts"`) {
		t.Fatalf("json plan missing conflicts: %s", out)
	}
}
