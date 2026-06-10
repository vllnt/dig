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

// Summary is what one reconcile run did (or, dry-run, would do).
type Summary struct {
	Absorbed  []Change            `json:"absorbed,omitempty"`  // external edits folded into history
	Applied   []organize.Op       `json:"applied,omitempty"`   // policy ops executed
	Escalated []organize.Op       `json:"escalated,omitempty"` // ops on human-moved paths — held for a human
	Conflicts []organize.Conflict `json:"conflicts,omitempty"`
	Collapsed []dedup.Set         `json:"collapsed,omitempty"`
	DupTies   []dedup.Conflict    `json:"dup_ties,omitempty"`
	Head      string              `json:"head"`
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
func Reconcile(k kb.KB, st *store.Store, rules []policy.CompiledRule, dpol policy.DedupPolicy, dryRun bool) (*Summary, error) {
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
	sum.Applied = plan.Ops
	sum.Escalated = plan.Pinned
	sum.Conflicts = plan.Conflicts
	if !dryRun && (!plan.Empty() || len(plan.Unsorted) > 0) {
		head, err = organize.Apply(k.Root, st, head, plan)
		if err != nil {
			return nil, err
		}
	}

	// 3. dedup — collapse what is unambiguous.
	dheadEntries := head
	if dryRun {
		// Project the org ops onto the entries so dedup sees the would-be tree.
		dheadEntries = projectPlan(head, plan)
	}
	dplan, err := dedup.BuildPlan(dheadEntries, dpol)
	if err != nil {
		return nil, err
	}
	sum.Collapsed = dplan.Sets
	sum.DupTies = dplan.Conflicts
	if !dryRun && !dplan.Empty() {
		head, err = dedup.Apply(k.Root, st, head, dplan)
		if err != nil {
			return nil, err
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
