# 文件管理系统

基于 [filebrowser](https://github.com/filebrowser/filebrowser) v2.63.15 深度定制的中文文件管理系统，面向企业内网的文件/图纸检索与分享场景。

---

## 目录

- [一、项目简介](#一项目简介)
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

## 一、项目简介

本系统以 filebrowser（Go + Vue 3 文件管理框架）为基础

- **全中文界面**：默认语言简体中文，前端 + 后端双重兜底
- **iOS 风格 UI**：遵循 Apple Human Interface Guidelines 重构全部组件
- **品牌化**：网页标题、登录页、侧边栏统一为"文件管理系统"
- **便捷部署**：Windows 一键脚本（初始化 + 启动 + 停止），支持网络共享路径（UNC）
- **URL 自动登录**：`http://host/?u=admin&p=123456` 免输入直接进入首页
- **内网优化**：移除所有外部链接（GitHub/官方文档），弱密码校验改为可配置

---

## 二、定制内容清单

与原版 filebrowser v2.63.15 相比的全部改动：

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

### 2.5 部署脚本（Windows）

| 文件 | 功能 |
|---|---|
| `filebrowser.bat` | 一键初始化 + 启动（自动创建 admin/123456、设置品牌名、中文） |
| `start.bat` | 仅启动服务（检测数据库/端口占用） |
| `stop.bat` | 停止服务（按窗口标题 + 进程名双重 kill） |

脚本特点：GBK 编码（中文不乱码）、`pushd` 支持 UNC 网络路径、幂等可重复运行。

---

## 三、目录结构

```
filebrowser/
├── main.go                  # 入口
├── go.mod / go.sum          # Go 依赖
├── cmd/                     # CLI 命令（config/users/rules 等）
├── http/                    # HTTP 路由与处理器
│   ├── users.go             # 用户管理 API（含定制：移除二次密码校验）
│   └── static.go            # 静态资源服务（branding.files 自定义 Logo）
├── settings/                # 系统设置
│   └── storage.go           # 默认语言 zh-cn
├── users/                   # 用户模型与密码
│   ├── password.go          # 密码校验（移除弱密码黑名单）
│   └── assets/common-passwords.txt  # 弱密码列表（123456 已移除）
├── version/                 # 版本号
├── frontend/                # 前端（Vue 3 + Vite）
│   ├── index.html           # 开发模式入口
│   ├── public/index.html    # 生产模板（Go template，标题已定制）
│   ├── vite.config.ts       # Vite 配置
│   ├── package.json         # 前端依赖
│   └── src/
│       ├── App.vue          # 根组件（默认亮色主题）
│       ├── main.ts          # 入口
│       ├── i18n/            # 国际化（zh-cn 已补全）
│       ├── css/             # 样式（ios.css 为定制核心）
│       ├── components/      # 组件（HeaderBar/DropdownModal 等已定制）
│       ├── views/           # 页面（Login/FileListing/User 等已定制）
│       ├── stores/          # Pinia 状态
│       └── utils/           # 工具函数
└── www/                     # 官方文档站（未修改）
```

---

## 四、开发环境要求

| 工具 | 版本要求 | 说明 |
|---|---|---|
| Go | 1.21+ | 后端编译 |
| Node.js | 20.19+ / 22.12+ | 前端构建（推荐 22+） |
| pnpm | 10+ | 前端包管理器 |
| Git | 任意 | 源码管理 |

国内网络环境建议配置镜像：

```bash
# npm / pnpm 镜像
npm config set registry https://mirrors.tencent.com/npm/

# Go 模块镜像
export GOPROXY=https://mirrors.tencent.com/go/,direct
```

---

## 五、本地开发

### 5.1 安装前端依赖

```bash
cd frontend
pnpm install --frozen-lockfile
```

### 5.2 启动后端（8080 端口）

```bash
# 初始化数据库（首次）
go run . config init --database=dev.db
go run . users add admin admin123456 --perm.admin --database=dev.db
go run . config set --branding.name="文件管理系统" --database=dev.db

# 启动服务
go run . --database=dev.db --address=127.0.0.1 --port=8080 --root=./dev-files
```

### 5.3 启动前端开发服务器（HMR 热更新）

```bash
cd frontend
pnpm run dev
# 访问 http://localhost:5173 （自动代理 /api 到 8080）
```

修改前端代码后浏览器实时刷新；修改后端代码后需重启 `go run .`。

---

## 六、构建打包

### 6.1 前端构建

```bash
cd frontend
pnpm exec vite build
# 产物输出到 frontend/dist/（被 Go embed 进二进制）
```

### 6.2 Linux 构建

```bash
go build -ldflags='-s -w \
  -X "github.com/filebrowser/filebrowser/v2/version.Version=2.63.15" \
  -X "github.com/filebrowser/filebrowser/v2/version.CommitSHA=<commit>"' \
  -o filebrowser .
```

### 6.3 Windows 交叉编译（生成 exe）

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
  -ldflags='-s -w \
  -X "github.com/filebrowser/filebrowser/v2/version.Version=2.63.15" \
  -X "github.com/filebrowser/filebrowser/v2/version.CommitSHA=<commit>"' \
  -o filebrowser.exe .
```

### 6.4 一键构建脚本

```bash
# 前端 + Linux + Windows 全部构建
cd frontend && pnpm exec vite build && cd ..
go build -o filebrowser .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o filebrowser.exe .
```

---

## 七、部署运行

### 7.1 Windows 一键部署（推荐）

把以下 4 个文件放入同一目录（支持网络共享盘 `\\服务器\共享目录`）：

```
filebrowser.exe
filebrowser.bat     # 一键初始化 + 启动
start.bat           # 仅启动
stop.bat            # 停止
```

1. 双击 `filebrowser.bat` → 自动初始化数据库、创建 `admin / 123456`、设置品牌名、启动服务并打开浏览器
2. 日常启动用 `start.bat`，停止用 `stop.bat`
3. 文件根目录默认为 exe 所在目录下的 `file\` 文件夹（自动创建）

### 7.2 Linux 部署

```bash
./filebrowser config init --database=/opt/fb/filebrowser.db
./filebrowser users add admin "密码" --perm.admin --database=/opt/fb/filebrowser.db
./filebrowser config set --branding.name="文件管理系统" --database=/opt/fb/filebrowser.db
nohup ./filebrowser --database=/opt/fb/filebrowser.db \
  --address=0.0.0.0 --port=8080 --root=/opt/fb/files > fb.log 2>&1 &
```

### 7.3 常用启动参数

| 参数 | 说明 | 默认值 |
|---|---|---|
| `--address` | 监听地址 | `127.0.0.1` |
| `--port` | 端口 | `8080` |
| `--root` | 文件根目录 | 当前目录 |
| `--database` | 数据库路径 | `filebrowser.db` |
| `--branding.name` | 系统名称（网页标题） | 空 |

---

## 八、数据库与账号管理

### 8.1 初始化

```bash
filebrowser config init --database=filebrowser.db
```

### 8.2 创建 / 重置账号

```bash
# 创建（默认中文语言）
filebrowser users add admin "密码" --perm.admin --locale zh-cn --database=filebrowser.db

# 重置密码
filebrowser users update admin --password "新密码" --locale zh-cn --database=filebrowser.db

# 列出用户
filebrowser users ls --database=filebrowser.db
```

### 8.3 密码策略

- 最小长度：默认 12 位，可用 `config set --minimumPasswordLength=6` 调低
- **弱密码黑名单已移除**：123456 等简单密码可直接使用（内网场景）

---

## 九、品牌与自定义 Logo

### 9.1 系统名称

```bash
filebrowser config set --branding.name="文件管理系统" --database=filebrowser.db
```

### 9.2 自定义 Logo

1. 准备 `logo.svg`（建议正方形，登录页按 4.5em 显示）
2. 放入品牌目录，如 `D:\branding\logo.svg`
3. 设置：
   ```bash
   filebrowser config set --branding.files="D:\branding" --database=filebrowser.db
   ```
4. 重启服务后登录页 Logo 自动替换
   > 原理：后端对 `/img/*` 请求优先从 `branding.files` 目录读取（见 `http/static.go`）

### 9.3 主题

```bash
filebrowser config set --branding.theme=dark --database=filebrowser.db   # 深色
filebrowser config set --branding.theme=light --database=filebrowser.db  # 亮色（默认）
```

---

## 十、URL 自动登录

打开以下链接即可免输入登录并跳转首页：

```
http://localhost:8080/?u=admin&p=123456
```

也支持语义化参数与自定义跳转：

```
http://localhost:8080/?username=admin&password=123456&redirect=/settings/profile
```

实现位置：`frontend/src/views/Login.vue` 的 `autoLoginFromURL()`。

> 安全提示：URL 中的密码会出现在浏览器历史中，仅建议内网使用。

---

## 十一、常见问题

| 问题 | 解决方案 |
|---|---|
| 登录后界面是英文 | 确认使用定制版 exe（`config init` 输出 `Locale: zh-cn`）；浏览器 `Ctrl+F5` 强刷新 |
| 界面没有 iOS 样式 | 浏览器缓存旧版 JS/CSS → `Ctrl+Shift+Delete` 清除缓存 |
| 提示 `password is too easy` | 使用了旧版 exe，请更新到定制版（弱密码校验已移除） |
| 网络共享目录报 `UNC paths not supported` | 使用定制版 bat（已用 `pushd` 解决） |
| 中文乱码 | 确认使用 GBK 编码的 bat（定制版已处理） |
| 端口 8080 被占用 | 运行 `stop.bat` 停止旧服务，或修改脚本 `PORT` 变量 |
| 忘记密码 | 重新运行 `filebrowser.bat`（强制重置 admin/123456），或 `users update` 重置 |

---

## 十二、技术栈与二次开发指引

### 技术栈

- **后端**：Go 1.21 + Gorilla Mux + BoltDB（嵌入式 KV 数据库）
- **前端**：Vue 3 + Vite 8 + Pinia + vue-i18n + vue-toastification
- **构建**：前端产物通过 `go:embed` 打进单个二进制

### 常用二次开发入口

| 需求 | 修改位置 |
|---|---|
| 改界面文案 | `frontend/src/i18n/zh-cn.json` |
| 改主题色/圆角 | `frontend/src/css/_variables.css`（配色）、`frontend/src/css/ios.css`（组件覆盖） |
| 新增页面 | `frontend/src/views/` + `frontend/src/router/index.ts` |
| 改登录逻辑 | `frontend/src/views/Login.vue` |
| 改用户 API | `http/users.go` + `frontend/src/api/users.ts` |
| 改密码策略 | `users/password.go` |
| 改默认设置 | `cmd/root.go`（quickSetup）、`settings/storage.go` |

### 调试技巧

```bash
# 后端日志
tail -f filebrowser.log

# 前端快速验证样式（无需重启后端）
cd frontend && pnpm run dev
```

---

## 版本记录

- **v2.63.15-custom-1**：基础定制（中文 + 品牌 + iOS 风格）
- **v2.63.15-custom-2**：弹窗/输入框/toast 深度 iOS 化，弱密码黑名单移除
- **v2.63.15-custom-3**：一键部署脚本（UNC 支持）+ 自动登录 + 外部链接移除 + 自定义 Logo 指引
