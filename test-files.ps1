# ============================================================
# TEST COMPLET - File Presigned URLs
# ============================================================

Write-Host "`n🚀 Démarrage des tests File Presigned URLs`n" -ForegroundColor Cyan

# 1. LOGIN
Write-Host "📝 Étape 1: Login..." -ForegroundColor Yellow
$loginBody = @{ email = "test@test.com"; password = "password123" } | ConvertTo-Json
$loginResp = curl.exe -s -X POST http://localhost:8081/login `
  -H "Content-Type: application/json" `
  -d ($loginBody -replace '"', '\"')

$token = ($loginResp | ConvertFrom-Json).access_token

if (-not $token) {
    Write-Host "❌ Échec login. Réponse: $loginResp" -ForegroundColor Red
    Write-Host "💡 Créez d'abord un utilisateur via /register" -ForegroundColor Yellow
    exit 1
}

Write-Host "✅ Token obtenu: $($token.Substring(0, 20))..." -ForegroundColor Green

# 2. UPLOAD (si vous avez un fichier test.jpg)
Write-Host "`n📤 Étape 2: Upload fichier..." -ForegroundColor Yellow
if (Test-Path "test.jpg") {
    $uploadResp = curl.exe -s -X POST http://localhost:8081/api/upload/kyc `
      -H "Authorization: Bearer $token" `
      -F "file=@test.jpg" `
      -F "type=identity_card"
    Write-Host "Upload: $uploadResp" -ForegroundColor Gray
} else {
    Write-Host "⚠️  test.jpg non trouvé, on saute l'upload" -ForegroundColor Yellow
}

# 3. PRESIGN (URL pré-signée)
Write-Host "`n🔐 Étape 3: Générer URL pré-signée..." -ForegroundColor Yellow
$presignBody = '{\"file_path\": \"2026/08/15/test.jpg\"}'
$presignResp = curl.exe -s -X POST http://localhost:8081/api/files/presign `
  -H "Authorization: Bearer $token" `
  -H "Content-Type: application/json" `
  -d $presignBody

Write-Host "Réponse presign: $presignResp" -ForegroundColor Gray

$presignData = $presignResp | ConvertFrom-Json
if ($presignData.url) {
    Write-Host "✅ URL pré-signée: $($presignData.url)" -ForegroundColor Green
    Write-Host "⏰ Expire à: $($presignData.expires_at)" -ForegroundColor Gray
}

# 4. ANTI-PATH TRAVERSAL
Write-Host "`n🛡️ Étape 4: Test anti-path traversal..." -ForegroundColor Yellow
$traversalBody = '{\"file_path\": \"../../../etc/passwd\"}'
$traversalResp = curl.exe -s -X POST http://localhost:8081/api/files/presign `
  -H "Authorization: Bearer $token" `
  -H "Content-Type: application/json" `
  -d $traversalBody

Write-Host "Réponse traversal: $traversalResp" -ForegroundColor Gray

if ($traversalResp -match "INVALID_PATH") {
    Write-Host "✅ Protection anti-path traversal ACTIVE" -ForegroundColor Green
} else {
    Write-Host "⚠️  Vérifier la protection" -ForegroundColor Yellow
}

# 5. DOWNLOAD (si URL valide)
if ($presignData.url) {
    Write-Host "`n📥 Étape 5: Download fichier..." -ForegroundColor Yellow
    curl.exe -s "$($presignData.url)" --output downloaded.jpg
    if (Test-Path "downloaded.jpg") {
        $size = (Get-Item "downloaded.jpg").Length
        Write-Host "✅ Fichier téléchargé: downloaded.jpg ($size bytes)" -ForegroundColor Green
    }
}

Write-Host "`n🎉 Tests terminés!`n" -ForegroundColor Cyan