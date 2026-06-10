package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"go.etcd.io/bbolt"
)

// Store is a knowledge base's content-addressed store: a blob StorageBackend
// plus a bbolt database holding the manifest journal and the head pointer.
//
// History is append-only: each commit writes a new manifest whose Parent is the
// current head, then advances head. Undo moves head back to a parent — content
// is never deleted, so undo restores byte-identical state.
type Store struct {
	blobs StorageBackend
	db    *bbolt.DB
	clock func() time.Time
}

const (
	dbFile = "store.db"
	blobs  = "blobs"
)

var (
	bktManifests = []byte("manifests") // ID -> JSON manifest
	bktMeta      = []byte("meta")      // "head" -> manifest ID, "seq" -> counter
	keyHead      = []byte("head")
	keySeq       = []byte("seq")
)

// Open opens (creating if needed) the store rooted at a KB's .dig directory.
func Open(digDir string) (*Store, error) {
	be, err := NewFSBackend(filepath.Join(digDir, blobs))
	if err != nil {
		return nil, err
	}
	db, err := bbolt.Open(filepath.Join(digDir, dbFile), 0o644, &bbolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("open store db: %w", err)
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		for _, b := range [][]byte{bktManifests, bktMeta, bktViews} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init buckets: %w", err)
	}
	return &Store{blobs: be, db: db, clock: time.Now}, nil
}

// Close releases the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// Blobs exposes the backend so callers (scan) can Put content.
func (s *Store) Blobs() StorageBackend { return s.blobs }

// Commit appends a new manifest with the given entries, parented on the current
// head, and advances head. Returns the stored manifest. kind is KindObserve for
// commits that record disk as found (scan) or KindMutate for commits recording
// changes dig itself made (org, dedup) — empty defaults to KindObserve.
func (s *Store) Commit(createdBy string, kind string, entries []Entry) (*Manifest, error) {
	if kind == "" {
		kind = KindObserve
	}
	var out *Manifest
	err := s.db.Update(func(tx *bbolt.Tx) error {
		meta := tx.Bucket(bktMeta)
		man := tx.Bucket(bktManifests)

		seq := decodeSeq(meta.Get(keySeq)) + 1
		m := &Manifest{
			ID:        "M" + strconv.FormatUint(seq, 10),
			Parent:    string(meta.Get(keyHead)),
			CreatedAt: s.clock().UTC(),
			CreatedBy: createdBy,
			Kind:      kind,
			Entries:   entries,
		}
		buf, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("marshal manifest: %w", err)
		}
		if err := man.Put([]byte(m.ID), buf); err != nil {
			return err
		}
		if err := meta.Put(keySeq, encodeSeq(seq)); err != nil {
			return err
		}
		if err := meta.Put(keyHead, []byte(m.ID)); err != nil {
			return err
		}
		out = m
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Head returns the current manifest, or nil if the store has no commits yet.
func (s *Store) Head() (*Manifest, error) {
	var out *Manifest
	err := s.db.View(func(tx *bbolt.Tx) error {
		id := tx.Bucket(bktMeta).Get(keyHead)
		if len(id) == 0 {
			return nil
		}
		return loadManifest(tx, string(id), &out)
	})
	return out, err
}

// Get returns the manifest with the given ID, or nil if absent.
func (s *Store) Get(id string) (*Manifest, error) {
	var out *Manifest
	err := s.db.View(func(tx *bbolt.Tx) error {
		return loadManifest(tx, id, &out)
	})
	return out, err
}

// History returns manifests from head back to root (newest first).
func (s *Store) History() ([]*Manifest, error) {
	var out []*Manifest
	err := s.db.View(func(tx *bbolt.Tx) error {
		id := string(tx.Bucket(bktMeta).Get(keyHead))
		for id != "" {
			var m *Manifest
			if err := loadManifest(tx, id, &m); err != nil {
				return err
			}
			if m == nil {
				break
			}
			out = append(out, m)
			id = m.Parent
		}
		return nil
	})
	return out, err
}

// Undo moves head to the current head's parent. It returns the manifest that
// was undone and the new head, so callers can reverse disk changes when the
// undone manifest was a mutation (KindMutate). Errors if there is nothing to
// undo (no commits, or at root).
func (s *Store) Undo() (undone *Manifest, head *Manifest, err error) {
	err = s.db.Update(func(tx *bbolt.Tx) error {
		meta := tx.Bucket(bktMeta)
		id := string(meta.Get(keyHead))
		if id == "" {
			return fmt.Errorf("nothing to undo: store is empty")
		}
		var cur *Manifest
		if err := loadManifest(tx, id, &cur); err != nil {
			return err
		}
		if cur == nil || cur.Parent == "" {
			return fmt.Errorf("nothing to undo: already at root manifest")
		}
		if err := meta.Put(keyHead, []byte(cur.Parent)); err != nil {
			return err
		}
		undone = cur
		return loadManifest(tx, cur.Parent, &head)
	})
	if err != nil {
		return nil, nil, err
	}
	return undone, head, nil
}

func loadManifest(tx *bbolt.Tx, id string, dst **Manifest) error {
	raw := tx.Bucket(bktManifests).Get([]byte(id))
	if raw == nil {
		*dst = nil
		return nil
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("unmarshal manifest %s: %w", id, err)
	}
	*dst = &m
	return nil
}

func encodeSeq(n uint64) []byte { return []byte(strconv.FormatUint(n, 10)) }

func decodeSeq(b []byte) uint64 {
	if len(b) == 0 {
		return 0
	}
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
