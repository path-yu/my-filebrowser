package fbhttp

// 共用的 JSON 响应写入辅助：
// 1. 先序列化成字节切片，算 Content-Length
// 2. 再一次性写 Header(Content-Type/Content-Length) → WriteHeader → Write(body)
//
// 这样做的原因：Vite dev server（Node http-proxy）转发带 body 的 5xx 响应时，
// 如果后端先 WriteHeader(code) 再 json.Encode 分块写，Node 代理在等待第二块
// TCP 数据时若遇到对端 FIN 先到（正常 EOF 但 HTTP/1.1 chunked 没写 0\r\n\r\n），
// 会把它当成 "后端 socket 意外关闭（ECONNRESET/HPE_INVALID）"，最终给前端返回
// 502 Bad Gateway，而不是我们预期的 501/正常 JSON。
// 而只要显式写 Content-Length（非 chunked），http-proxy 会用固定长度对比 body
// 字节数，读到预期字节就认为响应完整，不会误判成 socket 崩掉 502。

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("json.Marshal failed: " + err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", itoa(len(body)))
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

// 小工具：Content-Length 用的 int -> string，避免额外 import strconv。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
