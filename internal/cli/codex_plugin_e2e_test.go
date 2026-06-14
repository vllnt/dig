package cli

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// readJSON loads and decodes a repo-relative JSON file into v, failing the test
// with the path on any parse error — the plugin ships these files verbatim.
func readJSON(t *testing.T, rel string, v any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
}

// TestCodexPluginManifest checks the .codex-plugin install graph is internally
// consistent: the manifest's required fields are present, every path it
// references resolves to a real bundled asset, the MCP config declares the
// `dig mcp` server, and the repo-scoped marketplace points back at this plugin.
func TestCodexPluginManifest(t *testing.T) {
	var manifest struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
		Skills      string `json:"skills"`
		MCPServers  string `json:"mcpServers"`
	}
	readJSON(t, ".codex-plugin/plugin.json", &manifest)

	if manifest.Name == "" || manifest.Version == "" || manifest.Description == "" {
		t.Fatalf("manifest missing required field(s): %+v", manifest)
	}
	if manifest.Name != "dig" {
		t.Fatalf("manifest name = %q, want dig", manifest.Name)
	}

	// Every referenced path must resolve inside the repo (the plugin root).
	if manifest.Skills == "" {
		t.Fatal("manifest does not bundle the dig skill")
	}
	skillDoc := filepath.Join(repoRoot(t), filepath.Clean(manifest.Skills), "dig", "SKILL.md")
	if _, err := os.Stat(skillDoc); err != nil {
		t.Fatalf("manifest skills path does not resolve to %s: %v", skillDoc, err)
	}
	if manifest.MCPServers == "" {
		t.Fatal("manifest does not wire an MCP server")
	}
	// Deliberately NOT named .mcp.json: a root .mcp.json would auto-load the dig
	// server into Claude Code sessions in dig's own repo, which the Claude plugin
	// avoids on purpose (ROADMAP harness-plugins.2).
	if filepath.Base(manifest.MCPServers) == ".mcp.json" {
		t.Fatal("MCP config must not be a root .mcp.json (auto-loads in dig's own repo sessions)")
	}
	mcpPath := filepath.Join(repoRoot(t), filepath.Clean(manifest.MCPServers))
	if _, err := os.Stat(mcpPath); err != nil {
		t.Fatalf("manifest mcpServers path does not resolve to %s: %v", mcpPath, err)
	}

	// The MCP config declares exactly the `dig mcp` stdio server.
	var servers map[string]struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	readJSON(t, filepath.Clean(manifest.MCPServers), &servers)
	dig, ok := servers["dig"]
	if !ok {
		t.Fatalf("mcp config has no dig server: %+v", servers)
	}
	if dig.Command != "dig" || len(dig.Args) != 1 || dig.Args[0] != "mcp" {
		t.Fatalf("dig server is not `dig mcp`: command=%q args=%v", dig.Command, dig.Args)
	}

	// The repo-scoped marketplace lists this plugin and points back at the repo.
	var mkt struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name   string `json:"name"`
			Source struct {
				Source string `json:"source"`
				URL    string `json:"url"`
			} `json:"source"`
			Policy struct {
				Installation   string `json:"installation"`
				Authentication string `json:"authentication"`
			} `json:"policy"`
			Category string `json:"category"`
		} `json:"plugins"`
	}
	readJSON(t, ".agents/plugins/marketplace.json", &mkt)
	if len(mkt.Plugins) != 1 {
		t.Fatalf("marketplace should list exactly the dig plugin, got %d", len(mkt.Plugins))
	}
	p := mkt.Plugins[0]
	if p.Name != manifest.Name {
		t.Fatalf("marketplace plugin name %q != manifest name %q", p.Name, manifest.Name)
	}
	if p.Source.Source != "url" || !strings.Contains(p.Source.URL, "vllnt/dig") {
		t.Fatalf("marketplace source not the dig repo: %+v", p.Source)
	}
	// Policy values must be variants the Codex CLI accepts — verified live against
	// codex-cli 0.130.0, which rejects anything else at `marketplace add` time.
	validInstall := map[string]bool{"AVAILABLE": true, "INSTALLED_BY_DEFAULT": true, "NOT_AVAILABLE": true}
	validAuth := map[string]bool{"ON_INSTALL": true, "ON_USE": true}
	if !validInstall[p.Policy.Installation] {
		t.Fatalf("policy.installation %q not a Codex variant", p.Policy.Installation)
	}
	if !validAuth[p.Policy.Authentication] {
		t.Fatalf("policy.authentication %q not a Codex variant (want ON_INSTALL|ON_USE)", p.Policy.Authentication)
	}
}

// TestCodexPluginBootsMCPServer is the real end-to-end: it reads the exact
// command the plugin ships in mcp.json, launches it as a subprocess (the built
// dig binary, no mock), and drives the MCP protocol over its stdio — proving an
// installed Codex plugin actually gives the agent dig's tool surface.
func TestCodexPluginBootsMCPServer(t *testing.T) {
	var servers map[string]struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	readJSON(t, "mcp.json", &servers)
	dig := servers["dig"]

	// Resolve the plugin's "dig" command to a freshly built binary so the test
	// is hermetic and runs the real server the config points at.
	binDir := t.TempDir()
	bin := buildDig(t, binDir)

	cmd := exec.Command(bin, dig.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("launch `%s %s`: %v", filepath.Base(bin), strings.Join(dig.Args, " "), err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	})
	out := bufio.NewReader(stdout)

	send := func(id int, method string, params any) {
		req := map[string]any{"jsonrpc": "2.0", "method": method}
		if id != 0 {
			req["id"] = id
		}
		if params != nil {
			req["params"] = params
		}
		b, _ := json.Marshal(req)
		if _, err := stdin.Write(append(b, '\n')); err != nil {
			t.Fatalf("write %s: %v", method, err)
		}
	}
	readResult := func(method string) map[string]any {
		line, err := out.ReadBytes('\n')
		if err != nil && err != io.EOF {
			t.Fatalf("read %s response: %v", method, err)
		}
		var resp struct {
			Result map[string]any `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(line, &resp); err != nil {
			t.Fatalf("decode %s response %q: %v", method, line, err)
		}
		if resp.Error != nil {
			t.Fatalf("%s rpc error: %s", method, resp.Error.Message)
		}
		return resp.Result
	}

	// initialize → the launched server identifies itself as dig.
	send(1, "initialize", map[string]any{"protocolVersion": "2025-06-18"})
	init := readResult("initialize")
	if init["protocolVersion"] != "2025-06-18" {
		t.Fatalf("server did not echo protocolVersion: %v", init["protocolVersion"])
	}
	info, _ := init["serverInfo"].(map[string]any)
	if info["name"] != "dig" {
		t.Fatalf("serverInfo.name = %v, want dig", info["name"])
	}
	send(0, "notifications/initialized", nil)

	// tools/list → the dig surface the plugin promises is actually advertised.
	send(2, "tools/list", nil)
	list := readResult("tools/list")
	tools, _ := list["tools"].([]any)
	names := map[string]bool{}
	for _, raw := range tools {
		if tl, ok := raw.(map[string]any); ok {
			if n, ok := tl["name"].(string); ok {
				names[n] = true
			}
		}
	}
	for _, want := range []string{"dig_find", "dig_recall", "dig_retain"} {
		if !names[want] {
			t.Fatalf("plugin's MCP server did not advertise %s (got %v)", want, names)
		}
	}
}
