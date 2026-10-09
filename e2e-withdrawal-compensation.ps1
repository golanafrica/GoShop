# ============================================================
# GOSHOP E2E - Withdrawal Compensation (P0 Validation)
# Scénario : Retrait débité -> Rejet définitif du provider -> Vérification du remboursement
# ============================================================

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

# 0. CHARGEMENT DU FICHIER .ENV
$envFile = ".env"
if (Test-Path $envFile) {
    Get-Content $envFile | ForEach-Object {
        if ($_ -match "^\s*([^#][^=]+)=(.*)$") {
            $key = $matches[1].Trim()
            $val = $matches[2].Trim().Trim('"').Trim("'")
            [Environment]::SetEnvironmentVariable($key, $val, "Process")
        }
    }
}

# 1. CONFIGURATION
$BaseUrl       = if ($env:GOSHOP_BASE_URL) { $env:GOSHOP_BASE_URL } else { "http://localhost:8080" }
$AdminEmail    = $env:ADMIN_EMAIL
$AdminPassword = $env:ADMIN_PASSWORD
$DbService     = if ($env:DB_SERVICE) { $env:DB_SERVICE } else { "db" }
$DbUser        = if ($env:DB_USER) { $env:DB_USER } else { "postgres" }
$DbName        = if ($env:DB_NAME) { $env:DB_NAME } else { "goshop_db" }

$Timestamp     = Get-Date -Format "yyyyMMddHHmmss"
$ShopSlug      = "withdrawal-comp-shop-$Timestamp"
$MerchantEmail = "merchant.comp.$Timestamp@goshop.com"
$MerchantPass  = "Password123!"

$script:Passed = 0
$script:Failed = 0

# 2. HELPERS
function Write-Step { param([string]$N, [string]$Msg); Write-Host ("[{0}] {1}" -f $N, $Msg) -ForegroundColor Cyan }
function Write-Ok { param([string]$Msg); Write-Host ("  [OK] {0}" -f $Msg) -ForegroundColor Green; $script:Passed++ }
function Write-Fail { param([string]$Msg); Write-Host ("  [FAIL] {0}" -f $Msg) -ForegroundColor Red; $script:Failed++; throw $Msg }

function Invoke-Json {
    param([string]$Method, [string]$Uri, [hashtable]$Headers = @{}, [object]$Body = $null, [int[]]$OkStatus = @(200, 201, 202, 400, 500))
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
        return @{ Ok = ($OkStatus -contains $code); Status = $code; Data = $data; Raw = $raw }
    }
}

function Invoke-Sql {
    param([string]$Sql)
    $argList = @("compose", "exec", "-T", $DbService, "psql", "-U", $DbUser, "-d", $DbName, "-t", "-A", "-c", $Sql)
    $out = & docker @argList 2>&1
    if ($LASTEXITCODE -ne 0) { throw ("SQL failed: {0}" -f $out) }
    return (($out | ForEach-Object { "$_" }) -join "`n").Trim()
}

# 3. MAIN EXECUTION
Write-Host "================================================================" -ForegroundColor Magenta
Write-Host " GOSHOP E2E - Withdrawal Compensation (P0 Validation)" -ForegroundColor Magenta
Write-Host "================================================================" -ForegroundColor Magenta

try {
    # 00. NETTOYAGE PRÉVENTIF
    Write-Step -N "00/06" -Msg "Cleaning old test data"
    Invoke-Sql "DELETE FROM wallet_transactions WHERE shop_id IN (SELECT id FROM shops WHERE slug LIKE 'withdrawal-comp-shop-%');" | Out-Null
    Invoke-Sql "DELETE FROM withdrawals WHERE shop_id IN (SELECT id FROM shops WHERE slug LIKE 'withdrawal-comp-shop-%');" | Out-Null
    Invoke-Sql "DELETE FROM merchant_wallets WHERE shop_id IN (SELECT id FROM shops WHERE slug LIKE 'withdrawal-comp-shop-%');" | Out-Null
    Invoke-Sql "DELETE FROM shops WHERE slug LIKE 'withdrawal-comp-shop-%';" | Out-Null
    Write-Ok "Old test data cleaned"

    # 01. LOGIN ADMIN & CRÉATION MARCHAND
    Write-Step -N "01/06" -Msg "Admin Login & Merchant Setup"
    $al = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $AdminEmail; password = $AdminPassword }
    if (-not $al.Ok) { Write-Fail "Admin login failed" }
    $adminTok = $al.Data.access_token
    if (-not $adminTok) { $adminTok = $al.Data.data.access_token }
    $adminHeaders = @{ Authorization = "Bearer $adminTok" }

    $null = Invoke-Json -Method POST -Uri "$BaseUrl/register" -Body @{ email = $MerchantEmail; password = $MerchantPass; role = "merchant" } -OkStatus @(200, 201)
    $ml = Invoke-Json -Method POST -Uri "$BaseUrl/login" -Body @{ email = $MerchantEmail; password = $MerchantPass }
    $mTok = $ml.Data.access_token
    if (-not $mTok) { $mTok = $ml.Data.data.access_token }
    $merchantHeaders = @{ Authorization = "Bearer $mTok"; "X-Shop-Slug" = $ShopSlug }

    $shop = Invoke-Json -Method POST -Uri "$BaseUrl/api/shops" -Headers $merchantHeaders -Body @{ name = "Comp Test Shop"; slug = $ShopSlug } -OkStatus @(200, 201)
    $shopId = $shop.Data.id
    if (-not $shopId) { $shopId = $shop.Data.data.id }
    Write-Ok "Merchant created (Shop ID: $shopId)"

    # 02. FUNDING & INITIAL STATE CHECK
    Write-Step -N "02/06" -Msg "Funding Wallet & Initial State Check"
    # On simule un solde initial de 1000 FCFA (100000 centimes)
    Invoke-Sql "UPDATE shops SET kyc_status = 'verified' WHERE id = '$shopId'::uuid;" | Out-Null
    Invoke-Sql "INSERT INTO merchant_wallets (shop_id, balance_cents, held_cents, debt_cents, is_frozen) VALUES ('$shopId'::uuid, 100000, 0, 0, false);" | Out-Null
    
    $initialBalance = Invoke-Sql "SELECT balance_cents FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    if ([int]$initialBalance -ne 100000) { Write-Fail "Initial balance setup failed. Expected 100000, got $initialBalance" }
    Write-Ok "Initial balance verified: 100000 cents (1000 FCFA)"

    # 03. TRIGGER WITHDRAWAL (Definitive Failure Simulation)
    Write-Step -N "03/06" -Msg "Triggering Withdrawal with Invalid Destination (Definitive Failure)"
    # On utilise un numéro de destination clairement invalide pour forcer un rejet définitif du provider (pas un timeout)
    $withdrawalReq = @{
        amount_cents = 50000
        payment_method = "orange_money"
        destination_number = "0000000000" # Numéro invalide pour forcer l'échec définitif
    }

    $wr = Invoke-Json -Method POST -Uri "$BaseUrl/api/withdrawals" -Headers $merchantHeaders -Body $withdrawalReq -OkStatus @(400, 500)
    
    # On s'attend à un échec (400 ou 500) car le provider va rejeter le numéro invalide
    if ($wr.Status -eq 200 -or $wr.Status -eq 201) {
        Write-Fail "Withdrawal should have failed definitively, but it succeeded or is processing."
    }
    Write-Ok "Withdrawal correctly rejected by provider (Status: $($wr.Status))"

    # Extraire l'ID du retrait depuis la réponse d'erreur ou la DB
    $withdrawalId = Invoke-Sql "SELECT id FROM withdrawals WHERE shop_id = '$shopId'::uuid ORDER BY created_at DESC LIMIT 1;"
    Write-Host "  [INFO] Withdrawal ID: $withdrawalId" -ForegroundColor Gray

    # 04. VERIFY DB STATE: WITHDRAWAL STATUS
    Write-Step -N "04/06" -Msg "Verifying Withdrawal Status in DB"
    $wStatus = Invoke-Sql "SELECT status FROM withdrawals WHERE id = '$withdrawalId'::uuid;"
    if ($wStatus -ne "failed") {
        Write-Fail "Withdrawal status should be 'failed', but got: $wStatus"
    }
    Write-Ok "Withdrawal status is correctly 'FAILED'"

    # 05. VERIFY DB STATE: WALLET BALANCE RESTORED
    Write-Step -N "05/06" -Msg "Verifying Wallet Balance Restoration (Compensation)"
    $finalBalance = Invoke-Sql "SELECT balance_cents FROM merchant_wallets WHERE shop_id = '$shopId'::uuid;"
    if ([int]$finalBalance -ne 100000) {
        Write-Fail "CRITICAL: Wallet balance was not restored! Expected 100000, got $finalBalance"
    }
    Write-Ok "Wallet balance correctly restored to 100000 cents"

    # 06. VERIFY DB STATE: LEDGER REVERSAL ENTRY
    Write-Step -N "06/06" -Msg "Verifying Ledger Reversal Entry"
    $reversalCount = Invoke-Sql "SELECT COUNT(*) FROM wallet_transactions WHERE shop_id = '$shopId'::uuid AND reference_type = 'withdrawal_reversal' AND reference_id = '$withdrawalId';"
    if ([int]$reversalCount -lt 1) {
        Write-Fail "CRITICAL: No 'withdrawal_reversal' transaction found in the ledger for this withdrawal!"
    }
    Write-Ok "Ledger correctly contains 'withdrawal_reversal' entry"

    # SUMMARY
    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Magenta
    $color = if ($script:Failed -eq 0) { "Green" } else { "Yellow" }
    Write-Host (" RESULT : {0} OK / {1} FAIL" -f $script:Passed, $script:Failed) -ForegroundColor $color
    Write-Host "================================================================" -ForegroundColor Magenta

    if ($script:Failed -eq 0) {
        Write-Host ""
        Write-Host "  WITHDRAWAL COMPENSATION E2E VERT" -ForegroundColor Green
        Write-Host "  (Definitive failure detected, balance restored, ledger updated)" -ForegroundColor Green
    } else {
        exit 1
    }
} catch {
    Write-Host ""
    Write-Host ("[CRITICAL FAILURE] {0}" -f $_.Exception.Message) -ForegroundColor Red
    exit 1
}
Write-Host ""