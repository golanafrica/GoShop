## Audit README.md (avant réécriture)

| Zone | Problème | Priorité |
|------|----------|----------|
| **Statut / version** | Affiche encore **v4.5.0** (21 juil. 2026) alors que `main` a **v5.0 zones** + **v5.1 debt-sweep** | P0 |
| **Fonctionnalités** | Pas de section **Wallet / escrow / dette / clawback / gate retrait** | P0 |
| **Nouveautés** | v5.0 présent en double (haut + bas) ; **aucune** entrée v5.1 | P0 |
| **Tests** | Uniquement Go + WebSocket PS1 ; **scripts finance** absents | P0 |
| **Doc links** | Chemins faux (`docs/architecture.md` vs `docs/01-architecture.md`, etc.) | P0 |
| **Licence footer** | En-tête **Apache 2.0**, bas de page **Propriétaire** — incohérent | P0 |
| **Go version** | Badge **1.25**, texte « Go 1.23+ » / stack « 1.23 » | P1 |
| **API Routes** | Withdrawals listés ; **pas** `GET /api/wallet` ni champs debt | P1 |
| **Migrations** | Tableau s’arrête à 034 + 051–053 en vrac ; **pas** `debt_cents` / idempotency wallet si présents en repo | P1 |
| **Tables** | Pas `merchant_wallets`, `wallet_transactions`, `escrow`, `disputes` | P1 |
| **Roadmap** | Phase 8+ « en cours » ; **debt-sweep non coché** | P1 |
| **Archi tree** | OK globalement ; schedulers escrow/installment non cités | P2 |

**Verdict :** le README est une bonne vitrine **v4.5**, mais **trompeur** pour un clone 2026-10 (finance anti-fraude non documentée, liens docs cassés, version / licence contradictoires).

---

Ci-dessous le **README complet** aligné v5.1 (contenu utile conservé, corrections ci-dessus appliquées).

```markdown
# 🛒 GoShop - Plateforme E-commerce SaaS Multi-tenant pour l'Afrique

<div align="center">

# 🛒 GoShop

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8)](https://go.dev/)
[![Production Ready](https://img.shields.io/badge/Status-Production_Ready-green)]()

**Plateforme E-commerce SaaS Multi-tenant pour l'Afrique**

</div>

---

## 📜 Licence

GoShop est distribué sous licence **Apache 2.0**. Voir le fichier [LICENSE](./LICENSE) pour plus de détails.

**Copyright © 2025-2026 GolAfrica. Tous droits réservés.**

**Vision** : Devenir le Shopify africain avec paiements Mobile Money intégrés (Wave, Orange Money, Moov Money), vente à crédit, système de tontine, et notifications temps réel.

**Statut** : ✅ **v5.1.0** — zones de livraison, paiement en tranches, **wallet anti-fraude** (dette résiduelle, clawback, debt sweep), E2E finance validés

---

## 🎯 Fonctionnalités Majeures

### 💳 Paiements Multi-providers
- **Orange Money** : USSD `#144*111#`
- **Moov Money** : USSD `#135*2#`
- **Wave** : Support complet
- **Yenga Pay** : Intégration API réelle
- **Cash on Delivery (COD)** : Workflow complet avec preuve de livraison
- **Validation HMAC-SHA256** des webhooks
- **Machine à états** pour les paiements

### 💰 Wallet marchand, escrow & anti-fraude (v5.1)
- **Escrow** : fonds séquestrés jusqu’à délai zone + livraison (ou résolution de litige)
- **Auto-release** : crédit wallet avec **debt sweep** si dette résiduelle
- **Clawback post-release** (`customer_wins`) : débit balance puis `debt_cents` (sans freeze auto)
- **Debt sweep** : recovery automatique sur les prochains crédits / releases (sale, merchant_wins, installment, tontine held)
- **Retrait bloqué** tant que `debt_cents > 0` (en plus de KYC et freeze)
- **API** : `GET /api/wallet` expose `debt_cents`, `withdrawal_blocked`
- **Doc** : [docs/12-wallet-debt-sweep.md](docs/12-wallet-debt-sweep.md)

### 💰 Crédit à la Consommation (v3.5.0)
- Plans de paiement personnalisables (3, 6, 12 mois)
- Score de fiabilité client automatique
- Validation KYC obligatoire
- Calcul d'intérêts conforme BCEAO (max 15% annuel)
- Gestion des pénalités de retard
- Scheduler automatique de relances

### 🤝 Tontine (v3.3.0)
- Système d'épargne collective
- Cycles de paiement automatisés
- Commission plateforme configurable
- Gestion des groupes thématiques
- Vouchers de participation
- Release held + debt sweep à la redemption (aligné v5.1)

### 🔔 Notifications Temps Réel (v4.5.0)
- **WebSocket** avec Redis Pub/Sub pour scalabilité horizontale
- Notifications : commande, paiement, livraisons, rappels de crédit
- Fallback email (SMTP) si WebSocket indisponible

### 🔒 Sécurité Renforcée (v4.5.0+)
- **JWT avec secret obligatoire** (min 32 caractères)
- **Bcrypt coût minimum 10**
- **Anti-injection SQL** (whitelist `sort_by`)
- **Isolation multi-tenant** (`RequireShopAccess`)
- **Logs sécurisés** : pas de mots de passe en clair
- **Intégrité financière** : verrous `FOR UPDATE`, idempotence crédit, gate retrait sur dette

### 👥 Gestion des Utilisateurs
- **RBAC** : super_admin, admin, merchant, customer, collaborator, guest, …
- **2FA (TOTP)**
- **Sessions** : révocation, historique
- **API Keys** : scopes granulaires (header only, pas `?api_key=`)

### 🏪 Multi-tenant Avancé
- Isolation par `shop_id`
- Header `X-Shop-Slug` ou domaine personnalisé
- KYC marchand obligatoire pour retraits / opérations sensibles

### 📦 Paiement en Tranches & Zones de Livraison (v5.0.0)
- **47+ zones** (Afrique de l’Ouest) : délais urbain / rural / international
- **Scheduler** : libération escrow = livraison + délai zone (fallback 30 j)
- **Litige** : statut `disputed` bloque toute release
- Dashboard marchand + notifications proactives

---

## 🎉 Nouveautés v5.1.0 (Debt sweep)

- Dette résiduelle explicite (`debt_cents`) après clawback post-release
- `CreditWithDebtSweep` / release installment & tontine held alignés
- Ledger : `clawback`, `debt_add`, `debt_sweep`
- E2E PowerShell : merchant_wins, fraud inject, chaîne pay-in réel
- Doc : [docs/12-wallet-debt-sweep.md](docs/12-wallet-debt-sweep.md) · [CHANGELOG](CHANGELOG.md)

### Nouveautés v5.0.0
- Zones de livraison dynamiques + paiement en tranches + auto-release escrow
- Blocage atomique par litige

### Nouveautés v4.5.0
- WebSocket + Redis Pub/Sub
- Durcissement JWT / bcrypt / RequireShopAccess
- Liaison `customers.user_id` (migration 033)

*(Historique détaillé v4.x / v3.x : voir [CHANGELOG.md](CHANGELOG.md))*

---

## 🚀 Démarrage Rapide

### Prérequis
- Go **1.25** (voir `go.mod`)
- PostgreSQL 16+
- Redis 7+
- Docker (recommandé)

### Installation

```bash
git clone https://github.com/golanafrica/GoShop.git
cd GoShop

cp .env.example .env
# Éditer .env (JWT_SECRET ≥ 32 caractères, DB, Redis, Yenga…)

docker compose up -d
# ou : docker-compose up -d postgres redis

# Migrations selon votre toolchain (golang-migrate / cmd local)
go run ./cmd/api
```

### Variables d'environnement critiques

```bash
JWT_SECRET=une_chaine_tres_longue_et_aleatoire_d_au_moins_32_caracteres
BCRYPT_COST=12

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=goshop_db

REDIS_ADDR=localhost:6379
APP_PORT=8080
APP_ENV=development

YENGA_PAY_WEBHOOK_SECRET=your_webhook_secret
YENGA_PAY_ENV=test
```

### Endpoints techniques

| Service    | Endpoint                               |
| ---------- | -------------------------------------- |
| API        | `http://localhost:8080`                |
| WebSocket  | `ws://localhost:8080/ws/notifications` |
| Liveness   | `GET /health/live`                     |
| Readiness  | `GET /health/ready`                    |
| Metrics    | `GET /metrics`                         |
| Swagger UI | `GET /swagger/index.html`              |

---

## 📈 Routes API (extrait)

### Authentification
- `POST /register` · `POST /login` · `POST /logout 🔒` · `POST /auth/refresh` · `GET /auth/me 🔒`

### Boutiques / multi-tenant
- `POST|GET /api/shops 🔒` · `PUT /api/shops/{id} 🔒`
- Header **`X-Shop-Slug`** requis sur la plupart des routes métier

### Paiements
- `POST /api/orders/{id}/pay 🔒`
- `GET /api/payments` · `GET /api/payments/{id}`
- `POST /webhooks/{provider}` (HMAC)

### Wallet & retraits (v5.1)
- `GET /api/wallet 🔒` — solde, held, **debt_cents**, **withdrawal_blocked**
- `GET /api/wallet/freeze-status 🔒`
- `POST /api/withdrawals 🔒` — **refusé si `debt_cents > 0`**
- `GET /api/withdrawals 🔒`

### Litiges (admin)
- Ouverture dispute (routes client/marchand selon config)
- `POST /api/admin/disputes/{id}/resolve 🔒👑` — `customer_wins` | `merchant_wins`

### COD · Crédit · Tontine · 2FA · Sessions · API Keys · Admin
Voir [docs/03-api-reference.md](docs/03-api-reference.md) et Swagger.

---

## 🧪 Tests

### Go

```bash
go test ./... -v
go test -tags=integration ./tests/integration/... -v
go test -tags=e2e ./tests/e2e/... -v
```

### E2E Finance (PowerShell) — smoke wallet / debt

```powershell
$env:YENGA_PAY_WEBHOOK_SECRET = "..."
$env:ADMIN_EMAIL = "..."
$env:ADMIN_PASSWORD = "..."

.\e2e-dispute-merchant-wins.ps1
$env:INJECT_DEBT_CENTS = "50000"
.\e2e-dispute-merchant-wins.ps1
.\e2e-debt-sweep-fraud.ps1
# Pay-in réel (sandbox Yenga) :
.\e2e-clawback-debt-sweep-chain.ps1
.\e2e-clawback-real-payin.ps1
```

Détail : [docs/08-testing-guide.md](docs/08-testing-guide.md) · [docs/12-wallet-debt-sweep.md](docs/12-wallet-debt-sweep.md)

### WebSocket / charge

```powershell
.\Test-WebSocket-Notification.ps1
```

```bash
k6 run tests/loadtest/scripts/auth_load.js
```

---

## 📊 Observabilité

- Logs JSON (zerolog), Request ID, Loki / Grafana
- Prometheus : `GET /metrics` (paiements, auth, websocket, …)
- Health : `/health/live`, `/health/ready`

---

## 🔒 Sécurité

- JWT + bcrypt ≥ 10 + 2FA + sessions + API keys (header)
- Multi-tenant : `shop_id` + `RequireShopAccess`
- Webhooks HMAC-SHA256
- **Finance** : `FOR UPDATE`, idempotence crédits, **retrait bloqué sous dette**

---

## 🛠️ Architecture

Clean Architecture / DDD : `domain` → `application` → `interfaces` → `infrastructure`, bootstrap `internal/app`.

Schedulers : COD, paiements, crédit, tontine, **escrow auto-release**, **installment auto-release**.

Arborescence détaillée : [docs/01-architecture.md](docs/01-architecture.md)

**Stack** : Go (voir `go.mod`), Chi, PostgreSQL 16, Redis 7, Prometheus, Zerolog, Docker, Kubernetes.

---

## 🗄️ Base de données

Migrations versionnées sous `migrations/` (001 → 034 sécurité/RBAC, puis zones **051+**, wallet debt / idempotency selon fichiers présents dans le repo).

Tables finance clés :
- `merchant_wallets` (`balance_cents`, `held_cents`, `debt_cents`, freeze…)
- `wallet_transactions` (dont `clawback`, `debt_add`, `debt_sweep`)
- `payments`, `withdrawals`, escrow / disputes, `delivery_zones`

Modèle : [docs/02-domain-model.md](docs/02-domain-model.md)

---

## 🗺️ Roadmap

| Phase | Statut |
|-------|--------|
| 1–7 Multi-tenant, paiements, COD, crédit, tontine, RBAC/2FA, WebSocket | ✅ |
| **Zones + installment (v5.0)** | ✅ |
| **Debt-sweep / clawback (v5.1)** | ✅ |
| Intégrations Mobile Money natives (Orange/Moov/Wave au-delà de Yenga) | 🚧 |
| Analytics avancés | 📋 |
| Apps mobile | 📋 |

---

## 📚 Documentation

| Doc | Lien |
|-----|------|
| Architecture | [docs/01-architecture.md](docs/01-architecture.md) |
| Modèle de données | [docs/02-domain-model.md](docs/02-domain-model.md) |
| API Reference | [docs/03-api-reference.md](docs/03-api-reference.md) |
| Multi-tenant | [docs/04-multi-tenant.md](docs/04-multi-tenant.md) |
| Tests | [docs/08-testing-guide.md](docs/08-testing-guide.md) |
| Glossaire | [docs/09-glossary.md](docs/09-glossary.md) |
| **Wallet debt / clawback** | [docs/12-wallet-debt-sweep.md](docs/12-wallet-debt-sweep.md) |
| Tontine | [docs/11-tontine-system.md](docs/11-tontine-system.md) |
| KYC | [docs/KYC.md](docs/KYC.md) |
| Changelog | [CHANGELOG.md](CHANGELOG.md) |
| Sécurité | [SECURITY.md](SECURITY.md) |

---

## 🤝 Contribuer

Voir [docs/07-contributing.md](docs/07-contributing.md).

**Commits** : `feat:` · `fix:` · `docs:` · `test:` · `chore:` · `security:` · `perf:` · `refactor:`

---

## 🎉 Remerciements

Développé avec passion pour l'Afrique 🌍 — Équipe Golanafrica

---

**Dernière mise à jour** : 3 octobre 2026  
**Version** : **v5.1.0**  
**Statut** : ✅ Production-ready (finance escrow + debt-sweep E2E VERT)
```

---
