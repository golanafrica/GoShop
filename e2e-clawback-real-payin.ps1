# ============================================================
# GOSHOP E2E - Clawback post-release + REAL PAY-IN (navigateur)
# ============================================================
# Flux:
#   register/login -> shop + KYC -> product/customer/order
#   -> pay-in REEL (POST /api/orders/{id}/pay + poll)
#   -> PAS de UPDATE SQL sur customer_phone
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

$PayInTimeoutSec         = 300
$EligibilityBackdateDays = 6
$SchedulerWaitSec        = 12

$Timestamp        = Get-Date -Format "yyyyMMddHHmmss"
$MerchantEmail    = "merchant.claw.real.$Timestamp@goshop.com"
$MerchantPassword = "Password123!"
$ShopName         = "Claw Real $Timestamp"
$ShopSlug         = "claw-real-$Timestamp"

$script:Passed = 0
$script:Failed = 0
$script:State  = @{
    MerchantHeaders = $null
    AdminHeaders    = $null
    ShopId          = $null
    OrderId         = $null
    PaymentId       = $null
    ProviderRef     = $null
    CheckoutUrl     = $null
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
    $id = Get-Prop $Data @("id", "data.id", "payment_id", "data.payment_id", "shop_id", "data.shop_id")
    if (-not $id) {
        Write-Fail ("id missing for {0}: {1}" -f $Label, ($Data | ConvertTo-Json -Compress -Depth 5))
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

function Test-IsSeedPhone {
    param([string]$Phone)
    if (-not $Phone) { return $true }
    $d = ($Phone -replace '\D', '')
    return ($d -eq "22670000000" -or $d -eq "70000000" -or $Phone -match "70000000")
}

Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Clawback post-release + REAL PAY-IN" -ForegroundColor Magenta
Write-Host (" BaseUrl: {0} | BackdateDays={1}" -f $BaseUrl, $EligibilityBackdateDays) -ForegroundColor DarkGray
Write-Host " NO SQL force on customer_phone" -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # 01 Health
    Write-Step -N "01/10" -Msg "Health check"
    $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health/live"
    if ($res.StatusCode -ne 200 -and $res.StatusCode -ne 404) {
        $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health"
    }
    if ($res.StatusCode -ge 500) { Write-Fail "API down" }
    Write-Ok ("API live HTTP {0}" -f $res.StatusCode)

    # 02 Admin
    Write-Step -N "02/10" -Msg "Admin login"
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
    Write-Ok ("Admin token OK ({0})" -f $AdminEmail)

    # 03 Merchant + shop + KYC
    Write-Step -N "03/10" -Msg "Register + login merchant + shop + KYC"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/register" -Body @{
        email      = $MerchantEmail
        password   = $MerchantPassword
        first_name = "Claw"
        last_name  = "Real"
    }
    Assert-Ok -Res $res -Label "Register" -Codes @(200, 201, 409)
    Write-Ok ("Register HTTP {0}" -f $res.StatusCode)

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email    = $MerchantEmail
        password = $MerchantPassword
    }
    Assert-Ok -Res $res -Label "Merchant login"
    $merchantToken = Get-AuthToken $res.Data
    Write-Ok "Merchant token OK"

    $mh = @{
        Authorization  = "Bearer $merchantToken"
        "Content-Type" = "application/json"
    }
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/shops" -Headers $mh -Body @{
        name = $ShopName
        slug = $ShopSlug
    }
    Assert-Ok -Res $res -Label "Create shop" -Codes @(200, 201)
    $shopId = Get-EntityId $res.Data "shop"
    $script:State.ShopId = $shopId
    $script:State.MerchantHeaders = @{
        Authorization  = "Bearer $merchantToken"
        "Content-Type" = "application/json"
        "X-Shop-Slug"  = $ShopSlug
    }
    Write-Ok ("Shop {0} ({1})" -f $shopId, $ShopSlug)

    $sqlKyc = "UPDATE shops SET kyc_status = 'verified', updated_at = NOW() WHERE id = '$shopId'::uuid;"
    Invoke-Sql -Sql $sqlKyc | Out-Null
    Write-Ok "KYC forced to verified"

    # 04 Product, customer, order
    Write-Step -N "04/10" -Msg "Product, customer, order"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/products" -Headers $script:State.MerchantHeaders -Body @{
        name        = "Produit claw real $Timestamp"
        description = "E2E clawback real"
        price_cents = 100000
        stock       = 100
    }
    Assert-Ok -Res $res -Label "Product" -Codes @(200, 201)
    $productId = Get-EntityId $res.Data "product"

    $sqlStock = "UPDATE products SET stock = 100, updated_at = NOW() WHERE id = '$productId'::uuid;"
    Invoke-Sql -Sql $sqlStock | Out-Null
    Write-Ok ("Product {0} stock=100" -f $productId)

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" -Headers $script:State.MerchantHeaders -Body @{
        first_name = "Client"
        last_name  = "ClawReal"
        email      = "client.claw.real.$Timestamp@test.com"
        phone      = "+22677515151"
    }
    if (-not $res.Success) {
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" -Headers $script:State.MerchantHeaders -Body @{
            first_name   = "Client"
            last_name    = "ClawReal"
            email        = "client.claw.real.$Timestamp@test.com"
            phone_number = "+22677515151"
        }
    }
    Assert-Ok -Res $res -Label "Customer" -Codes @(200, 201)
    $customerId = Get-EntityId $res.Data "customer"
    Write-Ok ("Customer {0}" -f $customerId)

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders" -Headers $script:State.MerchantHeaders -Body @{
        customer_id    = $customerId
        payment_method = "mobile_money"
        items          = @(@{ product_id = $productId; quantity = 1 })
    }
    Assert-Ok -Res $res -Label "Order" -Codes @(200, 201)
    $orderId = Get-EntityId $res.Data "order"
    $script:State.OrderId = $orderId
    Write-Ok ("Order {0}" -f $orderId)

    $zoneRow = Invoke-Sql -Sql "SELECT id::text FROM delivery_zones WHERE zone_code = 'BF-OUAGA-URB' AND is_active = true LIMIT 1;"
    if ($zoneRow -match "[0-9a-fA-F-]{36}") {
        $zoneId = $Matches[0]
        $sqlZone = "UPDATE orders SET delivery_zone_id = '$zoneId'::uuid WHERE id = '$orderId'::uuid;"
        Invoke-Sql -Sql $sqlZone | Out-Null
        Write-Ok "Order zone BF-OUAGA-URB"
    }
    else {
        Write-Warn "Zone BF-OUAGA-URB introuvable - fallback delai defaut"
    }

    # 05 REAL pay-in (route qui marche)
    Write-Step -N "05/10" -Msg "Initiate YengaPay + REAL checkout (NO phone SQL)"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/pay" -Headers $script:State.MerchantHeaders -Body @{
        provider     = "yenga_pay"
        phone_number = "+22677515151"
        description  = "E2E clawback real pay-in order $orderId"
        metadata     = @{ flow = "indirect" }
    }
    Assert-Ok -Res $res -Label "Initiate payment" -Codes @(200, 201)

    $paymentId = Get-Prop $res.Data @("payment_id", "id", "data.payment_id", "data.id")
    $providerRef = Get-Prop $res.Data @("provider_ref", "data.provider_ref")
    $checkout = Get-Prop $res.Data @(
        "redirect_url", "data.redirect_url",
        "checkout_url", "data.checkout_url",
        "payment_url", "data.payment_url",
        "metadata.payment_url", "data.metadata.payment_url"
    )
    if (-not $paymentId) {
        Write-Fail ("payment_id missing: {0}" -f ($res.Data | ConvertTo-Json -Compress))
    }
    $script:State.PaymentId   = [string]$paymentId
    $script:State.ProviderRef = [string]$providerRef
    $script:State.CheckoutUrl = [string]$checkout
    Write-Ok ("Payment {0} | ref={1}" -f $paymentId, $providerRef)

    if (-not $checkout) {
        Write-Fail "Pas de checkout URL - flux indirect requis pour REAL pay-in"
    }

    Write-Host ""
    Write-Host "  ============================================================" -ForegroundColor Yellow
    Write-Host "  PAIEMENT REEL - checkout Yenga" -ForegroundColor Yellow
    Write-Host ("  {0}" -f $checkout) -ForegroundColor White
    Write-Host "  Utilise TON vrai numero sandbox (pas 70000000)" -ForegroundColor Yellow
    Write-Host "  ============================================================" -ForegroundColor Yellow
    try {
        Start-Process $checkout
        Write-Ok "Navigateur ouvert"
    }
    catch {
        Write-Warn "Start-Process failed - ouvre le lien manuellement"
    }

    Write-Host ("  En attente statut success (max {0}s)..." -f $PayInTimeoutSec) -ForegroundColor DarkGray
    $deadline  = (Get-Date).AddSeconds($PayInTimeoutSec)
    $payStatus = "processing"
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Seconds 5
        $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/payments/$paymentId" -Headers $script:State.MerchantHeaders
        $payStatus = Get-Prop $res.Data @("status", "data.status")
        $ts = Get-Date -Format "HH:mm:ss"
        Write-Host ("  ... payment status = {0}  ({1})" -f $payStatus, $ts) -ForegroundColor DarkGray
        if ($payStatus -eq "success" -or $payStatus -eq "failed" -or $payStatus -eq "cancelled") { break }
    }
    if ($payStatus -ne "success") {
        Write-Fail ("Pay-in non success apres timeout (status={0}). Complete le checkout." -f $payStatus)
    }
    Write-Ok "Pay-in REEL detecte: status=success"

    $sqlCanal = "SELECT COALESCE(customer_phone, '') || '|' || COALESCE(metadata->>'customer_number', '') || '|' || COALESCE(metadata->>'payment_source', '') FROM payments WHERE id = '$paymentId'::uuid;"
    $canal = Invoke-Sql -Sql $sqlCanal
    Write-Host ("  Canal pay-in (DB, no force): {0}" -f $canal) -ForegroundColor White
    $phonePart = ($canal -split '\|')[0]
    if (Test-IsSeedPhone -Phone $phonePart) {
        Write-Fail ("Canal pay-in encore seed ({0}). Sans webhook/CheckStatus MSISDN, refund non fiable." -f $phonePart)
    }
    Write-Ok ("Canal pay-in non-seed: {0}" -f $canal)

    # 06 Shipping + delivery (routes E2E verts)
    Write-Step -N "06/10" -Msg "Shipping + delivery proofs"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/shipping" -Headers $script:State.MerchantHeaders -Body @{
        order_id        = $orderId
        proof_url       = "https://example.com/shipping-claw-$Timestamp.jpg"
        tracking_number = "CLAW-REAL-$Timestamp"
        carrier         = "E2E"
        notes           = "E2E shipped"
    }
    if (-not $res.Success) {
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/shipping-proof" -Headers $script:State.MerchantHeaders -Body @{
            tracking_number = "CLAW-REAL-$Timestamp"
            carrier         = "E2E"
            shipped_at      = (Get-Date).ToUniversalTime().ToString("o")
        }
    }
    Assert-Ok -Res $res -Label "Shipping" -Codes @(200, 201)
    Write-Ok "Shipping OK"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/delivery" -Headers $script:State.MerchantHeaders -Body @{
        order_id  = $orderId
        proof_url = "https://example.com/delivery-claw-$Timestamp.jpg"
        signature = "SIG-CLAW-$Timestamp"
        notes     = "E2E delivered"
        rating    = 5
    }
    if (-not $res.Success) {
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/delivery-proof" -Headers $script:State.MerchantHeaders -Body @{
            recipient_name = "Client ClawReal"
            delivered_at   = (Get-Date).ToUniversalTime().ToString("o")
            notes          = "E2E clawback real delivery"
        }
    }
    Assert-Ok -Res $res -Label "Delivery" -Codes @(200, 201)
    Write-Ok "Delivery OK"

    # 07 Auto-release
    Write-Step -N "07/10" -Msg "Escrow auto-release (scheduler)"
    $sqlBackdate = "UPDATE delivery_proofs SET delivery_date = NOW() - INTERVAL '$EligibilityBackdateDays days', updated_at = NOW() - INTERVAL '$EligibilityBackdateDays days' WHERE order_id = '$orderId'::uuid;"
    Invoke-Sql -Sql $sqlBackdate | Out-Null
    Write-Ok ("Delivery backdated {0}d" -f $EligibilityBackdateDays)

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/admin/scheduler/trigger-escrow-auto-release" -Headers $script:State.AdminHeaders
    Assert-Ok -Res $res -Label "Scheduler" -Codes @(200, 202)
    Write-Ok ("Scheduler triggered (HTTP {0})" -f $res.StatusCode)
    Start-Sleep -Seconds $SchedulerWaitSec

    $esc = Invoke-Sql -Sql "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    Write-Host ("  Escrow after release: {0}" -f $esc) -ForegroundColor White
    if ($esc -notmatch "released") {
        Write-Fail ("Escrow not released ({0})" -f $esc)
    }
    Write-Ok "Escrow = released"

    $wb = Invoke-Sql -Sql "SELECT COALESCE(balance_cents,0)::text FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    Write-Host ("  Wallet BEFORE dispute: {0} cents" -f $wb) -ForegroundColor White

    # ----- 08 Open dispute post-release -----
Write-Step -N "08/10" -Msg "Open dispute on RELEASED escrow"

$openBody = @{
    reason = "E2E clawback real post-release product not as described $Timestamp"
}
# Route officielle GoShop: POST /api/orders/{id}/dispute
$res = Invoke-SafeApi -Method Post `
    -Uri "$BaseUrl/api/orders/$orderId/dispute" `
    -Headers $script:State.MerchantHeaders `
    -Body $openBody

if (-not $res.Success) {
    Write-Fail ("Open dispute failed HTTP {0}: {1}" -f $res.StatusCode, $res.Raw)
}

$disputeId = Get-Prop $res.Data @("dispute.id", "data.dispute.id", "id", "data.id")
if (-not $disputeId) {
    Write-Fail ("dispute id missing: {0}" -f ($res.Data | ConvertTo-Json -Compress -Depth 6))
}
$script:State.DisputeId = [string]$disputeId
Write-Ok ("Dispute opened {0} (post-release)" -f $disputeId)

$esc2 = Invoke-Sql -Sql "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
Write-Host ("  Escrow after open dispute: {0}" -f $esc2) -ForegroundColor White
# post-release: reste "released" (normal)

    # 09 Resolve customer_wins
    Write-Step -N "09/10" -Msg "Admin resolve customer_wins (clawback + real refund canal)"
    Write-Host ("  Wallet BEFORE resolve: {0} cents" -f $wb) -ForegroundColor White

    $resolveBody = @{
        resolution = "customer_wins"
        notes      = "E2E clawback REAL post-release $Timestamp"
    }
    $resolved = $false
    $resolveUrls = @(
    "$BaseUrl/api/admin/disputes/$disputeId/resolve",
    "$BaseUrl/api/admin/orders/$orderId/dispute/resolve"
)
    foreach ($url in $resolveUrls) {
        $res = Invoke-SafeApi -Method Post -Uri $url -Headers $script:State.AdminHeaders -Body $resolveBody
        if ($res.Success -and ($res.StatusCode -eq 200 -or $res.StatusCode -eq 201)) {
            Write-Ok ("Resolve OK via {0}" -f $url)
            $resolved = $true
            break
        }
        Write-Warn ("Try {0} -> HTTP {1} {2}" -f $url, $res.StatusCode, $res.Raw)
    }
    if (-not $resolved) {
        Write-Fail "customer_wins failed on all routes"
    }

    Start-Sleep -Seconds 2
    $wa = Invoke-Sql -Sql "SELECT COALESCE(balance_cents,0)::text FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $debt = Invoke-Sql -Sql "SELECT COALESCE(debt_cents,0)::text FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $sqlClaw = "SELECT transaction_type || '|' || amount_cents || '|' || COALESCE(balance_after_cents::text,'') || '|' || COALESCE(reference_type,'') FROM wallet_transactions WHERE shop_id = '$shopId'::uuid AND (transaction_type = 'clawback' OR reference_type = 'dispute_clawback' OR transaction_type = 'debt_add') ORDER BY created_at DESC LIMIT 3;"
    $clawRows = Invoke-Sql -Sql $sqlClaw
    $escFinal = Invoke-Sql -Sql "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    $dispFinal = Invoke-Sql -Sql "SELECT status FROM disputes WHERE id = '$disputeId'::uuid LIMIT 1;"

    Write-Host ("  Wallet AFTER : {0} cents" -f $wa) -ForegroundColor White
    Write-Host ("  Debt         : {0} cents" -f $debt) -ForegroundColor White
    Write-Host ("  Clawback txn : {0}" -f $clawRows) -ForegroundColor White
    Write-Host ("  Escrow       : {0}" -f $escFinal) -ForegroundColor White
    Write-Host ("  Dispute      : {0}" -f $dispFinal) -ForegroundColor White

    $wbN = 0
    $waN = 0
    [void][int]::TryParse("$wb", [ref]$wbN)
    [void][int]::TryParse("$wa", [ref]$waN)
    if ($waN -lt $wbN) {
        Write-Ok ("Wallet reduced by clawback ({0} -> {1})" -f $wb, $wa)
    }
    else {
        Write-Fail ("Wallet not reduced ({0} -> {1}) - check ApplyClawback / logs" -f $wb, $wa)
    }
    if ("$clawRows" -match "clawback|debt_add") {
        Write-Ok "Ligne clawback/debt_add presente"
    }
    else {
        Write-Warn "Pas de ligne clawback visible"
    }
    if ("$dispFinal" -match "resolved_customer") {
        Write-Ok "Dispute resolved_customer"
    }
    else {
        Write-Warn ("Dispute status: {0}" -f $dispFinal)
    }
    if ("$escFinal" -match "refunded") {
        Write-Ok "Escrow refunded"
    }
    else {
        Write-Warn ("Escrow status: {0}" -f $escFinal)
    }

    # 10 Summary
    Write-Step -N "10/10" -Msg "Summary"
    Write-Host ("  Shop     : {0} ({1})" -f $shopId, $ShopSlug) -ForegroundColor White
    Write-Host ("  Order    : {0}" -f $orderId) -ForegroundColor White
    Write-Host ("  Payment  : {0}" -f $paymentId) -ForegroundColor White
    Write-Host ("  Dispute  : {0}" -f $disputeId) -ForegroundColor White
    Write-Host ("  Wallet   : {0} -> {1} cents | debt={2}" -f $wb, $wa, $debt) -ForegroundColor White
    Write-Host ("  Checkout : {0}" -f $script:State.CheckoutUrl) -ForegroundColor DarkGray
    Write-Host ("  Canal    : {0}" -f $canal) -ForegroundColor DarkGray
    Write-Host ("  WebhookSecret configured: {0}" -f ($WebhookSecret.Length -gt 10)) -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Logs:" -ForegroundColor Yellow
    Write-Host '  docker compose logs goshop 2>&1 | Select-String -Pattern "clawback|Pay-in channel|customer_wins|cashout_method"' -ForegroundColor DarkGray

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $resultColor = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    $resultMsg = " RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed
    Write-Host $resultMsg -ForegroundColor $resultColor
    Write-Host "================================================================" -ForegroundColor Magenta

    if ($script:Failed -eq 0) {
        Write-Host ""
        Write-Host "  CLAWBACK REAL PAY-IN E2E VERT" -ForegroundColor Green
        Write-Host "  (canal non-seed, pas de SQL force telephone)" -ForegroundColor Green
    }
}
catch {
    Write-Host ""
    Write-Host ("[CRITICAL FAILURE] {0}" -f $_.Exception.Message) -ForegroundColor Red
    exit 1
}

Write-Host ""