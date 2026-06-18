markdown
# Guide des tests

## Types de tests

### 1. Tests unitaires
- Couvrent les use cases (`application`) et les entités (`domain`).
- Utilisent des mocks (générés avec `mockgen`).
- Exécution : `go test ./application/... -v`.

### 2. Tests d'intégration
- Testent les repositories (`infrastructure`) avec une vraie base de données.
- Utilisent `testcontainers` ou une DB dockerisée.
- Exécution : `go test -tags=integration ./infrastructure/... -v`.

### 3. Tests E2E
- Testent les endpoints HTTP (handlers).
- Utilisent une stack complète : API + DB + Redis.
- Scénarios : auth, produits, commandes, paiements.
- Exécution : `go test -tags=e2e ./tests/e2e/... -v`.

### 4. Tests de charge (Load)
- Utilisent k6.
- Simulent des utilisateurs simultanés.
- Exécution : `k6 run tests/loadtest/script.js`.

## Structure des tests
tests/
├── unit/ # Tests unitaires (mocks)
├── integration/ # Tests d'intégration
├── e2e/ # Tests end-to-end
└── loadtest/ # Tests de charge (k6)

text

## Bonnes pratiques
- **Chaque PR** doit passer tous les tests.
- **Nouvelle fonctionnalité** → nouveaux tests.
- **Mock des dépendances externes** (DB, Redis, API).
- **Utiliser `testify`** pour les assertions.