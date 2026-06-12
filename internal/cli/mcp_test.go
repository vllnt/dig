package cli

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/vllnt/dig/internal/mcp"
)

// mcpConn drives a Server over in-memory pipes, like a real MCP client.
type mcpConn struct {
	in  *io.PipeWriter
	out *bufio.Reader
	wg  *sync.WaitGroup
}

func startMCP(t *testing.T) *mcpConn {
	t.Helper()
	srv := mcp.NewServer("dig", "test")
	registerDigTools(srv)

	serverR, clientW := io.Pipe() // client → server
	clientR, serverW := io.Pipe() // server → client
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Serve(serverR, serverW)
		_ = serverW.Close()
	}()
	t.Cleanup(func() {
		_ = clientW.Close()
		wg.Wait()
	})
	return &mcpConn{in: clientW, out: bufio.NewReader(clientR), wg: wg}
}

// call sends a JSON-RPC request and returns the decoded result for its id.
func (c *mcpConn) call(t *testing.T, id int, method string, params any) map[string]any {
	t.Helper()
	req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	b, _ := json.Marshal(req)
	if _, err := c.in.Write(append(b, '\n')); err != nil {
		t.Fatalf("write: %v", err)
	}
	line, err := c.out.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var resp struct {
		ID     int            `json:"id"`
		Result map[string]any `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatalf("decode %q: %v", line, err)
	}
	if resp.ID != id {
		t.Fatalf("response id %d != request %d", resp.ID, id)
	}
	if resp.Error != nil {
		t.Fatalf("%s rpc error: %s", method, resp.Error.Message)
	}
	return resp.Result
}

// notify sends a notification (no id, no response expected).
func (c *mcpConn) notify(t *testing.T, method string) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method})
	if _, err := c.in.Write(append(b, '\n')); err != nil {
		t.Fatalf("notify: %v", err)
	}
}

// callToolText extracts the text content of a tools/call result.
func callToolText(t *testing.T, result map[string]any) (string, bool) {
	t.Helper()
	isError, _ := result["isError"].(bool)
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("tool result missing content: %v", result)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	return text, isError
}

// TestMCPHandshakeAndTools drives the real protocol: initialize, list tools,
// then call read tools against a real KB and a mutate tool in both dry-run and
// apply modes — proving the stdio server speaks MCP and reuses the CLI.
func TestMCPHandshakeAndTools(t *testing.T) {
	root := t.TempDir()
	write(t, root, "inbox/acme.pdf", "ACME invoice #1007")
	write(t, root, "inbox/todo.md", "- [ ] things")
	run(t, "init", root)
	write(t, root, ".dig/policy.toml", e2ePolicy)
	run(t, "--kb", root, "scan")

	c := startMCP(t)

	// initialize → server advertises name + tools capability, echoes version.
	init := c.call(t, 1, "initialize", map[string]any{"protocolVersion": "2025-06-18"})
	if init["protocolVersion"] != "2025-06-18" {
		t.Fatalf("protocolVersion not echoed: %v", init["protocolVersion"])
	}
	info, _ := init["serverInfo"].(map[string]any)
	if info["name"] != "dig" {
		t.Fatalf("serverInfo.name = %v", info["name"])
	}
	c.notify(t, "notifications/initialized")

	// tools/list → the dig surface is advertised with schemas.
	list := c.call(t, 2, "tools/list", nil)
	tools, _ := list["tools"].([]any)
	names := map[string]bool{}
	for _, raw := range tools {
		tl, _ := raw.(map[string]any)
		names[tl["name"].(string)] = true
		if tl["inputSchema"] == nil {
			t.Fatalf("tool %v missing inputSchema", tl["name"])
		}
	}
	for _, want := range []string{"dig_find", "dig_retain", "dig_recall", "dig_drift", "dig_log", "dig_export", "dig_org", "dig_reconcile", "dig_undo"} {
		if !names[want] {
			t.Fatalf("tool %s not advertised", want)
		}
	}

	// dig_find returns the indexed doc as JSON.
	res := c.call(t, 3, "tools/call", map[string]any{
		"name": "dig_find", "arguments": map[string]any{"kb": root, "query": "invoice"},
	})
	text, isErr := callToolText(t, res)
	if isErr || !strings.Contains(text, "acme.pdf") {
		t.Fatalf("dig_find result: err=%v %s", isErr, text)
	}

	// dig_org defaults to a dry-run preview — nothing committed.
	res = c.call(t, 4, "tools/call", map[string]any{
		"name": "dig_org", "arguments": map[string]any{"kb": root},
	})
	text, isErr = callToolText(t, res)
	if isErr || !strings.Contains(text, "dry-run") {
		t.Fatalf("dig_org should preview by default: err=%v %s", isErr, text)
	}
	if _, moved := diskState(t, root)["finance/invoices/acme.pdf"]; moved {
		t.Fatal("dig_org dry-run mutated the disk")
	}

	// dig_org apply=true commits; the file moves and is reversible.
	res = c.call(t, 5, "tools/call", map[string]any{
		"name": "dig_org", "arguments": map[string]any{"kb": root, "apply": true},
	})
	if _, isErr = callToolText(t, res); isErr {
		t.Fatal("dig_org apply failed")
	}
	if _, moved := diskState(t, root)["finance/invoices/acme.pdf"]; !moved {
		t.Fatal("dig_org apply did not move the file")
	}

	// dig_undo reverses it.
	c.call(t, 6, "tools/call", map[string]any{
		"name": "dig_undo", "arguments": map[string]any{"kb": root},
	})
	if _, moved := diskState(t, root)["finance/invoices/acme.pdf"]; moved {
		t.Fatal("dig_undo did not revert the org")
	}

	// An unknown tool is a tool error, not a transport error.
	res = c.call(t, 7, "tools/call", map[string]any{"name": "dig_nope", "arguments": map[string]any{}})
	if _, isErr = callToolText(t, res); !isErr {
		t.Fatal("unknown tool should be an isError result")
	}
}

// TestMCPMemoryRoundTrip drives the agent-memory loop entirely over MCP: a
// harness captures content with dig_retain, then loads it back with dig_recall,
// then dig_undo rewinds the capture out of recall — proving any MCP client can
// use dig as its memory layer with no CLI shell-out.
func TestMCPMemoryRoundTrip(t *testing.T) {
	root := t.TempDir()
	run(t, "init", root)
	run(t, "--kb", root, "scan") // baseline manifest so undo has a parent

	c := startMCP(t)

	// retain: a harness pipes a session fact into memory.
	fact := "Decision: migrate billing to the new ledger in Q3; owner is Dana."
	res := c.call(t, 1, "tools/call", map[string]any{
		"name": "dig_retain",
		"arguments": map[string]any{
			"kb": root, "content": fact, "as": "memory/decision.md",
		},
	})
	text, isErr := callToolText(t, res)
	if isErr || !strings.Contains(text, "Retained memory/decision.md") {
		t.Fatalf("dig_retain: err=%v %s", isErr, text)
	}

	// recall: the same harness loads it back as a budgeted pack.
	res = c.call(t, 2, "tools/call", map[string]any{
		"name": "dig_recall",
		"arguments": map[string]any{
			"kb": root, "query": "billing ledger migration owner", "budget": 500,
		},
	})
	text, isErr = callToolText(t, res)
	if isErr {
		t.Fatalf("dig_recall errored: %s", text)
	}
	var pack struct {
		BudgetTokens int `json:"budgetTokens"`
		Items        []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(text), &pack); err != nil {
		t.Fatalf("dig_recall not JSON: %v\n%s", err, text)
	}
	if pack.BudgetTokens != 500 || len(pack.Items) == 0 || !strings.Contains(pack.Items[0].Content, "ledger") {
		t.Fatalf("recall did not surface the retained fact: %+v", pack)
	}

	// undo: rewind the capture; recall no longer surfaces it.
	c.call(t, 3, "tools/call", map[string]any{"name": "dig_undo", "arguments": map[string]any{"kb": root}})
	res = c.call(t, 4, "tools/call", map[string]any{
		"name": "dig_recall", "arguments": map[string]any{"kb": root, "query": "billing ledger migration owner"},
	})
	text, _ = callToolText(t, res)
	pack.Items = nil
	if err := json.Unmarshal([]byte(text), &pack); err != nil {
		t.Fatalf("dig_recall not JSON: %v\n%s", err, text)
	}
	if len(pack.Items) != 0 {
		t.Fatalf("after undo, recall should be empty: %+v", pack.Items)
	}

	// retain requires content.
	res = c.call(t, 5, "tools/call", map[string]any{
		"name": "dig_retain", "arguments": map[string]any{"kb": root},
	})
	if _, isErr = callToolText(t, res); !isErr {
		t.Fatal("dig_retain without content should be an isError result")
	}
}

// TestMCPUnknownMethod proves a bad method yields a JSON-RPC error, not a crash.
func TestMCPUnknownMethod(t *testing.T) {
	c := startMCP(t)
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "frobnicate"})
	if _, err := c.in.Write(append(b, '\n')); err != nil {
		t.Fatal(err)
	}
	line, err := c.out.ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(line), "method not found") {
		t.Fatalf("expected method-not-found error, got %s", line)
	}
}
