package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bntvllnt/dig/internal/kb"
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
