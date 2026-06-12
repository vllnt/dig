package recall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vllnt/dig/internal/index"
	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/retrieval"
	"github.com/vllnt/dig/internal/scan"
	"github.com/vllnt/dig/internal/store"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rebuild(t *testing.T, dig string, st *store.Store, m *store.Manifest) {
	t.Helper()
	idx, err := index.Open(dig)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = idx.Close() }()
	if err := idx.Rebuild(m, index.BlobContent(st.Blobs())); err != nil {
		t.Fatal(err)
	}
}

// buildKB writes files, scans them, and returns the resolved KB.
func buildKB(t *testing.T, files map[string]string) kb.KB {
	t.Helper()
	root := t.TempDir()
	for path, content := range files {
		writeFile(t, root, path, content)
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
	entries, err := scan.Walk(k, st, false)
	if err != nil {
		t.Fatal(err)
	}
	m, err := st.Commit("scan", store.KindObserve, entries)
	if err != nil {
		t.Fatal(err)
	}
	rebuild(t, dig, st, m)
	_ = st.Close()
	return k
}

func TestBuildBudgetsAndProvenance(t *testing.T) {
	k := buildKB(t, map[string]string{
		"notes/renewal.md": "The ACME renewal is due in March; budget approved for the contract.",
		"notes/travel.md":  "Packing list for the autumn hiking trip in the mountains.",
		"notes/recipe.md":  "Slow-cooked lamb with rosemary and garlic.",
	})

	pack, err := Build(k, policy.RetrievalPolicy{}, retrieval.ModeFTS, "renewal contract budget", 2000)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Manifest == "" || pack.KB != k.Root {
		t.Fatalf("pack provenance missing: %+v", pack)
	}
	if len(pack.Items) == 0 || pack.Items[0].Path != "notes/renewal.md" {
		t.Fatalf("most relevant doc should lead the pack: %+v", pack.Items)
	}
	if !strings.Contains(pack.Items[0].Content, "renewal") {
		t.Fatalf("item content should carry the source text: %q", pack.Items[0].Content)
	}
}

func TestBuildRespectsBudget(t *testing.T) {
	big := strings.Repeat("renewal contract budget words here. ", 2000) // ~72k chars
	k := buildKB(t, map[string]string{"notes/big.md": big})

	pack, err := Build(k, policy.RetrievalPolicy{}, retrieval.ModeFTS, "renewal", 100) // 100 tokens
	if err != nil {
		t.Fatal(err)
	}
	if pack.UsedTokens > pack.BudgetTokens {
		t.Fatalf("used %d tokens over budget %d", pack.UsedTokens, pack.BudgetTokens)
	}
	total := 0
	for _, it := range pack.Items {
		total += len(it.Content)
	}
	if total > pack.BudgetTokens*charsPerToken {
		t.Fatalf("pack content %d chars exceeds budget %d", total, pack.BudgetTokens*charsPerToken)
	}
}

func TestBuildEmptyOnNoMatch(t *testing.T) {
	k := buildKB(t, map[string]string{"notes/a.md": "completely unrelated content"})
	pack, err := Build(k, policy.RetrievalPolicy{}, retrieval.ModeFTS, "zzz nonexistent term", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Items) != 0 {
		t.Fatalf("no match should give an empty pack: %+v", pack.Items)
	}
}

func TestBuildDefaultsBudget(t *testing.T) {
	k := buildKB(t, map[string]string{"notes/a.md": "renewal note"})
	pack, err := Build(k, policy.RetrievalPolicy{}, retrieval.ModeFTS, "renewal", 0)
	if err != nil {
		t.Fatal(err)
	}
	if pack.BudgetTokens != DefaultBudgetTokens {
		t.Fatalf("0 budget should default to %d, got %d", DefaultBudgetTokens, pack.BudgetTokens)
	}
}

// TestBuildLandsOnPassage proves recall returns the query-relevant window of a
// long document, not its head — the "lands on the exact exchange" behavior for
// captured sessions.
func TestBuildLandsOnPassage(t *testing.T) {
	head := strings.Repeat("HEADMARKER unrelated preamble chatter. ", 150) // ~5.7k chars
	passage := "The ACME ledger migration is owned by Dana, targeted for Q3."
	tail := strings.Repeat(" closing remarks and goodbyes. ", 50)
	k := buildKB(t, map[string]string{"memory/session.md": head + passage + tail})

	pack, err := Build(k, policy.RetrievalPolicy{}, retrieval.ModeFTS, "ledger migration Dana Q3", 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Items) == 0 {
		t.Fatal("expected the session to be recalled")
	}
	// The passage sits ~5.7k chars in; the per-item budget is only a few hundred
	// chars, so head-truncation (the old behavior) would return pure HEADMARKER
	// filler with no "Dana". Surfacing the passage at all proves windowing works.
	got := pack.Items[0].Content
	if !strings.Contains(got, "Dana") || !strings.Contains(got, "ledger migration") {
		t.Fatalf("recall did not land on the matching passage:\n%q", got)
	}
	if strings.HasPrefix(got, "HEADMARKER") {
		t.Fatalf("recall returned the document head instead of the passage:\n%q", got)
	}
}

// TestBuildWindowFallsBackToHead proves a matched document with no query term in
// its body (FTS matched on a label/path) still yields a head snippet, not empty.
func TestBuildWindowFallsBackToHead(t *testing.T) {
	long := "alpha beta " + strings.Repeat("ordinary sentence content here. ", 200)
	if got := bestWindow(long, "zzdoesnotappear", 300); got == "" || !strings.HasPrefix(long, got) {
		t.Fatalf("no-match window should fall back to the head prefix, got %q", got)
	}
	// Empty query → head.
	if got := bestWindow(long, "", 300); !strings.HasPrefix(long, got) {
		t.Fatalf("empty query should fall back to the head, got %q", got)
	}
	// Small doc → whole doc.
	if got := bestWindow("tiny doc", "doc", 300); got != "tiny doc" {
		t.Fatalf("small doc should return whole, got %q", got)
	}
}

func TestTruncateRuneSafe(t *testing.T) {
	s := "héllo wörld"
	for n := 0; n <= len(s)+2; n++ {
		out := truncate(s, n)
		if len(out) > n {
			t.Fatalf("truncate(%d) returned %d bytes", n, len(out))
		}
		if !strings.HasPrefix(s, out) {
			t.Fatalf("truncate must be a prefix: %q not in %q", out, s)
		}
	}
}
