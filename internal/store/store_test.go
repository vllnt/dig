package store

import (
	"bytes"
	"io"
	"testing"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func put(t *testing.T, be StorageBackend, content []byte) string {
	t.Helper()
	h := HashBytes(content)
	if err := be.Put(h, bytes.NewReader(content)); err != nil {
		t.Fatalf("put: %v", err)
	}
	return h
}

// Identical content must produce identical hashes and be stored once — dedup
// by construction (architecture.md §1).
func TestDedupByHash(t *testing.T) {
	st := openTest(t)
	be := st.Blobs()

	h1 := put(t, be, []byte("same bytes"))
	h2 := put(t, be, []byte("same bytes"))
	if h1 != h2 {
		t.Fatalf("identical content hashed differently: %s vs %s", h1, h2)
	}
	h3 := put(t, be, []byte("other bytes"))
	if h3 == h1 {
		t.Fatal("different content collided to same hash")
	}
	// Put of an existing blob is a no-op and content stays readable.
	rc, err := be.Get(h1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != "same bytes" {
		t.Fatalf("blob content corrupted: %q", got)
	}
}

func TestGetMissingBlob(t *testing.T) {
	st := openTest(t)
	if _, err := st.Blobs().Get("b3:deadbeef"); err != ErrBlobNotFound {
		t.Fatalf("want ErrBlobNotFound, got %v", err)
	}
}

// Each commit appends a manifest parented on the prior head; history walks the
// chain newest-first (architecture.md §1, versioning).
func TestCommitAppendsAndChains(t *testing.T) {
	st := openTest(t)

	m1, err := st.Commit("scan", []Entry{{Path: "a.txt", Blob: "b3:1"}})
	if err != nil {
		t.Fatal(err)
	}
	if m1.ID != "M1" || m1.Parent != "" {
		t.Fatalf("root manifest wrong: id=%s parent=%q", m1.ID, m1.Parent)
	}
	m2, err := st.Commit("org", []Entry{{Path: "b.txt", Blob: "b3:2"}})
	if err != nil {
		t.Fatal(err)
	}
	if m2.ID != "M2" || m2.Parent != "M1" {
		t.Fatalf("second manifest wrong: id=%s parent=%s", m2.ID, m2.Parent)
	}
	head, _ := st.Head()
	if head.ID != "M2" {
		t.Fatalf("head should be M2, got %s", head.ID)
	}
	hist, _ := st.History()
	if len(hist) != 2 || hist[0].ID != "M2" || hist[1].ID != "M1" {
		t.Fatalf("history not newest-first chain: %+v", hist)
	}
}

// Undo moves head back to the parent and restores byte-identical entries; the
// old manifest is retained (immutability), so redoing is possible later
// (architecture.md §7 data integrity).
func TestUndoRestoresPriorState(t *testing.T) {
	st := openTest(t)

	st.Commit("scan", []Entry{{Path: "a.txt", Blob: "b3:a"}})
	st.Commit("org", []Entry{{Path: "a.txt", Blob: "b3:a"}, {Path: "b.txt", Blob: "b3:b"}})

	prev, err := st.Undo()
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if prev.ID != "M1" {
		t.Fatalf("undo should land on M1, got %s", prev.ID)
	}
	head, _ := st.Head()
	if head.ID != "M1" || len(head.Entries) != 1 || head.Entries[0].Path != "a.txt" {
		t.Fatalf("undo did not restore M1 state: %+v", head)
	}
	// M2 still exists in the store — not destroyed by undo.
	if m2, _ := st.Get("M2"); m2 == nil {
		t.Fatal("undo destroyed M2; manifests must be immutable")
	}
}

func TestUndoAtRootErrors(t *testing.T) {
	st := openTest(t)
	st.Commit("scan", []Entry{{Path: "a.txt", Blob: "b3:a"}})
	if _, err := st.Undo(); err == nil {
		t.Fatal("undo at root manifest should error")
	}
}

func TestUndoEmptyErrors(t *testing.T) {
	st := openTest(t)
	if _, err := st.Undo(); err == nil {
		t.Fatal("undo on empty store should error")
	}
}
