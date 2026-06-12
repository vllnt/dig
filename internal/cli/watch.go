package cli

import (
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/vllnt/dig/internal/drift"
	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/store"
	"github.com/vllnt/dig/internal/watch"
)

func newWatchCmd() *cobra.Command {
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Run as a harness: observe edits + reconcile continuously",
		Long: "Polls the KB, absorbs external edits, applies rules marked\n" +
			"autonomy = \"auto\", proposes everything else, and surfaces the\n" +
			"escalation queue. Duplicates are reported, never auto-removed.\n" +
			"Stop with Ctrl-C; every applied change remains undoable.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			rules, dpol, err := loadRules(k)
			if err != nil {
				return err
			}
			st, err := store.Open(k.Dig())
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "watching %s every %s — Ctrl-C to stop\n", k.Root, interval)
			return watch.Run(ctx, k, st, rules, dpol, watch.Options{
				Interval:  interval,
				Retrieval: loadRetrieval(k.Dig()),
				Warn:      cmd.ErrOrStderr(),
				OnPass: func(sum *drift.Summary, escalated []*store.View) {
					watch.Render(cmd.OutOrStdout(), sum, escalated)
				},
			})
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "poll cadence")
	return cmd
}
