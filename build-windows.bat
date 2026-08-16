@echo off
REM ============================================================================
REM  FileBrowser  一键打包脚本（Windows .bat）
REM  输出产物：项目根目录 \output\windows-<arch>\filebrowser.exe
REM
REM  用法：
REM    .\build-windows.bat                          默认全量构建 (amd64)
REM    .\build-windows.bat arm64                    指定架构 amd64 / arm64 / 386
REM    .\build-windows.bat clean                    删除前端 dist + 旧 output + go 缓存
REM    .\build-windows.bat skipFrontend             跳过前端构建（已有 dist 时）
REM    .\build-windows.bat skipBackend              跳过后端构建（仅打包前端）
REM    .\build-windows.bat clean amd64 skipFrontend 参数可自由组合，顺序不限
REM ============================================================================

setlocal EnableExtensions EnableDelayedExpansion

pushd "%~dp0"
if errorlevel 1 (
    echo [ERROR] 无法切换到脚本所在目录: "%~dp0"
    exit /b 1
)

REM =======================  参数解析  =======================
set "CLEAN_ONLY=0"
set "SKIP_FRONTEND=0"
set "SKIP_BACKEND=0"
set "TARGET_ARCH=amd64"

:parseArgs
if "%~1"=="" goto parseEnd
set "A=%~1"
if /I "%A%"=="clean"        ( set "CLEAN_ONLY=1"   & shift & goto parseArgs )
if /I "%A%"=="skipfrontend" ( set "SKIP_FRONTEND=1" & shift & goto parseArgs )
if /I "%A%"=="skipbackend"  ( set "SKIP_BACKEND=1"  & shift & goto parseArgs )
if /I "%A%"=="amd64"        ( set "TARGET_ARCH=amd64" & shift & goto parseArgs )
if /I "%A%"=="arm64"        ( set "TARGET_ARCH=arm64" & shift & goto parseArgs )
if /I "%A%"=="386"          ( set "TARGET_ARCH=386"   & shift & goto parseArgs )
echo [WARN]  忽略未知参数: %A%
shift
goto parseArgs
:parseEnd

REM =======================  日志辅助  =======================
set "LOG_INFO=[INFO ]"
set "LOG_WARN=[WARN ]"
set "LOG_ERR=[ERROR]"

set "ESC="
for /F "delims=#" %%E in ('"prompt #$E# & for %%E in (1) do rem"') do set "ESC=%%E"
set "C_RESET=%ESC%[0m"
set "C_GREEN=%ESC%[92m"
set "C_YELLOW=%ESC%[93m"
set "C_CYAN=%ESC%[96m"
set "C_RED=%ESC%[91m"

title FileBrowser Build - arch=%TARGET_ARCH%

echo =======================================================
echo   %C_CYAN%FileBrowser%EC_RESET%  一键打包脚本 (Windows)
echo   架构  : %C_YELLOW%windows/%TARGET_ARCH%%C_RESET%
echo   目录  : %C_CYAN%%CD%%C_RESET%
echo =======================================================
echo.

REM =======================  Clean 模式  =======================
if "%CLEAN_ONLY%"=="1" (
    echo %LOG_INFO% 执行 clean ...
    if exist "output" ( rmdir /S /Q "output" & echo %LOG_INFO%   已删除 output\ )
    if exist "frontend\dist" (
        echo %LOG_INFO%   清理 frontend\dist （保留 .gitkeep）...
        pushd "frontend\dist" || ( echo %LOG_ERR% 无法进入 frontend\dist & exit /b 2 )
        for /F "delims=" %%F in ('dir /B * ^| findstr /V /I /L /C:".gitkeep"') do (
            if exist "%%F\" ( rmdir /S /Q "%%F" ) else ( del /F /Q "%%F" )
        )
        popd
    )
    echo %LOG_INFO% 执行 go clean -cache ...
    go clean -cache 2>nul
    echo.
    echo %C_GREEN%%LOG_INFO% clean 完成%^C_RESET%
    exit /b 0
)

REM =======================  环境检测  =======================
set "NEED_NODE=0"
set "NEED_GO=0"
if "%SKIP_FRONTEND%"=="0" set "NEED_NODE=1"
if "%SKIP_BACKEND%"=="0"  set "NEED_GO=1"

if "%NEED_NODE%"=="1" (
    echo %LOG_INFO% 检测 Node.js ...
    where node >nul 2>&1
    if errorlevel 1 (
        echo %LOG_ERR% 未找到 node.exe，请先安装 Node.js ^>= 24 并加入 PATH
        echo          下载: https://nodejs.org/
        exit /b 3
    )
    for /F "tokens=*" %%V in ('node -v 2^>nul') do set "NODE_VER=%%V"
    echo   - %C_GREEN%Node.js %NODE_VER%%C_RESET%

    echo %LOG_INFO% 检测 pnpm ...
    where pnpm >nul 2>&1
    if errorlevel 1 (
        echo %LOG_WARN% 未找到 pnpm，尝试通过 corepack enable 启用 ...
        where corepack >nul 2>&1
        if errorlevel 1 (
            echo %LOG_ERR% 未找到 pnpm 或 corepack，请先 `npm install -g pnpm` ^>= 10
            exit /b 3
        )
        corepack enable || ( echo %LOG_ERR% corepack enable 失败 & exit /b 3 )
        corepack prepare pnpm@latest --activate 2>nul
    )
    for /F "tokens=*" %%V in ('pnpm -v 2^>nul') do set "PNPM_VER=%%V"
    echo   - %C_GREEN%pnpm %PNPM_VER%%C_RESET%
)

if "%NEED_GO%"=="1" (
    echo %LOG_INFO% 检测 Go ...
    where go >nul 2>&1
    if errorlevel 1 (
        echo %LOG_ERR% 未找到 go.exe，请先安装 Go 并加入 PATH
        echo          下载: https://go.dev/dl/
        exit /b 3
    )
    for /F "tokens=*" %%V in ('go version 2^>nul') do set "GO_VER=%%V"
    echo   - %C_GREEN%%GO_VER%%C_RESET%
)
echo.

REM =======================  版本信息（给 ldflags 注入）  =======================
set "VERSION=dev"
set "GIT_COMMIT=nogit"

if exist ".git" (
    for /F "delims=" %%V in ('git describe --tags --abbrev^=0 --match^=v* 2^>nul ^| findstr /R "^v"') do (
        set "VERSION=%%V"
        set "VERSION=!VERSION:v=!"
    )
    for /F "delims=" %%V in ('git log -n 1 --format^=%%h 2^>nul') do set "GIT_COMMIT=%%V"
) else (
    if exist "frontend\package.json" (
        for /F "usebackq tokens=2 delims=:,  " %%K in (` findstr /I /C:"\"version\"" "frontend\package.json" 2^>nul `) do (
            set "VERSION=%%~K"
        )
    )
)
echo %LOG_INFO% 元信息: Version=%C_CYAN%%VERSION%%C_RESET%  Commit=%C_CYAN%%GIT_COMMIT%%C_RESET%
echo.

REM =======================  产物目录  =======================
set "OUT_DIR=output\windows-%TARGET_ARCH%"
if not exist "%OUT_DIR%" mkdir "%OUT_DIR%"

set "EXE_NAME=filebrowser.exe"

REM =======================  1. 前端构建  =======================
if "%SKIP_FRONTEND%"=="0" (
    echo ============  %C_CYAN%[1/2] 构建前端%^C_RESET%  ============
    pushd "frontend" || ( echo %LOG_ERR% 无法进入 frontend\ & exit /b 4 )

    REM -- node_modules 存在时跳过 pnpm install，除非缺失
    if not exist "node_modules" (
        echo %LOG_INFO% 首次依赖安装: pnpm install --frozen-lockfile
        pnpm install --frozen-lockfile
        if errorlevel 1 (
            echo %LOG_ERR% pnpm install 失败
            popd
            exit /b 5
        )
    ) else (
        echo %LOG_INFO% node_modules 已存在，跳过 install。如需重装请 `build-windows clean`。
    )

    echo %LOG_INFO% pnpm run build  ( typecheck + vite build )
    pnpm run build
    if errorlevel 1 (
        echo %LOG_ERR% 前端构建失败，请检查上面的错误信息
        popd
        exit /b 6
    )

    REM 最小健康度校验：assets.go 用 //go:embed dist/*，dist/index.html 必须存在
    if not exist "dist\index.html" (
        echo %LOG_ERR% 异常：前端构建完成但 dist\index.html 不存在
        popd
        exit /b 7
    )
    popd
    echo %C_GREEN%%LOG_INFO% 前端构建成功 ^(frontend\dist^)%C_RESET%
    echo.
) else (
    echo %LOG_WARN% 已设置 skipFrontend：跳过前端构建，直接使用现有 frontend\dist
    if not exist "frontend\dist\index.html" (
        echo %LOG_ERR% 无法跳过：frontend\dist\index.html 不存在，请先构建前端
        exit /b 8
    )
    echo.
)

REM =======================  2. 后端构建  =======================
if "%SKIP_BACKEND%"=="0" (
    echo ============  %C_CYAN%[2/2] 构建后端 EXE%^C_RESET%  ============
    echo %LOG_INFO% 目标平台: windows / %TARGET_ARCH%
    echo %LOG_INFO% 注入 ldflags:
    echo           -s -w
    echo           -X github.com/filebrowser/filebrowser/v2/version.Version=%VERSION%
    echo           -X github.com/filebrowser/filebrowser/v2/version.CommitSHA=%GIT_COMMIT%

    REM -- 构建输出临时文件（避免覆盖正在运行的同名 filebrowser.exe）
    set "BUILD_TMP=%OUT_DIR%\%EXE_NAME%.tmp"
    set "BUILD_FINAL=%OUT_DIR%\%EXE_NAME%"
    if exist "%BUILD_TMP%" del /F /Q "%BUILD_TMP%"

    set CGO_ENABLED=0
    set GOOS=windows
    set GOARCH=%TARGET_ARCH%

    go build ^
        -trimpath ^
        -ldflags="-s -w -X \"github.com/filebrowser/filebrowser/v2/version.Version=%VERSION%\" -X \"github.com/filebrowser/filebrowser/v2/version.CommitSHA=%GIT_COMMIT%\"" ^
        -o "%BUILD_TMP%" ^
        .
    if errorlevel 1 (
        echo %LOG_ERR% Go 构建失败 (exit=%errorlevel%)，请检查上面的错误信息
        if exist "%BUILD_TMP%" del /F /Q "%BUILD_TMP%"
        exit /b 9
    )

    REM -- 原子替换（先删旧的，再改名）
    if exist "%BUILD_FINAL%" (
        del /F /Q "%BUILD_FINAL%" 2>nul
        if exist "%BUILD_FINAL%" (
            echo %LOG_WARN% 旧文件 %EXE_NAME% 正被占用，本次保留为 .new，你可以稍后自行替换
            move /Y "%BUILD_TMP%" "%OUT_DIR%\%EXE_NAME%.new" >nul
            set "BUILD_FINAL=%OUT_DIR%\%EXE_NAME%.new"
        )
    )
    if exist "%BUILD_TMP%" move /Y "%BUILD_TMP%" "%BUILD_FINAL%" >nul

    if not exist "%BUILD_FINAL%" (
        echo %LOG_ERR% 产物不存在: %BUILD_FINAL%
        exit /b 10
    )

    REM -- 基础大小/信息打印
    for %%F in ("%BUILD_FINAL%") do set "EXE_SIZE=%%~zF"
    echo %LOG_INFO% 产物路径: %C_GREEN%%CD%\%BUILD_FINAL%\%C_RESET%
    echo %LOG_INFO% 产物大小: %C_YELLOW%!EXE_SIZE! bytes%C_RESET%

    for /F "tokens=*" %%R in ('"%BUILD_FINAL%" version 2^>nul') do (
        echo %LOG_INFO% 版本自检: %%R
        goto :versionCheckDone
    )
    :versionCheckDone

    echo.
    echo %C_GREEN%============  后端构建成功  ============%C_RESET%
    echo.
) else (
    echo %LOG_WARN% 已设置 skipBackend：跳过后端构建
)

REM =======================  收尾  =======================
echo =======================================================
echo  构建结束
if "%SKIP_BACKEND%"=="0" (
    echo   EXE   : %C_GREEN%%CD%\%OUT_DIR%\%EXE_NAME%%C_RESET%
    echo   启动  : %C_CYAN%^"%OUT_DIR%\%EXE_NAME%^" --address 0.0.0.0 --port 8080 --root .\%C_RESET%
)
if "%SKIP_FRONTEND%"=="0" (
    echo   Dist  : %CD%\frontend\dist\index.html  ^(已嵌入到上列 EXE 中^)
)
echo =======================================================

REM  提示：如需单 exe 分发，直接复制上面 EXE 即可，frontend\dist 已 embed。
endlocal
exit /b 0
