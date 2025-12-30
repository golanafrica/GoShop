# 🛒 GoShop API

Backend e-commerce moderne en Go avec authentification JWT, gestion de produits, clients et commandes.  
Conçu avec une **architecture DDD/Hexagonale**, **tests complets**, **observabilité intégrée**, et **déploiement Kubernetes production-ready**.

---

## 🚀 Démarrage rapide

### Prérequis
- Docker et Docker Compose
- Go 1.25+ (optionnel, pour le développement natif)

### Lancement
```bash
# Démarrer l'application complète (API + DB + Redis + Prometheus)
docker-compose up --build

# API disponible sur http://localhost:8080


Service
URL
API
http://localhost:8080
Health Check
GET /health/live
Readiness
GET /health/ready
Métriques
GET /metrics → Prometheus UI
Swagger UI
GET /swagger/index.html ✅

###🧪 Tests
###Tests unitaires et d'intégration
go test ./... -v

### Tests End-to-End (E2E)
# Exécuter les tests E2E
go test -tags=e2e ./tests/e2e/... -v

Tests de charge (k6)
# Vérifier l'installation de k6
k6 version

# Exécuter les tests de charge
go test ./tests/loadtest/... -v

Scénarios E2E couverts
✅ Authentification : Inscription → Connexion → Accès profil
✅ Gestion produits : CRUD complet
✅ Commandes : Création avec items multiples
✅ Sécurité : Headers de sécurité, CORS, endpoints publics

📊 Observabilité
Logs structurés
Format JSON avec zerolog
Niveaux dynamiques (debug, info, warn, error)
Request ID pour le tracing
Audit des connexions (emails masqués)
Métriques Prometheus
Latence par endpoint (goshop_http_duration_seconds)
Statistiques de pool de connexions DB
Disponible sur http://localhost:8080/metrics
Health Checks
Liveness : GET /health/live → État du serveur
Readiness : GET /health/ready → Dépendances (DB, Redis)
🔒 Sécurité
Middlewares de sécurité
Secure Headers : X-Content-Type-Options, X-Frame-Options, X-XSS-Protection
Rate Limiting : Protection contre les abus
CORS : Configuration flexible pour les clients web
RBAC : Contrôle d'accès basé sur les rôles
Recovery : Gestion des pannes sans crash
Authentication : JWT avec tokens d'accès et de rafraîchissement
Bonnes pratiques
Non-root user dans les conteneurs Docker
Mot de passe hashé (bcrypt) en base de données
Variables sensibles via variables d'environnement (pas dans le code)
Requêtes SQL paramétrées (protection contre les injections)


🛠️ Architecture
Structure du projet (Clean Architecture)

├── cmd/api              # Point d'entrée
├── internal/app         # Application principale
├── domain               # Entités métier et interfaces
├── application          # Use cases et DTOs
├── interfaces           # Handlers HTTP et middlewares
├── infrastructure       # Implémentations (PostgreSQL, Redis)
├── config               # Configuration et logging
├── tests                # Tests à tous les niveaux
└── migrations           # Scripts d'initialisation DB

Stack technique
Langage : Go 1.25
Framework : chi (router léger)
Base de données : PostgreSQL 16
Cache/Sessions : Redis 7
Observabilité : Prometheus + zerolog
Tests :
Unitaires : testing + mocks
E2E : Serveur HTTP réel + base de test
Charge : k6
Conteneurisation : Docker multi-stage, Alpine
📈 Routes API
Authentification
POST /register - Créer un compte
POST /login - Se connecter
POST /auth/refresh - Renouveler le token
GET /auth/me - Obtenir le profil (protégé)
Ressources protégées (/api)
Customers : GET|POST|PUT|DELETE /api/customers
Products : GET|POST|PUT|DELETE /api/products
Orders : GET|POST /api/orders
Endpoints publics
GET /health/live - Liveness probe
GET /health/ready - Readiness probe
GET /help - Vérification de disponibilité
🐳 Docker Compose
Services


Service
Port
Description
goshop
8080
API principale
db
5432
PostgreSQL
redis
6379
Cache et sessions
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

🔍 Runbook Opérationnel
Diagnostique
Pods stuck : kubectl logs <pod> -n goshop -c migrate
État DB : kubectl exec deployment/postgres -n goshop -- psql -U postgres goshop -c "\dt"
Logs API : kubectl logs -l app=goshop -n goshop
Scaling

kubectl scale deployment/goshop --replicas=5 -n goshop

Mise à jour
docker build -t goshop:new .
Mettre à jour image: goshop:new dans k8s/goshop.yaml
kubectl apply -f k8s/goshop.yaml

