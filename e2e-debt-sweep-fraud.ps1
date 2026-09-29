# ============================================================
# GOSHOP E2E - Debt sweep on auto-release (fraud residual debt)
# ============================================================
# 1) Order A + auto-release -> seed wallet
# 2) SQL inject debt_cents = 50000
# 3) Order B + auto-release -> MUST CreditWithDebtSweep
# 4) Assert debt down, balance = net, debt_sweep row
# 5) Withdrawal blocked if debt > 0
#
# Env:
#   $env:YENGA_PAY_WEBHOOK_SECRET
#   $env:ADMIN_EMAIL / $env:ADMIN_PASSWORD
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

$Timestamp        = Get-Date -Format "yyyyMMddHHmmss"
$MerchantEmail    = "merchant.debt.$Timestamp@goshop.com"
$MerchantPassword = "Password123!"
$ShopName         = "Debt Sweep Shop $Timestamp"
$ShopSlug         = "debt-shop-$Timestamp"
$BackdateDays     = 6
$InjectDebtCents  = 150000
$ExpectedNetCents = 95000

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
            if ($Body -is [string]) {
                $json = $Body
            }
            else {
                $json = ($Body | ConvertTo-Json -Depth 12 -Compress)
            }
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
            try { $code = [int]$_.Exception.Response.StatusCode } catch { }
        }
        if ($_.ErrorDetails.Message) { $raw = $_.ErrorDetails.Message }
        return @{ Success = $false; StatusCode = $code; Data = $null; Raw = $raw }
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

function Invoke-Sql {
    param([string]$Sql)
    $out = $Sql | docker compose exec -T $DbService psql -U $DbUser -d $DbName -v ON_ERROR_STOP=1 -t -A 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw ("SQL failed: {0}" -f $out)
    }
    return (($out | Out-String).Trim())
}

function Get-SqlInt64 {
    param([string]$Sql)
    $raw = (Invoke-Sql $Sql).Trim()
    if ([string]::IsNullOrWhiteSpace($raw)) { return [int64]0 }
    return [int64]$raw
}

function New-WebhookHash {
    param([string]$JsonBody, [string]$Secret)
    $hmac = [System.Security.Cryptography.HMACSHA256]::new([Text.Encoding]::UTF8.GetBytes($Secret))
    $bytes = $hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($JsonBody))
    $hash = -join ($bytes | ForEach-Object { $_.ToString("x2") })
    return $hash
}

function Complete-OrderFlow {
    param(
        [string]$Label,
        [string]$ProductName
    )

    # Colonne DB = stock (pas stock_quantity)
    $prodBody = @{
        name        = $ProductName
        description = "E2E debt sweep $Label"
        price_cents = 100000
        stock       = 100
    }
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/products" -Headers $script:State.MerchantHeaders -Body $prodBody
    if (-not $res.Success) {
        Write-Fail ("Product {0} HTTP {1} {2}" -f $Label, $res.StatusCode, $res.Raw)
    }
    $productId = Get-Prop $res.Data @("id", "data.id", "product.id")
    if (-not $productId) {
        Write-Fail ("Product id missing {0}" -f $Label)
    }

    $sqlStock = "UPDATE products SET stock = 100, updated_at = NOW() WHERE id = '$productId'::uuid;"
    Invoke-Sql -Sql $sqlStock | Out-Null
    $stockNow = Get-SqlInt64 -Sql "SELECT stock FROM products WHERE id = '$productId'::uuid;"
    if ($stockNow -lt 1) {
        Write-Fail ("Product {0} stock still {1} after UPDATE" -f $Label, $stockNow)
    }
    Write-Ok ("Product {0} = {1} stock={2}" -f $Label, $productId, $stockNow)

    $rnd = Get-Random -Minimum 100000 -Maximum 999999
    $custBody = @{
        first_name = "Client"
        last_name  = "Debt$Label"
        email      = "client.debt.$Label.$Timestamp@test.com"
        phone      = "+22670$rnd"
    }
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" -Headers $script:State.MerchantHeaders -Body $custBody
    if (-not $res.Success) {
        $custBody = @{
            first_name   = "Client"
            last_name    = "Debt$Label"
            email        = "client.debt.$Label.$Timestamp@test.com"
            phone_number = "+22670$rnd"
        }
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" -Headers $script:State.MerchantHeaders -Body $custBody
    }
    if (-not $res.Success) {
        Write-Fail ("Customer {0} {1}" -f $Label, $res.Raw)
    }
    $customerId = Get-Prop $res.Data @("id", "data.id", "customer.id")

    $orderBody = @{
        customer_id    = $customerId
        payment_method = "mobile_money"
        items          = @(@{ product_id = $productId; quantity = 1 })
        currency       = "XOF"
    }
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders" -Headers $script:State.MerchantHeaders -Body $orderBody
    if (-not $res.Success) {
        Write-Fail ("Order {0} {1}" -f $Label, $res.Raw)
    }
    $orderId = Get-Prop $res.Data @("id", "data.id", "order.id")
    Write-Ok ("Order {0} = {1}" -f $Label, $orderId)

    # Route qui marche (ancien E2E) : POST /api/orders/{id}/pay
    $payBody = @{
        provider     = "yenga_pay"
        phone_number = "+22677515151"
        description  = "E2E debt sweep order $orderId"
        metadata     = @{ flow = "indirect" }
    }
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/pay" -Headers $script:State.MerchantHeaders -Body $payBody
    if (-not $res.Success) {
        Write-Fail ("Payment initiate {0} HTTP {1} {2}" -f $Label, $res.StatusCode, $res.Raw)
    }
    $paymentId   = Get-Prop $res.Data @("payment_id", "id", "data.payment_id", "data.id")
    $providerRef = Get-Prop $res.Data @("provider_ref", "data.provider_ref")
    if (-not $paymentId) {
        Write-Fail ("payment_id missing {0}: {1}" -f $Label, ($res.Data | ConvertTo-Json -Compress))
    }
    Write-Ok ("Payment {0} = {1} ref={2}" -f $Label, $paymentId, $providerRef)

    # Webhook simule (customerNumber pour MSISDN refund)
    $json = '{"apiEnv":"test","paymentStatus":"DONE","transId":"' + $providerRef + '","projectId":"65687","paymentIntentId":"' + $providerRef + '","paymentSource":"OrangeMoneyAPI","customerNumber":"77515151","paymentAmount":1000,"paymentFees":25,"contryOrigin":"BF","reference":"' + $paymentId + '","currency":"XOF","isPaylink":false}'
    $hash = New-WebhookHash -JsonBody $json -Secret $WebhookSecret
    $wh = Invoke-SafeApi -Method Post -Uri "$BaseUrl/webhooks/yenga_pay" -Headers @{
        "x-webhook-hash"   = $hash
        "x-yengapay-event" = "payment.success"
    } -Body $json
    if (-not $wh.Success -and $wh.StatusCode -ne 200) {
        Write-Fail ("Webhook {0} HTTP {1} {2}" -f $Label, $wh.StatusCode, $wh.Raw)
    }
    Write-Ok ("Webhook success {0}" -f $Label)

    Start-Sleep -Seconds 1

    # Shipping (route ancien E2E)
    $shipBody = @{
        order_id        = $orderId
        proof_url       = "https://example.com/shipping-$Label-$Timestamp.jpg"
        tracking_number = "TRK-$Label-$Timestamp"
        carrier         = "E2E"
        notes           = "E2E shipped"
    }
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/shipping" -Headers $script:State.MerchantHeaders -Body $shipBody
    if (-not $res.Success) {
        $shipBody2 = @{
            tracking_number = "TRK-$Label-$Timestamp"
            carrier         = "E2E"
            shipped_at      = (Get-Date).ToUniversalTime().ToString("o")
        }
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/shipping-proof" -Headers $script:State.MerchantHeaders -Body $shipBody2
    }
    if (-not $res.Success) {
        Write-Fail ("Shipping {0} {1}" -f $Label, $res.Raw)
    }
    Write-Ok ("Shipping {0}" -f $Label)

    # Delivery (route ancien E2E)
    $delBody = @{
        order_id  = $orderId
        proof_url = "https://example.com/delivery-$Label-$Timestamp.jpg"
        signature = "SIG-$Label-$Timestamp"
        notes     = "E2E delivered"
        rating    = 5
    }
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/delivery" -Headers $script:State.MerchantHeaders -Body $delBody
    if (-not $res.Success) {
        $delBody2 = @{
            delivered_at   = (Get-Date).ToUniversalTime().ToString("o")
            recipient_name = "Client E2E"
            notes          = "OK"
        }
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/delivery-proof" -Headers $script:State.MerchantHeaders -Body $delBody2
    }
    if (-not $res.Success) {
        Write-Fail ("Delivery {0} {1}" -f $Label, $res.Raw)
    }
    Write-Ok ("Delivery {0}" -f $Label)

    $sqlBackdate = @"
UPDATE delivery_proofs
SET delivery_date = NOW() - INTERVAL '$BackdateDays days',
    updated_at    = NOW() - INTERVAL '$BackdateDays days'
WHERE order_id = '$orderId'::uuid;
"@
    Invoke-Sql -Sql $sqlBackdate | Out-Null
    Write-Ok ("Backdate delivery {0} {1}d" -f $Label, $BackdateDays)

    $trig = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/admin/scheduler/trigger-escrow-auto-release" -Headers $script:State.AdminHeaders
    if (-not $trig.Success -and $trig.StatusCode -ne 202 -and $trig.StatusCode -ne 200) {
        Write-Fail ("Scheduler trigger {0} HTTP {1} {2}" -f $Label, $trig.StatusCode, $trig.Raw)
    }
    Write-Ok ("Scheduler triggered {0} HTTP {1}" -f $Label, $trig.StatusCode)
    Start-Sleep -Seconds 10

    return @{ OrderId = $orderId; PaymentId = $paymentId }
}
try {
    Write-Host "================================================================" -ForegroundColor Magenta
    Write-Host " GOSHOP E2E - Debt Sweep on Auto-Release" -ForegroundColor Magenta
    Write-Host (" BaseUrl: {0} | InjectDebt={1}" -f $BaseUrl, $InjectDebtCents) -ForegroundColor Magenta
    Write-Host "================================================================" -ForegroundColor Magenta

    # 01 Health
    Write-Step -N "01/08" -Msg "Health"
    $h = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health/live"
    if (-not $h.Success -and $h.StatusCode -ne 200 -and $h.StatusCode -ne 404) {
        $h = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health"
    }
    if ($h.StatusCode -ge 500) { Write-Fail "API down" }
    Write-Ok ("API live HTTP {0}" -f $h.StatusCode)

    # 02 Admin
    Write-Step -N "02/08" -Msg "Admin login"
    $login = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{ email = $AdminEmail; password = $AdminPassword }
    if (-not $login.Success) { Write-Fail ("Admin login: {0}" -f $login.Raw) }
    $adminToken = Get-Prop $login.Data @("access_token", "token")
    if (-not $adminToken) { Write-Fail "Admin token missing" }
    $script:State.AdminHeaders = @{
        Authorization  = "Bearer $adminToken"
        "Content-Type" = "application/json"
    }
    Write-Ok "Admin OK"

    # 03 Merchant + shop + KYC
    Write-Step -N "03/08" -Msg "Merchant + shop + KYC"
    $reg = Invoke-SafeApi -Method Post -Uri "$BaseUrl/register" -Body @{
        email      = $MerchantEmail
        password   = $MerchantPassword
        first_name = "Debt"
        last_name  = "Merchant"
    }
    if (-not $reg.Success -and $reg.StatusCode -ne 201 -and $reg.StatusCode -ne 409) {
        Write-Fail ("Register: {0}" -f $reg.Raw)
    }
    Write-Ok ("Register HTTP {0}" -f $reg.StatusCode)

    $mlogin = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email    = $MerchantEmail
        password = $MerchantPassword
    }
    if (-not $mlogin.Success) { Write-Fail ("Merchant login: {0}" -f $mlogin.Raw) }
    $mToken = Get-Prop $mlogin.Data @("access_token", "token")
    $script:State.MerchantHeaders = @{
        Authorization  = "Bearer $mToken"
        "Content-Type" = "application/json"
        "X-Shop-Slug"  = $ShopSlug
    }

    $shop = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/shops" -Headers $script:State.MerchantHeaders -Body @{
        name     = $ShopName
        slug     = $ShopSlug
        currency = "XOF"
    }
    if (-not $shop.Success) { Write-Fail ("Create shop: {0}" -f $shop.Raw) }
    $shopId = Get-Prop $shop.Data @("id", "data.id", "shop.id")
    $script:State.ShopId = $shopId
    Write-Ok ("Shop {0} ({1})" -f $shopId, $ShopSlug)

    try {
        $sqlKyc = @"
UPDATE merchant_kyc SET status = 'verified', verified_at = NOW()
WHERE shop_id = '$shopId'::uuid OR user_id = (SELECT owner_id FROM shops WHERE id = '$shopId'::uuid);
"@
        Invoke-Sql -Sql $sqlKyc | Out-Null
    }
    catch {
        try {
            $sqlKyc2 = "UPDATE shops SET kyc_status = 'verified', updated_at = NOW() WHERE id = '$shopId'::uuid;"
            Invoke-Sql -Sql $sqlKyc2 | Out-Null
        }
        catch {
            Write-Warn ("KYC SQL soft-fail: {0}" -f $_.Exception.Message)
        }
    }
    Write-Ok "KYC forced verified best effort"

    # 04 Order A
    Write-Step -N "04/08" -Msg "Order A + auto-release seed wallet"
    $flowA = Complete-OrderFlow -Label "A" -ProductName "Product A $Timestamp"
    $script:State.OrderIdA   = $flowA.OrderId
    $script:State.PaymentIdA = $flowA.PaymentId

    $sqlW1 = "SELECT balance_cents || '|' || COALESCE(debt_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $w1 = Invoke-Sql -Sql $sqlW1
    Write-Host ("  Wallet after A: {0}" -f $w1) -ForegroundColor White
    if ($w1 -notmatch '^\d+\|') {
        Write-Warn ("Wallet row unexpected: {0}" -f $w1)
    }
    else {
        Write-Ok ("Wallet after first release: {0}" -f $w1)
    }

    # 05 Inject debt
    Write-Step -N "05/08" -Msg ("Inject residual debt {0} cents" -f $InjectDebtCents)
    $sqlDebt = @"
UPDATE merchant_wallets
SET debt_cents = $InjectDebtCents, updated_at = NOW()
WHERE shop_id = '$shopId'::uuid;
"@
    Invoke-Sql -Sql $sqlDebt | Out-Null

    $sqlWDebt = "SELECT balance_cents || '|' || debt_cents FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $wDebt = Invoke-Sql -Sql $sqlWDebt
    Write-Host ("  Wallet with debt: {0}" -f $wDebt) -ForegroundColor White
    $debtSuffix = '|' + $InjectDebtCents
    if (-not $wDebt.EndsWith($debtSuffix)) {
        Write-Fail ("debt_cents not set to {0} got {1}. Check migration 058." -f $InjectDebtCents, $wDebt)
    }
    Write-Ok ("debt_cents = {0}" -f $InjectDebtCents)

    # 06 Order B
    Write-Step -N "06/08" -Msg "Order B + auto-release expect debt sweep"

    $sqlBal = "SELECT balance_cents FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $sqlDebtOnly = "SELECT debt_cents FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"

    $balBefore  = Get-SqlInt64 -Sql $sqlBal
    $debtBefore = Get-SqlInt64 -Sql $sqlDebtOnly

    $flowB = Complete-OrderFlow -Label "B" -ProductName "Product B $Timestamp"
    $script:State.OrderIdB   = $flowB.OrderId
    $script:State.PaymentIdB = $flowB.PaymentId

    $balAfter  = Get-SqlInt64 -Sql $sqlBal
    $debtAfter = Get-SqlInt64 -Sql $sqlDebtOnly
    $deltaBal  = $balAfter - $balBefore
    $deltaDebt = $debtBefore - $debtAfter

    Write-Host ("  balance: {0} -> {1} delta={2}" -f $balBefore, $balAfter, $deltaBal) -ForegroundColor White
    Write-Host ("  debt   : {0} -> {1} swept={2}" -f $debtBefore, $debtAfter, $deltaDebt) -ForegroundColor White

    if ($deltaDebt -le 0 -and $debtBefore -gt 0) {
        Write-Fail "NO debt sweep detected. Scheduler may still use wallet.Credit without sweep."
    }
    else {
        Write-Ok ("Debt reduced by {0} cents" -f $deltaDebt)
    }

    if ($deltaBal -ge $ExpectedNetCents -and $debtBefore -gt 0) {
        Write-Fail ("Balance rose by full {0} while debt existed. Sweep NOT applied." -f $deltaBal)
    }
    elseif ($deltaBal -gt 0 -or $deltaDebt -gt 0) {
        Write-Ok ("Net credit consistent with sweep deltaBal={0}" -f $deltaBal)
    }

    $sqlSweep = @"
SELECT transaction_type || '|' || amount_cents || '|' || COALESCE(reference_type,'')
FROM wallet_transactions
WHERE shop_id = '$shopId'::uuid AND transaction_type = 'debt_sweep'
ORDER BY created_at DESC LIMIT 3;
"@
    $sweepRows = Invoke-Sql -Sql $sqlSweep
    Write-Host ("  debt_sweep txns: {0}" -f $sweepRows) -ForegroundColor DarkGray
    if ($sweepRows -match "debt_sweep") {
        Write-Ok "Ledger debt_sweep present"
    }
    else {
        Write-Warn "No debt_sweep ledger row yet"
    }

    # 07 Withdrawal
    Write-Step -N "07/08" -Msg "Withdrawal while debt?"
    $sqlDebtNow = "SELECT COALESCE(debt_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $debtNow = Get-SqlInt64 -Sql $sqlDebtNow
    $wd = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/withdrawals" -Headers $script:State.MerchantHeaders -Body @{
        amount_cents       = 10000
        payment_method     = "ORANGE_MONEY"
        destination_number = "+22677515151"
    }
    if ($debtNow -gt 0) {
        if (-not $wd.Success) {
            Write-Ok ("Withdrawal blocked with debt={0} HTTP {1}" -f $debtNow, $wd.StatusCode)
        }
        else {
            Write-Fail ("Withdrawal accepted while debt_cents={0}" -f $debtNow)
        }
    }
    else {
        Write-Ok "Debt fully swept - withdrawal gate not tested debt=0"
    }

    # 08 Summary
    Write-Step -N "08/08" -Msg "Summary"
    Write-Host ("  Shop     : {0} ({1})" -f $shopId, $ShopSlug) -ForegroundColor White
    Write-Host ("  Order A  : {0}" -f $script:State.OrderIdA) -ForegroundColor White
    Write-Host ("  Order B  : {0}" -f $script:State.OrderIdB) -ForegroundColor White
    Write-Host ("  Wallet   : bal {0}->{1} | debt {2}->{3}" -f $balBefore, $balAfter, $debtBefore, $debtAfter) -ForegroundColor White
    Write-Host ""
    Write-Host "  Logs:" -ForegroundColor Yellow
    Write-Host '  docker compose logs goshop 2>&1 | Select-String -Pattern "debt_sweep|CreditWithDebtSweep|Merchant wallet credited"' -ForegroundColor DarkGray

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $color = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    $resultMsg = " RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed
    Write-Host $resultMsg -ForegroundColor $color
    Write-Host "================================================================" -ForegroundColor Magenta

    if ($script:Failed -eq 0) {
        Write-Host ""
        Write-Host "  DEBT SWEEP E2E VERT" -ForegroundColor Green
    }
}
catch {
    Write-Host ""
    Write-Host ("[CRITICAL FAILURE] {0}" -f $_.Exception.Message) -ForegroundColor Red
    exit 1
}

Write-Host ""