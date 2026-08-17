package search

import (
	"strings"
	"testing"
)

// TestContainsTerm_Cases 覆盖产品编号搜索的典型场景
func TestContainsTerm_Cases(t *testing.T) {
	cases := []struct {
		fileName, term string
		want           bool
		desc           string
	}{
		// ===== 老语义必须保留（CQG5 不匹配 CQG50 / WBCQG5）=====
		{"CQG50.pdf", "cqg5", false, "旧：CQG5 不应命中 CQG50.pdf（数字段长度不同）"},
		{"WBCQG5.pdf", "cqg5", false, "旧：CQG5 不应命中 WBCQG5.pdf（WB 前缀段挡在前面）"},
		{"CQG501.pdf", "cqg5", false, "旧：CQG5 不应命中 CQG501.pdf（501 段）"},

		// ===== 老语义必须保留（CQG5 匹配带分隔符的变体）=====
		{"CQG5-0.88.pdf", "cqg5", true, "旧：CQG5 应命中 CQG5-0.88.pdf"},
		{"CQG5 0.88.pdf", "cqg5", true, "旧：CQG5 应命中 CQG5 0.88.pdf"},
		{"CQG5(0.88).pdf", "cqg5", true, "旧：CQG5 应命中 CQG5(0.88).pdf"},
		{"CQG5（0.88）.pdf", "cqg5", true, "旧：CQG5 应命中全角括号 CQG5（0.88）.pdf"},
		{"CQG5氩气.pdf", "cqg5", true, "旧：CQG5 应命中 CQG5氩气.pdf（氩=非ASCII=词边界）"},
		{"CQG5_xxx.pdf", "cqg5", true, "旧：CQG5 应命中 CQG5_xxx.pdf"},
		{"CQG5.pdf", "cqg5", true, "旧：CQG5 应命中 CQG5.pdf"},
		{"[CQG5]方案.pdf", "cqg5", true, "旧：CQG5 应命中 [CQG5]方案.pdf"},

		// ===== 新语义：尺寸/参数（纯数字或纯字母段）要能在标识符中间命中 =====
		{"ZKG2 (-0.1-DN150JC) .pdf", "zkg2", true, "新：ZKG2 应该命中 ZKG2 (-0.1-DN150JC) .pdf"},
		{"ZKG2 (-0.1-DN150JC) .pdf", "150", true, "新：150 应该命中 DN150JC 标识符中的 150 段"},
		{"ZKG2(-0.1,DN150JC).pdf", "150", true, "新：150 应命中 ZKG2(-0.1,DN150JC).pdf（逗号分隔）"},
		{"ZKG2(-0.1 DN150JC).pdf", "150", true, "新：150 应命中空格分隔的 DN150JC"},
		{"ZKG2-0.1-DN150JC 内环.pdf", "150", true, "新：150 应命中 DN150JC（短线分隔路径）"},

		// ===== 新语义：字母+数字代码段前缀要能连续匹配 =====
		{"DN150JC.pdf", "dn150", true, "新：DN150 段[DN,150]应命中 DN150JC 的前两段"},
		{"DN150JC.pdf", "150jc", true, "新：150JC 段[150,JC]应命中 DN150JC 后两段"},
		{"DN150JC.pdf", "dn150jc", true, "新：DN150JC 与标识符完全相等当然命中"},
		{"ZKG25(-0.1,D1200-DN80JC).pdf", "80", true, "新：80 段应命中 DN80JC 里的 80 段"},
		{"ZKG25(-0.1,D1200-DN80JC).pdf", "d1200", true, "新：D1200 段[D,1200]应命中 D1200 标识符"},

		// ===== 边界防御：错误匹配不能发生 =====
		{"CQG50.pdf", "50", true, "新：纯数字 50 应命中 CQG50 的 50 段（Windows 也会匹配）"},
		{"CQG50.pdf", "cqg", true, "新：字母段 CQG 应命中 CQG50 的 CQG 段"},
		{"ABC123DEF.pdf", "12", false, "新：12 不能命中 123（段值 123 不匹配 12）"},
		{"ABC123DEF.pdf", "ab", false, "新：ab 不能命中 ABC（段值 ABC≠ab）"},
		{"ABC123DEF.pdf", "abc", true, "新：abc 段=ABC，命中"},
		{"ABC123DEF.pdf", "123", true, "新：123 段=123，命中"},

		// ===== 用户实际 case：ZKG2 150 AND 两关键词都必须命中 =====
		{"ZKG2 (-0.1-DN150JC) .pdf", "zkg2", true, "用户case：term ZKG2 命中"},
		{"ZKG2 (-0.1-DN150JC) .pdf", "150", true, "用户case：term 150 命中"},
		{"ZKG2(-0.1,DN200JC).pdf", "150", false, "用户case：term 150 不应命中 DN200JC"},
		{"ZKG2(-0.1,DN1500JC).pdf", "150", false, "用户case：term 150 不应命中 DN1500JC（段 1500≠150）"},
	}

	// 由于 search.Search 在进入 ContainsTerm 之前已经 fileName = strings.ToLower，
	// 这里我们也把 fileName 转成小写来模拟真实调用路径。
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			fn := strings.ToLower(c.fileName)
			got := ContainsTerm(fn, c.term)
			if got != c.want {
				t.Errorf("ContainsTerm(%q, %q) = %v, want %v (%s)",
					c.fileName, c.term, got, c.want, c.desc)
			}
		})
	}
}

// TestTokensAnd 模拟真实 parseSearch → Search.Terms AND 语义在用户关键字下的行为
func TestTokensAnd(t *testing.T) {
	type tc struct {
		queryTokens []string
		fileName    string
		want        bool
		desc        string
	}
	cases := []tc{
		{
			queryTokens: []string{"zkg2", "150"},
			fileName:    "ZKG2 (-0.1-DN150JC) .pdf",
			want:        true,
			desc:        "用户真实 query：ZKG2 150 应命中 DN150JC 的两个文件",
		},
		{
			queryTokens: []string{"zkg2", "150"},
			fileName:    "ZKG2 (-0.1, DN150JC) .pdf",
			want:        true,
			desc:        "用户真实 query：逗号空格版本同样命中",
		},
		{
			queryTokens: []string{"zkg2", "150"},
			fileName:    "ZKG2(-0.1-DN200JC).pdf",
			want:        false,
			desc:        "150 vs 200：不应命中",
		},
		{
			queryTokens: []string{"cqg5", "0.88"},
			fileName:    "CQG5-0.88.pdf",
			want:        true,
			desc:        "旧 case：CQG5 0.88 应命中 CQG5-0.88.pdf",
		},
		{
			queryTokens: []string{"cqg5", "0.88"},
			fileName:    "CQG50-0.88.pdf",
			want:        false,
			desc:        "旧 case：CQG5 0.88 不应命中 CQG50-0.88.pdf",
		},
		{
			queryTokens: []string{"zkg25", "1200", "80"},
			fileName:    "ZKG25(-0.1,D1200-DN80JC) .pdf",
			want:        true,
			desc:        "三关键词 AND 都命中（25、1200、80 段匹配）",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			fn := strings.ToLower(c.fileName)
			all := true
			for _, tok := range c.queryTokens {
				if !ContainsTerm(fn, tok) {
					all = false
					break
				}
			}
			if all != c.want {
				t.Errorf("AND %v on %q = %v, want %v (%s)",
					c.queryTokens, c.fileName, all, c.want, c.desc)
			}
		})
	}
}
