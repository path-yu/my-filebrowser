param(
  [string]$Mode = "Run",
  [string]$OutputExe = "",
  [switch]$NoDrawingSearch,

  [string]$DbFile = "",
  [string]$JRootPath = "",
  [string]$KBindAddr = "",
  [string]$QListenPort = "",

  [Parameter(Mandatory = $false, ValueFromRemainingArguments = $true)]
  [Alias("ExtraArgs", "Flags", "FilebrowserArgs")]
  [string[]]$ZArgs = @()
)

Write-Host "--- Parsed inputs ---" -ForegroundColor Cyan
Write-Host "Mode=[$Mode]"
Write-Host "NoDrawingSearch=[$NoDrawingSearch]"
Write-Host "DbFile=[$DbFile]  JRootPath=[$JRootPath]  KBindAddr=[$KBindAddr]  QListenPort=[$QListenPort]"
Write-Host "ZArgs count=$($ZArgs.Count): [$($ZArgs -join ' | ')]"

# ---------- Diagnostics: detect common wrong param patterns ----------
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
    $msg += "  - Found old-style aliases: $($foundWrongAliases -join ', '). Do NOT use @() array literals with powershell -File.`n"
  }
  if ($dashPrefixTokens.Count -gt 0) {
    $msg += "  - Found dash-prefixed tokens in catch-all ZArgs: $($dashPrefixTokens -join ', '). Short flags -d/-r/-a/-p are NOT reliable under powershell -File (some get lost, some throw NamedParameterNotFound).`n"
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

Write-Host ""
Write-Host "--- Final RunArgArray (count=$($RunArgArray.Count)) ---" -ForegroundColor Green
for ($i=0; $i -lt $RunArgArray.Count; $i++) { Write-Host "  [$i]  $($RunArgArray[$i])" }
