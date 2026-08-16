package files

import (
	"bytes"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestScopedFs(t *testing.T) {
	t.Run("path inside scope is allowed", func(t *testing.T) {
		scope := t.TempDir()
		if err := os.WriteFile(filepath.Join(scope, "file.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		fs := NewScopedFs(afero.NewOsFs(), scope)

		if _, err := fs.Stat("/file.txt"); err != nil {
			t.Fatalf("expected in-scope file to be accessible, got %v", err)
		}
	})

	t.Run("new file inside scope can be created", func(t *testing.T) {
		scope := t.TempDir()
		fs := NewScopedFs(afero.NewOsFs(), scope)

		f, err := fs.OpenFile("/does-not-exist-yet.txt", os.O_RDWR|os.O_CREATE, 0o644)
		if err != nil {
			t.Fatalf("expected to create a new in-scope file, got %v", err)
		}
		_ = f.Close()
	})

	// Regression for #5975: when the scope resolves to the filesystem root,
	// root+separator used to be "//", which no path matched, so every write
	// was rejected with os.ErrPermission (HTTP 403).
	t.Run("filesystem root scope allows access", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "file.txt")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}

		// On Windows the filesystem root is the drive root (e.g. "C:\"), not
		// "/", and virtual paths must stay relative to it — an absolute
		// "C:\..." virtual path is rejected by BasePathFs by design.
		scope, rel := "/", f
		if vol := filepath.VolumeName(f); vol != "" {
			scope = vol + string(filepath.Separator)
			rel = strings.TrimPrefix(f, vol)
		}
		fs := NewScopedFs(afero.NewOsFs(), scope)

		if _, err := fs.Stat(rel); err != nil {
			t.Fatalf("expected a path under root scope to be accessible, got %v", err)
		}
	})

	t.Run("escaping symlink to a sibling is rejected", func(t *testing.T) {
		base := t.TempDir()
		scope := filepath.Join(base, "srv")
		sibling := filepath.Join(base, "srvother")
		for _, d := range []string{scope, sibling} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(sibling, "secret.txt"), []byte("secret"), 0o644); err != nil {
			t.Fatal(err)
		}
		// A symlink lexically inside the scope pointing at a sibling directory
		// must not be followed for reads or stats.
		if err := os.Symlink(sibling, filepath.Join(scope, "escape")); err != nil {
			t.Skipf("cannot create symlink: %v", err)
		}
		fs := NewScopedFs(afero.NewOsFs(), scope)

		if _, err := fs.Stat("/escape"); !os.IsPermission(err) {
			t.Fatalf("expected stat of escaping symlink to be rejected, got %v", err)
		}
		if _, err := fs.Open("/escape/secret.txt"); !os.IsPermission(err) {
			t.Fatalf("expected read through escaping symlink to be rejected, got %v", err)
		}
	})

	t.Run("symlink whose target stays within scope is allowed", func(t *testing.T) {
		scope := t.TempDir()
		if err := os.MkdirAll(filepath.Join(scope, "real"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(scope, "real", "f.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(scope, "real"), filepath.Join(scope, "link")); err != nil {
			t.Skipf("cannot create symlink: %v", err)
		}
		fs := NewScopedFs(afero.NewOsFs(), scope)

		if _, err := fs.Stat("/link/f.txt"); err != nil {
			t.Fatalf("expected in-scope symlink target to be accessible, got %v", err)
		}
	})
}

// stat must reject a regular file reached through a symlinked ancestor that
// escapes the scope (GHSA-hf77-9m7w-fq8q), while still serving in-scope files.
func TestStatRejectsLinkedAncestorEscape(t *testing.T) {
	scope := t.TempDir()
	if err := os.MkdirAll(filepath.Join(scope, "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(scope, "private"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scope, "private", "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scope, "shared", "ok.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(scope, "private"), filepath.Join(scope, "shared", "link")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// Filesystem scoped to the shared directory, as a public share would be.
	bfs := NewScopedFs(afero.NewOsFs(), filepath.Join(scope, "shared"))

	if _, err := stat(&FileOptions{Fs: bfs, Path: "/link/secret.txt"}); !os.IsPermission(err) {
		t.Fatalf("expected permission error for linked-ancestor escape, got %v", err)
	}
	if _, err := stat(&FileOptions{Fs: bfs, Path: "/ok.txt"}); err != nil {
		t.Fatalf("expected in-scope file to be served, got %v", err)
	}
}

type allowAllChecker struct{}

func (allowAllChecker) Check(string) bool {
	return true
}

type inaccessibleChildFs struct {
	afero.Fs
	child string
}

func (fs inaccessibleChildFs) Open(name string) (afero.File, error) {
	file, err := fs.Fs.Open(name)
	if err != nil {
		return nil, err
	}

	if path.Clean(name) == "/" {
		return inaccessibleChildDir{File: file}, nil
	}

	return file, nil
}

func (fs inaccessibleChildFs) Stat(name string) (os.FileInfo, error) {
	if path.Clean(name) == fs.child {
		return nil, os.ErrPermission
	}

	return fs.Fs.Stat(name)
}

func (fs inaccessibleChildFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	if path.Clean(name) == fs.child {
		return nil, false, os.ErrPermission
	}

	if lstater, ok := fs.Fs.(afero.Lstater); ok {
		return lstater.LstatIfPossible(name)
	}

	info, err := fs.Fs.Stat(name)
	return info, false, err
}

type inaccessibleChildDir struct {
	afero.File
}

func (dir inaccessibleChildDir) Readdir(int) ([]os.FileInfo, error) {
	return nil, os.ErrPermission
}

func TestReadListingSkipsInaccessibleChildren(t *testing.T) {
	memFs := afero.NewMemMapFs()
	for _, dir := range []string{"/media", "/proton-mount"} {
		if err := memFs.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	file, err := NewFileInfo(&FileOptions{
		Fs:      inaccessibleChildFs{Fs: memFs, child: "/proton-mount"},
		Path:    "/",
		Expand:  true,
		Checker: allowAllChecker{},
	})
	if err != nil {
		t.Fatal(err)
	}

	if file.Listing == nil {
		t.Fatal("expected root listing")
	}

	if got := len(file.Items); got != 1 {
		t.Fatalf("expected one accessible child, got %d", got)
	}

	if got := file.Items[0].Name; got != "media" {
		t.Fatalf("expected accessible child to be listed, got %q", got)
	}

	if got := file.NumDirs; got != 1 {
		t.Fatalf("expected one listed directory, got %d", got)
	}
}

func TestFileInfoRealPathUsesScopedFsRealPath(t *testing.T) {
	root := t.TempDir()
	file := &FileInfo{
		Fs:   NewScopedFs(afero.NewOsFs(), root),
		Path: "/root/downloads",
	}

	got := file.RealPath()
	want := filepath.Join(root, "root", "downloads")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// 回归测试：".ts" 扩展名歧义。Windows 注册表 / 系统 MIME 表常把 .ts
// 映射为 video/mp2t（MPEG-TS 视频），导致 TypeScript 源码（甚至刚创建
// 的空 .ts 文件）被误判为视频，前端用视频播放器打开随即报
// MEDIA_ERR_SRC_NOT_SUPPORTED。detectType 必须对 .ts 做内容嗅探：
// 只有真正的 MPEG-TS 同步字节模式（0x47 @ 每 188 字节）才算视频。
func TestTsExtensionClassification(t *testing.T) {
	// 无论宿主系统如何注册 .ts，统一强制为 video/mp2t 以复现歧义场景。
	// （仅影响当前测试进程的 MIME 表。）
	if err := mime.AddExtensionType(".ts", "video/mp2t"); err != nil {
		t.Fatalf("failed to register .ts mimetype: %v", err)
	}

	newTsFile := func(t *testing.T, content []byte) *FileInfo {
		t.Helper()
		root := t.TempDir()
		name := "script.ts"
		if err := os.WriteFile(filepath.Join(root, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
		return &FileInfo{
			Fs:        afero.NewOsFs(),
			Path:      filepath.Join(root, name),
			Name:      name,
			Extension: ".ts",
			Size:      int64(len(content)),
		}
	}

	t.Run("typeScript source is text, not video", func(t *testing.T) {
		file := newTsFile(t, []byte("const answer: number = 42;\nexport default answer;\n"))
		if err := file.detectType(true, false, true, false); err != nil {
			t.Fatal(err)
		}
		if file.Type != "text" {
			t.Fatalf("expected TypeScript source to be %q, got %q", "text", file.Type)
		}
	})

	t.Run("empty new .ts file is text, not video", func(t *testing.T) {
		file := newTsFile(t, nil)
		if err := file.detectType(true, false, true, false); err != nil {
			t.Fatal(err)
		}
		if file.Type != "text" {
			t.Fatalf("expected empty .ts file to be %q, got %q", "text", file.Type)
		}
	})

	t.Run("real MPEG-TS stream stays video", func(t *testing.T) {
		// 构造 512 字节的 MPEG-TS 头部：0x47 同步字节位于
		// 0 / 188 / 376 偏移处，其余字节填充 0x00。
		content := make([]byte, 512)
		for _, offset := range []int{0, 188, 376} {
			content[offset] = 0x47
		}
		file := newTsFile(t, content)
		if err := file.detectType(true, false, true, false); err != nil {
			t.Fatal(err)
		}
		if file.Type != "video" {
			t.Fatalf("expected MPEG-TS stream to be %q, got %q", "video", file.Type)
		}
	})
}

func TestLooksLikeMpegTS(t *testing.T) {
	cases := []struct {
		name  string
		bytes []byte
		want  bool
	}{
		{"nil buffer", nil, false},
		{"short buffer", []byte{0x47, 0x00}, false},
		{"plain text", bytes.Repeat([]byte("const a = 1;\n"), 32), false},
		{"sync byte only at offset 0", append([]byte{0x47}, make([]byte, 300)...), false},
	}

	// 真 MPEG-TS：188 字节数据包，同步字节在包头。
	realTS := make([]byte, 188*3)
	for offset := 0; offset < len(realTS); offset += 188 {
		realTS[offset] = 0x47
	}
	cases = append(cases, struct {
		name  string
		bytes []byte
		want  bool
	}{"three synced packets", realTS, true})

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeMpegTS(tt.bytes); got != tt.want {
				t.Fatalf("looksLikeMpegTS(%v…) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
