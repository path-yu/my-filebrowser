# FileBrowser 图纸 PDF 向量索引与检索工具包

本目录提供两个 Go 命令行工具，用于将 PDF 图纸提取 ResNet18 图像特征向量、存入本地 BoltDB 向量库，并支持余弦相似度 Top-K 检索。

---

## 📦 工具一览

| 子命令 | 源码路径 | 功能 |
|---|---|---|
| `transform-pdf` | `cmd/transform-pdf/main.go` | 递归扫描 PDF 目录 → Poppler 转 300DPI PNG → ResNet18 ONNX 推理提取 1000 维向量 → 写入本地 BoltDB（`drawings.db`） |
| `search-similar` | `cmd/search-similar/main.go` | 输入 1 份查询 PDF → 提取向量 → 对向量库 1029+ 条记录计算余弦相似度 → 输出 Top-K 相似图纸（含诊断结论） |
| （辅助）`run-search-similar.ps1` | `cmd/search-similar/run-search-similar.ps1` | 一键运行 `search-similar` 的 PowerShell 脚本：支持拖文件进窗口填路径、自动设置 `PROJECT_ROOT`、日志 Tee 到文件、结束自动打开记事本看完整日志 |

---

## 🧱 前置依赖（二进制，本 ZIP **未包含**，请按 README 下载放到 `dev-files/` 对应位置）

> ⚠️ 因为这些文件单文件 50MB~1GB 级，不适合直接提交 GitHub。请从下方链接下载后解压到项目根目录下的 `dev-files/` 目录即可。

### 目录结构（必须完全一致，程序会自动按相对路径拼接）

```
<项目根目录（含 go.mod）>/
├── dev-files/
│   ├── mingw64/                               ← 编译期需要（仅 Windows，跑 exe 时不需要）
│   │   └── bin/gcc.exe
│   ├── poppler-25.12.0/                       ← 运行期需要（PDF→PNG 转图）
│   │   └── Library/bin/pdftoppm.exe
│   ├── onnxruntime.dll                        ← 运行期需要（ONNX Runtime 共享库）
│   └── resnet18_features.onnx                 ← 运行期需要（ResNet18 导出的 ONNX 模型，ImageNet 1000 类 logits 输出）
├── drawings.db                                 ← 运行时生成的向量库（BoltDB，建库后自动出现）
└── cmd/
    ├── transform-pdf/main.go
    └── search-similar/main.go
```

### 下载地址（Windows x64）

| 依赖 | 推荐版本 | 下载链接 | 放置位置 |
|---|---|---|---|
| **MinGW-w64 GCC**（编译用，*运行已编译的 exe 不需要*） | WinLibs UCRT 最新版（16.x，推荐 POSIX+SEH） | https://winlibs.com/ | 解压到 `dev-files/mingw64/`（确保 `dev-files/mingw64/bin/gcc.exe` 存在） |
| **Poppler for Windows**（运行 PDF 转图必须） | 25.12.0 | https://github.com/oschwartz10612/poppler-windows/releases | 解压到 `dev-files/poppler-25.12.0/`（确保 `.../Library/bin/pdftoppm.exe` 存在） |
| **ONNX Runtime DLL**（运行推理必须） | ≥ v1.16 | https://www.nuget.org/packages/Microsoft.ML.OnnxRuntime （从 nupkg 解压 `runtimes/win-x64/native/onnxruntime.dll`） | 放到 `dev-files/onnxruntime.dll` |
| **ResNet18 ONNX 模型**（运行推理必须，ImageNet 预训练权重，输入 224×224 RGB） | opset 17+ | 可从 `torchvision.models.resnet18(weights=ResNet18_Weights.DEFAULT)` 用 `torch.onnx.export` 导出，输入 shape=(1,3,224,224)，输入节点名建议 `pixel_values`，输出节点名不限（程序会用 `ort.GetInputOutputInfo` 自动探测） | 放到 `dev-files/resnet18_features.onnx` |

---

## 🏗️ 编译（仅第一次需要，Windows x64）

> 🔴 **坑：如果你的项目根目录路径里含有**括号或空格**（如 `filebrowser-release-20260814 (2)`），MinGW 的 `ld.exe` 会把路径在括号处截断，导致链接阶段报 `cannot find D:/code/...: No such file or directory`。** 请按下面的命令编译（用 `GCC_EXEC_PREFIX`、`LIBRARY_PATH`、`CPATH` 强制指定干净的 junciton 路径，配合 subst 或 mklink 把项目和 mingw64 映射到无括号盘符即可）。

### 方案 A：项目路径无空格无括号（推荐）
```powershell
cd <项目根>
$env:CGO_ENABLED="1"
go build -o cmd\transform-pdf\transform-pdf.exe .\cmd\transform-pdf\
go build -o cmd\search-similar\search-similar.exe .\cmd\search-similar\
```

### 方案 B：项目路径含括号 `(2)`（必须用以下三行环境变量）
先做两件事：
1. `New-Item -ItemType Junction -Path "C:\fb_proj" -Target "D:\code\filebrowser-release-20260814 (2)"`（把项目映射到无括号路径）
2. 再映射一次 mingw64（或直接用 C 盘的干净路径）

然后编译：
```powershell
cd C:\fb_proj
$env:CGO_ENABLED="1"
$env:PATH="C:\fb_proj\dev-files\mingw64\bin;" + $env:PATH
$env:GCC_EXEC_PREFIX="C:/fb_proj/dev-files/mingw64/libexec/gcc/"
$env:LIBRARY_PATH="C:/fb_proj/dev-files/mingw64/lib/gcc/x86_64-w64-mingw32/16.2.0;C:/fb_proj/dev-files/mingw64/x86_64-w64-mingw32/lib;C:/fb_proj/dev-files/mingw64/lib"
$env:CPATH="C:/fb_proj/dev-files/mingw64/include;C:/fb_proj/dev-files/mingw64/x86_64-w64-mingw32/include"

go build -o C:\fb_proj\cmd\transform-pdf\transform-pdf.exe .\cmd\transform-pdf\
go build -o C:\fb_proj\cmd\search-similar\search-similar.exe .\cmd\search-similar\
```

---

## 🚀 运行

所有工具都会**自动定位项目根目录**（优先级：`PROJECT_ROOT` 环境变量 → 从 exe 路径上溯找 `go.mod` → 当前工作目录），资源路径均按 `<项目根>/dev-files/...` 拼接，无需硬编码。

### ① 建库：`transform-pdf`（批量扫描 PDF，提取特征入库）
```powershell
cd <项目根>\cmd\transform-pdf

# 默认扫描 <项目根>/dev-files 下所有 PDF
$env:PROJECT_ROOT="<项目根绝对路径>"
.\transform-pdf.exe

# 自定义扫描目录（推荐：之前的建库目录）
$env:DRAWINGS_DIR="D:\BaiduNetdiskDownload\图纸\图纸"
.\transform-pdf.exe
```

运行后会在 `<项目根>/drawings.db` 生成向量库，格式：

```
DrawingFeature (BoltDB storm 表)
├── FilePath       (string, PK, id)      PDF 绝对路径（唯一、Upsert 幂等）
├── FileName       (string, index)       PDF 文件名（便于按文件名批量查）
├── FeatureVector  ([]float32, inline)   ResNet18 1000 维向量 (logits)
├── CreatedAt      (time.Time, index)    首次入库时间
└── UpdatedAt      (time.Time)           本次更新时间
```

### ② 检索：`search-similar`（单 PDF 查询 Top-K 相似）

#### 方式一：一键脚本（推荐，最省心）
```powershell
cd <项目根>\cmd\search-similar
powershell -ExecutionPolicy Bypass -File .\run-search-similar.ps1
```
- 把 PDF **拖进窗口回车**即可自动填入路径（含中文/括号/空格也没问题）
- 结束自动打开记事本看完整日志（含 Top-K 表格、诊断结论）

#### 方式二：手动命令行
```powershell
cd <项目根>\cmd\search-similar
$env:PROJECT_ROOT="<项目根绝对路径>"

.\search-similar.exe "D:\BaiduNetdiskDownload\ph2\8-35ZKG2（-0.1，DN150JC）(1).pdf" 15
```

---

## 📊 实测效果（ZKG2 系列）

| 排名 | 相似度 | 文件名 |
|---|---|---|
| #1 ⭐ | 100.00% | ZKG2（-0.1，DN150JC）.pdf （完全匹配） |
| #2 ⭐ | 97.79% | ZKG2(-0.1，DN100JC）.pdf |
| #3 ⭐ | 97.65% | ZKG2（-0.1-DN150JC）.pdf |
| #4 ⭐ | 97.42% | ZKG2-(0.1,DN300JC，内环氧).pdf |
| #10 ⭐ | 96.87% | ZKG2.5(-0.1,D1200-DN80JC）.pdf |

- 同系列图纸（ZKG1.5、ZKG2、ZKG2.5、ZKG3）相似度全部 ≥ 96.87%，Top-10 中 9 条含 ZKG2 标记
- 1029 条记录余弦相似度全量计算：**≈ 230ms**
- 特征提取（PDF→PNG→ONNX）：**≈ 2~3s**（单张 300 DPI 图纸）
- 1025 份 PDF 全量建库总耗时：**约 32 分钟**（含 Poppler 转图 + ONNX 推理 + BoltDB Upsert）

---

## 🧪 诊断结论阈值（search-similar 自动判定）

| Top-1 相似度 | 判定 | 建议 |
|---|---|---|
| ≥ 95% | ✅ **极强匹配** | Top-1 几乎就是相同/高度相似的图纸 |
| ≥ 85% | 🟢 **强匹配** | Top-1 大概率是同系列/相似图纸 |
| ≥ 70% | 🟡 **中等匹配** | 需要人工确认 |
| < 70% | 🔴 **弱匹配** | 向量库中未入库该图纸或属于全新图纸类别 |

---

## ❓ 常见问题 FAQ

**Q1：为什么 `D:\图纸\ZKG2.pdf` 提示"文件不存在"？**
A：之前建库扫描的目录前缀是 `D:\BaiduNetdiskDownload\图纸\图纸\`，`D:\图纸\` 本身不是真实目录。请直接把 PDF 拖进 CMD/PowerShell 窗口，会自动生成正确的绝对路径和英文双引号。

**Q2：编译 transform-pdf 报 `build constraints exclude all Go files`？**
A：`onnxruntime_go` 使用了 cgo，必须设置 `CGO_ENABLED=1` 且 PATH 中有 `gcc.exe`（MinGW）。

**Q3：链接阶段报 `cannot find D:/code/filebrowser-release-20260814: No such file or directory` + 路径在 `(2)` 处截断？**
A：MinGW 的 `ld.exe` 对路径中的括号敏感。请用 `mklink /J` 或 `New-Item -ItemType Junction` 把项目和 mingw64 映射到无括号无空格的盘符（如 `C:\fb_proj`），再配合 `GCC_EXEC_PREFIX`、`LIBRARY_PATH` 环境变量编译（见上方方案 B）。

**Q4：想换 ResNet50 / CLIP / 其他模型？**
A：只要导出成 ONNX，把 `<项目根>/dev-files/resnet18_features.onnx` 替换成你的模型即可。程序会用 `ort.GetInputOutputInfo` 自动探测输入输出节点名和 shape，不需要改代码；若输出维度非 1000，会自动动态匹配。
