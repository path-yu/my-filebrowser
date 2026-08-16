package files

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

// benchName returns the deterministic file name used by benchDir.
func benchName(i int) string {
	return "file" + string(rune('a'+i%26)) + itoa(i) + ".txt"
}

// benchDir creates a temporary directory with n files and returns its path.
func benchDir(b *testing.B, n int) string {
	b.Helper()
	dir := b.TempDir()
	for i := 0; i < n; i++ {
		name := filepath.Join(dir, benchName(i))
		if err := os.WriteFile(name, []byte("hello world 0123456789"), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	return dir
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

// BenchmarkScopedFsStat measures the per-operation cost of Stat through
// ScopedFs, whose guard() resolves the scope root symlinks on every call.
func BenchmarkScopedFsStat(b *testing.B) {
	dir := benchDir(b, 50)
	fs := NewScopedFs(afero.OsFs{}, dir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := fs.Stat("/"+benchName(0)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkScopedFsOpen measures the per-operation cost of Open through
// ScopedFs (the dominant op during listings with header sniffing).
func BenchmarkScopedFsOpen(b *testing.B) {
	dir := benchDir(b, 50)
	fs := NewScopedFs(afero.OsFs{}, dir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f, err := fs.Open("/"+benchName(0))
		if err != nil {
			b.Fatal(err)
		}
		f.Close()
	}
}

// BenchmarkScopedFsListing measures an end-to-end 50-entry listing.
func BenchmarkScopedFsListing(b *testing.B) {
	dir := benchDir(b, 50)
	fs := NewScopedFs(afero.OsFs{}, dir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := afero.ReadDir(fs, "/"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkOsFsStat is the baseline: plain OsFs Stat without the scoped guard.
func BenchmarkOsFsStat(b *testing.B) {
	dir := benchDir(b, 50)
	fs := afero.OsFs{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := fs.Stat(filepath.Join(dir, benchName(0))); err != nil {
			b.Fatal(err)
		}
	}
}
