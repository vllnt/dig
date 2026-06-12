// Package vectortest provides a fake OpenAI-compatible embeddings endpoint
// for tests. The embedding endpoint is a true third-party network boundary —
// the one place a test double is allowed — and this fake keeps semantics
// real: vectors are bag-of-words projections, so texts sharing vocabulary
// genuinely score higher cosine. Ranking behavior under test is real, not
// scripted.
package vectortest

import (
	"encoding/json"
	"hash/fnv"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
)

// Dims is the dimensionality of fake embeddings.
const Dims = 256

// Server is a running fake embeddings endpoint with usage counters.
type Server struct {
	*httptest.Server
	Requests atomic.Int64 // HTTP calls received
	Embedded atomic.Int64 // total texts embedded
}

// New starts the fake endpoint. Callers must Close it (or use t.Cleanup).
func New() *Server {
	s := &Server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			http.NotFound(w, r)
			return
		}
		s.Requests.Add(1)
		var req struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.Embedded.Add(int64(len(req.Input)))
		type datum struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}
		out := struct {
			Data []datum `json:"data"`
		}{}
		for i, text := range req.Input {
			out.Data = append(out.Data, datum{Index: i, Embedding: Embed(text)})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))
	return s
}

// BaseURL returns the OpenAI-style base URL (".../v1") for client config.
func (s *Server) BaseURL() string { return s.URL + "/v1" }

// Embed maps text to a bag-of-words vector: each word hashes to a dimension,
// so shared vocabulary between two texts yields higher cosine similarity.
func Embed(text string) []float32 {
	v := make([]float32, Dims)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		h := fnv.New32a()
		_, _ = h.Write([]byte(strings.Trim(w, ".,!?:;\"'")))
		v[h.Sum32()%Dims]++
	}
	return v
}
