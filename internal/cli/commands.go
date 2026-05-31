package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bntvllnt/dig/internal/index"
	"github.com/bntvllnt/dig/internal/kb"
	"github.com/bntvllnt/dig/internal/scan"
	"github.com/bntvllnt/dig/internal/store"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <root>",
		Short: "Create a knowledge base at a directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Init(args[0])
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Initialized dig KB at %s\n", k.Root)
			return nil
		},
	}
}

func newScanCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Index files into the content-addressed store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			dig, err := k.EnsureDig()
			if err != nil {
				return err
			}
			st, err := store.Open(dig)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			entries, err := scan.Walk(k, st, dryRun)
			if err != nil {
				return err
			}
			if dryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would scan %d file(s); no changes written\n", len(entries))
				return nil
			}
			m, err := st.Commit("scan", entries)
			if err != nil {
				return err
			}
			idx, err := index.Open(dig)
			if err != nil {
				return err
			}
			defer func() { _ = idx.Close() }()
			if err := idx.Rebuild(m); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Scanned %d file(s) → manifest %s\n", len(entries), m.ID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without writing to the store")
	return cmd
}

func newFindCmd() *cobra.Command {
	var asJSON bool
	var limit int
	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Search the knowledge base, ranked results",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			idx, err := index.Open(k.Dig())
			if err != nil {
				return err
			}
			defer func() { _ = idx.Close() }()

			q := args[0]
			for _, a := range args[1:] {
				q += " " + a
			}
			results, err := idx.Query(q, limit)
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			if len(results) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no matches")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			for _, r := range results {
				labels := ""
				if len(r.Labels) > 0 {
					labels = fmt.Sprintf("%v", r.Labels)
				}
				_, _ = fmt.Fprintf(tw, "%s\t%s\n", r.Path, labels)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON for other harnesses")
	cmd.Flags().IntVar(&limit, "limit", 20, "max results")
	return cmd
}

func newLogCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Browse change history (newest first)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			st, err := store.Open(k.Dig())
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			hist, err := st.History()
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(hist)
			}
			if len(hist) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no history yet — run 'dig scan'")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			for _, m := range hist {
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%d entries\t%s\n",
					m.ID, m.CreatedBy, len(m.Entries), m.CreatedAt.Format("2006-01-02 15:04:05"))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON for other harnesses")
	return cmd
}

func newUndoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undo",
		Short: "Revert the last changeset (move head to its parent)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			dig := k.Dig()
			st, err := store.Open(dig)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			m, err := st.Undo()
			if err != nil {
				return err
			}
			idx, err := index.Open(dig)
			if err != nil {
				return err
			}
			defer func() { _ = idx.Close() }()
			if err := idx.Rebuild(m); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Reverted → head is now %s\n", m.ID)
			return nil
		},
	}
}
