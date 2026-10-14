# scripts/reconciliation-report.ps1
# Rapport de réconciliation financière en lecture seule (Lot C) - Version robuste

$ErrorActionPreference = "Stop"
$DbName = "goshop_db"
$DbUser = "postgres"

Write-Host "========================================================" -ForegroundColor Cyan
Write-Host " RAPPORT DE RÉCONCILIATION FINANCIÈRE (Lecture Seule)" -ForegroundColor Cyan
Write-Host "========================================================" -ForegroundColor Cyan

try {
    $query = @"
-- 1. État global des portefeuilles
SELECT 'Total Wallet Balance (FCFA)' AS metric, 
       (SUM(balance_cents) / 100)::text AS value 
FROM merchant_wallets

UNION ALL

-- 2. Retraits en attente de compensation (Alerte P0)
SELECT 'Withdrawals Stuck in Compensation' AS metric, 
       COUNT(*)::text AS value 
FROM withdrawals 
WHERE status = 'compensation_pending'

UNION ALL

-- 3. Dettes résiduelles actives (Debt Sweep en attente)
SELECT 'Active Debt Cents (FCFA)' AS metric, 
       (SUM(debt_cents) / 100)::text AS value 
FROM merchant_wallets 
WHERE debt_cents > 0

UNION ALL

-- 4. Escrows bloqués par un litige
SELECT 'Escrows Disputed' AS metric, 
       COUNT(*)::text AS value 
FROM escrow_accounts 
WHERE status = 'disputed';
"@

    Write-Host "Exécution des requêtes de contrôle..." -ForegroundColor Yellow
    $result = docker compose exec -T db psql -U $DbUser -d $DbName -c $query

    Write-Host $result -ForegroundColor White

    # Analyse robuste des alertes (requête dédiée pour éviter les problèmes de parsing)
    $stuckCount = docker compose exec -T db psql -U $DbUser -d $DbName -t -A -c "SELECT COUNT(*) FROM withdrawals WHERE status = 'compensation_pending';"
    
    if ([int]$stuckCount -gt 0) {
        Write-Host "⚠️  ALERTE : $stuckCount retrait(s) bloqué(s) en compensation_pending." -ForegroundColor Red
        Write-Host "   👉 Action requise : Vérifier le job de retry et les logs YengaPay." -ForegroundColor Yellow
    } else {
        Write-Host "✅ Aucun retrait bloqué en compensation_pending." -ForegroundColor Green
    }

    Write-Host "========================================================" -ForegroundColor Cyan
    Write-Host " ✅ RAPPORT GÉNÉRÉ AVEC SUCCÈS" -ForegroundColor Green
    Write-Host "========================================================" -ForegroundColor Cyan

} catch {
    Write-Host "❌ Échec de la génération du rapport : $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}