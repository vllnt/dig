package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/vllnt/dig/internal/index"
	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/sink"
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
	if warn == nil {
		warn = io.Discard
	}
	idx, err := index.Open(digDir)
	if err != nil {
		return err
	}
	defer func() { _ = idx.Close() }()
	if err := idx.Rebuild(m, index.BlobContent(st.Blobs())); err != nil {
		return err
	}

	p := loadPolicy(digDir)
	if p != nil && p.Retrieval.Enabled() {
		if err := rebuildVectors(digDir, st, m, p.Retrieval, warn); err != nil {
			return err
		}
	}
	if p != nil && len(p.EventSinks) > 0 {
		fireSinks(digDir, m, p.EventSinks, warn)
	}
	return nil
}

// rebuildVectors syncs the vector docs view and drains a small inline embedding
// budget; embedding failures warn (graceful degradation) rather than fail.
func rebuildVectors(digDir string, st *store.Store, m *store.Manifest, rp policy.RetrievalPolicy, warn io.Writer) error {
	vx, err := vector.Open(digDir)
	if err != nil {
		return err
	}
	defer func() { _ = vx.Close() }()
	_, _, chunkSize, chunkOverlap := rp.Tuning()
	client := vector.NewClient(rp.BaseURL, rp.Model, rp.APIKeyEnv, rp.DocPrefix, rp.QueryPrefix, chunkSize, chunkOverlap)
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

// fireSinks runs the KB's event sinks for a committed manifest. Sinks observe —
// failures warn but never affect the changeset (it is already committed).
func fireSinks(digDir string, m *store.Manifest, sinks []policy.EventSink, warn io.Writer) {
	ev := sink.Event{
		Event:     policy.EventCommitted,
		KB:        filepath.Dir(digDir),
		Manifest:  m.ID,
		Kind:      m.Kind,
		CreatedBy: m.CreatedBy,
		Entries:   len(m.Entries),
	}
	for _, err := range sink.Fire(sinks, ev) {
		_, _ = fmt.Fprintf(warn, "warning: %v\n", err)
	}
}

// loadPolicy reads the KB's policy, returning nil when absent or invalid —
// policy errors surface loudly on the policy/org paths, never silently break
// find or scan.
func loadPolicy(digDir string) *policy.Policy {
	p, err := policy.Load(filepath.Join(digDir, policy.File))
	if err != nil {
		return nil
	}
	return p
}

// loadRetrieval reads the [retrieval] section of the KB's policy, if any.
func loadRetrieval(digDir string) policy.RetrievalPolicy {
	if p := loadPolicy(digDir); p != nil {
		return p.Retrieval
	}
	return policy.RetrievalPolicy{}
}
