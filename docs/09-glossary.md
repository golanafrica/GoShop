# 📖 Glossaire (GoShop v5.1.0)

**Version** : v5.1.0  
**Dernière mise à jour** : 2026-10-03

Ce glossaire définit les termes métier, techniques et architecturaux utilisés dans le projet GoShop. Il est organisé par domaine pour faciliter la consultation.

---

## 📋 Table des matières
1. [Concepts Métier](#1-concepts-métier)
2. [Architecture & Design](#2-architecture--design)
3. [Sécurité & Authentification](#3-sécurité--authentification)
4. [Paiements & Finance](#4-paiements--finance)
5. [Multi-tenant & Infrastructure](#5-multi-tenant--infrastructure)
6. [Tests & Qualité](#6-tests--qualité)
7. [Outils & Technologies](#7-outils--technologies)

---

## 1. Concepts Métier

| Terme | Définition |
| ----- | ---------- |
| **Boutique (Shop)** | Espace en ligne d'un marchand, avec ses produits, commandes, paramètres. Entité racine du multi-tenant. |
| **Marchand (Merchant)** | Propriétaire d'une boutique, gère produits, commandes, paiements. Rôle `merchant` dans le RBAC. |
| **Client (Customer)** | Personne qui achète dans une boutique. Peut être connecté (compte `User` lié) ou anonyme. Profil isolé par `shop_id`. |
| **Utilisateur (User)** | Identité numérique authentifiée dans le système. Peut être marchand, client, admin, etc. |
| **Collaborateur (Collaborator)** | Utilisateur invité à gérer une boutique (rôle `shop_admin`, `manager`, `support`) ou la plateforme. |
| **Produit (Product)** | Article vendu dans une boutique. Stocké en centimes FCFA, avec gestion de stock. |
| **Commande (Order)** | Demande d'achat passée par un client. Cycle de vie typique : confirmation → livraison → (éventuel litige). |
| **Cash on Delivery (COD)** | Paiement à la livraison. Workflow avec preuve de livraison et collecte de commission. |
| **Crédit** | Paiement en plusieurs fois (acompte + échéances). Conforme BCEAO (max 15% d'intérêt annuel). Distinct du **paiement en tranches** escrow. |
| **Acompte (Down Payment)** | Premier paiement à la commande (ex. 30% du total). Obligatoire pour activer un crédit. |
| **Échéance (Installment crédit)** | Date à laquelle une tranche de **crédit BCEAO** doit être payée. Pénalités possibles en retard. |
| **Paiement en tranches (Installment order)** | Commande payée par tranches avec fonds en **escrow** jusqu'à délai zone + livraison ; release via scheduler ou usecase dédié (v5.x). |
| **Score de fiabilité** | Indicateur de ponctualité des paiements d'un client. Influence l'éligibilité au crédit / tranches. |
| **Plan de paiement (Credit Plan)** | Template marchand (ex. « 3 fois sans frais ») : `down_payment_pct`, `installments_count`, `interest_rate_pct`. |
| **Tontine** | Épargne collective pour un bien physique à tour de rôle. |
| **Voucher** | Bon de livraison numérique (12 caractères, validité 6 mois) en fin de cycle tontine. Mono-boutique. |
| **Cycle (Tontine)** | Période où tous les participants paient leur cotisation ; un bénéficiaire par cycle. |
| **Code d'invitation** | Code unique 8 caractères pour rejoindre un groupe de tontine. |
| **KYC (Know Your Customer)** | Vérification d'identité (CNI, passeport). Requis pour tontine, crédit, et **retraits** marchand. |
| **Commission** | Frais plateforme (COD, online, tontine, etc.) — taux selon canal / config. |
| **Batch (Commission)** | Regroupement de commissions traité par le scheduler. |
| **Wallet (Portefeuille)** | Compte virtuel marchand (`merchant_wallets`) : `balance_cents`, `held_cents`, `debt_cents`. Montants en centimes FCFA. |
| **Held (Fonds séquestrés)** | Partie non retirable (`held_cents`) tant que l'escrow ou le hold tontine n'est pas libéré. |
| **Disponible (Available)** | `max(0, balance_cents − held_cents)`. Montant éligible au retrait **si** `debt_cents = 0`. |
| **Dette résiduelle (`debt_cents`)** | Montant dû après clawback post-release si le solde ne couvrait pas tout. Toujours ≥ 0. **Bloque les retraits**. |
| **Clawback** | Récupération des fonds déjà crédités au marchand si le client gagne un litige **après** libération escrow. Priorité : balance puis `debt_cents`. Pas de freeze automatique. |
| **Debt sweep** | Remboursement auto de `debt_cents` sur le prochain crédit ou release (auto-release, merchant_wins, installment, tontine release_held). Ledger : `debt_sweep`. |
| **Escrow (Séquestre)** | Fonds bloqués pour une commande jusqu'à délai zone + livraison (ou résolution litige). Statuts : `funds_held`, `disputed`, `released`, `refunded`. |
| **Litige (Dispute)** | Contestation client. Pré-release : bloque l'auto-release. Post-release `customer_wins` : clawback + éventuelle dette. `merchant_wins` : crédit marchand (+ sweep si dette). |
| **Auto-release** | Job scheduler qui libère l'escrow éligible (délai écoulé, pas de litige actif) et crédite le wallet avec debt sweep si besoin. |
| **Retrait (Withdrawal)** | Transfert wallet → Mobile Money. Exige KYC validé, wallet non gelé, **`debt_cents = 0`**, montant ≤ disponible. |
| **Withdrawal blocked** | Règle / flag API : retrait impossible si dette, gel, ou disponible ≤ 0. |

---

## 2. Architecture & Design

| Terme | Définition |
| ----- | ---------- |
| **Clean Architecture** | Séparation des couches : Domain → Application → Interfaces → Infrastructure. |
| **DDD (Domain-Driven Design)** | Conception centrée domaine : Entités, Value Objects, Agrégats, Repositories. |
| **Entité (Entity)** | Objet métier avec identité unique (UUID). Ex. : `Order`, `Customer`, `Shop`. |
| **Value Object** | Objet sans identité propre, défini par ses attributs. Ex. : `Money`. |
| **Agrégat (Aggregate)** | Groupe d'entités traitées comme une unité. Ex. : `Order` + `OrderItems`. |
| **Repository** | Interface de persistance ; implémentation dans l'infrastructure (PostgreSQL). |
| **Use Case (Cas d'usage)** | Action métier orchestrant repositories. Ex. : `CreateCustomerUsecase`. |
| **DTO (Data Transfer Object)** | Transfert de données entre couches (requête HTTP → use case). |
| **Handler** | Composant HTTP : requête → use case → JSON. |
| **Middleware** | Intercepteur HTTP transversal (auth, logs, multi-tenant). |
| **Dependency Injection (DI)** | Dépendances injectées depuis l'extérieur. Centralisé dans `internal/app/app.go`. |
| **Transaction (DB)** | Opération atomique : `BeginTx` → ops → `Commit` / `Rollback`. |
| **Soft Delete** | Suppression logique via `deleted_at`. |

---

## 3. Sécurité & Authentification

| Terme | Définition |
| ----- | ---------- |
| **JWT (JSON Web Token)** | Token signé (HS256) : `access_token` (court) + `refresh_token` (long). |
| **JWT_SECRET** | Clé de signature JWT. **Obligatoire**, **≥ 32 caractères** (v4.5.0). |
| **RBAC** | Contrôle d'accès par rôles (`super_admin`, `admin`, `merchant`, `user`, etc.). |
| **2FA (TOTP)** | Authentification à deux facteurs (Google Authenticator, codes de récupération). |
| **API Key** | Auth intégrations tierces (`gsk_live_...`), scopes granulaires. Header uniquement (pas query string). |
| **Session** | Connexion active ; révocable unitairement ou globalement. |
| **Bcrypt** | Hash mots de passe ; coût minimum **10** en production. |
| **HMAC-SHA256** | Signature des webhooks de paiement. |
| **Rate Limiting** | Limitation de débit (Redis, fallback mémoire). |
| **IDOR** | Accès non autorisé à une ressource par ID. Mitigé par `RequireShopAccess`. |
| **RequireShopAccess** | Middleware : propriétaire ou collaborateur de la boutique. |
| **TenantResolver** | Résout la boutique active (`X-Shop-Slug` ou `Host`). |
| **CORS** | Accès cross-origin navigateur → API. |
| **Recovery Middleware** | Capture les panics Go → HTTP 500 propre. |

---

## 4. Paiements & Finance

| Terme | Définition |
| ----- | ---------- |
| **Provider** | Service de paiement externe (Wave, Orange Money, Moov Money, Yenga Pay). |
| **Webhook** | POST provider → GoShop pour notifier un statut de paiement. |
| **USSD** | Code court Mobile Money (ex. `#144*111#`). |
| **Centimes FCFA** | Unité de stockage : **1 FCFA = 100 centimes**. Toujours `int64`. |
| **Machine à états (Payment)** | `pending` → `processing` → `success` / `failed` / `refunded`. |
| **Preuve de livraison (COD Proof)** | Preuve que le client a reçu / payé en cash. |
| **BCEAO** | Banque centrale UEMOA ; plafond d'intérêt crédit (ex. 15% annuel). |
| **Réconciliation** | Alignement écritures internes ↔ fonds réellement reçus. |
| **CreditWithDebtSweep** | Crédit wallet qui rembourse d'abord `debt_cents` avant d'augmenter le net disponible. |
| **ApplyClawbackToDebt** | Débite la balance puis affecte le reliquat à `debt_cents` (sans freeze auto). |
| **Ledger clawback / debt_add / debt_sweep** | Types de `wallet_transactions` pour auditer litige et recovery dette. |

---

## 5. Multi-tenant & Infrastructure

| Terme | Définition |
| ----- | ---------- |
| **Multi-tenant** | Plusieurs boutiques partagent la même app et la même DB. |
| **Option B (Phase 1)** | Isolation par colonne `shop_id` + filtrage applicatif. |
| **Option A (Phase 2)** | Schéma PostgreSQL distinct par boutique (cible future). |
| **ShopSlug** | Identifiant lisible unique (header `X-Shop-Slug`). |
| **Custom Domain** | Domaine boutique (ex. `shop.maboutique.com`). |
| **Redis Pub/Sub** | Diffusion notifications WebSocket multi-instances. |
| **WebSocket Hub** | Gestion des connexions WS persistantes. |
| **Notification Dispatcher** | Orchestration WS + email (+ Telegram si configuré). |
| **Scheduler (Cron)** | Jobs périodiques : commissions, tontine, **escrow auto-release**, **installment auto-release**, relances. |
| **Migration (DB)** | Script SQL versionné (`golang-migrate`). |
| **Idempotence** | Réexécution sans effet de bord au-delà de la 1re application. |
| **Zero-downtime deployment** | Déploiement sans interruption de service. |
| **Rollback** | Annulation migration ou déploiement. |

---

## 6. Tests & Qualité

| Terme | Définition |
| ----- | ---------- |
| **Test unitaire** | Test isolé avec mocks. Rapide. |
| **Test d'intégration** | Composants + vraie DB. |
| **Test E2E** | Scénario complet API + DB + Redis (Go tags ou scripts PowerShell). |
| **E2E PowerShell (finance)** | Scripts racine : `e2e-dispute-merchant-wins.ps1`, `e2e-debt-sweep-fraud.ps1`, `e2e-clawback-debt-sweep-chain.ps1`, `e2e-clawback-real-payin.ps1`. |
| **Test de charge** | Charge multi-utilisateurs (**k6**). |
| **Mock** | Dépendance simulée (`gomock`). |
| **Fixture** | Données de test réutilisables. |
| **Coverage** | % de code exécuté par les tests ; objectif élevé sur modules finance. |
| **Race Detector** | `go test -race`. |
| **Testify** | Assertions Go (`assert.NoError`, etc.). |
| **Conventional Commits** | `feat:`, `fix:`, `docs:`, `test:`, `chore:`, `security:`. |

---

## 7. Outils & Technologies

| Terme | Définition |
| ----- | ---------- |
| **Go (Golang)** | Langage backend GoShop (voir `go.mod` pour la version exacte). |
| **Chi** | Router HTTP. |
| **PostgreSQL** | DB relationnelle (16+). |
| **Redis** | Cache, rate limit, sessions, Pub/Sub WS. |
| **Zerolog** | Logs JSON structurés. |
| **Prometheus** | Métriques (`/metrics`). |
| **Loki / Promtail** | Agrégation de logs. |
| **Grafana** | Dashboards métriques / logs. |
| **Docker** | Conteneurisation. |
| **Kubernetes (K8s)** | Orchestration production. |
| **k6** | Tests de charge. |
| **golang-migrate** | Migrations SQL. |
| **gomock** | Mocks d'interfaces. |
| **Swagger** | Doc API (`/swagger/index.html`). |
| **GitHub Actions** | CI/CD. |

---

## 📚 Références complémentaires

- [Architecture GoShop](01-architecture.md)
- [Modèle de données](02-domain-model.md)
- [Wallet — dette, clawback & sweep](12-wallet-debt-sweep.md)
- [Stratégie Multi-tenant](04-multi-tenant.md)
- [Plan de Migration](06-migration-plan.md)
- [Guide des Tests](08-testing-guide.md)

---

**Dernière mise à jour** : 2026-10-03