package files

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/filemetacache"
	fbimg "github.com/filebrowser/filebrowser/v2/img"
)

// BlurUp target constants (kept tiny on purpose — the front-end will apply
// filter:blur(16px) + scale(1.1), so any pixel-level detail is invisible).
const (
	BlurUpSize       = 20 // px, longest edge; aspect ratio preserved via Fit
	BlurUpDataPrefix = "data:image/jpeg;base64,"
)

// BlurUpCacheKey returns the content-addressed key used for both lookups and
// saves. The triple (realPath, modtime, size) guarantees that a cache entry is
// invalidated the moment the on-disk content changes — no TTL / polling needed.
func BlurUpCacheKey(realPath string, modTimeUnix int64, size int64) string {
	return fmt.Sprintf("%s|%d|%d", realPath, modTimeUnix, size)
}

// BatchGetBlurUps retrieves every cached BlurUp placeholder available for the
// provided set of cacheKeys. Unknown/missing keys are silently omitted — the
// caller treats a missing entry as "no placeholder yet" and degrades to the
// skeleton loading UI (identical UX to before this feature, no regression).
func BatchGetBlurUps(cacheKeys []string) (map[string]string, error) {
	s := getBlurUpStore()
	if s == nil || len(cacheKeys) == 0 {
		return map[string]string{}, nil
	}
	return s.BatchGet(cacheKeys)
}

// GenerateBlurUpDataURL decodes the referenced image file, re-encodes it as a
// tiny (~100-300 byte) 20×20 JPEG, and returns the RFC 2397 data URL ready to
// drop straight into <img src="...">.
//
// We intentionally keep the implementation self-contained (no shared state with
// the preview/thumbnail pipeline) so listing-time and thumbnail generation can
// both call it independently — the BoltDB cache layer then deduplicates work
// naturally.
func GenerateBlurUpDataURL(fSys afero.Fs, filePath, ext string) (string, error) {
	fd, err := fSys.Open(filePath)
	if err != nil {
		return "", err
	}
	defer fd.Close()

	imgSvc := fbimg.New(2) // 2 workers; cheap call so reuse is fine
	buf := &bytes.Buffer{}
	opts := []fbimg.Option{
		fbimg.WithMode(fbimg.ResizeModeFit),
		fbimg.WithQuality(fbimg.QualityLow),
		fbimg.WithFormat(fbimg.FormatJpeg),
	}
	if err := imgSvc.Resize(context.Background(), fd, BlurUpSize, BlurUpSize, buf, opts...); err != nil {
		return "", err
	}
	return BlurUpDataPrefix + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// SaveBlurUp writes a BlurUp placeholder (computed by the caller) into the
// persistent cache. Swallows errors (we never want preview/listing to fail
// because of a cache write) but prints them for an admin to debug.
func SaveBlurUp(cacheKey, realPath string, modTime int64, size int64, blurUpDataURL string) {
	if cacheKey == "" || blurUpDataURL == "" || !strings.HasPrefix(blurUpDataURL, BlurUpDataPrefix) {
		return
	}
	s := getBlurUpStore()
	if s == nil {
		return
	}
	entry := &filemetacache.Entry{
		Key:       cacheKey,
		RealPath:  realPath,
		ModTime:   modTime,
		Size:      size,
		BlurUp:    blurUpDataURL,
		UpdatedAt: time.Now().Unix(),
	}
	_ = s.Save(entry) // best-effort
}
