<#
  One-click script for:
    (a) building filebrowser.exe with similar-PDF vector search enabled, OR
    (b) running filebrowser with extra args directly.

  Why this script is needed (vs plain `go run` / `go build`):
    - github.com/yalue/onnxruntime_go REQUIRES CGO, so CGO_ENABLED MUST be 1
      AND a MinGW-w64 gcc.exe MUST be on PATH.
    - The Go project path often contains " (" / spaces
      (e.g. "filebrowser-release-20260814 (2)").  MinGW ld.exe treats '(' as a
      command line terminator and fails with "cannot find D:/code/...".
      We work around this by creating two NTFS Junctions to bracket-free paths
      under %TEMP%: <temp>\fb_proj (-> project root) and <temp>\fb_mingw64
      (-> dev-files/mingw64).

  ============================================================
  【重要】参数传递 — 请严格按照以下示例使用，否则会报错！
  ============================================================

  ❌ 绝对不要这样写（会报错 MissingArgument / 参数找不到）：
       -RawArgs @("-d", ".\db", "-r", "D:\...")   ← @() 数组字面量在 -File 模式下不支持
       -d .\db -r "D:\..." -a 127.0.0.1 -p 8080   ← -d/-r/-a/-p 等短 flag 在 -File 模式下
                                                     会被 PowerShell 参数绑定器错误解析
                                                     （有的丢失，有的报 NamedParameterNotFound）

  ✅ 唯一推荐：使用脚本的快捷长参数（名称首字母都不冲突）：
       -DbFile       对应 filebrowser -d   (数据库文件路径)
       -JRootPath    对应 filebrowser -r   (文件根目录)
       -KBindAddr    对应 filebrowser -a   (绑定地址)
       -QListenPort  对应 filebrowser -p   (监听端口)

  --- 用法示例 ---

  # 1) 仅编译，生成 output\filebrowser.exe
  powershell -ExecutionPolicy Bypass -File .\run-filebrowser.ps1 -Mode Build

  # 2) 运行 + 启用相似 PDF 检索（推荐，多行写法）
  powershell -ExecutionPolicy Bypass -File .\run-filebrowser.ps1 -Mode Run `
    -DbFile .\filebrowser.db `
    -JRootPath "D:\BaiduNetdiskDownload\图纸\图纸" `
    -KBindAddr 127.0.0.1 `
    -QListenPort 8080

  # 3) 同上，单行（复制粘贴友好）
  powershell -ExecutionPolicy Bypass -File .\run-filebrowser.ps1 -Mode Run -DbFile .\filebrowser.db -JRootPath "D:\BaiduNetdiskDownload\图纸\图纸" -KBindAddr 127.0.0.1 -QListenPort 8080

  # 4) 不启用相似 PDF 检索（纯文件服务，不需要 MinGW / CGO）
  powershell -ExecutionPolicy Bypass -File .\run-filebrowser.ps1 -Mode Run -NoDrawingSearch -DbFile .\filebrowser.db -JRootPath "D:\BaiduNetdiskDownload\图纸\图纸" -KBindAddr 127.0.0.1 -QListenPort 8080
#>

param(
  [Parameter(Mandatory = $false)]
  [ValidateSet("Run", "Build")]
  [string]$Mode = "Run",

  [Parameter(Mandatory = $false)]
  [switch]$NoDrawingSearch,

  [Parameter(Mandatory = $false)]
  [string]$OutputExe = "",

  # ---------- 快捷参数：对应 filebrowser 的 4 个常用 flag ----------
  #   命名规则：首字母 D/J/K/Q 唯一，绝不与其他参数首字母 (M/N/O) 冲突，
  #   这样在 "powershell -File ..." 模式下不会发生前缀匹配歧义。
  #   【注意】-File 模式下不要用 -d/-r/-a/-p 等短 flag，PowerShell 参数绑定器
  #   会误解析（有的丢失、有的报 NamedParameterNotFound）。请只用下方的长参数。
  [Parameter(Mandatory = $false)]
  [string]$DbFile = "",

  [Parameter(Mandatory = $false)]
  [string]$JRootPath = "",

  [Parameter(Mandatory = $false)]
  [string]$KBindAddr = "",

  [Parameter(Mandatory = $false)]
  [string]$QListenPort = "",

  # ---------- 剩余参数捕获（预留，不作为主要传参入口） ----------
  #   首字母 Z 与所有其他参数 (M/N/O/D/J/K/Q) 不冲突。
  #   【注意】由于 -File 模式下 PowerShell 对以 "-" 开头的 token 会尝试绑定到
  #   具名参数，因此不要指望通过这里捕获 -d/-r 等短 flag，仅限捕获纯位置参数
  #   （例如额外的自定义路径等不带 "-" 的 token）。
  [Parameter(Mandatory = $false, ValueFromRemainingArguments = $true)]
  [Alias("ExtraArgs", "Flags", "FilebrowserArgs")]
  [string[]]$ZArgs = @()
)

$ErrorActionPreference = "Stop"

# ---------- 1. Resolve project root (script folder, go.mod sibling) ----------
$ProjectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
Write-Host ""
Write-Host ("Project root: {0}" -f $ProjectRoot) -ForegroundColor Cyan

if (-not (Test-Path (Join-Path $ProjectRoot "go.mod"))) {
  throw "go.mod not found next to run-filebrowser.ps1. Place this script in project root (next to main.go)."
}

# ---------- 2. Decide build & runtime parameters ----------
$FinalRoot = $ProjectRoot
$env:CGO_ENABLED = "0"
$TagArgs = @()

function To-UnixPath {
  param([string]$p)
  if ([string]::IsNullOrEmpty($p)) { return "" }
  return $p.Replace('\','/')
}

function Ensure-Junction {
  param(
    [Parameter(Mandatory = $true)][string]$Link,
    [Parameter(Mandatory = $true)][string]$Target,
    [Parameter(Mandatory = $true)][string]$Label
  )
  if (Test-Path -LiteralPath $Link) {
    $item = $null
    try { $item = Get-Item -LiteralPath $Link -Force } catch {}
    if ($item) {
      $recreate = $true
      if (($item.LinkType -like "*Junction*") -or ($item.LinkType -like "*SymbolicLink*")) {
        $actual = $item.Target
        if ($actual -and $actual.Count -gt 0) {
          try {
            $resolved = (Resolve-Path -LiteralPath $actual[0] -ErrorAction SilentlyContinue).Path
            $expected = (Resolve-Path -LiteralPath $Target).Path
            if ($resolved -ieq $expected) {
              $recreate = $false
              Write-Host ("   [ok   ] {0} Junction {1} -> {2}" -f $Label, $Link, $Target) -ForegroundColor DarkGray
            }
          } catch { }
        }
      }
      if ($recreate) {
        Write-Warning ("{0} Junction {1} target mismatch, recreating..." -f $Label, $Link)
        try {
          Remove-Item -LiteralPath $Link -Recurse -Force -ErrorAction Stop
        } catch {
          # Some sandboxed shells block direct removal; fall back to cmd rd on reparse points
          & cmd /c rd /s /q ("`"$Link`"") | Out-Null
          Start-Sleep -Milliseconds 300
        }
      }
    }
  }
  if (-not (Test-Path -LiteralPath $Link)) {
    # Guarantee parent folder exists
    $Parent = Split-Path -Parent $Link
    if ($Parent -and -not (Test-Path -LiteralPath $Parent)) {
      New-Item -ItemType Directory -Path $Parent -Force | Out-Null
    }
    $created = $false
    try {
      New-Item -ItemType Junction -Path $Link -Target $Target -Force | Out-Null
      $created = Test-Path -LiteralPath $Link
    } catch { }
    if (-not $created) {
      # cmd /c mklink /J never depends on the shell's filesystem view
      $out = & cmd /c mklink /J ("`"$Link`"") ("`"$Target`"") 2>&1 | Out-String
      Start-Sleep -Milliseconds 300
      if (-not (Test-Path -LiteralPath $Link)) {
        throw "Cannot create Junction $Link -> $Target. mklink output: $out. Run this script from a normal (non-sandbox) PowerShell, or create the two junctions manually."
      }
    }
    Write-Host ("   [new  ] {0} Junction {1} -> {2}" -f $Label, $Link, $Target) -ForegroundColor Green
  }
}

if (-not $NoDrawingSearch) {
  # ---------- 2a. DrawingSearch ON: create junctions + configure env ----------
  Write-Host "Mode: ENABLE similar PDF search (-tags drawingsearch) -> requires MinGW-w64 + CGO" -ForegroundColor Green

  $MingwGcc = Join-Path $ProjectRoot "dev-files\mingw64\bin\gcc.exe"
  if (-not (Test-Path $MingwGcc)) {
    throw @"
Missing MinGW-w64 GCC: $MingwGcc
Download a WinLibs UCRT build (POSIX threads + SEH) from https://winlibs.com/ ,
extract it into $ProjectRoot\dev-files\mingw64\  so that mingw64\bin\gcc.exe
exists.
"@
  }

  # Place junctions under %TEMP% to avoid sandbox ACL restrictions on C:\.
  $TempRoot = $env:TEMP
  if ([string]::IsNullOrWhiteSpace($TempRoot)) { $TempRoot = Join-Path $ProjectRoot "_junction_temp" }
  $JuncProject = Join-Path $TempRoot "fb_proj"
  $JuncMingw   = Join-Path $TempRoot "fb_mingw64"

  Ensure-Junction -Link $JuncProject -Target $ProjectRoot                                -Label "Project"
  Ensure-Junction -Link $JuncMingw   -Target (Join-Path $ProjectRoot "dev-files\mingw64") -Label "MinGW"

  $FinalRoot = $JuncProject
  $env:CGO_ENABLED = "1"
  $env:PROJECT_ROOT = $FinalRoot

  # Binaries (gcc + pdftoppm) - use junction paths so no parens ever appear.
  $env:PATH = "$JuncMingw\bin;$FinalRoot\dev-files\poppler-25.12.0\Library\bin;" + $env:PATH

  # Detect GCC version subfolder
  $MingwTriple = "x86_64-w64-mingw32"
  $GccVerDirs = Get-ChildItem -LiteralPath "$JuncMingw\lib\gcc\$MingwTriple" -Directory -ErrorAction SilentlyContinue | Sort-Object Name -Descending
  if (-not $GccVerDirs -or $GccVerDirs.Count -eq 0) {
    throw "No GCC version folder found under $JuncMingw\lib\gcc\$MingwTriple. Your MinGW-w64 extract may be incomplete."
  }
  $GccVer = $GccVerDirs[0].Name
  Write-Host ("   GCC version: lib\gcc\{0}\{1}" -f $MingwTriple, $GccVer) -ForegroundColor DarkGray

  # Environment variables that MinGW ld / cpp actually honour.  All paths must
  # be bracket-free so ld.exe never stops parsing mid-path.
  $MingwUnix = To-UnixPath $JuncMingw
  $env:GCC_EXEC_PREFIX = "$MingwUnix/libexec/gcc/"
  $env:LIBRARY_PATH    = "$MingwUnix/lib/gcc/$MingwTriple/$GccVer;$MingwUnix/$MingwTriple/lib;$MingwUnix/lib"
  $env:CPATH           = "$MingwUnix/include;$MingwUnix/$MingwTriple/include"
  # Force Go to pick up exactly the right gcc.exe (never fall back to any other
  # gcc that may exist elsewhere on PATH with a different $GCC_EXEC_PREFIX layout).
  $env:CC  = "$MingwUnix/bin/gcc.exe"
  $env:CXX = "$MingwUnix/bin/g++.exe"

  $TagArgs = @("-tags", "drawingsearch")
} else {
  Write-Host "Mode: -NoDrawingSearch => skipping MinGW / drawingsearch build tag" -ForegroundColor Green
}

# ---------- 2b. Diagnostics: detect common wrong param patterns ----------
#   Many users copy old tutorials and write:
#     -RawArgs @("-d", ...)      <- @() array literals don't work with -File
#     -d .\db -r "D:\" -a ... -p ...   <- short flags get mis-parsed by -File
#   We detect these footprints and show a clear warning + remediation.
$dashPrefixTokens = @()
$foundWrongAliases = @()
foreach ($tok in $ZArgs) {
  if (-not $tok) { continue }
  $t = [string]$tok
  if ($t.Length -gt 1 -and $t.Substring(0,1) -eq '-') {
    if ($t -eq '-RawArgs' -or $t -eq '-ExtraArgs' -or $t -eq '-Flags' -or $t -eq '-FilebrowserArgs') {
      $foundWrongAliases += $t
    } elseif ($t -ne '--') {
      $dashPrefixTokens += $t
    }
  }
}
if ($foundWrongAliases.Count -gt 0 -or $dashPrefixTokens.Count -gt 0) {
  Write-Host ""
  $msg = "WARNING: Possibly incorrect argument style detected!`n"
  if ($foundWrongAliases.Count -gt 0) {
    $msg += "  - Found old-style aliases: $($foundWrongAliases -join ', '). Do NOT use @() array literals with powershell -File mode.`n"
  }
  if ($dashPrefixTokens.Count -gt 0) {
    $msg += "  - Found dash-prefixed tokens in catch-all ZArgs: $($dashPrefixTokens -join ', '). Short flags -d/-r/-a/-p are NOT reliable under powershell -File (some get silently lost, some throw NamedParameterNotFound).`n"
  }
  $msg += "`nCORRECT WAY: Use the 4 shortcut long-params (unique first letters, never ambiguous):`n"
  $msg += "  powershell -ExecutionPolicy Bypass -File .\run-filebrowser.ps1 -Mode Run ``n"
  $msg += "    -DbFile .\filebrowser.db ``n"
  $msg += "    -JRootPath 'D:\BaiduNetdiskDownload\图纸\图纸' ``n"
  $msg += "    -KBindAddr 127.0.0.1 ``n"
  $msg += "    -QListenPort 8080`n"
  $msg += "`nThe script will still try to forward current args, but results are likely wrong. Please re-run with the correct style above."
  Write-Warning $msg
  Write-Host ""
}

# ---------- 2c. Assemble filebrowser CLI args ----------
#   来源：
#     1. ZArgs (ValueFromRemainingArguments 捕获到的位置参数，通常为空)
#     2. 4 个快捷长参数 (LAST，会覆盖对应位置的 flag)
$cliArgs = New-Object System.Collections.Generic.List[string]
foreach ($a in $ZArgs)  { if ($null -ne $a -and $a -ne "") { $cliArgs.Add([string]$a) } }
function Set-ShortcutFlag {
  param(
    [string]$Flag,
    [string]$Value,
    [System.Collections.Generic.List[string]]$List
  )
  if ([string]::IsNullOrWhiteSpace($Value)) { return }
  $newList = New-Object System.Collections.Generic.List[string]
  $skipNext = $false
  for ($i = 0; $i -lt $List.Count; $i++) {
    if ($skipNext) { $skipNext = $false; continue }
    $tok = $List[$i]
    if ($tok -ceq $Flag -or $tok -eq $Flag) { $skipNext = $true; continue }
    $newList.Add($tok)
  }
  $newList.Add($Flag)
  $newList.Add([string]$Value)
  $List.Clear()
  foreach ($t in $newList) { $List.Add($t) }
}
Set-ShortcutFlag -Flag "-d" -Value $DbFile       -List $cliArgs
Set-ShortcutFlag -Flag "-r" -Value $JRootPath    -List $cliArgs
Set-ShortcutFlag -Flag "-a" -Value $KBindAddr    -List $cliArgs
Set-ShortcutFlag -Flag "-p" -Value $QListenPort  -List $cliArgs
[string[]]$RunArgArray = $cliArgs.ToArray()

# ---------- 3. Execute ----------
Push-Location -LiteralPath $FinalRoot
try {
  Write-Host ("Working dir: {0}" -f (Get-Location).Path) -ForegroundColor Cyan
  Write-Host ("CGO_ENABLED = {0}" -f $env:CGO_ENABLED) -ForegroundColor Cyan
  if ($TagArgs.Count -gt 0) {
    Write-Host ("Go tags:      {0}" -f ($TagArgs -join " ")) -ForegroundColor Cyan
  }

  if ($Mode -eq "Build") {
    if ([string]::IsNullOrWhiteSpace($OutputExe)) {
      $OutDir = Join-Path $ProjectRoot "output"
      if (-not (Test-Path $OutDir)) { New-Item -ItemType Directory -Path $OutDir -Force | Out-Null }
      $OutputExe = Join-Path $OutDir "filebrowser.exe"
    }
    if (-not [System.IO.Path]::IsPathRooted($OutputExe)) {
      $OutputExe = [System.IO.Path]::GetFullPath((Join-Path $ProjectRoot $OutputExe))
    }
    $OutDir = Split-Path -Parent $OutputExe
    if (-not (Test-Path $OutDir)) { New-Item -ItemType Directory -Path $OutDir -Force | Out-Null }

    Write-Host ""
    Write-Host ("Building exe -> {0}" -f $OutputExe) -ForegroundColor Yellow
    $buildArgs = @("build") + $TagArgs + @("-trimpath", "-ldflags", "-s -w", "-o", $OutputExe, ".")
    Write-Host ("  go {0}" -f ($buildArgs -join " ")) -ForegroundColor DarkGray
    & go @buildArgs
    $ec = $LASTEXITCODE
    if ($ec -ne 0) { throw "go build exited with code $ec" }
    $f = Get-Item $OutputExe
    Write-Host ("`nOK build: {0} ({1:N2} MB)" -f $f.FullName, ($f.Length / 1MB)) -ForegroundColor Green
  } else {
    Write-Host ""
    if ($RunArgArray.Count -gt 0) {
      Write-Host ("Launching filebrowser (args: {0})" -f ($RunArgArray -join " ")) -ForegroundColor Yellow
    } else {
      Write-Host "Launching filebrowser (no extra args)" -ForegroundColor Yellow
    }
    $runArgs = @("run") + $TagArgs + @(".") + $RunArgArray
    & go @runArgs
    $ec = $LASTEXITCODE
    if ($ec -ne 0) {
      Write-Warning ("filebrowser exited with code {0}" -f $ec)
    }
  }
} finally {
  Pop-Location
}
