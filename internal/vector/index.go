package vector

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/vllnt/dig/internal/index"
	"github.com/vllnt/dig/internal/store"
)

// Result is one semantic search hit, best first.
type Result struct {
	Path   string
	Blob   string
	Labels []string
	Score  float32
}

// Index is the local vector store: blob-keyed chunk embeddings (immutable
// cache — content-addressing means a blob's embedding never changes) plus a
// docs view rebuilt from the current manifest. Like the FTS index it is a
// derived view, never a source of truth.
type Index struct {
	db *sql.DB
}

const vectorFile = "vectors.db"

// Open opens (creating if needed) the vector index in a KB's .dig directory.
func Open(digDir string) (*Index, error) {
	db, err := sql.Open("sqlite", filepath.Join(digDir, vectorFile))
	if err != nil {
		return nil, fmt.Errorf("open vector index: %w", err)
	}
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS meta(k TEXT PRIMARY KEY, v TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS cache_blobs(blob TEXT PRIMARY KEY, chunks INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS cache(
			blob TEXT NOT NULL, seq INTEGER NOT NULL, vec BLOB NOT NULL,
			PRIMARY KEY(blob, seq)
		)`,
		`CREATE TABLE IF NOT EXISTS docs(path TEXT PRIMARY KEY, blob TEXT NOT NULL, labels TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS pending(blob TEXT PRIMARY KEY)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("create vector schema: %w", err)
		}
	}
	return &Index{db: db}, nil
}

// Close releases the database.
func (x *Index) Close() error { return x.db.Close() }

// fingerprint identifies the embedding configuration the cache was built
// with. Any change (model, prefixes, chunking) invalidates every cached
// vector — silently mixing embedding spaces would corrupt ranking.
func fingerprint(c *Client) string {
	return fmt.Sprintf("%s|%s|%s|%d|%d", c.Model, c.DocPrefix, c.QueryPrefix, c.ChunkSize, c.ChunkOverlap)
}

// ensureFingerprint drops the cache when the embedding config changed.
func (x *Index) ensureFingerprint(c *Client) error {
	var cur string
	err := x.db.QueryRow(`SELECT v FROM meta WHERE k='fingerprint'`).Scan(&cur)
	switch {
	case err == sql.ErrNoRows:
	case err != nil:
		return err
	case cur == fingerprint(c):
		return nil
	default:
		for _, stmt := range []string{`DELETE FROM cache`, `DELETE FROM cache_blobs`, `DELETE FROM docs`, `DELETE FROM pending`} {
			if _, err := x.db.Exec(stmt); err != nil {
				return err
			}
		}
	}
	_, err = x.db.Exec(`INSERT OR REPLACE INTO meta(k,v) VALUES('fingerprint',?)`, fingerprint(c))
	return err
}

// SyncDocs refreshes the docs view from m and enqueues blobs the cache has
// never seen into the pending queue. No network, no embedding — it returns
// instantly however large the manifest is. The embedding work happens in
// DrainPending, on whatever schedule the caller chooses (inline for small
// backlogs, `dig embed` / `dig watch` in the background for large ones).
// It returns the number of blobs pending after the sync.
func (x *Index) SyncDocs(m *store.Manifest, c *Client) (int, error) {
	if err := x.ensureFingerprint(c); err != nil {
		return 0, err
	}
	tx, err := x.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.Exec(`DELETE FROM docs`); err != nil {
		return 0, err
	}
	if m != nil {
		for _, e := range m.Entries {
			if _, err := tx.Exec(`INSERT OR REPLACE INTO docs(path,blob,labels) VALUES(?,?,?)`,
				e.Path, e.Blob, strings.Join(e.Labels, " ")); err != nil {
				return 0, err
			}
			if _, err := tx.Exec(`INSERT OR IGNORE INTO pending(blob)
				SELECT ? WHERE NOT EXISTS (SELECT 1 FROM cache_blobs WHERE blob = ?)`,
				e.Blob, e.Blob); err != nil {
				return 0, err
			}
		}
	}
	// Pending blobs no longer referenced by any doc are dead work — drop them.
	if _, err := tx.Exec(`DELETE FROM pending WHERE blob NOT IN (SELECT blob FROM docs)`); err != nil {
		return 0, err
	}
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM pending`).Scan(&n); err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

// DrainPending embeds up to maxBlobs queued blobs (maxBlobs <= 0 drains all).
// Each blob commits in its own transaction, so the drain is interruptible and
// resumable — a killed run loses at most one blob of work, never hours.
// Returns blobs processed and blobs still pending.
func (x *Index) DrainPending(content index.ContentFunc, c *Client, maxBlobs int) (int, int, error) {
	if err := x.ensureFingerprint(c); err != nil {
		return 0, 0, err
	}
	done := 0
	for maxBlobs <= 0 || done < maxBlobs {
		var blob string
		err := x.db.QueryRow(`SELECT blob FROM pending LIMIT 1`).Scan(&blob)
		if err == sql.ErrNoRows {
			break
		}
		if err != nil {
			return done, -1, err
		}
		if err := x.embedBlob(blob, content, c); err != nil {
			remaining, _ := x.PendingCount()
			return done, remaining, err
		}
		done++
	}
	remaining, err := x.PendingCount()
	return done, remaining, err
}

// embedBlob chunks, embeds, and commits one blob's vectors atomically.
func (x *Index) embedBlob(blob string, content index.ContentFunc, c *Client) error {
	var chunks []string
	if content != nil {
		if text, ok := content(blob); ok {
			chunks = Chunk(string(text), c.ChunkSize, c.ChunkOverlap)
		}
	}
	var vecs [][]float32
	if len(chunks) > 0 {
		var err error
		vecs, err = c.EmbedDocs(chunks)
		if err != nil {
			return fmt.Errorf("embed %s: %w", blob, err)
		}
	}
	tx, err := x.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	for seq, v := range vecs {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO cache(blob,seq,vec) VALUES(?,?,?)`,
			blob, seq, encodeVec(v)); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO cache_blobs(blob,chunks) VALUES(?,?)`,
		blob, len(chunks)); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM pending WHERE blob = ?`, blob); err != nil {
		return err
	}
	return tx.Commit()
}

// PendingCount reports how many blobs still wait for embedding.
func (x *Index) PendingCount() (int, error) {
	var n int
	err := x.db.QueryRow(`SELECT COUNT(*) FROM pending`).Scan(&n)
	return n, err
}

// Rebuild fully synchronizes and embeds a manifest: SyncDocs + a complete
// drain. Convenience for small KBs, tests, and eval tooling; large corpora
// should sync and drain on their own schedule.
func (x *Index) Rebuild(m *store.Manifest, content index.ContentFunc, c *Client) error {
	if _, err := x.SyncDocs(m, c); err != nil {
		return err
	}
	_, _, err := x.DrainPending(content, c, 0)
	return err
}

// Query ranks current docs by cosine similarity between qvec and their best
// chunk (max-pooling over chunks), best first.
func (x *Index) Query(qvec []float32, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := x.db.Query(`
		SELECT d.path, d.blob, d.labels, c.vec
		FROM docs d JOIN cache c ON c.blob = d.blob`)
	if err != nil {
		return nil, fmt.Errorf("vector query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type best struct {
		score  float32
		blob   string
		labels string
	}
	scores := map[string]*best{}
	for rows.Next() {
		var path, blob, labels string
		var raw []byte
		if err := rows.Scan(&path, &blob, &labels, &raw); err != nil {
			return nil, err
		}
		v, err := decodeVec(raw)
		if err != nil {
			return nil, fmt.Errorf("vector %s: %w", path, err)
		}
		if len(v) != len(qvec) {
			return nil, fmt.Errorf("vector %s: dimension %d != query %d (re-scan to rebuild)", path, len(v), len(qvec))
		}
		s := dot(qvec, v)
		if b, ok := scores[path]; !ok || s > b.score {
			scores[path] = &best{score: s, blob: blob, labels: labels}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]Result, 0, len(scores))
	for path, b := range scores {
		r := Result{Path: path, Blob: b.blob, Score: b.score}
		if b.labels != "" {
			r.Labels = strings.Fields(b.labels)
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Path < out[j].Path
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Matrix is an in-memory snapshot of the index for query-many workloads
// (eval harnesses): vectors load once, then every query is a pure in-memory
// scan instead of a full table read per query.
type Matrix struct {
	paths  []string
	blobs  []string
	labels []string
	vecs   [][]float32
}

// Matrix loads the current docs view and their chunk vectors into memory.
func (x *Index) Matrix() (*Matrix, error) {
	rows, err := x.db.Query(`
		SELECT d.path, d.blob, d.labels, c.vec
		FROM docs d JOIN cache c ON c.blob = d.blob`)
	if err != nil {
		return nil, fmt.Errorf("load matrix: %w", err)
	}
	defer func() { _ = rows.Close() }()
	m := &Matrix{}
	for rows.Next() {
		var path, blob, labels string
		var raw []byte
		if err := rows.Scan(&path, &blob, &labels, &raw); err != nil {
			return nil, err
		}
		v, err := decodeVec(raw)
		if err != nil {
			return nil, fmt.Errorf("vector %s: %w", path, err)
		}
		m.paths = append(m.paths, path)
		m.blobs = append(m.blobs, blob)
		m.labels = append(m.labels, labels)
		m.vecs = append(m.vecs, v)
	}
	return m, rows.Err()
}

// Query ranks docs by best-chunk cosine, identical semantics to Index.Query.
func (m *Matrix) Query(qvec []float32, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 20
	}
	type best struct {
		score  float32
		blob   string
		labels string
	}
	scores := map[string]*best{}
	for i, v := range m.vecs {
		if len(v) != len(qvec) {
			return nil, fmt.Errorf("vector %s: dimension %d != query %d (re-scan to rebuild)", m.paths[i], len(v), len(qvec))
		}
		s := dot(qvec, v)
		if b, ok := scores[m.paths[i]]; !ok || s > b.score {
			scores[m.paths[i]] = &best{score: s, blob: m.blobs[i], labels: m.labels[i]}
		}
	}
	out := make([]Result, 0, len(scores))
	for path, b := range scores {
		r := Result{Path: path, Blob: b.blob, Score: b.score}
		if b.labels != "" {
			r.Labels = strings.Fields(b.labels)
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Path < out[j].Path
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// encodeVec serializes a float32 vector little-endian.
func encodeVec(v []float32) []byte {
	buf := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(x))
	}
	return buf
}

// decodeVec deserializes a little-endian float32 vector.
func decodeVec(raw []byte) ([]float32, error) {
	if len(raw)%4 != 0 {
		return nil, fmt.Errorf("corrupt vector: %d bytes", len(raw))
	}
	v := make([]float32, len(raw)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	return v, nil
}
