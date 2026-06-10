// Package organize turns policy rules into changesets and applies them to
// disk, reversibly. The flow is propose → preview → apply → (undo):
// BuildPlan never touches anything; Apply commits one journaled mutation.
package organize

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/store"
)

// OpKind enumerates plan operations.
const (
	OpMove  = "move"  // path changes (folder and/or name)
	OpLabel = "label" // labels added, path unchanged
)

// Op is one planned change to one file.
type Op struct {
	Kind   string   `json:"kind"`
	Rule   string   `json:"rule"`
	From   string   `json:"from"`
	To     string   `json:"to,omitempty"`
	Labels []string `json:"labels,omitempty"`
}

// Conflict is a change the plan refuses to make, with the reason. Conflicts
// are reported, never silently resolved (escalate-don't-guess).
type Conflict struct {
	Path   string `json:"path"`
	Rule   string `json:"rule"`
	Reason string `json:"reason"`
}

// Plan is the full proposed changeset for one org run.
type Plan struct {
	Ops       []Op       `json:"ops"`
	Pinned    []Op       `json:"pinned,omitempty"` // would-be moves on human-pinned entries — never auto-applied
	Conflicts []Conflict `json:"conflicts,omitempty"`
	Unsorted  []string   `json:"unsorted,omitempty"` // files no rule matched
}

// Empty reports whether the plan changes nothing (labels included).
func (p *Plan) Empty() bool { return len(p.Ops) == 0 }

// maxContentProbe mirrors policy's content probe cap.
const maxContentProbe = 1 << 20

// BuildPlan evaluates compiled rules over the head manifest's entries.
// First matching rule wins (file order in the policy = precedence).
// It reads file content lazily and only for rules that need it.
func BuildPlan(kbRoot string, head *store.Manifest, rules []policy.CompiledRule) (*Plan, error) {
	plan := &Plan{}
	if head == nil {
		return plan, nil
	}

	contentOf := func(rel string) ([]byte, error) {
		f, err := os.Open(filepath.Join(kbRoot, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		return io.ReadAll(io.LimitReader(f, maxContentProbe))
	}

	// targets tracks claimed destination paths so two files never plan onto
	// the same target (first claim wins, second becomes a conflict).
	targets := map[string]string{} // target path → claiming source
	for _, e := range head.Entries {
		targets[e.Path] = e.Path // current paths are occupied by definition
	}

	for _, e := range head.Entries {
		// Rule semantics: label-only rules ACCUMULATE (every matching one
		// contributes labels and matching continues); the first matching
		// PLACEMENT rule (into/rename) decides the target and stops the scan.
		matched := false
		firstRule := ""
		var wantLabels []string
		var placement *policy.CompiledRule
		for i := range rules {
			r := &rules[i]
			ok, err := r.Matches(e.Path, contentOf)
			if err != nil {
				return nil, fmt.Errorf("rule %q on %s: %w", r.Name, e.Path, err)
			}
			if !ok {
				continue
			}
			if !matched {
				matched, firstRule = true, r.Name
			}
			wantLabels = append(wantLabels, r.Label...)
			if r.Into != "" || r.Rename != "" {
				placement = r
				break // first placement rule wins
			}
		}
		if !matched {
			if !hasLabel(e.Labels, policy.UnsortedLabel) {
				plan.Unsorted = append(plan.Unsorted, e.Path)
			}
			continue
		}

		target := e.Path
		ruleName := ""
		if placement != nil {
			target = placement.Target(e.Path, e.ModTime)
			ruleName = placement.Name
		}
		missing := newLabels(e.Labels, wantLabels)

		// A pinned entry was deliberately placed by a human: policy never
		// auto-moves it. The would-be op is surfaced for a human instead.
		// Labels still accumulate — they don't fight the human's placement.
		if hasLabel(e.Labels, policy.PinnedLabel) {
			if target != e.Path {
				plan.Pinned = append(plan.Pinned, Op{
					Kind: OpMove, Rule: ruleName, From: e.Path, To: target,
				})
			}
			if len(missing) > 0 {
				plan.Ops = append(plan.Ops, Op{Kind: OpLabel, Rule: labelRule(ruleName, firstRule), From: e.Path, Labels: missing})
			}
			continue
		}
		if escapes(target) {
			plan.Conflicts = append(plan.Conflicts, Conflict{
				Path: e.Path, Rule: ruleName,
				Reason: fmt.Sprintf("target %q escapes the KB root", target),
			})
			continue
		}
		if target != e.Path {
			if claimer, taken := targets[target]; taken && claimer != e.Path {
				plan.Conflicts = append(plan.Conflicts, Conflict{
					Path: e.Path, Rule: ruleName,
					Reason: fmt.Sprintf("target %q already taken by %s", target, claimer),
				})
				continue
			}
			delete(targets, e.Path)
			targets[target] = e.Path
			plan.Ops = append(plan.Ops, Op{
				Kind: OpMove, Rule: ruleName, From: e.Path, To: target, Labels: missing,
			})
		} else if len(missing) > 0 {
			plan.Ops = append(plan.Ops, Op{
				Kind: OpLabel, Rule: labelRule(ruleName, firstRule), From: e.Path, Labels: missing,
			})
		}
	}

	sort.Slice(plan.Ops, func(i, j int) bool { return plan.Ops[i].From < plan.Ops[j].From })
	sort.Strings(plan.Unsorted)
	return plan, nil
}

// labelRule names a label op's rule attribution, falling back when no
// placement rule fired (labels came from label-only rules).
func labelRule(placement, fallback string) string {
	if placement != "" {
		return placement
	}
	return fallback
}

// newLabels returns the labels in want that are not already in have.
func newLabels(have, want []string) []string {
	var out []string
	for _, w := range want {
		if !hasLabel(have, w) {
			out = append(out, w)
		}
	}
	return out
}

func hasLabel(labels []string, l string) bool {
	for _, v := range labels {
		if v == l {
			return true
		}
	}
	return false
}

// escapes reports whether a cleaned KB-relative path climbs out of the root.
func escapes(rel string) bool {
	clean := filepath.ToSlash(filepath.Clean(rel))
	return clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/")
}
