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
	"time"

	"github.com/bntvllnt/dig/internal/drift"
	"github.com/bntvllnt/dig/internal/index"
	"github.com/bntvllnt/dig/internal/kb"
	"github.com/bntvllnt/dig/internal/policy"
	"github.com/bntvllnt/dig/internal/store"
)

// Options configures the loop.
type Options struct {
	Interval time.Duration                       // poll cadence (default 2s)
	OnPass   func(*drift.Summary, []*store.View) // called after every non-empty pass (and the first)
}

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

	lastHead := ""
	for {
		sum, err := drift.Reconcile(k, st, rules, dpol, false, drift.ModeWatch)
		if err != nil {
			return fmt.Errorf("watch pass: %w", err)
		}
		if sum.Head != "" && sum.Head != lastHead {
			head, err := st.Head()
			if err != nil {
				return err
			}
			idx, err := index.Open(k.Dig())
			if err != nil {
				return err
			}
			rebuildErr := idx.Rebuild(head)
			_ = idx.Close()
			if rebuildErr != nil {
				return rebuildErr
			}
			lastHead = sum.Head
		}
		if opts.OnPass != nil && !sum.Empty() {
			views, _ := st.ListViews()
			escalated := views[:0:0]
			for _, v := range views {
				if v.State == store.StateEscalated {
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

// Render writes a human-readable pass summary — the escalation queue last so
// it is the thing a glancing human sees.
func Render(w io.Writer, sum *drift.Summary, escalated []*store.View) {
	for _, c := range sum.Absorbed {
		_, _ = fmt.Fprintf(w, "watch: absorbed %s %s\n", c.Kind, c.Path)
	}
	for _, op := range sum.Applied {
		_, _ = fmt.Fprintf(w, "watch: applied %s → %s (%s)\n", op.From, op.To, op.Rule)
	}
	for _, op := range sum.Proposed {
		_, _ = fmt.Fprintf(w, "watch: PROPOSED %s → %s (%s — autonomy not granted; run 'dig reconcile' to apply)\n", op.From, op.To, op.Rule)
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
