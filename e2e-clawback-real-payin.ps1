# ============================================================
# GOSHOP E2E - Clawback post-release + REAL PAY-IN (navigateur)
# ============================================================
# Flux:
#   register/login -> shop + KYC -> product/customer/order
#   -> pay-in REEL (checkout Yenga + poll CheckStatus)
#   -> PAS de UPDATE SQL sur customer_phone
#   -> shipping + delivery
#   -> backdate delivery + trigger auto-release
#   -> OPEN dispute (escrow deja released)
#   -> admin resolve customer_wins
#   -> verifie wallet reduit + ligne clawback
#
# Prerequis:
#   - API localhost:8080
#   - YENGA_PAY_* OK
#   - $env:YENGA_PAY_WEBHOOK_SECRET (payout simulé si besoin)
#   - $env:ADMIN_EMAIL / $env:ADMIN_PASSWORD
#   - Optionnel: webhook ngrok pour payment_source / MSISDN
#     (sinon CheckStatus doit enrichir le canal au poll)
#
# Honnêteté:
#   Si customer_phone reste seed (+22670000000) après poll,
#   le resolve DOIT échouer -> test ROUGE (pas de force SQL).
# ============================================================

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

# -------------------- CONFIG --------------------
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

# -------------------- HELPERS --------------------
function Write-Step([string]$n, [string]$msg) {
    Write-Host ""
    Write-Host "[$n] $msg" -ForegroundColor Cyan
}
function Write-Ok([string]$msg) {
    Write-Host "  [OK] $msg" -ForegroundColor Green
    $script:Passed++
}
function Write-Warn([string]$msg) {
    Write-Host "  [WARN] $msg" -ForegroundColor Yellow
}
function Write-Fail([string]$msg) {
    Write-Host "  [FAIL] $msg" -ForegroundColor Red
    $script:Failed++
    throw $msg
}

function Invoke-SafeApi {
    param(
        [string]$Method,
        [string]$Uri,
        [hashtable]$Headers = @{},
        [object]$Body = $null,
        [int[]]$OkCodes = @(200, 201, 202)
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
        $raw = $_.Exception.Message
        if ($_.Exception.Response) {
            try { $code = [int]$_.Exception.Response.StatusCode } catch {}
            try {
                $stream = $_.Exception.Response.GetResponseStream()
                if ($stream) {
                    $reader = New-Object System.IO.StreamReader($stream)
                    $raw = $reader.ReadToEnd()
                }
            } catch {}
        }
        if ($_.ErrorDetails.Message) { $raw = $_.ErrorDetails.Message }
        $data = $null
        try { $data = $raw | ConvertFrom-Json } catch {}
        return @{ Success = $false; StatusCode = $code; Data = $data; Raw = $raw; Error = $_.Exception.Message }
    }
}

function Assert-Ok {
    param($res, [string]$label, [int[]]$codes = @(200, 201, 202))
    if (-not $res.Success -or ($codes -notcontains $res.StatusCode)) {
        $detail = if ($res.Raw) { $res.Raw } else { $res.Error }
        Write-Fail "$label -> HTTP $($res.StatusCode) $detail"
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
        if ($ok -and $null -ne $cur -and "$cur" -ne "") { return $cur }
    }
    return $null
}

function Extract-Token($data) {
    $t = Get-Prop $data @("access_token", "token", "data.access_token", "data.token")
    if (-not $t) { Write-Fail "token missing in login response" }
    return $t
}

function Extract-Id($data) {
    $id = Get-Prop $data @("id", "data.id", "shop_id", "data.shop_id", "order_id", "data.order_id", "payment_id", "data.payment_id")
    if (-not $id) { Write-Fail "id missing in response: $($data | ConvertTo-Json -Compress -Depth 5)" }
    return "$id"
}

function Invoke-Sql([string]$sql) {
    $out = $sql | docker compose exec -T $DbService psql -U $DbUser -d $DbName -v ON_ERROR_STOP=1 -t -A 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Fail "SQL failed (exit $LASTEXITCODE): $out"
    }
    return ($out | Out-String).Trim()
}

function Get-HmacHex([string]$secret, [string]$body) {
    $hmac = [System.Security.Cryptography.HMACSHA256]::new([Text.Encoding]::UTF8.GetBytes($secret))
    $hash = $hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($body))
    return -join ($hash | ForEach-Object { $_.ToString("x2") })
}

function Test-IsSeedPhone([string]$phone) {
    if (-not $phone) { return $true }
    $d = ($phone -replace '\D', '')
    return ($d -eq "22670000000" -or $d -eq "70000000" -or $phone -match '70000000')
}

# -------------------- MAIN --------------------
Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Clawback post-release + REAL PAY-IN" -ForegroundColor Magenta
Write-Host " BaseUrl: $BaseUrl | BackdateDays=$EligibilityBackdateDays" -ForegroundColor DarkGray
Write-Host " NO SQL force on customer_phone (canal = CheckStatus/webhook)" -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # ----- 01 Health -----
    Write-Step "01/10" "Health check"
    $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health/live" -OkCodes @(200, 404)
    if ($res.StatusCode -eq 200 -or $res.StatusCode -eq 404) {
        Write-Ok "API live (HTTP $($res.StatusCode))"
    } else {
        $res2 = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health"
        Assert-Ok $res2 "Health"
        Write-Ok "API live"
    }

    # ----- 02 Admin -----
    Write-Step "02/10" "Admin login"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email    = $AdminEmail
        password = $AdminPassword
    }
    Assert-Ok $res "Admin login"
    $adminToken = Extract-Token $res.Data
    $script:State.AdminHeaders = @{
        Authorization  = "Bearer $adminToken"
        "Content-Type" = "application/json"
    }
    Write-Ok "Admin token OK ($AdminEmail)"

    # ----- 03 Merchant + shop + KYC -----
    Write-Step "03/10" "Register + login merchant + shop + KYC"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/register" -Body @{
        email      = $MerchantEmail
        password   = $MerchantPassword
        first_name = "Claw"
        last_name  = "Real"
    }
    Assert-Ok $res "Register" @(200, 201)
    Write-Ok "Register HTTP $($res.StatusCode)"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email    = $MerchantEmail
        password = $MerchantPassword
    }
    Assert-Ok $res "Merchant login"
    $merchantToken = Extract-Token $res.Data
    Write-Ok "Merchant token OK"

    $mh = @{
        Authorization  = "Bearer $merchantToken"
        "Content-Type" = "application/json"
    }
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/shops" -Headers $mh -Body @{
        name = $ShopName
        slug = $ShopSlug
    }
    Assert-Ok $res "Create shop" @(200, 201)
    $shopId = Extract-Id $res.Data
    $script:State.ShopId = $shopId
    $script:State.MerchantHeaders = @{
        Authorization  = "Bearer $merchantToken"
        "Content-Type" = "application/json"
        "X-Shop-Slug"  = $ShopSlug
    }
    Write-Ok "Shop $shopId ($ShopSlug)"

    Invoke-Sql @"
UPDATE shops SET kyc_status = 'verified', updated_at = NOW()
WHERE id = '$shopId'::uuid;
"@ | Out-Null
    Write-Ok "KYC forced to verified"

    # ----- 04 Product, customer, order -----
    Write-Step "04/10" "Product, customer, order"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/products" -Headers $script:State.MerchantHeaders -Body @{
        name             = "Produit claw real $Timestamp"
        price_cents      = 100000
        currency         = "XOF"
        stock_quantity   = 10
        is_active        = $true
    }
    Assert-Ok $res "Product" @(200, 201)
    $productId = Extract-Id $res.Data
    Write-Ok "Product $productId"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" -Headers $script:State.MerchantHeaders -Body @{
        first_name = "Client"
        last_name  = "ClawReal"
        email      = "client.claw.real.$Timestamp@test.com"
        phone      = "+22677515151"
    }
    Assert-Ok $res "Customer" @(200, 201)
    $customerId = Extract-Id $res.Data
    Write-Ok "Customer $customerId"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders" -Headers $script:State.MerchantHeaders -Body @{
        customer_id = $customerId
        items       = @(
            @{ product_id = $productId; quantity = 1 }
        )
    }
    Assert-Ok $res "Order" @(200, 201)
    $orderId = Extract-Id $res.Data
    $script:State.OrderId = $orderId
    Write-Ok "Order $orderId"

    # Zone BF-OUAGA-URB (delay 5j) pour backdate 6j
    $zoneRow = Invoke-Sql "SELECT id FROM delivery_zones WHERE zone_code = 'BF-OUAGA-URB' LIMIT 1;"
    if ($zoneRow -and $zoneRow -match '[0-9a-fA-F-]{36}') {
        $zoneId = $Matches[0]
        Invoke-Sql "UPDATE orders SET delivery_zone_id = '$zoneId'::uuid WHERE id = '$orderId'::uuid;" | Out-Null
        Write-Ok "Order zone BF-OUAGA-URB"
    } else {
        Write-Warn "Zone BF-OUAGA-URB introuvable — fallback delai defaut"
    }

    # ----- 05 REAL pay-in (no phone SQL force) -----
    Write-Step "05/10" "Initiate YengaPay + REAL checkout (NO phone SQL)"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/payments" -Headers $script:State.MerchantHeaders -Body @{
        order_id    = $orderId
        provider    = "yenga_pay"
        amount_cents = 100000
        currency    = "XOF"
        description = "E2E clawback real pay-in order $orderId"
    }
    Assert-Ok $res "Initiate payment" @(200, 201)
    $paymentId = Extract-Id $res.Data
    $providerRef = Get-Prop $res.Data @("provider_ref", "data.provider_ref", "reference", "data.reference")
    $checkout = Get-Prop $res.Data @("redirect_url", "data.redirect_url", "checkout_url", "data.checkout_url", "payment_url", "data.payment_url")
    $script:State.PaymentId = $paymentId
    $script:State.ProviderRef = $providerRef
    $script:State.CheckoutUrl = $checkout
    Write-Ok "Payment $paymentId | ref=$providerRef"

    if (-not $checkout) {
        Write-Fail "Pas de checkout URL — flux indirect requis pour REAL pay-in"
    }

    Write-Host ""
    Write-Host "  ============================================================" -ForegroundColor Yellow
    Write-Host "  PAIEMENT REEL - checkout Yenga" -ForegroundColor Yellow
    Write-Host "  $checkout" -ForegroundColor White
    Write-Host "  Utilise TON vrai numero sandbox (pas 70000000)" -ForegroundColor Yellow
    Write-Host "  ============================================================" -ForegroundColor Yellow
    try { Start-Process $checkout } catch { Write-Warn "Start-Process failed — ouvre le lien manuellement" }
    Write-Ok "Navigateur ouvert"

    Write-Host "  En attente statut success (max ${PayInTimeoutSec}s)..." -ForegroundColor DarkGray
    $deadline = (Get-Date).AddSeconds($PayInTimeoutSec)
    $payStatus = "processing"
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Seconds 5
        $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/payments/$paymentId" -Headers $script:State.MerchantHeaders
        $payStatus = Get-Prop $res.Data @("status", "data.status")
        $ts = Get-Date -Format "HH:mm:ss"
        Write-Host "  ... payment status = $payStatus  ($ts)" -ForegroundColor DarkGray
        if ($payStatus -eq "success" -or $payStatus -eq "failed" -or $payStatus -eq "cancelled") { break }
    }
    if ($payStatus -ne "success") {
        Write-Fail "Pay-in non success apres timeout (status=$payStatus). Complete le checkout."
    }
    Write-Ok "Pay-in REEL detecte: status=success"

    # Lecture canal — AUCUN UPDATE
    $canal = Invoke-Sql @"
SELECT COALESCE(customer_phone, '') || '|' || COALESCE(metadata->>'customer_number', '') || '|' || COALESCE(metadata->>'payment_source', '')
FROM payments WHERE id = '$paymentId'::uuid;
"@
    Write-Host "  Canal pay-in (DB, no force): $canal" -ForegroundColor White
    $phonePart = ($canal -split '\|')[0]
    if (Test-IsSeedPhone $phonePart) {
        Write-Fail "Canal pay-in encore seed ($phonePart). Sans webhook/CheckStatus MSISDN, refund/clawback non fiable. Configure ngrok webhook ou corrige CheckStatus — PAS de SQL force dans ce script."
    }
    Write-Ok "Canal pay-in non-seed: $canal"

    # ----- 06 Shipping + delivery -----
    Write-Step "06/10" "Shipping + delivery proofs"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/shipping-proof" -Headers $script:State.MerchantHeaders -Body @{
        carrier_name   = "E2E Carrier"
        tracking_number = "CLAW-REAL-$Timestamp"
        shipped_at     = (Get-Date).ToUniversalTime().ToString("o")
    }
    Assert-Ok $res "Shipping" @(200, 201)
    Write-Ok "Shipping OK"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/delivery-proof" -Headers $script:State.MerchantHeaders -Body @{
        recipient_name = "Client ClawReal"
        delivered_at   = (Get-Date).ToUniversalTime().ToString("o")
        notes          = "E2E clawback real delivery"
    }
    Assert-Ok $res "Delivery" @(200, 201)
    Write-Ok "Delivery OK"

    # ----- 07 Auto-release -----
    Write-Step "07/10" "Escrow auto-release (scheduler)"
    Invoke-Sql @"
UPDATE delivery_proofs
SET delivery_date = NOW() - INTERVAL '$EligibilityBackdateDays days'
WHERE order_id = '$orderId'::uuid;
"@ | Out-Null
    Write-Ok "Delivery backdated ${EligibilityBackdateDays}d"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/admin/scheduler/trigger-escrow-auto-release" -Headers $script:State.AdminHeaders
    Assert-Ok $res "Scheduler" @(200, 202)
    Write-Ok "Scheduler triggered (HTTP $($res.StatusCode))"
    Start-Sleep -Seconds $SchedulerWaitSec

    $esc = Invoke-Sql "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    Write-Host "  Escrow after release: $esc" -ForegroundColor White
    if ($esc -notmatch "released") {
        Write-Fail "Escrow not released ($esc) — check zone delay vs backdate / logs scheduler"
    }
    Write-Ok "Escrow = released"

    $wb = Invoke-Sql "SELECT COALESCE(balance_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    Write-Host "  Wallet BEFORE dispute: $wb cents" -ForegroundColor White

    # ----- 08 Open dispute post-release -----
    Write-Step "08/10" "Open dispute on RELEASED escrow"
    $openBody = @{
        order_id = $orderId
        reason   = "E2E clawback real post-release $Timestamp"
        details  = "Customer claims issue after funds released"
    }
    $disputeId = $null
    $openUrls = @(
        "$BaseUrl/api/disputes",
        "$BaseUrl/api/orders/$orderId/disputes",
        "$BaseUrl/api/admin/disputes"
    )
    foreach ($url in $openUrls) {
        $res = Invoke-SafeApi -Method Post -Uri $url -Headers $script:State.MerchantHeaders -Body $openBody
        if ($res.Success -and ($res.StatusCode -eq 200 -or $res.StatusCode -eq 201)) {
            $disputeId = Extract-Id $res.Data
            break
        }
        # admin headers fallback
        $res = Invoke-SafeApi -Method Post -Uri $url -Headers $script:State.AdminHeaders -Body $openBody
        if ($res.Success -and ($res.StatusCode -eq 200 -or $res.StatusCode -eq 201)) {
            $disputeId = Extract-Id $res.Data
            break
        }
    }
    if (-not $disputeId) {
        Write-Fail "Open dispute failed on all routes"
    }
    $script:State.DisputeId = $disputeId
    Write-Ok "Dispute opened $disputeId (post-release)"

    $esc2 = Invoke-Sql "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    Write-Host "  Escrow after open dispute: $esc2" -ForegroundColor White

    # ----- 09 Resolve customer_wins -----
    Write-Step "09/10" "Admin resolve customer_wins (clawback + real refund canal)"
    Write-Host "  Wallet BEFORE resolve: $wb cents" -ForegroundColor White

    $resolveBody = @{
        resolution = "customer_wins"
        notes      = "E2E clawback REAL post-release $Timestamp"
    }
    $resolved = $false
    $resolveUrls = @(
        "$BaseUrl/api/admin/disputes/$disputeId/resolve",
        "$BaseUrl/api/disputes/$disputeId/resolve",
        "$BaseUrl/api/admin/orders/$orderId/dispute/resolve"
    )
    foreach ($url in $resolveUrls) {
        $res = Invoke-SafeApi -Method Post -Uri $url -Headers $script:State.AdminHeaders -Body $resolveBody
        if ($res.Success -and ($res.StatusCode -eq 200 -or $res.StatusCode -eq 201)) {
            Write-Ok "Resolve OK via $url"
            $resolved = $true
            break
        }
        Write-Warn "Try $url -> HTTP $($res.StatusCode) $($res.Raw)"
    }
    if (-not $resolved) {
        Write-Fail "customer_wins failed on all routes"
    }

    Start-Sleep -Seconds 2
    $wa = Invoke-Sql "SELECT COALESCE(balance_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $debt = Invoke-Sql "SELECT COALESCE(debt_cents,0) FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $clawRows = Invoke-Sql @"
SELECT transaction_type || '|' || amount_cents || '|' || balance_after_cents || '|' || COALESCE(reference_type,'')
FROM wallet_transactions
WHERE shop_id = '$shopId'::uuid
  AND (transaction_type = 'clawback' OR reference_type = 'dispute_clawback' OR transaction_type = 'debt_add')
ORDER BY created_at DESC
LIMIT 3;
"@
    $escFinal = Invoke-Sql "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    $dispFinal = Invoke-Sql "SELECT status FROM disputes WHERE id = '$disputeId'::uuid LIMIT 1;"

    Write-Host "  Wallet AFTER : $wa cents" -ForegroundColor White
    Write-Host "  Debt         : $debt cents" -ForegroundColor White
    Write-Host "  Clawback txn : $clawRows" -ForegroundColor White
    Write-Host "  Escrow       : $escFinal" -ForegroundColor White
    Write-Host "  Dispute      : $dispFinal" -ForegroundColor White

    $wbN = 0; $waN = 0
    [int]::TryParse("$wb", [ref]$wbN) | Out-Null
    [int]::TryParse("$wa", [ref]$waN) | Out-Null
    if ($waN -lt $wbN) {
        Write-Ok "Wallet reduced by clawback ($wb -> $wa)"
    } else {
        Write-Fail "Wallet not reduced ($wb -> $wa) — check ApplyClawback / logs"
    }
    if ("$clawRows" -match "clawback") {
        Write-Ok "Ligne clawback presente"
    } else {
        Write-Warn "Pas de ligne clawback visible (debt_add only? check ledger)"
    }
    if ("$dispFinal" -match "resolved_customer") {
        Write-Ok "Dispute resolved_customer"
    } else {
        Write-Warn "Dispute status: $dispFinal"
    }
    if ("$escFinal" -match "refunded") {
        Write-Ok "Escrow refunded"
    } else {
        Write-Warn "Escrow status: $escFinal"
    }

    # ----- 10 Summary -----
    Write-Step "10/10" "Summary"
    Write-Host "  Shop     : $shopId ($ShopSlug)" -ForegroundColor White
    Write-Host "  Order    : $orderId" -ForegroundColor White
    Write-Host "  Payment  : $paymentId" -ForegroundColor White
    Write-Host "  Dispute  : $disputeId" -ForegroundColor White
    Write-Host "  Wallet   : $wb -> $wa cents | debt=$debt" -ForegroundColor White
    Write-Host "  Checkout : $($script:State.CheckoutUrl)" -ForegroundColor DarkGray
    Write-Host "  Canal    : $canal" -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Logs:" -ForegroundColor Yellow
    Write-Host "  docker compose logs goshop 2>&1 | Select-String -Pattern 'clawback|Pay-in channel|customer_wins|cashout_method'" -ForegroundColor DarkGray

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
    Write-Host "[CRITICAL FAILURE] $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host ""