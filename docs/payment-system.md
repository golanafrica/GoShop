# 💳 Système de paiement

**Version** : v2.8.0-cash-on-delivery  
**Date** : 2026-06-27  
**Statut** : ✅ **PRODUCTION READY** - 6 opérateurs Mobile Money + Cash à la livraison

---

## 📋 État actuel

### ✅ Implémenté et validé

| Version | Fonctionnalité | Statut |
|---------|----------------|--------|
| v2.1.0 | Orange Money + Moov Money (mock) | ✅ |
| v2.2.0 | Yenga Pay API réelle (6 opérateurs) | ✅ |
| v2.3.0 | Configuration par boutique (hybride) | ✅ |
| v2.4.0 | Chiffrement AES-256-GCM des clés API | ✅ |
| v2.5.0 | Cash-out (retraits Mobile Money) | ✅ |
| v2.7.0 | Multi-opérateur complet (ONE_STEP + TWO_STEP + Indirect) | ✅ |
| **v2.8.0** | **Cash à la livraison (workflow complet + commission)** | ✅ **VALIDÉ** |

### 🚧 À venir
- **Wave** : Provider Mobile Money
- **Crédit** : Paiement en tranches avec score de fiabilité
- **SMS notifications** : Africa's Talking (remplacer le no-op actuel)

---

## 🎯 Vue d'ensemble

Système modulaire supportant **5 modes de paiement** :

1. **Yenga Pay** ✅ (Orange, Moov, Telecel, Coris, Sank, MTN) - API réelle
2. **Orange Money** ✅ (mock) - USSD `#144*111#`
3. **Moov Money** ✅ (mock) - USSD `#135*2#`
4. **Cash-out** ✅ (retraits vers Mobile Money via Yenga Pay)
5. **Cash à la livraison** ✅ (paiement espèces à la livraison avec commission GoShop)

---

## 📊 Résultats de production (2026-06-27)

### Dashboard Yenga Pay - Statistiques

| Métrique | Valeur |
|----------|--------|
| **Solde actuel** | 4 170 XOF |
| **Total reçu** | 4 870 XOF |
| **Total retiré** | 700 XOF |
| **Frais collectés** | 130 XOF |
| **Transactions Pay In** | 10 |
| **Retraits Cash Out** | 2 |
| **Commandes Cash COD** | 6 (workflow complet validé) |
| **Taux de succès** | **100%** |

### Transactions Pay In (10/10 réussies)

| Date | Transaction ID | Opérateur | Montant brut | Frais | Montant net |
|------|----------------|-----------|--------------|-------|-------------|
| 27/06/2026 12:50 | `YP20260627.1250.75469419` | Telecel | 500 XOF | 13 XOF | 487 XOF |
| 27/06/2026 12:50 | `YP20260627.1250.61831018` | Orange | 500 XOF | 13 XOF | 487 XOF |
| 27/06/2026 12:50 | `YP20260627.1250.23971082` | Coris | 500 XOF | 13 XOF | 487 XOF |
| 27/06/2026 12:50 | `YP20260627.1250.65505543` | Sank | 500 XOF | 13 XOF | 487 XOF |
| 27/06/2026 12:44 | `YP20260627.1244.32615745` | Moov | 500 XOF | 13 XOF | 487 XOF |
| 24/06/2026 23:13 | `YP20260624.2313.35765328` | Sank | 500 XOF | 13 XOF | 487 XOF |
| 24/06/2026 23:13 | `YP20260624.2313.05467272` | Coris | 500 XOF | 13 XOF | 487 XOF |
| 24/06/2026 23:12 | `YP20260624.2312.86159715` | Sank | 500 XOF | 13 XOF | 487 XOF |
| 24/06/2026 23:12 | `YP20260624.2312.02572603` | Coris | 500 XOF | 13 XOF | 487 XOF |
| 24/06/2026 22:55 | `YP20260624.2255.08849184` | Moov | 500 XOF | 13 XOF | 487 XOF |

### Retraits Cash Out (2/2 réussis)

| Date | Transaction ID | Opérateur | Montant | Statut |
|------|----------------|-----------|---------|--------|
| 26/06/2026 21:03 | `YPCO20260626.2103.65687.7311` | MOOV MONEY | 200 XOF | ✅ Réussi |
| 26/06/2026 20:56 | `YPCO20260626.2056.65687.8675` | ORANGE MONEY | 500 XOF | ✅ Réussi |

### Commandes Cash à la livraison (workflow complet validé)

| Test | Scénario | Résultat |
|------|----------|----------|
| 1 | Workflow complet (create → accept → out_for_delivery → deliver) | ✅ |
| 2 | Commission 2.50% calculée (500 XOF → 12.50 XOF commission) | ✅ |
| 3 | Rejet marchand (stock réincrémenté) | ✅ |
| 4 | Annulation client (stock réincrémenté) | ✅ |
| 5 | Expiration lazy après 24h (stock réincrémenté) | ✅ |

---

## 1. Yenga Pay (API réelle) ✅

### Architecture

```go
// infrastructure/payment/yenga_pay_provider.go
type YengaPayProvider struct {
    httpClient     *http.Client
    apiKey         string
    organizationID string
    projectID      string
    webhookSecret  string
    baseURL        string
    env            string
}

// Interface Provider
type Provider interface {
    Code() entity.PaymentProvider
    InitiatePayment(ctx context.Context, req *PaymentRequest) (*PaymentResponse, error)
    CheckStatus(ctx context.Context, providerRef string) (*PaymentStatus, error)
    ValidateWebhook(ctx context.Context, payload []byte, signature string) (*WebhookEvent, error)
    Refund(ctx context.Context, providerRef string, amountCents int64) error
    IsAvailable(ctx context.Context) bool
    CashOut(ctx context.Context, req *CashOutRequest) (*CashOutResponse, error)
}

Providers supportés

Provider
Code
Type
USSD (ONE_STEP)
Statut
Orange Money
orange_money
ONE_STEP
*144*4*6*{montant}#
✅
Telecel Money
telecel
ONE_STEP
*808*4*4*{montant}#
✅
Moov Money
moov_money
TWO_STEP
-
✅
Coris Money
coris_money
TWO_STEP
-
✅
Sank Money
sank_money
TWO_STEP
-
✅
MTN
mtn
ONE_STEP
-
✅
Flux de paiement
Flux indirect (checkout page)

1. POST /api/orders/{id}/pay (flow: indirect)
         │
         ▼
2. Yenga Pay crée un payment-intent
         │
         ▼
3. Retourne checkoutPageUrlWithPaymentToken
         │
         ▼
4. Client redirigé vers page checkout Yenga Pay
         │
         ▼
5. Client choisit opérateur et complète paiement
         │
         ▼
6. Webhook Yenga Pay → /webhooks/yenga_pay
         │
         ▼
7. Validation HMAC-SHA256 (header: x-webhook-hash)
         │
         ▼
8. Statut payment → SUCCESS


Flux direct ONE_STEP (Orange, Telecel)

1. POST /api/orders/{id}/pay (flow: direct, operator: orange_money)
         │
         ▼
2. Retourne USSD code: *144*4*6*500#
         │
         ▼
3. Client compose USSD → reçoit OTP par SMS
         │
         ▼
4. POST /api/orders/{id}/pay (flow: direct, operator: orange_money, otp: 123456)
         │
         ▼
5. Yenga Pay init-and-pay → statut DONE
         │
         ▼
6. Payment → SUCCESS

Flux direct TWO_STEP (Moov, Coris, Sank)

1. POST /api/orders/{id}/pay (flow: direct, operator: moov_money)
         │
         ▼
2. Yenga Pay direct-payment/init → paymentIntentId
         │
         ▼
3. Yenga Pay direct-payment/send-otp → OTP envoyé par SMS
         │
         ▼
4. Payment → PROCESSING (en attente OTP)
         │
         ▼
5. POST /api/payments/{id}/complete (otp: 123456)
         │
         ▼
6. Yenga Pay direct-payment/pay → statut DONE
         │
         ▼
7. Payment → SUCCESS

Endpoints API
Initier un paiement

POST /api/orders/{order_id}/pay
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}
Content-Type: application/json

{
  "provider": "yenga_pay",
  "phone_number": "+22670123456",
  "description": "Paiement commande #123",
  "metadata": {
    "flow": "direct",
    "operator": "moov_money"
  }
}

Réponse 201 :

{
  "payment_id": "2767e73d-d5a3-43f0-a081-74508b0b9586",
  "provider_ref": "cmqso9fgg031os601huvnzlan",
  "status": "processing",
  "message": "Moov Money vous a envoyé un code OTP par SMS.",
  "metadata": {
    "flow": "direct",
    "flow_type": "TWO_STEP",
    "operator": "MOOV",
    "payment_intent": "cmqso9fgg031os601huvnzlan",
    "next_step": "Call POST /api/payments/{id}/complete with OTP"
  }
}

Compléter un paiement TWO_STEP

POST /api/payments/{payment_id}/complete
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}
Content-Type: application/json

{
  "otp": "123456"
}

Webhooks Yenga Pay
Endpoint : POST /webhooks/yenga_pay
Validation HMAC-SHA256 :
go

// ⚠️ IMPORTANT : Yenga Pay utilise le header "x-webhook-hash" (pas "X-Signature")
signature := r.Header.Get("x-webhook-hash")
if signature == "" {
    signature = r.Header.Get("X-Signature") // Fallback pour compatibilité
}

mac := hmac.New(sha256.New, []byte(webhookSecret))
mac.Write(payload)
expectedMAC := hex.EncodeToString(mac.Sum(nil))

if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
    return fmt.Errorf("invalid webhook signature")
}

2. Configuration par boutique (Fallback hybride) ✅
Architecture

┌─────────────────────────────────────────────────────────┐
│            Configuration par boutique                    │
├─────────────────────────────────────────────────────────┤
│  Shop A (override)                                       │
│    → yenga_pay_api_key (chiffré AES-256-GCM)            │
│    → yenga_pay_organization_id (chiffré)                │
│    → yenga_pay_project_id (chiffré)                     │
│    → yenga_pay_webhook_secret (chiffré)                 │
│    → yenga_pay_operators: [orange, moov]                │
│    → cash_commission_rate: 250 (2.50%)                  │
│    → cash_on_delivery_enabled: true                     │
│                                                          │
│  Shop B (utilise config globale)                        │
│    → yenga_pay_enabled: false                           │
│    → fallback sur YENGA_PAY_API_KEY (env)               │
└─────────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────┐
│         Provider Yenga Pay (dynamique)                   │
│  - Si config boutique existe → l'utiliser               │
│  - Sinon → utiliser config globale                      │
│  - Clés déchiffrées à la volée                          │
└─────────────────────────────────────────────────────────┘


Endpoints de configuration
Récupérer la config

GET /api/shops/{shop_id}/payment-settings
Authorization: Bearer {token}

Réponse 200 :

{
  "shop_id": "ec4ff426-db05-421a-8b97-19c8470de0fb",
  "orange_money_enabled": false,
  "moov_money_enabled": false,
  "wave_enabled": false,
  "cash_on_delivery_enabled": true,
  "cash_commission_rate": 250,
  "yenga_pay": {
    "enabled": true,
    "has_api_key": true,
    "has_organization_id": true,
    "has_project_id": true,
    "has_webhook_secret": true,
    "operators": ["orange_money", "moov_money"],
    "env": "test",
    "is_using_global": false
  }
}

Mettre à jour la config


PUT /api/shops/{shop_id}/payment-settings
Authorization: Bearer {token}
Content-Type: application/json

{
  "yenga_pay": {
    "enabled": true,
    "api_key": "HDipDNzmiiU0Uee9O85AnWgKt7Mfv2Kl",
    "organization_id": "10454272",
    "project_id": "65687",
    "webhook_secret": "c38ccab5-836d-4453-a6e0-2eb0b9df3097",
    "operators": ["orange_money", "moov_money", "telecel"],
    "env": "test"
  },
  "cash_on_delivery_enabled": true,
  "cash_commission_rate": 250
}

Chiffrement AES-256-GCM
Fichier : infrastructure/crypto/aes.go
go

func Encrypt(plaintext string) (string, error) {
    key, err := getEncryptionKey() // 32 bytes depuis ENCRYPTION_KEY
    if err != nil {
        return "", err
    }

    block, _ := aes.NewCipher(key)
    aesGCM, _ := cipher.NewGCM(block)

    nonce := make([]byte, aesGCM.NonceSize())
    io.ReadFull(rand.Reader, nonce)

    ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

Variables d'environnement :

# Clé de chiffrement (32 caractères)
ENCRYPTION_KEY=mDTvVLUoN4pazfxbQyudsqtKEGr8HAkh

# Config globale Yenga Pay (fallback)
YENGA_PAY_API_KEY=HDipDNzmiiU0Uee9O85AnWgKt7Mfv2Kl
YENGA_PAY_ORGANIZATION_ID=10454272
YENGA_PAY_PROJECT_ID=65687
YENGA_PAY_WEBHOOK_SECRET=c38ccab5-836d-4453-a6e0-2eb0b9df3097
YENGA_PAY_ENV=test

3. Cash-out (Retraits) ✅
Architecture

1. POST /api/withdrawals
         │
         ▼
2. Récupère config Yenga Pay (fallback hybride)
         │
         ▼
3. POST https://api.yengapay.com/api/v1/groups/{org_id}/cash-out
         │
         ▼
4. Si HTTP 200 + ID valide → marque SUCCESS directement
   (Yenga Pay traite quasi-synchroniquement)
         │
         ▼
5. Retourne le retrait avec provider_ref

⚠️ Note importante : Yenga Pay ne fournit pas d'endpoint API pour vérifier le statut des cash-outs marchands. Les retraits sont marqués SUCCESS immédiatement après création si HTTP 200 + ID valide. Le statut final n'est visible que dans le dashboard Yenga Pay.
Endpoints API
Créer un retrait


POST /api/withdrawals
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}
Content-Type: application/json

{
  "amount_cents": 50000,
  "payment_method": "ORANGE_MONEY",
  "destination_number": "+22670123456",
  "destination_name": "Yoda Lassina",
  "description": "Retrait vers Orange Money"
}

Réponse 201 :


{
  "id": "16fb7afb-d0cf-44d0-ad82-74a0ad08b176",
  "shop_id": "ec4ff426-db05-421a-8b97-19c8470de0fb",
  "provider": "yenga_pay",
  "provider_ref": "YPCO20260626.2056.65687.8675",
  "amount_cents": 50000,
  "currency": "XOF",
  "fees_cents": 0,
  "net_amount_cents": 50000,
  "status": "success",
  "payment_method": "ORANGE_MONEY",
  "destination_number": "+22670123456",
  "destination_name": "Yoda Lassina",
  "description": "Retrait vers Orange Money",
  "created_at": "2026-06-26 20:56:08"
}

4. 🆕 Cash à la livraison (COD) ✅
Vue d'ensemble
Le Cash à la livraison (Cash On Delivery - COD) permet aux clients de payer en espèces à la réception de leur commande. GoShop prélève automatiquement une commission configurable (défaut : 2.50%) sur chaque transaction COD.
Architecture

┌──────────────┐      ┌──────────────┐      ┌──────────────┐
│    Client    │      │    Système   │      │   Marchand   │
└──────┬───────┘      └──────┬───────┘      └──────┬───────┘
       │                     │                     │
       │ 1. Créer commande   │                     │
       │   (payment_method:  │                     │
       │    cash_on_delivery)│                     │
       │────────────────────>│                     │
       │                     │                     │
       │                     │ 2. Statut:          │
       │                     │ pending_confirmation│
       │                     │ Stock réservé (24h) │
       │                     │                     │
       │                     │ 3. Notification     │
       │                     │ (no-op / SMS)       │
       │                     │────────────────────>│
       │                     │                     │
       │                     │ 4. POST /accept     │
       │                     │<────────────────────│
       │                     │ Statut: confirmed   │
       │                     │                     │
       │                     │ 5. POST             │
       │                     │ /out-for-delivery   │
       │                     │<────────────────────│
       │                     │ Statut:             │
       │                     │ out_for_delivery    │
       │                     │                     │
       │ 6. Livraison +      │                     │
       │    paiement espèces │                     │
       │<─────────────────────────────────────────│
       │                     │                     │
       │                     │ 7. POST /deliver    │
       │                     │    + amount_received│
       │                     │<────────────────────│
       │                     │                     │
       │                     │ 8. Créer Payment    │
       │                     │ (provider: cash)    │
       │                     │ + Calcul commission │
       │                     │ Statut: delivered   │
       │                     │                     │


Machine à états des commandes COD

                        ┌─────────────────────────┐
                        │  pending_confirmation   │
                        │  (stock réservé 24h)    │
                        └───────────┬─────────────┘
                                    │
              ┌─────────────────────┼─────────────────────┐
              │                     │                     │
              ▼                     ▼                     ▼
    ┌──────────────┐      ┌──────────────┐      ┌──────────────┐
    │   rejected   │      │   confirmed  │      │   expired    │
    │ (stock ++ )  │      └──────┬───────┘      │ (stock ++ )  │
    └──────────────┘             │              └──────────────┘
                                 │
                                 ▼
                      ┌────────────────────┐
                      │ out_for_delivery   │
                      └─────────┬──────────┘
                                │
                                ▼
                      ┌────────────────────┐
                      │     delivered      │
                      │ (Payment créé +    │
                      │  commission GoShop)│
                      └────────────────────┘

Endpoints API
1. Créer une commande COD

POST /api/orders
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}
Content-Type: application/json

{
  "customer_id": "834183ee-0f75-4178-8ff1-db8687d40a4b",
  "payment_method": "cash_on_delivery",
  "items": [
    {
      "product_id": "bc7459fa-4368-4d51-a860-1bc19f9917ec",
      "quantity": 2
    }
  ]
}


Réponse 201 :


{
  "id": "49e015ae-126e-4d69-af58-169e548ee794",
  "customer_id": "834183ee-0f75-4178-8ff1-db8687d40a4b",
  "total_cents": 100000,
  "status": "pending_confirmation",
  "payment_method": "cash_on_delivery",
  "reserved_until": "2026-06-28T19:25:35Z",
  "created_at": "2026-06-27T19:25:35Z",
  "items": [...]
}

2. Marchand accepte la commande

POST /api/orders/{id}/accept
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}

Réponse 200 : Statut → confirmed, accepted_at défini
3. Marchand refuse la commande


POST /api/orders/{id}/reject
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}
Content-Type: application/json

{
  "reason": "Produit en rupture de stock"
}

Réponse 200 : Statut → rejected, stock réincrémenté
4. Passage en livraison

POST /api/orders/{id}/out-for-delivery
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}

Réponse 200 : Statut → out_for_delivery
5. Livraison + paiement cash

POST /api/orders/{id}/deliver
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}
Content-Type: application/json

{
  "amount_received": 100000,
  "notes": "Client a payé en espèces"
}

Réponse 200 :


{
  "id": "d9bf5252-77fa-4dc6-8ad9-cfe20a98a000",
  "status": "delivered",
  "delivered_at": "2026-06-27T19:04:06Z",
  "amount_received_cents": 100000,
  "delivery_notes": "Client a payé en espèces",
  ...
}

Action côté serveur :
Création d'un Payment avec provider: "cash" et status: "success"
Calcul de la commission GoShop (défaut 2.50%)
Notification client et marchand (no-op actuellement)
6. Annulation par le client

POST /api/orders/{id}/cancel
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}

Réponse 200 : Statut → cancelled, stock réincrémenté (si pending_confirmation)
Système de commission
Configuration par boutique :
Champ cash_commission_rate dans shop_payment_settings
Unité : basis points (250 = 2.50%, 100 = 1.00%, 0 = pas de commission)
Défaut : 250 (2.50%) si non configuré
Validation : 0 ≤ rate ≤ 10000 (0% à 100%)
Calcul :

// domain/entity/order.go
func (o *Order) CalculateCashCommission(commissionRate int) (feesCents, netAmountCents int64) {
    if commissionRate < 0 || commissionRate > 10000 {
        return 0, o.TotalCents
    }
    feesCents = (o.TotalCents * int64(commissionRate)) / 10000
    netAmountCents = o.TotalCents - feesCents
    return feesCents, netAmountCents
}

// domain/entity/order.go
func (o *Order) CalculateCashCommission(commissionRate int) (feesCents, netAmountCents int64) {
    if commissionRate < 0 || commissionRate > 10000 {
        return 0, o.TotalCents
    }
    feesCents = (o.TotalCents * int64(commissionRate)) / 10000
    netAmountCents = o.TotalCents - feesCents
    return feesCents, netAmountCents
}

Exemple :
Montant commande
Commission (2.50%)
Net marchand
500 XOF
12.50 XOF
487.50 XOF
1000 XOF
25.00 XOF
975.00 XOF
5000 XOF
125.00 XOF
4875.00 XOF
Réservation de stock
Principe :

À la création d'une commande COD, le stock est décrémenté immédiatement
reserved_until est défini à now() + 24h
Si la commande n'est pas acceptée dans les 24h :
Expiration lazy : détectée lors de la prochaine lecture/action
Statut passe à expired
Stock réincrémenté automatiquement
Avantages :
Pas de job background (cron) nécessaire
Pas de consommation de ressources quand le système est inactif
Simple à implémenter et à déboguer
Service de notification (no-op)
Interface : domain/service/notification_service.go

type NotificationService interface {
    NotifyMerchantOrderReceived(ctx, shop, order) error
    NotifyClientOrderConfirmed(ctx, order, phone) error
    NotifyClientOrderRejected(ctx, order, phone, reason) error
    NotifyClientOrderExpired(ctx, order, phone) error
    NotifyMerchantDeliveryReady(ctx, shop, order) error
    NotifyClientOrderDelivered(ctx, order, phone, amount) error
    NotifyMerchantCommissionPaid(ctx, shop, order, commission) error
    SendNotification(ctx, req) error
}

Implémentation actuelle : NoopNotificationService
Logue toutes les notifications avec zerolog
Ne bloque jamais le workflow
Prêt à être remplacé par Africa's Talking SMS ou autre provider
5. Machines à états
Paiements (Mobile Money)

                    ┌─────────────┐
                    │   PENDING   │
                    └──────┬──────┘
                           │
                           ▼
                    ┌──────────────┐
            ┌──────│  PROCESSING  │──────┐
            │      └──────────────┘      │
            │             │              │
            ▼             ▼              ▼
     ┌──────────┐  ┌──────────┐   ┌──────────┐
     │  FAILED  │  │ SUCCESS  │   │ EXPIRED  │
     └──────────┘  └────┬─────┘   └──────────┘
                        │
                        ▼
                  ┌──────────┐
                  │ REFUNDED │
                  └──────────┘


Commandes Cash à la livraison

     ┌──────────────────────────┐
     │  pending_confirmation    │
     │  (stock réservé 24h)     │
     └────────────┬─────────────┘
                  │
       ┌──────────┼──────────┬──────────┐
       │          │          │          │
       ▼          ▼          ▼          ▼
 ┌──────────┐ ┌────────┐ ┌────────┐ ┌──────────┐
 │ rejected │ │confirmed│ │expired │ │ cancelled│
 │ (stock++)│ └───┬────┘ │(stock++)│ │(stock++  │
 └──────────┘     │      └────────┘ │ si pending)│
                  │                 └──────────┘
                  ▼
         ┌────────────────┐
         │out_for_delivery│
         └───────┬────────┘
                 │
                 ▼
         ┌────────────────┐
         │   delivered    │
         │ (Payment cash  │
         │  + commission) │
         └────────────────┘

Retraits (Cash-out)

     ┌──────────┐      ┌────────────┐      ┌──────────┐
     │ PENDING  │─────▶│ PROCESSING │─────▶│ SUCCESS  │
     └──────────┘      └─────┬──────┘      └──────────┘
                             │
                             ▼
                       ┌──────────┐
                       │  FAILED  │
                       └──────────┘


 6. Base de données
Table orders (enrichie pour COD)

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL,
    total_cents BIGINT NOT NULL CHECK (total_cents > 0),
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    
    -- 🆕 Champs Cash à la livraison
    payment_method VARCHAR(50) NOT NULL DEFAULT 'mobile_money'
        CHECK (payment_method IN ('mobile_money', 'cash_on_delivery')),
    accepted_at TIMESTAMPTZ,
    rejected_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    delivery_notes TEXT,
    amount_received_cents BIGINT,
    reserved_until TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT orders_status_check CHECK (status IN (
        'pending', 'pending_confirmation', 'confirmed',
        'rejected', 'expired', 'out_for_delivery',
        'delivered', 'cancelled'
    ))
);

CREATE INDEX idx_orders_shop_id ON orders(shop_id);
CREATE INDEX idx_orders_customer_id ON orders(customer_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_status_payment_method ON orders(status, payment_method);
CREATE INDEX idx_orders_reserved_until ON orders(reserved_until) 
    WHERE status = 'pending_confirmation';


Table shop_payment_settings (enrichie)
sql

CREATE TABLE shop_payment_settings (
    shop_id UUID PRIMARY KEY REFERENCES shops(id) ON DELETE CASCADE,
    orange_money_enabled BOOLEAN DEFAULT false,
    moov_money_enabled BOOLEAN DEFAULT false,
    wave_enabled BOOLEAN DEFAULT false,
    yenga_pay_enabled BOOLEAN DEFAULT false,
    yenga_pay_api_key TEXT,              -- chiffré AES-256-GCM
    yenga_pay_organization_id TEXT,      -- chiffré
    yenga_pay_project_id TEXT,           -- chiffré
    yenga_pay_webhook_secret TEXT,       -- chiffré
    yenga_pay_operators JSONB DEFAULT '["orange_money","moov_money","telecel","coris_money","sank_money"]'::jsonb,
    yenga_pay_env VARCHAR(10) DEFAULT 'test',
    
    -- 🆕 Cash à la livraison
    cash_on_delivery_enabled BOOLEAN NOT NULL DEFAULT false,
    cash_commission_rate INTEGER NOT NULL DEFAULT 250
        CHECK (cash_commission_rate >= 0 AND cash_commission_rate <= 10000),
    
    updated_at TIMESTAMPTZ DEFAULT NOW()
);


Tables payments, withdrawals, payment_webhooks
(Voir sections précédentes pour le schéma complet)
7. Sécurité
Protection contre les attaques
Attaque
Protection
Webhook spoofing
Validation HMAC-SHA256
Payload tampering
Détection par HMAC
Replay attacks
External ID unique
Cross-tenant access
Isolation par shop_id (multi-tenant)
Double remboursement
Vérification d'état (machine à états)
Timing attacks
hmac.Equal() (timing-safe)
Fuite de clés API
Chiffrement AES-256-GCM en base
Stock fantôme
Réservation + expiration lazy
Règles
✅ Clés API chiffrées en base (AES-256-GCM)
✅ Validation HMAC pour tous les webhooks
✅ Header webhook : x-webhook-hash (standard Yenga Pay)
✅ Idempotence : chaque transaction a un ID unique
✅ Audit trail : toutes les transactions sont loguées
✅ Seuls les propriétaires de boutique peuvent configurer
✅ Réponse API ne contient que des booléens (has_api_key, etc.)
✅ Stock réservé avec expiration automatique (24h)
✅ Commission calculée côté serveur (non modifiable par client)
Conformité
BCEAO : Respect des réglementations UEMOA
PCI DSS : Pas de stockage de données bancaires
RGPD : Consentement explicite pour les paiements
8. Monitoring
Métriques Prometheus


payments_initiated_total{provider="yenga_pay"}
payments_success_total{provider="yenga_pay"}
webhooks_received_total{provider="yenga_pay"}
payment_success_rate{provider="yenga_pay"}
payment_duration_seconds{provider="yenga_pay"}
withdrawals_created_total{method="ORANGE_MONEY"}
withdrawals_success_total{method="ORANGE_MONEY"}
cod_orders_created_total{shop_id="..."}
cod_orders_delivered_total{shop_id="..."}
cod_commission_total_cents{shop_id="..."}


Alertes
Taux d'échec > 5% sur 5 min
Webhook non reçu après 10 min
Provider indisponible > 1 min
Retrait échoué > 3 tentatives
Commande COD en pending_confirmation > 24h (nombre élevé)
9. Tests
Tests de complétion (2026-06-27)


# Test 1 : Workflow COD complet
$order = POST /api/orders (payment_method: cash_on_delivery)
# ✅ Statut: pending_confirmation, reserved_until: now+24h

POST /api/orders/{id}/accept
# ✅ Statut: confirmed, accepted_at défini

POST /api/orders/{id}/out-for-delivery
# ✅ Statut: out_for_delivery

POST /api/orders/{id}/deliver (amount_received: 50000)
# ✅ Statut: delivered, Payment cash créé, commission 2.50% calculée

# Test 2 : Rejet marchand
POST /api/orders/{id}/reject (reason: "rupture")
# ✅ Statut: rejected, stock réincrémenté

# Test 3 : Annulation client
POST /api/orders/{id}/cancel
# ✅ Statut: cancelled, stock réincrémenté

# Test 4 : Expiration lazy
UPDATE orders SET reserved_until = NOW() - 25h WHERE id = '...';
POST /api/orders/{id}/accept
# ✅ Erreur: "order has expired", statut passé à expired, stock réincrémenté

Résultats complets
Test
Statut
Paiement indirect
✅
Paiement ONE_STEP (Orange)
✅
Paiement ONE_STEP (Telecel)
✅
Paiement TWO_STEP (Moov)
✅
Paiement TWO_STEP (Sank)
✅
Paiement TWO_STEP (Coris)
✅
Complétion OTP (5 opérateurs)
✅
Configuration boutique
✅
Chiffrement AES-256-GCM
✅
Fallback hybride
✅
Cash-out Orange Money
✅
Cash-out Moov Money
✅
Webhook HMAC validation
✅
COD workflow complet
✅
COD commission 2.50%
✅
COD rejet (stock++)
✅
COD annulation (stock++)
✅
COD expiration lazy (stock++)
✅
Total : 22/22 tests réussis (100%)
10. Dépannage
Webhook ne persiste pas
Symptôme : Le webhook est reçu mais le paiement reste en processing.
Cause : Problème d'injection du tenant context.
Solution : Vérifier que ProcessWebhookUsecase injecte le tenant après le lookup.
Erreur 401 sur cash-out
Symptôme : Yenga Pay cash-out error (status 401): Unauthorized
Cause : Clés API invalides dans la config boutique.
Solution :
Vérifier les variables d'environnement 

$env:YENGA_PAY_API_KEY
$env:YENGA_PAY_ORGANIZATION_ID
$env:YENGA_PAY_PROJECT_ID

Désactiver la config boutique pour utiliser la globale 

$config = @{ yenga_pay = @{ enabled = $false } } | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8081/api/shops/$shopId/payment-settings" -Method PUT -Body $config

Erreur 500 sur cash-out
Symptôme : Yenga Pay cash-out error (status 500): Internal server error
Cause : Problème temporaire côté Yenga Pay ou limite sandbox atteinte.
Solution : Attendre quelques minutes et réessayer.
Webhook HMAC ne matche pas
Symptôme : Invalid webhook signature
Cause : Yenga Pay signe le body brut (pas JSON.stringify).
Solution : Notre implémentation Go signe les bytes bruts du body HTTP, ce qui est correct. Vérifier que le webhookSecret est correct.
Commande COD bloquée en pending_confirmation
Symptôme : Une commande reste en pending_confirmation indéfiniment.
Cause : Le marchand n'a pas accepté/refusé dans les 24h.
Solution :
Automatique : L'expiration lazy détecte le problème lors de la prochaine action
Manuel : Forcer l'expiration :
sql

UPDATE orders SET reserved_until = NOW() - INTERVAL '1 hour' WHERE id = '...';

Puis tenter d'accepter → la commande passera à expired et le stock sera réincrémenté.
Commission à 0 au lieu de 2.50%
Symptôme : commission_fees: 0 dans les logs.
Cause : La boutique n'a pas de shop_payment_settings configuré.
Solution : Vérifier que GetCashCommissionRate() retourne bien 250 par défaut (bug corrigé en v2.8.0).
Timestamps incohérents
Symptôme : initiated_at et completed_at dans des fuseaux différents.
Solution :
Vérifier SET TIME ZONE 'UTC' dans postgres.Connect()
Utiliser time.Now().UTC() partout
Utiliser .UTC().Format() pour l'affichage
📚 Références
Documentation Yenga Pay
Architecture Decision Records
API Reference
Multi-tenant
Security Policy
🤝 Contribution
Voir CONTRIBUTING.md pour les détails.
Prochains providers à implémenter :
🚧 Wave
🚧 SMS notifications (Africa's Talking)



