# 🛒 GoShop - Plateforme E-commerce SaaS pour l'Afrique

**Vision** : Devenir le Shopify africain avec paiements Mobile Money intégrés (Wave, Orange Money, MTN MoMo), vente à crédit et système de tontine.

**Statut** : ✅ **v1.0.0-mvp** - Backend API stable, prêt pour évolution multi-tenant

---

## 🚀 Démarrage rapide

### Prérequis
- Docker & Docker Compose
- Go 1.23+ (optionnel, pour développement local)

### Lancement
```bash
# Démarrer l'ensemble de la stack (API + DB + Redis + Prometheus)
docker-compose up --build

API disponible sur : http://localhost:8080

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
POST /register
POST /login
POST /auth/refresh
GET /auth/me 🔒
API protégée (/api)
Customers : GET | POST | PUT | DELETE /api/customers
Products : GET | POST | PUT | DELETE /api/products
Orders : GET | POST /api/orders
Endpoints publics
GET /health/live
GET /health/ready
GET /help
🧪 Tests
Tests unitaires & intégration

go test ./... -v

Tests End-to-End (E2E)

go test -tags=e2e ./tests/e2e/... -v

Tests de charge (k6)

k6 version
go test ./tests/loadtest/... -v

Scénarios E2E couverts
✅ Authentification : inscription → connexion → accès profil
✅ Produits : CRUD complet
✅ Commandes : création avec items multiples
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
🛠️ Architecture
Clean Architecture / DDD

├── cmd/api              # Point d'entrée
├── internal/app         # Initialisation application
├── domain               # Entités métier & interfaces
├── application          # Use cases & DTOs
├── interfaces           # Handlers HTTP & middlewares
├── infrastructure       # PostgreSQL, Redis
├── config               # Configuration & logging
├── tests                # Unit, E2E, load
└── migrations           # Migrations SQL

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

🚢 Déploiement Kubernetes (Minikube)

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
Phase 1 : Multi-tenant (Option B → A)
🚧 Boutiques indépendantes avec isolation par shop_id
🚧 Middleware de résolution de tenant
🚧 Migration progressive vers schémas PostgreSQL
Phase 2 : Paiements avancés
🚧 Cash à la livraison avec workflow complet
🚧 Mobile Money : Wave, Orange Money, MTN MoMo
🚧 Webhooks pour confirmation des paiements
Phase 3 : Crédit
🚧 Plans de paiement en tranches
🚧 Score de fiabilité client
🚧 Rappels automatiques
Phase 4 : Tontine
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
API Tontine
Frontend Tontine
🤝 Contribuer
Voir CONTRIBUTING.md pour les détails.
Convention de commits :
feat: Nouvelle fonctionnalité
fix: Correction de bug
docs: Documentation
test: Ajout de tests
chore: Maintenance
📄 Licence
Propriétaire - Golanafrica


