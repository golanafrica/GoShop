

```markdown
# 📱 Spécifications API — Paiement en tranches & dashboard marchand

**Version doc** : v5.1.0  
**Public** : Frontend (Web / React Native)  
**Base URL dev** : `http://localhost:8080`  
**Source code** : `interfaces/handler/installment_handler`, `application/dto/installment_dto`, `domain/entity/installment.go`

> En cas de doute sur un champ exact : **Swagger** `/swagger/index.html` prime.

---

## Authentification

```http
Authorization: Bearer <access_token>
X-Shop-Slug: <slug_boutique>
Content-Type: application/json
```

Le contexte boutique est résolu par **`TenantResolver`** (owner ou collaborateur). Accès refusé si boutique inactive ou hors périmètre.

---

## 1. Dashboard marchand

| | |
|--|--|
| **Méthode** | `GET` |
| **Path** | `/api/merchant/installments/dashboard` |

### Réponse 200

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
        }
      ]
    }
  ]
}
```

**Règle FE** : traiter `total_*`, `*_amount` et `amount_cents` comme **centimes XOF** (÷ 100 pour affichage), sauf confirmation contraire d’un endpoint précis.

**Cache** : un TTL Redis ~5 min a été prévu côté produit ; en cas de doute, ne pas spammer le refresh (après livraison / paiement, un refresh manuel est OK).

---

## 2. Tranches d’une commande

| | |
|--|--|
| **Méthode** | `GET` |
| **Path** | `/api/orders/{order_id}/installments` |

### Réponse 200 (handler actuel)

```json
{
  "success": true,
  "installments": [ /* OrderInstallment[] */ ],
  "total_paid": 1000000,
  "total_remaining": 2000000
}
```

Champs tranche typiques : `tranche_number`, `amount_cents`, `due_date`, `status` (`pending` \| `paid` \| `overdue`), `paid_at`, éventuellement `payment_ref`.

> Le `order_id` n’est **pas** renvoyé explicitement dans le JSON actuel : le FE le connaît déjà via la navigation.

---

## 3. Plan produit — configurer

| | |
|--|--|
| **Méthode** | `POST` |
| **Path** | `/api/products/{product_id}/installment-plan` |

### Body

```json
{
  "nb_tranches": 3,
  "delai_jours": 15,
  "delivery_zone_code": "BF-OUAGA-URB"
}
```

Contraintes métier (entity) : **2 ≤ nb_tranches ≤ 10**, `delai_jours ≥ 1`.  
Le code zone sert à résoudre **`installment_release_delay_days`** (fallback ~7 j si zone inconnue).

### Réponse 200

```json
{
  "success": true,
  "plan": {
    "id": "uuid",
    "product_id": "uuid",
    "shop_id": "uuid",
    "nb_tranches": 3,
    "delai_jours": 15,
    "delivery_zone_id": "BF-OUAGA-URB",
    "installment_release_delay_days": 5,
    "is_active": true,
    "created_at": "2026-09-12T10:00:00Z",
    "updated_at": "2026-09-12T10:00:00Z"
  }
}
```

---

## 4. Plan produit — lire

| | |
|--|--|
| **Méthode** | `GET` |
| **Path** | `/api/products/{product_id}/installment-plan` |

- Plan existant → `{ "success": true, "plan": { … } }`  
- Absent → `{ "success": true, "plan": null, "message": "Aucun plan configuré" }`

---

## 5. Libération séquestre (release escrow)

| | |
|--|--|
| **Méthode** | `POST` |
| **Path** | `/api/orders/{order_id}/release-escrow` |

Utilisé quand les conditions métier le permettent (tranches payées, délai zone, pas de litige bloquant — le backend valide).

### Réponse 200 (v5.1+)

```json
{
  "success": true,
  "release": {
    "net_merchant_cents": 95000,
    "commission_cents": 5000,
    "swept_cents": 0,
    "debt_cents_after": 0,
    "balance_cents": 95000,
    "available_cents": 95000
  }
}
```

- `swept_cents` > 0 : une partie du disponible a **remboursé une dette** wallet (`debt_cents`).  
- Voir aussi [12-wallet-debt-sweep.md](12-wallet-debt-sweep.md).

En production, la libération est aussi pilotée par **`InstallmentAutoReleaseScheduler`** (délai post-livraison / zone).

---

## 6. Dictionnaire métier FE

### Statuts commande (dashboard `orders[].status`)

| Valeur | Sens |
|--------|------|
| `pending` | Aucune tranche payée |
| `partial` | Au moins une payée, pas tout |
| `complete` | Toutes payées |
| `overdue` | Au moins une tranche en retard |

### Statuts tranche

| Valeur | Sens |
|--------|------|
| `pending` | À payer |
| `paid` | Payée (`paid_at` renseigné) |
| `overdue` | Échue non payée |

### Dates
ISO-8601 **UTC** (`Z`). Convertir en local à l’affichage.

### Calcul tranches (référence backend)
Montant total divisé en N parts égales ; le **reste d’arrondi** va sur la **dernière** tranche.

---

## 7. Erreurs

Format typique :

```json
{
  "code": "UNAUTHORIZED",
  "message": "Boutique non identifiée",
  "status": 401
}
```

| HTTP | Cas |
|------|-----|
| 400 | Plan invalide, release impossible |
| 401 | JWT / tenant manquant |
| 403 | Pas owner/collab de la boutique |
| 404 | Ressource absente (selon handlers) |
| 500 | Erreur serveur |

---

## 8. Périmètre hors de ce doc (à brancher ailleurs)

| Besoin FE | Où regarder |
|-----------|-------------|
| Créer commande en tranches | `create_order_with_installments` + routes order/payment |
| Payer une tranche (Yenga) | `process_installment_payment` + webhooks |
| Preuve livraison | delivery proof handlers |
| Zones admin | `/api/admin/delivery-zones` |
| Wallet marchand (dette, held) | `GET /api/wallet` |

---

## 9. Checklist intégration FE

1. Toujours envoyer **JWT + X-Shop-Slug**.  
2. Afficher les montants en **FCFA = cents / 100**.  
3. Écrans : Dashboard → détail commande (tranches) → config plan produit.  
4. Après release / paiement : rafraîchir dashboard + éventuellement `GET /api/wallet`.  
5. Gérer `plan: null` sur GET plan.  
6. Ne pas inventer de champs `release_delay_days` sur le **plan** : utiliser `installment_release_delay_days`.

---

## Références

- [03-api-reference.md](03-api-reference.md)
- [12-wallet-debt-sweep.md](12-wallet-debt-sweep.md)
- Migrations : `050_installment_escrow_refactor`, `051`–`052` delivery zones  
- Code : `installment_handler.go`, `merchant_dashboard_dto.go`

---

**Dernière mise à jour** : 2026-10-05
```

