package productcode

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// newBlankPDF 生成一个空白的合法 PDF（通过 pdfcpu 构建无页文档并序列化）。
func newBlankPDF(t *testing.T) []byte {
	t.Helper()
	ctx, err := pdfcpu.CreateContextWithXRefTable(nil, types.PaperSize["A4"])
	if err != nil {
		t.Fatalf("create context: %v", err)
	}
	var buf bytes.Buffer
	if err := api.WriteContext(ctx, &buf); err != nil {
		t.Fatalf("write blank pdf: %v", err)
	}
	return buf.Bytes()
}

// TestMain 统一禁用 pdfcpu 的用户配置目录访问（沙箱/无权限环境下会 panic）。
func TestMain(m *testing.M) {
	disableUserConfig()
	os.Exit(m.Run())
}

func keywordsOf(t *testing.T, data []byte) []string {
	t.Helper()
	kw, err := api.Keywords(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatalf("list keywords: %v", err)
	}
	return kw
}

func TestWriteAndReadProductCode(t *testing.T) {
	pdf := newBlankPDF(t)

	// 初始无产品编号
	code, err := ReadCodeFromPDF(bytes.NewReader(pdf))
	if err != nil {
		t.Fatalf("read blank: %v", err)
	}
	if code != "" {
		t.Fatalf("expect empty code, got %q", code)
	}

	// 写入
	var out bytes.Buffer
	if err := WriteCodeToPDF(bytes.NewReader(pdf), &out, "AB-123"); err != nil {
		t.Fatalf("write code: %v", err)
	}
	withCode := out.Bytes()

	if code, err = ReadCodeFromPDF(bytes.NewReader(withCode)); err != nil || code != "AB-123" {
		t.Fatalf("expect AB-123, got %q (err=%v)", code, err)
	}

	// 覆盖为另一个编号：不产生重复标记
	out.Reset()
	if err := WriteCodeToPDF(bytes.NewReader(withCode), &out, "CD-456"); err != nil {
		t.Fatalf("rewrite code: %v", err)
	}
	replaced := out.Bytes()

	if code, err = ReadCodeFromPDF(bytes.NewReader(replaced)); err != nil || code != "CD-456" {
		t.Fatalf("expect CD-456, got %q (err=%v)", code, err)
	}
	marks := 0
	for _, kw := range keywordsOf(t, replaced) {
		if strings.HasPrefix(kw, KeywordPrefix) {
			marks++
		}
	}
	if marks != 1 {
		t.Fatalf("expect exactly 1 product-code keyword, got %d (%v)", marks, keywordsOf(t, replaced))
	}

	// 清除
	out.Reset()
	if err := WriteCodeToPDF(bytes.NewReader(replaced), &out, ""); err != nil {
		t.Fatalf("clear code: %v", err)
	}
	cleared := out.Bytes()

	if code, err = ReadCodeFromPDF(bytes.NewReader(cleared)); err != nil || code != "" {
		t.Fatalf("expect empty code after clear, got %q (err=%v)", code, err)
	}
}

func TestWritePreservesExistingKeywords(t *testing.T) {
	pdf := newBlankPDF(t)

	// 先用 pdfcpu 追加业务关键词
	var seeded bytes.Buffer
	if err := api.AddKeywords(bytes.NewReader(pdf), &seeded, []string{"质检报告", "2024"}, nil); err != nil {
		t.Fatalf("seed keywords: %v", err)
	}

	var out bytes.Buffer
	if err := WriteCodeToPDF(bytes.NewReader(seeded.Bytes()), &out, "AB-123"); err != nil {
		t.Fatalf("write code: %v", err)
	}

	got := keywordsOf(t, out.Bytes())
	want := map[string]bool{"质检报告": false, "2024": false, "product-code:AB-123": false}
	for _, kw := range got {
		if _, ok := want[kw]; ok {
			want[kw] = true
		}
	}
	for kw, found := range want {
		if !found {
			t.Fatalf("keyword %q missing, got %v", kw, got)
		}
	}
}

func TestReadInvalidPDF(t *testing.T) {
	if _, err := ReadCodeFromPDF(strings.NewReader("not a pdf")); err == nil {
		t.Fatal("expect error for invalid pdf")
	}
}
