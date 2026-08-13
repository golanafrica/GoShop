# ============================================================
# SCRIPT DE VALIDATION DES VARIABLES D'ENVIRONNEMENT
# ============================================================
# Usage : .\scripts\validate-env.ps1
# Vérifie que toutes les variables 🔴 OBLIGATOIRES sont définies
# ============================================================

$envFile = ".env"

if (-not (Test-Path $envFile)) {
    Write-Error "❌ Fichier .env introuvable. Copiez .env.example vers .env"
    Write-Host "  cp .env.example .env" -ForegroundColor Yellow
    exit 1
}

Write-Host "🔍 Validation de $envFile..." -ForegroundColor Cyan

# Charger le fichier .env
$envVars = @{}
Get-Content $envFile | ForEach-Object {
    if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
        $envVars[$matches[1].Trim()] = $matches[2].Trim()
    }
}

# Variables obligatoires
$required = @(
    'APP_ENV',
    'JWT_SECRET',
    'REFRESH_SECRET',
    'ENCRYPTION_KEY',
    'DB_HOST',
    'DB_PORT',
    'DB_USER',
    'DB_PASSWORD',
    'DB_NAME',
    'REDIS_HOST',
    'REDIS_PORT'
)

# Patterns invalides (placeholders)
$invalidPatterns = @('CHANGE_ME_', 'changeme', 'password123', 'root')

$errors = @()

# Vérifier les variables obligatoires
foreach ($var in $required) {
    if (-not $envVars.ContainsKey($var) -or [string]::IsNullOrWhiteSpace($envVars[$var])) {
        $errors += "❌ Variable obligatoire manquante : $var"
    }
}

# Vérifier les placeholders invalides
foreach ($key in $envVars.Keys) {
    $value = $envVars[$key]
    foreach ($pattern in $invalidPatterns) {
        if ($value -match $pattern) {
            $errors += "⚠️  $key contient un placeholder invalide : '$value'"
            break
        }
    }
}

# Vérifications spécifiques
if ($envVars['JWT_SECRET'] -and $envVars['JWT_SECRET'].Length -lt 32) {
    $errors += "❌ JWT_SECRET doit faire au moins 32 caractères (actuel : $($envVars['JWT_SECRET'].Length))"
}

if ($envVars['ENCRYPTION_KEY'] -and $envVars['ENCRYPTION_KEY'].Length -ne 32) {
    $errors += "❌ ENCRYPTION_KEY doit faire exactement 32 caractères (actuel : $($envVars['ENCRYPTION_KEY'].Length))"
}

if ($envVars['APP_ENV'] -eq 'production' -and $envVars['FRONTEND_URL'] -match '\*') {
    $errors += "❌ FRONTEND_URL ne peut pas contenir * en production"
}

# Afficher le résultat
if ($errors.Count -eq 0) {
    Write-Host "✅ Toutes les validations ont réussi !" -ForegroundColor Green
    Write-Host "   Variables obligatoires : OK"
    Write-Host "   Placeholders invalides : Aucun"
    Write-Host "   Longueur JWT_SECRET : OK"
    Write-Host "   Longueur ENCRYPTION_KEY : OK"
    exit 0
} else {
    Write-Host "`n❌ Erreurs détectées :" -ForegroundColor Red
    $errors | ForEach-Object { Write-Host "   $_" -ForegroundColor Red }
    Write-Host "`n📝 Corriger les erreurs et relancer la validation" -ForegroundColor Yellow
    exit 1
}