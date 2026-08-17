package files

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/afero"
)

// MountOverlayFs 是一个 afero.Fs 的组合层：
//
//   - 有一个「主 Fs」对应 server.Root（所有非挂载路径都走它，保持原有 ScopedFs 安全边界）
//   - 叠加了若干个顶层「虚拟目录」：/MountName/... 会转发到对应的独立 ScopedFs(物理路径)
//
// 设计目的：保持 filebrowser 原生 --root 行为不变，同时可以把多个本地/UNC 目录
// "挂到"首页下面当虚拟子目录（相当于多根目录），而且 ScopedFs 的安全边界
// 对每个挂载点仍然独立生效（不会因为 symlink / .. 逃出各自的物理根）。
//
// 虚拟挂载点只出现在一级目录；任何路径只要首段命中挂载名就转发，否则继续走主 Fs。
type MountOverlayFs struct {
	base   afero.Fs              // 主 Fs（通常是 ScopedFs(Root)）
	mounts map[string]*mountInfo // name → ScopedFs(realPath)

	mu sync.RWMutex // 保护 mountFakeRootStat 单例
	// 虚拟挂载目录本身需要对外"看起来存在"（Stat("/挂载名")、列表 root 时），
	// 但它们没有主 Fs 下的真实 inode。这里缓存一个 fakeDirInfo 的指针作为 root 的替身，
	// 每次传入不同的 name 再包装。
}

type mountInfo struct {
	fs       afero.Fs // 独立的 ScopedFs，自带安全边界
	realPath string   // 原始物理路径（用于日志 / 错误信息）
}

var (
	_ afero.Fs      = (*MountOverlayFs)(nil)
	_ afero.Lstater = (*MountOverlayFs)(nil)
)

// NewMountOverlayFs 组合主 Fs 与若干挂载点。
// mounts: 虚拟名 → 物理绝对路径。Name 必须是纯文件名（调用方已校验）。
func NewMountOverlayFs(base afero.Fs, mounts map[string]string, source afero.Fs) *MountOverlayFs {
	m := make(map[string]*mountInfo, len(mounts))
	if source == nil {
		source = afero.NewOsFs()
	}
	for name, realPath := range mounts {
		m[name] = &mountInfo{
			fs:       NewScopedFs(source, realPath),
			realPath: realPath,
		}
	}
	return &MountOverlayFs{base: base, mounts: m}
}

// splitMount 把一个 afero 路径（Unix 形式 "/foo/bar" 或 Windows 形式 "\\foo\\bar" 或
// foo/bar/baz.pdf）统一切成「首段挂载名」和「挂载点内相对路径」。
//
// 注意：afero.Walk 在 Windows 内部会用 filepath.Join 拼路径，传进来的 name 会带
// 反斜杠；而虚拟目录名匹配、主 Fs 分发都必须用 "/"。这里先 filepath.ToSlash 再处理，
// 避免跨平台路径分隔符不一致导致路由失败（之前 bug：虚拟挂载下 Walk 不到内部文件）。
//
// 如果首段不是已知挂载点，则 mountName=""，rest 保持清理好的以 / 开头的相对路径。
func (m *MountOverlayFs) splitMount(name string) (mountName string, rest string) {
	clean := path.Clean("/" + filepath.ToSlash(name))
	if clean == "/" || clean == "." {
		return "", clean
	}
	clean = strings.TrimPrefix(clean, "/")
	firstEnd := strings.IndexByte(clean, '/')
	if firstEnd < 0 {
		first := clean
		if _, ok := m.mounts[first]; ok {
			return first, "/"
		}
		return "", "/" + clean
	}
	first := clean[:firstEnd]
	if _, ok := m.mounts[first]; ok {
		rel := clean[firstEnd:]
		if rel == "" {
			rel = "/"
		}
		return first, rel
	}
	return "", "/" + clean
}

// SplitMount 公开别名，方便外部（FullPath）把用户路径按当前挂载点拆分。
func (m *MountOverlayFs) SplitMount(name string) (mountName string, rest string) {
	return m.splitMount(name)
}

// MountBase 返回指定挂载点的底层 BasePathFs（用来把语义路径转成物理磁盘路径）。
func (m *MountOverlayFs) MountBase(name string) (*afero.BasePathFs, error) {
	mi, ok := m.mounts[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	if hb, ok := mi.fs.(interface{ Base() *afero.BasePathFs }); ok {
		return hb.Base(), nil
	}
	return nil, fmt.Errorf("mount %q does not expose a BasePathFs", name)
}

// Base 返回 MountOverlayFs 的主 Fs（通常是 Root 对应的 ScopedFs）。
// 当用户 Fs 被重新套一层 ScopedFs（如 share rebase）时，外层代码可通过
// Base() 解包回到 Root 主 Fs，避免 MountOverlayFs 与 ScopedFs 的互相嵌套。
func (m *MountOverlayFs) Base() afero.Fs { return m.base }

// MountNames 当前注册的挂载点名，按字典序返回。
func (m *MountOverlayFs) MountNames() []string {
	names := make([]string, 0, len(m.mounts))
	for n := range m.mounts {
		names = append(names, n)
	}
	sortStrings(names)
	return names
}

func sortStrings(a []string) {
	// 懒得引 sort 包，手写稳定冒泡（n 通常 < 20，挂载点数量极少，可忽略）。
	for i := 0; i < len(a); i++ {
		for j := i + 1; j < len(a); j++ {
			if a[i] > a[j] {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}

// --- helpers for fake dir info ----------------------------------------------

type fakeMountDirInfo struct {
	name string
}

func (f *fakeMountDirInfo) Name() string       { return f.name }
func (f *fakeMountDirInfo) Size() int64        { return 0 }
func (f *fakeMountDirInfo) Mode() os.FileMode  { return os.ModeDir | 0o755 }
func (f *fakeMountDirInfo) ModTime() time.Time { return time.Time{} }
func (f *fakeMountDirInfo) IsDir() bool        { return true }
func (f *fakeMountDirInfo) Sys() interface{}   { return nil }

// pick 根据路径选择要走的真实 Fs + 子路径。
func (m *MountOverlayFs) pick(name string) (afero.Fs, string) {
	mountName, rest := m.splitMount(name)
	if mountName != "" {
		return m.mounts[mountName].fs, rest
	}
	return m.base, rest
}

// --- afero.Fs 接口 ---------------------------------------------------------

func (m *MountOverlayFs) Name() string { return "MountOverlayFs" }

func (m *MountOverlayFs) Create(name string) (afero.File, error) {
	fs, p := m.pick(name)
	return fs.Create(p)
}

func (m *MountOverlayFs) Mkdir(name string, perm os.FileMode) error {
	// Mkdir("/foo") 这种路径：若 foo 是挂载点虚拟目录名则拒绝（虚拟目录本身不可被 Mkdir 覆盖）
	if mount, _ := m.splitMount(name); mount != "" && path.Clean("/"+name) == "/"+mount {
		return os.ErrPermission
	}
	fs, p := m.pick(name)
	return fs.Mkdir(p, perm)
}

func (m *MountOverlayFs) MkdirAll(name string, perm os.FileMode) error {
	if mount, _ := m.splitMount(name); mount != "" && path.Clean("/"+name) == "/"+mount {
		return os.ErrPermission
	}
	fs, p := m.pick(name)
	return fs.MkdirAll(p, perm)
}

func (m *MountOverlayFs) Open(name string) (afero.File, error) {
	mountName, rest := m.splitMount(name)
	if mountName != "" {
		f, err := m.mounts[mountName].fs.Open(rest)
		return f, err
	}
	// Open("/") 或 "/."：主 Fs 打开根目录，然后 Readdir/Stat 返回需要额外补挂载项
	// 这里直接打开主 Fs 的根，由 mountOverlayFile 包装结果。
	f, err := m.base.Open(rest)
	if err != nil {
		return nil, err
	}
	if rest == "/" || rest == "." || rest == "" {
		return &mountOverlayRootFile{File: f, overlay: m, readDone: false}, nil
	}
	return f, nil
}

func (m *MountOverlayFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	mountName, rest := m.splitMount(name)
	if mountName != "" {
		return m.mounts[mountName].fs.OpenFile(rest, flag, perm)
	}
	f, err := m.base.OpenFile(rest, flag, perm)
	if err != nil {
		return nil, err
	}
	if rest == "/" || rest == "." || rest == "" {
		return &mountOverlayRootFile{File: f, overlay: m, readDone: false}, nil
	}
	return f, nil
}

func (m *MountOverlayFs) Remove(name string) error {
	if mount, _ := m.splitMount(name); mount != "" && path.Clean("/"+name) == "/"+mount {
		return os.ErrPermission
	}
	fs, p := m.pick(name)
	return fs.Remove(p)
}

func (m *MountOverlayFs) RemoveAll(name string) error {
	if mount, _ := m.splitMount(name); mount != "" && path.Clean("/"+name) == "/"+mount {
		return os.ErrPermission
	}
	fs, p := m.pick(name)
	return fs.RemoveAll(p)
}

func (m *MountOverlayFs) Rename(oldname, newname string) error {
	om, op := m.splitMount(oldname)
	nm, np := m.splitMount(newname)
	// 跨挂载点 / 跨虚拟与主 Fs 的 Rename 不支持，真实 os.Rename 跨卷也会失败，保持一致
	if om != nm {
		return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: os.ErrInvalid}
	}
	var fs afero.Fs
	if om != "" {
		fs = m.mounts[om].fs
	} else {
		fs = m.base
	}
	return fs.Rename(op, np)
}

func (m *MountOverlayFs) Stat(name string) (os.FileInfo, error) {
	clean := path.Clean("/" + filepath.ToSlash(name))
	mount, rest := m.splitMount(name)
	if mount != "" && clean != "/" {
		// rest == "/" 说明是 Stat("虚拟挂载目录自身") → 需要把 Name() 重写为挂载名，
		// 但其他字段（ModTime 等）取真实挂载根的信息；如果真实 UNC 掉线，
		// 仍然返回一个假的 DirInfo，保持首页目录列表不崩。
		if rest == "/" {
			info, err := m.mounts[mount].fs.Stat(rest)
			if err != nil {
				return &fakeMountDirInfo{name: mount}, nil
			}
			return renameFileInfo{FileInfo: info, rename: mount}, nil
		}
		// 其余情况（挂载点内的文件/子目录）：直接透传 mountFs 的 Stat 结果，
		// 不改名、不吞掉 ErrNotExist（之前 bug：把 aa.pdf Stat 的 Name 也改成了挂载名，
		// 且不存在的文件被替换成 fakeDirInfo，导致 search.Search 逻辑完全错乱）。
		return m.mounts[mount].fs.Stat(rest)
	}
	return m.base.Stat(clean)
}

func (m *MountOverlayFs) Chmod(name string, mode os.FileMode) error {
	fs, p := m.pick(name)
	return fs.Chmod(p, mode)
}

func (m *MountOverlayFs) Chown(name string, uid, gid int) error {
	fs, p := m.pick(name)
	return fs.Chown(p, uid, gid)
}

func (m *MountOverlayFs) Chtimes(name string, atime, mtime time.Time) error {
	fs, p := m.pick(name)
	return fs.Chtimes(p, atime, mtime)
}

// LstatIfPossible：正确把任何「属于挂载点」的路径分发到对应挂载 Fs，
// 只有挂载根自身（rest=="/"）需要做 Name 重写，其他路径直接透传挂载 Fs 的结果。
// 之前 bug：只处理 clean == "/"+mount 的挂载根本身，内部子路径都错误地走到了 base Fs，
// 导致 afero.Walk 时 lstat 失败 → FileInfo=nil → search 回调 f.IsDir() panic。
func (m *MountOverlayFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	clean := path.Clean("/" + filepath.ToSlash(name))
	mount, rest := m.splitMount(name)
	if mount != "" && clean != "/" {
		if lst, ok := m.mounts[mount].fs.(afero.Lstater); ok {
			info, link, err := lst.LstatIfPossible(rest)
			if err == nil && rest == "/" {
				// 虚拟挂载目录自身，把 Name() 改写为挂载名
				info = renameFileInfo{FileInfo: info, rename: mount}
			} else if err != nil && rest == "/" {
				// UNC 掉线时保持虚拟目录仍可 Lstat
				return &fakeMountDirInfo{name: mount}, true, nil
			}
			return info, link, err
		}
		// 挂载 Fs 没有 Lstater：退回 Stat
		info, err := m.mounts[mount].fs.Stat(rest)
		if err == nil && rest == "/" {
			info = renameFileInfo{FileInfo: info, rename: mount}
		} else if err != nil && rest == "/" {
			return &fakeMountDirInfo{name: mount}, false, nil
		}
		return info, false, err
	}
	if lst, ok := m.base.(afero.Lstater); ok {
		return lst.LstatIfPossible(clean)
	}
	info, err := m.base.Stat(clean)
	return info, false, err
}

// --- root file wrapper: 在根目录 Readdir(-1) 的结果后追加挂载点 DirInfo ---

type mountOverlayRootFile struct {
	afero.File
	overlay  *MountOverlayFs
	readDone bool
}

func (f *mountOverlayRootFile) Readdir(count int) ([]os.FileInfo, error) {
	// 只在第一次（count <=0 或 count>0 尚未 read）时读一次底层 + 追加入口
	var base []os.FileInfo
	var err error
	if !f.readDone {
		base, err = f.File.Readdir(-1)
		f.readDone = true
		if err != nil && len(base) == 0 {
			return nil, err
		}
		// 追加挂载点（过滤掉和物理真实目录同名的冲突：真实目录优先，避免重复）
		existing := make(map[string]struct{}, len(base))
		for _, fi := range base {
			existing[fi.Name()] = struct{}{}
		}
		for _, name := range f.overlay.MountNames() {
			if _, dup := existing[name]; dup {
				continue
			}
			// 优先用真实挂载目录的 Stat，能拿到正确 ModTime / Size / 权限
			// 失败再退回 fake
			var info os.FileInfo
			if mi, ok := f.overlay.mounts[name]; ok {
				if raw, e := mi.fs.Stat("/"); e == nil {
					info = renameFileInfo{FileInfo: raw, rename: name}
				} else {
					info = &fakeMountDirInfo{name: name}
				}
			}
			base = append(base, info)
		}
	}
	if count <= 0 {
		return base, nil
	}
	if count >= len(base) {
		return base, io.EOF
	}
	return base[:count], nil
}

func (f *mountOverlayRootFile) Readdirnames(count int) ([]string, error) {
	infos, err := f.Readdir(count)
	names := make([]string, 0, len(infos))
	for _, fi := range infos {
		names = append(names, fi.Name())
	}
	return names, err
}

// renameFileInfo 把一个底层 FileInfo 的 Name() 重写为指定字符串，其他字段不变。
type renameFileInfo struct {
	os.FileInfo
	rename string
}

func (r renameFileInfo) Name() string { return r.rename }
