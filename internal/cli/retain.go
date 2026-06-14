package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/store"
	"github.com/vllnt/dig/internal/transcript"
)

// newRetainCmd captures content (an agent session, a note, a document) into the
// KB and indexes it, so `dig find` / `dig recall` surface it later. It is the
// ingestion entry point agent-memory hooks pipe a transcript into. Reversible
// like any other change: a retain is a journaled changeset, `dig undo`-able.
func newRetainCmd() *cobra.Command {
	var asPath string
	var now string
	var transcriptPath string
	cmd := &cobra.Command{
		Use:   "retain [file]",
		Short: "Capture content (session, note, document) into the KB and index it",
		Long: "Writes content (a file argument, stdin, or a rendered agent transcript via\n" +
			"--transcript) into the KB and scans it in. Defaults to a dated memory/ path;\n" +
			"override with --as. The capture entry point for agent-memory: a retention\n" +
			"hook renders a finished session with --transcript and pipes it to `dig retain`.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			data, err := retainInput(cmd, args, transcriptPath)
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
			info, err := os.Stat(full)
			if err != nil {
				return err
			}

			st, err := store.Open(dig)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			// retain is a creation, not a re-scan: record exactly the new file on
			// top of the current head and commit it as a mutation. That keeps the
			// changeset's diff to its parent equal to {this file}, so `dig undo`
			// removes precisely what retain wrote and never disturbs unrelated
			// drift sitting un-indexed on disk.
			hash := store.HashBytes(data)
			if err := st.Blobs().Put(hash, bytes.NewReader(data)); err != nil {
				return fmt.Errorf("store %s: %w", rel, err)
			}
			head, err := st.Head()
			if err != nil {
				return err
			}
			created := store.Entry{
				Path:    filepath.ToSlash(rel),
				Blob:    hash,
				Size:    info.Size(),
				ModTime: info.ModTime().UTC(),
			}
			m, err := st.Commit("retain", store.KindMutate, mergeEntry(head, created))
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
	cmd.Flags().StringVar(&transcriptPath, "transcript", "", "render an agent session transcript (JSONL) to markdown and retain that")
	return cmd
}

// mergeEntry returns the head's entries with e added — or replaced, if a file
// already lives at its path — sorted by path for a deterministic manifest. A
// nil head (no commits yet) yields a single-entry manifest.
func mergeEntry(head *store.Manifest, e store.Entry) []store.Entry {
	var base []store.Entry
	if head != nil {
		base = head.Entries
	}
	out := make([]store.Entry, 0, len(base)+1)
	replaced := false
	for _, cur := range base {
		if cur.Path == e.Path {
			out = append(out, e)
			replaced = true
			continue
		}
		out = append(out, cur)
	}
	if !replaced {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// retainInput resolves the content to capture: a rendered transcript when
// --transcript is set, else a file argument, else stdin.
func retainInput(cmd *cobra.Command, args []string, transcriptPath string) ([]byte, error) {
	if transcriptPath != "" {
		if len(args) == 1 {
			return nil, fmt.Errorf("--transcript and a file argument are mutually exclusive")
		}
		f, err := os.Open(transcriptPath)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		md, err := transcript.Render(f)
		if err != nil {
			return nil, err
		}
		return []byte(md), nil
	}
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
