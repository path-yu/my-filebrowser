package main

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/asdine/storm/v3"
	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/image/draw"
)

// DrawingFeature 与 transform-pdf 完全一致（BoltDB 表结构相同）
type DrawingFeature struct {
	FilePath      string    `storm:"id"`
	FileName      string    `storm:"index"`
	FeatureVector []float32 `storm:"inline"`
	CreatedAt     time.Time `storm:"index"`
	UpdatedAt     time.Time
}

const (
	ImgWidth  = 224
	ImgHeight = 224
)

// ---------- 路径解析（与 transform-pdf 保持一致） ----------

func resolveProjectRoot() (string, error) {
	if p := strings.TrimSpace(os.Getenv("PROJECT_ROOT")); p != "" {
		abs, err := filepath.Abs(p)
		if err == nil {
			if _, staterr := os.Stat(filepath.Join(abs, "go.mod")); staterr == nil {
				return abs, nil
			}
			return abs, nil
		}
	}
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		for {
			if _, staterr := os.Stat(filepath.Join(dir, "go.mod")); staterr == nil {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("无法定位项目根目录: %w", err)
	}
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); errors.Is(err, os.ErrNotExist) {
		fmt.Printf("⚠️  无法定位 go.mod，回退使用当前工作目录 %s 作为项目根。\n", wd)
	}
	return wd, nil
}

var (
	projectRoot  string
	devFilesDir  string
	dbPath       string
	onnxDllPath  string
	modelPath    string
	pdfToPpmPath string
)

func initPaths() error {
	root, err := resolveProjectRoot()
	if err != nil {
		return err
	}
	projectRoot = filepath.Clean(root)
	devFilesDir = filepath.Join(projectRoot, "dev-files")
	dbPath = filepath.Join(projectRoot, "drawings.db")
	onnxDllPath = filepath.Join(devFilesDir, "onnxruntime.dll")
	modelPath = filepath.Join(devFilesDir, "resnet18_features.onnx")
	pdfToPpmPath = filepath.Join(devFilesDir, "poppler-25.12.0", "Library", "bin", "pdftoppm.exe")
	fmt.Println("📦 项目根目录:", projectRoot)
	fmt.Println("📁 dev-files  :", devFilesDir)
	fmt.Println("💾 drawings.db:", dbPath)
	fmt.Println("🟢 onnxruntime:", onnxDllPath)
	fmt.Println("🧠 ONNX 模型  :", modelPath)
	fmt.Println("🖨️  pdftoppm   :", pdfToPpmPath)
	return nil
}

// ---------- PDF → PNG → 特征向量（与 transform-pdf 保持一致） ----------

func convertPdfToPng(pdfPath, outputDir string, index int) (string, error) {
	if _, err := os.Stat(pdfToPpmPath); err != nil {
		return "", fmt.Errorf("找不到 pdftoppm.exe (%s): %v", pdfToPpmPath, err)
	}
	outPrefix := filepath.Join(outputDir, fmt.Sprintf("page_%d", index))
	cmd := exec.Command(pdfToPpmPath, "-png", "-r", "300", "-f", "1", "-l", "1", pdfPath, outPrefix)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%v, 命令输出: %s", err, string(output))
	}
	return outPrefix + "-1.png", nil
}

func extractFeatureVector(imgPath, inputName, outputName string, outputInfo ort.InputOutputInfo) ([]float32, error) {
	tensorData, err := preprocessImage(imgPath)
	if err != nil {
		return nil, err
	}
	inputShape := ort.NewShape(1, 3, ImgHeight, ImgWidth)
	inputTensor, err := ort.NewTensor(inputShape, tensorData)
	if err != nil {
		return nil, err
	}
	defer inputTensor.Destroy()

	var outputShape ort.Shape
	if len(outputInfo.Dimensions) == 2 {
		dim0 := int64(outputInfo.Dimensions[0])
		if dim0 <= 0 {
			dim0 = 1
		}
		dim1 := int64(outputInfo.Dimensions[1])
		if dim1 <= 0 {
			dim1 = 1000
		}
		outputShape = ort.NewShape(dim0, dim1)
	} else {
		outputShape = ort.NewShape(1, 1000)
	}
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, err
	}
	defer outputTensor.Destroy()

	session, err := ort.NewAdvancedSession(
		modelPath,
		[]string{inputName},
		[]string{outputName},
		[]ort.Value{inputTensor},
		[]ort.Value{outputTensor},
		nil,
	)
	if err != nil {
		return nil, err
	}
	defer session.Destroy()
	if err := session.Run(); err != nil {
		return nil, err
	}
	return outputTensor.GetData(), nil
}

func preprocessImage(imgPath string) ([]float32, error) {
	file, err := os.Open(imgPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	srcImg, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	dstImg := image.NewRGBA(image.Rect(0, 0, ImgWidth, ImgHeight))
	draw.BiLinear.Scale(dstImg, dstImg.Bounds(), srcImg, srcImg.Bounds(), draw.Over, nil)

	mean := []float32{0.485, 0.456, 0.406}
	std := []float32{0.229, 0.224, 0.225}

	tensorData := make([]float32, 1*3*ImgHeight*ImgWidth)
	stride := ImgHeight * ImgWidth

	for y := 0; y < ImgHeight; y++ {
		for x := 0; x < ImgWidth; x++ {
			r, g, b, _ := dstImg.At(x, y).RGBA()
			rf := float32(r>>8) / 255.0
			gf := float32(g>>8) / 255.0
			bf := float32(b>>8) / 255.0
			rf = (rf - mean[0]) / std[0]
			gf = (gf - mean[1]) / std[1]
			bf = (bf - mean[2]) / std[2]
			idx := y*ImgWidth + x
			tensorData[0*stride+idx] = rf
			tensorData[1*stride+idx] = gf
			tensorData[2*stride+idx] = bf
		}
	}
	return tensorData, nil
}

// ---------- 余弦相似度 + Top-K ----------

type SearchResult struct {
	FileName  string
	FilePath  string
	Similarity float64
	UpdatedAt time.Time
}

// ---------- 日志辅助：同时输出到 stdout + run.log 文件（避免滚屏丢失） ----------

var logFile *os.File

func setupLogFile() {
	// 把日志写到 exe 同目录下的 search-similar.log，即使 cmd 滚屏也能翻日志查
	exe, _ := os.Executable()
	logPath := filepath.Join(filepath.Dir(exe), "search-similar.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	logFile = f
	log.SetOutput(io.MultiWriter(os.Stderr, f)) // log.* 默认写 stderr，这里让它也写文件
}

func logPrintln(a ...interface{}) {
	s := fmt.Sprintln(a...)
	fmt.Print(s)
	if logFile != nil {
		logFile.WriteString(s)
	}
}

func logPrintf(format string, a ...interface{}) {
	s := fmt.Sprintf(format, a...)
	fmt.Print(s)
	if logFile != nil {
		logFile.WriteString(s)
	}
}

func fatalf(format string, a ...interface{}) {
	msg := fmt.Sprintf("❌ "+format+"\n", a...)
	fmt.Fprint(os.Stdout, msg)
	fmt.Fprint(os.Stderr, msg)
	if logFile != nil {
		logFile.WriteString(msg)
		logFile.Close()
	}
	os.Exit(1)
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := 0; i < len(a); i++ {
		av, bv := float64(a[i]), float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func main() {
	setupLogFile()
	logPrintln("══════════════════════════════════════════════════════════════════")
	logPrintf("🕒 启动时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	// ====== 0. 诊断参数：先打印 os.Args 让用户确认传参是否正确 ======
	logPrintf("📝 命令行参数 (共 %d 个):\n", len(os.Args))
	for i, a := range os.Args {
		logPrintf("   Args[%d] = %q\n", i, a)
	}
	logPrintln()

	if len(os.Args) < 2 {
		logPrintln("用法: search-similar <PDF文件路径> [TopK]")
		logPrintln("示例: search-similar \"D:\\BaiduNetdiskDownload\\图纸\\图纸\\ZKG2.pdf\" 10")
		logPrintln()
		logPrintln("⚠️  常见错误提醒:")
		logPrintln("  · PDF 路径请用绝对路径，并对含中文/空格/括号的路径用 \"英文双引号\" 包裹")
		logPrintln("  · 本脚本之前建库扫描目录是 D:\\BaiduNetdiskDownload\\图纸\\图纸\\，请从该目录选文件")
		logPrintln("  · 如果提示 \"查询 PDF 不存在\"，请先在资源管理器里确认文件路径真的存在")
		logPrintln()
		logPrintln("环境变量:")
		logPrintln("  PROJECT_ROOT = 强制指定项目根目录（含 go.mod）")
		logPrintln("  TOP_K        = 默认 10，返回最相似的前 N 个")
		if logFile != nil {
			exe, _ := os.Executable()
			logPrintf("\n💡 完整日志已保存到: %s\\search-similar.log\n", filepath.Dir(exe))
			logFile.Close()
		}
		os.Exit(1)
	}
	queryPdf := os.Args[1]
	abs, err := filepath.Abs(queryPdf)
	if err == nil {
		queryPdf = abs
	}
	logPrintf("🔍 用户传入 PDF 路径 (解析为绝对路径): %s\n", queryPdf)
	if st, err := os.Stat(queryPdf); err != nil {
		logPrintln()
		logPrintln("────────────────────────────────────────────────────────────")
		logPrintf("❌ 查询 PDF 不存在: %s\n", queryPdf)
		logPrintf("   系统错误: %v\n", err)
		logPrintln()
		logPrintln("🛠️  排查建议:")
		logPrintln("   1) 打开资源管理器，直接把 PDF 文件拖进 CMD 窗口（会自动生成正确的绝对路径+引号）")
		logPrintln("   2) 检查是否把目录写成了 D:\\图纸\\... —— 此前建库目录实际是 D:\\BaiduNetdiskDownload\\图纸\\图纸\\")
		logPrintln("   3) 含中文括号 ( ) 、空格 必须用 \"英文双引号\" 包住路径，如:")
		logPrintln(`      .\search-similar.exe "D:\BaiduNetdiskDownload\ph2\8-35ZKG2（-0.1，DN150JC）(1).pdf" 15`)
		logPrintln("   4) 请复制上面这条命令在 ph2 目录下找一个真实存在的 PDF 测试")
		logPrintln("────────────────────────────────────────────────────────────")
		if logFile != nil {
			exe, _ := os.Executable()
			logPrintf("\n💡 完整日志已保存到: %s\\search-similar.log\n", filepath.Dir(exe))
			logFile.Close()
		}
		os.Exit(1)
	} else {
		logPrintf("   ✅ 文件存在，大小: %.2f MB\n", float64(st.Size())/1024/1024)
	}

	topK := 10
	if len(os.Args) >= 3 {
		n, err := fmt.Sscanf(os.Args[2], "%d", &topK)
		if err != nil || n != 1 || topK <= 0 {
			logPrintf("   ⚠️  TopK 参数 %q 无效，回退使用默认值 10\n", os.Args[2])
			topK = 10
		} else {
			logPrintf("📊 TopK = %d\n", topK)
		}
	} else {
		logPrintf("📊 TopK = %d (默认)\n", topK)
	}

	// 1. 路径 + DB + ONNX 初始化
	logPrintln()
	logPrintln("━━━ 阶段 1/4: 初始化环境 ━━━")
	if err := initPaths(); err != nil {
		fatalf("路径初始化失败: %v", err)
	}

	db, err := storm.Open(dbPath)
	if err != nil {
		fatalf("打开 drawings.db 失败: %v", err)
	}
	defer db.Close()
	total, _ := db.Select().Count(&DrawingFeature{})
	logPrintf("   ✅ drawings.db 已打开，当前向量库: %d 条记录\n", total)

	if _, err := os.Stat(onnxDllPath); err != nil {
		fatalf("找不到 onnxruntime.dll: %v", err)
	}
	if _, err := os.Stat(modelPath); err != nil {
		fatalf("找不到 ONNX 模型: %v", err)
	}
	logPrintln("   → 正在加载 ONNX Runtime ...")
	ort.SetSharedLibraryPath(onnxDllPath)
	if err := ort.InitializeEnvironment(); err != nil {
		fatalf("初始化 ONNX Runtime 失败: %v", err)
	}
	defer ort.DestroyEnvironment()

	inputInfos, outputInfos, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		fatalf("探测 ONNX 模型输入输出失败: %v", err)
	}
	if len(inputInfos) == 0 || len(outputInfos) == 0 {
		fatalf("ONNX 模型输入输出为空")
	}
	inputName := inputInfos[0].Name
	outputName := outputInfos[0].Name
	logPrintf("🧠 ONNX 模型 — 输入: %s shape=%v, 输出: %s shape=%v\n",
		inputName, inputInfos[0].Dimensions, outputName, outputInfos[0].Dimensions)

	// 2. 提取查询 PDF 的特征向量
	logPrintln()
	logPrintln("━━━ 阶段 2/4: 提取查询 PDF 特征向量 ━━━")
	logPrintf("📄 PDF 文件: %s\n", filepath.Base(queryPdf))
	logPrintln("   → 正在调用 Poppler 将第 1 页转 PNG (300 DPI) ...")
	tempDir, err := os.MkdirTemp("", "query_img_*")
	if err != nil {
		fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pngPath, err := convertPdfToPng(queryPdf, tempDir, 0)
	if err != nil {
		fatalf("查询 PDF 转图失败: %v", err)
	}
	logPrintln("   → 正在运行 ONNX 推理提取 1000 维向量 (ResNet18) ...")
	queryVec, err := extractFeatureVector(pngPath, inputName, outputName, outputInfos[0])
	os.Remove(pngPath)
	if err != nil {
		fatalf("查询 PDF 特征提取失败: %v", err)
	}
	logPrintf("   ✅ 特征提取完成，向量维度: %d\n", len(queryVec))

	// 3. 读取 DB 全部记录并计算相似度
	logPrintln()
	logPrintln("━━━ 阶段 3/4: 全库相似度计算 ━━━")
	logPrintf("📚 向量库: %d 条，算法: 余弦相似度 ...\n", total)
	start := time.Now()

	var all []DrawingFeature
	if err := db.All(&all); err != nil {
		fatalf("读取 DB 失败: %v", err)
	}

	results := make([]SearchResult, 0, len(all))
	selfFile := strings.ToLower(queryPdf)
	for _, r := range all {
		if strings.ToLower(r.FilePath) == selfFile {
			continue
		}
		sim := cosineSimilarity(queryVec, r.FeatureVector)
		results = append(results, SearchResult{
			FileName:   r.FileName,
			FilePath:   r.FilePath,
			Similarity: sim,
			UpdatedAt:  r.UpdatedAt,
		})
	}

	// 4. 按相似度降序排序取 Top-K
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	if len(results) > topK {
		results = results[:topK]
	}
	logPrintf("   ✅ 计算完成，总耗时: %v\n\n", time.Since(start))

	// 5. 打印漂亮的 Top-K 表格
	logPrintln("━━━ 阶段 4/4: Top-K 检索结果 ━━━")
	logPrintln("═══════════════════════════════════════════════════════════════════════════")
	logPrintf("🔝  Top-%d 相似图纸检索结果 (查询: %s)\n", topK, filepath.Base(queryPdf))
	logPrintln("───────────────────────────────────────────────────────────────────────────")
	logPrintf("%-4s  %-8s  %-50s  %s\n", "排名", "相似度", "文件名", "更新时间")
	logPrintln("──────  ────────  ────────────────────────────────────────────────────────  ───────────────────")
	for i, r := range results {
		star := "  "
		if strings.Contains(strings.ToLower(r.FileName), "zkg2") ||
			strings.Contains(strings.ToLower(filepath.Base(queryPdf)), strings.TrimSuffix(strings.ToLower(r.FileName), ".pdf")) {
			star = "⭐"
		}
		logPrintf("%-4s %-5s %6.2f%%   %-50s  %s\n",
			fmt.Sprintf("#%d", i+1),
			star,
			r.Similarity*100,
			truncate(r.FileName, 50),
			r.UpdatedAt.Format("2006-01-02 15:04"),
		)
		logPrintf("        路径: %s\n", r.FilePath)
	}
	logPrintln("═══════════════════════════════════════════════════════════════════════════")

	// 6. 诊断：检查 Top-1 是否足够相似
	if len(results) > 0 {
		topSim := results[0].Similarity
		logPrintf("\n📊 诊断结论:\n")
		switch {
		case topSim >= 0.95:
			logPrintf("   ✅ 极强匹配（%.2f%%）：Top-1 很可能是相同/高度相似的图纸\n", topSim*100)
		case topSim >= 0.85:
			logPrintf("   🟢 强匹配（%.2f%%）：Top-1 大概率是同系列/相似图纸\n", topSim*100)
		case topSim >= 0.70:
			logPrintf("   🟡 中等匹配（%.2f%%）：Top-1 有一定相似度，需要人工确认\n", topSim*100)
		default:
			logPrintf("   🔴 弱匹配（%.2f%%）：库中未找到高度相似的图纸，可能是新图纸或未入库\n", topSim*100)
		}
		queryBase := strings.ToLower(filepath.Base(queryPdf))
		zkg2Found := false
		for _, r := range results {
			if strings.Contains(strings.ToLower(r.FileName), "zkg2") {
				zkg2Found = true
				break
			}
		}
		if strings.Contains(queryBase, "zkg2") && !zkg2Found {
			logPrintf("   ⚠️  查询文件名含 ZKG2，但 Top-%d 中未出现 ZKG2 条目，请检查是否入库完整\n", topK)
		} else if strings.Contains(queryBase, "zkg2") && zkg2Found {
			logPrintf("   ✅ 查询文件名含 ZKG2，Top-%d 中已出现 ZKG2 相关条目\n", topK)
		}
	}
	if logFile != nil {
		exe, _ := os.Executable()
		logPrintf("\n💡 完整日志已保存到: %s\\search-similar.log\n", filepath.Dir(exe))
		logFile.Close()
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
