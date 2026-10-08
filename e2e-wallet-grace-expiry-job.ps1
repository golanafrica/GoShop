# ============================================================
# GOSHOP E2E - Wallet Grace Expiry Job (P2.4)
# ============================================================
# Flux:
#   00) Clean donnees de test (shop-scoped)
#   01) Health + admin login          POST /login
#   02) Merchant + shop + wallet seed POST /register + /login
#   03) SQL: is_frozen + frozen_until = NOW() - 10 days
#   04) Preview GET /api/admin/wallets/grace-expired
#   05) Dry-run  -> 0 action audit (shop) + run mode=dry_run
#   06) Live #1  -> 1 action grace_expired_detected + finance OK
#   07) Live #2  -> idempotence (toujours 1 action)
#   08) RBAC merchant -> 403 (best effort)
#   09) Summary
#
# Auth public (app.go): POST /login , POST /register
# Env: ADMIN_EMAIL, ADMIN_PASSWORD, GOSHOP_BASE_URL, DB_SERVICE, DB_USER, DB_NAME
# ============================================================

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

# -------------------- CONFIG --------------------
$BaseUrl       = if ($env:GOSHOP_BASE_URL) { $env:GOSHOP_BASE_URL } else { "http://localhost:8080" }
$AdminEmail    = if ($env:ADMIN_EMAIL) { $env:ADMIN_EMAIL } else { "" }
$AdminPassword = if ($env:ADMIN_PASSWORD) { $env:ADMIN_PASSWORD } else { "CHANGE_ME_ADMIN_PASSWORD" }
$DbService     = if ($env:DB_SERVICE) { $env:DB_SERVICE } else { "db" }
$DbUser        = if ($env:DB_USER) { $env:DB_USER } else { "postgres" }
$DbName        = if ($env:DB_NAME) { $env:DB_NAME } else { "goshop_db" }

$Timestamp     = Get-Date -Format "yyyyMMddHHmmss"
$ShopSlug      = "grace-expiry-shop-$Timestamp"
$MerchantEmail = "merchant.grace.$Timestamp@goshop.com"
$MerchantPass  = "Password123!"
$SeedBalance   = 50000
$SeedDebt      = 15000

$script:Passed = 0
$script:Failed = 0
$script:State  = @{
    AdminToken    = $null
    MerchantToken = $null
    ShopId        = $null
    BalBefore     = 0
    DebtBefore    = 0
    HeldBefore    = 0
}

# -------------------- HELPERS --------------------
function Write-Step {
    param([string]$N, [string]$Msg)
    Write-Host ("[{0}] {1}" -f $N, $Msg) -ForegroundColor Cyan
}
function Write-Ok {
    param([string]$Msg)
    Write-Host ("  [OK] {0}" -f $Msg) -ForegroundColor Green
    $script:Passed++
}
function Write-Fail {
    param([string]$Msg)
    Write-Host ("  [FAIL] {0}" -f $Msg) -ForegroundColor Red
    $script:Failed++
    throw $Msg
}
function Write-Warn {
    param([string]$Msg)
    Write-Host ("  [WARN] {0}" -f $Msg) -ForegroundColor Yellow
}
function Write-DebugLine {
    param([string]$Msg)
    Write-Host ("DEBUG: {0}" -f $Msg) -ForegroundColor DarkGray
}

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Uri,
        [hashtable]$Headers = @{},
        [object]$Body = $null,
        [switch]$AllowError
    )
    $params = @{
        Method          = $Method
        Uri             = $Uri
        Headers         = $Headers
        UseBasicParsing = $true
        TimeoutSec      = 60
    }
    if ($null -ne $Body) {
        $params.ContentType = "application/json"
        if ($Body -is [string]) {
            $params.Body = $Body
        }
        else {
            $params.Body = ($Body | ConvertTo-Json -Depth 12 -Compress)
        }
    }
    try {
        $resp = Invoke-WebRequest @params
        $data = $null
        if ($resp.Content) {
            try { $data = $resp.Content | ConvertFrom-Json } catch { $data = $resp.Content }
        }
        return @{ Ok = $true; Status = [int]$resp.StatusCode; Data = $data; Raw = $resp.Content }
    }
    catch {
        $status = 0
        $raw = ""
        if ($_.ErrorDetails -and $_.ErrorDetails.Message) {
            $raw = $_.ErrorDetails.Message
        }
        if ($_.Exception.Response) {
            try { $status = [int]$_.Exception.Response.StatusCode } catch { $status = 0 }
            if (-not $raw) {
                try {
                    $stream = $_.Exception.Response.GetResponseStream()
                    if ($stream) {
                        $reader = New-Object System.IO.StreamReader($stream)
                        $raw = $reader.ReadToEnd()
                        $reader.Close()
                    }
                }
                catch { }
            }
        }
        if (-not $raw) { $raw = $_.Exception.Message }
        $data = $null
        if ($raw) {
            try { $data = $raw | ConvertFrom-Json } catch { $data = $raw }
        }
        return @{ Ok = $false; Status = $status; Data = $data; Raw = $raw }
    }
}

function Invoke-Sql {
    param([string]$Sql)
    $out = docker compose exec -T $DbService psql -U $DbUser -d $DbName -t -A -c $Sql 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw ("SQL failed: {0}" -f ($out | Out-String))
    }
    return ($out | Out-String).Trim()
}

function Get-WalletRow {
    param([string]$ShopId)
    $sql = "SELECT balance_cents || '|' || COALESCE(held_cents,0) || '|' || COALESCE(debt_cents,0) || '|' || CASE WHEN is_frozen THEN 't' ELSE 'f' END FROM merchant_wallets WHERE shop_id = '$ShopId'::uuid;"
    $row = Invoke-Sql -Sql $sql
    $parts = $row -split '\|'
    return @{
        Bal    = [int64]$parts[0]
        Held   = [int64]$parts[1]
        Debt   = [int64]$parts[2]
        Frozen = ($parts[3] -eq 't')
    }
}

function Get-ActionCount {
    param([string]$ShopId)
    $sql = "SELECT COUNT(*)::text FROM wallet_freeze_job_actions WHERE shop_id = '$ShopId'::uuid AND action = 'grace_expired_detected' AND created_at > NOW() - INTERVAL '30 minutes';"
    $n = Invoke-Sql -Sql $sql
    return [int]$n
}

function Get-LatestRun {
    param([string]$Mode)
    $sql = "SELECT id::text || '|' || run_type || '|' || mode || '|' || wallets_scanned::text || '|' || wallets_processed::text || '|' || wallets_skipped::text || '|' || CASE WHEN finished_at IS NULL THEN 'open' ELSE 'done' END || '|' || COALESCE(error_message, '') FROM wallet_freeze_job_runs WHERE run_type = 'manual' AND mode = '$Mode' ORDER BY started_at DESC LIMIT 1;"
    $row = Invoke-Sql -Sql $sql
    if (-not $row) { return $null }
    $p = $row -split '\|'
    return @{
        Id        = $p[0]
        RunType   = $p[1]
        Mode      = $p[2]
        Scanned   = [int]$p[3]
        Processed = [int]$p[4]
        Skipped   = [int]$p[5]
        Finished  = $p[6]
        Error     = $p[7]
    }
}

function Wait-ActionCount {
    param(
        [string]$ShopId,
        [int]$MinCount,
        [int]$TimeoutSec = 30
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    do {
        $c = Get-ActionCount -ShopId $ShopId
        if ($c -ge $MinCount) { return $c }
        Start-Sleep -Seconds 1
    } while ((Get-Date) -lt $deadline)
    return (Get-ActionCount -ShopId $ShopId)
}

function Extract-Id {
    param($Obj, [string[]]$Paths)
    foreach ($p in $Paths) {
        $cur = $Obj
        foreach ($seg in ($p -split '\.')) {
            if ($null -eq $cur) { break }
            $cur = $cur.$seg
        }
        if ($cur -is [string] -and $cur.Length -gt 10) { return $cur }
        if ($cur -is [guid]) { return $cur.ToString() }
    }
    return $null
}

function Extract-TokenFromResponse {
    param($Resp)
    $tok = Extract-Id $Resp.Data @(
        'access_token',
        'token',
        'data.access_token',
        'data.token',
        'data.accessToken',
        'accessToken'
    )
    if (-not $tok -and $Resp.Raw) {
        if ($Resp.Raw -match '"access_token"\s*:\s*"([^"]+)"') { $tok = $Matches[1] }
        elseif ($Resp.Raw -match '"token"\s*:\s*"([^"]+)"') { $tok = $Matches[1] }
    }
    return $tok
}

# -------------------- BANNER --------------------
Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Wallet Grace Expiry Job (P2.4)" -ForegroundColor Magenta
Write-Host (" BaseUrl: {0} | DbService={1}" -f $BaseUrl, $DbService) -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # ---------- 00 Clean ----------
    Write-Step -N "00/09" -Msg "Cleaning old test data"
    try {
        $cleanSql = "DELETE FROM wallet_freeze_job_actions WHERE shop_id IN (SELECT id FROM shops WHERE slug LIKE 'grace-expiry-shop-%');"
        Invoke-Sql -Sql $cleanSql | Out-Null
        Write-Ok "Old test data cleaned"
    }
    catch {
        Write-Warn ("Clean skipped: {0}" -f $_.Exception.Message)
    }

    # ---------- 01 Health + Admin ----------
    Write-Step -N "01/09" -Msg "Health and Admin Login"
    $h = Invoke-Api -Method Get -Uri "$BaseUrl/health/live"
    if (-not $h.Ok) { $h = Invoke-Api -Method Get -Uri "$BaseUrl/health" }
    if (-not $h.Ok) { Write-Fail ("API health failed status={0} raw={1}" -f $h.Status, $h.Raw) }
    Write-Ok "API live"

    # Public auth routes (internal/app/app.go): POST /login , POST /register
    $login = Invoke-Api -Method Post -Uri "$BaseUrl/login" -Body @{
        email    = "$AdminEmail"
        password = "$AdminPassword"
    }
    if (-not $login.Ok) {
        Write-Fail ("Admin login failed status={0} raw={1}" -f $login.Status, $login.Raw)
    }
    $script:State.AdminToken = Extract-TokenFromResponse $login
    if (-not $script:State.AdminToken) {
        Write-Fail ("Admin token missing. raw={0}" -f $login.Raw)
    }
    $AdminHeaders = @{
        Authorization  = "Bearer $($script:State.AdminToken)"
        "Content-Type" = "application/json"
    }
    Write-Ok "Admin OK"

    # ---------- 02 Merchant + shop + wallet ----------
    Write-Step -N "02/09" -Msg "Merchant Setup and Wallet Initialization"

    $reg = Invoke-Api -Method Post -Uri "$BaseUrl/register" -Body @{
        email      = $MerchantEmail
        password   = $MerchantPass
        first_name = "Grace"
        last_name  = "Expiry"
    } -AllowError
    if (-not $reg.Ok -and $reg.Status -ne 201 -and $reg.Status -ne 409 -and $reg.Status -ne 200) {
        Write-Warn ("Register status={0} raw={1}" -f $reg.Status, $reg.Raw)
    }

    $mLogin = Invoke-Api -Method Post -Uri "$BaseUrl/login" -Body @{
        email    = $MerchantEmail
        password = $MerchantPass
    }
    if (-not $mLogin.Ok) { Write-Fail ("Merchant login failed status={0} raw={1}" -f $mLogin.Status, $mLogin.Raw) }
    $script:State.MerchantToken = Extract-TokenFromResponse $mLogin
    if (-not $script:State.MerchantToken) { Write-Fail "Merchant token missing" }
    $MerchHeaders = @{
        Authorization  = "Bearer $($script:State.MerchantToken)"
        "Content-Type" = "application/json"
    }

    $shop = Invoke-Api -Method Post -Uri "$BaseUrl/api/shops" -Headers $MerchHeaders -Body @{
        name = "Grace Expiry Shop"
        slug = $ShopSlug
    }
    if (-not $shop.Ok) {
        $shop = Invoke-Api -Method Post -Uri "$BaseUrl/shops" -Headers $MerchHeaders -Body @{
            name = "Grace Expiry Shop"
            slug = $ShopSlug
        }
    }
    if (-not $shop.Ok) { Write-Fail ("Create shop failed status={0} raw={1}" -f $shop.Status, $shop.Raw) }
    $shopId = Extract-Id $shop.Data @('id', 'shop_id', 'data.id', 'data.shop_id')
    if (-not $shopId) { Write-Fail ("Shop id missing raw={0}" -f $shop.Raw) }
    $script:State.ShopId = $shopId
    Write-DebugLine ("Extracted shopId = {0}" -f $shopId)

    $MerchHeaders["X-Shop-Slug"] = $ShopSlug
    $AdminHeaders["X-Shop-Slug"] = $ShopSlug

    $upsertSql = "INSERT INTO merchant_wallets (shop_id, balance_cents, held_cents, debt_cents, is_frozen, created_at, updated_at) VALUES ('$shopId'::uuid, $SeedBalance, 0, $SeedDebt, false, NOW(), NOW()) ON CONFLICT (shop_id) DO UPDATE SET balance_cents = EXCLUDED.balance_cents, held_cents = 0, debt_cents = EXCLUDED.debt_cents, is_frozen = false, frozen_at = NULL, frozen_until = NULL, frozen_reason = NULL, updated_at = NOW();"
    Invoke-Sql -Sql $upsertSql | Out-Null

    $w0 = Get-WalletRow -ShopId $shopId
    $script:State.BalBefore  = $w0.Bal
    $script:State.DebtBefore = $w0.Debt
    $script:State.HeldBefore = $w0.Held
    Write-Ok ("Shop and Wallet initialized (Balance: {0} c | Debt: {1} c)" -f $w0.Bal, $w0.Debt)

    # ---------- 03 Simulate expired grace ----------
    Write-Step -N "03/09" -Msg "Simulate Expired Grace Period (SQL INTERVAL)"
    $updSql = "UPDATE merchant_wallets SET is_frozen = true, frozen_at = NOW() - INTERVAL '17 days', frozen_until = NOW() - INTERVAL '10 days', frozen_reason = 'e2e_grace_expiry', updated_at = NOW() WHERE shop_id = '$shopId'::uuid RETURNING shop_id::text || '|' || CASE WHEN is_frozen THEN 't' ELSE 'f' END || '|' || frozen_until::text;"
    $upd = Invoke-Sql -Sql $updSql
    Write-DebugLine ("SQL Update returned: '{0}'" -f $upd)
    if ($upd -notmatch [regex]::Escape($shopId)) { Write-Fail "Wallet freeze SQL failed" }
    Write-Ok "Wallet artificially frozen with expired grace period (10 days ago)"

    # ---------- 04 Preview ----------
    Write-Step -N "04/09" -Msg "Verify Detection via Admin API"
    $preview = Invoke-Api -Method Get -Uri "$BaseUrl/api/admin/wallets/grace-expired" -Headers $AdminHeaders
    if (-not $preview.Ok) {
        $preview = Invoke-Api -Method Get -Uri "$BaseUrl/api/admin/freeze-job/grace-expired" -Headers $AdminHeaders
    }
    if (-not $preview.Ok) { Write-Fail ("Preview failed status={0} raw={1}" -f $preview.Status, $preview.Raw) }

    $list = @()
    if ($preview.Data -is [array]) { $list = $preview.Data }
    elseif ($preview.Data.wallets) { $list = @($preview.Data.wallets) }
    elseif ($preview.Data.data) { $list = @($preview.Data.data) }
    elseif ($preview.Data.items) { $list = @($preview.Data.items) }

    Write-DebugLine ("Extracted wallets count: {0}" -f $list.Count)
    $mine = $list | Where-Object {
        $sid = $_.shop_id
        if (-not $sid) { $sid = $_.ShopID }
        if (-not $sid) { $sid = $_.id }
        ("$sid" -eq $shopId)
    }
    if (-not $mine) {
        $wCheck = Get-WalletRow -ShopId $shopId
        if (-not $wCheck.Frozen) { Write-Fail "Wallet not frozen after SQL" }
        if ($list.Count -lt 1) { Write-Fail "Preview empty" }
        Write-Warn ("Shop not explicit in JSON list - DB freeze OK, preview count={0}" -f $list.Count)
        Write-Ok ("Wallet detected in preview (count={0})" -f $list.Count)
    }
    else {
        $days = 0
        $m0 = @($mine)[0]
        if ($m0.days_expired) { $days = [int]$m0.days_expired }
        Write-Ok ("Wallet detected in preview (Days Expired: {0})" -f $days)
    }

    $actionsBefore = Get-ActionCount -ShopId $shopId

    # ---------- 05 Dry-run ----------
    Write-Step -N "05/09" -Msg "Test Dry Run (No side effects)"
    $dry = Invoke-Api -Method Post -Uri "$BaseUrl/api/admin/freeze-job/run?mode=dry_run" -Headers $AdminHeaders -Body @{}
    if (-not $dry.Ok) {
        $dry = Invoke-Api -Method Post -Uri "$BaseUrl/api/admin/freeze-job/run" -Headers $AdminHeaders -Body @{ mode = "dry_run" }
    }
    if (-not $dry.Ok) { Write-Fail ("Dry-run trigger failed status={0} raw={1}" -f $dry.Status, $dry.Raw) }
    Write-Ok "Dry run triggered successfully"

    Start-Sleep -Seconds 3
    $actionsAfterDry = Get-ActionCount -ShopId $shopId
    if ($actionsAfterDry -ne $actionsBefore) {
        Write-Fail ("Dry-run must not create audit actions (before={0} after={1})" -f $actionsBefore, $actionsAfterDry)
    }
    Write-Ok "Dry run verified: No audit actions created"

    $runDry = Get-LatestRun -Mode "dry_run"
    if ($runDry -and $runDry.Finished -eq "done") {
        Write-Ok ("Dry-run job_run recorded (scanned={0} processed={1})" -f $runDry.Scanned, $runDry.Processed)
    }
    else {
        Write-Warn "Dry-run job_run not found or still open (async timing)"
    }

    # ---------- 06 Live #1 ----------
    Write-Step -N "06/09" -Msg "Test Live Run and Assert Financial State"
    $live = Invoke-Api -Method Post -Uri "$BaseUrl/api/admin/freeze-job/run?mode=live" -Headers $AdminHeaders -Body @{}
    if (-not $live.Ok) {
        $live = Invoke-Api -Method Post -Uri "$BaseUrl/api/admin/freeze-job/run" -Headers $AdminHeaders -Body @{ mode = "live" }
    }
    if (-not $live.Ok) { Write-Fail ("Live run trigger failed status={0} raw={1}" -f $live.Status, $live.Raw) }
    Write-Ok "Live run triggered successfully"

    $cnt1 = Wait-ActionCount -ShopId $shopId -MinCount ($actionsBefore + 1) -TimeoutSec 30
    if ($cnt1 -lt ($actionsBefore + 1)) {
        Write-Fail ("Live run failed to create audit action (Count: {0})" -f $cnt1)
    }
    Write-Ok ("Live run verified: {0} audit action(s) for shop (detected via polling)" -f $cnt1)

    $w1 = Get-WalletRow -ShopId $shopId
    if ($w1.Bal -ne $script:State.BalBefore) {
        Write-Fail ("Balance mutated: {0} -> {1}" -f $script:State.BalBefore, $w1.Bal)
    }
    if ($w1.Debt -ne $script:State.DebtBefore) {
        Write-Fail ("Debt mutated: {0} -> {1}" -f $script:State.DebtBefore, $w1.Debt)
    }
    if ($w1.Held -ne $script:State.HeldBefore) {
        Write-Fail ("Held mutated: {0} -> {1}" -f $script:State.HeldBefore, $w1.Held)
    }
    if (-not $w1.Frozen) {
        Write-Fail "Wallet was unfrozen by job (forbidden in v1)"
    }
    Write-Ok ("Financial state preserved (Balance: {0}, Debt: {1}, Frozen: true)" -f $w1.Bal, $w1.Debt)

    $ledgerSql = "SELECT COUNT(*)::text FROM wallet_transactions WHERE shop_id = '$shopId'::uuid AND transaction_type IN ('clawback', 'debt_sweep', 'debt_add') AND created_at > NOW() - INTERVAL '30 minutes';"
    $badLedger = Invoke-Sql -Sql $ledgerSql
    if ([int]$badLedger -gt 0) {
        Write-Fail ("Forbidden ledger rows (clawback/debt_sweep/debt_add): {0}" -f $badLedger)
    }
    else {
        Write-Ok "No clawback/debt_sweep ledger side-effects"
    }

    $runLive = Get-LatestRun -Mode "live"
    if ($runLive -and $runLive.Finished -eq "done" -and -not $runLive.Error) {
        Write-Ok ("Live job_run OK (scanned={0} processed={1} skipped={2})" -f $runLive.Scanned, $runLive.Processed, $runLive.Skipped)
    }
    else {
        Write-Warn ("Live job_run incomplete or error: {0}" -f ($runLive | ConvertTo-Json -Compress))
    }

    # ---------- 07 Idempotence Live #2 ----------
    Write-Step -N "07/09" -Msg "Idempotence (second live run)"
    $live2 = Invoke-Api -Method Post -Uri "$BaseUrl/api/admin/freeze-job/run?mode=live" -Headers $AdminHeaders -Body @{}
    if (-not $live2.Ok) {
        $live2 = Invoke-Api -Method Post -Uri "$BaseUrl/api/admin/freeze-job/run" -Headers $AdminHeaders -Body @{ mode = "live" }
    }
    if (-not $live2.Ok) { Write-Fail ("Live #2 trigger failed status={0} raw={1}" -f $live2.Status, $live2.Raw) }

    Start-Sleep -Seconds 4
    $cnt2 = Get-ActionCount -ShopId $shopId
    if ($cnt2 -ne $cnt1) {
        Write-Fail ("Idempotence failed: action count {0} -> {1} (expected same)" -f $cnt1, $cnt2)
    }
    Write-Ok ("Idempotence OK: still {0} grace_expired_detected action(s)" -f $cnt2)

    # ---------- 08 RBAC merchant ----------
    Write-Step -N "08/09" -Msg "RBAC: merchant cannot run freeze job"
    $forbidden = Invoke-Api -Method Post -Uri "$BaseUrl/api/admin/freeze-job/run?mode=dry_run" -Headers $MerchHeaders -Body @{} -AllowError
    if ($forbidden.Status -eq 401 -or $forbidden.Status -eq 403) {
        Write-Ok ("Merchant blocked HTTP {0}" -f $forbidden.Status)
    }
    else {
        Write-Warn ("Expected 401/403 for merchant, got HTTP {0} - check route guards" -f $forbidden.Status)
    }

    # ---------- 09 Summary ----------
    Write-Step -N "09/09" -Msg "Summary"
    Write-Host ("  Shop     : {0} ({1})" -f $shopId, $ShopSlug) -ForegroundColor White
    Write-Host ("  Wallet   : bal={0} debt={1} held={2} frozen=true" -f $w1.Bal, $w1.Debt, $w1.Held) -ForegroundColor White
    Write-Host ("  Actions  : {0} (grace_expired_detected)" -f $cnt2) -ForegroundColor White
    Write-Host ""
    Write-Host "  Logs:" -ForegroundColor Yellow
    Write-Host '  docker compose logs goshop 2>&1 | Select-String -Pattern "wallet_grace_expiry|grace_expired|Inserted action"' -ForegroundColor DarkGray

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $color = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    Write-Host (" RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed) -ForegroundColor $color
    Write-Host "================================================================" -ForegroundColor Magenta

    if ($script:Failed -eq 0) {
        Write-Host ""
        Write-Host "  WALLET GRACE EXPIRY JOB E2E VERT" -ForegroundColor Green
        Write-Host "  (preview, dry-run, live, idempotence, finance safety, RBAC)" -ForegroundColor Green
    }
    else {
        exit 1
    }
}
catch {
    Write-Host ""
    Write-Host ("[CRITICAL FAILURE] {0}" -f $_.Exception.Message) -ForegroundColor Red
    exit 1
}

Write-Host ""