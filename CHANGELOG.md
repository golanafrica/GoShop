Voici un **CHANGELOG à jour** (v5.2 en tête, restes inchangés en substance). Prêt à coller dans `CHANGELOG.md`.

```markdown
# Changelog

Toutes les modifications notables de ce projet sont documentées dans ce fichier.

Le format est basé sur [Keep a Changelog](https://keepachangelog.com/fr/1.0.0/),
et ce projet adhère au [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [v5.2.0-financial-integrity] - 2026-10-08

### 🚀 Nouveautés Majeures
- **Wallet Grace Expiry Job (P2)** : scheduler + API admin pour détecter les grâces de gel expirées — **audit + notification / preview uniquement** (pas de clawback, debt sweep, unfreeze auto). OFF par défaut (`platform_settings`), kill switch `FREEZE_JOB_HARD_DISABLED`.
- **Intégrité cash-out (P0)** : si le débit wallet a lieu avant un CashOut Yenga en échec → **reverse** synchrone ; webhook `payout.failed` → reverse **idempotent** avec ledger.
- **Webhook pay-in (P1-C)** : `payment_webhooks.processed = true` **uniquement après succès métier** ; en échec métier, `processed` reste false pour permettre le **retry** provider.

### 🛠️ Architecture & Ops
- **Migration auto** : service Compose one-shot `migrate` + `tests/loadtest/scripts/migrate.sh` (`schema_migrations`, skip déjà appliquées) — plus de dépendance exclusive à `initdb.d` pour les nouveaux `.sql`.
- **Freeze job** : tables `wallet_freeze_job_runs` / `wallet_freeze_job_actions`, advisory lock, dry-run / live, idempotence `shop_id` + `frozen_until`.
- **Installment** : ledger `commission_debit` (et `debt_sweep` si câblé) sur release escrow ; wiring `WalletTxnRepo` dans `internal/app/app.go`.
- **Tontine** : alignement crédit **net + held**, redeem + debt sweep (E2E) ; migration `name` sur `tontine_groups` si absente en env legacy.

### 🧪 Tests & Qualité
- `e2e-wallet-grace-expiry-job.ps1` — preview, dry-run, live, idempotence, sécurité finance, RBAC marchand.
- `e2e-tontine-net-held.ps1`, `e2e-tontine-redeem-debt-sweep.ps1`, `e2e-installment-escrow-release.ps1`, `e2e-cash-on-delivery.ps1`.
- Unitaires : `withdrawal_usecase` (reverse), `payment_usecase` (webhook processed lifecycle).
- Auth E2E : login public **`POST /login`** (pas `/api/auth/login`).

### 📚 Documentation
- [docs/13-wallet-grace-expiry-job.md](docs/13-wallet-grace-expiry-job.md) + [docs/adr/ADR-wallet-grace-expiry-job.md](docs/adr/ADR-wallet-grace-expiry-job.md)
- [docs/payment-system.md](docs/payment-system.md) **v5.2** — reverse cash-out, payout webhook, cycle `processed`
- [docs/12-wallet-debt-sweep.md](docs/12-wallet-debt-sweep.md) — compensation retrait
- [docs/11-tontine-system.md](docs/11-tontine-system.md) — net+held / redeem
- [docs/06-migration-plan.md](docs/06-migration-plan.md) — runner Compose

### 🔒 Security / Intéité financière
- Pas de solde « avalé » sur CashOut refusé (reverse P0-A / P0-B).
- Webhooks pay-in rejouables après erreur métier (P1-C).
- Job freeze **sans** impact sur `debt_cents` / ledger clawback.

### 📝 Commits de référence (indicatif)
- `a826c92` — grace expiry job + admin API + E2E + docs
- `07d7418` / `b58a09e` — reverse withdrawal + webhook processed after success
- `ca3d737` — docs finance v5.2
- `a68e341` — auto-migrate Compose

---

## [v5.1.0-debt-sweep] - 2026-10-03

### 🚀 Nouveautés Majeures
- **Dette résiduelle post-clawback (`debt_cents`)** : après un litige `customer_wins` sur escrow déjà `released`, le solde est d’abord débité ; le reste est enregistré en dette explicite (≥ 0), sans freeze automatique.
- **Debt sweep automatique** : les crédits / libérations suivants réduisent la dette avant d’augmenter le disponible — auto-release escrow, `merchant_wins`, release installment, release held tontine, dépôt manuel.
- **Gate retrait anti-fraude** : `POST /api/withdrawals` refusé tant que `debt_cents > 0` (en plus de KYC, freeze et `available_cents`).
- **API wallet enrichie** : `GET /api/wallet` expose `debt_cents`, `debt_formatted`, `withdrawal_blocked` ; `freeze-status` priorise `debt_cents` pour `amount_due_cents`.

### 🛠️ Architecture & Domaine
- **Entity** : `ApplyClawbackToDebt`, `CreditWithDebtSweep`, `ReleaseHeld` + sweep sur disponible (`domain/entity/merchant_wallet.go`).
- **Dispute** : branche post-release clawback + ledger `clawback` / `debt_add` ; `merchant_wins` crédite avec sweep (`resolve_dispute.go`).
- **Installment** : release escrow = `ReleaseHeld` → commission → debt sweep + ledger optionnel via `WithTxnRepo` (`release_escrow_funds.go`, wiring `internal/app/app.go`).
- **Tontine** : `release_held_wallet` applique le même sweep + `WalletTxDebtSweep`.
- **Withdrawal** : fail-fast `DebtCents > 0` dans `create_withdrawal.go`.

### 🧪 Tests & Qualité
- **E2E PowerShell** (racine du repo) :
  - `e2e-dispute-merchant-wins.ps1` — litige pré-release ± inject dette
  - `e2e-debt-sweep-fraud.ps1` — inject SQL + 2e auto-release + ledger `debt_sweep`
  - `e2e-clawback-debt-sweep-chain.ps1` — 2 pay-in réels, clawback → debt → sweep
  - `e2e-clawback-real-payin.ps1` — clawback canal pay-in réel
- Scénarios validés VERT : sweep partiel/total, retrait bloqué sous dette, crédit net sans dette.

### 📚 Documentation
- **Nouveau** : [docs/12-wallet-debt-sweep.md](docs/12-wallet-debt-sweep.md) — modèle, flux, API, E2E, fichiers de référence.

### 🔒 Security / Intéité financière
- Impossible de retirer des fonds tant qu’une dette post-litige n’est pas soldée par les rentrées suivantes.
- Ledger audit : types `clawback`, `debt_add`, `debt_sweep` (idempotence soft sur contraintes uniques).

---

## [v5.0.0-installment-zones] - 2026-09-12

### 🚀 Nouveautés Majeures
- **Système de Zones de Livraison Dynamiques** : 47+ zones pré-configurées (BF, CI, SN, ML, etc.) avec délais personnalisables (min 3j, max 30j).
- **Paiement en Tranches (Installments)** : Workflow complet de configuration, paiement échelonné, et libération différée des fonds en séquestre.
- **Scheduler Auto-Release Intelligent** : Libération automatique basée sur `DeliveredAt + ZoneDelay`, avec fallback de sécurité à 30 jours si `DeliveredAt` est NULL.
- **Blocage par Litige** : Intégration atomique avec le module Dispute. Un litige ouvert passe l'escrow en statut `disputed` et bloque immédiatement toute libération.
- **Notifications Proactives** : Email/WebSocket au marchand (dernière tranche payée) et au client (rappel 24h avant libération).
- **Dashboard Marchand** : API dédiée avec statistiques globales (montants en attente, séquestre, retard) et cache Redis (TTL 5 min) pour des performances optimales.

### 🛠️ Architecture & Base de Données
- **Nouvelles Tables** : `delivery_zones` (avec seed de 47+ zones).
- **Nouvelles Colonnes** : `delivery_zone_id` ajouté à `orders`, `tontine_groups`, `cod_proofs`, et `installment_plans` (prêt pour une future extension).
- **Fix Critique** : Correction des erreurs de scan `NULL` dans `DeliveryZoneRepository` via l'utilisation de `COALESCE` sur les champs optionnels (`region`, `description`).

### 🧪 Tests & Qualité
- **3 Nouveaux Tests E2E** : Validation des délais dynamiques (urbain 5j vs rural 10j), test du fallback 30 jours, et test de blocage par litige.
- **Couverture** : 100% des scénarios critiques du flux de paiement en tranches sont maintenant testés en intégration.

---

## [v4.5.0-production-ready] - 2026-07-21

### 🚨 Sécurité & Durcissement (CRITIQUE)
- **Validation du Secret JWT** : Le serveur refuse désormais de démarrer si `JWT_SECRET` est absent ou fait moins de 32 caractères.
- **Coût Bcrypt Forcé** : Imposition d'un coût de hachage minimum de 10, ignorant les configurations d'environnement dangereuses (ex: `BCRYPT_COST=4`).
- **Prévention d'Injection SQL** : Ajout d'une whitelist stricte (`allowedSortColumns`) pour tous les paramètres `sort_by` dans les repositories (ex: `shop_repository.go`).
- **Prévention IDOR (Multi-tenant)** : Nouveau middleware `RequireShopAccess` garantissant qu'un utilisateur ne peut accéder/modifier que les ressources des boutiques dont il est propriétaire ou collaborateur.
- **Logs Sécurisés** : Suppression complète des logs de `raw_body` brut dans les handlers d'authentification et de création de client pour empêcher toute fuite de mot de passe en clair.
- **Purge de l'Historique Git** : Suppression définitive des fichiers SQL sensibles (`fix_admin_password.sql`, etc.) de *tous* les commits passés via `git filter-branch` et `git gc`.

### 🔔 Notifications en Temps Réel (WebSocket)
- **WebSocket Hub Scalable** : Implémentation d'une infrastructure WebSocket utilisant Redis Pub/Sub pour supporter le scaling horizontal.
- **Notification Dispatcher** : Nouveau service orchestrant l'envoi d'événements temps réel (commande confirmée, rejetée, livrée, etc.) aux clients connectés.
- **Liaison User ↔ Customer** : Ajout de la colonne `user_id` dans la table `customers` (Migration `033`) pour router avec précision les notifications WebSocket vers l'utilisateur authentifié derrière un profil client.
- **Nouvel Endpoint** : `GET /ws/notifications 🔒` pour que les clients s'abonnent aux mises à jour en temps réel.

### 🛠️ Architecture & Base de Données
- **Correction Contrainte de Rôle** : La migration `034` met à jour `users_role_check` pour autoriser explicitement le rôle `'user'`, corrigeant les échecs d'inscription publique.
- **Rôle par Défaut Sécurisé** : L'inscription publique assigne désormais correctement le rôle `'user'` au lieu de `'merchant'`, empêchant toute élévation de privilèges.
- **Repository Customer** : Mise à jour de toutes les requêtes (`Create`, `FindByID`, `FindByEmail`, `FindAll`) pour persister et récupérer correctement le nouveau champ `user_id`.

### 🧪 Tests & Outillage
- **Test E2E WebSocket** : Ajout du script `Test-WebSocket-Notification.ps1` pour automatiser et valider le flux complet de notification en temps réel (Création User → Shop → Produit → Commande → Acceptation → Réception payload WebSocket).

---

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
   ```

   Aucun changement d'API breaking : rétrocompatible.  
   Nouvelles routes : voir [docs/11-tontine-system.md](docs/11-tontine-system.md) et [docs/KYC.md](docs/KYC.md).

### 📚 Documentation
- [docs/11-tontine-system.md](docs/11-tontine-system.md)
- [docs/KYC.md](docs/KYC.md)
- [docs/payment-system.md](docs/payment-system.md)

---

## 🔒 Breaking Changes - Security

### API Key Transport (v4.5.1)

**⚠️ BREAKING:** API keys passed via query parameter `?api_key=...` are no longer accepted.

**Why:** Query parameters are logged by servers, proxies, CDNs, and browsers, exposing your API key to unauthorized parties.

**Migration Required:**

❌ **Before (insecure):**

```http
GET https://api.goshop.com/api/products?api_key=gsk_live_xxx
```

✅ **After (secure - Option 1):**

```http
GET https://api.goshop.com/api/products
X-API-Key: gsk_live_xxx
```

✅ **After (secure - Option 2):**

```http
GET https://api.goshop.com/api/products
Authorization: Bearer gsk_live_xxx
```

**Error Code:** `API_KEY_INSECURE_TRANSPORT` (HTTP 401)

**Action Required:** Update your API client code to use HTTP headers instead of query parameters.

---

## Annexes — génération de secrets (dev)

```bash
# JWT Secret (64 caractères hex)
openssl rand -hex 32

# Refresh Secret (76 caractères base64)
openssl rand -base64 48

# Encryption Key (32 caractères)
openssl rand -base64 32 | Select-Object -First 1 | ForEach-Object { $_.Substring(0, [Math]::Min(32, $_.Length)) }

# Database Password (PowerShell)
-join ((65..90) + (97..122) + (48..57) + (33..47) | Get-Random -Count 24 | ForEach-Object {[char]$_})
```
