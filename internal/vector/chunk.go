package vector

import (
	"math"
	"strings"
)

// Default chunking parameters: ~1000 chars ≈ 250 tokens per chunk keeps every
// chunk well inside small-model context windows; the overlap preserves
// continuity across boundaries so a fact straddling two chunks still embeds
// whole once. Both are configurable via the [retrieval] policy.
const (
	defaultChunkSize    = 1000
	defaultChunkOverlap = 200
)

// Chunk splits text into overlapping chunks of ~size chars (overlap between
// adjacent chunks), preferring paragraph then sentence then word boundaries.
// size <= 0 / overlap < 0 fall back to the defaults. Deterministic — the same
// (text, size, overlap) always yields the same chunks, which keeps blob-keyed
// embedding caching sound.
func Chunk(text string, size, overlap int) []string {
	if size <= 0 {
		size = defaultChunkSize
	}
	if overlap < 0 || overlap >= size {
		overlap = min(defaultChunkOverlap, size/2)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) <= size {
		return []string{text}
	}
	var out []string
	for start := 0; start < len(text); {
		end := start + size
		if end >= len(text) {
			out = append(out, strings.TrimSpace(text[start:]))
			break
		}
		cut := boundary(text, start, end, size)
		piece := strings.TrimSpace(text[start:cut])
		if piece != "" {
			out = append(out, piece)
		}
		next := cut - overlap
		if next <= start {
			next = cut
		}
		start = next
	}
	return out
}

// boundary finds a natural cut point at or before end: paragraph break,
// then sentence end, then whitespace, else the hard limit.
func boundary(text string, start, end, size int) int {
	window := text[start:end]
	for _, sep := range []string{"\n\n", ". ", "\n", " "} {
		if i := strings.LastIndex(window, sep); i > size/2 {
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
