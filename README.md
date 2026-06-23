# 🛒 GoShop - Plateforme E-commerce SaaS Multi-tenant pour l'Afrique

**Vision** : Devenir le Shopify africain avec paiements Mobile Money intégrés (Wave, Orange Money, Moov Money), vente à crédit et système de tontine.

**Statut** : ✅ **v2.1.0-payment-system** - Système de paiement complet avec Orange Money + Moov Money, 130+ tests passent

---

## 🎯 Nouveautés v2.1.0

### 💳 Système de paiement complet
Intégration complète de Mobile Money pour l'Afrique de l'Ouest :

#### Providers supportés
- **Orange Money** : USSD `#144*111#`, délai 2s
- **Moov Money** : USSD `#135*2#`, délai 3s
- **Architecture extensible** : Ajout facile de Wave, MTN MoMo, etc.

#### Flux de paiement

pending → processing → success → refunded
→ failed
→ expired
→ cancelled


#### Sécurité
- **Validation HMAC-SHA256** des webhooks (secrets séparés par provider)
- **Protection contre le payload tampering**
- **Isolation multi-tenant** vérifiée (cross-tenant = 404)
- **Injection du tenant context** pour les webhooks publics

#### Endpoints de paiement
- `POST /api/orders/{id}/pay 🔒` - Initier un paiement
- `GET /api/payments 🔒` - Lister les paiements
- `GET /api/payments/{id} 🔒` - Détails d'un paiement
- `POST /api/payments/{id}/refund 🔒` - Rembourser
- `POST /webhooks/{provider}` - Webhook provider (public)

#### Mise à jour automatique
- **Statut commande** : `PENDING` → `PAID` automatiquement après paiement réussi
- **Timestamps UTC** : Cohérence garantie pour l'audit
- **Audit trail** : Table `payment_webhooks` pour traçabilité

### 🐛 Corrections critiques
- **Webhook** : Injection du tenant context pour persister correctement
- **Timestamps** : UTC forcé partout (PostgreSQL + Go)
- **Tests** : Filtrage par CustomerID pour isolation des tests

---

## 🎯 Nouveautés v2.0.0

### 🏪 Multi-tenant complet
Chaque marchand dispose de sa propre boutique isolée :
- **Isolation des données** : produits, clients, commandes filtrés par `shop_id`
- **Header `X-Shop-Slug`** : sélection de la boutique active
- **Domaine personnalisé** : support des sous-domaines (futur)
- **Gestion des boutiques** : créer, lister, modifier via `/api/shops`

### 📊 Statistiques
- **130+ tests** qui passent (unit, intégration, E2E, load)
- **100% de réussite** sur les tests de charge
- **0 deadlock** grâce à `sync.Once` pour les migrations

---

## 🚀 Démarrage rapide

### Prérequis
- Go 1.23+
- PostgreSQL 16
- Redis 7
- k6 (optionnel, pour tests de charge)

### Lancement
```bash
# Configuration
cp .env.example .env
# Éditer .env avec vos valeurs

# Lancer PostgreSQL et Redis (localement ou Docker)

# Lancer l'API
go run cmd/api/main.go

# API disponible sur : http://localhost:8080 (ou port configuré)

Accès & Endpoints techniques
Service
Endpoint
API
http://localhost:8080
Liveness
GET /health/live
Readiness
GET /health/ready
Metrics
GET /metrics
Swagger UI
GET /swagger/index.html
📈 Routes API
Authentification
POST /register - Inscription
POST /login - Connexion
POST /auth/refresh - Rafraîchir le token
GET /auth/me 🔒 - Profil utilisateur
Gestion des boutiques (Multi-tenant)
POST /api/shops 🔒 - Créer une boutique
GET /api/shops 🔒 - Lister mes boutiques
PUT /api/shops/{id} 🔒 - Modifier une boutique
💳 Paiements (NOUVEAU v2.1.0)
POST /api/orders/{id}/pay 🔒 - Initier un paiement
GET /api/payments 🔒 - Lister les paiements
GET /api/payments/{id} 🔒 - Détails d'un paiement
POST /api/payments/{id}/refund 🔒 - Rembourser
POST /webhooks/{provider} - Webhook provider (public, HMAC validé)
API protégée (nécessite X-Shop-Slug)
Customers : GET | POST | PUT | DELETE /api/customers
Products : GET | POST | PUT | DELETE /api/products
Orders : GET | POST /api/orders
Endpoints publics
GET /health/live
GET /health/ready
GET /help
💳 Utilisation du système de paiement
Exemple : Initier un paiement Orange Money

curl -X POST http://localhost:8080/api/orders/{order_id}/pay \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Shop-Slug: ma-boutique" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "orange_money",
    "phone_number": "+22670123456",
    "description": "Paiement commande #123"
  }'

  Réponse :

  {
  "payment_id": "57f92b94-5187-4b84-84f4-a31081de634a",
  "provider_ref": "MV-bb5a95c54010a058",
  "status": "processing",
  "ussd_code": "#135*2#MV-bb5a95c54010a058#",
  "message": "Composez #135*2*MV-bb5a95c54010a058# pour valider le paiement de 20000 FCFA via Moov Money"
}

Exemple : Webhook HMAC

# Générer la signature HMAC-SHA256
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | cut -d' ' -f2)

curl -X POST http://localhost:8080/webhooks/orange_money \
  -H "Content-Type: application/json" \
  -H "X-Signature: $SIGNATURE" \
  -d "$PAYLOAD"

  Providers disponibles
Provider
Code
USSD
Délai mock
Orange Money
orange_money
#144*111#
2s
Moov Money
moov_money
#135*2#
3s
🏪 Utilisation Multi-tenant
Exemple : Créer une boutique

curl -X POST http://localhost:8080/api/shops \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Ma Boutique", "slug": "ma-boutique"}'

  Exemple : Utiliser une boutique


# Lister les produits de "ma-boutique"
curl http://localhost:8080/api/products \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Shop-Slug: ma-boutique"

  Isolation garantie
Les produits créés dans ma-boutique ne sont pas visibles dans demo
Les clients sont isolés par boutique
Les commandes sont liées à leur boutique
Les paiements sont isolés par boutique
🧪 Tests
Tests unitaires & intégration

go test ./... -v

Tests End-to-End (E2E)

go test ./tests/e2e/... -v

Tests de paiement (NOUVEAU v2.1.0)

go test ./tests/e2e/... -v -run TestPaymentFlowE2E

Tests de sécurité (NOUVEAU v2.1.0)

# Tests manuels d'isolation multi-tenant + HMAC webhook
# Voir docs/payment-system.md pour les scripts PowerShell

Tests de charge (k6)

k6 version
go test ./tests/loadtest/... -v

Scénarios E2E couverts
✅ Authentification : inscription → connexion → accès profil
✅ Multi-tenant : création shop → customers → products → orders
✅ Isolation : vérification cross-shop
✅ Sécurité : routes publiques / protégées, CORS, headers
✅ Paiement : initiation → success → refund (NOUVEAU)
✅ Webhook HMAC : validation + rejet payload tamperé (NOUVEAU)
📊 Observabilité
Logs structurés
Format JSON (zerolog)
Niveaux dynamiques : debug, info, warn, error
Request ID pour corrélation des logs
Audit des connexions (emails masqués)
Compatible Loki / Grafana
Métriques Prometheus
orders_created_total
order_revenue_cents_total
products_created_total
auth_login_total
auth_login_failed_total
payments_initiated_total (NOUVEAU)
payments_success_total (NOUVEAU)
payments_refunded_total (NOUVEAU)
webhooks_received_total (NOUVEAU)
Latence HTTP par endpoint
📍 Exposées via : GET /metrics
❤️ Health Checks
Endpoint
Description
/health/live
Serveur actif
/health/ready
DB + Redis opérationnels
➡️ Prêt pour livenessProbe et readinessProbe Kubernetes.
🔒 Sécurité
✅ Authentification JWT (access + refresh tokens)
✅ Hash des mots de passe (bcrypt)
✅ Headers HTTP de sécurité
✅ CORS configurable
✅ Rate limiting
✅ Middleware de recovery (pas de crash serveur)
✅ Requêtes SQL paramétrées
✅ Secrets via variables d'environnement
✅ Conteneurs Docker en non-root
✅ Isolation multi-tenant par shop_id
✅ Validation HMAC-SHA256 des webhooks (NOUVEAU)
✅ Protection contre payload tampering (NOUVEAU)
✅ Timestamps UTC cohérents pour l'audit (NOUVEAU)
🛠️ Architecture
Clean Architecture / DDD


├── cmd/api              # Point d'entrée
├── internal/app         # Initialisation application
├── domain               # Entités métier & interfaces
│   ├── entity           # Shop, Product, Customer, Order, Payment
│   ├── repository       # Interfaces des repositories
│   └── tenant           # Contexte multi-tenant
├── application          # Use cases & DTOs
│   └── usecase/
│       └── payment_usecase/  # 5 usecases paiement (NOUVEAU)
├── interfaces           # Handlers HTTP & middlewares
│   ├── handler/
│   │   └── payment_handler/  # PaymentHandler + WebhookHandler (NOUVEAU)
│   ├── middl            # Middlewares (TenantResolver, Auth)
│   └── utils            # Utilitaires (erreurs, JWT)
├── infrastructure       # PostgreSQL, Redis
│   └── payment/         # Registry + Providers (NOUVEAU)
│       └── mock/        # Orange Money + Moov Money (NOUVEAU)
├── config               # Configuration & logging
├── tests                # Unit, E2E, load
└── migrations           # Migrations SQL (001-004)

Stack technique
Go 1.23
Chi (router HTTP v5)
PostgreSQL 16
Redis 7
Prometheus
Zerolog
Docker multi-stage (Alpine)
Kubernetes (Minikube)
🗄️ Base de données
Migrations
001_init : Tables de base (users, products, customers, orders)
002_multi_tenant : Tables shops, shop_payment_settings
003_payment_system : Tables payments, payment_webhooks (NOUVEAU)
004_add_indexes : Indexes de performance (NOUVEAU)
Tables principales
users : Utilisateurs authentifiés
shops : Boutiques multi-tenant
products : Produits par shop
customers : Clients par shop
orders : Commandes par shop
order_items : Items de commande
payments : Paiements avec machine à états (NOUVEAU)
payment_webhooks : Audit trail des webhooks (NOUVEAU)
🗺️ Roadmap
✅ Phase 1 : Multi-tenant (COMPLÉTÉ - v2.0.0)
✅ Boutiques indépendantes avec isolation par shop_id
✅ Middleware TenantResolver (X-Shop-Slug + Host)
✅ Endpoints de gestion des boutiques (/api/shops)
✅ Repositories multi-tenant
✅ 130+ tests passent
✅ Phase 2 : Paiements avancés (EN COURS - v2.1.0)
✅ Orange Money (mock, USSD #144*111#)
✅ Moov Money (mock, USSD #135*2#)
✅ Validation HMAC-SHA256 des webhooks
✅ Machine à états pour les paiements
✅ Auto-update du statut commande (PENDING → PAID)
✅ Remboursements complets
✅ Tests de sécurité (isolation + HMAC)
🚧 Cash à la livraison avec workflow complet
🚧 Wave provider (prochain)
🚧 Intégration avec vrais APIs Orange Money / Moov Money
🚧 Notifications SMS des paiements
🚧 Phase 3 : Crédit
🚧 Plans de paiement en tranches
🚧 Score de fiabilité client
🚧 Rappels automatiques
🚧 Phase 4 : Tontine
🚧 Système d'épargne collective
🚧 Catégories thématiques (Moto, Voiture, Ciment, etc.)
🚧 Retraits pour payer les commandes
📚 Documentation
Architecture
Modèle de données
API Reference
Multi-tenant
Système de paiement (NOUVEAU)
Déploiement
Contribuer
🤝 Contribuer
Voir CONTRIBUTING.md pour les détails.
Convention de commits
feat: Nouvelle fonctionnalité
fix: Correction de bug
docs: Documentation
test: Ajout de tests
chore: Maintenance
security: Correction de sécurité
📄 Licence
Propriétaire - Golanafrica
🎉 Remerciements
Développé avec passion pour l'Afrique 🌍



