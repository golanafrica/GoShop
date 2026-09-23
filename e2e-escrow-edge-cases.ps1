# ============================================================
# GOSHOP E2E EDGE - Delivery sans shipping + Double release
# ============================================================
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

$BaseUrl       = if ($env:GOSHOP_BASE_URL) { $env:GOSHOP_BASE_URL } else { "http://localhost:8080" }
$WebhookSecret = if ($env:YENGA_PAY_WEBHOOK_SECRET) { $env:YENGA_PAY_WEBHOOK_SECRET } else { "CHANGE_ME_YENGA_PAY_WEBHOOK_SECRET" }
$AdminEmail    = if ($env:ADMIN_EMAIL) { $env:ADMIN_EMAIL } else { "superadmin.yacine@goshop.com" }
$AdminPassword = if ($env:ADMIN_PASSWORD) { $env:ADMIN_PASSWORD } else { "CHANGE_ME_ADMIN_PASSWORD" }
$DbService = if ($env:DB_SERVICE) { $env:DB_SERVICE } else { "db" }
$DbUser    = if ($env:DB_USER) { $env:DB_USER } else { "postgres" }
$DbName    = if ($env:DB_NAME) { $env:DB_NAME } else { "goshop_db" }

$Timestamp         = Get-Date -Format "yyyyMMddHHmmss"
$MerchantEmail     = "edge.e2e.$Timestamp@goshop.com"
$MerchantPassword  = "Password123!"
$ShopName          = "Edge Shop $Timestamp"
$ShopSlug          = "edge-shop-$Timestamp"
$CustomerPhone     = "+22670000000"
$ProductPriceCents = 100000
$feesFcfa          = 25

$script:Passed = 0
$script:Failed = 0

function Write-Ok($m)   { Write-Host "  [OK] $m" -ForegroundColor Green; $script:Passed++ }
function Write-Warn($m) { Write-Host "  [WARN] $m" -ForegroundColor Yellow }
function Write-Fail($m) { Write-Host "  [FAIL] $m" -ForegroundColor Red; $script:Failed++; throw $m }
function Write-Step($n, $m) { Write-Host ""; Write-Host "[$n] $m" -ForegroundColor Cyan }

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
    param([string]$Method, [string]$Uri, [hashtable]$Headers = @{}, [object]$Body = $null)
    try {
        $params = @{ Method = $Method; Uri = $Uri; Headers = $Headers; UseBasicParsing = $true }
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
        return @{ Success = $true; StatusCode = [int]$wr.StatusCode; Data = $data; Error = $null; Raw = $wr.Content }
    }
    catch {
        $status = 0
        $errBody = $_.Exception.Message
        if ($_.ErrorDetails -and $_.ErrorDetails.Message) { $errBody = $_.ErrorDetails.Message }
        $resp = $_.Exception.Response
        if ($resp) {
            try { $status = [int]$resp.StatusCode } catch {}
            try {
                if ($resp -is [System.Net.Http.HttpResponseMessage]) {
                    $errBody = $resp.Content.ReadAsStringAsync().Result
                } else {
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
            return @{ Success = $true; StatusCode = 202; Data = $null; Error = $null; Raw = $errBody }
        }
        return @{ Success = $false; StatusCode = $status; Data = $null; Error = $errBody; Raw = $errBody }
    }
}

function Assert-Ok {
    param($Res, [string]$Context, [int[]]$Codes = @(200, 201, 202))
    if (-not $Res.Success -or ($Codes -notcontains $Res.StatusCode)) {
        Write-Fail "$Context -> HTTP $($Res.StatusCode) - $($Res.Error)"
    }
}

function Get-AuthToken {
    param($LoginData)
    $t = Get-Prop $LoginData @("access_token", "token", "data.access_token", "data.token", "data.accessToken")
    if (-not $t) { Write-Fail "Token missing: $($LoginData | ConvertTo-Json -Compress)" }
    return [string]$t
}

function Get-EntityId {
    param($Data, [string]$Label)
    $id = Get-Prop $Data @("id", "data.id", "data.data.id", "shop.id", "product.id", "customer.id", "order.id")
    if (-not $id) {
        Write-Fail "ID not found for $Label : $($Data | ConvertTo-Json -Depth 6 -Compress)"
    }
    return [string]$id
}

function Invoke-Sql {
    param([string]$Sql)
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $Sql | & docker-compose exec -T $DbService psql -U $DbUser -d $DbName -v ON_ERROR_STOP=1 2>&1 | Out-Null
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prev
    return ($code -eq 0)
}

function Invoke-SqlQuery {
    param([string]$Sql)
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $out = $Sql | & docker-compose exec -T $DbService psql -U $DbUser -d $DbName -t -A -v ON_ERROR_STOP=1 2>&1
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prev
    if ($code -ne 0) { return $null }
    return ($out | Out-String).Trim()
}

function Get-HmacSha256Hex {
    param([string]$Payload, [string]$Secret)
    $hmac = New-Object System.Security.Cryptography.HMACSHA256
    $hmac.Key = [System.Text.Encoding]::UTF8.GetBytes($Secret)
    $hash = $hmac.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($Payload))
    return ([BitConverter]::ToString($hash) -replace '-', '').ToLowerInvariant()
}

Write-Host ""
Write-Host "=== EDGE: delivery sans shipping + double release ===" -ForegroundColor Magenta
Write-Host " BaseUrl=$BaseUrl" -ForegroundColor DarkGray

try {
    # ----- 0 Setup -----
    Write-Step "0" "Setup merchant + shop + order + pay (webhook)"

    $res = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health/live"
    Assert-Ok $res "health"
    Write-Ok "API live"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/register" -Body @{
        email = $MerchantEmail; password = $MerchantPassword
    }
    if (-not $res.Success -and $res.StatusCode -ne 409) {
        Write-Fail "Register: $($res.Error)"
    }
    Write-Ok "Register HTTP $($res.StatusCode)"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email = $MerchantEmail; password = $MerchantPassword
    }
    Assert-Ok $res "login"
    $merchantToken = Get-AuthToken $res.Data
    $headers = @{
        "Authorization" = "Bearer $merchantToken"
        "X-Shop-Slug"   = $ShopSlug
    }
    Write-Ok "Merchant token OK"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/shops" -Headers $headers -Body @{
        name = $ShopName; slug = $ShopSlug
    }
    Assert-Ok $res "create shop"
    $shopId = Get-EntityId $res.Data "shop"
    Write-Ok "Shop $shopId"

    if (-not (Invoke-Sql "UPDATE shops SET kyc_status = 'verified', kyc_verified_at = NOW() WHERE id = '$shopId'::uuid;")) {
        Write-Warn "KYC SQL failed"
    } else {
        Write-Ok "KYC verified"
    }

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email = $MerchantEmail; password = $MerchantPassword
    }
    Assert-Ok $res "re-login"
    $headers["Authorization"] = "Bearer $(Get-AuthToken $res.Data)"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/products" -Headers $headers -Body @{
        name = "Edge Product $Timestamp"
        description = "edge"
        price_cents = $ProductPriceCents
        stock = 10
    }
    Assert-Ok $res "create product"
    $productId = Get-EntityId $res.Data "product"
    Write-Ok "Product $productId"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" -Headers $headers -Body @{
        first_name = "Edge"; last_name = "Test"
        email = "edge.$Timestamp@test.com"; phone = $CustomerPhone
    }
    if (-not $res.Success) {
        $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" -Headers $headers -Body @{
            first_name = "Edge"; last_name = "Test"
            email = "edge.$Timestamp@test.com"; phone_number = $CustomerPhone
        }
    }
    Assert-Ok $res "create customer"
    $customerId = Get-EntityId $res.Data "customer"
    Write-Ok "Customer $customerId"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders" -Headers $headers -Body @{
        customer_id = $customerId
        payment_method = "mobile_money"
        items = @(@{ product_id = $productId; quantity = 1 })
    }
    Assert-Ok $res "create order"
    $orderId = Get-EntityId $res.Data "order"
    Write-Ok "Order $orderId"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/pay" -Headers $headers -Body @{
        provider = "yenga_pay"
        phone_number = $CustomerPhone
        description = "edge pay $orderId"
        metadata = @{ flow = "indirect" }
    }
    Assert-Ok $res "pay"
    $paymentId = [string](Get-Prop $res.Data @("payment_id", "id", "data.payment_id", "data.id"))
    $providerRef = [string](Get-Prop $res.Data @("provider_ref", "data.provider_ref"))
    if (-not $paymentId) { Write-Fail "payment_id missing: $($res.Data | ConvertTo-Json -Compress)" }
    Write-Ok "Payment $paymentId ref=$providerRef"

    $amountFcfa = [int]($ProductPriceCents / 100)
    $payloadJson = "{`"apiEnv`":`"test`",`"paymentStatus`":`"DONE`",`"transId`":`"$providerRef`",`"projectId`":`"00000`",`"paymentIntentId`":`"$providerRef`",`"paymentSource`":`"OrangeMoneyAPI`",`"customerNumber`":`"70000000`",`"paymentAmount`":$amountFcfa,`"paymentFees`":$feesFcfa,`"contryOrigin`":`"BF`",`"reference`":`"$paymentId`",`"currency`":`"XOF`",`"isPaylink`":false}"
    $whash = Get-HmacSha256Hex -Payload $payloadJson -Secret $WebhookSecret
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/webhooks/yenga_pay" -Headers @{
        "x-webhook-hash"   = $whash
        "x-yengapay-event" = "payment.success"
    } -Body $payloadJson
    if ($res.StatusCode -eq 200) { Write-Ok "Payment webhook 200" }
    else { Write-Warn "Payment webhook HTTP $($res.StatusCode): $($res.Error)" }
    Start-Sleep -Seconds 2

    $paySt = Get-Prop (Invoke-SafeApi -Method Get -Uri "$BaseUrl/api/payments/$paymentId" -Headers $headers).Data @("status", "data.status")
    Write-Host "  Payment status after webhook: $paySt" -ForegroundColor DarkGray
    if ("$paySt" -ne "success") {
        Write-Warn "Payment not success yet - edge delivery test may still run; release needs success+escrow"
    }

    # ----- 1 Delivery WITHOUT shipping -----
    Write-Step "1" "EDGE: delivery proof WITHOUT shipping"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/delivery" -Headers $headers -Body @{
        order_id  = $orderId
        proof_url = "https://example.com/delivery-only-$Timestamp.jpg"
        signature = "SIG-NOSHIP"
        notes     = "should fail"
        rating    = 5
    }
    if (-not $res.Success) {
        Write-Ok "Delivery refused (HTTP $($res.StatusCode)): $($res.Error)"
    }
    else {
        Write-Fail "Delivery accepted WITHOUT shipping - BUG"
    }

    $esc = Invoke-SqlQuery "SELECT COALESCE(status,'none') FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    Write-Host "  Escrow status: $esc" -ForegroundColor DarkGray
    if ("$esc" -match "released") { Write-Fail "Escrow released without shipping" }
    else { Write-Ok "Escrow not released (status=$esc)" }

    # ----- 2 Shipping then delivery -----
    Write-Step "2" "Shipping then delivery (happy path for double-release)"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/shipping" -Headers $headers -Body @{
        order_id         = $orderId
        proof_url        = "https://example.com/ship-$Timestamp.jpg"
        tracking_number  = "TRK$Timestamp"
        carrier          = "DHL"
        notes            = "shipped"
    }
    if (-not $res.Success) { Write-Fail "Shipping failed: $($res.Error)" }
    Write-Ok "Shipping OK"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/delivery" -Headers $headers -Body @{
        order_id  = $orderId
        proof_url = "https://example.com/del-$Timestamp.jpg"
        signature = "SIG-OK"
        notes     = "delivered"
        rating    = 5
    }
    if (-not $res.Success) { Write-Fail "Delivery after shipping failed: $($res.Error)" }
    Write-Ok "Delivery OK after shipping"

    $zoneId = Invoke-SqlQuery "SELECT id::text FROM delivery_zones WHERE zone_code = 'BF-OUAGA-URB' LIMIT 1;"
    if ($zoneId) {
        Invoke-Sql "UPDATE orders SET delivery_zone_id = '$zoneId'::uuid WHERE id = '$orderId'::uuid;" | Out-Null
    }
    Invoke-Sql @"
UPDATE delivery_proofs
SET delivery_date = NOW() - INTERVAL '6 days', updated_at = NOW() - INTERVAL '6 days'
WHERE order_id = '$orderId'::uuid AND escrow_status = 'delivered';
UPDATE escrow_accounts
SET updated_at = NOW() - INTERVAL '6 days'
WHERE order_id = '$orderId'::uuid AND status = 'funds_held';
"@ | Out-Null
    Write-Ok "Backdate 6d (BF-OUAGA-URB)"

    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email = $AdminEmail; password = $AdminPassword
    }
    Assert-Ok $res "admin login"
    $adminH = @{ "Authorization" = "Bearer $(Get-AuthToken $res.Data)" }

    # ----- 3 First release -----
    Write-Step "3" "First scheduler release"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/admin/scheduler/trigger-escrow-auto-release" -Headers $adminH
    Assert-Ok $res "trigger1" -Codes @(200, 201, 202)
    Write-Ok "Trigger #1 HTTP $($res.StatusCode)"
    Start-Sleep -Seconds 12

    $esc1  = Invoke-SqlQuery "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    $wal1  = Invoke-SqlQuery "SELECT COALESCE(balance_cents,0)::text FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $txn1  = Invoke-SqlQuery @"
SELECT COUNT(*)::text FROM platform_revenue_transactions prt
JOIN delivery_proofs dp ON prt.reference_id = dp.id
WHERE dp.order_id = '$orderId'::uuid AND prt.reference_type = 'escrow_auto_release';
"@
    Write-Host "  after#1 escrow=$esc1 wallet=$wal1 txn=$txn1" -ForegroundColor White
    if ("$esc1" -ne "released") { Write-Fail "First release: escrow=$esc1" }
    if ([int64]$wal1 -le 0) { Write-Fail "First release: wallet empty" }
    if ([int64]$txn1 -lt 1) { Write-Fail "First release: no platform txn" }
    Write-Ok "First release OK"

    # ----- 4 Double trigger -----
    Write-Step "4" "EDGE: second scheduler (no double credit)"
    $res = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/admin/scheduler/trigger-escrow-auto-release" -Headers $adminH
    Assert-Ok $res "trigger2" -Codes @(200, 201, 202)
    Write-Ok "Trigger #2 HTTP $($res.StatusCode)"
    Start-Sleep -Seconds 12

    $esc2 = Invoke-SqlQuery "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    $wal2 = Invoke-SqlQuery "SELECT COALESCE(balance_cents,0)::text FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    $txn2 = Invoke-SqlQuery @"
SELECT COUNT(*)::text FROM platform_revenue_transactions prt
JOIN delivery_proofs dp ON prt.reference_id = dp.id
WHERE dp.order_id = '$orderId'::uuid AND prt.reference_type = 'escrow_auto_release';
"@
    Write-Host "  after#2 escrow=$esc2 wallet=$wal2 txn=$txn2" -ForegroundColor White

    if ("$esc2" -ne "released") { Write-Fail "Escrow left released" }
    if ([int64]$wal2 -ne [int64]$wal1) {
        Write-Fail "DOUBLE CREDIT wallet $wal1 -> $wal2"
    } else {
        Write-Ok "Wallet unchanged ($wal2)"
    }
    if ([int64]$txn2 -ne [int64]$txn1) {
        Write-Fail "DOUBLE platform txn $txn1 -> $txn2"
    } else {
        Write-Ok "Platform txn unchanged ($txn2)"
    }

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $c = if ($script:Failed -eq 0) { "Green" } else { "Red" }
    Write-Host " EDGE RESULT : $($script:Passed) OK | $($script:Failed) FAIL" -ForegroundColor $c
    Write-Host "================================================================" -ForegroundColor Magenta
}
catch {
    Write-Host ""
    Write-Host "[CRITICAL] $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host ""