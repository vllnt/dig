package watch

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/vector"
	"github.com/vllnt/dig/internal/vector/vectortest"
)

// TestWatchDrainsEmbedBacklog proves watch is the background semantic
// indexer: files dropped into the KB get absorbed by reconcile and their
// embed backlog (larger than one tick's budget) drains across ticks while
// the loop keeps running.
func TestWatchDrainsEmbedBacklog(t *testing.T) {
	srv := vectortest.New()
	defer srv.Close()

	k, st, rules, dpol := setup(t)
	const n = 70 // > embedBudgetPerTick — forces multi-tick draining
	for i := 0; i < n; i++ {
		write(t, k.Root, fmt.Sprintf("inbox/n%02d.txt", i), fmt.Sprintf("note %02d body", i))
	}

	rp := policy.RetrievalPolicy{Mode: "hybrid", BaseURL: srv.BaseURL(), Model: "fake-model"}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, k, st, rules, dpol, Options{
			Interval:  10 * time.Millisecond,
			Retrieval: rp,
		})
	}()

	deadline := time.After(15 * time.Second)
	for srv.Embedded.Load() < n {
		select {
		case <-deadline:
			t.Fatalf("backlog never drained: %d embedded", srv.Embedded.Load())
		case <-time.After(50 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("watch exited with error: %v", err)
	}

	// Loop stopped — the queue is empty and the index is complete.
	vx, err := vector.Open(k.Dig())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = vx.Close() }()
	if pending, _ := vx.PendingCount(); pending != 0 {
		t.Fatalf("%d blobs still pending after drain", pending)
	}
}

// TestWatchEmbedEndpointDownIsSoft proves a dead endpoint never kills the
// watch loop — reconcile keeps running and the stall is reported once, not
// every tick.
func TestWatchEmbedEndpointDownIsSoft(t *testing.T) {
	k, st, rules, dpol := setup(t)
	write(t, k.Root, "inbox/a.txt", "alpha")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	var buf bytes.Buffer
	err := Run(ctx, k, st, rules, dpol, Options{
		Interval:  20 * time.Millisecond,
		Retrieval: policy.RetrievalPolicy{Mode: "vector", BaseURL: "http://127.0.0.1:1/v1", Model: "m"},
		Warn:      &buf,
	})
	if err != nil {
		t.Fatalf("dead endpoint must not kill watch: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "semantic index stalled") {
		t.Fatalf("stall should be reported: %q", out)
	}
	if c := strings.Count(out, "semantic index stalled"); c != 1 {
		t.Fatalf("stall reported %d times across ticks, want 1", c)
	}
}
