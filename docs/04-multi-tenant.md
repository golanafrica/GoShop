

```markdown
# 🏪 Stratégie Multi-tenant (GoShop)

**Version** : v4.5.0  
**Dernière mise à jour** : 2026-07-21  
**Statut** : ✅ Phase 1 (Isolation par `shop_id`) + Sécurisation IDOR active

---

## 📋 Table des matières

1. [Objectif](#1-objectif)
2. [Choix d'architecture](#2-choix-darchitecture)
3. [Implémentation actuelle (Phase 1)](#3-implémentation-actuelle-phase-1)
4. [Sécurité et Isolation (v4.5.0)](#4-sécurité-et-isolation-v450)
5. [Architecture des routes](#5-architecture-des-routes)
6. [Testabilité et Couverture](#6-testabilité-et-couverture)
7. [Migration vers la Phase 2](#7-migration-vers-la-phase-2)

---

## 1. Objectif

Permettre à chaque marchand d'avoir sa propre boutique isolée, avec son propre domaine personnalisé (custom domain), tout en partageant la même infrastructure backend et la même base de données PostgreSQL.

---

## 2. Choix d'architecture

Nous avons opté pour une **migration progressive** en deux phases :

### Phase 1 (Actuelle) : Isolation logique par colonne `shop_id`
- **Principe** : Toutes les tables métier (`products`, `customers`, `orders`, etc.) possèdent une colonne `shop_id` (UUID).
- **Filtrage** : Toutes les requêtes SQL sont automatiquement filtrées par ce `shop_id` via le contexte.
- **Avantages** : Simple, rapide à implémenter, requêtes SQL standard, peu coûteux en ressources.
- **Inconvénients** : Risque théorique de fuite de données (IDOR) si un filtre `WHERE shop_id = $1` est oublié dans un repository. *(Mitigé en v4.5.0, voir section 4)*.

### Phase 2 (Future) : Isolation physique par schéma PostgreSQL
- **Principe** : Chaque boutique possède son propre schéma PostgreSQL (ex: `tenant_ma_boutique`).
- **Avantages** : Isolation des données parfaite au niveau SGBD, sauvegardes et restaurations par client facilitées, conformité RGPD/BCEAO renforcée.
- **Inconvénients** : Migrations de schéma plus complexes, légère surcharge de gestion des connexions.

---

## 3. Implémentation actuelle (Phase 1)

### 3.1. Table `shops`
La table centrale qui définit le tenant. Le champ `db_schema` est预留 pour la Phase 2.

```sql
CREATE TABLE IF NOT EXISTS shops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    custom_domain VARCHAR(255) UNIQUE,
    owner_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    logo_url TEXT,
    theme JSONB DEFAULT '{}'::jsonb,
    plan VARCHAR(50) DEFAULT 'free', -- free, pro, business
    db_schema VARCHAR(100) UNIQUE,   -- NULL en Phase 1, rempli en Phase 2
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 3.2. Middleware `TenantResolver`
Ce middleware résout le tenant à partir de deux sources (dans l'ordre de priorité) et l'injecte dans le contexte :
1. Le header `X-Shop-Slug` (prioritaire pour les tests, Postman et les apps mobiles).
2. Le `Host` HTTP (pour les domaines personnalisés en production).

```go
// interfaces/middl/tenant.go
func TenantResolver(shopRepo repository.ShopRepository, logger *zerolog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            var shop *entity.Shop
            var err error

            // 1. Essayer le header X-Shop-Slug
            if slug := r.Header.Get("X-Shop-Slug"); slug != "" {
                shop, err = shopRepo.FindBySlug(r.Context(), slug)
            } else {
                // 2. Essayer le Host (domaine personnalisé)
                shop, err = shopRepo.FindByCustomDomain(r.Context(), r.Host)
            }

            if err != nil || shop == nil {
                utils.WriteError(w, http.StatusNotFound, "Shop not found")
                return
            }

            // Injecter le shop dans le contexte
            ctx := tenant.WithTenant(r.Context(), shop)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### 3.3. Contexte Multi-tenant
```go
// domain/tenant/context.go
type contextKey string
const tenantKey contextKey = "tenant"

func WithTenant(ctx context.Context, shop *entity.Shop) context.Context {
    return context.WithValue(ctx, tenantKey, shop)
}

func FromContext(ctx context.Context) (*entity.Shop, error) {
    shop, ok := ctx.Value(tenantKey).(*entity.Shop)
    if !ok || shop == nil {
        return nil, fmt.Errorf("no tenant in context")
    }
    return shop, nil
}
```

### 3.4. Pattern des Repositories
Tous les repositories métier extraient le `shop_id` du contexte pour l'injecter dans les requêtes SQL.

```go
// Exemple dans infrastructure/postgres/product/product_repositoryInfrastructure.go
func (pr *ProductRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
    shop, err := tenant.FromContext(ctx)
    if err != nil {
        return "", fmt.Errorf("multi-tenant: %w", err)
    }
    return shop.ID.String(), nil
}

func (pr *ProductRepositoryInfrastructure) FindAll(ctx context.Context, limit, offset int) ([]*entity.Product, error) {
    shopID, err := pr.getShopID(ctx)
    if err != nil {
        return nil, err
    }

    query := `SELECT id, name, description, price_cents, stock, created_at, updated_at 
              FROM products 
              WHERE shop_id = $1
              ORDER BY created_at DESC 
              LIMIT $2 OFFSET $3`

    rows, err := pr.queryContext(ctx, query, shopID, limit, offset)
    // ... scan et retour
}
```

---

## 4. Sécurité et Isolation (v4.5.0)

Pour éliminer tout risque de fuite de données (IDOR) où un utilisateur malveillant changerait le header `X-Shop-Slug` pour accéder aux données d'une autre boutique, nous avons ajouté une couche de sécurité supplémentaire.

### Middleware `RequireShopAccess`
Placé **immédiatement après** le `TenantResolver`, ce middleware vérifie que l'utilisateur authentifié (via son JWT) est soit le **propriétaire** (`owner_id`), soit un **collaborateur actif** de la boutique demandée.

```go
// interfaces/middl/shop_access.go
func RequireShopAccess(collabRepo repository.ShopCollaboratorRepository) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx := r.Context()
            
            shop, _ := tenant.FromContext(ctx)
            userID, _ := utils.GetUserID(ctx) // Extrait du JWT

            // 1. Vérifier si c'est le propriétaire
            if shop.OwnerID == userID {
                next.ServeHTTP(w, r)
                return
            }

            // 2. Sinon, vérifier si c'est un collaborateur actif
            _, err := collabRepo.FindByShopIDAndUserID(ctx, shop.ID, userID)
            if err != nil {
                utils.WriteError(w, http.StatusForbidden, "Accès refusé : vous n'avez pas les droits sur cette boutique")
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

### Règles de sécurité strictes
1. **Double vérification** : `TenantResolver` (quelle boutique ?) + `RequireShopAccess` (ai-je le droit d'y accéder ?).
2. **Requêtes paramétrées** : Aucune concaténation de chaîne dans les requêtes SQL pour éviter les injections.
3. **Contraintes de base de données** : Toutes les tables métier ont une contrainte `REFERENCES shops(id) ON DELETE CASCADE`.
4. **Indexation** : Des index sur `shop_id` garantissent que les filtres multi-tenants restent rapides (< 10ms) même avec des millions de lignes.

---

## 5. Architecture des routes

L'application Chi sépare clairement les routes de gestion des boutiques (qui n'ont pas besoin de contexte tenant) des routes métier (qui en ont absolument besoin).

```go
r.Route("/api", func(r chi.Router) {
    r.Use(middleware.AuthMiddleware) // Vérifie le JWT et injecte userID

    // 1. Routes de gestion des shops (SANS TenantResolver)
    r.Route("/shops", func(r chi.Router) {
        r.Post("/", shopHandler.CreateShop)
        r.Get("/", shopHandler.ListShops) // Liste les shops dont l'utilisateur est owner
        r.Put("/{id}", shopHandler.UpdateShop) // Vérifie l'ownership dans le usecase
    })

    // 2. Routes multi-tenant (AVEC double protection)
    r.Group(func(r chi.Router) {
        r.Use(middl.TenantResolver(shopRepo, logger))
        r.Use(middl.RequireShopAccess(shopCollabRepo)) // 🛡️ Sécurité v4.5.0

        r.Route("/products", productRoutes)
        r.Route("/customers", customerRoutes)
        r.Route("/orders", orderRoutes)
        r.Route("/payments", paymentRoutes)
        r.Route("/tontine", tontineRoutes)
    })
})
```

---

## 6. Testabilité et Couverture

Le design par interfaces permet un mocking facile pour tester l'isolation.

### Tests unitaires et d'intégration
- **`shop_usecase`** : 16 tests (Create/List/Update scenarios).
- **`order_usecase`** : 13 tests (dont 3 tests d'intégration vérifiant le filtrage par `shop_id`).
- **`customer_repository`** : Tests vérifiant que `FindAllCustomers` ne retourne que les clients du `shop_id` injecté dans le contexte.

### Tests E2E d'isolation
Le script `Test-E2E-GoShop.ps1` et les tests Go `TestCreateOrderE2E` valident systématiquement :
1. Création de `Shop A` et `Shop B`.
2. Création d'un produit dans `Shop A`.
3. Tentative de récupération du produit via l'API avec le header `X-Shop-Slug: shop-b`.
4. **Résultat attendu** : `404 Not Found` (le produit n'existe pas dans le contexte de `Shop B`).

---

## 7. Migration vers la Phase 2

Lorsque le volume de données ou les exigences de conformité nécessiteront la Phase 2 (schémas PostgreSQL), la transition sera transparente pour l'application grâce à l'abstraction des repositories.

**Étapes prévues :**
1. **Remplir le champ `db_schema`** : `UPDATE shops SET db_schema = 'tenant_' || replace(slug, '-', '_');`
2. **Créer les schémas** : Script Go ou PL/pgSQL pour créer le schéma et y copier les tables (`CREATE SCHEMA IF NOT EXISTS ...`).
3. **Bascule dans le conteneur d'injection de dépendances (DI)** :
   ```go
   if shop.DBSchema != nil && *shop.DBSchema != "" {
       // Utiliser l'implémentation Schema-aware (ex: productRepoSchema)
       // qui préfixe les tables : fmt.Sprintf("%s.products", shop.DBSchema)
   } else {
       // Fallback sur l'implémentation Phase 1 (shop_id)
   }
   ```

---

## 📚 Références
- [Architecture Decision Record : Multi-tenant](01-architecture.md)
- [Migration 002 : Multi-tenant](../migrations/002_multi_tenant.sql)
- [Tests E2E d'isolation](../tests/e2e/)
```

---

