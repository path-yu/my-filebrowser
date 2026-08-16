package bolt

import (
	"errors"
	"strings"

	"github.com/asdine/storm/v3"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/productcode"
)

type productCodeBackend struct {
	db *storm.DB
}

func (b productCodeBackend) Get(path string) (*productcode.Entry, error) {
	var v productcode.Entry
	err := b.db.One("Path", path, &v)
	if errors.Is(err, storm.ErrNotFound) {
		return nil, fberrors.ErrNotExist
	}
	return &v, err
}

// Save 写入（upsert）一条产品编号记录。
// storm 的 Save 在主键已存在时报 ErrAlreadyExists，Update 在不存在时报
// ErrNotFound，这里组合二者实现覆盖语义。
func (b productCodeBackend) Save(e *productcode.Entry) error {
	if err := b.db.Update(e); err != nil {
		if errors.Is(err, storm.ErrNotFound) {
			return b.db.Save(e)
		}
		return err
	}
	return nil
}

func (b productCodeBackend) Delete(path string) error {
	err := b.db.DeleteStruct(&productcode.Entry{Path: path})
	if errors.Is(err, storm.ErrNotFound) {
		return nil
	}
	return err
}

// FindByCodePrefix 按产品编号前缀匹配（大小写不敏感），用于“输入编号搜文件”。
func (b productCodeBackend) FindByCodePrefix(prefix string) ([]*productcode.Entry, error) {
	var all []*productcode.Entry
	if err := b.db.All(&all); err != nil {
		if errors.Is(err, storm.ErrNotFound) {
			return nil, fberrors.ErrNotExist
		}
		return nil, err
	}

	p := strings.ToLower(prefix)
	out := make([]*productcode.Entry, 0, len(all))
	for _, e := range all {
		if strings.HasPrefix(strings.ToLower(e.Code), p) {
			out = append(out, e)
		}
	}
	return out, nil
}

func (b productCodeBackend) All() ([]*productcode.Entry, error) {
	var v []*productcode.Entry
	err := b.db.All(&v)
	if errors.Is(err, storm.ErrNotFound) {
		return v, fberrors.ErrNotExist
	}
	return v, err
}
