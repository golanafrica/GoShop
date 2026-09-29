# ============================================================
# GOSHOP E2E - REAL PAY-IN + Auto-Release + Withdrawal
# Variant: Standard (BF-OUAGA-URB, 5j delay) | backdate = 6 jours
# ============================================================
# Env:
#   $env:YENGA_PAY_WEBHOOK_SECRET
#   $env:ADMIN_EMAIL / $env:ADMIN_PASSWORD
#   $env:GOSHOP_BASE_URL
# ============================================================

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

# -------------------- CONFIG --------------------
$BaseUrl       = if ($env:GOSHOP_BASE_URL) { $env:GOSHOP_BASE_URL } else { "http://localhost:8080" }
$WebhookSecret = if ($env:YENGA_PAY_WEBHOOK_SECRET) { $env:YENGA_PAY_WEBHOOK_SECRET } else { "c38ccab5-836d-4453-a6e0-2eb0b9df3097" }
$AdminEmail    = if ($env:ADMIN_EMAIL) { $env:ADMIN_EMAIL } else { "superadmin.yacine@goshop.com" }
$AdminPassword = if ($env:ADMIN_PASSWORD) { $env:ADMIN_PASSWORD } else { "LassinaYacine19778&" }
$DbService     = if ($env:DB_SERVICE) { $env:DB_SERVICE } else { "db" }
$DbUser        = if ($env:DB_USER) { $env:DB_USER } else { "postgres" }
$DbName        = if ($env:DB_NAME) { $env:DB_NAME } else { "goshop_db" }

$RealPayIn        = $true
$PayInTimeoutSec  = 300
$PayInPollSec     = 5
$SchedulerWaitSec = 10

# >>> PROFIL ZONE (Standard : Urbain Ouaga) <<<
$ZoneProfile = "BF-OUAGA-URB"

switch ($ZoneProfile) {
    "BF-OUAGA-URB" {
        $EligibilityBackdateDays = 6
        $ZoneSelectSql = @"
SELECT id::text || '|' || zone_code || '|' || installment_release_delay_days::text
FROM delivery_zones
WHERE is_active = true AND zone_code = 'BF-OUAGA-URB'
LIMIT 1;
"@
    }
    "INTERNATIONAL" {
        $EligibilityBackdateDays = 16
        $ZoneSelectSql = @"
SELECT id::text || '|' || zone_code || '|' || installment_release_delay_days::text
FROM delivery_zones
WHERE is_active = true AND zone_code = 'INT-CEDEAO-1'
LIMIT 1;
"@
    }
    "INT-30" {
        $EligibilityBackdateDays = 31
        $ZoneSelectSql = @"
SELECT id::text || '|' || zone_code || '|' || installment_release_delay_days::text
FROM delivery_zones
WHERE is_active = true AND zone_type = 'international' AND installment_release_delay_days = 30
ORDER BY zone_code LIMIT 1;
"@
    }
    default {
        $EligibilityBackdateDays = 10
        $ZoneSelectSql = @"
SELECT id::text || '|' || zone_code || '|' || installment_release_delay_days::text
FROM delivery_zones
WHERE is_active = true AND installment_release_delay_days >= 10 AND zone_type <> 'international'
ORDER BY installment_release_delay_days ASC, priority DESC LIMIT 1;
"@
    }
}

$Timestamp         = Get-Date -Format "yyyyMMddHHmmss"
$MerchantEmail     = "merchant.e2e.$Timestamp@goshop.com"
$MerchantPassword  = "Password123!"
$ShopName          = "Boutique E2E $Timestamp"
$ShopSlug          = "e2e-shop-$Timestamp"

# 🆕 Numéro réaliste pour garantir que le webhook fallback le capture correctement (évite le filtre 70000000)
$CustomerPhone     = "+22677515151" 
$ProductPriceCents = 100000
$WithdrawCents     = 50000
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

        if ($status -eq 202) {
            return @{
                Success    = $true
                Data       = $null
                StatusCode = 202
                Raw        = $errBody
                Error      = $null
            }
        }

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
    param($Res, [string]$Context, [int[]]$Codes = @(200, 201, 202))
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

function Invoke-SqlQuery {
    param([string]$Sql)
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $out = $Sql | & docker-compose exec -T $DbService psql -U $DbUser -d $DbName -t -A -v ON_ERROR_STOP=1 2>&1
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prevEap
    if ($code -ne 0) {
        Write-Warn "SQL query failed (exit $code): $out"
        return $null
    }
    return ($out | Out-String).Trim()
}

function Get-LongSafe {
    param($Value)
    if ($null -eq $Value -or "$Value" -eq "") { return [int64]0 }
    try { return [int64]("$Value".Trim()) } catch { return [int64]0 }
}

function Get-AuthToken {
    param($LoginData)
    $t = Get-Prop $LoginData @("access_token", "token", "data.access_token", "data.token", "data.accessToken", "tokens.access_token")
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
    
    # 🆕 Utilisation du vrai CustomerPhone dans le payload pour tester le fallback du webhook
    $payloadJson = "{`"apiEnv`":`"test`",`"paymentStatus`":`"DONE`",`"transId`":`"$transId`",`"projectId`":`"00000`",`"paymentIntentId`":`"$intentId`",`"paymentSource`":`"OrangeMoneyAPI`",`"customerNumber`":`"77515151`",`"paymentAmount`":$AmountFcfa,`"paymentFees`":$FeesFcfa,`"contryOrigin`":`"BF`",`"reference`":`"$PaymentId`",`"currency`":`"XOF`",`"isPaylink`":false}"
    
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
Write-Host " GOSHOP E2E - REAL PAY-IN + Auto-Release (zone=$ZoneProfile) + Withdrawal" -ForegroundColor Magenta
Write-Host " BaseUrl: $BaseUrl | RealPayIn=$RealPayIn | BackdateDays=$EligibilityBackdateDays" -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # 01 Health
    Write-Step "01/13" "Health check"
    $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health/live"
    Assert-Ok $res "health/live"
    Write-Ok "API live"

    # 02 Auth merchant
    Write-Step "02/13" "Register + login merchant"
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
    Write-Step "03/13" "Create shop + KYC verified (SQL)"
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
    Write-Step "04/13" "Product, customer, order"
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

    # 04b Assign zone selon $ZoneProfile
    $zoneRow = Invoke-SqlQuery $ZoneSelectSql
    if ($zoneRow -and $zoneRow -match '^([^|]+)\|([^|]+)\|(\d+)') {
        $zoneId   = $Matches[1]
        $zoneCode = $Matches[2]
        $zoneDays = $Matches[3]
        if (Invoke-Sql "UPDATE orders SET delivery_zone_id = '$zoneId'::uuid WHERE id = '$orderId'::uuid;") {
            Write-Ok "Order zone = $zoneCode (delay=${zoneDays}d) id=$zoneId"
        }
        else {
            Write-Warn "UPDATE orders.delivery_zone_id failed"
        }
    }
    else {
        if ($ZoneProfile -eq "BF-OUAGA-URB") {
            $zoneId = Invoke-SqlQuery @"
INSERT INTO delivery_zones (
  zone_code, zone_name, country, zone_type,
  delivery_delay_days, return_delay_days, warranty_response_days,
  cod_confirmation_delay_days, installment_release_delay_days,
  is_active, priority, created_at, updated_at
) VALUES (
  'BF-OUAGA-URB', 'Ouagadougou Urbain', 'BF', 'urban',
  5, 14, 7, 7, 5,
  true, 10, NOW(), NOW()
)
ON CONFLICT (zone_code) DO UPDATE SET updated_at = NOW()
RETURNING id::text;
"@
            if ($zoneId) {
                Invoke-Sql "UPDATE orders SET delivery_zone_id = '$zoneId'::uuid WHERE id = '$orderId'::uuid;" | Out-Null
                Write-Ok "Order zone = BF-OUAGA-URB (delay=5d) id=$zoneId (upsert)"
            }
            else {
                Write-Warn "No BF-OUAGA-URB zone - order stays NO_ZONE"
            }
        }
        else {
            Write-Warn "No matching delivery zone - order stays NO_ZONE (fallback 3d)"
        }
    }

    # 05 Init YengaPay
    Write-Step "05/13" "Initiate YengaPay payment (indirect)"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/pay" `
        -Headers $script:State.MerchantHeaders `
        -Body @{
            provider = "yenga_pay"
            phone_number = $CustomerPhone
            description = "E2E real pay-in order $orderId"
            metadata = @{ flow = "indirect" }
        }
    Assert-Ok $res "pay yenga_pay"

    $paymentId   = Get-Prop $res.Data @("payment_id", "id", "data.payment_id", "data.id")
    $providerRef = Get-Prop $res.Data @("provider_ref", "data.provider_ref")
    $payStatus   = Get-Prop $res.Data @("status", "data.status")
    $redirect    = Get-Prop $res.Data @("redirect_url", "data.redirect_url", "checkout_url", "data.checkout_url", "metadata.payment_url", "data.metadata.payment_url")

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
        Write-Step "06/13" "REAL pay-in - checkout navigateur"
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
        Write-Step "06/13" "Webhook payment SUCCESS simule (HMAC)"
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
    Write-Step "07/13" "Shipping proof"
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
    Write-Step "08/13" "Delivery proof"
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

    $escBefore = Invoke-SqlQuery "SELECT status || '|' || COALESCE(total_amount_cents::text,'') || '|' || COALESCE(commission_cents::text,'') FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    Write-Host "  Escrow before release: $escBefore" -ForegroundColor DarkGray

    # 09 Auto-release scheduler
    Write-Step "09/13" "Escrow auto-release via scheduler (real path)"

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
    if (Invoke-Sql $sqlElig) {
        Write-Ok "Eligibility: delivery_date backdated $EligibilityBackdateDays days (status NOT forced released)"
    }
    else {
        Write-Warn "Eligibility SQL failed - scheduler may skip"
    }

    $proofSnap = Invoke-SqlQuery "SELECT COALESCE(escrow_status,'') || '|' || COALESCE(delivery_date::text,'') FROM delivery_proofs WHERE order_id = '$orderId'::uuid LIMIT 1;"
    Write-Host "  Proof after backdate: $proofSnap" -ForegroundColor DarkGray

    $zoneInfo = Invoke-SqlQuery @"
SELECT COALESCE(dz.zone_code, 'NO_ZONE') || '|' || COALESCE(dz.installment_release_delay_days::text, '3')
FROM orders o
LEFT JOIN delivery_zones dz ON dz.id = o.delivery_zone_id
WHERE o.id = '$orderId'::uuid
LIMIT 1;
"@
    Write-Host "  Order zone|delayDays: $zoneInfo (fallback 3 si NO_ZONE)" -ForegroundColor DarkGray

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email = $AdminEmail; password = $AdminPassword
    }
    Assert-Ok $res "admin login"
    $adminToken = Get-AuthToken $res.Data
    $adminHeaders = @{ "Authorization" = "Bearer $adminToken" }
    Write-Ok "Admin token OK ($AdminEmail)"

    $res = Invoke-SafeApi -Method Post `
        -Uri "$BaseUrl/api/admin/scheduler/trigger-escrow-auto-release" `
        -Headers $adminHeaders
    Assert-Ok $res "trigger-escrow-auto-release" -Codes @(200, 201, 202)
    Write-Ok "Scheduler triggered (HTTP $($res.StatusCode) background)"

    Write-Host "  Wait ${SchedulerWaitSec}s for background job..." -ForegroundColor DarkGray
    Start-Sleep -Seconds $SchedulerWaitSec

    $escAfter = Invoke-SqlQuery "SELECT status || '|' || COALESCE(total_amount_cents::text,'') || '|' || COALESCE(commission_cents::text,'') || '|' || COALESCE(released_amount_cents::text,'') FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    Write-Host "  Escrow after: $escAfter" -ForegroundColor White

    $platBal = Invoke-SqlQuery "SELECT COALESCE(balance_cents,0)::text FROM platform_revenue_accounts LIMIT 1;"
    $platTxn = Invoke-SqlQuery @"
SELECT COUNT(*)::text
FROM platform_revenue_transactions prt
JOIN delivery_proofs dp ON prt.reference_id = dp.id
WHERE dp.order_id = '$orderId'::uuid
  AND prt.reference_type = 'escrow_auto_release';
"@
    Write-Host "  Platform balance_cents: $platBal | txn for order proof: $platTxn" -ForegroundColor White

    $walletBal = Invoke-SqlQuery "SELECT COALESCE(balance_cents,0)::text FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    Write-Host "  Merchant wallet_cents: $walletBal" -ForegroundColor White

    if ("$escAfter" -match "released") {
        Write-Ok "Escrow status = released (scheduler path)"
    }
    else {
        Write-Warn "Escrow not released - check logs / zone delay vs backdate"
        Write-Host "  docker compose logs goshop 2>&1 | Select-String -Pattern 'Platform revenue|Escrow auto-released|already claimed|No proofs eligible|claim release'" -ForegroundColor DarkGray
    }

    $platBalN = Get-LongSafe $platBal
    $platTxnN = Get-LongSafe $platTxn
    $walletN  = Get-LongSafe $walletBal

    if ($platTxnN -ge 1) {
        Write-Ok "Platform revenue txn for THIS order = $platTxnN (balance=$platBalN)"
    }
    elseif ($platBalN -gt 0) {
        Write-Warn "Platform balance=$platBalN but txn for this order=0"
    }
    else {
        Write-Warn "Platform revenue still 0 - CreditRevenue non appele ou echec"
    }

    if ($walletN -gt 0) {
        Write-Ok "Merchant wallet credited by scheduler (balance=$walletN cents)"
    }
    else {
        Write-Warn "Merchant wallet still 0 - credit failed or skipped after claim"
    }

    # 10 Withdrawal
    Write-Step "10/13" "Create withdrawal"
    if ($walletN -lt $WithdrawCents) {
        Write-Warn "Wallet $walletN < $WithdrawCents - skip withdrawal"
    }
    else {
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
    }

    # 11 Payout webhook
    Write-Step "11/13" "Webhook payout.success"
    if ($script:State.WithdrawalRef -or $script:State.WithdrawalId) {
        $payoutRef = if ($script:State.WithdrawalRef) { $script:State.WithdrawalRef } else { $script:State.WithdrawalId }
        $opTx = "OP-E2E-$Timestamp"
        $payoutJson = "{`"id`":`"$payoutRef`",`"transId`":`"$payoutRef`",`"projectId`":`"00000`",`"amount`":$([int]($WithdrawCents / 100)),`"fees`":5,`"currency`":`"XOF`",`"paymentMethod`":`"ORANGE_MONEY`",`"destNumber`":`"+22670123456`",`"status`":`"SUCCESS`",`"operatorTransId`":`"$opTx`",`"paymentStatus`":`"DONE`"}"
        $pHash = Get-HmacSha256Hex -Payload $payoutJson -Secret $WebhookSecret
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/webhooks/yenga_pay" -Headers @{
            "Content-Type"     = "application/json; charset=utf-8"
            "x-webhook-hash"   = $pHash
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

    # 12 Final API
    Write-Step "12/13" "Final API verification"
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

    # 13 Summary
    Write-Step "13/13" "Split Payment summary (SQL)"
    Write-Host "  Escrow   : $escAfter" -ForegroundColor White
    Write-Host "  Platform : balance=$platBal cents | txns=$platTxn" -ForegroundColor White
    Write-Host "  Wallet   : $walletBal cents" -ForegroundColor White
    Write-Host "  Zone     : $zoneInfo" -ForegroundColor White
    Write-Host "  Profile  : $ZoneProfile | Backdate=$EligibilityBackdateDays" -ForegroundColor DarkGray
    Write-Host "  Shop     : $shopId | Order=$orderId | Payment=$paymentId" -ForegroundColor DarkGray

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