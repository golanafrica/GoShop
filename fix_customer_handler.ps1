# fix_customer_handler.ps1
Write-Host "Correction customer_handler..." -ForegroundColor Cyan

# 1. Créez le fichier compat.go
$compatContent = @'
package customerhandler

import (
	"Goshop/config/setupLogging"
	"Goshop/domain/repository"
)

func NewCustomerHandlerOld(
	repo repository.CustomerRepositoryInterface,
	txManager repository.TxManager,
) *CustomerHandler {
	return NewCustomerHandler(repo, txManager, setupLogging.GetTestLogger())
}
'@

Set-Content -Path "interfaces/handler/customer_handler/compat.go" -Value $compatContent -Encoding UTF8
Write-Host "✓ Fichier compat.go créé" -ForegroundColor Green

# 2. Corrigez le fichier de test
$testFile = "interfaces/handler/customer_handler/customer_handler_test.go"
if (Test-Path $testFile) {
    $content = Get-Content $testFile -Raw
    
    # 2a. Ajoutez l'alias d'import si nécessaire
    if ($content -notmatch 'customerhandler "Goshop/interfaces/handler/customer_handler"') {
        # Cherche l'import existant
        if ($content -match 'import \(\r\n') {
            # Ajoute l'alias
            $content = $content -replace 'import \(\r\n', "import (`r`n`tcustomerhandler `"Goshop/interfaces/handler/customer_handler`"`r`n"
            Write-Host "✓ Alias d'import ajouté" -ForegroundColor Green
        }
    }
    
    # 2b. Corrigez les appels NewCustomerHandler
    $content = $content -replace 'customerhandler\.NewCustomerHandler\(', 'customerhandler.NewCustomerHandlerOld('
    
    Set-Content $testFile $content -Encoding UTF8
    Write-Host "✓ Tests corrigés" -ForegroundColor Green
}

# 3. Vérifiez aussi les usecases customer
$customerUsecaseTest = "application/usecase/customer_usecase/customer_usecase_test.go"
if (Test-Path $customerUsecaseTest) {
    $content = Get-Content $customerUsecaseTest -Raw
    
    # Supprimez les problèmes de syntaxe
    $content = $content -replace 'setupLogging\.GetTestLogger\(\)', 'nil'
    $content = $content -replace '\(\$1, ', '('
    $content = $content -replace ', \$2\)', ')'
    
    Set-Content $customerUsecaseTest $content -Encoding UTF8
    Write-Host "✓ Tests customer_usecase nettoyés" -ForegroundColor Green
}

Write-Host "`nTerminé !" -ForegroundColor Green