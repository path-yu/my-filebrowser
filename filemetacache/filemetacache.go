// Package filemetacache stores lightweight per-file derived metadata (most
// notably Blur-Up placeholders) in a persistent key-value cache (BoltDB via
// storm). This is the back-end companion to the front-end's progressive
// <LazyImage blurUp="..."> rendering.
//
// Design goals:
//   - Content-addressed cache: a cache entry is considered valid ONLY while
//     the triple (RealPath, ModTimeUnix, Size) matches. Any change to a file
//     (overwrite, touch, append) naturally produces a different key and thus
//     falls back to "no placeholder" until recomputed — no invalidation logic.
//   - O(1) point lookups; listing paths do a single BatchGet over every
//     image in the directory, which is a memory lookup + 1 bolt All() on
//     first use. Typical directory sizes are well below the memory budget
//     required to hold all placeholders of a full drive.
//   - Zero blocking on the listing hot path: a miss simply leaves BlurUp
//     empty, which the front-end degrades gracefully to the skeleton screen
//     (our previous loading state, so users experience no regression). The
//     placeholder is produced lazily when the image preview/thumbnail API is
//     invoked (http/preview.go createPreview), which already decoded the
//     whole image — generating the 20x20 mini JPEG at that point is free.
package filemetacache

// Entry is a single placeholder record stored via storm (BoltDB).
//
// Key is intentionally the "cacheKey" string built by files.BlurUpCacheKey
// (realPath + modtime unix + size). Storm uses this as the primary ID so
// both point queries and batch queries are a single map lookup. We store
// the parsed components alongside for future debugging / migration scripts.
//
// BlurUp is stored as a full data URL (data:image/jpeg;base64,...) because
// the front-end can directly assign it to <img src>. Size-wise a 20x20 JPEG
// at quality 40 is ~100..300 bytes per image; a 100k-image catalog stores
// only ~20 MB of placeholders.
type Entry struct {
	Key       string `json:"key" storm:"id"`
	RealPath  string `json:"realPath"`
	ModTime   int64  `json:"modTime"` // unix seconds
	Size      int64  `json:"size"`
	BlurUp    string `json:"blurUp"` // data:image/jpeg;base64,...
	UpdatedAt int64  `json:"updatedAt"`
}

// StorageBackend is implemented by storage adapters (bolt, in-memory, ...).
// BatchGet is the performance-sensitive path used by directory listings.
type StorageBackend interface {
	BatchGet(keys []string) (map[string]string, error) // key -> BlurUp (misses omitted)
	Get(key string) (*Entry, error)
	Save(e *Entry) error
	Delete(key string) error
}

// Storage is the public façade used by the rest of the app.
type Storage struct {
	back StorageBackend
}

// NewStorage constructs a Storage around any backend.
func NewStorage(back StorageBackend) *Storage {
	return &Storage{back: back}
}

// BatchGet returns key → BlurUp for every key that has a cached entry.
// Misses are simply omitted from the map; callers MUST treat a missing key
// as "no placeholder available" and never hard-fail.
func (s *Storage) BatchGet(keys []string) (map[string]string, error) {
	return s.back.BatchGet(keys)
}

// Get fetches a single entry (primarily for debugging / migrations).
func (s *Storage) Get(key string) (*Entry, error) {
	return s.back.Get(key)
}

// Save upserts one BlurUp placeholder record.
func (s *Storage) Save(e *Entry) error {
	return s.back.Save(e)
}

// Delete removes a single cache entry (used by re-index / migration tools).
func (s *Storage) Delete(key string) error {
	return s.back.Delete(key)
}
