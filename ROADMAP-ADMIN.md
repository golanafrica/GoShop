📄 ROADMAP-ADMIN.md 
# 🗺️ Roadmap Admin - GoShop v4.2.0+

**Date** : 2026-07-04  
**Statut** : 🔄 Phase 2 en cours  
**Priorité** : 🔴 Critique pour la production  
**Inspiration** : Amazon Seller Central, Stripe Dashboard

---

## 🎯 Contexte

### ✅ Versions livrées

| Version | Date | Fonctionnalité | Statut |
|---------|------|----------------|--------|
| v3.5.0 | 03/07/2026 | Schedulers automatiques (COD, Online, Tontine, Credit) | ✅ **LIVRÉ** |
| v4.0.0 | 03/07/2026 | RBAC + JWT rôles + Super admin | ✅ **LIVRÉ** |
| v4.1.0 | 04/07/2026 | Workflow KYC Marchand progressif | ✅ **LIVRÉ** |

### 🎯 Fonctionnalités v4.0.0 (RBAC)

- ✅ 6 rôles système : `super_admin`, `admin`, `credit_analyst`, `support_agent`, `moderator`, `merchant`
- ✅ JWT avec claim `role` (access + refresh tokens)
- ✅ Middleware `RequireRoles()` pour protéger routes admin
- ✅ Super admin créé : `admin@goshop.com`
- ✅ Marchand test créé : `merchant@test.com`
- ✅ Routes admin protégées : `/api/admin/scheduler/*`, `/api/admin/commission-rates/*`

### 🎯 Fonctionnalités v4.1.0 (KYC Marchand)

- ✅ Workflow KYC progressif (`unverified` → `pending` → `verified`)
- ✅ Blocage retraits si KYC non vérifié (HTTP 400)
- ✅ 4 endpoints KYC (2 marchand + 2 admin)
- ✅ Badge "✅ Marchand vérifié"
- ✅ Audit trail complet (`verified_by`, `verified_at`)
- ✅ Documents requis : `identity_card` + `business_registry`

---

## 🏗️ Architecture des collaborateurs (inspirée d'Amazon)

### Vue d'ensemble à 3 niveaux


┌─────────────────────────────────────────────────────────────┐
│ PLATEFORME GoShop │
│ │
│ 👑 SUPER ADMIN (admin@goshop.com) │
│ │ │
│ ├── 👥 COLLABORATEURS PLATEFORME │
│ │ ├── finance_manager → Finances + rapports BCEAO │
│ │ ├── support_manager → Tickets + litiges │
│ │ ├── kyc_reviewer → Validation KYC marchands │
│ │ ├── marketing_manager → Campagnes + promos │
│ │ └── tech_admin → Config + maintenance │
│ │ │
│ └── 🏪 BOUTIQUES (multi-tenant) │
│ ├── Boutique A │
│ │ └── 👥 COLLABORATEURS BOUTIQUE │
│ │ ├── shop_admin → Gère boutique complète │
│ │ ├── seller → Produits + commandes │
│ │ ├── support → Support client │
│ │ └── accountant → Finances seulement │
│ │ │
│ └── Boutique B │
│ └── 👥 COLLABORATEURS BOUTIQUE │
│ └── ... │
└─────────────────────────────────────────────────────────────┘


### Parallèles avec Amazon Seller Central

| Fonctionnalité Amazon | Équivalent GoShop | Statut |
|----------------------|-------------------|--------|
| **Multi-niveaux d'accès** | RBAC avec 6 rôles | ✅ v4.0.0 |
| **User Permissions** | Permissions granulaires JSONB | ✅ v4.0.0 |
| **KYC/Verification** | Workflow KYC marchand | ✅ v4.1.0 |
| **Account suspension** | `shop.is_active` | ✅ Existe |
| **Account Health Score** | Score santé boutique | 🔄 Phase 2 |
| **Business Reports** | Analytics détaillés | ⏳ Phase 5 |
| **Performance Notifications** | Alertes violations | ⏳ Phase 6 |
| **Brand Registry** | Vérification marque | 🟢 Future |
| **Advertising Console** | Système promotion | 🟢 Future |

---

## 🚀 Plan d'implémentation

### Phase 2 : Gestion cross-tenant (v4.2.0) - 2 jours

**Objectif** : Permettre aux admins de gérer toutes les boutiques + Account Health Score

#### 2.1 Liste cross-tenant
- [ ] `GET /api/admin/shops` - Liste TOUTES les shops (pas filtré par tenant)
- [ ] Pagination + filtres (status, plan, kyc_status, date)
- [ ] Recherche par nom/slug
- [ ] Tri par colonnes

#### 2.2 Account Health Score ⭐ NOUVEAU (inspiré d'Amazon)
- [ ] Score 0-1000 par boutique
- [ ] Niveaux : `excellent` (800-1000), `good` (600-799), `warning` (400-599), `critical` (0-399)
- [ ] Métriques :
  - KYC status (-200 si non vérifié)
  - Taux succès paiements (-100 si < 90%)
  - Taux litiges (-150 si > 5%)
  - Temps réponse moyen (-50 si > 24h)
  - Ancienneté (+100 si > 6 mois)
- [ ] `GET /api/admin/shops/{id}/health` - Score détaillé
- [ ] Alertes automatiques si score < 500

#### 2.3 Actions sur les shops
- [ ] `PUT /api/admin/shops/{id}/suspend` - Suspendre (avec raison)
- [ ] `PUT /api/admin/shops/{id}/activate` - Réactiver
- [ ] `GET /api/admin/shops/{id}/stats` - Statistiques détaillées
- [ ] `GET /api/admin/shops/{id}/transactions` - Historique

#### 2.4 Gestion des abonnements
- [ ] `PUT /api/admin/shops/{id}/plan` - Changer plan (super_admin uniquement)
- [ ] `GET /api/admin/shops/{id}/billing` - Facturation

**Livrables** :
- ✅ Gestion complète des shops (cross-tenant)
- ✅ Account Health Score (inspiré d'Amazon)
- ✅ Suspension/activation avec audit trail
- ✅ Stats détaillées par shop

---

### Phase 3 : Système de collaborateurs (v4.3.0) - 3 jours

**Objectif** : Système moderne à 2 niveaux (plateforme + boutique) inspiré d'Amazon User Permissions

#### 3.1 Collaborateurs Plateforme

**Endpoints** :
- [ ] `GET /api/admin/collaborators` - Liste collaborateurs plateforme
- [ ] `POST /api/admin/collaborators/invite` - Inviter collaborateur (email)
- [ ] `PUT /api/admin/collaborators/{id}/role` - Changer rôle
- [ ] `DELETE /api/admin/collaborators/{id}` - Supprimer
- [ ] `PUT /api/admin/collaborators/{id}/permissions` - Modifier permissions

**Rôles plateforme** :
```go
const (
    RoleFinanceManager   = "finance_manager"   // Finances + rapports BCEAO
    RoleSupportManager   = "support_manager"   // Tickets + litiges
    RoleKYCReviewer      = "kyc_reviewer"      // Validation KYC
    RoleMarketingManager = "marketing_manager" // Campagnes + promos
    RoleTechAdmin        = "tech_admin"        // Config + maintenance
)

Permissions granulaires :

const (
    // Finance
    PermFinanceView      = "finance.view"
    PermFinanceExport    = "finance.export"
    PermFinanceApprove   = "finance.approve"
    
    // Support
    PermSupportView      = "support.view"
    PermSupportRespond   = "support.respond"
    PermSupportEscalate  = "support.escalate"
    
    // KYC
    PermKYCView          = "kyc.view"
    PermKYCApprove       = "kyc.approve"
    PermKYCReject        = "kyc.reject"
    
    // Marketing
    PermMarketingView    = "marketing.view"
    PermMarketingCreate  = "marketing.create"
    PermMarketingPublish = "marketing.publish"
    
    // Tech
    PermTechConfig       = "tech.config"
    PermTechMaintenance  = "tech.maintenance"
    PermTechWebhooks     = "tech.webhooks"
)


3.2 Collaborateurs Boutique
Endpoints :
GET /api/shops/{id}/collaborators - Liste collaborateurs boutique
POST /api/shops/{id}/collaborators/invite - Inviter
PUT /api/shops/{id}/collaborators/{id}/role - Changer rôle
DELETE /api/shops/{id}/collaborators/{id} - Supprimer
Rôles boutique :

const (
    RoleShopAdmin    = "shop_admin"    // Gère boutique complète
    RoleSeller       = "seller"        // Produits + commandes
    RoleSupport      = "support"       // Support client
    RoleAccountant   = "accountant"    // Finances seulement
)

Permissions granulaires :

const (
    // Shop admin
    PermShopAdmin = "shop.admin"  // Toutes les permissions
    
    // Seller
    PermProductsCreate  = "products.create"
    PermProductsUpdate  = "products.update"
    PermProductsDelete  = "products.delete"
    PermOrdersView      = "orders.view"
    PermOrdersProcess   = "orders.process"
    
    // Support
    PermCustomersView   = "customers.view"
    PermTicketsRespond  = "tickets.respond"
    
    // Accountant
    PermFinanceView     = "finance.view"
    PermReportsView     = "reports.view"
)

3.3 Invitation par email
Système d'invitation par email
Token d'invitation (expire après 7 jours)
Page d'acceptation d'invitation
Notification email personnalisable
Historique des invitations
Livrables :
✅ Système de collaborateurs à 2 niveaux
✅ Invitation par email avec token
✅ Permissions granulaires JSONB
✅ Historique des invitations
Phase 4 : Sécurité avancée (v4.4.0) - 2 jours
Objectif : Sécurité moderne pour super admin (inspiré d'Amazon Security Center)
4.1 2FA (Two-Factor Authentication)
TOTP (Google Authenticator, Authy)
Obligatoire pour super_admin
Optionnel pour autres rôles
Codes de récupération (10 codes à usage unique)
POST /api/admin/2fa/enable - Activer 2FA
POST /api/admin/2fa/disable - Désactiver 2FA
POST /api/admin/2fa/verify - Vérifier code
4.2 Session management
GET /api/admin/sessions - Liste sessions actives
DELETE /api/admin/sessions/{id} - Déconnecter session
DELETE /api/admin/sessions/all - Déconnecter toutes sessions
Détection d'activité suspecte (IP différente, device différent)
Alertes email pour connexions inhabituelles
4.3 API keys
GET /api/admin/api-keys - Liste clés API
POST /api/admin/api-keys - Créer clé API
DELETE /api/admin/api-keys/{id} - Révoquer clé
Permissions par clé API (JSONB)
Expiration configurable
Rate limiting par clé
4.4 Webhooks admin
GET /api/admin/webhooks - Liste webhooks
POST /api/admin/webhooks - Créer webhook
PUT /api/admin/webhooks/{id} - Modifier
DELETE /api/admin/webhooks/{id} - Supprimer
Test de webhook
Historique des livraisons
Signature HMAC pour sécurité
Livrables :
✅ 2FA obligatoire pour super_admin
✅ Session management complet
✅ API keys avec permissions
✅ Webhooks admin
Phase 5 : Dashboard & Monitoring (v4.5.0) - 3 jours
Objectif : Vue d'ensemble de la plateforme en temps réel (inspiré d'Amazon Business Reports)
5.1 Dashboard global
GET /api/admin/dashboard/overview
Total users, shops, transactions
Revenus du jour/semaine/mois
Alertes actives
Shops KYC en attente
Account Health Score moyen
5.2 Métriques temps réel
GET /api/admin/dashboard/metrics
Taux de succès paiements
Volume transactions
Commissions collectées
Performance schedulers
Latence API
5.3 Activity feed ⭐ NOUVEAU (inspiré d'Amazon Activity Log)
GET /api/admin/activity - Timeline des actions admin
Filtres : user, action, date, target_type
Export CSV
Pagination infinie
Notifications en temps réel (WebSocket)
5.4 Alertes configurables
GET /api/admin/alerts - Liste alertes
POST /api/admin/alerts - Créer alerte
PUT /api/admin/alerts/{id} - Modifier
DELETE /api/admin/alerts/{id} - Supprimer
Types : email, SMS, Slack, webhook
Conditions : seuils, patterns, anomalies
5.5 Graphiques
GET /api/admin/dashboard/charts/revenue - Revenus
GET /api/admin/dashboard/charts/users - Utilisateurs
GET /api/admin/dashboard/charts/transactions - Transactions
GET /api/admin/dashboard/charts/health - Health scores
Livrables :
✅ Dashboard complet avec métriques temps réel
✅ Activity feed (timeline des actions)
✅ Système d'alertes configurables
✅ Graphiques interactifs
Phase 6 : Performance Notifications (v4.6.0) - 2 jours
Objectif : Système d'alertes inspiré d'Amazon Performance Notifications
6.1 Système de notifications
GET /api/admin/notifications - Liste notifications
POST /api/admin/notifications/broadcast - Notification globale
PUT /api/admin/notifications/{id}/read - Marquer lu
PUT /api/admin/notifications/{id}/acknowledge - Accuser réception
DELETE /api/admin/notifications/{id} - Supprimer
6.2 Types de notifications

const (
    // Violations
    NotifViolationPolicy      = "violation.policy"
    NotifViolationKYC         = "violation.kyc"
    NotifViolationPayment     = "violation.payment"
    
    // Avertissements
    NotifWarningHealthScore   = "warning.health_score"
    NotifWarningLowBalance    = "warning.low_balance"
    NotifWarningExpiringKYC   = "warning.expiring_kyc"
    
    // Informations
    NotifInfoNewFeature       = "info.new_feature"
    NotifInfoMaintenance      = "info.maintenance"
    NotifInfoRegulation       = "info.regulation"
    
    // Actions requises
    NotifActionKYCRequired    = "action.kyc_required"
    NotifActionDocumentsNeeded = "action.documents_needed"
    NotifActionPaymentFailed  = "action.payment_failed"
)

6.3 Canaux de notification
Email (SendGrid/Mailgun)
SMS (Africa's Talking)
In-app (WebSocket)
Slack (webhook)
Push notifications (mobile)
6.4 Templates personnalisables
Templates email (HTML)
Templates SMS
Variables dynamiques ({{shop_name}}, {{amount}})
Preview avant envoi
Livrables :
✅ Système de notifications multi-canal
✅ Templates personnalisables
✅ Historique complet
✅ Statistiques d'ouverture
Phase 7 : Rapports & Conformité (v4.7.0) - 2 jours
Objectif : Rapports financiers et conformité BCEAO (inspiré d'Amazon Tax Document Library)
7.1 Rapports financiers
GET /api/admin/reports/financial
Revenus par période
Commissions par type (COD, Online, Tontine, Credit)
Top boutiques (par revenus)
Tendances (jour/semaine/mois)
Comparaison période précédente
7.2 Rapports BCEAO ⭐ CRITIQUE
GET /api/admin/reports/bceao
Transactions suspectes (> 5 000 000 FCFA)
Activités inhabituelles (patterns anormaux)
Fréquence transactions élevées
Multi-comptes suspects
Export format BCEAO
7.3 Export multi-format
POST /api/admin/reports/export
Format : CSV, PDF, Excel
Filtres : date, shop, type
Compression ZIP pour gros fichiers
Téléchargement asynchrone (job queue)
7.4 Rapports planifiés
POST /api/admin/reports/schedule
Envoi automatique (quotidien/hebdomadaire/mensuel)
Destinataires : email
Format : PDF/CSV
Historique des envois
Livrables :
✅ Rapports financiers complets
✅ Conformité BCEAO (critique)
✅ Export multi-format
✅ Rapports planifiés
📅 Timeline révisée
Phase
Version
Durée
Date cible
Statut
Phase 1
v4.0.0
2 jours
2026-07-03
✅ LIVRÉ
Phase 1b
v4.1.0
1 jour
2026-07-04
✅ LIVRÉ
Phase 2
v4.2.0
2 jours
2026-07-06
🔄 À FAIRE
Phase 3
v4.3.0
3 jours
2026-07-09
⏳ À venir
Phase 4
v4.4.0
2 jours
2026-07-11
⏳ À venir
Phase 5
v4.5.0
3 jours
2026-07-14
⏳ À venir
Phase 6
v4.6.0
2 jours
2026-07-16
⏳ À venir
Phase 7
v4.7.0
2 jours
2026-07-18
⏳ À venir
Total restant : 14 jours (2 semaines)
🎯 Priorités immédiates
🔴 À faire MAINTENANT (Phase 2 - v4.2.0)
Gestion cross-tenant - Lister toutes les shops
Account Health Score - Score santé boutique (inspiré d'Amazon)
Suspendre/activer shops - Action critique pour production
Stats par shop - Dashboard admin
🟡 À faire BIENTÔT (Phase 3 - v4.3.0)
Collaborateurs plateforme - Système moderne à 2 niveaux
Collaborateurs boutique - Multi-niveaux avec permissions
Invitation par email - UX moderne
Permissions granulaires - JSONB flexible
🟢 À faire PLUS TARD (Phases 4-7)
2FA - Sécurité avancée pour super_admin
Dashboard - Vue d'ensemble temps réel
Activity feed - Timeline des actions
Notifications - Multi-canal (email, SMS, Slack)
Rapports BCEAO - Conformité réglementaire
🔐 Système de permissions granulaires (détaillé)
Permissions plateforme (5 rôles)


// Finance Manager
FinanceManagerPermissions = []string{
    "finance.view",
    "finance.export",
    "finance.approve",
    "reports.view",
    "reports.export",
}

// Support Manager
SupportManagerPermissions = []string{
    "support.view",
    "support.respond",
    "support.escalate",
    "tickets.view",
    "tickets.respond",
}

// KYC Reviewer
KYCReviewerPermissions = []string{
    "kyc.view",
    "kyc.approve",
    "kyc.reject",
    "documents.view",
}

// Marketing Manager
MarketingManagerPermissions = []string{
    "marketing.view",
    "marketing.create",
    "marketing.publish",
    "campaigns.view",
    "campaigns.manage",
}

// Tech Admin
TechAdminPermissions = []string{
    "tech.config",
    "tech.maintenance",
    "tech.webhooks",
    "features.manage",
    "settings.view",
    "settings.update",
}


Permissions boutique (4 rôles)
go

// Shop Admin (toutes les permissions)
ShopAdminPermissions = []string{
    "shop.admin",  // Permission spéciale = toutes les autres
}

// Seller
SellerPermissions = []string{
    "products.create",
    "products.update",
    "products.delete",
    "products.view",
    "orders.view",
    "orders.process",
    "inventory.view",
    "inventory.update",
}

// Support
SupportPermissions = []string{
    "customers.view",
    "customers.update",
    "tickets.view",
    "tickets.respond",
    "orders.view",
}

// Accountant
AccountantPermissions = []string{
    "finance.view",
    "reports.view",
    "reports.export",
    "withdrawals.view",
    "transactions.view",
}

📊 Account Health Score (détaillé)
Calcul du score

func CalculateHealthScore(shop *Shop) int {
    score := 1000
    
    // KYC non vérifié : -200
    if shop.KYCStatus != "verified" {
        score -= 200
    }
    
    // Taux de succès paiements < 90% : -100
    paymentSuccess := getPaymentSuccessRate(shop.ID)
    if paymentSuccess < 0.90 {
        score -= 100
    }
    
    // Taux de litiges > 5% : -150
    disputeRate := getDisputeRate(shop.ID)
    if disputeRate > 0.05 {
        score -= 150
    }
    
    // Temps de réponse > 24h : -50
    avgResponseTime := getAverageResponseTime(shop.ID)
    if avgResponseTime > 24*time.Hour {
        score -= 50
    }
    
    // Ancienneté > 6 mois : +100
    if time.Since(shop.CreatedAt) > 6*time.Month {
        score += 100
    }
    
    // Volume transactions > 1M FCFA/mois : +50
    monthlyVolume := getMonthlyVolume(shop.ID)
    if monthlyVolume > 100000000 { // 1M FCFA en centimes
        score += 50
    }
    
    return max(0, min(1000, score))
}

Niveaux de santé
Score
Niveau
Couleur
Action
800-1000
Excellent
🟢 Vert
Aucune
600-799
Good
🔵 Bleu
Surveillance
400-599
Warning
🟡 Jaune
Alerte admin
0-399
Critical
🔴 Rouge
Action immédiate
Alertes automatiques


// Si score < 500, créer notification
if healthScore < 500 {
    createNotification(shop.ID, "warning.health_score", 
        "Votre score de santé est critique. Actions requises.")
}

// Si score < 300, suspendre automatiquement
if healthScore < 300 {
    suspendShop(shop.ID, "Score de santé critique")
}

📚 Ressources
Architecture GoShop
Multi-tenant
Sécurité (à créer)
RBAC Guide (à créer)
Collaborators Guide (à créer)
Amazon Seller Central (inspiration)
✅ Checklist de progression
Phase 1 terminée (v4.0.0) - RBAC
Phase 1b terminée (v4.1.0) - KYC Marchand
Phase 2 terminée (v4.2.0) - Gestion cross-tenant + Health Score
Phase 3 terminée (v4.3.0) - Collaborateurs
Phase 4 terminée (v4.4.0) - Sécurité avancée
Phase 5 terminée (v4.5.0) - Dashboard
Phase 6 terminée (v4.6.0) - Notifications
Phase 7 terminée (v4.7.0) - Rapports
Documentation complète
Tests E2E admin
Déploiement production
🎯 Prochaine étape
Phase 2 (v4.2.0) - Gestion cross-tenant + Account Health Score
Tâches prioritaires
✅ Créer AdminShopUsecase (5 méthodes)
✅ Créer AdminShopHandler (5 endpoints)
✅ Implémenter CalculateHealthScore()
✅ Ajouter routes dans app.go
✅ Tester avec super_admin
Endpoints à créer

GET    /api/admin/shops                    - Liste toutes les shops
GET    /api/admin/shops/{id}               - Détails shop
GET    /api/admin/shops/{id}/health        - Health score
GET    /api/admin/shops/{id}/stats         - Statistiques
PUT    /api/admin/shops/{id}/suspend       - Suspendre
PUT    /api/admin/shops/{id}/activate      - Réactiver
PUT    /api/admin/shops/{id}/plan          - Changer plan

Dernière mise à jour : 2026-07-04
Prochaine revue : Après Phase 2 (v4.2.0) - estimée 2026-07-06

