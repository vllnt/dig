package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vllnt/dig/internal/index"
	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/store"
	"github.com/vllnt/dig/internal/vector"
)

// drainBatch is how many blobs dig embed processes between progress lines.
const drainBatch = 32

func newEmbedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "embed",
		Short: "Drain the semantic-index backlog (resumable)",
		Long: "Embeds files still pending in the vector index, committing per file —\n" +
			"safe to interrupt and re-run; completed work is never lost. Requires a\n" +
			"[retrieval] policy. Foreground commands only embed a small budget inline;\n" +
			"this command (or a running 'dig watch') finishes the rest.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			rp := loadRetrieval(k.Dig())
			if !rp.Enabled() {
				return fmt.Errorf("semantic retrieval is off — set [retrieval] mode = \"hybrid\" or \"vector\" in policy.toml")
			}
			st, err := store.Open(k.Dig())
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			vx, err := vector.Open(k.Dig())
			if err != nil {
				return err
			}
			defer func() { _ = vx.Close() }()

			_, _, chunkSize, chunkOverlap := rp.Tuning()
			client := vector.NewClient(rp.BaseURL, rp.Model, rp.APIKeyEnv, rp.DocPrefix, rp.QueryPrefix, chunkSize, chunkOverlap)
			head, err := st.Head()
			if err == nil {
				if _, err := vx.SyncDocs(head, client); err != nil {
					return err
				}
			}
			content := index.BlobContent(st.Blobs())
			total := 0
			for {
				done, remaining, err := vx.DrainPending(content, client, drainBatch)
				total += done
				if err != nil {
					return fmt.Errorf("after %d file(s): %w", total, err)
				}
				if remaining == 0 {
					break
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "embedded %d file(s), %d remaining\n", total, remaining)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "semantic index complete — %d file(s) embedded this run\n", total)
			return nil
		},
	}
}
