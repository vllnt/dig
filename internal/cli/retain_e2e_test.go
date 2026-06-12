package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/vllnt/dig/internal/kb"
)

// runStdin executes the CLI with data on stdin, like a piped capture.
func runStdin(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	root := NewRoot()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("dig %s: %v\n%s", strings.Join(args, " "), err, buf.String())
	}
	return buf.String()
}

// TestChainRetainRecall is the capture→recall loop over the real CLI: pipe a
// session in, then find and recall surface it; undo reverts the capture.
func TestChainRetainRecall(t *testing.T) {
	root := t.TempDir()
	run(t, "init", root)
	run(t, "--kb", root, "scan") // M1 — baseline (empty)

	session := "User asked about the ACME renewal. Decision: prioritize the contract; budget approved for March."
	out := runStdin(t, session, "--kb", root, "retain", "--as", "memory/session-1.md")
	if !strings.Contains(out, "Retained memory/session-1.md") || !strings.Contains(out, "manifest M2") {
		t.Fatalf("retain output: %s", out)
	}
	if diskState(t, root)["memory/session-1.md"] != session {
		t.Fatal("retain did not write the content into the KB")
	}

	// find surfaces the retained session.
	if !strings.Contains(run(t, "--kb", root, "find", "renewal contract"), "memory/session-1.md") {
		t.Fatal("find did not surface the retained session")
	}
	// recall packs it.
	if !strings.Contains(run(t, "--kb", root, "recall", "renewal contract budget"), "budget approved") {
		t.Fatal("recall did not surface the retained content")
	}

	// undo rewinds the capture (M2 → M1): the index no longer surfaces it. The
	// file itself stays on disk — dig never deletes files when undoing an
	// observation (the same guarantee that makes undoing a scan safe).
	run(t, "--kb", root, "undo")
	if _, ok := diskState(t, root)["memory/session-1.md"]; !ok {
		t.Fatal("undo of a retain must not delete the file (observe-undo is non-destructive)")
	}
	if strings.Contains(run(t, "--kb", root, "find", "renewal contract"), "memory/session-1.md") {
		t.Fatal("after undo the retained session should be out of the index")
	}
}

// TestChainRetainDefaultPath proves the dated default path (reproducible with
// --date) and that stdin capture lands content-addressed under memory/.
func TestChainRetainDefaultPath(t *testing.T) {
	root := t.TempDir()
	run(t, "init", root)

	out := runStdin(t, "a note worth keeping", "--kb", root, "retain", "--date", "2026-06-13")
	if !strings.Contains(out, "Retained memory/2026/06/13/") {
		t.Fatalf("default dated path wrong: %s", out)
	}
	// Same content + date → same path (content-addressed filename).
	root2 := t.TempDir()
	run(t, "init", root2)
	out2 := runStdin(t, "a note worth keeping", "--kb", root2, "retain", "--date", "2026-06-13")
	pathOf := func(s string) string {
		for _, f := range strings.Fields(s) {
			if strings.HasPrefix(f, "memory/") {
				return f
			}
		}
		return ""
	}
	if pathOf(out) != pathOf(out2) {
		t.Fatalf("same content+date should be reproducible: %q vs %q", pathOf(out), pathOf(out2))
	}
}

// TestChainRetainTranscript proves --transcript renders an agent session
// (JSONL) to readable markdown and retains that, so recall surfaces the
// conversation — not raw JSON.
func TestChainRetainTranscript(t *testing.T) {
	root := t.TempDir()
	run(t, "init", root)

	session := root + "/session.jsonl"
	if err := os.WriteFile(session, []byte(
		`{"type":"user","message":{"role":"user","content":"What did we decide about the ledger migration?"}}`+"\n"+
			`{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"internal"},{"type":"text","text":"We migrate billing to the new ledger in Q3; Dana owns it."}]}}`+"\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	out := run(t, "--kb", root, "retain", "--transcript", session, "--as", "memory/sessions/s.md")
	if !strings.Contains(out, "Retained memory/sessions/s.md") {
		t.Fatalf("transcript retain output: %s", out)
	}
	md := diskState(t, root)["memory/sessions/s.md"]
	if !strings.Contains(md, "## User") || !strings.Contains(md, "## Assistant") {
		t.Fatalf("transcript not rendered to turns:\n%s", md)
	}
	if strings.Contains(md, "internal") || strings.Contains(md, `"type"`) {
		t.Fatalf("raw JSON / thinking leaked into memory:\n%s", md)
	}
	// recall surfaces the captured decision.
	if !strings.Contains(run(t, "--kb", root, "recall", "ledger migration Dana Q3"), "new ledger in Q3") {
		t.Fatal("recall did not surface the captured session decision")
	}

	// --transcript and a file argument are mutually exclusive.
	runExpectErr(t, "--kb", root, "retain", "--transcript", session, "somefile.md")
}

// TestChainRetainGuards proves empty input and path escapes are rejected, and
// the .dig directory is off-limits.
func TestChainRetainGuards(t *testing.T) {
	root := t.TempDir()
	run(t, "init", root)

	runExpectErr(t, "--kb", root, "retain", "--as", "../escape.md")
	runExpectErr(t, "--kb", root, "retain", "--as", kb.DigDir+"/policy.toml")
	// Empty stdin → error.
	r := NewRoot()
	var buf bytes.Buffer
	r.SetOut(&buf)
	r.SetErr(&buf)
	r.SetIn(strings.NewReader(""))
	r.SetArgs([]string{"--kb", root, "retain", "--as", "memory/x.md"})
	if err := r.Execute(); err == nil {
		t.Fatal("empty retain should error")
	}
}
