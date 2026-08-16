# 文件管理系统

基于 [filebrowser](https://github.com/filebrowser/filebrowser) v2.63.15 深度定制的中文文件管理系统，面向企业内网的文件/图纸检索与分享场景，**4000 条文件列表无卡顿滚动**、**图片预览左右切换**、**全量图片懒加载 + iOS 风格菊花指示器**、**Windows 一键打包为单 exe**。

---

## 目录

- [一、项目亮点（本轮新增）](#一项目亮点本轮新增)
- [二、定制内容清单](#二定制内容清单)
- [三、目录结构](#三目录结构)
- [四、开发环境要求](#四开发环境要求)
- [五、本地开发](#五本地开发)
- [六、构建打包](#六构建打包)
- [七、部署运行](#七部署运行)
- [八、数据库与账号管理](#八数据库与账号管理)
- [九、品牌与自定义 Logo](#九品牌与自定义-logo)
- [十、URL 自动登录](#十url-自动登录)
- [十一、常见问题](#十一常见问题)
- [十二、技术栈与二次开发指引](#十二技术栈与二次开发指引)

---

## 一、项目亮点（本轮新增）

相比上一版的中文 + iOS 风格基础，本轮重点补了 **性能**、**预览体验** 与 **工程化**：

| 能力 | 说明 |
|---|---|
| **列表视图虚拟滚动** | 4000+ 文件的长列表保持 60fps；网格 / 画廊视图保持原渲染逻辑不变（见 `VirtualList.vue`） |
| **图片预览左右切换** | 预览 JPG / PNG / GIF / WebP 等任意图片时，可左右按钮循环切换同目录内所有图片（与音视频共用按钮） |
| **统一图片懒加载** | 全项目所有 `<img>` 已收敛到 `LazyImage.vue` 组件：IntersectionObserver 懒加载 + iOS 风格菊花 + 错误重试 + 首屏 eager 兜底 |
| **Windows 一键打包** | `build-windows.bat`（bat 启动器）+ `build-core.ps1`（PowerShell 核心构建），支持 `skipFrontend` 模式 5 秒出 exe |
| **右键菜单自适应** | 文件右键菜单 / 更多操作菜单会根据页面可视区域动态调整上下左右方向，不会滚出视口 |

---

## 二、定制内容清单

与原版 filebrowser v2.63.15 相比的全部改动（含本轮）：

### 2.1 默认语言（中文）

| 文件 | 改动 |
|---|---|
| `frontend/src/i18n/index.ts` | `detectLocale()` 固定返回 `zh-cn`；`createI18n` 默认 `locale: "zh-cn"` |
| `frontend/src/stores/auth.ts` | `setLocale(user.locale \|\| "zh-cn")`，登录后中文兜底 |
| `settings/storage.go` | 默认 `Defaults.Locale = "zh-cn"` |
| `cmd/root.go` | quickSetup 初始 `Locale: "zh-cn"` |
| `cmd/users.go` | `--locale` flag 默认 `zh-cn` |
| `frontend/src/i18n/zh-cn.json` | 补齐 22 个漏翻译的英文 key（currentPassword、冲突处理、按钮等） |

### 2.2 品牌化（网页标题）

| 文件 | 改动 |
|---|---|
| `frontend/public/index.html` | `<title>`、`apple-mobile-web-app-title`、manifest 名称 → "文件管理系统" |
| `frontend/index.html` | 开发模式入口同步修改 |
| `frontend/src/utils/constants.ts` | `name` 默认值改为系统名称 |
| `frontend/src/components/Sidebar.vue` | 底部 credits 显示系统名称 + 版本 |
| `frontend/src/views/Login.vue` | Logo alt 改为系统名称 |

### 2.3 iOS 风格 UI（核心改动）

| 文件 | 内容 |
|---|---|
| `frontend/src/css/_variables.css` | iOS 配色体系：主色 `#007AFF`、背景 `#F2F2F7`、深色 `#000`/`#1C1C1E` |
| `frontend/src/css/_buttons.css` | 胶囊按钮（`border-radius: 980px`）+ 按压缩放反馈 |
| `frontend/src/css/_inputs.css` | 圆角灰底输入框 + 蓝色聚焦光晕 |
| `frontend/src/css/login.css` | iOS 登录卡片（渐变背景 + 圆角 + 柔和阴影） |
| `frontend/src/css/header.css` | 毛玻璃头部（`backdrop-filter`）+ 标题加粗 |
| `frontend/src/css/_shell.css` | Shell 终端顶部圆角 + 阴影 |
| `frontend/src/css/_share.css` | 分享卡片圆角 14px |
| `frontend/src/css/upload-files.css` | 进度条 iOS 蓝 |
| `frontend/src/css/styles.css` | `.credits` flex 单行布局 |
| **`frontend/src/css/ios.css`（新增 ~900 行）** | 全部 iOS 覆盖：toast、卡片圆角、SegmentedControl、Sheet 弹窗、按压反馈、滚动条、select/checkbox 等 |
| `frontend/src/components/header/HeaderBar.vue` | `<slot />` 包裹 `.slot-wrapper`（顶部按钮单行） |
| `frontend/src/components/DropdownModal.vue` | 下拉菜单 iOS 圆角 |
| `frontend/src/views/files/FileListing.vue` | 视图切换改为 iOS SegmentedControl 三段控件 |

### 2.4 功能定制

| 功能 | 实现 |
|---|---|
| **URL 自动登录** | `frontend/src/views/Login.vue` 新增 `autoLoginFromURL()`：解析 `?u=&p=` 或 `?username=&password=`，自动登录后跳转并清除敏感参数 |
| **去除二次密码校验** | `frontend/src/views/settings/User.vue` `save()`/`deletePrompt()` 不再弹 CurrentPassword；`http/users.go` 三处 `CheckPwd` 校验禁用 |
| **弱密码黑名单移除** | `users/password.go` 移除 `commonPasswords` 检查（允许 123456 等简单密码，最小长度可配） |
| **移除外部链接** | Global 设置页帮助链接、Sidebar GitHub 链接、CustomToast 报告问题按钮全部移除 |
| **自定义 Logo 指引** | 设置页品牌区新增 `logo.svg` 说明（通过 `branding.files` 目录生效） |
| **右键菜单自适应位置** | `FileListing.vue` 打开右键菜单时检测剩余空间，自动翻转 |

### 2.5 性能优化（本轮核心）

#### 2.5.1 列表视图虚拟滚动（VirtualList）

仅 **列表视图** 启用虚拟滚动，网格 / 画廊视图保持原渲染与样式逻辑不变，避免布局变化带来视觉回归。

- 组件：`frontend/src/components/files/VirtualList.vue`
- 使用方：`frontend/src/views/files/FileListing.vue`
- 关键技术点：
  - 支持 `mode="parent"` 父滚动驱动模式（用外层 `#listing` 作为滚动容器，避免双滚动条）
  - `measuredHeights` 缓存已测行高 + `offsets` 前缀和数组，支持**可变行高**
  - `selfOffsetTop` 用滚动容器坐标系计算（BBox 差值 + `container.scrollTop`），已修复「向上滚动元素消失」Bug（需 `#listing` 设置 `position: relative`）
  - `measuredOnce` 锁：首次测量前强制 `startIndex = 0`，避免 BBox 未就绪导致的偏移跳变
  - `buffer=8` 可视区前后各多渲染 8 行，滚动时肉眼无白屏

#### 2.5.2 图片懒加载组件（LazyImage）

所有 `<img>` 统一收敛到 `frontend/src/components/files/LazyImage.vue`：

- **懒加载核心**：IntersectionObserver + `rootMargin: 300px`，进入可视区前 300px 提前触发；离开视图不取消已发起请求（避免网络抖动回流）
- **iOS 风格加载指示器**：12 根径向 bar + 错位 fade 动画（`ios-spinner-fade 1s`，`animation-delay` 负偏移分 12 相位），与 iOS `UIActivityIndicatorView` 视觉一致
- **错误占位 + 重试按钮**：加载失败显示"图片占位图标 + Retry"按钮，点击执行 `reload()`（给 `<img>` 重新换 src + 时间戳）
- **与原生 `<img>` 100% 兼容**：`defineOptions({ inheritAttrs: false })` + `useAttrs` 把 `class`/`style`/`alt`/`id`/`draggable` 等**全部只绑定到 inner `<img>`**，原有 `header img { height: 2.5em }` 等全局 CSS 直接命中；`@load`/`@error`/`ref` 也照常可用（`defineExpose({imgEl})` 暴露原始 img DOM）
- **`eager` 首屏兜底**：登录页 Logo、HeaderBar Logo 传 `eager` 直接加载（不进入 IO，避免白屏闪烁）
- **`fill` 尺寸模式**：`fill=false`（默认）= 自然尺寸；`fill=true` = 填满父容器（`width/height: 100%` + `object-fit: contain`），用于 ExtendedImage、PDF 缩略图等被父容器显式定高的场景

已用 LazyImage 替换的页面/组件：
- `views/files/Preview.vue`（PDF 侧栏缩略图）
- `components/files/ExtendedImage.vue`（图片预览缩放平移）
- `components/header/HeaderBar.vue`（顶部 Logo）
- `views/Login.vue`（登录页 Logo）
- `views/Share.vue`（分享卡片内嵌图片）

#### 2.5.3 图片预览左右切换

`views/files/Preview.vue` 中：
- `isMediaPreview` 已扩展为 `["image","audio","video"]` → 图片预览也显示左右切换按钮（与音视频共用一套导航）
- `mediaList` 构建：当 `currentType === "image"` 时，从当前 `listing` 过滤 `type === "image"` 形成循环序列
- 切换时直接修改 `req.path` 走现有预览管线，`ExtendedImage`、`LazyImage eager fill` 透明工作

### 2.6 构建脚本（Windows）

为解决 cmd 脆弱性（ANSI 转义、编码乱码、`&` 解析错误），构建系统**拆为两层**：

| 文件 | 作用 |
|---|---|
| `build-windows.bat` | 启动器（ASCII，编码安全）：参数归一化（`skipFrontend` → `-SkipFrontend`）→ 调 pwsh，回退到 powershell.exe |
| `build-core.ps1` | 核心构建脚本（全英文 ASCII）：环境检测 → 前端构建 → 后端构建 → 原子替换 exe |

脚本能力：
- 自动检测 Go / Node / pnpm 是否安装；缺 pnpm 时尝试 `corepack enable`
- `go build` 注入 Version / CommitSHA 版本信息（`-ldflags` 用空格两 token，避免 PowerShell `"$ldflags"` 字面量 Bug）
- exe 重名名冲突时原子替换：先写到 `filebrowser.exe.tmp` → 删旧 exe → Move-Item；删不掉时改输出到 `dist\filebrowser.exe.new`
- 支持参数：
  ```bat
  build-windows.bat skipFrontend    :: 跳 pnpm build，直接 go build（5 秒出 exe）
  build-windows.bat skipBackend     :: 只做前端构建
  build-windows.bat                   :: 前端 + 后端完整出 dist\filebrowser.exe
  ```

---

## 三、目录结构

```
filebrowser-release-2026/
├── main.go                     # 入口（//go:embed frontend/dist 打单 exe）
├── go.mod / go.sum             # Go 依赖（Go 1.25）
├── build-windows.bat           # ⭐ Windows 构建启动器
├── build-core.ps1              # ⭐ Windows 构建核心逻辑（pwsh）
├── filebrowser-test.exe        # 本地构建产出（gitignore 中会忽略）
├── dummy.pdf / test.docx       # 前端预览测试用例
├── cmd/                        # CLI 命令（config/users/rules 等）
├── http/                       # HTTP 路由与处理器
│   ├── users.go                # 用户管理 API（移除二次密码校验）
│   ├── static.go               # 静态资源服务（branding.files 自定义 Logo）
│   ├── preview.go / docconvert.go   # 预览、Office 文档转图片
│   └── raw.go / tus_handlers.go     # 流式下载 + TUS 断点续传
├── settings/                   # 系统设置
│   └── storage.go              # 默认语言 zh-cn
├── users/                      # 用户模型与密码
│   ├── password.go             # 密码校验（移除弱密码黑名单）
│   └── assets/common-passwords.txt
├── storage/bolt/               # BoltDB 嵌入式存储（用户/分享/配置）
├── img/service.go              # 图片尺寸、缩略图、EXIF
├── productcode/                # 产品编码（PDF 元数据提取）
├── version/                    # 版本号（被 ldflags 注入）
├── branding/                   # 默认品牌资源（logo/banner/icon）
├── frontend/                   # ⭐ 前端（Vue 3 + Vite 8）
│   ├── index.html              # 开发模式入口
│   ├── public/index.html       # 生产模板（Go template，标题已定制）
│   ├── public/img/logo.svg     # 默认 Logo（可被 branding.files 覆盖）
│   ├── vite.config.ts          # Vite 配置（代理后端 8080 + alias）
│   ├── package.json            # Node >=24、pnpm >=10
│   ├── tsconfig.app.json       # TS typecheck 入口（vue-tsc）
│   └── src/
│       ├── main.ts / App.vue   # 入口
│       ├── i18n/               # 国际化（zh-cn 已补全）
│       ├── css/                # 样式（ios.css 为定制核心）
│       ├── components/
│       │   ├── files/
│       │   │   ├── VirtualList.vue      # ⭐ 通用虚拟滚动
│       │   │   ├── LazyImage.vue        # ⭐ 懒加载图片（iOS 菊花）
│       │   │   ├── ExtendedImage.vue    # ⭐ 图片预览平移缩放（内嵌 LazyImage）
│       │   │   └── ListingItem.vue
│       │   ├── header/HeaderBar.vue     # 顶部导航
│       │   └── prompts/                 # 操作弹窗（复制/移动/冲突）
│       ├── views/
│       │   ├── Login.vue                # 登录（URL 自动登录）
│       │   ├── Share.vue                # 分享卡片
│       │   ├── settings/                # 设置页（用户/分享/全局/个人）
│       │   └── files/
│       │       ├── FileListing.vue      # ⭐ 文件列表（虚拟滚动 + SegmentedControl）
│       │       ├── Preview.vue          # ⭐ 预览（图片左右切换 + PDF）
│       │       └── Editor.vue           # 文本/Markdown/代码编辑器
│       ├── stores/                      # Pinia（auth/file/layout/upload…）
│       ├── router/                      # Vue Router
│       └── utils/previewLoaders.ts      # pdfjs/docx/mammoth/epub 预览装载器
├── docker/                     # Docker 部署（alpine / s6-overlay）
├── www/docs/                   # 官方 CLI 参考（mkdocs 站点源）
└── dist/                       # 构建产物目录（脚本会自动创建）
    └── filebrowser.exe
```

---

## 四、开发环境要求

| 工具 | 版本要求 | 说明 |
|---|---|---|
| **Go** | **1.25.x** | `go.mod` 声明 `go 1.25.0`；编译单 exe |
| **Node.js** | **>=24.0.0** | `package.json` engines.node；旧版本（20/22）会有 Vite 8 + vue-tsc 3 API 不兼容 |
| **pnpm** | **>=10.0.0** | `packageManager: pnpm@10.33.4` |
| Git | 任意 | 源码管理 |
| PowerShell 7+ | 可选 | 用于跑 `build-core.ps1`；系统自带 Windows PowerShell 5.1 也能用（脚本内已 ASCII 英文） |

国内网络建议配置镜像：

```powershell
# npm / pnpm 镜像（腾讯云内网友好）
pnpm config set registry https://mirrors.tencent.com/npm/

# Go 模块镜像
$env:GOPROXY = "https://mirrors.tencent.com/go/,direct"
```

---

## 五、本地开发

前端 Vite 开发服务器（5173）代理 `/api`、`/static`、`/raw`、`/share` 到后端（8080）；两端**分别起两个终端**。

### 5.1 安装前端依赖

```powershell
cd frontend
pnpm install --frozen-lockfile
```

### 5.2 启动后端（8080 端口）

```powershell
# ======= 仅首次：初始化数据库 + admin 用户 =======
go run . config init --database=dev.db
go run . users add admin 123456 --perm.admin --locale zh-cn --database=dev.db
go run . config set --branding.name="文件管理系统" --database=dev.db

# ======= 启动服务 =======
go run . --database=dev.db --address=127.0.0.1 --port=8080 --root=./dev-files
# admin / 123456
```

> 提示：`dev-files/` 目录已放入多种样本（PDF / MP4 / MP3 / DOCX / EPUB / CSV / PPTX / 中日韩文件名）用于日常调试虚拟滚动、懒加载、预览切换。

### 5.3 启动前端开发服务器（HMR 热更新）

```powershell
cd frontend
pnpm run dev
# 默认 http://localhost:5173
```

修改前端代码后浏览器实时刷新；修改后端代码后重启 `go run .`。

### 5.4 日常质量门禁

```powershell
# 类型检查（vue-tsc，CI 必跑）
pnpm run typecheck

# ESLint
pnpm run lint

# Vitest 单测
pnpm run test
```

---

## 六、构建打包

目标是**单文件 `filebrowser.exe`**：前端 `frontend/dist/` 通过 `//go:embed` 整体打入 Go 二进制，运行时无外部依赖。

### 6.1 Windows 一键打包（推荐）

```powershell
cd  # 到项目根目录（包含 build-windows.bat / build-core.ps1 的那一级）

# 完整构建：前端 build → go build → dist\filebrowser.exe
.\build-windows.bat

# 快速迭代模式：跳过前端构建（例如只改了后端想验证逻辑），5 秒左右出 exe
.\build-windows.bat skipFrontend

# 只做前端构建（调试前端构建配置 / 产物分析）
.\build-windows.bat skipBackend
```

产物：`dist\filebrowser.exe`（若构建时原 exe 正在占用，会自动改写到 `dist\filebrowser.exe.new`）

### 6.2 命令行手动构建

当你在 CI / Linux 环境上跑时可手动执行：

```powershell
# ====== 前端 ======
cd frontend
pnpm install --frozen-lockfile
pnpm run build        # 产物：frontend/dist/
cd ..

# ====== 后端 ======
$Version = "2.63.15"
$CommitSHA = (git rev-parse --short HEAD 2>$null) -replace "`n",""
$ldflags = "-s -w -X `"github.com/filebrowser/filebrowser/v2/version.Version=$Version`" -X `"github.com/filebrowser/filebrowser/v2/version.CommitSHA=$CommitSHA`""

New-Item -ItemType Directory -Force dist | Out-Null
$tmp = Join-Path $PWD "dist\filebrowser.exe.tmp"
$out = Join-Path $PWD "dist\filebrowser.exe"

& go build -trimpath -ldflags $ldflags -o $tmp .
# 原子替换（旧 exe 被占用时 Move-Item -Force 通常能强制覆盖；极端情况保留 .new）
if (Test-Path -LiteralPath $out) { try { Remove-Item $out -Force -ErrorAction Stop } catch { $out = "$out.new" } }
Move-Item -LiteralPath $tmp -Destination $out -Force -ErrorAction Stop
```

> ⚠ `go build` 的 `-ldflags` 和它的值必须是**两个独立 token**（空格分开），不能写成 `-ldflags=$ldflags`，PowerShell 不会在等号里展开变量，Go 会收到字面量 `$ldflags` 直接报 `invalid value "$ldflags" for flag -ldflags`。

### 6.3 Linux 构建

```bash
cd frontend && pnpm install --frozen-lockfile && pnpm run build && cd ..

VERSION=2.63.15
COMMIT=$(git rev-parse --short HEAD)

go build -trimpath -ldflags="-s -w \
  -X github.com/filebrowser/filebrowser/v2/version.Version=$VERSION \
  -X github.com/filebrowser/filebrowser/v2/version.CommitSHA=$COMMIT" \
  -o dist/filebrowser .
```

### 6.4 Windows 交叉编译（从 Linux 打出 exe）

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w -X ..." \
  -o dist/filebrowser.exe .
```

---

## 七、部署运行

### 7.1 Windows 一键部署（内网推荐）

将以下 4 个文件放入同一目录（**支持 UNC 网络共享盘 `\\服务器\共享目录`**）：

```
filebrowser.exe
filebrowser.bat     # 一键初始化 + 启动
start.bat           # 仅启动（已检测数据库 / 端口占用）
stop.bat            # 停止（窗口标题 + 进程名双重 kill）
```

1. 双击 `filebrowser.bat` → 自动初始化数据库、创建 `admin / 123456`、设置品牌名、启动服务、打开浏览器
2. 日常启动：双击 `start.bat`
3. 停止服务：双击 `stop.bat`
4. 文件根目录：**exe 所在目录下的 `file\` 文件夹**（自动创建）

> UNC 路径说明：定制版 bat 开头用 `pushd %~dp0` 把网络共享盘自动映射为临时盘符，彻底避免 `CMD does not support UNC paths as current directories`。

### 7.2 Linux 部署

```bash
./filebrowser config init --database=/opt/fb/filebrowser.db
./filebrowser users add admin "密码" --perm.admin --locale zh-cn --database=/opt/fb/filebrowser.db
./filebrowser config set --branding.name="文件管理系统" --database=/opt/fb/filebrowser.db

nohup ./filebrowser --database=/opt/fb/filebrowser.db \
  --address=0.0.0.0 --port=8080 --root=/opt/fb/files > fb.log 2>&1 &
```

建议用 systemd / docker / s6-overlay 托管进程。

### 7.3 Docker 部署

```bash
# 本仓库 docker/ 目录有官方 alpine / s6 的 Dockerfile
docker build -t filebrowser-custom -f Dockerfile .
docker run -d -p 8080:80 \
  -v /opt/fb/database:/database \
  -v /opt/fb/files:/srv \
  -e FB_DATABASE=/database/filebrowser.db \
  -e FB_ROOT=/srv \
  -e FB_BRANDING_NAME="文件管理系统" \
  filebrowser-custom
```

### 7.4 常用启动参数

| 参数 | 说明 | 默认值 |
|---|---|---|
| `--address` | 监听地址 | `127.0.0.1`（生产改为 `0.0.0.0`） |
| `--port` | 端口 | `8080` |
| `--root` | 文件根目录 | 当前目录 |
| `--database` | BoltDB 数据库路径 | `./filebrowser.db` |
| `--branding.name` | 系统名称（网页标题） | 空 |
| `--locale` | 后端 CLI 输出语言（用户浏览器显示的前端语言已经强制 zh-cn） | `zh-cn` |

---

## 八、数据库与账号管理

单文件 BoltDB（Go 嵌入式 KV），**无需单独安装数据库**。

### 8.1 初始化

```powershell
filebrowser.exe config init --database=filebrowser.db
```

### 8.2 创建 / 重置账号

```powershell
# 创建（默认中文；--perm.admin=true 授予超级管理员）
filebrowser.exe users add admin "密码" --perm.admin --locale zh-cn --database=filebrowser.db

# 重置密码
filebrowser.exe users update admin --password "新密码" --locale zh-cn --database=filebrowser.db

# 列出 / 查找 / 导入导出
filebrowser.exe users ls              --database=filebrowser.db
filebrowser.exe users find admin      --database=filebrowser.db
filebrowser.exe users export users.json --database=filebrowser.db
filebrowser.exe users import users.json --database=filebrowser.db

# 删除（谨慎）
filebrowser.exe users rm olduser      --database=filebrowser.db
```

### 8.3 密码策略

- **最小长度**：默认 12 位，可通过 `config set --minimumPasswordLength=6` 调低（内网场景）
- **弱密码黑名单已移除**：`123456` / `admin` / `password` 等可直接使用（原 `users/assets/common-passwords.txt` 中的检查逻辑已移除）

---

## 九、品牌与自定义 Logo

### 9.1 系统名称

```powershell
filebrowser.exe config set --branding.name="文件管理系统" --database=filebrowser.db
```

### 9.2 自定义 Logo

后端对 `/img/*` 请求会**优先从 `branding.files` 目录读取**（见 `http/static.go`），因此替换 Logo 不需要重新编译：

1. 准备 `logo.svg`（正方形；建议同时导出 192/512 PNG 做 PWA 图标）
2. 放入品牌目录，如 `D:\branding\`
   ```
   D:\branding\logo.svg
   D:\branding\img\icons\android-chrome-192x192.png
   D:\branding\img\icons\android-chrome-512x512.png
   ```
3. 配置：
   ```powershell
   filebrowser.exe config set --branding.files="D:\branding" --database=filebrowser.db
   ```
4. 重启服务生效

### 9.3 主题（亮色 / 深色）

```powershell
filebrowser.exe config set --branding.theme=dark  --database=filebrowser.db
filebrowser.exe config set --branding.theme=light --database=filebrowser.db  # 默认
```

iOS 风格颜色 token 在 `frontend/src/css/_variables.css`，可直接调主色 `#007AFF`、圆角半径等。

---

## 十、URL 自动登录

实现：`frontend/src/views/Login.vue` 顶部 `autoLoginFromURL()`

```
http://host:8080/?u=admin&p=123456
```

参数别名与自定义跳转：

```
http://host:8080/?username=admin&password=123456&redirect=/settings/profile
```

- `u` / `username`：用户名
- `p` / `password`：密码
- `redirect`：登录成功后跳转的内部路径（默认 `/files/`）
- 登录成功后浏览器 History 会用 `replaceState` 移除 `u`/`p` 敏感参数（避免留在地址栏 / 历史记录里）

> ⚠ 安全提示：密码会出现在浏览器历史、反向代理 access log、WAF 审计日志中；仅内网可信场景使用，外网部署建议关闭该逻辑（删除 `autoLoginFromURL()` 调用）。

---

## 十一、常见问题

| 问题 | 解决方案 |
|---|---|
| **登录后界面是英文** | 确认用定制版 exe（`config cat` 输出 `Locale: zh-cn`）；浏览器 `Ctrl+F5` 强刷；若仍英文 → 检查 frontend/dist 是否为中文包产物（或 `pnpm run build` 重新打） |
| **4000 条文件滚动卡顿** | 确认当前是「列表视图」（切换按钮是中间的 SegmentedControl）；网格 / 画廊不走虚拟滚动，可切到列表视图体验顺滑 |
| **往上滚时文件条目消失** | 已修复（`VirtualList.vue` 的 `selfOffsetTop` 改用滚动容器 BBox 差值计算 + `#listing` 必须 `position: relative`）。若仍复现 → 检查是否引入自定义 CSS 改了 `#listing` 定位，或给 `VirtualList` 传 `mode="self"` 走自滚动模式 |
| **HeaderBar Logo 被拉成大图**（560×560） | 已修复（LazyImage `inheritAttrs:false` + 去掉 `height:auto` 强制样式）；若仍出问题 → 确认已运行 `pnpm run build` 或 dev server 刷新缓存 |
| **图片 loading 不是 iOS 菊花** | 若旧缓存 → `Ctrl+Shift+Delete` 清缓存；夜间模式下菊花颜色可用 `:root { --lazy-image-spinner: rgba(235,235,245,.6); }` 调整 |
| **图片预览左右按钮不显示** | 确认当前打开的是 `type === "image"` 的文件；只有目录内有多张图片才会形成循环序列（单张时按钮无效果但仍显示） |
| **界面没有 iOS 样式** | 旧版 JS/CSS 缓存 → `Ctrl+Shift+Delete` 清缓存或 `F12` 勾选 Disable Cache |
| **提示 password is too easy** | 用了旧 exe，请切换到本分支构建的版本（弱密码黑名单已从 `password.go` 移除） |
| **UNC 网络共享目录 `\服务器\共享` 打不开 cmd** | 用定制版部署 bat（已 `pushd` 临时映射盘符） |
| **构建 bat 中文乱码** | `build-core.ps1` 全英文避免编码问题；只有部署用的 `start.bat / filebrowser.bat` 保留 GBK 中文给最终用户 |
| **go build 报 `invalid value "$ldflags" for flag -ldflags`** | PowerShell 中必须写 `-ldflags $ldflags`（两个 token 空格分开），不能写 `-ldflags=$ldflags`；直接用 `build-windows.bat` 已正确处理 |
| **端口 8080 被占用** | 运行 `stop.bat` 停旧服务，或 `netstat -ano \| findstr :8080` 找 PID；也可改启动脚本 `PORT=8081` |
| **忘记 admin 密码** | 运行部署 `filebrowser.bat`（会强制重置 admin/123456），或 CLI 执行 `filebrowser users update admin --password "123456" --database=filebrowser.db` |
| **打包后 exe 运行报 `前端资源缺失`** | 必须先 `pnpm run build`（或 `build-windows.bat` 不带 skipFrontend），再 go build；否则 `frontend/dist/` 为空 → embed 打进的是空目录 |

---

## 十二、技术栈与二次开发指引

### 12.1 技术栈

| 层 | 选型 | 说明 |
|---|---|---|
| 后端 | **Go 1.25** + Gorilla/Mux + BoltDB（`asdine/storm/v3` 封装） | 单文件嵌入 + 零外部依赖数据库 |
| 前端 | **Vue 3.5** + **Vite 8** + Pinia + vue-i18n 11 + vue-router 5 + TypeScript 5.9 | 类型安全；HMR 开发；vue-tsc 类型检查 |
| 特殊预览 | pdfjs-dist 6 / mammoth / docx-preview / epubjs / utif / video.js 8 / vue-reader | PDF / Office / EPUB / TIFF / 视频 内置开箱即用 |
| 构建 | `//go:embed`（embed.FS 包前端 dist） + `go build -trimpath -ldflags` 注入版本 | 产物 100% 单文件 |
| 质量门禁 | ESLint 10 + Prettier 3 + vue-tsc 3 + Vitest 4 | `pnpm typecheck` 与 `pnpm test` 必跑 |

### 12.2 常用二次开发入口

| 需求 | 修改位置 |
|---|---|
| 改界面文案 | `frontend/src/i18n/zh-cn.json` |
| 改主题色 / 圆角 / 阴影 | `frontend/src/css/_variables.css`（token）→ `frontend/src/css/ios.css`（组件覆盖） |
| 新增页面 | `frontend/src/views/xxx.vue` + `frontend/src/router/index.ts` 注册路由 |
| 改登录逻辑 / 自动登录 | `frontend/src/views/Login.vue` |
| 改文件列表渲染 / 虚拟滚动 | `views/files/FileListing.vue` + `components/files/VirtualList.vue` |
| 改图片懒加载 / 菊花 / 错误态 | `components/files/LazyImage.vue` |
| 改图片预览平移缩放 / 左右切换 | `components/files/ExtendedImage.vue` + `views/files/Preview.vue` |
| 改用户 API / 密码策略 | `http/users.go` + `users/password.go` + `frontend/src/api/users.ts` |
| 改默认设置（语言、首屏目录…） | `cmd/root.go`（quickSetup）+ `settings/storage.go` |
| 加新的 CLI 子命令 | `cmd/*.go`（参考 `cmds_add.go` 结构） |
| Office 预览转码（DOCX/PPTX→图片） | `http/docconvert_windows.go`（Word COM）/ `http/preview.go`（通用预览管线） |

### 12.3 调试技巧

```powershell
# 后端日志：直接运行默认会打到 stdout，也可重定向
go run . --database=dev.db --port=8080 --root=./dev-files 2>&1 | Tee-Object -FilePath fb.log

# 前端快速样式/交互调试（无需后端每次重启）
cd frontend; pnpm run dev   # 5173 代理到 8080

# 只改后端想看结果，用 skipFrontend 秒出 exe
.\build-windows.bat skipFrontend; .\dist\filebrowser.exe --database=...
```

### 12.4 构建产物大小优化

已经默认使用 `go build -trimpath -ldflags="-s -w"` 去除调试符号和路径信息；还可进一步：

```powershell
# 1. 前端启用 gzip（vite-plugin-compression2 已在 vite.config.ts 默认启用 .gz）
# 2. 用 UPX 压缩（可选；部分杀软可能误报）
upx --best --lzma .\dist\filebrowser.exe
```

---

## 版本记录

| 标签 | 发布说明 |
|---|---|
| **v2.63.15-custom-1** | 基础定制：中文语言 + 品牌名 + 登录页 iOS 风格 |
| **v2.63.15-custom-2** | 弹窗/输入框/toast 深度 iOS 化；移除弱密码黑名单；部署 bat（UNC） |
| **v2.63.15-custom-3** | URL 自动登录；外部链接移除；自定义 Logo 指引；夜间模式 |
| **v2.63.15-custom-4** ⭐ | **列表视图虚拟滚动；LazyImage 全图替换 + iOS 菊花；图片预览左右切换；build-windows.bat + build-core.ps1（skipFrontend）；右键菜单自适应位置** |
