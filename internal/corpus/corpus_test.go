package corpus

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

// tree reads every file under root (skipping .dig) into a path→content map.
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".dig" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestGenerateIsDeterministic(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	specA, err := Generate(a, 42, Medium)
	if err != nil {
		t.Fatal(err)
	}
	specB, err := Generate(b, 42, Medium)
	if err != nil {
		t.Fatal(err)
	}

	ta, tb := tree(t, a), tree(t, b)
	if len(ta) != len(tb) {
		t.Fatalf("same seed produced different file counts: %d vs %d", len(ta), len(tb))
	}
	for path, content := range ta {
		if tb[path] != content {
			t.Fatalf("same seed diverged at %s", path)
		}
	}
	if specA.Files != specB.Files || specA.Files != len(ta) {
		t.Fatalf("spec file count mismatch: %d / %d / %d", specA.Files, specB.Files, len(ta))
	}
}

func TestDifferentSeedsDiffer(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	if _, err := Generate(a, 1, Medium); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(b, 2, Medium); err != nil {
		t.Fatal(err)
	}
	ta, tb := tree(t, a), tree(t, b)
	same := len(ta) == len(tb)
	if same {
		for path, content := range ta {
			if tb[path] != content {
				same = false
				break
			}
		}
	}
	if same {
		t.Fatal("different seeds produced an identical corpus")
	}
}

func TestSpecMatchesDisk(t *testing.T) {
	root := t.TempDir()
	spec, err := Generate(root, 7, Small)
	if err != nil {
		t.Fatal(err)
	}
	disk := tree(t, root)

	if spec.Files != len(disk) {
		t.Fatalf("spec.Files %d != %d on disk", spec.Files, len(disk))
	}
	if spec.Files != len(spec.Paths) {
		t.Fatalf("spec.Files %d != len(Paths) %d", spec.Files, len(spec.Paths))
	}
	if spec.Files != spec.Documents+spec.Duplicates+spec.Binaries {
		t.Fatalf("spec components don't sum: %d != %d+%d+%d",
			spec.Files, spec.Documents, spec.Duplicates, spec.Binaries)
	}

	binaries := 0
	for path, content := range disk {
		if !utf8.ValidString(content) {
			binaries++
		}
		_ = path
	}
	if binaries != spec.Binaries {
		t.Fatalf("found %d non-UTF-8 files, spec says %d", binaries, spec.Binaries)
	}
}

func TestSizesScale(t *testing.T) {
	small, err := Generate(t.TempDir(), 3, Small)
	if err != nil {
		t.Fatal(err)
	}
	large, err := Generate(t.TempDir(), 3, Large)
	if err != nil {
		t.Fatal(err)
	}
	if large.Files <= small.Files {
		t.Fatalf("large (%d) should plant more than small (%d)", large.Files, small.Files)
	}
}

// TestGeneratePreservesExistingDig proves generation never writes into a KB's
// .dig directory, so a corpus can be regenerated over an initialized KB.
func TestGeneratePreservesExistingDig(t *testing.T) {
	root := t.TempDir()
	digMarker := filepath.Join(root, ".dig", "marker")
	if err := os.MkdirAll(filepath.Dir(digMarker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(digMarker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(root, 9, Small); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(digMarker)
	if err != nil || string(b) != "keep" {
		t.Fatalf(".dig marker was disturbed: %q err=%v", b, err)
	}
}
