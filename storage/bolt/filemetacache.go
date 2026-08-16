package bolt

import (
	"errors"
	"sync"

	"github.com/asdine/storm/v3"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/filemetacache"
)

// fileMetaCacheBackend persists filemetacache.Entry records in storm/BoltDB.
//
// Listing performance note: BatchGet is implemented as a load-once in-memory
// index (allEntries + byKey) rebuilt on first use + invalidated whenever we
// Save/Delete. This keeps listing hot-paths allocation-free. The maximum
// practical dataset size for this design is a few million entries (each
// ~150 bytes of Go heap for the key strings), which comfortably covers
// multi-million file media libraries. Larger installations should replace
// this with a proper indexed KV backend.
type fileMetaCacheBackend struct {
	db *storm.DB

	mu         sync.RWMutex
	loadedOnce bool
	byKey      map[string]string // key -> BlurUp (subset used by BatchGet)
}

// rebuildIndex reads ALL entries into the byKey map.
func (b *fileMetaCacheBackend) rebuildIndex() error {
	var all []*filemetacache.Entry
	if err := b.db.All(&all); err != nil {
		if errors.Is(err, storm.ErrNotFound) {
			b.byKey = make(map[string]string)
			b.loadedOnce = true
			return nil
		}
		return err
	}
	byKey := make(map[string]string, len(all))
	for _, e := range all {
		if e.Key != "" && e.BlurUp != "" {
			byKey[e.Key] = e.BlurUp
		}
	}
	b.byKey = byKey
	b.loadedOnce = true
	return nil
}

func (b *fileMetaCacheBackend) ensureLoaded() error {
	b.mu.RLock()
	ok := b.loadedOnce
	b.mu.RUnlock()
	if ok {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.loadedOnce {
		return nil
	}
	return b.rebuildIndex()
}

// BatchGet returns every cached BlurUp value for the requested key set.
// Misses are silently omitted; it never returns an error for partial hits.
func (b *fileMetaCacheBackend) BatchGet(keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}
	if err := b.ensureLoaded(); err != nil {
		return nil, err
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := b.byKey[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func (b *fileMetaCacheBackend) Get(key string) (*filemetacache.Entry, error) {
	var v filemetacache.Entry
	err := b.db.One("Key", key, &v)
	if errors.Is(err, storm.ErrNotFound) {
		return nil, fberrors.ErrNotExist
	}
	return &v, err
}

// Save upserts a record and refreshes the in-memory BatchGet index.
func (b *fileMetaCacheBackend) Save(e *filemetacache.Entry) error {
	// Storm Update requires the struct to already exist; Save errors on
	// duplicate primary keys. The classic storm "upsert" idiom is to try
	// Update first, then fall back to Save on ErrNotFound.
	if err := b.db.Update(e); err != nil {
		if errors.Is(err, storm.ErrNotFound) {
			if saveErr := b.db.Save(e); saveErr != nil {
				return saveErr
			}
		} else {
			return err
		}
	}
	b.mu.Lock()
	if b.loadedOnce {
		if b.byKey == nil {
			b.byKey = make(map[string]string)
		}
		b.byKey[e.Key] = e.BlurUp
	}
	b.mu.Unlock()
	return nil
}

func (b *fileMetaCacheBackend) Delete(key string) error {
	err := b.db.DeleteStruct(&filemetacache.Entry{Key: key})
	if errors.Is(err, storm.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	b.mu.Lock()
	delete(b.byKey, key)
	b.mu.Unlock()
	return nil
}
