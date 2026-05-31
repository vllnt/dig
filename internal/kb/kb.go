// Package kb resolves and represents a knowledge base on disk.
//
// A KB is a directory (its root) containing a .dig/ subdirectory that holds the
// store, index, and config. Per docs/architecture.md, config travels with the
// data (git-style), so a KB is self-contained and portable.
package kb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DigDir is the per-KB metadata directory at the KB root.
const DigDir = ".dig"

// KB is a resolved knowledge base.
type KB struct {
	Root string // absolute path to the KB root
}

// digPath returns the absolute .dig directory for the KB.
func (k KB) digPath() string { return filepath.Join(k.Root, DigDir) }

// DigDir returns the absolute .dig directory, creating it if missing.
func (k KB) EnsureDig() (string, error) {
	d := k.digPath()
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", d, err)
	}
	return d, nil
}

// Dig returns the absolute .dig directory without creating it.
func (k KB) Dig() string { return k.digPath() }

// Init creates a new KB rooted at dir. Errors if one already exists there.
func Init(dir string) (KB, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return KB{}, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return KB{}, fmt.Errorf("create root: %w", err)
	}
	d := filepath.Join(abs, DigDir)
	if _, err := os.Stat(d); err == nil {
		return KB{}, fmt.Errorf("KB already initialized at %s", abs)
	}
	if err := os.MkdirAll(d, 0o755); err != nil {
		return KB{}, fmt.Errorf("create %s: %w", d, err)
	}
	return KB{Root: abs}, nil
}

// ErrNotFound means no KB root was located.
var ErrNotFound = errors.New("no dig knowledge base found (run 'dig init' or pass --kb)")

// Resolve locates a KB. If root is non-empty it is used directly; otherwise it
// walks up from the current directory looking for a .dig/ directory.
func Resolve(root string) (KB, error) {
	if root != "" {
		abs, err := filepath.Abs(root)
		if err != nil {
			return KB{}, err
		}
		if _, err := os.Stat(filepath.Join(abs, DigDir)); err != nil {
			return KB{}, fmt.Errorf("no KB at %s: %w", abs, ErrNotFound)
		}
		return KB{Root: abs}, nil
	}
	cur, err := os.Getwd()
	if err != nil {
		return KB{}, err
	}
	for {
		if _, err := os.Stat(filepath.Join(cur, DigDir)); err == nil {
			return KB{Root: cur}, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return KB{}, ErrNotFound
		}
		cur = parent
	}
}
