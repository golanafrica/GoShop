
---

### 📄 Fichier : `README.md`
*(Copie et remplace **intégralement** le contenu de ce fichier)*

```markdown
# 🛒 GoShop - Plateforme E-commerce SaaS Multi-tenant pour l'Afrique

<div align="center">

# 🛒 GoShop

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8)](https://go.dev/)
[![Status](https://img.shields.io/badge/Status-v5.2.0_Audit_Hardened-blue)]()

**Plateforme E-commerce SaaS Multi-tenant pour l'Afrique**

</div>

---

## 📜 Licence

GoShop est distribué sous licence **Apache 2.0**. Voir le fichier [LICENSE](./LICENSE) pour plus de détails.

**Copyright © 2025-2026 GolAfrica. Tous droits réservés.**

**Vision** : Devenir le Shopify africain avec paiements Mobile Money intégrés, vente à crédit, système de tontine, et notifications temps réel.

**Statut** : ✅ **v5.2.0** — Audit de sécurité financière complété (Webhooks P0, Compensation P0, Idempotence P1, Multi-tenant P1).

---

## 🎯 Fonctionnalités Majeures

### 💳 Paiements Multi-providers
- **Yenga Pay** : Intégration API réelle avec validation HMAC-SHA256 des webhooks.
- **Orange Money / Moov Money / Wave** : Support via adaptateurs.
- **Cash on Delivery (COD)** : Workflow complet avec preuve de livraison et collecte de commission.
- **Machine à états robuste** pour les paiements et les retraits.

### 💰 Wallet marchand, Escrow & Anti-fraude (v5.1 / v5.2)
- **Escrow** : Fonds séquestrés jusqu’au délai de la zone de livraison + confirmation (ou résolution de litige).
- **Auto-release** : Crédit wallet avec **debt sweep** automatique si dette résiduelle.
- **Clawback post-release** (`customer_wins`) : Débit de la balance, puis inscription en `debt_cents` (sans freeze automatique).
- **Debt sweep** : Recouvrement automatique sur les prochains crédits / releases (vente, litige gagné, tranches, tontine).
- **Retrait bloqué** tant que `debt_cents > 0` (en plus des vérifications KYC et freeze).
- **Compensation Pending (v5.2)** : Machine à états de retrait avec état `compensation_pending`, job de retry automatique et tests E2E de chaos validant la restauration du solde.

### 💰 Crédit à la Consommation
- Plans de paiement personnalisables.
- Score de fiabilité client automatique.
- Validation KYC obligatoire.
- Calcul d'intérêts conforme aux régulations locales (ex: BCEAO, max 15% annuel).

### 🤝 Tontine
- Système d'épargne collective avec cycles de paiement automatisés.
- Commission plateforme configurable.
- Vouchers de participation et release held + debt sweep à la redemption.

### 🔔 Notifications Temps Réel
- **WebSocket** avec Redis Pub/Sub pour scalabilité horizontale.
- Notifications : commande, paiement, livraisons, rappels de crédit.
- Fallback email (SMTP) et Telegram si WebSocket indisponible.

### 🔒 Sécurité & Résilience Renforcées (v5.2.0)
- **Webhooks (P0)** : Matching strict par `provider_ref` (plus de fallback ambigu) et atomicité garantie entre la création d'escrow et la confirmation de commande.
- **Idempotence HTTP (P1)** : Prévention du cache poisoning inter-utilisateurs via vérification stricte `UserID` + `Endpoint` avant replay.
- **Isolation Multi-tenant (P1)** : Restriction stricte du header `X-Shop-Slug` en production (ignoré par défaut, forçant l'utilisation du Host/Custom Domain).
- **Freeze Job (P1/P2)** : Pagination SQL (`LIMIT`/`OFFSET`) pour éviter les fuites de mémoire (OOM) et envoi **réel** des notifications administrateur.
- **Auth** : JWT avec secret obligatoire (min 32 caractères), Bcrypt coût minimum 10, 2FA (TOTP), et API Keys (header uniquement, pas de query param).

### 📦 Paiement en Tranches & Zones de Livraison (v5.0.0)
- **47+ zones** (Afrique de l’Ouest) : délais urbain / rural / international.
- **Scheduler** : Libération escrow = livraison + délai zone (fallback 30 jours).
- **Litige** : Statut `disputed` bloque atomiquement toute libération de fonds.

---

## 🚀 Démarrage Rapide

### Prérequis
- Go **1.21+**
- PostgreSQL 16+
- Redis 7+
- Docker & Docker Compose

### Installation

```bash
git clone https://github.com/golanafrica/GoShop.git
cd GoShop

cp .env.example .env
# Éditer .env (JWT_SECRET ≥ 32 caractères, DB, Redis, Yenga…)

docker compose up -d
# ou : docker-compose up -d postgres redis

# Lancer l'API
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

### Boutiques / Multi-tenant
- `POST|GET /api/shops 🔒` · `PUT /api/shops/{id} 🔒`
- Header **`X-Shop-Slug`** requis (uniquement en dev/staging, ignoré en prod).

### Paiements & Webhooks
- `POST /api/orders/{id}/pay 🔒`
- `GET /api/payments` · `GET /api/payments/{id}`
- `POST /webhooks/{provider}` (Validation HMAC-SHA256)

### Wallet & Retraits
- `GET /api/wallet 🔒` — expose `balance_cents`, `held_cents`, **`debt_cents`**, **`withdrawal_blocked`**
- `POST /api/withdrawals 🔒` — **refusé si `debt_cents > 0`**
- `GET /api/withdrawals 🔒`

### Administration
- Gestion des litiges, taux de commission, jobs planifiés (Freeze Job, Compensation Retry), et révocation de sessions.

---

## 🧪 Tests & Qualité

### Go

```bash
go test ./... -v
go test -tags=integration ./tests/integration/... -v
```

### E2E Finance (PowerShell)

```powershell
$env:YENGA_PAY_WEBHOOK_SECRET = "..."
$env:ADMIN_EMAIL = "superadmin@goshop.com"
$env:ADMIN_PASSWORD = "..."

# Scénarios de dette et litiges
.\e2e-dispute-merchant-wins.ps1
.\e2e-debt-sweep-fraud.ps1
.\e2e-clawback-debt-sweep-chain.ps1

# Scénarios de compensation de retrait (v5.2)
.\e2e-withdrawal-compensation-pending.ps1
.\e2e-compensation-pending-job.ps1

# Scénario Freeze Job
.\e2e-wallet-grace-expiry-job.ps1
```

---

## 📊 Observabilité

- Logs structurés JSON (Zerolog) avec Request ID.
- Métriques Prometheus : `GET /metrics` (paiements, auth, websocket, durée des requêtes).
- Stack d'observabilité optionnelle : Loki / Grafana / Promtail (voir `docker-compose.yml`).

---

## 🛠️ Architecture

Clean Architecture / Domain-Driven Design (DDD) : `domain` → `application` → `interfaces` → `infrastructure`, avec bootstrap dans `internal/app`.

**Stack** : Go, Chi Router, PostgreSQL 16, Redis 7, Prometheus, Zerolog, Docker.

---

## 🗄️ Base de données

Migrations versionnées sous `migrations/`.
Tables finance clés :
- `merchant_wallets` (`balance_cents`, `held_cents`, `debt_cents`, `is_frozen`…)
- `wallet_transactions` (types : `clawback`, `debt_add`, `debt_sweep`, `withdrawal_reversal`)
- `payments`, `withdrawals`, `escrow_accounts`, `disputes`, `delivery_zones`

---

## 📚 Documentation

| Doc | Lien |
|-----|------|
| Architecture | [docs/01-architecture.md](docs/01-architecture.md) |
| Modèle de données | [docs/02-domain-model.md](docs/02-domain-model.md) |
| API Reference | [docs/03-api-reference.md](docs/03-api-reference.md) |
| Tests & E2E | [docs/08-testing-guide.md](docs/08-testing-guide.md) |
| Wallet Debt / Clawback | [docs/12-wallet-debt-sweep.md](docs/12-wallet-debt-sweep.md) |
| Tontine | [docs/11-tontine-system.md](docs/11-tontine-system.md) |
| Changelog | [CHANGELOG.md](CHANGELOG.md) |
| Sécurité | [SECURITY.md](SECURITY.md) |

---

## 🤝 Contribuer

Voir [docs/07-contributing.md](docs/07-contributing.md).

**Commits** : `feat:` · `fix:` · `docs:` · `test:` · `chore:` · `security:` · `perf:` · `refactor:`

---

**Dernière mise à jour** : Octobre 2026  
**Version** : **v5.2.0**  
**Statut** : ✅ Audit de sécurité financière P0/P1 complété et validé par tests E2E.
```

---
