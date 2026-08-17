package search

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/rules"
)

// asciiWordDelimiters 定义产品编号/型号搜索常用的 ASCII 分隔符。
// 注意：中文字符、Unicode 空白字符不在这里，通过 unicode.IsSpace / 非 ASCII 中文边界逻辑统一处理。
const asciiWordDelimiters = "-_()[]{},;.\\/|~`!@#$%^&*+=<>?:'\""

// IsWordDelim 判断 rune r 是否属于「词边界分隔符」。
// 与 Windows 资源管理器搜索语义保持对齐：
//   - 所有 ASCII 特殊符号（见 asciiWordDelimiters）
//   - 所有 Unicode 空白字符（包含 ASCII 空格、Tab、全角空格 U+3000、不间断空格 U+00A0、EN/EM SPACE 等）
//   - 任何非 ASCII 字符（>= U+0080，比如中日韩文字、中文标点）与其相邻的 ASCII 字母数字之间
//     也视为天然的词边界。
func IsWordDelim(r rune) bool {
	if r < 0x80 {
		// ASCII 范围：空白字符 + 预定义符号
		if r <= ' ' { // 0x00-0x20：控制字符 + 普通空格
			return true
		}
		for i := 0; i < len(asciiWordDelimiters); i++ {
			if rune(asciiWordDelimiters[i]) == r {
				return true
			}
		}
		return false
	}
	// 非 ASCII：Unicode 空白字符 → 是分隔符
	if unicode.IsSpace(r) {
		return true
	}
	// 其它非 ASCII（中日韩文字、符号等）：
	// 当它与 ASCII 字母数字（term 通常由 ASCII 构成）相邻时，
	// 我们希望它起到词边界作用（例如 "CQG5氩气" 中 氩 与 5 的边界）。
	// 因此这里统一把非 ASCII 视为潜在分隔符。
	return true
}

// ContainsTerm 判断 fileName 中是否存在 term 作为一个「整词」出现。
// 使用 rune 级别的匹配，避免 UTF-8 多字节字符（中文、全角空格等）导致的边界判断错误。
//   - term 的前一个 rune 要么不存在（fileName 开头），要么是 IsWordDelim(...)
//   - term 的后一个 rune 要么不存在（fileName 结尾），要么是 IsWordDelim(...)
//
// 这样 "CQG5" 不会命中 WBCQG5 / CQG50，但会命中：
//   CQG5-0.88.pdf / CQG5 0.88.pdf / CQG5（0.88）.pdf / CQG5氩气.pdf / CQG5 全角空格 0.88.pdf
func ContainsTerm(fileName, term string) bool {
	if term == "" {
		return true
	}
	if term == fileName {
		return true
	}

	// 统一转成 rune 切片，彻底避免 UTF-8 字节偏移困扰
	fr := []rune(fileName)
	tr := []rune(term)
	n := len(fr)
	m := len(tr)
	if m > n {
		return false
	}

	for i := 0; i+m <= n; i++ {
		// 快速失败：首位 rune 不相等直接跳过（避免每轮全量比较）
		if fr[i] != tr[0] {
			continue
		}
		// 检查 term 全部 rune 是否匹配
		match := true
		for j := 1; j < m; j++ {
			if fr[i+j] != tr[j] {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		frontOK := i == 0 || IsWordDelim(fr[i-1])
		backOK := i+m == n || IsWordDelim(fr[i+m])
		if frontOK && backOK {
			return true
		}
	}
	return false
}

type searchOptions struct {
	CaseSensitive bool
	Conditions    []condition
	Terms         []string
}

// Search searches for a query in a fs.
func Search(ctx context.Context,
	fs afero.Fs, scope, query string, checker rules.Checker, found func(path string, f os.FileInfo) error) error {
	search := parseSearch(query)

	scope = filepath.ToSlash(filepath.Clean(scope))
	scope = path.Join("/", scope)

	return afero.Walk(fs, scope, func(fPath string, f os.FileInfo, walkErr error) error {
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		// 防御性兜底：任何 Walk 阶段出现的单条错误（比如某文件权限不足、
		// 底层 Fs 接口异常、虚拟挂载分发 bug 遗留等）都只跳过该条目，
		// 绝对不能把 nil 的 FileInfo 继续往下传，否则上层调用 f.IsDir()
		// / f.Name() 会直接 panic，HTTP 返回 502 崩整个搜索请求。
		if walkErr != nil || f == nil {
			return nil
		}
		fPath = filepath.ToSlash(filepath.Clean(fPath))
		fPath = path.Join("/", fPath)
		relativePath := strings.TrimPrefix(fPath, scope)
		relativePath = strings.TrimPrefix(relativePath, "/")

		if fPath == scope {
			return nil
		}

		if !checker.Check(fPath) {
			return nil
		}

		if len(search.Conditions) > 0 {
			match := false

			for _, t := range search.Conditions {
				if t(fPath) {
					match = true
					break
				}
			}

			if !match {
				return nil
			}
		}

		if len(search.Terms) > 0 {
			_, fileName := path.Split(fPath)
			if !search.CaseSensitive {
				fileName = strings.ToLower(fileName)
			}
			// 多关键词必须同时命中（AND 语义），且每个关键词都必须是「分隔符整词」：
			// 关键词两侧要么是文件名首尾两端，要么是分隔符（- _ . () 空格 等）。
			// 这样与 Windows 资源管理器搜索语义一致：
			//   - "CQG5" 不会命中 WBCQG5 / CQG50（只是子串）
			//   - 只会命中 CQG5-0.88 / CQG5(...) / CQG5_... 这类整词匹配
			for _, term := range search.Terms {
				t := term
				if !search.CaseSensitive {
					t = strings.ToLower(t)
				}
				if !ContainsTerm(fileName, t) {
					return nil
				}
			}
			return found(fPath, f)
		}

		return found(fPath, f)
	})
}
