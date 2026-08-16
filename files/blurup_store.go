package files

import (
	"sync"

	"github.com/filebrowser/filebrowser/v2/filemetacache"
)

// blurUpStore is the package-level handle to the persistent placeholder cache.
// It's injected exactly once during HTTP server construction (NewHandler) via
// SetBlurUpStore; before that all lookup/save calls are safe no-ops so the
// listing path never crashes when the storage layer isn't wired yet (tests,
// one-off CLI commands).
var (
	blurUpMu    sync.RWMutex
	blurUpStore *filemetacache.Storage
)

// SetBlurUpStore wires the persistent cache backing into the files package.
// Called once from fbhttp.NewHandler after bolt.NewStorage produced a *Storage.
func SetBlurUpStore(s *filemetacache.Storage) {
	blurUpMu.Lock()
	defer blurUpMu.Unlock()
	blurUpStore = s
}

// getBlurUpStore safely returns the current cache (nil when not yet wired).
func getBlurUpStore() *filemetacache.Storage {
	blurUpMu.RLock()
	defer blurUpMu.RUnlock()
	return blurUpStore
}
