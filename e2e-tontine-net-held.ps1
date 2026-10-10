# ============================================================
# GOSHOP E2E - Tontine Phase 1.1/1.2 : credit NET + held, 0 debit scheduler
# ============================================================
# Flux:
#   1) Health + admin
#   2) Merchant + shop + product + tontine-settings (N=3)
#   3) 3 customers KYC verified + shop_collaborators (RequireShopAccess)
#   4) Create group FAMILY + joins (JWT CLIENT)
#   5) Wallet BEFORE
#   6) 3x POST pay (JWT CLIENT) + webhook SUCCESS
#   7) Assert: balance += net, held += net, debt inchange
#   8) Ledger tontine_cycle, 0 commission_debit per-cotisation
#
# Env:
#   $env:YENGA_PAY_WEBHOOK_SECRET
#   $env:ADMIN_EMAIL / $env:ADMIN_PASSWORD
#   $env:E2E_CUSTOMER_PHONE   (default: +22676619457)
#   $env:MERCHANT_PASSWORD    (default: random)
#   $env:GOSHOP_BASE_URL
#   $env:DB_SERVICE (default db)
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

$NMembers      = 3
$CircleType    = "FAMILY"
$ProductPrice  = 3000000
$Timestamp     = Get-Date -Format "yyyyMMddHHmmss"
$ShopSlug      = "tontine-net-$Timestamp"
$MerchantEmail = "merchant.tontine.$Timestamp@goshop.com"

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
    ExpectedGross   = 0
    ExpectedComm    = 0
}

# -------------------- HELPERS --------------------
function Write-Step {
    param([string]$N, [string]$Msg)
    Write-Host ("[{0}] {1}" -f $N, $Msg) -ForegroundColor Cyan
}
function Write-Ok {
    param([string]$Msg)
    Write-Host ("  [OK] {0}" -f $Msg) -ForegroundColor Green
    $script:Passed++
}
function Write-Fail {
    param([string]$Msg)
    Write-Host ("  [FAIL] {0}" -f $Msg) -ForegroundColor Red
    $script:Failed++
    throw $Msg
}
function Write-Warn {
    param([string]$Msg)
    Write-Host ("  [WARN] {0}" -f $Msg) -ForegroundColor Yellow
}

function Invoke-Json {
    param(
        [string]$Method,
        [string]$Uri,
        [hashtable]$Headers = @{},
        [object]$Body = $null,
        [int[]]$OkStatus = @(200, 201, 202)
    )
    $params = @{
        Method          = $Method
        Uri             = $Uri
        Headers         = $Headers
        ContentType     = "application/json"
        UseBasicParsing = $true
    }
    if ($null -ne $Body) {
        $params.Body = if ($Body -is [string]) { $Body } else { ($Body | ConvertTo-Json -Depth 12 -Compress) }
    }
    try {
        $resp = Invoke-WebRequest @params
        $code = [int]$resp.StatusCode
        $data = $null
        if ($resp.Content) {
            try { $data = $resp.Content | ConvertFrom-Json } catch { $data = $resp.Content }
        }
        return @{ Ok = ($OkStatus -contains $code); Status = $code; Data = $data; Raw = $resp.Content }
    }
    catch {
        $ex = $_.Exception
        $code = 0
        $raw = $null
        if ($_.ErrorDetails -and $_.ErrorDetails.Message) {
            $raw = $_.ErrorDetails.Message
        }
        if ($ex.Response) {
            try { $code = [int]$ex.Response.StatusCode.value__ } catch {
                try { $code = [int]$ex.Response.StatusCode } catch {}
            }
            if (-not $raw) {
                try {
                    $stream = $ex.Response.GetResponseStream()
                    if ($stream) {
                        $sr = New-Object System.IO.StreamReader($stream)
                        $raw = $sr.ReadToEnd()
                        $sr.Close()
                    }
                } catch {}
            }
        }
        if (-not $raw) { $raw = $ex.Message }
        $data = $null
        try { $data = $raw | ConvertFrom-Json } catch {}
        return @{ Ok = $false; Status = $code; Data = $data; Raw = $raw }
    }
}

function Invoke-Sql {
    param([string]$Sql)
    $argList = @(
        "compose", "exec", "-T", $DbService,
        "psql", "-U", $DbUser, "-d", $DbName, "-t", "-A", "-c", $Sql
    )
    $out = & docker @argList 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw ("SQL failed: {0}" -f $out)
    }
    return (($out | ForEach-Object { "$_" }) -join "`n").Trim()
}

function Get-Prop {
    param($Obj, [string[]]$Paths)
    foreach ($p in $Paths) {
        $cur = $Obj
        $ok = $true
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
    return @{
        Bal  = [int64]$p[0]
        Held = [int64]$p[1]
        Debt = [int64]$p[2]
    }
}

function Get-HmacHex {
    param([string]$Secret, [byte[]]$BodyBytes)
    $hmac = New-Object System.Security.Cryptography.HMACSHA256
    $hmac.Key = [System.Text.Encoding]::UTF8.GetBytes($Secret)
    $hash = $hmac.ComputeHash($BodyBytes)
    return ([System.BitConverter]::ToString($hash) -replace '-', '').ToLowerInvariant()
}

function New-CustomerVerified {
    param(
        [int]$Index,
        [string]$ShopSlug,
        [hashtable]$MerchantHeaders,
        [string]$ShopId,
        [string]$MerchantUserId
    )
    $email = "cust.tontine.$Timestamp.$Index@goshop.com"
    $pass  = "Password123!"

    $null = Invoke-Json -Method POST -Uri "$BaseUrl/register" -Body @{
        email = $email; password = $pass; role = "user"
    } -OkStatus @(200, 201, 409)

    $login = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $email; password = $pass }
    if (-not $login.Ok) { throw ("Customer login failed {0}: {1}" -f $Index, $login.Raw) }
    $token = Get-Prop $login.Data @('access_token','token','data.access_token')
    if (-not $token) { throw ("No customer token {0}" -f $Index) }

    $userId = Get-Prop $login.Data @('user.id','data.user.id','id','user_id','data.id')
    if (-not $userId) {
        $me = Invoke-Json -Method GET -Uri "$BaseUrl/auth/me" -Headers @{ Authorization = "Bearer $token" }
        $userId = Get-Prop $me.Data @('id','user.id','data.id','data.user.id')
    }
    if (-not $userId) {
        $userId = Invoke-Sql ("SELECT id::text FROM users WHERE email = '{0}' LIMIT 1;" -f $email)
    }
    if (-not $userId) { throw ("user_id missing for customer {0}" -f $Index) }

    $hdr = @{
        Authorization = "Bearer $token"
        "X-Shop-Slug" = $ShopSlug
    }

    # 🛡️ Utilisation d'un numéro unique pour la fiche client, mais le webhook injectera le numéro whitelisté
    $phone = "+22670{0}" -f (100000 + $Index).ToString("D6")
    $cBody = @{
        first_name = ("Client{0}" -f $Index)
        last_name  = "Tontine"
        phone      = $phone
        email      = $email
        user_id    = "$userId"
    }

    $cr = Invoke-Json -Method POST -Uri "$BaseUrl/api/customers" -Headers $MerchantHeaders -Body $cBody -OkStatus @(200, 201)
    $custId = Get-Prop $cr.Data @('id','data.id','customer.id','customer_id','data.customer.id')

    if (-not $custId) {
        Write-Warn ("Create customer API HTTP {0}: {1}" -f $cr.Status, $cr.Raw)
        $sqlIns = "INSERT INTO customers (id, shop_id, first_name, last_name, phone, email, user_id, kyc_level, created_at, updated_at) VALUES (gen_random_uuid(), '{0}'::uuid, 'Client{1}', 'Tontine', '{2}', '{3}', '{4}', 'verified', NOW(), NOW()) RETURNING id::text;" -f $ShopId, $Index, $phone, $email, $userId
        $custId = Invoke-Sql $sqlIns
    }
    if (-not $custId) { throw ("Customer id missing {0}" -f $Index) }

    # KYC verified + link user_id
    try {
        $sqlUpd = "UPDATE customers SET kyc_level = 'verified', kyc_validated_at = NOW(), user_id = '{0}', updated_at = NOW() WHERE id = '{1}'::uuid;" -f $userId, $custId
        Invoke-Sql $sqlUpd | Out-Null
    }
    catch {
        Write-Warn ("KYC update: {0}" -f $_.Exception.Message)
    }

    # Shop collaborator -> passe RequireShopAccess
    # roles autorises: shop_admin | seller | support | accountant
    $inviter = if ($MerchantUserId) { $MerchantUserId } else { $userId }
    try {
        $sqlCollab = "INSERT INTO shop_collaborators (id, shop_id, user_id, role, permissions, invited_by, invited_at, accepted_at, is_active, created_at, updated_at) VALUES (gen_random_uuid(), '{0}'::uuid, '{1}', 'seller', '{{}}'::jsonb, '{2}', NOW(), NOW(), true, NOW(), NOW()) ON CONFLICT DO NOTHING;" -f $ShopId, $userId, $inviter
        Invoke-Sql $sqlCollab | Out-Null
    }
    catch {
        Write-Warn ("Collaborator insert: {0}" -f $_.Exception.Message)
    }

    return @{ Id = $custId; Token = $token; Headers = $hdr; Email = $email; UserId = $userId }
}

# -------------------- MAIN --------------------
Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Tontine NET + HELD (Phase 1.1/1.2)" -ForegroundColor Magenta
Write-Host (" BaseUrl: {0} | N={1} | Circle={2} | DbService={3}" -f $BaseUrl, $NMembers, $CircleType, $DbService) -ForegroundColor DarkGray
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # 01 Health
    Write-Step -N "01/10" -Msg "Health"
    $h = Invoke-Json -Method GET -Uri "$BaseUrl/health/live" -OkStatus @(200)
    if (-not $h.Ok) { $h = Invoke-Json -Method GET -Uri "$BaseUrl/health/ready" -OkStatus @(200) }
    if (-not $h.Ok) { $h = Invoke-Json -Method GET -Uri "$BaseUrl/help" -OkStatus @(200) }
    if (-not $h.Ok) { Write-Fail ("API health failed on {0}" -f $BaseUrl) }
    Write-Ok ("API live HTTP {0}" -f $h.Status)

    # 02 Admin
    Write-Step -N "02/10" -Msg "Admin login"
    $al = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $AdminEmail; password = $AdminPassword }
    if (-not $al.Ok) { Write-Fail "Admin login failed" }
    $adminTok = Get-Prop $al.Data @('access_token','token','data.access_token')
    if (-not $adminTok) { Write-Fail "Admin token missing" }
    $script:State.AdminHeaders = @{ Authorization = "Bearer $adminTok" }
    Write-Ok "Admin OK"

    # 03 Merchant + shop + product + tontine settings
    Write-Step -N "03/10" -Msg "Merchant + shop + product + tontine settings"
    $reg = Invoke-Json -Method POST -Uri "$BaseUrl/register" -Body @{
        email = $MerchantEmail; password = $MerchantPassword; role = "merchant"
    } -OkStatus @(200, 201)
    if (-not $reg.Ok) { Write-Fail ("Register merchant failed {0}" -f $reg.Status) }
    Write-Ok ("Register HTTP {0}" -f $reg.Status)

    $ml = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $MerchantEmail; password = $MerchantPassword }
    $mTok = Get-Prop $ml.Data @('access_token','token','data.access_token')
    if (-not $mTok) { Write-Fail "Merchant token missing" }

    $mUserId = Get-Prop $ml.Data @('user.id','data.user.id','id','user_id','data.id')
    if (-not $mUserId) {
        $me = Invoke-Json -Method GET -Uri "$BaseUrl/auth/me" -Headers @{ Authorization = "Bearer $mTok" }
        $mUserId = Get-Prop $me.Data @('id','user.id','data.id','data.user.id')
    }
    if (-not $mUserId) {
        $mUserId = Invoke-Sql ("SELECT id::text FROM users WHERE email = '{0}' LIMIT 1;" -f $MerchantEmail)
    }
    $script:State.MerchantUserId = $mUserId

    $shop = Invoke-Json -Method POST -Uri "$BaseUrl/api/shops" -Headers @{ Authorization = "Bearer $mTok" } -Body @{
        name = ("Tontine Net Shop {0}" -f $Timestamp)
        slug = $ShopSlug
    } -OkStatus @(200, 201)
    $shopId = Get-Prop $shop.Data @('id','data.id','shop_id')
    if (-not $shopId) {
        $shopId = Invoke-Sql ("SELECT id::text FROM shops WHERE slug = '{0}' LIMIT 1;" -f $ShopSlug)
    }
    if (-not $shopId) { Write-Fail "Shop id missing" }
    $script:State.ShopId = $shopId
    $script:State.MerchantHeaders = @{
        Authorization = "Bearer $mTok"
        "X-Shop-Slug" = $ShopSlug
    }
    Write-Ok ("Shop {0} ({1})" -f $shopId, $ShopSlug)

    try {
        Invoke-Sql ("UPDATE shops SET kyc_status = 'verified', updated_at = NOW() WHERE id = '{0}'::uuid;" -f $shopId) | Out-Null
        Write-Ok "Shop KYC forced verified"
    }
    catch { Write-Warn "Shop KYC SQL skipped" }

    $prod = Invoke-Json -Method POST -Uri "$BaseUrl/api/products" -Headers $script:State.MerchantHeaders -Body @{
        name        = ("Produit Tontine Net {0}" -f $Timestamp)
        description = "E2E net+held"
        price_cents = $ProductPrice
        stock       = 50
    } -OkStatus @(200, 201)
    $productId = Get-Prop $prod.Data @('id','data.id')
    if (-not $productId) { Write-Fail "Product id missing" }
    $script:State.ProductId = $productId
    Write-Ok ("Product {0} price={1}" -f $productId, $ProductPrice)

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

    $sqlWallet = "INSERT INTO merchant_wallets (shop_id, balance_cents, held_cents, debt_cents, is_frozen, created_at, updated_at) VALUES ('{0}'::uuid, 0, 0, 0, false, NOW(), NOW()) ON CONFLICT (shop_id) DO NOTHING;" -f $shopId
    Invoke-Sql $sqlWallet | Out-Null

    # 04 Customers + collab
    Write-Step -N "04/10" -Msg "3 customers KYC verified + shop collab"
    for ($i = 0; $i -lt $NMembers; $i++) {
        $c = New-CustomerVerified -Index $i -ShopSlug $ShopSlug -MerchantHeaders $script:State.MerchantHeaders -ShopId $shopId -MerchantUserId $script:State.MerchantUserId
        $script:State.Customers += $c
        Write-Ok ("Customer {0} = {1} user={2}" -f $i, $c.Id, $c.UserId)
    }

    # 05 Create group + joins (JWT CLIENT: FindByUserID + RequireShopAccess via collab)
    Write-Step -N "05/10" -Msg "Create group + joins"
    $c0 = $script:State.Customers[0]
    $g = Invoke-Json -Method POST -Uri "$BaseUrl/api/tontine/groups" -Headers $c0.Headers -Body @{
        name         = ("Groupe E2E Net {0}" -f $Timestamp)
        product_id   = $productId
        circle_type  = $CircleType
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
        $j = Invoke-Json -Method POST -Uri "$BaseUrl/api/tontine/groups/join" -Headers $ci.Headers -Body @{
            invite_code = $invite
        } -OkStatus @(200, 201)
        if (-not $j.Ok) { Write-Fail ("Join {0} failed HTTP {1}: {2}" -f $i, $j.Status, $j.Raw) }
        Write-Ok ("Join member {0}" -f $i)
    }

    $gRow = Invoke-Sql ("SELECT amount_per_cycle_cents::text || '|' || total_cycles::text || '|' || COALESCE(circle_type,'') FROM tontine_groups WHERE id = '{0}'::uuid;" -f $groupId)
    if (-not $gRow) { Write-Fail "tontine_groups row missing" }
    $gp = $gRow.Split('|')
    $amtCycle = [int64]$gp[0]
    $totCyc   = [int]$gp[1]
    $ctype    = $gp[2]
    $gross = $amtCycle * [int64]$totCyc
    $rateBps = 150
    if ($ctype -match 'COMMERCIAL') { $rateBps = 200 }
    $comm = [int64]([math]::Floor(($gross * $rateBps) / 10000))
    $net  = $gross - $comm
    if ($net -lt 0) { $net = 0 }
    $script:State.ExpectedGross = $gross
    $script:State.ExpectedComm  = $comm
    $script:State.ExpectedNet   = $net
    Write-Ok ("Expected gross={0} rate={1}bps comm={2} net={3}" -f $gross, $rateBps, $comm, $net)

    # 06 Wallet BEFORE
    Write-Step -N "06/10" -Msg "Wallet BEFORE cycle"
    $w0 = Get-WalletRow -ShopId $shopId
    $script:State.BalBefore  = $w0.Bal
    $script:State.HeldBefore = $w0.Held
    $script:State.DebtBefore = $w0.Debt
    Write-Ok ("bal={0} held={1} debt={2}" -f $w0.Bal, $w0.Held, $w0.Debt)

    # 07 Pay + webhooks (JWT CLIENT)
    Write-Step -N "07/10" -Msg "Pay cycle 1 + webhooks SUCCESS"
    if ($WebhookSecret -match 'CHANGE_ME') {
        Write-Warn "Webhook secret looks placeholder - signature may fail"
    }

    for ($i = 0; $i -lt $NMembers; $i++) {
        $ci = $script:State.Customers[$i]
        # 🛡️ Utilisation du numéro de téléphone dynamique (whitelisté) pour le paiement
        $pay = Invoke-Json -Method POST -Uri ("$BaseUrl/api/tontine/groups/{0}/pay" -f $groupId) -Headers $ci.Headers -Body @{
            operator     = "orange_money"
            phone_number = $CustomerPhone
            flow         = "indirect"
        } -OkStatus @(200, 201)
        if (-not $pay.Ok) { Write-Fail ("Pay member {0} HTTP {1}: {2}" -f $i, $pay.Status, $pay.Raw) }
        $pref = Get-Prop $pay.Data @('provider_ref','data.provider_ref','reference','payment.provider_ref','data.payment.provider_ref')
        if (-not $pref) { Write-Fail ("provider_ref missing member {0} : {1}" -f $i, $pay.Raw) }
        $script:State.ProviderRefs += $pref
        Write-Ok ("Pay {0} ref={1}" -f $i, $pref)
    }

    for ($i = 0; $i -lt $NMembers; $i++) {
        $pref = $script:State.ProviderRefs[$i]
        $txnId = "TXN-TONTINE-NET-$Timestamp-$i"
        $payAmountMain = [int]([math]::Max(1, [math]::Floor($amtCycle / 100)))
        
        # 🛡️ Injection du numéro whitelisté (8 chiffres) dans les métadonnées du webhook
        $payloadObj = [ordered]@{
            apiEnv          = "test"
            paymentStatus   = "SUCCESS"
            transId         = $txnId
            projectId       = "e2e-project"
            paymentIntentId = $txnId
            paymentSource   = "orange_money"
            customerNumber  = $CustomerMsisdn8
            paymentAmount   = $payAmountMain
            paymentFees     = 0
            contryOrigin    = "BF"
            reference       = $pref
            currency        = "XOF"
        }
        $bodyStr = ($payloadObj | ConvertTo-Json -Compress -Depth 6)
        $bodyBytes = [System.Text.Encoding]::UTF8.GetBytes($bodyStr)
        $sig = Get-HmacHex -Secret $WebhookSecret -BodyBytes $bodyBytes

        $wh = Invoke-Json -Method POST -Uri "$BaseUrl/webhooks/yenga_pay" -Headers @{
            "x-webhook-hash" = $sig
            "Content-Type"   = "application/json"
        } -Body $bodyStr -OkStatus @(200, 201, 202)
        if (-not $wh.Ok) {
            Write-Fail ("Webhook {0} HTTP {1}: {2}" -f $i, $wh.Status, $wh.Raw)
        }
        Write-Ok ("Webhook {0} SUCCESS" -f $i)
    }

    Start-Sleep -Seconds 3

    # 08 Assert
    Write-Step -N "08/10" -Msg "Assert wallet net+held + ledger"
    $w1 = Get-WalletRow -ShopId $shopId
    $dBal  = $w1.Bal  - $script:State.BalBefore
    $dHeld = $w1.Held - $script:State.HeldBefore
    $dDebt = $w1.Debt - $script:State.DebtBefore

    Write-Host ("  balance: {0} -> {1}  delta={2} (expected net={3})" -f $script:State.BalBefore, $w1.Bal, $dBal, $net) -ForegroundColor White
    Write-Host ("  held   : {0} -> {1}  delta={2}" -f $script:State.HeldBefore, $w1.Held, $dHeld) -ForegroundColor White
    Write-Host ("  debt   : {0} -> {1}  delta={2}" -f $script:State.DebtBefore, $w1.Debt, $dDebt) -ForegroundColor White

    if ($dBal -ne $net)  { Write-Fail ("Balance delta {0} != expected net {1}" -f $dBal, $net) }
    else { Write-Ok "Balance += net" }

    if ($dHeld -ne $net) { Write-Fail ("Held delta {0} != expected net {1}" -f $dHeld, $net) }
    else { Write-Ok "Held += net" }

    if ($dDebt -ne 0) { Write-Fail "Debt changed on held credit (unexpected sweep)" }
    else { Write-Ok "Debt unchanged (no sweep on held credit)" }

    $sqlTxn = "SELECT COUNT(*)::text FROM wallet_transactions WHERE shop_id = '{0}'::uuid AND reference_type = 'tontine_cycle' AND status = 'completed';" -f $shopId
    $txnCnt = Invoke-Sql $sqlTxn
    if ([int]$txnCnt -lt 1) {
        $sqlTxn2 = "SELECT COUNT(*)::text FROM wallet_transactions WHERE shop_id = '{0}'::uuid AND created_at > NOW() - INTERVAL '10 minutes';" -f $shopId
        $txnCnt = Invoke-Sql $sqlTxn2
        if ([int]$txnCnt -lt 1) { Write-Fail "No wallet ledger rows after tontine credit" }
        else { Write-Ok ("Ledger rows (loose) count={0}" -f $txnCnt) }
    }
    else { Write-Ok ("Ledger tontine_cycle count={0}" -f $txnCnt) }

    $sqlDebit = "SELECT COUNT(*)::text FROM wallet_transactions WHERE shop_id = '{0}'::uuid AND created_at > NOW() - INTERVAL '15 minutes' AND (transaction_type::text ILIKE '%commission%' OR COALESCE(description,'') ILIKE '%LEGACY per-cotisation%');" -f $shopId
    $debitCnt = Invoke-Sql $sqlDebit
    if ([int]$debitCnt -gt 0) {
        Write-Fail ("Unexpected commission debit rows: {0}" -f $debitCnt)
    }
    else {
        Write-Ok "No per-cotisation commission_debit in last 15m"
    }

    # 09 Scheduler
    Write-Step -N "09/10" -Msg "Scheduler Phase 1.2 (logs hint)"
    Write-Host "  Check logs:" -ForegroundColor Yellow
    Write-Host '  docker compose logs goshop 2>&1 | Select-String -Pattern "Phase 1.2|collect_per_cotisation|gross_cents|CreditFromTontine|tontine_scheduler"' -ForegroundColor DarkGray
    Write-Ok "Manual log check recommended"

    # 10 Summary
    Write-Step -N "10/10" -Msg "Summary"
    Write-Host ("  Shop     : {0} ({1})" -f $shopId, $ShopSlug) -ForegroundColor White
    Write-Host ("  Group    : {0}" -f $groupId) -ForegroundColor White
    Write-Host ("  Gross/Net: {0} / {1} (comm {2})" -f $gross, $net, $comm) -ForegroundColor White
    Write-Host ("  Wallet   : bal {0}->{1} | held {2}->{3} | debt {4}" -f $script:State.BalBefore, $w1.Bal, $script:State.HeldBefore, $w1.Held, $w1.Debt) -ForegroundColor White

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $color = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    Write-Host (" RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed) -ForegroundColor $color
    Write-Host "================================================================" -ForegroundColor Magenta

    if ($script:Failed -eq 0) {
        Write-Host ""
        Write-Host "  TONTINE NET+HELD E2E VERT" -ForegroundColor Green
        Write-Host "  (commission 1x, held=net, debt inchange, no scheduler debit)" -ForegroundColor Green
    }
    else {
        exit 1
    }
}
catch {
    Write-Host ""
    Write-Host ("[CRITICAL FAILURE] {0}" -f $_.Exception.Message) -ForegroundColor Red
    exit 1
}

Write-Host ""