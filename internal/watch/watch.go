// Package watch runs dig as a continuous harness: observe → reconcile →
// surface, on an interval, until the context ends. It is a polling loop —
// deliberately: identical behavior on every OS, no watcher bookkeeping, and
// reconcile is already incremental in effect (a quiet tick commits nothing).
// An inotify-based trigger is a later optimization, not a semantic change.
//
// Autonomy in watch mode is earned rule-by-rule (drift.ModeWatch): only rules
// marked autonomy = "auto" act; everything else is proposed and surfaced.
package watch

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/vllnt/dig/internal/drift"
	"github.com/vllnt/dig/internal/index"
	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/store"
	"github.com/vllnt/dig/internal/vector"
)

// Options configures the loop.
type Options struct {
	Interval  time.Duration                       // poll cadence (default 2s)
	OnPass    func(*drift.Summary, []*store.View) // called after every non-empty pass (and the first)
	Retrieval policy.RetrievalPolicy              // when enabled, watch drains the semantic-index backlog each tick
	Warn      io.Writer                           // soft-error sink (embedding failures); nil = discard
}

// embedBudgetPerTick bounds background embedding work per watch tick so the
// reconcile loop never starves — the backlog drains steadily between passes.
const embedBudgetPerTick = 32

// Run loops until ctx is done. Each tick reconciles in watch mode, rebuilds
// the index when the head moved, and reports the escalation queue (ESCALATED
// views + standing pins) so a human can act. Errors are returned (the caller
// decides whether to restart); a clean ctx cancel returns nil.
func Run(ctx context.Context, k kb.KB, st *store.Store, rules []policy.CompiledRule, dpol policy.DedupPolicy, opts Options) error {
	interval := opts.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	warn := opts.Warn
	if warn == nil {
		warn = io.Discard
	}
	var embedClient *vector.Client
	if opts.Retrieval.Enabled() {
		_, _, chunkSize, chunkOverlap := opts.Retrieval.Tuning()
		embedClient = vector.NewClient(opts.Retrieval.BaseURL, opts.Retrieval.Model,
			opts.Retrieval.APIKeyEnv, opts.Retrieval.DocPrefix, opts.Retrieval.QueryPrefix, chunkSize, chunkOverlap)
	}
	lastEmbedErr := ""

	lastHead := ""
	surfaced := map[string]bool{} // standing items already shown — only deltas re-render
	for {
		sum, err := drift.Reconcile(k, st, rules, dpol, false, drift.ModeWatch)
		if err != nil {
			return fmt.Errorf("watch pass: %w", err)
		}
		dedupeStanding(sum, surfaced)
		headMoved := sum.Head != "" && sum.Head != lastHead
		if headMoved {
			head, err := st.Head()
			if err != nil {
				return err
			}
			idx, err := index.Open(k.Dig())
			if err != nil {
				return err
			}
			rebuildErr := idx.Rebuild(head, index.BlobContent(st.Blobs()))
			_ = idx.Close()
			if rebuildErr != nil {
				return rebuildErr
			}
			lastHead = sum.Head
		}
		// Background semantic indexing: sync the docs view when the head moved,
		// then drain a bounded slice of the embed backlog. Endpoint failures are
		// soft — the deterministic loop never depends on an endpoint being up.
		if embedClient != nil {
			if err := drainEmbedBacklog(k, st, embedClient, headMoved); err != nil {
				if err.Error() != lastEmbedErr {
					lastEmbedErr = err.Error()
					_, _ = fmt.Fprintf(warn, "watch: semantic index stalled (%v) — will keep retrying\n", err)
				}
			} else {
				lastEmbedErr = ""
			}
		}
		if opts.OnPass != nil && !sum.Empty() {
			views, _ := st.ListViews()
			escalated := views[:0:0]
			for _, v := range views {
				if v.State == store.StateEscalated && !surfaced["view:"+v.Name+":"+v.Conflict] {
					surfaced["view:"+v.Name+":"+v.Conflict] = true
					escalated = append(escalated, v)
				}
			}
			opts.OnPass(sum, escalated)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// drainEmbedBacklog advances the background semantic index by one bounded
// step: re-sync the docs view when the head moved, then embed up to
// embedBudgetPerTick pending blobs.
func drainEmbedBacklog(k kb.KB, st *store.Store, c *vector.Client, headMoved bool) error {
	vx, err := vector.Open(k.Dig())
	if err != nil {
		return err
	}
	defer func() { _ = vx.Close() }()
	if headMoved {
		head, err := st.Head()
		if err != nil {
			return err
		}
		if _, err := vx.SyncDocs(head, c); err != nil {
			return err
		}
	}
	_, _, err = vx.DrainPending(index.BlobContent(st.Blobs()), c, embedBudgetPerTick)
	return err
}

// dedupeStanding filters standing items (proposals, pins, pending dups) that
// were already surfaced — a long-running harness reports each once, not every
// tick. Absorbed/applied are events, never filtered.
func dedupeStanding(sum *drift.Summary, surfaced map[string]bool) {
	keep := func(key string) bool {
		if surfaced[key] {
			return false
		}
		surfaced[key] = true
		return true
	}
	proposed := sum.Proposed[:0:0]
	for _, op := range sum.Proposed {
		if keep("prop:" + op.From + "→" + op.To + strings.Join(op.Labels, ",")) {
			proposed = append(proposed, op)
		}
	}
	sum.Proposed = proposed
	pinned := sum.Escalated[:0:0]
	for _, op := range sum.Escalated {
		if keep("pin:" + op.From + "→" + op.To) {
			pinned = append(pinned, op)
		}
	}
	sum.Escalated = pinned
	dups := sum.DupPending[:0:0]
	for _, s := range sum.DupPending {
		if keep("dup:" + s.Blob) {
			dups = append(dups, s)
		}
	}
	sum.DupPending = dups
}

// OpTarget renders an op's effect: a move arrow or a label addition.
func OpTarget(from, to string, labels []string) string {
	if to != "" && to != from {
		return "→ " + to
	}
	return "+" + strings.Join(labels, ",")
}

// Render writes a human-readable pass summary — the escalation queue last so
// it is the thing a glancing human sees.
func Render(w io.Writer, sum *drift.Summary, escalated []*store.View) {
	for _, c := range sum.Absorbed {
		_, _ = fmt.Fprintf(w, "watch: absorbed %s %s\n", c.Kind, c.Path)
	}
	for _, op := range sum.Applied {
		_, _ = fmt.Fprintf(w, "watch: applied %s %s (%s)\n", op.From, OpTarget(op.From, op.To, op.Labels), op.Rule)
	}
	for _, op := range sum.Proposed {
		_, _ = fmt.Fprintf(w, "watch: PROPOSED %s %s (%s — autonomy not granted; run 'dig reconcile' to apply)\n", op.From, OpTarget(op.From, op.To, op.Labels), op.Rule)
	}
	for _, s := range sum.DupPending {
		_, _ = fmt.Fprintf(w, "watch: DUPLICATES %s ↔ %v (run 'dig dedup' to collapse)\n", s.Keep, s.Remove)
	}
	for _, op := range sum.Escalated {
		_, _ = fmt.Fprintf(w, "watch: PINNED %s (would → %s)\n", op.From, op.To)
	}
	for _, v := range escalated {
		_, _ = fmt.Fprintf(w, "watch: ESCALATED view %q — %s (dig work resolve %s --mine|--theirs)\n", v.Name, v.Conflict, v.Name)
	}
}
