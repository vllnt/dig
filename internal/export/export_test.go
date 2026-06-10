package export

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bntvllnt/dig/internal/kb"
	"github.com/bntvllnt/dig/internal/scan"
	"github.com/bntvllnt/dig/internal/store"
)

func setupKB(t *testing.T, files map[string]string) (kb.KB, *store.Store, *store.Manifest) {
	t.Helper()
	root := t.TempDir()
	mt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	k, err := kb.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	dig, _ := k.EnsureDig()
	st, err := store.Open(dig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	entries, err := scan.Walk(k, st, false)
	if err != nil {
		t.Fatal(err)
	}
	head, err := st.Commit("scan", store.KindObserve, entries)
	if err != nil {
		t.Fatal(err)
	}
	return k, st, head
}

func emit(t *testing.T, st *store.Store, m *store.Manifest, filter string) string {
	t.Helper()
	f, err := ParseFilter(filter)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := Write(&buf, st, m, f); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// THE contract: pinned manifest → byte-identical re-export, even after the
// live tree was mutated. Content must come from the blob store, never disk.
func TestExportDeterministicAfterDiskMutation(t *testing.T) {
	k, st, head := setupKB(t, map[string]string{"a.txt": "original", "b.txt": "bravo"})

	first := emit(t, st, head, "")

	// Mutate the live tree: edit one file, delete the other.
	if err := os.WriteFile(filepath.Join(k.Root, "a.txt"), []byte("CHANGED"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(k.Root, "b.txt")); err != nil {
		t.Fatal(err)
	}

	second := emit(t, st, head, "")
	if first != second {
		t.Fatal("pinned export must be byte-identical after disk mutation")
	}
	if !strings.Contains(first, `"original"`) {
		t.Fatal("export should carry the manifest-time content")
	}
}

func TestExportProvenancePerRow(t *testing.T) {
	_, st, head := setupKB(t, map[string]string{"a.txt": "alpha"})
	out := emit(t, st, head, "")
	for _, want := range []string{`"src":"b3:`, `"manifest":"M1"`, `"path":"a.txt"`, `"text":"alpha"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("row missing %s:\n%s", want, out)
		}
	}
}

func TestExportBinaryRowsHaveNoText(t *testing.T) {
	_, st, head := setupKB(t, map[string]string{"blob.bin": "\xff\xfe\x00binary"})
	out := emit(t, st, head, "")
	if strings.Contains(out, `"text"`) {
		t.Fatalf("binary content must not be emitted as text: %s", out)
	}
	if !strings.Contains(out, `"binary":true`) {
		t.Fatalf("binary row should be flagged: %s", out)
	}
}

func TestFilters(t *testing.T) {
	_, st, head := setupKB(t, map[string]string{
		"docs/x.pdf": "pdf",
		"notes/y.md": "md",
	})
	// Inject labels as a mutate commit (what org does).
	entries := append([]store.Entry{}, head.Entries...)
	for i := range entries {
		if entries[i].Path == "docs/x.pdf" {
			entries[i].Labels = []string{"finance"}
		}
	}
	labeled, err := st.Commit("org", store.KindMutate, entries)
	if err != nil {
		t.Fatal(err)
	}

	if out := emit(t, st, labeled, "label:finance"); !strings.Contains(out, "x.pdf") || strings.Contains(out, "y.md") {
		t.Fatalf("label filter wrong:\n%s", out)
	}
	if out := emit(t, st, labeled, "path:*.md"); !strings.Contains(out, "y.md") || strings.Contains(out, "x.pdf") {
		t.Fatalf("path filter wrong:\n%s", out)
	}
	if out := emit(t, st, labeled, "after:2024-01-01 before:2025-01-01"); !strings.Contains(out, "x.pdf") {
		t.Fatalf("date window should include 2024-06 files:\n%s", out)
	}
	if out := emit(t, st, labeled, "after:2025-01-01"); strings.Contains(out, "x.pdf") {
		t.Fatalf("after-filter should exclude 2024 files:\n%s", out)
	}
}

// Contract POV: malformed filters fail loudly.
func TestFilterParseErrors(t *testing.T) {
	for _, expr := range []string{"bogus:x", "label", "after:junk", "path:[", "nokey"} {
		if _, err := ParseFilter(expr); err == nil {
			t.Errorf("filter %q should be rejected", expr)
		}
	}
}

func TestExportNilManifestErrors(t *testing.T) {
	_, st, _ := setupKB(t, map[string]string{"a.txt": "x"})
	var buf bytes.Buffer
	if _, err := Write(&buf, st, nil, nil); err == nil {
		t.Fatal("nil manifest must error with guidance")
	}
}
