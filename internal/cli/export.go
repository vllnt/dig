package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vllnt/dig/internal/export"
	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/store"
)

func newExportCmd() *cobra.Command {
	var filterExpr, format, at string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Emit a reproducible, manifest-pinned dataset (JSONL)",
		Long: "Streams the KB (or a filtered slice) as JSONL to stdout, one record per\n" +
			"file, with provenance (src blob + manifest id) on every row. Content is\n" +
			"read from the blob store, not the live tree: the same --at manifest\n" +
			"re-emits a byte-identical dataset regardless of disk changes since.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "jsonl" {
				return fmt.Errorf("unsupported format %q (supported: jsonl)", format)
			}
			f, err := export.ParseFilter(filterExpr)
			if err != nil {
				return err
			}
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			st, err := store.Open(k.Dig())
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			var m *store.Manifest
			if at == "" {
				m, err = st.Head()
			} else {
				m, err = st.Get(at)
				if err == nil && m == nil {
					err = fmt.Errorf("manifest %q not found (see 'dig log')", at)
				}
			}
			if err != nil {
				return err
			}

			n, err := export.Write(cmd.OutOrStdout(), st, m, f)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "exported %d record(s) from %s\n", n, m.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&filterExpr, "filter", "", `slice selection, e.g. "label:finance path:*.pdf after:2024-01-01"`)
	cmd.Flags().StringVar(&format, "format", "jsonl", "output format (jsonl)")
	cmd.Flags().StringVar(&at, "at", "", "pin to a manifest ID (e.g. M3) for reproducible export; default head")
	return cmd
}
