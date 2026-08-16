# start-observability.ps1
# Script PowerShell pour démarrer Prometheus localement sur Windows

$ErrorActionPreference = "Stop"
$ObservabilityDir = "$PSScriptRoot\observability"

Write-Host "🚀 GoShop - Démarrage de l'Observabilité" -ForegroundColor Green
Write-Host "=========================================" -ForegroundColor Green
Write-Host ""

# ============================================================
# ÉTAPE 1 : Vérifier si Prometheus est installé
# ============================================================
$PrometheusDir = "$ObservabilityDir\prometheus"
$PrometheusExe = "$PrometheusDir\prometheus.exe"

if (-Not (Test-Path $PrometheusExe)) {
    Write-Host "📦 Prometheus non trouvé. Installation en cours..." -ForegroundColor Yellow
    
    # Créer le dossier
    New-Item -ItemType Directory -Path $ObservabilityDir -Force | Out-Null
    
    # Télécharger Prometheus
    $PrometheusVersion = "2.47.0"
    $DownloadUrl = "https://github.com/prometheus/prometheus/releases/download/v$PrometheusVersion/prometheus-$PrometheusVersion.windows-amd64.zip"
    $ZipFile = "$ObservabilityDir\prometheus.zip"
    
    Write-Host "⬇️  Téléchargement de Prometheus v$PrometheusVersion..." -ForegroundColor Cyan
    try {
        Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipFile -UseBasicParsing
    } catch {
        Write-Host "❌ Erreur de téléchargement: $_" -ForegroundColor Red
        Write-Host ""
        Write-Host "💡 Téléchargez manuellement depuis :" -ForegroundColor Yellow
        Write-Host "   $DownloadUrl" -ForegroundColor White
        Write-Host "   Et extrayez dans: $PrometheusDir" -ForegroundColor White
        exit 1
    }
    
    # Extraire
    Write-Host "📂 Extraction..." -ForegroundColor Cyan
    Expand-Archive -Path $ZipFile -DestinationPath $ObservabilityDir -Force
    
    # Renommer
    $ExtractedDir = Get-ChildItem -Path $ObservabilityDir -Directory | Where-Object { $_.Name -like "prometheus-*" } | Select-Object -First 1
    if ($ExtractedDir) {
        Rename-Item -Path $ExtractedDir.FullName -NewName "prometheus"
    }
    
    # Nettoyer
    Remove-Item $ZipFile -Force
    
    Write-Host "✅ Prometheus installé avec succès !" -ForegroundColor Green
}

# ============================================================
# ÉTAPE 2 : Copier la configuration
# ============================================================
$ConfigSource = "$PSScriptRoot\prometheus.yml"
$ConfigDest = "$PrometheusDir\prometheus.yml"

if (Test-Path $ConfigSource) {
    Copy-Item -Path $ConfigSource -Destination $ConfigDest -Force
    Write-Host "✅ Configuration Prometheus copiée" -ForegroundColor Green
} else {
    Write-Host "⚠️  Fichier prometheus.yml non trouvé à la racine" -ForegroundColor Yellow
}

# Créer le dossier data
$DataDir = "$PrometheusDir\data"
if (-Not (Test-Path $DataDir)) {
    New-Item -ItemType Directory -Path $DataDir -Force | Out-Null
}

# ============================================================
# ÉTAPE 3 : Démarrer Prometheus
# ============================================================
Write-Host ""
Write-Host "🔄 Démarrage de Prometheus..." -ForegroundColor Cyan

$PrometheusProcess = Start-Process -FilePath $PrometheusExe `
    -ArgumentList "--config.file=$ConfigDest", "--storage.tsdb.path=$DataDir", "--web.listen-address=:9090" `
    -PassThru `
    -WindowStyle Normal

# Attendre que Prometheus démarre
Start-Sleep -Seconds 3

# Vérifier si Prometheus répond
try {
    $Response = Invoke-WebRequest -Uri "http://localhost:9090/-/healthy" -UseBasicParsing -TimeoutSec 5
    if ($Response.StatusCode -eq 200) {
        Write-Host "✅ Prometheus démarré avec succès !" -ForegroundColor Green
    }
} catch {
    Write-Host "⚠️  Prometheus en cours de démarrage..." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "=========================================" -ForegroundColor Green
Write-Host "📊 URLs disponibles :" -ForegroundColor Cyan
Write-Host "  🌐 Prometheus UI : http://localhost:9090" -ForegroundColor White
Write-Host "  📈 GoShop Metrics : http://localhost:8081/metrics" -ForegroundColor White
Write-Host ""
Write-Host "💡 Prochaines étapes :" -ForegroundColor Yellow
Write-Host "  1. Lancez votre app : go run cmd/api/main.go" -ForegroundColor White
Write-Host "  2. Ouvrez http://localhost:9090" -ForegroundColor White
Write-Host "  3. Allez dans Status → Targets" -ForegroundColor White
Write-Host "  4. Vérifiez que 'goshop-api' est UP" -ForegroundColor White
Write-Host ""
Write-Host "🔍 Requêtes PromQL à tester :" -ForegroundColor Yellow
Write-Host "  sum(rate(goshop_http_requests_total[5m])) by (path)" -ForegroundColor Gray
Write-Host "  histogram_quantile(0.95, sum(rate(goshop_http_request_duration_seconds_bucket[5m])) by (le))" -ForegroundColor Gray
Write-Host ""
Write-Host "🛑 Pour arrêter Prometheus : Fermez la fenêtre ou Ctrl+C ici" -ForegroundColor Red
Write-Host ""

# Attendre l'arrêt
try {
    Wait-Process -Id $PrometheusProcess.Id
} catch {
    Write-Host "Prometheus arrêté." -ForegroundColor Yellow
}