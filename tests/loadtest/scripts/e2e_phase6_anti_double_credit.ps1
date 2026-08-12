# Phase 6 E2E - Anti double-credit wallet (avec setup des données)
# Usage:
#   powershell -ExecutionPolicy Bypass -File .\tests\loadtest\scripts\e2e_phase6_anti_double_credit.ps1

param(
    [string]$BaseUrl    = "http://localhost:8081",
    [string]$PgUser     = "postgres",
    [string]$PgDb       = "goshop_db",
    [string]$PgHost     = "localhost",
    [string]$PgPort     = "5432",
    [string]$OrderId    = "",
    [string]$DisputeId  = ""
)

$ErrorActionPreference = "Stop"

# ============================================================
# SECURE CREDENTIALS LOADER
# ============================================================

function Import-EnvFile {
    param([string]$Path)
    if (-not (Test-Path $Path)) { return $false }
    
    Get-Content $Path | ForEach-Object {
        $line = $_.Trim()
        if ($line -and $line -notmatch '^\s*#' -and $line -match '^\s*([^=]+)=(.*)$') {
            $key = $matches[1].Trim()
            $value = $matches[2].Trim().Trim('"').Trim("'")
            
            if ($key -eq "PGPASSWORD") {
                $env:PGPASSWORD = $value
                Write-Host "  ✅ PGPASSWORD loaded (length: $($value.Length))" -ForegroundColor DarkGray
            }
            elseif ($key -eq "GOSHOP_ADMIN_TOKEN") { $env:GOSHOP_ADMIN_TOKEN = $value }
            elseif ($key -eq "ADMIN_EMAIL") { $env:ADMIN_EMAIL = $value }
            elseif ($key -eq "ADMIN_PASSWORD") { $env:ADMIN_PASSWORD = $value }
            elseif ($key -eq "ENABLE_FORCE_RELEASE") { $env:ENABLE_FORCE_RELEASE = $value }
        }
    }
    return $true
}

$envPaths = @(".env.local", ".env", "../../.env.local", "../../.env")
foreach ($path in $envPaths) {
    if (Import-EnvFile $path) {
        Write-Host "📄 Loaded credentials from $path" -ForegroundColor DarkGray
        break
    }
}

# ============================================================
# AUTO-LOGIN ADMIN
# ============================================================

function Get-AdminToken {
    Write-Host ""
    Write-Host "🔐 Auto-login admin..." -ForegroundColor Cyan
    
    $adminEmail = $env:ADMIN_EMAIL
    $adminPassword = $env:ADMIN_PASSWORD
    
    if (-not $adminEmail -or -not $adminPassword) {
        Write-Host "  ⚠️ ADMIN_EMAIL/ADMIN_PASSWORD not set" -ForegroundColor Yellow
        return $null
    }
    
    $body = @{
        email    = $adminEmail
        password = $adminPassword
    } | ConvertTo-Json
    
    try {
        $response = Invoke-RestMethod -Method POST -Uri "$BaseUrl/login" `
            -ContentType "application/json" -Body $body
        
        if ($response.access_token) {
            Write-Host "  ✅ Admin token obtained (length: $($response.access_token.Length))" -ForegroundColor Green
            return $response.access_token
        }
        else {
            Write-Host "  ❌ No access_token in response" -ForegroundColor Red
            return $null
        }
    }
    catch {
        Write-Host "  ❌ Login failed: $($_.Exception.Message)" -ForegroundColor Red
        return $null
    }
}

$AdminToken = $env:GOSHOP_ADMIN_TOKEN
if (-not $AdminToken) {
    $AdminToken = Get-AdminToken
}

if (-not $env:PGPASSWORD) {
    Write-Host ""
    Write-Host "ERROR: PGPASSWORD not set" -ForegroundColor Red
    exit 1
}

# ============================================================
# HELPER FUNCTIONS
# ============================================================

function Invoke-Psql {
    param([string]$Sql)
    
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    
    $raw = & psql -U $PgUser -d $PgDb -h $PgHost -p $PgPort -t -A -c $Sql 2>&1
    $code = $LASTEXITCODE
    
    $ErrorActionPreference = $prev
    $text = (($raw | ForEach-Object { "$_" }) -join "`n").Trim()
    
    if ($code -ne 0) {
        throw "psql failed (exit $code): $text"
    }
    return $text
}

function Assert-Eq {
    param($Actual, $Expected, $Label)
    if ("$Actual" -ne "$Expected") {
        Write-Host "FAIL  $Label  got=[$Actual] expected=[$Expected]" -ForegroundColor Red
        exit 1
    }
    Write-Host "OK    $Label = $Actual" -ForegroundColor Green
}

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Uri,
        [string]$Token,
        [object]$Body = $null
    )
    
    $headers = @{
        "Authorization" = "Bearer $Token"
        "Content-Type"  = "application/json"
    }
    
    if ($Body) {
        $jsonBody = $Body | ConvertTo-Json -Depth 10
        return Invoke-RestMethod -Method $Method -Uri $Uri -Headers $headers -Body $jsonBody
    }
    else {
        return Invoke-RestMethod -Method $Method -Uri $Uri -Headers $headers
    }
}

# ============================================================
# MAIN TESTS
# ============================================================

Write-Host ""
Write-Host "=== Phase 6 E2E Anti double-credit ===" -ForegroundColor Cyan
Write-Host "Database: ${PgUser}@${PgHost}:${PgPort}/${PgDb}" -ForegroundColor DarkGray

if (-not $AdminToken) {
    Write-Host "ERROR: No admin token" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "[D] Index + duplicates" -ForegroundColor Cyan

$idx = Invoke-Psql "SELECT COUNT(*) FROM pg_indexes WHERE indexname = 'uq_wallet_txn_ref_completed';"
Assert-Eq -Actual $idx -Expected "1" -Label "uq_wallet_txn_ref_completed exists"

$dupes = Invoke-Psql "SELECT COUNT(*) FROM (SELECT 1 FROM wallet_transactions WHERE reference_type IS NOT NULL AND reference_id IS NOT NULL AND status = 'completed' GROUP BY reference_type, reference_id HAVING COUNT(*) > 1) t;"
Assert-Eq -Actual $dupes -Expected "0" -Label "no duplicate completed refs"

# ============================================================
# [A] Force auto-release x2 parallel
# ============================================================

Write-Host ""
Write-Host "[A] Force auto-release x2 parallel" -ForegroundColor Cyan

if (-not $OrderId) {
    # Chercher un order avec escrow funds_held ET une delivery proof existante
    $OrderId = Invoke-Psql "SELECT o.id::text FROM orders o JOIN escrow_accounts e ON e.order_id = o.id JOIN delivery_proofs dp ON dp.order_id = o.id WHERE e.status = 'funds_held' LIMIT 1;"
}

if (-not $OrderId) {
    Write-Host "  📝 No funds_held order+proof found, looking for order with escrow funds_held (no proof)..." -ForegroundColor Yellow
    
    # Chercher un order avec escrow funds_held mais SANS delivery proof
    $OrderId = Invoke-Psql "SELECT o.id::text FROM orders o JOIN escrow_accounts e ON e.order_id = o.id WHERE e.status = 'funds_held' AND NOT EXISTS (SELECT 1 FROM delivery_proofs dp WHERE dp.order_id = o.id) LIMIT 1;"
    
    if ($OrderId) {
        Write-Host "  📦 Found order $OrderId with escrow funds_held, injecting delivery proof..." -ForegroundColor Cyan
        
        # ✅ CORRECTION : Enlever ::text sur gen_random_uuid(), ajouter ::uuid sur order_id
        $injectSql = @"
INSERT INTO delivery_proofs (id, order_id, escrow_status, delivery_date, created_at, updated_at)
VALUES (gen_random_uuid(), '$OrderId'::uuid, 'delivered', NOW(), NOW(), NOW());
"@
        try {
            Invoke-Psql $injectSql | Out-Null
            Write-Host "  ✅ Delivery proof injected for order $OrderId" -ForegroundColor Green
        }
        catch {
            Write-Host "  ❌ Failed to inject delivery proof: $_" -ForegroundColor Red
            $OrderId = ""
        }
    }
}

if (-not $OrderId) {
    Write-Host "SKIP A - no funds_held order+proof found (even after injection attempt)" -ForegroundColor Yellow
}
else {
    Write-Host "OrderId = $OrderId"

    $before = Invoke-Psql "SELECT COUNT(*) FROM wallet_transactions WHERE reference_type = 'escrow_auto_release' AND status = 'completed' AND reference_id IN (SELECT id::text FROM delivery_proofs WHERE order_id = '$OrderId'::uuid);"
    Write-Host "credits before = $before"

    $url = "$BaseUrl/api/admin/scheduler/force-auto-release/${OrderId}?days=4"

    # Lancer 2 appels parallèles pour tester le double-crédit
    $j1 = Start-Job -ScriptBlock {
        param($u, $tok)
        $h = @{ Authorization = "Bearer $tok"; "Content-Type" = "application/json" }
        try { Invoke-RestMethod -Method POST -Uri $u -Headers $h | ConvertTo-Json -Compress } catch { $_.Exception.Message }
    } -ArgumentList $url, $AdminToken

    $j2 = Start-Job -ScriptBlock {
        param($u, $tok)
        $h = @{ Authorization = "Bearer $tok"; "Content-Type" = "application/json" }
        try { Invoke-RestMethod -Method POST -Uri $u -Headers $h | ConvertTo-Json -Compress } catch { $_.Exception.Message }
    } -ArgumentList $url, $AdminToken

    Wait-Job $j1, $j2 | Out-Null
    Write-Host "call1: $(Receive-Job $j1)"
    Write-Host "call2: $(Receive-Job $j2)"
    Remove-Job $j1, $j2 -Force

    Start-Sleep -Seconds 5

    $after = Invoke-Psql "SELECT COUNT(*) FROM wallet_transactions WHERE reference_type = 'escrow_auto_release' AND status = 'completed' AND reference_id IN (SELECT id::text FROM delivery_proofs WHERE order_id = '$OrderId'::uuid);"
    Write-Host "credits after = $after"

    $delta = [int]$after - [int]$before
    if ($delta -gt 1) {
        Write-Host "FAIL  A double credit delta=$delta" -ForegroundColor Red
        exit 1
    }
    Write-Host "OK    A delta credits = $delta (<=1)" -ForegroundColor Green

    $escrowStatus = Invoke-Psql "SELECT status FROM escrow_accounts WHERE order_id = '$OrderId'::uuid LIMIT 1;"
    Write-Host "escrow status = $escrowStatus"
}

# ============================================================
# [B] Tontine cycle refs
# ============================================================

Write-Host ""
Write-Host "[B] Tontine cycle refs" -ForegroundColor Cyan

$tontineDupes = Invoke-Psql "SELECT COUNT(*) FROM (SELECT 1 FROM wallet_transactions WHERE reference_type = 'tontine_cycle' AND status = 'completed' GROUP BY reference_id HAVING COUNT(*) > 1) t;"
Assert-Eq -Actual $tontineDupes -Expected "0" -Label "no duplicate tontine_cycle"

$sample = Invoke-Psql "SELECT COALESCE(reference_id, '') FROM wallet_transactions WHERE reference_type = 'tontine_cycle' AND status = 'completed' ORDER BY created_at DESC LIMIT 1;"
if ($sample) {
    Write-Host "sample tontine_cycle ref = $sample"
    if ($sample -notmatch ':cycle:') {
        Write-Host "FAIL  tontine_cycle ref must contain :cycle:" -ForegroundColor Red
        exit 1
    }
    Write-Host "OK    tontine_cycle format" -ForegroundColor Green
}
else {
    Write-Host "SKIP B sample - no tontine_cycle row yet" -ForegroundColor Yellow
}

# ============================================================
# [C] Dispute merchant_wins x2 parallel
# ============================================================

Write-Host ""
Write-Host "[C] Dispute merchant_wins x2 parallel" -ForegroundColor Cyan

if (-not $DisputeId) {
    # Chercher une dispute existante en statut pending/under_review avec escrow disputed
    $DisputeId = Invoke-Psql "SELECT d.id::text FROM disputes d JOIN escrow_accounts e ON e.order_id = d.order_id WHERE d.status IN ('pending', 'under_review') AND e.status = 'disputed' LIMIT 1;"
}

if (-not $DisputeId) {
    Write-Host "  📝 No pending dispute with disputed escrow found in DB" -ForegroundColor Yellow
    Write-Host "  💡 To create test data manually:" -ForegroundColor Gray
    Write-Host "     1. Create order + pay (webhook) → order confirmed, escrow funds_held" -ForegroundColor Gray
    Write-Host "     2. Customer creates dispute → escrow becomes 'disputed'" -ForegroundColor Gray
    Write-Host "     3. Re-run this script" -ForegroundColor Gray
    Write-Host "SKIP C - no pending dispute + disputed escrow" -ForegroundColor Yellow
}
else {
    Write-Host "DisputeId = $DisputeId"

    $beforeD = Invoke-Psql "SELECT COUNT(*) FROM wallet_transactions WHERE reference_type = 'dispute_resolution' AND reference_id = '$DisputeId' AND status = 'completed';"
    Write-Host "credits before = $beforeD"

    $body = '{"resolution":"merchant_wins","notes":"E2E phase6 parallel"}'
    $dUrl = "$BaseUrl/api/admin/disputes/$DisputeId/resolve"

    # Lancer 2 résolutions parallèles pour tester le double-crédit
    $dj1 = Start-Job -ScriptBlock {
        param($u, $tok, $b)
        $h = @{ Authorization = "Bearer $tok"; "Content-Type" = "application/json" }
        try { Invoke-RestMethod -Method POST -Uri $u -Headers $h -Body $b | ConvertTo-Json -Compress } catch { $_.Exception.Message }
    } -ArgumentList $dUrl, $AdminToken, $body

    $dj2 = Start-Job -ScriptBlock {
        param($u, $tok, $b)
        $h = @{ Authorization = "Bearer $tok"; "Content-Type" = "application/json" }
        try { Invoke-RestMethod -Method POST -Uri $u -Headers $h -Body $b | ConvertTo-Json -Compress } catch { $_.Exception.Message }
    } -ArgumentList $dUrl, $AdminToken, $body

    Wait-Job $dj1, $dj2 | Out-Null
    Write-Host "resolve1: $(Receive-Job $dj1)"
    Write-Host "resolve2: $(Receive-Job $dj2)"
    Remove-Job $dj1, $dj2 -Force

    Start-Sleep -Seconds 3

    $afterD = Invoke-Psql "SELECT COUNT(*) FROM wallet_transactions WHERE reference_type = 'dispute_resolution' AND reference_id = '$DisputeId' AND status = 'completed';"
    Write-Host "credits after = $afterD"
    
    $deltaD = [int]$afterD - [int]$beforeD
    if ($deltaD -gt 1) {
        Write-Host "FAIL  C double credit delta=$deltaD" -ForegroundColor Red
        exit 1
    }
    Write-Host "OK    C delta credits = $deltaD (<=1)" -ForegroundColor Green
}

# ============================================================
# SUMMARY
# ============================================================

Write-Host ""
Write-Host "=== SUMMARY by reference_type ===" -ForegroundColor Cyan
Invoke-Psql "SELECT reference_type || '=' || COUNT(*)::text FROM wallet_transactions WHERE status = 'completed' AND reference_type IS NOT NULL GROUP BY reference_type ORDER BY COUNT(*) DESC;"

Write-Host ""
Write-Host "Phase 6 E2E DONE" -ForegroundColor Green

# powershell -ExecutionPolicy Bypass -File .\tests\loadtest\scripts\e2e_phase6_anti_double_credit.ps1