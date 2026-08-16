# ===== Filebrowser API audit script (ASCII only for PS5.1) =====
$base = "http://127.0.0.1:8123"
$out = @()

function T($name, $ok, $detail) {
  $script:out += [pscustomobject]@{ Test = $name; Result = $ok; Detail = $detail }
}

# ---------- 1. unauthenticated access ----------
try {
  $r = Invoke-WebRequest -Uri "$base/api/resources/" -Method GET -UseBasicParsing -ErrorAction Stop
  T "unauth /resources -> 401?" ($r.StatusCode -eq 401) "got $($r.StatusCode)"
} catch {
  $code = $_.Exception.Response.StatusCode.value__
  T "unauth /resources -> 401?" ($code -eq 401) "got $code"
}

# ---------- 2. normal login ----------
$adminToken = $null
try {
  $body = @{ username = "admin"; password = "Admin12345678" } | ConvertTo-Json
  $r = Invoke-WebRequest -Uri "$base/api/login" -Method POST -Body $body -ContentType "application/json" -UseBasicParsing
  $adminToken = $r.Content
  T "login admin" ($r.StatusCode -eq 200 -and $adminToken.Length -gt 20) "token len $($adminToken.Length)"
} catch { T "login admin" $false $_.Exception.Message }

$bobToken = $null
try {
  $body = @{ username = "bob"; password = "BobBobBob12345" } | ConvertTo-Json
  $r = Invoke-WebRequest -Uri "$base/api/login" -Method POST -Body $body -ContentType "application/json" -UseBasicParsing
  $bobToken = $r.Content
  T "login bob" ($r.StatusCode -eq 200) "ok"
} catch { T "login bob" $false $_.Exception.Message }

# ---------- 3. wrong password ----------
try {
  $body = @{ username = "admin"; password = "wrongpassword1" } | ConvertTo-Json
  $r = Invoke-WebRequest -Uri "$base/api/login" -Method POST -Body $body -ContentType "application/json" -UseBasicParsing -ErrorAction Stop
  T "login wrong pwd rejected" $false "got $($r.StatusCode)"
} catch {
  $code = $_.Exception.Response.StatusCode.value__
  T "login wrong pwd rejected" ($code -eq 403 -or $code -eq 401) "got $code"
}

# ---------- 4. forged JWT ----------
try {
  $h = Invoke-WebRequest -Uri "$base/api/resources/" -Headers @{ "X-Auth" = "fake.token.here" } -UseBasicParsing -ErrorAction Stop
  T "fake jwt rejected" $false "got $($h.StatusCode)"
} catch {
  $code = $_.Exception.Response.StatusCode.value__
  T "fake jwt rejected" ($code -eq 401) "got $code"
}
try {
  $noneHdr = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('{"alg":"none","typ":"JWT"}')).TrimEnd('=').Replace('+','-').Replace('/','_')
  $parts = $adminToken.Split('.')
  $forged = "$noneHdr.$($parts[1])."
  $r = Invoke-WebRequest -Uri "$base/api/resources/" -Headers @{ "X-Auth" = $forged } -UseBasicParsing -ErrorAction Stop
  T "jwt alg=none rejected" $false "got $($r.StatusCode) ACCEPTED"
} catch {
  $code = $_.Exception.Response.StatusCode.value__
  T "jwt alg=none rejected" ($code -eq 401) "got $code"
}

# ---------- 5. non-admin accessing admin APIs ----------
try {
  $r = Invoke-WebRequest -Uri "$base/api/users" -Headers @{ "X-Auth" = $bobToken } -UseBasicParsing -ErrorAction Stop
  T "bob GET /users forbidden" $false "got $($r.StatusCode) ALLOWED"
} catch {
  $code = $_.Exception.Response.StatusCode.value__
  T "bob GET /users forbidden" ($code -eq 403) "got $code"
}
try {
  $body = @{ what = "commands"; which = @() } | ConvertTo-Json
  $r = Invoke-WebRequest -Uri "$base/api/users/1" -Method PUT -Body $body -ContentType "application/json" -Headers @{ "X-Auth" = $bobToken } -UseBasicParsing -ErrorAction Stop
  T "bob PUT admin user forbidden" $false "got $($r.StatusCode) ALLOWED"
} catch {
  $code = $_.Exception.Response.StatusCode.value__
  T "bob PUT admin user forbidden" ($code -eq 403) "got $code"
}
try {
  $r = Invoke-WebRequest -Uri "$base/api/settings" -Headers @{ "X-Auth" = $bobToken } -UseBasicParsing -ErrorAction Stop
  T "bob GET /settings forbidden" $false "got $($r.StatusCode) ALLOWED"
} catch {
  $code = $_.Exception.Response.StatusCode.value__
  T "bob GET /settings forbidden" ($code -eq 403) "got $code"
}

# ---------- 6. path traversal (authenticated) ----------
$traversals = @(
  "/api/resources/../../",
  "/api/resources/%2e%2e%2f%2e%2e%2f",
  "/api/resources/..%2f..%2f",
  "/api/raw/../../go.mod",
  "/api/raw/%2e%2e%2fgo.mod"
)
foreach ($p in $traversals) {
  $rawUrl = "$base$p"
  try {
    $r = Invoke-WebRequest -Uri $rawUrl -Headers @{ "X-Auth" = $adminToken } -UseBasicParsing -ErrorAction Stop
    $leak = ($r.Content -match "go.mod|for 16-bit app support")
    T "traversal blocked: $p" (-not $leak) "status $($r.StatusCode) leak=$leak len=$($r.Content.Length)"
  } catch {
    $code = $_.Exception.Response.StatusCode.value__
    T "traversal blocked: $p" ($code -eq 404 -or $code -eq 403 -or $code -eq 400) "got $code"
  }
}

# ---------- 7. share link ----------
$hash = $null
try {
  $body = @{ password = "SharePwd12345" } | ConvertTo-Json
  $r = Invoke-WebRequest -Uri "$base/api/share/dev-files/test.csv" -Method POST -Body $body -ContentType "application/json" -Headers @{ "X-Auth" = $adminToken } -UseBasicParsing
  $j = $r.Content | ConvertFrom-Json
  $hash = $j.hash
  T "create share" ($null -ne $hash) "hash=$hash"
} catch { T "create share" $false $_.Exception.Message }

if ($hash) {
  try {
    $r = Invoke-WebRequest -Uri "$base/api/public/share/$hash/" -UseBasicParsing -ErrorAction Stop
    T "share no-pwd rejected" $false "got $($r.StatusCode) ALLOWED"
  } catch {
    $code = $_.Exception.Response.StatusCode.value__
    T "share no-pwd rejected" ($code -eq 401) "got $code"
  }
  try {
    $r = Invoke-WebRequest -Uri "$base/api/public/share/$hash/" -Headers @{ "X-SHARE-PASSWORD" = "wrong" } -UseBasicParsing -ErrorAction Stop
    T "share wrong-pwd rejected" $false "got $($r.StatusCode) ALLOWED"
  } catch {
    $code = $_.Exception.Response.StatusCode.value__
    T "share wrong-pwd rejected" ($code -eq 401) "got $code"
  }
  try {
    $r = Invoke-WebRequest -Uri "$base/api/public/dl/$hash/..%2f..%2fgo.mod" -UseBasicParsing -ErrorAction Stop
    $leak = ($r.Content -match "module ")
    T "share traversal blocked" (-not $leak) "status $($r.StatusCode) leak=$leak len=$($r.Content.Length)"
  } catch {
    $code = $_.Exception.Response.StatusCode.value__
    T "share traversal blocked" ($code -ne 200) "got $code"
  }
  try {
    $r = Invoke-WebRequest -Uri "$base/api/share/$hash" -Method DELETE -Headers @{ "X-Auth" = $bobToken } -UseBasicParsing -ErrorAction Stop
    T "bob delete foreign share forbidden" $false "got $($r.StatusCode) ALLOWED"
  } catch {
    $code = $_.Exception.Response.StatusCode.value__
    T "bob delete foreign share forbidden" ($code -eq 403) "got $code"
  }
}

# ---------- 8. signup disabled ----------
try {
  $body = @{ username = "hacker"; password = "HackHack12345" } | ConvertTo-Json
  $r = Invoke-WebRequest -Uri "$base/api/signup" -Method POST -Body $body -ContentType "application/json" -UseBasicParsing -ErrorAction Stop
  T "signup disabled" $false "got $($r.StatusCode) ALLOWED"
} catch {
  $code = $_.Exception.Response.StatusCode.value__
  T "signup disabled" ($code -eq 405) "got $code"
}

# ---------- 9. response timing (20x) ----------
$times = @()
for ($i = 0; $i -lt 20; $i++) {
  $sw = [System.Diagnostics.Stopwatch]::StartNew()
  Invoke-WebRequest -Uri "$base/api/resources/dev-files" -Headers @{ "X-Auth" = $adminToken } -UseBasicParsing | Out-Null
  $sw.Stop()
  $times += $sw.ElapsedMilliseconds
}
$avg = [Math]::Round(($times | Measure-Object -Average).Average, 1)
T "GET /resources avg ms (20x)" $true "avg=${avg}ms min=$(($times | Measure-Object -Minimum).Minimum)ms max=$(($times | Measure-Object -Maximum).Maximum)ms"

$out | Format-Table -AutoSize -Wrap
$out | ConvertTo-Json | Out-File audit_result.json -Encoding utf8
"saved audit_result.json"
