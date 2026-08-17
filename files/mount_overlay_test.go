package files

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/search"
)

// 一个允许所有路径通过的 rules.Checker，用于单元测试。
type passChecker struct{}

func (passChecker) Check(string) bool { return true }

var _ rules.Checker = passChecker{}

// setupOverlayFs 造一个最小 MountOverlayFs，用于本地单测。
// ScopedFs 内部的安全边界（within / fastWithin / resolvedRoot）会直接走 OS
// (os.Lstat / filepath.EvalSymlinks)，因此这里必须用 afero.NewOsFs() + 真实临时目录，
// 不能用 MemFs，否则 ScopedFs 守卫会去查询不存在的 OS 路径报错。
func setupOverlayFs(t *testing.T) (*MountOverlayFs, afero.Fs, map[string]string) {
	t.Helper()
	tmp := t.TempDir()

	OsFs := afero.NewOsFs()

	// --- main ---
	mainRoot := filepath.Join(tmp, "main")
	if err := OsFs.MkdirAll(mainRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll mainRoot: %v", err)
	}
	mainFs := NewScopedFs(OsFs, mainRoot)

	writeFs(t, OsFs, filepath.Join(mainRoot, "hello.pdf"), "main-hello")
	mkdirFs(t, OsFs, filepath.Join(mainRoot, "sub"))
	writeFs(t, OsFs, filepath.Join(mainRoot, "sub", "world.pdf"), "main-world")

	// --- mountA ---
	aRoot := filepath.Join(tmp, "a")
	if err := OsFs.MkdirAll(aRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll aRoot: %v", err)
	}
	aFs := NewScopedFs(OsFs, aRoot)
	_ = aFs
	writeFs(t, OsFs, filepath.Join(aRoot, "aa.pdf"), "a-aa")
	mkdirFs(t, OsFs, filepath.Join(aRoot, "sub"))
	writeFs(t, OsFs, filepath.Join(aRoot, "sub", "bb.pdf"), "a-bb")

	// --- mountB ---
	bRoot := filepath.Join(tmp, "b")
	if err := OsFs.MkdirAll(bRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll bRoot: %v", err)
	}
	bFs := NewScopedFs(OsFs, bRoot)
	_ = bFs
	writeFs(t, OsFs, filepath.Join(bRoot, "cc.pdf"), "b-cc")

	mounts := map[string]string{
		"MountA": aRoot,
		"MountB": bRoot,
	}

	overlay := NewMountOverlayFs(mainFs, mounts, OsFs)
	return overlay, OsFs, mounts
}

func writeFs(t *testing.T, fs afero.Fs, path, content string) {
	t.Helper()
	if err := afero.WriteFile(fs, path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func mkdirFs(t *testing.T, fs afero.Fs, path string) {
	t.Helper()
	if err := fs.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", path, err)
	}
}

// TestMountOverlayFs_StatAndLstat 复现线上 502 的直接根因：
// LstatIfPossible 没转发到挂载 Fs，导致 /MountA/aa.pdf 的 lstat 走了主 Fs
// 返回 ErrNotExist，afero.Walk 传 nil FileInfo 给 WalkFn，search 回调 f.IsDir() 就 panic。
func TestMountOverlayFs_StatAndLstat(t *testing.T) {
	ov, _, _ := setupOverlayFs(t)

	cases := []struct {
		path     string
		wantName string
		wantErr  error
	}{
		{path: "/", wantErr: nil},
		{path: "/hello.pdf", wantName: "hello.pdf"},
		{path: "/sub/world.pdf", wantName: "world.pdf"},
		// 挂载根自身
		{path: "/MountA", wantName: "MountA"},
		{path: "/MountB", wantName: "MountB"},
		// 挂载内文件 — 这几行之前会 fail，因为 Lstat 跑错了 Fs
		{path: "/MountA/aa.pdf", wantName: "aa.pdf"},
		{path: "/MountA/sub/bb.pdf", wantName: "bb.pdf"},
		{path: "/MountB/cc.pdf", wantName: "cc.pdf"},
		// 不存在
		{path: "/MountA/does-not-exist.pdf", wantErr: os.ErrNotExist},
		{path: "/MountA/does/not/exist/deep", wantErr: os.ErrNotExist},
	}

	for _, c := range cases {
		c := c
		t.Run("Stat:"+c.path, func(t *testing.T) {
			info, err := ov.Stat(c.path)
			if c.wantErr != nil {
				if err == nil {
					t.Fatalf("want err %v, got nil (info=%+v)", c.wantErr, info)
				}
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("err mismatch: want Is(%v), got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Stat err: %v", err)
			}
			if c.wantName != "" && info.Name() != c.wantName {
				t.Fatalf("Name: want %q, got %q", c.wantName, info.Name())
			}
		})

		t.Run("LstatIfPossible:"+c.path, func(t *testing.T) {
			info, _, err := ov.LstatIfPossible(c.path)
			if c.wantErr != nil {
				if err == nil {
					t.Fatalf("want err %v, got nil (info=%+v)", c.wantErr, info)
				}
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("err mismatch: want Is(%v), got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("LstatIfPossible err: %v", err)
			}
			if c.wantName != "" && info.Name() != c.wantName {
				t.Fatalf("Name: want %q, got %q", c.wantName, info.Name())
			}
		})
	}
}

// TestMountOverlayFs_WalkSearch 走完整 search.Search 链路，
// 专门复现「在虚拟挂载点目录下搜索时 f=nil → panic」的问题。
func TestMountOverlayFs_WalkSearch(t *testing.T) {
	ov, _, _ := setupOverlayFs(t)

	// 模拟用户点进了虚拟目录 /MountA，URL 中 scope 是 /MountA。
	// 这是线上 502 的精确场景：scope 本身是挂载点。
	foundMap := map[string]bool{}
	var panicked interface{}
	func() {
		defer func() {
			panicked = recover()
		}()
		err := search.Search(context.Background(), ov, "/MountA", "aa", passChecker{},
			func(p string, f os.FileInfo) error {
				// 真实 searchHandler 在这里会调用 f.IsDir() / f.Name()
				// 任何 f=nil 的情况都会 panic
				_ = f.IsDir()
				_ = f.Name()
				foundMap[p] = true
				return nil
			})
		if err != nil {
			t.Fatalf("search.Search err: %v", err)
		}
	}()
	if panicked != nil {
		t.Fatalf("search.Search panicked: %v", panicked)
	}
	if !foundMap["/MountA/aa.pdf"] {
		t.Errorf("expected to find /MountA/aa.pdf, got %+v", foundMap)
	}

	// 再换一个 scope 全量搜索（空 query 不过滤任何文件名，直接 found 每个条目）
	t.Run("walk from root / (empty query)", func(t *testing.T) {
		var panicked interface{}
		all := map[string]bool{}
		func() {
			defer func() { panicked = recover() }()
			err := search.Search(context.Background(), ov, "/", "", passChecker{},
				func(p string, f os.FileInfo) error {
					_ = f.IsDir()
					all[p] = true
					return nil
				})
			if err != nil {
				t.Fatalf("search / err: %v", err)
			}
		}()
		if panicked != nil {
			t.Fatalf("search from root panicked: %v", panicked)
		}
		// 主 Fs: hello.pdf / sub/world.pdf（sub 是目录，会被 walk 到，但 found 回调
		// 是「每个非 scope 节点」，目录本身也会触发 found）
		// MountA: aa.pdf / sub/bb.pdf
		// MountB: cc.pdf
		// 共 5 个非目录 PDF + 3 个目录 sub（每个 root 一个）？不，MountA/sub 也一个、主 sub 一个、MountB 没有 sub。
		// 总之至少要包含 3 个挂载内 pdf（aa, bb, cc）+ 2 个主 pdf（hello, world）
		mustContain := []string{
			"/MountA/aa.pdf",
			"/MountA/sub/bb.pdf",
			"/MountB/cc.pdf",
			"/hello.pdf",
			"/sub/world.pdf",
		}
		for _, p := range mustContain {
			if !all[p] {
				t.Errorf("missing %q in walked set; got %v", p, sortedKeys(all))
			}
		}
	})
}

// TestMountOverlayFs_RootReaddir_MountNamesMerged 确保首页列表把 MountA / MountB
// 两个虚拟子目录一起列出来。
func TestMountOverlayFs_RootReaddir_MountNamesMerged(t *testing.T) {
	ov, _, _ := setupOverlayFs(t)

	f, err := ov.Open("/")
	if err != nil {
		t.Fatalf("Open /: %v", err)
	}
	defer f.Close()

	infos, err := f.Readdir(-1)
	if err != nil {
		t.Fatalf("Readdir /: %v", err)
	}

	names := map[string]bool{}
	for _, fi := range infos {
		names[fi.Name()] = true
	}
	for _, want := range []string{"hello.pdf", "sub", "MountA", "MountB"} {
		if !names[want] {
			t.Errorf("Readdir missing %q; got=%v", want, mapKeys(names))
		}
	}
}

func mapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	ks := mapKeys(m)
	// 简单冒泡（n<50，无性能问题），避免引入 sort 包
	for i := 0; i < len(ks); i++ {
		for j := i + 1; j < len(ks); j++ {
			if ks[i] > ks[j] {
				ks[i], ks[j] = ks[j], ks[i]
			}
		}
	}
	return ks
}

// TestMountOverlayFs_OpenFileInsideMount 直接打开 /MountA/aa.pdf 能读出内容（非空）。
func TestMountOverlayFs_OpenFileInsideMount(t *testing.T) {
	ov, _, _ := setupOverlayFs(t)

	f, err := ov.Open("/MountA/aa.pdf")
	if err != nil {
		t.Fatalf("Open MountA/aa.pdf: %v", err)
	}
	defer f.Close()
	buf := make([]byte, 10)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("Read: %v", err)
	}
	if n == 0 {
		t.Fatalf("got 0 bytes, expected some content")
	}
	if runtime.GOOS != "" {
		// 静默避免 unused
		_ = strings.Contains
	}
}
