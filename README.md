

---

# 🛒 GoShop - Plateforme E-commerce SaaS Multi-tenant pour l'Afrique

**Vision** : Devenir le Shopify africain avec paiements Mobile Money intégrés (Wave, Orange Money, Moov Money), vente à crédit, système de tontine, et notifications temps réel.

**Statut** : ✅ **v4.5.0-production-ready** - Plateforme complète avec sécurité renforcée, WebSockets, et 200+ tests

---

## 🎯 Fonctionnalités Majeures

### 💳 Paiements Multi-providers
- **Orange Money** : USSD `#144*111#`
- **Moov Money** : USSD `#135*2#`
- **Wave** : Support complet
- **Yenga Pay** : Intégration API réelle
- **Cash on Delivery (COD)** : Workflow complet avec preuve de livraison
- **Validation HMAC-SHA256** des webhooks
- **Machine à états** pour les paiements

### 💰 Crédit à la Consommation (v3.5.0)
- Plans de paiement personnalisables (3, 6, 12 mois)
- Score de fiabilité client automatique
- Validation KYC obligatoire
- Calcul d'intérêts conforme BCEAO (max 15% annuel)
- Gestion des pénalités de retard
- Scheduler automatique de relances

### 🤝 Tontine (v3.3.0)
- Système d'épargne collective
- Cycles de paiement automatisés
- Commission plateforme configurable
- Gestion des groupes thématiques
- Vouchers de participation

### 🔔 Notifications Temps Réel (v4.5.0)
- **WebSocket** avec Redis Pub/Sub pour scalabilité horizontale
- Notifications instantanées pour :
  - Confirmation de commande
  - Statut de paiement
  - Livraisons
  - Rappels de crédit
- Fallback email (SMTP) si WebSocket indisponible

### 🔒 Sécurité Renforcée (v4.5.0)
- **JWT avec secret obligatoire** (min 32 caractères)
- **Bcrypt coût minimum 10** (ignore configurations dangereuses)
- **Anti-injection SQL** avec whitelist stricte sur `sort_by`
- **Isolation multi-tenant** vérifiée via middleware `RequireShopAccess`
- **Logs sécurisés** : pas de mots de passe en clair
- **Purge historique Git** : aucun secret commité

### 👥 Gestion des Utilisateurs
- **RBAC complet** : 6 rôles (super_admin, admin, merchant, customer, collaborator, guest)
- **2FA (TOTP)** : Google Authenticator compatible
- **Gestion des sessions** : Révocation, historique, détection d'anomalies
- **API Keys** : Pour intégrations tierces avec scopes granulaires

### 🏪 Multi-tenant Avancé
- Isolation complète par `shop_id`
- Header `X-Shop-Slug` ou domaine personnalisé
- Vérification propriétaire/collaborateur avant toute action
- KYC Marchand obligatoire pour certaines opérations

---

## 🎉 Nouveautés v4.5.0

### 🔔 Système de Notifications WebSocket
Architecture scalable avec Redis Pub/Sub pour supporter des milliers de connexions simultanées.

**Endpoints** :
- `GET /ws/notifications 🔒` - Connexion WebSocket (nécessite JWT)

**Types de notifications** :
```json
{
  "type": "client_order_confirmed",
  "title": "Commande confirmée",
  "message": "Votre commande #123 a été acceptée",
  "data": {
    "order_id": "uuid",
    "shop_name": "Ma Boutique"
  }
}
```

### 🔒 Durcissement de Sécurité
1. **Validation JWT au démarrage** : Le serveur refuse de démarrer sans `JWT_SECRET` valide
2. **Coût bcrypt minimum** : Force un coût de 10 même si `BCRYPT_COST=4` est configuré
3. **Anti-injection SQL** : Whitelist stricte sur tous les paramètres `sort_by`
4. **Middleware RequireShopAccess** : Vérifie que l'utilisateur est propriétaire ou collaborateur
5. **Logs sécurisés** : Suppression des logs de `raw_body` dans les handlers sensibles

### 🔗 Liaison User ↔ Customer
Migration `033_add_user_id_to_customers.sql` permet de lier un `Customer` à un `User` authentifié pour :
- Notifications personnalisées
- Historique cross-shop
- Dashboard client unifié

---

## 🎯 Nouveautés v4.x

### v4.4.3 : API Keys Management
- Création/révocation de clés API pour intégrations tierces
- Scopes granulaires (read:products, write:orders, etc.)
- Statistiques d'utilisation
- Rotation automatique

### v4.4.2 : Gestion des Sessions
- Historique des sessions actives
- Révocation individuelle ou globale
- Détection d'anomalies (nouvelle IP, nouvel appareil)
- Expiration automatique

### v4.4.0 : Authentification 2FA (TOTP)
- Compatible Google Authenticator, Authy
- Codes de récupération (10 codes à usage unique)
- Activation/désactivation sécurisée
- Protection contre le replay

### v4.3.0 : Système de Collaborateurs
- **Collaborateurs Plateforme** : Accès global avec rôles personnalisés
- **Collaborateurs Boutique** : Accès limité à une boutique
- Invitations par email avec token sécurisé
- Permissions granulaires (lecture, écriture, admin)

### v4.2.0 : Administration des Boutiques
- Dashboard admin avec statistiques globales
- Suspension/activation de boutiques
- Notes admin et historique d'audit
- Score de santé des boutiques

### v4.1.0 : KYC Marchand
- Upload de documents (CNI, registre de commerce)
- Validation manuelle par admin
- Statuts : pending, verified, rejected
- Blocage automatique si KYC non validé

### v4.0.0 : RBAC Complet
- 6 rôles : super_admin, admin, merchant, customer, collaborator, guest
- Permissions par endpoint
- Middleware `RequireRoles` pour protection granulaire

---

## 🎯 Nouveautés v3.x

### v3.6.0 : Catalogue Public
- `GET /api/public/products` - Catalogue visible sans authentification
- Filtres par catégorie, prix, disponibilité
- Pagination optimisée

### v3.5.0 : Crédit à la Consommation
- Plans de paiement personnalisables
- Score de fiabilité automatique
- Validation KYC obligatoire
- Scheduler de relances automatiques
- **Merchant Overview** : Dashboard unifié pour les marchands

### v3.4.0 : Scheduler Crédit
- Relances automatiques pour échéances en retard
- Calcul de pénalités configurable
- Notifications email/SMS

### v3.3.0 : Scheduler Tontine
- Clôture automatique des cycles
- Distribution des gains
- Commission plateforme

### v3.2.0 : Scheduler Paiements en Ligne
- Vérification des paiements en attente
- Mise à jour automatique des statuts
- Relances pour paiements échoués

### v3.1.0 : Scheduler COD
- Collecte automatique des commissions (2.5%)
- Génération de batches de commission
- Réconciliation automatique

### v3.0.0 : Cash on Delivery (COD)
- Workflow complet : pending → accepted → out_for_delivery → delivered
- Preuve de livraison (photo + signature)
- Commission plateforme automatique
- Protection contre la fraude

---

## 🚀 Démarrage Rapide

### Prérequis
- Go 1.23+
- PostgreSQL 16+
- Redis 7+
- Docker (optionnel)

### Installation

```bash
# Cloner le dépôt
git clone https://github.com/golanafrica/GoShop.git
cd GoShop

# Configuration
cp .env.example .env
# Éditer .env avec vos valeurs (voir section Variables d'Environnement)

# Lancer PostgreSQL et Redis
docker-compose up -d postgres redis

# Appliquer les migrations
go run cmd/migrate/main.go

# Lancer l'API
go run cmd/api/main.go
```

### Variables d'Environnement Critiques

```bash
# 🔒 SÉCURITÉ - OBLIGATOIRE
JWT_SECRET=une_chaine_tres_longue_et_aleatoire_d_au_moins_32_caracteres
BCRYPT_COST=12  # Recommandé : 12 en production, 4 en dev

# Base de données
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=goshop_db

# Redis
REDIS_ADDR=localhost:6379

# Serveur
APP_PORT=8080
APP_ENV=development  # development | staging | production

# Email (SMTP)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=noreply@goshop.com
SMTP_PASSWORD=your_app_password

# Yenga Pay (optionnel)
YENGA_PAY_API_KEY=your_api_key
YENGA_PAY_ORGANIZATION_ID=your_org_id
YENGA_PAY_PROJECT_ID=your_project_id
YENGA_PAY_WEBHOOK_SECRET=your_webhook_secret
YENGA_PAY_ENV=test  # test | production
```

### Endpoints Techniques

| Service | Endpoint |
|---------|----------|
| API | `http://localhost:8080` |
| WebSocket | `ws://localhost:8080/ws/notifications` |
| Liveness | `GET /health/live` |
| Readiness | `GET /health/ready` |
| Metrics | `GET /metrics` |
| Swagger UI | `GET /swagger/index.html` |

---

## 📈 Routes API

### Authentification
- `POST /register` - Inscription
- `POST /login` - Connexion
- `POST /logout 🔒` - Déconnexion (révoque session)
- `POST /auth/refresh` - Rafraîchir le token
- `GET /auth/me 🔒` - Profil utilisateur

### 2FA (v4.4.0)
- `POST /api/auth/2fa/setup 🔒` - Initialiser 2FA
- `POST /api/auth/2fa/enable 🔒` - Activer avec code TOTP
- `POST /api/auth/2fa/disable 🔒` - Désactiver
- `GET /api/auth/2fa/status 🔒` - Statut 2FA
- `POST /api/auth/2fa/regenerate 🔒` - Régénérer codes de récupération

### Gestion des Sessions (v4.4.2)
- `GET /api/sessions 🔒` - Lister sessions actives
- `DELETE /api/sessions/{id} 🔒` - Révoquer une session
- `DELETE /api/sessions 🔒` - Révoquer toutes les sessions

### API Keys (v4.4.3)
- `POST /api/api-keys 🔒` - Créer une clé API
- `GET /api/api-keys 🔒` - Lister mes clés
- `DELETE /api/api-keys/{id} 🔒` - Révoquer une clé
- `GET /api/api-keys/stats 🔒` - Statistiques d'utilisation

### Gestion des Boutiques (Multi-tenant)
- `POST /api/shops 🔒` - Créer une boutique
- `GET /api/shops 🔒` - Lister mes boutiques
- `PUT /api/shops/{id} 🔒` - Modifier une boutique
- `GET /api/shops/{id}/payment-settings 🔒` - Configuration paiements
- `PUT /api/shops/{id}/payment-settings 🔒` - Modifier configuration

### KYC Marchand (v4.1.0)
- `POST /api/merchant/kyc 🔒` - Soumettre documents KYC
- `GET /api/merchant/kyc/status 🔒` - Statut KYC
- `GET /api/admin/merchant/kyc/pending 🔒👑` - Lister KYC en attente (admin)
- `POST /api/admin/merchant/kyc/{id}/review 🔒👑` - Valider/rejeter KYC (admin)

### Administration (v4.2.0)
- `GET /api/admin/shops 🔒👑` - Lister toutes les boutiques
- `PUT /api/admin/shops/{id}/suspend 🔒👑` - Suspendre une boutique
- `PUT /api/admin/shops/{id}/activate 🔒👑` - Réactiver une boutique
- `GET /api/admin/dashboard 🔒👑` - Dashboard admin

### Collaborateurs (v4.3.0)
- `POST /api/collaborators/invite 🔒` - Inviter un collaborateur
- `POST /api/collaborators/accept/{token}` - Accepter une invitation
- `GET /api/collaborators 🔒` - Lister mes collaborateurs
- `PUT /api/collaborators/{id}/role 🔒` - Modifier rôle
- `DELETE /api/collaborators/{id} 🔒` - Supprimer collaborateur

### 💳 Paiements
- `POST /api/orders/{id}/pay 🔒` - Initier un paiement
- `GET /api/payments 🔒` - Lister les paiements
- `GET /api/payments/{id} 🔒` - Détails d'un paiement
- `POST /api/payments/{id}/refund 🔒` - Rembourser
- `POST /webhooks/{provider}` - Webhook provider (public, HMAC validé)

### 📦 Cash on Delivery (v3.0.0)
- `POST /api/orders/{id}/accept 🔒` - Accepter commande COD
- `POST /api/orders/{id}/reject 🔒` - Rejeter commande COD
- `POST /api/orders/{id}/out-for-delivery 🔒` - Marquer en livraison
- `POST /api/orders/{id}/deliver 🔒` - Confirmer livraison avec preuve

### 💰 Crédit (v3.5.0)
- `POST /api/credit/plans 🔒` - Créer un plan de crédit
- `POST /api/credit/apply 🔒` - Demander un crédit
- `POST /api/credit/{id}/approve 🔒` - Approuver une demande
- `POST /api/credit/{id}/pay-down-payment 🔒` - Payer l'apport
- `POST /api/credit/{id}/pay-installment 🔒` - Payer une échéance

### 🤝 Tontine (v3.3.0)
- `POST /api/tontine/groups 🔒` - Créer un groupe
- `POST /api/tontine/groups/{id}/join 🔒` - Rejoindre un groupe
- `POST /api/tontine/groups/{id}/pay 🔒` - Payer sa cotisation
- `GET /api/tontine/groups/{id}/payments 🔒` - Historique paiements

### API Protégée (nécessite `X-Shop-Slug`)
- **Customers** : `GET | POST | PUT | DELETE /api/customers`
- **Products** : `GET | POST | PUT | DELETE /api/products`
- **Orders** : `GET | POST /api/orders`
- **Withdrawals** : `GET | POST /api/withdrawals`

### Endpoints Publics
- `GET /api/public/products` - Catalogue public (v3.6.0)
- `GET /health/live`
- `GET /health/ready`
- `GET /help`

### 🔔 WebSocket (v4.5.0)
- `GET /ws/notifications 🔒` - Connexion WebSocket temps réel

---

## 💳 Utilisation du Système de Paiement

### Exemple : Initier un paiement Orange Money

```bash
curl -X POST http://localhost:8080/api/orders/{order_id}/pay \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Shop-Slug: ma-boutique" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "orange_money",
    "phone_number": "+22670123456",
    "description": "Paiement commande #123"
  }'
```

**Réponse** :
```json
{
  "payment_id": "57f92b94-5187-4b84-84f4-a31081de634a",
  "provider_ref": "MV-bb5a95c54010a058",
  "status": "processing",
  "ussd_code": "#135*2#MV-bb5a95c54010a058#",
  "message": "Composez #135*2*MV-bb5a95c54010a058# pour valider le paiement de 20000 FCFA via Moov Money"
}
```

### Exemple : Webhook HMAC

```bash
# Générer la signature HMAC-SHA256
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | cut -d' ' -f2)

curl -X POST http://localhost:8080/webhooks/orange_money \
  -H "Content-Type: application/json" \
  -H "X-Signature: $SIGNATURE" \
  -d "$PAYLOAD"
```

### Providers Disponibles

| Provider | Code | USSD | Délai mock |
|----------|------|------|------------|
| Orange Money | `orange_money` | `#144*111#` | 2s |
| Moov Money | `moov_money` | `#135*2#` | 3s |
| Wave | `wave` | N/A | 1s |
| Yenga Pay | `yenga_pay` | N/A | Réel |

---

## 🔔 Utilisation des WebSocket (v4.5.0)

### Connexion avec JavaScript

```javascript
const token = "your_jwt_token_here";
const ws = new WebSocket(`ws://localhost:8080/ws/notifications?token=${token}`);

ws.onmessage = (event) => {
  const notification = JSON.parse(event.data);
  console.log("Notification reçue:", notification);
  
  if (notification.type === "client_order_confirmed") {
    alert(notification.message);
  }
};

ws.onclose = () => {
  console.log("Connexion WebSocket fermée");
};
```

### Test avec le Script PowerShell

```powershell
.\Test-WebSocket-Notification.ps1
```

Ce script crée automatiquement :
1. Un utilisateur de test
2. Une boutique
3. Un produit
4. Une commande
5. Accepte la commande (déclenche la notification WebSocket)

---

## 🏪 Utilisation Multi-tenant

### Exemple : Créer une boutique

```bash
curl -X POST http://localhost:8080/api/shops \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Ma Boutique", "slug": "ma-boutique"}'
```

### Exemple : Utiliser une boutique

```bash
# Lister les produits de "ma-boutique"
curl http://localhost:8080/api/products \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Shop-Slug: ma-boutique"
```

### Isolation Garantie
- ✅ Les produits créés dans `ma-boutique` ne sont pas visibles dans `demo`
- ✅ Les clients sont isolés par boutique
- ✅ Les commandes sont liées à leur boutique
- ✅ Les paiements sont isolés par boutique
- ✅ **v4.5.0** : Vérification propriétaire/collaborateur avant toute action

---

## 🧪 Tests

### Tests Unitaires & Intégration

```bash
go test ./... -v
```

### Tests End-to-End (E2E)

```bash
go test ./tests/e2e/... -v
```

### Tests de Paiement

```bash
go test ./tests/e2e/... -v -run TestPaymentFlowE2E
```

### Tests de Sécurité

```bash
# Tests d'isolation multi-tenant + HMAC webhook
go test ./tests/e2e/... -v -run TestSecurityE2E

# Tests de validation JWT
go test ./tests/e2e/... -v -run TestJWTSecurityE2E
```

### Tests WebSocket (v4.5.0)

```powershell
.\Test-WebSocket-Notification.ps1
```

### Tests de Charge (k6)

```bash
k6 run tests/loadtest/scripts/auth_load.js
go test ./tests/loadtest/... -v
```

### Scénarios E2E Couverts
- ✅ Authentification : inscription → connexion → accès profil
- ✅ Multi-tenant : création shop → customers → products → orders
- ✅ Isolation : vérification cross-shop
- ✅ Sécurité : routes publiques / protégées, CORS, headers
- ✅ Paiement : initiation → success → refund
- ✅ Webhook HMAC : validation + rejet payload tamperé
- ✅ COD : workflow complet avec preuve de livraison
- ✅ Crédit : demande → approbation → paiement échéances
- ✅ Tontine : création groupe → cotisations → distribution
- ✅ 2FA : activation → validation → désactivation
- ✅ Sessions : création → révocation → détection anomalies
- ✅ API Keys : création → utilisation → révocation
- ✅ WebSocket : connexion → notification → déconnexion

---

## 📊 Observabilité

### Logs Structurés
- Format JSON (zerolog)
- Niveaux dynamiques : debug, info, warn, error
- Request ID pour corrélation des logs
- Audit des connexions (emails masqués)
- **v4.5.0** : Logs sécurisés (pas de mots de passe en clair)
- Compatible Loki / Grafana

### Métriques Prometheus
- `orders_created_total`
- `order_revenue_cents_total`
- `products_created_total`
- `auth_login_total`
- `auth_login_failed_total`
- `payments_initiated_total`
- `payments_success_total`
- `payments_refunded_total`
- `webhooks_received_total`
- `websocket_connections_active` (v4.5.0)
- `notifications_sent_total` (v4.5.0)
- Latence HTTP par endpoint

📍 **Exposées via** : `GET /metrics`

### ❤️ Health Checks

| Endpoint | Description |
|----------|-------------|
| `/health/live` | Serveur actif |
| `/health/ready` | DB + Redis opérationnels |

➡️ Prêt pour `livenessProbe` et `readinessProbe` Kubernetes.

---

## 🔒 Sécurité

### Authentification & Autorisation
- ✅ JWT (access + refresh tokens) avec **secret obligatoire** (v4.5.0)
- ✅ Hash des mots de passe (bcrypt coût **minimum 10**) (v4.5.0)
- ✅ 2FA TOTP (Google Authenticator)
- ✅ Gestion des sessions avec révocation
- ✅ API Keys avec scopes granulaires
- ✅ RBAC complet (6 rôles)

### Protection des Données
- ✅ Headers HTTP de sécurité
- ✅ CORS configurable
- ✅ Rate limiting (Redis avec fallback mémoire)
- ✅ Requêtes SQL paramétrées
- ✅ **Anti-injection SQL** avec whitelist stricte (v4.5.0)
- ✅ Secrets via variables d'environnement (jamais commités)

### Isolation Multi-tenant
- ✅ Isolation par `shop_id`
- ✅ **Vérification propriétaire/collaborateur** via middleware (v4.5.0)
- ✅ Validation HMAC-SHA256 des webhooks
- ✅ Protection contre payload tampering
- ✅ Timestamps UTC cohérents pour l'audit

### Hygiène du Code
- ✅ **Purge historique Git** : aucun secret commité (v4.5.0)
- ✅ **Logs sécurisés** : pas de mots de passe en clair (v4.5.0)
- ✅ Conteneurs Docker en non-root
- ✅ Middleware de recovery (pas de crash serveur)

---

## 🛠️ Architecture

### Clean Architecture / DDD

```
├── cmd/api              # Point d'entrée
├── internal/app         # Initialisation application
├── domain               # Entités métier & interfaces
│   ├── entity           # Shop, Product, Customer, Order, Payment, etc.
│   ├── repository       # Interfaces des repositories
│   ├── service          # Services métier (NotificationService, etc.)
│   └── tenant           # Contexte multi-tenant
├── application          # Use cases & DTOs
│   ├── usecase/
│   │   ├── payment_usecase/      # 5 usecases paiement
│   │   ├── order_usecase/        # Workflow COD complet
│   │   ├── credit_usecase/       # Crédit à la consommation
│   │   ├── tontine_usecase/      # Tontine
│   │   ├── wallet_usecase/       # Portefeuille marchand
│   │   ├── auth_usecase/         # 2FA, sessions
│   │   └── collaborator_usecase/ # Gestion collaborateurs
│   └── scheduler/       # Schedulers Cron (COD, paiements, crédit, tontine)
├── interfaces           # Handlers HTTP & middlewares
│   ├── handler/
│   │   ├── payment_handler/      # PaymentHandler + WebhookHandler
│   │   ├── ws_handler/           # WebSocket handler (v4.5.0)
│   │   └── ...
│   ├── middl            # Middlewares (TenantResolver, Auth, RequireShopAccess)
│   └── utils            # Utilitaires (erreurs, JWT, logs sécurisés)
├── infrastructure       # PostgreSQL, Redis, WebSocket
│   ├── payment/         # Registry + Providers
│   ├── postgres/        # Repositories PostgreSQL
│   ├── websocket/       # Hub WebSocket + Redis Pub/Sub (v4.5.0)
│   └── notification/    # Dispatcher notifications (WebSocket + Email)
├── config               # Configuration & logging
├── tests                # Unit, E2E, load
└── migrations           # Migrations SQL (001-034)
```

### Stack Technique
- **Go 1.23**
- **Chi** (router HTTP v5)
- **PostgreSQL 16**
- **Redis 7** (cache, sessions, WebSocket Pub/Sub)
- **Prometheus** (métriques)
- **Zerolog** (logs structurés)
- **Gorilla WebSocket** (connexions temps réel)
- **Docker multi-stage** (Alpine)
- **Kubernetes** (Minikube)

---

## 🗄️ Base de Données

### Migrations

| Migration | Description |
|-----------|-------------|
| `001_init` | Tables de base (users, products, customers, orders) |
| `002_multi_tenant` | Tables shops, shop_payment_settings |
| `003_payment_system` | Tables payments, payment_webhooks |
| `004_add_indexes` | Indexes de performance |
| `005_add_yenga_pay_provider` | Configuration Yenga Pay |
| `006_add_yenga_pay_shop_settings` | Paramètres Yenga Pay par boutique |
| `007_fix_shop_payment_settings` | Correction configuration paiements |
| `008_add_withdrawals` | Retraits marchand |
| `009_add_cash_on_delivery` | Workflow COD |
| `010_add_tontine` | Système de tontine |
| `011_add_kyc` | KYC client |
| `012_add_paid_status` | Statuts de paiement |
| `013_add_credit_wallet_cod` | Crédit, wallet, COD |
| `014_add_commission_batches` | Batches de commission |
| `015_add_payment_commissions` | Commissions paiements |
| `016_add_tontine_commissions` | Commissions tontine |
| `017_add_credit_commissions` | Commissions crédit |
| `018_add_rbac` | RBAC complet |
| `019_add_merchant_kyc` | KYC marchand |
| `020_admin_shop_management` | Administration boutiques |
| `021_collaborators` | Système de collaborateurs |
| `022_fix_unique_constraint` | Correction contraintes uniques |
| `023_add_2fa` | Authentification 2FA |
| `024_add_replay_protection` | Protection contre replay |
| `025_add_sessions` | Gestion des sessions |
| `026_add_api_keys` | API Keys management |
| `027_add_payment_references` | Références paiements |
| `028_add_phone_number_to_customers` | Numéro téléphone clients |
| `029_drop_payments_order_id_fkey` | Correction FK paiements |
| `030_add_penalty_tracking` | Suivi pénalités crédit |
| `031_add_indexes_for_public_catalog` | Indexes catalogue public |
| `032_add_product_full_text_search` | Recherche full-text produits |
| `033_add_user_id_to_customers` | Liaison User ↔ Customer (v4.5.0) |
| `034_fix_users_role_check` | Correction contrainte rôle user (v4.5.0) |

### Tables Principales
- `users` : Utilisateurs authentifiés (avec 2FA, sessions)
- `shops` : Boutiques multi-tenant
- `products` : Produits par shop
- `customers` : Clients par shop (liés à users via user_id)
- `orders` : Commandes par shop (avec workflow COD)
- `order_items` : Items de commande
- `payments` : Paiements avec machine à états
- `payment_webhooks` : Audit trail des webhooks
- `credit_plans` : Plans de crédit
- `credit_applications` : Demandes de crédit
- `credit_contracts` : Contrats de crédit
- `tontine_groups` : Groupes de tontine
- `tontine_payments` : Paiements tontine
- `user_sessions` : Sessions actives
- `api_keys` : Clés API
- `collaborators` : Collaborateurs plateforme/boutique

---

## 🗺️ Roadmap

### ✅ Phase 1 : Multi-tenant (COMPLÉTÉ - v2.0.0)
- ✅ Boutiques indépendantes avec isolation par shop_id
- ✅ Middleware TenantResolver (X-Shop-Slug + Host)
- ✅ Endpoints de gestion des boutiques (/api/shops)
- ✅ Repositories multi-tenant
- ✅ 130+ tests passent

### ✅ Phase 2 : Paiements Avancés (COMPLÉTÉ - v2.1.0)
- ✅ Orange Money (mock, USSD #144*111#)
- ✅ Moov Money (mock, USSD #135*2#)
- ✅ Validation HMAC-SHA256 des webhooks
- ✅ Machine à états pour les paiements
- ✅ Auto-update du statut commande (PENDING → PAID)
- ✅ Remboursements complets
- ✅ Tests de sécurité (isolation + HMAC)

### ✅ Phase 3 : Cash on Delivery (COMPLÉTÉ - v3.0.0)
- ✅ Workflow complet (pending → accepted → delivered)
- ✅ Preuve de livraison (photo + signature)
- ✅ Commission plateforme automatique
- ✅ Scheduler de collecte des commissions

### ✅ Phase 4 : Crédit à la Consommation (COMPLÉTÉ - v3.5.0)
- ✅ Plans de paiement personnalisables
- ✅ Score de fiabilité client
- ✅ Validation KYC obligatoire
- ✅ Scheduler de relances automatiques
- ✅ Merchant Overview

### ✅ Phase 5 : Tontine (COMPLÉTÉ - v3.3.0)
- ✅ Système d'épargne collective
- ✅ Cycles de paiement automatisés
- ✅ Commission plateforme configurable
- ✅ Scheduler de clôture des cycles

### ✅ Phase 6 : Sécurité & Administration (COMPLÉTÉ - v4.x)
- ✅ RBAC complet (6 rôles)
- ✅ 2FA (TOTP)
- ✅ Gestion des sessions
- ✅ API Keys management
- ✅ KYC Marchand
- ✅ Administration des boutiques
- ✅ Système de collaborateurs

### ✅ Phase 7 : Temps Réel (COMPLÉTÉ - v4.5.0)
- ✅ WebSocket avec Redis Pub/Sub
- ✅ Notifications instantanées
- ✅ Durcissement sécurité (JWT, bcrypt, anti-injection)
- ✅ Purge historique Git
- ✅ Logs sécurisés

### 🚧 Phase 8 : Intégrations Réelles (EN COURS)
- 🚧 Intégration API réelle Orange Money
- 🚧 Intégration API réelle Moov Money
- 🚧 Intégration Wave
- 🚧 Notifications SMS (Twilio, Orange SMS)

### 🚧 Phase 9 : Analytics Avancés (PRÉVU)
- 🚧 Dashboard analytics pour marchands
- 🚧 Rapports de ventes personnalisés
- 🚧 Prédictions de ventes (ML)
- 🚧 Segmentation clients automatique

### 🚧 Phase 10 : Mobile (PRÉVU)
- 🚧 Application mobile iOS/Android
- 🚧 Notifications push
- 🚧 Mode hors ligne
- 🚧 Scanner de codes-barres

---

## 📚 Documentation

- [Architecture](docs/architecture.md)
- [Modèle de données](docs/domain-model.md)
- [API Reference](docs/api-reference.md)
- [Multi-tenant](docs/multi-tenant.md)
- [Système de paiement](docs/payment-system.md)
- [Cash on Delivery](docs/cod-workflow.md)
- [Crédit à la consommation](docs/credit-system.md)
- [Tontine](docs/tontine-system.md)
- [Sécurité](docs/security.md)
- [Déploiement](docs/deployment.md)
- [Contribuer](docs/contributing.md)

---

## 🤝 Contribuer

Voir [CONTRIBUTING.md](docs/contributing.md) pour les détails.

### Convention de Commits
- `feat:` Nouvelle fonctionnalité
- `fix:` Correction de bug
- `docs:` Documentation
- `test:` Ajout de tests
- `chore:` Maintenance
- `security:` Correction de sécurité
- `perf:` Amélioration de performance
- `refactor:` Refactoring de code

---

## 📄 Licence

Propriétaire - Golanafrica

---

## 🎉 Remerciements

Développé avec passion pour l'Afrique 🌍

**Contributeurs** :
- Équipe Golanafrica
- Communauté open source

---

**Dernière mise à jour** : 21 juillet 2026  
**Version** : v4.5.0-production-ready  
**Statut** : ✅ Prêt pour la production

---
