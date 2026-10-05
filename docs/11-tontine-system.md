
```markdown
# 🏦 Système de Tontine GoShop

**Version** : v5.1.0  
**Date** : 2026-10-05  
**Statut** : ✅ Implémenté (groupes, pay-in Yenga, webhook, voucher, redeem + release held / debt sweep)

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

Système de **tontine de biens physiques** : un groupe cotise régulièrement pour qu’un participant reçoive à tour de rôle un **produit** de la boutique (moto, électroménager, etc.), matérialisé par un **voucher** mono-boutique.

### Différence avec une tontine financière

| Projet | Nature | Objet |
|--------|--------|--------|
| **GoShop** | E-commerce SaaS | Bien physique (voucher produit) |
| Autres fintechs | Super-app cash | Redistribution d’argent |

### Cas d’usage typique

8 collègues, moto 500 000 FCFA → 8 cycles × 62 500 F → 8 vouchers / 8 livraisons.  
Commission plateforme (ex. 2,50 %) prélevée sur les cotisations (taux boutique configurable 0–15 %).

---

## 2. Modèle économique

### Flux (simplifié)

```text
Participants ──YengaPay──► webhook GoShop
                              │
                              ├─ enregistre tontine_payment
                              ├─ commission plateforme (net)
                              └─ crédit / hold wallet marchand selon cycle
Cycle complet ──► voucher bénéficiaire
Redeem en boutique ──► release held (+ debt_sweep si dette)
```

### Commission

```go
goshop_part := amount_cents * tontine_commission_rate / 10000
// rate en basis points (250 = 2,50 %)
```

**Phase actuelle (scheduler)** : la collecte « par cotisation » en batch est **désactivée** (`collectPerCotisation = false`).  
La commission est déjà prise en compte dans le **crédit net cycle** via `process_tontine_webhook` (évite double débit).

### Risque

| Acteur | Risque |
|--------|--------|
| GoShop | Principalement opérationnel / conformité |
| Marchand | Stock, litiges livraison ; wallet peut avoir **dette** post-clawback autre canal |
| Participants | Confiance sociale (défauts non assurés au MVP) |

---

## 3. Acteurs et rôles

| Acteur | Rôle | KYC |
|--------|------|-----|
| Marchand | Active tontine produit, livre, **redeem voucher** | KYC marchand boutique |
| Client créateur | Crée le groupe, invite | `kyc_level = verified` |
| Client participant | Cotise, reçoit voucher à son tour | `kyc_level = verified` |

Niveaux KYC client : `none` → `pending` → `verified` / `rejected`.

Lien **Customer ↔ User** (`user_id`) pour notifications WebSocket.

---

## 4. Workflow KYC

1. Client tente create/join → bloqué si non `verified`
2. Upload CNI / passeport → `pending`
3. Marchand review → `verified` ou `rejected`
4. Client peut créer / rejoindre

Contraintes fichiers : taille / MIME limités (voir handler KYC).

---

## 5. Workflow Tontine

### États groupe

```text
PENDING_MEMBERS → ACTIVE → COMPLETED
```

### Cycle

1. Notifications participants  
2. Paiements Yenga par participant (`pay_cycle`)  
3. Webhook → `process_tontine_webhook`  
4. Cycle complet → voucher bénéficiaire (+ hold éventuel)  
5. Marchand **redeem** voucher à la remise du bien  
6. Dernier cycle → `COMPLETED`

### Voucher

- Code unique, validité typique **6 mois**  
- **Mono-boutique** (`IsRedeemableInShop`)  
- Redeem : `RedeemTontineVoucherUsecase`

---

## 6. Wallet, held & debt sweep

Aligné modèle v5.1 ([12-wallet-debt-sweep.md](12-wallet-debt-sweep.md)) :

| Étape | Effet wallet |
|-------|----------------|
| Cotisations / fin de cycle | Crédit marchand (souvent **held** tant que bien non remis) |
| **Redeem voucher** | `ReleaseHeld(amount)` puis **debt_sweep** si `debt_cents > 0` |
| Ledger | Types `debt_sweep` possibles sur release held |

Fichiers clés :

- `application/usecase/tontine_usecase/redeem_tontine_voucher.go`
- `application/usecase/wallet_usecase/release_held_wallet.go`

Si redeem DB OK mais release held échoue → log critique (réconciliation manuelle possible).

---

## 7. Architecture technique

```text
interfaces/handler/tontine_handler + shop tontine_settings + kyc
        ↓
application/usecase/tontine_usecase
  create_group, join_group, pay_cycle,
  redeem_tontine_voucher, sync_tontine_payment
application/usecase/payment_usecase/process_tontine_webhook
application/scheduler/tontine_scheduler  (commission legacy OFF)
        ↓
domain/entity/tontine.go + repositories
        ↓
infrastructure/postgres/tontine/*
infrastructure/payment (Yenga)
```

Autres : preuves livraison tontine (`delivery_proof_usecase/*tontine*`).

---

## 8. Modèle de données

Tables (migration de base **`010_add_tontine.sql`** + évolutions) :

| Table | Rôle |
|-------|------|
| `product_tontine_settings` | Activation / cercles / min-max participants |
| `tontine_groups` | Groupe, invite_code, cycles, status |
| `tontine_participants` | Membres + `payout_position` |
| `tontine_payments` | Cotisations, commission, refs provider |
| `tontine_vouchers` | Code, shop_id, held, statut redeem |
| `customer_kyc_documents` | Pièces KYC |

Évolutions notables : commissions (**016**), intent provider / min participants / rates (**042–044**), **held voucher (046)**.

Colonnes boutique : `shop_payment_settings.tontine_enabled`, `tontine_commission_rate` (bps).

---

## 9. API Endpoints

### KYC

| Méthode | Endpoint | Acteur |
|---------|----------|--------|
| `POST` | `/api/customers/kyc/upload` | Client |
| `GET` | `/api/customers/{id}/kyc/status` | Client |
| `GET` | `/api/merchant/kyc/pending` | Marchand |
| `POST` | `/api/merchant/kyc/{customer_id}/review` | Marchand |

### Settings produit / boutique

| Méthode | Endpoint |
|---------|----------|
| `GET` | `/api/shops/{id}/tontine-settings?product_id=…` |
| `PUT` | `/api/shops/{id}/tontine-settings` |

### Groupes

| Méthode | Endpoint |
|---------|----------|
| `POST` | `/api/tontine/groups` |
| `POST` | `/api/tontine/groups/join` |
| `POST` | `/api/tontine/groups/{id}/pay` |
| `GET` | `/api/tontine/groups/{id}/payments` |

### Redeem (marchand, X-Shop-Slug)

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/tontine/vouchers/redeem` (ou route handler équivalente) | Body : `voucher_code`, `redeemed_by` → release held |

Vérifier le path exact dans Swagger / `tontine_handler.go`.

Toutes les routes métier : **JWT + `X-Shop-Slug`**.

---

## 10. Intégration YengaPay

Référence métier (pas de metadata riche) :

```text
TONTINE:{groupID[:8]}:{cycleNumber}:{participantID[:8]}
```

Webhook générique → détection référence tontine → `ProcessTontineWebhookUsecase`.

---

## 11. Sécurité et conformité

| Règle | Implémentation |
|-------|----------------|
| Multi-tenant | `shop_id` + `TenantResolver` (owner/collab) |
| Voucher | Check shop au redeem |
| KYC | Gate create/join/pay |
| Montants | `int64` centimes |
| Audit | zerolog |
| BCEAO | Cadre juridique à valider avant scale public |

---

## 12. Tests

```bash
go test ./tests/e2e/ -run TestTontine -v
go test ./application/usecase/tontine_usecase/... -v
```

Couverture typique E2E : shop, produit, settings, 4 clients KYC, create/join, pay cycle, isolation tenant, rejet non vérifié.

Tests unitaires : create/join/pay, redeem, process_tontine_webhook.

---

## 13. Limites et décisions

| Sujet | Décision |
|-------|----------|
| Distribution | ROTATING (LOCKED_SAVINGS reporté) |
| Position | Ordre d’arrivée |
| Commission | Net à la cotisation / cycle (scheduler legacy OFF) |
| Défauts | Confiance sociale, pas d’assurance |
| Relances auto cotisation | Limitées / à renforcer |
| Redeem + held | **Implémenté** + debt sweep |

---

## 14. Roadmap

| Phase | Contenu | Statut |
|-------|---------|--------|
| MVP groupes / pay / webhook / E2E | v2.9 | ✅ |
| WS notifications | v4.5 | ✅ |
| Redeem + held + debt sweep | v5.1 | ✅ |
| Relances défaut, SMS, dashboard stats | — | 📋 |
| LOCKED_SAVINGS, marketplace tontines | — | 📋 |

---

## 📚 Références

- [Architecture](01-architecture.md)
- [Multi-tenant](04-multi-tenant.md)
- [API](03-api-reference.md)
- [Wallet debt-sweep](12-wallet-debt-sweep.md)
- [KYC](KYC.md)
- Migrations : `010`, `016`, `042`–`046`

---

**Dernière mise à jour** : 2026-10-05
```

