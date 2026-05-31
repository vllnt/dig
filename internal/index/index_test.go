package index

import (
	"testing"

	"github.com/bntvllnt/dig/internal/store"
)

func openTest(t *testing.T) *FTS {
	t.Helper()
	idx, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func TestQueryMatchesPathAndLabels(t *testing.T) {
	idx := openTest(t)
	m := &store.Manifest{Entries: []store.Entry{
		{Path: "finance/invoices/2024/acme-1007.pdf", Blob: "b3:1", Labels: []string{"finance", "invoice"}},
		{Path: "media/photos/2024/05/beach.jpg", Blob: "b3:2", Labels: []string{"photo"}},
	}}
	if err := idx.Rebuild(m); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	// Match by path term.
	res, err := idx.Query("acme", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Blob != "b3:1" {
		t.Fatalf("expected the invoice, got %+v", res)
	}
	// Match by label.
	res, _ = idx.Query("photo", 10)
	if len(res) != 1 || res[0].Path != "media/photos/2024/05/beach.jpg" {
		t.Fatalf("label query failed: %+v", res)
	}
	// Labels are returned parsed.
	res, _ = idx.Query("invoice", 10)
	if len(res) != 1 || len(res[0].Labels) != 2 {
		t.Fatalf("labels not round-tripped: %+v", res)
	}
}

func TestQueryEmptyReturnsNil(t *testing.T) {
	idx := openTest(t)
	idx.Rebuild(&store.Manifest{Entries: []store.Entry{{Path: "a.txt", Blob: "b3:1"}}})
	res, err := idx.Query("   ", 10)
	if err != nil || res != nil {
		t.Fatalf("empty query should be (nil,nil), got (%v,%v)", res, err)
	}
}

// Rebuild is a full replace: the index is a derived view, never accumulates
// stale rows (architecture.md §1 — index rebuilt from manifests).
func TestRebuildReplaces(t *testing.T) {
	idx := openTest(t)
	idx.Rebuild(&store.Manifest{Entries: []store.Entry{{Path: "old.txt", Blob: "b3:old"}}})
	idx.Rebuild(&store.Manifest{Entries: []store.Entry{{Path: "new.txt", Blob: "b3:new"}}})

	if res, _ := idx.Query("old", 10); len(res) != 0 {
		t.Fatalf("stale entry survived rebuild: %+v", res)
	}
	if res, _ := idx.Query("new", 10); len(res) != 1 {
		t.Fatal("new entry missing after rebuild")
	}
}

// A query containing FTS5 syntax must not error (injection-safe quoting).
func TestQuerySyntaxSafe(t *testing.T) {
	idx := openTest(t)
	idx.Rebuild(&store.Manifest{Entries: []store.Entry{{Path: "report.txt", Blob: "b3:1"}}})
	if _, err := idx.Query(`report" OR 1=1 --`, 10); err != nil {
		t.Fatalf("query with FTS metacharacters errored: %v", err)
	}
}
