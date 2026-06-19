# 🛒 GoShop - Plateforme E-commerce SaaS Multi-tenant pour l'Afrique

**Vision** : Devenir le Shopify africain avec paiements Mobile Money intégrés (Wave, Orange Money, MTN MoMo), vente à crédit et système de tontine.

**Statut** : ✅ **v2.0.0-multi-tenant** - Architecture multi-tenant complète, 130+ tests passent

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
- **100% de réussite** sur les tests de charge (1815/1815 checks)
- **10 VUs simultanés** avec 35.7 req/s
- **0 deadlock** grâce à `sync.Once` pour les migrations

---

## 🚀 Démarrage rapide

### Prérequis
- Docker & Docker Compose
- Go 1.23+ (optionnel, pour développement local)
- PostgreSQL 16
- Redis 7

### Lancement
```bash
# Démarrer l'ensemble de la stack (API + DB + Redis + Prometheus)
docker-compose up --build

# API disponible sur : http://localhost:8080

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
PUT /api/shops/{id} 🔒 - Modifier une boutique (propriétaire uniquement)
API protégée (nécessite X-Shop-Slug)
Customers : GET | POST | PUT | DELETE /api/customers
Products : GET | POST | PUT | DELETE /api/products
Orders : GET | POST /api/orders
Endpoints publics
GET /health/live
GET /health/ready
GET /help
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
🧪 Tests
Tests unitaires & intégration

go test ./... -v

Tests End-to-End (E2E)

go test ./tests/e2e/... -v

Tests de charge (k6)

k6 version
go test ./tests/loadtest/... -v

Scénarios E2E couverts
✅ Authentification : inscription → connexion → accès profil
✅ Multi-tenant : création shop → customers → products → orders
✅ Isolation : vérification cross-shop
✅ Sécurité : routes publiques / protégées, CORS, headers
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
🛠️ Architecture
Clean Architecture / DDD

├── cmd/api              # Point d'entrée
├── internal/app         # Initialisation application
├── domain               # Entités métier & interfaces
│   ├── entity           # Shop, Product, Customer, Order
│   ├── repository       # Interfaces des repositories
│   └── tenant           # Contexte multi-tenant
├── application          # Use cases & DTOs
├── interfaces           # Handlers HTTP & middlewares
│   ├── handler          # Handlers par domaine
│   ├── middl            # Middlewares (TenantResolver, Auth)
│   └── utils            # Utilitaires (erreurs, JWT)
├── infrastructure       # PostgreSQL, Redis
├── config               # Configuration & logging
├── tests                # Unit, E2E, load
└── migrations           # Migrations SQL (001_init, 002_multi_tenant)

Stack technique
Go 1.23
Chi (router HTTP v5)
PostgreSQL 16
Redis 7
Prometheus
Zerolog
Docker multi-stage (Alpine)
Kubernetes (Minikube)
🐳 Docker Compose
Services
Service
Port
Description
goshop
8080
API
db
5432
PostgreSQL
redis
6379
Cache / sessions
prometheus
9090
Monitoring
Variables d'environnement

APP_ENV=development
LOG_LEVEL=debug
DB_HOST=db
DB_USER=postgres
DB_PASSWORD=root
DB_NAME=goshop_db
REDIS_HOST=redis

Déploiement Kubernetes (Minikube)

minikube start
kubectl apply -f k8s/
minikube service goshop -n goshop

Runbook Opérationnel
Logs

kubectl logs -l app=goshop -n goshop

Base de données

kubectl exec deployment/postgres -n goshop -- \
  psql -U postgres goshop -c "\dt"
  
Scaling

kubectl scale deployment/goshop --replicas=5 -n goshop

Mise à jour

docker build -t goshop:new .
# Modifier l'image dans k8s/goshop.yaml
kubectl apply -f k8s/goshop.yaml

Roadmap - Évolutions prévues
✅ Phase 1 : Multi-tenant (COMPLÉTÉ)
✅ Boutiques indépendantes avec isolation par shop_id
✅ Middleware TenantResolver (X-Shop-Slug + Host)
✅ Endpoints de gestion des boutiques (/api/shops)
✅ Repositories multi-tenant (Product, Customer, Order, OrderItem)
✅ 130+ tests passent
🔄 Migration progressive vers schémas PostgreSQL (Phase 2 future)
🚧 Phase 2 : Paiements avancés
🚧 Cash à la livraison avec workflow complet
🚧 Mobile Money : Wave, Orange Money, MTN MoMo
🚧 Webhooks pour confirmation des paiements
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
Système de paiement
Déploiement
Contribuer
Guide des tests
Glossaire
Frontend Guidelines
Système de tontine
🤝 Contribuer
Voir CONTRIBUTING.md pour les détails.
Convention de commits
feat: Nouvelle fonctionnalité
fix: Correction de bug
docs: Documentation
test: Ajout de tests
chore: Maintenance
📄 Licence
Propriétaire - Golanafrica
🎉 Remerciements
Développé avec passion pour l'Afrique 🌍

