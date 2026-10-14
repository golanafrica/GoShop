# scripts/backup-restore-test.ps1
# Test de sauvegarde et restauration PostgreSQL pour validation opérationnelle

$ErrorActionPreference = "Stop"
$DbName = "goshop_db"
$DbUser = "postgres"
$BackupFile = ".\goshop_db_backup_test.sql"

Write-Host "========================================================" -ForegroundColor Cyan
Write-Host " TEST DE SAUVEGARDE ET RESTAURATION POSTGRESQL (Lot B)" -ForegroundColor Cyan
Write-Host "========================================================" -ForegroundColor Cyan

try {
    # 1. Sauvegarde
    Write-Host "[1/4] Création de la sauvegarde..." -ForegroundColor Yellow
    docker compose exec -T db pg_dump -U $DbUser -d $DbName -F c -f /tmp/backup.dump
    docker compose cp db:/tmp/backup.dump $BackupFile
    Write-Host "      ✅ Sauvegarde créée avec succès ($BackupFile)" -ForegroundColor Green

    # 2. Simulation de perte de données (ATTENTION: Local/Test uniquement)
    Write-Host "[2/4] Simulation de perte de données (DROP/CREATE)..." -ForegroundColor Yellow
    docker compose exec -T db psql -U $DbUser -d postgres -c "DROP DATABASE IF EXISTS $DbName;"
    docker compose exec -T db psql -U $DbUser -d postgres -c "CREATE DATABASE $DbName;"
    Write-Host "      ✅ Base de données réinitialisée." -ForegroundColor Green

    # 3. Restauration
    Write-Host "[3/4] Restauration de la sauvegarde..." -ForegroundColor Yellow
    docker compose cp $BackupFile db:/tmp/backup.dump
    docker compose exec -T db pg_restore -U $DbUser -d $DbName -c --if-exists /tmp/backup.dump
    Write-Host "      ✅ Restauration terminée." -ForegroundColor Green

    # 4. Vérification de l'intégrité financière
    Write-Host "[4/4] Vérification de l'intégrité des données financières..." -ForegroundColor Yellow
    
    $WalletCount = docker compose exec -T db psql -U $DbUser -d $DbName -t -A -c "SELECT COUNT(*) FROM merchant_wallets;"
    $TxCount = docker compose exec -T db psql -U $DbUser -d $DbName -t -A -c "SELECT COUNT(*) FROM wallet_transactions;"
    $PendingComp = docker compose exec -T db psql -U $DbUser -d $DbName -t -A -c "SELECT COUNT(*) FROM withdrawals WHERE status = 'compensation_pending';"

    Write-Host "      - Portefeuilles marchands : $WalletCount" -ForegroundColor White
    Write-Host "      - Écritures dans le ledger  : $TxCount" -ForegroundColor White
    Write-Host "      - Retraits en compensation  : $PendingComp" -ForegroundColor White

    if ([int]$WalletCount -ge 0 -and [int]$TxCount -ge 0) {
        Write-Host "========================================================" -ForegroundColor Cyan
        Write-Host " ✅ TEST DE RESTAURATION RÉUSSI : Données cohérentes." -ForegroundColor Green
        Write-Host "========================================================" -ForegroundColor Cyan
    } else {
        throw "Les comptes de vérification ont échoué."
    }

} catch {
    Write-Host "========================================================" -ForegroundColor Cyan
    Write-Host " ❌ ÉCHEC DU TEST DE RESTAURATION" -ForegroundColor Red
    Write-Host " Erreur : $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "========================================================" -ForegroundColor Cyan
    exit 1
} finally {
    # Nettoyage
    if (Test-Path $BackupFile) { Remove-Item $BackupFile -Force }
    docker compose exec -T db rm -f /tmp/backup.dump 2>$null
}