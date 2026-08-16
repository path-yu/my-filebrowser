package files

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/afero"
)

// TestListingSpeedUNC measures directory listing latency with ReadHeader=true
// against the real UNC share, using the same ScopedFs the server uses.
// Skipped when the share is unreachable.
func TestListingSpeedUNC(t *testing.T) {
	root := `\\Sjwh\技术部\方案图检索系统V1.0\file`
	if _, err := os.Stat(root); err != nil {
		t.Skip("UNC share not reachable:", err)
	}

	fs := NewScopedFs(afero.OsFs{}, root)

	start := time.Now()
	file, err := NewFileInfo(&FileOptions{
		Fs:         fs,
		Path:       "/图纸",
		Modify:     false,
		Expand:     true,
		ReadHeader: true,
		Checker:    allowAll{},
	})
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	t.Logf("图纸 listing took %v, items=%d", elapsed, len(file.Listing.Items))

	if file.Listing.NumFiles < 1000 {
		t.Fatalf("expected 1000+ files, got %d", file.Listing.NumFiles)
	}
	pdfCount := 0
	for _, item := range file.Listing.Items {
		if item.Type == "pdf" {
			pdfCount++
		}
	}
	if pdfCount < 1000 {
		t.Fatalf("expected 1000+ pdf-typed items, got %d", pdfCount)
	}
	// Listing a 1000+ file directory should stay well under 3 seconds.
	if elapsed > 3*time.Second {
		t.Fatalf("listing too slow: %v", elapsed)
	}
}

type allowAll struct{}

func (allowAll) Check(string) bool { return true }
