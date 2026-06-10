package organize

import (
	"testing"
	"time"

	"github.com/bntvllnt/dig/internal/policy"
	"github.com/bntvllnt/dig/internal/store"
)

func rules(t *testing.T, src string) []policy.CompiledRule {
	t.Helper()
	p, err := policy.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := p.Compile()
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return out
}

func manifest(entries ...store.Entry) *store.Manifest {
	return &store.Manifest{ID: "M1", Entries: entries}
}

var mod = time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)

func TestPlanMovesAndLabels(t *testing.T) {
	rs := rules(t, `
[[rule]]
name  = "pdfs"
match = { ext = ["pdf"] }
into  = "docs/{year}"
label = ["doc"]`)

	plan, err := BuildPlan(t.TempDir(), manifest(
		store.Entry{Path: "inbox/a.pdf", Blob: "b3:1", ModTime: mod},
		store.Entry{Path: "notes/keep.txt", Blob: "b3:2", ModTime: mod},
	), rs)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ops) != 1 || plan.Ops[0].Kind != OpMove || plan.Ops[0].To != "docs/2024/a.pdf" {
		t.Fatalf("move op wrong: %+v", plan.Ops)
	}
	if len(plan.Ops[0].Labels) != 1 || plan.Ops[0].Labels[0] != "doc" {
		t.Fatalf("labels not carried on move: %+v", plan.Ops[0])
	}
	if len(plan.Unsorted) != 1 || plan.Unsorted[0] != "notes/keep.txt" {
		t.Fatalf("unmatched file not reported unsorted: %+v", plan.Unsorted)
	}
}

// First matching rule wins — policy order is precedence.
func TestPlanFirstRuleWins(t *testing.T) {
	rs := rules(t, `
[[rule]]
name  = "first"
match = { ext = ["pdf"] }
into  = "first"
[[rule]]
name  = "second"
match = { ext = ["pdf"] }
into  = "second"`)

	plan, err := BuildPlan(t.TempDir(), manifest(
		store.Entry{Path: "a.pdf", Blob: "b3:1", ModTime: mod},
	), rs)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ops) != 1 || plan.Ops[0].Rule != "first" {
		t.Fatalf("first rule should win: %+v", plan.Ops)
	}
}

// Concurrency-adjacent POV: two files claiming one target = conflict, never
// a silent overwrite.
func TestPlanCollisionEscalates(t *testing.T) {
	rs := rules(t, `
[[rule]]
name   = "flatten"
match  = { ext = ["pdf"] }
into   = "all"
rename = "doc.pdf"`)

	plan, err := BuildPlan(t.TempDir(), manifest(
		store.Entry{Path: "x/a.pdf", Blob: "b3:1", ModTime: mod},
		store.Entry{Path: "y/b.pdf", Blob: "b3:2", ModTime: mod},
	), rs)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ops) != 1 || len(plan.Conflicts) != 1 {
		t.Fatalf("want 1 op + 1 conflict, got ops=%+v conflicts=%+v", plan.Ops, plan.Conflicts)
	}
}

// Idempotency POV: planning over an already-organized manifest is a no-op.
func TestPlanIdempotent(t *testing.T) {
	rs := rules(t, `
[[rule]]
name  = "pdfs"
match = { ext = ["pdf"] }
into  = "docs/{year}"
label = ["doc"]`)

	plan, err := BuildPlan(t.TempDir(), manifest(
		store.Entry{Path: "docs/2024/a.pdf", Blob: "b3:1", ModTime: mod, Labels: []string{"doc"}},
		store.Entry{Path: "notes/keep.txt", Blob: "b3:2", ModTime: mod, Labels: []string{policy.UnsortedLabel}},
	), rs)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Empty() || len(plan.Unsorted) != 0 {
		t.Fatalf("organized KB should produce empty plan, got %+v", plan)
	}
}

// Regression (#5): label-only rules ACCUMULATE — they don't consume the
// match. Every matching label rule contributes; the first placement rule
// still decides the target and stops the scan.
func TestPlanLabelRulesAccumulate(t *testing.T) {
	rs := rules(t, `
[[rule]]
name  = "all-docs"
match = { ext = ["md"] }
label = ["doc"]
[[rule]]
name  = "architecture"
match = { path = "arch*" }
label = ["architecture"]
[[rule]]
name  = "shelve"
match = { ext = ["md"] }
into  = "library"
label = ["shelved"]
[[rule]]
name  = "never-reached"
match = { ext = ["md"] }
into  = "elsewhere"`)

	plan, err := BuildPlan(t.TempDir(), manifest(
		store.Entry{Path: "architecture.md", Blob: "b3:1", ModTime: mod},
	), rs)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ops) != 1 || plan.Ops[0].Kind != OpMove || plan.Ops[0].To != "library/architecture.md" {
		t.Fatalf("placement should come from the first placement rule: %+v", plan.Ops)
	}
	got := map[string]bool{}
	for _, l := range plan.Ops[0].Labels {
		got[l] = true
	}
	if !got["doc"] || !got["architecture"] || !got["shelved"] {
		t.Fatalf("labels from ALL matching rules should accumulate: %+v", plan.Ops[0].Labels)
	}
	if plan.Ops[0].Rule != "shelve" {
		t.Fatalf("move attribution should be the placement rule: %+v", plan.Ops[0])
	}
}

// Label-only op when the file is already in place but missing labels.
func TestPlanLabelOnly(t *testing.T) {
	rs := rules(t, `
[[rule]]
name  = "pdfs"
match = { ext = ["pdf"] }
into  = "docs/{year}"
label = ["doc"]`)

	plan, err := BuildPlan(t.TempDir(), manifest(
		store.Entry{Path: "docs/2024/a.pdf", Blob: "b3:1", ModTime: mod},
	), rs)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ops) != 1 || plan.Ops[0].Kind != OpLabel {
		t.Fatalf("want label-only op, got %+v", plan.Ops)
	}
}
