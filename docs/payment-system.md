# Système de paiement

## Vue d'ensemble
Système modulaire supportant 3 modes de paiement :
1. **Cash à la livraison** (priorité MVP)
2. **Mobile Money** (Wave, Orange Money, MTN MoMo)
3. **Paiement par tranches (crédit)**

---

## 1. Cash à la livraison

### Flux complet
(Voir diagramme `/docs/diagrams/cash-flow.puml`)

### États d'une commande cash
- `pending_confirmation` → `confirmed` → `out_for_delivery` → `delivered`
- Ou `cancelled`, `expired`

### Règles configurables par le marchand
- Délai de réservation stock (6h à 72h)
- Zones de livraison
- Montant minimum de commande
- Score client minimum pour éligibilité
- Rappel automatique J-1

---

## 2. Mobile Money

### Architecture (Provider Pattern)

```go
// domain/payment/provider.go
type Provider interface {
    Code() string
    InitiatePayment(ctx context.Context, req *PaymentRequest) (*PaymentResponse, error)
    CheckStatus(ctx context.Context, providerRef string) (*PaymentStatus, error)
    ValidateWebhook(payload []byte, signature string) (*WebhookEvent, error)
}

Providers supportés
Wave 

Orange Money (priorité)

MTN MoMo

Moov Money

Webhooks
Endpoint : POST /webhooks/{provider}

Validation de signature

Mise à jour du statut

3. Crédit (paiement en tranches)
Plans de paiement
Définis par le marchand :

Nom, description

Pourcentage d'acompte

Nombre de tranches

Intervalle (jours)

Taux d'intérêt (optionnel)

Score de fiabilité client
Excellente : Accès à tous les plans

Bonne : Plans standards

Passable : Acompte plus élevé

Mauvaise : Uniquement comptant

Rappels automatiques
48h, 24h, 1h avant échéance

SMS / WhatsApp

