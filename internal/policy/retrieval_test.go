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

func TestRetrievalTuningDefaults(t *testing.T) {
	rrfK, factor, size, overlap := RetrievalPolicy{}.Tuning()
	if rrfK != DefaultRRFK || factor != DefaultCandidateFactor ||
		size != DefaultChunkSize || overlap != DefaultChunkOverlap {
		t.Fatalf("unset knobs should be defaults, got %d/%d/%d/%d", rrfK, factor, size, overlap)
	}

	rp := RetrievalPolicy{RRFK: 30, CandidateFactor: 8, ChunkSize: 512, ChunkOverlap: 64}
	rrfK, factor, size, overlap = rp.Tuning()
	if rrfK != 30 || factor != 8 || size != 512 || overlap != 64 {
		t.Fatalf("set knobs should pass through, got %d/%d/%d/%d", rrfK, factor, size, overlap)
	}
}

func TestRetrievalTuningValidation(t *testing.T) {
	base := `
[retrieval]
mode = "hybrid"
base_url = "http://x"
model = "m"
`
	cases := map[string]string{
		"negative rrf_k":            base + "rrf_k = -1\n",
		"negative candidate_factor": base + "candidate_factor = -2\n",
		"overlap exceeds chunk":     base + "chunk_size = 100\nchunk_overlap = 100\n",
	}
	for name, toml := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(toml)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	// Valid custom tuning passes.
	if _, err := Parse([]byte(base + "rrf_k = 40\nchunk_size = 800\nchunk_overlap = 100\n")); err != nil {
		t.Fatalf("valid tuning rejected: %v", err)
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
