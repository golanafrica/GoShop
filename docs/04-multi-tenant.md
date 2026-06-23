# Stratégie Multi-tenant

## Objectif
Permettre à chaque marchand d'avoir sa propre boutique isolée, avec son propre domaine (custom), tout en partageant la même infrastructure.

## Choix d'architecture : Option B → A (Migration progressive)

### Phase 1 (Actuelle) - Option B : Colonne shop_id
- **Principe** : Toutes les tables ont une colonne `shop_id` (UUID).
- **Filtrage** : Toutes les requêtes SQL sont filtrées par `shop_id`.
- **Middleware** : Chi résout le `shop_id` depuis le `Host` HTTP et le place dans le contexte.
- **Avantages** : Simple, rapide à implémenter, peu coûteux.
- **Inconvénients** : Risque de fuite de données si un filtre est oublié.

### Phase 2 (Futur) - Option A : Schéma PostgreSQL par tenant
- **Principe** : Chaque shop a son propre schéma (`tenant_<slug>`).
- **Migration** : Sans changer l'interface des repositories, on change l'implémentation.
- **Avantages** : Isolation parfaite, sauvegardes par client.
- **Inconvénients** : Migrations plus complexes, surcharge minimale.

## Implémentation en Go

### 1. Table `shops` (avec champ `db_schema` pour préparer A)
```sql
CREATE TABLE shops (
    id UUID PRIMARY KEY,
    name VARCHAR(255),
    slug VARCHAR(100) UNIQUE,
    custom_domain VARCHAR(255) UNIQUE,
    owner_id UUID REFERENCES users(id),
    db_schema VARCHAR(100) UNIQUE, -- NULL en Phase 1, rempli en Phase 2
    is_active BOOLEAN DEFAULT true
);


2. Middleware Tenant (Chi)
go
func TenantResolver(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        host := r.Host
        shop, err := resolveTenantByDomain(host) // FROM shops WHERE custom_domain = host
        if err != nil {
            http.NotFound(w, r)
            return
        }
        ctx := context.WithValue(r.Context(), "tenant", shop)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
3. Repository Product (Phase 1 - Option B)
go
func (r *productRepo) FindAll(ctx context.Context) ([]Product, error) {
    shop := tenant.FromContext(ctx)
    rows, err := r.db.QueryContext(ctx,
        `SELECT * FROM products WHERE shop_id = $1`, shop.ID)
    // ...
}
4. Migration vers Phase 2
On remplit le champ db_schema pour chaque shop.

On crée une nouvelle implémentation productRepoSchema qui utilise %s.products.

Dans le DI container, on bascule selon shop.DBSchema != nil.

text

---

### 05-payment-system.md (Système de paiement)

```markdown
# Système de paiement

## Vue d'ensemble
Système modulaire supportant plusieurs modes de paiement :
1. **Cash à la livraison** (MVP prioritaire).
2. **Mobile Money** : Wave, Orange Money, MTN MoMo.
3. **Paiement par tranches (crédit)** : Acompte + échéances.

## 1. Cash à la livraison

### Flux complet
1. **Client** commande en ligne, choisit "Cash à la livraison".
2. **Système** : Crée commande en `pending_confirmation`, **stock réservé 24h**.
3. **Marchand** reçoit notification (dashboard + SMS/WhatsApp).
4. **Marchand** accepte ou refuse la commande (depuis dashboard).
5. **Système** : Si acceptée, commande passe en `confirmed`, client notifié.
6. **Livreur** collecte les espèces, marque `delivered` via app.
7. **Commande clôturée**, score client mis à jour.

### États d'une commande cash
- `pending_confirmation` → `confirmed` → `out_for_delivery` → `delivered`.
- Ou `cancelled`, `expired`.

### Règles configurables par le marchand
- Délai de réservation stock (6h à 72h).
- Zones de livraison.
- Montant minimum de commande.
- Score client minimum pour éligibilité.
- Rappel automatique J-1.

## 2. Mobile Money

### Architecture (Provider Pattern)
```go
type Provider interface {
    Code() string
    InitiatePayment(ctx context.Context, req *PaymentRequest) (*PaymentResponse, error)
    CheckStatus(ctx context.Context, providerRef string) (*PaymentStatus, error)
    ValidateWebhook(payload []byte, signature string) (*WebhookEvent, error)
}
Registre de providers
wave, orange_money, mtn_momo.

Chaque marchand active les providers souhaités (shop_payment_methods).

Les clés API sont chiffrées en base.

Webhooks
Endpoint unique : POST /webhooks/{provider}.

Validation de signature, mise à jour du statut de la transaction.

Notification au marchand et au client.

3. Crédit (paiement en tranches)
Plans de paiement (définis par le marchand)
Exemple : "3 fois sans frais" → 30% acompte, 3 tranches, 30 jours.

Le marchand paramètre : down_payment_pct, installments_count, interval_days, interest_rate_pct.

Calcul des tranches
go
downPayment = orderAmount * downPaymentPct / 100
remaining = orderAmount - downPayment
baseInstallment = remaining / installmentsCount
// Création des échéances à J+interval_days, J+2*interval_days...
Score de fiabilité client
Excellente : Accès à tous les plans.

Bonne : Accès aux plans standards.

Passable : Acompte plus élevé.

Mauvaise : Uniquement paiement comptant.

Rappels automatiques
48h avant échéance, 24h avant, 1h après.

Notification par SMS/WhatsApp.

text

---

### 06-deployment.md (Déploiement)

```markdown
# Déploiement

## Environnements
- **Développement** : Docker Compose (local).
- **Production** : Kubernetes (Minikube ou cloud).

## Docker Compose (Dev)
```bash
docker-compose up --build
Services : API (8080), PostgreSQL (5432), Redis (6379), Prometheus (9090), Loki, Grafana.

Kubernetes (Production)
Manifests dans /k8s.

Liveness/Readiness probes.

ConfigMaps, Secrets.

Ingress pour le routage.

Variables d'environnement
Variable	Description
APP_ENV	development / production
LOG_LEVEL	debug / info / warn / error
DB_HOST, DB_USER, DB_PASSWORD, DB_NAME	PostgreSQL
REDIS_HOST	Redis
JWT_SECRET, REFRESH_SECRET	Secrets JWT
WAVE_API_KEY	Wave (si activé)
ORANGE_MERCHANT_CODE	Orange Money
CI/CD
GitHub Actions.

Build de l'image Docker.

Push vers GHCR.

Déploiement sur Kubernetes (via kubectl/helm).

text

---

### 07-contributing.md (Guide contributeurs)

```markdown
# Contribuer à GoShop

## Comment contribuer ?
1. **Fork** le projet.
2. **Créer une branche** `feature/ma-fonctionnalite`.
3. **Écrire des tests** pour votre code.
4. **Commiter** avec des messages clairs.
5. **Ouvrir une Pull Request** vers `develop`.

## Convention de code (Go)
- Suivre les standards `gofmt` et `go vet`.
- Documenter les fonctions publiques.
- Utiliser `zerolog` pour les logs structurés.

## Style des commits
- `feat:` Nouvelle fonctionnalité.
- `fix:` Correction de bug.
- `docs:` Documentation.
- `test:` Ajout de tests.
- `chore:` Maintenance.

## Tests requis
- **Unitaires** : `go test ./... -v`.
- **Intégration** : `go test -tags=integration ./... -v`.
- **E2E** : `go test -tags=e2e ./tests/e2e/... -v`.

## Environnement de développement
- Installer Go 1.25+.
- Docker & Docker Compose.
- (Optionnel) Minikube.

## Relecture de code
- Respect de l'architecture (Domain → Application → Interfaces → Infrastructure).
- Pas de fuite de `shop_id`.
- Toutes les nouvelles routes documentées.
# Stratégie Multi-tenant

## Objectif
Permettre à chaque marchand d'avoir sa propre boutique isolée, avec son propre domaine (custom), tout en partageant la même infrastructure.

## Choix d'architecture : Option B → A (Migration progressive)

### Phase 1 (Actuelle) - Option B : Colonne `shop_id`
- **Principe** : Toutes les tables ont une colonne `shop_id` (UUID).
- **Filtrage** : Toutes les requêtes SQL sont filtrées par `shop_id`.
- **Middleware** : Chi résout le `shop_id` depuis le header `X-Shop-Slug` ou le `Host` HTTP et le place dans le contexte.
- **Avantages** : Simple, rapide à implémenter, peu coûteux.
- **Inconvénients** : Risque de fuite de données si un filtre est oublié.

### Phase 2 (Futur) - Option A : Schéma PostgreSQL par tenant
- **Principe** : Chaque shop a son propre schéma (`tenant_<slug>`).
- **Migration** : Sans changer l'interface des repositories, on change l'implémentation.
- **Avantages** : Isolation parfaite, sauvegardes par client.
- **Inconvénients** : Migrations plus complexes, surcharge minimale.

---

## Implémentation actuelle (Phase 1)

### 1. Table `shops`

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

2. Table shop_payment_settings

CREATE TABLE IF NOT EXISTS shop_payment_settings (
    shop_id UUID PRIMARY KEY REFERENCES shops(id) ON DELETE CASCADE,
    cash_enabled BOOLEAN DEFAULT true,
    cash_reservation_hours INTEGER DEFAULT 24,
    cash_min_amount BIGINT DEFAULT 10000,
    cash_delivery_zones JSONB DEFAULT '[]'::jsonb,
    cash_requires_approval BOOLEAN DEFAULT true,
    wave_enabled BOOLEAN DEFAULT false,
    wave_api_key TEXT,
    orange_money_enabled BOOLEAN DEFAULT false,
    orange_merchant_code TEXT,
    orange_money_api_key TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

3. Tables avec shop_id

ALTER TABLE products ADD COLUMN IF NOT EXISTS shop_id UUID REFERENCES shops(id) ON DELETE CASCADE;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS shop_id UUID REFERENCES shops(id) ON DELETE CASCADE;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS shop_id UUID REFERENCES shops(id) ON DELETE CASCADE;

-- Index pour la performance
CREATE INDEX IF NOT EXISTS idx_products_shop ON products(shop_id);
CREATE INDEX IF NOT EXISTS idx_customers_shop ON customers(shop_id);
CREATE INDEX IF NOT EXISTS idx_orders_shop ON orders(shop_id);

Middleware TenantResolver
Le middleware résout le tenant depuis deux sources (dans l'ordre) :
Header X-Shop-Slug (prioritaire pour les tests et APIs)
Host HTTP (pour les domaines personnalisés en production)
go

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
                http.Error(w, "Shop not found", http.StatusNotFound)
                return
            }

            // Injecter le shop dans le contexte
            ctx := tenant.WithTenant(r.Context(), shop)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}


Contexte Multi-tenant

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

Repositories Multi-tenant
Tous les repositories extraient le shop_id du contexte et filtrent les requêtes.
Pattern commun
go

func (r *Repository) getShopID(ctx context.Context) (string, error) {
    shop, err := tenant.FromContext(ctx)
    if err != nil {
        return "", fmt.Errorf("multi-tenant: %w", err)
    }
    return shop.ID.String(), nil
}

Exemple : ProductRepository

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
    // ...
}

Repositories implémentés
Repository
Méthodes multi-tenant
ProductRepository
Create, FindByID, FindAll, Update, Delete
CustomerRepository
Create, FindByCustomerID, FindAllCustomers, UpdateCustomer, DeleteCustomer, FindByEmail, CountAllCustomers, FindAllCustomersWithPagination, FindAllCustomersWithSorting
OrderRepository
Create, FindByID, FindAll, CountByCustomerID, CountAll, FindAllWithPagination
OrderItemRepository
Create (vérifie parent), FindByID (JOIN), FindAll (JOIN)
Endpoints de gestion des shops
Routes

Exemple d'utilisation

# Créer une boutique
curl -X POST http://localhost:8080/api/shops \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Ma Boutique", "slug": "ma-boutique"}'

# Lister mes boutiques
curl http://localhost:8080/api/shops \
  -H "Authorization: Bearer $TOKEN"

# Utiliser une boutique comme tenant
curl http://localhost:8080/api/products \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Shop-Slug: ma-boutique"

  Architecture des routes

  r.Route("/api", func(r chi.Router) {
    r.Use(middleware.AuthMiddleware)

    // Routes de gestion des shops (SANS TenantResolver)
    r.Route("/shops", func(r chi.Router) {
        r.Post("/", shopHandler.CreateShop)
        r.Get("/", shopHandler.ListShops)
        r.Put("/{id}", shopHandler.UpdateShop)
    })

    // Routes multi-tenant (AVEC TenantResolver)
    r.Group(func(r chi.Router) {
        r.Use(middl.TenantResolver(shopRepo, logger))

        r.Route("/products", productRoutes)
        r.Route("/customers", customerRoutes)
        r.Route("/orders", orderRoutes)
    })
})

Testabilité
Le ShopHandler utilise des interfaces pour permettre le mocking :

type CreateShopUseCaseInterface interface {
    Execute(ctx context.Context, name, slug, customDomain string) (*entity.Shop, error)
}

type ListShopsUseCaseInterface interface {
    Execute(ctx context.Context) ([]*entity.Shop, error)
}

type UpdateShopUseCaseInterface interface {
    Execute(ctx context.Context, shopID string, name *string, ...) (*entity.Shop, error)
}

Couverture de tests
Tests unitaires (130+ tests)
Package
Tests
shop_usecase
16 tests (Create/List/Update)
shop_handler
9 tests (Create/List/Update scenarios)
order_usecase
13 tests (dont 3 intégration)
order_repository
3 tests
Tests E2E
TestAuthFlowE2E : Inscription → Connexion → Profil
TestCreateOrderE2E : Création shop → Customer → Products → Order (avec multi-tenant)
TestSecurityHeaders, TestCORS, TestPublicEndpoints
Tests de charge
TestLoadSmoke : 5s avec UUID-based emails
TestLoadAuth : 30s avec 10 VUs
Vérification manuelle

# Isolation produits
curl -H "X-Shop-Slug: demo" .../api/products      # 50 produits
curl -H "X-Shop-Slug: shop2" .../api/products     # 1 produit

# Isolation customers
curl -H "X-Shop-Slug: demo" .../api/customers     # 51 customers
curl -H "X-Shop-Slug: shop2" .../api/customers    # 1 customer

Migration vers Phase 2
Étape 1 : Remplir db_schema

UPDATE shops SET db_schema = 'tenant_' || replace(slug, '-', '_');

Étape 2 : Créer les schémas

DO $$ 
DECLARE shop_rec RECORD;
BEGIN
    FOR shop_rec IN SELECT id, db_schema FROM shops WHERE db_schema IS NOT NULL
    LOOP
        EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', shop_rec.db_schema);
        -- Copier les tables dans le schéma
    END LOOP;
END $$;

tape 3 : Bascule dans le DI

if shop.DBSchema != nil {
    // Utiliser productRepoSchema
} else {
    // Utiliser productRepoShopID (Phase 1)
}

Sécurité
Règles
Toutes les requêtes vers /api/* (sauf /api/shops) passent par TenantResolver
Toutes les requêtes SQL incluent WHERE shop_id = $X
OrderItemRepository vérifie que la commande parente appartient au shop
Mise à jour de shop : vérification du propriétaire
Risques et mitigations
Risque
Mitigation
Oubli de filtre shop_id
Revue de code + tests E2E
Fuite via JOIN
Index + monitoring
Accès cross-tenant
Tests manuels d'isolation
Injection SQL
Requêtes paramétrées
Références
Architecture Decision Record : Multi-tenant
Migration 002
Tests E2E

