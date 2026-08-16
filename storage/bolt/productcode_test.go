package bolt

import (
	"path/filepath"
	"testing"

	"github.com/asdine/storm/v3"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/productcode"
)

func newProductCodeStore(t *testing.T) *productcode.Storage {
	t.Helper()
	db, err := storm.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open storm: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return productcode.NewStorage(productCodeBackend{db: db})
}

func TestProductCodeStoreCRUD(t *testing.T) {
	s := newProductCodeStore(t)

	// 不存在 → ErrNotExist
	if _, err := s.Get("/a.pdf"); err != fberrors.ErrNotExist {
		t.Fatalf("expect ErrNotExist, got %v", err)
	}

	// 新增
	if err := s.Save(&productcode.Entry{Path: "/a.pdf", Code: "AB-1", UserID: 1, UpdatedAt: 1}); err != nil {
		t.Fatalf("save: %v", err)
	}
	e, err := s.Get("/a.pdf")
	if err != nil || e.Code != "AB-1" {
		t.Fatalf("get: err=%v code=%q", err, e.Code)
	}

	// 前缀反查（大小写不敏感）
	hits, err := s.FindByCodePrefix("ab")
	if err != nil || len(hits) != 1 || hits[0].Path != "/a.pdf" {
		t.Fatalf("FindByCodePrefix: err=%v hits=%+v", err, hits)
	}

	// 覆盖（upsert）
	if err := s.Save(&productcode.Entry{Path: "/a.pdf", Code: "CD-2", UserID: 1, UpdatedAt: 2}); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if hits, _ = s.FindByCodePrefix("ab"); len(hits) != 0 {
		t.Fatalf("expect no hits after overwrite, got %+v", hits)
	}
	if e, err = s.Get("/a.pdf"); err != nil || e.Code != "CD-2" {
		t.Fatalf("after overwrite: err=%v code=%q", err, e.Code)
	}

	// 删除 + 幂等删除
	if err := s.Delete("/a.pdf"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.Delete("/a.pdf"); err != nil {
		t.Fatalf("delete again should be no-op: %v", err)
	}
	if _, err := s.Get("/a.pdf"); err != fberrors.ErrNotExist {
		t.Fatalf("expect ErrNotExist after delete, got %v", err)
	}
}

func TestProductCodeStoreAll(t *testing.T) {
	s := newProductCodeStore(t)

	// 空表
	all, err := s.All()
	if err != nil && err != fberrors.ErrNotExist {
		t.Fatalf("all on empty: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expect empty, got %+v", all)
	}

	for i, code := range []string{"X-1", "X-2", "Y-9"} {
		if err := s.Save(&productcode.Entry{Path: "/" + code + ".pdf", Code: code, UserID: 1, UpdatedAt: int64(i)}); err != nil {
			t.Fatalf("save %s: %v", code, err)
		}
	}

	hits, err := s.FindByCodePrefix("X-")
	if err != nil || len(hits) != 2 {
		t.Fatalf("expect 2 hits for X-, err=%v hits=%+v", err, hits)
	}
}
