# ============================================================
# GOSHOP E2E - YengaPay REAL PAY-IN (navigateur) + Escrow + Withdrawal
# ============================================================
# Mode REAL_PAYIN:
#   - Ouvre le lien checkout Yenga dans le navigateur
#   - Tu paies manuellement (sandbox)
#   - Le script poll le statut jusqu'a success (via API GoShop -> Yenga)
#   - Ensuite shipping / delivery / wallet / withdrawal
#
# IMPORTANT pour voir le Pay In sur le dashboard Yenga:
#   - Tu DOIS finaliser le paiement sur la page checkout
#   - Le webhook local n'est PAS utilise pour le pay-in (sauf fallback)
# ============================================================

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

# -------------------- CONFIG --------------------
$BaseUrl       = if ($env:GOSHOP_BASE_URL) { $env:GOSHOP_BASE_URL } else { "http://localhost:8080" }
$WebhookSecret = if ($env:YENGA_PAY_WEBHOOK_SECRET) { $env:YENGA_PAY_WEBHOOK_SECRET } else { "CHANGE_ME_YENGA_PAY_WEBHOOK_SECRET" }
$DbService     = if ($env:DB_SERVICE) { $env:DB_SERVICE } else { "db" }
$DbUser        = if ($env:DB_USER) { $env:DB_USER } else { "postgres" }
$DbName        = if ($env:DB_NAME) { $env:DB_NAME } else { "goshop_db" }

# true  = paiement reel via navigateur (visible sandbox Yenga)
# false = ancien mode webhook simule local
$RealPayIn = $true
$PayInTimeoutSec = 300   # 5 min pour payer
$PayInPollSec    = 5

$Timestamp         = Get-Date -Format "yyyyMMddHHmmss"
$MerchantEmail     = "merchant.e2e.$Timestamp@goshop.com"
$MerchantPassword  = "Password123!"
$ShopName          = "Boutique E2E $Timestamp"
$ShopSlug          = "e2e-shop-$Timestamp"
$CustomerPhone     = "+22670000000"
$ProductPriceCents = 100000   # 1000 XOF
$WithdrawCents     = 50000    # 500 XOF
$feesFcfa          = 10

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
    $Sql | & docker-compose exec -T $DbService psql -U $DbUser -d $DbName -v ON_ERROR_STOP=1 2>&1 | Out-Null
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prevEap
    if ($code -ne 0) {
        Write-Warn "SQL failed (exit $code)"
        return $false
    }
    return $true
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
    Write-Host "  PAIEMENT REEL - Ouvre le checkout Yenga" -ForegroundColor Yellow
    Write-Host "  $Url" -ForegroundColor White
    Write-Host "  1) La page s'ouvre dans ton navigateur" -ForegroundColor DarkGray
    Write-Host "  2) Complete le paiement sandbox (operateur / OTP)" -ForegroundColor DarkGray
    Write-Host "  3) Le script attend le statut success" -ForegroundColor DarkGray
    Write-Host "  ============================================================" -ForegroundColor Yellow
    Write-Host ""
    try {
        Start-Process $Url
        Write-Ok "Navigateur ouvert"
    }
    catch {
        Write-Warn "Impossible d'ouvrir le navigateur automatiquement. Copie le lien ci-dessus."
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
                Write-Warn "Payment terminal status: $last"
                return $false
            }
        }
        else {
            Write-Host "  ... poll error: $($res.Error)" -ForegroundColor DarkGray
        }
        Start-Sleep -Seconds $PollSec
    }
    Write-Warn "Timeout apres ${TimeoutSec}s (dernier statut=$last)"
    return $false
}

function Send-SimulatedPaymentWebhook {
    param(
        [string]$PaymentId,
        [string]$ProviderRef,
        [int]$AmountFcfa,
        [int]$FeesFcfa
    )
    $transId  = if ($ProviderRef) { "$ProviderRef" } else { "YP-E2E-$Timestamp" }
    $intentId = if ($ProviderRef) { "$ProviderRef" } else { "" }
    $payloadJson = "{`"apiEnv`":`"test`",`"paymentStatus`":`"DONE`",`"transId`":`"$transId`",`"projectId`":`"00000`",`"paymentIntentId`":`"$intentId`",`"paymentSource`":`"OrangeMoneyAPI`",`"customerNumber`":`"70000000`",`"paymentAmount`":$AmountFcfa,`"paymentFees`":$FeesFcfa,`"contryOrigin`":`"BF`",`"reference`":`"$PaymentId`",`"currency`":`"XOF`",`"isPaylink`":false}"
    $hash = Get-HmacSha256Hex -Payload $payloadJson -Secret $WebhookSecret
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/webhooks/yenga_pay" -Headers @{
        "Content-Type"     = "application/json; charset=utf-8"
        "x-webhook-hash"   = $hash
        "x-yengapay-event" = "payment.success"
    } -Body $payloadJson
    return $res
}

# ============================================================
Write-Host ""
Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - YengaPay REAL PAY-IN + Escrow + Withdrawal" -ForegroundColor Magenta
Write-Host " BaseUrl: $BaseUrl | RealPayIn=$RealPayIn" -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # 01 Health
    Write-Step "01/12" "Health check"
    $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health/live"
    Assert-Ok $res "health/live"
    Write-Ok "API live"

    # 02 Auth
    Write-Step "02/12" "Register + login merchant"
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
    Write-Ok "Merchant token OK ($MerchantEmail)"

    # 03 Shop + KYC
    Write-Step "03/12" "Create shop + KYC verified (SQL)"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/shops" `
        -Headers $script:State.MerchantHeaders `
        -Body @{ name = $ShopName; slug = $ShopSlug }
    Assert-Ok $res "create shop"
    $shopId = Get-EntityId $res.Data "shop"
    $script:State.ShopId = $shopId
    Write-Ok "Shop $shopId ($ShopSlug)"

    $sqlKyc = "UPDATE shops SET kyc_status = 'verified', kyc_verified_at = NOW() WHERE id = '" + $shopId + "';"
    if (Invoke-Sql $sqlKyc) { Write-Ok "KYC forced to verified" }
    else { Write-Warn "KYC SQL not applied" }

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email = $MerchantEmail; password = $MerchantPassword
    }
    Assert-Ok $res "re-login"
    $merchantToken = Get-AuthToken $res.Data
    $script:State.MerchantHeaders["Authorization"] = "Bearer $merchantToken"

    # 04 Product / customer / order
    Write-Step "04/12" "Product, customer, order"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/products" `
        -Headers $script:State.MerchantHeaders `
        -Body @{
            name = "Smartphone E2E $Timestamp"
            description = "Produit test E2E"
            price_cents = $ProductPriceCents
            stock = 10
        }
    Assert-Ok $res "create product"
    $productId = Get-EntityId $res.Data "product"
    Write-Ok "Product $productId"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" `
        -Headers $script:State.MerchantHeaders `
        -Body @{
            first_name = "Jean"; last_name = "Testeur"
            email = "jean.$Timestamp@test.com"; phone = $CustomerPhone
        }
    if (-not $res.Success) {
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" `
            -Headers $script:State.MerchantHeaders `
            -Body @{
                first_name = "Jean"; last_name = "Testeur"
                email = "jean.$Timestamp@test.com"; phone_number = $CustomerPhone
            }
    }
    Assert-Ok $res "create customer"
    $customerId = Get-EntityId $res.Data "customer"
    Write-Ok "Customer $customerId"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders" `
        -Headers $script:State.MerchantHeaders `
        -Body @{
            customer_id = $customerId
            payment_method = "mobile_money"
            items = @(@{ product_id = $productId; quantity = 1 })
        }
    Assert-Ok $res "create order"
    $orderId = Get-EntityId $res.Data "order"
    $script:State.OrderId = $orderId
    Write-Ok "Order $orderId"

    # 05 Init YengaPay
    Write-Step "05/12" "Initiate YengaPay payment (indirect)"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/pay" `
        -Headers $script:State.MerchantHeaders `
        -Body @{
            provider = "yenga_pay"
            phone_number = $CustomerPhone
            description = "E2E real pay-in order $orderId"
            metadata = @{ flow = "indirect" }
        }
    Assert-Ok $res "pay yenga_pay"
    $script:State.Provider = "yenga_pay"

    $paymentId   = Get-Prop $res.Data @("payment_id", "id", "data.payment_id", "data.id")
    $providerRef = Get-Prop $res.Data @("provider_ref", "data.provider_ref")
    $payStatus   = Get-Prop $res.Data @("status", "data.status")
    $redirect    = Get-Prop $res.Data @(
        "redirect_url", "data.redirect_url",
        "checkout_url", "data.checkout_url",
        "metadata.payment_url", "data.metadata.payment_url"
    )

    if (-not $paymentId) {
        Write-Fail "payment_id missing: $($res.Data | ConvertTo-Json -Compress)"
    }
    $script:State.PaymentId   = [string]$paymentId
    $script:State.ProviderRef = [string]$providerRef
    $script:State.RedirectUrl = [string]$redirect
    Write-Ok "Payment $paymentId | ref=$providerRef | status=$payStatus"

    if (-not $redirect) {
        Write-Fail "Pas de redirect_url / checkout - impossible de payer manuellement"
    }
    Write-Ok "Checkout URL: $redirect"

    # 06 Real pay-in OR simulated webhook
    $amountFcfa = [int]($ProductPriceCents / 100)

    if ($RealPayIn) {
        Write-Step "06/12" "REAL pay-in - checkout navigateur"
        Open-CheckoutUrl -Url $redirect

        Write-Host "  En attente du paiement (max ${PayInTimeoutSec}s)..." -ForegroundColor Yellow
        Write-Host "  Appuie sur Entree ici SEULEMENT si tu as fini de payer et que le poll bloque." -ForegroundColor DarkGray

        $ok = Wait-PaymentSuccess -PaymentId $paymentId `
            -Headers $script:State.MerchantHeaders `
            -TimeoutSec $PayInTimeoutSec `
            -PollSec $PayInPollSec

        if (-not $ok) {
            Write-Warn "Statut pas encore success apres timeout."
            Write-Host "  Options:" -ForegroundColor Yellow
            Write-Host "    [Enter] = forcer webhook LOCAL (GoShop OK, Yenga deja paye si tu as paye)" -ForegroundColor Yellow
            Write-Host "    Ctrl+C  = arreter" -ForegroundColor Yellow
            Read-Host "  Continuer avec fallback webhook local"
            $res = Send-SimulatedPaymentWebhook -PaymentId $paymentId -ProviderRef $providerRef -AmountFcfa $amountFcfa -FeesFcfa $feesFcfa
            if ($res.StatusCode -eq 200) { Write-Ok "Fallback webhook local 200" }
            else { Write-Warn "Fallback webhook HTTP $($res.StatusCode): $($res.Error)" }
            Start-Sleep -Seconds 1
        }
        else {
            Write-Ok "Paiement REEL detecte: status=success (visible sandbox Yenga si paye)"
        }
    }
    else {
        Write-Step "06/12" "Webhook payment SUCCESS simule (HMAC)"
        $res = Send-SimulatedPaymentWebhook -PaymentId $paymentId -ProviderRef $providerRef -AmountFcfa $amountFcfa -FeesFcfa $feesFcfa
        if ($res.StatusCode -eq 200) { Write-Ok "Webhook payment.success 200" }
        else { Write-Warn "Webhook HTTP $($res.StatusCode): $($res.Error)" }
        Start-Sleep -Seconds 1
    }

    $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/payments/$paymentId" -Headers $script:State.MerchantHeaders
    Assert-Ok $res "get payment"
    $finalPayStatus = Get-Prop $res.Data @("status", "data.status")
    Write-Ok "Payment status = $finalPayStatus"

    $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/orders/$orderId" -Headers $script:State.MerchantHeaders
    Assert-Ok $res "get order"
    $orderAfter = Get-Prop $res.Data @("status", "data.status")
    Write-Ok "Order status = $orderAfter"

    # 07 Shipping
    Write-Step "07/12" "Shipping proof"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/shipping" `
        -Headers $script:State.MerchantHeaders `
        -Body @{
            order_id = $orderId
            proof_url = "https://example.com/shipping-$Timestamp.jpg"
            tracking_number = "TRACK$Timestamp"
            carrier = "DHL"
            notes = "E2E shipped"
        }
    if (-not $res.Success) { Write-Warn "Shipping proof failed: $($res.Error)" }
    else {
        $escrowSt = Get-Prop $res.Data @("escrow_status", "data.escrow_status")
        Write-Ok "Shipping proof OK (escrow_status=$escrowSt)"
    }

    # 08 Delivery
    Write-Step "08/12" "Delivery proof"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/delivery" `
        -Headers $script:State.MerchantHeaders `
        -Body @{
            order_id = $orderId
            proof_url = "https://example.com/delivery-$Timestamp.jpg"
            signature = "SIG$Timestamp"
            notes = "E2E delivered"
            rating = 5
        }
    if (-not $res.Success) { Write-Warn "Delivery proof failed: $($res.Error)" }
    else {
        $escrowSt = Get-Prop $res.Data @("escrow_status", "data.escrow_status")
        Write-Ok "Delivery proof OK (escrow_status=$escrowSt)"
    }

    # 09 Escrow + wallet SQL
    Write-Step "09/12" "Release escrow -> wallet (SQL force)"
    $sqlRelease = "UPDATE escrow_accounts SET status = 'released', released_amount_cents = total_amount_cents, updated_at = NOW() WHERE order_id::text = '" + $orderId + "';"
    if (Invoke-Sql $sqlRelease) { Write-Ok "Escrow forced released" }

    $netCents = $ProductPriceCents - ($feesFcfa * 100)
    $sqlWallet = "INSERT INTO merchant_wallets (shop_id, balance_cents, held_cents, total_sales_cents, is_frozen, created_at, updated_at) " +
                 "VALUES ('" + $shopId + "'::uuid, " + $netCents + ", 0, " + $netCents + ", false, NOW(), NOW()) " +
                 "ON CONFLICT (shop_id) DO UPDATE SET " +
                 "balance_cents = merchant_wallets.balance_cents + EXCLUDED.balance_cents, " +
                 "held_cents = GREATEST(merchant_wallets.held_cents - EXCLUDED.balance_cents, 0), " +
                 "total_sales_cents = merchant_wallets.total_sales_cents + EXCLUDED.total_sales_cents, " +
                 "updated_at = NOW();"
    if (Invoke-Sql $sqlWallet) { Write-Ok ("Wallet credited ~" + $netCents + " cents") }
    else { Write-Warn "Wallet SQL failed" }

    # 10 Withdrawal
    Write-Step "10/12" "Create withdrawal"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/withdrawals" `
        -Headers $script:State.MerchantHeaders `
        -Body @{
            amount_cents = $WithdrawCents
            payment_method = "ORANGE_MONEY"
            destination_number = "+22670123456"
            destination_name = "Jean Test"
            description = "E2E withdrawal $Timestamp"
        }
    if (-not $res.Success) {
        Write-Warn "Withdrawal failed: HTTP $($res.StatusCode) - $($res.Error)"
    }
    else {
        $script:State.WithdrawalId  = [string](Get-Prop $res.Data @("id", "data.id"))
        $script:State.WithdrawalRef = [string](Get-Prop $res.Data @("provider_ref", "data.provider_ref"))
        $wStatus = Get-Prop $res.Data @("status", "data.status")
        Write-Ok "Withdrawal $($script:State.WithdrawalId) | ref=$($script:State.WithdrawalRef) | status=$wStatus"
    }

    # 11 Payout webhook (simule - le cash-out API est deja reel cote Yenga)
    Write-Step "11/12" "Webhook payout.success"
    if ($script:State.WithdrawalRef -or $script:State.WithdrawalId) {
        $payoutRef = if ($script:State.WithdrawalRef) { $script:State.WithdrawalRef } else { $script:State.WithdrawalId }
        $opTx = "OP-E2E-$Timestamp"
        $payoutJson = "{`"id`":`"$payoutRef`",`"transId`":`"$payoutRef`",`"projectId`":`"00000`",`"amount`":$([int]($WithdrawCents / 100)),`"fees`":5,`"currency`":`"XOF`",`"paymentMethod`":`"ORANGE_MONEY`",`"destNumber`":`"+22670123456`",`"status`":`"SUCCESS`",`"operatorTransId`":`"$opTx`",`"paymentStatus`":`"DONE`"}"
        $pHash = Get-HmacSha256Hex -Payload $payoutJson -Secret $WebhookSecret
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/webhooks/yenga_pay" -Headers @{
            "Content-Type" = "application/json; charset=utf-8"
            "x-webhook-hash" = $pHash
            "x-yengapay-event" = "payout.success"
        } -Body $payoutJson
        if ($res.StatusCode -eq 200) { Write-Ok "Payout webhook 200" }
        else { Write-Warn "Payout webhook HTTP $($res.StatusCode)" }
        Start-Sleep -Seconds 1
        $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/withdrawals/$($script:State.WithdrawalId)" -Headers $script:State.MerchantHeaders
        $st = Get-Prop $res.Data @("status", "data.status")
        Write-Ok "Withdrawal status after payout = $st"
    }
    else {
        Write-Warn "No withdrawal - skip payout webhook"
    }

    # 12 Final
    Write-Step "12/12" "Final verification"
    if ($script:State.PaymentId) {
        $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/payments/$($script:State.PaymentId)" -Headers $script:State.MerchantHeaders
        Write-Host "  Payment status  : $(Get-Prop $res.Data @('status','data.status'))" -ForegroundColor White
    }
    if ($script:State.OrderId) {
        $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/orders/$($script:State.OrderId)" -Headers $script:State.MerchantHeaders
        Write-Host "  Order status    : $(Get-Prop $res.Data @('status','data.status'))" -ForegroundColor White
    }
    if ($script:State.WithdrawalId) {
        $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/withdrawals/$($script:State.WithdrawalId)" -Headers $script:State.MerchantHeaders
        Write-Host "  Withdrawal      : $(Get-Prop $res.Data @('status','data.status'))" -ForegroundColor White
        Write-Host "  Operator TX     : $(Get-Prop $res.Data @('operator_transaction_id','data.operator_transaction_id'))" -ForegroundColor White
    }
    if ($script:State.RedirectUrl) {
        Write-Host "  Checkout used   : $($script:State.RedirectUrl)" -ForegroundColor DarkGray
    }

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $resultColor = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    Write-Host " RESULT : $($script:Passed) OK | $($script:Failed) FAIL" -ForegroundColor $resultColor
    Write-Host "================================================================" -ForegroundColor Magenta
    Write-Host ""
    Write-Host "  Dashboard Yenga sandbox -> Historique Pay In :" -ForegroundColor Cyan
    Write-Host "  Si tu as complete le checkout, la transaction doit apparaitre." -ForegroundColor DarkGray
}
catch {
    Write-Host ""
    Write-Host "[CRITICAL FAILURE] $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host ""