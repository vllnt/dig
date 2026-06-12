// Package sink fires declarative event sinks (T0 extensibility) after a
// changeset commits. Sinks OBSERVE — a sink failure is reported to the caller
// but never alters or rolls back the committed changeset.
//
// Two kinds:
//   - webhook: POST the event as JSON to a URL. Enabled by default.
//   - exec:    run a shell command with DIG_* env + the event JSON on stdin.
//     Gated behind DIG_ALLOW_EXEC_SINKS=1 — exec sinks run code from the KB's
//     policy file, so they are off unless the operator opts in.
package sink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/vllnt/dig/internal/policy"
)

// Event describes a committed changeset handed to each sink.
type Event struct {
	Event     string `json:"event"`     // policy.EventCommitted
	KB        string `json:"kb"`        // KB root path
	Manifest  string `json:"manifest"`  // manifest id (e.g. M3)
	Kind      string `json:"kind"`      // observe | mutate
	CreatedBy string `json:"createdBy"` // the command that committed
	Entries   int    `json:"entries"`   // entry count in the manifest
}

// ExecEnvVar opts exec sinks in. Without it, exec sinks are skipped (with a
// reported error so the user knows one was configured but not run).
const ExecEnvVar = "DIG_ALLOW_EXEC_SINKS"

const sinkTimeout = 30 * time.Second

// Fire runs every sink that matches ev (on == "" or changeset.committed),
// returning one error per sink that failed or was skipped. The slice is empty
// when everything fired cleanly. Never returns early — one bad sink does not
// stop the others.
func Fire(sinks []policy.EventSink, ev Event) []error {
	var errs []error
	for i := range sinks {
		s := sinks[i]
		if s.On != "" && s.On != ev.Event {
			continue
		}
		if err := fireOne(s, ev); err != nil {
			errs = append(errs, fmt.Errorf("event_sink %d (%s): %w", i+1, s.Type, err))
		}
	}
	return errs
}

func fireOne(s policy.EventSink, ev Event) error {
	switch s.Type {
	case "webhook":
		return fireWebhook(s.URL, ev)
	case "exec":
		return fireExec(s.Command, ev)
	default:
		return fmt.Errorf("unknown type %q", s.Type)
	}
}

func fireWebhook(url string, ev Event) error {
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), sinkTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %s", resp.Status)
	}
	return nil
}

func fireExec(command string, ev Event) error {
	if os.Getenv(ExecEnvVar) != "1" {
		return fmt.Errorf("exec sink skipped — set %s=1 to allow running commands from policy.toml", ExecEnvVar)
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), sinkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stdin = bytes.NewReader(body)
	cmd.Env = append(os.Environ(),
		"DIG_EVENT="+ev.Event,
		"DIG_KB="+ev.KB,
		"DIG_MANIFEST="+ev.Manifest,
		"DIG_KIND="+ev.Kind,
		"DIG_CREATED_BY="+ev.CreatedBy,
		"DIG_ENTRIES="+strconv.Itoa(ev.Entries),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}
