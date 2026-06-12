// Package recall assembles a token-budgeted, provenance-tagged context pack
// from a knowledge base — the budgeted-recall primitive an agent calls to load
// "what do I know about X" without blowing its context window. It is built on
// the same retrieval the CLI's find uses; recall just ranks, snippets, and caps
// to a budget.
package recall

import (
	"unicode/utf8"

	"github.com/vllnt/dig/internal/index"
	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/retrieval"
	"github.com/vllnt/dig/internal/store"
)

// charsPerToken approximates token budgets in characters (~4 chars/token).
// Deliberately rough — the goal is a predictable cap, not exact tokenization.
const charsPerToken = 4

// DefaultBudgetTokens is the recall budget when none is given.
const DefaultBudgetTokens = 4000

// Item is one retrieved source in a pack, snippet + provenance.
type Item struct {
	Path    string  `json:"path"`
	Blob    string  `json:"blob"`
	Score   float32 `json:"score,omitempty"`
	Content string  `json:"content"`
}

// Pack is a budgeted recall result. Manifest pins the head it was drawn from,
// so a pack is reproducible.
type Pack struct {
	Query        string `json:"query"`
	KB           string `json:"kb"`
	Manifest     string `json:"manifest"`
	BudgetTokens int    `json:"budgetTokens"`
	UsedTokens   int    `json:"usedTokens"`
	Items        []Item `json:"items"`
}

// Build assembles a pack: rank the KB by query (mode from policy/override),
// pull each hit's text from the blob store, and accumulate snippets newest-rank
// first until the token budget is spent. content is the blob reader (so recall
// stays a derived view of the manifest, never live disk).
func Build(k kb.KB, rp policy.RetrievalPolicy, mode retrieval.Mode, query string, budgetTokens int) (*Pack, error) {
	if budgetTokens <= 0 {
		budgetTokens = DefaultBudgetTokens
	}
	digDir := k.Dig()

	st, err := store.Open(digDir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = st.Close() }()
	head, err := st.Head()
	if err != nil {
		return nil, err
	}
	content := index.BlobContent(st.Blobs())

	// Pull a generous candidate pool; the budget, not the count, bounds output.
	results, err := retrieval.Search(digDir, rp, mode, query, 50)
	if err != nil {
		return nil, err
	}

	pack := &Pack{
		Query:        query,
		KB:           k.Root,
		Manifest:     manifestID(head),
		BudgetTokens: budgetTokens,
	}
	budgetChars := budgetTokens * charsPerToken
	// Per-item cap keeps one big document from eating the whole budget.
	perItemChars := budgetChars / 3
	if perItemChars < charsPerToken {
		perItemChars = budgetChars
	}
	used := 0
	for _, r := range results {
		if used >= budgetChars {
			break
		}
		text, ok := content(r.Blob)
		if !ok {
			continue // binary / unreadable — not recallable context
		}
		remaining := budgetChars - used
		limit := perItemChars
		if limit > remaining {
			limit = remaining
		}
		snippet := truncate(string(text), limit)
		if snippet == "" {
			continue
		}
		used += len(snippet)
		pack.Items = append(pack.Items, Item{
			Path:    r.Path,
			Blob:    r.Blob,
			Score:   r.Score,
			Content: snippet,
		})
	}
	pack.UsedTokens = used / charsPerToken
	return pack, nil
}

// truncate caps s to at most n bytes, backing off to a rune boundary so it
// never splits a multibyte character.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func manifestID(m *store.Manifest) string {
	if m == nil {
		return ""
	}
	return m.ID
}
