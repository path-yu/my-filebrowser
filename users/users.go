package users

import (
	"fmt"
	"path/filepath"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	fbfiles "github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/spf13/afero"
)

// ViewMode describes a view mode.
type ViewMode string

const (
	ListViewMode   ViewMode = "list"
	MosaicViewMode ViewMode = "mosaic"
)

// User describes a user.
type User struct {
	ID                    uint            `storm:"id,increment" json:"id"`
	Username              string          `storm:"unique" json:"username"`
	Password              string          `json:"password"`
	Scope                 string          `json:"scope"`
	Locale                string          `json:"locale"`
	LockPassword          bool            `json:"lockPassword"`
	ViewMode              ViewMode        `json:"viewMode"`
	SingleClick           bool            `json:"singleClick"`
	RedirectAfterCopyMove bool            `json:"redirectAfterCopyMove"`
	Perm                  Permissions     `json:"perm"`
	Commands              []string        `json:"commands"`
	Sorting               fbfiles.Sorting `json:"sorting"`
	// Fs 存当前用户用的「文件系统抽象」，之前只用 ScopedFs，现在支持 MountOverlayFs（多根目录挂载）、
	// public share 再套一层 ScopedFs 等组合，因此改为 afero.Fs 接口。
	// 所有需要 ScopedFs 的地方（比如 public.go share rebasing），统一用接口断言处理。
	Fs                    afero.Fs        `json:"-" yaml:"-"`
	Rules                 []rules.Rule    `json:"rules"`
	HideDotfiles          bool            `json:"hideDotfiles"`
	DateFormat            bool            `json:"dateFormat"`
	AceEditorTheme        string          `json:"aceEditorTheme"`
}

// GetRules implements rules.Provider.
func (u *User) GetRules() []rules.Rule {
	return u.Rules
}

var checkableFields = []string{
	"Username",
	"Password",
	"Scope",
	"ViewMode",
	"Commands",
	"Sorting",
	"Rules",
}

// Clean cleans up a user and verifies if all its fields
// are alright to be saved.
func (u *User) Clean(baseScope string, fields ...string) error {
	if len(fields) == 0 {
		fields = checkableFields
	}

	for _, field := range fields {
		switch field {
		case "Username":
			if u.Username == "" {
				return fberrors.ErrEmptyUsername
			}
		case "Password":
			if u.Password == "" {
				return fberrors.ErrEmptyPassword
			}
		case "ViewMode":
			if u.ViewMode == "" {
				u.ViewMode = ListViewMode
			}
		case "Commands":
			if u.Commands == nil {
				u.Commands = []string{}
			}
		case "Sorting":
			if u.Sorting.By == "" {
				u.Sorting.By = "name"
			}
		case "Rules":
			if u.Rules == nil {
				u.Rules = []rules.Rule{}
			}
		}
	}

	if u.Fs == nil {
		scope := u.Scope
		scope = filepath.Join(baseScope, filepath.Join("/", scope))
		u.Fs = fbfiles.NewScopedFs(afero.NewOsFs(), scope)
	}

	return nil
}

// FsBase 尽量把 u.Fs 解包到底层 *BasePathFs，用来获取真实磁盘路径。
// 顺序：MountOverlayFs → Base；ScopedFs → Base；其他 afero.Fs → 报错。
func (u *User) FsBase() (*afero.BasePathFs, error) {
	type hasBase interface{ Base() *afero.BasePathFs }
	switch fs := u.Fs.(type) {
	case nil:
		return nil, fmt.Errorf("user.Fs is nil")
	case *fbfiles.ScopedFs:
		return fs.Base(), nil
	case *fbfiles.MountOverlayFs:
		// 注意：MountOverlayFs 里不同虚拟路径对应不同物理根，这里只能返回"主 Fs"的 Base。
		// 调用方如果是在挂载子路径下，应该先手动分发到对应挂载点的 Base（目前
		// FullPath 主要用于主目录路径，故退回主 Fs 是合理兜底）。
		if scoped, ok := fs.Base().(*fbfiles.ScopedFs); ok {
			return scoped.Base(), nil
		}
		if hb, ok := fs.Base().(hasBase); ok {
			return hb.Base(), nil
		}
		return nil, fmt.Errorf("MountOverlayFs base is not ScopedFs: %T", fs.Base())
	case hasBase:
		return fs.Base(), nil
	}
	return nil, fmt.Errorf("user.Fs type %T has no BasePathFs accessor", u.Fs)
}

// FullPath gets the full path for a user's relative path.
// 如果是虚拟挂载点下的路径，会把挂载名替换为对应的物理根。
func (u *User) FullPath(p string) string {
	if mfs, ok := u.Fs.(*fbfiles.MountOverlayFs); ok {
		if mount, rest := mfs.SplitMount(p); mount != "" {
			// rest 是挂载点内部的相对路径（含前导 /）
			// 对应物理路径 = 挂载根 + rest
			if base, err := mfs.MountBase(mount); err == nil {
				return afero.FullBaseFsPath(base, rest)
			}
		}
	}
	base, err := u.FsBase()
	if err != nil {
		// 最后兜底：主 Fs 上的语义路径
		return p
	}
	return afero.FullBaseFsPath(base, p)
}
