package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"go.etcd.io/bbolt"
)

// View states — the changeset state machine (architecture.md §4). Implemented
// as an explicit transition table, not ad-hoc booleans.
const (
	StateDraft     = "DRAFT"     // view opened on a base manifest
	StateProposed  = "PROPOSED"  // ops attached
	StateStaged    = "STAGED"    // validated, ready to commit
	StateMerged    = "MERGED"    // committed; head advanced
	StateConflict  = "CONFLICT"  // overlapped with commits since base
	StateEscalated = "ESCALATED" // human decision pending (conflict-escalation phase)
	StateAborted   = "ABORTED"   // discarded; head never touched
)

// transitions is the single source of truth for legal state changes.
// MERGED and ABORTED are terminal: manifests are immutable — "changing"
// merged work means opening a new view.
var transitions = map[string][]string{
	StateDraft:     {StateProposed, StateAborted},
	StateProposed:  {StateStaged, StateAborted},
	StateStaged:    {StateMerged, StateConflict, StateAborted},
	StateConflict:  {StateMerged, StateEscalated, StateAborted},
	StateEscalated: {StateMerged, StateAborted},
	StateMerged:    {},
	StateAborted:   {},
}

func canTransition(from, to string) bool {
	for _, t := range transitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// ViewOp is one proposed change in a view: a move (To set) and/or labels.
type ViewOp struct {
	From   string   `json:"from"`
	To     string   `json:"to,omitempty"`
	Labels []string `json:"labels,omitempty"`
}

// View is an isolated unit of work: a pointer to a base manifest plus
// proposed ops. Forking is O(1) — no file copying (architecture.md §1).
type View struct {
	Name      string    `json:"name"`
	Base      string    `json:"base"` // manifest ID the view forked from
	State     string    `json:"state"`
	Ops       []ViewOp  `json:"ops,omitempty"`
	Conflict  string    `json:"conflict,omitempty"` // why CONFLICT, when it is
	CreatedAt time.Time `json:"created_at"`
}

var bktViews = []byte("views")

func (s *Store) viewsBucket(tx *bbolt.Tx) (*bbolt.Bucket, error) {
	return tx.CreateBucketIfNotExists(bktViews)
}

func putView(b *bbolt.Bucket, v *View) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return b.Put([]byte(v.Name), raw)
}

func getView(b *bbolt.Bucket, name string) (*View, error) {
	raw := b.Get([]byte(name))
	if raw == nil {
		return nil, fmt.Errorf("view %q not found", name)
	}
	var v View
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// CreateView opens a new isolated view on the current head.
func (s *Store) CreateView(name string) (*View, error) {
	if name == "" {
		return nil, fmt.Errorf("view name required")
	}
	var out *View
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b, err := s.viewsBucket(tx)
		if err != nil {
			return err
		}
		if b.Get([]byte(name)) != nil {
			return fmt.Errorf("view %q already exists", name)
		}
		head := string(tx.Bucket(bktMeta).Get(keyHead))
		if head == "" {
			return fmt.Errorf("no manifest yet — run 'dig scan' first")
		}
		out = &View{Name: name, Base: head, State: StateDraft, CreatedAt: s.clock().UTC()}
		return putView(b, out)
	})
	return out, err
}

// GetView loads a view by name.
func (s *Store) GetView(name string) (*View, error) {
	var out *View
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bktViews)
		if b == nil {
			return fmt.Errorf("view %q not found", name)
		}
		v, err := getView(b, name)
		if err != nil {
			return err
		}
		out = v
		return nil
	})
	return out, err
}

// ListViews returns all views, newest first.
func (s *Store) ListViews() ([]*View, error) {
	var out []*View
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bktViews)
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, raw []byte) error {
			var v View
			if err := json.Unmarshal(raw, &v); err != nil {
				return err
			}
			out = append(out, &v)
			return nil
		})
	})
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, err
}

// transitionView loads, guards, mutates, stores — the one path every state
// change goes through.
func (s *Store) transitionView(name, to string, mutate func(*View) error) (*View, error) {
	var out *View
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b, err := s.viewsBucket(tx)
		if err != nil {
			return err
		}
		v, err := getView(b, name)
		if err != nil {
			return err
		}
		if !canTransition(v.State, to) {
			return fmt.Errorf("invalid transition %s → %s for view %q", v.State, to, name)
		}
		v.State = to
		if mutate != nil {
			if err := mutate(v); err != nil {
				return err
			}
		}
		out = v
		return putView(b, v)
	})
	return out, err
}

// ProposeView attaches ops to a DRAFT view (DRAFT → PROPOSED).
func (s *Store) ProposeView(name string, ops []ViewOp) (*View, error) {
	if len(ops) == 0 {
		return nil, fmt.Errorf("propose: at least one op required")
	}
	return s.transitionView(name, StateProposed, func(v *View) error {
		v.Ops = ops
		return nil
	})
}

// StageView validates a PROPOSED view (PROPOSED → STAGED): ops must reference
// paths that exist in the base manifest and must not escape the KB root.
func (s *Store) StageView(name string) (*View, error) {
	return s.transitionView(name, StateStaged, func(v *View) error {
		var base *Manifest
		err := s.db.View(func(tx *bbolt.Tx) error { return loadManifest(tx, v.Base, &base) })
		if err != nil || base == nil {
			return fmt.Errorf("base manifest %s missing", v.Base)
		}
		for _, op := range v.Ops {
			if _, ok := base.Lookup(op.From); !ok {
				return fmt.Errorf("op references %q which is not in base %s", op.From, v.Base)
			}
			if op.To != "" && escapesRoot(op.To) {
				return fmt.Errorf("op target %q escapes the KB root", op.To)
			}
		}
		return nil
	})
}

// AbortView discards a view (any non-terminal state → ABORTED). Head and disk
// are never touched by an abort.
func (s *Store) AbortView(name string) (*View, error) {
	return s.transitionView(name, StateAborted, nil)
}

// MergeView commits a STAGED view (STAGED → MERGED | CONFLICT) under the CAS
// guard: in one serialized transaction it diffs the paths touched since the
// view's base against the view's own paths. Disjoint → disk ops are applied,
// the merged manifest commits, head advances. Overlap → CONFLICT with the
// offending paths recorded; head and disk untouched.
//
// kbRoot is needed because merging applies the view's moves to the live tree
// (same disk-then-manifest pattern as organize.Apply, inside the tx so merges
// serialize and the disjoint check cannot race).
func (s *Store) MergeView(kbRoot, name string) (*View, *Manifest, error) {
	var outV *View
	var outM *Manifest
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b, err := s.viewsBucket(tx)
		if err != nil {
			return err
		}
		v, err := getView(b, name)
		if err != nil {
			return err
		}
		if !canTransition(v.State, StateMerged) {
			return fmt.Errorf("invalid transition %s → MERGED for view %q", v.State, name)
		}

		meta := tx.Bucket(bktMeta)
		headID := string(meta.Get(keyHead))
		var head, base *Manifest
		if err := loadManifest(tx, headID, &head); err != nil || head == nil {
			return fmt.Errorf("head manifest missing")
		}
		if err := loadManifest(tx, v.Base, &base); err != nil || base == nil {
			return fmt.Errorf("base manifest %s missing", v.Base)
		}

		// CAS + disjointness: paths changed between base and head vs paths
		// the view touches.
		changed := changedPaths(base, head)
		var overlap []string
		for _, op := range v.Ops {
			if changed[op.From] {
				overlap = append(overlap, op.From)
			}
			if op.To != "" && changed[op.To] {
				overlap = append(overlap, op.To)
			}
		}
		if len(overlap) > 0 {
			v.State = StateConflict
			v.Conflict = fmt.Sprintf("paths changed since base %s: %v", v.Base, overlap)
			outV = v
			return putView(b, v)
		}

		// Disjoint — apply ops to head's entries.
		entries := make([]Entry, len(head.Entries))
		copy(entries, head.Entries)
		byPath := map[string]int{}
		for i, e := range entries {
			byPath[e.Path] = i
		}
		for _, op := range v.Ops {
			i, ok := byPath[op.From]
			if !ok {
				v.State = StateConflict
				v.Conflict = fmt.Sprintf("path %q vanished from head", op.From)
				outV = v
				return putView(b, v)
			}
			if op.To != "" {
				if j, taken := byPath[op.To]; taken && j != i {
					v.State = StateConflict
					v.Conflict = fmt.Sprintf("target %q already occupied at head", op.To)
					outV = v
					return putView(b, v)
				}
				// Disk move (inside the tx: merges serialize on the db lock,
				// so the check above cannot race with another merge).
				src := filepath.Join(kbRoot, filepath.FromSlash(op.From))
				dst := filepath.Join(kbRoot, filepath.FromSlash(op.To))
				if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
					return err
				}
				if err := os.Rename(src, dst); err != nil {
					return fmt.Errorf("merge move %s: %w", op.From, err)
				}
				delete(byPath, op.From)
				entries[i].Path = op.To
				byPath[op.To] = i
			}
			for _, l := range op.Labels {
				entries[i].Labels = appendLabel(entries[i].Labels, l)
			}
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

		// Commit the merged manifest inside this same tx.
		seq := decodeSeq(meta.Get(keySeq)) + 1
		m := &Manifest{
			ID:        "M" + fmt.Sprint(seq),
			Parent:    headID,
			CreatedAt: s.clock().UTC(),
			CreatedBy: "merge/" + name,
			Kind:      KindMutate,
			Entries:   entries,
		}
		raw, err := json.Marshal(m)
		if err != nil {
			return err
		}
		if err := tx.Bucket(bktManifests).Put([]byte(m.ID), raw); err != nil {
			return err
		}
		if err := meta.Put(keySeq, encodeSeq(seq)); err != nil {
			return err
		}
		if err := meta.Put(keyHead, []byte(m.ID)); err != nil {
			return err
		}
		v.State = StateMerged
		outV, outM = v, m
		return putView(b, v)
	})
	if err != nil {
		return nil, nil, err
	}
	return outV, outM, nil
}

// changedPaths returns every path that differs between two manifests (added,
// removed, or content-changed on either side).
func changedPaths(a, b *Manifest) map[string]bool {
	out := map[string]bool{}
	am := map[string]string{}
	for _, e := range a.Entries {
		am[e.Path] = e.Blob
	}
	bm := map[string]string{}
	for _, e := range b.Entries {
		bm[e.Path] = e.Blob
	}
	for p, blob := range am {
		if other, ok := bm[p]; !ok || other != blob {
			out[p] = true
		}
	}
	for p := range bm {
		if _, ok := am[p]; !ok {
			out[p] = true
		}
	}
	return out
}

func appendLabel(labels []string, l string) []string {
	for _, v := range labels {
		if v == l {
			return labels
		}
	}
	return append(labels, l)
}

// escapesRoot reports whether a KB-relative target climbs out of the root.
func escapesRoot(rel string) bool {
	if filepath.IsAbs(rel) {
		return true
	}
	clean := filepath.ToSlash(filepath.Clean(rel))
	return clean == ".." || len(clean) >= 3 && clean[:3] == "../"
}
