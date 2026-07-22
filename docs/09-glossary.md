

---

```markdown
# 📖 Glossaire (GoShop v4.5.0)

**Version** : v4.5.0  
**Dernière mise à jour** : 2026-07-21

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
|-------|------------|
| **Boutique (Shop)** | Espace en ligne d'un marchand, avec ses produits, commandes, paramètres. Entité racine du multi-tenant. |
| **Marchand (Merchant)** | Propriétaire d'une boutique, gère produits, commandes, paiements. Rôle `merchant` dans le RBAC. |
| **Client (Customer)** | Personne qui achète dans une boutique. Peut être connecté (compte `User` lié) ou anonyme. Profil isolé par `shop_id`. |
| **Utilisateur (User)** | Identité numérique authentifiée dans le système. Peut être marchand, client, admin, etc. |
| **Collaborateur (Collaborator)** | Utilisateur invité à gérer une boutique (rôle `shop_admin`, `manager`, `support`) ou la plateforme. |
| **Produit (Product)** | Article vendu dans une boutique. Stocké en centimes FCFA, avec gestion de stock. |
| **Commande (Order)** | Demande d'achat passée par un client. Possède un cycle de vie : `pending_confirmation` → `confirmed` → `delivered`. |
| **Cash on Delivery (COD)** | Paiement à la livraison. Workflow complet avec preuve de livraison et collecte de commission automatique. |
| **Crédit** | Paiement en plusieurs fois : acompte + échéances mensuelles. Conforme BCEAO (max 15% d'intérêt annuel). |
| **Acompte (Down Payment)** | Premier paiement effectué à la commande (ex: 30% du total). Obligatoire pour activer un crédit. |
| **Échéance (Installment)** | Date à laquelle une tranche de crédit doit être payée. Génère des pénalités si en retard. |
| **Score de fiabilité** | Indicateur de la ponctualité des paiements d'un client (Excellent → Mauvaise). Détermine l'éligibilité au crédit. |
| **Plan de paiement (Credit Plan)** | Template défini par le marchand (ex: "3 fois sans frais"). Paramètres : `down_payment_pct`, `installments_count`, `interest_rate_pct`. |
| **Tontine** | Système d'épargne collective permettant à un groupe de cotiser pour acquérir un bien physique à tour de rôle. |
| **Voucher** | Bon de livraison numérique (12 caractères, validité 6 mois) généré à la fin d'un cycle de tontine. Mono-boutique. |
| **Cycle (Tontine)** | Période pendant laquelle tous les participants d'une tontine paient leur cotisation. Un bénéficiaire est désigné par cycle. |
| **Code d'invitation** | Code unique de 8 caractères alphanumériques permettant de rejoindre un groupe de tontine. |
| **KYC (Know Your Customer)** | Processus de vérification d'identité (CNI, passeport) obligatoire pour participer à une tontine ou demander un crédit. |
| **Commission** | Frais prélevés par GoShop sur chaque transaction (COD : 2.5%, Tontine : configurable 0-15%). |
| **Batch (Commission)** | Regroupement de commissions collectées traité périodiquement par le scheduler. |
| **Wallet (Portefeuille)** | Compte virtuel du marchand où sont crédités les paiements reçus. Solde en centimes FCFA. |
| **Retrait (Withdrawal)** | Demande de transfert de fonds du wallet vers un compte Mobile Money. Nécessite KYC validé. |

---

## 2. Architecture & Design

| Terme | Définition |
|-------|------------|
| **Clean Architecture** | Architecture logicielle avec séparation stricte des couches : Domain → Application → Interfaces → Infrastructure. |
| **DDD (Domain-Driven Design)** | Paradigme de conception centré sur le domaine métier. Utilise Entités, Value Objects, Agrégats, et Repositories. |
| **Entité (Entity)** | Objet métier avec une identité unique (UUID) et un cycle de vie. Ex: `Order`, `Customer`, `Shop`. |
| **Value Object** | Objet sans identité propre, défini uniquement par ses attributs. Ex: `Money`, `Address`. |
| **Agrégat (Aggregate)** | Groupe d'entités liées traitées comme une unité cohérente. Ex: `Order` + `OrderItems`. |
| **Repository** | Interface définissant les opérations de persistance pour une entité. Implémentation concrète dans l'infrastructure (PostgreSQL). |
| **Use Case (Cas d'usage)** | Action métier spécifique orchestrant les repositories. Ex: `CreateCustomerUsecase`, `AcceptOrderUsecase`. |
| **DTO (Data Transfer Object)** | Objet pour transférer des données entre couches (ex: requête HTTP → Use Case). Évite l'exposition des entités. |
| **Handler** | Composant HTTP qui reçoit une requête, appelle un use case, et retourne une réponse JSON. |
| **Middleware** | Fonction interceptant les requêtes HTTP pour ajouter un comportement transversal (auth, logs, multi-tenant). |
| **Dependency Injection (DI)** | Pattern consistant à fournir les dépendances d'un composant depuis l'extérieur. Centralisé dans `internal/app/app.go`. |
| **Transaction (DB)** | Opération atomique garantissant l'intégrité des données. Pattern : `BeginTx` → opérations → `Commit` / `Rollback`. |
| **Soft Delete** | Suppression logique via un champ `deleted_at` au lieu d'une suppression physique. Permet la restauration. |

---

## 3. Sécurité & Authentification

| Terme | Définition |
|-------|------------|
| **JWT (JSON Web Token)** | Token d'authentification signé (HS256). Composé d'un `access_token` (15min) et d'un `refresh_token` (7j). |
| **JWT_SECRET** | Clé secrète utilisée pour signer les JWT. **Obligatoire** et doit faire **au moins 32 caractères** (v4.5.0). |
| **RBAC (Role-Based Access Control)** | Contrôle d'accès basé sur les rôles. 6 rôles : `super_admin`, `admin`, `merchant`, `user`, `credit_analyst`, `support_agent`. |
| **2FA (Two-Factor Authentication)** | Authentification à deux facteurs via TOTP (Google Authenticator). Codes de récupération inclus. |
| **TOTP (Time-based One-Time Password)** | Algorithme générant des codes à usage unique basés sur le temps. Standard pour la 2FA. |
| **API Key** | Clé d'authentification pour intégrations tierces. Préfixe `gsk_live_...`, scopes granulaires (`read:products`, `write:orders`). |
| **Session** | Enregistrement d'une connexion utilisateur active. Peut être révoquée individuellement ou globalement. |
| **Bcrypt** | Algorithme de hachage de mots de passe. Coût minimum **10** imposé en production (v4.5.0). |
| **HMAC-SHA256** | Signature cryptographique utilisée pour valider l'authenticité des webhooks de paiement. |
| **Rate Limiting** | Limitation du nombre de requêtes par client/IP. Implémenté via Redis avec fallback mémoire. |
| **IDOR (Insecure Direct Object Reference)** | Vulnérabilité permettant d'accéder aux ressources d'un autre utilisateur. **Mitigée en v4.5.0** par `RequireShopAccess`. |
| **RequireShopAccess** | Middleware v4.5.0 vérifiant que l'utilisateur est propriétaire ou collaborateur de la boutique demandée. |
| **TenantResolver** | Middleware résolvant la boutique active depuis le header `X-Shop-Slug` ou le `Host` HTTP. |
| **CORS (Cross-Origin Resource Sharing)** | Mécanisme HTTP permettant à un frontend d'accéder à l'API depuis un domaine différent. |
| **Recovery Middleware** | Middleware interceptant les panics Go pour éviter le crash du serveur et retourner une erreur 500 propre. |

---

## 4. Paiements & Finance

| Terme | Définition |
|-------|------------|
| **Provider** | Service de paiement externe intégré (Wave, Orange Money, Moov Money, Yenga Pay). |
| **Webhook** | Appel HTTP POST effectué par le provider pour notifier GoShop d'un changement de statut de paiement. |
| **USSD** | Code court (ex: `#144*111#`) composé par le client pour initier un paiement Mobile Money. |
| **Centimes FCFA** | Unité de stockage des montants. **1 FCFA = 100 centimes**. Toujours stocké en `int64` pour éviter les erreurs de virgule flottante. |
| **Machine à états (Payment)** | Cycle de vie d'un paiement : `pending` → `processing` → `success` / `failed` / `refunded`. |
| **Preuve de livraison (COD Proof)** | Document (photo + signature) attestant que le client a bien reçu et payé sa commande en cash. |
| **BCEAO** | Banque Centrale des États de l'Afrique de l'Ouest. Réglemente les taux d'intérêt (max 15% annuel pour le crédit). |
| **Réconciliation** | Processus de vérification que les transactions enregistrées correspondent aux fonds effectivement reçus. |

---

## 5. Multi-tenant & Infrastructure

| Terme | Définition |
|-------|------------|
| **Multi-tenant** | Architecture où plusieurs boutiques (tenants) partagent la même application et base de données. |
| **Option B (Phase 1)** | Multi-tenant avec une colonne `shop_id` dans chaque table métier. Filtrage applicatif. |
| **Option A (Phase 2)** | Multi-tenant avec un schéma PostgreSQL distinct par boutique (`tenant_<slug>`). Isolation physique. |
| **Shop Slug** | Identifiant unique et lisible d'une boutique (ex: `ma-boutique`). Utilisé dans le header `X-Shop-Slug`. |
| **Custom Domain** | Domaine personnalisé (ex: `shop.maboutique.com`) pointant vers une boutique spécifique. |
| **Redis Pub/Sub** | Mécanisme de messagerie Redis utilisé pour diffuser les notifications WebSocket entre plusieurs instances du serveur. |
| **WebSocket Hub** | Composant gérant les connexions WebSocket persistantes avec les clients. Scalable via Redis. |
| **Notification Dispatcher** | Service orchestrant l'envoi de notifications via WebSocket et/ou Email selon la disponibilité. |
| **Scheduler (Cron)** | Tâche automatisée exécutée périodiquement (collecte commissions, relances crédit, clôture tontines). |
| **Migration (DB)** | Script SQL versionné modifiant le schéma de la base de données. Outil : `golang-migrate`. |
| **Idempotence** | Propriété d'une opération pouvant être exécutée plusieurs fois sans changer le résultat au-delà de la première application. |
| **Zero-downtime deployment** | Déploiement sans interruption de service. Requis pour les migrations de production. |
| **Rollback** | Annulation d'une migration ou d'un déploiement en cas d'échec. |

---

## 6. Tests & Qualité

| Terme | Définition |
|-------|------------|
| **Test unitaire** | Test isolé d'une fonction ou méthode utilisant des mocks. Rapide (< 1s). |
| **Test d'intégration** | Test validant l'interaction entre plusieurs composants avec une vraie base de données. |
| **Test E2E (End-to-End)** | Test simulant un scénario utilisateur complet à travers toute la stack (API + DB + Redis). |
| **Test de charge (Load Test)** | Test de performance simulant de nombreux utilisateurs simultanés. Outil : **k6**. |
| **Mock** | Simulation d'une dépendance (repository, service) pour isoler le code testé. Généré avec `gomock`. |
| **Fixture** | Données de test pré-définies et réutilisables (ex: `CreateTestShop`, `CreateTestCustomer`). |
| **Coverage (Couverture)** | Pourcentage du code exécuté par les tests. Objectif : > 80% sur le code critique. |
| **Race Detector** | Outil Go (`go test -race`) détectant les accès concurrents non synchronisés aux variables. |
| **Testify** | Librairie Go fournissant des assertions lisibles (`assert.NoError`, `assert.Equal`). |
| **Conventional Commits** | Convention de nommage des commits : `feat:`, `fix:`, `docs:`, `test:`, `chore:`, `security:`. |

---

## 7. Outils & Technologies

| Terme | Définition |
|-------|------------|
| **Go (Golang)** | Langage de programmation compilé, utilisé pour le backend de GoShop. Version 1.23+. |
| **Chi** | Router HTTP léger et performant pour Go. Utilisé pour définir les routes de l'API. |
| **PostgreSQL** | Base de données relationnelle avancée. Version 16+. Supporte JSONB, UUID, transactions ACID. |
| **Redis** | Base de données en mémoire utilisée pour le cache, les sessions, le rate limiting et WebSocket Pub/Sub. |
| **Zerolog** | Librairie de logging Go produisant des logs JSON structurés. Rapide et sans allocation. |
| **Prometheus** | Système de monitoring collectant les métriques de l'API via l'endpoint `/metrics`. |
| **Loki / Promtail** | Stack d'agrégation de logs compatible avec Grafana pour la visualisation centralisée. |
| **Grafana** | Outil de visualisation des métriques Prometheus et logs Loki via dashboards. |
| **Docker** | Technologie de conteneurisation pour packager l'application et ses dépendances. |
| **Kubernetes (K8s)** | Orchestrateur de conteneurs pour le déploiement en production. |
| **k6** | Outil de test de charge moderne écrit en Go. Scripts en JavaScript. |
| **golang-migrate** | Outil de gestion des migrations de base de données pour Go. |
| **gomock** | Framework de mocking pour Go. Génère des mocks à partir d'interfaces. |
| **Swagger** | Standard de documentation d'API. Interface disponible sur `/swagger/index.html`. |
| **GitHub Actions** | Service CI/CD de GitHub pour automatiser les tests et déploiements. |

---

## 📚 Références complémentaires
- [Architecture GoShop](01-architecture.md)
- [Modèle de données](02-domain-model.md)
- [Stratégie Multi-tenant](04-multi-tenant.md)
- [Plan de Migration](06-migration-plan.md)
- [Guide des Tests](08-testing-guide.md)

---

**Dernière mise à jour** : 2026-07-21
```

