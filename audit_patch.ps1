# ===== probe: PATCH with nonexistent src (os.SameFile nil panic?) =====
$base = "http://127.0.0.1:8123"
$body = @{ username = "admin"; password = "Admin12345678" } | ConvertTo-Json
$tok = (Invoke-WebRequest -Uri "$base/api/login" -Method POST -Body $body -ContentType "application/json" -UseBasicParsing).Content

# src does not exist -> Fs.Stat returns nil,nil -> os.SameFile(nil,nil)
try {
  $r = Invoke-WebRequest -Uri "$base/api/resources/definitely_not_exists_file?destination=%2Fdev-files%2Frenamed.txt&action=rename" -Method PATCH -Headers @{ "X-Auth" = $tok } -UseBasicParsing -ErrorAction Stop
  "PATCH nonexistent src -> $($r.StatusCode)"
} catch {
  $resp = $_.Exception.Response
  if ($resp -eq $null) { "PATCH nonexistent src -> CONNECTION ERROR (likely panic: $($_.Exception.Message))" }
  else { "PATCH nonexistent src -> HTTP $([int]$resp.StatusCode)" }
}

# share expiry live check: create 2-second share then wait 3s
$body = @{ expires = "2"; unit = "seconds" } | ConvertTo-Json
$j = (Invoke-WebRequest -Uri "$base/api/share/dev-files/test.csv" -Method POST -Body $body -ContentType "application/json" -Headers @{ "X-Auth" = $tok } -UseBasicParsing).Content | ConvertFrom-Json
Start-Sleep -Seconds 3
try {
  $r = Invoke-WebRequest -Uri "$base/api/public/share/$($j.hash)/" -UseBasicParsing -ErrorAction Stop
  "expired share -> $($r.StatusCode) LEAK"
} catch {
  $code = [int]$_.Exception.Response.StatusCode
  "expired share -> $code (404=expired OK)"
}
