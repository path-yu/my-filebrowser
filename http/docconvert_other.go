//go:build !windows

package fbhttp

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// wordConvertDocToDocx 在非 Windows 平台：优先使用 LibreOffice / OpenOffice
// 无头模式转换；不可用时返回详细安装指引。
func wordConvertDocToDocx(src, dst string) error {
	if err := libreOfficeConvertDocToDocxOther(src, dst); err == nil {
		return nil
	} else {
		loErr := err
		return combineConvertErrorsOther(loErr)
	}
}

// libreOfficeConvertDocToDocxOther 通过 soffice 无头模式转换（跨平台）
func libreOfficeConvertDocToDocxOther(src, dst string) error {
	soffice, err := findSofficeOther()
	if err != nil {
		return fmt.Errorf("LibreOffice 未就绪: %w", err)
	}

	outDir := filepath.Dir(dst)
	baseNoExt := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	expectedOut := filepath.Join(outDir, baseNoExt+".docx")

	cmd := exec.Command(
		soffice,
		"--headless",
		"--norestore",
		"--nolockcheck",
		"--nologo",
		"--nodefault",
		"--convert-to", "docx",
		"--outdir", outDir,
		src,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("LibreOffice 转换失败: %w, 输出: %s", err, strings.TrimSpace(string(out)))
	}

	if expectedOut != dst {
		if err2 := renameFile(expectedOut, dst); err2 != nil {
			if _, statErr := statFile(dst); statErr != nil {
				return fmt.Errorf("LibreOffice 转换后无法重命名到目标文件: %w（原始输出: %s）", err2, expectedOut)
			}
		}
	} else {
		if _, statErr := statFile(expectedOut); statErr != nil {
			return fmt.Errorf("LibreOffice 未生成预期的输出文件: %s", expectedOut)
		}
	}
	return nil
}

func findSofficeOther() (string, error) {
	if p, err := exec.LookPath("soffice"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("libreoffice"); err == nil {
		return p, nil
	}
	// macOS 常见路径
	if runtime.GOOS == "darwin" {
		candidates := []string{
			"/Applications/LibreOffice.app/Contents/MacOS/soffice",
		}
		for _, c := range candidates {
			if _, err := statFile(c); err == nil {
				return c, nil
			}
		}
	}
	return "", errors.New("未找到 soffice/libreoffice，请安装 LibreOffice：https://www.libreoffice.org/")
}

func combineConvertErrorsOther(loErr error) error {
	msg := &strings.Builder{}
	fmt.Fprintf(msg, ".doc 预览转换失败：当前平台（%s/%s）不支持 Microsoft Office COM，且 LibreOffice 未就绪。\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(msg, "  LibreOffice 检查结果：%s\n", firstLine(loErr.Error()))
	msg.WriteString("\n解决方案：安装 LibreOffice（免费开源）：\n")
	msg.WriteString("  - macOS：brew install --cask libreoffice，或从官网 https://www.libreoffice.org/ 下载 dmg\n")
	msg.WriteString("  - Debian/Ubuntu：sudo apt-get install libreoffice\n")
	msg.WriteString("  - RHEL/CentOS：sudo yum install libreoffice\n")
	msg.WriteString("\n临时替代方式：直接下载原文件，用本机 Word/WPS 打开查看。")
	return errors.New(msg.String())
}
