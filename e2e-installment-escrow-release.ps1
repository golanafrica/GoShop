# ============================================================
# GOSHOP E2E - Installment Payment & Escrow Release (Crédit)
# Flux réaliste (aligné code) :
#   1) Health + admin
#   2) Merchant + shop + product + plan 3 tranches
#   3) Customer + score fiabilité + order installment
#   4) Vérif order_installments
#   5) Simuler chaque tranche : credit balance + hold (comme ProcessInstallmentPayment)
#   6) POST /api/orders/{id}/release-escrow
#   7) Assert held↓, balance = net (total - 5%), ledger
#
# Env:
#   $env:ADMIN_EMAIL / $env:ADMIN_PASSWORD
#   $env:E2E_CUSTOMER_PHONE   (default: +22676619457)
#   $env:MERCHANT_PASSWORD    (default: random)
#   $env:CUSTOMER_PASSWORD    (default: random)
#   $env:GOSHOP_BASE_URL      (default http://localhost:8080)
#   $env:DB_SERVICE           (default db)
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

# 🛡️ Numéros de téléphone et mots de passe dynamiques (pas de hardcode)
$CustomerPhone   = if ($env:E2E_CUSTOMER_PHONE) { $env:E2E_CUSTOMER_PHONE } else { "+22676619457" }
$MerchantPassword = if ($env:MERCHANT_PASSWORD) { $env:MERCHANT_PASSWORD } else { "TestPass!" + (Get-Random -Minimum 1000 -Maximum 9999) }
$CustomerPassword = if ($env:CUSTOMER_PASSWORD) { $env:CUSTOMER_PASSWORD } else { "TestPass!" + (Get-Random -Minimum 1000 -Maximum 9999) }

$NbTranches    = 3
$DelaiJours    = 5
$TotalCents    = 300000   # 3 000 FCFA
$CommissionBps = 500      # 5% (release_escrow_funds.go)
$Timestamp     = Get-Date -Format "yyyyMMddHHmmss"
$ShopSlug      = "installment-shop-$Timestamp"
$MerchantEmail = "merchant.installment.$Timestamp@goshop.com"
$CustomerEmail = "customer.installment.$Timestamp@goshop.com"

$ExpectedCommission = [int64](($TotalCents * $CommissionBps) / 10000)
$ExpectedNet        = $TotalCents - $ExpectedCommission

$script:Passed = 0
$script:Failed = 0
$script:State  = @{
    MerchantHeaders = $null
    AdminHeaders    = $null
    ShopId          = $null
    ProductId       = $null
    OrderId         = $null
    CustomerId      = $null
}

# -------------------- HELPERS --------------------
function Write-Step { param([string]$N, [string]$Msg); Write-Host ("[{0}] {1}" -f $N, $Msg) -ForegroundColor Cyan }
function Write-Ok   { param([string]$Msg); Write-Host ("  [OK] {0}" -f $Msg) -ForegroundColor Green; $script:Passed++ }
function Write-Fail { param([string]$Msg); Write-Host ("  [FAIL] {0}" -f $Msg) -ForegroundColor Red; $script:Failed++; throw $Msg }
function Write-Warn { param([string]$Msg); Write-Host ("  [WARN] {0}" -f $Msg) -ForegroundColor Yellow }

function Invoke-Json {
    param(
        [string]$Method,
        [string]$Uri,
        [hashtable]$Headers = @{},
        [object]$Body = $null,
        [int[]]$OkStatus = @(200, 201, 202)
    )
    $params = @{
        Method          = $Method
        Uri             = $Uri
        Headers         = $Headers
        UseBasicParsing = $true
    }
    if ($null -ne $Body) {
        $jsonBody = if ($Body -is [string]) { $Body } else { ($Body | ConvertTo-Json -Depth 12 -Compress) }
        $params.Body = [System.Text.Encoding]::UTF8.GetBytes($jsonBody)
        $params.ContentType = "application/json; charset=utf-8"
    }
    try {
        $resp = Invoke-WebRequest @params
        $code = [int]$resp.StatusCode
        $data = $null
        if ($resp.Content) {
            try { $data = $resp.Content | ConvertFrom-Json } catch { $data = $resp.Content }
        }
        return @{ Ok = ($OkStatus -contains $code); Status = $code; Data = $data; Raw = $resp.Content }
    }
    catch {
        $code = 0
        $raw = $null
        if ($_.ErrorDetails -and $_.ErrorDetails.Message) { $raw = $_.ErrorDetails.Message }
        if ($_.Exception.Response) {
            try { $code = [int]$_.Exception.Response.StatusCode.value__ } catch {
                try { $code = [int]$_.Exception.Response.StatusCode } catch {}
            }
            if (-not $raw) {
                try {
                    $stream = $_.Exception.Response.GetResponseStream()
                    if ($stream) {
                        $reader = New-Object System.IO.StreamReader($stream, [System.Text.Encoding]::UTF8)
                        $raw = $reader.ReadToEnd()
                        $reader.Close()
                    }
                } catch {}
            }
        }
        if (-not $raw) { $raw = $_.Exception.Message }
        $data = $null
        try { $data = $raw | ConvertFrom-Json } catch {}
        return @{ Ok = $false; Status = $code; Data = $data; Raw = $raw }
    }
}

function Invoke-Sql {
    param([string]$Sql)
    $argList = @("compose", "exec", "-T", $DbService, "psql", "-U", $DbUser, "-d", $DbName, "-t", "-A", "-c", $Sql)
    $out = & docker @argList 2>&1
    if ($LASTEXITCODE -ne 0) { throw ("SQL failed: {0}" -f $out) }
    return (($out | ForEach-Object { "$_" }) -join "`n").Trim()
}

function Get-Prop {
    param($Obj, [string[]]$Paths)
    foreach ($p in $Paths) {
        $cur = $Obj; $ok = $true
        foreach ($seg in ($p -split '\.')) {
            if ($null -eq $cur) { $ok = $false; break }
            if ($cur -is [hashtable] -and $cur.ContainsKey($seg)) { $cur = $cur[$seg]; continue }
            $prop = $cur.PSObject.Properties[$seg]
            if ($prop) { $cur = $prop.Value } else { $ok = $false; break }
        }
        if ($ok -and $null -ne $cur -and "$cur" -ne "") { return $cur }
    }
    return $null
}

function Get-WalletRow {
    param([string]$ShopId)
    $sql = "SELECT balance_cents::text || '|' || COALESCE(held_cents,0)::text || '|' || COALESCE(debt_cents,0)::text FROM merchant_wallets WHERE shop_id = '{0}'::uuid;" -f $ShopId
    $row = Invoke-Sql $sql
    if (-not $row) { return @{ Bal = 0; Held = 0; Debt = 0 } }
    $p = $row.Split('|')
    return @{ Bal = [int64]$p[0]; Held = [int64]$p[1]; Debt = [int64]$p[2] }
}

# -------------------- MAIN --------------------
Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Installment Payment & Escrow Release" -ForegroundColor Magenta
Write-Host (" BaseUrl: {0} | Tranches: {1} | Total: {2} c | Net attendu: {3} (comm {4})" -f `
    $BaseUrl, $NbTranches, $TotalCents, $ExpectedNet, $ExpectedCommission) -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # 01 Health + Admin
    Write-Step -N "01/08" -Msg "Health & Admin Login"
    $h = Invoke-Json -Method GET -Uri "$BaseUrl/health/live" -OkStatus @(200)
    if (-not $h.Ok) { $h = Invoke-Json -Method GET -Uri "$BaseUrl/health/ready" -OkStatus @(200) }
    if (-not $h.Ok) { Write-Fail "API health failed" }
    Write-Ok "API live"

    $al = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $AdminEmail; password = $AdminPassword }
    if (-not $al.Ok) { Write-Fail ("Admin login failed: {0}" -f $al.Raw) }
    $adminTok = Get-Prop $al.Data @('access_token','token','data.access_token')
    if (-not $adminTok) { Write-Fail "Admin token missing" }
    $script:State.AdminHeaders = @{ Authorization = "Bearer $adminTok" }
    Write-Ok "Admin OK"

    # 02 Merchant + plan
    Write-Step -N "02/08" -Msg "Merchant + product + installment plan"
    $reg = Invoke-Json -Method POST -Uri "$BaseUrl/register" -Body @{
        email = $MerchantEmail; password = $MerchantPassword; role = "merchant"
    } -OkStatus @(200, 201)
    if (-not $reg.Ok) { Write-Fail ("Register merchant failed: {0}" -f $reg.Raw) }

    $ml = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $MerchantEmail; password = $MerchantPassword }
    $mTok = Get-Prop $ml.Data @('access_token','token','data.access_token')
    if (-not $mTok) { Write-Fail "Merchant token missing" }

    $shop = Invoke-Json -Method POST -Uri "$BaseUrl/api/shops" -Headers @{ Authorization = "Bearer $mTok" } -Body @{
        name = ("Installment Shop {0}" -f $Timestamp)
        slug = $ShopSlug
    } -OkStatus @(200, 201)
    $shopId = Get-Prop $shop.Data @('id','data.id','shop_id')
    if (-not $shopId) {
        $shopId = Invoke-Sql ("SELECT id::text FROM shops WHERE slug = '{0}' LIMIT 1;" -f $ShopSlug)
    }
    if (-not $shopId) { Write-Fail "Shop id missing" }
    $script:State.ShopId = $shopId
    $script:State.MerchantHeaders = @{
        Authorization = "Bearer $mTok"
        "X-Shop-Slug" = $ShopSlug
    }
    Write-Ok ("Shop {0}" -f $shopId)

    try {
        Invoke-Sql ("UPDATE shops SET kyc_status = 'verified', updated_at = NOW() WHERE id = '{0}'::uuid;" -f $shopId) | Out-Null
    } catch { Write-Warn "Shop KYC SQL skipped" }

    Invoke-Sql ("INSERT INTO merchant_wallets (shop_id, balance_cents, held_cents, debt_cents, is_frozen, created_at, updated_at) VALUES ('{0}'::uuid, 0, 0, 0, false, NOW(), NOW()) ON CONFLICT (shop_id) DO NOTHING;" -f $shopId) | Out-Null

    $prod = Invoke-Json -Method POST -Uri "$BaseUrl/api/products" -Headers $script:State.MerchantHeaders -Body @{
        name        = ("Produit Credit {0}" -f $Timestamp)
        description = "E2E installment"
        price_cents = $TotalCents
        stock       = 10
    } -OkStatus @(200, 201)
    $productId = Get-Prop $prod.Data @('id','data.id')
    if (-not $productId) { Write-Fail ("Product failed: {0}" -f $prod.Raw) }
    $script:State.ProductId = $productId
    Write-Ok ("Product {0}" -f $productId)

    $zoneSql = "INSERT INTO delivery_zones (zone_code, zone_name, country, zone_type, delivery_delay_days, return_delay_days, warranty_response_days, cod_confirmation_delay_days, installment_release_delay_days, is_active, priority) VALUES ('BF-OUAGA-URB', 'Ouagadougou Urbain', 'BF', 'urban', 5, 14, 7, 7, 5, true, 10) ON CONFLICT (zone_code) DO UPDATE SET installment_release_delay_days = 5, is_active = true, updated_at = NOW();"
    try { Invoke-Sql $zoneSql | Out-Null } catch { Write-Warn ("Zone upsert: {0}" -f $_.Exception.Message) }

    $plan = Invoke-Json -Method POST -Uri ("$BaseUrl/api/products/{0}/installment-plan" -f $productId) -Headers $script:State.MerchantHeaders -Body @{
        nb_tranches        = $NbTranches
        delai_jours        = $DelaiJours
        delivery_zone_code = "BF-OUAGA-URB"
    } -OkStatus @(200, 201)
    if (-not $plan.Ok) { Write-Fail ("Configure plan failed: {0}" -f $plan.Raw) }
    Write-Ok ("Plan {0} tranches / {1}j" -f $NbTranches, $DelaiJours)

    $planDb = Invoke-Sql ("SELECT id::text || '|' || is_active::text FROM installment_plans WHERE product_id = '{0}'::uuid LIMIT 1;" -f $productId)
    if (-not $planDb) { Write-Fail "Plan absent en DB" }
    if ($planDb -match '\|f') { Write-Fail "Plan is_active=false" }
    Write-Ok ("Plan DB OK ({0})" -f $planDb)

    # 03 Customer + order
    Write-Step -N "03/08" -Msg "Customer + installment order"
    $null = Invoke-Json -Method POST -Uri "$BaseUrl/register" -Body @{
        email = $CustomerEmail; password = $CustomerPassword; role = "user"
    } -OkStatus @(200, 201, 409)
    $cl = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $CustomerEmail; password = $CustomerPassword }
    $cTok = Get-Prop $cl.Data @('access_token','token','data.access_token')
    $cUserId = Get-Prop $cl.Data @('user.id','data.user.id','id','data.id')
    if (-not $cUserId) {
        $cUserId = Invoke-Sql ("SELECT id::text FROM users WHERE email = '{0}' LIMIT 1;" -f $CustomerEmail)
    }

    $cust = Invoke-Json -Method POST -Uri "$BaseUrl/api/customers" -Headers $script:State.MerchantHeaders -Body @{
        first_name = "Client"
        last_name  = "Credit"
        email      = $CustomerEmail
        phone      = $CustomerPhone
        user_id    = "$cUserId"
    } -OkStatus @(200, 201)
    $custId = Get-Prop $cust.Data @('id','data.id','customer.id')
    if (-not $custId) {
        $custId = Invoke-Sql ("INSERT INTO customers (id, shop_id, first_name, last_name, phone, email, user_id, kyc_level, created_at, updated_at) VALUES (gen_random_uuid(), '{0}'::uuid, 'Client', 'Credit', '{1}', '{2}', '{3}', 'verified', NOW(), NOW()) RETURNING id::text;" -f $shopId, $CustomerPhone, $CustomerEmail, $cUserId)
    }
    if (-not $custId) { Write-Fail "Customer id missing" }
    $script:State.CustomerId = $custId
    Write-Ok ("Customer {0}" -f $custId)

    # Score SILVER (max 5 tranches) — Bronze aussi OK pour 3
    try {
        Invoke-Sql ("INSERT INTO customer_reliability_scores (id, customer_id, score, tier, last_calculated_at, created_at, updated_at) VALUES (gen_random_uuid(), '{0}'::uuid, 650, 'SILVER', NOW(), NOW(), NOW()) ON CONFLICT (customer_id) DO UPDATE SET score = 650, tier = 'SILVER', updated_at = NOW();" -f $custId) | Out-Null
    } catch {
        Write-Warn ("Reliability score: {0}" -f $_.Exception.Message)
    }

    $order = Invoke-Json -Method POST -Uri "$BaseUrl/api/orders/installment" -Headers $script:State.MerchantHeaders -Body @{
        customer_id    = $custId
        payment_method = "mobile_money"
        items          = @(@{
            product_id  = $productId
            quantity    = 1
            price_cents = $TotalCents
        })
    } -OkStatus @(200, 201)
    if (-not $order.Ok) { Write-Fail ("Order creation failed: {0}" -f $order.Raw) }
    $orderId = Get-Prop $order.Data @('id','data.id','order.id','data.order.id')
    if (-not $orderId) { Write-Fail ("Order id missing: {0}" -f $order.Raw) }
    $script:State.OrderId = $orderId
    Write-Ok ("Order {0}" -f $orderId)

    # 04 Verify installments
    Write-Step -N "04/08" -Msg "Verify installments generation"
    $instCount = Invoke-Sql ("SELECT COUNT(*)::text FROM order_installments WHERE order_id = '{0}'::uuid;" -f $orderId)
    if ([int]$instCount -ne $NbTranches) {
        Write-Fail ("Expected {0} installments, got {1}" -f $NbTranches, $instCount)
    }
    Write-Ok ("{0} installments in DB" -f $instCount)

    $rows = Invoke-Sql ("SELECT tranche_number::text || '=' || amount_cents::text FROM order_installments WHERE order_id = '{0}'::uuid ORDER BY tranche_number;" -f $orderId)
    Write-Host ("  Tranches amounts: {0}" -f ($rows -replace "`n", "; ")) -ForegroundColor DarkGray

    # 05 Simulate REAL tranche payments (credit + hold)
    Write-Step -N "05/08" -Msg "Simulate tranche payments (balance+= + held+=)"
    $wBefore = Get-WalletRow -ShopId $shopId
    Write-Host ("  Wallet BEFORE: bal={0} held={1} debt={2}" -f $wBefore.Bal, $wBefore.Held, $wBefore.Debt) -ForegroundColor DarkGray

    $sumPaid = [int64]0
    for ($i = 1; $i -le $NbTranches; $i++) {
        $amtStr = Invoke-Sql ("SELECT amount_cents::text FROM order_installments WHERE order_id = '{0}'::uuid AND tranche_number = {1};" -f $orderId, $i)
        if (-not $amtStr) { Write-Fail ("Tranche {0} amount missing" -f $i) }
        $amt = [int64]$amtStr

        # 1) Mark paid (équivalent MarkAsPaid)
        Invoke-Sql ("UPDATE order_installments SET status = 'paid', paid_at = NOW(), payment_ref = 'SIM-INST-{0}-{1}' WHERE order_id = '{2}'::uuid AND tranche_number = {1};" -f $Timestamp, $i, $orderId) | Out-Null

        # 2) Credit pay-in puis hold (état final = ProcessInstallmentPayment cohérent avec Hold())
        #    balance += amt  (fonds reçus)
        #    held    += amt  (séquestre)
        Invoke-Sql ("UPDATE merchant_wallets SET balance_cents = balance_cents + {0}, held_cents = held_cents + {0}, updated_at = NOW() WHERE shop_id = '{1}'::uuid;" -f $amt, $shopId) | Out-Null

        $sumPaid += $amt
        $wMid = Get-WalletRow -ShopId $shopId
        Write-Ok ("Tranche {0} paid amount={1} | bal={2} held={3}" -f $i, $amt, $wMid.Bal, $wMid.Held)
    }

    if ($sumPaid -ne $TotalCents) {
        Write-Warn ("Sum tranches {0} != TotalCents {1} (reste éventuel sur dernière tranche)" -f $sumPaid, $TotalCents)
    }

    try {
        Invoke-Sql ("UPDATE orders SET is_fully_paid_in_escrow = true, updated_at = NOW() WHERE id = '{0}'::uuid;" -f $orderId) | Out-Null
    } catch { Write-Warn "is_fully_paid_in_escrow column skipped" }

    $wAfterHold = Get-WalletRow -ShopId $shopId
    if ($wAfterHold.Held -lt $TotalCents) {
        Write-Fail ("Held after payments {0} < total {1}" -f $wAfterHold.Held, $TotalCents)
    }
    Write-Ok ("All tranches paid | held={0} bal={1}" -f $wAfterHold.Held, $wAfterHold.Bal)

    # 06 Release escrow
    Write-Step -N "06/08" -Msg "Trigger escrow release"
    $release = Invoke-Json -Method POST -Uri ("$BaseUrl/api/orders/{0}/release-escrow" -f $orderId) -Headers $script:State.MerchantHeaders -OkStatus @(200, 201, 202)
    if (-not $release.Ok) {
        Write-Fail ("Release escrow failed HTTP {0}: {1}" -f $release.Status, $release.Raw)
    }
    $netApi  = Get-Prop $release.Data @('release.net_merchant_cents','data.release.net_merchant_cents','net_merchant_cents')
    $commApi = Get-Prop $release.Data @('release.commission_cents','data.release.commission_cents','commission_cents')
    Write-Ok ("Release OK net={0} commission={1}" -f $netApi, $commApi)

    # 07 Asserts
    Write-Step -N "07/08" -Msg "Assert financial state"
    Start-Sleep -Seconds 1
    $wFinal = Get-WalletRow -ShopId $shopId

    Write-Host ("  BEFORE hold : bal={0} held={1}" -f $wBefore.Bal, $wBefore.Held) -ForegroundColor DarkGray
    Write-Host ("  AFTER  hold : bal={0} held={1}" -f $wAfterHold.Bal, $wAfterHold.Held) -ForegroundColor DarkGray
    Write-Host ("  FINAL       : bal={0} held={1} debt={2}" -f $wFinal.Bal, $wFinal.Held, $wFinal.Debt) -ForegroundColor Cyan
    Write-Host ("  Expected    : bal~={0} held=0 (comm {1})" -f $ExpectedNet, $ExpectedCommission) -ForegroundColor DarkGray

    $heldDrop = $wAfterHold.Held - $wFinal.Held
    if ($heldDrop -lt $TotalCents) {
        Write-Fail ("Held drop {0} < total {1}" -f $heldDrop, $TotalCents)
    } else {
        Write-Ok ("Held released drop={0}" -f $heldDrop)
    }

    if ($wFinal.Held -ne 0) {
        Write-Warn ("Held residual={0} (attendu 0 si full release)" -f $wFinal.Held)
    } else {
        Write-Ok "Held = 0"
    }

    $deltaBal = $wFinal.Bal - $wBefore.Bal
    if ($deltaBal -ne $ExpectedNet) {
        Write-Fail ("Balance delta {0} != expected net {1}" -f $deltaBal, $ExpectedNet)
    } else {
        Write-Ok ("Balance += net {0}" -f $ExpectedNet)
    }

    $ledgerCount = Invoke-Sql ("SELECT COUNT(*)::text FROM wallet_transactions WHERE shop_id = '{0}'::uuid AND created_at > NOW() - INTERVAL '10 minutes';" -f $shopId)
    if ([int]$ledgerCount -ge 1) {
        Write-Ok ("Ledger rows last 10m: {0}" -f $ledgerCount)
    } else {
        Write-Warn "No ledger rows (release peut ne pas écrire hors debt_sweep)"
    }

    # 08 Summary
    Write-Step -N "08/08" -Msg "Summary"
    Write-Host ("  Shop    : {0} ({1})" -f $shopId, $ShopSlug) -ForegroundColor White
    Write-Host ("  Product : {0}" -f $productId) -ForegroundColor White
    Write-Host ("  Order   : {0}" -f $orderId) -ForegroundColor White
    Write-Host ("  Total   : {0} c / {1} tranches | net {2}" -f $TotalCents, $NbTranches, $ExpectedNet) -ForegroundColor White
    Write-Host ("  Final   : bal={0} held={1} debt={2}" -f $wFinal.Bal, $wFinal.Held, $wFinal.Debt) -ForegroundColor White

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $color = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    Write-Host (" RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed) -ForegroundColor $color
    Write-Host "================================================================" -ForegroundColor Magenta

    if ($script:Failed -eq 0) {
        Write-Host ""
        Write-Host "  INSTALLMENT ESCROW RELEASE E2E VERT" -ForegroundColor Green
        Write-Host "  (credit+hold par tranche, release API, commission 5%)" -ForegroundColor Green
    } else {
        exit 1
    }
}
catch {
    Write-Host ""
    Write-Host ("[CRITICAL FAILURE] {0}" -f $_.Exception.Message) -ForegroundColor Red
    exit 1
}

Write-Host ""