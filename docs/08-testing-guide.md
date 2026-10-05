Voici **`docs/08-testing-guide.md`** complet, prêt à coller (v5.1 + section E2E finance PowerShell).

```markdown
# 🧪 Guide des Tests (GoShop v5.1.0)

**Version** : v5.1.0  
**Dernière mise à jour** : 2026-10-03  
**Outils** : Go testing, testify, gomock, k6, PowerShell

---

## 📋 Table des matières
1. [Vue d'ensemble](#1-vue-densemble)
2. [Types de tests](#2-types-de-tests)
3. [Structure des tests](#3-structure-des-tests)
4. [Exécution des tests](#4-exécution-des-tests)
5. [Tests de sécurité spécifiques](#5-tests-de-sécurité-spécifiques)
6. [Tests WebSocket](#6-tests-websocket)
7. [E2E Finance — Debt sweep & clawback (PowerShell)](#7-e2e-finance--debt-sweep--clawback-powershell)
8. [Bonnes pratiques](#8-bonnes-pratiques)
9. [CI/CD et automatisation](#9-cicd-et-automatisation)

---

## 1. Vue d'ensemble

GoShop suit une **stratégie de tests en pyramide** :

```
        ┌─────────────┐
        │  Tests E2E  │  ← API + DB + Redis (+ scripts PowerShell finance)
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
- Couverture élevée sur le code métier critique (wallet, payment, dispute, withdrawal).
- Zéro régression sur les flux financiers (crédit, held, debt, retrait).
- Validation multi-tenant et sécurité.

Doc métier associée : [12-wallet-debt-sweep.md](12-wallet-debt-sweep.md).

---

## 2. Types de tests

### 2.1. Tests Unitaires
Logique métier pure (use cases, entités) avec **mocks** (`gomock`).

```bash
go test ./application/... -v
go test ./domain/... -v
```

Cible prioritaire v5.1 : `domain/entity/merchant_wallet*` (clawback, debt sweep), `wallet_usecase`, `dispute_usecase`.

### 2.2. Tests d'Intégration
Repositories + **PostgreSQL** réelle.

```bash
go test -tags=integration ./tests/integration/... -v
```

### 2.3. Tests End-to-End (Go)
API HTTP complète + DB + Redis.

**Scénarios historiques (tags `e2e`)** :
- Auth, order multi-tenant, payment webhook, tontine, crédit, withdrawal, JWT

```bash
go test -tags=e2e ./tests/e2e/... -v
```

### 2.4. Tests de Charge (k6)
```bash
k6 run tests/loadtest/scripts/auth_load.js
```

### 2.5. E2E Finance PowerShell (v5.1)
Scripts à la **racine du repo** — stack Docker/API locale, parfois **pay-in Yenga réel**.  
Voir [section 7](#7-e2e-finance--debt-sweep--clawback-powershell).

---

## 3. Structure des tests

```
tests/
├── unit/
├── integration/
├── e2e/                     # Tests E2E Go
├── loadtest/
└── testutils/

# Racine repo — E2E finance (PowerShell)
e2e-dispute-merchant-wins.ps1
e2e-debt-sweep-fraud.ps1
e2e-clawback-debt-sweep-chain.ps1
e2e-clawback-real-payin.ps1
e2e-dispute-customer-wins.ps1
e2e-clawback-post-release.ps1
e2e-yengapay-full-flow*.ps1
...
```

---

## 4. Exécution des tests

```bash
go test ./... -v
go test ./... -coverprofile=coverage.out
go test -tags=integration ./tests/integration/... -v
go test -tags=e2e ./tests/e2e/... -v
go test -race ./application/usecase/wallet_usecase/... -v
```

---

## 5. Tests de sécurité spécifiques

- Isolation multi-tenant (IDOR) via `X-Shop-Slug` + `RequireShopAccess`
- Validation `sort_by` (anti-injection)
- JWT expiré / secret invalide

(Exemples Go inchangés par rapport à v4.5 — conserver les patterns existants dans `tests/e2e`.)

---

## 6. Tests WebSocket

```powershell
.\Test-WebSocket-Notification.ps1
```

Ou test Go `TestWebSocketNotification` si présent sous `tests/e2e`.

---

## 7. E2E Finance — Debt sweep & clawback (PowerShell)

### 7.1. Prérequis

- API live : `http://localhost:8080` (`/health/live`)
- Postgres accessible via `docker compose exec db` (service `db`, DB `goshop_db`, user `postgres` — ajuster env si besoin)
- Variables :

```powershell
$env:YENGA_PAY_WEBHOOK_SECRET = "<secret webhook>"
$env:ADMIN_EMAIL              = "superadmin.yacine@goshop.com"
$env:ADMIN_PASSWORD           = "<password>"
# optionnel :
# $env:GOSHOP_BASE_URL = "http://localhost:8080"
# $env:INJECT_DEBT_CENTS = "50000"
```

### 7.2. Matrice smoke recommandée (post-release / CI manuelle)

| Script | Scénario | Attendu VERT |
|--------|----------|--------------|
| `e2e-dispute-merchant-wins.ps1` | Litige **pré-release**, `merchant_wins` | Escrow `released`, crédit ~net, **pas** de clawback |
| idem + `$env:INJECT_DEBT_CENTS="50000"` | Idem + dette injectée | Sweep dette, ledger `debt_sweep` |
| `e2e-debt-sweep-fraud.ps1` | Order A release → inject `debt_cents` → Order B release | Debt ↓, balance net cohérente, retrait bloqué si debt > 0 |
| `e2e-clawback-debt-sweep-chain.ps1` | 2 **pay-in réels** : release → retrait partiel → `customer_wins` → Order B sweep | debt créée par métier, puis swept ; ledger clawback/debt_add/debt_sweep |
| `e2e-clawback-real-payin.ps1` | Clawback seul, canal pay-in réel (sans force SQL téléphone) | Wallet ↓, escrow refunded, dispute resolved_customer |

### 7.3. Commandes types

```powershell
cd C:\Users\ifbbu\Desktop\GoShop   # ou chemin du clone

# 1) merchant_wins sans dette
Remove-Item Env:INJECT_DEBT_CENTS -ErrorAction SilentlyContinue
.\e2e-dispute-merchant-wins.ps1

# 2) merchant_wins avec inject
$env:INJECT_DEBT_CENTS = "50000"
.\e2e-dispute-merchant-wins.ps1

# 3) inject + double auto-release (webhook sim)
.\e2e-debt-sweep-fraud.ps1

# 4) chaîne réelle (navigateur / sandbox Yenga)
.\e2e-clawback-debt-sweep-chain.ps1
```

### 7.4. Assertions métier à ne pas régresser

1. **Clawback post-release** : `balance` puis `debt_cents` ; pas de freeze auto.
2. **Sweep** : `swept = min(debt, available)` ; ledger `transaction_type = debt_sweep`.
3. **Retrait** : HTTP 4xx si `debt_cents > 0`.
4. **merchant_wins** : pas de ligne clawback ; crédit (éventuellement net de dette).

### 7.5. Logs utiles

```powershell
docker compose logs goshop 2>&1 | Select-String -Pattern "debt_sweep|CreditWithDebtSweep|ClawbackToDebt|customer_wins|merchant_wins"
```

---

## 8. Bonnes pratiques

- Nommage : `TestXxx_Success` / `_ValidationError`
- Isolation : pas d’état global partagé entre tests
- Finance : préférer asserts sur **balance + debt + ledger**, pas seulement HTTP 200
- E2E PS1 : `-UseBasicParsing`, UTF-8, routes réelles (`/login`, `/api/orders/{id}/pay`, `/health/live`)

---

## 9. CI/CD et automatisation

Les workflows GitHub Actions exécutent en général les tests **Go** (unit / integration).  
Les scripts PowerShell finance restent un **smoke manuel** (ou job dédié Windows) tant qu’ils dépendent de Docker local et éventuellement d’un checkout Yenga réel.

```yaml
# Extrait typique — adapter à .github/workflows/ci-cd.yml
- name: Run Go tests
  run: |
    go test ./... -v -coverprofile=coverage.out
```

---

## 📚 Références

- [Wallet — dette, clawback & sweep](12-wallet-debt-sweep.md)
- [Glossaire](09-glossary.md)
- [Modèle de données](02-domain-model.md)
- [Documentation Go testing](https://pkg.go.dev/testing)
- [testify](https://github.com/stretchr/testify)
- [k6](https://k6.io/docs/)

---

**Dernière mise à jour** : 2026-10-03
```

---
