
```markdown
# 📚 API Reference (GoShop v5.1.0)

Endpoints REST + WebSocket. Pour le détail des payloads, voir aussi **Swagger** : `GET /swagger/index.html`.

**Dernière mise à jour** : 2026-10-05

---

## 🌐 Base URL

| Environnement | URL |
|---------------|-----|
| **Développement** | `http://localhost:8080` (défaut `APP_PORT` / E2E) |
| Staging | `https://staging-api.golanafrica.com` |
| Production | `https://api.golanafrica.com` |

---

## 🔐 Authentification & sécurité

### JWT
- Header : `Authorization: Bearer <access_token>`
- Refresh : `POST /auth/refresh`

### Clés API
- Header : `X-API-Key: gsk_live_...` (ou Bearer selon config)
- **Pas** de `?api_key=` en query (rejeté)

### 2FA
Si `requires_2fa: true` au login → vérifier TOTP avant tokens complets.

### Multi-tenant (routes métier)
- Header **`X-Shop-Slug: <slug>`** (ou custom domain / sous-domaine en prod)
- `TenantResolver` : résout la boutique, refuse si inactive, et vérifie **owner ou collaborateur**

---

## 📡 Public (sans JWT marchand)

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/register` | Inscription |
| `POST` | `/login` | Connexion |
| `POST` | `/auth/refresh` | Refresh tokens |
| `POST` | `/webhooks/{provider}` | Webhooks paiement (HMAC) |
| `GET` | `/api/public/products` | Catalogue public |
| `GET` | `/health/live` | Liveness |
| `GET` | `/health/ready` | Readiness (DB + Redis) |
| `GET` | `/metrics` | Prometheus |
| `GET` | `/swagger/index.html` | Swagger UI |

---

## 🛡️ Routes protégées

### Utilisateur & sessions

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `GET` | `/api/auth/me` | Profil |
| `POST` | `/logout` ou `/api/auth/logout` | Déconnexion (selon wiring) |
| `GET` | `/api/sessions` | Sessions actives |
| `DELETE` | `/api/sessions/{id}` | Révoquer une session |
| `DELETE` | `/api/sessions` | Toutes les sessions |

### Clés API

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/api-keys` | Créer |
| `GET` | `/api/api-keys` | Lister |
| `DELETE` | `/api/api-keys/{id}` | Révoquer |
| `GET` | `/api/api-keys/stats` | Stats |

### Boutiques

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/shops` | Créer |
| `GET` | `/api/shops` | Mes boutiques |
| `PUT` | `/api/shops/{id}` | Modifier |
| `GET` | `/api/shops/{id}/payment-settings` | Config paiement |
| `PUT` | `/api/shops/{id}/payment-settings` | Modifier config |

### Collaborateurs

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/collaborators/invite` | Inviter |
| `POST` | `/api/collaborators/accept/{token}` | Accepter |
| `GET` | `/api/collaborators` | Lister |
| `PUT` | `/api/collaborators/{id}/role` | Rôle |
| `DELETE` | `/api/collaborators/{id}` | Retirer |

### Produits *(X-Shop-Slug)*

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/products` | Créer |
| `GET` | `/api/products` | Lister |
| `GET` | `/api/products/{id}` | Détail |
| `PUT` | `/api/products/{id}` | MAJ |
| `DELETE` | `/api/products/{id}` | Soft delete |

### Clients & KYC *(X-Shop-Slug)*

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/customers` | Créer |
| `GET` | `/api/customers` | Lister |
| `GET` | `/api/customers/{id}` | Détail |
| `PUT` | `/api/customers/{id}` | MAJ |
| `DELETE` | `/api/customers/{id}` | Supprimer |
| `POST` | `/api/customers/kyc/upload` | Upload pièce |
| `GET` | `/api/customers/{id}/kyc/status` | Statut KYC |
| `GET` | `/api/merchant/kyc/pending` | File d’attente marchand |
| `POST` | `/api/merchant/kyc/{id}/review` | Valider / rejeter |

### Commandes & COD *(X-Shop-Slug)*

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/orders` | Créer |
| `GET` | `/api/orders` | Lister |
| `GET` | `/api/orders/{id}` | Détail |
| `POST` | `/api/orders/{id}/pay` | Initier paiement (ex. Yenga) |
| `POST` | `/api/orders/{id}/accept` | COD accept |
| `POST` | `/api/orders/{id}/reject` | COD reject |
| `POST` | `/api/orders/{id}/out-for-delivery` | En livraison |
| `POST` | `/api/orders/{id}/deliver` | Livré + preuve |
| `POST` | `/api/orders/{id}/cancel` | Annuler |

### Paiements

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `GET` | `/api/payments` | Lister |
| `GET` | `/api/payments/{id}` | Détail (statut pay-in) |
| `POST` | `/api/payments/{id}/refund` | Remboursement |

---

### 💰 Wallet marchand (v5.1) *(X-Shop-Slug)*

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `GET` | `/api/wallet` | Solde, held, available, **debt_cents**, **withdrawal_blocked** |
| `GET` | `/api/wallet/freeze-status` | Gel + montant dû (priorité dette) |
| `POST` | `/api/wallet/deposit` | Dépôt manuel (applique **debt sweep** si dette) |
| `POST` | `/api/wallet/withdraw` | Débit wallet (souvent via flux withdrawals MM) |

**Champs clés `GET /api/wallet` :**

```json
{
  "shop_id": "uuid",
  "balance_cents": 95000,
  "held_cents": 0,
  "available_cents": 95000,
  "debt_cents": 0,
  "is_frozen": false,
  "withdrawal_blocked": false
}
```

`withdrawal_blocked` ≈ `is_frozen || debt_cents > 0 || available_cents <= 0`.

Doc métier : [12-wallet-debt-sweep.md](12-wallet-debt-sweep.md).

### Retraits Mobile Money *(X-Shop-Slug)*

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/withdrawals` | Demande cash-out → **refusé si `debt_cents > 0`** (HTTP 400) |
| `GET` | `/api/withdrawals` | Historique |
| `GET` | `/api/withdrawals/{id}` | Détail (si exposé) |

---

### ⚖️ Litiges / disputes (v5.x)

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/orders/{id}/disputes` (ou route open équivalente) | Ouvrir un litige sur commande |
| `GET` | `/api/admin/disputes` | Liste admin (filtres status) |
| `GET` | `/api/admin/disputes/{id}` | Détail |
| `POST` | `/api/admin/disputes/{id}/resolve` | Résoudre : `customer_wins` \| `merchant_wins` |

**Effets métier (résumé) :**
- **Pré-release** + `merchant_wins` → release / crédit wallet (+ sweep si dette)
- **Post-release** + `customer_wins` → clawback puis `debt_cents` si besoin, pas de freeze auto

---

### Crédit à la consommation

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/credit/plans` | Plan marchand |
| `POST` | `/api/credit/apply` | Demande client |
| `POST` | `/api/credit/{id}/approve` | Approuver |
| `POST` | `/api/credit/{id}/pay-down-payment` | Acompte |
| `POST` | `/api/credit/{id}/pay-installment` | Échéance |

### Tontine

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/tontine/groups` | Créer groupe |
| `POST` | `/api/tontine/groups/join` | Rejoindre |
| `POST` | `/api/tontine/groups/{id}/pay` | Cotisation |
| `GET` | `/api/tontine/groups/{id}/payments` | Historique |

### Paiement en tranches & zones (v5.0)

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/products/{product_id}/installment-plan` | Config plan |
| `GET` | `/api/products/{product_id}/installment-plan` | Lire plan |
| `GET` | `/api/orders/{order_id}/installments` | Tranches commande |
| `GET` | `/api/merchant/installments/dashboard` | Dashboard marchand |

### Admin — zones de livraison

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/admin/delivery-zones` | Créer |
| `GET` | `/api/admin/delivery-zones` | Lister |
| `PUT` | `/api/admin/delivery-zones/{id}` | Modifier |
| `DELETE` | `/api/admin/delivery-zones/{id}` | Soft delete |

### WebSocket

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `GET` | `/ws/notifications` | Notifications temps réel (JWT) |

---

## 👑 Administration

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `GET` | `/api/admin/shops` | Toutes les boutiques |
| `PUT` | `/api/admin/shops/{id}/suspend` | Suspendre |
| `PUT` | `/api/admin/shops/{id}/activate` | Réactiver |
| `GET` | `/api/admin/merchant-kyc/pending` | KYC marchand |
| `POST` | `/api/admin/merchant-kyc/{id}/review` | Review KYC |
| `POST` | `/api/admin/scheduler/trigger` | Collecte commissions |
| `POST` | `/api/admin/scheduler/trigger-escrow-auto-release` | **Auto-release escrow** (HTTP 202) |
| `POST` | `/api/admin/scheduler/force-auto-release/{order_id}` | Dev/test only (`ENABLE_FORCE_RELEASE`) |
| `GET` | `/api/admin/scheduler/batches` | Batches commissions |
| `GET` | `/api/admin/scheduler/batches/{id}` | Détail batch |
| `GET` | `/api/admin/scheduler/stats` | Stats collectes |

---

## 🔌 Webhooks paiement

| Méthode | Endpoint | Signature |
|---------|----------|-----------|
| `POST` | `/webhooks/yenga_pay` | `X-Signature` HMAC-SHA256 |
| `POST` | `/webhooks/orange_money` | idem |
| `POST` | `/webhooks/moov_money` | idem |

---

## ⚠️ Erreurs (exemples)

```json
{
  "code": "VALIDATION_FAILED",
  "message": "…",
  "status": 400
}
```

| HTTP | Cas fréquent |
|------|----------------|
| 400 | Validation, **retrait avec dette**, payload invalide |
| 401 | JWT manquant / expiré |
| 403 | RBAC ou accès boutique refusé |
| 404 | Ressource absente / hors tenant |
| 409 | Conflit (dispute déjà résolu, slug pris, …) |
| 429 | Rate limit |
| 500 | Erreur serveur |

---

## 📚 Références

- [Architecture](01-architecture.md)
- [Multi-tenant](04-multi-tenant.md)
- [Wallet debt / clawback / sweep](12-wallet-debt-sweep.md)
- [Tests E2E](08-testing-guide.md)
- [Tontine](11-tontine-system.md)
- Swagger live : `/swagger/index.html`

---

**Note** : certaines variantes de path (logout, open dispute) peuvent différer légèrement selon le wiring Chi ; en cas de doute, **Swagger** et les scripts `e2e-*.ps1` font foi.
```

