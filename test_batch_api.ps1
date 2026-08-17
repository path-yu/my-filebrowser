$ErrorActionPreference='Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

Write-Host "=== [1/3] Build backend ==="
Get-Process -Name filebrowser -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
$build = & go build -o filebrowser.exe . 2>&1
if ($LASTEXITCODE -ne 0) {
  Write-Host "BUILD FAIL: $build"
  exit 1
}
Write-Host "build ok"

Write-Host ""
Write-Host "=== [2/3] Start server ==="
$proc = Start-Process -FilePath ".\filebrowser.exe" -ArgumentList '-d','.\dev.db' -PassThru -NoNewWindow
for ($i=0; $i -lt 40; $i++) {
  if (Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -eq 8080 -and $_.OwningProcess -eq $proc.Id }) { break }
  Start-Sleep -Milliseconds 350
}

# 登录：拿到 token + session cookie
$loginResp = Invoke-WebRequest -Uri "http://127.0.0.1:8080/api/login" -Method Post -Body '{"username":"admin","password":"123456"}' -ContentType "application/json" -UseBasicParsing -SessionVariable sess
$token = $loginResp.Content.Trim()
Write-Host "server online, token(len=$($token.Length))"

function RunCase($name, $bodyObj, [int]$expectedStatus) {
  if ($null -eq $bodyObj) { $json = $null }
  elseif ($bodyObj -is [string]) { $json = $bodyObj }
  else { $json = ConvertTo-Json $bodyObj -Depth 10 -Compress }
  Write-Host ""
  Write-Host "CASE: $name"
  Write-Host "  payload: $(if ($null -eq $json) {'<no body>'} else {$json})"
  try {
    $p = @{
      Uri = "http://127.0.0.1:8080/api/productcode/batch"
      Method = "Post"
      ContentType = "application/json"
      Headers = @{ "X-Auth" = $token }
      UseBasicParsing = $true
      WebSession = $sess
    }
    if ($json -ne $null) { $p.Body = $json }
    $r = Invoke-WebRequest @p
    Write-Host "  STATUS=$($r.StatusCode) EXPECTED=$expectedStatus"
    Write-Host "  BODY  =$($r.Content)"
    $pass = $r.StatusCode -eq $expectedStatus
    Write-Host "  $(if ($pass) {'PASS'} else {'FAIL'})"
    return $(if ($pass) {1} else {0})
  } catch {
    $status = 0
    $txt = $_.Exception.Message
    if ($_.Exception -and $_.Exception.Response) {
      try { $status = [int]$_.Exception.Response.StatusCode } catch {}
      try {
        $st = $_.Exception.Response.GetResponseStream()
        $st.Position = 0
        $txt = (New-Object System.IO.StreamReader($st)).ReadToEnd()
      } catch {}
    }
    Write-Host "  STATUS=$status EXPECTED=$expectedStatus"
    Write-Host "  BODY  =$txt"
    $pass = $status -eq $expectedStatus
    Write-Host "  $(if ($pass) {'PASS'} else {'FAIL'})"
    return $(if ($pass) {1} else {0})
  }
}

Write-Host ""
Write-Host "=== [3/3] Run test cases ==="
$score = 0
$total = 0
# A. 空 paths [] -> 200 {}
$total++; $score += RunCase -name "A. 空 paths=[] → 200 {}" -bodyObj @{ paths=@() } -expectedStatus 200
# B. 1 条正常 PDF 路径 (未入库也应 200, 空 {})
$total++; $score += RunCase -name "B. 1 PDF path → 200" -bodyObj @{ paths=@("/图纸/CQG50-0.88.pdf") } -expectedStatus 200
# C. >1000 → 400
$many = 1..1001 | ForEach-Object { "/x/$_.pdf" }
$total++; $score += RunCase -name "C. >1000 paths → 400" -bodyObj @{ paths=$many } -expectedStatus 400
# D. 不传 body (EOF) → 200 {}  (本次修复的关键！)
$total++; $score += RunCase -name "D. 不传 Body (EOF) → 200 {}" -bodyObj $null -expectedStatus 200
# E. 非法 JSON → 400
$total++; $score += RunCase -name "E. 非法 JSON → 400" -bodyObj "not json" -expectedStatus 400
# F. 额外：paths 包含目录路径 / 非 PDF，应该被后端 Check 过滤跳过 → 200 {}
$total++; $score += RunCase -name "F. paths 包含目录路径 → 200 {} (被 Check/PDF 过滤)" -bodyObj @{ paths=@("/图纸/") } -expectedStatus 200
# G. 额外：paths=[空字符串, 正常 PDF] 混合 → 仍然 200
$total++; $score += RunCase -name "G. paths 混有空串 → 200" -bodyObj @{ paths=@("", "/图纸/a.pdf") } -expectedStatus 200

Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "====================================="
Write-Host "TOTAL PASS: $score / $total"
Write-Host "====================================="
if ($score -lt $total) { exit 2 }
exit 0
