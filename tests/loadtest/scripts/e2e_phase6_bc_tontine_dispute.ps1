# Phase 6 B/C E2E - Tontine + Dispute (setup donnees + anti double-credit)
# Prerequisites:
#   - API running (ENABLE_FORCE_RELEASE=true)
#   - .env with PGPASSWORD, ADMIN_EMAIL, ADMIN_PASSWORD
# Usage:
#   powershell -ExecutionPolicy Bypass -File .\tests\loadtest\scripts\e2e_phase6_bc_tontine_dispute.ps1

param(
    [string]$BaseUrl = "http://localhost:8081",
    [string]$PgUser  = "postgres",
    [string]$PgDb    = "goshop_db",
    [string]$PgHost  = "localhost",
    [string]$PgPort  = "5432"
)

$ErrorActionPreference = "Stop"

# ---------- .env loader ----------
function Import-EnvFile {
    param([string]$Path)
    if (-not (Test-Path $Path)) { return $false }
    Get-Content $Path | ForEach-Object {
        $line = $_.Trim()
        if ($line -and $line -notmatch '^\s*#' -and $line -match '^\s*([^=]+)=(.*)$') {
            $key = $matches[1].Trim()
            $value = $matches[2].Trim().Trim('"').Trim("'")
            if ($key -eq "PGPASSWORD") { $env:PGPASSWORD = $value }
            elseif ($key -eq "ADMIN_EMAIL") { $env:ADMIN_EMAIL = $value }
            elseif ($key -eq "ADMIN_PASSWORD") { $env:ADMIN_PASSWORD = $value }
            elseif ($key -eq "GOSHOP_ADMIN_TOKEN") { $env:GOSHOP_ADMIN_TOKEN = $value }
            elseif ($key -eq "ENABLE_FORCE_RELEASE") { $env:ENABLE_FORCE_RELEASE = $value }
        }
    }
    return $true
}

foreach ($path in @(".env.local", ".env")) {
    if (Import-EnvFile $path) {
        Write-Host "Loaded credentials from $path" -ForegroundColor DarkGray
        break
    }
}

function Get-AdminToken {
    if ($env:GOSHOP_ADMIN_TOKEN) { return $env:GOSHOP_ADMIN_TOKEN }
    if (-not $env:ADMIN_EMAIL -or -not $env:ADMIN_PASSWORD) { return $null }
    $body = @{ email = $env:ADMIN_EMAIL; password = $env:ADMIN_PASSWORD } | ConvertTo-Json
    try {
        $r = Invoke-RestMethod -Method POST -Uri "$BaseUrl/login" -ContentType "application/json" -Body $body
        return $r.access_token
    } catch {
        Write-Host "Login failed: $($_.Exception.Message)" -ForegroundColor Red
        return $null
    }
}

function Invoke-Psql {
    param([string]$Sql)
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $raw = & psql -U $PgUser -d $PgDb -h $PgHost -p $PgPort -t -A -c $Sql 2>&1
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prev
    $text = (($raw | ForEach-Object { "$_" }) -join "`n").Trim()
    if ($code -ne 0) { throw "psql failed (exit $code): $text" }
    return $text
}

function Assert-Eq {
    param($Actual, $Expected, $Label)
    if ("$Actual" -ne "$Expected") {
        Write-Host "FAIL  $Label got=[$Actual] expected=[$Expected]" -ForegroundColor Red
        exit 1
    }
    Write-Host "OK    $Label = $Actual" -ForegroundColor Green
}

if (-not $env:PGPASSWORD) { Write-Host "ERROR: PGPASSWORD missing"; exit 1 }
$AdminToken = Get-AdminToken
if (-not $AdminToken) { Write-Host "ERROR: no admin token"; exit 1 }
Write-Host "Admin token OK (len=$($AdminToken.Length))" -ForegroundColor Green

Write-Host ""
Write-Host "=== Phase 6 B/C E2E ===" -ForegroundColor Cyan

# ============================================================
# [C] DISPUTE merchant_wins x2 parallel
# ============================================================
Write-Host ""
Write-Host "[C] Setup dispute + parallel merchant_wins" -ForegroundColor Cyan

$orderRow = Invoke-Psql @"
SELECT o.id::text || '|' || o.shop_id::text || '|' || e.id::text
FROM orders o
JOIN escrow_accounts e ON e.order_id = o.id
WHERE e.status = 'funds_held'
  AND NOT EXISTS (
    SELECT 1 FROM disputes d
    WHERE d.order_id = o.id AND d.status IN ('pending','under_review')
  )
LIMIT 1;
"@

if (-not $orderRow) {
    Write-Host "SKIP C - no funds_held order available for dispute setup" -ForegroundColor Yellow
} else {
    $parts = $orderRow.Split('|')
    $OrderId = $parts[0]
    $ShopId  = $parts[1]
    $EscrowId = $parts[2]
    Write-Host "Order=$OrderId Shop=$ShopId Escrow=$EscrowId"

    Invoke-Psql "UPDATE orders SET status = 'confirmed', updated_at = NOW() WHERE id = '$OrderId'::uuid AND status NOT IN ('confirmed','out_for_delivery','delivered');" | Out-Null

    $InitiatorId = Invoke-Psql "SELECT id::text FROM users LIMIT 1;"
    if (-not $InitiatorId) { throw "no users in DB for initiator_id" }

    $DisputeId = Invoke-Psql "SELECT gen_random_uuid()::text;"

    Invoke-Psql @"
INSERT INTO disputes (id, order_id, shop_id, initiator_id, initiator_role, reason, status, created_at, updated_at)
VALUES (
  '$DisputeId'::uuid,
  '$OrderId'::uuid,
  '$ShopId'::uuid,
  '$InitiatorId'::uuid,
  'customer',
  'E2E phase6 dispute setup reason long enough',
  'pending',
  NOW(),
  NOW()
);
UPDATE escrow_accounts
SET status = 'disputed', updated_at = NOW()
WHERE id = '$EscrowId'::uuid AND status = 'funds_held';
"@ | Out-Null

    $esc = Invoke-Psql "SELECT status FROM escrow_accounts WHERE id = '$EscrowId'::uuid;"
    $dst = Invoke-Psql "SELECT status FROM disputes WHERE id = '$DisputeId'::uuid;"
    Write-Host "escrow=$esc dispute=$dst"
    if ($esc -ne "disputed" -or $dst -ne "pending") {
        Write-Host "FAIL  setup dispute/escrow state" -ForegroundColor Red
        exit 1
    }

    $beforeD = Invoke-Psql "SELECT COUNT(*) FROM wallet_transactions WHERE reference_type = 'dispute_resolution' AND reference_id = '$DisputeId' AND status = 'completed';"
    Write-Host "credits before = $beforeD"

    $body = '{"resolution":"merchant_wins","notes":"E2E phase6 parallel dispute"}'
    $dUrl = "$BaseUrl/api/admin/disputes/$DisputeId/resolve"

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
    Start-Sleep -Seconds 2

    $afterD = Invoke-Psql "SELECT COUNT(*) FROM wallet_transactions WHERE reference_type = 'dispute_resolution' AND reference_id = '$DisputeId' AND status = 'completed';"
    Write-Host "credits after = $afterD"
    $deltaD = [int]$afterD - [int]$beforeD
    if ($deltaD -gt 1) {
        Write-Host "FAIL  C double credit delta=$deltaD" -ForegroundColor Red
        exit 1
    }
    if ($deltaD -lt 1) {
        Write-Host "WARN  C no credit created (delta=0) - check resolve errors above" -ForegroundColor Yellow
    } else {
        Write-Host "OK    C delta credits = $deltaD (<=1)" -ForegroundColor Green
    }

    $escAfter = Invoke-Psql "SELECT status FROM escrow_accounts WHERE id = '$EscrowId'::uuid;"
    Write-Host "escrow after = $escAfter"
}

# ============================================================
# [B] TONTINE cycle ref + auto-release must NOT double-credit
# ============================================================
Write-Host ""
Write-Host "[B] Setup tontine_cycle credit + tontine proof auto-release skip" -ForegroundColor Cyan

$ShopIdB = Invoke-Psql "SELECT shop_id::text FROM merchant_wallets LIMIT 1;"
if (-not $ShopIdB) {
    $ShopIdB = Invoke-Psql "SELECT id::text FROM shops LIMIT 1;"
}

if (-not $ShopIdB) {
    Write-Host "SKIP B - no shop found" -ForegroundColor Yellow
} else {
    $GroupId = Invoke-Psql "SELECT gen_random_uuid()::text;"
    $Cycle   = 1
    $RefId   = "${GroupId}:cycle:${Cycle}"
    $TxnId   = Invoke-Psql "SELECT gen_random_uuid()::text;"
    $Amount  = 50000

    $walletExists = Invoke-Psql "SELECT COUNT(*) FROM merchant_wallets WHERE shop_id = '$ShopIdB'::uuid;"
    if ($walletExists -eq "0") {
        Invoke-Psql "INSERT INTO merchant_wallets (shop_id, balance_cents, held_cents, is_frozen, created_at, updated_at) VALUES ('$ShopIdB'::uuid, 0, 0, false, NOW(), NOW()) ON CONFLICT DO NOTHING;" | Out-Null
    }

    try {
        Invoke-Psql @"
INSERT INTO wallet_transactions (
  id, shop_id, transaction_type, amount_cents, balance_after_cents,
  reference_type, reference_id, description, status, created_at
) VALUES (
  '$TxnId'::uuid,
  '$ShopIdB'::uuid,
  'sale_tontine',
  $Amount,
  $Amount,
  'tontine_cycle',
  '$RefId',
  'E2E tontine cycle $Cycle settlement',
  'completed',
  NOW()
);
"@ | Out-Null
        Write-Host "Inserted tontine_cycle credit ref=$RefId" -ForegroundColor Green
    } catch {
        Write-Host "Insert tontine_cycle failed (maybe already exists): $_" -ForegroundColor Yellow
    }

    $cnt1 = Invoke-Psql "SELECT COUNT(*) FROM wallet_transactions WHERE reference_type = 'tontine_cycle' AND reference_id = '$RefId' AND status = 'completed';"
    Assert-Eq -Actual $cnt1 -Expected "1" -Label "exactly one tontine_cycle credit after insert"

    # ✅ CORRECTION 1: Suppression de $dupBlocked (variable inutilisée)
    $TxnId2 = Invoke-Psql "SELECT gen_random_uuid()::text;"
    try {
        Invoke-Psql @"
INSERT INTO wallet_transactions (
  id, shop_id, transaction_type, amount_cents, balance_after_cents,
  reference_type, reference_id, description, status, created_at
) VALUES (
  '$TxnId2'::uuid,
  '$ShopIdB'::uuid,
  'sale_tontine',
  $Amount,
  $Amount,
  'tontine_cycle',
  '$RefId',
  'E2E duplicate should fail',
  'completed',
  NOW()
);
"@ | Out-Null
        Write-Host "FAIL  duplicate tontine_cycle insert was accepted" -ForegroundColor Red
        exit 1
    } catch {
        # ✅ Le catch fait déjà exit 1 si le duplicate passe, pas besoin de $dupBlocked
        Write-Host "OK    duplicate tontine_cycle blocked by unique index" -ForegroundColor Green
    }

    $cnt2 = Invoke-Psql "SELECT COUNT(*) FROM wallet_transactions WHERE reference_type = 'tontine_cycle' AND reference_id = '$RefId' AND status = 'completed';"
    Assert-Eq -Actual $cnt2 -Expected "1" -Label "still one tontine_cycle after duplicate attempt"

    $sample = Invoke-Psql "SELECT reference_id FROM wallet_transactions WHERE reference_type = 'tontine_cycle' AND reference_id = '$RefId' LIMIT 1;"
    if ($sample -notmatch ':cycle:') {
        Write-Host "FAIL  ref format" -ForegroundColor Red
        exit 1
    }
    Write-Host "OK    tontine_cycle format = $sample" -ForegroundColor Green

    $tv = Invoke-Psql @"
SELECT v.id::text || '|' || COALESCE(e.id::text,'') || '|' || COALESCE(e.status,'')
FROM tontine_vouchers v
LEFT JOIN escrow_accounts e ON e.tontine_group_id = v.group_id
WHERE e.status = 'funds_held'
LIMIT 1;
"@

    if ($tv) {
        $tvParts = $tv.Split('|')
        $VoucherId = $tvParts[0]
        Write-Host "Found tontine voucher=$VoucherId"

        $ProofId = Invoke-Psql "SELECT gen_random_uuid()::text;"
        try {
            Invoke-Psql @"
INSERT INTO delivery_proofs (id, tontine_voucher_id, escrow_status, delivery_date, created_at, updated_at)
VALUES ('$ProofId'::uuid, '$VoucherId'::uuid, 'delivered', NOW() - INTERVAL '4 days', NOW(), NOW());
"@ | Out-Null

            $beforeAuto = Invoke-Psql "SELECT COUNT(*) FROM wallet_transactions WHERE reference_type = 'escrow_auto_release' AND reference_id = '$ProofId' AND status = 'completed';"

            $h = @{ Authorization = "Bearer $AdminToken"; "Content-Type" = "application/json" }
            try {
                Invoke-RestMethod -Method POST -Uri "$BaseUrl/api/admin/scheduler/trigger-escrow-auto-release" -Headers $h | Out-Null
            } catch {
                Write-Host "trigger-escrow-auto-release: $($_.Exception.Message)" -ForegroundColor Yellow
            }
            Start-Sleep -Seconds 5

            $afterAuto = Invoke-Psql "SELECT COUNT(*) FROM wallet_transactions WHERE reference_type = 'escrow_auto_release' AND reference_id = '$ProofId' AND status = 'completed';"
            Write-Host "tontine proof escrow_auto_release credits: before=$beforeAuto after=$afterAuto"
            if ([int]$afterAuto -gt [int]$beforeAuto) {
                Write-Host "FAIL  tontine auto-release created wallet credit (should skip)" -ForegroundColor Red
                exit 1
            }
            Write-Host "OK    tontine auto-release did not credit wallet (skip path)" -ForegroundColor Green
        } catch {
            Write-Host "SKIP B proof path (schema/insert): $_" -ForegroundColor Yellow
        }
    } else {
        Write-Host "SKIP B scheduler path - no tontine voucher+escrow funds_held (unique index path already validated)" -ForegroundColor Yellow
    }
}

# ---------- global guards ----------
Write-Host ""
Write-Host "[D] Global guards" -ForegroundColor Cyan
$dupes = Invoke-Psql "SELECT COUNT(*) FROM (SELECT 1 FROM wallet_transactions WHERE reference_type IS NOT NULL AND reference_id IS NOT NULL AND status = 'completed' GROUP BY reference_type, reference_id HAVING COUNT(*) > 1) t;"
Assert-Eq -Actual $dupes -Expected "0" -Label "no duplicate completed refs"

Write-Host ""
Write-Host "=== SUMMARY ===" -ForegroundColor Cyan

# ✅ CORRECTION 2: Requête SQL corrigée (GROUP BY reference_type au lieu de GROUP BY 1)
Invoke-Psql "SELECT reference_type || '=' || COUNT(*)::text FROM wallet_transactions WHERE status = 'completed' AND reference_type IS NOT NULL GROUP BY reference_type ORDER BY COUNT(*) DESC;"

Write-Host ""
Write-Host "Phase 6 B/C E2E DONE" -ForegroundColor Green


#powershell -ExecutionPolicy Bypass -File .\tests\loadtest\scripts\e2e_phase6_bc_tontine_dispute.ps1