package vector

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/vllnt/dig/internal/index"
	"github.com/vllnt/dig/internal/store"
	"github.com/vllnt/dig/internal/vector/vectortest"
)

// fakeEndpoint starts the shared fake embeddings endpoint (vectortest): a
// real HTTP server at the one boundary a double is allowed, with bag-of-words
// semantics so ranking behavior under test is real, not scripted.
func fakeEndpoint(t *testing.T) *vectortest.Server {
	t.Helper()
	srv := vectortest.New()
	t.Cleanup(srv.Close)
	return srv
}

func testClient(url string) *Client {
	return NewClient(url+"/v1", "fake-model", "", "doc: ", "query: ")
}

// --- chunking ---

func TestChunkDeterministicAndBounded(t *testing.T) {
	text := strings.Repeat("Sentence one is here. Sentence two follows on.\n\n", 200)
	a := Chunk(text)
	b := Chunk(text)
	if len(a) == 0 || len(a) != len(b) {
		t.Fatalf("chunking not deterministic: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("chunk %d differs between runs", i)
		}
		if len(a[i]) > chunkSize {
			t.Fatalf("chunk %d exceeds size: %d", i, len(a[i]))
		}
		if a[i] == "" {
			t.Fatalf("chunk %d empty", i)
		}
	}
}

func TestChunkEdgeCases(t *testing.T) {
	if got := Chunk(""); got != nil {
		t.Fatalf("empty text should yield no chunks, got %v", got)
	}
	if got := Chunk("   \n\t  "); got != nil {
		t.Fatalf("whitespace should yield no chunks, got %v", got)
	}
	small := "tiny note"
	if got := Chunk(small); len(got) != 1 || got[0] != small {
		t.Fatalf("small text should be one chunk, got %v", got)
	}
	// Pathological: no separators at all — must still terminate and cover.
	solid := strings.Repeat("x", 5*chunkSize)
	chunks := Chunk(solid)
	if len(chunks) < 4 {
		t.Fatalf("solid text should split, got %d chunks", len(chunks))
	}
}

func TestChunkOverlapPreservesContinuity(t *testing.T) {
	// A sentence near a boundary must appear whole in at least one chunk.
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&sb, "Filler sentence number %d pads the text. ", i)
	}
	needle := "The critical fact straddles a boundary."
	text := sb.String() + needle + " " + sb.String()
	found := false
	for _, c := range Chunk(text) {
		if strings.Contains(c, needle) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("fact lost at chunk boundary")
	}
}

// --- vector math + serialization ---

func TestEncodeDecodeRoundtrip(t *testing.T) {
	v := []float32{0.1, -2.5, 3.75, 0, float32(math.Pi)}
	got, err := decodeVec(encodeVec(v))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(v) {
		t.Fatalf("length: %d vs %d", len(got), len(v))
	}
	for i := range v {
		if got[i] != v[i] {
			t.Fatalf("index %d: %v vs %v", i, got[i], v[i])
		}
	}
	if _, err := decodeVec([]byte{1, 2, 3}); err == nil {
		t.Fatal("corrupt vector must error")
	}
}

func TestNormalizeMakesUnitVectors(t *testing.T) {
	v := normalize([]float32{3, 4})
	if math.Abs(float64(dot(v, v))-1) > 1e-6 {
		t.Fatalf("not unit length: %v", v)
	}
	zero := normalize([]float32{0, 0})
	if zero[0] != 0 || zero[1] != 0 {
		t.Fatal("zero vector must stay zero, not NaN")
	}
}

// --- client against the fake endpoint ---

func TestClientBatchingAndPrefixes(t *testing.T) {
	srv := fakeEndpoint(t)
	c := testClient(srv.URL)

	texts := make([]string, maxBatch+3) // forces 2 requests
	for i := range texts {
		texts[i] = fmt.Sprintf("text number %d", i)
	}
	vecs, err := c.EmbedDocs(texts)
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("got %d vectors for %d texts", len(vecs), len(texts))
	}
	if srv.Requests.Load() != 2 {
		t.Fatalf("expected 2 batched requests, got %d", srv.Requests.Load())
	}
	// Doc and query prefixes place the same text at different points in space.
	q, err := c.EmbedQuery("text number 0")
	if err != nil {
		t.Fatal(err)
	}
	if dot(q, vecs[0]) >= 1-1e-6 {
		t.Fatal("doc and query prefixes should differentiate embeddings")
	}
}

func TestClientErrorPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not loaded", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	c := testClient(srv.URL)
	if _, err := c.EmbedQuery("q"); err == nil || !strings.Contains(err.Error(), "model not loaded") {
		t.Fatalf("endpoint error must propagate with body, got %v", err)
	}
	down := testClient("http://127.0.0.1:1") // nothing listens
	if _, err := down.EmbedQuery("q"); err == nil {
		t.Fatal("unreachable endpoint must error")
	}
}

// --- index: rebuild, cache, fingerprint, query ---

func makeManifest(entries ...store.Entry) *store.Manifest {
	return &store.Manifest{ID: "M1", Entries: entries}
}

func contentMap(m map[string]string) index.ContentFunc {
	return func(blob string) ([]byte, bool) {
		s, ok := m[blob]
		if !ok {
			return nil, false
		}
		return []byte(s), true
	}
}

func TestIndexRebuildQueryAndCache(t *testing.T) {
	srv := fakeEndpoint(t)
	c := testClient(srv.URL)

	dir := t.TempDir()
	x, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = x.Close() }()

	content := contentMap(map[string]string{
		"b1": "the annual financial report covers revenue and profit margins",
		"b2": "holiday photos from the beach trip with family",
		"b3": "\x00\xffbinary", // ContentFunc returns ok=false for unknown blobs only; b3 included
	})
	m := makeManifest(
		store.Entry{Path: "docs/report.md", Blob: "b1", Labels: []string{"finance"}},
		store.Entry{Path: "media/photos.txt", Blob: "b2"},
		store.Entry{Path: "bin/raw.bin", Blob: "unknown"}, // skipped: not text
	)
	if err := x.Rebuild(m, content, c); err != nil {
		t.Fatal(err)
	}
	firstEmbeds := srv.Embedded.Load()
	if firstEmbeds == 0 {
		t.Fatal("nothing embedded")
	}

	q, err := c.EmbedQuery("financial revenue report")
	if err != nil {
		t.Fatal(err)
	}
	// Query embedding counts too — baseline AFTER it for cache assertions.
	firstEmbeds = srv.Embedded.Load()
	res, err := x.Query(q, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 || res[0].Path != "docs/report.md" {
		t.Fatalf("expected report first, got %+v", res)
	}
	if len(res[0].Labels) != 1 || res[0].Labels[0] != "finance" {
		t.Fatalf("labels lost: %+v", res[0])
	}

	// Rebuild with the same manifest: every blob cached — zero new embeds.
	if err := x.Rebuild(m, content, c); err != nil {
		t.Fatal(err)
	}
	if srv.Embedded.Load() != firstEmbeds {
		t.Fatalf("cache miss on identical rebuild: %d → %d embeds", firstEmbeds, srv.Embedded.Load())
	}

	// A moved file (same blob) also costs nothing — content addressing.
	moved := makeManifest(
		store.Entry{Path: "archive/2024/report.md", Blob: "b1", Labels: []string{"finance"}},
		store.Entry{Path: "media/photos.txt", Blob: "b2"},
	)
	if err := x.Rebuild(moved, content, c); err != nil {
		t.Fatal(err)
	}
	if srv.Embedded.Load() != firstEmbeds {
		t.Fatalf("move should not re-embed: %d → %d", firstEmbeds, srv.Embedded.Load())
	}
	res, err = x.Query(q, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 || res[0].Path != "archive/2024/report.md" {
		t.Fatalf("query after move: %+v", res)
	}
	for _, r := range res {
		if r.Path == "docs/report.md" {
			t.Fatal("stale path survived rebuild")
		}
	}
}

func TestIndexFingerprintInvalidation(t *testing.T) {

	srv := fakeEndpoint(t)

	dir := t.TempDir()
	x, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = x.Close() }()

	content := contentMap(map[string]string{"b1": "some text"})
	m := makeManifest(store.Entry{Path: "a.md", Blob: "b1"})

	c1 := NewClient(srv.URL+"/v1", "model-one", "", "", "")
	if err := x.Rebuild(m, content, c1); err != nil {
		t.Fatal(err)
	}
	n := srv.Embedded.Load()

	// Same model: cache holds.
	if err := x.Rebuild(m, content, c1); err != nil {
		t.Fatal(err)
	}
	if srv.Embedded.Load() != n {
		t.Fatal("same-model rebuild should hit cache")
	}

	// Model change: embedding space changed — cache must be dropped.
	c2 := NewClient(srv.URL+"/v1", "model-two", "", "", "")
	if err := x.Rebuild(m, content, c2); err != nil {
		t.Fatal(err)
	}
	if srv.Embedded.Load() == n {
		t.Fatal("model change must invalidate the cache")
	}
}

func TestQueryDimensionMismatch(t *testing.T) {

	srv := fakeEndpoint(t)
	c := testClient(srv.URL)

	dir := t.TempDir()
	x, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = x.Close() }()
	m := makeManifest(store.Entry{Path: "a.md", Blob: "b1"})
	if err := x.Rebuild(m, contentMap(map[string]string{"b1": "text"}), c); err != nil {
		t.Fatal(err)
	}
	if _, err := x.Query(make([]float32, 7), 5); err == nil {
		t.Fatal("dimension mismatch must error, not mis-rank")
	}
}

func TestSyncDocsAndDrainResumable(t *testing.T) {
	srv := fakeEndpoint(t)
	c := testClient(srv.URL)

	dir := t.TempDir()
	x, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	content := map[string]string{}
	entries := make([]store.Entry, 0, 10)
	for i := 0; i < 10; i++ {
		blob := fmt.Sprintf("b%d", i)
		content[blob] = fmt.Sprintf("document number %d about topic %d", i, i)
		entries = append(entries, store.Entry{Path: fmt.Sprintf("f%d.md", i), Blob: blob})
	}
	m := makeManifest(entries...)

	// Sync is instant and embeds nothing — it only queues.
	pending, err := x.SyncDocs(m, c)
	if err != nil {
		t.Fatal(err)
	}
	if pending != 10 {
		t.Fatalf("want 10 pending after sync, got %d", pending)
	}
	if srv.Embedded.Load() != 0 {
		t.Fatal("SyncDocs must not embed")
	}

	// Drain in budgeted steps.
	done, remaining, err := x.DrainPending(contentMap(content), c, 3)
	if err != nil || done != 3 || remaining != 7 {
		t.Fatalf("first drain: done=%d remaining=%d err=%v", done, remaining, err)
	}

	// Interrupt: close and reopen — committed work must survive.
	_ = x.Close()
	x, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = x.Close() }()

	embedsBefore := srv.Embedded.Load()
	done, remaining, err = x.DrainPending(contentMap(content), c, 0) // drain all
	if err != nil || done != 7 || remaining != 0 {
		t.Fatalf("resume drain: done=%d remaining=%d err=%v", done, remaining, err)
	}
	// Exactly the 7 outstanding blobs were embedded — nothing re-done.
	if srv.Embedded.Load() != embedsBefore+7 {
		t.Fatalf("resume re-embedded: %d → %d", embedsBefore, srv.Embedded.Load())
	}

	// Fully drained index answers queries.
	q, err := c.EmbedQuery("document number 4 about topic 4")
	if err != nil {
		t.Fatal(err)
	}
	res, err := x.Query(q, 1)
	if err != nil || len(res) == 0 || res[0].Path != "f4.md" {
		t.Fatalf("query after drain: %+v err=%v", res, err)
	}

	// A manifest that drops files clears their dead pending work.
	smaller := makeManifest(entries[:2]...)
	if _, err := x.SyncDocs(smaller, c); err != nil {
		t.Fatal(err)
	}
	if n, _ := x.PendingCount(); n != 0 {
		t.Fatalf("dead pending rows survived: %d", n)
	}
}

func TestDrainPendingEndpointDownKeepsQueue(t *testing.T) {
	srv := fakeEndpoint(t)
	c := testClient(srv.URL)
	dir := t.TempDir()
	x, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = x.Close() }()

	m := makeManifest(store.Entry{Path: "a.md", Blob: "b1"})
	if _, err := x.SyncDocs(m, c); err != nil {
		t.Fatal(err)
	}
	srv.Close() // endpoint dies before the drain
	done, remaining, err := x.DrainPending(contentMap(map[string]string{"b1": "text"}), c, 0)
	if err == nil {
		t.Fatal("drain against a dead endpoint must error")
	}
	if done != 0 || remaining != 1 {
		t.Fatalf("queue must survive the failure: done=%d remaining=%d", done, remaining)
	}
}

// --- live integration (gated): true paraphrase recall with zero shared terms.
// Run with DIG_EMBED_URL=http://127.0.0.1:8092/v1 DIG_EMBED_MODEL=... against
// a real local endpoint; skipped otherwise so CI stays hermetic.

func TestLiveParaphraseRecall(t *testing.T) {
	url := os.Getenv("DIG_EMBED_URL")
	model := os.Getenv("DIG_EMBED_MODEL")
	if url == "" || model == "" {
		t.Skip("set DIG_EMBED_URL and DIG_EMBED_MODEL to run against a live endpoint")
	}
	c := NewClient(url, model, "", "search_document: ", "search_query: ")

	dir := t.TempDir()
	x, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = x.Close() }()

	content := contentMap(map[string]string{
		"car":    "Changed the oil in the Honda today, rotated the tires, replaced the cabin air filter.",
		"meet":   "Quarterly planning session: prioritize the billing migration, defer the dashboard redesign.",
		"recipe": "Slow-cooked the lamb shoulder for six hours with rosemary and garlic.",
	})
	m := makeManifest(
		store.Entry{Path: "notes/car.md", Blob: "car"},
		store.Entry{Path: "notes/meeting.md", Blob: "meet"},
		store.Entry{Path: "notes/recipe.md", Blob: "recipe"},
	)
	if err := x.Rebuild(m, content, c); err != nil {
		t.Fatal(err)
	}

	// Zero shared terms with the target document — FTS cannot answer these.
	for query, want := range map[string]string{
		"vehicle upkeep":                    "notes/car.md",
		"what did we decide about invoices": "notes/meeting.md",
		"dinner preparation":                "notes/recipe.md",
	} {
		qv, err := c.EmbedQuery(query)
		if err != nil {
			t.Fatal(err)
		}
		res, err := x.Query(qv, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(res) == 0 || res[0].Path != want {
			t.Errorf("query %q: want %s first, got %+v", query, want, res)
		}
	}
}
