// Command eval runs dig's retrieval against memory benchmarks (LongMemEval,
// LoCoMo): it builds a KB from the benchmark corpus, scores FTS / vector /
// hybrid retrieval with the standard IR metrics, and prints a scoreboard.
// Development tooling — not part of the dig product surface.
//
//	go run ./tools/eval --bench locomo --data locomo10.json \
//	  --workdir /tmp/kb-locomo --base-url http://127.0.0.1:8092/v1 \
//	  --model nomic-embed-text-v1.5 \
//	  --doc-prefix "search_document: " --query-prefix "search_query: "
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vllnt/dig/internal/index"
	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/retrieval"
	"github.com/vllnt/dig/internal/scan"
	"github.com/vllnt/dig/internal/store"
	"github.com/vllnt/dig/internal/vector"
)

// Query is one benchmark question mapped onto the KB.
type Query struct {
	ID    string
	Text  string
	Gold  map[string]bool // KB paths holding the evidence
	Scope map[string]bool // KB paths eligible for this question (nil = all)
	Type  string
}

// Corpus is the benchmark mapped onto KB files.
type Corpus struct {
	Files   map[string]string // KB-relative path → content
	Queries []Query
}

var ks = []int{1, 3, 5, 10}

func main() {
	bench := flag.String("bench", "", "benchmark: locomo | longmemeval")
	data := flag.String("data", "", "path to the benchmark JSON")
	workdir := flag.String("workdir", "", "KB directory (reused across runs — embeddings cache)")
	mode := flag.String("mode", "all", "fts | vector | hybrid | all")
	baseURL := flag.String("base-url", "http://127.0.0.1:8092/v1", "embedding endpoint")
	model := flag.String("model", "nomic-embed-text-v1.5", "embedding model")
	docPrefix := flag.String("doc-prefix", "search_document: ", "document prefix")
	queryPrefix := flag.String("query-prefix", "search_query: ", "query prefix")
	maxQ := flag.Int("max-questions", 0, "cap question count (0 = all)")
	stride := flag.Int("stride", 1, "keep every k-th question (stratified subset; question types stay mixed)")
	asJSON := flag.Bool("json", false, "emit JSON")
	flag.Parse()

	if *bench == "" || *data == "" || *workdir == "" {
		flag.Usage()
		os.Exit(2)
	}

	var corpus *Corpus
	var err error
	switch *bench {
	case "locomo":
		corpus, err = loadLoCoMo(*data)
	case "longmemeval":
		corpus, err = loadLongMemEval(*data)
	case "beam":
		corpus, err = loadBEAM(*data)
	default:
		log.Fatalf("unknown bench %q", *bench)
	}
	if err != nil {
		log.Fatal(err)
	}
	if *stride > 1 {
		kept := make([]Query, 0, len(corpus.Queries)/(*stride)+1)
		for i := 0; i < len(corpus.Queries); i += *stride {
			kept = append(kept, corpus.Queries[i])
		}
		corpus.Queries = kept
	}
	if *maxQ > 0 && *maxQ < len(corpus.Queries) {
		corpus.Queries = corpus.Queries[:*maxQ]
	}
	if *stride > 1 || *maxQ > 0 {
		restrictFiles(corpus)
	}
	log.Printf("%s: %d files, %d questions", *bench, len(corpus.Files), len(corpus.Queries))

	rp := policy.RetrievalPolicy{
		Mode: "hybrid", BaseURL: *baseURL, Model: *model,
		DocPrefix: *docPrefix, QueryPrefix: *queryPrefix,
	}
	digDir, err := buildKB(*workdir, corpus, rp)
	if err != nil {
		log.Fatal(err)
	}

	modes := []string{*mode}
	if *mode == "all" {
		modes = []string{"fts", "vector", "hybrid"}
	}
	client := vector.NewClient(rp.BaseURL, rp.Model, "", rp.DocPrefix, rp.QueryPrefix)
	// Embed queries once — every semantic mode shares them.
	var qvecs [][]float32
	for _, m := range modes {
		if m == "vector" || m == "hybrid" {
			texts := make([]string, len(corpus.Queries))
			for i, q := range corpus.Queries {
				texts[i] = q.Text
			}
			var err error
			if qvecs, err = client.EmbedQueries(texts); err != nil {
				log.Fatal(err)
			}
			break
		}
	}
	results := map[string]map[string]float64{}
	for _, m := range modes {
		scores, err := evaluate(digDir, corpus, m, qvecs)
		if err != nil {
			log.Fatal(err)
		}
		results[m] = scores
	}

	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"bench": *bench, "model": *model, "questions": len(corpus.Queries),
			"files": len(corpus.Files), "scores": results,
		})
		return
	}
	printTable(*bench, results)
}

// restrictFiles drops corpus files outside every kept question's scope, so a
// --max-questions iteration only embeds what it scores. Scoring is unchanged
// — each question is judged inside its own haystack either way — and the
// blob-keyed cache means a later full run reuses every embedding made here.
func restrictFiles(c *Corpus) {
	keep := map[string]bool{}
	for _, q := range c.Queries {
		if q.Scope == nil {
			return // an unscoped question needs the whole corpus
		}
		for p := range q.Scope {
			keep[p] = true
		}
	}
	for p := range c.Files {
		if !keep[p] {
			delete(c.Files, p)
		}
	}
}

// buildKB materializes the corpus as a dig KB and indexes it (FTS + vectors).
// Reused across runs: same files → same blobs → embedding cache hits.
func buildKB(root string, corpus *Corpus, rp policy.RetrievalPolicy) (string, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	for path, content := range corpus.Files {
		p := filepath.Join(root, filepath.FromSlash(path))
		if st, err := os.Stat(p); err == nil && st.Size() == int64(len(content)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	k, err := kb.Resolve(root)
	if err != nil {
		k, err = kb.Init(root)
		if err != nil {
			return "", err
		}
	}
	dig, err := k.EnsureDig()
	if err != nil {
		return "", err
	}
	st, err := store.Open(dig)
	if err != nil {
		return "", err
	}
	defer func() { _ = st.Close() }()
	entries, err := scan.Walk(k, st, false)
	if err != nil {
		return "", err
	}
	m, err := st.Commit("eval", store.KindObserve, entries)
	if err != nil {
		return "", err
	}
	idx, err := index.Open(dig)
	if err != nil {
		return "", err
	}
	defer func() { _ = idx.Close() }()
	if err := idx.Rebuild(m, index.BlobContent(st.Blobs())); err != nil {
		return "", err
	}
	vx, err := vector.Open(dig)
	if err != nil {
		return "", err
	}
	defer func() { _ = vx.Close() }()
	client := vector.NewClient(rp.BaseURL, rp.Model, "", rp.DocPrefix, rp.QueryPrefix)
	start := time.Now()
	if _, err := vx.SyncDocs(m, client); err != nil {
		return "", fmt.Errorf("vector sync: %w", err)
	}
	// Budgeted drain with progress — interruptible and resumable (per-blob
	// commits): a killed run loses at most one blob of embedding work.
	totalDone := 0
	for {
		done, remaining, err := vx.DrainPending(index.BlobContent(st.Blobs()), client, 500)
		totalDone += done
		if err != nil {
			return "", fmt.Errorf("vector embed: %w", err)
		}
		if remaining == 0 {
			break
		}
		rate := float64(totalDone) / time.Since(start).Seconds()
		log.Printf("embedding: %d blob(s) remaining (%.0f blobs/min)", remaining, rate*60)
	}
	log.Printf("vector index ready in %s", time.Since(start).Round(time.Second))
	return dig, nil
}

// evaluate scores one retrieval mode over all questions. Each ranker returns
// its FULL ranking; per question that ranking is filtered to the question's
// scope (its haystack) BEFORE fusion and cutoffs — pool-limited global
// retrieval would starve scoped metrics on large corpora. qvecs are the
// pre-embedded queries (shared across modes; unused in fts mode).
func evaluate(digDir string, corpus *Corpus, mode string, qvecs [][]float32) (map[string]float64, error) {
	log.Printf("evaluating mode=%s ...", mode)
	start := time.Now()
	pool := len(corpus.Files)

	var ftsIdx *index.FTS
	var matrix *vector.Matrix
	var err error

	if mode == "fts" || mode == "hybrid" {
		ftsIdx, err = index.Open(digDir)
		if err != nil {
			return nil, err
		}
		defer func() { _ = ftsIdx.Close() }()
	}
	if mode == "vector" || mode == "hybrid" {
		vx, err := vector.Open(digDir)
		if err != nil {
			return nil, err
		}
		matrix, err = vx.Matrix()
		_ = vx.Close()
		if err != nil {
			return nil, err
		}
	}

	agg := newAggregate()
	for i, q := range corpus.Queries {
		var ranked []vector.Result
		switch mode {
		case "fts":
			ranked, err = ftsResults(ftsIdx, q.Text, pool)
			ranked = inScope(ranked, q.Scope)
		case "vector":
			ranked, err = matrix.Query(qvecs[i], pool)
			ranked = inScope(ranked, q.Scope)
		case "hybrid":
			var f, v []vector.Result
			if f, err = ftsResults(ftsIdx, q.Text, pool); err == nil {
				if v, err = matrix.Query(qvecs[i], pool); err == nil {
					ranked = retrieval.Fuse(inScope(f, q.Scope), inScope(v, q.Scope), pool)
				}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("question %s: %w", q.ID, err)
		}
		paths := make([]string, 0, len(ranked))
		for _, r := range ranked {
			paths = append(paths, r.Path)
		}
		agg.add(paths, q.Gold)
	}
	scores := agg.summary()
	log.Printf("mode=%s done in %s", mode, time.Since(start).Round(time.Millisecond))
	return scores, nil
}

// inScope filters a ranking to a question's haystack (nil scope = keep all).
func inScope(ranked []vector.Result, scope map[string]bool) []vector.Result {
	if scope == nil {
		return ranked
	}
	out := make([]vector.Result, 0, len(scope))
	for _, r := range ranked {
		if scope[r.Path] {
			out = append(out, r)
		}
	}
	return out
}

func ftsResults(idx *index.FTS, q string, limit int) ([]vector.Result, error) {
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

// aggregate accumulates IR metrics over questions.
type aggregate struct {
	n      int
	recall map[int]float64 // Recall@k: gold coverage in top-k
	hit    map[int]float64 // Hit@k: any gold in top-k
	ndcg10 float64
	mrr    float64
}

func newAggregate() *aggregate {
	return &aggregate{recall: map[int]float64{}, hit: map[int]float64{}}
}

func (a *aggregate) add(ranked []string, gold map[string]bool) {
	if len(gold) == 0 {
		return
	}
	a.n++
	for _, k := range ks {
		got := 0
		top := ranked
		if len(top) > k {
			top = top[:k]
		}
		for _, p := range top {
			if gold[p] {
				got++
			}
		}
		a.recall[k] += float64(got) / float64(len(gold))
		if got > 0 {
			a.hit[k]++
		}
	}
	// MRR over the full ranked list.
	for i, p := range ranked {
		if gold[p] {
			a.mrr += 1.0 / float64(i+1)
			break
		}
	}
	// NDCG@10 with binary relevance.
	var dcg, idcg float64
	for i := 0; i < 10 && i < len(ranked); i++ {
		if gold[ranked[i]] {
			dcg += 1 / math.Log2(float64(i)+2)
		}
	}
	ideal := len(gold)
	if ideal > 10 {
		ideal = 10
	}
	for i := 0; i < ideal; i++ {
		idcg += 1 / math.Log2(float64(i)+2)
	}
	if idcg > 0 {
		a.ndcg10 += dcg / idcg
	}
}

func (a *aggregate) summary() map[string]float64 {
	out := map[string]float64{"questions": float64(a.n)}
	if a.n == 0 {
		return out
	}
	for _, k := range ks {
		out[fmt.Sprintf("recall@%d", k)] = a.recall[k] / float64(a.n)
		out[fmt.Sprintf("hit@%d", k)] = a.hit[k] / float64(a.n)
	}
	out["ndcg@10"] = a.ndcg10 / float64(a.n)
	out["mrr"] = a.mrr / float64(a.n)
	return out
}

func printTable(bench string, results map[string]map[string]float64) {
	modes := make([]string, 0, len(results))
	for m := range results {
		modes = append(modes, m)
	}
	sort.Strings(modes)
	metrics := []string{"recall@1", "recall@3", "recall@5", "recall@10", "hit@1", "hit@5", "hit@10", "ndcg@10", "mrr"}
	fmt.Printf("\n%s — dig retrieval scoreboard\n", bench)
	fmt.Printf("%-10s", "metric")
	for _, m := range modes {
		fmt.Printf("%10s", m)
	}
	fmt.Println()
	for _, met := range metrics {
		fmt.Printf("%-10s", met)
		for _, m := range modes {
			fmt.Printf("%9.1f%%", results[m][met]*100)
		}
		fmt.Println()
	}
}

// --- LoCoMo adapter ---
// locomo10.json: samples with multi-session two-speaker conversations and QA
// whose evidence cites dialog ids "D<session>:<turn>". One KB file per
// session; gold = sessions holding evidence; scope = the sample's sessions.
// Category 5 (adversarial, unanswerable) is excluded from retrieval scoring.

func loadLoCoMo(file string) (*Corpus, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var samples []struct {
		SampleID     string            `json:"sample_id"`
		Conversation map[string]any    `json:"conversation"`
		QA           []json.RawMessage `json:"qa"`
	}
	if err := json.Unmarshal(raw, &samples); err != nil {
		return nil, err
	}
	c := &Corpus{Files: map[string]string{}}
	for _, s := range samples {
		scope := map[string]bool{}
		sessionPath := map[string]string{} // "D1" → file path
		for key, val := range s.Conversation {
			if !strings.HasPrefix(key, "session_") || strings.HasSuffix(key, "_date_time") {
				continue
			}
			turns, ok := val.([]any)
			if !ok {
				continue
			}
			num := strings.TrimPrefix(key, "session_")
			date, _ := s.Conversation[key+"_date_time"].(string)
			var sb strings.Builder
			if date != "" {
				fmt.Fprintf(&sb, "DATE: %s\n", date)
			}
			for _, t := range turns {
				turn, ok := t.(map[string]any)
				if !ok {
					continue
				}
				speaker, _ := turn["speaker"].(string)
				text, _ := turn["text"].(string)
				fmt.Fprintf(&sb, "%s: %s\n", speaker, text)
			}
			path := fmt.Sprintf("%s/session_%s.txt", s.SampleID, num)
			c.Files[path] = sb.String()
			scope[path] = true
			sessionPath["D"+num] = path
		}
		for qi, qraw := range s.QA {
			var q struct {
				Question string   `json:"question"`
				Evidence []string `json:"evidence"`
				Category any      `json:"category"`
			}
			if err := json.Unmarshal(qraw, &q); err != nil {
				return nil, fmt.Errorf("%s qa[%d]: %w", s.SampleID, qi, err)
			}
			if fmt.Sprint(q.Category) == "5" || len(q.Evidence) == 0 {
				continue
			}
			gold := map[string]bool{}
			for _, ev := range q.Evidence {
				sess, _, ok := strings.Cut(ev, ":")
				if !ok {
					continue
				}
				if p, ok := sessionPath[sess]; ok {
					gold[p] = true
				}
			}
			if len(gold) == 0 {
				continue
			}
			c.Queries = append(c.Queries, Query{
				ID:    fmt.Sprintf("%s-%d", s.SampleID, qi),
				Text:  q.Question,
				Gold:  gold,
				Scope: scope,
				Type:  fmt.Sprint(q.Category),
			})
		}
	}
	return c, nil
}

// --- BEAM adapter ---
// Input is the JSON emitted by beam_convert.py from a BEAM parquet split:
// conversations of turns (global turn ids) plus probing questions whose
// source_chat_ids name the evidence turns. One KB file per turn; each
// question is scoped to its own conversation; gold = its evidence turns.
// Abstention questions are excluded upstream (no retrievable evidence).

func loadBEAM(file string) (*Corpus, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var convs []struct {
		ConversationID string `json:"conversation_id"`
		Turns          []struct {
			ID      int    `json:"id"`
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"turns"`
		Questions []struct {
			QID         string `json:"qid"`
			Type        string `json:"type"`
			Question    string `json:"question"`
			EvidenceIDs []int  `json:"evidence_ids"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(raw, &convs); err != nil {
		return nil, err
	}
	c := &Corpus{Files: map[string]string{}}
	for _, conv := range convs {
		scope := map[string]bool{}
		pathFor := func(turn int) string {
			return fmt.Sprintf("c%s/t%05d.txt", conv.ConversationID, turn)
		}
		for _, t := range conv.Turns {
			p := pathFor(t.ID)
			c.Files[p] = t.Role + ": " + t.Content
			scope[p] = true
		}
		for _, q := range conv.Questions {
			gold := map[string]bool{}
			for _, id := range q.EvidenceIDs {
				if p := pathFor(id); scope[p] {
					gold[p] = true
				}
			}
			if len(gold) == 0 {
				continue
			}
			c.Queries = append(c.Queries, Query{
				ID: q.QID, Text: q.Question, Gold: gold, Scope: scope, Type: q.Type,
			})
		}
	}
	return c, nil
}

// --- LongMemEval adapter ---
// longmemeval_s: 500 questions, each with ~50 haystack chat sessions and the
// evidence sessions named in answer_session_ids. Sessions are shared across
// questions — the KB holds each unique session once; scope restricts scoring
// to the question's haystack. Abstention questions (no evidence) are skipped.

func loadLongMemEval(file string) (*Corpus, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var questions []struct {
		QuestionID       string   `json:"question_id"`
		QuestionType     string   `json:"question_type"`
		Question         string   `json:"question"`
		QuestionDate     string   `json:"question_date"`
		HaystackIDs      []string `json:"haystack_session_ids"`
		HaystackDates    []string `json:"haystack_dates"`
		HaystackSessions [][]struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"haystack_sessions"`
		AnswerSessionIDs []string `json:"answer_session_ids"`
	}
	if err := json.Unmarshal(raw, &questions); err != nil {
		return nil, err
	}
	c := &Corpus{Files: map[string]string{}}
	pathFor := func(sid string) string { return "sessions/" + sid + ".txt" }
	for _, q := range questions {
		scope := map[string]bool{}
		for i, sid := range q.HaystackIDs {
			p := pathFor(sid)
			scope[p] = true
			if _, seen := c.Files[p]; seen || i >= len(q.HaystackSessions) {
				continue
			}
			var sb strings.Builder
			if i < len(q.HaystackDates) {
				fmt.Fprintf(&sb, "DATE: %s\n", q.HaystackDates[i])
			}
			for _, turn := range q.HaystackSessions[i] {
				fmt.Fprintf(&sb, "%s: %s\n", turn.Role, turn.Content)
			}
			c.Files[p] = sb.String()
		}
		gold := map[string]bool{}
		for _, sid := range q.AnswerSessionIDs {
			if p := pathFor(sid); scope[p] {
				gold[p] = true
			}
		}
		if len(gold) == 0 {
			continue // abstention questions have no retrievable evidence
		}
		c.Queries = append(c.Queries, Query{
			ID: q.QuestionID, Text: q.Question, Gold: gold, Scope: scope,
			Type: q.QuestionType,
		})
	}
	return c, nil
}
