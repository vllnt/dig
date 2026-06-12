package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/scan"
	"github.com/vllnt/dig/internal/store"
)

// newRetainCmd captures content (an agent session, a note, a document) into the
// KB and indexes it, so `dig find` / `dig recall` surface it later. It is the
// ingestion entry point agent-memory hooks pipe a transcript into. Reversible
// like any other change: a retain is a journaled changeset, `dig undo`-able.
func newRetainCmd() *cobra.Command {
	var asPath string
	var now string
	cmd := &cobra.Command{
		Use:   "retain [file]",
		Short: "Capture content (session, note, document) into the KB and index it",
		Long: "Writes content (a file argument, or stdin) into the KB and scans it in.\n" +
			"Defaults to a dated memory/ path; override with --as. The capture entry\n" +
			"point for agent-memory: pipe a session transcript to `dig retain`.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			data, err := readRetainInput(cmd, args)
			if err != nil {
				return err
			}
			if len(data) == 0 {
				return fmt.Errorf("nothing to retain (empty input)")
			}

			rel := asPath
			if rel == "" {
				rel = defaultMemoryPath(data, now)
			}
			if err := safeKBPath(rel); err != nil {
				return err
			}

			dig, err := k.EnsureDig()
			if err != nil {
				return err
			}
			full := filepath.Join(k.Root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(full, data, 0o644); err != nil {
				return err
			}

			st, err := store.Open(dig)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			entries, err := scan.Walk(k, st, false)
			if err != nil {
				return err
			}
			m, err := st.Commit("retain", store.KindObserve, entries)
			if err != nil {
				return err
			}
			if err := rebuildIndex(dig, st, m, cmd.ErrOrStderr()); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Retained %s → manifest %s\n", rel, m.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&asPath, "as", "", "target path in the KB (default: memory/<date>/<hash>.md)")
	cmd.Flags().StringVar(&now, "date", "", "date for the default path as YYYY-MM-DD (default: today) — for reproducible captures")
	return cmd
}

// readRetainInput reads content from a file argument or stdin.
func readRetainInput(cmd *cobra.Command, args []string) ([]byte, error) {
	if len(args) == 1 {
		return os.ReadFile(args[0])
	}
	return io.ReadAll(cmd.InOrStdin())
}

// defaultMemoryPath builds a dated, content-addressed memory path. A fixed
// date (YYYY-MM-DD) can be supplied for reproducible captures; otherwise today.
func defaultMemoryPath(data []byte, date string) string {
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	parts := strings.SplitN(date, "-", 3)
	sum := sha256.Sum256(data)
	id := hex.EncodeToString(sum[:])[:8]
	if len(parts) == 3 {
		return fmt.Sprintf("memory/%s/%s/%s/%s.md", parts[0], parts[1], parts[2], id)
	}
	return fmt.Sprintf("memory/%s/%s.md", date, id)
}

// safeKBPath rejects absolute paths and any segment that escapes the KB root.
func safeKBPath(rel string) error {
	if rel == "" {
		return fmt.Errorf("--as path is empty")
	}
	if strings.HasPrefix(rel, "/") {
		return fmt.Errorf("--as %q: absolute paths not allowed", rel)
	}
	for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
		if seg == ".." {
			return fmt.Errorf("--as %q: path escape (..) not allowed", rel)
		}
	}
	if strings.HasPrefix(filepath.ToSlash(rel), kb.DigDir+"/") {
		return fmt.Errorf("--as %q: cannot write into the .dig directory", rel)
	}
	return nil
}
