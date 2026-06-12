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
