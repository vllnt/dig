// Package dedup detects and collapses duplicate files. Duplicates are free to
// find in a content-addressed store — two paths mapping to one blob ARE the
// duplicate set. Collapsing keeps one canonical copy per policy and removes
// the rest from disk; content always survives in the blob store, so undo
// restores every copy byte-identically. Ambiguity escalates, never guesses.
package dedup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/store"
)

// Set is one group of duplicates: the copy to keep and the copies to remove.
type Set struct {
	Blob   string   `json:"blob"`
	Keep   string   `json:"keep"`
	Remove []string `json:"remove"`
}

// Conflict is a duplicate group dedup refuses to collapse, with the reason.
type Conflict struct {
	Blob   string   `json:"blob"`
	Paths  []string `json:"paths"`
	Reason string   `json:"reason"`
}

// Plan is the proposed collapse for one dedup run.
type Plan struct {
	Sets      []Set      `json:"sets"`
	Conflicts []Conflict `json:"conflicts,omitempty"`
}

// Empty reports whether there is nothing to collapse.
func (p *Plan) Empty() bool { return len(p.Sets) == 0 }

// Removals counts the files the plan would remove.
func (p *Plan) Removals() int {
	n := 0
	for _, s := range p.Sets {
		n += len(s.Remove)
	}
	return n
}

// BuildPlan groups the head manifest's entries by blob and picks a canonical
// copy per group using the policy strategy (default keep-oldest). A tie on the
// deciding timestamp is ambiguous → conflict (escalate, don't guess).
func BuildPlan(head *store.Manifest, pol policy.DedupPolicy) (*Plan, error) {
	plan := &Plan{}
	if head == nil {
		return plan, nil
	}
	strategy := pol.Strategy
	if strategy == "" {
		strategy = "keep-oldest"
	}

	groups := map[string][]store.Entry{}
	for _, e := range head.Entries {
		groups[e.Blob] = append(groups[e.Blob], e)
	}

	blobs := make([]string, 0, len(groups))
	for b, g := range groups {
		if len(g) > 1 {
			blobs = append(blobs, b)
		}
	}
	sort.Strings(blobs) // deterministic plan order

	for _, b := range blobs {
		g := groups[b]
		sort.Slice(g, func(i, j int) bool { return g[i].Path < g[j].Path })

		keep := g[0]
		tie := false
		for _, e := range g[1:] {
			switch strategy {
			case "keep-oldest":
				if e.ModTime.Before(keep.ModTime) {
					keep, tie = e, false
				} else if e.ModTime.Equal(keep.ModTime) {
					tie = true
				}
			case "keep-newest":
				if e.ModTime.After(keep.ModTime) {
					keep, tie = e, false
				} else if e.ModTime.Equal(keep.ModTime) {
					tie = true
				}
			default:
				return nil, fmt.Errorf("unknown dedup strategy %q", strategy)
			}
		}
		if tie {
			paths := make([]string, len(g))
			for i, e := range g {
				paths[i] = e.Path
			}
			plan.Conflicts = append(plan.Conflicts, Conflict{
				Blob: b, Paths: paths,
				Reason: fmt.Sprintf("modification times tie under %s — choose manually", strategy),
			})
			continue
		}

		set := Set{Blob: b, Keep: keep.Path}
		for _, e := range g {
			if e.Path != keep.Path {
				set.Remove = append(set.Remove, e.Path)
			}
		}
		plan.Sets = append(plan.Sets, set)
	}
	return plan, nil
}

// Apply removes the non-canonical copies from disk and commits the collapsed
// tree as ONE KindMutate manifest — reversible with undo (content stays in
// the blob store). The canonical copy is verified present before anything is
// removed: dedup must never delete the last copy.
func Apply(kbRoot string, st *store.Store, head *store.Manifest, plan *Plan) (*store.Manifest, error) {
	if head == nil {
		return nil, fmt.Errorf("nothing to dedup: no manifest — run 'dig scan' first")
	}
	removed := map[string]bool{}
	for _, set := range plan.Sets {
		keep := filepath.Join(kbRoot, filepath.FromSlash(set.Keep))
		if _, err := os.Stat(keep); err != nil {
			return nil, fmt.Errorf("canonical copy %s missing on disk — refusing to remove its duplicates", set.Keep)
		}
		for _, rel := range set.Remove {
			if err := os.Remove(filepath.Join(kbRoot, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("remove duplicate %s: %w", rel, err)
			}
			removed[rel] = true
		}
	}

	entries := make([]store.Entry, 0, len(head.Entries))
	for _, e := range head.Entries {
		if !removed[e.Path] {
			entries = append(entries, e)
		}
	}
	return st.Commit("dedup", store.KindMutate, entries)
}
