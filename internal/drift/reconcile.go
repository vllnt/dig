package drift

import (
	"github.com/bntvllnt/dig/internal/dedup"
	"github.com/bntvllnt/dig/internal/kb"
	"github.com/bntvllnt/dig/internal/organize"
	"github.com/bntvllnt/dig/internal/policy"
	"github.com/bntvllnt/dig/internal/store"
)

// Report is everything that has drifted: what externals changed, what policy
// would fix, and which duplicates exist. Read-only — `dig drift` output.
type Report struct {
	Changes      []Change            `json:"changes,omitempty"`       // external edits since last manifest
	PolicyOps    []organize.Op       `json:"policy_ops,omitempty"`    // moves/labels policy wants
	Pinned       []organize.Op       `json:"pinned,omitempty"`        // standing escalations on human-placed files
	Conflicts    []organize.Conflict `json:"conflicts,omitempty"`     // policy ops it refuses to guess on
	Unsorted     []string            `json:"unsorted,omitempty"`      // files no rule matches
	DupSets      []dedup.Set         `json:"dup_sets,omitempty"`      // collapsible duplicates
	DupConflicts []dedup.Conflict    `json:"dup_conflicts,omitempty"` // ambiguous duplicates
}

// Clean reports whether nothing has drifted. Standing pins don't count — they
// are decided, just not by policy.
func (r *Report) Clean() bool {
	return len(r.Changes) == 0 && len(r.PolicyOps) == 0 && len(r.Conflicts) == 0 &&
		len(r.Unsorted) == 0 && len(r.DupSets) == 0 && len(r.DupConflicts) == 0
}

// BuildReport measures drift without committing anything.
func BuildReport(k kb.KB, st *store.Store, rules []policy.CompiledRule, dpol policy.DedupPolicy) (*Report, error) {
	head, err := st.Head()
	if err != nil {
		return nil, err
	}
	current, err := DiskEntries(k, st)
	if err != nil {
		return nil, err
	}
	changes, absorbed := Diff(head, current)

	// Evaluate policy against the WOULD-BE manifest (absorbed disk state),
	// so the report reflects reality, not stale history.
	hypothetical := &store.Manifest{ID: "(disk)", Entries: absorbed}
	plan, err := organize.BuildPlan(k.Root, hypothetical, rules)
	if err != nil {
		return nil, err
	}
	dplan, err := dedup.BuildPlan(hypothetical, dpol)
	if err != nil {
		return nil, err
	}
	return &Report{
		Changes:      changes,
		PolicyOps:    plan.Ops,
		Pinned:       plan.Pinned,
		Conflicts:    plan.Conflicts,
		Unsorted:     plan.Unsorted,
		DupSets:      dplan.Sets,
		DupConflicts: dplan.Conflicts,
	}, nil
}

// Mode selects the autonomy posture of a reconcile run.
const (
	// ModeOneShot: the user invoked reconcile explicitly — that is consent.
	// Every rule applies unless it is marked autonomy = "propose".
	ModeOneShot = "oneshot"
	// ModeWatch: unattended. Only rules marked autonomy = "auto" apply;
	// everything else proposes. Autonomy is earned rule-by-rule.
	ModeWatch = "watch"
)

// Summary is what one reconcile run did (or, dry-run, would do).
type Summary struct {
	Absorbed   []Change            `json:"absorbed,omitempty"`  // external edits folded into history
	Applied    []organize.Op       `json:"applied,omitempty"`   // policy ops executed
	Proposed   []organize.Op       `json:"proposed,omitempty"`  // ops held by rule autonomy — awaiting consent
	Escalated  []organize.Op       `json:"escalated,omitempty"` // ops on human-moved paths — held for a human
	Conflicts  []organize.Conflict `json:"conflicts,omitempty"`
	Collapsed  []dedup.Set         `json:"collapsed,omitempty"`   // dup sets removed (one-shot)
	DupPending []dedup.Set         `json:"dup_pending,omitempty"` // dup sets reported, not removed (watch)
	DupTies    []dedup.Conflict    `json:"dup_ties,omitempty"`
	Head       string              `json:"head"`
}

// Empty reports whether the run found nothing at all to do or surface.
func (s *Summary) Empty() bool {
	return len(s.Absorbed)+len(s.Applied)+len(s.Proposed)+len(s.Escalated)+
		len(s.Conflicts)+len(s.Collapsed)+len(s.DupPending)+len(s.DupTies) == 0
}

// Reconcile converges the KB on policy in one shot:
//
//  1. absorb — commit the current disk as an observe manifest (human edits
//     become history; labels preserved across renames/edits)
//  2. organize — apply policy ops, EXCEPT ops that would reverse a path the
//     human just moved/renamed: those are escalated, never auto-applied
//     (a human move is intent; policy disagreement goes to a human)
//  3. dedup — collapse unambiguous duplicate sets per policy; ties escalate
//
// Every mutation is a separate journaled commit — `dig undo` unwinds
// reconcile step by step. Dry-run computes everything and commits nothing.
// mode (ModeOneShot|ModeWatch) decides which rules may act unattended.
func Reconcile(k kb.KB, st *store.Store, rules []policy.CompiledRule, dpol policy.DedupPolicy, dryRun bool, mode string) (*Summary, error) {
	head, err := st.Head()
	if err != nil {
		return nil, err
	}
	current, err := DiskEntries(k, st)
	if err != nil {
		return nil, err
	}
	changes, absorbed := Diff(head, current)
	sum := &Summary{Absorbed: changes}

	// 1. absorb external edits (only if there are any).
	if len(changes) > 0 && !dryRun {
		head, err = st.Commit("reconcile/observe", store.KindObserve, absorbed)
		if err != nil {
			return nil, err
		}
	} else if len(changes) > 0 {
		head = &store.Manifest{ID: "(dry)", Entries: absorbed}
	}
	if head == nil {
		head = &store.Manifest{Entries: absorbed}
	}

	// 2. organize — the coexistence contract is enforced durably by pinning:
	// Diff pinned every human-renamed entry at absorb time, and BuildPlan
	// never emits auto-ops for pinned entries. plan.Pinned carries the
	// standing escalations (would-be moves a human must decide on).
	plan, err := organize.BuildPlan(k.Root, head, rules)
	if err != nil {
		return nil, err
	}
	// Partition by rule autonomy: an op only acts unattended when its rule's
	// autonomy permits it in this mode.
	autonomy := map[string]string{}
	for _, r := range rules {
		autonomy[r.Name] = r.Autonomy
	}
	actPlan := &organize.Plan{Conflicts: plan.Conflicts, Unsorted: plan.Unsorted}
	for _, op := range plan.Ops {
		a := autonomy[op.Rule]
		acts := a != "propose" // one-shot default
		if mode == ModeWatch {
			acts = a == "auto"
		}
		if acts {
			actPlan.Ops = append(actPlan.Ops, op)
		} else {
			sum.Proposed = append(sum.Proposed, op)
		}
	}
	sum.Applied = actPlan.Ops
	sum.Escalated = plan.Pinned
	sum.Conflicts = plan.Conflicts
	if !dryRun && (!actPlan.Empty() || len(actPlan.Unsorted) > 0) {
		head, err = organize.Apply(k.Root, st, head, actPlan)
		if err != nil {
			return nil, err
		}
	}

	// 3. dedup — collapse what is unambiguous. Removing files unattended is a
	// step beyond labeling/moving: watch mode only REPORTS duplicate sets
	// (DupPending); collapsing needs an explicit one-shot reconcile or
	// 'dig dedup'.
	dheadEntries := head
	if dryRun {
		// Project the org ops onto the entries so dedup sees the would-be tree.
		dheadEntries = projectPlan(head, actPlan)
	}
	dplan, err := dedup.BuildPlan(dheadEntries, dpol)
	if err != nil {
		return nil, err
	}
	sum.DupTies = dplan.Conflicts
	if mode == ModeWatch {
		sum.DupPending = dplan.Sets
	} else {
		sum.Collapsed = dplan.Sets
		if !dryRun && !dplan.Empty() {
			head, err = dedup.Apply(k.Root, st, head, dplan)
			if err != nil {
				return nil, err
			}
		}
	}

	if head != nil {
		sum.Head = head.ID
	}
	return sum, nil
}

// projectPlan returns a manifest view with the plan's moves applied in memory
// (dry-run only — lets later stages reason about the would-be tree).
func projectPlan(m *store.Manifest, plan *organize.Plan) *store.Manifest {
	moved := map[string]string{}
	for _, op := range plan.Ops {
		if op.Kind == organize.OpMove {
			moved[op.From] = op.To
		}
	}
	out := &store.Manifest{ID: m.ID, Entries: make([]store.Entry, len(m.Entries))}
	copy(out.Entries, m.Entries)
	for i := range out.Entries {
		if to, ok := moved[out.Entries[i].Path]; ok {
			out.Entries[i].Path = to
		}
	}
	return out
}
