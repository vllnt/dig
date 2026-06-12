package vector

import (
	"math"
	"strings"
)

// Chunking parameters: ~1000 chars ≈ 250 tokens per chunk keeps every chunk
// well inside small-model context windows; the overlap preserves continuity
// across boundaries so a fact straddling two chunks still embeds whole once.
const (
	chunkSize    = 1000
	chunkOverlap = 200
)

// Chunk splits text into overlapping chunks, preferring paragraph then
// sentence then word boundaries near the target size. Deterministic — the
// same text always yields the same chunks, which keeps blob-keyed embedding
// caching sound.
func Chunk(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) <= chunkSize {
		return []string{text}
	}
	var out []string
	for start := 0; start < len(text); {
		end := start + chunkSize
		if end >= len(text) {
			out = append(out, strings.TrimSpace(text[start:]))
			break
		}
		cut := boundary(text, start, end)
		piece := strings.TrimSpace(text[start:cut])
		if piece != "" {
			out = append(out, piece)
		}
		next := cut - chunkOverlap
		if next <= start {
			next = cut
		}
		start = next
	}
	return out
}

// boundary finds a natural cut point at or before end: paragraph break,
// then sentence end, then whitespace, else the hard limit.
func boundary(text string, start, end int) int {
	window := text[start:end]
	for _, sep := range []string{"\n\n", ". ", "\n", " "} {
		if i := strings.LastIndex(window, sep); i > chunkSize/2 {
			return start + i + len(sep)
		}
	}
	return end
}

// normalize L2-normalizes v in place so cosine similarity reduces to a dot
// product at query time.
func normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return v
	}
	inv := float32(1 / math.Sqrt(sum))
	for i := range v {
		v[i] *= inv
	}
	return v
}

// dot computes the inner product of two same-length vectors.
func dot(a, b []float32) float32 {
	var s float32
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}
