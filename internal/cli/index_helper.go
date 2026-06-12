package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/vllnt/dig/internal/index"
	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/store"
	"github.com/vllnt/dig/internal/vector"
)

// inlineEmbedBudget bounds how many blobs a foreground command embeds before
// deferring the rest to the background (`dig embed` / `dig watch`). Small KBs
// stay seamless; large scans return instantly instead of blocking for hours.
const inlineEmbedBudget = 64

// rebuildIndex refreshes the KB's search index from a manifest, feeding file
// text from the blob store so find matches content, not just paths. The one
// path every command that moves the head goes through. When a [retrieval]
// policy is enabled the vector docs view syncs too (instant, no network) and
// a small embedding budget drains inline — embedding failures warn instead of
// failing the command (graceful degradation, architecture.md §5): the
// deterministic path never depends on an endpoint being up.
func rebuildIndex(digDir string, st *store.Store, m *store.Manifest, warn io.Writer) error {
	idx, err := index.Open(digDir)
	if err != nil {
		return err
	}
	defer func() { _ = idx.Close() }()
	if err := idx.Rebuild(m, index.BlobContent(st.Blobs())); err != nil {
		return err
	}

	rp := loadRetrieval(digDir)
	if !rp.Enabled() {
		return nil
	}
	if warn == nil {
		warn = io.Discard
	}
	vx, err := vector.Open(digDir)
	if err != nil {
		return err
	}
	defer func() { _ = vx.Close() }()
	client := vector.NewClient(rp.BaseURL, rp.Model, rp.APIKeyEnv, rp.DocPrefix, rp.QueryPrefix)
	if _, err := vx.SyncDocs(m, client); err != nil {
		return err
	}
	_, remaining, err := vx.DrainPending(index.BlobContent(st.Blobs()), client, inlineEmbedBudget)
	if err != nil {
		_, _ = fmt.Fprintf(warn, "warning: semantic index stale (embedding failed: %v)\n", err)
		return nil
	}
	if remaining > 0 {
		_, _ = fmt.Fprintf(warn, "semantic index: %d file(s) pending — run 'dig embed' (or keep 'dig watch' running) to finish in the background\n", remaining)
	}
	return nil
}

// loadRetrieval reads the [retrieval] section of the KB's policy, if any.
// A missing or invalid policy file means retrieval stays off — policy errors
// surface loudly on the policy/org paths, never silently break find or scan.
func loadRetrieval(digDir string) policy.RetrievalPolicy {
	p, err := policy.Load(filepath.Join(digDir, policy.File))
	if err != nil {
		return policy.RetrievalPolicy{}
	}
	return p.Retrieval
}
