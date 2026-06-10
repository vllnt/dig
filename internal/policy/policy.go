// Package policy parses and validates a KB's organization policy: declarative
// [[rule]] entries that decide where files belong, what they are named, and
// which labels they carry. Rules are deterministic — no AI on this path.
//
// The policy file lives at .dig/policy.toml and travels with the KB.
package policy

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// File is the policy file name inside a KB's .dig directory.
const File = "policy.toml"

// UnsortedLabel marks files no rule matched.
const UnsortedLabel = "unsorted"

// PinnedLabel marks files a human deliberately placed (detected as a rename/
// move by the reconcile loop). Policy never auto-moves a pinned entry — rule
// disagreements on it are escalated instead. Removing the label re-subjects
// the file to policy.
const PinnedLabel = "dig:pinned"

// Policy is the parsed, validated policy document.
type Policy struct {
	Rules []Rule      `toml:"rule"`
	Dedup DedupPolicy `toml:"dedup"`
}

// Rule maps matching files to a target folder, name, and labels.
// At least one of Into / Rename / Label must be set.
//
// Autonomy is earned rule-by-rule (architecture.md §3): in watch mode only
// rules marked "auto" apply unattended — everything else proposes. In an
// explicit one-shot reconcile the user's invocation is consent, so rules
// apply unless marked "propose".
type Rule struct {
	Name     string   `toml:"name"`
	Match    Match    `toml:"match"`
	Into     string   `toml:"into"`   // target dir template, KB-root-relative
	Rename   string   `toml:"rename"` // target filename template
	Label    []string `toml:"label"`
	Autonomy string   `toml:"autonomy"` // "" (default) | "auto" | "propose"
}

// Match holds the conditions a file must meet. All set fields must hold (AND).
type Match struct {
	Ext            []string `toml:"ext"`             // extensions, no dot: ["pdf"]
	Mime           []string `toml:"mime"`            // mime prefix match: "image/*" or exact
	Path           string   `toml:"path"`            // glob on KB-relative path
	ContentMatches string   `toml:"content_matches"` // regexp over file content (case-insensitive)
}

// DedupPolicy configures duplicate collapsing (used by the dedupe phase).
type DedupPolicy struct {
	Strategy   string `toml:"strategy"`    // keep-oldest | keep-newest
	OnConflict string `toml:"on_conflict"` // escalate (default) — never silently delete
}

// Load reads and validates the policy at path. Unknown keys are errors —
// a typoed key silently matching nothing is worse than failing loudly.
func Load(file string) (*Policy, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return Parse(raw)
}

// Parse decodes and validates policy TOML.
func Parse(raw []byte) (*Policy, error) {
	var p Policy
	meta, err := toml.Decode(string(raw), &p)
	if err != nil {
		return nil, fmt.Errorf("parse policy: %w", err)
	}
	if undec := meta.Undecoded(); len(undec) > 0 {
		keys := make([]string, len(undec))
		for i, k := range undec {
			keys[i] = k.String()
		}
		return nil, fmt.Errorf("unknown policy key(s): %s", strings.Join(keys, ", "))
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// templateVar matches {var} placeholders in into/rename templates.
var templateVar = regexp.MustCompile(`\{([a-z_]+)\}`)

// knownVars are the metadata-derived template variables available in this
// phase. Content-derived fields ({vendor}, ...) arrive with extractors.
var knownVars = map[string]bool{
	"year": true, "month": true, "day": true,
	"name": true, "ext": true,
}

// Validate enforces structural rules. BLOCKING errors — a policy that fails
// validation is never applied.
func (p *Policy) Validate() error {
	if len(p.Rules) == 0 {
		return fmt.Errorf("policy has no rules")
	}
	seen := map[string]bool{}
	for i := range p.Rules {
		r := &p.Rules[i]
		if r.Name == "" {
			return fmt.Errorf("rule %d: missing name", i+1)
		}
		if seen[r.Name] {
			return fmt.Errorf("rule %q: duplicate name", r.Name)
		}
		seen[r.Name] = true

		if r.Into == "" && r.Rename == "" && len(r.Label) == 0 {
			return fmt.Errorf("rule %q: must set at least one of into/rename/label", r.Name)
		}
		for _, tmpl := range []string{r.Into, r.Rename} {
			if err := validateTemplate(tmpl); err != nil {
				return fmt.Errorf("rule %q: %w", r.Name, err)
			}
		}
		if r.Match.Path != "" {
			if _, err := path.Match(r.Match.Path, "probe"); err != nil {
				return fmt.Errorf("rule %q: bad path glob %q: %w", r.Name, r.Match.Path, err)
			}
		}
		if r.Match.ContentMatches != "" {
			if _, err := regexp.Compile("(?i)" + r.Match.ContentMatches); err != nil {
				return fmt.Errorf("rule %q: bad content_matches regexp: %w", r.Name, err)
			}
		}
		if r.Match.Ext == nil && r.Match.Mime == nil && r.Match.Path == "" && r.Match.ContentMatches == "" {
			return fmt.Errorf("rule %q: empty match would apply to every file; use path = \"*\" to be explicit", r.Name)
		}
		switch r.Autonomy {
		case "", "auto", "propose":
		default:
			return fmt.Errorf("rule %q: autonomy %q must be auto or propose", r.Name, r.Autonomy)
		}
	}
	switch p.Dedup.Strategy {
	case "", "keep-oldest", "keep-newest":
	default:
		return fmt.Errorf("dedup.strategy %q: must be keep-oldest or keep-newest", p.Dedup.Strategy)
	}
	switch p.Dedup.OnConflict {
	case "", "escalate":
	default:
		return fmt.Errorf("dedup.on_conflict %q: only escalate is supported", p.Dedup.OnConflict)
	}
	return nil
}

// validateTemplate rejects unknown variables and path escapes. Targets must
// stay inside the KB root — a template that climbs out of the KB is an attack
// or a mistake; either way it is refused at validation time.
func validateTemplate(tmpl string) error {
	if tmpl == "" {
		return nil
	}
	for _, m := range templateVar.FindAllStringSubmatch(tmpl, -1) {
		if !knownVars[m[1]] {
			return fmt.Errorf("unknown template variable {%s} (known: year, month, day, name, ext)", m[1])
		}
	}
	if strings.HasPrefix(tmpl, "/") {
		return fmt.Errorf("template %q: absolute paths not allowed", tmpl)
	}
	for _, seg := range strings.Split(tmpl, "/") {
		if seg == ".." {
			return fmt.Errorf("template %q: path escape (..) not allowed", tmpl)
		}
	}
	return nil
}
