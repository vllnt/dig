package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vllnt/dig/internal/recall"
)

// TestChainRecall drives recall over the real CLI: a relevant query returns a
// budgeted, provenance-tagged pack (text + JSON); an irrelevant query is empty.
func TestChainRecall(t *testing.T) {
	root := t.TempDir()
	write(t, root, "notes/renewal.md", "The ACME renewal is due in March; the contract budget was approved.")
	write(t, root, "notes/recipe.md", "Slow-cooked lamb shoulder with rosemary and garlic.")
	run(t, "init", root)
	run(t, "--kb", root, "scan")

	// text pack leads with the relevant note and shows provenance + budget.
	out := run(t, "--kb", root, "recall", "renewal contract budget")
	if !strings.Contains(out, "notes/renewal.md") || !strings.Contains(out, "renewal is due") {
		t.Fatalf("recall text pack missing the relevant note:\n%s", out)
	}
	if !strings.Contains(out, "tokens") || !strings.Contains(out, "M1") {
		t.Fatalf("recall should show budget + manifest provenance:\n%s", out)
	}

	// JSON pack is machine-consumable with budget accounting.
	out = run(t, "--kb", root, "recall", "renewal", "--json", "--budget", "500")
	var pack recall.Pack
	if err := json.Unmarshal([]byte(out), &pack); err != nil {
		t.Fatalf("recall --json invalid: %v\n%s", err, out)
	}
	if pack.BudgetTokens != 500 || pack.Manifest == "" || len(pack.Items) == 0 {
		t.Fatalf("recall pack wrong: %+v", pack)
	}
	if pack.Items[0].Path != "notes/renewal.md" {
		t.Fatalf("most relevant should lead: %+v", pack.Items)
	}

	// Irrelevant query → empty pack, not an error.
	out = run(t, "--kb", root, "recall", "zzzz nonexistent")
	if !strings.Contains(out, "no relevant memory") {
		t.Fatalf("irrelevant recall should be empty: %s", out)
	}
}
