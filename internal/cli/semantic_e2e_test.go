package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vllnt/dig/internal/vector/vectortest"
)

// semanticPolicy wires a KB to a (fake) embedding endpoint. The endpoint is
// the one third-party boundary a test double is allowed; everything else in
// the chain — CLI, store, indexes, fusion — is the real thing.
func semanticPolicy(url string) string {
	return fmt.Sprintf(`
[retrieval]
mode = "hybrid"
base_url = %q
model = "fake-model"
doc_prefix = "doc: "
query_prefix = "query: "
`, url)
}

// TestChainSemanticFind drives the full semantic journey over the CLI:
// init → scan (embeds) → find in all three modes → file moves → re-scan
// (cache hit) → find tracks the new path → endpoint dies → deterministic
// modes keep working and vector mode fails loudly.
func TestChainSemanticFind(t *testing.T) {
	srv := vectortest.New()
	defer srv.Close()

	root := t.TempDir()
	write(t, root, "notes/finances.md", "the quarterly budget covers salaries and cloud spend")
	write(t, root, "notes/travel.md", "packing list for the mountain hiking trip in autumn")
	run(t, "init", root)
	write(t, root, ".dig/policy.toml", semanticPolicy(srv.BaseURL()))

	run(t, "--kb", root, "scan")
	if srv.Embedded.Load() == 0 {
		t.Fatal("scan with [retrieval] enabled must embed content")
	}

	// Vector mode ranks by shared vocabulary (real cosine over the fake space).
	out := run(t, "--kb", root, "find", "--mode", "vector", "--limit", "1", "budget", "salaries")
	if !strings.Contains(out, "notes/finances.md") {
		t.Fatalf("vector find missed: %s", out)
	}

	// Hybrid agrees when both rankers agree; FTS stays available untouched.
	out = run(t, "--kb", root, "find", "--mode", "hybrid", "--limit", "1", "hiking", "trip")
	if !strings.Contains(out, "notes/travel.md") {
		t.Fatalf("hybrid find missed: %s", out)
	}
	out = run(t, "--kb", root, "find", "--mode", "fts", "budget")
	if !strings.Contains(out, "notes/finances.md") {
		t.Fatalf("fts find broken with retrieval on: %s", out)
	}

	// Default mode comes from policy (hybrid here) — no flag needed.
	out = run(t, "--kb", root, "find", "--limit", "1", "budget", "salaries")
	if !strings.Contains(out, "notes/finances.md") {
		t.Fatalf("policy-default mode find missed: %s", out)
	}

	// JSON surface carries scores for other harnesses.
	out = run(t, "--kb", root, "find", "--json", "--mode", "hybrid", "budget")
	if !strings.Contains(out, `"Path"`) || !strings.Contains(out, `"Score"`) {
		t.Fatalf("hybrid --json missing fields: %s", out)
	}

	// A human moves the file; re-scan re-embeds nothing (same blob — query
	// embeds from the finds above don't matter, only the delta across scan).
	mustRename(t, root, "notes/finances.md", "archive/budget-2026.md")
	embedsBeforeRescan := srv.Embedded.Load()
	run(t, "--kb", root, "scan")
	if srv.Embedded.Load() != embedsBeforeRescan {
		t.Fatalf("re-scan of moved file re-embedded: %d → %d", embedsBeforeRescan, srv.Embedded.Load())
	}
	out = run(t, "--kb", root, "find", "--mode", "vector", "--limit", "1", "budget", "salaries")
	if !strings.Contains(out, "archive/budget-2026.md") {
		t.Fatalf("vector find lost moved file: %s", out)
	}

	// Endpoint goes down: deterministic paths unaffected, semantic fails loudly,
	// and the user can recover by retrying once the endpoint is back.
	srv.Close()
	out = run(t, "--kb", root, "find", "--mode", "fts", "budget")
	if !strings.Contains(out, "archive/budget-2026.md") {
		t.Fatalf("fts must work with endpoint down: %s", out)
	}
	if msg := runErrMessage(t, "--kb", root, "find", "--mode", "vector", "budget"); !strings.Contains(msg, "embedding endpoint") {
		t.Fatalf("vector mode with endpoint down should explain the failure: %s", msg)
	}
}

// runErrMessage executes the real cobra tree expecting failure and returns
// the error text (root silences errors, so Execute's return is the message).
func runErrMessage(t *testing.T, args ...string) string {
	t.Helper()
	root := NewRoot()
	root.SetOut(&strings.Builder{})
	root.SetErr(&strings.Builder{})
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		t.Fatalf("dig %s: expected error, got success", strings.Join(args, " "))
	}
	return err.Error()
}

// TestChainSemanticBacklogDefersToEmbed drives the background-indexing story:
// a scan bigger than the inline budget returns immediately with a pending
// notice, `dig embed` drains the backlog (resumably), and semantic find then
// covers the whole KB.
func TestChainSemanticBacklogDefersToEmbed(t *testing.T) {
	srv := vectortest.New()
	defer srv.Close()

	root := t.TempDir()
	n := inlineEmbedBudget + 20
	for i := 0; i < n; i++ {
		write(t, root, fmt.Sprintf("notes/doc-%03d.md", i), fmt.Sprintf("note number %03d about subject %03d", i, i))
	}
	run(t, "init", root)
	write(t, root, ".dig/policy.toml", semanticPolicy(srv.BaseURL()))

	// Scan embeds only the inline budget and says what remains.
	out := run(t, "--kb", root, "scan")
	if !strings.Contains(out, "manifest M1") {
		t.Fatalf("scan failed: %s", out)
	}
	if int(srv.Embedded.Load()) >= n {
		t.Fatalf("scan embedded the whole backlog inline: %d", srv.Embedded.Load())
	}
	if !strings.Contains(out, "pending — run 'dig embed'") {
		t.Fatalf("scan should surface the pending backlog: %s", out)
	}

	// dig embed drains the rest; progress + completion are reported.
	out = run(t, "--kb", root, "embed")
	if !strings.Contains(out, "semantic index complete") {
		t.Fatalf("embed output: %s", out)
	}

	// The last file (beyond the inline budget) is semantically findable.
	// Top-5 not top-1: the fake's hashed dimensions can collide across this
	// many docs, producing exact ties — membership is the real assertion.
	out = run(t, "--kb", root, "find", "--mode", "vector", "--limit", "5",
		fmt.Sprintf("note number %03d", n-1))
	if !strings.Contains(out, fmt.Sprintf("doc-%03d.md", n-1)) {
		t.Fatalf("backlogged file not findable after embed: %s", out)
	}

	// Re-running embed is a no-op (idempotent).
	embeds := srv.Embedded.Load()
	out = run(t, "--kb", root, "embed")
	if !strings.Contains(out, "0 file(s) embedded this run") {
		t.Fatalf("second embed should be a no-op: %s", out)
	}
	if srv.Embedded.Load() != embeds {
		t.Fatal("idempotent embed re-embedded blobs")
	}
}

// TestChainSemanticScanDegradesGracefully proves an unreachable endpoint
// never blocks the deterministic spine: scan succeeds with a warning, FTS
// keeps answering.
func TestChainSemanticScanDegradesGracefully(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.md", "alpha document")
	run(t, "init", root)
	write(t, root, ".dig/policy.toml", semanticPolicy("http://127.0.0.1:1/v1")) // nothing listens

	out := run(t, "--kb", root, "scan") // must not fail
	if !strings.Contains(out, "manifest M1") {
		t.Fatalf("scan failed under dead endpoint: %s", out)
	}
	out = run(t, "--kb", root, "find", "--mode", "fts", "alpha")
	if !strings.Contains(out, "a.md") {
		t.Fatalf("fts must survive a dead endpoint: %s", out)
	}
}

// TestChainSemanticUndo proves semantic state follows history: undo rewinds
// the docs view (no re-embedding thanks to the blob cache) and find answers
// from the restored manifest.
func TestChainSemanticUndo(t *testing.T) {
	srv := vectortest.New()
	defer srv.Close()

	root := t.TempDir()
	write(t, root, "one.md", "first document about gardening tomatoes")
	run(t, "init", root)
	write(t, root, ".dig/policy.toml", semanticPolicy(srv.BaseURL()))
	run(t, "--kb", root, "scan") // M1

	write(t, root, "two.md", "second document about deep sea diving")
	run(t, "--kb", root, "scan") // M2

	out := run(t, "--kb", root, "find", "--mode", "vector", "--limit", "1", "diving", "sea")
	if !strings.Contains(out, "two.md") {
		t.Fatalf("vector find at M2: %s", out)
	}

	embedsBeforeUndo := srv.Embedded.Load()
	run(t, "--kb", root, "undo") // back to M1
	if srv.Embedded.Load() != embedsBeforeUndo {
		t.Fatal("undo must not re-embed — cache is blob-keyed")
	}
	out = run(t, "--kb", root, "find", "--mode", "vector", "diving", "sea")
	if strings.Contains(out, "two.md") {
		t.Fatalf("undone file still semantically findable: %s", out)
	}
}
