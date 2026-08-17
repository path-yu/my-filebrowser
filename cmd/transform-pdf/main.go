package main

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	// 按用户要求：用 filebrowser 项目在用的本地 BoltDB (storm ORM)，
	// 完全抛弃 SQLite (modernc.org/sqlite / mattn/go-sqlite3)。
	// storm 是纯 Go 实现的 BoltDB 包装，不需要 CGO 与 gcc。
	"github.com/asdine/storm/v3"
	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/image/draw"
)

// DrawingFeature BoltDB (storm) 里的表结构：一个 PDF 图纸对应一条记录。
// FilePath 作为唯一主键：同一文件重跑会自动 Upsert 更新向量。
type DrawingFeature struct {
	FilePath      string    `storm:"id"`     // 绝对路径当主键（唯一索引天然保证）
	FileName      string    `storm:"index"`  // 文件名单独建索引，便于 "按文件名批量查询"
	FeatureVector []float32 `storm:"inline"` // 512/1000 维向量，storm 自动 gob 序列化存入 BoltDB
	CreatedAt     time.Time `storm:"index"`  // 创建时间索引，便于按时间范围取最新入库的图纸
	UpdatedAt     time.Time                  // 本次跑批的更新时间
}

const (
	ImgWidth  = 224
	ImgHeight = 224
)

// resolveProjectRoot 动态计算"项目根目录"（即 go.mod 所在的目录），
// 优先级：
//   1. 环境变量 PROJECT_ROOT（手动指定最优先）
//   2. os.Executable() 所在目录逐层向上，找到包含 "go.mod" 的目录
//   3. 当前工作目录 os.Getwd()（找不到 go.mod 时回退，并打印警告）
// 这样所有路径都是"相对项目根目录稳定拼接"，项目搬到任何目录都能找到 dev-files / drawings.db，
// 避免任何 "D:\code\xxx" 硬编码路径导致换机器/改目录就失效。
func resolveProjectRoot() (string, error) {
	// 1) 环境变量优先级最高
	if p := strings.TrimSpace(os.Getenv("PROJECT_ROOT")); p != "" {
		abs, err := filepath.Abs(p)
		if err == nil {
			if _, staterr := os.Stat(filepath.Join(abs, "go.mod")); staterr == nil {
				return abs, nil
			}
			// 没 go.mod 也接受，用户显式指定了就信用户
			return abs, nil
		}
	}

	// 2) 从可执行文件位置往上爬，找 go.mod
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		for {
			if _, staterr := os.Stat(filepath.Join(dir, "go.mod")); staterr == nil {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir { // 爬到盘符根了，没找到
				break
			}
			dir = parent
		}
	}

	// 3) 回退：当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("无法定位项目根目录（PROJECT_ROOT/Executable/Getwd 均失败: %w）", err)
	}
	// 若 wd 没有 go.mod，说明用户在别的目录启动了 exe，只做 warning
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); errors.Is(err, os.ErrNotExist) {
		fmt.Printf("⚠️  无法定位 go.mod，回退使用当前工作目录 %s 作为项目根。建议显式设置 PROJECT_ROOT 环境变量以避免路径错配。\n", wd)
	}
	return wd, nil
}

// 运行时解析得到的全局绝对路径基准（均由项目根 + 相对片段拼接而成）
var (
	projectRoot string
	devFilesDir string

	dbPath       string
	onnxDllPath  string
	modelPath    string
	pdfToPpmPath string
	defaultScanDir string
)

// initPaths 在 main 函数开头调用一次，把所有"资源路径"解析为绝对路径，
// 并打印解析结果，便于排查"找不到 dev-files 下 xxx"类问题。
func initPaths() error {
	root, err := resolveProjectRoot()
	if err != nil {
		return err
	}
	projectRoot = filepath.Clean(root)
	devFilesDir = filepath.Join(projectRoot, "dev-files")

	defaultScanDir = devFilesDir                    // 默认扫描 dev-files 下的 PDF
	dbPath       = filepath.Join(projectRoot, "drawings.db")       // drawings.db 放在项目根，和 filebrowser.db 平级
	onnxDllPath  = filepath.Join(devFilesDir, "onnxruntime.dll")
	modelPath    = filepath.Join(devFilesDir, "resnet18_features.onnx")
	pdfToPpmPath = filepath.Join(devFilesDir, "poppler-25.12.0", "Library", "bin", "pdftoppm.exe")

	fmt.Println("📦 项目根目录:", projectRoot)
	fmt.Println("📁 dev-files  :", devFilesDir)
	fmt.Println("💾 drawings.db:", dbPath)
	fmt.Println("🟢 onnxruntime:", onnxDllPath)
	fmt.Println("🧠 ONNX 模型  :", modelPath)
	fmt.Println("🖨️  pdftoppm   :", pdfToPpmPath)
	return nil
}

func main() {
	// 0. 先做路径解析：把项目根目录、dev-files、dll、模型、poppler 全部按
	//    "相对项目根"的方式拼出绝对路径，避免硬编码 D:\code\xxx 导致换位置失效。
	if err := initPaths(); err != nil {
		log.Fatalf("路径初始化失败: %v", err)
	}

	// 允许环境变量覆盖扫描目录（方便复用工具：想扫其他目录时 set DRAWINGS_DIR=xxx）
	scanDir := strings.TrimSpace(os.Getenv("DRAWINGS_DIR"))
	if scanDir == "" {
		scanDir = defaultScanDir
	}
	// 把传入的扫描目录也 canonicalize 成绝对路径（避免用户传相对路径时，BoltDB 主键 FilePath 不一致）
	if abs, err := filepath.Abs(scanDir); err == nil {
		scanDir = abs
	}

	// 1. 初始化本地 BoltDB（storm 封装，和 filebrowser 主程序同款）
	db, err := storm.Open(dbPath)
	if err != nil {
		log.Fatalf("初始化 drawings.db (BoltDB) 失败: %v", err)
	}
	defer db.Close()

	// 初始化桶 / 建索引（storm 的 Init 是幂等的，重复调用无害）
	if err := db.Init(&DrawingFeature{}); err != nil {
		log.Fatalf("初始化 DrawingFeature bucket 失败: %v", err)
	}

	// 2. 初始化 ONNX Runtime 环境（会通过 syscall 动态加载 onnxruntime.dll）
	if _, err := os.Stat(onnxDllPath); err != nil {
		log.Fatalf("找不到 onnxruntime.dll (%s)：请将 Microsoft.ML.OnnxRuntime 包内的 onnxruntime.dll 放到 dev-files 目录下。\n原始错误: %v", onnxDllPath, err)
	}
	if _, err := os.Stat(modelPath); err != nil {
		log.Fatalf("找不到 resnet18_features.onnx 模型 (%s)：请将训练导出的 ONNX 模型放到 dev-files 目录下。\n原始错误: %v", modelPath, err)
	}
	ort.SetSharedLibraryPath(onnxDllPath)
	if err := ort.InitializeEnvironment(); err != nil {
		log.Fatalf("初始化 ONNX Runtime 环境失败: %v", err)
	}
	defer ort.DestroyEnvironment()

	// 2b. 自动探测 ONNX 模型的真实输入/输出节点名。
	// 原脚本硬编码的 pixel_values / image_embeds 只对应 CLIP 风格的模型，
	// 如果是普通 torchvision ResNet18 导出的模型，名字通常是 input / onnx::Flatten_0 / layer / output 等，
	// 不匹配会报 "Invalid output name: image_embeds"。
	inputInfos, outputInfos, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		log.Fatalf("探测 ONNX 模型输入输出失败: %v", err)
	}
	if len(inputInfos) == 0 || len(outputInfos) == 0 {
		log.Fatalf("ONNX 模型输入 (%d) 或输出 (%d) 为空，模型文件可能损坏。", len(inputInfos), len(outputInfos))
	}
	inputName := inputInfos[0].Name
	outputName := outputInfos[0].Name
	fmt.Printf("🧠 ONNX 模型自动识别 —— 输入节点: %s (shape=%v, dtype=%v)\n", inputName, inputInfos[0].Dimensions, inputInfos[0].DataType)
	fmt.Printf("🧠 ONNX 模型自动识别 —— 输出节点: %s (shape=%v, dtype=%v)\n", outputName, outputInfos[0].Dimensions, outputInfos[0].DataType)
	if len(outputInfos) > 1 {
		fmt.Printf("⚠️  ONNX 模型有 %d 个输出，默认选取第 1 个作为特征向量。如果需要选其他输出，请修改 main.go 中 outputName 的取值。\n", len(outputInfos))
		for i, o := range outputInfos {
			fmt.Printf("   输出 [%d]: name=%s shape=%v dtype=%v\n", i, o.Name, o.Dimensions, o.DataType)
		}
	}

	// 3. 扫描目标目录下的所有 .pdf 文件
	pdfFiles, err := scanPdfFiles(scanDir)
	if err != nil {
		log.Fatalf("扫描 PDF 文件失败: %v", err)
	}
	fmt.Printf("找到 %d 个 PDF 图纸文件（扫描根目录: %s），开始提取特征...\n", len(pdfFiles), scanDir)
	if len(pdfFiles) == 0 {
		fmt.Println("⚠️  未找到任何 .pdf 文件：请将 PDF 图纸放到 dev-files 目录或通过环境变量 DRAWINGS_DIR 指定正确目录。")
	}

	// 4. 创建工作临时目录用于存放生成的 PNG 图片
	tempDir, err := os.MkdirTemp("", "pdf_images_*")
	if err != nil {
		log.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	start := time.Now()
	successCount := 0

	// 5. 遍历处理每一个 PDF 文件
	for i, pdfPath := range pdfFiles {
		fileName := filepath.Base(pdfPath)
		fmt.Printf("[%d/%d] 正在处理: %s\n", i+1, len(pdfFiles), fileName)

		// Step A: 使用 Poppler 将 PDF 第 1 页转为 300 DPI 高清 PNG
		pngPath, err := convertPdfToPng(pdfPath, tempDir, i)
		if err != nil {
			log.Printf("❌ PDF 转图失败 [%s]: %v", fileName, err)
			continue
		}

		// Step B: 使用 ONNX Runtime 提取 512 维特征向量
		vector, err := extractFeatureVector(pngPath, inputName, outputName, outputInfos[0])
		os.Remove(pngPath) // 无论成功失败，用完即删临时图片
		if err != nil {
			log.Printf("❌ 特征提取失败 [%s]: %v", fileName, err)
			continue
		}

		// Step C: 写入本地 BoltDB (storm)。FilePath 当主键，storm.Save 天然幂等（有则更新，无则插入）。
		now := time.Now()
		rec := &DrawingFeature{
			FilePath:      pdfPath,
			FileName:      fileName,
			FeatureVector: vector,
			UpdatedAt:     now,
		}
		// 如果是首次入库，CreatedAt 也打上同样时间；否则保留旧 CreatedAt。
		var old DrawingFeature
		if lookupErr := db.One("FilePath", pdfPath, &old); lookupErr == nil {
			rec.CreatedAt = old.CreatedAt
		} else {
			rec.CreatedAt = now
		}
		if err := db.Save(rec); err != nil {
			log.Printf("❌ 写入 drawings.db 失败 [%s]: %v", fileName, err)
		} else {
			successCount++
		}
	}

	// 6. 输出汇总 + 简单验证
	fmt.Printf("\n✅ 全量索引建库完成！成功处理 %d/%d 张图纸，总耗时: %v\n", successCount, len(pdfFiles), time.Since(start))
	total, countErr := db.Select().Count(&DrawingFeature{})
	if countErr != nil {
		fmt.Printf("ℹ️  读取 records 总数失败（忽略）: %v\n", countErr)
	} else {
		fmt.Printf("🧾 drawings.db 当前 DrawingFeature 记录总数: %d\n", total)
	}
}

// scanPdfFiles 递归扫描目录下的所有 .pdf 文件。
func scanPdfFiles(dirPath string) ([]string, error) {
	var pdfFiles []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".pdf") {
			pdfFiles = append(pdfFiles, path)
		}
		return nil
	})
	return pdfFiles, err
}

// convertPdfToPng 调用 pdftoppm (Poppler) 将 PDF 第 1 页转为 300 DPI 高清 PNG。
func convertPdfToPng(pdfPath, outputDir string, index int) (string, error) {
	if _, err := os.Stat(pdfToPpmPath); err != nil {
		return "", fmt.Errorf("找不到 pdftoppm.exe (%s): %v，请确认 dev-files\\poppler-25.12.0 目录完整", pdfToPpmPath, err)
	}
	outPrefix := filepath.Join(outputDir, fmt.Sprintf("page_%d", index))
	cmd := exec.Command(pdfToPpmPath, "-png", "-r", "300", "-f", "1", "-l", "1", pdfPath, outPrefix)

	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%v, 命令输出: %s", err, string(output))
	}
	return outPrefix + "-1.png", nil
}

// extractFeatureVector 使用 ONNX Runtime 执行推理，提取 512 维图像特征向量。
// inputName / outputName / outputInfo 由上层调用方通过 ort.GetInputOutputInfo() 在程序启动时一次性探测传入，
// 避免硬编码 "pixel_values / image_embeds" 与用户实际导出的 ResNet18 模型不匹配。
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

	// 输出 shape 通常是 (1, 512)。但模型可能把 batch 标为 dynamic（即 -1），
	// 所以如果 Dimensions 里有 0 或负值（dynamic），就回退使用 NewShape(1, 512)。
	var outputShape ort.Shape
	if len(outputInfo.Dimensions) == 2 {
		dim0 := int64(outputInfo.Dimensions[0])
		if dim0 <= 0 {
			dim0 = 1
		}
		dim1 := int64(outputInfo.Dimensions[1])
		if dim1 <= 0 {
			dim1 = 512
		}
		outputShape = ort.NewShape(dim0, dim1)
	} else {
		outputShape = ort.NewShape(1, 512)
	}
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, err
	}
	defer outputTensor.Destroy()

	session, err := ort.NewAdvancedSession(
		modelPath,
		[]string{inputName},  // 用自动探测得到的真实输入名，而不是硬编码 "pixel_values"
		[]string{outputName}, // 用自动探测得到的真实输出名，而不是硬编码 "image_embeds"
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

// preprocessImage 图像预处理：Resize 到 224x224 + ResNet 标准归一化 (ImageNet mean/std)。
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
