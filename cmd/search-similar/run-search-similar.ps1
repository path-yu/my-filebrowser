# search-similar 一键运行脚本（双击 .ps1 不方便时，可以在 CMD 里跑：
#   powershell -ExecutionPolicy Bypass -File .\run-search-similar.ps1
# ）
#
# 功能：
#   1. 提示用户输入 PDF 路径（或直接把 PDF 拖进窗口回车）
#   2. 提示用户输入 Top-K（默认 10）
#   3. 调用 search-similar.exe，把 stdout+stderr 同时输出到屏幕和 search-similar.log
#   4. 运行完毕自动用 记事本 打开 search-similar.log，方便回看完整结果

$ErrorActionPreference = "Continue"

# 切换到脚本所在目录（search-similar.exe 应该就在旁边）
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ScriptDir

Write-Host "══════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "🔎 图纸相似向量检索 — 一键运行脚本" -ForegroundColor Cyan
Write-Host "══════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host ""

# --- 检查 exe 是否存在 ---
$ExePath = Join-Path $ScriptDir "search-similar.exe"
if (-not (Test-Path $ExePath)) {
    Write-Host "❌ 找不到 search-similar.exe！请先 go build 编译。" -ForegroundColor Red
    Read-Host "按回车退出"
    exit 1
}

# --- 让用户输入 PDF 路径（支持拖文件进窗口） ---
Write-Host "💡 请把要检索的 PDF 文件直接用鼠标拖进这个窗口，然后按回车" -ForegroundColor Yellow
Write-Host "   （也可以手动输入绝对路径，含中文/空格/括号请对 CMD 加英文双引号，这里不用）" -ForegroundColor Gray
$pdf = Read-Host "PDF 路径"
$pdf = $pdf.Trim('"', "'", ' ')
if ([string]::IsNullOrWhiteSpace($pdf)) {
    Write-Host "❌ 路径为空，退出" -ForegroundColor Red
    Read-Host "按回车退出"
    exit 1
}
if (-not (Test-Path -LiteralPath $pdf)) {
    Write-Host ""
    Write-Host "⚠️  警告: 路径 $pdf 似乎不存在" -ForegroundColor Yellow
    Write-Host "   之前建库的扫描目录是 D:\BaiduNetdiskDownload\图纸\图纸\" -ForegroundColor Gray
    Write-Host "   ph2 测试目录: D:\BaiduNetdiskDownload\ph2\" -ForegroundColor Gray
    $cont = Read-Host "   仍然继续尝试吗？(y/N)"
    if ($cont -notmatch '^[yY]') {
        exit 1
    }
} else {
    $f = Get-Item -LiteralPath $pdf
    Write-Host "✅ 文件存在: $($f.Name) ($([math]::Round($f.Length/1MB,2)) MB)" -ForegroundColor Green
}

# --- TopK ---
Write-Host ""
$topkRaw = Read-Host "返回 Top-K 最相似（默认 10）"
if ([string]::IsNullOrWhiteSpace($topkRaw)) {
    $topk = 10
} else {
    $topk = 10
    [int]::TryParse($topkRaw.Trim(), [ref]$topk) | Out-Null
    if ($topk -le 0) { $topk = 10 }
}
Write-Host "📊 TopK = $topk" -ForegroundColor Cyan

# --- 设置 PROJECT_ROOT ---
# 脚本目录在 <项目根>\cmd\search-similar\ → 往上 2 级就是项目根
$projectRoot = Resolve-Path (Join-Path $ScriptDir "..\..\")
$env:PROJECT_ROOT = $projectRoot
Write-Host "📍 PROJECT_ROOT = $projectRoot" -ForegroundColor Cyan
Write-Host ""

# --- 运行 exe，stdout + stderr 同时 tee 到文件 + 屏幕 ---
$logPath = Join-Path $ScriptDir "search-similar.log"
Remove-Item -Force -ErrorAction SilentlyContinue $logPath

Write-Host "🚀 开始检索 ..." -ForegroundColor Magenta
Write-Host "（如果屏幕滚动太快，结束后会自动打开记事本查看完整日志）" -ForegroundColor Gray
Write-Host ""

& $ExePath $pdf $topk 2>&1 | Tee-Object -FilePath $logPath | Write-Host

Write-Host ""
Write-Host "══════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "✅ 运行结束" -ForegroundColor Green
Write-Host "📝 日志路径: $logPath" -ForegroundColor Cyan
Write-Host "   正在用记事本打开日志 ..." -ForegroundColor Gray
notepad.exe $logPath
Read-Host "按回车退出"
