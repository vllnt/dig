// Package store implements dig's content-addressed store: blobs keyed by
// content hash, versioned tree manifests, and an append-only journal.
//
// Per docs/architecture.md, blob bytes live behind a StorageBackend interface
// so the location of bytes (local dir, S3, ...) is pluggable, while the store
// semantics (hashing, manifests, journal) stay in the core.
package store

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"lukechampine.com/blake3"
)

// HashPrefix marks the hash algorithm in a blob key, e.g. "b3:9f2a...".
const HashPrefix = "b3:"

// ErrBlobNotFound is returned by a StorageBackend when a hash is absent.
var ErrBlobNotFound = errors.New("blob not found")

// StorageBackend is the seam for where blob bytes physically live.
// First-party impl is the local filesystem; extensions provide S3/GCS/etc.
type StorageBackend interface {
	// Put stores content under hash. Storing an existing hash is a no-op
	// (content-addressed: identical content => identical hash => dedup).
	Put(hash string, r io.Reader) error
	// Get returns a reader for the blob, or ErrBlobNotFound.
	Get(hash string) (io.ReadCloser, error)
	// Has reports whether the blob exists.
	Has(hash string) (bool, error)
}

// HashBytes returns the canonical blob key for content.
func HashBytes(b []byte) string {
	sum := blake3.Sum256(b)
	return fmt.Sprintf("%s%x", HashPrefix, sum)
}

// FSBackend stores blobs as files under root, sharded by the first two hex
// characters of the hash to avoid oversized directories.
type FSBackend struct {
	root string
}

// NewFSBackend creates a filesystem-backed store rooted at dir.
func NewFSBackend(dir string) (*FSBackend, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create blob dir: %w", err)
	}
	return &FSBackend{root: dir}, nil
}

func (b *FSBackend) shardPath(hash string) (dir, file string) {
	h := hash[len(HashPrefix):] // strip "b3:"
	return filepath.Join(b.root, h[:2]), filepath.Join(b.root, h[:2], h[2:])
}

// Put writes content to its sharded path. No-op if the blob already exists.
func (b *FSBackend) Put(hash string, r io.Reader) error {
	dir, file := b.shardPath(hash)
	if _, err := os.Stat(file); err == nil {
		return nil // already stored — dedup
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create shard dir: %w", err)
	}
	tmp := file + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create blob tmp: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("write blob: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close blob: %w", err)
	}
	// Atomic publish so a crashed write never leaves a partial blob.
	if err := os.Rename(tmp, file); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("publish blob: %w", err)
	}
	return nil
}

// Get opens the blob for reading.
func (b *FSBackend) Get(hash string) (io.ReadCloser, error) {
	_, file := b.shardPath(hash)
	f, err := os.Open(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrBlobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open blob: %w", err)
	}
	return f, nil
}

// Has reports whether the blob exists.
func (b *FSBackend) Has(hash string) (bool, error) {
	_, file := b.shardPath(hash)
	_, err := os.Stat(file)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
