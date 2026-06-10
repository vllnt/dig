package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bntvllnt/dig/internal/index"
	"github.com/bntvllnt/dig/internal/kb"
	"github.com/bntvllnt/dig/internal/store"
)

func newWorkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work",
		Short: "Manage isolated work views (worktree-like)",
		Long:  "A view is an isolated unit of work: a pointer to a base manifest plus\nproposed ops. Workers fill views in parallel; 'dig merge' folds them back —\ndisjoint changes automatically, overlaps as conflicts.",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "create <name>",
		Short: "Open a new view on the current head",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, closeFn, err := openStore()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := st.CreateView(args[0])
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "view %q opened on %s (%s)\n", v.Name, v.Base, v.State)
			return nil
		},
	})

	var asJSON bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List views and their states",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, closeFn, err := openStore()
			if err != nil {
				return err
			}
			defer closeFn()
			views, err := st.ListViews()
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(views)
			}
			if len(views) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no views — open one with 'dig work create <name>'")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			for _, v := range views {
				extra := ""
				if v.Conflict != "" {
					extra = v.Conflict
				}
				_, _ = fmt.Fprintf(tw, "%s\t%s\tbase %s\t%d op(s)\t%s\n", v.Name, v.State, v.Base, len(v.Ops), extra)
			}
			return tw.Flush()
		},
	}
	list.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "abort <name>",
		Short: "Discard a view (head and disk untouched)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, closeFn, err := openStore()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := st.AbortView(args[0])
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "view %q aborted\n", v.Name)
			return nil
		},
	})
	return cmd
}

func newMergeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "merge <view>",
		Short: "Merge a staged view back; disjoint auto-merges, overlap conflicts",
		Args:  cobra.ExactArgs(1),
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

			v, m, err := st.MergeView(k.Root, args[0])
			if err != nil {
				return err
			}
			if v.State == store.StateConflict {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "CONFLICT: %s — view %q held for resolution\n", v.Conflict, v.Name)
				return nil
			}
			idx, err := index.Open(k.Dig())
			if err != nil {
				return err
			}
			defer func() { _ = idx.Close() }()
			if err := idx.Rebuild(m); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "merged %q → manifest %s\n", v.Name, m.ID)
			return nil
		},
	}
}

// openStore resolves the KB and opens its store — shared by the small view
// subcommands that don't need the root path.
func openStore() (*store.Store, func(), error) {
	k, err := kb.Resolve(kbFlag)
	if err != nil {
		return nil, nil, err
	}
	st, err := store.Open(k.Dig())
	if err != nil {
		return nil, nil, err
	}
	return st, func() { _ = st.Close() }, nil
}
