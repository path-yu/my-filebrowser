package productcode

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// KeywordPrefix 是写入 PDF Keywords 元数据的产品编号标记前缀，
// 例如产品编号 "AB-123" 会以 "product-code:AB-123" 的形式写入。
// 这样不会破坏用户已有的其他 Keywords，也能被任意 PDF 阅读器识别。
const KeywordPrefix = "product-code:"

var configOnce sync.Once

// disableUserConfig 阻止 pdfcpu 读写用户配置目录（%AppData%/pdfcpu）。
// 默认行为会在无写权限的服务器环境中 panic（fault.Fail），这里统一关闭。
func disableUserConfig() {
	configOnce.Do(func() {
		model.ConfigPath = "disable"
	})
}

func newConf(cmd model.CommandMode) *model.Configuration {
	disableUserConfig()
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	conf.Cmd = cmd
	return conf
}

// ReadCodeFromPDF 从 PDF 的 Keywords 元数据中读取产品编号。
// 未设置时返回空字符串；文件不是合法 PDF 时返回错误。
func ReadCodeFromPDF(rs io.ReadSeeker) (string, error) {
	keywords, err := api.Keywords(rs, newConf(model.LISTKEYWORDS))
	if err != nil {
		return "", err
	}
	for _, kw := range keywords {
		if code, ok := strings.CutPrefix(kw, KeywordPrefix); ok {
			return code, nil
		}
	}
	return "", nil
}

// WriteCodeToPDF 将产品编号写入 PDF 的 Keywords 元数据并把结果写到 w。
// code 为空表示移除产品编号标记；文件的其他 Keywords 保持不变。
func WriteCodeToPDF(rs io.ReadSeeker, w io.Writer, code string) error {
	conf := newConf(model.ADDKEYWORDS)

	ctx, err := api.ReadValidateAndOptimize(rs, conf)
	if err != nil {
		return fmt.Errorf("read pdf: %w", err)
	}

	// 移除旧的产品编号标记（避免重复追加）
	existing, err := pdfcpu.KeywordsList(ctx)
	if err != nil {
		return fmt.Errorf("list keywords: %w", err)
	}
	var stale []string
	for _, kw := range existing {
		if strings.HasPrefix(kw, KeywordPrefix) {
			stale = append(stale, kw)
		}
	}
	if len(stale) > 0 {
		if _, err := pdfcpu.KeywordsRemove(ctx, stale); err != nil {
			return fmt.Errorf("remove old product code: %w", err)
		}
	}

	if code != "" {
		if err := pdfcpu.KeywordsAdd(ctx, []string{KeywordPrefix + code}); err != nil {
			return fmt.Errorf("add product code: %w", err)
		}
	}

	return api.WriteContext(ctx, w)
}
