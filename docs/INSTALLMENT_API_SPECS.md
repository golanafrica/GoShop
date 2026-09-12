Parfait ! C'est exactement ce dont ton équipe Frontend (React Native / Web) a besoin pour démarrer immédiatement sans avoir à deviner la structure des données.

Voici le fichier **`INSTALLMENT_API_SPECS.md`** complet, professionnel et prêt à être partagé avec tes développeurs Frontend.

---

### 📄 Fichier à créer : `INSTALLMENT_API_SPECS.md`

Crée ce fichier à la racine de ton projet (ou dans le dossier `docs/`) et copie ce contenu :

```markdown
# 📱 Spécifications API : Paiement en Tranches & Dashboard Marchand (v5.0.0)

Ce document détaille les endpoints, les payloads et les structures de données nécessaires pour intégrer le système de paiement en tranches et le tableau de bord marchand dans l'application Frontend (React Native / Web).

---

## 🔐 Authentification & En-têtes Requis

Tous les endpoints ci-dessous sont **protégés** et nécessitent les en-têtes suivants :

```http
Authorization: Bearer <votre_jwt_token>
X-Shop-Slug: <slug_de_la_boutique>
Content-Type: application/json
```
> ⚠️ **Important** : Le middleware `RequireShopAccess` vérifie automatiquement que l'utilisateur connecté est bien le propriétaire ou un collaborateur de la boutique spécifiée dans `X-Shop-Slug`.

---

## 📊 1. Tableau de Bord Marchand (Dashboard Global)

Récupère les statistiques globales des ventes en tranches et la liste résumée des commandes concernées. Idéal pour l'écran d'accueil du dashboard marchand.

- **Méthode** : `GET`
- **Endpoint** : `/api/merchant/installments/dashboard`

### ✅ Réponse Succès (200 OK)

```json
{
  "success": true,
  "dashboard": {
    "total_pending_orders": 12,
    "total_amount_pending": 4500000,
    "total_held_amount": 1500000,
    "total_overdue_orders": 2
  },
  "orders": [
    {
      "order_id": "f4c72889-038e-47ab-962f-3669a4febc61",
      "customer_name": "Client a1b2c3d4",
      "total_amount": 3000000,
      "paid_amount": 1000000,
      "remaining_amount": 2000000,
      "status": "overdue",
      "next_due_date": "2026-09-15T12:00:00Z",
      "delivery_zone_name": "Zone Urbaine Test",
      "release_delay_days": 5,
      "expected_release_date": "2026-09-20T12:00:00Z",
      "installments": [
        {
          "tranche_number": 1,
          "amount_cents": 1000000,
          "due_date": "2026-09-01T12:00:00Z",
          "status": "paid",
          "paid_at": "2026-09-01T10:30:00Z"
        },
        {
          "tranche_number": 2,
          "amount_cents": 1000000,
          "due_date": "2026-09-15T12:00:00Z",
          "status": "overdue",
          "paid_at": null
        },
        {
          "tranche_number": 3,
          "amount_cents": 1000000,
          "due_date": "2026-09-30T12:00:00Z",
          "status": "pending",
          "paid_at": null
        }
      ]
    }
  ]
}
```

---

## 🔍 2. Détail des Tranches d'une Commande

Récupère le détail complet et la timeline des tranches pour une commande spécifique. Idéal pour l'écran de "Détail de la commande".

- **Méthode** : `GET`
- **Endpoint** : `/api/orders/{order_id}/installments`

### ✅ Réponse Succès (200 OK)

```json
{
  "success": true,
  "order_id": "f4c72889-038e-47ab-962f-3669a4febc61",
  "total_paid": 1000000,
  "total_remaining": 2000000,
  "installments": [
    {
      "tranche_number": 1,
      "amount_cents": 1000000,
      "due_date": "2026-09-01T12:00:00Z",
      "status": "paid",
      "paid_at": "2026-09-01T10:30:00Z"
    },
    {
      "tranche_number": 2,
      "amount_cents": 1000000,
      "due_date": "2026-09-15T12:00:00Z",
      "status": "overdue",
      "paid_at": null
    },
    {
      "tranche_number": 3,
      "amount_cents": 1000000,
      "due_date": "2026-09-30T12:00:00Z",
      "status": "pending",
      "paid_at": null
    }
  ]
}
```

---

## ⚙️ 3. Configurer un Plan de Tranches (Produit)

Permet au marchand de définir un plan de paiement en tranches pour un produit spécifique, en associant une zone de livraison pour calculer le délai de libération des fonds.

- **Méthode** : `POST`
- **Endpoint** : `/api/products/{product_id}/installment-plan`

### 📥 Requête (Body)

```json
{
  "nb_tranches": 3,
  "delai_jours": 15,
  "delivery_zone_code": "BF-OUAGA-URB"
}
```

### ✅ Réponse Succès (200 OK)

```json
{
  "success": true,
  "plan": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "product_id": "prod_123",
    "shop_id": "shop_456",
    "nb_tranches": 3,
    "delai_jours": 15,
    "delivery_zone_code": "BF-OUAGA-URB",
    "release_delay_days": 5,
    "is_active": true,
    "created_at": "2026-09-12T10:00:00Z"
  }
}
```

---

## 📚 Dictionnaire de Données & Règles Métier

### 1. Gestion des Montants
- **Tous les montants (`*_cents`) sont exprimés en centimes de FCFA.**
- **Conversion pour l'affichage** : Diviser par 100 et ajouter le symbole FCFA.
  - *Exemple* : `3000000` centimes = `30 000 FCFA`.

### 2. Statuts de Commande (`status` dans `orders`)
- `"pending"` : En cours de paiement, aucune tranche payée.
- `"partial"` : Au moins une tranche payée, mais pas la totalité.
- `"complete"` : Toutes les tranches sont payées.
- `"overdue"` : Au moins une tranche est en retard de paiement.

### 3. Statuts de Tranche (`status` dans `installments`)
- `"pending"` : À payer, date d'échéance future.
- `"paid"` : Payée avec succès (le champ `paid_at` est rempli).
- `"overdue"` : En retard (la date actuelle est postérieure à `due_date` et le statut n'est pas `"paid"`).

### 4. Dates et Fuseaux Horaires
- Toutes les dates (`due_date`, `paid_at`, `expected_release_date`) sont au format **ISO 8601** et en **UTC** (`Z` à la fin).
- Le frontend doit les convertir en heure locale du dispositif de l'utilisateur pour l'affichage.

### 5. Performance & Cache
- L'endpoint `/api/merchant/installments/dashboard` est mis en cache côté serveur (Redis) avec un **TTL de 5 minutes**.
- **Conseil Frontend** : Il est inutile de "pull-to-refresh" plus d'une fois toutes les 5 minutes, sauf après une action critique (ex: confirmation manuelle de livraison).

---

## ⚠️ Gestion des Erreurs

En cas d'erreur, l'API retourne toujours ce format standardisé :

```json
{
  "code": "UNAUTHORIZED",
  "message": "Boutique non identifiée ou accès refusé",
  "status": 401
}
```

**Codes d'erreur courants pour ce module :**
- `401 UNAUTHORIZED` : Token JWT manquant ou invalide.
- `403 FORBIDDEN` : L'utilisateur n'est pas propriétaire/collaborateur de la boutique (`X-Shop-Slug`).
- `404 NOT_FOUND` : La commande ou le produit demandé n'existe pas.
- `500 INTERNAL_SERVER_ERROR` : Erreur serveur (contacter le support backend).

```

---

