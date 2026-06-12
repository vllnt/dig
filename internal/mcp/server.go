// Package mcp implements a minimal Model Context Protocol server over a stdio
// transport: newline-delimited JSON-RPC 2.0. It is transport-and-protocol only
// — tools are registered by the caller (cmd `dig mcp`), so any agent harness
// that speaks MCP can drive dig's surface.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// protocolVersion is the MCP revision this server defaults to when a client
// does not request one. When the client sends a version we echo it back —
// this server is tool-only and version-agnostic.
const protocolVersion = "2024-11-05"

// Tool is a callable exposed to MCP clients.
type Tool struct {
	// Name is the unique tool id (e.g. "dig_find").
	Name string
	// Description tells the agent when to use the tool.
	Description string
	// InputSchema is the tool's JSON Schema for arguments.
	InputSchema json.RawMessage
	// Handler runs the tool; raw is the client-supplied arguments object. The
	// returned string is sent back as text content; an error becomes an
	// isError tool result (not a transport error).
	Handler func(raw json.RawMessage) (string, error)
}

// Server is a registry of tools served over one stdio connection.
type Server struct {
	name    string
	version string
	mu      sync.Mutex
	order   []string
	tools   map[string]Tool
}

// NewServer creates a server advertising the given name + version.
func NewServer(name, version string) *Server {
	return &Server{name: name, version: version, tools: map[string]Tool{}}
}

// Register adds a tool. Later registrations override an earlier same name.
func (s *Server) Register(t Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tools[t.Name]; !ok {
		s.order = append(s.order, t.Name)
	}
	s.tools[t.Name] = t
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve runs the read/dispatch loop until in is exhausted (EOF). Each line is
// one JSON-RPC message; responses are written newline-delimited to out.
// Notifications (no id) get no response. Returns nil on clean EOF.
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	w := bufio.NewWriter(out)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			if resp, ok := s.handle(line); ok {
				if err := writeMessage(w, resp); err != nil {
					return err
				}
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// handle parses and dispatches one message. ok is false for notifications and
// unparseable input (no response is written).
func (s *Server) handle(line []byte) (response, bool) {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return response{}, false
	}
	if len(req.ID) == 0 {
		return response{}, false // notification — e.g. notifications/initialized
	}
	resp := response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = s.initialize(req.Params)
	case "tools/list":
		resp.Result = s.listTools()
	case "tools/call":
		resp.Result = s.callTool(req.Params)
	case "ping":
		resp.Result = map[string]any{}
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
	return resp, true
}

func (s *Server) initialize(params json.RawMessage) map[string]any {
	version := protocolVersion
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if json.Unmarshal(params, &p) == nil && p.ProtocolVersion != "" {
		version = p.ProtocolVersion
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": s.name, "version": s.version},
	}
}

type toolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

func (s *Server) listTools() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	infos := make([]toolInfo, 0, len(s.order))
	for _, name := range s.order {
		t := s.tools[name]
		infos = append(infos, toolInfo{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
	}
	return map[string]any{"tools": infos}
}

func (s *Server) callTool(params json.RawMessage) map[string]any {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return toolResult("invalid tools/call params: "+err.Error(), true)
	}
	s.mu.Lock()
	t, ok := s.tools[call.Name]
	s.mu.Unlock()
	if !ok {
		return toolResult("unknown tool: "+call.Name, true)
	}
	text, err := t.Handler(call.Arguments)
	if err != nil {
		return toolResult(fmt.Sprintf("%s failed: %v", call.Name, err), true)
	}
	return toolResult(text, false)
}

// toolResult builds an MCP tool result with a single text content block.
func toolResult(text string, isError bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isError,
	}
}

func writeMessage(w *bufio.Writer, resp response) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	return w.Flush()
}
