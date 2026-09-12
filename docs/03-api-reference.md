

---

```markdown
# 📚 API Reference (GoShop v4.5.0)

Ce document décrit les endpoints de l'API REST de GoShop, ainsi que les connexions WebSocket pour les notifications en temps réel.

## 🌐 Base URL
- **Développement** : `http://localhost:8081`
- **Staging** : `https://staging-api.golanafrica.com`
- **Production** : `https://api.golanafrica.com`

---

## 🔐 Authentification & Sécurité

### 1. JWT (JSON Web Token)
La majorité des routes `/api/*` nécessitent un token d'accès valide.
- **Header** : `Authorization: Bearer <access_token>`
- **Durée de vie** : 15 minutes (Access), 7 jours (Refresh).
- **Refresh** : `POST /auth/refresh` (nécessite le refresh token).

### 2. Clés API (v4.4.3)
Pour les intégrations tierces (ERP, logistique), utilisez une clé API au lieu d'un JWT.
- **Header** : `X-API-Key: gsk_live_...`
- **Scope** : Défini lors de la création de la clé (ex: `read:products`, `write:orders`).

### 3. Authentification à 2 facteurs (2FA - v4.4.0)
Si activée sur le compte, le login renvoie un flag `requires_2fa: true`. L'utilisateur doit alors appeler `POST /auth/2fa/verify` avec son code TOTP avant de recevoir les tokens JWT.

### 4. En-têtes Multi-tenant (Obligatoire pour les routes métier)
Pour interagir avec les ressources d'une boutique spécifique, vous **devez** inclure :
- **Header** : `X-Shop-Slug: <slug-de-la-boutique>`
> *Note : En v4.5.0, le middleware `RequireShopAccess` vérifie automatiquement que l'utilisateur authentifié est le propriétaire ou un collaborateur de cette boutique.*

---

## 📡 Endpoints Publics (Pas d'authentification requise)

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/register` | Inscription d'un nouvel utilisateur (rôle `user` par défaut) |
| `POST` | `/login` | Connexion (renvoie JWT ou demande 2FA) |
| `POST` | `/auth/refresh` | Obtention d'un nouveau couple de tokens |
| `POST` | `/webhooks/{provider}` | Réception des webhooks de paiement (Wave, Orange, YengaPay) |
| `GET` | `/api/public/products` | Catalogue public des produits (avec filtres et pagination) |
| `GET` | `/health/live` | Liveness probe (Kubernetes) |
| `GET` | `/health/ready` | Readiness probe (vérifie DB + Redis) |
| `GET` | `/metrics` | Métriques Prometheus |
| `GET` | `/swagger/index.html` | Documentation Swagger UI interactive |

---

## 🛡️ Routes Protégées (`/api/*`)

### 👤 Utilisateur & Sessions
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `GET` | `/api/auth/me` | Profil de l'utilisateur connecté |
| `POST` | `/api/auth/logout` | Révoquer la session courante (invalidation du token) |
| `GET` | `/api/sessions` | Lister les sessions actives de l'utilisateur |
| `DELETE` | `/api/sessions/{id}` | Révoquer une session spécifique |
| `DELETE` | `/api/sessions` | Révoquer toutes les sessions (déconnexion globale) |

### 🔑 Gestion des Clés API (v4.4.3)
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/api-keys` | Créer une nouvelle clé API avec des scopes spécifiques |
| `GET` | `/api/api-keys` | Lister mes clés API (masquées) |
| `DELETE` | `/api/api-keys/{id}` | Révoquer une clé API |
| `GET` | `/api/api-keys/stats` | Statistiques d'utilisation de mes clés |

### 🏪 Gestion des Boutiques (Multi-tenant)
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/shops` | Créer une nouvelle boutique |
| `GET` | `/api/shops` | Lister les boutiques dont je suis propriétaire |
| `PUT` | `/api/shops/{id}` | Mettre à jour les infos d'une boutique |
| `GET` | `/api/shops/{id}/payment-settings` | Voir la config de paiement (Mobile Money, COD, Tontine) |
| `PUT` | `/api/shops/{id}/payment-settings` | Modifier la config de paiement |

### 👥 Collaborateurs (v4.3.0)
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/collaborators/invite` | Inviter un collaborateur (Plateforme ou Boutique) |
| `POST` | `/api/collaborators/accept/{token}` | Accepter une invitation par lien |
| `GET` | `/api/collaborators` | Lister les collaborateurs de mes boutiques |
| `PUT` | `/api/collaborators/{id}/role` | Modifier le rôle d'un collaborateur |
| `DELETE` | `/api/collaborators/{id}` | Retirer un collaborateur |

### 📦 Produits
*(Nécessite le header `X-Shop-Slug`)*
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/products` | Créer un produit |
| `GET` | `/api/products` | Lister les produits de la boutique (avec pagination/tri) |
| `GET` | `/api/products/{id}` | Détails d'un produit |
| `PUT` | `/api/products/{id}` | Mettre à jour un produit |
| `DELETE` | `/api/products/{id}` | Supprimer (soft delete) un produit |

### 👤 Clients & KYC
*(Nécessite le header `X-Shop-Slug`)*
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/customers` | Créer un profil client (lié automatiquement au User si email existant) |
| `GET` | `/api/customers` | Lister les clients de la boutique |
| `GET` | `/api/customers/{id}` | Détails d'un client |
| `PUT` | `/api/customers/{id}` | Mettre à jour un client |
| `DELETE` | `/api/customers/{id}` | Supprimer un client |
| `POST` | `/api/customers/kyc/upload` | (Client) Uploader un document d'identité (CNI/Passeport) |
| `GET` | `/api/customers/{id}/kyc/status` | (Client) Vérifier le statut de validation KYC |
| `GET` | `/api/merchant/kyc/pending` | (Marchand) Lister les clients en attente de validation KYC |
| `POST` | `/api/merchant/kyc/{id}/review` | (Marchand) Valider ou rejeter un dossier KYC |

### 🛒 Commandes & Cash on Delivery (COD)
*(Nécessite le header `X-Shop-Slug`)*
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/orders` | Créer une nouvelle commande |
| `GET` | `/api/orders` | Lister les commandes de la boutique |
| `GET` | `/api/orders/{id}` | Détails d'une commande |
| `POST` | `/api/orders/{id}/pay` | Initier un paiement en ligne (Mobile Money) |
| `POST` | `/api/orders/{id}/accept` | (COD) Le marchand accepte la commande |
| `POST` | `/api/orders/{id}/reject` | (COD) Le marchand refuse la commande |
| `POST` | `/api/orders/{id}/out-for-delivery` | (COD) Marquer comme "en cours de livraison" |
| `POST` | `/api/orders/{id}/deliver` | (COD) Confirmer la livraison et encaisser le paiement |
| `POST` | `/api/orders/{id}/cancel` | Annuler une commande (libère le stock) |

### 💰 Paiements & Retraits
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `GET` | `/api/payments` | Lister les transactions de paiement |
| `GET` | `/api/payments/{id}` | Détails d'une transaction |
| `POST` | `/api/payments/{id}/refund` | Initier un remboursement partiel ou total |
| `POST` | `/api/withdrawals` | Demander un retrait de fonds vers Mobile Money |
| `GET` | `/api/withdrawals` | Historique des retraits |

### 💳 Crédit à la Consommation (v3.5.0)
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/credit/plans` | (Marchand) Créer un plan de financement |
| `POST` | `/api/credit/apply` | (Client) Soumettre une demande de crédit |
| `POST` | `/api/credit/{id}/approve` | (Marchand) Approuver une demande |
| `POST` | `/api/credit/{id}/pay-down-payment` | (Client) Payer l'acompte initial |
| `POST` | `/api/credit/{id}/pay-installment` | (Client) Payer une échéance mensuelle |

### 🤝 Tontine de Biens Physiques (v2.9.0)
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/tontine/groups` | Créer un nouveau groupe de tontine |
| `POST` | `/api/tontine/groups/join` | Rejoindre un groupe via code d'invitation |
| `POST` | `/api/tontine/groups/{id}/pay` | Payer sa cotisation pour le cycle en cours |
| `GET` | `/api/tontine/groups/{id}/payments` | Historique des paiements du groupe |
### 💳 Paiement en Tranches & Zones de Livraison (v5.0.0)
*(Nécessite le header `X-Shop-Slug` pour les routes marchand)*

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/products/{product_id}/installment-plan` | (Marchand) Configurer un plan de paiement en tranches pour un produit (délai basé sur la zone) |
| `GET` | `/api/products/{product_id}/installment-plan` | (Public/Marchand) Récupérer le plan de tranches d'un produit |
| `GET` | `/api/orders/{order_id}/installments` | (Marchand/Client) Récupérer le détail et le statut de toutes les tranches d'une commande |
| `GET` | `/api/merchant/installments/dashboard` | (Marchand) Tableau de bord global : stats (en attente, séquestre, retard) et résumé des commandes |

#### 👑 Administration des Zones de Livraison
*(Nécessite le rôle `super_admin` ou `admin`)*

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/admin/delivery-zones` | Créer une nouvelle zone de livraison (ex: "Ouaga Urbain", 5 jours) |
| `GET` | `/api/admin/delivery-zones` | Lister toutes les zones de livraison (avec filtres par pays/type) |
| `PUT` | `/api/admin/delivery-zones/{id}` | Modifier les délais ou le statut d'une zone |
| `DELETE` | `/api/admin/delivery-zones/{id}` | Désactiver une zone de livraison (soft delete) |

### 🔔 Notifications Temps Réel (v4.5.0)
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `GET` | `/ws/notifications` | Connexion WebSocket (nécessite `Authorization: Bearer <token>` en header ou query param). Reçoit les événements en temps réel (ex: `client_order_confirmed`, `payment_success`). |

---

## 👑 Routes Administration (Super Admin / Admin)

Ces routes nécessitent le rôle `super_admin` ou `admin` et sont protégées par le middleware RBAC.

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `GET` | `/api/admin/shops` | Lister toutes les boutiques de la plateforme |
| `PUT` | `/api/admin/shops/{id}/suspend` | Suspendre une boutique (avec motif) |
| `PUT` | `/api/admin/shops/{id}/activate` | Réactiver une boutique suspendue |
| `GET` | `/api/admin/merchant-kyc/pending` | Lister les KYC marchands en attente de validation |
| `POST` | `/api/admin/merchant-kyc/{id}/review` | Valider ou rejeter un dossier KYC marchand |
| `GET` | `/api/admin/collaborators/platform` | Gérer les collaborateurs au niveau de la plateforme |
| `POST` | `/api/admin/scheduler/trigger` | Déclencher manuellement le scheduler de commissions |
| `GET` | `/api/admin/scheduler/batches` | Voir l'historique des batches de commissions traités |

---

## 🔌 Webhooks (Fournisseurs de Paiement)

Ces endpoints sont publics mais sécurisés par validation de signature HMAC-SHA256.

| Méthode | Endpoint | Fournisseur | Header de Signature |
|---------|----------|-------------|---------------------|
| `POST` | `/webhooks/yenga_pay` | Yenga Pay | `X-Signature: <hmac_sha256>` |
| `POST` | `/webhooks/orange_money` | Orange Money | `X-Signature: <hmac_sha256>` |
| `POST` | `/webhooks/moov_money` | Moov Money | `X-Signature: <hmac_sha256>` |

> **Format de la signature** : `HMAC-SHA256(payload, webhook_secret)`

---

## ⚠️ Codes d'Erreur Standardisés

L'API retourne toujours des erreurs au format JSON structuré :

```json
{
  "code": "VALIDATION_FAILED",
  "message": "Le champ 'email' est requis et doit être une adresse email valide.",
  "status": 400
}
```

| Code HTTP | Code Erreur | Description |
|-----------|-------------|-------------|
| `400` | `INVALID_PAYLOAD` | Corps de la requête malformé ou manquant |
| `400` | `VALIDATION_FAILED` | Échec des règles de validation métier |
| `401` | `UNAUTHORIZED` | Token JWT manquant, expiré ou invalide |
| `401` | `TOKEN_EXPIRED` | Le token d'accès a expiré (utiliser le refresh token) |
| `403` | `FORBIDDEN` | L'utilisateur n'a pas les permissions RBAC requises |
| `403` | `SHOP_ACCESS_DENIED` | L'utilisateur n'est pas propriétaire/collaborateur de ce shop (v4.5.0) |
| `404` | `NOT_FOUND` | La ressource demandée n'existe pas |
| `409` | `CONFLICT` | Conflit (ex: email déjà enregistré, slug déjà pris) |
| `429` | `TOO_MANY_ATTEMPTS` | Limite de taux (Rate Limit) dépassée (ex: login) |
| `500` | `INTERNAL_SERVER_ERROR` | Erreur interne du serveur |

---

## 📚 Références complémentaires
- [Guide d'Authentification et 2FA](01-architecture.md)
- [Système de Paiement et Webhooks](payment-system.md)
- [Stratégie Multi-tenant](04-multi-tenant.md)
- [Système de Tontine](11-tontine-system.md)
```

---
