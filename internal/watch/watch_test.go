package watch

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/vllnt/dig/internal/drift"
	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/store"
)

const watchPolicy = `
[[rule]]
name     = "auto-pdfs"
match    = { ext = ["pdf"] }
into     = "docs"
autonomy = "auto"

[[rule]]
name  = "manual-images"
match = { ext = ["jpg"] }
into  = "gallery"
`

func setup(t *testing.T) (kb.KB, *store.Store, []policy.CompiledRule, policy.DedupPolicy) {
	t.Helper()
	root := t.TempDir()
	k, err := kb.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	dig, _ := k.EnsureDig()
	st, err := store.Open(dig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	pol, err := policy.Parse([]byte(watchPolicy))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := pol.Compile()
	if err != nil {
		t.Fatal(err)
	}
	return k, st, rules, pol.Dedup
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Autonomy POV: watch mode applies only autonomy="auto" rules; the rest are
// proposed and the files stay put.
func TestWatchModeAutonomyPartition(t *testing.T) {
	k, st, rules, dpol := setup(t)
	write(t, k.Root, "inbox/a.pdf", "pdf")
	write(t, k.Root, "inbox/b.jpg", "jpg")

	sum, err := drift.Reconcile(k, st, rules, dpol, false, drift.ModeWatch)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Applied) != 1 || sum.Applied[0].Rule != "auto-pdfs" {
		t.Fatalf("only the auto rule should act in watch mode: %+v", sum)
	}
	if len(sum.Proposed) != 1 || sum.Proposed[0].Rule != "manual-images" {
		t.Fatalf("the non-auto rule should propose: %+v", sum)
	}
	if _, err := os.Stat(filepath.Join(k.Root, "docs/a.pdf")); err != nil {
		t.Fatal("auto rule did not apply on disk")
	}
	if _, err := os.Stat(filepath.Join(k.Root, "inbox/b.jpg")); err != nil {
		t.Fatal("proposed file must stay put")
	}
}

// One-shot consent POV: the same policy applies BOTH rules when the user
// invokes reconcile explicitly (no autonomy="propose" set).
func TestOneShotAppliesNonAutoRules(t *testing.T) {
	k, st, rules, dpol := setup(t)
	write(t, k.Root, "inbox/b.jpg", "jpg")

	sum, err := drift.Reconcile(k, st, rules, dpol, false, drift.ModeOneShot)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Applied) != 1 || len(sum.Proposed) != 0 {
		t.Fatalf("one-shot should apply the manual rule: %+v", sum)
	}
}

// Watch mode never auto-removes duplicates — they are reported pending.
func TestWatchModeNeverCollapsesDups(t *testing.T) {
	k, st, rules, dpol := setup(t)
	write(t, k.Root, "x.txt", "same")
	write(t, k.Root, "y.txt", "same")
	older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(k.Root, "x.txt"), older, older); err != nil {
		t.Fatal(err)
	}

	sum, err := drift.Reconcile(k, st, rules, dpol, false, drift.ModeWatch)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Collapsed) != 0 || len(sum.DupPending) != 1 {
		t.Fatalf("watch must report dups, never collapse: %+v", sum)
	}
	if _, err := os.Stat(filepath.Join(k.Root, "y.txt")); err != nil {
		t.Fatal("watch removed a duplicate — forbidden")
	}
}

// Regression (#4): standing items surface once, not every tick.
func TestStandingItemsSurfaceOnce(t *testing.T) {
	k, st, rules, dpol := setup(t)
	write(t, k.Root, "inbox/photo.jpg", "jpg") // manual rule → standing proposal

	surfaced := map[string]bool{}
	s1, err := drift.Reconcile(k, st, rules, dpol, false, drift.ModeWatch)
	if err != nil {
		t.Fatal(err)
	}
	dedupeStanding(s1, surfaced)
	if len(s1.Proposed) != 1 {
		t.Fatalf("first pass should surface the proposal: %+v", s1)
	}
	s2, err := drift.Reconcile(k, st, rules, dpol, false, drift.ModeWatch)
	if err != nil {
		t.Fatal(err)
	}
	dedupeStanding(s2, surfaced)
	if len(s2.Proposed) != 0 {
		t.Fatalf("second pass must not re-surface the same proposal: %+v", s2.Proposed)
	}
}

// THE soak test: a live KB with files dropped while the loop runs. The auto
// rule converges them on disk; proposals surface; quiet ticks commit nothing;
// ctx cancel exits cleanly.
func TestWatchLoopSoak(t *testing.T) {
	k, st, rules, dpol := setup(t)
	write(t, k.Root, "seed.txt", "seed")

	var mu sync.Mutex
	var passes []*drift.Summary
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, k, st, rules, dpol, Options{
			Interval: 50 * time.Millisecond,
			OnPass: func(s *drift.Summary, _ []*store.View) {
				mu.Lock()
				passes = append(passes, s)
				mu.Unlock()
			},
		})
	}()

	// Drop files mid-watch, in waves.
	time.Sleep(120 * time.Millisecond)
	write(t, k.Root, "inbox/one.pdf", "pdf one")
	time.Sleep(150 * time.Millisecond)
	write(t, k.Root, "inbox/two.pdf", "pdf two")
	write(t, k.Root, "inbox/photo.jpg", "jpg")
	time.Sleep(250 * time.Millisecond)

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("watch loop errored: %v", err)
	}

	// Converged: auto-ruled files are in place.
	for _, want := range []string{"docs/one.pdf", "docs/two.pdf"} {
		if _, err := os.Stat(filepath.Join(k.Root, want)); err != nil {
			t.Fatalf("watch did not converge %s", want)
		}
	}
	// The manual-rule file was proposed, not moved.
	if _, err := os.Stat(filepath.Join(k.Root, "inbox/photo.jpg")); err != nil {
		t.Fatal("watch moved a file whose rule has no autonomy")
	}
	mu.Lock()
	defer mu.Unlock()
	proposedSeen := false
	for _, s := range passes {
		for _, op := range s.Proposed {
			if op.Rule == "manual-images" {
				proposedSeen = true
			}
		}
	}
	if !proposedSeen {
		t.Fatal("proposal for the manual rule never surfaced")
	}

	// No empty commits from quiet ticks: history = only meaningful commits.
	hist, _ := st.History()
	for _, m := range hist {
		if len(m.Entries) == 0 && m.Parent != "" {
			t.Fatalf("empty commit found: %+v", m)
		}
	}
	// A converged follow-up pass reports nothing.
	sum, err := drift.Reconcile(k, st, rules, dpol, false, drift.ModeWatch)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Absorbed)+len(sum.Applied) != 0 {
		t.Fatalf("post-soak KB should be converged: %+v", sum)
	}
}
