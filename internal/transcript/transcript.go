// Package transcript renders an agent session transcript (Claude Code's
// newline-delimited JSON) into readable markdown: the user and assistant turns,
// with tool calls summarized to one line and internal noise (thinking, tool
// outputs, injected system reminders) dropped. It is the input adapter a
// retention hook pipes into `dig retain`, so a captured session is something
// `dig find` / `dig recall` can surface — not a wall of raw JSON.
//
// The parser is deliberately tolerant: transcripts mix many line shapes and a
// single malformed or oversized line must never lose the whole session, so
// unparseable lines are skipped rather than failing the render.
package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// line is one transcript record. Only type, the meta flag, and message are
// needed; everything else (attachments, snapshots, mode changes, ...) is
// ignored. isMeta marks harness-injected messages (skill bodies, command
// caveats, hook prompts) — content the human never wrote, dropped from memory.
type line struct {
	Type    string          `json:"type"`
	IsMeta  bool            `json:"isMeta"`
	Message json.RawMessage `json:"message"`
}

// message is the chat message inside a user/assistant line. Content is either a
// plain string (a user prompt) or an array of typed blocks (assistant output,
// or a user message carrying tool results).
type message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// block is one content block: text, thinking, tool_use, or tool_result.
type block struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// systemReminder strips harness-injected reminder blocks from user text — they
// are context for the model, not part of what the human said, and only pollute
// recall.
var systemReminder = regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`)

// commandName / commandArgs pull a slash-command invocation out of its XML
// wrapper, so "/whats-next" reads as a command rather than a tag soup.
var commandName = regexp.MustCompile(`(?s)<command-name>(.*?)</command-name>`)
var commandArgs = regexp.MustCompile(`(?s)<command-args>(.*?)</command-args>`)

// toolInputFields are tried in order for a one-line tool-call summary.
var toolInputFields = []string{"description", "command", "query", "pattern", "file_path", "path", "prompt"}

// Render reads a JSONL transcript and returns markdown of the conversation.
// Returns an empty string (no error) when the transcript holds no renderable
// turns.
func Render(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	var b strings.Builder
	for {
		raw, err := br.ReadBytes('\n')
		if len(raw) > 0 {
			if section := renderLine(raw); section != "" {
				if b.Len() > 0 {
					b.WriteString("\n\n")
				}
				b.WriteString(section)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return b.String(), err
		}
	}
	return b.String(), nil
}

// renderLine turns one transcript record into a markdown section, or "" to skip
// it. A line that does not parse is skipped, never fatal.
func renderLine(raw []byte) string {
	var l line
	if json.Unmarshal(raw, &l) != nil {
		return ""
	}
	if l.Type != "user" && l.Type != "assistant" {
		return ""
	}
	if l.IsMeta {
		return "" // harness-injected (skill body, caveat, hook prompt) — not the human
	}
	var m message
	if json.Unmarshal(l.Message, &m) != nil || len(m.Content) == 0 {
		return ""
	}

	// User content is usually a plain string prompt.
	var str string
	if json.Unmarshal(m.Content, &str) == nil {
		body := cleanUserText(str)
		if body == "" {
			return ""
		}
		return "## User\n\n" + body
	}

	// Otherwise it is an array of blocks.
	var blocks []block
	if json.Unmarshal(m.Content, &blocks) != nil {
		return ""
	}
	if l.Type == "user" {
		return renderUserBlocks(blocks)
	}
	return renderAssistantBlocks(blocks)
}

// renderUserBlocks keeps only real user text (e.g. a prompt sent alongside an
// attachment); tool results are dropped as noise.
func renderUserBlocks(blocks []block) string {
	var parts []string
	for _, bl := range blocks {
		if bl.Type == "text" {
			if t := cleanUserText(bl.Text); t != "" {
				parts = append(parts, t)
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "## User\n\n" + strings.Join(parts, "\n\n")
}

// renderAssistantBlocks emits assistant prose and a compact summary of each tool
// call; thinking blocks are dropped.
func renderAssistantBlocks(blocks []block) string {
	var prose []string
	var tools []string
	for _, bl := range blocks {
		switch bl.Type {
		case "text":
			if t := strings.TrimSpace(bl.Text); t != "" {
				prose = append(prose, t)
			}
		case "tool_use":
			tools = append(tools, "- "+toolSummary(bl))
		}
	}
	if len(prose) == 0 && len(tools) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Assistant\n\n")
	b.WriteString(strings.Join(prose, "\n\n"))
	if len(tools) > 0 {
		if len(prose) > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("_tools:_\n")
		b.WriteString(strings.Join(tools, "\n"))
	}
	return b.String()
}

// toolSummary builds a one-line summary of a tool call: the tool name plus the
// most descriptive input field available, truncated.
func toolSummary(bl block) string {
	name := bl.Name
	if name == "" {
		name = "tool"
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(bl.Input, &fields)
	for _, key := range toolInputFields {
		if raw, ok := fields[key]; ok {
			var v string
			if json.Unmarshal(raw, &v) == nil && strings.TrimSpace(v) != "" {
				return fmt.Sprintf("%s: %s", name, truncateLine(v, 100))
			}
		}
	}
	return name
}

// cleanUserText normalizes a user message for memory: a slash-command
// invocation collapses to "/name args"; injected system reminders and
// local-command output are stripped. Returns "" when nothing meaningful
// remains.
func cleanUserText(s string) string {
	// A command invocation is XML wrapper, not prose — render just the command.
	if m := commandName.FindStringSubmatch(s); m != nil {
		cmd := strings.TrimSpace(m[1])
		if cmd == "" {
			return ""
		}
		if a := commandArgs.FindStringSubmatch(s); a != nil {
			if args := strings.TrimSpace(a[1]); args != "" {
				return cmd + " " + args
			}
		}
		return cmd
	}
	s = systemReminder.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Local-command caveat dumps are tool plumbing, not conversation.
	if strings.HasPrefix(s, "<local-command-caveat>") {
		return ""
	}
	return s
}

// truncateLine collapses whitespace and caps a summary to n runes.
func truncateLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
