package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/recall"
	"github.com/vllnt/dig/internal/retrieval"
)

// newRecallCmd emits a token-budgeted, provenance-tagged context pack for a
// query — the recall primitive an agent uses to load relevant memory without
// overflowing its context window. It is retrieval (find) shaped into a bounded
// bundle: ranked hits, snippets, and a budget cap.
func newRecallCmd() *cobra.Command {
	var asJSON bool
	var budget int
	var modeFlag string
	cmd := &cobra.Command{
		Use:   "recall <query>",
		Short: "Emit a token-budgeted context pack for a query",
		Long: "Ranks the KB by the query and assembles a budgeted, provenance-tagged\n" +
			"context bundle (snippets + their paths, pinned to the head manifest). Use\n" +
			"--mode hybrid for semantic recall and --budget to size the bundle.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			rp := loadRetrieval(k.Dig())
			if modeFlag == "" && rp.Enabled() {
				modeFlag = rp.Mode
			}
			mode, err := retrieval.ParseMode(modeFlag)
			if err != nil {
				return err
			}
			q := args[0]
			for _, a := range args[1:] {
				q += " " + a
			}
			pack, err := recall.Build(k, rp, mode, q, budget)
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(pack)
			}
			return renderPack(cmd, pack)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the pack as JSON")
	cmd.Flags().IntVar(&budget, "budget", recall.DefaultBudgetTokens, "token budget for the pack")
	cmd.Flags().StringVar(&modeFlag, "mode", "", "retrieval mode: fts, vector, hybrid (default: policy mode, else fts)")
	return cmd
}

func renderPack(cmd *cobra.Command, pack *recall.Pack) error {
	out := cmd.OutOrStdout()
	if len(pack.Items) == 0 {
		_, _ = fmt.Fprintln(out, "no relevant memory found")
		return nil
	}
	_, _ = fmt.Fprintf(out, "# recall: %q  (~%d/%d tokens, %s)\n\n",
		pack.Query, pack.UsedTokens, pack.BudgetTokens, pack.Manifest)
	for _, it := range pack.Items {
		_, _ = fmt.Fprintf(out, "## %s\n%s\n\n", it.Path, it.Content)
	}
	return nil
}
