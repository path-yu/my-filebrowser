//go:build drawingsearch
// +build drawingsearch

package fbhttp

// Drawing vector search: POST /api/search/similar-pdf (multipart/form-data)
// - Requires CGO + MinGW-w64 + onnxruntime.dll + Poppler.
// - Enabled by building with: go build -tags drawingsearch
// - Otherwise drawingsearch_off.go is compiled and returns a clean "not enabled" error.

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // Go 1.0+ 即位于标准库
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/gorilla/mux"
	ort "github.com/yalue/onnxruntime_go"
	// 非标准库内的图像格式（兼容性更好：不依赖本机 Go 安装是否带对应子包）
	_ "golang.org/x/image/bmp" // 原 import 了 std 的 image/bmp，在部分 Go 版本/精简安装中会报 not in std
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// --------------- DrawingFeature (与 cmd/transform-pdf、cmd/search-similar 完全一致) ---------------

type DrawingFeature struct {
	FilePath      string    `storm:"id"`
	FileName      string    `storm:"index"`
	FeatureVector []float32 `storm:"inline"`
	CreatedAt     time.Time `storm:"index"`
	UpdatedAt     time.Time
}

// DrawingSearchResult 返回给前端的单条相似结果
type DrawingSearchResult struct {
	FilePath   string  `json:"path"`       // PDF 绝对路径
	FileName   string  `json:"name"`       // 文件名（含 .pdf）
	Similarity float64 `json:"similarity"` // 0~1 余弦相似度
	Dir        bool    `json:"dir"`        // 兼容搜索结果 UI（固定 false）
	Size       int64   `json:"size"`       // 文件大小（若能 stat）
	Modified   string  `json:"modified"`   // RFC3339 修改时间
}

// drawingSearchResponseBody 完整响应
type drawingSearchResponseBody struct {
	Query     string                `json:"query"`     // 上传的文件名
	TotalInDB int                   `json:"totalInDB"` // 向量库总记录数
	Results   []DrawingSearchResult `json:"results"`   // Top-K 结果（已按相似度降序）
	Diagnosis string                `json:"diagnosis"` // 诊断说明文字
	Elapsed   string                `json:"elapsed"`   // 总耗时
}

const (
	drawingImgWidth  = 224
	drawingImgHeight = 224
)

// --------------- 环境路径（与 cmd/* 一致）---------------

func drawingResolveProjectRoot() (string, error) {
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
	return wd, nil
}

// --------------- ONNX Runtime 单例（只初始化一次，多请求并发复用）---------------

type drawingRuntime struct {
	initOnce sync.Once
	initErr  error

	projectRoot  string
	devFilesDir  string
	dbPath       string
	onnxDllPath  string
	modelPath    string
	pdfToPpmPath string // 若为空字符串 = Poppler 未安装，仅 PDF 上传路径会报错（纯图片不受影响）

	pdfWarn     string // 非空时说明 Poppler/pdftoppm 不可用（作为诊断信息返回给前端，而非 init 直接失败）
	inputName   string
	outputName  string
	outputShape ort.InputOutputInfo
}

// 支持的图片扩展名（小写，含点）
var supportedImageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
	".bmp": true, ".tif": true, ".tiff": true, ".gif": true,
}

func isPdfExt(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".pdf")
}
func isImageExt(name string) bool {
	return supportedImageExts[strings.ToLower(filepath.Ext(name))]
}
func isSupportedUploadExt(name string) bool {
	return isPdfExt(name) || isImageExt(name)
}

var drt = &drawingRuntime{}

func (r *drawingRuntime) ensureReady() error {
	r.initOnce.Do(func() {
		root, err := drawingResolveProjectRoot()
		if err != nil {
			r.initErr = err
			return
		}
		r.projectRoot = filepath.Clean(root)
		r.devFilesDir = filepath.Join(r.projectRoot, "dev-files")
		r.dbPath = filepath.Join(r.projectRoot, "drawings.db")
		r.onnxDllPath = filepath.Join(r.devFilesDir, "onnxruntime.dll")
		r.modelPath = filepath.Join(r.devFilesDir, "resnet18_features.onnx")
		r.pdfToPpmPath = filepath.Join(r.devFilesDir, "poppler-25.12.0", "Library", "bin", "pdftoppm.exe")

		// 检查 drawings.db（允许不存在 → 接口返回空 + 诊断说明）
		// 检查 ONNX dll + model（必须存在）
		if _, err := os.Stat(r.onnxDllPath); err != nil {
			r.initErr = fmt.Errorf("缺少 onnxruntime.dll (%s): %w。请将 Microsoft.ML.OnnxRuntime 包内的 onnxruntime.dll 放到 dev-files/ 目录下。", r.onnxDllPath, err)
			return
		}
		if _, err := os.Stat(r.modelPath); err != nil {
			r.initErr = fmt.Errorf("缺少 ONNX 模型 (%s): %w。请将 ResNet18 导出的 resnet18_features.onnx 放到 dev-files/ 目录下。", r.modelPath, err)
			return
		}
		// pdftoppm：仅 PDF → PNG 转图需要。纯图片上传不依赖，因此缺失仅记 warning，不中断初始化。
		// 同时尝试兼容 poppler-25.12.0/bin（用户可能直接放了 bin 目录）
		altPath := filepath.Join(r.devFilesDir, "poppler-25.12.0", "bin", "pdftoppm.exe")
		if _, primaryErr := os.Stat(r.pdfToPpmPath); primaryErr != nil {
			// altPath 回退探测：用 else 分支持有 if-init 作用域里的变量，避免 undefined
			if _, fallbackErr := os.Stat(altPath); fallbackErr == nil {
				r.pdfToPpmPath = altPath
			} else {
				r.pdfWarn = fmt.Sprintf("未找到 pdftoppm.exe（%s：%v；尝试回退 %s：%v）。PDF 上传将不可用，但图片上传不受影响。", r.pdfToPpmPath, primaryErr, altPath, fallbackErr)
				r.pdfToPpmPath = ""
			}
		}

		ort.SetSharedLibraryPath(r.onnxDllPath)
		if err := ort.InitializeEnvironment(); err != nil {
			r.initErr = fmt.Errorf("初始化 ONNX Runtime 失败: %w", err)
			return
		}

		inputInfos, outputInfos, err := ort.GetInputOutputInfo(r.modelPath)
		if err != nil {
			r.initErr = fmt.Errorf("探测 ONNX 模型输入输出失败: %w", err)
			return
		}
		if len(inputInfos) == 0 || len(outputInfos) == 0 {
			r.initErr = errors.New("ONNX 模型输入输出为空")
			return
		}
		r.inputName = inputInfos[0].Name
		r.outputName = outputInfos[0].Name
		r.outputShape = outputInfos[0]
	})
	return r.initErr
}

// --------------- PDF -> PNG -> Feature vector ---------------

func (r *drawingRuntime) convertPdfFirstPageToPng(pdfPath, outDir string) (string, error) {
	outPrefix := filepath.Join(outDir, "page")
	cmd := exec.Command(r.pdfToPpmPath, "-png", "-r", "300", "-f", "1", "-l", "1", pdfPath, outPrefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pdftoppm 失败: %w, 输出: %s", err, string(out))
	}
	return outPrefix + "-1.png", nil
}

func drawingPreprocessImage(imgPath string) ([]float32, error) {
	f, err := os.Open(imgPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	srcImg, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("解码图片失败 (支持 JPG/PNG/WebP/BMP/TIFF/GIF): %w", err)
	}

	// 先把源图画到白底的 RGBA 上：
	//   - 透明 PNG/WebP/GIF/TIFF（alpha < 255）在 Resize 时若直接按 RGBA At() 读会得到(0,0,0) → 黑底
	//   - 工程 PDF 转图/扫描件一般都在白底上，统一贴白底能让特征更稳定。
	bounds := srcImg.Bounds()
	whiteBase := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(whiteBase, whiteBase.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(whiteBase, whiteBase.Bounds(), srcImg, bounds.Min, draw.Over)
	srcImg = whiteBase

	dstImg := image.NewRGBA(image.Rect(0, 0, drawingImgWidth, drawingImgHeight))
	draw.BiLinear.Scale(dstImg, dstImg.Bounds(), srcImg, srcImg.Bounds(), draw.Over, nil)

	mean := []float32{0.485, 0.456, 0.406}
	std := []float32{0.229, 0.224, 0.225}
	tensorData := make([]float32, 1*3*drawingImgHeight*drawingImgWidth)
	stride := drawingImgHeight * drawingImgWidth
	for y := 0; y < drawingImgHeight; y++ {
		for x := 0; x < drawingImgWidth; x++ {
			r, g, b, _ := dstImg.At(x, y).RGBA()
			rf := float32(r>>8) / 255.0
			gf := float32(g>>8) / 255.0
			bf := float32(b>>8) / 255.0
			rf = (rf - mean[0]) / std[0]
			gf = (gf - mean[1]) / std[1]
			bf = (bf - mean[2]) / std[2]
			idx := y*drawingImgWidth + x
			tensorData[0*stride+idx] = rf
			tensorData[1*stride+idx] = gf
			tensorData[2*stride+idx] = bf
		}
	}
	return tensorData, nil
}

func (r *drawingRuntime) extractFeature(imgPath string) ([]float32, error) {
	tensorData, err := drawingPreprocessImage(imgPath)
	if err != nil {
		return nil, err
	}
	inputShape := ort.NewShape(1, 3, drawingImgHeight, drawingImgWidth)
	inputTensor, err := ort.NewTensor(inputShape, tensorData)
	if err != nil {
		return nil, err
	}
	defer inputTensor.Destroy()

	var outputShape ort.Shape
	if len(r.outputShape.Dimensions) == 2 {
		d0 := int64(r.outputShape.Dimensions[0])
		if d0 <= 0 {
			d0 = 1
		}
		d1 := int64(r.outputShape.Dimensions[1])
		if d1 <= 0 {
			d1 = 1000
		}
		outputShape = ort.NewShape(d0, d1)
	} else {
		outputShape = ort.NewShape(1, 1000)
	}
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, err
	}
	defer outputTensor.Destroy()

	session, err := ort.NewAdvancedSession(
		r.modelPath,
		[]string{r.inputName},
		[]string{r.outputName},
		[]ort.Value{inputTensor},
		[]ort.Value{outputTensor},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("创建 ONNX Session 失败: %w", err)
	}
	defer session.Destroy()
	if err := session.Run(); err != nil {
		return nil, fmt.Errorf("ONNX 推理失败: %w", err)
	}
	return outputTensor.GetData(), nil
}

// --------------- 余弦相似度 ---------------

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

// --------------- HTTP Handler ---------------

const drawingSearchMaxFileBytes = 200 * 1024 * 1024 // 上传文件最大 200MB（PDF/图片共用）

var similarPdfSearchHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	start := time.Now()
	if err := drt.ensureReady(); err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("向量检索环境未就绪: %w", err)
	}

	// 1. 解析 multipart/form-data: field="file"，支持 .pdf + 常见图片扩展名
	if err := r.ParseMultipartForm(drawingSearchMaxFileBytes); err != nil {
		return http.StatusBadRequest, fmt.Errorf("解析 multipart 失败 (请上传 <200MB 的 PDF 或图片): %w", err)
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("读取 file 字段失败: %w", err)
	}
	defer file.Close()
	uploadName := header.Filename
	if !isSupportedUploadExt(uploadName) {
		return http.StatusBadRequest, fmt.Errorf("文件类型不支持 (%q)：仅支持 PDF 或图片（JPG/PNG/WebP/BMP/TIFF/GIF）", filepath.Ext(uploadName))
	}

	// 2. 把上传文件落到临时目录（保留原扩展名，便于后续 image.Decode 识别 / pdftoppm 处理）
	tempDir, err := os.MkdirTemp("", "similar-drawing-*")
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)
	ext := strings.ToLower(filepath.Ext(uploadName))
	if ext == "" {
		ext = ".bin"
	}
	tempInPath := filepath.Join(tempDir, "upload"+ext)
	dst, err := os.Create(tempInPath)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("写临时文件失败: %w", err)
	}
	n, copyErr := io.Copy(dst, io.LimitReader(file, drawingSearchMaxFileBytes))
	closeErr := dst.Close()
	if copyErr != nil {
		return http.StatusInternalServerError, fmt.Errorf("保存上传文件失败: %w", copyErr)
	}
	if closeErr != nil {
		return http.StatusInternalServerError, fmt.Errorf("关闭临时文件失败: %w", closeErr)
	}
	if n == 0 {
		return http.StatusBadRequest, errors.New("上传的文件为空")
	}

	// 3. 分支：PDF → pdftoppm → PNG → 特征向量；图片 → 直接提取特征向量
	var queryVec []float32
	if isPdfExt(uploadName) {
		if drt.pdfToPpmPath == "" {
			msg := "PDF 上传路径未启用：未找到 pdftoppm.exe（Poppler）。"
			if drt.pdfWarn != "" {
				msg += " " + drt.pdfWarn
			}
			return http.StatusUnprocessableEntity, errors.New(msg)
		}
		pngPath, convErr := drt.convertPdfFirstPageToPng(tempInPath, tempDir)
		if convErr != nil {
			return http.StatusUnprocessableEntity, fmt.Errorf("PDF 转图失败: %w", convErr)
		}
		vec, e2 := drt.extractFeature(pngPath)
		if e2 != nil {
			return http.StatusUnprocessableEntity, fmt.Errorf("特征提取失败 (PDF→PNG 后): %w", e2)
		}
		queryVec = vec
	} else {
		vec, e2 := drt.extractFeature(tempInPath)
		if e2 != nil {
			return http.StatusUnprocessableEntity, fmt.Errorf("图片特征提取失败: %w", e2)
		}
		queryVec = vec
	}

	// 4. Top-K 参数（可选，默认 10）
	topK := 10
	if kStr := strings.TrimSpace(r.FormValue("k")); kStr != "" {
		var k int
		if _, err := fmt.Sscanf(kStr, "%d", &k); err == nil && k > 0 && k <= 100 {
			topK = k
		}
	}

	// 5. 打开 drawings.db（storm.Open 每次打开文件锁，用完 Close；
	//    因为数据库更新频率低，单请求打开成本远小于并发冲突）
	db, err := storm.Open(drt.dbPath)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("打开 drawings.db 失败 (%s): %w", drt.dbPath, err)
	}
	defer db.Close()
	total, _ := db.Select().Count(&DrawingFeature{})
	if total == 0 {
		body := drawingSearchResponseBody{
			Query:     uploadName,
			TotalInDB: 0,
			Results:   []DrawingSearchResult{},
			Diagnosis: "向量库 drawings.db 中暂无记录，请先运行 cmd/transform-pdf 建库。",
			Elapsed:   time.Since(start).String(),
		}
		writeJSON(w, http.StatusOK, body)
		return 0, nil
	}

	var all []DrawingFeature
	if err := db.All(&all); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("读取向量库失败: %w", err)
	}

	// 6. 计算相似度 + Top-K 排序
	type pair struct {
		feat DrawingFeature
		sim  float64
	}
	pairs := make([]pair, 0, len(all))
	for _, f := range all {
		sim := cosineSimilarity(queryVec, f.FeatureVector)
		pairs = append(pairs, pair{feat: f, sim: sim})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].sim > pairs[j].sim
	})
	if len(pairs) > topK {
		pairs = pairs[:topK]
	}

	// 7. 组装结果：补 size/modified（若路径可 stat）
	results := make([]DrawingSearchResult, 0, len(pairs))
	for _, p := range pairs {
		item := DrawingSearchResult{
			FilePath:   p.feat.FilePath,
			FileName:   p.feat.FileName,
			Similarity: p.sim,
			Dir:        false,
			Modified:   p.feat.UpdatedAt.UTC().Format(time.RFC3339Nano),
		}
		if st, stErr := os.Stat(p.feat.FilePath); stErr == nil && !st.IsDir() {
			item.Size = st.Size()
			item.Modified = st.ModTime().UTC().Format(time.RFC3339Nano)
		}
		results = append(results, item)
	}

	// 8. 诊断说明文字（附加 pdfWarn：提示用户 PDF 上传能力是否缺失）
	var diagnosis string
	if len(results) > 0 {
		topSim := results[0].Similarity
		switch {
		case topSim >= 0.95:
			diagnosis = fmt.Sprintf("极强匹配（%.2f%%）：Top-1 很可能是相同/高度相似的图纸", topSim*100)
		case topSim >= 0.85:
			diagnosis = fmt.Sprintf("强匹配（%.2f%%）：Top-1 大概率是同系列/相似图纸", topSim*100)
		case topSim >= 0.70:
			diagnosis = fmt.Sprintf("中等匹配（%.2f%%）：需要人工确认", topSim*100)
		default:
			diagnosis = fmt.Sprintf("弱匹配（%.2f%%）：库中未找到高度相似的图纸，可能是新图纸或未入库", topSim*100)
		}
	} else {
		diagnosis = "未返回结果"
	}
	if drt.pdfWarn != "" {
		diagnosis += " ⚠【环境提示】" + drt.pdfWarn
	}

	body := drawingSearchResponseBody{
		Query:     uploadName,
		TotalInDB: total,
		Results:   results,
		Diagnosis: diagnosis,
		Elapsed:   time.Since(start).String(),
	}
	writeJSON(w, http.StatusOK, body)
	return 0, nil
})

// attachDrawingSearchRouter 在启用 -tags drawingsearch 时注册实际的相似 PDF 检索路由。
// 未启用时 drawingsearch_off.go 的同名函数会被编译。
func attachDrawingSearchRouter(r *mux.Router, monkey func(fn handleFunc, prefix string) http.Handler) {
	// 精确路由 + 空前缀，避免 stripPrefix 对 POST 请求做 301 重定向
	r.Handle("/search/similar-pdf", monkey(similarPdfSearchHandler, "")).Methods("POST")
}
