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
