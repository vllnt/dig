package store

import (
	"time"
)

// Entry is one file in a manifest: a path mapped to its content blob plus
// metadata. Labels carry policy-assigned tags; Size/ModTime aid the index.
type Entry struct {
	Path    string    `json:"path"`
	Blob    string    `json:"blob"` // content hash, e.g. "b3:9f2a..."
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Labels  []string  `json:"labels,omitempty"`
}

// Manifest is an immutable snapshot of the tree at one point in history.
// ID is sequential (M1, M2, ...); Parent links to the previous head, forming
// the journal's chain. The root manifest has Parent == "".
type Manifest struct {
	ID        string    `json:"id"`
	Parent    string    `json:"parent"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"` // e.g. "scan", "org", "work/cleanup"
	Entries   []Entry   `json:"entries"`
}

// Lookup returns the entry for path, or false if absent.
func (m *Manifest) Lookup(path string) (Entry, bool) {
	for _, e := range m.Entries {
		if e.Path == path {
			return e, true
		}
	}
	return Entry{}, false
}
