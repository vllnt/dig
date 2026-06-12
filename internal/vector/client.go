// Package vector provides opt-in semantic retrieval: an embeddings client
// against any OpenAI-compatible endpoint and a local vector index derived
// from manifests. Per docs/architecture.md §5 the AI layer is optional and
// off by default — FTS stays the deterministic default; vectors only add
// paraphrase recall when a [retrieval] policy turns them on.
package vector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client embeds text via an OpenAI-compatible /embeddings endpoint
// (Ollama, llama.cpp, LM Studio, vLLM, LiteLLM, ...). One client,
// base_url + model — no vendor SDK.
type Client struct {
	BaseURL     string // e.g. http://127.0.0.1:8092/v1
	Model       string
	APIKey      string // optional bearer token
	DocPrefix   string // prepended to documents (e.g. "search_document: ")
	QueryPrefix string // prepended to queries (e.g. "search_query: ")

	ChunkSize    int // document chunk length in chars (<=0 = default)
	ChunkOverlap int // overlap between chunks in chars

	HTTP *http.Client
}

// NewClient builds a Client from retrieval policy settings. The API key is
// read from the environment variable named by apiKeyEnv — the key itself
// never lives in the policy file. chunkSize/chunkOverlap configure document
// chunking; 0 selects the defaults.
func NewClient(baseURL, model, apiKeyEnv, docPrefix, queryPrefix string, chunkSize, chunkOverlap int) *Client {
	key := ""
	if apiKeyEnv != "" {
		key = os.Getenv(apiKeyEnv)
	}
	return &Client{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		Model:        model,
		APIKey:       key,
		DocPrefix:    docPrefix,
		QueryPrefix:  queryPrefix,
		ChunkSize:    chunkSize,
		ChunkOverlap: chunkOverlap,
		HTTP:         &http.Client{Timeout: 120 * time.Second},
	}
}

// exceedsContext recognizes "input too long for the model window" errors
// across common OpenAI-compatible runtimes (llama.cpp, vLLM, Ollama).
func exceedsContext(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context") &&
		(strings.Contains(msg, "exceed") || strings.Contains(msg, "larger than") || strings.Contains(msg, "too long"))
}

// truncateHalf halves a text by runes (UTF-8 safe).
func truncateHalf(s string) string {
	r := []rune(s)
	return string(r[:len(r)/2])
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// maxBatch bounds how many texts go in one request; chunks are small
// (~chunkSize chars) so this stays well inside typical context budgets and
// keeps a multi-slot local server's queue fed.
const maxBatch = 64

// EmbedDocs embeds document chunks (DocPrefix applied), batching requests.
func (c *Client) EmbedDocs(texts []string) ([][]float32, error) {
	return c.embedAll(texts, c.DocPrefix)
}

// EmbedQuery embeds a single search query (QueryPrefix applied).
func (c *Client) EmbedQuery(q string) ([]float32, error) {
	out, err := c.embedAll([]string{q}, c.QueryPrefix)
	if err != nil {
		return nil, err
	}
	return out[0], nil
}

// EmbedQueries embeds many search queries in batches (QueryPrefix applied) —
// the query-many path eval harnesses use.
func (c *Client) EmbedQueries(qs []string) ([][]float32, error) {
	return c.embedAll(qs, c.QueryPrefix)
}

func (c *Client) embedAll(texts []string, prefix string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += maxBatch {
		end := min(start+maxBatch, len(texts))
		batch := make([]string, end-start)
		for i, t := range texts[start:end] {
			batch[i] = prefix + t
		}
		vecs, err := c.embedBatch(batch)
		if err != nil && exceedsContext(err) {
			// A chunk can out-token a small endpoint's window (token/char
			// ratios vary by text). Degrade gracefully: halve the inputs and
			// retry once — a truncated embedding beats a failed rebuild.
			for i, t := range batch {
				batch[i] = truncateHalf(t)
			}
			vecs, err = c.embedBatch(batch)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, vecs...)
	}
	return out, nil
}

func (c *Client) embedBatch(texts []string) ([][]float32, error) {
	body, err := json.Marshal(embedRequest{Model: c.Model, Input: texts})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("embedding endpoint %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	var er embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, fmt.Errorf("decode embeddings: %w", err)
	}
	if len(er.Data) != len(texts) {
		return nil, fmt.Errorf("embedding endpoint returned %d vectors for %d inputs", len(er.Data), len(texts))
	}
	vecs := make([][]float32, len(texts))
	for _, d := range er.Data {
		if d.Index < 0 || d.Index >= len(vecs) {
			return nil, fmt.Errorf("embedding endpoint returned out-of-range index %d", d.Index)
		}
		vecs[d.Index] = normalize(d.Embedding)
	}
	return vecs, nil
}
