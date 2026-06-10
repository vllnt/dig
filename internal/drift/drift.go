// Package drift implements the reconcile loop: policy is the desired state,
// the disk is the actual state, drift is the diff. Humans edit files with
// their own tools between dig runs; drift treats them as a concurrent writer
// whose changeset it reconstructs after the fact (architecture.md §3).
//
// Coexistence contract enforced here:
//   - a human rename/move is INTENT — accepted into history, never reversed;
//     if it violates policy the fix is ESCALATED, not auto-applied
//   - new files are filed per policy automatically
//   - deletions are absorbed as fact
package drift

import (
	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/scan"
	"github.com/vllnt/dig/internal/store"
)

// withLabel returns labels with l appended if absent.
func withLabel(labels []string, l string) []string {
	for _, v := range labels {
		if v == l {
			return labels
		}
	}
	return append(append([]string{}, labels...), l)
}

// Change kinds — what the human (or any external writer) did since the last
// manifest.
const (
	Added    = "added"
	Removed  = "removed"
	Modified = "modified"
	Renamed  = "renamed"
)

// Change is one reconstructed external edit.
type Change struct {
	Kind string `json:"kind"`
	Path string `json:"path"`           // current path (for removed: the old path)
	From string `json:"from,omitempty"` // previous path (renamed only)
	Blob string `json:"blob"`
}

// DiskEntries walks the live tree into store entries (blobs stored — external
// content enters the store immediately so every later step stays reversible).
func DiskEntries(k kb.KB, st *store.Store) ([]store.Entry, error) {
	return scan.Walk(k, st, false)
}

// Diff reconstructs the external changeset between the last manifest and the
// current disk entries. Renames are detected by content: a removed path and an
// added path sharing a blob are one rename. Labels survive the diff — they are
// dig metadata the human's tools cannot see, so the human cannot have removed
// them: same-path entries keep their labels through edits, renamed entries
// carry them to the new path.
func Diff(head *store.Manifest, current []store.Entry) ([]Change, []store.Entry) {
	oldByPath := map[string]store.Entry{}
	if head != nil {
		for _, e := range head.Entries {
			oldByPath[e.Path] = e
		}
	}

	var changes []Change
	absorbed := make([]store.Entry, len(current))
	copy(absorbed, current)

	curByPath := map[string]int{} // path → index into absorbed
	for i, e := range absorbed {
		curByPath[e.Path] = i
	}

	// removed candidates: in head, not on disk (by path).
	removedByBlob := map[string][]store.Entry{}
	var removedOrder []store.Entry
	for _, e := range oldByPath {
		if _, ok := curByPath[e.Path]; !ok {
			removedByBlob[e.Blob] = append(removedByBlob[e.Blob], e)
			removedOrder = append(removedOrder, e)
		}
	}

	for i, e := range absorbed {
		old, existed := oldByPath[e.Path]
		switch {
		case existed && old.Blob == e.Blob:
			absorbed[i].Labels = old.Labels // unchanged file keeps its labels
		case existed: // same path, new content
			absorbed[i].Labels = old.Labels // edits don't strip dig labels
			changes = append(changes, Change{Kind: Modified, Path: e.Path, Blob: e.Blob})
		default: // path is new — rename (blob seen among removed) or addition
			if cands := removedByBlob[e.Blob]; len(cands) > 0 {
				src := cands[0]
				removedByBlob[e.Blob] = cands[1:]
				// Labels travel with the rename, and the entry is PINNED:
				// the human chose this placement; policy hands off until a
				// human unpins (the durable form of "escalate, don't overwrite").
				absorbed[i].Labels = withLabel(src.Labels, policy.PinnedLabel)
				changes = append(changes, Change{Kind: Renamed, Path: e.Path, From: src.Path, Blob: e.Blob})
			} else {
				changes = append(changes, Change{Kind: Added, Path: e.Path, Blob: e.Blob})
			}
		}
	}

	// Whatever remains in removedByBlob was deleted, not renamed.
	for _, e := range removedOrder {
		stillRemoved := false
		for _, r := range removedByBlob[e.Blob] {
			if r.Path == e.Path {
				stillRemoved = true
				break
			}
		}
		if stillRemoved {
			changes = append(changes, Change{Kind: Removed, Path: e.Path, Blob: e.Blob})
		}
	}
	return changes, absorbed
}
