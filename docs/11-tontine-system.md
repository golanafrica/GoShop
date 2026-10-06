Les deux E2E tontine sont **VERT** → le palier tontine du chronogramme est **fermé**.

Voici `docs/11-tontine-system.md` **mis à jour** (E2E net+held / redeem+sweep, migration `054` `name`, runner migrations, date 2026-10-06) — prêt à coller :

```markdown
# 🏦 Système de Tontine GoShop

**Version** : v5.2.0  
**Date** : 2026-10-06  
**Statut** : ✅ Implémenté et validé E2E (groupes, pay-in Yenga, webhook, crédit **net + held**, voucher, redeem + **debt_sweep**)

---

## 📋 Table des matières

1. [Vue d'ensemble](#1-vue-densemble)
2. [Modèle économique](#2-modèle-économique)
3. [Acteurs et rôles](#3-acteurs-et-rôles)
4. [Workflow KYC](#4-workflow-kyc)
5. [Workflow Tontine](#5-workflow-tontine)
6. [Wallet, held & debt sweep](#6-wallet-held--debt-sweep)
7. [Architecture technique](#7-architecture-technique)
8. [Modèle de données](#8-modèle-de-données)
9. [API Endpoints](#9-api-endpoints)
10. [Intégration YengaPay](#10-intégration-yengapay)
11. [Sécurité et conformité](#11-sécurité-et-conformité)
12. [Tests](#12-tests)
13. [Limites et décisions](#13-limites-et-décisions)
14. [Roadmap](#14-roadmap)

---

## 1. Vue d'ensemble

### Qu'est-ce que la Tontine GoShop ?

Système de **tontine de biens physiques** : un groupe cotise régulièrement pour qu’un participant reçoive à tour de rôle un **produit** de la boutique, matérialisé par un **voucher** mono-boutique.

### Différence avec une tontine financière

| Projet     | Nature          | Objet                           |
| ---------- | --------------- | ------------------------------- |
| **GoShop** | E-commerce SaaS | Bien physique (voucher produit) |
| Fintechs   | Super-app cash  | Redistribution d’argent         |

### Cas d’usage typique

N participants, produit à prix P → N cycles de cotisation → vouchers / livraisons.  
Commission plateforme en **basis points** (configurable boutique, typiquement 0–15 %).

---

## 2. Modèle économique

### Flux

```text
Participants ──YengaPay──► webhook GoShop (process_tontine_webhook)
        │
        ├─ tontine_payment
        ├─ commission (une fois / cycle net)
        └─ crédit wallet marchand : balance += net ET held += net
Cycle complet ──► voucher bénéficiaire
Redeem en boutique ──► ReleaseHeld(net) + debt_sweep si debt_cents > 0
```

### Commission

```text
commission = gross * tontine_commission_rate_bps / 10000
net        = gross - commission
```

Exemple E2E (N=3, prix produit 3 000 000 centimes, **150 bps**) :

| Grandeur | Valeur (centimes) |
| -------- | ----------------- |
| Gross    | 3 000 000         |
| Comm     | 45 000            |
| Net      | 2 955 000         |

**Phase actuelle** : collecte scheduler « par cotisation » **désactivée** (`collect_per_cotisation` / Phase 1.2 OFF).  
La commission est prise **une seule fois** dans le crédit net cycle (pas de `commission_debit` par cotisation).

---

## 3. Acteurs et rôles

| Acteur             | Rôle                                              | Prérequis              |
| ------------------ | ------------------------------------------------- | ---------------------- |
| Marchand           | Settings produit, livre, **redeem voucher**       | KYC boutique           |
| Client créateur    | Crée le groupe, invite                            | KYC client `verified`  |
| Client participant | Cotise, reçoit voucher                            | KYC client `verified`  |

Lien **Customer ↔ User** (`user_id`) pour collab / WS selon routes.

---

## 4. Workflow KYC

1. Create/join bloqué si client non `verified`
2. Upload document → `pending`
3. Review marchand → `verified` / `rejected`
4. Accès create / join / pay

*(Les documents KYC sont dans `customer_kyc_documents` — pas une colonne `kyc_status` sur `customers`.)*

---

## 5. Workflow Tontine

### États groupe

```text
PENDING_MEMBERS → ACTIVE → COMPLETED
```

### Cycle

1. Groupe créé (`circle_type` FAMILY / etc., `total_cycles`, produit tontine-enabled)
2. Joins jusqu’à N participants
3. `POST …/pay` + webhooks SUCCESS pour le cycle
4. Crédit **net + held** sur le wallet marchand (une ligne ledger cycle)
5. Voucher créé (souvent à la clôture cycle / webhook)
6. Marchand **redeem** → held libéré + éventuel debt sweep
7. Dernier cycle → `COMPLETED`

### Voucher

- Code unique, mono-boutique
- Redeem : `RedeemTontineVoucherUsecase` → `ReleaseHeldWalletUsecase`

---

## 6. Wallet, held & debt sweep

Aligné v5.1+ ([12-wallet-debt-sweep.md](12-wallet-debt-sweep.md)) :

| Étape                         | Effet wallet                                      | Assert E2E                          |
| ----------------------------- | ------------------------------------------------- | ----------------------------------- |
| Fin de cycle (3× pay SUCCESS) | `balance += net`, `held += net`, `debt` inchangé  | `e2e-tontine-net-held.ps1`          |
| Inject dette + **redeem**     | `held → 0`, `debt` balayée, `balance` = net − debt | `e2e-tontine-redeem-debt-sweep.ps1` |
| Ledger                        | 1× crédit cycle ; `debt_sweep` au redeem si dette | count cycle = 1 ; type debt_sweep   |

Fichiers :

- `application/usecase/payment_usecase/process_tontine_webhook.go` (crédit net)
- `application/usecase/tontine_usecase/redeem_tontine_voucher.go`
- `application/usecase/wallet_usecase/release_held_wallet.go`

Si redeem DB OK mais release held échoue → log critique (réconciliation manuelle).

---

## 7. Architecture technique

```text
interfaces/handler (tontine, settings, kyc)
    ↓
application/usecase/tontine_usecase
    create_group, join_group, pay_cycle, redeem_tontine_voucher
application/usecase/payment_usecase/process_tontine_webhook
application/scheduler/tontine_scheduler  (commission legacy OFF)
    ↓
domain/entity + repositories
    ↓
infrastructure/postgres/tontine/*
infrastructure/payment (Yenga)
```

---

## 8. Modèle de données

| Table                      | Rôle                                        |
| -------------------------- | ------------------------------------------- |
| `product_tontine_settings` | Activation, min/max, `allow_family_circle`  |
| `tontine_groups`           | Groupe, **name** (mig. 054), invite, cycles |
| `tontine_participants`     | Membres + position                          |
| `tontine_payments`         | Cotisations, refs provider                  |
| `tontine_vouchers`         | Code, shop, held, redeem                    |
| `customer_kyc_documents`   | Pièces KYC                                  |

Migrations notables : **010** (base), **016** (commissions), **042–044**, **046** (held voucher), **054** (`name` sur `tontine_groups`).

Schéma appliqué via runner Compose : `tests/loadtest/scripts/migrate.sh` + `schema_migrations` ([06-migration-plan.md](06-migration-plan.md)).

---

## 9. API Endpoints

### KYC

| Méthode | Endpoint                                 |
| ------- | ---------------------------------------- |
| `POST`  | `/api/customers/kyc/upload`              |
| `GET`   | `/api/customers/{id}/kyc/status`         |
| `GET`   | `/api/merchant/kyc/pending`              |
| `POST`  | `/api/merchant/kyc/{customer_id}/review` |

### Settings

| Méthode | Endpoint                                        |
| ------- | ----------------------------------------------- |
| `GET`   | `/api/shops/{id}/tontine-settings?product_id=…` |
| `PUT`   | `/api/shops/{id}/tontine-settings`              |

### Groupes

| Méthode | Endpoint                            |
| ------- | ----------------------------------- |
| `POST`  | `/api/tontine/groups`               |
| `POST`  | `/api/tontine/groups/join`          |
| `POST`  | `/api/tontine/groups/{id}/pay`      |
| `GET`   | `/api/tontine/groups/{id}/payments` |

### Redeem (marchand, JWT + `X-Shop-Slug`)

| Méthode | Endpoint                         |
| ------- | -------------------------------- |
| `POST`  | `/api/tontine/vouchers/redeem`   |

Path exact : Swagger / `tontine_handler.go`.

---

## 10. Intégration YengaPay

Référence métier typique :

```text
TONTINE:{groupID}:{cycleNumber}:{participantRef}
```

Webhook générique → détection référence tontine → `ProcessTontineWebhookUsecase`.

---

## 11. Sécurité et conformité

| Règle        | Implémentation                              |
| ------------ | ------------------------------------------- |
| Multi-tenant | `shop_id` + accès owner/collab              |
| Voucher      | Check shop au redeem                        |
| KYC          | Gate create/join/pay                        |
| Montants     | `int64` centimes                            |
| Audit        | zerolog + ledger wallet                     |

---

## 12. Tests

### Unit / Go E2E

```bash
go test ./tests/e2e/ -run TestTontine -v
go test ./application/usecase/tontine_usecase/... -v
```

### PowerShell E2E (validés 2026-10-06)

| Script                              | Scénario                                      | Résultat |
| ----------------------------------- | --------------------------------------------- | -------- |
| `e2e-tontine-net-held.ps1`          | 3 pay-in cycle → bal=held=net, 1 ledger cycle | ✅ VERT  |
| `e2e-tontine-redeem-debt-sweep.ps1` | Inject debt → redeem → held=0, debt=0, sweep  | ✅ VERT  |

Prérequis env : `YENGA_PAY_WEBHOOK_SECRET`, `ADMIN_EMAIL`, `ADMIN_PASSWORD` ; stack `docker compose up -d` (service `db`, pas `postgres`).

---

## 13. Limites et décisions

| Sujet                    | Décision                                           |
| ------------------------ | -------------------------------------------------- |
| Distribution             | ROTATING (LOCKED_SAVINGS reporté)                  |
| Commission               | Net au cycle ; scheduler per-cotisation **OFF**    |
| Crédit cycle             | **balance += net** et **held += net**              |
| Redeem                   | Release held + **debt_sweep** si dette             |
| Défauts participants     | Confiance sociale (pas d’assurance MVP)            |

---



## 📚 Références

- [Architecture](01-architecture.md)
- [Multi-tenant](04-multi-tenant.md)
- [API](03-api-reference.md)
- [Wallet debt-sweep](12-wallet-debt-sweep.md)
- [Plan migrations](06-migration-plan.md)
- [KYC](KYC.md)
- Migrations : `010`, `016`, `042`–`046`, `054`

---

**Dernière mise à jour** : 2026-10-06  
