package policy

import (
	"strings"
	"testing"
	"time"
)

const validPolicy = `
[[rule]]
name  = "invoices"
match = { ext = ["pdf"], content_matches = "invoice" }
into  = "finance/invoices/{year}"
label = ["finance"]

[[rule]]
name  = "photos"
match = { mime = ["image/*"] }
into  = "media/{year}/{month}"

[dedup]
strategy    = "keep-oldest"
on_conflict = "escalate"
`

func TestParseValid(t *testing.T) {
	p, err := Parse([]byte(validPolicy))
	if err != nil {
		t.Fatalf("valid policy rejected: %v", err)
	}
	if len(p.Rules) != 2 || p.Rules[0].Name != "invoices" {
		t.Fatalf("parse shape wrong: %+v", p)
	}
	if p.Dedup.Strategy != "keep-oldest" {
		t.Fatalf("dedup not parsed: %+v", p.Dedup)
	}
}

// Contract POV: malformed policies must fail loudly at parse time, never
// silently match nothing.
func TestValidationRejects(t *testing.T) {
	cases := map[string]string{
		"unknown key": `
[[rule]]
name = "r"
match = { ext = ["pdf"] }
into = "x"
typoed_key = true`,
		"duplicate name": `
[[rule]]
name = "r"
match = { ext = ["a"] }
into = "x"
[[rule]]
name = "r"
match = { ext = ["b"] }
into = "y"`,
		"no action": `
[[rule]]
name = "r"
match = { ext = ["pdf"] }`,
		"empty match": `
[[rule]]
name = "r"
into = "x"`,
		"unknown template var": `
[[rule]]
name = "r"
match = { ext = ["pdf"] }
into = "docs/{vendor}"`,
		"bad regexp": `
[[rule]]
name = "r"
match = { content_matches = "([" }
into = "x"`,
		"bad dedup strategy": `
[[rule]]
name = "r"
match = { ext = ["pdf"] }
into = "x"
[dedup]
strategy = "keep-biggest"`,
	}
	for name, src := range cases {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("%s: accepted invalid policy", name)
		}
	}
}

// Security POV: templates must never climb out of the KB root.
func TestValidationRejectsPathEscape(t *testing.T) {
	for _, tmpl := range []string{"../outside", "a/../../b", "/etc/cron.d"} {
		src := `
[[rule]]
name = "evil"
match = { ext = ["pdf"] }
into = "` + tmpl + `"`
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("path escape %q accepted", tmpl)
		}
	}
}

func compile(t *testing.T, src string) []CompiledRule {
	t.Helper()
	p, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rules, err := p.Compile()
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return rules
}

func TestMatchers(t *testing.T) {
	rules := compile(t, validPolicy)
	inv, photo := &rules[0], &rules[1]

	content := func(string) ([]byte, error) { return []byte("ACME Invoice #1007"), nil }

	// ext + content, case-insensitive on both.
	if ok, _ := inv.Matches("inbox/scan.PDF", content); !ok {
		t.Error("ext should match case-insensitively and content_matches case-insensitively")
	}
	// ext mismatch short-circuits before content.
	if ok, _ := inv.Matches("inbox/scan.txt", nil); ok {
		t.Error("txt must not match ext=[pdf]")
	}
	// content_matches with non-matching content.
	noMatch := func(string) ([]byte, error) { return []byte("holiday photo"), nil }
	if ok, _ := inv.Matches("inbox/scan.pdf", noMatch); ok {
		t.Error("content gate failed")
	}
	// mime wildcard.
	if ok, _ := photo.Matches("x/y/beach.jpg", nil); !ok {
		t.Error("image/* should match .jpg")
	}
	if ok, _ := photo.Matches("x/y/doc.pdf", nil); ok {
		t.Error("image/* must not match .pdf")
	}
}

// Robustness POV: binary content must not break content matching.
func TestContentMatchBinarySafe(t *testing.T) {
	rules := compile(t, validPolicy)
	binary := func(string) ([]byte, error) { return []byte{0x00, 0xFF, 0x1B, 0x00, 'i', 'n', 'v'}, nil }
	if _, err := rules[0].Matches("blob.pdf", binary); err != nil {
		t.Fatalf("binary content errored: %v", err)
	}
}

func TestPathGlobMatchesBasename(t *testing.T) {
	rules := compile(t, `
[[rule]]
name = "drafts"
match = { path = "*.draft" }
label = ["draft"]`)
	if ok, _ := rules[0].Matches("deep/nested/note.draft", nil); !ok {
		t.Error("basename glob should match at any depth")
	}
}

func TestTemplateExpansionAndTarget(t *testing.T) {
	mod := time.Date(2024, 3, 7, 0, 0, 0, 0, time.UTC)
	rules := compile(t, `
[[rule]]
name   = "r"
match  = { ext = ["pdf"] }
into   = "finance/{year}/{month}"
rename = "{name}-archived.{ext}"`)

	got := rules[0].Target("inbox/acme.pdf", mod)
	want := "finance/2024/03/acme-archived.pdf"
	if got != want {
		t.Fatalf("target = %q, want %q", got, want)
	}
}

// Label-only rule keeps the path untouched.
func TestTargetWithoutIntoKeepsDir(t *testing.T) {
	rules := compile(t, `
[[rule]]
name   = "r"
match  = { ext = ["md"] }
rename = "{name}.{ext}"`)
	if got := rules[0].Target("notes/a.md", time.Now()); got != "notes/a.md" {
		t.Fatalf("dir should be preserved, got %q", got)
	}
}

func TestUnknownKeysErrorListsThem(t *testing.T) {
	_, err := Parse([]byte(`
[[rule]]
name = "r"
match = { ext = ["pdf"] }
into = "x"
bogus = 1`))
	if err == nil || !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("error should name the unknown key, got: %v", err)
	}
}
