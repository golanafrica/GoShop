# ============================================================
# GOSHOP E2E - Dispute merchant_wins
# Aligné e2e-debt-sweep-fraud.ps1 (webhook Yenga + routes proof)
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
$InjectDebt    = if ($env:INJECT_DEBT_CENTS) { [int64]$env:INJECT_DEBT_CENTS } else { [int64]0 }

$Timestamp     = Get-Date -Format "yyyyMMddHHmmss"
$MerchantEmail = "merchant.mwins.$Timestamp@goshop.com"
$MerchantPass  = "Password123!"
$ShopSlug      = "mwins-shop-$Timestamp"
$ShopName      = "MWins Shop $Timestamp"
$CustomerPhone = "+22677515151"
$PriceCents    = 100000
$PaymentAmount = 1000

$script:Passed = 0
$script:Failed = 0
$script:State  = @{
    AdminHeaders    = $null
    MerchantHeaders = $null
    ShopId          = $null
    OrderId         = $null
    PaymentId       = $null
    DisputeId       = $null
    DebtBefore      = 0
}

function Write-Step {
    param([string]$N, [string]$Msg)
    Write-Host ""
    Write-Host ("[{0}] {1}" -f $N, $Msg) -ForegroundColor Cyan
}
function Write-Ok {
    param([string]$Msg)
    $script:Passed++
    Write-Host ("  [OK] {0}" -f $Msg) -ForegroundColor Green
}
function Write-Fail {
    param([string]$Msg)
    $script:Failed++
    Write-Host ("  [FAIL] {0}" -f $Msg) -ForegroundColor Red
    throw $Msg
}
function Write-Warn {
    param([string]$Msg)
    Write-Host ("  [WARN] {0}" -f $Msg) -ForegroundColor Yellow
}
function Write-Info {
    param([string]$Msg)
    Write-Host ("  {0}" -f $Msg) -ForegroundColor DarkGray
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
            } else {
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
            try { $code = [int]$_.Exception.Response.StatusCode.value__ } catch {
                try { $code = [int]$_.Exception.Response.StatusCode } catch {}
            }
            try {
                $sr = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
                $raw = $sr.ReadToEnd()
            } catch {}
        }
        $data = $null
        if ($raw) { try { $data = $raw | ConvertFrom-Json } catch { $data = $raw } }
        return @{ Success = $false; StatusCode = $code; Data = $data; Raw = $raw; Error = $_.Exception.Message }
    }
}

function Get-Prop {
    param($Obj, [string[]]$Paths)
    foreach ($p in $Paths) {
        $cur = $Obj
        $ok = $true
        foreach ($seg in ($p -split '\.')) {
            if ($null -eq $cur) { $ok = $false; break }
            if ($cur.PSObject.Properties.Name -contains $seg) {
                $cur = $cur.$seg
            } else {
                $ok = $false
                break
            }
        }
        if ($ok -and $null -ne $cur -and "$cur" -ne "") { return $cur }
    }
    return $null
}

function Invoke-Sql {
    param([string]$Sql)
    $out = docker compose exec -T $DbService psql -U $DbUser -d $DbName -t -A -c $Sql 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw ("SQL failed: {0}" -f ($out | Out-String))
    }
    return (($out | Out-String).Trim())
}

function Get-WalletPair {
    param([string]$ShopId)
    $q = "SELECT COALESCE(balance_cents,0)::text || '|' || COALESCE(debt_cents,0)::text FROM merchant_wallets WHERE shop_id = '$ShopId'::uuid LIMIT 1;"
    $row = Invoke-Sql $q
    if (-not $row -or $row -notmatch '\|') {
        return @{ Bal = [int64]0; Debt = [int64]0 }
    }
    $parts = $row -split '\|'
    return @{ Bal = [int64]$parts[0]; Debt = [int64]$parts[1] }
}

function New-WebhookHash {
    param([string]$JsonBody, [string]$Secret)
    $hmac = [System.Security.Cryptography.HMACSHA256]::new([Text.Encoding]::UTF8.GetBytes($Secret))
    $bytes = $hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($JsonBody))
    return (-join ($bytes | ForEach-Object { $_.ToString("x2") }))
}

Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Dispute merchant_wins" -ForegroundColor Magenta
Write-Host (" BaseUrl: {0} | InjectDebt={1}" -f $BaseUrl, $InjectDebt) -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # 01 Health
    Write-Step -N "01/09" -Msg "Health"
    $h = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health/live"
    if ($h.StatusCode -ne 200 -and $h.StatusCode -ne 404) {
        $h = Invoke-SafeApi -Method Get -Uri "$BaseUrl/health"
    }
    if ($h.StatusCode -ge 500 -or $h.StatusCode -eq 0) {
        throw ("API down HTTP {0}" -f $h.StatusCode)
    }
    Write-Ok ("API live HTTP {0}" -f $h.StatusCode)

    # 02 Admin
    Write-Step -N "02/09" -Msg "Admin login"
    $login = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email    = $AdminEmail
        password = $AdminPassword
    }
    if (-not $login.Success) { throw ("Admin login: {0}" -f $login.Raw) }
    $adminToken = Get-Prop $login.Data @("access_token", "token")
    if (-not $adminToken) { throw "Admin token missing" }
    $script:State.AdminHeaders = @{
        Authorization  = "Bearer $adminToken"
        "Content-Type" = "application/json"
    }
    Write-Ok "Admin OK"

    # 03 Merchant + shop + KYC
    Write-Step -N "03/09" -Msg "Merchant + shop + KYC"
    $reg = Invoke-SafeApi -Method Post -Uri "$BaseUrl/register" -Body @{
        email      = $MerchantEmail
        password   = $MerchantPass
        first_name = "Merchant"
        last_name  = "MWins"
    }
    if ($reg.Success -or $reg.StatusCode -eq 409) {
        Write-Ok ("Register HTTP {0}" -f $reg.StatusCode)
    } else {
        Write-Warn ("Register: {0}" -f $reg.Raw)
    }

    $mLogin = Invoke-SafeApi -Method Post -Uri "$BaseUrl/login" -Body @{
        email    = $MerchantEmail
        password = $MerchantPass
    }
    if (-not $mLogin.Success) { throw ("Merchant login: {0}" -f $mLogin.Raw) }
    $mToken = Get-Prop $mLogin.Data @("access_token", "token")
    if (-not $mToken) { throw "Merchant token missing" }

    $mh = @{
        Authorization  = "Bearer $mToken"
        "Content-Type" = "application/json"
    }
    $shop = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/shops" -Headers $mh -Body @{
        name     = $ShopName
        slug     = $ShopSlug
        currency = "XOF"
    }
    if (-not $shop.Success) { throw ("Shop: {0}" -f $shop.Raw) }
    $shopId = Get-Prop $shop.Data @("id", "data.id", "shop.id")
    if (-not $shopId) { throw "Shop id missing" }
    $script:State.ShopId = $shopId
    $script:State.MerchantHeaders = @{
        Authorization  = "Bearer $mToken"
        "Content-Type" = "application/json"
        "X-Shop-Slug"  = $ShopSlug
    }
    Write-Ok ("Shop {0} ({1})" -f $shopId, $ShopSlug)

    try {
        Invoke-Sql "UPDATE shops SET kyc_status = 'verified', updated_at = NOW() WHERE id = '$shopId'::uuid;" | Out-Null
        Write-Ok "KYC forced verified"
    } catch {
        Write-Warn ("KYC SQL: {0}" -f $_.Exception.Message)
    }

    # 04 Product / order / payment / webhook Yenga
    Write-Step -N "04/09" -Msg "Product, order, payment, webhook"
    $prod = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/products" -Headers $script:State.MerchantHeaders -Body @{
        name        = "Produit MWins $Timestamp"
        description = "E2E"
        price_cents = $PriceCents
        stock       = 100
    }
    if (-not $prod.Success) { throw ("Product: {0}" -f $prod.Raw) }
    $productId = Get-Prop $prod.Data @("id", "data.id")
    Invoke-Sql "UPDATE products SET stock = 100, updated_at = NOW() WHERE id = '$productId'::uuid;" | Out-Null
    Write-Ok ("Product {0}" -f $productId)

    $cust = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" -Headers $script:State.MerchantHeaders -Body @{
        first_name = "Client"
        last_name  = "MWins"
        email      = "client.mwins.$Timestamp@test.com"
        phone      = $CustomerPhone
    }
    if (-not $cust.Success) {
        $cust = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/customers" -Headers $script:State.MerchantHeaders -Body @{
            first_name   = "Client"
            last_name    = "MWins"
            email        = "client.mwins.$Timestamp@test.com"
            phone_number = $CustomerPhone
        }
    }
    if (-not $cust.Success) { throw ("Customer: {0}" -f $cust.Raw) }
    $customerId = Get-Prop $cust.Data @("id", "data.id")
    Write-Ok ("Customer {0}" -f $customerId)

    $ord = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders" -Headers $script:State.MerchantHeaders -Body @{
        customer_id    = $customerId
        payment_method = "mobile_money"
        items          = @(@{ product_id = $productId; quantity = 1 })
        currency       = "XOF"
    }
    if (-not $ord.Success) { throw ("Order: {0}" -f $ord.Raw) }
    $orderId = Get-Prop $ord.Data @("id", "data.id", "order.id")
    $script:State.OrderId = $orderId
    Write-Ok ("Order {0}" -f $orderId)

    try {
        $zoneId = Invoke-Sql "SELECT id::text FROM delivery_zones WHERE zone_code = 'BF-OUAGA-URB' AND is_active = true LIMIT 1;"
        if ($zoneId) {
            Invoke-Sql "UPDATE orders SET delivery_zone_id = '$zoneId'::uuid WHERE id = '$orderId'::uuid;" | Out-Null
            Write-Ok "Order zone BF-OUAGA-URB"
        }
    } catch {}

    $pay = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/pay" -Headers $script:State.MerchantHeaders -Body @{
        provider     = "yenga_pay"
        phone_number = $CustomerPhone
        description  = "E2E merchant_wins order $orderId"
        metadata     = @{ flow = "indirect" }
    }
    if (-not $pay.Success) { throw ("Pay HTTP {0}: {1}" -f $pay.StatusCode, $pay.Raw) }
    $paymentId   = Get-Prop $pay.Data @("payment_id", "id", "data.payment_id", "data.id")
    $providerRef = Get-Prop $pay.Data @("provider_ref", "data.provider_ref")
    if (-not $paymentId) { throw "payment_id missing" }
    $script:State.PaymentId = $paymentId
    Write-Ok ("Payment {0} ref={1}" -f $paymentId, $providerRef)

    # Webhook format exact debt-sweep (CRITIQUE)
    $json = '{"apiEnv":"test","paymentStatus":"DONE","transId":"' + $providerRef + '","projectId":"65687","paymentIntentId":"' + $providerRef + '","paymentSource":"OrangeMoneyAPI","customerNumber":"77515151","paymentAmount":' + $PaymentAmount + ',"paymentFees":25,"contryOrigin":"BF","reference":"' + $paymentId + '","currency":"XOF","isPaylink":false}'
    $hash = New-WebhookHash -JsonBody $json -Secret $WebhookSecret
    $wh = Invoke-SafeApi -Method Post -Uri "$BaseUrl/webhooks/yenga_pay" -Headers @{
        "x-webhook-hash"   = $hash
        "x-yengapay-event" = "payment.success"
    } -Body $json
    if (-not $wh.Success -and $wh.StatusCode -ne 200) {
        throw ("Webhook HTTP {0}: {1}" -f $wh.StatusCode, $wh.Raw)
    }
    Write-Ok "Webhook success"
    Start-Sleep -Seconds 1

    $paySt = Invoke-Sql "SELECT status FROM payments WHERE id = '$paymentId'::uuid LIMIT 1;"
    Write-Info ("Payment status DB: {0}" -f $paySt)
    if ($paySt -notmatch 'success|completed|paid|DONE') {
        Write-Warn ("Payment status unexpected: {0}" -f $paySt)
    }

    # Shipping
    $ship = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/shipping" -Headers $script:State.MerchantHeaders -Body @{
        order_id        = $orderId
        proof_url       = "https://example.com/shipping-mwins-$Timestamp.jpg"
        tracking_number = "TRK-MWINS-$Timestamp"
        carrier         = "E2E"
        notes           = "E2E shipped"
    }
    if (-not $ship.Success) {
        $ship = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/shipping-proof" -Headers $script:State.MerchantHeaders -Body @{
            tracking_number = "TRK-MWINS-$Timestamp"
            carrier         = "E2E"
            shipped_at      = (Get-Date).ToUniversalTime().ToString("o")
        }
    }
    if (-not $ship.Success) {
        throw ("Shipping failed HTTP {0}: {1}" -f $ship.StatusCode, $ship.Raw)
    }
    Write-Ok "Shipping OK"

    # Delivery
    $del = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/delivery/proof/delivery" -Headers $script:State.MerchantHeaders -Body @{
        order_id  = $orderId
        proof_url = "https://example.com/delivery-mwins-$Timestamp.jpg"
        signature = "SIG-MWINS-$Timestamp"
        notes     = "E2E delivered"
        rating    = 5
    }
    if (-not $del.Success) {
        $del = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/delivery-proof" -Headers $script:State.MerchantHeaders -Body @{
            recipient_name = "Client MWins"
            delivered_at   = (Get-Date).ToUniversalTime().ToString("o")
            notes          = "OK"
        }
    }
    if (-not $del.Success) {
        throw ("Delivery failed HTTP {0}: {1}" -f $del.StatusCode, $del.Raw)
    }
    Write-Ok "Delivery OK"

    $escBefore = Invoke-Sql "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    Write-Info ("Escrow before dispute: {0}" -f $escBefore)

    # 05 Open dispute PRE-release
    Write-Step -N "05/09" -Msg "Open dispute (PRE-release)"
    $disp = Invoke-SafeApi -Method Post -Uri "$BaseUrl/api/orders/$orderId/dispute" -Headers $script:State.MerchantHeaders -Body @{
        reason = "E2E merchant_wins pre-release $Timestamp"
    }
    if (-not $disp.Success) {
        throw ("Open dispute HTTP {0}: {1}" -f $disp.StatusCode, $disp.Raw)
    }
    $disputeId = Get-Prop $disp.Data @("dispute.id", "data.dispute.id", "id", "data.id")
    if (-not $disputeId) { throw "dispute id missing" }
    $script:State.DisputeId = $disputeId
    Write-Ok ("Dispute {0}" -f $disputeId)

    $escDisputed = Invoke-Sql "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    Write-Info ("Escrow after open: {0}" -f $escDisputed)
    if ($escDisputed -match "disputed") { Write-Ok "Escrow = disputed" }
    else { Write-Warn ("Escrow={0} (attendu disputed)" -f $escDisputed) }

    $w0 = Get-WalletPair -ShopId $shopId
    Write-Info ("Wallet before resolve: bal={0} debt={1}" -f $w0.Bal, $w0.Debt)

    # 06 Debt inject
         # 06 Optional debt inject (PK = shop_id, pas de colonne id)
    Write-Step -N "06/09" -Msg ("Optional debt inject ({0})" -f $InjectDebt)
    if ($InjectDebt -gt 0) {
        $sqlUpsert = @"
INSERT INTO merchant_wallets (
  shop_id, balance_cents, held_cents, debt_cents, is_frozen,
  max_negative_balance_cents, total_sales_cents, total_commissions_cents, total_payouts_cents,
  created_at, updated_at
) VALUES (
  '$shopId'::uuid, 0, 0, $InjectDebt, false,
  -100000, 0, 0, 0,
  NOW(), NOW()
)
ON CONFLICT (shop_id) DO UPDATE
  SET debt_cents = $InjectDebt,
      updated_at = NOW()
RETURNING debt_cents;
"@
        $ret = Invoke-Sql $sqlUpsert
        Write-Info ("UPSERT RETURNING debt_cents={0}" -f $ret)

        $wInj = Get-WalletPair -ShopId $shopId
        $script:State.DebtBefore = $wInj.Debt

        if ($wInj.Debt -lt $InjectDebt) {
            $row = Invoke-Sql "SELECT balance_cents::text || '|' || debt_cents::text || '|' || held_cents::text FROM merchant_wallets WHERE shop_id = '$shopId'::uuid LIMIT 1;"
            throw ("Debt inject failed: expected debt>=$InjectDebt got $($wInj.Debt) | row=$row")
        }
        Write-Ok ("debt_cents = {0}" -f $wInj.Debt)
    } else {
        $script:State.DebtBefore = $w0.Debt
        Write-Ok "No debt inject"
    }

    # 07 Resolve merchant_wins
    Write-Step -N "07/09" -Msg "Admin resolve merchant_wins"
    $resolveBody = @{
        resolution = "merchant_wins"
        notes      = "E2E merchant_wins $Timestamp"
    }
    $resolved = $false
    foreach ($url in @(
        "$BaseUrl/api/admin/disputes/$disputeId/resolve",
        "$BaseUrl/api/admin/orders/$orderId/dispute/resolve"
    )) {
        $res = Invoke-SafeApi -Method Post -Uri $url -Headers $script:State.AdminHeaders -Body $resolveBody
        if ($res.Success) {
            Write-Ok ("Resolve OK via {0}" -f $url)
            $resolved = $true
            break
        }
        Write-Warn ("Try {0} HTTP {1}" -f $url, $res.StatusCode)
    }
    if (-not $resolved) { throw "Resolve merchant_wins failed" }
    Start-Sleep -Seconds 2

    # 08 Asserts
    Write-Step -N "08/09" -Msg "Asserts"
    $escAfter  = Invoke-Sql "SELECT status FROM escrow_accounts WHERE order_id = '$orderId'::uuid LIMIT 1;"
    $dispAfter = Invoke-Sql "SELECT status FROM disputes WHERE id = '$disputeId'::uuid LIMIT 1;"
    $w1 = Get-WalletPair -ShopId $shopId

    Write-Info ("Escrow  : {0}" -f $escAfter)
    Write-Info ("Dispute : {0}" -f $dispAfter)
    Write-Info ("Wallet  : bal {0}->{1} | debt {2}->{3}" -f $w0.Bal, $w1.Bal, $script:State.DebtBefore, $w1.Debt)

    if ($escAfter -match "released") { Write-Ok "Escrow = released" }
    else { Write-Fail ("Escrow attendu released, got {0}" -f $escAfter) }

    if ($dispAfter -match "resolved_merchant|merchant_wins|resolved") {
        Write-Ok ("Dispute OK ({0})" -f $dispAfter)
    } else {
        Write-Fail ("Dispute status: {0}" -f $dispAfter)
    }

    $claw = Invoke-Sql @"
SELECT COALESCE(string_agg(x.t, ','), '') FROM (
  SELECT transaction_type || '|' || amount_cents::text AS t
  FROM wallet_transactions
  WHERE shop_id = '$shopId'::uuid
    AND (transaction_type ILIKE '%clawback%' OR COALESCE(description,'') ILIKE '%clawback%')
  ORDER BY created_at DESC LIMIT 5
) x;
"@
    if (-not $claw) { Write-Ok "Aucune ligne clawback (attendu)" }
    else { Write-Fail ("Clawback inattendu: {0}" -f $claw) }

    if ($InjectDebt -gt 0) {
        $swept = $script:State.DebtBefore - $w1.Debt
        if ($swept -gt 0) { Write-Ok ("Debt reduced by {0}" -f $swept) }
        else { Write-Fail "Debt not reduced" }
        $sweepLed = Invoke-Sql @"
SELECT COALESCE(string_agg(x.t, ','), '') FROM (
  SELECT transaction_type || '|' || amount_cents::text AS t
  FROM wallet_transactions
  WHERE shop_id = '$shopId'::uuid
    AND (transaction_type ILIKE '%debt_sweep%' OR COALESCE(description,'') ILIKE '%debt_sweep%')
  ORDER BY created_at DESC LIMIT 5
) x;
"@
        if ($sweepLed) { Write-Ok ("Ledger debt_sweep: {0}" -f $sweepLed) }
        else { Write-Warn "Ledger debt_sweep introuvable" }
    } else {
        if ($w1.Bal -gt $w0.Bal) { Write-Ok ("Wallet credited delta={0}" -f ($w1.Bal - $w0.Bal)) }
        else { Write-Warn ("Balance {0}->{1}" -f $w0.Bal, $w1.Bal) }
    }

    # 09 Summary
    Write-Step -N "09/09" -Msg "Summary"
    Write-Host ("  Shop    : {0}" -f $shopId) -ForegroundColor White
    Write-Host ("  Order   : {0}" -f $orderId) -ForegroundColor White
    Write-Host ("  Payment : {0}" -f $paymentId) -ForegroundColor White
    Write-Host ("  Dispute : {0}" -f $disputeId) -ForegroundColor White
    Write-Host ("  Escrow  : {0}" -f $escAfter) -ForegroundColor White
    Write-Host ("  Wallet  : bal {0}->{1} | debt {2}->{3}" -f $w0.Bal, $w1.Bal, $script:State.DebtBefore, $w1.Debt) -ForegroundColor White

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $color = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    Write-Host (" RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed) -ForegroundColor $color
    Write-Host "================================================================" -ForegroundColor Magenta
    if ($script:Failed -eq 0) {
        Write-Host ""
        Write-Host "  MERCHANT_WINS E2E VERT" -ForegroundColor Green
    } else { exit 1 }
}
catch {
    Write-Host ""
    Write-Host ("[CRITICAL FAILURE] {0}" -f $_.Exception.Message) -ForegroundColor Red
    exit 1
}

Write-Host ""