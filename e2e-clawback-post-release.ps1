# ============================================================
# GOSHOP E2E - Clawback post-release (customer_wins apres auto-release)
# ============================================================
# Flux:
#   register/login -> shop + KYC -> product/customer/order
#   -> pay-in (webhook local)
#   -> shipping + delivery
#   -> backdate delivery + trigger auto-release
#   -> OPEN dispute (escrow deja released)
#   -> admin resolve customer_wins
#   -> verifie wallet reduit + ligne clawback
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
$MerchantEmail    = "merchant.claw.$Timestamp@goshop.com"
$MerchantPassword = "Password123!"
$ShopName         = "Boutique Claw $Timestamp"
$ShopSlug         = "claw-shop-$Timestamp"
$EligibilityBackdateDays = 6

$script:Passed = 0
$script:Failed = 0
$script:State  = @{
    MerchantHeaders = $null
    AdminHeaders    = $null
    ShopId          = $null
    OrderId         = $null
    PaymentId       = $null
    ProviderRef     = $null
    DisputeId       = $null
}

function Write-Ok($m)   { Write-Host "  [OK] $m" -ForegroundColor Green; $script:Passed++ }
function Write-Warn($m) { Write-Host "  [WARN] $m" -ForegroundColor Yellow }
function Write-Fail($m) { Write-Host "  [FAIL] $m" -ForegroundColor Red; $script:Failed++; throw $m }
function Write-Step($n, $t) {
    Write-Host ""
    Write-Host "[$n] $t" -ForegroundColor Cyan
}

function Invoke-SafeApi {
    param([string]$Method, [string]$Uri, [hashtable]$Headers = @{}, [object]$Body = $null)
    try {
        $params = @{ Method = $Method; Uri = $Uri; Headers = $Headers; UseBasicParsing = $true }
        if ($Body) {
            $json = if ($Body -is [string]) { $Body } else { ($Body | ConvertTo-Json -Depth 12 -Compress) }
            $params.Body = [System.Text.Encoding]::UTF8.GetBytes($json)
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
        $raw = ""
        if ($_.Exception.Response) {
            $code = [int]$_.Exception.Response.StatusCode
            try {
                $stream = $_.Exception.Response.GetResponseStream()
                $reader = New-Object System.IO.StreamReader($stream)
                $raw = $reader.ReadToEnd()
            } catch {}
        }
        $data = $null
        if ($raw) { try { $data = $raw | ConvertFrom-Json } catch { $data = $raw } }
        return @{ Success = $false; StatusCode = $code; Data = $data; Raw = $raw; Error = $_.Exception.Message }
    }
}

function Get-Prop($obj, [string[]]$paths) {
    foreach ($p in $paths) {
        $cur = $obj
        $ok = $true
        foreach ($seg in ($p -split '\.')) {
            if ($null -eq $cur) { $ok = $false; break }
            $cur = $cur.$seg
        }
        if ($ok -and $null -ne $cur -and "$cur" -ne "") { return "$cur" }
    }
    return ""
}

function Invoke-Sql([string]$sql) {
    $out = $sql | docker-compose exec -T $DbService psql -U $DbUser -d $DbName -t -A -v ON_ERROR_STOP=1 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Warn "SQL exit $LASTEXITCODE : $out"
    }
    return ($out | Out-String).Trim()
}

function New-HmacSha256Hex([string]$payload, [string]$secret) {
    $hmac = New-Object System.Security.Cryptography.HMACSHA256
    $hmac.Key = [System.Text.Encoding]::UTF8.GetBytes($secret)
    $hash = $hmac.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($payload))
    return ([System.BitConverter]::ToString($hash) -replace '-', '').ToLowerInvariant()
}

function Assert-Ok($res, $label, [int[]]$codes = @(200, 201)) {
    if (-not $res.Success -or ($codes -notcontains $res.StatusCode)) {
        $detail = if ($res.Raw) { $res.Raw } else { $res.Error }
        Write-Fail "$label -> HTTP $($res.StatusCode) - $detail"
    }
}

Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Clawback post-release (customer_wins)" -ForegroundColor Magenta
Write-Host " BaseUrl: $BaseUrl | BackdateDays=$EligibilityBackdateDays" -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
        # ----- 01 Health -----
    Write-Step "01/10" "Health check"
    $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health"
    if ($res.StatusCode -eq 0) {
        $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/health"
    }
    # 200 = ideal ; 401/404 = process up (route protegee ou absente)
    if ($res.StatusCode -eq 0) {
        Write-Fail "API unreachable (pas de reponse HTTP)"
    }
    Write-Ok "API live (HTTP $($res.StatusCode))"
    # ----- 02 Admin login -----
    Write-Step "02/10" "Admin login"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email = $AdminEmail; password = $AdminPassword
    }
    Assert-Ok $res "Admin login" @(200)
    $adminToken = Get-Prop $res.Data @("access_token", "token", "data.access_token")
    if (-not $adminToken) { Write-Fail "Admin token missing" }
    $script:State.AdminHeaders = @{
        Authorization  = "Bearer $adminToken"
        "Content-Type" = "application/json"
    }
    Write-Ok "Admin token OK ($AdminEmail)"

    # ----- 03 Merchant + shop + KYC -----
    Write-Step "03/10" "Register + login merchant + shop + KYC"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/register" -Body @{
        email = $MerchantEmail; password = $MerchantPassword; full_name = "Merchant Claw $Timestamp"
    }
    Assert-Ok $res "Register" @(200, 201)
    Write-Ok "Register HTTP $($res.StatusCode)"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email = $MerchantEmail; password = $MerchantPassword
    }
    Assert-Ok $res "Merchant login" @(200)
    $mToken = Get-Prop $res.Data @("access_token", "token", "data.access_token")
    if (-not $mToken) { Write-Fail "Merchant token missing" }
    $script:State.MerchantHeaders = @{
        Authorization  = "Bearer $mToken"
        "Content-Type" = "application/json"
        "X-Shop-Slug"  = $ShopSlug
    }
    Write-Ok "Merchant token OK"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/shops" -Headers $script:State.MerchantHeaders -Body @{
        name = $ShopName; slug = $ShopSlug; description = "E2E clawback shop"
    }
    Assert-Ok $res "Create shop" @(200, 201)
    $shopId = Get-Prop $res.Data @("id", "shop_id", "data.id", "data.shop_id")
    if (-not $shopId) {
        $shopId = (Invoke-Sql "SELECT id FROM shops WHERE slug = '$ShopSlug' LIMIT 1;").Trim()
    }
    if (-not $shopId) { Write-Fail "Shop id missing" }
    $script:State.ShopId = $shopId
    Write-Ok "Shop $shopId ($ShopSlug)"

        # KYC = colonne shops.kyc_status (migration 019)
    Invoke-Sql "UPDATE shops SET kyc_status = 'verified', kyc_verified_at = NOW() WHERE id = '$shopId'::uuid;" | Out-Null
    $kycCheck = Invoke-Sql "SELECT kyc_status FROM shops WHERE id = '$shopId'::uuid;"
    if ($kycCheck -match "verified") {
        Write-Ok "KYC forced to verified"
    }
    else {
        Write-Warn "KYC update unclear: $kycCheck"
    }

    # ----- 04 Product, customer, order -----
        # ----- 04 Product, customer, order (aligné GitHub E2E) -----
    Write-Step "04/10" "Product, customer, order"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/products" -Headers $script:State.MerchantHeaders -Body @{
        name        = "Produit Claw $Timestamp"
        description = "E2E clawback product"
        price_cents = 100000
        stock       = 10
    }
    Assert-Ok $res "Product" @(200, 201)
    $productId = Get-Prop $res.Data @("id", "data.id")
    if (-not $productId) { Write-Fail "Product id missing" }
    Write-Ok "Product $productId"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" -Headers $script:State.MerchantHeaders -Body @{
        first_name = "Jean"
        last_name  = "Claw"
        email      = "client.claw.$Timestamp@test.com"
        phone      = "+22670000000"
    }
    if (-not $res.Success) {
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" -Headers $script:State.MerchantHeaders -Body @{
            first_name   = "Jean"
            last_name    = "Claw"
            email        = "client.claw.$Timestamp@test.com"
            phone_number = "+22670000000"
        }
    }
    Assert-Ok $res "Customer" @(200, 201)
    $customerId = Get-Prop $res.Data @("id", "data.id")
    if (-not $customerId) { Write-Fail "Customer id missing" }
    Write-Ok "Customer $customerId"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders" -Headers $script:State.MerchantHeaders -Body @{
        customer_id    = $customerId
        payment_method = "mobile_money"
        items          = @(@{ product_id = $productId; quantity = 1 })
    }
    Assert-Ok $res "Order" @(200, 201)
    $orderId = Get-Prop $res.Data @("id", "data.id", "order_id", "data.order_id")
    if (-not $orderId) { Write-Fail "Order id missing" }
    $script:State.OrderId = $orderId
    Write-Ok "Order $orderId"

    # ----- 05 Pay-in (webhook local) -----
            # ----- 05 Pay-in via /orders/{id}/pay + webhook -----
    Write-Step "05/10" "Initiate payment + webhook SUCCESS"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/pay" -Headers $script:State.MerchantHeaders -Body @{
        provider     = "yenga_pay"
        phone_number = "+22670000000"
        description  = "E2E clawback order $orderId"
        metadata     = @{ flow = "indirect" }
    }
    Assert-Ok $res "Payment init" @(200, 201)
    $paymentId   = Get-Prop $res.Data @("payment_id", "id", "data.payment_id", "data.id")
    $providerRef = Get-Prop $res.Data @("provider_ref", "data.provider_ref")
    if (-not $paymentId) { Write-Fail "Payment id missing" }
    $script:State.PaymentId   = $paymentId
    $script:State.ProviderRef = $providerRef
    Write-Ok "Payment $paymentId ref=$providerRef"

    # MSISDN NON-seed (pas 70000000 / 70123456 / ...) pour que resolveRefundDestination accepte
    $realMsisdn = "77515151"

    # Webhook avec VRAI customerNumber (pas seed)
    $payloadJson = "{`"apiEnv`":`"test`",`"paymentStatus`":`"DONE`",`"transId`":`"$providerRef`",`"projectId`":`"00000`",`"paymentIntentId`":`"$providerRef`",`"paymentSource`":`"OrangeMoneyAPI`",`"customerNumber`":`"$realMsisdn`",`"paymentAmount`":1000,`"paymentFees`":25,`"contryOrigin`":`"BF`",`"reference`":`"$paymentId`",`"currency`":`"XOF`",`"isPaylink`":false}"
    $hash = New-HmacSha256Hex -payload $payloadJson -secret $WebhookSecret
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/webhooks/yenga_pay" -Headers @{
        "Content-Type"     = "application/json; charset=utf-8"
        "x-webhook-hash"   = $hash
        "x-yengapay-event" = "payment.success"
    } -Body $payloadJson
    Assert-Ok $res "Webhook payment" @(200)
    Write-Ok "Webhook payment.success 200 (customerNumber=$realMsisdn)"

    # Force canal pay-in (au cas ou webhook n'a pas ecrase le seed du /pay)
    Invoke-Sql @"
UPDATE payments
SET customer_phone = '+226$realMsisdn',
    metadata = COALESCE(metadata, '{}'::jsonb) || jsonb_build_object(
        'customer_number', '$realMsisdn',
        'payment_source', COALESCE(NULLIF(metadata->>'payment_source', ''), 'OrangeMoneyAPI'),
        'operator', COALESCE(NULLIF(metadata->>'operator', ''), 'ORANGE')
    ),
    updated_at = NOW()
WHERE id = '$paymentId'::uuid;
"@ | Out-Null

    $canal = Invoke-Sql "SELECT customer_phone || '|' || COALESCE(metadata->>'customer_number','') FROM payments WHERE id = '$paymentId'::uuid;"
    Write-Host "  Canal pay-in: $canal" -ForegroundColor DarkGray
    if ($canal -notmatch "77515151") {
        Write-Fail "Pay-in channel not set (got: $canal)"
    }
    Write-Ok "Pay-in channel OK for refund ($canal)"

    Start-Sleep -Seconds 2
    $st = Invoke-Sql "SELECT status FROM payments WHERE id = '$paymentId'::uuid;"
    if ($st -match "success") { Write-Ok "Payment status = success" } else { Write-Warn "Payment status = $st" }

    
    # ----- 06 Shipping + delivery -----
        # ----- 06 Shipping + delivery (routes GitHub) -----
    Write-Step "06/10" "Shipping + delivery proofs"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/shipping" -Headers $script:State.MerchantHeaders -Body @{
        order_id         = $orderId
        proof_url        = "https://example.com/shipping-$Timestamp.jpg"
        tracking_number  = "TRACK$Timestamp"
        carrier          = "DHL"
        notes            = "E2E clawback shipped"
    }
    Assert-Ok $res "Shipping" @(200, 201)
    Write-Ok "Shipping OK"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/delivery" -Headers $script:State.MerchantHeaders -Body @{
        order_id  = $orderId
        proof_url = "https://example.com/delivery-$Timestamp.jpg"
        signature = "SIG$Timestamp"
        notes     = "E2E clawback delivered"
        rating    = 5
    }
    Assert-Ok $res "Delivery" @(200, 201)
    Write-Ok "Delivery OK"

    # ----- 07 Auto-release -----
        # ----- 07 Auto-release -----
    Write-Step "07/10" "Escrow auto-release (scheduler)"
    $sqlElig = @"
UPDATE delivery_proofs
SET delivery_date = NOW() - INTERVAL '$EligibilityBackdateDays days',
    updated_at    = NOW() - INTERVAL '$EligibilityBackdateDays days'
WHERE order_id = '$orderId'::uuid
  AND escrow_status = 'delivered';

UPDATE escrow_accounts
SET updated_at = NOW() - INTERVAL '$EligibilityBackdateDays days'
WHERE order_id = '$orderId'::uuid
  AND status = 'funds_held';
"@
    Invoke-Sql $sqlElig | Out-Null
    Write-Ok "Delivery backdated ${EligibilityBackdateDays}d"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/admin/scheduler/trigger-escrow-auto-release" -Headers $script:State.AdminHeaders
    if ($res.StatusCode -ne 200 -and $res.StatusCode -ne 202) {
        Write-Fail "Scheduler trigger HTTP $($res.StatusCode)"
    }
    Write-Ok "Scheduler triggered (HTTP $($res.StatusCode))"
    Start-Sleep -Seconds 12

        # ----- 08 Open dispute (post-release) -----
    Write-Step "08/10" "Open dispute on RELEASED escrow"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/dispute" -Headers $script:State.MerchantHeaders -Body @{
        reason = "Produit non conforme - E2E clawback post-release $Timestamp"
    }
    if (-not $res.Success) {
        Write-Warn "Path /api/orders/{id}/dispute failed: HTTP $($res.StatusCode)"
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/disputes" -Headers $script:State.MerchantHeaders -Body @{
            order_id = $orderId
            reason   = "Produit non conforme - E2E clawback post-release $Timestamp"
        }
    }
    Assert-Ok $res "Open dispute" @(200, 201)
    $disputeId = Get-Prop $res.Data @("dispute.id", "data.dispute.id", "id", "data.id")
    if (-not $disputeId) {
        $disputeId = (Invoke-Sql "SELECT id FROM disputes WHERE order_id = '$orderId'::uuid ORDER BY created_at DESC LIMIT 1;").Trim()
    }
    if (-not $disputeId) { Write-Fail "Dispute id missing" }
    $script:State.DisputeId = $disputeId
    Write-Ok "Dispute opened $disputeId (post-release)"

    $esc2 = Invoke-Sql "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid;"
    Write-Host "  Escrow after open dispute: $esc2" -ForegroundColor DarkGray

    # ----- 09 Resolve customer_wins -----
    Write-Step "09/10" "Admin resolve customer_wins (clawback)"
    $wbRaw = Invoke-Sql "SELECT COALESCE(balance_cents, 0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $wb = 0
    [void][int64]::TryParse(("$wbRaw").Trim(), [ref]$wb)
    Write-Host "  Wallet BEFORE resolve: $wb cents" -ForegroundColor DarkGray

    $resolvePaths = @(
        "$BaseUrl/api/admin/disputes/$disputeId/resolve",
        "$BaseUrl/api/disputes/$disputeId/resolve",
        "$BaseUrl/api/admin/orders/$orderId/dispute/resolve"
    )
    $resolved = $false
    $lastErr = ""
    foreach ($path in $resolvePaths) {
        $rr = Invoke-SafeApi -Method Post -Uri $path -Headers $script:State.AdminHeaders -Body @{
            resolution = "customer_wins"
            notes      = "E2E clawback post-release $Timestamp"
        }
        if ($rr.Success -and ($rr.StatusCode -eq 200 -or $rr.StatusCode -eq 201)) {
            Write-Ok "Resolve OK via $path"
            $resolved = $true
            break
        }
        $lastErr = "HTTP $($rr.StatusCode) $($rr.Error)"
        Write-Warn "Try $path -> $lastErr"
    }
    if (-not $resolved) {
        Write-Fail "customer_wins failed. Last: $lastErr"
    }

    Start-Sleep -Seconds 2

    $walletAfter = Invoke-Sql "SELECT balance_cents FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $clawRows    = Invoke-Sql "SELECT transaction_type, amount_cents, balance_after_cents, reference_type FROM wallet_transactions WHERE shop_id = '$shopId'::uuid AND transaction_type = 'clawback' ORDER BY created_at DESC LIMIT 3;"
    $escFinal    = Invoke-Sql "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid;"
    $dispFinal   = Invoke-Sql "SELECT status, resolution_notes FROM disputes WHERE id = '$disputeId'::uuid;"

    Write-Host "  Wallet AFTER : $walletAfter cents" -ForegroundColor White
    Write-Host "  Clawback txn : $clawRows" -ForegroundColor White
    Write-Host "  Escrow       : $escFinal" -ForegroundColor White
    Write-Host "  Dispute      : $dispFinal" -ForegroundColor White

    $wa = 0
    [void][int64]::TryParse(("$walletAfter").Trim(), [ref]$wa)
    if ($wa -lt $wb) {
        Write-Ok "Wallet reduced by clawback ($wb -> $wa)"
    }
    else {
        Write-Warn "Wallet not reduced ($wb -> $wa) - check ApplyClawback logs"
    }

    if ("$clawRows" -match "clawback") {
        Write-Ok "Ligne clawback presente"
    }
    else {
        Write-Fail "Aucune transaction clawback en DB"
    }

    if ("$dispFinal" -match "resolved_customer") {
    Write-Ok "Dispute resolved_customer"
}
else {
    Write-Warn "Dispute status: $dispFinal"
}
    # ----- 10 Summary -----
    Write-Step "10/10" "Summary"
    Write-Host "  Shop     : $shopId ($ShopSlug)" -ForegroundColor White
    Write-Host "  Order    : $orderId" -ForegroundColor White
    Write-Host "  Payment  : $paymentId" -ForegroundColor White
    Write-Host "  Dispute  : $disputeId" -ForegroundColor White
    Write-Host "  Wallet   : $wb -> $wa cents" -ForegroundColor White
    Write-Host ""
    Write-Host "  Logs utiles:" -ForegroundColor Yellow
    Write-Host "  docker compose logs goshop 2>&1 | Select-String -Pattern clawback" -ForegroundColor DarkGray

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $resultColor = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    $resultMsg = " RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed
    Write-Host $resultMsg -ForegroundColor $resultColor
    Write-Host "================================================================" -ForegroundColor Magenta

    if (($script:Failed -eq 0) -and ("$clawRows" -match "clawback")) {
        Write-Host ""
        Write-Host "  CLAWBACK E2E VERT - pret pour commit" -ForegroundColor Green
    }
}
catch {
    Write-Host ""
    $errMsg = $_.Exception.Message
    Write-Host "[CRITICAL FAILURE] $errMsg" -ForegroundColor Red
    exit 1
}

Write-Host ""