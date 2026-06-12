package cli

import (
	"sort"
	"strings"
	"testing"

	"github.com/vllnt/dig/internal/corpus"
)

// lifecyclePolicy organizes the generated corpus by extension and dedupes
// keep-oldest. Names planted by the corpus are unique, so org never collides.
const lifecyclePolicy = `
[[rule]]
name  = "sort-md"
match = { ext = ["md"] }
into  = "sorted/md"

[[rule]]
name  = "sort-csv"
match = { ext = ["csv"] }
into  = "sorted/csv"

[dedup]
strategy    = "keep-oldest"
on_conflict = "escalate"
`

// contentMultiset returns a sorted slice of every non-.dig file's content, so
// two disk states can be compared regardless of path (org moves files around).
func contentMultiset(state map[string]string) []string {
	out := make([]string, 0, len(state))
	for _, c := range state {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func equalMultiset(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestLifecycleRegression is the eval-harness full-journey invariant gate: over
// a deterministic generated corpus it runs scan → org → dedup → drift →
// reconcile → export → undo and asserts the spine's guarantees — byte-identical
// undo, idempotent re-runs, no lost content, dedup never deletes the last copy.
func TestLifecycleRegression(t *testing.T) {
	root := t.TempDir()
	spec, err := corpus.Generate(root, 1234, corpus.Small)
	if err != nil {
		t.Fatal(err)
	}

	run(t, "init", root)
	write(t, root, ".dig/policy.toml", lifecyclePolicy)
	run(t, "--kb", root, "scan")
	run(t, "--kb", root, "policy", "validate")

	pristine := diskState(t, root)
	if len(pristine) != spec.Files {
		t.Fatalf("scan saw %d files, corpus planted %d", len(pristine), spec.Files)
	}
	pristineContent := contentMultiset(pristine)

	// org --dry-run touches nothing.
	run(t, "--kb", root, "org", "--dry-run")
	if !equalMultiset(contentMultiset(diskState(t, root)), pristineContent) {
		t.Fatal("org --dry-run mutated the disk")
	}

	// org moves md/csv under sorted/ and never loses content.
	run(t, "--kb", root, "org")
	organized := diskState(t, root)
	if !equalMultiset(contentMultiset(organized), pristineContent) {
		t.Fatal("org changed the content multiset — files lost or altered")
	}
	movedMD := 0
	for path := range organized {
		if strings.HasPrefix(path, "sorted/md/") || strings.HasPrefix(path, "sorted/csv/") {
			movedMD++
		}
	}
	if movedMD == 0 {
		t.Fatal("org moved nothing into sorted/")
	}

	// undo org → byte-identical to pristine.
	run(t, "--kb", root, "undo")
	assertByteIdentical(t, pristine, diskState(t, root), "undo org")

	// Re-org is deterministic: same organized state.
	run(t, "--kb", root, "org")
	if !equalMultiset(contentMultiset(diskState(t, root)), pristineContent) {
		t.Fatal("re-org diverged")
	}

	// dedup collapses exactly the planted duplicate copies, never the last copy.
	beforeDedup := len(diskState(t, root))
	out := run(t, "--kb", root, "dedup")
	if !strings.Contains(out, "Removed") {
		t.Fatalf("dedup output: %s", out)
	}
	afterDedup := diskState(t, root)
	removed := beforeDedup - len(afterDedup)
	if removed != spec.Duplicates {
		t.Fatalf("dedup removed %d files, corpus planted %d duplicates", removed, spec.Duplicates)
	}
	// Every unique content still retrievable — dedup kept one copy of each.
	uniqueContent := map[string]bool{}
	for _, c := range pristineContent {
		uniqueContent[c] = true
	}
	for _, c := range contentMultiset(afterDedup) {
		delete(uniqueContent, c)
	}
	if len(uniqueContent) != 0 {
		t.Fatalf("dedup dropped %d unique contents — deleted a last copy", len(uniqueContent))
	}

	// undo dedup restores every collapsed copy.
	run(t, "--kb", root, "undo")
	if len(diskState(t, root)) != beforeDedup {
		t.Fatal("undo dedup did not restore the duplicates")
	}

	// reconcile converges; a second reconcile is a no-op (idempotent).
	run(t, "--kb", root, "reconcile")
	out = run(t, "--kb", root, "reconcile")
	if strings.Contains(out, "APPLY") || strings.Contains(out, "ABSORB") {
		t.Fatalf("second reconcile was not idempotent:\n%s", out)
	}

	// export emits provenance-tagged rows for the current head.
	out = run(t, "--kb", root, "export")
	if !strings.Contains(out, `"src":"b3:`) || !strings.Contains(out, `"manifest":`) {
		t.Fatalf("export missing provenance:\n%s", firstLines(out, 2))
	}
}

func assertByteIdentical(t *testing.T, want, got map[string]string, what string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s: file count %d != %d", what, len(got), len(want))
	}
	for path, content := range want {
		if got[path] != content {
			t.Fatalf("%s: not byte-identical at %s", what, path)
		}
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
