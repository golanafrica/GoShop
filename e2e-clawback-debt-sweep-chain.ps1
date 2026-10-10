# ============================================================
# GOSHOP E2E - Clawback post-release -> residual debt -> sweep
# ============================================================
# 1) Order A + REAL pay-in + release  -> wallet ~95000
# 2) Withdraw partial 50000           -> wallet ~45000
# 3) Dispute A + customer_wins        -> clawback + debt_cents > 0
# 4) Order B + REAL pay-in + release  -> CreditWithDebtSweep
# 5) Assert debt down + debt_sweep ledger
# 6) Withdrawal blocked if debt remains
#
# NO SQL force on customer_phone / debt_cents
#
# Env:
#   $env:YENGA_PAY_WEBHOOK_SECRET
#   $env:ADMIN_EMAIL / $env:ADMIN_PASSWORD
#   $env:E2E_CUSTOMER_PHONE   (default: +22676619457)
#   $env:E2E_WITHDRAW_PHONE   (default: +22665150303)
# ============================================================

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

$BaseUrl       = if ($env:GOSHOP_BASE_URL) { $env:GOSHOP_BASE_URL } else { "http://localhost:8080" }
$WebhookSecret = if ($env:YENGA_PAY_WEBHOOK_SECRET) { $env:YENGA_PAY_WEBHOOK_SECRET } else { "CHANGE_ME_YENGA_PAY_WEBHOOK_SECRET" }
$AdminEmail    = if ($env:ADMIN_EMAIL) { $env:ADMIN_EMAIL } else { "superadmin.yacine@goshop.com" }
$AdminPassword = if ($env:ADMIN_PASSWORD) { $env:ADMIN_PASSWORD } else { "CHANGE_ME_ADMIN_PASSWORD" }
$DbService     = if ($env:DB_SERVICE) { $env:DB_SERVICE } else { "db" }
$DbUser        = if ($env:DB_USER) { $env:DB_USER } else { "postgres" }
$DbName        = if ($env:DB_NAME) { $env:DB_NAME } else { "goshop_db" }

# 🛡️ Numéros de téléphone whitelistés pour le Sandbox YengaPay
$CustomerPhone = if ($env:E2E_CUSTOMER_PHONE) { $env:E2E_CUSTOMER_PHONE } else { "+22676619457" }
$WithdrawPhone = if ($env:E2E_WITHDRAW_PHONE) { $env:E2E_WITHDRAW_PHONE } else { "+22665150303" }

# 🛡️ Mot de passe généré dynamiquement (pas de mot de passe en clair dans le code)
$MerchantPassword = if ($env:MERCHANT_PASSWORD) { $env:MERCHANT_PASSWORD } else { "TestPass!" + (Get-Random -Minimum 1000 -Maximum 9999) }

$PayInTimeoutSec         = 300
$EligibilityBackdateDays = 6
$SchedulerWaitSec        = 12
$ProductPriceCents       = 100000
$WithdrawPartialCents    = 50000
$ExpectedNetRelease      = 95000

$Timestamp        = Get-Date -Format "yyyyMMddHHmmss"
$MerchantEmail    = "merchant.chain.$Timestamp@goshop.com"
$ShopName         = "Chain Debt $Timestamp"
$ShopSlug         = "chain-debt-$Timestamp"

$script:Passed = 0
$script:Failed = 0
$script:State  = @{
    MerchantHeaders = $null
    AdminHeaders    = $null
    ShopId          = $null
    OrderIdA        = $null
    OrderIdB        = $null
    PaymentIdA      = $null
    PaymentIdB      = $null
    DisputeId       = $null
}

function Write-Step {
    param([string]$N, [string]$Msg)
    Write-Host ""
    Write-Host "[$N] $Msg" -ForegroundColor Cyan
}

function Write-Ok {
    param([string]$Msg)
    Write-Host "  [OK] $Msg" -ForegroundColor Green
    $script:Passed++
}

function Write-Warn {
    param([string]$Msg)
    Write-Host "  [WARN] $Msg" -ForegroundColor Yellow
}

function Write-Fail {
    param([string]$Msg)
    Write-Host "  [FAIL] $Msg" -ForegroundColor Red
    $script:Failed++
    throw $Msg
}

function Invoke-SafeApi {
    param(
        [string]$Method,
        [string]$Uri,
        [hashtable]$Headers = @{},
        [object]$Body = $null
    )
    try {
        $params = @{
            Method          = $Method
            Uri             = $Uri
            Headers         = $Headers
            UseBasicParsing = $true
        }
        if ($null -ne $Body) {
            $json = if ($Body -is [string]) { $Body } else { ($Body | ConvertTo-Json -Depth 12 -Compress) }
            $params.Body        = [System.Text.Encoding]::UTF8.GetBytes($json)
            $params.ContentType = "application/json; charset=utf-8"
        }
        $wr = Invoke-WebRequest @params
        $data = $null
        if ($wr.Content) {
            try { $data = $wr.Content | ConvertFrom-Json } catch { $data = $wr.Content }
        }
        return @{ Success = $true; StatusCode = [int]$wr.StatusCode; Data = $data; Raw = $wr.Content }
    }
    catch {
        $code = 0
        $raw  = $_.Exception.Message
        if ($_.Exception.Response) {
            try { $code = [int]$_.Exception.Response.StatusCode } catch {}
        }
        if ($_.ErrorDetails.Message) { $raw = $_.ErrorDetails.Message }
        return @{ Success = $false; StatusCode = $code; Data = $null; Raw = $raw }
    }
}

function Assert-Ok {
    param($Res, [string]$Label, [int[]]$Codes = @(200, 201, 202))
    if (-not $Res.Success -or ($Codes -notcontains $Res.StatusCode)) {
        $detail = if ($Res.Raw) { $Res.Raw } else { "" }
        Write-Fail ("{0} -> HTTP {1} {2}" -f $Label, $Res.StatusCode, $detail)
    }
}

function Get-Prop {
    param($Obj, [string[]]$Paths)
    foreach ($p in $Paths) {
        $cur = $Obj
        $ok  = $true
        foreach ($seg in ($p -split '\.')) {
            if ($null -eq $cur) { $ok = $false; break }
            $cur = $cur.$seg
        }
        if ($ok -and $null -ne $cur -and "$cur" -ne "") { return $cur }
    }
    return $null
}

function Get-AuthToken {
    param($LoginData)
    $t = Get-Prop $LoginData @("access_token", "token", "data.access_token", "data.token")
    if (-not $t) { Write-Fail "token missing in login response" }
    return [string]$t
}

function Get-EntityId {
    param($Data, [string]$Label)
    $id = Get-Prop $Data @(
        "id", "data.id", "payment_id", "data.payment_id",
        "dispute.id", "data.dispute.id", "shop_id", "data.shop_id"
    )
    if (-not $id) {
        Write-Fail ("id missing for {0}: {1}" -f $Label, ($Data | ConvertTo-Json -Compress -Depth 6))
    }
    return [string]$id
}

function Invoke-Sql {
    param([string]$Sql)
    $out = $Sql | docker compose exec -T $DbService psql -U $DbUser -d $DbName -v ON_ERROR_STOP=1 -t -A 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Fail ("SQL failed (exit {0}): {1}" -f $LASTEXITCODE, $out)
    }
    return (($out | Out-String).Trim())
}

function Get-SqlInt64 {
    param([string]$Sql)
    $raw = (Invoke-Sql -Sql $Sql).Trim()
    if ([string]::IsNullOrWhiteSpace($raw)) { return [int64]0 }
    $n = [int64]0
    if (-not [int64]::TryParse($raw, [ref]$n)) { return [int64]0 }
    return $n
}

function Test-IsSeedPhone {
    param([string]$Phone)
    if (-not $Phone) { return $true }
    $d = ($Phone -replace '\D', '')
    return ($d -eq "22670000000" -or $d -eq "70000000" -or $Phone -match "70000000")
}

function Complete-ReleasedOrder {
    param(
        [string]$Label,
        [string]$ProductName
    )

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/products" -Headers $script:State.MerchantHeaders -Body @{
        name        = $ProductName
        description = "E2E chain $Label"
        price_cents = $ProductPriceCents
        stock       = 100
    }
    Assert-Ok -Res $res -Label ("Product {0}" -f $Label) -Codes @(200, 201)
    $productId = Get-EntityId $res.Data "product"
    $sqlStock = "UPDATE products SET stock = 100, updated_at = NOW() WHERE id = '$productId'::uuid;"
    Invoke-Sql -Sql $sqlStock | Out-Null
    Write-Ok ("Product {0} = {1} stock=100" -f $Label, $productId)

    $rnd = Get-Random -Minimum 100000 -Maximum 999999
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" -Headers $script:State.MerchantHeaders -Body @{
        first_name = "Client"
        last_name  = "Chain$Label"
        email      = "client.chain.$Label.$Timestamp@test.com"
        phone      = $CustomerPhone
    }
    if (-not $res.Success) {
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" -Headers $script:State.MerchantHeaders -Body @{
            first_name   = "Client"
            last_name    = "Chain$Label"
            email        = "client.chain.$Label.$Timestamp@test.com"
            phone_number = $CustomerPhone
        }
    }
    Assert-Ok -Res $res -Label ("Customer {0}" -f $Label) -Codes @(200, 201)
    $customerId = Get-EntityId $res.Data "customer"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders" -Headers $script:State.MerchantHeaders -Body @{
        customer_id    = $customerId
        payment_method = "mobile_money"
        items          = @(@{ product_id = $productId; quantity = 1 })
    }
    Assert-Ok -Res $res -Label ("Order {0}" -f $Label) -Codes @(200, 201)
    $orderId = Get-EntityId $res.Data "order"
    Write-Ok ("Order {0} = {1}" -f $Label, $orderId)

    $zoneRow = Invoke-Sql -Sql "SELECT id::text FROM delivery_zones WHERE zone_code = 'BF-OUAGA-URB' AND is_active = true LIMIT 1;"
    if ($zoneRow -match "[0-9a-fA-F-]{36}") {
        $zoneId = $Matches[0]
        Invoke-Sql -Sql "UPDATE orders SET delivery_zone_id = '$zoneId'::uuid WHERE id = '$orderId'::uuid;" | Out-Null
        Write-Ok ("Order {0} zone BF-OUAGA-URB" -f $Label)
    }
    else {
        Write-Warn ("Order {0}: zone BF-OUAGA-URB missing" -f $Label)
    }

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/pay" -Headers $script:State.MerchantHeaders -Body @{
        provider     = "yenga_pay"
        phone_number = $CustomerPhone
        description  = "E2E chain $Label order $orderId"
        metadata     = @{ flow = "indirect" }
    }
    Assert-Ok -Res $res -Label ("Pay {0}" -f $Label) -Codes @(200, 201)
    $paymentId = Get-Prop $res.Data @("payment_id", "id", "data.payment_id", "data.id")
    $providerRef = Get-Prop $res.Data @("provider_ref", "data.provider_ref")
    $checkout = Get-Prop $res.Data @(
        "redirect_url", "data.redirect_url",
        "checkout_url", "data.checkout_url",
        "payment_url", "data.payment_url",
        "metadata.payment_url", "data.metadata.payment_url"
    )
    if (-not $paymentId) {
        Write-Fail ("payment_id missing {0}" -f $Label)
    }
    Write-Ok ("Payment {0} = {1} ref={2}" -f $Label, $paymentId, $providerRef)

    if (-not $checkout) {
        Write-Fail ("No checkout URL for {0}" -f $Label)
    }

    Write-Host ""
    Write-Host "  ============================================================" -ForegroundColor Yellow
    Write-Host ("  PAIEMENT REEL Order {0}" -f $Label) -ForegroundColor Yellow
    Write-Host ("  {0}" -f $checkout) -ForegroundColor White
    Write-Host "  ============================================================" -ForegroundColor Yellow
    try { Start-Process $checkout } catch { Write-Warn "Start-Process failed - open URL manually" }

    $deadline  = (Get-Date).AddSeconds($PayInTimeoutSec)
    $payStatus = "processing"
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Seconds 5
        $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/payments/$paymentId" -Headers $script:State.MerchantHeaders
        $payStatus = Get-Prop $res.Data @("status", "data.status")
        $ts = Get-Date -Format "HH:mm:ss"
        Write-Host ("  ... {0} status = {1} ({2})" -f $Label, $payStatus, $ts) -ForegroundColor DarkGray
        if ($payStatus -eq "success" -or $payStatus -eq "failed" -or $payStatus -eq "cancelled") { break }
    }
    if ($payStatus -ne "success") {
        Write-Fail ("Pay-in {0} not success (status={1})" -f $Label, $payStatus)
    }
    Write-Ok ("Pay-in {0} success" -f $Label)

    $sqlCanal = "SELECT COALESCE(customer_phone,'') || '|' || COALESCE(metadata->>'customer_number','') || '|' || COALESCE(metadata->>'payment_source','') FROM payments WHERE id = '$paymentId'::uuid;"
    $canal = Invoke-Sql -Sql $sqlCanal
    $phonePart = ($canal -split '\|')[0]
    if (Test-IsSeedPhone -Phone $phonePart) {
        Write-Fail ("Canal still seed for {0}: {1}" -f $Label, $canal)
    }
    Write-Ok ("Canal {0}: {1}" -f $Label, $canal)

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/shipping" -Headers $script:State.MerchantHeaders -Body @{
        order_id        = $orderId
        proof_url       = "https://example.com/ship-$Label-$Timestamp.jpg"
        tracking_number = "TRK-$Label-$Timestamp"
        carrier         = "E2E"
        notes           = "shipped"
    }
    if (-not $res.Success) {
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/shipping-proof" -Headers $script:State.MerchantHeaders -Body @{
            tracking_number = "TRK-$Label-$Timestamp"
            carrier         = "E2E"
            shipped_at      = (Get-Date).ToUniversalTime().ToString("o")
        }
    }
    Assert-Ok -Res $res -Label ("Shipping {0}" -f $Label) -Codes @(200, 201)
    Write-Ok ("Shipping {0}" -f $Label)

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/delivery" -Headers $script:State.MerchantHeaders -Body @{
        order_id  = $orderId
        proof_url = "https://example.com/del-$Label-$Timestamp.jpg"
        signature = "SIG-$Label-$Timestamp"
        notes     = "delivered"
        rating    = 5
    }
    if (-not $res.Success) {
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/delivery-proof" -Headers $script:State.MerchantHeaders -Body @{
            recipient_name = "Client Chain"
            delivered_at   = (Get-Date).ToUniversalTime().ToString("o")
            notes          = "OK"
        }
    }
    Assert-Ok -Res $res -Label ("Delivery {0}" -f $Label) -Codes @(200, 201)
    Write-Ok ("Delivery {0}" -f $Label)

    $sqlBack = "UPDATE delivery_proofs SET delivery_date = NOW() - INTERVAL '$EligibilityBackdateDays days', updated_at = NOW() - INTERVAL '$EligibilityBackdateDays days' WHERE order_id = '$orderId'::uuid;"
    Invoke-Sql -Sql $sqlBack | Out-Null
    Write-Ok ("Backdate {0} {1}d" -f $Label, $EligibilityBackdateDays)

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/admin/scheduler/trigger-escrow-auto-release" -Headers $script:State.AdminHeaders
    Assert-Ok -Res $res -Label ("Scheduler {0}" -f $Label) -Codes @(200, 202)
    Write-Ok ("Scheduler {0} HTTP {1}" -f $Label, $res.StatusCode)
    Start-Sleep -Seconds $SchedulerWaitSec

    $esc = Invoke-Sql -Sql "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    if ($esc -notmatch "released") {
        Write-Fail ("Escrow {0} not released: {1}" -f $Label, $esc)
    }
    Write-Ok ("Escrow {0} = released" -f $Label)

    return @{ OrderId = $orderId; PaymentId = $paymentId }
}

Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Clawback -> Debt residual -> Sweep" -ForegroundColor Magenta
Write-Host (" BaseUrl: {0} | WithdrawPartial={1}" -f $BaseUrl, $WithdrawPartialCents) -ForegroundColor DarkGray
Write-Host " 2x REAL pay-in | NO SQL force phone/debt" -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # 01 Health
    Write-Step -N "01/11" -Msg "Health"
    $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health/live"
    if ($res.StatusCode -ne 200 -and $res.StatusCode -ne 404) {
        $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health"
    }
    if ($res.StatusCode -ge 500) { Write-Fail "API down" }
    Write-Ok ("API live HTTP {0}" -f $res.StatusCode)

    # 02 Admin
    Write-Step -N "02/11" -Msg "Admin login"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email    = $AdminEmail
        password = $AdminPassword
    }
    Assert-Ok -Res $res -Label "Admin login"
    $adminToken = Get-AuthToken $res.Data
    $script:State.AdminHeaders = @{
        Authorization  = "Bearer $adminToken"
        "Content-Type" = "application/json"
    }
    Write-Ok "Admin OK"

    # 03 Merchant + shop + KYC
    Write-Step -N "03/11" -Msg "Merchant + shop + KYC"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/register" -Body @{
        email      = $MerchantEmail
        password   = $MerchantPassword
        first_name = "Chain"
        last_name  = "Debt"
    }
    Assert-Ok -Res $res -Label "Register" -Codes @(200, 201, 409)
    Write-Ok ("Register HTTP {0}" -f $res.StatusCode)

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email    = $MerchantEmail
        password = $MerchantPassword
    }
    Assert-Ok -Res $res -Label "Merchant login"
    $mToken = Get-AuthToken $res.Data

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/shops" -Headers @{
        Authorization  = "Bearer $mToken"
        "Content-Type" = "application/json"
    } -Body @{
        name = $ShopName
        slug = $ShopSlug
    }
    Assert-Ok -Res $res -Label "Create shop" -Codes @(200, 201)
    $shopId = Get-EntityId $res.Data "shop"
    $script:State.ShopId = $shopId
    $script:State.MerchantHeaders = @{
        Authorization  = "Bearer $mToken"
        "Content-Type" = "application/json"
        "X-Shop-Slug"  = $ShopSlug
    }
    Write-Ok ("Shop {0}" -f $shopId)

    Invoke-Sql -Sql "UPDATE shops SET kyc_status = 'verified', updated_at = NOW() WHERE id = '$shopId'::uuid;" | Out-Null
    Write-Ok "KYC verified"

    # 04 Order A + release
    Write-Step -N "04/11" -Msg "Order A + REAL pay-in + auto-release"
    $flowA = Complete-ReleasedOrder -Label "A" -ProductName "Chain Product A $Timestamp"
    $script:State.OrderIdA   = $flowA.OrderId
    $script:State.PaymentIdA = $flowA.PaymentId

    $balA = Get-SqlInt64 -Sql "SELECT COALESCE(balance_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $debtA = Get-SqlInt64 -Sql "SELECT COALESCE(debt_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    Write-Host ("  Wallet after A: balance={0} debt={1}" -f $balA, $debtA) -ForegroundColor White
    if ($balA -lt 10000) {
        Write-Fail ("Wallet after A too low: {0}" -f $balA)
    }
    Write-Ok ("Wallet seeded balance={0}" -f $balA)

    # 05 Partial withdrawal
    Write-Step -N "05/11" -Msg ("Partial withdrawal {0} cents" -f $WithdrawPartialCents)
    if ($balA -le $WithdrawPartialCents) {
        Write-Fail ("Cannot withdraw {0}: balance only {1}" -f $WithdrawPartialCents, $balA)
    }
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/withdrawals" -Headers $script:State.MerchantHeaders -Body @{
        amount_cents       = $WithdrawPartialCents
        payment_method     = "ORANGE_MONEY"
        destination_number = $WithdrawPhone
        destination_name   = "Chain Test"
        description        = "E2E partial withdraw $Timestamp"
    }
    Assert-Ok -Res $res -Label "Partial withdrawal" -Codes @(200, 201)
    Write-Ok ("Withdrawal accepted HTTP {0}" -f $res.StatusCode)

    $balAfterWd = Get-SqlInt64 -Sql "SELECT COALESCE(balance_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $debtAfterWd = Get-SqlInt64 -Sql "SELECT COALESCE(debt_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    Write-Host ("  Wallet after withdraw: balance={0} debt={1}" -f $balAfterWd, $debtAfterWd) -ForegroundColor White
    if ($balAfterWd -ge $balA) {
        Write-Fail ("Balance did not decrease after withdraw ({0} -> {1})" -f $balA, $balAfterWd)
    }
    Write-Ok ("Balance reduced {0} -> {1}" -f $balA, $balAfterWd)

    # 06 Dispute + customer_wins (creates residual debt)
    Write-Step -N "06/11" -Msg "Open dispute A + resolve customer_wins (expect debt)"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$($script:State.OrderIdA)/dispute" -Headers $script:State.MerchantHeaders -Body @{
        reason = "E2E chain clawback residual debt after partial withdraw $Timestamp"
    }
    Assert-Ok -Res $res -Label "Open dispute" -Codes @(200, 201)
    $disputeId = Get-Prop $res.Data @("dispute.id", "data.dispute.id", "id", "data.id")
    if (-not $disputeId) {
        Write-Fail ("dispute id missing: {0}" -f ($res.Data | ConvertTo-Json -Compress -Depth 6))
    }
    $script:State.DisputeId = [string]$disputeId
    Write-Ok ("Dispute opened {0}" -f $disputeId)

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/admin/disputes/$disputeId/resolve" -Headers $script:State.AdminHeaders -Body @{
        resolution = "customer_wins"
        notes      = "E2E chain residual debt $Timestamp"
    }
    Assert-Ok -Res $res -Label "Resolve customer_wins" -Codes @(200, 201)
    Write-Ok "Resolve customer_wins OK"
    Start-Sleep -Seconds 2

    $balAfterClaw = Get-SqlInt64 -Sql "SELECT COALESCE(balance_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $debtAfterClaw = Get-SqlInt64 -Sql "SELECT COALESCE(debt_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $sqlClaw = "SELECT transaction_type || '|' || amount_cents FROM wallet_transactions WHERE shop_id = '$shopId'::uuid AND (transaction_type IN ('clawback','debt_add') OR reference_type IN ('dispute_clawback','dispute_debt_add')) ORDER BY created_at DESC LIMIT 5;"
    $clawRows = Invoke-Sql -Sql $sqlClaw
    Write-Host ("  Wallet after clawback: balance={0} debt={1}" -f $balAfterClaw, $debtAfterClaw) -ForegroundColor White
    Write-Host ("  Ledger: {0}" -f $clawRows) -ForegroundColor DarkGray

    if ($debtAfterClaw -le 0) {
        Write-Fail ("Expected debt_cents > 0 after clawback > remaining balance. debt={0} bal={1} ledger={2}" -f $debtAfterClaw, $balAfterClaw, $clawRows)
    }
    Write-Ok ("Residual debt created debt_cents={0}" -f $debtAfterClaw)

    if ("$clawRows" -notmatch "clawback|debt_add") {
        Write-Warn "No clawback/debt_add row visible in ledger sample"
    }
    else {
        Write-Ok "Ledger clawback/debt_add present"
    }

    # 07 Withdrawal blocked while debt
    Write-Step -N "07/11" -Msg "Withdrawal blocked while debt > 0"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/withdrawals" -Headers $script:State.MerchantHeaders -Body @{
        amount_cents       = 10000
        payment_method     = "ORANGE_MONEY"
        destination_number = $WithdrawPhone
    }
    if ($res.Success) {
        Write-Fail ("Withdrawal accepted while debt={0}" -f $debtAfterClaw)
    }
    Write-Ok ("Withdrawal blocked debt={0} HTTP {1}" -f $debtAfterClaw, $res.StatusCode)

    # 08 Order B + release (must sweep debt)
    Write-Step -N "08/11" -Msg "Order B + REAL pay-in + auto-release (expect debt sweep)"
    $balBeforeB  = Get-SqlInt64 -Sql "SELECT COALESCE(balance_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $debtBeforeB = Get-SqlInt64 -Sql "SELECT COALESCE(debt_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"

    $flowB = Complete-ReleasedOrder -Label "B" -ProductName "Chain Product B $Timestamp"
    $script:State.OrderIdB   = $flowB.OrderId
    $script:State.PaymentIdB = $flowB.PaymentId

    $balAfterB  = Get-SqlInt64 -Sql "SELECT COALESCE(balance_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $debtAfterB = Get-SqlInt64 -Sql "SELECT COALESCE(debt_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $deltaBal   = $balAfterB - $balBeforeB
    $deltaDebt  = $debtBeforeB - $debtAfterB

    Write-Host ("  balance: {0} -> {1} delta={2}" -f $balBeforeB, $balAfterB, $deltaBal) -ForegroundColor White
    Write-Host ("  debt   : {0} -> {1} swept={2}" -f $debtBeforeB, $debtAfterB, $deltaDebt) -ForegroundColor White

    if ($deltaDebt -le 0 -and $debtBeforeB -gt 0) {
        Write-Fail "NO debt reduction after Order B release - CreditWithDebtSweep missing?"
    }
    Write-Ok ("Debt reduced by {0}" -f $deltaDebt)

    if ($deltaBal -ge $ExpectedNetRelease -and $debtBeforeB -gt 0) {
        Write-Fail ("Balance rose by full net {0} while debt existed - sweep not applied" -f $deltaBal)
    }
    Write-Ok ("Net credit consistent with sweep deltaBal={0}" -f $deltaBal)

    # 09 debt_sweep ledger
    Write-Step -N "09/11" -Msg "Assert debt_sweep ledger"
    $sqlSweep = "SELECT transaction_type || '|' || amount_cents || '|' || COALESCE(reference_type,'') FROM wallet_transactions WHERE shop_id = '$shopId'::uuid AND transaction_type = 'debt_sweep' ORDER BY created_at DESC LIMIT 3;"
    $sweepRows = Invoke-Sql -Sql $sqlSweep
    Write-Host ("  debt_sweep: {0}" -f $sweepRows) -ForegroundColor DarkGray
    if ($sweepRows -notmatch "debt_sweep") {
        Write-Fail "No debt_sweep ledger row"
    }
    Write-Ok "Ledger debt_sweep present"

    # 10 Final withdrawal gate
    Write-Step -N "10/11" -Msg "Final withdrawal gate"
    $debtNow = Get-SqlInt64 -Sql "SELECT COALESCE(debt_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/withdrawals" -Headers $script:State.MerchantHeaders -Body @{
        amount_cents       = 5000
        payment_method     = "ORANGE_MONEY"
        destination_number = $WithdrawPhone
    }
    if ($debtNow -gt 0) {
        if ($res.Success) {
            Write-Fail ("Withdrawal accepted while residual debt={0}" -f $debtNow)
        }
        Write-Ok ("Still blocked debt={0} HTTP {1}" -f $debtNow, $res.StatusCode)
    }
    else {
        Write-Ok "Debt fully swept - withdrawal gate not re-tested"
    }

    # 11 Summary
    Write-Step -N "11/11" -Msg "Summary"
    Write-Host ("  Shop      : {0} ({1})" -f $shopId, $ShopSlug) -ForegroundColor White
    Write-Host ("  Order A   : {0}" -f $script:State.OrderIdA) -ForegroundColor White
    Write-Host ("  Order B   : {0}" -f $script:State.OrderIdB) -ForegroundColor White
    Write-Host ("  Dispute   : {0}" -f $script:State.DisputeId) -ForegroundColor White
    Write-Host ("  After A   : bal={0} debt={1}" -f $balA, $debtA) -ForegroundColor White
    Write-Host ("  After WD  : bal={0}" -f $balAfterWd) -ForegroundColor White
    Write-Host ("  After claw: bal={0} debt={1}" -f $balAfterClaw, $debtAfterClaw) -ForegroundColor White
    Write-Host ("  After B   : bal={0} debt={1}" -f $balAfterB, $debtAfterB) -ForegroundColor White
    Write-Host ("  WebhookSecret len: {0}" -f $WebhookSecret.Length) -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Logs:" -ForegroundColor Yellow
    Write-Host '  docker compose logs goshop 2>&1 | Select-String -Pattern "ClawbackToDebt|debt_sweep|CreditWithDebtSweep|customer_wins"' -ForegroundColor DarkGray

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $color = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    Write-Host (" RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed) -ForegroundColor $color
    Write-Host "================================================================" -ForegroundColor Magenta

    if ($script:Failed -eq 0) {
        Write-Host ""
        Write-Host "  CHAIN CLAWBACK -> DEBT -> SWEEP E2E VERT" -ForegroundColor Green
    }
}
catch {
    Write-Host ""
    Write-Host ("[CRITICAL FAILURE] {0}" -f $_.Exception.Message) -ForegroundColor Red
    exit 1
}

Write-Host ""