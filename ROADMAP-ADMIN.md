# 🗺️ Roadmap Admin - GoShop v4.0.0+

**Date** : 2026-07-02  
**Statut** : 📋 Planifié  
**Priorité** : 🔴 Critique pour la production

---

## 🎯 Contexte

La version v3.5.0 (commit actuel) a implémenté avec succès :
- ✅ 4 schedulers automatiques (COD, Online, Tontine, Credit)
- ✅ Helpers monétaires centralisés (`utils.FormatMoney`)
- ✅ Batch tracking avec audit complet
- ✅ 43 tests unitaires pour les helpers

**Mais il manque toute la couche admin** pour gérer la plateforme efficacement.

---

## 📊 Audit de l'existant

### ✅ Ce qui existe (Routes Admin)

| Module | Endpoints | Statut |
|--------|-----------|--------|
| **Schedulers** | `/api/admin/scheduler/*` | ✅ Complet |
| **Commission Rates** | `/api/admin/commission-rates/*` | ✅ Complet |
| **Gestion boutiques** | `/api/shops/*` | ⚠️ Basique |

### ❌ Ce qui manque (Critique)

| Catégorie | Manquant | Priorité |
|-----------|----------|----------|
| **Gestion Users** | CRUD admins, rôles, permissions | 🔴 Critique |
| **Middleware RBAC** | Protection des routes admin | 🔴 Critique |
| **Audit Trail** | Traçabilité des actions admin | 🔴 Critique |
| **Dashboard** | Vue d'ensemble plateforme | 🔴 Critique |
| **Monitoring** | Métriques temps réel | 🟡 Important |
| **Rapports** | Financiers, BCEAO | 🟡 Important |
| **Sécurité** | Détection fraude, blacklist | 🟡 Important |

---

## 🚀 Plan d'implémentation

### Phase 1 : Fondations (v4.0.0) - 2 jours

**Objectif** : Sécuriser et tracer les actions admin

#### 1.1 Middleware RBAC
- [ ] `interfaces/middl/rbac.go`
- [ ] Fonction `RequireRole(roles ...string)`
- [ ] Intégration avec les rôles existants (super_admin, admin, etc.)

#### 1.2 Audit Trail
- [ ] Migration `018_add_admin_audit_logs.sql`
- [ ] Entity `AdminAuditLog`
- [ ] Repository `AdminAuditLogRepository`
- [ ] Middleware `AuditMiddleware` (log automatique)

#### 1.3 Gestion des utilisateurs admin
- [ ] `GET /api/admin/users` - Liste tous les users
- [ ] `POST /api/admin/users` - Créer un admin
- [ ] `GET /api/admin/users/{id}` - Détails
- [ ] `PUT /api/admin/users/{id}/role` - Changer rôle
- [ ] `PUT /api/admin/users/{id}/status` - Activer/désactiver
- [ ] `POST /api/admin/users/{id}/reset-password`

#### 1.4 Script d'initialisation
- [ ] Créer le premier super_admin
- [ ] Commande CLI : `go run cmd/cli/create_super_admin.go`

**Livrables** :
- ✅ Middleware RBAC fonctionnel
- ✅ Audit trail complet
- ✅ CRUD admins basique
- ✅ Super admin créé

---

### Phase 2 : Gestion cross-tenant (v4.1.0) - 2 jours

**Objectif** : Permettre aux admins de gérer toutes les boutiques

#### 2.1 Liste cross-tenant
- [ ] `GET /api/admin/shops` - Liste TOUTES les shops (pas filtré par tenant)
- [ ] Pagination + filtres (status, plan, date)

#### 2.2 Actions sur les shops
- [ ] `PUT /api/admin/shops/{id}/suspend` - Suspendre
- [ ] `PUT /api/admin/shops/{id}/activate` - Réactiver
- [ ] `GET /api/admin/shops/{id}/stats` - Statistiques détaillées
- [ ] `GET /api/admin/shops/{id}/transactions` - Historique

#### 2.3 Gestion des abonnements
- [ ] `PUT /api/admin/shops/{id}/plan` - Changer de plan (free/pro/business)
- [ ] `GET /api/admin/shops/{id}/billing` - Facturation

**Livrables** :
- ✅ Gestion complète des shops
- ✅ Suspension/activation
- ✅ Stats par shop

---

### Phase 3 : Dashboard & Monitoring (v4.2.0) - 3 jours

**Objectif** : Vue d'ensemble de la plateforme en temps réel

#### 3.1 Dashboard global
- [ ] `GET /api/admin/dashboard/overview`
  - Total users, shops, transactions
  - Revenus du jour/semaine/mois
  - Alertes actives
  
#### 3.2 Métriques temps réel
- [ ] `GET /api/admin/dashboard/metrics`
  - Taux de succès paiements
  - Volume transactions
  - Commissions collectées
  - Performance schedulers

#### 3.3 Graphiques
- [ ] `GET /api/admin/dashboard/charts/revenue`
- [ ] `GET /api/admin/dashboard/charts/users`
- [ ] `GET /api/admin/dashboard/charts/transactions`

#### 3.4 Alertes configurables
- [ ] `GET /api/admin/alerts` - Liste alertes
- [ ] `POST /api/admin/alerts` - Créer alerte
- [ ] `PUT /api/admin/alerts/{id}` - Modifier
- [ ] Types : email, SMS, webhook

**Livrables** :
- ✅ Dashboard complet
- ✅ Métriques temps réel
- ✅ Système d'alertes

---

### Phase 4 : Rapports & Conformité (v4.3.0) - 2 jours

**Objectif** : Rapports financiers et conformité BCEAO

#### 4.1 Rapports financiers
- [ ] `GET /api/admin/reports/financial`
  - Revenus par période
  - Commissions par type
  - Top boutiques
  - Tendances

#### 4.2 Rapports BCEAO
- [ ] `GET /api/admin/reports/bceao`
  - Transactions suspectes
  - Montants > seuil
  - Activités inhabituelles

#### 4.3 Export
- [ ] `POST /api/admin/reports/export`
  - Format : CSV, PDF, Excel
  - Filtres : date, shop, type

#### 4.4 Rapports planifiés
- [ ] `POST /api/admin/reports/schedule`
  - Envoi automatique (quotidien/hebdomadaire/mensuel)
  - Destinataires : email

**Livrables** :
- ✅ Rapports financiers complets
- ✅ Conformité BCEAO
- ✅ Export multi-format

---

### Phase 5 : Sécurité avancée (v4.4.0) - 2 jours

**Objectif** : Détection de fraude et protection

#### 5.1 Audit log avancé
- [ ] `GET /api/admin/security/audit-log`
  - Filtres : user, action, date
  - Export

#### 5.2 Détection d'activités suspectes
- [ ] `GET /api/admin/security/suspicious`
  - Connexions inhabituelles
  - Transactions suspectes
  - Patterns anormaux

#### 5.3 Blacklist
- [ ] `PUT /api/admin/security/blacklist/{user_id}`
- [ ] `DELETE /api/admin/security/blacklist/{user_id}`
- [ ] `GET /api/admin/security/blacklist`

#### 5.4 Gestion des gels
- [ ] `GET /api/admin/security/freeze-requests`
- [ ] `PUT /api/admin/security/freeze/{shop_id}`
- [ ] `PUT /api/admin/security/unfreeze/{shop_id}`

**Livrables** :
- ✅ Audit log complet
- ✅ Détection fraude
- ✅ Blacklist fonctionnelle

---

### Phase 6 : Notifications & Settings (v4.5.0) - 2 jours

**Objectif** : Communication et configuration globale

#### 6.1 Notifications admin
- [ ] `GET /api/admin/notifications` - Liste
- [ ] `POST /api/admin/notifications/broadcast` - Notification globale
- [ ] `PUT /api/admin/notifications/{id}/read` - Marquer lu

#### 6.2 Feature flags
- [ ] `GET /api/admin/settings/features` - Liste features
- [ ] `PUT /api/admin/settings/features/{name}` - Activer/désactiver
- [ ] Exemples : `tontine_enabled`, `credit_enabled`, `wave_enabled`

#### 6.3 Configuration globale
- [ ] `GET /api/admin/settings` - Voir config
- [ ] `PUT /api/admin/settings` - Modifier
- [ ] Paramètres : taux par défaut, limites, seuils

#### 6.4 Maintenance
- [ ] `POST /api/admin/maintenance/enable` - Mode maintenance
- [ ] `POST /api/admin/maintenance/disable` - Désactiver
- [ ] Message personnalisé pour utilisateurs

**Livrables** :
- ✅ Système de notifications
- ✅ Feature flags
- ✅ Configuration globale

---

## 📅 Timeline estimée

| Phase | Version | Durée | Date cible |
|-------|---------|-------|------------|
| **Phase 1** | v4.0.0 | 2 jours | 2026-07-04 |
| **Phase 2** | v4.1.0 | 2 jours | 2026-07-06 |
| **Phase 3** | v4.2.0 | 3 jours | 2026-07-09 |
| **Phase 4** | v4.3.0 | 2 jours | 2026-07-11 |
| **Phase 5** | v4.4.0 | 2 jours | 2026-07-13 |
| **Phase 6** | v4.5.0 | 2 jours | 2026-07-15 |

**Total** : 13 jours (2 semaines et demie)

---

## 🎯 Priorités immédiates

### 🔴 À faire MAINTENANT (Phase 1)

1. **Middleware RBAC** - Essentiel pour sécuriser
2. **Audit trail** - Conformité BCEAO
3. **Gestion users** - Créer des admins
4. **Super admin** - Script d'initialisation

### 🟡 À faire BIENTÔT (Phase 2-3)

1. **Gestion cross-tenant** - Suspendre/activer shops
2. **Dashboard** - Vue d'ensemble
3. **Métriques** - Monitoring temps réel

### 🟢 À faire PLUS TARD (Phase 4-6)

1. **Rapports** - Financiers, BCEAO
2. **Sécurité** - Détection fraude
3. **Notifications** - Alertes admin

---

## 📚 Ressources

- [Architecture GoShop](docs/01-architecture.md)
- [Multi-tenant](docs/04-multi-tenant.md)
- [Sécurité](docs/security.md) (à créer)
- [RBAC Guide](docs/rbac.md) (à créer)

---

## ✅ Checklist de progression

- [ ] Phase 1 terminée (v4.0.0)
- [ ] Phase 2 terminée (v4.1.0)
- [ ] Phase 3 terminée (v4.2.0)
- [ ] Phase 4 terminée (v4.3.0)
- [ ] Phase 5 terminée (v4.4.0)
- [ ] Phase 6 terminée (v4.5.0)
- [ ] Documentation complète
- [ ] Tests E2E admin
- [ ] Déploiement production

---

**Dernière mise à jour** : 2026-07-02  
**Prochaine revue** : Après Phase 1 (v4.0.0)