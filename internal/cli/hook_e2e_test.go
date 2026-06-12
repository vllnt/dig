package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildDig compiles the dig binary into dir and returns its path, so the hook
// script can invoke a real `dig` from PATH (no mock).
func buildDig(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "dig")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/dig")
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build dig: %v\n%s", err, out)
	}
	return bin
}

// repoRoot resolves the module root from this test's directory (internal/cli).
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// writeTranscript drops a tiny Claude-Code-shaped JSONL transcript.
func writeTranscript(t *testing.T, path string) {
	t.Helper()
	doc := `{"type":"user","message":{"role":"user","content":"Where did we land on the ledger migration?"}}` + "\n" +
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Migrate billing to the new ledger in Q3; Dana owns it."}]}}` + "\n"
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runHook executes the retain-session.sh hook with the given stdin payload and
// extra env, against a PATH that contains the built dig. Returns nothing — the
// hook is fire-and-forget — so callers assert on the KB state.
func runHook(t *testing.T, binDir, payload string, env ...string) {
	t.Helper()
	script := filepath.Join(repoRoot(t), "hooks", "retain-session.sh")
	cmd := exec.Command("sh", script)
	cmd.Stdin = strings.NewReader(payload)
	cmd.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook exited non-zero (must always exit 0): %v\n%s", err, out)
	}
}

// TestRetentionHook drives the real SessionEnd hook script end-to-end: with the
// opt-in set and a dig KB at cwd, a finished session is rendered and retained so
// recall surfaces it; without the opt-in, or outside a KB, it is a safe no-op.
func TestRetentionHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell hook")
	}
	binDir := t.TempDir()
	buildDig(t, binDir)

	root := t.TempDir()
	run(t, "init", root)
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	writeTranscript(t, transcript)

	payload := `{"hook_event_name":"SessionEnd","session_id":"sess-42","transcript_path":"` + transcript + `","cwd":"` + root + `"}`

	// No opt-in → no capture.
	runHook(t, binDir, payload)
	if hasSessionMemory(t, root) {
		t.Fatal("hook captured a session without DIG_RETAIN_SESSIONS opt-in")
	}

	// Opt-in + KB at cwd → the session is retained and recallable.
	runHook(t, binDir, payload, "DIG_RETAIN_SESSIONS=1")
	if !hasSessionMemory(t, root) {
		t.Fatal("hook did not retain the session into memory/sessions/")
	}
	if !strings.Contains(run(t, "--kb", root, "recall", "ledger migration Dana"), "new ledger in Q3") {
		t.Fatal("recall did not surface the hook-captured session")
	}

	// Opt-in but cwd outside any KB → no-op (no crash, nothing written).
	outside := t.TempDir()
	payloadOutside := `{"session_id":"sess-9","transcript_path":"` + transcript + `","cwd":"` + outside + `"}`
	runHook(t, binDir, payloadOutside, "DIG_RETAIN_SESSIONS=1")
	if _, err := os.Stat(filepath.Join(outside, "memory")); !os.IsNotExist(err) {
		t.Fatal("hook wrote outside a dig KB")
	}
}

// hasSessionMemory reports whether any memory/sessions/**/<id>.md was captured.
func hasSessionMemory(t *testing.T, root string) bool {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(root, "memory", "sessions", "*", "*", "*", "*.md"))
	return len(matches) > 0
}
