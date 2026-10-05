```markdown
# 🏪 Stratégie Multi-tenant (GoShop)

**Version** : v5.1.0  
**Dernière mise à jour** : 2026-10-05  
**Statut** : ✅ Phase 1 (isolation par `shop_id`) + résolution tenant + contrôle d’accès owner/collaborateur

---

## 📋 Table des matières

1. [Objectif](#1-objectif)
2. [Choix d'architecture](#2-choix-darchitecture)
3. [Implémentation actuelle (Phase 1)](#3-implémentation-actuelle-phase-1)
4. [Sécurité et isolation](#4-sécurité-et-isolation)
5. [Architecture des routes](#5-architecture-des-routes)
6. [Finance & multi-tenant](#6-finance--multi-tenant)
7. [Testabilité](#7-testabilité)
8. [Migration vers la Phase 2](#8-migration-vers-la-phase-2)

---

## 1. Objectif

Chaque marchand dispose d’une **boutique isolée** (slug, domaine optionnel, données métier), sur **la même** application et **la même** base PostgreSQL.

---

## 2. Choix d'architecture

### Phase 1 (actuelle) — isolation logique par `shop_id`
- Tables métier : colonne `shop_id` (UUID → `shops.id`).
- Filtrage applicatif via le **contexte tenant** injecté par middleware.
- **Avantages** : simple, SQL standard, faible coût ops.
- **Risque** : oubli d’un `WHERE shop_id = $1` → mitigé par middleware d’accès + reviews / tests.

### Phase 2 (future) — isolation physique par schéma
- Un schéma PostgreSQL par boutique (`db_schema` sur `shops`).
- Meilleure isolation / backup par client ; migrations plus complexes.
- Champ `shops.db_schema` **réservé** dès la migration `002` (NULL en Phase 1).

---

## 3. Implémentation actuelle (Phase 1)

### 3.1. Table `shops` (extrait migration `002`)

```sql
CREATE TABLE IF NOT EXISTS shops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    custom_domain VARCHAR(255) UNIQUE,
    owner_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    logo_url TEXT,
    theme JSONB DEFAULT '{}'::jsonb,
    plan VARCHAR(50) DEFAULT 'free',
    db_schema VARCHAR(100) UNIQUE,   -- Phase 2
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

Autres tables (`products`, `customers`, `orders`, …) reçoivent `shop_id` + index.

### 3.2. Middleware `TenantResolver`

Fichier : `interfaces/middl/tenant.go` (logique réelle sur `main`).

**Signature :**
```go
func TenantResolver(
    shopRepo repository.ShopRepository,
    shopCollabRepo repository.ShopCollaboratorRepository,
    logger zerolog.Logger,
) func(http.Handler) http.Handler
```

**Ordre de résolution du shop :**
1. Header **`X-Shop-Slug`** (tests, Postman, mobile, E2E)
2. **`Host`** → `custom_domain`
3. **Sous-domaine** → premier label comme slug (`demo.golanafrica.com` → `demo`)

**Contrôles supplémentaires dans le même middleware :**
- Boutique **introuvable** → 404  
- Boutique **inactive** (`!IsActive`) → 403  
- Utilisateur **non authentifié** → 401  
- Utilisateur **ni owner ni collaborateur** → 403 (anti spoofing de `X-Shop-Slug`)  
- Succès → `tenant.WithTenant(ctx, shop)`

### 3.3. Contexte (`domain/tenant/context.go`)

```go
func WithTenant(ctx context.Context, shop *entity.Shop) context.Context
func FromContext(ctx context.Context) (*entity.Shop, error)  // ErrNoTenant
func GetID(ctx context.Context) (string, error)
```

Les repositories / usecases lisent le tenant via `FromContext` / `GetID` et filtrent par `shop_id`.

### 3.4. Pattern repository

Toute lecture/écriture métier côté marchand doit être scopée :

```sql
SELECT ... FROM products WHERE shop_id = $1 ...
```

`shop_id` issu du contexte, **jamais** d’un body client non validé.

### 3.5. `RequireShopAccess` (fichier legacy)

`interfaces/middl/shop_access.go` vérifie encore owner / collab **si** le tenant est déjà dans le contexte.

Sur `main`, **`TenantResolver` intègre déjà cette autorisation**.  
Selon le montage des routes dans `internal/app`, `RequireShopAccess` peut être **redondant** — ne pas le documenter comme seule barrière : la source de vérité actuelle est **`TenantResolver`**.

---

## 4. Sécurité et isolation

| Couche | Rôle |
|--------|------|
| Auth JWT | Identifie `user_id` |
| `TenantResolver` | Choisit le shop + vérifie owner/collab + `is_active` |
| Repositories | `WHERE shop_id = …` |
| FK `ON DELETE CASCADE` | Cohérence référentielle |
| Index `shop_id` | Perf des filtres tenant |

**Règles :**
1. Changer `X-Shop-Slug` vers une boutique dont on n’est pas owner/collab → **403**.
2. SQL paramétré uniquement (pas de concat de slug / id).
3. Routes **admin** : souvent **cross-tenant** (rôle `super_admin` / `admin`), hors groupe « shop courant ».
4. **Webhooks** providers : **sans** `X-Shop-Slug` (identif. via payment / signature HMAC).
5. **Catalogue public** / health : hors isolation marchand stricte.

---

## 5. Architecture des routes (schéma)

```text
/api/shops              → gestion boutiques (auth, pas de TenantResolver obligatoire)
/api/... métier         → Auth + TenantResolver
  products, customers, orders, payments
  tontine, wallet, withdrawals, installments, …
/api/admin/...          → Auth + rôles admin (vue multi-boutiques)
/webhooks/{provider}    → public + HMAC
/health/*, /metrics     → infra
/api/public/...         → catalogue public
```

Exemple de montage conceptuel :

```go
r.Route("/api", func(r chi.Router) {
    r.Use(authMiddleware)

    r.Route("/shops", shopRoutes) // create/list/update ownership dans usecase

    r.Group(func(r chi.Router) {
        r.Use(middl.TenantResolver(shopRepo, shopCollabRepo, logger))
        // routes métier scopées shop
    })

    r.Route("/admin", adminRoutes) // RBAC admin
})
```

*(Le détail exact des groupes est dans le wiring `internal/app` + handlers.)*

---

## 6. Finance & multi-tenant

La finance reste **par boutique** :

| Objet | Isolation |
|-------|-----------|
| `merchant_wallets` | PK / clé = **`shop_id`** |
| `wallet_transactions` | liées au wallet du shop |
| `debt_cents`, held, freeze | état **du** marchand de ce shop |
| Retraits | shop courant + règles debt / KYC |
| Escrow / disputes | liés à `order` → `shop_id` |

Un marchand **ne peut pas** créditer / débiter / retirer le wallet d’un autre shop via spoofing de header (bloqué par `TenantResolver`).

Doc métier : [12-wallet-debt-sweep.md](12-wallet-debt-sweep.md).

---

## 7. Testabilité

- Unit : `interfaces/middl/tenant_test.go`
- E2E : création Shop A / Shop B, produit sur A, requête avec `X-Shop-Slug: B` → **404/403**
- E2E finance (PowerShell) : chaque run crée un **shop dédié** + header slug → isolation de fait

Voir aussi [08-testing-guide.md](08-testing-guide.md).

---

## 8. Migration vers la Phase 2

Quand volume / conformité l’exigeront :

1. Renseigner `shops.db_schema` (`tenant_<slug_normalisé>`).
2. Créer schémas + tables (script ops).
3. Brancher des repos « schema-aware » dans le DI **sans** casser Phase 1 (`db_schema` NULL → comportement actuel).

---

## 📚 Références

- Code : `interfaces/middl/tenant.go`, `interfaces/middl/shop_access.go`, `domain/tenant/context.go`
- Migration : [`migrations/002_multi_tenant.sql`](../migrations/002_multi_tenant.sql)
- [Architecture](01-architecture.md) · [API](03-api-reference.md) · [Wallet debt](12-wallet-debt-sweep.md)
- Diagramme : `docs/diagrams/multi-tenant-strategy.puml`

---

**Dernière mise à jour** : 2026-10-05
```
