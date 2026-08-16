//go:build windows

package fbhttp

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

const wdFormatXMLDocument = 12 // .docx

// wordConvertDocToDocx 先尝试 Microsoft Word COM；
// 不可用时自动回退到 LibreOffice（soffice.exe --headless --convert-to docx）。
// 两者皆不可用时返回带安装指引的详细错误。
func wordConvertDocToDocx(src, dst string) error {
	err := wordConvertByCOM(src, dst)
	if err == nil {
		return nil
	}
	comErr := err

	err = libreOfficeConvertDocToDocx(src, dst)
	if err == nil {
		return nil
	}
	loErr := err

	return combineConvertErrors(comErr, loErr)
}

// wordConvertByCOM 通过本机 Word COM 自动化将 .doc 另存为 .docx。
// 要求服务器安装 Microsoft Office（Word）。Word 2007 使用 SaveAs，
// Word 2010+ 优先使用 SaveAs2。
func wordConvertByCOM(src, dst string) error {
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return fmt.Errorf("Word COM: CoInitializeEx: %w", err)
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("Word.Application")
	if err != nil {
		return fmt.Errorf("Word COM 未就绪（未安装 Microsoft Office Word?）: %w", err)
	}
	word, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		unknown.Release()
		return fmt.Errorf("Word COM: QueryInterface IDispatch: %w", err)
	}
	defer word.Release()

	// 无论成功与否都退出 Word 实例
	defer oleutil.CallMethod(word, "Quit", 0)

	if _, err := oleutil.PutProperty(word, "Visible", false); err != nil {
		return fmt.Errorf("Word COM: set Visible: %w", err)
	}
	if _, err := oleutil.PutProperty(word, "DisplayAlerts", 0); err != nil {
		return fmt.Errorf("Word COM: set DisplayAlerts: %w", err)
	}

	docsV, err := oleutil.GetProperty(word, "Documents")
	if err != nil {
		return fmt.Errorf("Word COM: get Documents: %w", err)
	}
	docsDisp := docsV.ToIDispatch()
	defer docsDisp.Release()

	// Open(FileName, ConfirmConversions, ReadOnly, AddToRecentFiles)
	docV, err := oleutil.CallMethod(docsDisp, "Open", src, false, true, false)
	if err != nil {
		return fmt.Errorf("Word COM 打开文档失败: %w", err)
	}
	docDisp := docV.ToIDispatch()
	defer docDisp.Release()

	// SaveAs2（Word 2010+），失败回退 SaveAs（Word 2007）
	if _, err := oleutil.CallMethod(docDisp, "SaveAs2", dst, wdFormatXMLDocument); err != nil {
		if _, err2 := oleutil.CallMethod(docDisp, "SaveAs", dst, wdFormatXMLDocument); err2 != nil {
			return fmt.Errorf("Word COM 另存为 docx 失败: %w", err2)
		}
	}

	oleutil.CallMethod(docDisp, "Close", 0)
	return nil
}

// libreOfficeConvertDocToDocx 通过 LibreOffice / OpenOffice 无头模式转换文档。
// 命令：soffice.exe --headless --convert-to docx --outdir <dir> <src>
// 若输出文件与期望 dst 不同名，则重命名到 dst。
func libreOfficeConvertDocToDocx(src, dst string) error {
	soffice, err := findSoffice()
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

	// 若 soffice 输出的文件与目标不一致，则重命名为 dst
	if expectedOut != dst {
		if err2 := renameFile(expectedOut, dst); err2 != nil {
			// 容忍：如果 dst 本身已经被写出（同名）也算成功
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

// findSoffice 查找 soffice.exe 路径：环境变量 PATH → 常见安装目录
func findSoffice() (string, error) {
	if p, err := exec.LookPath("soffice.exe"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("soffice"); err == nil {
		return p, nil
	}
	candidates := []string{
		`C:\Program Files\LibreOffice\program\soffice.exe`,
		`C:\Program Files (x86)\LibreOffice\program\soffice.exe`,
		`C:\Program Files\OpenOffice.org 3\program\soffice.exe`,
		`C:\Program Files (x86)\OpenOffice.org 3\program\soffice.exe`,
	}
	for _, c := range candidates {
		if _, err := statFile(c); err == nil {
			return c, nil
		}
	}
	return "", errors.New("未找到 soffice.exe，请安装 LibreOffice（推荐）或 OpenOffice，或把其 program 目录加入 PATH")
}

// combineConvertErrors 将两条转换路径的错误合并为一条带安装指引的用户可读错误。
func combineConvertErrors(comErr, loErr error) error {
	msg := &strings.Builder{}
	msg.WriteString(".doc 预览转换失败，当前服务器两条转换路径均不可用：\n")
	fmt.Fprintf(msg, "  1) Microsoft Office Word：%s\n", firstLine(comErr.Error()))
	fmt.Fprintf(msg, "  2) LibreOffice：%s\n", firstLine(loErr.Error()))
	msg.WriteString("\n解决方案（任选其一）：\n")
	msg.WriteString("  A. 安装 Microsoft Office（Word），支持 Windows 原生 COM 自动化（保真度最高）\n")
	msg.WriteString("  B. 安装 LibreOffice（免费开源）：从 https://www.libreoffice.org/ 下载 Windows 版，")
	msg.WriteString("安装时勾选「添加到 PATH」，或手动把 C:\\Program Files\\LibreOffice\\program 加入系统环境变量 PATH，然后重启服务\n")
	msg.WriteString("\n临时替代方式：直接下载原文件，用本机 Word/WPS 打开查看。")
	return errors.New(msg.String())
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return s
}
