
## Fichier complet prêt à coller — `docs/KYC.md`

```markdown
# 🔐 Système KYC (Know Your Customer)

**Version** : v5.1.0  
**Date** : 2026-10-05  
**Statut** : ✅ Client (tontine) + ✅ Marchand (boutique / admin)

---

## 📋 Table des matières

1. [Vue d'ensemble](#1-vue-densemble)
2. [Deux pistes KYC](#2-deux-pistes-kyc)
3. [KYC client (Customer)](#3-kyc-client-customer)
4. [KYC marchand (Shop)](#4-kyc-marchand-shop)
5. [Architecture](#5-architecture)
6. [Modèle de données](#6-modèle-de-données)
7. [API](#7-api)
8. [Sécurité](#8-sécurité)
9. [Intégrations (tontine, retraits)](#9-intégrations-tontine-retraits)
10. [Tests](#10-tests)
11. [Roadmap](#11-roadmap)

---

## 1. Vue d'ensemble

Le KYC GoShop sert à **identifier** les acteurs avant des actions à risque :

| Objectif | Exemple |
|---------|---------|
| Conformité / confiance | BCEAO, anti-fraude basique |
| Gate produit | Tontine réservée aux clients `verified` |
| Gate boutique | Retrait / opérations sensibles si boutique `kyc_status = verified` |
| Traçabilité | Documents + review audit |

---

## 2. Deux pistes KYC

| Piste | Qui | Validé par | Migration |
|-------|-----|------------|-----------|
| **Customer** | Acheteur / participant tontine | **Marchand** de la boutique | `011_add_kyc.sql` |
| **Merchant** | Owner / boutique | **Admin plateforme** | `019_add_merchant_kyc.sql` |

Ne pas confondre :
- « Marchand vérifie le client » ≠ « Admin vérifie le marchand ».

---

## 3. KYC client (Customer)

### Niveaux (`customers.kyc_level`)

| Niveau | Capacités tontine |
|--------|-------------------|
| `none` | ❌ |
| `pending` | ❌ (en attente) |
| `verified` | ✅ create / join / pay |
| `rejected` | ❌ (re-upload) |

Transitions : `none → pending → verified | rejected → pending`.

### Documents

Types : `cni`, `passport`, `other`.  
Contraintes typiques : max **5 Mo**, MIME jpeg/png/pdf, max **3** docs, pas de doublon de type en pending.

### Workflow

1. Client upload → document `pending`, niveau client souvent `pending`  
2. Marchand liste `/api/merchant/kyc/pending`  
3. Review approve/reject → `verified` / `rejected`  
4. Tontine : `CanParticipateInTontine()` exige `verified`

### Code

- `application/usecase/customer_usecase/upload_kyc.go`
- `application/usecase/customer_usecase/review_kyc.go`
- `interfaces/handler/customer_handler/kyc_handler.go`
- `domain/entity/customer_kyc.go`

---

## 4. KYC marchand (Shop)

### Statuts (`shops.kyc_status`)

| Statut | Signification |
|--------|----------------|
| `unverified` | Jamais soumis / défaut |
| `pending` | Dossier soumis, review admin |
| `verified` | Approuvé plateforme |
| `rejected` | Rejeté (raison + resoumission) |

### Documents (`shop_kyc_documents`)

Types : `identity_card`, `passport`, `business_registry`, `tax_certificate`, `bank_statement`, `other`.

### Workflow

1. Marchand `POST /api/merchant/kyc/submit`  
2. Boutique → `pending`  
3. Admin `GET /api/admin/merchant-kyc/pending`  
4. Admin `PUT|POST /api/admin/merchant-kyc/{shop_id}/review`  
5. `verified` ou `rejected`

### Code

- `application/usecase/merchant_kyc_usecase/submit_kyc.go`
- `get_kyc_status.go`, `review_kyc.go`
- `interfaces/handler/merchant_kyc_handler/merchant_kyc_handler.go`
- `infrastructure/postgres/shop/kyc_document_repository.go`

Fonction SQL illustrative : `shop_can_withdraw(shop_id)` → `kyc_status = 'verified'`.

---

## 5. Architecture

```text
Customer path                    Merchant path
─────────────                    ─────────────
kyc_handler (customer)           merchant_kyc_handler
  ↓                                ↓
upload_kyc / review_kyc          submit / status / admin review
  ↓                                ↓
customer_kyc_documents           shop_kyc_documents
customers.kyc_level              shops.kyc_status
```

Multi-tenant : documents client toujours scopés `shop_id`.  
Admin merchant-kyc : hors tenant marchand, rôle admin/super_admin.

---

## 6. Modèle de données

### Client — `customer_kyc_documents`

- `customer_id`, `shop_id`, `document_type`, `file_path`, `mime_type`, `status`
- `reviewed_by`, `reviewed_at`, `rejection_reason`

### Client — `customers`

- `kyc_level`, `kyc_validated_at`, `kyc_validated_by`

### Marchand — `shops`

- `kyc_status`, `kyc_submitted_at`, `kyc_verified_at`, `kyc_verified_by`
- `kyc_rejection_reason`, `kyc_submissions_count`, …

### Marchand — `shop_kyc_documents`

- Types business + index unique partiel sur (shop, type) pour pending/approved

Vues utiles (019) : `v_shops_kyc_pending`, `v_shops_kyc_stats`, …

---

## 7. API

### Client / marchand boutique (Customer KYC)

| Méthode | Endpoint | Qui |
|---------|----------|-----|
| `POST` | `/api/customers/kyc/upload` | Client |
| `GET` | `/api/customers/{id}/kyc/status` | Client |
| `GET` | `/api/merchant/kyc/pending` | Marchand (file clients) |
| `POST` | `/api/merchant/kyc/{customer_id}/review` | Marchand |

Headers : `Authorization` + `X-Shop-Slug`.

### Marchand plateforme (Merchant KYC)

| Méthode | Endpoint | Qui |
|---------|----------|-----|
| `POST` | `/api/merchant/kyc/submit` | Owner boutique |
| `GET` | `/api/merchant/kyc/status` | Owner |
| `GET` | `/api/admin/merchant-kyc/pending` | Admin |
| `PUT` / `POST` | `/api/admin/merchant-kyc/{shop_id}/review` | Admin |

---

## 8. Sécurité

| Règle | Application |
|-------|-------------|
| Isolation shop | Customer docs filtrés `shop_id` |
| RBAC | Review client = marchand ; review boutique = admin |
| Fichiers | Limite taille/MIME ; stockage non public |
| Audit | `reviewed_by` / timestamps / motif rejet |
| Pas de secrets dans les logs | zerolog filtré |

---

## 9. Intégrations (tontine, retraits)

### Tontine

Create / join / pay exigent client **`kyc_level = verified`**.  
Voir [11-tontine-system.md](11-tontine-system.md).

### Retraits & wallet (v5.1)

Un retrait peut être bloqué si :

1. Boutique **non** `kyc_status = verified` (gate métier / SQL), et/ou  
2. Wallet : **`debt_cents > 0`**, frozen, ou `available <= 0`  

Voir [12-wallet-debt-sweep.md](12-wallet-debt-sweep.md).

Les scripts E2E forcent parfois le KYC marchand pour isoler le test finance — ce n’est **pas** le parcours production.

---

## 10. Tests

```bash
go test ./application/usecase/customer_usecase/ -run KYC -v
go test ./application/usecase/merchant_kyc_usecase/ -v
go test ./tests/e2e/ -run TestTontine -v
```

Scénarios client : upload, limites taille/MIME, approve, rejet non verified sur tontine.  
Scénarios marchand : submit, status, admin review (tests unitaires package `merchant_kyc_usecase`).

---

## 11. Roadmap

| Phase | Contenu | Statut |
|-------|---------|--------|
| Customer KYC + tontine | v2.9 | ✅ |
| Merchant KYC + admin | v4.1 / 019 | ✅ |
| OCR / liveness / AML auto | — | 📋 |
| Notifications SMS statut KYC | partiel WS | 📋 |

---

## 📚 Références

- Migrations : `011_add_kyc.sql`, `019_add_merchant_kyc.sql`, `019_fix_kyc_pending_view.sql`
- [Tontine](11-tontine-system.md)
- [API Reference](03-api-reference.md)
- [Wallet / debt](12-wallet-debt-sweep.md)
- [Architecture](01-architecture.md)

---

**Dernière mise à jour** : 2026-10-05
```

