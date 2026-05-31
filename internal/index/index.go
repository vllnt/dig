// Package index provides full-text search over a manifest's entries.
//
// Per docs/architecture.md the index is an IndexBackend seam; the first-party
// implementation is SQLite FTS5 via modernc.org/sqlite (pure-Go, no cgo). The
// index is a derived view — it is rebuilt from a manifest and never a source of
// truth.
package index

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/bntvllnt/dig/internal/store"
)

// Result is one search hit, ranked by FTS5 relevance (best first).
type Result struct {
	Path   string
	Blob   string
	Labels []string
}

// IndexBackend is the seam for where the searchable index lives.
type IndexBackend interface {
	Rebuild(m *store.Manifest) error
	Query(q string, limit int) ([]Result, error)
	Close() error
}

// FTS is the SQLite FTS5 implementation of IndexBackend.
type FTS struct {
	db *sql.DB
}

const indexFile = "index.db"

// Open opens (creating if needed) the FTS index in a KB's .dig directory.
func Open(digDir string) (*FTS, error) {
	db, err := sql.Open("sqlite", filepath.Join(digDir, indexFile))
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS docs USING fts5(
		path, labels, body, blob UNINDEXED
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create fts table: %w", err)
	}
	return &FTS{db: db}, nil
}

// Close releases the database.
func (f *FTS) Close() error { return f.db.Close() }

// Rebuild replaces the index contents with the entries of m. Body text is
// seeded from the path's terms; richer extracted text is added in a later phase
// (the extraction pipeline). Rebuilding from the manifest keeps the index a
// pure derived view.
func (f *FTS) Rebuild(m *store.Manifest) error {
	tx, err := f.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.Exec(`DELETE FROM docs`); err != nil {
		return fmt.Errorf("clear index: %w", err)
	}
	if m == nil {
		return tx.Commit()
	}
	stmt, err := tx.Prepare(`INSERT INTO docs(path, labels, body, blob) VALUES(?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range m.Entries {
		labels := strings.Join(e.Labels, " ")
		body := pathTerms(e.Path)
		if _, err := stmt.Exec(e.Path, labels, body, e.Blob); err != nil {
			return fmt.Errorf("index %s: %w", e.Path, err)
		}
	}
	return tx.Commit()
}

// Query runs an FTS5 match across path, labels, and body, ranked by relevance.
func (f *FTS) Query(q string, limit int) ([]Result, error) {
	if strings.TrimSpace(q) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := f.db.Query(
		`SELECT path, blob, labels FROM docs WHERE docs MATCH ? ORDER BY rank LIMIT ?`,
		ftsQuery(q), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var out []Result
	for rows.Next() {
		var r Result
		var labels string
		if err := rows.Scan(&r.Path, &r.Blob, &labels); err != nil {
			return nil, err
		}
		if labels != "" {
			r.Labels = strings.Fields(labels)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// pathTerms expands a path into searchable tokens (directory and filename
// components, separators normalized to spaces).
func pathTerms(p string) string {
	repl := strings.NewReplacer("/", " ", "_", " ", "-", " ", ".", " ")
	return repl.Replace(p)
}

// ftsQuery turns a user query into a safe FTS5 expression: each term becomes a
// quoted prefix match, AND-ed together. Quoting avoids FTS5 syntax injection.
func ftsQuery(q string) string {
	fields := strings.Fields(q)
	terms := make([]string, 0, len(fields))
	for _, t := range fields {
		t = strings.ReplaceAll(t, `"`, `""`)
		terms = append(terms, `"`+t+`"*`)
	}
	return strings.Join(terms, " AND ")
}
