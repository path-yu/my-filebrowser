#Requires -Version 5.1
<#
.SYNOPSIS
  FileBrowser Windows build core (PowerShell, invoked by build-windows.bat).

.PARAMETER Arch
  amd64 (default) | arm64 | 386
.PARAMETER SkipFrontend
  Skip frontend build (reuse existing frontend/dist).
.PARAMETER SkipBackend
  Skip backend build (frontend only).
.PARAMETER Clean
  Remove output/, frontend/dist/* (keep .gitkeep), run go clean -cache, then exit.
#>
[CmdletBinding()]
param(
    [ValidateSet('amd64','arm64','386')]
    [string]$Arch = 'amd64',
    [switch]$SkipFrontend,
    [switch]$SkipBackend,
    [switch]$Clean
)

$ErrorActionPreference = 'Stop'

# ===== Lock ROOT to script directory (caller CWD does not matter) =====
$ROOT = Split-Path -Parent $MyInvocation.MyCommand.Path
$ROOT = (Resolve-Path -LiteralPath $ROOT).Path
Set-Location -LiteralPath $ROOT

$sep = '======================================================='
Write-Host $sep -ForegroundColor Cyan
Write-Host ('  FileBrowser build (Windows)  PowerShell {0}' -f $PSVersionTable.PSVersion) -ForegroundColor Cyan
Write-Host ('  ROOT         : {0}' -f $ROOT)
Write-Host ('  Arch         : windows/{0}' -f $Arch) -ForegroundColor Yellow
Write-Host ('  SkipFrontend : {0,-5}  SkipBackend : {1}' -f $SkipFrontend,$SkipBackend)
Write-Host $sep
Write-Host ''

# ===== Critical file assertion =====
$critical = @(
    (Join-Path $ROOT 'go.mod'),
    (Join-Path $ROOT 'main.go'),
    (Join-Path $ROOT 'frontend\package.json'),
    (Join-Path $ROOT 'frontend\assets.go')
)
foreach ($p in $critical) {
    if (-not (Test-Path -LiteralPath $p)) {
        Write-Host ('[FAIL] Missing required file: {0}' -f $p) -ForegroundColor Red
        Write-Host '       Make sure build-windows.bat / build-core.ps1 live in project root next to main.go.' -ForegroundColor Red
        exit 2
    }
}

# ===== Clean mode =====
if ($Clean) {
    Write-Host '[INFO ] clean ...'
    $outDir = Join-Path $ROOT 'output'
    if (Test-Path -LiteralPath $outDir) {
        Remove-Item -LiteralPath $outDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-Host '         rm output\'
    }
    $dist = Join-Path $ROOT 'frontend\dist'
    if (Test-Path -LiteralPath $dist) {
        Write-Host '         clear frontend\dist (keep .gitkeep) ...'
        Get-ChildItem -LiteralPath $dist -Force |
            Where-Object { $_.Name -ne '.gitkeep' } |
            ForEach-Object { Remove-Item -LiteralPath $_.FullName -Recurse -Force -ErrorAction Stop }
    }
    if (Get-Command go -ErrorAction SilentlyContinue) {
        Write-Host '[INFO ] go clean -cache ...'
        & go clean -cache 2>$null | Out-Null
    }
    Write-Host ''
    Write-Host '[OK  ] clean done.' -ForegroundColor Green
    exit 0
}

# ===== Helpers =====
function Get-ProgVersion([Parameter(Mandatory)][string]$Command) {
    try {
        $cmd = Get-Command $Command -ErrorAction Stop
        switch ($Command) {
            'go'    { [string](& $cmd.Source version 2>&1 | Select-Object -First 1) }
            'node'  { [string](& $cmd.Source -v 2>&1) }
            'pnpm'  { [string](& $cmd.Source -v 2>&1) }
            default { [string](& $cmd.Source --version 2>&1 | Select-Object -First 1) }
        }
    } catch { return $null }
}

function Fail-Missing([Parameter(Mandatory)][string]$Name, [string]$Hint='') {
    Write-Host ('[FAIL] {0} not found in PATH. Please install it first.' -f $Name) -ForegroundColor Red
    if ($Hint) { Write-Host ('       Download: {0}' -f $Hint) -ForegroundColor Red }
    exit 3
}

# ===== Environment detection =====
if (-not $SkipBackend) {
    Write-Host '[INFO ] Detect Go ...'
    $gv = Get-ProgVersion go
    if (-not $gv) { Fail-Missing 'go.exe' 'https://go.dev/dl/' }
    Write-Host ('         - {0}' -f $gv) -ForegroundColor Green
}
if (-not $SkipFrontend) {
    Write-Host '[INFO ] Detect Node.js ...'
    $nv = Get-ProgVersion node
    if (-not $nv) { Fail-Missing 'node.exe (>=24)' 'https://nodejs.org/' }
    Write-Host ('         - {0}' -f $nv) -ForegroundColor Green

    Write-Host '[INFO ] Detect pnpm ...'
    $pv = Get-ProgVersion pnpm
    if (-not $pv) {
        Write-Host '         pnpm not found; trying corepack enable ...' -ForegroundColor Yellow
        if (-not (Get-Command corepack -ErrorAction SilentlyContinue)) { Fail-Missing 'pnpm / corepack' 'Run: npm install -g pnpm' }
        & corepack enable 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Write-Host ('[FAIL] corepack enable exit={0}' -f $LASTEXITCODE) -ForegroundColor Red
            exit 4
        }
        $pv = Get-ProgVersion pnpm
        if (-not $pv) { Fail-Missing 'pnpm / corepack' }
    }
    Write-Host ('         - pnpm {0}' -f $pv) -ForegroundColor Green
}
Write-Host ''

# ===== ldflags version injection =====
$VERSION = 'dev'
$COMMIT  = 'nogit'
$hasGit = [bool](Get-Command git -ErrorAction SilentlyContinue)
if ($hasGit -and (Test-Path -LiteralPath (Join-Path $ROOT '.git'))) {
    try {
        $tag = & git -C $ROOT describe --tags --abbrev=0 --match=v* 2>$null
        if ($tag -match '^v(.+)$') { $VERSION = $Matches[1] }
    } catch {}
    try {
        $c = & git -C $ROOT log -1 --format='%h' 2>$null
        if ($c) { $COMMIT = $c }
    } catch {}
}
if ($VERSION -eq 'dev') {
    try {
        $pkg = Get-Content -LiteralPath (Join-Path $ROOT 'frontend\package.json') -Raw -Encoding UTF8 |
               ConvertFrom-Json -ErrorAction Stop
        if ($pkg.version) { $VERSION = $pkg.version }
    } catch {}
}
Write-Host ('[INFO ] version={0}  commit={1}' -f $VERSION,$COMMIT) -ForegroundColor Cyan
Write-Host ''

# ===== Output dir =====
$OUT = Join-Path $ROOT ('output\windows-{0}' -f $Arch)
New-Item -ItemType Directory -Path $OUT -Force | Out-Null
$EXE = Join-Path $OUT 'filebrowser.exe'

# ============================================================
#  [1/2] Frontend
# ============================================================
if (-not $SkipFrontend) {
    Write-Host '===  [1/2] Build frontend  ===' -ForegroundColor Cyan
    Push-Location -LiteralPath (Join-Path $ROOT 'frontend')
    try {
        Write-Host ('         cwd = {0}' -f (Get-Location))
        if (-not (Test-Path -LiteralPath 'node_modules')) {
            Write-Host '[INFO ] pnpm install --frozen-lockfile ...'
            & pnpm install --frozen-lockfile
            if ($LASTEXITCODE -ne 0) {
                Write-Host ('[FAIL] pnpm install exit={0}' -f $LASTEXITCODE) -ForegroundColor Red
                exit 11
            }
        } else {
            Write-Host '[INFO ] node_modules exists; skip install (to reinstall run: build-windows clean)'
        }
        Write-Host '[INFO ] pnpm run build (typecheck and vite build) ...'
        & pnpm run build
        if ($LASTEXITCODE -ne 0) {
            Write-Host ('[FAIL] pnpm run build exit={0}' -f $LASTEXITCODE) -ForegroundColor Red
            exit 12
        }
        $idx = Join-Path $ROOT 'frontend\dist\index.html'
        if (-not (Test-Path -LiteralPath $idx)) {
            Write-Host '[FAIL] frontend/dist/index.html missing after build (assets.go embed will fail)' -ForegroundColor Red
            exit 13
        }
    } finally { Pop-Location }
    Write-Host '[OK  ] frontend build done.' -ForegroundColor Green
    Write-Host ''
} else {
    Write-Host '[INFO ] SkipFrontend; reusing existing frontend/dist'
    $idx = Join-Path $ROOT 'frontend\dist\index.html'
    if (-not (Test-Path -LiteralPath $idx)) {
        Write-Host ('[FAIL] SkipFrontend failed: index.html not found at {0}' -f $idx) -ForegroundColor Red
        Write-Host '       Run a full build once first.' -ForegroundColor Red
        exit 14
    }
    Write-Host '         OK: dist\index.html exists' -ForegroundColor Green
    Write-Host ''
}

# ============================================================
#  [2/2] Backend
# ============================================================
if (-not $SkipBackend) {
    Write-Host ('===  [2/2] Build backend (windows/{0})  ===' -f $Arch) -ForegroundColor Cyan
    Write-Host '[INFO ] ldflags = -s -w'
    Write-Host ('                  -X github.com/filebrowser/filebrowser/v2/version.Version={0}' -f $VERSION)
    Write-Host ('                  -X github.com/filebrowser/filebrowser/v2/version.CommitSHA={0}' -f $COMMIT)

    $tmpExe = Join-Path $OUT 'filebrowser.exe.tmp'
    Remove-Item -LiteralPath $tmpExe -Force -ErrorAction SilentlyContinue

    Push-Location -LiteralPath $ROOT
    try {
        $env:CGO_ENABLED = '0'
        $env:GOOS        = 'windows'
        $env:GOARCH      = $Arch
        $ldflags = '-s -w -X "github.com/filebrowser/filebrowser/v2/version.Version={0}" -X "github.com/filebrowser/filebrowser/v2/version.CommitSHA={1}"' -f $VERSION,$COMMIT

        Write-Host ('[INFO ] go build -trimpath -ldflags "..." -o "{0}" .' -f $tmpExe)
        # NOTE: Do NOT write -ldflags=$ldflags (single token with `=`).
        # PowerShell 5.1 binds that inconsistently for native commands and Go
        # sometimes receives the literal string "$ldflags". Keep flag and value
        # as two separate tokens (space-separated), which is the idiomatic way
        # to invoke native CLIs from PowerShell and always expands correctly.
        & go build -trimpath -ldflags $ldflags -o $tmpExe .
        $exit = $LASTEXITCODE
    } finally {
        Remove-Item Env:\GOOS, Env:\GOARCH, Env:\CGO_ENABLED -ErrorAction SilentlyContinue
        Pop-Location
    }

    if ($exit -ne 0) {
        Remove-Item -LiteralPath $tmpExe -Force -ErrorAction SilentlyContinue
        Write-Host ('[FAIL] go build exit={0}' -f $exit) -ForegroundColor Red
        exit 21
    }

    # Atomic replace: if target locked, save to .new
    if (Test-Path -LiteralPath $EXE) {
        try {
            Remove-Item -LiteralPath $EXE -Force -ErrorAction Stop
        } catch {
            Write-Host '[WARN ] Existing filebrowser.exe is in use; saving as filebrowser.exe.new. Replace manually.' -ForegroundColor Yellow
            $EXE = Join-Path $OUT 'filebrowser.exe.new'
            Remove-Item -LiteralPath $EXE -Force -ErrorAction SilentlyContinue
        }
    }
    Move-Item -LiteralPath $tmpExe -Destination $EXE -Force -ErrorAction Stop

    $sz = (Get-Item -LiteralPath $EXE).Length
    $mb = [math]::Round($sz / 1MB, 2)

    Write-Host ''
    Write-Host $sep -ForegroundColor Green
    Write-Host '  BUILD SUCCESS' -ForegroundColor Green
    Write-Host ('  EXE   : {0}' -f $EXE)
    Write-Host ('  SIZE  : {0} bytes  ({1} MB)' -f $sz, $mb)
    $run = '"{0}" --address 0.0.0.0 --port 8080 --root "{1}\dev-files"' -f $EXE, $ROOT
    Write-Host ('  RUN   : {0}' -f $run)
    Write-Host $sep -ForegroundColor Green

    Write-Host ''
    try {
        $vline = & $EXE version 2>$null | Select-Object -First 1
        if ($vline) { Write-Host ('[INFO ] filebrowser version => {0}' -f $vline) }
    } catch {}
} else {
    Write-Host '[INFO ] SkipBackend; frontend step only.'
}

exit 0
