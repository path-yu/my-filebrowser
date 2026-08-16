package fbhttp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/filebrowser/filebrowser/v2/files"
)

// docConvertMu 串行化 Word COM 转换（避免并发启动多个 WINWORD 实例）
var docConvertMu sync.Mutex

// docCacheDir 返回 .doc → .docx 转换缓存目录（懒创建）
func docCacheDir() (string, error) {
	dir := filepath.Join(os.TempDir(), "filebrowser-doc-convert")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// docConvertHandler 将旧版 .doc 通过本机 Word COM 转换为 .docx 后返回（已登录用户）。
var docConvertHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Download {
		return http.StatusForbidden, nil
	}

	file, err := files.NewFileInfo(&files.FileOptions{
		Fs:         d.user.Fs,
		Path:       r.URL.Path,
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: false,
		Checker:    d,
	})
	if err != nil {
		return errToStatus(err), err
	}
	return convertAndServeDocx(w, r, file)
})

// publicConvertDocHandler 公开分享链接的 .doc → .docx 转换（鉴权复用 withHashFile）
// 注意：withHashFile 会把 d.user.Fs 重新绑定到 ScopedFs（以 share root 为基准），
// 但 ScopedFs 的 RealPath 在单文件分享时可能无法正确解析磁盘路径（fallback 到相对路径），
// 而 doc 转换需要把真实文件拷贝到临时目录后交给 Word/LibreOffice 打开（必须是带盘符的本地路径）。
// 所以这里用 withHashFile 保存的 d.userOriginalFs（原始未 scoped 的用户文件系统） +
// d.checkerPrefix（share 根的 user 级路径） + scopedFile.Path（share 内相对路径）
// 重新 new FileInfo，这样 RealPath() 能正确解析到真实磁盘位置。
var publicConvertDocHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	scopedFile := d.raw.(*files.FileInfo)

	if d.userOriginalFs == nil {
		return http.StatusInternalServerError, fmt.Errorf("share user original Fs is nil")
	}

	// Resolve user-rooted path: checkerPrefix (share root) + scoped relative path
	scopedPath := scopedFile.Path
	userPath := filepath.Join(d.checkerPrefix, scopedPath)
	// Normalize to forward slash for the internal path semantics (used by afero)
	userPath = filepath.ToSlash(userPath)

	file, err := files.NewFileInfo(&files.FileOptions{
		Fs:         d.userOriginalFs,
		Path:       userPath,
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: false,
		CalcImgRes: false,
		Checker:    d,
	})
	if err != nil {
		return errToStatus(err), fmt.Errorf("resolve share file for convert: %w (userPath=%q)", err, userPath)
	}

	return convertAndServeDocx(w, r, file)
})

// convertAndServeDocx 共享的 .doc → .docx 转换 + 响应逻辑：
// - 校验文件类型（非目录、.doc 后缀）
// - 按真实路径+大小+修改时间缓存转换结果
// - 拷贝源文件到临时目录 → Word COM / LibreOffice 转换
// - 转换失败：把详细错误（含安装指引）写入响应 body
// - 转换成功：以 docx MIME inline 返回
func convertAndServeDocx(w http.ResponseWriter, r *http.Request, file *files.FileInfo) (int, error) {
	if file.IsDir || !strings.EqualFold(filepath.Ext(file.Name), ".doc") {
		return http.StatusBadRequest, fmt.Errorf("仅支持 .doc 文件转换")
	}

	cacheDir, err := docCacheDir()
	if err != nil {
		return http.StatusInternalServerError, err
	}

	srcPath := file.RealPath()

	// 缓存 key：真实路径 + 大小 + 修改时间
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d", srcPath, file.Size, file.ModTime.UnixNano())))
	dstPath := filepath.Join(cacheDir, hex.EncodeToString(sum[:16])+".docx")

	docConvertMu.Lock()
	defer docConvertMu.Unlock()

	if _, err := os.Stat(dstPath); err == nil {
		return serveConvertedDocx(w, r, dstPath)
	}

	// 1) 拷贝源文件到本地临时目录（避免 Word 直接打开 UNC/网络共享文件
	//    产生 ~$ 锁文件或因网络波动挂起）
	tmpSrc := dstPath + ".src.doc"
	if err := copyFile(srcPath, tmpSrc); err != nil {
		return http.StatusInternalServerError, err
	}
	defer os.Remove(tmpSrc)

	// 2) Word COM / LibreOffice 转换为 docx
	if err := wordConvertDocToDocx(tmpSrc, dstPath); err != nil {
		os.Remove(dstPath)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(err.Error()))
		return 0, nil
	}

	return serveConvertedDocx(w, r, dstPath)
}

// copyFile 拷贝本地磁盘文件（含 UNC 路径）
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// serveConvertedDocx 以 docx MIME 返回转换后的文件内容
func serveConvertedDocx(w http.ResponseWriter, r *http.Request, path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, "converted.docx", st.ModTime(), f)
	return 0, nil
}

// statFile 是 os.Stat 的薄封装，便于替换测试
func statFile(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// renameFile 是 os.Rename 的薄封装
func renameFile(oldName, newName string) error {
	return os.Rename(oldName, newName)
}
