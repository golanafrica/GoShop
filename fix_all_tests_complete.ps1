# fix_all_tests_complete.ps1
Write-Host "CORRECTION COMPLETE DE TOUS LES TESTS" -ForegroundColor Green
Write-Host "======================================" -ForegroundColor Green

# 1. D'abord, corrigez les tests product_uscase
Write-Host "`nPHASE 1: Tests product_uscase" -ForegroundColor Cyan

$productUsecaseTests = @(
    "application/usecase/product_uscase/create_product_usecase_test.go",
    "application/usecase/product_uscase/delete_product_usecase_test.go",
    "application/usecase/product_uscase/get_by_id_usecase_test.go",
    "application/usecase/product_uscase/update_product_usecase_test.go"
)

foreach ($file in $productUsecaseTests) {
    if (Test-Path $file) {
        Write-Host "`n  $($file.Split('/')[-1])" -ForegroundColor Yellow
        
        # Ajouter l'import setupLogging si necessaire
        $content = Get-Content $file -Raw
        
        if ($content -notmatch '"Goshop/config/setupLogging"') {
            $content = $content -replace '(import \(\r\n)', "`$1`t`"Goshop/config/setupLogging`"`r`n"
            Write-Host "    [OK] Ajoute import setupLogging" -ForegroundColor Green
        }
        
        # Remplacer tous les NewXUsecase par NewXUsecaseOld
        $content = $content -replace 'NewCreateProductUsecase\(', 'NewCreateProductUsecaseOld('
        $content = $content -replace 'NewDeleteProductUsecase\(', 'NewDeleteProductUsecaseOld('
        $content = $content -replace 'NewUpdateProductUsecase\(', 'NewUpdateProductUsecaseOld('
        $content = $content -replace 'NewGetProductByIdUsecase\(', 'NewGetProductByIdUsecaseOld('
        $content = $content -replace 'NewListProductUsecase\(', 'NewListProductUsecaseOld('
        $content = $content -replace 'NewGetAllProductByIdUsecase\(', 'NewGetProductByIdUsecaseOld('
        
        Set-Content $file $content -Encoding UTF8
        Write-Host "    [OK] Fichier corrige" -ForegroundColor Green
    }
}

# 2. Ensuite, corrigez les handlers
Write-Host "`nPHASE 2: Tests handlers" -ForegroundColor Cyan

$handlerTests = @(
    "interfaces/handler/customer_handler/customer_handler_test.go",
    "interfaces/handler/orders/order_handler_test.go",
    "interfaces/handler/product/product_handler_test.go",
    "interfaces/handler/refresh_handler/refresh_handler_test.go",
    "interfaces/handler/user_handler/user_handler_test.go"
)

foreach ($file in $handlerTests) {
    if (Test-Path $file) {
        Write-Host "`n  $($file.Split('/')[-1])" -ForegroundColor Yellow
        
        $content = Get-Content $file -Raw
        
        # Ajouter l'import setupLogging
        if ($content -notmatch '"Goshop/config/setupLogging"') {
            $content = $content -replace '(import \(\r\n)', "`$1`t`"Goshop/config/setupLogging`"`r`n"
            Write-Host "    [OK] Ajoute import setupLogging" -ForegroundColor Green
        }
        
        # Remplacer NewXHandler par NewXHandlerOld
        $content = $content -replace 'customerhandler\.NewCustomerHandler\(', 'customerhandler.NewCustomerHandlerOld('
        $content = $content -replace 'producthandler\.NewProductHandler\(', 'producthandler.NewProductHandlerOld('
        $content = $content -replace 'productHandler\.NewProductHandler\(', 'productHandler.NewProductHandlerOld('
        $content = $content -replace 'orderhandler\.NewOrderHandler\(', 'orderhandler.NewOrderHandlerOld('
        $content = $content -replace 'refreshhandler\.NewRefreshHandler\(', 'refreshhandler.NewRefreshHandlerOld('
        $content = $content -replace 'userhandler\.NewUserHandler\(', 'userhandler.NewUserHandlerOld('
        
        Set-Content $file $content -Encoding UTF8
        Write-Host "    [OK] Handler corrige" -ForegroundColor Green
    }
}

# 3. Enfin, corrigez les tests e2e
Write-Host "`nPHASE 3: Tests E2E" -ForegroundColor Cyan

$e2eTests = @(
    "tests/e2e/create_order_e2e_test.go",
    "tests/e2e/delete_customers_e2e_test.go",
    "tests/e2e/delete_product_e2e_test.go"
)

foreach ($file in $e2eTests) {
    if (Test-Path $file) {
        Write-Host "`n  $($file.Split('/')[-1])" -ForegroundColor Yellow
        
        $content = Get-Content $file -Raw
        
        # Ajouter l'import setupLogging
        if ($content -notmatch '"Goshop/config/setupLogging"') {
            $content = $content -replace '(import \(\r\n)', "`$1`t`"Goshop/config/setupLogging`"`r`n"
            Write-Host "    [OK] Ajoute import setupLogging" -ForegroundColor Green
        }
        
        # Remplacer les appels sans logger
        $content = $content -replace 'NewProductHandler\(([^,]+), ([^)]+)\)', 'NewProductHandler($1, $2, setupLogging.GetTestLogger())'
        $content = $content -replace 'NewCustomerHandler\(([^,]+), ([^)]+)\)', 'NewCustomerHandler($1, $2, setupLogging.GetTestLogger())'
        $content = $content -replace 'NewOrderHandler\(([^)]+)\)', 'NewOrderHandler($1, setupLogging.GetTestLogger())'
        
        Set-Content $file $content -Encoding UTF8
        Write-Host "    [OK] E2E corrige" -ForegroundColor Green
    }
}

# 4. Tests order_usecase
Write-Host "`nPHASE 4: Tests order_usecase" -ForegroundColor Cyan

$orderTests = @(
    "application/usecase/order_usecase/create_order_usecase_test.go",
    "application/usecase/order_usecase/create_order_usecase_integration_test.go",
    "application/usecase/order_usecase/get_orderUsecase_test.go",
    "application/usecase/order_usecase/get_order_by_id_integration_test.go"
)

foreach ($file in $orderTests) {
    if (Test-Path $file) {
        Write-Host "`n  $($file.Split('/')[-1])" -ForegroundColor Yellow
        
        $content = Get-Content $file -Raw
        
        # Ajouter l'import setupLogging
        if ($content -notmatch '"Goshop/config/setupLogging"') {
            $content = $content -replace '(import \(\r\n)', "`$1`t`"Goshop/config/setupLogging`"`r`n"
            Write-Host "    [OK] Ajoute import setupLogging" -ForegroundColor Green
        }
        
        # Remplacer NewXUsecase par NewXUsecase(..., logger)
        $content = $content -replace 'NewCreateOrderUsecase\(', 'NewCreateOrderUsecase($1, setupLogging.GetTestLogger())'
        $content = $content -replace 'NewGetAllOrderUsecase\(', 'NewGetAllOrderUsecase($1, setupLogging.GetTestLogger())'
        $content = $content -replace 'NewGetOrderByIdUsecase\(', 'NewGetOrderByIdUsecase($1, setupLogging.GetTestLogger())'
        
        Set-Content $file $content -Encoding UTF8
        Write-Host "    [OK] Order tests corriges" -ForegroundColor Green
    }
}

# 5. Tests customer_usecase
Write-Host "`nPHASE 5: Tests customer_usecase" -ForegroundColor Cyan

$customerTest = "application/usecase/customer_usecase/customer_usecase_test.go"
if (Test-Path $customerTest) {
    Write-Host "`n  customer_usecase_test.go" -ForegroundColor Yellow
    
    $content = Get-Content $customerTest -Raw
    
    if ($content -notmatch '"Goshop/config/setupLogging"') {
        $content = $content -replace '(import \(\r\n)', "`$1`t`"Goshop/config/setupLogging`"`r`n"
        Write-Host "    [OK] Ajoute import setupLogging" -ForegroundColor Green
    }
    
    $content = $content -replace 'NewCreateCustomerUsecase\(', 'NewCreateCustomerUsecase($1, setupLogging.GetTestLogger())'
    $content = $content -replace 'NewGetCustomerByIdUsecase\(', 'NewGetCustomerByIdUsecase($1, setupLogging.GetTestLogger())'
    $content = $content -replace 'NewGetAllCustomersUsecase\(', 'NewGetAllCustomersUsecase($1, setupLogging.GetTestLogger())'
    $content = $content -replace 'NewUpdateCustomerUsercase\(', 'NewUpdateCustomerUsecase($1, setupLogging.GetTestLogger())'
    $content = $content -replace 'NewDeleteCustomerUsecase\(', 'NewDeleteCustomerUsecase($1, setupLogging.GetTestLogger())'
    
    Set-Content $customerTest $content -Encoding UTF8
    Write-Host "    [OK] Customer tests corriges" -ForegroundColor Green
}

# 6. Tests user_usecase
Write-Host "`nPHASE 6: Tests user_usecase" -ForegroundColor Cyan

$userTests = @(
    "application/usecase/user_usecase/loginUsecase_test.go",
    "application/usecase/user_usecase/registerUsecase_test.go"
)

foreach ($file in $userTests) {
    if (Test-Path $file) {
        Write-Host "`n  $($file.Split('/')[-1])" -ForegroundColor Yellow
        
        $content = Get-Content $file -Raw
        
        if ($content -notmatch '"Goshop/config/setupLogging"') {
            $content = $content -replace '(import \(\r\n)', "`$1`t`"Goshop/config/setupLogging`"`r`n"
            Write-Host "    [OK] Ajoute import setupLogging" -ForegroundColor Green
        }
        
        $content = $content -replace 'NewLoginUsecase\(', 'NewLoginUsecase($1, setupLogging.GetTestLogger())'
        $content = $content -replace 'NewRegisterUsecase\(', 'NewRegisterUsecase($1, setupLogging.GetTestLogger())'
        
        Set-Content $file $content -Encoding UTF8
        Write-Host "    [OK] User tests corriges" -ForegroundColor Green
    }
}

Write-Host "`nCORRECTION TERMINEE !" -ForegroundColor Green
Write-Host "======================" -ForegroundColor Green
Write-Host "`nProchaines etapes :" -ForegroundColor Yellow
Write-Host "1. Verifiez la compilation: go build ./..." -ForegroundColor White
Write-Host "2. Testez product_uscase: go test ./application/usecase/product_uscase -v" -ForegroundColor White
Write-Host "3. Testez product_handler: go test ./interfaces/handler/product -v" -ForegroundColor White