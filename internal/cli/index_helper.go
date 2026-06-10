package cli

import (
	"github.com/bntvllnt/dig/internal/index"
	"github.com/bntvllnt/dig/internal/store"
)

// rebuildIndex refreshes the KB's search index from a manifest, feeding file
// text from the blob store so find matches content, not just paths. The one
// path every command that moves the head goes through.
func rebuildIndex(digDir string, st *store.Store, m *store.Manifest) error {
	idx, err := index.Open(digDir)
	if err != nil {
		return err
	}
	defer func() { _ = idx.Close() }()
	return idx.Rebuild(m, index.BlobContent(st.Blobs()))
}
