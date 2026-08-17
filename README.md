# 文件管理系统

基于 [filebrowser](https://github.com/filebrowser/filebrowser) v2.63.15 深度定制的中文文件管理系统，面向企业内网的文件/图纸检索与分享场景，**4000 条文件列表无卡顿滚动**、**相似 PDF 图纸向量检索（ResNet18 + 余弦相似度 Top-K）**、**图片/音视频预览左右循环切换**、**全量图片懒加载 + iOS 风格 UI**、**Windows 一键编译运行（含 CGO/Junction 处理）**。

---

## 目录

- [一、项目亮点（本轮新增）](#一项目亮点本轮新增)
- [二、定制内容清单](#二定制内容清单)
- [三、目录结构](#三目录结构)
- [四、开发环境要求](#四开发环境要求)
- [五、本地开发](#五本地开发)
- [六、构建打包](#六构建打包)
- [七、部署运行](#七部署运行)
- [八、相似 PDF 图纸向量检索（⭐ 新增）](#八相似-pdf-图纸向量检索-新增)
- [九、数据库与账号管理](#九数据库与账号管理)
- [十、品牌与自定义 Logo](#十品牌与自定义-logo)
- [十一、URL 自动登录](#十一url-自动登录)
- [十二、常见问题](#十二常见问题)
- [十三、技术栈与二次开发指引](#十三技术栈与二次开发指引)

---

## 一、项目亮点（本轮新增）

相比上一版（custom-4）的虚拟滚动/懒加载基础，本轮重点新增 **图纸向量检索能力**、**一键编译/运行脚本** 与 **搜索交互体验**：

| 能力 | 说明 |
|---|---|
| **相似 PDF 向量检索** | 上传任意 PDF → 后端 Poppler 转 300DPI → ResNet18 ONNX 提取 1000 维特征 → 对向量库做余弦 Top-K → 结果直接渲染到主文件列表（见 `drawingsearch.go` + `SimilarPdf.vue`） |
| **Windows 一键编译/运行脚本** | `run-filebrowser.ps1` 自动处理路径括号（NTFS Junction）、MinGW 环境变量、CGO_ENABLED=1、`-tags drawingsearch`，支持快捷长参数避免 PowerShell 参数歧义 |
| **搜索框样式与交互优化** | 宽度由 22em → 54em；聚焦时空标签行不再挤占宽度；搜索结果原子更新（避免中间空态白屏）；导航间自动取消 pending 搜索 + 防抖 |
| **后端 auth Cookie 写入** | 登录时 printToken 写入 `auth` Cookie（SameSite=Lax），浏览器原生 `<img>` / 新标签页打开图片不再 401，无需 fallback 到 blob URL |
| **编辑器保存后列表刷新** | `Editor.vue` save() 后设置 `fileStore.reload = true`，与重命名行为一致 |
| **右键菜单 fixed 定位** | `ContextMenu.vue` 改为 `position: fixed` + 组件内边界检测，不会被父容器 overflow 裁剪 |
| **视频主题适配** | video.js 动态生成元素统一继承 `html.light / html.dark`，所有潜在黑色背景层均被 !important 覆盖为主题色 |
| **PDF 产品编号双写** | storm 存 `path→code` 索引（列表查询）+ PDF Keywords 写 `"product-code:xxx"`（离线追溯），走临时文件+原子改名 |

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
| `frontend/src/css/login.css` | iOS 登录卡片（渐变背景 + 圆角 + 柔和阴影），登录标题完整显示不溢出 |
| `frontend/src/css/header.css` | 毛玻璃头部（`backdrop-filter`）+ 标题加粗 + 搜索框 54em 大宽度 + 尺寸状态锁定 |
| `frontend/src/css/_shell.css` | Shell 终端顶部圆角 + 阴影 |
| `frontend/src/css/_share.css` | 分享卡片圆角 14px |
| `frontend/src/css/upload-files.css` | 进度条 iOS 蓝 |
| `frontend/src/css/styles.css` | `.credits` flex 单行布局；全局搜索过渡动画颜色化（避免布局抖动） |
| **`frontend/src/css/ios.css`（新增 ~900 行）** | 全部 iOS 覆盖：toast、卡片圆角、SegmentedControl、Sheet 弹窗、按压反馈、滚动条、select/checkbox、macOS 风格 loading 组件、空文件夹 SVG（8E8E93 灰色）等 |
| `frontend/src/components/header/HeaderBar.vue` | `<slot />` 包裹 `.slot-wrapper`（顶部按钮单行）；PDF 相似搜索上传按钮位于搜索框最右侧 |
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
| **右键菜单自适应位置** | `ContextMenu.vue` 改为 `position: fixed` + 组件内边界检测，自动 flip 到可视区，避免被父容器 overflow 裁剪 |
| **编辑器/重命名后列表刷新** | `Editor.vue` save 成功后设置 `fileStore.reload = true`；`Rename.vue` 同步处理 |
| **后端 auth Cookie** | `http/auth.go` printToken 写入 `Set-Cookie: auth=xxx; Path=/; SameSite=Lax`，浏览器原生 `<img>`/新标签页可直接鉴权（避免 blob URL） |

### 2.5 性能优化（custom-4 核心）

#### 2.5.1 列表视图虚拟滚动（VirtualList）

仅 **列表视图** 启用虚拟滚动，网格 / 画廊视图保持原渲染与样式逻辑不变，避免布局变化带来视觉回归。

- 组件：`frontend/src/components/files/VirtualList.vue`
- 使用方：`frontend/src/views/files/FileListing.vue`
- 关键技术点：
  - 支持 `mode="parent"` 父滚动驱动模式（用外层 `#listing` 作为滚动容器，避免双滚动条）
  - `measuredHeights` 缓存已测行高 + `offsets` 前缀和数组，支持**可变行高**（含产品编号 subtitle 的行）
  - `selfOffsetTop` 用滚动容器坐标系计算（BBox 差值 + `container.scrollTop`），已修复「向上滚动元素消失」Bug（需 `#listing` 设置 `position: relative`）
  - `measuredOnce` 锁：首次测量前强制 `startIndex = 0`，避免 BBox 未就绪导致的偏移跳变
  - `buffer=8` 可视区前后各多渲染 8 行，滚动时肉眼无白屏

#### 2.5.2 图片懒加载组件（LazyImage）

所有 `<img>` 统一收敛到 `frontend/src/components/files/LazyImage.vue`：

- **懒加载核心**：IntersectionObserver + `rootMargin: 300px`，进入可视区前 300px 提前触发；离开视图不取消已发起请求（避免网络抖动回流）
- **iOS 风格加载指示器**：12 根径向 bar + 错位 fade 动画（`ios-spinner-fade 1s`，`animation-delay` 负偏移分 12 相位），与 iOS `UIActivityIndicatorView` 视觉一致
- **图片 BlurUp 占位**：后端生成 20×20 JPEG Base64（BoltDB 缓存，key = RealPath + ModTimeUnix + Size），CSS `filter: blur(16px) scale(1.1)` + 0.4s opacity fade → 高清图，释放 GPU 内存
- **错误占位 + 重试按钮**：加载失败显示"图片占位图标 + Retry"按钮，点击执行 `reload()`（给 `<img>` 重新换 src + 时间戳）
- **与原生 `<img>` 100% 兼容**：`defineOptions({ inheritAttrs: false })` + `useAttrs` 把 `class`/`style`/`alt`/`id`/`draggable` 等**全部只绑定到 inner `<img>`**，原有 `header img { height: 2.5em }` 等全局 CSS 直接命中；`@load`/`@error`/`ref` 也照常可用（`defineExpose({imgEl})` 暴露原始 img DOM）
- **`eager` 首屏兜底**：登录页 Logo、HeaderBar Logo、VirtualList 内缩略图（`thumbsEager=true`）传 `eager` 直接加载（不进入 IO，避免白屏闪烁）
- **`fill` 尺寸模式**：`fill=false`（默认）= 自然尺寸；`fill=true` = 填满父容器（`width/height: 100%` + `object-fit: contain`），用于 ExtendedImage、PDF 缩略图等被父容器显式定高的场景

已用 LazyImage 替换的页面/组件：
- `views/files/Preview.vue`（PDF 侧栏缩略图）
- `components/files/ExtendedImage.vue`（图片预览缩放平移）
- `components/header/HeaderBar.vue`（顶部 Logo）
- `views/Login.vue`（登录页 Logo）
- `views/Share.vue`（分享卡片内嵌图片）
- VirtualList 渲染的所有文件行（`thumbsEager=true`，跳过 IO 确保虚拟滚动缩略图正常）

#### 2.5.3 音视频 + 图片预览左右循环切换

`views/files/Preview.vue` 中：
- `isMediaPreview` 已扩展为 `["image","audio","video"]` → 图片预览也显示左右切换按钮（与音视频共用一套导航）
- **按钮永久可见**：`position: fixed; top:50%; transform:translateY(-50%)` + 半透明背景 + 明确 z-index，不依赖 hover/鼠标移动
- **循环切换**：目录内所有同类媒体（JPG/PNG/GIF/WebP 图片；MP3 音频；MP4 视频）形成循环序列
- **标题实时同步**：切换时立即更新预览标题（用 route params 优先，避免 fileStore 脏数据）
- **主题兼容**：video.js 的 wrapper/poster/tech/text-track 层全部被 !important 覆盖为应用主题色；动态 DOM 强制带 `html.light/html.dark` 类
- **健壮性**：MP3 ↔ MP4 切换的 player 初始化错误被 try/catch 包裹，不影响导航；HMR/CSS 注入顺序问题通过 unscoped styles.css + 组件内非 scoped style 双重覆盖

### 2.6 构建 / 运行脚本（本轮新增核心）

#### 2.6.1 `run-filebrowser.ps1` — 一键编译/运行（含 CGO + 相似 PDF 检索）

解决了三大类痛点：**路径括号/空格导致 MinGW ld.exe 截断**、**CGO 环境变量手工配置繁琐**、**PowerShell -File 模式参数传递歧义**。

| 能力 | 说明 |
|---|---|
| **Junction 路径映射** | 自动在 `%TEMP%` 下创建 NTFS Junction：`fb_proj` → 项目根、`fb_mingw64` → `dev-files/mingw64`，保证传给 `ld.exe`/`gcc.exe` 的路径不含 `(`、空格 |
| **CGO 自动配置** | 自动探测 MinGW GCC 版本目录，设置 `CGO_ENABLED=1`、`CC/CXX`、`GCC_EXEC_PREFIX`、`LIBRARY_PATH`、`CPATH`；PDF 转图所需 `pdftoppm.exe` 自动加入 PATH |
| **Build Tag 二选一** | 默认启用 `-tags drawingsearch`（编译真实向量检索）；加 `-NoDrawingSearch` 走 stub（纯文件服务，无需 MinGW） |
| **安全的参数传递 API** | 4 个快捷长参数（首字母唯一、前缀无歧义）：`-DbFile`（-d）、`-JRootPath`（-r）、`-KBindAddr`（-a）、`-QListenPort`（-p）；禁止 `@()` 数组字面量与 `-d/-r` 短 flag（会被 PowerShell -File 模式误解析） |
| **误用诊断** | 若检测到用户写 `-RawArgs @(…)` / `-d/-r/-a/-p` 等痕迹，立即弹出 WARNING 并给出正确的复制粘贴示例 |
| **`-Mode Build` 单独编译** | 输出 `output/filebrowser.exe`（40MB 级，含完整前端 embed） |

**用法示例（复制即用）：**
```powershell
# 运行：启用相似 PDF 检索（多行续行）
powershell -ExecutionPolicy Bypass -File .\run-filebrowser.ps1 -Mode Run `
  -DbFile .\filebrowser.db `
  -JRootPath "D:\BaiduNetdiskDownload\图纸\图纸" `
  -KBindAddr 127.0.0.1 `
  -QListenPort 8080

# 运行：纯文件服务（无需 MinGW/CGO）
powershell -ExecutionPolicy Bypass -File .\run-filebrowser.ps1 -Mode Run -NoDrawingSearch `
  -DbFile .\filebrowser.db -JRootPath "D:\图纸" -KBindAddr 127.0.0.1 -QListenPort 8080

# 仅编译：产出 output\filebrowser.exe
powershell -ExecutionPolicy Bypass -File .\run-filebrowser.ps1 -Mode Build
```

#### 2.6.2 原 `build-windows.bat` + `build-core.ps1`

仍然保留，适合**无 CGO**（纯文件服务，不带 drawingsearch）的快速构建；若需要相似 PDF 检索功能，请使用上面的 `run-filebrowser.ps1 -Mode Build`。

### 2.7 搜索框优化（本轮新增）

| 问题 | 修复 |
|---|---|
| 搜索框太窄（max-width:22em） | `#search` 增大到 `max-width:54em; min-width:22em`；输入框 `flex:2.5; min-width:320px`；标签行缩到 30% |
| 点击输入框时宽度跳变缩小 | 空 `.search-tags` 条件渲染（无标签时不占空间）；`transition: all` → 仅颜色属性过渡；`:is()` 选择器锁定 focus/open/ongoing/active 状态下的尺寸 |
| 搜索过程出现中间空态白屏 | 所有分片搜索结果收集完成后**原子**写入 `searchResults`，避免用户看到空列表闪烁 |
| 导航目录后旧搜索结果延迟弹出 | 路由切换时自动 `AbortController.abort()` + 清除防抖定时器，避免"切到 B 目录却显示 A 目录搜索结果"的延迟错乱 |
| Vite 代理 502 Bad Gateway | `vite.config.ts` 增加 `agent: keepAlive:false`、10min 超时、proxyReq 对 `[]` 编码、proxy `error` 事件返回结构化 JSON 错误与排查步骤 |

### 2.8 相似 PDF 向量检索（⭐ 本轮新增核心，详见第八章）

| 层级 | 实现 |
|---|---|
| **后端 API** | `http/drawingsearch.go`（`go:build drawingsearch`）：`POST /api/search/similar-pdf`，多部分 form-data 上传 PDF → pdftoppm 转 300DPI PNG → ResNet18 ONNX 推理 → 对 storm 索引的向量库做余弦相似度 → 返回 Top-K + 诊断信息 |
| **后端 stub** | `http/drawingsearch_off.go`（默认编译）：返回 **HTTP 501** + 详细修复 hint（引导用户用 `run-filebrowser.ps1` 重编） |
| **前端上传** | 搜索框最右侧新增上传按钮 → `SimilarPdf.vue` 弹窗：拖拽上传 / 进度展示 / 错误处理（501/502/0 分别给出详细中文排查路径） |
| **结果渲染** | 点击单行或"显示全部结果" → 把 Top-K 结果构造成 ResourceItem（附加相似度 subtitle）→ 调用 `fileStore.setSearchResults(...)` → 主文件列表进入 searchMode 渲染 + 自动关闭弹窗 |
| **离线建库工具** | `cmd/transform-pdf/main.go`（批量扫描 PDF 提取向量入 drawings.db BoltDB）；`cmd/search-similar/main.go`（离线查询 PDF 相似度 Top-K，拖文件 PowerShell 脚本） |
| **CGO 坑位规避** | 项目路径含括号时自动创建 Junction；`productcode/drawingsearch` 包调用 pdfcpu 前强制 `model.ConfigPath = "disable"`（避免无 %AppData% 写权限环境 panic） |

---

## 三、目录结构

```
filebrowser-release-2026/
├── main.go                     # 入口（//go:embed frontend/dist 打单 exe）
├── go.mod / go.sum             # Go 依赖（Go 1.25）
├── .gitignore                  # 已忽略 dev-files/mingw64、*.exe、*.log、*.dll 等
├── build-windows.bat           # ⭐ Windows 纯 Go 构建启动器（无 CGO，无 PDF 向量检索）
├── build-core.ps1              # ⭐ Windows 构建核心逻辑（pwsh）
├── run-filebrowser.ps1         # ⭐⭐ 一键编译/运行（CGO + 相似 PDF 检索 + Junction 路径映射 + 无歧义长参数）
├── dummy.pdf / test.docx       # 前端预览测试用例
├── dev-files/                  # 运行/构建依赖目录（已 gitignore，见第八章 8.1）
│   ├── mingw64/                # MinGW-w64 GCC（x86_64-posix-seh，CGO 必需）
│   ├── poppler-25.12.0/bin/    # pdftoppm.exe（PDF 转 300DPI PNG）
│   ├── onnxruntime.dll         # ONNX Runtime C API（onnxruntime_go 运行时）
│   ├── resnet18_features.onnx  # ResNet18 特征提取模型（[1,3,224,224] → [1,1000]）
│   └── drawings.db             # 向量库（BoltDB，由 transform-pdf 首次建库）
├── cmd/                        # CLI 子命令
│   ├── root.go / users.go / config.go   # 官方子命令（config/users/rules…）
│   ├── transform-pdf/          # ⭐⭐ 离线批量 PDF 建库（扫描目录 → 提取特征 → drawings.db）
│   │   ├── main.go             #    相对路径拼接 + resolveProjectRoot
│   │   └── transform-pdf.exe   #    （编译产物，gitignore）
│   └── search-similar/         # ⭐⭐ 离线相似度 Top-K（单文件对向量库检索）
│       ├── main.go             #    4 阶段进度 + 三路日志（stdout/stderr/log）
│       ├── run-search-similar.ps1  #    拖拽 PDF 快速查询脚本
│       └── search-similar.log  #    （运行产物，gitignore）
├── http/                       # HTTP 路由与处理器
│   ├── auth.go                 # 登录/鉴权（printToken 写入 auth Cookie，SameSite=Lax）
│   ├── users.go                # 用户管理 API（移除二次密码校验）
│   ├── static.go               # 静态资源服务（branding.files 自定义 Logo）
│   ├── preview.go / docconvert.go   # 预览、Office 文档转图片
│   ├── raw.go / tus_handlers.go     # 流式下载 + TUS 断点续传
│   ├── jsonutil.go             # writeJSON() 统一响应（显式 Content-Length，避免代理 chunked 问题）
│   ├── drawingsearch.go        # ⭐⭐ 相似 PDF 检索真实实现（go:build drawingsearch，需 CGO）
│   ├── drawingsearch_off.go    # ⭐⭐ 默认 stub（HTTP 501 + 修复 hint）
│   └── transformPdf.go         # PDF 产品编号双写改写（临时文件 + 原子改名）
├── productcode/                # PDF 产品编码（元数据提取/写入；调用 pdfcpu 前强制 ConfigPath=disable）
├── drawingsearch/              # 向量检索共享代码（DrawingFeature struct、余弦相似度、onnx 封装）
├── settings/                   # 系统设置
│   └── storage.go              # 默认语言 zh-cn
├── users/                      # 用户模型与密码
│   ├── password.go             # 密码校验（移除弱密码黑名单）
│   └── assets/common-passwords.txt
├── storage/bolt/               # BoltDB 嵌入式存储（用户/分享/配置）
├── img/service.go              # 图片尺寸、缩略图、EXIF + 20×20 BlurUp Base64
├── version/                    # 版本号（被 ldflags 注入）
├── branding/                   # 默认品牌资源（logo/banner/icon）
├── frontend/                   # ⭐ 前端（Vue 3 + Vite 8）
│   ├── index.html              # 开发模式入口
│   ├── public/index.html       # 生产模板（Go template，标题已定制）
│   ├── public/img/logo.svg     # 默认 Logo（可被 branding.files 覆盖）
│   ├── vite.config.ts          # Vite 代理 8080（keepAlive:false、10min 超时、[] 编码、结构化 502）
│   ├── package.json            # Node >=24、pnpm >=10
│   ├── tsconfig.app.json       # TS typecheck 入口（vue-tsc）
│   └── src/
│       ├── main.ts / App.vue   # 入口
│       ├── api/utils.ts        # StatusError 增强：解析后端 JSON error + hint
│       ├── i18n/               # 国际化（zh-cn 已补全 22 key）
│       ├── css/                # 样式（ios.css/ header:54em 搜索框 / 搜索尺寸锁定）
│       ├── components/
│       │   ├── files/
│       │   │   ├── VirtualList.vue      # ⭐ 通用虚拟滚动（父滚动 + 可变行高）
│       │   │   ├── LazyImage.vue        # ⭐ 懒加载图片（iOS 菊花 + BlurUp + eager/fill）
│       │   │   ├── ExtendedImage.vue    # ⭐ 图片预览平移缩放（内嵌 LazyImage）
│       │   │   └── ListingItem.vue
│       │   ├── header/HeaderBar.vue     # 顶部导航（搜索框最右侧「PDF 检索」按钮）
│       │   ├── ContextMenu.vue          # 右键菜单（position: fixed + 边界 flip）
│       │   └── prompts/                 # 操作弹窗
│       │       ├── Prompts.vue          # 弹窗总注册（含 SimilarPdf）
│       │       └── SimilarPdf.vue       # ⭐⭐ PDF 上传/检索弹窗（拖拽 + Top-K 渲染主列表）
│       ├── views/
│       │   ├── Login.vue                # 登录（URL 自动登录）
│       │   ├── Share.vue                # 分享卡片
│       │   ├── settings/                # 设置页（用户/分享/全局/个人）
│       │   └── files/
│       │       ├── FileListing.vue      # ⭐ 文件列表（虚拟滚动 + SegmentedControl + 搜索原子更新）
│       │       ├── Preview.vue          # ⭐ 预览（图片/音视频左右循环 + 标题实时同步 + video.js 主题覆盖）
│       │       └── Editor.vue           # 编辑器（save 后 fileStore.reload 刷新列表）
│       ├── stores/                      # Pinia（auth/file/layout/upload…）
│       ├── router/                      # Vue Router
│       └── utils/previewLoaders.ts      # pdfjs/docx/mammoth/epub 预览装载器
├── docker/                     # Docker 部署（alpine / s6-overlay）
├── www/docs/                   # 官方 CLI 参考（mkdocs 站点源）
├── dist/                       # 纯 Go 构建产物（build-windows.bat 输出）
│   └── filebrowser.exe
└── output/                     # run-filebrowser.ps1 -Mode Build 输出
    └── windows-amd64/filebrowser.exe
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

## 八、相似 PDF 图纸向量检索（⭐ 新增）

面向**工程图纸快速检索**场景：上传任意一张 PDF 图纸（无需文件名/编号），在已入库的海量 PDF 向量库中做**余弦相似度 Top-K** 检索，结果直接渲染到主文件列表供预览/下载/分享。

整体架构：
```
用户上传 PDF → 后端 pdftoppm 转 300DPI PNG
               → ResNet18 (ONNX) 提取 1000 维特征向量
               → 对 drawings.db (BoltDB) 中全部向量做余弦相似度
               → 按相似度降序取 Top-K → 返回文件元信息 + 诊断数据
               → 前端 fileStore.setSearchResults → 主文件列表 searchMode 渲染
```

### 8.1 前置依赖（dev-files/ 目录）

**必须放在 `dev-files/` 下**（`run-filebrowser.ps1` 会自动把该目录加入 PATH / LIBRARY_PATH）：

| 文件 | 用途 | 获取方式 |
|---|---|---|
| `mingw64/` | MinGW-w64 GCC（x86_64-posix-seh，含 `gcc.exe` / `ld.exe` / `g++.exe`） | [WinLibs 官网](https://winlibs.com/) 下载 UCRT runtime 版，解压到 `dev-files/mingw64/` |
| `poppler-25.12.0/` | `pdftoppm.exe` （PDF → PNG 转图，300DPI 提取首页） | Poppler for Windows 官方构建；确保 `bin/pdftoppm.exe` 存在 |
| `onnxruntime.dll` | ONNX Runtime C API（`onnxruntime_go` 运行时依赖） | [microsoft/onnxruntime](https://github.com/microsoft/onnxruntime/releases) 下载 win-x64 包，取 `lib/onnxruntime.dll` |
| `resnet18_features.onnx` | ResNet18 特征提取模型（输入 NCHW `[1,3,224,224]` float32；输出 `[1,1000]` 特征向量） | 用 torchvision `resnet18(weights=DEFAULT)` → `torch.onnx.export` 导出；或参考 `cmd/transform-pdf/` 说明 |
| `drawings.db` | **向量库**（BoltDB / storm，存储 `DrawingFeature{FilePath, FileName, FeatureVector[]float32, ...}`） | 运行下面「8.3 离线建库工具」首次批量生成；后续增量扫描会自动追加 |

### 8.2 启用方式（两种编译路径）

由于 `onnxruntime_go` 强依赖 **CGO**，该功能默认**不编译**（返回 501），需要用 Build Tag 显式启用：

| 编译方式 | 命令 | 效果 |
|---|---|---|
| **默认编译（纯文件服务）** | `go build -o filebrowser.exe .` | `POST /api/search/similar-pdf` → **HTTP 501** + 修复 hint（stub 在 `drawingsearch_off.go`） |
| **启用向量检索** | `go build -tags drawingsearch -o filebrowser.exe .`（需 CGO_ENABLED=1 + MinGW GCC） | 真实推理；`drawingsearch.go` 生效 |

**强烈推荐直接用 `run-filebrowser.ps1`**，它会：
- 自动探测 MinGW / Poppler / ONNX DLL 是否存在
- 路径含括号/空格时自动创建 NTFS Junction 映射（避免 ld.exe 路径截断 Bug）
- 自动注入所有环境变量（CGO_ENABLED、CC、PATH、LIBRARY_PATH）

### 8.3 离线建库工具（cmd/transform-pdf）

**首次使用必须先建库**（否则向量库为空，检索无结果）。扫描目标目录下全部 PDF → 逐页提取特征 → 写入 `drawings.db`：

```powershell
cd cmd\transform-pdf
# 先把 transform-pdf.exe 放在 dev-files 同级，或直接 go run（需 CGO）：
.\transform-pdf.exe -scan "D:\BaiduNetdiskDownload\图纸\图纸" -db "..\..\dev-files\drawings.db"
```

关键参数：
| 参数 | 说明 | 默认 |
|---|---|---|
| `-scan` | 要扫描的 PDF 根目录（递归子目录） | `{项目根}/dev-files/` |
| `-db` | drawings.db BoltDB 输出路径 | `{项目根}/dev-files/drawings.db` |
| `-resnet` | ONNX 模型路径 | `{项目根}/dev-files/resnet18_features.onnx` |
| `-poppler` | poppler bin 目录（含 pdftoppm） | 自动探测 `{项目根}/dev-files/poppler-25.12.0/bin` |
| `-dpi` | 转图 DPI | `300` |
| `-workers` | 并发数 | CPU 核数 |
| `-pageMode` | `first`=仅首页（推荐，工程图纸通常首页为主图）/ `all`=每页一向量 | `first` |

进度：每处理 10 个 PDF 打印一次统计（已入库 / 跳过已存在 / 失败列表）。重复 PDF 通过 `FilePath`（storm id）去重，二次扫描不会重复入库。

### 8.4 离线查询工具（cmd/search-similar）

不启动 Web 服务，直接对指定 PDF 文件做 Top-K 相似度：

```powershell
cd cmd\search-similar
# 推荐：拖拽 PDF 到 run-search-similar.ps1（会自动开日志）
.\run-search-similar.ps1 "D:\BaiduNetdiskDownload\ph2\8-35ZKG2（-0.1，DN150JC）(1).pdf" 10
```

输出：Top-K 结果含相似度%、文件名、完整磁盘路径、大小/修改时间，方便核对。所有日志同时写入 stdout + stderr + `search-similar.log`（避免 CMD 滚屏丢失）。

### 8.5 前端使用

1. 打开主界面 → 搜索框最右侧有「📎 PDF 检索」按钮（蓝色小图标）
2. 点击弹出「相似图纸检索」弹窗，**支持拖拽** PDF 到虚线框，或点击「选择 PDF 文件」
3. 点击「开始检索相似图纸」→ 进度条转完后显示 Top-K（默认 20）结果列表
4. **点击任意一行**（或底部「显示全部结果」按钮）→ 弹窗自动关闭，结果以「搜索结果视图」渲染到主文件列表（每条带 `相似度 xx%` subtitle），可直接预览 PDF / 重命名 / 分享

### 8.6 技术细节与坑位规避

| 坑 | 根因 | 解决 |
|---|---|---|
| **HTTP 501 功能未启用** | 默认编译走 stub（`drawingsearch_off.go`） | 用 `run-filebrowser.ps1`（不加 `-NoDrawingSearch`）重编；或 `CGO_ENABLED=1 go build -tags drawingsearch` |
| **MinGW ld.exe 报路径找不到 / 截断** | 项目路径含 `(`（如 `filebrowser-release-20260814 (2)`）或空格 | `run-filebrowser.ps1` 自动在 `%TEMP%` 下建 NTFS Junction：`fb_proj` → 项目根、`fb_mingw64` → mingw64；所有传给 gcc/ld 的路径都用映射后的 ASCII 路径 |
| **pdfcpu 初始化 panic（尝试创建 certs 目录）** | pdfcpu 默认读 `%AppData%/pdfcpu/config.yml`，无写权限会 `fault.Fail` | `productcode` 与 `drawingsearch` 包在任何 pdfcpu API 调用前强制 `model.ConfigPath = "disable"` |
| **pdftoppm 找不到** | 没把 poppler/bin 加入 PATH | `run-filebrowser.ps1` 自动追加；或手动把 `dev-files/poppler-25.12.0/bin` 放系统 PATH |
| **onnxruntime_go: build constraints exclude** | `CGO_ENABLED=0` 时 Go 不编译 `.c/.go` 混合包 | 设置 `CGO_ENABLED=1` + 可用 MinGW GCC（或用脚本自动） |
| **向量库为空，检索无结果** | 未跑 `transform-pdf.exe -scan` 建库 | 首次使用按「8.3」批量扫描图纸目录；运行完后 drawings.db 应有几十 MB |
| **相似度全是 0 / NaN** | ONNX 模型与输入 shape 不匹配；或 PDF 转图失败（空首页） | 用 `search-similar.exe` 单文件调试，看日志中的特征向量是否全为 0；确认模型输入 shape 为 `[1,3,224,224]` NCHW |
| **搜索结果路径与主文件根不匹配** | `drawingsearch` 存的是**真实磁盘绝对路径**（如 `D:\图纸\ZKG2.pdf`），fileStore 用户根可能是子目录（如 `/` 映射到 `D:\BaiduNetdiskDownload\图纸\图纸`） | `SimilarPdf.vue` 中 `mapDiskPathToVirtual()` 会做反向映射，把磁盘路径转回 fileStore 能识别的虚拟路径；找不到映射时会用 "/" 前缀兜底 |

---

## 九、数据库与账号管理

单文件 BoltDB（Go 嵌入式 KV），**无需单独安装数据库**。

### 9.1 初始化

```powershell
filebrowser.exe config init --database=filebrowser.db
```

### 9.2 创建 / 重置账号

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

### 9.3 密码策略

- **最小长度**：默认 12 位，可通过 `config set --minimumPasswordLength=6` 调低（内网场景）
- **弱密码黑名单已移除**：`123456` / `admin` / `password` 等可直接使用（原 `users/assets/common-passwords.txt` 中的检查逻辑已移除）

---

## 十、品牌与自定义 Logo

### 10.1 系统名称

```powershell
filebrowser.exe config set --branding.name="文件管理系统" --database=filebrowser.db
```

### 10.2 自定义 Logo

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

### 10.3 主题（亮色 / 深色）

```powershell
filebrowser.exe config set --branding.theme=dark  --database=filebrowser.db
filebrowser.exe config set --branding.theme=light --database=filebrowser.db  # 默认
```

iOS 风格颜色 token 在 `frontend/src/css/_variables.css`，可直接调主色 `#007AFF`、圆角半径等。

---

## 十一、URL 自动登录

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

## 十二、常见问题

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
| **HTTP 501：相似 PDF 检索功能未启用** | 当前 exe 未启用 CGO 编译。直接用 `run-filebrowser.ps1 -Mode Run`（不加 `-NoDrawingSearch`）重新编译运行；或参考「第八章 8.2」手动配置 MinGW + `go build -tags drawingsearch` |
| **PowerShell 运行脚本报 `MissingArgument` / 参数丢失** | 禁止用 `-RawArgs @(...)` 数组字面量与 `-d/-r/-a/-p` 短 flag（PowerShell -File 模式解析歧义）。改用脚本提供的 4 个长参数：`-DbFile`、`-JRootPath`、`-KBindAddr`、`-QListenPort`，见「2.6.1」复制粘贴示例 |
| **Vite 代理 /api 报 502 Bad Gateway** | 首先确认后端 8080 端口真的启动（`netstat -ano \| findstr :8080`）。若后端已起 → 检查 502 返回 JSON 中 `err` 字段：常见是 `[]` 方括号未编码（`vite.config.ts` 已配置 proxyReq 自动编码）、或后端还在构建中、或 multipart/form-data 超时。开发模式下建议使用 `run-filebrowser.ps1` 统一启动后端，它自动处理 PATH |
| **相似 PDF 检索有结果但路径不对 / 无法预览** | drawings.db 存的是 PDF 真实磁盘绝对路径（如 `D:\图纸\ZKG2.pdf`）。若 fileBrowser 启动的 `--root` 与该路径不重合（如只映射了 `D:\图纸\子目录`），前端 `mapDiskPathToVirtual()` 映射失败 → 用 "/" 前缀兜底但无法走原始路由。**解决办法**：将 `--root` 设置为图纸库的公共上级目录（如 `D:\BaiduNetdiskDownload\图纸`），或重新建库时扫描路径与运行路径一致 |
| **MinGW ld.exe 报找不到库文件 / 路径截断** | 项目路径含 `(`、`)`、空格、中文字符时 MinGW 连接器会截断。请使用 `run-filebrowser.ps1`，它自动在 `%TEMP%` 下创建 NTFS Junction（`C:\fb_proj` → 项目根、`C:\fb_mingw64` → mingw64），所有传给 gcc/ld 的路径都走无空格/ASCII 的映射盘 |
| **相似 PDF 结果渲染到主列表但文件名竖排** | 已修复（弹窗 420~880px 宽、grid 用 `minmax(0,1fr)`、文件名 `white-space:nowrap + text-overflow:ellipsis`）。若仍出现 → 确认已 `pnpm run build` 或 dev server 清缓存 |

---

## 十三、技术栈与二次开发指引

### 13.1 技术栈

| 层 | 选型 | 说明 |
|---|---|---|
| 后端 | **Go 1.25** + Gorilla/Mux + BoltDB（`asdine/storm/v3` 封装） | 单文件嵌入 + 零外部依赖数据库 |
| 前端 | **Vue 3.5** + **Vite 8** + Pinia + vue-i18n 11 + vue-router 5 + TypeScript 5.9 | 类型安全；HMR 开发；vue-tsc 类型检查 |
| 特殊预览 | pdfjs-dist 6 / mammoth / docx-preview / epubjs / utif / video.js 8 / vue-reader | PDF / Office / EPUB / TIFF / 视频 内置开箱即用 |
| 构建 | `//go:embed`（embed.FS 包前端 dist） + `go build -trimpath -ldflags` 注入版本 | 产物 100% 单文件 |
| 质量门禁 | ESLint 10 + Prettier 3 + vue-tsc 3 + Vitest 4 | `pnpm typecheck` 与 `pnpm test` 必跑 |

### 13.2 常用二次开发入口

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
| **相似 PDF 检索后端** | 启用版 `http/drawingsearch.go` / stub 版 `http/drawingsearch_off.go`；向量模型与建库在 `cmd/transform-pdf/main.go` |
| **相似 PDF 上传弹窗** | `frontend/src/components/prompts/SimilarPdf.vue`（拖拽上传、错误处理、结果渲染到主列表） |
| **搜索框与 PDF 检索按钮** | `frontend/src/components/header/HeaderBar.vue`（按钮布局）+ `frontend/src/components/prompts/Prompts.vue`（弹窗注册） |
| **一键编译/运行脚本** | `run-filebrowser.ps1`（Junction 路径、CGO 环境、长参数 API）；纯 Go 构建用 `build-windows.bat` + `build-core.ps1` |

### 13.3 调试技巧

```powershell
# 后端日志：直接运行默认会打到 stdout，也可重定向
go run . --database=dev.db --port=8080 --root=./dev-files 2>&1 | Tee-Object -FilePath fb.log

# 前端快速样式/交互调试（无需后端每次重启）
cd frontend; pnpm run dev   # 5173 代理到 8080

# 只改后端想看结果，用 skipFrontend 秒出 exe
.\build-windows.bat skipFrontend; .\dist\filebrowser.exe --database=...

# 相似 PDF 功能调试（需 CGO；直接用脚本避免手工配环境）
powershell -ExecutionPolicy Bypass -File .\run-filebrowser.ps1 -Mode Run `
  -DbFile .\dev.db -JRootPath "D:\图纸" -KBindAddr 127.0.0.1 -QListenPort 8080
```

### 13.4 构建产物大小优化

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
| **v2.63.15-custom-5** ⭐⭐ | **相似 PDF 图纸向量检索（ResNet18 ONNX + 余弦 Top-K + 前端拖拽上传 + 结果渲染到主列表）；`run-filebrowser.ps1` 一键编译/运行（Junction 规避路径括号 + MinGW/CGO 自动配置 + 无歧义长参数 API）；搜索框 22→54em + 聚焦尺寸锁定 + 搜索原子更新/取消；后端 auth Cookie 写入；编辑器保存后列表刷新；PDF 产品编号双写（storm 索引 + PDF Keywords）** |
