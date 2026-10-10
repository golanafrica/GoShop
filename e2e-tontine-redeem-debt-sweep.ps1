# ============================================================
# GOSHOP E2E - Tontine Phase 1.2 : Redeem Voucher + Debt Sweep
# Scénario : Tontine payée (held) -> Injection dette -> Redeem -> Sweep auto
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

# 🛡️ Numéros de téléphone et mots de passe dynamiques (pas de hardcode)
$CustomerPhone   = if ($env:E2E_CUSTOMER_PHONE) { $env:E2E_CUSTOMER_PHONE } else { "+22676619457" }
$CustomerMsisdn8 = $CustomerPhone -replace '^\+226', '' # Extrait les 8 chiffres pour le webhook
$MerchantPassword = if ($env:MERCHANT_PASSWORD) { $env:MERCHANT_PASSWORD } else { "TestPass!" + (Get-Random -Minimum 1000 -Maximum 9999) }
$CustomerPassword = if ($env:CUSTOMER_PASSWORD) { $env:CUSTOMER_PASSWORD } else { "TestPass!" + (Get-Random -Minimum 1000 -Maximum 9999) }

$NMembers      = 3
$CircleType    = "FAMILY"
$ProductPrice  = 3000000
$Timestamp     = Get-Date -Format "yyyyMMddHHmmss"
$ShopSlug      = "tontine-redeem-$Timestamp"
$MerchantEmail = "merchant.tontine.redeem.$Timestamp@goshop.com"
$InjectDebt    = 1000000

$script:Passed = 0
$script:Failed = 0
$script:State  = @{
    AdminHeaders    = $null
    MerchantHeaders = $null
    MerchantUserId  = $null
    ShopId          = $null
    ProductId       = $null
    GroupId         = $null
    InviteCode      = $null
    Customers       = @()
    ProviderRefs    = @()
    BalBefore       = 0
    HeldBefore      = 0
    DebtBefore      = 0
    ExpectedNet     = 0
}

# -------------------- HELPERS --------------------
function Write-Step { param([string]$N, [string]$Msg); Write-Host ("[{0}] {1}" -f $N, $Msg) -ForegroundColor Cyan }
function Write-Ok { param([string]$Msg); Write-Host ("  [OK] {0}" -f $Msg) -ForegroundColor Green; $script:Passed++ }
function Write-Fail { param([string]$Msg); Write-Host ("  [FAIL] {0}" -f $Msg) -ForegroundColor Red; $script:Failed++; throw $Msg }
function Write-Warn { param([string]$Msg); Write-Host ("  [WARN] {0}" -f $Msg) -ForegroundColor Yellow }

function Invoke-Json {
    param([string]$Method, [string]$Uri, [hashtable]$Headers = @{}, [object]$Body = $null, [int[]]$OkStatus = @(200, 201, 202))
    $params = @{ Method = $Method; Uri = $Uri; Headers = $Headers; ContentType = "application/json"; UseBasicParsing = $true }
    if ($null -ne $Body) { $params.Body = if ($Body -is [string]) { $Body } else { ($Body | ConvertTo-Json -Depth 12 -Compress) } }
    try {
        $resp = Invoke-WebRequest @params
        $code = [int]$resp.StatusCode
        $data = $null; if ($resp.Content) { try { $data = $resp.Content | ConvertFrom-Json } catch { $data = $resp.Content } }
        return @{ Ok = ($OkStatus -contains $code); Status = $code; Data = $data; Raw = $resp.Content }
    } catch {
        $ex = $_.Exception; $code = 0; $raw = $ex.Message
        if ($ex.Response) {
            try { $code = [int]$ex.Response.StatusCode.value__ } catch { try { $code = [int]$ex.Response.StatusCode } catch {} }
            try { $stream = $ex.Response.GetResponseStream(); if ($stream) { $sr = New-Object System.IO.StreamReader($stream); $raw = $sr.ReadToEnd(); $sr.Close() } } catch {}
        }
        $data = $null; try { $data = $raw | ConvertFrom-Json } catch {}
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
    $sql = "SELECT balance_cents::text || '|' || COALESCE(held_cents,0)::text || '|' || COALESCE(debt_cents,0)::text FROM merchant_wallets WHERE shop_id = '{0}'::uuid;" -f $ShopId
    $row = Invoke-Sql $sql
    if (-not $row) { return @{ Bal = 0; Held = 0; Debt = 0 } }
    $p = $row.Split('|')
    return @{ Bal = [int64]$p[0]; Held = [int64]$p[1]; Debt = [int64]$p[2] }
}

function Get-HmacHex {
    param([string]$Secret, [byte[]]$BodyBytes)
    $hmac = New-Object System.Security.Cryptography.HMACSHA256
    $hmac.Key = [System.Text.Encoding]::UTF8.GetBytes($Secret)
    $hash = $hmac.ComputeHash($BodyBytes)
    return ([System.BitConverter]::ToString($hash) -replace '-', '').ToLowerInvariant()
}

function New-CustomerVerified {
    param([int]$Index, [string]$ShopSlug, [hashtable]$MerchantHeaders, [string]$ShopId, [string]$MerchantUserId)
    $email = "cust.tontine.redeem.$Timestamp.$Index@goshop.com"
    
    $null = Invoke-Json -Method POST -Uri "$BaseUrl/register" -Body @{ email = $email; password = $CustomerPassword; role = "user" } -OkStatus @(200, 201, 409)
    $login = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $email; password = $CustomerPassword }
    if (-not $login.Ok) { throw ("Customer login failed {0}: {1}" -f $Index, $login.Raw) }
    $token = Get-Prop $login.Data @('access_token','token','data.access_token')
    $userId = Get-Prop $login.Data @('user.id','data.user.id','id','user_id','data.id')
    if (-not $userId) {
        $me = Invoke-Json -Method GET -Uri "$BaseUrl/auth/me" -Headers @{ Authorization = "Bearer $token" }
        $userId = Get-Prop $me.Data @('id','user.id','data.id','data.user.id')
    }
    if (-not $userId) { $userId = Invoke-Sql ("SELECT id::text FROM users WHERE email = '{0}' LIMIT 1;" -f $email) }
    
    $hdr = @{ Authorization = "Bearer $token"; "X-Shop-Slug" = $ShopSlug }
    # 🛡️ Utilisation d'un numéro unique pour la fiche client, mais le webhook injectera le numéro whitelisté
    $phone = "+22670{0}" -f (100000 + $Index).ToString("D6")
    $cBody = @{ first_name = ("Client{0}" -f $Index); last_name = "Tontine"; phone = $phone; email = $email; user_id = "$userId" }
    
    $cr = Invoke-Json -Method POST -Uri "$BaseUrl/api/customers" -Headers $MerchantHeaders -Body $cBody -OkStatus @(200, 201)
    $custId = Get-Prop $cr.Data @('id','data.id','customer.id','customer_id','data.customer.id')
    if (-not $custId) {
        $sqlIns = "INSERT INTO customers (id, shop_id, first_name, last_name, phone, email, user_id, kyc_level, created_at, updated_at) VALUES (gen_random_uuid(), '{0}'::uuid, 'Client{1}', 'Tontine', '{2}', '{3}', '{4}', 'verified', NOW(), NOW()) RETURNING id::text;" -f $ShopId, $Index, $phone, $email, $userId
        $custId = Invoke-Sql $sqlIns
    }
    if (-not $custId) { throw ("Customer id missing {0}" -f $Index) }
    
    try { Invoke-Sql ("UPDATE customers SET kyc_level = 'verified', kyc_validated_at = NOW(), user_id = '{0}', updated_at = NOW() WHERE id = '{1}'::uuid;" -f $userId, $custId) | Out-Null } catch {}
    try { Invoke-Sql ("INSERT INTO shop_collaborators (id, shop_id, user_id, role, permissions, invited_by, invited_at, accepted_at, is_active, created_at, updated_at) VALUES (gen_random_uuid(), '{0}'::uuid, '{1}', 'seller', '{{}}'::jsonb, '{2}', NOW(), NOW(), true, NOW(), NOW()) ON CONFLICT DO NOTHING;" -f $ShopId, $userId, $(if ($MerchantUserId) { $MerchantUserId } else { $userId })) | Out-Null } catch {}
    
    return @{ Id = $custId; Token = $token; Headers = $hdr; Email = $email; UserId = $userId }
}

# -------------------- MAIN --------------------
Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Tontine Redeem + Debt Sweep (Phase 1.2)" -ForegroundColor Magenta
Write-Host (" BaseUrl: {0} | N={1} | InjectDebt={2}" -f $BaseUrl, $NMembers, $InjectDebt) -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # 01 Health & Admin
    Write-Step -N "01/08" -Msg "Health & Admin"
    $h = Invoke-Json -Method GET -Uri "$BaseUrl/health/live" -OkStatus @(200)
    if (-not $h.Ok) { Write-Fail "API health failed" }
    Write-Ok ("API live HTTP {0}" -f $h.Status)
    
    $al = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $AdminEmail; password = $AdminPassword }
    $adminTok = Get-Prop $al.Data @('access_token','token','data.access_token')
    if (-not $adminTok) { Write-Fail "Admin login failed" }
    $script:State.AdminHeaders = @{ Authorization = "Bearer $adminTok" }
    Write-Ok "Admin OK"

    # 02 Merchant + Shop + Product + Tontine Settings
    Write-Step -N "02/08" -Msg "Merchant + shop + product + tontine settings"
    $reg = Invoke-Json -Method POST -Uri "$BaseUrl/register" -Body @{ email = $MerchantEmail; password = $MerchantPassword; role = "merchant" } -OkStatus @(200, 201)
    $ml = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $MerchantEmail; password = $MerchantPassword }
    $mTok = Get-Prop $ml.Data @('access_token','token','data.access_token')
    $mUserId = Get-Prop $ml.Data @('user.id','data.user.id','id','user_id','data.id')
    if (-not $mUserId) { $mUserId = Invoke-Sql ("SELECT id::text FROM users WHERE email = '{0}' LIMIT 1;" -f $MerchantEmail) }
    $script:State.MerchantUserId = $mUserId
    
    $shop = Invoke-Json -Method POST -Uri "$BaseUrl/api/shops" -Headers @{ Authorization = "Bearer $mTok" } -Body @{ name = "Tontine Redeem Shop"; slug = $ShopSlug } -OkStatus @(200, 201)
    $shopId = Get-Prop $shop.Data @('id','data.id','shop_id')
    if (-not $shopId) { Write-Fail "Shop id missing" }
    $script:State.ShopId = $shopId
    $script:State.MerchantHeaders = @{ Authorization = "Bearer $mTok"; "X-Shop-Slug" = $ShopSlug }
    Write-Ok ("Shop {0} ({1})" -f $shopId, $ShopSlug)
    
    Invoke-Sql ("UPDATE shops SET kyc_status = 'verified', updated_at = NOW() WHERE id = '{0}'::uuid;" -f $shopId) | Out-Null
    
    $prod = Invoke-Json -Method POST -Uri "$BaseUrl/api/products" -Headers $script:State.MerchantHeaders -Body @{ name = "Produit Tontine Redeem"; price_cents = $ProductPrice; stock = 50 } -OkStatus @(200, 201)
    $productId = Get-Prop $prod.Data @('id','data.id')
    if (-not $productId) { Write-Fail "Product id missing" }
    $script:State.ProductId = $productId
    Write-Ok ("Product {0}" -f $productId)

    $ts = Invoke-Json -Method PUT -Uri ("$BaseUrl/api/shops/{0}/tontine-settings" -f $shopId) -Headers $script:State.MerchantHeaders -Body @{
        product_id              = $productId
        is_tontine_enabled      = $true
        allow_commercial_circle = $true
        allow_corporate_circle  = $true
        allow_family_circle     = $true
        min_participants        = $NMembers
        max_participants        = $NMembers
    } -OkStatus @(200, 201)
    if (-not $ts.Ok) { Write-Warn ("Tontine settings HTTP {0} - continue" -f $ts.Status) }
    else { Write-Ok ("Tontine settings N={0}" -f $NMembers) }
    
    Invoke-Sql ("INSERT INTO merchant_wallets (shop_id, balance_cents, held_cents, debt_cents) VALUES ('{0}'::uuid, 0, 0, 0) ON CONFLICT (shop_id) DO NOTHING;" -f $shopId) | Out-Null

    # 03 Customers + Group
    Write-Step -N "03/08" -Msg "3 customers KYC verified + Create group"
    for ($i = 0; $i -lt $NMembers; $i++) {
        $c = New-CustomerVerified -Index $i -ShopSlug $ShopSlug -MerchantHeaders $script:State.MerchantHeaders -ShopId $shopId -MerchantUserId $script:State.MerchantUserId
        $script:State.Customers += $c
    }
    Write-Ok "3 Customers ready"

    $c0 = $script:State.Customers[0]
    $g = Invoke-Json -Method POST -Uri "$BaseUrl/api/tontine/groups" -Headers $c0.Headers -Body @{
        name = "Groupe E2E Redeem $Timestamp"
        product_id = $productId
        circle_type = $CircleType
        total_cycles = $NMembers
    } -OkStatus @(200, 201)
    
    if (-not $g.Ok) { Write-Fail ("Create group failed HTTP {0}: {1}" -f $g.Status, $g.Raw) }
    
    $groupId = Get-Prop $g.Data @('id','data.id','group.id','data.group.id')
    $invite  = Get-Prop $g.Data @('invite_code','data.invite_code','data.group.invite_code')
    if (-not $groupId) { Write-Fail ("group id missing: {0}" -f $g.Raw) }
    
    $script:State.GroupId = $groupId
    $script:State.InviteCode = $invite
    Write-Ok ("Group {0} invite={1}" -f $groupId, $invite)

    for ($i = 1; $i -lt $NMembers; $i++) {
        $ci = $script:State.Customers[$i]
        $j = Invoke-Json -Method POST -Uri "$BaseUrl/api/tontine/groups/join" -Headers $ci.Headers -Body @{ invite_code = $invite } -OkStatus @(200, 201)
        if (-not $j.Ok) { Write-Fail ("Join {0} failed HTTP {1}: {2}" -f $i, $j.Status, $j.Raw) }
    }
    Write-Ok "Members joined"

    $gRow = Invoke-Sql ("SELECT amount_per_cycle_cents::text || '|' || total_cycles::text FROM tontine_groups WHERE id = '{0}'::uuid;" -f $groupId)
    if (-not $gRow) { Write-Fail "tontine_groups row missing" }
    $gp = $gRow.Split('|')
    $amtCycle = [int64]$gp[0]
    $totCyc   = [int]$gp[1]
    $gross = $amtCycle * [int64]$totCyc
    $comm = [int64]([math]::Floor(($gross * 150) / 10000)) # 1.5% FAMILY
    $net  = $gross - $comm
    $script:State.ExpectedNet = $net
    Write-Ok ("Expected gross={0} comm={1} net={2}" -f $gross, $comm, $net)

    # 04 Pay + webhooks (Simule les fonds en HELD)
    Write-Step -N "04/08" -Msg "Pay cycle 1 + webhooks SUCCESS"
    for ($i = 0; $i -lt $NMembers; $i++) {
        $ci = $script:State.Customers[$i]
        # 🛡️ Utilisation du numéro de téléphone dynamique (whitelisté) pour le paiement
        $pay = Invoke-Json -Method POST -Uri ("$BaseUrl/api/tontine/groups/{0}/pay" -f $groupId) -Headers $ci.Headers -Body @{ operator = "orange_money"; phone_number = $CustomerPhone; flow = "indirect" } -OkStatus @(200, 201)
        if (-not $pay.Ok) { Write-Fail ("Pay member {0} HTTP {1}: {2}" -f $i, $pay.Status, $pay.Raw) }
        $pref = Get-Prop $pay.Data @('provider_ref','data.provider_ref','reference','payment.provider_ref','data.payment.provider_ref')
        if (-not $pref) { Write-Fail ("provider_ref missing member {0} : {1}" -f $i, $pay.Raw) }
        $script:State.ProviderRefs += $pref
        Write-Ok ("Pay {0} ref={1}" -f $i, $pref)
    }
    
    for ($i = 0; $i -lt $NMembers; $i++) {
        $pref = $script:State.ProviderRefs[$i]
        $txnId = "TXN-TONTINE-REDEEM-$Timestamp-$i"
        $payAmountMain = [int]([math]::Max(1, [math]::Floor($amtCycle / 100)))
        
        # 🛡️ Injection du numéro whitelisté (8 chiffres) dans les métadonnées du webhook
        $payloadObj = [ordered]@{
            apiEnv = "test"; paymentStatus = "SUCCESS"; transId = $txnId; projectId = "e2e-project"
            paymentIntentId = $txnId; paymentSource = "orange_money"; customerNumber = $CustomerMsisdn8
            paymentAmount = $payAmountMain; paymentFees = 0; contryOrigin = "BF"; reference = $pref; currency = "XOF"
        }
        $bodyStr = ($payloadObj | ConvertTo-Json -Compress -Depth 6)
        $bodyBytes = [System.Text.Encoding]::UTF8.GetBytes($bodyStr)
        $sig = Get-HmacHex -Secret $WebhookSecret -BodyBytes $bodyBytes
        
        $wh = Invoke-Json -Method POST -Uri "$BaseUrl/webhooks/yenga_pay" -Headers @{ "x-webhook-hash" = $sig; "Content-Type" = "application/json" } -Body $bodyStr -OkStatus @(200, 201, 202)
        if (-not $wh.Ok) { Write-Fail ("Webhook {0} HTTP {1}: {2}" -f $i, $wh.Status, $wh.Raw) }
        Write-Ok ("Webhook {0} SUCCESS" -f $i)
    }
    Start-Sleep -Seconds 2
    Write-Ok "Webhooks processed"

    # 05 Wallet BEFORE Redeem
    Write-Step -N "05/08" -Msg "Wallet BEFORE redeem"
    $wBefore = Get-WalletRow -ShopId $shopId
    $script:State.BalBefore  = $wBefore.Bal
    $script:State.HeldBefore = $wBefore.Held
    $script:State.DebtBefore = $wBefore.Debt
    Write-Ok ("bal={0} held={1} debt={2}" -f $wBefore.Bal, $wBefore.Held, $wBefore.Debt)

    # 06 Inject Debt
    Write-Step -N "06/08" -Msg "Inject Residual Debt ({0} cents)" -f $InjectDebt
    Invoke-Sql ("UPDATE merchant_wallets SET debt_cents = {0} WHERE shop_id = '{1}'::uuid;" -f $InjectDebt, $shopId) | Out-Null
    $wDebt = Get-WalletRow -ShopId $shopId
    Write-Ok ("debt_cents = {0}" -f $wDebt.Debt)
    
    # 07 Create Voucher & Redeem
    Write-Step -N "07/08" -Msg "Create Voucher & Redeem"
    $c0Id = $script:State.Customers[0].Id
    
    $participantId = Invoke-Sql ("SELECT id::text FROM tontine_participants WHERE group_id = '{0}'::uuid AND customer_id = '{1}'::uuid LIMIT 1;" -f $groupId, $c0Id)
    if (-not $participantId) { Write-Fail "Participant ID not found for customer $c0Id" }
    
    $existingVoucher = Invoke-Sql ("SELECT voucher_code FROM tontine_vouchers WHERE group_id = '{0}'::uuid AND participant_id = '{1}'::uuid AND cycle_number = 1 LIMIT 1;" -f $groupId, $participantId)
    
    if ($existingVoucher) {
        $voucherCode = $existingVoucher.Trim()
        Write-Ok "Voucher already exists (created by webhook): $voucherCode"
    } else {
        $voucherCode = "E2E-REDEEM-001"
        $sqlVoucher = "INSERT INTO tontine_vouchers (id, group_id, participant_id, customer_id, product_id, shop_id, cycle_number, voucher_code, held_amount_cents, status, expires_at, created_at) VALUES (gen_random_uuid(), '{0}'::uuid, '{1}'::uuid, '{2}'::uuid, '{3}'::uuid, '{4}'::uuid, 1, '{5}', {6}, 'generated', NOW() + INTERVAL '6 months', NOW()) ON CONFLICT (group_id, participant_id, cycle_number) DO UPDATE SET voucher_code = EXCLUDED.voucher_code RETURNING voucher_code;" -f $groupId, $participantId, $c0Id, $productId, $shopId, $voucherCode, $net
        $voucherCode = (Invoke-Sql $sqlVoucher).Trim()
        Write-Ok "Voucher created/fetched: $voucherCode"
    }
    
    $redeemRes = Invoke-Json -Method POST -Uri "$BaseUrl/api/tontine/vouchers/redeem" -Headers $c0.Headers -Body @{
        voucher_code = $voucherCode
        redeemed_by  = $script:State.Customers[0].UserId
    } -OkStatus @(200, 201)
    
    if (-not $redeemRes.Ok) { Write-Fail ("Redeem failed: {0}" -f $redeemRes.Raw) }
    Write-Ok "Voucher redeemed successfully"

    # 08 Assert Debt Sweep
    Write-Step -N "08/08" -Msg "Assert Debt Sweep on Redeem"
    Start-Sleep -Seconds 1
    $wAfter = Get-WalletRow -ShopId $shopId
    
    Write-Host ("  Wallet after redeem: bal={0} | held={1} | debt={2}" -f $wAfter.Bal, $wAfter.Held, $wAfter.Debt) -ForegroundColor Cyan

    $expectedBal = $net - $InjectDebt
    if ($wAfter.Held -eq 0 -and $wAfter.Debt -eq 0 -and $wAfter.Bal -eq $expectedBal) {
        Write-Ok "🎉 SWEEP SUCCESSFUL! Held released (0), debt cleared (0), net balance = $expectedBal cents."
    } else {
        Write-Fail "Sweep failed. Expected Bal=$expectedBal, Held=0, Debt=0. Got Bal=$($wAfter.Bal), Held=$($wAfter.Held), Debt=$($wAfter.Debt)"
    }

    $sweepTxn = Invoke-Sql ("SELECT COUNT(*) FROM wallet_transactions WHERE shop_id = '{0}'::uuid AND transaction_type = 'debt_sweep';" -f $shopId)
    if ([int64]$sweepTxn -ge 1) {
        Write-Ok "Ledger audit trail confirmed: 'debt_sweep' transaction recorded."
    } else {
        Write-Fail "Missing 'debt_sweep' audit transaction in ledger."
    }

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $color = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    Write-Host (" RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed) -ForegroundColor $color
    Write-Host "================================================================" -ForegroundColor Magenta

    if ($script:Failed -eq 0) {
        Write-Host ""
        Write-Host "  TONTINE REDEEM + DEBT SWEEP E2E VERT" -ForegroundColor Green
    } else {
        exit 1
    }
} catch {
    Write-Host ""
    Write-Host ("[CRITICAL FAILURE] {0}" -f $_.Exception.Message) -ForegroundColor Red
    exit 1
}
Write-Host ""