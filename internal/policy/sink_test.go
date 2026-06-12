package policy

import (
	"strings"
	"testing"
)

func TestEventSinkValidation(t *testing.T) {
	base := `
[[rule]]
name  = "all"
match = { path = "*" }
label = ["seen"]
`
	cases := map[string]struct {
		toml    string
		wantErr string
	}{
		"valid webhook": {
			toml: base + `
[[event_sink]]
type = "webhook"
url  = "https://example.com/hook"
`,
		},
		"valid exec": {
			toml: base + `
[[event_sink]]
on      = "changeset.committed"
type    = "exec"
command = "echo hi"
`,
		},
		"unknown type": {
			toml:    base + "[[event_sink]]\ntype = \"carrier-pigeon\"\n",
			wantErr: "must be exec or webhook",
		},
		"webhook without url": {
			toml:    base + "[[event_sink]]\ntype = \"webhook\"\n",
			wantErr: "needs a url",
		},
		"exec without command": {
			toml:    base + "[[event_sink]]\ntype = \"exec\"\n",
			wantErr: "needs a command",
		},
		"bad event": {
			toml:    base + "[[event_sink]]\non = \"file.deleted\"\ntype = \"webhook\"\nurl = \"https://x\"\n",
			wantErr: "changeset.committed",
		},
		"unknown key rejected": {
			toml:    base + "[[event_sink]]\ntype = \"webhook\"\nurl = \"https://x\"\nretries = 3\n",
			wantErr: "unknown policy key",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(tc.toml))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
