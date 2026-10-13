# ============================================================
# GOSHOP E2E - CompensationPendingScheduler job
# Simule compensation_pending -> force Run -> assert failed + balance
# ============================================================

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

$BaseUrl       = if ($env:GOSHOP_BASE_URL) { $env:GOSHOP_BASE_URL } else { "http://localhost:8080" }
$AdminEmail    = if ($env:ADMIN_EMAIL) { $env:ADMIN_EMAIL } else { "superadmin.yacine@goshop.com" }
$AdminPassword = if ($env:ADMIN_PASSWORD) { $env:ADMIN_PASSWORD } else { "CHANGE_ME" }
$DbService     = if ($env:DB_SERVICE) { $env:DB_SERVICE } else { "db" }
$DbUser        = if ($env:DB_USER) { $env:DB_USER } else { "postgres" }
$DbName        = if ($env:DB_NAME) { $env:DB_NAME } else { "goshop_db" }

$SeedBalance = [int64]100000
$WithdrawAmt = [int64]50000
$Timestamp   = Get-Date -Format "yyyyMMddHHmmss"
$ShopSlug    = "wd-job-$Timestamp"
$MerchantEmail = "merchant.wdjob.$Timestamp@goshop.com"
$MerchantPassword = "TestPass!" + (Get-Random -Minimum 1000 -Maximum 9999)

$script:Passed = 0
$script:Failed = 0
$script:State = @{
    AdminHeaders = $null
    MerchantHeaders = $null
    ShopId = $null
    WithdrawalId = $null
}

function Write-Step { param($N,$Msg) Write-Host ("[{0}] {1}" -f $N,$Msg) -ForegroundColor Cyan }
function Write-Ok   { param($Msg) Write-Host ("  [OK] {0}" -f $Msg) -ForegroundColor Green; $script:Passed++ }
function Write-Fail { param($Msg) Write-Host ("  [FAIL] {0}" -f $Msg) -ForegroundColor Red; $script:Failed++; throw $Msg }

function Invoke-Json {
    param([string]$Method,[string]$Uri,[hashtable]$Headers=@{},[object]$Body=$null)
    $params = @{ Method=$Method; Uri=$Uri; Headers=$Headers; UseBasicParsing=$true; TimeoutSec=60 }
    if ($null -ne $Body) {
        $params.ContentType = "application/json; charset=utf-8"
        $params.Body = if ($Body -is [string]) { $Body } else { ($Body | ConvertTo-Json -Depth 12 -Compress) }
    }
    try {
        $resp = Invoke-WebRequest @params
        $data = $null
        if ($resp.Content) { try { $data = $resp.Content | ConvertFrom-Json } catch {} }
        return @{ Ok=$true; Status=[int]$resp.StatusCode; Data=$data; Raw=$resp.Content }
    } catch {
        $status = 0; $raw = ""
        if ($_.Exception.Response) { try { $status = [int]$_.Exception.Response.StatusCode } catch {} }
        if ($_.ErrorDetails -and $_.ErrorDetails.Message) { $raw = $_.ErrorDetails.Message }
        $data = $null
        if ($raw) { try { $data = $raw | ConvertFrom-Json } catch {} }
        return @{ Ok=$false; Status=$status; Data=$data; Raw=$raw; Error=$_.Exception.Message }
    }
}

function Get-Prop {
    param($Obj, [string[]]$Paths)
    foreach ($p in $Paths) {
        $cur = $Obj; $ok = $true
        foreach ($seg in ($p -split '\.')) {
            if ($null -eq $cur) { $ok = $false; break }
            $prop = $cur.PSObject.Properties[$seg]
            if (-not $prop) { $ok = $false; break }
            $cur = $prop.Value
        }
        if ($ok -and $null -ne $cur -and "$cur" -ne "") { return $cur }
    }
    return $null
}

function Invoke-Sql {
    param([string]$Sql)
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $out = $Sql | docker compose exec -T $DbService psql -U $DbUser -d $DbName -v ON_ERROR_STOP=1 -t -A 2>&1
        $code = $LASTEXITCODE
    } finally { $ErrorActionPreference = $prev }
    $text = ($out | Out-String).Trim()
    if ($code -ne 0) { throw ("SQL failed (exit {0}): {1}" -f $code, $text) }
    return $text
}

function Get-WalletRow {
    param([string]$ShopId)
    $line = Invoke-Sql @"
SELECT balance_cents || '|' || COALESCE(held_cents,0) || '|' || COALESCE(debt_cents,0)
FROM merchant_wallets WHERE shop_id = '$ShopId'::uuid;
"@
    if (-not $line) { return @{ Bal=0; Held=0; Debt=0 } }
    $p = $line.Split("|")
    return @{ Bal=[int64]$p[0]; Held=[int64]$p[1]; Debt=[int64]$p[2] }
}

Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - CompensationPending JOB" -ForegroundColor Magenta
Write-Host (" BaseUrl={0} Seed={1} Withdraw={2}" -f $BaseUrl, $SeedBalance, $WithdrawAmt) -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # 01 Health
    Write-Step "01/08" "Health"
    $h = Invoke-Json -Method Get -Uri "$BaseUrl/health/live"
    if (-not $h.Ok) {
        $h = Invoke-Json -Method Get -Uri "$BaseUrl/health"
        if (-not $h.Ok) { Write-Fail "API health failed" }
    }
    Write-Ok "API live"

    # 02 Admin login
    Write-Step "02/08" "Admin login"
    $login = Invoke-Json -Method Post -Uri "$BaseUrl/login" -Body @{ email=$AdminEmail; password=$AdminPassword }
    if (-not $login.Ok) { Write-Fail ("Admin login failed: {0}" -f $login.Raw) }
    $adminToken = Get-Prop $login.Data @("access_token","token","data.access_token","data.token")
    if (-not $adminToken) { Write-Fail "Admin token missing" }
    $script:State.AdminHeaders = @{ Authorization="Bearer $adminToken"; "Content-Type"="application/json" }
    Write-Ok "Admin OK"

    # 03 Merchant + shop + KYC
    Write-Step "03/08" "Merchant + shop + KYC"
    $reg = Invoke-Json -Method Post -Uri "$BaseUrl/register" -Body @{
        email=$MerchantEmail; password=$MerchantPassword
        first_name="Wd"; last_name="Job"; role="merchant"
    }
    if (-not $reg.Ok -and $reg.Status -ne 201 -and $reg.Status -ne 200 -and $reg.Status -ne 409) {
        Write-Fail ("Register failed: {0}" -f $reg.Raw)
    }

    $mLogin = Invoke-Json -Method Post -Uri "$BaseUrl/login" -Body @{ email=$MerchantEmail; password=$MerchantPassword }
    if (-not $mLogin.Ok) { Write-Fail ("Merchant login: {0}" -f $mLogin.Raw) }
    $mToken = Get-Prop $mLogin.Data @("access_token","token","data.access_token","data.token")
    if (-not $mToken) { Write-Fail "Merchant token missing" }
    $script:State.MerchantHeaders = @{
        Authorization="Bearer $mToken"
        "X-Shop-Slug"=$ShopSlug
        "Content-Type"="application/json"
    }

    $shop = Invoke-Json -Method Post -Uri "$BaseUrl/api/shops" -Headers $script:State.MerchantHeaders -Body @{
        name=("WD Job Shop {0}" -f $Timestamp); slug=$ShopSlug
    }
    if (-not $shop.Ok) {
        $shop = Invoke-Json -Method Post -Uri "$BaseUrl/api/merchant/shops" -Headers $script:State.MerchantHeaders -Body @{
            name=("WD Job Shop {0}" -f $Timestamp); slug=$ShopSlug
        }
    }
    if (-not $shop.Ok) { Write-Fail ("Create shop: {0}" -f $shop.Raw) }
    $shopId = Get-Prop $shop.Data @("id","shop_id","data.id","data.shop_id")
    if (-not $shopId) { Write-Fail "shop id missing" }
    $script:State.ShopId = "$shopId"
    Write-Ok ("Shop {0}" -f $shopId)

    try {
        Invoke-Sql @"
DO `$`$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='public' AND table_name='shops' AND column_name='kyc_status'
  ) THEN
    UPDATE shops SET kyc_status='verified', updated_at=NOW() WHERE id='$shopId'::uuid;
  END IF;
END
`$`$;
"@ | Out-Null
        Write-Ok "KYC forced verified"
    } catch { Write-Host ("  [WARN] KYC SQL: {0}" -f $_.Exception.Message) -ForegroundColor Yellow }

    # 04 Seed wallet DEBITED (funds at risk)
    Write-Step "04/08" "Seed wallet debited (sim cash-out done)"
    $debited = $SeedBalance - $WithdrawAmt
    Invoke-Sql @"
INSERT INTO merchant_wallets (shop_id, balance_cents, held_cents, debt_cents, is_frozen, created_at, updated_at)
VALUES ('$shopId'::uuid, $debited, 0, 0, false, NOW(), NOW())
ON CONFLICT (shop_id) DO UPDATE SET
  balance_cents=EXCLUDED.balance_cents, held_cents=0, debt_cents=0, is_frozen=false, updated_at=NOW();
"@ | Out-Null
    $w0 = Get-WalletRow -ShopId $shopId
    if ($w0.Bal -ne $debited) { Write-Fail ("Seed bal={0} expected {1}" -f $w0.Bal, $debited) }
    Write-Ok ("Wallet BEFORE job: bal={0}" -f $w0.Bal)

    # 05 Insert compensation_pending (last_compensation_at old enough > minAge 60s)
    Write-Step "05/08" "SQL insert compensation_pending"
    $wdId = [guid]::NewGuid().ToString()
    $script:State.WithdrawalId = $wdId
    Invoke-Sql @"
INSERT INTO withdrawals (
  id, shop_id, provider, amount, currency, fees, net_amount,
  status, payment_method, destination_number, destination_name,
  description, error_message, failure_reason,
  compensation_attempts, last_compensation_at, created_at, updated_at
) VALUES (
  '$wdId'::uuid, '$shopId'::uuid, 'yengapay', $WithdrawAmt, 'XOF', 0, $WithdrawAmt,
  'compensation_pending', 'ORANGE_MONEY', '+22699999999', 'E2E Job',
  'e2e-job-$Timestamp',
  'cash-out failed | reverse failed: simulated',
  'simulated reverse failure for job e2e',
  1, NOW() - INTERVAL '5 minutes', NOW(), NOW()
);
"@ | Out-Null
    $st = Invoke-Sql "SELECT status FROM withdrawals WHERE id='$wdId'::uuid;"
    if ($st -ne "compensation_pending") { Write-Fail ("status={0}" -f $st) }
    Write-Ok ("Withdrawal {0} status=compensation_pending attempts=1" -f $wdId)

    # 06 Force job Run via admin
    Write-Step "06/08" "POST /api/admin/scheduler/trigger-compensation-pending"
    $trig = Invoke-Json -Method Post -Uri "$BaseUrl/api/admin/scheduler/trigger-compensation-pending" `
        -Headers $script:State.AdminHeaders
    if (-not $trig.Ok -or ($trig.Status -ne 200 -and $trig.Status -ne 202)) {
        Write-Fail ("Trigger failed HTTP={0} body={1}" -f $trig.Status, $trig.Raw)
    }
    Write-Ok ("Trigger HTTP {0}" -f $trig.Status)

    # 07 Assert status + balance
    Write-Step "07/08" "Assert job outcome"
    Start-Sleep -Milliseconds 500
    $finalStatus = Invoke-Sql "SELECT status FROM withdrawals WHERE id='$wdId'::uuid;"
    $attempts = Invoke-Sql "SELECT compensation_attempts::text FROM withdrawals WHERE id='$wdId'::uuid;"
    $w1 = Get-WalletRow -ShopId $shopId

    Write-Host ("  DB status={0} attempts={1} bal={2}" -f $finalStatus, $attempts, $w1.Bal) -ForegroundColor DarkGray

    if ($finalStatus -ne "failed") {
        Write-Fail ("Expected status failed, got '{0}'" -f $finalStatus)
    }
    Write-Ok "Status failed (compensation resolved)"

    if ($w1.Bal -ne $SeedBalance) {
        Write-Fail ("Balance not restored: got={0} expected={1}" -f $w1.Bal, $SeedBalance)
    }
    Write-Ok ("Balance restored to {0}" -f $w1.Bal)

    $rev = Invoke-Sql @"
SELECT COUNT(*)::text FROM wallet_transactions
WHERE shop_id='$shopId'::uuid
  AND (
    COALESCE(reference_type,'') ILIKE '%withdrawal_reversal%'
    OR COALESCE(reference_id,'') = '$wdId'
  );
"@
    if ([int]$rev -lt 1) {
        Write-Fail "No withdrawal_reversal ledger found"
    }
    Write-Ok ("Reversal ledger rows={0}" -f $rev)

    # 08 Summary
    Write-Step "08/08" "Summary"
    Write-Host ""
    Write-Host ("  Shop       : {0}" -f $shopId) -ForegroundColor White
    Write-Host ("  Withdrawal : {0}" -f $wdId) -ForegroundColor White
    Write-Host ("  Status end : {0}" -f $finalStatus) -ForegroundColor White
    Write-Host ("  Wallet end : bal={0}" -f $w1.Bal) -ForegroundColor White
    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    Write-Host (" RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed) -ForegroundColor Green
    Write-Host "================================================================" -ForegroundColor Magenta
    Write-Host "  COMPENSATION PENDING JOB E2E VERT" -ForegroundColor Green
}
catch {
    Write-Host ""
    Write-Host ("[CRITICAL FAILURE] {0}" -f $_.Exception.Message) -ForegroundColor Red
    exit 1
}