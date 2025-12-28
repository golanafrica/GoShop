# Makefile pour Windows - Tests de charge GoShop
.PHONY: help prepare smoke load stress report clean generate-data check-k6 check-data check-server create-dirs products-admin products-smoke products-load all-tests

# Configuration
SHELL = cmd.exe

help:
	@echo Tests de Charge GoShop avec k6
	@echo.
	@echo Commandes disponibles:
	@echo   make prepare         - Preparer l'environnement
	@echo   make smoke           - Smoke test auth (5 VUs, 30s)
	@echo   make load            - Load test auth (50 VUs, 2m)
	@echo   make stress          - Stress test (100 VUs, 3m)
	@echo   make products-admin  - Test CRUD admin produits (50 VUs, 2m)
	@echo   make products-smoke  - Smoke test produits (10 VUs, 1m)
	@echo   make all-tests       - Executer tous les tests (sauf admin)
	@echo   make report          - Ouvrir le dernier rapport HTML
	@echo   make clean           - Nettoyer les resultats
	@echo   make generate-data   - Generer les donnees de test
	@echo.
	@echo Configuration requise:
	@echo   - ADMIN_TOKEN pour les tests produits
	@echo   - Exemple: set ADMIN_TOKEN=votre_token_jwt

# === Preparation ===

prepare: check-k6 check-server create-dirs check-data

create-dirs:
	@if not exist results mkdir results
	@if not exist data mkdir data
	@if not exist scripts mkdir scripts
	@if not exist config mkdir config

check-k6:
	@k6 version >nul 2>&1 || (echo [ERROR] k6 non installe. && echo [INFO] Installez avec: winget install k6 && exit /b 1)
	@echo [OK] k6 installe

check-server:
	@echo [INFO] Verification du serveur GoShop...
	@powershell -Command "try { (Invoke-WebRequest -Uri 'http://localhost:8080/health/live' -UseBasicParsing -TimeoutSec 5).StatusCode -eq 200 } catch { exit 1 }" >nul 2>&1
	@if errorlevel 1 (echo [WARN] Serveur non accessible. Demarrez: go run cmd\api\main.go && exit /b 1)
	@echo [OK] Serveur GoShop OK

check-data:
	@if not exist data\users.json (echo [ERROR] users.json manquant. && echo [INFO] Executez: make generate-data && exit /b 1)
	@if not exist data\products.json (echo [ERROR] products.json manquant. && echo [INFO] Executez: make generate-data && exit /b 1)
	@echo [OK] Donnees de test OK

# === Tests d'authentification ===

smoke: prepare
	@echo [TEST] Execution du smoke test (auth)...
	@k6 run --vus 5 --duration 30s --env BASE_URL=http://localhost:8080 scripts/auth_smoke.js

load: prepare
	@echo [TEST] Execution du test de charge (auth)...
	@k6 run --vus 50 --duration 2m --env BASE_URL=http://localhost:8080 scripts/auth_load.js

stress: prepare
	@echo [TEST] Execution du test de stress (auth)...
	@k6 run --vus 100 --duration 3m --env BASE_URL=http://localhost:8080 scripts/stress_test.js

# === Tests produits ===

products-admin: prepare
	@echo [TEST] Test CRUD admin produits...
	@if not defined ADMIN_TOKEN ( \
		echo [ERROR] ADMIN_TOKEN non defini. && \
		echo [INFO] Comment definir le token : && \
		echo   Sous CMD : set ADMIN_TOKEN=votre_token_jwt && \
		echo   Sous PowerShell : $$env:ADMIN_TOKEN="votre_token_jwt" && \
		exit /b 1 \
	)
	@echo [INFO] Configuration :
	@echo   Token: %ADMIN_TOKEN:~0,50%...
	@echo   VUs: 50
	@echo   Duree: 2 minutes
	@echo.
	@k6 run --vus 50 --duration 2m --env BASE_URL=http://localhost:8080 --env ADMIN_TOKEN=%ADMIN_TOKEN% scripts/products_load.js

products-smoke: prepare
	@echo [TEST] Smoke test produits (lecture seule)...
	@if not defined ADMIN_TOKEN ( \
		echo [WARN] ADMIN_TOKEN non defini - test limite && \
		echo [INFO] Pour un test complet: set ADMIN_TOKEN=votre_token && \
		k6 run --vus 10 --duration 1m --env BASE_URL=http://localhost:8080 scripts/products_smoke.js \
	) else ( \
		echo [OK] Utilisation du token fourni... && \
		k6 run --vus 10 --duration 1m --env BASE_URL=http://localhost:8080 --env ADMIN_TOKEN=%ADMIN_TOKEN% scripts/products_smoke.js \
	)

# === Suite de tests complete ===

all-tests: smoke load stress
	@echo [OK] Tous les tests de base termines avec succes !
	@echo [INFO] Resume des tests :
	@echo   - Smoke auth: 5 VUs, 30s - OK
	@echo   - Load auth: 50 VUs, 2m - OK
	@echo   - Stress auth: 100 VUs, 3m - OK
	@echo.
	@echo [INFO] Pour tester les produits (necessite token) :
	@echo   set ADMIN_TOKEN=votre_token
	@echo   make products-smoke
	@echo   make products-admin

# === Utilitaires ===

generate-data: create-dirs
	@echo [INFO] Generation des donnees de test...
	@cd scripts && go run main.go -generate

report:
	@echo [INFO] Recherche du dernier rapport HTML...
	@for /f "delims=" %%f in ('dir /b /o-d results\*.html 2^>nul') do set "LATEST=%%f" && goto found
	@echo [ERROR] Aucun rapport HTML trouve.
	@echo [INFO] Executez d'abord: make smoke
	@exit /b 1
:found
	@echo [INFO] Ouverture du rapport: %LATEST%
	@start "" "results\%LATEST%"

clean:
	@echo [INFO] Nettoyage des resultats...
	@if exist results rmdir /s /q results
	@mkdir results
	@echo [OK] Dossier results reinitialise