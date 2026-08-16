// Package productcode 提供 PDF 产品编号的“数据库 + 文件元数据”双写存储。
//
// 设计目标：
//   - 数据库（storm/bolt）承载 path → code 的索引，支持 O(1) 查询与按编号反查文件；
//   - PDF 文件本身的 Keywords 元数据同步写入产品编号，文件被下载/拷贝离开系统后
//     仍可通过任意 PDF 阅读器的“文档属性”看到产品编号（离线可追溯）。
package productcode

// Entry 是一条产品编号记录，主键为用户空间内的文件路径。
type Entry struct {
	Path      string `json:"path" storm:"id,index"` // 用户可见路径（URL 语义，如 /58/xxx.pdf）
	Code      string `json:"code" storm:"index"`    // 产品编号
	UserID    uint   `json:"userID"`                // 录入人
	UpdatedAt int64  `json:"updatedAt"`             // unix 秒
}

// StorageBackend 是产品编号存储后端需要实现的接口。
type StorageBackend interface {
	Get(path string) (*Entry, error)
	Save(e *Entry) error
	Delete(path string) error
	FindByCodePrefix(prefix string) ([]*Entry, error)
	All() ([]*Entry, error)
}

// Storage 是对外暴露的产品编号存储。
type Storage struct {
	back StorageBackend
}

// NewStorage 基于任意后端创建存储。
func NewStorage(back StorageBackend) *Storage {
	return &Storage{back: back}
}

// Get wraps StorageBackend.Get.
func (s *Storage) Get(path string) (*Entry, error) {
	return s.back.Get(path)
}

// Save wraps StorageBackend.Save.
func (s *Storage) Save(e *Entry) error {
	return s.back.Save(e)
}

// Delete wraps StorageBackend.Delete.
func (s *Storage) Delete(path string) error {
	return s.back.Delete(path)
}

// FindByCodePrefix wraps StorageBackend.FindByCodePrefix.
func (s *Storage) FindByCodePrefix(prefix string) ([]*Entry, error) {
	return s.back.FindByCodePrefix(prefix)
}

// All wraps StorageBackend.All.
func (s *Storage) All() ([]*Entry, error) {
	return s.back.All()
}
