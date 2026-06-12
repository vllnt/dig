package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vllnt/dig/internal/mcp"
)

// newMcpCmd runs dig as an MCP server over stdio, exposing the CLI surface as
// tools any MCP client (Claude, Cursor, the AI SDK, ...) can drive. Tools
// execute the real dig commands in-process with --json, so there is no logic
// to drift from the CLI. Mutating tools default to a dry-run preview; an
// explicit apply runs the change, which stays journaled and undo-able.
func newMcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run dig as an MCP server (stdio) for agent harnesses",
		Long: "Speaks the Model Context Protocol over stdio. Register it with any MCP\n" +
			"client to give an agent dig's surface: find, recall, drift, log, export\n" +
			"(read-only), retain (capture into memory), and org / reconcile (preview by\n" +
			"default; pass apply=true to commit — every change is reversible with undo).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv := mcp.NewServer("dig", Version.Version)
			registerDigTools(srv)
			return srv.Serve(cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}

// digJSON runs the dig CLI in-process with the given args and returns its
// captured output. Reuses the whole command tree — the MCP layer is a pure
// protocol adapter.
func digJSON(args ...string) (string, error) {
	root := NewRoot()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// digJSONStdin runs the dig CLI in-process with content on stdin — for capture
// tools (dig_retain) whose CLI form reads the body from stdin.
func digJSONStdin(stdin string, args ...string) (string, error) {
	root := NewRoot()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// kbArgs prefixes --kb when the tool call named a KB.
func kbArgs(kb string, rest ...string) []string {
	if kb == "" {
		return rest
	}
	return append([]string{"--kb", kb}, rest...)
}

func schema(raw string) json.RawMessage { return json.RawMessage(raw) }

// registerDigTools wires the dig command surface as MCP tools.
func registerDigTools(srv *mcp.Server) {
	srv.Register(mcp.Tool{
		Name:        "dig_find",
		Description: "Search a dig knowledge base, ranked. mode is fts (default), vector, or hybrid (semantic). Returns JSON results.",
		InputSchema: schema(`{"type":"object","properties":{"kb":{"type":"string","description":"KB name or path; omit to use the KB at the working directory"},"query":{"type":"string"},"mode":{"type":"string","enum":["fts","vector","hybrid"]},"limit":{"type":"integer"}},"required":["query"]}`),
		Handler: func(raw json.RawMessage) (string, error) {
			var a struct {
				KB    string `json:"kb"`
				Query string `json:"query"`
				Mode  string `json:"mode"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", err
			}
			if a.Query == "" {
				return "", fmt.Errorf("query is required")
			}
			args := []string{"find", a.Query, "--json"}
			if a.Mode != "" {
				args = append(args, "--mode", a.Mode)
			}
			if a.Limit > 0 {
				args = append(args, "--limit", fmt.Sprint(a.Limit))
			}
			return digJSON(kbArgs(a.KB, args...)...)
		},
	})

	srv.Register(mcp.Tool{
		Name:        "dig_retain",
		Description: "Capture content (an agent session, a note, a document) into a dig KB and index it — the agent-memory capture primitive. Writes to a dated memory/ path by default; pass 'as' to choose the path. Reversible with dig_undo.",
		InputSchema: schema(`{"type":"object","properties":{"kb":{"type":"string"},"content":{"type":"string","description":"the text to capture into memory"},"as":{"type":"string","description":"target path in the KB; default a dated memory/ path"}},"required":["content"]}`),
		Handler: func(raw json.RawMessage) (string, error) {
			var a struct {
				KB      string `json:"kb"`
				Content string `json:"content"`
				As      string `json:"as"`
			}
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", err
			}
			if a.Content == "" {
				return "", fmt.Errorf("content is required")
			}
			args := []string{"retain"}
			if a.As != "" {
				args = append(args, "--as", a.As)
			}
			return digJSONStdin(a.Content, kbArgs(a.KB, args...)...)
		},
	})

	srv.Register(mcp.Tool{
		Name:        "dig_recall",
		Description: "Load a token-budgeted, provenance-tagged context pack from a dig KB for a query — the agent-memory recall primitive. mode is fts (default), vector, or hybrid; budget caps the pack in tokens. Returns JSON.",
		InputSchema: schema(`{"type":"object","properties":{"kb":{"type":"string"},"query":{"type":"string"},"mode":{"type":"string","enum":["fts","vector","hybrid"]},"budget":{"type":"integer","description":"token budget for the pack"}},"required":["query"]}`),
		Handler: func(raw json.RawMessage) (string, error) {
			var a struct {
				KB     string `json:"kb"`
				Query  string `json:"query"`
				Mode   string `json:"mode"`
				Budget int    `json:"budget"`
			}
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", err
			}
			if a.Query == "" {
				return "", fmt.Errorf("query is required")
			}
			args := []string{"recall", a.Query, "--json"}
			if a.Mode != "" {
				args = append(args, "--mode", a.Mode)
			}
			if a.Budget > 0 {
				args = append(args, "--budget", fmt.Sprint(a.Budget))
			}
			return digJSON(kbArgs(a.KB, args...)...)
		},
	})

	srv.Register(mcp.Tool{
		Name:        "dig_drift",
		Description: "Report how a dig KB diverges from its policy (misfiled, misnamed, duplicated, unsorted, external edits). Read-only. Returns JSON.",
		InputSchema: schema(`{"type":"object","properties":{"kb":{"type":"string"}}}`),
		Handler: func(raw json.RawMessage) (string, error) {
			kb := kbField(raw)
			return digJSON(kbArgs(kb, "drift", "--json")...)
		},
	})

	srv.Register(mcp.Tool{
		Name:        "dig_log",
		Description: "Browse a dig KB's change history, newest first. Read-only. Returns JSON.",
		InputSchema: schema(`{"type":"object","properties":{"kb":{"type":"string"}}}`),
		Handler: func(raw json.RawMessage) (string, error) {
			return digJSON(kbArgs(kbField(raw), "log", "--json")...)
		},
	})

	srv.Register(mcp.Tool{
		Name:        "dig_export",
		Description: "Export a reproducible, provenance-tagged dataset (JSONL) from a dig KB. filter selects a slice (e.g. 'label:finance path:*.pdf'); at pins a manifest id. Read-only.",
		InputSchema: schema(`{"type":"object","properties":{"kb":{"type":"string"},"filter":{"type":"string"},"at":{"type":"string"}}}`),
		Handler: func(raw json.RawMessage) (string, error) {
			var a struct {
				KB     string `json:"kb"`
				Filter string `json:"filter"`
				At     string `json:"at"`
			}
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", err
			}
			args := []string{"export"}
			if a.Filter != "" {
				args = append(args, "--filter", a.Filter)
			}
			if a.At != "" {
				args = append(args, "--at", a.At)
			}
			return digJSON(kbArgs(a.KB, args...)...)
		},
	})

	srv.Register(mcp.Tool{
		Name:        "dig_org",
		Description: "Apply organization policy (move/rename/label) to a dig KB. Previews the plan by default; pass apply=true to commit (reversible with dig_undo).",
		InputSchema: schema(`{"type":"object","properties":{"kb":{"type":"string"},"apply":{"type":"boolean","description":"commit the changes; default false = dry-run preview"}}}`),
		Handler:     mutateHandler("org"),
	})

	srv.Register(mcp.Tool{
		Name:        "dig_reconcile",
		Description: "Converge a dig KB to its policy, folding in human edits. Previews by default; pass apply=true to commit (reversible with dig_undo).",
		InputSchema: schema(`{"type":"object","properties":{"kb":{"type":"string"},"apply":{"type":"boolean","description":"commit the convergence; default false = dry-run preview"}}}`),
		Handler:     mutateHandler("reconcile"),
	})

	srv.Register(mcp.Tool{
		Name:        "dig_undo",
		Description: "Revert the last changeset in a dig KB (disk mutations are reversed; undoing a scan only rewinds history).",
		InputSchema: schema(`{"type":"object","properties":{"kb":{"type":"string"}}}`),
		Handler: func(raw json.RawMessage) (string, error) {
			return digJSON(kbArgs(kbField(raw), "undo")...)
		},
	})
}

// mutateHandler builds a handler for a mutating command that defaults to a
// --dry-run preview and commits only when apply=true.
func mutateHandler(command string) func(json.RawMessage) (string, error) {
	return func(raw json.RawMessage) (string, error) {
		var a struct {
			KB    string `json:"kb"`
			Apply bool   `json:"apply"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", err
		}
		args := []string{command}
		if !a.Apply {
			args = append(args, "--dry-run")
		}
		return digJSON(kbArgs(a.KB, args...)...)
	}
}

// kbField extracts an optional "kb" string from a tool's arguments.
func kbField(raw json.RawMessage) string {
	var a struct {
		KB string `json:"kb"`
	}
	_ = json.Unmarshal(raw, &a)
	return a.KB
}
