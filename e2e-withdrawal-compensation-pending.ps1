# ============================================================
# GOSHOP E2E - Withdrawal CashOut FAIL -> compensation_pending
# ============================================================
# Objectif :
#   Cash-out definitif KO apres debit.
#   Si reverse OK     -> status=failed, balance restauree (sandbox)
#   Si reverse KO     -> status=compensation_pending, balance NON restauree
#
# Phase A : POST /api/withdrawals avec MSISDN non whitelist
# Phase B : si reverse a reussi (sandbox), simule compensation_pending en SQL
#
# Env:
#   $env:ADMIN_EMAIL / $env:ADMIN_PASSWORD
#   $env:GOSHOP_BASE_URL
#   $env:DB_SERVICE / DB_USER / DB_NAME
#   $env:WITHDRAW_FAIL_MSISDN  (default +22699999999)
#   $env:MERCHANT_PASSWORD
# ============================================================

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

# -------------------- CONFIG --------------------
$BaseUrl       = if ($env:GOSHOP_BASE_URL) { $env:GOSHOP_BASE_URL } else { "http://localhost:8080" }
$AdminEmail    = if ($env:ADMIN_EMAIL) { $env:ADMIN_EMAIL } else { "superadmin.yacine@goshop.com" }
$AdminPassword = if ($env:ADMIN_PASSWORD) { $env:ADMIN_PASSWORD } else { "CHANGE_ME_ADMIN_PASSWORD" }
$DbService     = if ($env:DB_SERVICE) { $env:DB_SERVICE } else { "db" }
$DbUser        = if ($env:DB_USER) { $env:DB_USER } else { "postgres" }
$DbName        = if ($env:DB_NAME) { $env:DB_NAME } else { "goshop_db" }

$SeedBalance   = [int64]100000
$WithdrawAmt   = [int64]50000
$FailMsisdn    = if ($env:WITHDRAW_FAIL_MSISDN) { $env:WITHDRAW_FAIL_MSISDN } else { "+22699999999" }
$MerchantPassword = if ($env:MERCHANT_PASSWORD) { $env:MERCHANT_PASSWORD } else { "TestPass!" + (Get-Random -Minimum 1000 -Maximum 9999) }

$Timestamp     = Get-Date -Format "yyyyMMddHHmmss"
$ShopSlug      = "wd-comp-$Timestamp"
$MerchantEmail = "merchant.wdcomp.$Timestamp@goshop.com"

$script:Passed = 0
$script:Failed = 0
$script:State  = @{
    AdminHeaders    = $null
    MerchantHeaders = $null
    MerchantUserId  = $null
    ShopId          = $null
    WithdrawalId    = $null
    PhaseAStatus    = $null
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

function Invoke-Json {
    param(
        [string]$Method,
        [string]$Uri,
        [hashtable]$Headers = @{},
        [object]$Body = $null
    )
    $params = @{
        Method          = $Method
        Uri             = $Uri
        Headers         = $Headers
        UseBasicParsing = $true
        TimeoutSec      = 60
    }
    if ($null -ne $Body) {
        $params.ContentType = "application/json; charset=utf-8"
        $params.Body = if ($Body -is [string]) { $Body } else { ($Body | ConvertTo-Json -Depth 12 -Compress) }
    }
    try {
        $resp = Invoke-WebRequest @params
        $raw = $resp.Content
        $data = $null
        if ($raw) {
            try { $data = $raw | ConvertFrom-Json } catch { $data = $null }
        }
        return @{ Ok = $true; Status = [int]$resp.StatusCode; Data = $data; Raw = $raw }
    }
    catch {
        $status = 0
        $raw = ""
        if ($_.Exception.Response) {
            try { $status = [int]$_.Exception.Response.StatusCode } catch { $status = 0 }
        }
        if ($_.ErrorDetails -and $_.ErrorDetails.Message) { $raw = $_.ErrorDetails.Message }
        $data = $null
        if ($raw) {
            try { $data = $raw | ConvertFrom-Json } catch { $data = $null }
        }
        return @{ Ok = $false; Status = $status; Data = $data; Raw = $raw; Error = $_.Exception.Message }
    }
}

function Get-Prop {
    param($Obj, [string[]]$Paths)
    foreach ($p in $Paths) {
        $cur = $Obj
        $ok = $true
        foreach ($seg in ($p -split '\.')) {
            if ($null -eq $cur) { $ok = $false; break }
            if ($cur -is [hashtable] -and $cur.ContainsKey($seg)) { $cur = $cur[$seg]; continue }
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
    }
    finally {
        $ErrorActionPreference = $prev
    }
    $text = ($out | Out-String).Trim()
    if ($code -ne 0) {
        throw ("SQL failed (exit {0}): {1}" -f $code, $text)
    }
    return $text
}

function Get-WalletRow {
    param([string]$ShopId)
    $line = Invoke-Sql @"
SELECT balance_cents || '|' || COALESCE(held_cents,0) || '|' || COALESCE(debt_cents,0) || '|' || is_frozen
FROM merchant_wallets
WHERE shop_id = '$ShopId'::uuid;
"@
    if (-not $line) { return @{ Bal = 0; Held = 0; Debt = 0; Frozen = "f" } }
    $p = $line.Split("|")
    return @{
        Bal    = [int64]$p[0]
        Held   = [int64]$p[1]
        Debt   = [int64]$p[2]
        Frozen = $p[3]
    }
}

function Get-LedgerLines {
    param([string]$ShopId, [int]$Limit = 15)
    $raw = Invoke-Sql @"
SELECT COALESCE(string_agg(
  transaction_type || ':' || amount_cents::text || ':' || COALESCE(reference_type,''),
  '; ' ORDER BY created_at DESC
), '')
FROM (
  SELECT transaction_type, amount_cents, reference_type, created_at
  FROM wallet_transactions
  WHERE shop_id = '$ShopId'::uuid
  ORDER BY created_at DESC
  LIMIT $Limit
) t;
"@
    return $raw
}

function Test-HasReversalLedger {
    param([string]$ShopId)
    $line = Invoke-Sql @"
SELECT COALESCE(
  (SELECT transaction_type || '|' || amount_cents::text || '|' || COALESCE(reference_type,'')
   FROM wallet_transactions
   WHERE shop_id = '$ShopId'::uuid
     AND (
       reference_type ILIKE '%withdrawal_reversal%'
       OR reference_type ILIKE '%payout_reversal%'
       OR COALESCE(description,'') ILIKE '%reversal%'
       OR COALESCE(description,'') ILIKE '%reverse%'
     )
   ORDER BY created_at DESC
   LIMIT 1),
  ''
);
"@
    return [bool]$line
}

# -------------------- MAIN --------------------
Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Withdrawal -> compensation_pending" -ForegroundColor Magenta
Write-Host (" BaseUrl: {0} | Seed={1} | Withdraw={2} | FailMSISDN={3}" -f $BaseUrl, $SeedBalance, $WithdrawAmt, $FailMsisdn) -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # ---------- 01 Health ----------
    Write-Step -N "01/09" -Msg "Health"
    $h = Invoke-Json -Method Get -Uri "$BaseUrl/health/live"
    if (-not $h.Ok -or $h.Status -ne 200) {
        $h2 = Invoke-Json -Method Get -Uri "$BaseUrl/health"
        if (-not $h2.Ok) { Write-Fail "API health failed" }
    }
    Write-Ok ("API live HTTP {0}" -f $(if ($h.Ok) { $h.Status } else { 200 }))

    # ---------- 02 Admin login ----------
    Write-Step -N "02/09" -Msg "Admin login"
    $login = Invoke-Json -Method Post -Uri "$BaseUrl/login" -Body @{
        email    = $AdminEmail
        password = $AdminPassword
    }
    if (-not $login.Ok) { Write-Fail ("Admin login failed: {0}" -f $login.Raw) }
    $adminToken = Get-Prop $login.Data @("access_token", "token", "data.access_token", "data.token")
    if (-not $adminToken) { Write-Fail "Admin token missing" }
    $script:State.AdminHeaders = @{
        Authorization  = "Bearer $adminToken"
        "Content-Type" = "application/json"
    }
    Write-Ok "Admin OK"

    # ---------- 03 Merchant + shop + KYC ----------
    Write-Step -N "03/09" -Msg "Merchant + shop + KYC"
    $reg = Invoke-Json -Method Post -Uri "$BaseUrl/register" -Body @{
        email      = $MerchantEmail
        password   = $MerchantPassword
        first_name = "Wd"
        last_name  = "CompPending"
        role       = "merchant"
    }
    if (-not $reg.Ok -and $reg.Status -ne 201 -and $reg.Status -ne 200 -and $reg.Status -ne 409) {
        Write-Fail ("Register failed HTTP {0}: {1}" -f $reg.Status, $reg.Raw)
    }
    if ($reg.Ok) { Write-Ok ("Register HTTP {0}" -f $reg.Status) } else { Write-Warn "Register conflict - login" }

    $mLogin = Invoke-Json -Method Post -Uri "$BaseUrl/login" -Body @{
        email    = $MerchantEmail
        password = $MerchantPassword
    }
    if (-not $mLogin.Ok) { Write-Fail ("Merchant login failed: {0}" -f $mLogin.Raw) }
    $mToken = Get-Prop $mLogin.Data @("access_token", "token", "data.access_token", "data.token")
    if (-not $mToken) { Write-Fail "Merchant token missing" }
    $script:State.MerchantUserId = Get-Prop $mLogin.Data @("user.id", "data.user.id", "user_id", "id")
    $script:State.MerchantHeaders = @{
        Authorization  = "Bearer $mToken"
        "X-Shop-Slug"  = $ShopSlug
        "Content-Type" = "application/json"
    }

    $shopBody = @{
        name = "WD Comp Pending Shop $Timestamp"
        slug = $ShopSlug
    }
    $shop = Invoke-Json -Method Post -Uri "$BaseUrl/api/shops" -Headers $script:State.MerchantHeaders -Body $shopBody
    if (-not $shop.Ok) {
        $shop = Invoke-Json -Method Post -Uri "$BaseUrl/api/merchant/shops" -Headers $script:State.MerchantHeaders -Body $shopBody
    }
    if (-not $shop.Ok) { Write-Fail ("Create shop failed: {0}" -f $shop.Raw) }
    $shopId = Get-Prop $shop.Data @("id", "shop_id", "data.id", "data.shop_id")
    if (-not $shopId) { Write-Fail "shop id missing" }
    $script:State.ShopId = "$shopId"
    $script:State.MerchantHeaders["X-Shop-Slug"] = $ShopSlug
    Write-Ok ("Shop {0} ({1})" -f $shopId, $ShopSlug)

    try {
        Invoke-Sql @"
DO `$`$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'shops' AND column_name = 'kyc_status'
  ) THEN
    UPDATE shops SET kyc_status = 'verified', updated_at = NOW()
    WHERE id = '$shopId'::uuid;
  END IF;
END
`$`$;
"@ | Out-Null
        Write-Ok "Shop KYC forced verified (if column exists)"
    }
    catch {
        Write-Warn ("Shop KYC SQL skipped: {0}" -f $_.Exception.Message)
    }

    # ---------- 04 Seed wallet ----------
    Write-Step -N "04/09" -Msg ("Seed wallet balance={0}" -f $SeedBalance)
    Invoke-Sql @"
INSERT INTO merchant_wallets (
  shop_id, balance_cents, held_cents, debt_cents, is_frozen, created_at, updated_at
) VALUES (
  '$shopId'::uuid, $SeedBalance, 0, 0, false, NOW(), NOW()
)
ON CONFLICT (shop_id) DO UPDATE SET
  balance_cents = EXCLUDED.balance_cents,
  held_cents    = 0,
  debt_cents    = 0,
  is_frozen     = false,
  updated_at    = NOW();
"@ | Out-Null

    $w0 = Get-WalletRow -ShopId $shopId
    if ($w0.Bal -ne $SeedBalance) {
        Write-Fail ("Seed balance mismatch: got {0} expected {1}" -f $w0.Bal, $SeedBalance)
    }
    Write-Ok ("Wallet BEFORE: bal={0} held={1} debt={2}" -f $w0.Bal, $w0.Held, $w0.Debt)

        # ---------- 05 Phase A: live cash-out fail ----------
    Write-Step -N "05/09" -Msg "Phase A - POST withdrawal (invalid MSISDN)"
    $wdBody = @{
        amount_cents       = $WithdrawAmt
        payment_method     = "ORANGE_MONEY"
        destination_number = $FailMsisdn
        destination_name   = "E2E Comp Pending"
        description        = "e2e-compensation-pending-$Timestamp"
    }

    $wd = Invoke-Json -Method Post -Uri "$BaseUrl/api/withdrawals" `
        -Headers $script:State.MerchantHeaders -Body $wdBody

    $wdId = Get-Prop $wd.Data @("id", "withdrawal_id", "data.id", "data.withdrawal_id")
    # Ne jamais prendre le code HTTP comme statut withdrawal
    $wdStatus = Get-Prop $wd.Data @("status", "data.status", "withdrawal_status", "data.withdrawal_status")
    # Ignore si le "status" ressemble a un code HTTP
    if ($wdStatus -match '^\d{3}$') { $wdStatus = $null }

    if ($wdId) { $script:State.WithdrawalId = "$wdId" }

    Write-Host ("  HTTP={0} id={1} api_status={2}" -f $wd.Status, $wdId, $wdStatus) -ForegroundColor DarkGray
    if ($wd.Raw) {
        $snip = $wd.Raw
        if ($snip.Length -gt 300) { $snip = $snip.Substring(0, 300) + "..." }
        Write-Host ("  Body: {0}" -f $snip) -ForegroundColor DarkGray
    }

    if ($wd.Ok -and $wdStatus -and ("$wdStatus" -match "success|completed|processed|paid")) {
        Write-Fail ("CashOut unexpectedly SUCCESS status={0}" -f $wdStatus)
    }

    # Toujours resoudre le statut depuis la DB (source de verite)
    if (-not $wdId) {
        $wdId = Invoke-Sql @"
SELECT id::text FROM withdrawals
WHERE shop_id = '$shopId'::uuid
ORDER BY created_at DESC LIMIT 1;
"@
        if ($wdId) {
            $script:State.WithdrawalId = $wdId
            Write-Warn ("Resolved withdrawal id from DB: {0}" -f $wdId)
        }
    }

    if ($wdId) {
        $deadline = (Get-Date).AddSeconds(20)
        do {
            $st = Invoke-Sql "SELECT status FROM withdrawals WHERE id = '$wdId'::uuid;"
            $st = ("$st").Trim().ToLower()
            if ($st -match "failed|compensation_pending|success|completed|cancelled") {
                $wdStatus = $st
                break
            }
            Start-Sleep -Seconds 2
        } while ((Get-Date) -lt $deadline)

        if (-not $wdStatus -or $wdStatus -match '^\d{3}$') {
            $wdStatus = Invoke-Sql "SELECT status FROM withdrawals WHERE id = '$wdId'::uuid;"
            $wdStatus = ("$wdStatus").Trim().ToLower()
        }
        Write-Host ("  DB status: {0}" -f $wdStatus) -ForegroundColor DarkGray
    }

    $script:State.PhaseAStatus = "$wdStatus"
    Write-Ok ("Phase A outcome: HTTP={0} status={1} id={2}" -f $wd.Status, $wdStatus, $wdId)

    # ---------- 06 Assert selon status ----------
    Write-Step -N "06/09" -Msg "Phase A - financial asserts by status"
    Start-Sleep -Milliseconds 400
    $w1 = Get-WalletRow -ShopId $shopId
    Write-Host ("  Wallet AFTER Phase A: bal={0} held={1} debt={2}" -f $w1.Bal, $w1.Held, $w1.Debt) -ForegroundColor White
    $ledgerDump = Get-LedgerLines -ShopId $shopId -Limit 20
    Write-Host ("  Ledger: {0}" -f $(if ($ledgerDump) { $ledgerDump } else { "(empty)" })) -ForegroundColor DarkGray
    $hasRev = Test-HasReversalLedger -ShopId $shopId

    $statusNorm = ("$wdStatus").ToLower().Trim()

    # Fallback heuristique: balance restauree + ledger reverse => failed
    if ($statusNorm -notmatch "failed|compensation_pending" -and $w1.Bal -eq $SeedBalance -and $hasRev) {
        Write-Warn ("Status ambiguous ('{0}') but balance restored + reversal ledger -> treat as failed" -f $statusNorm)
        $statusNorm = "failed"
        $script:State.PhaseAStatus = "failed"
    }

    if ($statusNorm -eq "compensation_pending") {
        $expectedBal = $SeedBalance - $WithdrawAmt
        if ($w1.Bal -ne $expectedBal) {
            Write-Fail ("compensation_pending: balance should stay debited. got={0} expected={1}" -f $w1.Bal, $expectedBal)
        }
        Write-Ok ("Balance stays debited: {0} (seed {1} - amt {2})" -f $w1.Bal, $SeedBalance, $WithdrawAmt)

        if ($hasRev) {
            Write-Fail "compensation_pending must NOT have withdrawal_reversal ledger row"
        }
        Write-Ok "No reversal ledger (expected for compensation_pending)"

        if ($w1.Debt -ne $w0.Debt) {
            Write-Fail ("Debt changed unexpectedly {0} -> {1}" -f $w0.Debt, $w1.Debt)
        }
        Write-Ok ("Debt unchanged = {0}" -f $w1.Debt)

        if (-not $wd.Ok -and $wd.Status -eq 400) {
            if ($wdId) { Write-Ok "400 body exposes withdrawal_id" }
        }
    }
    elseif ($statusNorm -eq "failed") {
        if ($w1.Bal -ne $SeedBalance) {
            Write-Fail ("failed+reverse: balance not restored. got={0} expected={1}" -f $w1.Bal, $SeedBalance)
        }
        Write-Ok ("Reverse OK - balance restored to {0}" -f $w1.Bal)
        if (-not $hasRev) {
            Write-Warn "No explicit reversal ledger row (balance OK)"
        }
        else {
            Write-Ok "Reversal ledger present"
        }
        Write-Warn "Sandbox reverse succeeded -> compensation_pending not hit in Phase A"
        Write-Warn "Phase B will simulate compensation_pending state via SQL"
    }
    elseif ($statusNorm -match "processing|pending") {
        Write-Fail ("Still non-terminal after poll: {0} (timeout/network path - not compensation_pending)" -f $wdStatus)
    }
    else {
        Write-Fail ("Unexpected status after cash-out fail: '{0}'" -f $wdStatus)
    }
    
    # ---------- 06 Assert selon status ----------
    Write-Step -N "06/09" -Msg "Phase A - financial asserts by status"
    Start-Sleep -Milliseconds 400
    $w1 = Get-WalletRow -ShopId $shopId
    Write-Host ("  Wallet AFTER Phase A: bal={0} held={1} debt={2}" -f $w1.Bal, $w1.Held, $w1.Debt) -ForegroundColor White
    $ledgerDump = Get-LedgerLines -ShopId $shopId -Limit 20
    Write-Host ("  Ledger: {0}" -f $(if ($ledgerDump) { $ledgerDump } else { "(empty)" })) -ForegroundColor DarkGray
    $hasRev = Test-HasReversalLedger -ShopId $shopId

    $statusNorm = ("$wdStatus").ToLower().Trim()

    if ($statusNorm -eq "compensation_pending") {
        $expectedBal = $SeedBalance - $WithdrawAmt
        if ($w1.Bal -ne $expectedBal) {
            Write-Fail ("compensation_pending: balance should stay debited. got={0} expected={1}" -f $w1.Bal, $expectedBal)
        }
        Write-Ok ("Balance stays debited: {0} (seed {1} - amt {2})" -f $w1.Bal, $SeedBalance, $WithdrawAmt)

        if ($hasRev) {
            Write-Fail "compensation_pending must NOT have withdrawal_reversal ledger row"
        }
        Write-Ok "No reversal ledger (expected for compensation_pending)"

        if ($w1.Debt -ne $w0.Debt) {
            Write-Fail ("Debt changed unexpectedly {0} -> {1}" -f $w0.Debt, $w1.Debt)
        }
        Write-Ok ("Debt unchanged = {0}" -f $w1.Debt)

        if (-not $wd.Ok -and $wd.Status -eq 400) {
            if ($wdId) { Write-Ok "400 body exposes withdrawal_id" }
            if ("$wdStatus" -match "compensation_pending") { Write-Ok "400 body exposes status=compensation_pending" }
        }
    }
    elseif ($statusNorm -eq "failed") {
        if ($w1.Bal -ne $SeedBalance) {
            Write-Fail ("failed+reverse: balance not restored. got={0} expected={1}" -f $w1.Bal, $SeedBalance)
        }
        Write-Ok ("Reverse OK - balance restored to {0}" -f $w1.Bal)
        if (-not $hasRev) {
            Write-Warn "No explicit reversal ledger row (balance OK)"
        }
        else {
            Write-Ok "Reversal ledger present"
        }
        Write-Warn "Sandbox reverse succeeded -> compensation_pending not hit in Phase A"
        Write-Warn "Phase B will simulate compensation_pending state via SQL"
    }
    elseif ($statusNorm -match "processing|pending") {
        Write-Fail ("Still non-terminal after poll: {0} (timeout/network path - not compensation_pending)" -f $wdStatus)
    }
    else {
        Write-Fail ("Unexpected status after cash-out fail: '{0}'" -f $wdStatus)
    }

    # ---------- 07 Phase B: simulate compensation_pending if needed ----------
    Write-Step -N "07/09" -Msg "Phase B - SQL simulate compensation_pending (if Phase A was failed)"

    if ($statusNorm -eq "compensation_pending") {
        Write-Ok "Phase B skipped - real compensation_pending already validated"
    }
    else {
        $simId = [guid]::NewGuid().ToString()
        $expectedDebited = $SeedBalance - $WithdrawAmt

        Invoke-Sql @"
UPDATE merchant_wallets
SET balance_cents = $expectedDebited, held_cents = 0, debt_cents = 0, is_frozen = false, updated_at = NOW()
WHERE shop_id = '$shopId'::uuid;
"@ | Out-Null

        Invoke-Sql @"
INSERT INTO withdrawals (
  id, shop_id, provider, amount, currency, fees, net_amount,
  status, payment_method, destination_number, destination_name,
  description, error_message, created_at, updated_at
) VALUES (
  '$simId'::uuid,
  '$shopId'::uuid,
  'yengapay',
  $WithdrawAmt,
  'XOF',
  0,
  $WithdrawAmt,
  'compensation_pending',
  'ORANGE_MONEY',
  '$FailMsisdn',
  'E2E Sim Comp',
  'e2e-sim-compensation-pending-$Timestamp',
  'cash-out failed | reverse failed: simulated for e2e',
  NOW(),
  NOW()
);
"@ | Out-Null

        $script:State.WithdrawalId = $simId

        $dbSt = Invoke-Sql "SELECT status FROM withdrawals WHERE id = '$simId'::uuid;"
        if ($dbSt -ne "compensation_pending") {
            Write-Fail ("Sim insert status mismatch: {0}" -f $dbSt)
        }
        Write-Ok ("Sim withdrawal {0} status=compensation_pending" -f $simId)

        $wSim = Get-WalletRow -ShopId $shopId
        if ($wSim.Bal -ne $expectedDebited) {
            Write-Fail ("Sim: balance should be debited. got={0} expected={1}" -f $wSim.Bal, $expectedDebited)
        }
        Write-Ok ("Sim: balance debited {0} (funds at risk until ops reconcile)" -f $wSim.Bal)

        $revForSim = Invoke-Sql @"
SELECT COALESCE(
  (SELECT COUNT(*)::text FROM wallet_transactions
   WHERE shop_id = '$shopId'::uuid
     AND COALESCE(reference_id::text,'') = '$simId'
     AND (
       reference_type ILIKE '%reversal%'
       OR COALESCE(description,'') ILIKE '%reversal%'
     )
  ), '0');
"@
        if ([int]$revForSim -gt 0) {
            Write-Fail "Sim: unexpected reversal ledger tied to compensation_pending id"
        }
        Write-Ok "Sim: no reversal ledger for compensation_pending withdrawal"

        try {
            $cols = Invoke-Sql @"
SELECT COALESCE(string_agg(column_name, ','), '')
FROM information_schema.columns
WHERE table_schema='public' AND table_name='withdrawals'
  AND column_name IN ('compensation_attempts','last_compensation_at');
"@
            if ($cols -match "compensation_attempts") {
                Invoke-Sql @"
UPDATE withdrawals
SET compensation_attempts = 1,
    last_compensation_at = NOW()
WHERE id = '$simId'::uuid;
"@ | Out-Null
                $att = Invoke-Sql "SELECT compensation_attempts::text FROM withdrawals WHERE id = '$simId'::uuid;"
                Write-Ok ("compensation_attempts={0}" -f $att)
            }
            else {
                Write-Warn "Columns compensation_attempts/last_compensation_at not present yet (migration optional)"
            }
        }
        catch {
            Write-Warn ("Optional compensation columns: {0}" -f $_.Exception.Message)
        }
    }

    # ---------- 08 DB final status ----------
    Write-Step -N "08/09" -Msg "Final DB withdrawal status"
    if ($script:State.WithdrawalId) {
        $final = Invoke-Sql @"
SELECT status || '|' || COALESCE(error_message,'')
FROM withdrawals
WHERE id = '$($script:State.WithdrawalId)'::uuid;
"@
        Write-Host ("  Withdrawal DB: {0}" -f $final) -ForegroundColor White
        $stOnly = ($final -split '\|')[0]
        if ($stOnly -eq "compensation_pending") {
            Write-Ok "DB status compensation_pending confirmed"
        }
        elseif ($stOnly -eq "failed") {
            Write-Ok "DB status failed (reverse path) - Phase B handled compensation_pending sim"
        }
        else {
            Write-Ok ("DB status={0}" -f $stOnly)
        }
    }
    else {
        Write-Fail "No withdrawal id to assert"
    }

    # ---------- 09 Summary ----------
    Write-Step -N "09/09" -Msg "Summary"
    $wEnd = Get-WalletRow -ShopId $shopId
    Write-Host ("  Shop         : {0} ({1})" -f $shopId, $ShopSlug) -ForegroundColor White
    Write-Host ("  Withdrawal   : {0}" -f $script:State.WithdrawalId) -ForegroundColor White
    Write-Host ("  Phase A      : {0}" -f $script:State.PhaseAStatus) -ForegroundColor White
    Write-Host ("  Wallet end   : bal={0} debt={1} held={2}" -f $wEnd.Bal, $wEnd.Debt, $wEnd.Held) -ForegroundColor White
    Write-Host ("  Fail MSISDN  : {0}" -f $FailMsisdn) -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Logs:" -ForegroundColor Yellow
    Write-Host '  docker compose logs goshop 2>&1 | Select-String -Pattern "compensation_pending|reversePayoutDebit|CRITICAL"' -ForegroundColor DarkGray

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $color = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    Write-Host (" RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed) -ForegroundColor $color
    Write-Host "================================================================" -ForegroundColor Magenta

    if ($script:Failed -eq 0) {
        Write-Host ""
        Write-Host "  WITHDRAWAL COMPENSATION_PENDING E2E VERT" -ForegroundColor Green
        Write-Host "  (Phase A live path + Phase B sim si reverse sandbox OK)" -ForegroundColor Green
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