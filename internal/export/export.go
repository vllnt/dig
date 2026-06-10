// Package export emits a KB slice as a reproducible, provenance-tagged
// dataset. Reproducibility is the contract: records are built from the
// MANIFEST and the BLOB STORE, never from live disk — so the same manifest
// re-emits a byte-identical dataset months later, regardless of what happened
// to the files since. Every row traces back to its source blob and manifest.
package export

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bntvllnt/dig/internal/store"
)

// Record is one dataset row. Field order is fixed by the struct — part of the
// byte-identical guarantee.
type Record struct {
	Path     string   `json:"path"`
	Text     string   `json:"text,omitempty"`
	Binary   bool     `json:"binary,omitempty"`
	Labels   []string `json:"labels,omitempty"`
	Size     int64    `json:"size"`
	ModTime  string   `json:"mod_time"` // RFC3339 UTC
	Src      string   `json:"src"`      // content hash — provenance to the blob
	Manifest string   `json:"manifest"` // manifest ID — provenance to the KB version
}

// Filter is a compiled predicate over manifest entries.
type Filter struct {
	labels []string
	globs  []string
	after  time.Time
	before time.Time
}

// ParseFilter compiles a filter expression: whitespace-separated terms, all of
// which must hold (AND). Terms:
//
//	label:<name>        entry carries the label
//	path:<glob>         KB-relative path (or basename) matches the glob
//	after:<YYYY-MM-DD>  modified strictly after the date (UTC)
//	before:<YYYY-MM-DD> modified strictly before the date (UTC)
//
// Unknown keys are errors — a typo silently exporting everything is worse
// than failing loudly.
func ParseFilter(expr string) (*Filter, error) {
	f := &Filter{}
	for _, term := range strings.Fields(expr) {
		key, val, ok := strings.Cut(term, ":")
		if !ok || val == "" {
			return nil, fmt.Errorf("filter term %q: want key:value (label:, path:, after:, before:)", term)
		}
		switch key {
		case "label":
			f.labels = append(f.labels, val)
		case "path":
			if _, err := path.Match(val, "probe"); err != nil {
				return nil, fmt.Errorf("filter path glob %q: %w", val, err)
			}
			f.globs = append(f.globs, val)
		case "after":
			t, err := time.Parse("2006-01-02", val)
			if err != nil {
				return nil, fmt.Errorf("filter after %q: want YYYY-MM-DD", val)
			}
			f.after = t
		case "before":
			t, err := time.Parse("2006-01-02", val)
			if err != nil {
				return nil, fmt.Errorf("filter before %q: want YYYY-MM-DD", val)
			}
			f.before = t
		default:
			return nil, fmt.Errorf("unknown filter key %q (known: label, path, after, before)", key)
		}
	}
	return f, nil
}

// Match reports whether the entry passes every filter term.
func (f *Filter) Match(e store.Entry) bool {
	for _, want := range f.labels {
		found := false
		for _, l := range e.Labels {
			if l == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, g := range f.globs {
		ok, _ := path.Match(g, e.Path)
		if !ok {
			if ok2, _ := path.Match(g, path.Base(e.Path)); !ok2 {
				return false
			}
		}
	}
	if !f.after.IsZero() && !e.ModTime.After(f.after) {
		return false
	}
	if !f.before.IsZero() && !e.ModTime.Before(f.before) {
		return false
	}
	return true
}

// maxTextBytes caps how much blob content lands in a record's text field.
const maxTextBytes = 4 << 20 // 4 MiB

// Write streams the filtered entries of m as JSONL to w. Content comes from
// the blob store — never the live tree — preserving the reproducibility
// contract. Entries are emitted in path order (manifests are sorted, but the
// sort here makes the guarantee local, not an upstream assumption).
func Write(w io.Writer, st *store.Store, m *store.Manifest, f *Filter) (int, error) {
	if m == nil {
		return 0, fmt.Errorf("no manifest to export — run 'dig scan' first")
	}
	entries := append([]store.Entry{}, m.Entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	enc := json.NewEncoder(w)
	n := 0
	for _, e := range entries {
		if f != nil && !f.Match(e) {
			continue
		}
		rec := Record{
			Path:     e.Path,
			Labels:   e.Labels,
			Size:     e.Size,
			ModTime:  e.ModTime.UTC().Format(time.RFC3339),
			Src:      e.Blob,
			Manifest: m.ID,
		}
		rc, err := st.Blobs().Get(e.Blob)
		if err != nil {
			return n, fmt.Errorf("export %s: blob %s: %w", e.Path, e.Blob, err)
		}
		buf, err := io.ReadAll(io.LimitReader(rc, maxTextBytes))
		_ = rc.Close()
		if err != nil {
			return n, fmt.Errorf("export %s: %w", e.Path, err)
		}
		if utf8.Valid(buf) {
			rec.Text = string(buf)
		} else {
			rec.Binary = true // metadata + provenance only; no mangled bytes
		}
		if err := enc.Encode(rec); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
