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

// splitAlphaNumSegments 把一串 rune 按「字母段 / 数字段」拆分为连续的"段"，
// 非字母数字（例如中文、Unicode 标点等，只要不是 0-9 / A-Za-z）单独成段保留，
// 以便中文或混合字符场景下仍能做子串匹配。
//
// 例：
//   "CQG50"      → ["CQG", "50"]
//   "DN150JC"    → ["DN", "150", "JC"]
//   "ZKG2"       → ["ZKG", "2"]
//   "0.88"       → ["0", ".", "88"]     （. 非字母数字，单独成段）
//   "CQG5氩气"   → ["CQG", "5", "氩", "气"]
func splitAlphaNumSegments(r []rune) [][]rune {
	if len(r) == 0 {
		return nil
	}
	segs := make([][]rune, 0, 4)
	start := 0
	kind := runeKind(r[0])
	for i := 1; i < len(r); i++ {
		k := runeKind(r[i])
		if k != kind {
			segs = append(segs, r[start:i])
			start = i
			kind = k
		}
	}
	segs = append(segs, r[start:])
	return segs
}

// runeKind 给 rune 分类：数字(0) / ASCII 字母(1) / 其它(2)
func runeKind(r rune) int {
	if r >= '0' && r <= '9' {
		return 0
	}
	if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
		return 1
	}
	return 2
}

// segsEqual 逐 rune 比较两段是否完全相等
func segsEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// identifierMatchesTerm 判断单个标识符（按 IsWordDelim 切出来的连续块）
// 是否匹配搜索词 term：
//   - 先把 identifier 与 term 都按「字母段 / 数字段 / 其它段」拆段
//   - 如果 term 的段序列能作为「连续子数组」出现在 identifier 的段序列里，则匹配
//
// 这样同时满足两种语义：
//   1) CQG5 不匹配 CQG50：CQG5→[CQG,5] vs CQG50→[CQG,50]，长度相同但第二段 "5"!="50"，不命中
//   2) 150 匹配 DN150JC：150→[150] 与 DN150JC→[DN,150,JC] 第 2 段相等，命中
//   3) DN150 匹配 DN150JC：DN150→[DN,150] 与 DN150JC 前两段连续相等，命中
//   4) ZKG2 匹配 ZKG2(-0.1-DN150JC)：ZKG2→[ZKG,2] 等于标识符 ZKG2 两段，命中
func identifierMatchesTerm(identifier, term []rune) bool {
	idSegs := splitAlphaNumSegments(identifier)
	tSegs := splitAlphaNumSegments(term)
	if len(tSegs) == 0 {
		return true
	}
	if len(tSegs) > len(idSegs) {
		return false
	}
	// 滑动窗口：term 的段序列必须是 identifier 段序列的连续子数组
outer:
	for k := 0; k+len(tSegs) <= len(idSegs); k++ {
		for j := range tSegs {
			if !segsEqual(idSegs[k+j], tSegs[j]) {
				continue outer
			}
		}
		return true
	}
	return false
}

// splitByWordDelim 按 IsWordDelim 把 rune 切片拆成多个非空片段
func splitByWordDelim(r []rune) [][]rune {
	var out [][]rune
	start := 0
	for i := 0; i <= len(r); i++ {
		if i == len(r) || IsWordDelim(r[i]) {
			if i > start {
				out = append(out, r[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// ContainsTerm 判断 fileName 中是否存在 term 的匹配。
// 语义两步：
//   A) 按 IsWordDelim 分别切 fileName 和 term：
//        - fileName → identifiers（非空块，比如 "ZKG2 (-0.1-DN150JC) .pdf" → [ZKG2, 0, 1, DN150JC, pdf]）
//        - term     → termFragments（比如 "0.88" → [0, 88]；"ZKG2" → [ZKG2]）
//   B) 对每个 termFragment，按 identifierMatchesTerm 的段级连续子数组匹配，
//        要求它命中任意一个 identifier。所有 termFragment 都命中 → 返回 true。
//
// 这样解决两类问题：
//   - "CQG5 0.88" 多关键词 AND：每个 term 单独匹配（search.Search 外层循环）
//   - "0.88" 单 term 里含分隔符（. - _ 等）：自动拆成 [0,88]，要求在文件名里都能找到（AND）
//
// 段级匹配保留产品编号语义：
//   - CQG5 不匹配 CQG50：[CQG,5] vs [CQG,50] 段值不对
//   - 150 匹配 DN150JC：[150] 段在 [DN,150,JC] 里存在
//   - DN150 匹配 DN150JC：[DN,150] 是 [DN,150,JC] 的前缀连续子数组
func ContainsTerm(fileName, term string) bool {
	if term == "" {
		return true
	}
	if term == fileName {
		return true
	}

	fr := []rune(fileName)
	tr := []rune(term)
	if len(tr) > len(fr) {
		return false
	}

	// 文件名按词边界切标识符
	idents := splitByWordDelim(fr)
	if len(idents) == 0 {
		return false
	}
	// 搜索词本身也按词边界拆（比如 "0.88" → ["0","88"]）
	termFrags := splitByWordDelim(tr)
	if len(termFrags) == 0 {
		return true
	}

	// 每个 term 片段必须至少命中一个标识符（AND 语义）
outer:
	for _, tf := range termFrags {
		for _, id := range idents {
			if identifierMatchesTerm(id, tf) {
				continue outer
			}
		}
		// 有一个片段在所有标识符里都找不到 → 整体失败
		return false
	}
	return true
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
