
---

##  `CHANGELOG.md` (AJOUTER v2.9.0 EN HAUT)

**Ajouter au tout début du fichier** (après le header) :

```markdown
## [v2.9.0-tontine-kyc] - 2026-06-29

### 🎉 Added

#### Système de Tontine (Biens physiques)
- **Tontine de biens physiques** : Système complet permettant à un groupe de cotiser pour acquérir un bien à tour de rôle
- **5 entités domaine** : `TontineGroup`, `TontineParticipant`, `TontinePayment`, `TontineVoucher`, `ProductTontineSettings`
- **5 repositories Postgres** : Implémentations complètes avec support multi-tenant et transactions
- **4 usecases tontine** :
  - `CreateTontineGroupUsecase` : Création de groupe (marchand ou client)
  - `JoinTontineGroupUsecase` : Rejointure par code d'invitation
  - `PayCycleUsecase` : Paiement de cotisation via YengaPay
  - `ListCustomerPaymentsUsecase` : Historique des paiements
- **3 handlers HTTP** :
  - `TontineHandler` : Endpoints groupes (create, join, pay, list)
  - `TontineSettingsHandler` : Configuration par boutique
  - `KYCHandler` : Upload et validation KYC
- **Code d'invitation** : 8 caractères alphanumériques uniques (`crypto/rand`)
- **Système de commission** : Configurable 0-15% (défaut 2.50%) en basis points
- **Voucher de livraison** : Code 12 caractères, validité 6 mois, mono-boutique
- **Machine à états** : `PENDING_MEMBERS` → `ACTIVE` → `COMPLETED`

#### Système KYC (Know Your Customer)
- **Upload de documents** : CNI, passeport, autres (max 5 Mo, JPG/PNG/PDF)
- **Workflow de validation** : Marchand approuve/rejette avec raison
- **4 niveaux KYC** : `none`, `pending`, `verified`, `rejected`
- **2 usecases KYC** :
  - `UploadKYCDocumentUsecase` : Upload avec validation (taille, MIME, max 3 docs)
  - `ReviewKYCUsecase` : Validation/rejet par le marchand
- **KYC obligatoire** : Pour participer à une tontine
- **Liste des KYC en attente** : Pour le dashboard marchand

#### Intégration Webhook Tontine
- **Détection automatique** : Préfixe `TONTINE:` dans la référence YengaPay
- **ProcessTontineWebhookUsecase** : Traitement dédié des webhooks tontine
- **Complétion automatique** : Génération voucher quand tous les participants ont payé
- **Transition de cycle** : Passage automatique au cycle suivant
- **Complétion groupe** : Statut `COMPLETED` au dernier cycle

#### Base de données
- **Migration 010** : 5 tables tontine + 14 index de performance
- **Migration 011** : Table `customer_kyc_documents` + champs KYC sur `customers`
- **Migration 012** : Ajout statut `paid` à la contrainte `orders_status_check`
- **Modifications** : `shop_payment_settings` (tontine_enabled, tontine_commission_rate)

#### Endpoints HTTP
- `GET /api/shops/{id}/tontine-settings?product_id=...` - Lire config tontine
- `PUT /api/shops/{id}/tontine-settings` - Configurer tontine
- `POST /api/tontine/groups` - Créer un groupe
- `POST /api/tontine/groups/join` - Rejoindre par code
- `POST /api/tontine/groups/{id}/pay` - Payer cotisation
- `GET /api/tontine/groups/{id}/payments?customer_id=...` - Historique paiements
- `POST /api/customers/kyc/upload` - Upload document KYC
- `GET /api/customers/{id}/kyc/status` - Statut KYC
- `GET /api/merchant/kyc/pending` - Liste KYC en attente
- `POST /api/merchant/kyc/{id}/review` - Valider/Rejeter KYC

#### Tests E2E (5 nouveaux tests)
- **TestTontineWorkflowE2E** : Workflow complet (11 étapes, ~9s)
- **TestTontineWebhookSimulation** : Vérification détection TONTINE:
- **TestTontineSettingsE2E** : Configuration par boutique
- **TestTontineValidationRules** : 6 validations (cycles, types, taille, MIME)
- **TestTontineCommissionCalculation** : 6 calculs de commission

### 🔧 Changed

#### Architecture
- **ConfigureTontineUsecase** : Ne dépend plus de `tenant.FromContext` (ShopID explicite)
- **TontineSettingsHandler** : Injection du tenant via `tenant.WithTenant()`
- **ProcessWebhookUsecase** : Délègue à `ProcessTontineWebhookUsecase` si préfixe TONTINE:
- **CheckPaymentStatusUsecase** : Statut commande → `paid` (minuscules)

#### Base de données
- **orders_status_check** : Ajout du statut `paid` (migration 012)
- **Idempotence** : Toutes les migrations utilisent `IF NOT EXISTS`

### 🐛 Fixed

#### Critique
- **Webhook tontine URLs** : Utilisation de `client.BaseURL` dans les tests
- **Statut commande** : `PAID` → `paid` pour respecter la contrainte DB
- **Migration 008** : Index `idx_withdrawals_shop_id` rendu idempotent
- **Migration 011** : Contrainte `customers_kyc_level_check` avec `DO $$ ... $$`

### 🔒 Security

- **Multi-tenant isolation** : Toutes les routes tontine filtrent par `shop_id`
- **KYC obligatoire** : Rejet automatique des clients non vérifiés
- **Voucher mono-boutique** : Validation `shop_id` à la redemption
- **Codes uniques** : `crypto/rand` pour `invite_code` et `voucher_code`
- **Validation HMAC** : Webhooks YengaPay signés
- **File upload validation** : Taille max 5 Mo, MIME types autorisés

### 📊 Performance

- **TestTontineWorkflowE2E** : ~9s pour 11 étapes complètes
- **TestTontineSettingsE2E** : ~10s
- **TestTontineValidationRules** : ~9s
- **Commission calculation** : < 1ms

### 📝 Migration Guide

#### Depuis v2.8.x

1. **Appliquer les migrations 010, 011, 012** :
   ```bash
   psql -U postgres -d goshop_db -f migrations/010_add_tontine.sql
   psql -U postgres -d goshop_db -f migrations/011_add_kyc.sql
   psql -U postgres -d goshop_db -f migrations/012_add_paid_status.sql

   Aucun changement d'API breaking : Rétrocompatible
Nouvelles routes disponibles : Voir docs/11-tontine-system.md et docs/KYC.md
📚 Documentation
docs/11-tontine-system.md : Guide complet du système tontine
docs/KYC.md : Guide du système KYC
docs/payment-system.md : Mis à jour avec intégration tontine


