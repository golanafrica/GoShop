# Système de paiement

## Vue d'ensemble

Système modulaire supportant 3 modes de paiement :

1. **Cash à la livraison** (priorité MVP)
2. **Mobile Money** (Wave, Orange Money, MTN MoMo, Moov Money)
3. **Paiement par tranches (crédit)**

---

## 1. Cash à la livraison

### Flux complet

(Voir diagramme `/docs/diagrams/cash-flow.puml`)

1. **Client** commande en ligne, choisit "Cash à la livraison"
2. **Système** : Crée commande en `pending_confirmation`, **stock réservé 24h**
3. **Marchand** reçoit notification (dashboard + SMS/WhatsApp)
4. **Marchand** accepte ou refuse la commande (depuis dashboard)
5. **Système** : Si acceptée, commande passe en `confirmed`, client notifié
6. **Livreur** collecte les espèces, marque `delivered` via app
7. **Commande clôturée**, score client mis à jour

### États d'une commande cash

pending_confirmation → confirmed → out_for_delivery → delivered
↘ cancelled
↘ expired


### Règles configurables par le marchand

| Paramètre | Description | Valeurs |
|-----------|-------------|---------|
| `cash_reservation_hours` | Délai de réservation stock | 6h à 72h |
| `cash_delivery_zones` | Zones de livraison | JSON array |
| `cash_min_amount` | Montant minimum de commande | En FCFA |
| Score client minimum | Éligibilité au cash | Score calculé |
| Rappel automatique | Notification J-1 | SMS/WhatsApp |

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
Provider
Priorité
Statut
Orange Money
🔴 Haute
À implémenter
Moov Money
🔴 Haute
À implémenter
Wave
🟡 Moyenne
À implémenter
MTN MoMo
🟢 Basse
À implémenter
Coris Money
🟢 Basse
À implémenter
Registre de providers
Chaque marchand active les providers souhaités via shop_payment_settings :

CREATE TABLE shop_payment_settings (
    shop_id UUID PRIMARY KEY REFERENCES shops(id),
    orange_money_enabled BOOLEAN DEFAULT false,
    orange_merchant_code TEXT,
    orange_money_api_key TEXT,  -- chiffré en base
    moov_money_enabled BOOLEAN DEFAULT false,
    moov_merchant_code TEXT,
    moov_api_key TEXT,
    wave_enabled BOOLEAN DEFAULT false,
    wave_api_key TEXT,
    -- ...
);

Webhooks
Endpoint unique : POST /webhooks/{provider}
Validation de signature (HMAC-SHA256)
Mise à jour du statut de la transaction
Notification au marchand et au client
Flux de paiement

1. Client choisit "Orange Money"
2. Système appelle InitiatePayment()
3. Client reçoit USSD/popup pour valider
4. Provider envoie webhook de confirmation
5. Système valide la signature
6. Commande passe en "paid"
7. Notification client + marchand

3. Crédit (paiement en tranches)
Plans de paiement
Définis par le marchand :
Paramètre
Description
Exemple
Nom
Nom du plan
"3 fois sans frais"
Description
Description
"Payez en 3 mensualités"
down_payment_pct
Pourcentage d'acompte
30%
installments_count
Nombre de tranches
3
interval_days
Intervalle entre tranches
30 jours
interest_rate_pct
Taux d'intérêt (optionnel)
0%
Calcul des tranches


downPayment := orderAmount * downPaymentPct / 100
remaining := orderAmount - downPayment
baseInstallment := remaining / installmentsCount

// Création des échéances
for i := 1; i <= installmentsCount; i++ {
    dueDate := orderDate.Add(time.Duration(i * intervalDays) * 24 * time.Hour)
    installment := &Installment{
        OrderID:   order.ID,
        Amount:    baseInstallment,
        DueDate:   dueDate,
        Status:    "pending",
    }
    // Sauvegarde en base
}

Calcul des tranches


downPayment := orderAmount * downPaymentPct / 100
remaining := orderAmount - downPayment
baseInstallment := remaining / installmentsCount

// Création des échéances
for i := 1; i <= installmentsCount; i++ {
    dueDate := orderDate.Add(time.Duration(i * intervalDays) * 24 * time.Hour)
    installment := &Installment{
        OrderID:   order.ID,
        Amount:    baseInstallment,
        DueDate:   dueDate,
        Status:    "pending",
    }
    // Sauvegarde en base
}

Score de fiabilité client
Score
Accès
Excellente
Accès à tous les plans
Bonne
Plans standards uniquement
Passable
Acompte plus élevé requis
Mauvaise
Uniquement paiement comptant
Rappels automatiques
Délai
Action
48h avant échéance
SMS de rappel
24h avant échéance
SMS de rappel
1h avant échéance
SMS urgent
1h après échéance
Notification de retard
24h après échéance
Pénalités éventuelles
Configuration par boutique
Chaque boutique peut configurer ses propres règles :

{
  "shop_id": "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
  "payment_methods": {
    "cash_enabled": true,
    "cash_reservation_hours": 24,
    "cash_min_amount": 10000,
    "cash_delivery_zones": ["Bobo-Dioulasso", "Ouagadougou"],
    
    "orange_money_enabled": true,
    "moov_money_enabled": true,
    "wave_enabled": false,
    
    "credit_enabled": true,
    "credit_plans": [
      {
        "name": "3 fois sans frais",
        "down_payment_pct": 30,
        "installments_count": 3,
        "interval_days": 30,
        "interest_rate_pct": 0
      }
    ]
  }
}

Sécurité
Règles
Clés API chiffrées en base (AES-256)
Validation HMAC pour tous les webhooks
Idempotence : chaque transaction a un ID unique
Audit trail : toutes les transactions sont loguées
Rate limiting sur les endpoints de paiement
Conformité
BCEAO : Respect des réglementations UEMOA
PCI DSS : Pas de stockage de données bancaires
RGPD : Consentement explicite pour les paiements
Monitoring
Métriques
payment_success_rate : Taux de réussite des paiements
payment_duration_seconds : Durée des transactions
webhook_processing_time : Temps de traitement des webhooks
payment_errors_total : Nombre d'erreurs par provider
Alertes
Taux d'échec > 5% sur 5 min
Webhook non reçu après 10 min
Provider indisponible > 1 min
Références
Diagrammes de flux
API Reference
Multi-tenant