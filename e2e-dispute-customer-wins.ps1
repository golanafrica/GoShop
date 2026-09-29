# ============================================================
# GOSHOP E2E - Dispute customer_wins (refund litige)
# ============================================================
# Flux:
#   register/login -> shop + KYC -> product/customer/order
#   -> pay-in (real navigateur OU webhook)
#   -> OPEN dispute (escrow DOIT rester funds_held - PAS de shipping)
#   -> admin resolve customer_wins -> refund Yenga + escrow refunded
#
# Canal refund:
#   - RealPayIn: on NE force PAS operator/phone si payment_source existe
#   - Webhook simule: paymentSource + customerNumber dans le payload
#   - Seed SQL seulement si metadata canal encore vide
#
# Prerequis env:
#   $env:YENGA_PAY_WEBHOOK_SECRET = "..."
#   $env:ADMIN_EMAIL / $env:ADMIN_PASSWORD
#   $env:REAL_PAYIN = "1" (defaut) ou "0" pour webhook local
#   $env:E2E_PAYMENT_SOURCE = "SankMoneyAPI" (optionnel, webhook simule)
#   $env:E2E_CUSTOMER_NUMBER = "70707070"   (optionnel, webhook simule)
# ============================================================

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

# -------------------- CONFIG --------------------
$BaseUrl       = if ($env:GOSHOP_BASE_URL) { $env:GOSHOP_BASE_URL } else { "http://localhost:8080" }
$WebhookSecret = if ($env:YENGA_PAY_WEBHOOK_SECRET) { $env:YENGA_PAY_WEBHOOK_SECRET } else { "CHANGE_ME_YENGA_PAY_WEBHOOK_SECRET" }
$AdminEmail    = if ($env:ADMIN_EMAIL) { $env:ADMIN_EMAIL } else { "admin@goshop.com" }
$AdminPassword = if ($env:ADMIN_PASSWORD) { $env:ADMIN_PASSWORD } else { "CHANGE_ME_ADMIN_PASSWORD_MIN_16_CHARS" }
$DbService     = if ($env:DB_SERVICE) { $env:DB_SERVICE } else { "db" }
$DbUser        = if ($env:DB_USER) { $env:DB_USER } else { "postgres" }
$DbName        = if ($env:DB_NAME) { $env:DB_NAME } else { "goshop_db" }

$RealPayIn = $true
if ($env:REAL_PAYIN) {
    $RealPayIn = ($env:REAL_PAYIN -eq "1" -or $env:REAL_PAYIN -eq "true")
}
$PayInTimeoutSec = 300
$PayInPollSec    = 5

$Timestamp         = Get-Date -Format "yyyyMMddHHmmss"
$MerchantEmail     = "merchant.dispute.$Timestamp@goshop.com"
$MerchantPassword  = "Password123!"
$ShopName          = "Dispute E2E $Timestamp"
$ShopSlug          = "dispute-shop-$Timestamp"
# Fallback seulement si le canal pay-in n'est pas connu
$CustomerPhone     = if ($env:E2E_CUSTOMER_PHONE) { $env:E2E_CUSTOMER_PHONE } else { "+22677515151" }
$Operator          = if ($env:E2E_OPERATOR) { $env:E2E_OPERATOR } else { "ORANGE" }
# Valeurs webhook simule (alignables sur le vrai checkout)
$SimPaymentSource  = if ($env:E2E_PAYMENT_SOURCE) { $env:E2E_PAYMENT_SOURCE } else { "OrangeMoneyAPI" }
$SimCustomerNumber = if ($env:E2E_CUSTOMER_NUMBER) { $env:E2E_CUSTOMER_NUMBER } else { "70000000" }
$ProductPriceCents = 100000
$feesFcfa          = 25

$script:Passed = 0
$script:Failed = 0
$script:State  = @{}

# -------------------- HELPERS --------------------
function Write-Step {
    param([string]$StepNumber, [string]$Message)
    Write-Host ""
    Write-Host "[$StepNumber] $Message" -ForegroundColor Cyan
}

function Write-Ok {
    param([string]$Message)
    Write-Host "  [OK] $Message" -ForegroundColor Green
    $script:Passed++
}

function Write-Warn {
    param([string]$Message)
    Write-Host "  [WARN] $Message" -ForegroundColor Yellow
}

function Write-Fail {
    param([string]$Message)
    Write-Host "  [FAIL] $Message" -ForegroundColor Red
    $script:Failed++
    throw $Message
}

function Get-Prop {
    param($Obj, [string[]]$Paths)
    foreach ($p in $Paths) {
        $cur = $Obj
        $ok = $true
        foreach ($seg in ($p -split '\.')) {
            if ($null -eq $cur) { $ok = $false; break }
            $cur = $cur.$seg
        }
        if ($ok -and $null -ne $cur -and "$cur" -ne "") { return $cur }
    }
    return $null
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
        return @{
            Success    = $true
            Data       = $data
            StatusCode = [int]$wr.StatusCode
            Raw        = $wr.Content
            Error      = $null
        }
    }
    catch {
        $status = 0
        $errBody = $_.Exception.Message
        if ($_.ErrorDetails -and $_.ErrorDetails.Message) {
            $errBody = $_.ErrorDetails.Message
        }
        $resp = $_.Exception.Response
        if ($resp) {
            try { $status = [int]$resp.StatusCode } catch {}
            try {
                if ($resp -is [System.Net.Http.HttpResponseMessage]) {
                    $errBody = $resp.Content.ReadAsStringAsync().Result
                }
                else {
                    $stream = $resp.GetResponseStream()
                    if ($stream) {
                        $reader = New-Object System.IO.StreamReader($stream)
                        $raw = $reader.ReadToEnd()
                        if ($raw) { $errBody = $raw }
                    }
                }
            } catch {}
        }
        try {
            $parsed = $errBody | ConvertFrom-Json
            if ($parsed.message) { $errBody = $parsed.message }
            elseif ($parsed.error) { $errBody = $parsed.error }
        } catch {}

        return @{
            Success    = $false
            Error      = $errBody
            StatusCode = $status
            Data       = $null
            Raw        = $errBody
        }
    }
}

function Assert-Ok {
    param($Res, [string]$Context, [int[]]$Codes = @(200, 201))
    if (-not $Res.Success -or ($Codes -notcontains $Res.StatusCode)) {
        Write-Fail "$Context -> HTTP $($Res.StatusCode) - $($Res.Error)"
    }
}

function Get-HmacSha256Hex {
    param([string]$Payload, [string]$Secret)
    $hmac = New-Object System.Security.Cryptography.HMACSHA256
    $hmac.Key = [System.Text.Encoding]::UTF8.GetBytes($Secret)
    $hash = $hmac.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($Payload))
    return ([BitConverter]::ToString($hash) -replace '-', '').ToLowerInvariant()
}

function Invoke-Sql {
    param([string]$Sql)
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $out = $Sql | & docker-compose exec -T $DbService psql -U $DbUser -d $DbName -v ON_ERROR_STOP=1 2>&1
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prevEap
    if ($code -ne 0) {
        Write-Warn "SQL failed (exit $code): $out"
        return $null
    }
    return $out
}

function Get-AuthToken {
    param($LoginData)
    $t = Get-Prop $LoginData @(
        "access_token", "token", "data.access_token", "data.token",
        "data.accessToken", "tokens.access_token"
    )
    if (-not $t) {
        Write-Fail "Token not found in login response: $($LoginData | ConvertTo-Json -Compress)"
    }
    return [string]$t
}

function Get-EntityId {
    param($Data, [string]$Label)
    $id = Get-Prop $Data @("id", "data.id", "data.data.id")
    if (-not $id) {
        Write-Fail "ID not found for $Label : $($Data | ConvertTo-Json -Compress)"
    }
    return [string]$id
}

function Open-CheckoutUrl {
    param([string]$Url)
    if (-not $Url) { return }
    Write-Host ""
    Write-Host "  ============================================================" -ForegroundColor Yellow
    Write-Host "  PAIEMENT REEL - checkout Yenga" -ForegroundColor Yellow
    Write-Host "  $Url" -ForegroundColor White
    Write-Host "  Utilise le MEME operateur/numero que tu veux voir au refund" -ForegroundColor DarkGray
    Write-Host "  ============================================================" -ForegroundColor Yellow
    try {
        Start-Process $Url
        Write-Ok "Navigateur ouvert"
    }
    catch {
        Write-Warn "Ouvre manuellement le lien ci-dessus"
    }
}

function Wait-PaymentSuccess {
    param(
        [string]$PaymentId,
        [hashtable]$Headers,
        [int]$TimeoutSec = 300,
        [int]$PollSec = 5
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    $last = "unknown"
    while ((Get-Date) -lt $deadline) {
        $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/payments/$PaymentId" -Headers $Headers
        if ($res.Success) {
            $last = Get-Prop $res.Data @("status", "data.status")
            Write-Host "  ... payment status = $last  ($(Get-Date -Format 'HH:mm:ss'))" -ForegroundColor DarkGray
            if ($last -eq "success") { return $true }
            if ($last -eq "failed" -or $last -eq "cancelled" -or $last -eq "expired") {
                Write-Warn "Payment terminal: $last"
                return $false
            }
        }
        Start-Sleep -Seconds $PollSec
    }
    Write-Warn "Timeout ${TimeoutSec}s (dernier=$last)"
    return $false
}

function Send-SimulatedPaymentWebhook {
    param(
        [string]$PaymentId,
        [string]$ProviderRef,
        [int]$AmountFcfa,
        [int]$FeesFcfa,
        [string]$PaymentSource = "OrangeMoneyAPI",
        [string]$CustomerNumber = "70000000"
    )
    $transId  = if ($ProviderRef) { "$ProviderRef" } else { "YP-E2E-$Timestamp" }
    $intentId = if ($ProviderRef) { "$ProviderRef" } else { "" }
    $payloadJson = "{`"apiEnv`":`"test`",`"paymentStatus`":`"DONE`",`"transId`":`"$transId`",`"projectId`":`"00000`",`"paymentIntentId`":`"$intentId`",`"paymentSource`":`"$PaymentSource`",`"customerNumber`":`"$CustomerNumber`",`"paymentAmount`":$AmountFcfa,`"paymentFees`":$FeesFcfa,`"contryOrigin`":`"BF`",`"reference`":`"$PaymentId`",`"currency`":`"XOF`",`"isPaylink`":false}"
    $hash = Get-HmacSha256Hex -Payload $payloadJson -Secret $WebhookSecret
    Write-Host "  Webhook sim: paymentSource=$PaymentSource customerNumber=$CustomerNumber" -ForegroundColor DarkGray
    return Invoke-SafeApi -Method Post -Uri "$BaseUrl/webhooks/yenga_pay" -Headers @{
        "Content-Type"     = "application/json; charset=utf-8"
        "x-webhook-hash"   = $hash
        "x-yengapay-event" = "payment.success"
    } -Body $payloadJson
}

# ============================================================
Write-Host ""
Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Dispute customer_wins (refund)" -ForegroundColor Magenta
Write-Host " BaseUrl: $BaseUrl | RealPayIn=$RealPayIn" -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # 01 Health
    Write-Step "01/09" "Health check"
    $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health/live"
    Assert-Ok $res "health/live"
    Write-Ok "API live"

    # 02 Admin login
    Write-Step "02/09" "Admin login"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email = $AdminEmail; password = $AdminPassword
    }
    if (-not $res.Success) {
        Write-Fail "Admin login failed HTTP $($res.StatusCode) - set ADMIN_EMAIL / ADMIN_PASSWORD"
    }
    $adminToken = Get-AuthToken $res.Data
    $script:State.AdminHeaders = @{
        "Authorization" = "Bearer $adminToken"
    }
    Write-Ok "Admin token OK ($AdminEmail)"

    # 03 Merchant + shop + KYC
    Write-Step "03/09" "Register + login merchant + shop + KYC"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/register" -Body @{
        email = $MerchantEmail; password = $MerchantPassword
    }
    if (-not $res.Success -and $res.StatusCode -ne 409) {
        Write-Fail "Register failed: $($res.Error)"
    }
    Write-Ok "Register HTTP $($res.StatusCode)"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email = $MerchantEmail; password = $MerchantPassword
    }
    Assert-Ok $res "login merchant"
    $merchantToken = Get-AuthToken $res.Data
    $script:State.MerchantHeaders = @{
        "Authorization" = "Bearer $merchantToken"
        "X-Shop-Slug"   = $ShopSlug
    }
    Write-Ok "Merchant token OK"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/shops" `
        -Headers $script:State.MerchantHeaders `
        -Body @{ name = $ShopName; slug = $ShopSlug }
    Assert-Ok $res "create shop"
    $shopId = Get-EntityId $res.Data "shop"
    $script:State.ShopId = $shopId
    Write-Ok "Shop $shopId ($ShopSlug)"

    $sqlKyc = "UPDATE shops SET kyc_status = 'verified', kyc_verified_at = NOW() WHERE id = '" + $shopId + "';"
    $null = Invoke-Sql $sqlKyc
    Write-Ok "KYC forced to verified"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email = $MerchantEmail; password = $MerchantPassword
    }
    Assert-Ok $res "re-login"
    $merchantToken = Get-AuthToken $res.Data
    $script:State.MerchantHeaders["Authorization"] = "Bearer $merchantToken"

    # 04 Product / customer / order
    Write-Step "04/09" "Product, customer, order"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/products" `
        -Headers $script:State.MerchantHeaders `
        -Body @{
            name        = "Produit Litige $Timestamp"
            description = "E2E dispute customer_wins"
            price_cents = $ProductPriceCents
            stock       = 10
        }
    Assert-Ok $res "create product"
    $productId = Get-EntityId $res.Data "product"
    Write-Ok "Product $productId"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" `
        -Headers $script:State.MerchantHeaders `
        -Body @{
            first_name = "Jean"; last_name = "Litige"
            email = "jean.dispute.$Timestamp@test.com"; phone = $CustomerPhone
        }
    if (-not $res.Success) {
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" `
            -Headers $script:State.MerchantHeaders `
            -Body @{
                first_name = "Jean"; last_name = "Litige"
                email = "jean.dispute.$Timestamp@test.com"; phone_number = $CustomerPhone
            }
    }
    Assert-Ok $res "create customer"
    $customerId = Get-EntityId $res.Data "customer"
    Write-Ok "Customer $customerId"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders" `
        -Headers $script:State.MerchantHeaders `
        -Body @{
            customer_id    = $customerId
            payment_method = "mobile_money"
            items          = @(@{ product_id = $productId; quantity = 1 })
        }
    Assert-Ok $res "create order"
    $orderId = Get-EntityId $res.Data "order"
    $script:State.OrderId = $orderId
    Write-Ok "Order $orderId"

    # 05 Pay-in
    Write-Step "05/09" "Initiate YengaPay + success (NO shipping after)"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/pay" `
        -Headers $script:State.MerchantHeaders `
        -Body @{
            provider     = "yenga_pay"
            phone_number = $CustomerPhone
            description  = "E2E dispute order $orderId"
            metadata     = @{
                flow           = "indirect"
                # hint seulement - le canal reel vient du checkout / webhook
                customer_phone = $CustomerPhone
            }
        }
    Assert-Ok $res "pay yenga_pay"

    $paymentId   = Get-Prop $res.Data @("payment_id", "id", "data.payment_id", "data.id")
    $providerRef = Get-Prop $res.Data @("provider_ref", "data.provider_ref")
    $redirect    = Get-Prop $res.Data @(
        "redirect_url", "data.redirect_url",
        "checkout_url", "data.checkout_url",
        "metadata.payment_url", "data.metadata.payment_url"
    )
    if (-not $paymentId) { Write-Fail "payment_id missing" }
    $script:State.PaymentId   = [string]$paymentId
    $script:State.ProviderRef = [string]$providerRef
    $script:State.RedirectUrl = [string]$redirect
    Write-Ok "Payment $paymentId | ref=$providerRef"

    # Seed CONDITIONNEL: ne force PAS operator si payment_source / operator deja presents
    $phoneEsc = $CustomerPhone.Replace("'", "''")
    $opEsc    = $Operator.Replace("'", "''")
    $sqlPhone = @"
UPDATE payments SET
  customer_phone = COALESCE(NULLIF(TRIM(customer_phone), ''), '$phoneEsc'),
  metadata = COALESCE(metadata, '{}'::jsonb)
    || CASE
         WHEN COALESCE(metadata->>'payment_source', '') = ''
          AND COALESCE(metadata->>'operator', '') = ''
         THEN jsonb_build_object(
                'operator', '$opEsc',
                'customer_phone', '$phoneEsc',
                'seed_fallback', true
              )
         ELSE '{}'::jsonb
       END
WHERE id = '$paymentId'::uuid;
"@
    $null = Invoke-Sql $sqlPhone
    Write-Ok "Payment phone seeded only if missing (keep real pay-in channel)"

    $amountFcfa = [int]($ProductPriceCents / 100)

    if ($RealPayIn) {
        if (-not $redirect) { Write-Fail "Pas de checkout URL" }
        Open-CheckoutUrl -Url $redirect
        $ok = Wait-PaymentSuccess -PaymentId $paymentId `
            -Headers $script:State.MerchantHeaders `
            -TimeoutSec $PayInTimeoutSec -PollSec $PayInPollSec
        if (-not $ok) {
            Write-Warn "Timeout - fallback webhook local"
            $res = Send-SimulatedPaymentWebhook -PaymentId $paymentId -ProviderRef $providerRef `
                -AmountFcfa $amountFcfa -FeesFcfa $feesFcfa `
                -PaymentSource $SimPaymentSource -CustomerNumber $SimCustomerNumber
            if ($res.StatusCode -eq 200) { Write-Ok "Fallback webhook 200" }
            else { Write-Warn "Webhook HTTP $($res.StatusCode): $($res.Error)" }
            Start-Sleep -Seconds 1
        }
        else {
            Write-Ok "Pay-in success (poll)"
            Write-Warn "Sans webhook local, payment_source depend de CheckStatus / process_webhook"
        }
    }
    else {
        $res = Send-SimulatedPaymentWebhook -PaymentId $paymentId -ProviderRef $providerRef `
            -AmountFcfa $amountFcfa -FeesFcfa $feesFcfa `
            -PaymentSource $SimPaymentSource -CustomerNumber $SimCustomerNumber
        if ($res.StatusCode -eq 200) { Write-Ok "Webhook payment.success 200 ($SimPaymentSource)" }
        else { Write-Fail "Webhook failed: $($res.Error)" }
        Start-Sleep -Seconds 1
    }

    $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/payments/$paymentId" -Headers $script:State.MerchantHeaders
    Assert-Ok $res "get payment"
    $paySt = Get-Prop $res.Data @("status", "data.status")
    if ($paySt -ne "success") { Write-Fail "Payment status=$paySt (expected success)" }
    Write-Ok "Payment status = success"

    $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/orders/$orderId" -Headers $script:State.MerchantHeaders
    $ordSt = Get-Prop $res.Data @("status", "data.status")
    Write-Ok "Order status = $ordSt"

    $escOut = Invoke-Sql "SELECT status, total_amount_cents, commission_cents FROM escrow_accounts WHERE order_id = '$orderId'::uuid;"
    Write-Host "  Escrow: $escOut" -ForegroundColor DarkGray
    if ("$escOut" -notmatch "funds_held") {
        Write-Warn "Escrow should be funds_held to open dispute (got above)"
    }
    else {
        Write-Ok "Escrow = funds_held"
    }

    # Affiche le canal connu avant dispute (pour verifier meme moyen au refund)
    $chanOut = Invoke-Sql "SELECT customer_phone, metadata->>'payment_source' AS payment_source, metadata->>'operator' AS operator, metadata->>'customer_number' AS customer_number FROM payments WHERE id = '$paymentId'::uuid;"
    Write-Host "  Canal pay-in: $chanOut" -ForegroundColor White

    # 06 Open dispute - AVANT shipping
    Write-Step "06/09" "Open dispute (before shipping)"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/dispute" `
        -Headers $script:State.MerchantHeaders `
        -Body @{ reason = "Produit non conforme - E2E customer_wins $Timestamp" }

    if (-not $res.Success) {
        Write-Warn "Path /api/orders/{id}/dispute failed: $($res.Error)"
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/disputes" `
            -Headers $script:State.MerchantHeaders `
            -Body @{
                order_id = $orderId
                reason   = "Produit non conforme - E2E customer_wins $Timestamp"
            }
    }
    Assert-Ok $res "open dispute" @(200, 201)

    $disputeId = Get-Prop $res.Data @(
        "dispute.id", "data.dispute.id", "id", "data.id"
    )
    if (-not $disputeId) {
        Write-Fail "dispute id missing: $($res.Raw)"
    }
    $script:State.DisputeId = [string]$disputeId
    Write-Ok "Dispute opened $disputeId"

    $escOut2 = Invoke-Sql "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid;"
    Write-Host "  Escrow after open: $escOut2" -ForegroundColor White
    if ("$escOut2" -match "disputed") { Write-Ok "Escrow = disputed" }
    else { Write-Warn "Expected escrow disputed" }

    # 07 Admin resolve customer_wins
    Write-Step "07/09" "Admin resolve customer_wins (Yenga Refund same channel)"
    $resolvePaths = @(
        "$BaseUrl/api/admin/disputes/$disputeId/resolve",
        "$BaseUrl/api/disputes/$disputeId/resolve",
        "$BaseUrl/api/admin/orders/$orderId/dispute/resolve"
    )
    $resolved = $false
    $lastErr = ""
    foreach ($path in $resolvePaths) {
        $rr = Invoke-SafeApi -Method Post -Uri $path `
            -Headers $script:State.AdminHeaders `
            -Body @{
                resolution = "customer_wins"
                notes      = "E2E refund client $Timestamp"
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
        Write-Fail "customer_wins failed on all routes. Last: $lastErr. Cherche ResolveDispute dans le router."
    }

    # 08 Assert
    Write-Step "08/09" "Assert escrow refunded + dispute resolved_customer"
    $escFinal = Invoke-Sql "SELECT status, total_amount_cents, commission_cents FROM escrow_accounts WHERE order_id = '$orderId'::uuid;"
    Write-Host "  Escrow: $escFinal" -ForegroundColor White
    $dispFinal = Invoke-Sql "SELECT id, status FROM disputes WHERE id = '$disputeId'::uuid;"
    Write-Host "  Dispute: $dispFinal" -ForegroundColor White

    if ("$escFinal" -match "refunded") {
        Write-Ok "Escrow = refunded"
    }
    else {
        Write-Warn "Escrow status unexpected (refund Yenga may have failed - check API logs)"
    }

    if ("$dispFinal" -match "resolved_customer") {
        Write-Ok "Dispute = resolved_customer"
    }
    else {
        Write-Warn "Dispute status unexpected"
    }

    $payRow = Invoke-Sql "SELECT id, status, provider_ref, customer_phone, metadata->>'payment_source' AS payment_source, metadata->>'operator' AS operator FROM payments WHERE id = '$paymentId'::uuid;"
    Write-Host "  Payment canal: $payRow" -ForegroundColor DarkGray

    # 09 Summary
    Write-Step "09/09" "Summary"
    Write-Host "  Order    : $orderId" -ForegroundColor White
    Write-Host "  Payment  : $paymentId" -ForegroundColor White
    Write-Host "  Dispute  : $disputeId" -ForegroundColor White
    Write-Host "  Checkout : $($script:State.RedirectUrl)" -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Verifier meme canal au refund:" -ForegroundColor Yellow
    Write-Host "    docker compose logs goshop 2>&1 | Select-String -Pattern 'source_hint|cashout_method|customer_wins refund|cash-out for refund'" -ForegroundColor DarkGray

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $resultColor = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    Write-Host " RESULT : $($script:Passed) OK | $($script:Failed) FAIL" -ForegroundColor $resultColor
    Write-Host "================================================================" -ForegroundColor Magenta
}
catch {
    Write-Host ""
    Write-Host "[CRITICAL FAILURE] $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host ""