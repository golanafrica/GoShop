
---

```markdown
# 🧪 Guide des Tests (GoShop v4.5.0)

**Version** : v4.5.0  
**Dernière mise à jour** : 2026-07-21  
**Outils** : Go testing, testify, gomock, k6, PowerShell

---

## 📋 Table des matières
1. [Vue d'ensemble](#1-vue-densemble)
2. [Types de tests](#2-types-de-tests)
3. [Structure des tests](#3-structure-des-tests)
4. [Exécution des tests](#4-exécution-des-tests)
5. [Tests de sécurité spécifiques](#5-tests-de-sécurité-spécifiques)
6. [Tests WebSocket (v4.5.0)](#6-tests-websocket-v450)
7. [Bonnes pratiques](#7-bonnes-pratiques)
8. [CI/CD et automatisation](#8-cicd-et-automatisation)

---

## 1. Vue d'ensemble

GoShop suit une **stratégie de tests en pyramide** pour garantir une couverture complète et une exécution rapide :

```
        ┌─────────────┐
        │  Tests E2E  │  ← Tests complets (API + DB + Redis)
        │   (Lents)   │
        ├─────────────┤
        │ Intégration │  ← Repositories avec vraie DB
        │  (Moyens)   │
        ├─────────────┤
        │  Unitaires  │  ← Use cases & entités (mocks)
        │  (Rapides)  │
        └─────────────┘
```

**Objectifs** :
- ✅ Couverture > 80% sur le code métier critique (usecases, repositories).
- ✅ Zéro régression sur les fonctionnalités existantes.
- ✅ Validation stricte de l'isolation multi-tenant et de la sécurité.

---

## 2. Types de tests

### 2.1. Tests Unitaires
**Objectif** : Tester la logique métier pure (use cases, entités, value objects) en isolant les dépendances.

**Caractéristiques** :
- Utilisent des **mocks** générés avec `mockgen` (package `go.uber.org/mock`).
- Exécution rapide (< 1s par test).
- Pas de dépendance externe (DB, Redis, API).

**Exemple** :
```go
// application/usecase/customer_usecase/customer_usecase_test.go
func TestCreateCustomerUsecase_Success(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
    mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
    mockTxManager := mockrepo.NewMockTxManager(ctrl)

    // Setup des expectations
    mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
    mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
    mockRepoTx.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(nil, sql.ErrNoRows)
    mockRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(createdCustomer, nil)
    mockTx.EXPECT().Commit().Return(nil)

    uc := customerusecase.NewCreateCustomerUsecase(mockRepo, mockUserRepo, mockTxManager)
    result, err := uc.Execute(context.Background(), customer)

    assert.NoError(t, err)
    assert.Equal(t, "cust-123", result.ID)
}
```

**Exécution** :
```bash
go test ./application/... -v
go test ./domain/... -v
```

---

### 2.2. Tests d'Intégration
**Objectif** : Valider l'interaction entre les repositories et la vraie base de données PostgreSQL.

**Caractéristiques** :
- Utilisent une **vraie DB PostgreSQL** (locale ou Docker).
- Testent les requêtes SQL, les transactions, les contraintes.
- Plus lents que les tests unitaires (1-5s par test).

**Exemple** :
```go
// tests/integration/wallet_test.go
func TestWalletCreditDebit(t *testing.T) {
    db := setupTestDB(t) // Crée une DB de test isolée
    defer db.Close()

    repo := walletinfra.NewMerchantWalletRepositoryInfrastructure(db)
    
    // Crédit initial
    wallet, err := repo.Credit(ctx, shopID, 100000)
    assert.NoError(t, err)
    assert.Equal(t, int64(100000), wallet.BalanceCents)

    // Débit
    wallet, err = repo.Debit(ctx, shopID, 30000)
    assert.NoError(t, err)
    assert.Equal(t, int64(70000), wallet.BalanceCents)
}
```

**Exécution** :
```bash
# Nécessite PostgreSQL local sur localhost:5432
go test -tags=integration ./tests/integration/... -v
```

---

### 2.3. Tests End-to-End (E2E)
**Objectif** : Simuler des scénarios utilisateur complets, de l'inscription jusqu'à la livraison.

**Caractéristiques** :
- Testent l'API HTTP complète (handlers, middlewares, usecases, repositories).
- Utilisent une stack complète : API + DB + Redis.
- Valident l'isolation multi-tenant, les webhooks, les workflows métier.

**Scénarios couverts** :
- ✅ `TestAuthFlowE2E` : Inscription → Connexion → Profil → Logout
- ✅ `TestCreateOrderE2E` : Création shop → Customer → Products → Order (multi-tenant)
- ✅ `TestPaymentFlowE2E` : Initiation paiement → Webhook → Statut success
- ✅ `TestTontineWorkflowE2E` : Création groupe → Rejointure → Paiements → Voucher
- ✅ `TestCreditFlowE2E` : Demande crédit → Approbation → Paiement échéances
- ✅ `TestWithdrawalFlowE2E` : Demande retrait → Validation KYC → Traitement
- ✅ `TestJWTSecurityE2E` : Validation tokens, expiration, révocation

**Exemple** :
```go
// tests/e2e/create_order_e2e_test.go
func TestCreateOrderE2E(t *testing.T) {
    // 1. Créer une boutique
    shop := createTestShop(t, "test-shop")
    
    // 2. Créer un client
    customer := createTestCustomer(t, shop.ID, "customer@example.com")
    
    // 3. Créer un produit
    product := createTestProduct(t, shop.ID, 50000, 100)
    
    // 4. Créer une commande
    order := createTestOrder(t, shop.ID, customer.ID, []OrderItem{
        {ProductID: product.ID, Quantity: 2},
    })
    
    // 5. Vérifier isolation multi-tenant
    otherShop := createTestShop(t, "other-shop")
    _, err := getOrderFromOtherShop(t, otherShop.ID, order.ID)
    assert.Error(t, err, "Order should not be accessible from other shop")
}
```

**Exécution** :
```bash
# Nécessite API + DB + Redis running
go test -tags=e2e ./tests/e2e/... -v
```

---

### 2.4. Tests de Charge (Load Testing)
**Objectif** : Valider les performances et la scalabilité sous charge.

**Outil** : [k6](https://k6.io/) (Grafana k6)

**Scénarios disponibles** :
- `auth_load.js` : 100 utilisateurs simultanés (login/register)
- `products_load.js` : Lecture/écriture de produits
- `products_stress.js` : Stress test (1000 VUs)
- `stress_test.js` : Test de rupture

**Exécution** :
```bash
# Installer k6
# brew install k6 (macOS)
# choco install k6 (Windows)

# Lancer un scénario
k6 run tests/loadtest/scripts/auth_load.js

# Avec rapport HTML
k6 run --out html=report.html tests/loadtest/scripts/products_load.js
```

**Métriques surveillées** :
- Latence HTTP (p50, p95, p99)
- Taux d'erreur (< 1%)
- Requêtes par seconde (RPS)
- Utilisation CPU/Mémoire

---

## 3. Structure des tests

```
tests/
├── unit/                    # Tests unitaires (mocks)
│   ├── customer_usecase_test.go
│   ├── order_usecase_test.go
│   └── wallet_usecase_test.go
│
├── integration/             # Tests d'intégration (DB réelle)
│   ├── main_test.go         # Setup/Teardown DB
│   ├── wallet_test.go
│   ├── credit_test.go
│   └── cod_test.go
│
├── e2e/                     # Tests End-to-End (API complète)
│   ├── auth_flow_e2e_test.go
│   ├── create_order_e2e_test.go
│   ├── payment_flow_e2e_test.go
│   ├── tontine_flow_e2e_test.go
│   ├── credit_flow_e2e_test.go
│   ├── withdrawal_flow_e2e_test.go
│   └── jwt_security_e2e_test.go
│
├── loadtest/                # Tests de charge (k6)
│   ├── scripts/
│   │   ├── auth_load.js
│   │   ├── products_load.js
│   │   └── stress_test.js
│   └── results/             # Rapports HTML/JSON
│
└── testutils/               # Utilitaires partagés
    ├── fixtures.go          # Données de test
    ├── helpers.go           # Fonctions d'assistance
    ├── test_server.go       # Setup serveur de test
    └── migrate.go           # Application migrations
```

---

## 4. Exécution des tests

### 4.1. Tous les tests (CI/CD)
```bash
# Tests unitaires + intégration + E2E
go test ./... -v

# Avec couverture de code
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### 4.2. Tests par package
```bash
# Uniquement les usecases
go test ./application/usecase/... -v

# Uniquement les repositories
go test ./infrastructure/postgres/... -v

# Uniquement les handlers
go test ./interfaces/handler/... -v
```

### 4.3. Tests avec tags
```bash
# Tests d'intégration uniquement
go test -tags=integration ./tests/integration/... -v

# Tests E2E uniquement
go test -tags=e2e ./tests/e2e/... -v

# Tests de sécurité uniquement
go test -tags=security ./tests/e2e/... -run TestJWT -v
```

### 4.4. Tests spécifiques
```bash
# Un test spécifique
go test ./application/usecase/customer_usecase/... -run TestCreateCustomerUsecase_Success -v

# Avec race detector
go test -race ./... -v
```

---

## 5. Tests de sécurité spécifiques

### 5.1. Isolation Multi-tenant (IDOR)
**Objectif** : Vérifier qu'aucun utilisateur ne peut accéder aux ressources d'une autre boutique.

```go
// tests/e2e/security_test.go
func TestMultiTenantIsolation(t *testing.T) {
    shopA := createTestShop(t, "shop-a")
    shopB := createTestShop(t, "shop-b")
    
    productA := createTestProduct(t, shopA.ID, 50000, 100)
    
    // Tenter d'accéder au produit de shopA via shopB
    req := httptest.NewRequest("GET", "/api/products/"+productA.ID, nil)
    req.Header.Set("X-Shop-Slug", "shop-b")
    req.Header.Set("Authorization", "Bearer "+shopBOwnerToken)
    
    resp := executeRequest(t, req)
    assert.Equal(t, http.StatusNotFound, resp.Code, "Product should not be accessible from other shop")
}
```

### 5.2. Middleware RequireShopAccess (v4.5.0)
**Objectif** : Valider que le middleware bloque les accès non autorisés.

```go
func TestRequireShopAccessMiddleware(t *testing.T) {
    shop := createTestShop(t, "test-shop")
    otherUser := createTestUser(t, "other@example.com")
    
    // Tenter d'accéder à la boutique sans être owner/collaborator
    req := httptest.NewRequest("GET", "/api/products", nil)
    req.Header.Set("X-Shop-Slug", "test-shop")
    req.Header.Set("Authorization", "Bearer "+otherUserToken)
    
    resp := executeRequest(t, req)
    assert.Equal(t, http.StatusForbidden, resp.Code)
    assert.Contains(t, resp.Body.String(), "Accès refusé")
}
```

### 5.3. Injection SQL
**Objectif** : Vérifier que les paramètres `sort_by` sont validés.

```go
func TestSortByInjectionPrevention(t *testing.T) {
    maliciousSortBy := "id; DROP TABLE shops;--"
    
    req := httptest.NewRequest("GET", "/api/admin/shops?sort_by="+maliciousSortBy, nil)
    req.Header.Set("Authorization", "Bearer "+adminToken)
    
    resp := executeRequest(t, req)
    assert.Equal(t, http.StatusOK, resp.Code) // Doit utiliser la valeur par défaut "created_at"
    
    // Vérifier que la table shops existe toujours
    var count int
    db.QueryRow("SELECT COUNT(*) FROM shops").Scan(&count)
    assert.Greater(t, count, 0, "Table should not be dropped")
}
```

### 5.4. JWT Security
**Objectif** : Valider la gestion des tokens (expiration, révocation, secret).

```go
func TestJWTValidation(t *testing.T) {
    // Token expiré
    expiredToken := generateExpiredToken(t)
    req := httptest.NewRequest("GET", "/api/auth/me", nil)
    req.Header.Set("Authorization", "Bearer "+expiredToken)
    
    resp := executeRequest(t, req)
    assert.Equal(t, http.StatusUnauthorized, resp.Code)
    
    // Token avec secret invalide
    forgedToken := forgeTokenWithWrongSecret(t)
    req.Header.Set("Authorization", "Bearer "+forgedToken)
    
    resp = executeRequest(t, req)
    assert.Equal(t, http.StatusUnauthorized, resp.Code)
}
```

---

## 6. Tests WebSocket (v4.5.0)

### 6.1. Test manuel avec PowerShell
Le script `Test-WebSocket-Notification.ps1` automatise le test complet du flux WebSocket :

```powershell
# Lancer le script
.\Test-WebSocket-Notification.ps1
```

**Ce que fait le script** :
1. ✅ Connecte le marchand et récupère son token.
2. ✅ Récupère le slug de la boutique.
3. ✅ Crée un utilisateur client et le lie à un Customer.
4. ✅ Ouvre une connexion WebSocket avec le token client.
5. ✅ Crée un produit et une commande.
6. ✅ Accepte la commande (déclenche la notification).
7. ✅ Vérifie que le payload JSON arrive bien dans Postman.

### 6.2. Test automatisé WebSocket
```go
// tests/e2e/websocket_test.go
func TestWebSocketNotification(t *testing.T) {
    // 1. Créer un client et se connecter
    client := createTestClient(t)
    wsConn := connectWebSocket(t, client.Token)
    defer wsConn.Close()
    
    // 2. Créer une commande et l'accepter
    order := createTestOrder(t, shopID, client.CustomerID, items)
    acceptOrder(t, order.ID)
    
    // 3. Recevoir la notification WebSocket
    _, message, err := wsConn.ReadMessage()
    assert.NoError(t, err)
    
    var notification WebSocketNotification
    json.Unmarshal(message, &notification)
    
    assert.Equal(t, "client_order_confirmed", notification.Type)
    assert.Equal(t, order.ID, notification.Data["order_id"])
}
```

---

## 7. Bonnes pratiques

### 7.1. Nommage des tests
```go
// ✅ Bon : Descriptif et structuré
func TestCreateCustomerUsecase_Success(t *testing.T)
func TestCreateCustomerUsecase_ValidationError(t *testing.T)
func TestCreateCustomerUsecase_BeginTxError(t *testing.T)

// ❌ Mauvais : Trop vague
func TestCreateCustomer(t *testing.T)
func TestCustomer1(t *testing.T)
```

### 7.2. Isolation des tests
```go
// ✅ Bon : Chaque test est indépendant
func TestOrderCreation(t *testing.T) {
    db := setupTestDB(t) // DB isolée
    defer db.Close()
    
    shop := createTestShop(t, "test-shop")
    // ...
}

// ❌ Mauvais : Dépend de l'état global
var globalShop *entity.Shop

func TestOrderCreation(t *testing.T) {
    // Utilise globalShop créé dans un autre test
}
```

### 7.3. Mocks avec gomock
```go
// Générer les mocks
//go:generate mockgen -destination=../../mocks/repository/mock_customer_repository.go -package=mockrepo Goshop/domain/repository CustomerRepositoryInterface

// Utiliser les mocks
mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
mockRepo.EXPECT().FindByID(gomock.Any(), "cust-123").Return(customer, nil)
```

### 7.4. Assertions avec testify
```go
// ✅ Bon : Assertions claires
assert.NoError(t, err)
assert.Equal(t, expected, actual)
assert.Contains(t, message, "error")
assert.Len(t, results, 5)

// ❌ Mauvais : If/else manuels
if err != nil {
    t.Fatal(err)
}
if actual != expected {
    t.Errorf("Expected %v, got %v", expected, actual)
}
```

### 7.5. Données de test (Fixtures)
```go
// tests/testutils/fixtures.go
func CreateTestShop(t *testing.T, slug string) *entity.Shop {
    shop := &entity.Shop{
        ID:      uuid.NewString(),
        Name:    "Test Shop",
        Slug:    slug,
        OwnerID: "user-123",
    }
    // Insert en DB
    return shop
}
```

---

## 8. CI/CD et automatisation

### 8.1. GitHub Actions
```yaml
# .github/workflows/ci-cd.yml
name: CI/CD

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_PASSWORD: password
          POSTGRES_DB: goshop_test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      
      redis:
        image: redis:7
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      
      - name: Run migrations
        run: |
          migrate -path migrations -database "postgres://postgres:password@localhost:5432/goshop_test?sslmode=disable" up
      
      - name: Run tests
        run: |
          go test ./... -v -coverprofile=coverage.out
          go test -tags=integration ./tests/integration/... -v
          go test -tags=e2e ./tests/e2e/... -v
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
```

### 8.2. Pre-commit hooks
```bash
# .git/hooks/pre-commit
#!/bin/sh
echo "Running gofmt..."
gofmt -s -w .

echo "Running go vet..."
go vet ./...

echo "Running tests..."
go test ./... -v
```

---

## 📚 Références
- [Documentation Go testing](https://pkg.go.dev/testing)
- [testify](https://github.com/stretchr/testify)
- [gomock](https://github.com/golang/mock)
- [k6 Documentation](https://k6.io/docs/)
- [GitHub Actions](https://docs.github.com/en/actions)

---

**Dernière mise à jour** : 2026-07-21
```

