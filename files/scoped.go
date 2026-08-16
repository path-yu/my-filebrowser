package files

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/afero"
)

// ScopedFs is an afero.Fs that confines every operation to a base directory and
// refuses to follow a symbolic link whose on-disk target resolves outside that
// base. It wraps an *afero.BasePathFs — which already provides the lexical
// confinement — and adds a per-operation scope check on every call that would
// dereference a symlink at the OS layer (open, stat, lstat, chmod, …).
type ScopedFs struct {
	base *afero.BasePathFs

	rootMu     sync.Mutex
	rootCached bool
	rootReal   string // on-disk resolved scope root, cached after first success
}

var (
	_ afero.Fs      = (*ScopedFs)(nil)
	_ afero.Lstater = (*ScopedFs)(nil)
)

func NewScopedFs(source afero.Fs, path string) *ScopedFs {
	if s, ok := source.(*ScopedFs); ok {
		source = s.base
	}
	return &ScopedFs{base: afero.NewBasePathFs(source, path).(*afero.BasePathFs)}
}

// Base returns the underlying *afero.BasePathFs.
func (s *ScopedFs) Base() *afero.BasePathFs { return s.base }

// RealPath resolves a scoped path to the real on-disk path by delegating to
// the underlying BasePathFs. This is needed by callers that need the actual
// filesystem path (e.g. disk.UsageWithContext).
func (s *ScopedFs) RealPath(name string) (string, error) {
	return s.base.RealPath(name)
}

// guard returns an error if name's on-disk target resolves outside the scope.
func (s *ScopedFs) guard(name string) error {
	ok, err := s.within(name)
	if err != nil {
		return err
	}
	if !ok {
		return os.ErrPermission
	}
	return nil
}

// resolvedRoot returns the on-disk path of the scope root with all symbolic
// links resolved. The result is cached: the scope root is fixed for the
// lifetime of the ScopedFs, and resolving it on every guard call dominated the
// cost of every filesystem operation (two full EvalSymlinks walks per call).
// A failed resolution is not cached, so a root that appears later still works.
func (s *ScopedFs) resolvedRoot() (string, error) {
	s.rootMu.Lock()
	defer s.rootMu.Unlock()
	if s.rootCached {
		return s.rootReal, nil
	}
	root, err := filepath.EvalSymlinks(afero.FullBaseFsPath(s.base, "/"))
	if err != nil {
		return "", err
	}
	s.rootReal = root
	s.rootCached = true
	return root, nil
}

// within reports whether the on-disk target of p — after resolving any symbolic
// links — stays within the scoped root. It exists to stop a symlink that lives
// lexically inside the scope but points outside it from being followed for
// reads, writes, or shares.
//
// Paths that do not exist yet (e.g. a brand-new file being created) are
// validated against their nearest existing ancestor, so legitimate new files
// are always allowed.
//
// Note: a dangling symlink whose target does not yet exist resolves to its
// containing directory and is therefore allowed; writing through such a link
// could still create a file outside the scope. This is treated as best-effort
// and relies on rejecting existing escaping symlinks, which covers the
// disclosure and overwrite vectors.
func (s *ScopedFs) within(p string) (bool, error) {
	root, err := s.resolvedRoot()
	if err != nil {
		return false, err
	}

	target := afero.FullBaseFsPath(s.base, p)

	_, _, ok, err := fastWithin(root, target)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}

	// Slow path: the route contains a symbolic link (or the lexical target is
	// not even below the resolved root, e.g. after a ".." escape) — resolve
	// with the stdlib and compare against the root, exactly as before.
	resolved, err := filepath.EvalSymlinks(target)
	for errors.Is(err, fs.ErrNotExist) {
		parent := filepath.Dir(target)
		if parent == target {
			break
		}
		target = parent
		resolved, err = filepath.EvalSymlinks(target)
	}
	if err != nil {
		return false, err
	}

	// Compare against root with a trailing separator so a sibling like
	// "/srvother" is not treated as being inside "/srv". When root is itself the
	// filesystem boundary (e.g. "/"), it already ends in a separator, so avoid
	// producing "//" — which no path would match — and accept any path under it.
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}

	return resolved == root || strings.HasPrefix(resolved, prefix), nil
}

// fastWithin walks the components of target below the already-resolved scope
// root with os.Lstat. As long as every existing component is a real file or
// directory — not a symbolic link — the target cannot resolve outside the
// scope, so the expensive full symlink evaluation can be skipped entirely.
//
// It returns ok=true when target is proven to stay within the scope. In that
// case info carries the os.FileInfo of the final component when it exists (the
// Lstat of a proven non-symlink, i.e. identical to its Stat result), letting
// Stat/LstatIfPossible callers reuse it and save another round trip.
//
// It returns ok=false when a symlink is found anywhere on the route, or when
// target is not lexically below root (e.g. the root itself is reached through
// a symlink): the caller must then fall back to filepath.EvalSymlinks, which
// preserves the original semantics. Any other Lstat failure is returned as err.
func fastWithin(root, target string) (resolved string, info os.FileInfo, ok bool, err error) {
	if target == root {
		return target, nil, true, nil
	}

	var rest string
	switch {
	case strings.HasSuffix(root, string(filepath.Separator)) && strings.HasPrefix(target, root):
		// Root already ends in a separator (e.g. "/" or "D:\"): slicing after
		// it avoids producing a double separator.
		rest = target[len(root):]
	case strings.HasPrefix(target, root+string(filepath.Separator)):
		rest = target[len(root)+1:]
	default:
		// Not below the resolved root — fall back to full evaluation. This
		// also catches lexical escapes such as ".." leaving the scope, which
		// the prefix comparison in the slow path then rejects.
		return "", nil, false, nil
	}

	cur := root
	for _, comp := range strings.Split(rest, string(filepath.Separator)) {
		if comp == "" || comp == "." {
			continue
		}
		cur = filepath.Join(cur, comp)
		fi, lerr := os.Lstat(cur)
		if lerr != nil {
			if errors.Is(lerr, fs.ErrNotExist) {
				// The remaining components do not exist on disk, so none of
				// them can be a symlink; every existing component verified so
				// far is real and inside the scope.
				return cur, nil, true, nil
			}
			return "", nil, false, lerr
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return "", nil, false, nil
		}
		info = fi
	}
	return cur, info, true, nil
}

func (s *ScopedFs) Create(name string) (afero.File, error) {
	if err := s.guard(name); err != nil {
		return nil, err
	}
	return s.base.Create(name)
}

func (s *ScopedFs) Mkdir(name string, perm os.FileMode) error {
	if err := s.guard(name); err != nil {
		return err
	}
	return s.base.Mkdir(name, perm)
}

func (s *ScopedFs) MkdirAll(path string, perm os.FileMode) error {
	if err := s.guard(path); err != nil {
		return err
	}
	return s.base.MkdirAll(path, perm)
}

func (s *ScopedFs) Open(name string) (afero.File, error) {
	if err := s.guard(name); err != nil {
		return nil, err
	}
	return s.base.Open(name)
}

func (s *ScopedFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if err := s.guard(name); err != nil {
		return nil, err
	}
	return s.base.OpenFile(name, flag, perm)
}

func (s *ScopedFs) Remove(name string) error {
	return s.base.Remove(name)
}

func (s *ScopedFs) RemoveAll(path string) error {
	return s.base.RemoveAll(path)
}

func (s *ScopedFs) Rename(oldname, newname string) error {
	if err := s.guard(oldname); err != nil {
		return err
	}
	if err := s.guard(newname); err != nil {
		return err
	}
	return s.base.Rename(oldname, newname)
}

func (s *ScopedFs) Stat(name string) (os.FileInfo, error) {
	// Fast path: when every component below the scope root is proven real
	// (no symlinks), the final component's Lstat result is identical to its
	// Stat result, so the extra round trip through base.Stat is skipped.
	if root, err := s.resolvedRoot(); err == nil {
		if _, info, ok, ferr := fastWithin(root, afero.FullBaseFsPath(s.base, name)); ferr == nil && ok && info != nil {
			return info, nil
		}
	}
	if err := s.guard(name); err != nil {
		return nil, err
	}
	return s.base.Stat(name)
}

func (s *ScopedFs) Name() string { return "ScopedFs" }

func (s *ScopedFs) Chmod(name string, mode os.FileMode) error {
	if err := s.guard(name); err != nil {
		return err
	}
	return s.base.Chmod(name, mode)
}

func (s *ScopedFs) Chown(name string, uid, gid int) error {
	if err := s.guard(name); err != nil {
		return err
	}
	return s.base.Chown(name, uid, gid)
}

func (s *ScopedFs) Chtimes(name string, atime, mtime time.Time) error {
	if err := s.guard(name); err != nil {
		return err
	}
	return s.base.Chtimes(name, atime, mtime)
}

func (s *ScopedFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	// Fast path: the walk already performed the Lstat of the final component
	// while proving the route stays inside the scope — reuse it.
	if root, err := s.resolvedRoot(); err == nil {
		if _, info, ok, ferr := fastWithin(root, afero.FullBaseFsPath(s.base, name)); ferr == nil && ok && info != nil {
			return info, true, nil
		}
	}
	if err := s.guard(name); err != nil {
		return nil, false, err
	}
	return s.base.LstatIfPossible(name)
}
