package policy

import (
	"strings"
	"testing"
)

func TestRetrievalPolicyValidation(t *testing.T) {
	cases := []struct {
		name    string
		toml    string
		wantErr string
	}{
		{
			name: "retrieval-only policy needs no rules",
			toml: `
[retrieval]
mode = "hybrid"
base_url = "http://127.0.0.1:8092/v1"
model = "nomic-embed-text-v1.5"
`,
		},
		{
			name: "hybrid with prefixes and key env",
			toml: `
[retrieval]
mode = "vector"
base_url = "http://127.0.0.1:8092/v1"
model = "m"
api_key_env = "DIG_EMBED_KEY"
doc_prefix = "search_document: "
query_prefix = "search_query: "
`,
		},
		{
			name: "mode off alone still requires rules",
			toml: `
[retrieval]
mode = "off"
`,
			wantErr: "no rules",
		},
		{
			name: "unknown mode rejected",
			toml: `
[retrieval]
mode = "semantic"
base_url = "http://x"
model = "m"
`,
			wantErr: "retrieval.mode",
		},
		{
			name: "enabled without endpoint rejected",
			toml: `
[retrieval]
mode = "hybrid"
`,
			wantErr: "requires base_url and model",
		},
		{
			name: "inline api_key is not a key the schema knows — keys live in env vars",
			toml: `
[retrieval]
mode = "hybrid"
base_url = "http://x"
model = "m"
api_key = "placeholder-never-put-real-keys-in-policy"
`,
			wantErr: "unknown policy key",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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

func TestRetrievalEnabled(t *testing.T) {
	for mode, want := range map[string]bool{
		"": false, "off": false, "hybrid": true, "vector": true,
	} {
		if got := (RetrievalPolicy{Mode: mode}).Enabled(); got != want {
			t.Fatalf("Enabled(%q) = %v, want %v", mode, got, want)
		}
	}
}
