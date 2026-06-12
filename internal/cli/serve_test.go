package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// serveTestKB builds a scanned KB with the org policy and returns its root.
func serveTestKB(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "inbox/acme.pdf", "ACME invoice #1007")
	write(t, root, "inbox/todo.md", "- [ ] things")
	run(t, "init", root)
	write(t, root, ".dig/policy.toml", e2ePolicy)
	run(t, "--kb", root, "scan")
	return root
}

func httpGet(t *testing.T, base, path string, q url.Values) (int, string) {
	t.Helper()
	resp, err := http.Get(base + path + "?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func httpPost(t *testing.T, base, path string, q url.Values) (int, string) {
	t.Helper()
	resp, err := http.Post(base+path+"?"+q.Encode(), "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// TestServeDaemonDrivesKB exercises the daemon over real HTTP: health, read
// endpoints return dig's JSON, and the org mutation previews unless apply=true,
// then undo reverts — the same surface an SDK would call.
func TestServeDaemonDrivesKB(t *testing.T) {
	root := serveTestKB(t)
	ts := httptest.NewServer(digRoutes())
	defer ts.Close()

	// health
	if code, body := httpGet(t, ts.URL, "/health", nil); code != 200 || !strings.Contains(body, "ok") {
		t.Fatalf("health: %d %s", code, body)
	}

	// find returns valid JSON containing the indexed doc
	code, body := httpGet(t, ts.URL, "/find", url.Values{"kb": {root}, "query": {"invoice"}})
	if code != 200 {
		t.Fatalf("find: %d %s", code, body)
	}
	if !json.Valid([]byte(body)) || !strings.Contains(body, "acme.pdf") {
		t.Fatalf("find body not the expected JSON: %s", body)
	}

	// log + drift respond with JSON
	if code, body := httpGet(t, ts.URL, "/log", url.Values{"kb": {root}}); code != 200 || !json.Valid([]byte(body)) {
		t.Fatalf("log: %d %s", code, body)
	}
	if code, _ := httpGet(t, ts.URL, "/drift", url.Values{"kb": {root}}); code != 200 {
		t.Fatalf("drift: %d", code)
	}

	// org without apply is a dry-run preview — disk unchanged
	code, body = httpPost(t, ts.URL, "/org", url.Values{"kb": {root}})
	if code != 200 || !strings.Contains(body, "dry-run") {
		t.Fatalf("org preview: %d %s", code, body)
	}
	if _, moved := diskState(t, root)["finance/invoices/acme.pdf"]; moved {
		t.Fatal("org preview mutated the disk over HTTP")
	}

	// org?apply=true commits; the file moves
	if code, body = httpPost(t, ts.URL, "/org", url.Values{"kb": {root}, "apply": {"true"}}); code != 200 {
		t.Fatalf("org apply: %d %s", code, body)
	}
	if _, moved := diskState(t, root)["finance/invoices/acme.pdf"]; !moved {
		t.Fatal("org apply did not move the file")
	}

	// undo reverts
	if code, _ := httpPost(t, ts.URL, "/undo", url.Values{"kb": {root}}); code != 200 {
		t.Fatalf("undo: %d", code)
	}
	if _, moved := diskState(t, root)["finance/invoices/acme.pdf"]; moved {
		t.Fatal("undo over HTTP did not revert")
	}
}

// httpPostBody POSTs a raw body (the capture content) and returns status+body.
func httpPostBody(t *testing.T, base, path string, q url.Values, body string) (int, string) {
	t.Helper()
	resp, err := http.Post(base+path+"?"+q.Encode(), "text/plain", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// TestServeMemoryEndpoints drives the agent-memory loop over the real daemon: a
// client POSTs a session to /retain, then GETs /recall and the captured fact
// comes back in a budgeted pack — the surface a framework adapter consumes.
func TestServeMemoryEndpoints(t *testing.T) {
	root := t.TempDir()
	run(t, "init", root)
	run(t, "--kb", root, "scan")
	ts := httptest.NewServer(digRoutes())
	defer ts.Close()

	// retain a session over HTTP.
	fact := "Decision: adopt the new ledger in Q3; Dana owns the migration."
	code, body := httpPostBody(t, ts.URL, "/retain", url.Values{"kb": {root}, "as": {"memory/s.md"}}, fact)
	if code != 200 || !strings.Contains(body, "Retained memory/s.md") {
		t.Fatalf("retain over HTTP: %d %s", code, body)
	}
	if diskState(t, root)["memory/s.md"] != fact {
		t.Fatal("retain over HTTP did not persist the content")
	}

	// recall it back as a budgeted JSON pack.
	code, body = httpGet(t, ts.URL, "/recall", url.Values{"kb": {root}, "query": {"ledger migration Dana"}, "budget": {"400"}})
	if code != 200 || !json.Valid([]byte(body)) {
		t.Fatalf("recall over HTTP: %d %s", code, body)
	}
	var pack struct {
		BudgetTokens int                        `json:"budgetTokens"`
		Items        []struct{ Content string } `json:"items"`
	}
	if err := json.Unmarshal([]byte(body), &pack); err != nil {
		t.Fatalf("recall body not a pack: %v\n%s", err, body)
	}
	if pack.BudgetTokens != 400 || len(pack.Items) == 0 || !strings.Contains(pack.Items[0].Content, "new ledger in Q3") {
		t.Fatalf("recall did not surface the retained fact: %s", body)
	}

	// /retain rejects GET (it mutates).
	resp, err := http.Get(ts.URL + "/retain")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("retain should reject GET, got %d", resp.StatusCode)
	}
}

// TestServeMethodGuards proves read endpoints reject POST and mutations reject
// GET, so a client can't accidentally mutate via a GET.
func TestServeMethodGuards(t *testing.T) {
	ts := httptest.NewServer(digRoutes())
	defer ts.Close()

	if code, _ := httpPost(t, ts.URL, "/find", url.Values{"query": {"x"}}); code != http.StatusMethodNotAllowed {
		t.Fatalf("find should reject POST, got %d", code)
	}
	resp, err := http.Get(ts.URL + "/org")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("org should reject GET, got %d", resp.StatusCode)
	}
}

// TestServeRefusesNonLoopback proves dig serve will not bind a public address.
func TestServeRefusesNonLoopback(t *testing.T) {
	out := runExpectErr(t, "serve", "--addr", "8.8.8.8:9999")
	_ = out
	out = runExpectErr(t, "serve", "--addr", "0.0.0.0:9999")
	_ = out
}
