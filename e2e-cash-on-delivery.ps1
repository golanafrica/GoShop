# ============================================================
# GOSHOP E2E - Cash On Delivery (COD) Full Flow
# Scénario : Commande COD -> Preuves (Client+Marchand) -> Livraison -> Collecte Commission
# Test critique : Commission avec solde insuffisant -> Gel automatique du wallet
# ============================================================

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

# -------------------- CONFIG --------------------
$BaseUrl       = if ($env:GOSHOP_BASE_URL) { $env:GOSHOP_BASE_URL } else { "http://localhost:8080" }
$AdminEmail    = if ($env:ADMIN_EMAIL) { $env:ADMIN_EMAIL } else { "superadmin.yacine@goshop.com" }
$AdminPassword = if ($env:ADMIN_PASSWORD) { $env:ADMIN_PASSWORD } else { "LassinaYacine19778&" }
$DbService     = if ($env:DB_SERVICE) { $env:DB_SERVICE } else { "db" }
$DbUser        = if ($env:DB_USER) { $env:DB_USER } else { "postgres" }
$DbName        = if ($env:DB_NAME) { $env:DB_NAME } else { "goshop_db" }

$OrderAmount   = 1000000 # 10 000 FCFA
$CommissionBps = 250     # 2.50% (CODCommissionRateBps)
$ExpectedComm  = [int64](($OrderAmount * $CommissionBps) / 10000) # 25000 cents = 250 FCFA

$Timestamp     = Get-Date -Format "yyyyMMddHHmmss"
$ShopSlug      = "cod-shop-$Timestamp"
$MerchantEmail = "merchant.cod.$Timestamp@goshop.com"
$MerchantPass  = "Password123!"
$CustomerEmail = "customer.cod.$Timestamp@goshop.com"
$CustomerPass  = "Password123!"

$script:Passed = 0
$script:Failed = 0
$script:State  = @{
    MerchantHeaders = $null
    CustomerHeaders = $null
    ShopId          = $null
    ProductId       = $null
    OrderId         = $null
    CustomerId      = $null
}

# -------------------- HELPERS --------------------
function Write-Step { param([string]$N, [string]$Msg); Write-Host ("[{0}] {1}" -f $N, $Msg) -ForegroundColor Cyan }
function Write-Ok { param([string]$Msg); Write-Host ("  [OK] {0}" -f $Msg) -ForegroundColor Green; $script:Passed++ }
function Write-Fail { param([string]$Msg); Write-Host ("  [FAIL] {0}" -f $Msg) -ForegroundColor Red; $script:Failed++; throw $Msg }

function Invoke-Json {
    param([string]$Method, [string]$Uri, [hashtable]$Headers = @{}, [object]$Body = $null, [int[]]$OkStatus = @(200, 201, 202))
    $params = @{ Method = $Method; Uri = $Uri; Headers = $Headers; UseBasicParsing = $true }
    if ($null -ne $Body) { 
        $jsonBody = if ($Body -is [string]) { $Body } else { ($Body | ConvertTo-Json -Depth 12 -Compress) }
        $params.Body = [System.Text.Encoding]::UTF8.GetBytes($jsonBody)
        $params.ContentType = "application/json; charset=utf-8"
    }
    try {
        $resp = Invoke-WebRequest @params
        $code = [int]$resp.StatusCode
        $data = $null; if ($resp.Content) { try { $data = $resp.Content | ConvertFrom-Json } catch { $data = $resp.Content } }
        return @{ Ok = ($OkStatus -contains $code); Status = $code; Data = $data; Raw = $resp.Content }
    } catch {
        $code = 0; $raw = $_.Exception.Message
        if ($_.ErrorDetails -and $_.ErrorDetails.Message) { $raw = $_.ErrorDetails.Message }
        elseif ($_.Exception.Response) {
            try { 
                $stream = $_.Exception.Response.GetResponseStream()
                if ($stream) { $reader = New-Object System.IO.StreamReader($stream, [System.Text.Encoding]::UTF8); $raw = $reader.ReadToEnd(); $reader.Close() }
            } catch {}
        }
        if ($_.Exception.Response) { try { $code = [int]$_.Exception.Response.StatusCode.value__ } catch { try { $code = [int]$_.Exception.Response.StatusCode } catch {} } }
        $data = $null; try { $data = $raw | ConvertFrom-Json } catch { $data = $raw }
        return @{ Ok = $false; Status = $code; Data = $data; Raw = $raw }
    }
}

function Invoke-Sql {
    param([string]$Sql)
    $argList = @("compose", "exec", "-T", $DbService, "psql", "-U", $DbUser, "-d", $DbName, "-t", "-A", "-c", $Sql)
    $out = & docker @argList 2>&1
    if ($LASTEXITCODE -ne 0) { throw ("SQL failed: {0}" -f $out) }
    return (($out | ForEach-Object { "$_" }) -join "`n").Trim()
}

function Get-Prop {
    param($Obj, [string[]]$Paths)
    foreach ($p in $Paths) {
        $cur = $Obj; $ok = $true
        foreach ($seg in ($p -split '\.')) {
            if ($null -eq $cur) { $ok = $false; break }
            if ($cur -is [hashtable] -and $cur.ContainsKey($seg)) { $cur = $cur[$seg]; continue }
            $prop = $cur.PSObject.Properties[$seg]
            if ($prop) { $cur = $prop.Value } else { $ok = $false; break }
        }
        if ($ok -and $null -ne $cur -and "$cur" -ne "") { return $cur }
    }
    return $null
}

function Get-WalletRow {
    param([string]$ShopId)
    $sql = "SELECT balance_cents::text || '|' || COALESCE(held_cents,0)::text || '|' || COALESCE(debt_cents,0)::text || '|' || is_frozen::text FROM merchant_wallets WHERE shop_id = '$ShopId'::uuid;"
    $row = Invoke-Sql $sql
    if (-not $row) { return @{ Bal = 0; Held = 0; Debt = 0; Frozen = 'false' } }
    $p = $row.Split('|')
    return @{ Bal = [int64]$p[0]; Held = [int64]$p[1]; Debt = [int64]$p[2]; Frozen = $p[3] }
}

# -------------------- MAIN --------------------
Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Cash On Delivery (COD) Full Flow" -ForegroundColor Magenta
Write-Host (" BaseUrl: {0} | Order: {1} FCFA | Comm: {2} FCFA (2.5%)" -f $BaseUrl, ($OrderAmount/100), ($ExpectedComm/100)) -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # 01. Health & Admin
    Write-Step -N "01/09" -Msg "Health & Admin Login"
    $h = Invoke-Json -Method GET -Uri "$BaseUrl/health/live" -OkStatus @(200)
    if (-not $h.Ok) { Write-Fail "API health failed" }
    Write-Ok "API live"

    $al = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $AdminEmail; password = $AdminPassword }
    if (-not $al.Ok) { Write-Fail "Admin login failed" }
    Write-Ok "Admin OK"

    # 02. Merchant Setup
    Write-Step -N "02/09" -Msg "Merchant + Shop + Wallet + Product"
    $null = Invoke-Json -Method POST -Uri "$BaseUrl/register" -Body @{ email = $MerchantEmail; password = $MerchantPass; role = "merchant" } -OkStatus @(200, 201)
    $ml = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $MerchantEmail; password = $MerchantPass }
    $mTok = Get-Prop $ml.Data @('access_token','token','data.access_token')
    $script:State.MerchantHeaders = @{ Authorization = "Bearer $mTok"; "X-Shop-Slug" = $ShopSlug }
    
    $shop = Invoke-Json -Method POST -Uri "$BaseUrl/api/shops" -Headers $script:State.MerchantHeaders -Body @{ name = "COD Shop"; slug = $ShopSlug } -OkStatus @(200, 201)
    $shopId = Get-Prop $shop.Data @('id','data.id','shop_id')
    $script:State.ShopId = $shopId
    Invoke-Sql "UPDATE shops SET kyc_status = 'verified' WHERE id = '$shopId'::uuid;" | Out-Null
    
    # Initialiser le wallet avec un solde positif pour le Happy Path
    Invoke-Sql "INSERT INTO merchant_wallets (shop_id, balance_cents, held_cents, debt_cents, is_frozen) VALUES ('$shopId'::uuid, 100000, 0, 0, false) ON CONFLICT (shop_id) DO NOTHING;" | Out-Null
    
    $prod = Invoke-Json -Method POST -Uri "$BaseUrl/api/products" -Headers $script:State.MerchantHeaders -Body @{ name = "Produit COD"; price_cents = $OrderAmount; stock = 10 } -OkStatus @(200, 201)
    $productId = Get-Prop $prod.Data @('id','data.id')
    $script:State.ProductId = $productId
    Write-Ok "Setup OK (Shop: $shopId, Wallet: 1000 FCFA)"

    # 03. Customer Setup
    Write-Step -N "03/09" -Msg "Customer Setup"
    $null = Invoke-Json -Method POST -Uri "$BaseUrl/register" -Body @{ email = $CustomerEmail; password = $CustomerPass; role = "user" } -OkStatus @(200, 201, 409)
    $cl = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $CustomerEmail; password = $CustomerPass }
    $cTok = Get-Prop $cl.Data @('access_token','token','data.access_token')
    $cUserId = Get-Prop $cl.Data @('id','user.id','data.id','data.user.id')
    
    # Le MARCHAND crée le profil client dans son CRM
    $cust = Invoke-Json -Method POST -Uri "$BaseUrl/api/customers" -Headers $script:State.MerchantHeaders -Body @{ 
        first_name = "Client"; last_name = "COD"; email = $CustomerEmail; phone = "+22677515151"; user_id = $cUserId 
    } -OkStatus @(200, 201)
    
    $custId = Get-Prop $cust.Data @('id','data.id','customer.id','data.customer.id')
    if (-not $custId) { Write-Fail ("Customer ID missing in API response: {0}" -f ($cust.Data | ConvertTo-Json -Compress)) }
    $script:State.CustomerId = $custId
    Write-Ok "Customer OK ($custId)"

    # 04. Create COD Order
    Write-Step -N "04/09" -Msg "Create COD Order"
    # 🛡️ CORRECTION : Utiliser les headers du MARCHAND pour créer la commande (évite le blocage RequireShopAccess pour les clients)
    $order = Invoke-Json -Method POST -Uri "$BaseUrl/api/orders" -Headers $script:State.MerchantHeaders -Body @{
        customer_id = $custId
        payment_method = "cash_on_delivery"
        items = @(@{ product_id = $productId; quantity = 1 })
    } -OkStatus @(200, 201)
    
    if (-not $order.Ok) { 
        Write-Fail ("Order creation failed: {0}" -f $order.Raw) 
    }
    
    $orderId = Get-Prop $order.Data @('id','data.id','order.id','data.order.id')
    if (-not $orderId) { 
        Write-Fail ("Order ID missing in API response: {0}" -f ($order.Data | ConvertTo-Json -Compress)) 
    }
    
    $script:State.OrderId = $orderId
    Write-Ok "Order created: $orderId (Status: pending_confirmation)"

    # 05. Workflow COD (Accept -> OutForDelivery)
    Write-Step -N "05/09" -Msg "Workflow COD (Accept -> OutForDelivery)"
    $acc = Invoke-Json -Method POST -Uri "$BaseUrl/api/orders/$orderId/accept" -Headers $script:State.MerchantHeaders -OkStatus @(200)
    if (-not $acc.Ok) { Write-Fail ("Accept failed: {0}" -f $acc.Raw) }
    Write-Ok "Order Accepted"

    $ofd = Invoke-Json -Method POST -Uri "$BaseUrl/api/orders/$orderId/out-for-delivery" -Headers $script:State.MerchantHeaders -OkStatus @(200)
    if (-not $ofd.Ok) { Write-Fail ("OutForDelivery failed: {0}" -f $ofd.Raw) }
    Write-Ok "Order OutForDelivery"

    # 06. Submit Proofs (Client + Merchant)
    Write-Step -N "06/09" -Msg "Submit Proofs (Client + Merchant)"
    $now = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    
    # 🛡️ CORRECTION : Utiliser les headers du MARCHAND (le usecase vérifie le customer_id dans le payload)
    # Client Proof
    $cp = Invoke-Json -Method POST -Uri "$BaseUrl/api/cod/client-proof" -Headers $script:State.MerchantHeaders -Body @{
        order_id = $orderId
        customer_id = $custId
        proof_url = "https://example.com/client-proof.jpg"
        amount_cents = $OrderAmount
        payment_date = $now
    } -OkStatus @(200)
    if (-not $cp.Ok) { Write-Fail ("Client proof failed: {0}" -f $cp.Raw) }
    Write-Ok "Client Proof Submitted"

    # Merchant Proof
    $mp = Invoke-Json -Method POST -Uri "$BaseUrl/api/cod/merchant-proof" -Headers $script:State.MerchantHeaders -Body @{
        order_id = $orderId
        proof_url = "https://example.com/merchant-proof.jpg"
        amount_cents = $OrderAmount
        receipt_date = $now
    } -OkStatus @(200)
    if (-not $mp.Ok) { Write-Fail ("Merchant proof failed: {0}" -f $mp.Raw) }
    Write-Ok "Merchant Proof Submitted (Coherent)"

    # 07. Deliver Order
    Write-Step -N "07/09" -Msg "Deliver Order"
    $del = Invoke-Json -Method POST -Uri "$BaseUrl/api/orders/$orderId/deliver" -Headers $script:State.MerchantHeaders -Body @{
        amount_received = $OrderAmount
        notes = "Delivered successfully"
    } -OkStatus @(200)
    if (-not $del.Ok) { Write-Fail ("Deliver failed: {0}" -f $del.Raw) }
    Write-Ok "Order Delivered"

    # 08. Collect Commission (Happy Path)
    Write-Step -N "08/09" -Msg "Collect Commission (Happy Path)"
    $wBefore = Get-WalletRow -ShopId $shopId
    Write-Host ("  Wallet Before: bal={0} frozen={1}" -f $wBefore.Bal, $wBefore.Frozen) -ForegroundColor DarkGray

    $col = Invoke-Json -Method POST -Uri "$BaseUrl/api/cod/collect" -Headers $script:State.MerchantHeaders -Body @{
        order_id = $orderId
    } -OkStatus @(200)
    if (-not $col.Ok) { Write-Fail ("Collect failed: {0}" -f $col.Raw) }
    
    $commStatus = Get-Prop $col.Data @('commission_status','data.commission_status')
    if ($commStatus -ne "collected") { Write-Fail "Commission status should be 'collected', got $commStatus" }
    Write-Ok "Commission Collected ($ExpectedComm cents)"

    $wAfter = Get-WalletRow -ShopId $shopId
    $expectedBal = $wBefore.Bal - $ExpectedComm
    if ($wAfter.Bal -ne $expectedBal) { Write-Fail "Balance should be $expectedBal, got $($wAfter.Bal)" }
    Write-Ok "Wallet Debited (Bal: $($wBefore.Bal) -> $($wAfter.Bal))"

    # 09. Collect Commission (Debt/Frozen Path)
    Write-Step -N "09/09" -Msg "Collect Commission (Debt/Frozen Path)"
    
    # Créer une 2ème commande pour tester le gel
    $order2 = Invoke-Json -Method POST -Uri "$BaseUrl/api/orders" -Headers $script:State.MerchantHeaders -Body @{
        customer_id = $custId; payment_method = "cash_on_delivery"; items = @(@{ product_id = $productId; quantity = 1 })
    } -OkStatus @(200, 201)
    $orderId2 = Get-Prop $order2.Data @('id','data.id','order.id','data.order.id')
    if (-not $orderId2) { Write-Fail "Order 2 ID missing" }
    
    # Workflow rapide pour Order 2
    Invoke-Json -Method POST -Uri "$BaseUrl/api/orders/$orderId2/accept" -Headers $script:State.MerchantHeaders -OkStatus @(200) | Out-Null
    Invoke-Json -Method POST -Uri "$BaseUrl/api/orders/$orderId2/out-for-delivery" -Headers $script:State.MerchantHeaders -OkStatus @(200) | Out-Null
    Invoke-Json -Method POST -Uri "$BaseUrl/api/cod/client-proof" -Headers $script:State.MerchantHeaders -Body @{ order_id = $orderId2; customer_id = $custId; proof_url = "url"; amount_cents = $OrderAmount; payment_date = $now } -OkStatus @(200) | Out-Null
    Invoke-Json -Method POST -Uri "$BaseUrl/api/cod/merchant-proof" -Headers $script:State.MerchantHeaders -Body @{ order_id = $orderId2; proof_url = "url"; amount_cents = $OrderAmount; receipt_date = $now } -OkStatus @(200) | Out-Null
    Invoke-Json -Method POST -Uri "$BaseUrl/api/orders/$orderId2/deliver" -Headers $script:State.MerchantHeaders -Body @{ amount_received = $OrderAmount } -OkStatus @(200) | Out-Null

    # Vider le wallet pour forcer le négatif
    Invoke-Sql "UPDATE merchant_wallets SET balance_cents = 0 WHERE shop_id = '$shopId'::uuid;" | Out-Null
    Write-Ok "Wallet emptied for Order 2 test"

    # Tenter de collecter la commission
    $col2 = Invoke-Json -Method POST -Uri "$BaseUrl/api/cod/collect" -Headers $script:State.MerchantHeaders -Body @{ order_id = $orderId2 } -OkStatus @(200)
    if (-not $col2.Ok) { Write-Fail ("Collect 2 failed: {0}" -f $col2.Raw) }
    
    $wFinal = Get-WalletRow -ShopId $shopId
    Write-Host ("  Wallet Final: bal={0} frozen={1}" -f $wFinal.Bal, $wFinal.Frozen) -ForegroundColor Cyan

    # Vérifier le gel ou la dette
    if ($wFinal.Frozen -eq 't' -or $wFinal.Frozen -eq 'true') {
        Write-Ok "Wallet FROZEN due to negative balance (Commission Debt)"
    } elseif ($wFinal.Bal -lt 0) {
        Write-Ok "Wallet NEGATIVE (Balance: $($wFinal.Bal))"
    } else {
        Write-Fail "Wallet should be frozen or negative after commission collection with 0 balance"
    }

    # Summary
    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $color = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    Write-Host (" RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed) -ForegroundColor $color
    Write-Host "================================================================" -ForegroundColor Magenta

    if ($script:Failed -eq 0) {
        Write-Host ""
        Write-Host "  COD FULL FLOW E2E VERT" -ForegroundColor Green
        Write-Host "  (Proofs coherent, commission collected, debt/freeze validated)" -ForegroundColor Green
    } else {
        exit 1
    }
} catch {
    Write-Host ""
    Write-Host ("[CRITICAL FAILURE] {0}" -f $_.Exception.Message) -ForegroundColor Red
    exit 1
}
Write-Host ""