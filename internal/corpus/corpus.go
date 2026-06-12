// Package corpus generates deterministic, seeded "messy" knowledge bases for
// the eval harness: nested directories, duplicate-content files, binaries,
// misnamed and unsorted clutter. The same seed always produces a
// byte-identical tree, so lifecycle regressions and benchmark runs are
// reproducible.
package corpus

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
)

// Size selects the scale of a generated corpus.
type Size string

const (
	// Small is a quick smoke-scale corpus.
	Small Size = "small"
	// Medium is the default regression scale.
	Medium Size = "medium"
	// Large stresses the lifecycle with deeper nesting and more duplicates.
	Large Size = "large"
)

// scale holds the per-size generation counts.
type scale struct {
	docs       int // unique text documents
	dupes      int // extra copies of existing documents (duplicate content)
	binaries   int // non-UTF-8 files
	maxDepth   int // deepest directory nesting
	categories int // top-level subject folders
}

func scaleFor(s Size) scale {
	switch s {
	case Large:
		return scale{docs: 200, dupes: 40, binaries: 20, maxDepth: 5, categories: 8}
	case Small:
		return scale{docs: 15, dupes: 4, binaries: 2, maxDepth: 2, categories: 3}
	default:
		return scale{docs: 60, dupes: 12, binaries: 6, maxDepth: 3, categories: 5}
	}
}

// Spec describes what a generation planted, so tests can assert outcomes
// without re-deriving them.
type Spec struct {
	Seed       int64
	Size       Size
	Files      int      // total files written
	Documents  int      // unique-content text documents
	Duplicates int      // extra copies (same content as an existing doc)
	Binaries   int      // non-UTF-8 files
	Paths      []string // every written path, KB-relative, sorted
}

// vocab seeds readable, deterministic document content.
var (
	subjects = []string{"finance", "research", "media", "legal", "personal", "ops", "design", "archive"}
	nouns    = []string{"invoice", "report", "memo", "contract", "photo", "notes", "draft", "summary", "ledger", "plan"}
	words    = []string{"acme", "quarterly", "revenue", "renewal", "budget", "review", "vendor", "project", "milestone", "audit", "policy", "backup"}
	exts     = []string{"txt", "md", "log", "csv"}
)

// Generate writes a messy KB under root and returns a Spec. The same (seed,
// size) always produces a byte-identical tree. root is created if missing; an
// existing .dig directory is left untouched (generation never plants there).
func Generate(root string, seed int64, size Size) (*Spec, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	rng := rand.New(rand.NewSource(seed)) //nolint:gosec // determinism, not security
	sc := scaleFor(size)
	spec := &Spec{Seed: seed, Size: size, Documents: sc.docs, Duplicates: sc.dupes, Binaries: sc.binaries}

	type doc struct {
		path    string
		content string
	}
	var docs []doc
	write := func(path, content string) error {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return err
		}
		spec.Paths = append(spec.Paths, path)
		return nil
	}

	// Unique documents, scattered across nested category folders.
	for i := 0; i < sc.docs; i++ {
		path := randomPath(rng, sc, i)
		content := randomContent(rng, i)
		if err := write(path, content); err != nil {
			return nil, err
		}
		docs = append(docs, doc{path: path, content: content})
	}

	// Duplicate-content files: copy an existing doc's content to a new path.
	for i := 0; i < sc.dupes && len(docs) > 0; i++ {
		src := docs[rng.Intn(len(docs))]
		path := randomPath(rng, sc, sc.docs+i)
		if err := write(path, src.content); err != nil {
			return nil, err
		}
	}

	// Binary clutter (non-UTF-8) that the index must skip and dedup must ignore.
	for i := 0; i < sc.binaries; i++ {
		path := fmt.Sprintf("unsorted/blob-%03d.bin", i)
		if err := write(path, randomBinary(rng)); err != nil {
			return nil, err
		}
	}

	spec.Files = len(spec.Paths)
	spec.Documents = sc.docs
	sort.Strings(spec.Paths)
	return spec, nil
}

func randomPath(rng *rand.Rand, sc scale, n int) string {
	depth := 1 + rng.Intn(sc.maxDepth)
	parts := make([]string, 0, depth+1)
	parts = append(parts, subjects[rng.Intn(min(sc.categories, len(subjects)))])
	for d := 1; d < depth; d++ {
		parts = append(parts, fmt.Sprintf("%s-%d", words[rng.Intn(len(words))], rng.Intn(20)))
	}
	name := fmt.Sprintf("%s-%03d.%s", nouns[rng.Intn(len(nouns))], n, exts[rng.Intn(len(exts))])
	parts = append(parts, name)
	return filepath.ToSlash(filepath.Join(parts...))
}

func randomContent(rng *rand.Rand, n int) string {
	lines := 3 + rng.Intn(8)
	out := fmt.Sprintf("document %d\n", n)
	for i := 0; i < lines; i++ {
		wc := 4 + rng.Intn(8)
		line := ""
		for w := 0; w < wc; w++ {
			line += words[rng.Intn(len(words))] + " "
		}
		out += line + "\n"
	}
	return out
}

func randomBinary(rng *rand.Rand) string {
	b := make([]byte, 64+rng.Intn(192))
	for i := range b {
		b[i] = byte(rng.Intn(256))
	}
	// Guarantee a non-UTF-8 byte so it is never mistaken for text.
	b[0] = 0xff
	return string(b)
}
