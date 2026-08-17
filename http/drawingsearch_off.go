//go:build !drawingsearch
// +build !drawingsearch

package fbhttp

// 默认（不含 CGO/onnxruntime 时）编译的占位文件：
// 相似图纸（PDF / 图片）向量检索功能需要 -tags drawingsearch 编译（需 MinGW-w64 + CGO）。
// 未启用时 POST /api/search/similar-pdf 仍会被注册，但返回明确的 HTTP 501
// 错误，方便前端展示友好的"请重新启用"提示，而不是 404。

import (
	"net/http"

	"github.com/gorilla/mux"
)

var similarPdfStubHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	body := map[string]any{
		"error":     "相似图纸检索功能未启用（支持上传 PDF 或图片）",
		"hint":      "请使用 dev-files/mingw64 提供的 MinGW-w64 GCC + CGO_ENABLED=1 重新编译：go build -tags drawingsearch -o filebrowser.exe 。或直接执行项目根目录的 run-filebrowser.ps1（不加 -NoDrawingSearch），它会自动配置所有环境变量并启用该功能（图片上传无需 Poppler，PDF 上传需 Poppler）。",
		"query":     "not-available",
		"totalInDB": 0,
		"results":   []any{},
		"diagnosis": "当前 filebrowser 可执行文件未启用 CGO 编译，相似图纸（PDF/图片）向量检索被禁用，说明见 hint。",
		"elapsed":   "-",
	}
	writeJSON(w, http.StatusNotImplemented, body)
	return 0, nil
})

// attachDrawingSearchRouter（默认 stub 版本）
// 注册路由但返回"未启用"错误；前端仍能区分"功能不存在(404)"和"功能未启用(501)"。
func attachDrawingSearchRouter(r *mux.Router, monkey func(fn handleFunc, prefix string) http.Handler) {
	r.Handle("/search/similar-pdf", monkey(similarPdfStubHandler, "")).Methods("POST")
}
