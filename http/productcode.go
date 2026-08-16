package fbhttp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	gopath "path"
	"strings"
	"time"

	"github.com/spf13/afero"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/productcode"
)

const productCodeMaxLen = 128

// isPDFPath 判断给定路径是否为 PDF 文件（按扩展名，大小写不敏感）。
func isPDFPath(p string) bool {
	return strings.EqualFold(gopath.Ext(p), ".pdf")
}

func withPermModify(fn handleFunc) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Modify {
			return http.StatusForbidden, nil
		}
		return fn(w, r, d)
	})
}

// readPDFCode 从用户文件系统中的 PDF 读取元数据里的产品编号。
func readPDFCode(fsys afero.Fs, p string) (string, error) {
	f, err := fsys.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()

	rs, ok := f.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(f)
		if err != nil {
			return "", err
		}
		rs = bytes.NewReader(data)
	}
	return productcode.ReadCodeFromPDF(rs)
}

// writePDFCode 把产品编号写入 PDF 元数据，采用“临时文件 + 原子改名”，
// 失败时不会破坏原文件。
func writePDFCode(fsys afero.Fs, p string, code string) error {
	mode := os.FileMode(0o644)
	if info, err := fsys.Stat(p); err == nil {
		mode = info.Mode()
	}

	f, err := fsys.Open(p)
	if err != nil {
		return err
	}
	data, readErr := io.ReadAll(f)
	closeErr := f.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}

	var buf bytes.Buffer
	if err := productcode.WriteCodeToPDF(bytes.NewReader(data), &buf, code); err != nil {
		return err
	}

	tmp, err := afero.TempFile(fsys, gopath.Dir(p), ".productcode-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		fsys.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		fsys.Remove(tmpName)
		return err
	}
	_ = fsys.Chmod(tmpName, mode)
	if err := fsys.Rename(tmpName, p); err != nil {
		fsys.Remove(tmpName)
		return err
	}
	return nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// GET /api/productcode/{path} — 查询单个 PDF 的产品编号。
// 数据库优先；未命中时回读 PDF 元数据（离线拷贝回的文件也能识别）并回填数据库。
var productCodeGetHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	p := strings.TrimSuffix(r.URL.Path, "/")
	if p == "" || !isPDFPath(p) {
		return http.StatusBadRequest, nil
	}
	if _, err := d.user.Fs.Stat(p); err != nil {
		return errToStatus(err), err
	}

	entry, err := d.store.ProductCode.Get(p)
	switch {
	case err == nil:
		return renderJSON(w, r, entry)
	case errors.Is(err, fberrors.ErrNotExist):
		code, metaErr := readPDFCode(d.user.Fs, p)
		if metaErr != nil || code == "" {
			return renderJSON(w, r, &productcode.Entry{Path: p, Code: ""})
		}
		entry = &productcode.Entry{
			Path:      p,
			Code:      code,
			UserID:    d.user.ID,
			UpdatedAt: time.Now().Unix(),
		}
		// 回填数据库，后续查询走索引（best-effort）
		_ = d.store.ProductCode.Save(entry)
		return renderJSON(w, r, entry)
	default:
		return http.StatusInternalServerError, err
	}
})

type productCodePutBody struct {
	Code string `json:"code"`
}

type productCodePutResult struct {
	Path       string `json:"path"`
	Code       string `json:"code"`
	PDFUpdated bool   `json:"pdfUpdated"`
	PDFError   string `json:"pdfError,omitempty"`
}

// PUT /api/productcode/{path} — 设置/清除产品编号（双重方案）：
//  1. 数据库（查询索引）总是写入成功才算成功；
//  2. PDF Keywords 元数据尽力写入（离线可追溯），失败时在响应里说明原因。
var productCodePutHandler = withPermModify(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	p := strings.TrimSuffix(r.URL.Path, "/")
	if p == "" || !isPDFPath(p) {
		return http.StatusBadRequest, nil
	}

	var body productCodePutBody
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return http.StatusBadRequest, err
		}
		defer r.Body.Close()
	}
	code := strings.TrimSpace(body.Code)
	if code != "" && len(code) > productCodeMaxLen {
		return http.StatusBadRequest, nil
	}

	if _, err := d.user.Fs.Stat(p); err != nil {
		return errToStatus(err), err
	}

	// 1) 数据库
	if code == "" {
		if err := d.store.ProductCode.Delete(p); err != nil && !errors.Is(err, fberrors.ErrNotExist) {
			return http.StatusInternalServerError, err
		}
	} else {
		if err := d.store.ProductCode.Save(&productcode.Entry{
			Path:      p,
			Code:      code,
			UserID:    d.user.ID,
			UpdatedAt: time.Now().Unix(),
		}); err != nil {
			return http.StatusInternalServerError, err
		}
	}

	// 2) PDF 元数据（best-effort）
	pdfErr := writePDFCode(d.user.Fs, p, code)

	return renderJSON(w, r, productCodePutResult{
		Path:       p,
		Code:       code,
		PDFUpdated: pdfErr == nil,
		PDFError:   errString(pdfErr),
	})
})

type productCodeBatchBody struct {
	Paths []string `json:"paths"`
}

// POST /api/productcode/batch — 目录列表批量查询产品编号。
// 一次读取全表后在内存匹配，避免逐条查询。
var productCodeBatchHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var body productCodeBatchBody
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return http.StatusBadRequest, err
		}
		defer r.Body.Close()
	}
	if len(body.Paths) == 0 || len(body.Paths) > 1000 {
		return http.StatusBadRequest, nil
	}

	all, err := d.store.ProductCode.All()
	if err != nil && !errors.Is(err, fberrors.ErrNotExist) {
		return http.StatusInternalServerError, err
	}
	index := make(map[string]string, len(all))
	for _, e := range all {
		index[e.Path] = e.Code
	}

	out := make(map[string]string, len(body.Paths))
	for _, p := range body.Paths {
		if !isPDFPath(p) || !d.Check(p) {
			continue
		}
		if code, ok := index[p]; ok {
			out[p] = code
		}
	}
	return renderJSON(w, r, out)
})

// GET /api/productcode/search?query=xxx — 按产品编号前缀反查 PDF。
var productCodeSearchHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	q := r.URL.Query().Get("query")
	if q == "" {
		return renderJSON(w, r, []*productcode.Entry{})
	}

	entries, err := d.store.ProductCode.FindByCodePrefix(q)
	if err != nil && !errors.Is(err, fberrors.ErrNotExist) {
		return http.StatusInternalServerError, err
	}

	out := make([]*productcode.Entry, 0, len(entries))
	for _, e := range entries {
		if !d.Check(e.Path) {
			continue
		}
		if info, err := d.user.Fs.Stat(e.Path); err == nil && !info.IsDir() {
			out = append(out, e)
		}
	}
	return renderJSON(w, r, out)
})
