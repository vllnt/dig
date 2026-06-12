// Package retrieval fuses the deterministic FTS index with the opt-in vector
// index into one ranked answer. FTS remains the default; vector and hybrid
// modes activate only through a [retrieval] policy or an explicit flag —
// per docs/architecture.md §5 the AI layer is optional, local, off by default.
package retrieval

import (
	"fmt"
	"sort"

	"github.com/vllnt/dig/internal/index"
	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/vector"
)

// Mode selects the retrieval strategy for a query.
type Mode string

const (
	ModeFTS    Mode = "fts"    // deterministic full-text search (default)
	ModeVector Mode = "vector" // semantic similarity only
	ModeHybrid Mode = "hybrid" // FTS ∪ vector fused with RRF
)

// ParseMode validates a mode string ("" means FTS).
func ParseMode(s string) (Mode, error) {
	switch Mode(s) {
	case "", ModeFTS:
		return ModeFTS, nil
	case ModeVector:
		return ModeVector, nil
	case ModeHybrid:
		return ModeHybrid, nil
	}
	return "", fmt.Errorf("unknown retrieval mode %q (fts, vector, hybrid)", s)
}

// rrfK is the standard Reciprocal Rank Fusion constant: dampens the gap
// between adjacent top ranks so neither ranker dominates on rank-1 alone.
const rrfK = 60

// candidateFactor widens each ranker's candidate pool beyond the requested
// limit so fusion has real overlap to work with.
const candidateFactor = 4

// Search runs a query in the given mode against a KB's .dig directory.
// Vector and hybrid modes require a configured [retrieval] policy with a
// reachable embedding endpoint.
func Search(digDir string, rp policy.RetrievalPolicy, mode Mode, q string, limit int) ([]vector.Result, error) {
	if limit <= 0 {
		limit = 20
	}
	if mode == ModeFTS {
		return ftsOnly(digDir, q, limit)
	}
	if rp.BaseURL == "" || rp.Model == "" {
		return nil, fmt.Errorf("retrieval mode %s needs [retrieval] base_url and model in policy.toml", mode)
	}
	client := vector.NewClient(rp.BaseURL, rp.Model, rp.APIKeyEnv, rp.DocPrefix, rp.QueryPrefix)
	qvec, err := client.EmbedQuery(q)
	if err != nil {
		return nil, err
	}
	vx, err := vector.Open(digDir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = vx.Close() }()

	pool := limit * candidateFactor
	vres, err := vx.Query(qvec, pool)
	if err != nil {
		return nil, err
	}
	if mode == ModeVector {
		if len(vres) > limit {
			vres = vres[:limit]
		}
		return vres, nil
	}

	fres, err := ftsOnly(digDir, q, pool)
	if err != nil {
		return nil, err
	}
	return Fuse(fres, vres, limit), nil
}

// ftsOnly runs the deterministic FTS path and adapts results.
func ftsOnly(digDir string, q string, limit int) ([]vector.Result, error) {
	idx, err := index.Open(digDir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = idx.Close() }()
	res, err := idx.Query(q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]vector.Result, len(res))
	for i, r := range res {
		out[i] = vector.Result{Path: r.Path, Blob: r.Blob, Labels: r.Labels}
	}
	return out, nil
}

// Fuse merges two rankings with Reciprocal Rank Fusion:
// score(d) = Σ over rankings of 1/(rrfK + rank(d)).
func Fuse(fts, vec []vector.Result, limit int) []vector.Result {
	type fused struct {
		r     vector.Result
		score float64
	}
	byPath := map[string]*fused{}
	add := func(list []vector.Result) {
		for i, r := range list {
			f, ok := byPath[r.Path]
			if !ok {
				f = &fused{r: r}
				byPath[r.Path] = f
			}
			f.score += 1.0 / float64(rrfK+i+1)
		}
	}
	add(fts)
	add(vec)

	out := make([]fused, 0, len(byPath))
	for _, f := range byPath {
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].r.Path < out[j].r.Path
	})
	if len(out) > limit {
		out = out[:limit]
	}
	res := make([]vector.Result, len(out))
	for i, f := range out {
		res[i] = f.r
		res[i].Score = float32(f.score)
	}
	return res
}
