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
// Rule (optional) records the policy rule that produced the op — the handle
// precedence resolution uses.
type ViewOp struct {
	From   string   `json:"from"`
	To     string   `json:"to,omitempty"`
	Labels []string `json:"labels,omitempty"`
	Rule   string   `json:"rule,omitempty"`
}

// RuleRank resolves a rule name to its precedence (lower = stronger, i.e.
// earlier in the policy file). Unknown rules return a negative value.
type RuleRank func(rule string) int

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

// MergeView commits a STAGED (or ESCALATED, after resolution) view under the
// CAS guard, applying the escalation ladder per op (architecture.md §4):
//
//  1. path untouched since base ............... apply (clean)
//  2. touched but compatible .................. apply (label union; label ops
//     follow a file the head moved — blob retarget; move already done — noop)
//  3. touched, both sides rule-attributed ..... earlier policy rule wins
//     (incoming stronger → apply; weaker → drop, head stands)
//  4. still conflicting ....................... HELD: the op joins the
//     remainder, the view goes ESCALATED on the new head
//
// Escalation is surgical: clean/compatible/won ops MERGE in this call; only
// the conflicted remainder is held. A conflict on finance/ never blocks
// media/. Everything happens in one serialized tx (merges cannot race).
func (s *Store) MergeView(kbRoot, name string, rank RuleRank) (*View, *Manifest, error) {
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

		entries := make([]Entry, len(head.Entries))
		copy(entries, head.Entries)
		byPath := map[string]int{}
		blobAt := map[string]int{} // blob → entry index (first), for retargeting
		for i, e := range entries {
			byPath[e.Path] = i
			if _, ok := blobAt[e.Blob]; !ok {
				blobAt[e.Blob] = i
			}
		}
		changed := changedPaths(base, head)

		type diskMove struct{ src, dst string }
		var moves []diskMove
		var remainder []ViewOp
		applied := 0

		for _, op := range v.Ops {
			target := func(i int) { // apply op against entries[i]
				if op.To != "" && entries[i].Path != op.To {
					moves = append(moves, diskMove{
						src: filepath.Join(kbRoot, filepath.FromSlash(entries[i].Path)),
						dst: filepath.Join(kbRoot, filepath.FromSlash(op.To)),
					})
					delete(byPath, entries[i].Path)
					entries[i].Path = op.To
					byPath[op.To] = i
				}
				for _, l := range op.Labels {
					entries[i].Labels = appendLabel(entries[i].Labels, l)
				}
				if op.Rule != "" {
					entries[i].Rule = op.Rule
				}
				applied++
			}

			i, exists := byPath[op.From]
			// Only From-side changes make an op "touched": a target path that
			// changed since base is handled by occupancy (taken → hold) — a
			// VACATED target is an opportunity, not a conflict.
			touched := changed[op.From]

			switch {
			case exists && !touched:
				// Rung 1 — clean. Target collision with an untouched path is
				// still a hold (someone occupies it).
				if op.To != "" {
					if j, taken := byPath[op.To]; taken && j != i {
						remainder = append(remainder, op)
						continue
					}
				}
				target(i)

			case exists && op.To == "":
				// Rung 2 — label-only on a touched path: labels union safely.
				target(i)

			case exists && op.To != "" && entries[i].Path == op.To:
				// Rung 2 — head already moved it where we wanted: noop union.
				target(i)

			case !exists:
				// Path gone at head — follow the blob (head renamed the file):
				// label-only ops retarget; a move that agrees with where head
				// already put the file is a noop union; disagreeing moves hold.
				if srcBase, ok := base.Lookup(op.From); ok {
					if j, found := blobAt[srcBase.Blob]; found &&
						(op.To == "" || entries[j].Path == op.To) {
						target(j) // Rung 2 — follow the file
						continue
					}
				}
				remainder = append(remainder, op)

			default:
				// Rung 3 — true conflict on a touched path: precedence if both
				// sides carry rules, else hold for a human.
				if rank != nil && op.Rule != "" && entries[i].Rule != "" {
					or, hr := rank(op.Rule), rank(entries[i].Rule)
					if or >= 0 && hr >= 0 {
						if or < hr {
							target(i) // incoming rule outranks — apply
						}
						// weaker or equal: head stands; op resolved by drop
						continue
					}
				}
				remainder = append(remainder, op)
			}
		}

		// Disk + manifest only if something applied.
		if applied > 0 {
			for _, mv := range moves {
				if err := os.MkdirAll(filepath.Dir(mv.dst), 0o755); err != nil {
					return err
				}
				if err := os.Rename(mv.src, mv.dst); err != nil {
					return fmt.Errorf("merge move %s: %w", mv.src, err)
				}
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
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
			outM = m
			headID = m.ID
		}

		if len(remainder) == 0 {
			v.State = StateMerged
			v.Conflict = ""
			v.Ops = nil
		} else {
			// Surgical escalation: held ops re-based on the new head so a
			// later resolution merges against fresh state.
			v.State = StateEscalated
			v.Base = headID
			v.Ops = remainder
			v.Conflict = fmt.Sprintf("%d op(s) held for a human (others merged)", len(remainder))
		}
		outV = v
		return putView(b, v)
	})
	if err != nil {
		return nil, nil, err
	}
	return outV, outM, nil
}

// ResolveView settles an ESCALATED view: accept "mine" force-applies the held
// ops (the human decided the view wins), accept "theirs" drops them (head
// stands). Mine is implemented as a merge that treats every held op as clean.
func (s *Store) ResolveView(kbRoot, name, accept string) (*View, *Manifest, error) {
	switch accept {
	case "theirs":
		// Head stands; the held ops are discarded. ESCALATED → ABORTED.
		v, err := s.transitionView(name, StateAborted, nil)
		if err != nil {
			return nil, nil, err
		}
		return v, nil, nil
	case "mine":
		// Re-stage the remainder as if clean: temporarily rebase the view onto
		// the current head (transitionView guards ESCALATED → MERGED legality
		// inside MergeView via the table).
		v, err := s.GetView(name)
		if err != nil {
			return nil, nil, err
		}
		if v.State != StateEscalated {
			return nil, nil, fmt.Errorf("resolve: view %q is %s, want ESCALATED", name, v.State)
		}
		// Rebase: ops were already re-based onto the post-merge head by
		// MergeView, so a second merge now treats them against fresh state.
		return s.MergeView(kbRoot, name, nil)
	default:
		return nil, nil, fmt.Errorf("resolve: accept must be 'mine' or 'theirs'")
	}
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
