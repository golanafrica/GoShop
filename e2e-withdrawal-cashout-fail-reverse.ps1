# ============================================================
# GOSHOP E2E - Withdrawal CashOut FAIL -> reverse debit (P0-A)
# ============================================================
# Objectif P0 (audit finance) :
#   1) Wallet seed balance = 100000 cents
#   2) POST /api/withdrawals amount = 50000
#   3) CashOut provider MUST fail (definitive reject)
#   4) Assert:
#        - withdrawal status = failed (or API error after debit path)
#        - balance back to 100000 (no silent loss)
#        - ledger has withdrawal_reversal / payout_reversal (or equivalent)
#        - debt_cents unchanged
#        - held_cents unchanged
#
# Env:
#   $env:ADMIN_EMAIL / $env:ADMIN_PASSWORD
#   $env:GOSHOP_BASE_URL
#   $env:DB_SERVICE / DB_USER / DB_NAME
#   $env:WITHDRAW_FAIL_MSISDN  (default +22699999999 - MUST NOT be whitelisted)
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
# 🛡️ Numéro volontairement NON whitelisté pour forcer l'échec 403 du Sandbox YengaPay
$FailMsisdn    = if ($env:WITHDRAW_FAIL_MSISDN) { $env:WITHDRAW_FAIL_MSISDN } else { "+22699999999" }

# 🛡️ Mot de passe dynamique (pas de hardcode)
$MerchantPassword = if ($env:MERCHANT_PASSWORD) { $env:MERCHANT_PASSWORD } else { "TestPass!" + (Get-Random -Minimum 1000 -Maximum 9999) }

$Timestamp     = Get-Date -Format "yyyyMMddHHmmss"
$ShopSlug      = "wd-reverse-$Timestamp"
$MerchantEmail = "merchant.wdrev.$Timestamp@goshop.com"

$script:Passed = 0
$script:Failed = 0
$script:State  = @{
    AdminHeaders    = $null
    MerchantHeaders = $null
    MerchantUserId  = $null
    ShopId          = $null
    WithdrawalId    = $null
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

# SQL via stdin -> conteneur (PAS de -f path Windows)
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

function Find-LedgerMatch {
    param(
        [string]$ShopId,
        [string[]]$TypeNeedles,
        [string[]]$RefNeedles = @()
    )
    $qTypes = ($TypeNeedles | ForEach-Object { "'" + $_ + "'" }) -join ","
    $refClause = ""
    if ($RefNeedles.Count -gt 0) {
        $likes = $RefNeedles | ForEach-Object {
            "reference_type ILIKE '%$_%' OR COALESCE(description,'') ILIKE '%$_%'"
        }
        $refClause = " AND (" + ($likes -join " OR ") + ")"
    }
    $line = Invoke-Sql @"
SELECT COALESCE(
  (SELECT transaction_type || '|' || amount_cents::text || '|' || COALESCE(reference_type,'') || '|' || balance_after_cents::text
   FROM wallet_transactions
   WHERE shop_id = '$ShopId'::uuid
     AND (
       transaction_type::text IN ($qTypes)
       OR transaction_type::text ILIKE ANY (ARRAY[$qTypes])
     )
     $refClause
   ORDER BY created_at DESC
   LIMIT 1),
  ''
);
"@
    return $line
}

# -------------------- MAIN --------------------
Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Withdrawal CashOut FAIL -> Reverse Debit (P0)" -ForegroundColor Magenta
Write-Host (" BaseUrl: {0} | Seed={1} | Withdraw={2} | FailMSISDN={3} | DbService={4}" -f $BaseUrl, $SeedBalance, $WithdrawAmt, $FailMsisdn, $DbService) -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # ---------- 01 Health ----------
    Write-Step -N "01/08" -Msg "Health"
    $h = Invoke-Json -Method Get -Uri "$BaseUrl/health/live"
    if (-not $h.Ok -or $h.Status -ne 200) {
        $h2 = Invoke-Json -Method Get -Uri "$BaseUrl/health"
        if (-not $h2.Ok) { Write-Fail "API health failed" }
    }
    Write-Ok ("API live HTTP {0}" -f $(if ($h.Ok) { $h.Status } else { 200 }))

    # ---------- 02 Admin login ----------
    Write-Step -N "02/08" -Msg "Admin login"
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
    Write-Step -N "03/08" -Msg "Merchant + shop + KYC"
    $reg = Invoke-Json -Method Post -Uri "$BaseUrl/register" -Body @{
        email      = $MerchantEmail
        password   = $MerchantPassword
        first_name = "Wd"
        last_name  = "Reverse"
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
        name = "WD Reverse Shop $Timestamp"
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

    # KYC best-effort (colonne optionnelle)
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
    Write-Step -N "04/08" -Msg ("Seed wallet balance={0}" -f $SeedBalance)
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
    Write-Ok ("Wallet BEFORE: bal={0} held={1} debt={2} frozen={3}" -f $w0.Bal, $w0.Held, $w0.Debt, $w0.Frozen)

    # ---------- 05 Withdrawal (expect CashOut fail) ----------
    Write-Step -N "05/08" -Msg "POST withdrawal (invalid MSISDN -> expect provider fail + reverse)"
    $wdBody = @{
        amount_cents       = $WithdrawAmt
        payment_method     = "ORANGE_MONEY"
        destination_number = $FailMsisdn
        destination_name   = "E2E Reverse Test"
        description        = "e2e-cashout-fail-reverse-$Timestamp"
    }

    $wd = Invoke-Json -Method Post -Uri "$BaseUrl/api/withdrawals" `
        -Headers $script:State.MerchantHeaders -Body $wdBody

    $wdId = Get-Prop $wd.Data @("id", "withdrawal_id", "data.id", "data.withdrawal_id")
    $wdStatus = Get-Prop $wd.Data @("status", "data.status")
    if ($wdId) { $script:State.WithdrawalId = "$wdId" }

    Write-Host ("  HTTP={0} id={1} status={2}" -f $wd.Status, $wdId, $wdStatus) -ForegroundColor DarkGray
    if ($wd.Raw) {
        $snip = $wd.Raw
        if ($snip.Length -gt 240) { $snip = $snip.Substring(0, 240) + "..." }
        Write-Host ("  Body: {0}" -f $snip) -ForegroundColor DarkGray
    }

    if ($wd.Ok -and $wdStatus -and ("$wdStatus" -match "success|completed|processed|paid")) {
        Write-Fail ("CashOut unexpectedly SUCCESS status={0} - change WITHDRAW_FAIL_MSISDN or mock provider" -f $wdStatus)
    }

    if ($wd.Ok -and $wdId -and ("$wdStatus" -match "pending|processing|initiated")) {
        Write-Warn "Status non-terminal - poll DB 15s for failed/success"
        $deadline = (Get-Date).AddSeconds(15)
        do {
            Start-Sleep -Seconds 2
            $st = Invoke-Sql "SELECT status FROM withdrawals WHERE id = '$wdId'::uuid;"
            if ($st -match "failed|success|completed|cancelled") { break }
        } while ((Get-Date) -lt $deadline)
        $wdStatus = $st
        Write-Host ("  DB status after poll: {0}" -f $wdStatus) -ForegroundColor DarkGray
        if ($wdStatus -match "success|completed") {
            Write-Fail "Provider accepted withdrawal - cannot validate reverse path with this MSISDN"
        }
    }

    if (-not $wd.Ok) {
        Write-Ok ("API rejected withdrawal HTTP {0} (acceptable if reverse already applied server-side)" -f $wd.Status)
    }
    else {
        Write-Ok ("Withdrawal response received status={0}" -f $(if ($wdStatus) { $wdStatus } else { "n/a" }))
    }

    # ---------- 06 Assert wallet restored ----------
    Write-Step -N "06/08" -Msg "Assert financial state (balance restored)"
    Start-Sleep -Milliseconds 500
    $w1 = Get-WalletRow -ShopId $shopId
    Write-Host ("  Wallet AFTER : bal={0} held={1} debt={2} frozen={3}" -f $w1.Bal, $w1.Held, $w1.Debt, $w1.Frozen) -ForegroundColor White

    if ($w1.Bal -ne $SeedBalance) {
        Write-Fail ("Balance NOT restored: before={0} after={1} (expected reverse of {2})" -f $w0.Bal, $w1.Bal, $WithdrawAmt)
    }
    Write-Ok ("Balance restored {0} -> {1}" -f $w0.Bal, $w1.Bal)

    if ($w1.Debt -ne $w0.Debt) {
        Write-Fail ("Debt changed unexpectedly {0} -> {1}" -f $w0.Debt, $w1.Debt)
    }
    Write-Ok ("Debt unchanged = {0}" -f $w1.Debt)

    if ($w1.Held -ne $w0.Held) {
        Write-Fail ("Held changed unexpectedly {0} -> {1}" -f $w0.Held, $w1.Held)
    }
    Write-Ok ("Held unchanged = {0}" -f $w1.Held)

    # ---------- 07 Ledger asserts ----------
    Write-Step -N "07/08" -Msg "Assert ledger debit + reversal"
    $ledgerDump = Get-LedgerLines -ShopId $shopId -Limit 20
    Write-Host ("  Recent ledger: {0}" -f $(if ($ledgerDump) { $ledgerDump } else { "(empty)" })) -ForegroundColor DarkGray

    $rev = Find-LedgerMatch -ShopId $shopId -TypeNeedles @(
        "deposit", "payout_reversal", "withdrawal_reversal", "credit", "WalletTxDeposit"
    ) -RefNeedles @("withdrawal_reversal", "payout_reversal", "reversal", "wd-reverse", "e2e-cashout-fail")

    $revExact = Invoke-Sql @"
SELECT COALESCE(
  (SELECT transaction_type || '|' || amount_cents::text || '|' || COALESCE(reference_type,'') || '|' || COALESCE(reference_id::text,'')
   FROM wallet_transactions
   WHERE shop_id = '$shopId'::uuid
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

    if ($revExact) {
        Write-Ok ("Reversal ledger row: {0}" -f $revExact)
    }
    elseif ($rev) {
        Write-Ok ("Reversal-like ledger row: {0}" -f $rev)
    }
    else {
        Write-Fail "No withdrawal_reversal / payout_reversal ledger row found (balance OK is not enough)"
    }

    if ($script:State.WithdrawalId) {
        $dbWd = Invoke-Sql @"
SELECT status || '|' || COALESCE(error_message,'')
FROM withdrawals
WHERE id = '$($script:State.WithdrawalId)'::uuid;
"@
        Write-Host ("  Withdrawal DB: {0}" -f $dbWd) -ForegroundColor DarkGray
        if ($dbWd -match "^(success|completed)") {
            Write-Fail ("Withdrawal still success in DB: {0}" -f $dbWd)
        }
        Write-Ok ("Withdrawal not success in DB ({0})" -f ($dbWd -split '\|')[0])
    }
    else {
        Write-Warn "No withdrawal id in API response - status assert skipped (balance+ledger still required)"
    }

    # ---------- 08 Summary ----------
    Write-Step -N "08/08" -Msg "Summary"
    Write-Host ("  Shop       : {0} ({1})" -f $shopId, $ShopSlug) -ForegroundColor White
    Write-Host ("  Withdrawal : {0}" -f $(if ($script:State.WithdrawalId) { $script:State.WithdrawalId } else { "n/a" })) -ForegroundColor White
    Write-Host ("  Wallet     : {0} -> {1} (expected {0})" -f $SeedBalance, $w1.Bal) -ForegroundColor White
    Write-Host ("  Debt/Held  : debt={0} held={1}" -f $w1.Debt, $w1.Held) -ForegroundColor White
    Write-Host ("  Fail MSISDN: {0}" -f $FailMsisdn) -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Logs:" -ForegroundColor Yellow
    Write-Host '  docker compose logs goshop 2>&1 | Select-String -Pattern "reversePayoutDebit|withdrawal_reversal|CashOut|create yenga"' -ForegroundColor DarkGray

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $color = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    Write-Host (" RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed) -ForegroundColor $color
    Write-Host "================================================================" -ForegroundColor Magenta

    if ($script:Failed -eq 0) {
        Write-Host ""
        Write-Host "  WITHDRAWAL CASHOUT-FAIL REVERSE E2E VERT" -ForegroundColor Green
        Write-Host "  (balance restored + reversal ledger + no debt side-effect)" -ForegroundColor Green
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