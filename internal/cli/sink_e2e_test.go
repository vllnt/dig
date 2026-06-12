package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/vllnt/dig/internal/sink"
)

// TestChainWebhookSinkFiresOnCommit drives the real CLI: a [[event_sink]]
// webhook receives the changeset event on scan and on a mutating org. The
// receiver is a real HTTP server (the one external boundary).
func TestChainWebhookSinkFiresOnCommit(t *testing.T) {
	var mu sync.Mutex
	var events []sink.Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ev sink.Event
		_ = json.NewDecoder(r.Body).Decode(&ev)
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root := t.TempDir()
	write(t, root, "inbox/acme.pdf", "ACME invoice #1007")
	run(t, "init", root)
	write(t, root, ".dig/policy.toml", e2ePolicy+`
[[event_sink]]
on   = "changeset.committed"
type = "webhook"
url  = "`+srv.URL+`"
`)
	run(t, "--kb", root, "policy", "validate")

	run(t, "--kb", root, "scan") // M1 — observe
	run(t, "--kb", root, "org")  // M2 — mutate

	mu.Lock()
	defer mu.Unlock()
	if len(events) < 2 {
		t.Fatalf("expected a webhook per commit, got %d", len(events))
	}
	last := events[len(events)-1]
	if last.Event != "changeset.committed" || last.KB != root {
		t.Fatalf("bad event payload: %+v", last)
	}
	if last.Kind != "mutate" || last.Manifest == "" {
		t.Fatalf("org commit should be a mutate with a manifest id: %+v", last)
	}
}

// TestChainExecSinkGated proves exec sinks run ONLY with the opt-in env var,
// and that the command receives the event on stdin + DIG_* env.
func TestChainExecSinkGated(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(t.TempDir(), "fired")
	write(t, root, "a.txt", "alpha")
	run(t, "init", root)
	write(t, root, ".dig/policy.toml", `
[[rule]]
name  = "all"
match = { path = "*" }
label = ["seen"]

[[event_sink]]
type    = "exec"
command = "printf '%s' \"$DIG_MANIFEST $DIG_KIND\" > `+marker+`"
`)

	// Without the opt-in, the sink is skipped (warning on stderr), commit still ok.
	out := run(t, "--kb", root, "scan")
	if !strings.Contains(out, "DIG_ALLOW_EXEC_SINKS") {
		t.Fatalf("exec sink should warn it was skipped without opt-in: %s", out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("exec sink ran without the opt-in env var")
	}

	// With the opt-in, the command runs and sees the event.
	t.Setenv(sink.ExecEnvVar, "1")
	run(t, "--kb", root, "scan")
	b, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("exec sink did not run with opt-in: %v", err)
	}
	if !strings.Contains(string(b), "observe") || !strings.HasPrefix(string(b), "M") {
		t.Fatalf("exec sink env wrong: %q", b)
	}
}

// TestChainSinkFailureDoesNotBlockCommit proves a failing sink (unreachable
// webhook) warns but never rolls back the committed changeset.
func TestChainSinkFailureDoesNotBlockCommit(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.txt", "alpha")
	run(t, "init", root)
	write(t, root, ".dig/policy.toml", `
[[rule]]
name  = "all"
match = { path = "*" }
label = ["seen"]

[[event_sink]]
type = "webhook"
url  = "http://127.0.0.1:1/nope"
`)
	out := run(t, "--kb", root, "scan")
	if !strings.Contains(out, "manifest M1") {
		t.Fatalf("scan must commit despite a failing sink: %s", out)
	}
	if !strings.Contains(out, "warning") {
		t.Fatalf("a failing sink should warn: %s", out)
	}
	// The commit stands — history has M1.
	if !strings.Contains(run(t, "--kb", root, "log"), "M1") {
		t.Fatal("commit was rolled back by a sink failure")
	}
}
