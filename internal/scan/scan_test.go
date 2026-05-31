package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bntvllnt/dig/internal/kb"
	"github.com/bntvllnt/dig/internal/store"
)

func TestWalkScansFilesAndSkipsDig(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.txt", "alpha")
	write(t, root, "sub/b.txt", "bravo")
	write(t, root, "dup.txt", "alpha") // same content as a.txt

	k, err := kb.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	dig, _ := k.EnsureDig()
	st, err := store.Open(dig)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	entries, err := Walk(k, st, false)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	// 3 files, none under .dig.
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(entries), entries)
	}
	// Entries are sorted by path (deterministic manifests).
	for i := 1; i < len(entries); i++ {
		if entries[i-1].Path > entries[i].Path {
			t.Fatalf("entries not sorted: %+v", entries)
		}
	}
	// Identical content → identical blob (dedup visible at the manifest level).
	var aHash, dupHash string
	for _, e := range entries {
		switch e.Path {
		case "a.txt":
			aHash = e.Blob
		case "dup.txt":
			dupHash = e.Blob
		}
	}
	if aHash == "" || aHash != dupHash {
		t.Fatalf("duplicate content should share a blob: a=%s dup=%s", aHash, dupHash)
	}
}

func TestWalkDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.txt", "alpha")
	k, _ := kb.Init(root)
	dig, _ := k.EnsureDig()
	st, _ := store.Open(dig)
	defer st.Close()

	entries, err := Walk(k, st, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("dry-run should still list entries, got %d", len(entries))
	}
	if has, _ := st.Blobs().Has(entries[0].Blob); has {
		t.Fatal("dry-run must not write blobs to the store")
	}
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
