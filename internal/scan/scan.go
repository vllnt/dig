// Package scan walks a KB's files into the content-addressed store, producing
// the entry set for a manifest. It skips the .dig metadata directory.
package scan

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/bntvllnt/dig/internal/kb"
	"github.com/bntvllnt/dig/internal/store"
)

// Walk scans the KB root, stores each file's content as a blob, and returns
// the manifest entries (sorted by path for deterministic manifests). When
// dryRun is true, content is hashed but not written to the store.
func Walk(k kb.KB, st *store.Store, dryRun bool) ([]store.Entry, error) {
	var entries []store.Entry
	be := st.Blobs()

	err := filepath.WalkDir(k.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == kb.DigDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // skip symlinks, sockets, devices
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		hash := store.HashBytes(content)
		if !dryRun {
			if err := be.Put(hash, bytes.NewReader(content)); err != nil {
				return fmt.Errorf("store %s: %w", path, err)
			}
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(k.Root, path)
		if err != nil {
			return err
		}
		entries = append(entries, store.Entry{
			Path:    filepath.ToSlash(rel),
			Blob:    hash,
			Size:    info.Size(),
			ModTime: info.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}
