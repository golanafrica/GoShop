# 💳 Système de paiement

**Version** : v2.5.0-withdrawals  
**Date** : 2026-06-26  
**Statut** : ✅ Production-ready (Yenga Pay réel + Cash-out opérationnel)

---

## 📋 État actuel

### ✅ Implémenté

| Version | Fonctionnalité | Statut |
|---------|----------------|--------|
| v2.1.0 | Orange Money + Moov Money (mock) | ✅ |
| v2.2.0 | Yenga Pay API réelle (6 opérateurs) | ✅ |
| v2.3.0 | Configuration par boutique (hybride) | ✅ |
| v2.4.0 | Chiffrement AES-256-GCM des clés API | ✅ |
| v2.5.0 | Cash-out (retraits Mobile Money) | ✅ |

### 🚧 À venir
- **Wave** : Provider Mobile Money
- **MTN MoMo** : Provider Mobile Money
- **Cash à la livraison** : Workflow complet
- **Crédit** : Paiement en tranches avec score de fiabilité

---

## 🎯 Vue d'ensemble

Système modulaire supportant **4 modes de paiement** :

1. **Yenga Pay** ✅ (Orange, Moov, Telecel, Coris, Sank, MTN) - API réelle
2. **Orange Money** ✅ (mock) - USSD `#144*111#`
3. **Moov Money** ✅ (mock) - USSD `#135*2#`
4. **Cash-out** ✅ (retraits vers Mobile Money via Yenga Pay)

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
    CashOut(ctx context.Context, req *CashOutRequest) (*CashOutResponse, error) // 🆕
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
7. Validation HMAC-SHA256
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
  "message": "Moov Money vous a envoyé un code OTP par SMS. Code OTP reçu par SMS requis pour finaliser",
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

Réponse 200 :

{
  "id": "2767e73d-d5a3-43f0-a081-74508b0b9586",
  "order_id": "3fc84aa6-4dbd-4b2d-b8d4-e07dee77c8eb",
  "provider": "yenga_pay",
  "provider_ref": "cmqso9fgg031os601huvnzlan",
  "amount_cents": 50000,
  "currency": "XOF",
  "status": "success",
  "customer_phone": "+22670123456"
}

Webhooks Yenga Pay
Endpoint : POST /webhooks/yenga_pay
Validation HMAC-SHA256 :


mac := hmac.New(sha256.New, []byte(webhookSecret))
mac.Write(payload)
expectedMAC := hex.EncodeToString(mac.Sum(nil))

if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
    return fmt.Errorf("invalid webhook signature")
}

Payload :

{
  "apiEnv": "prod",
  "paymentStatus": "DONE",
  "transId": "YP20241021.1251.58962478",
  "projectId": "65687",
  "paymentIntentId": "cm0qobvl10001s6018w5ppwj8",
  "paymentSource": "MoovMoneyAPI",
  "customerNumber": "60606060",
  "paymentAmount": 195,
  "paymentFees": 5,
  "contryOrigin": "BF",
  "reference": "6646846426259895",
  "currency": "XOF"
}

Header : x-webhook-hash: {hmac_signature}

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
  }
}


Chiffrement AES-256-GCM
Fichier : infrastructure/crypto/aes.go

// Encrypt chiffre une valeur avec AES-256-GCM
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


Vérification en base

SELECT 
    shop_id,
    yenga_pay_enabled,
    LEFT(yenga_pay_api_key, 50) AS api_key_encrypted,
    yenga_pay_operators
FROM shop_payment_settings
WHERE shop_id = 'ec4ff426-db05-421a-8b97-19c8470de0fb';

Résultat :

shop_id              | yenga_pay_enabled | api_key_encrypted
---------------------+-------------------+----------------------------------------------
ec4ff426-db05-...    | t                 | jMTDyJfO2G6k0kfSRLsgAJvPmlsysl4rccfsAvqXvq7y


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
4. Sauvegarde Withdrawal en DB (statut: PENDING)
         │
         ▼
5. Retourne le retrait avec provider_ref
         │
         ▼
6. Yenga Pay traite le retrait (quelques secondes)
         │
         ▼
7. Statut → SUCCESS (frais calculés)


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
  "net_amount_cents": 0,
  "status": "processing",
  "payment_method": "ORANGE_MONEY",
  "destination_number": "+22670123456",
  "destination_name": "Yoda Lassina",
  "description": "Retrait vers Orange Money",
  "created_at": "2026-06-26 20:56:08"
}

Lister les retraits

GET /api/withdrawals?limit=50&offset=0
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}

Récupérer un retrait

GET /api/withdrawals/{withdrawal_id}
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}

Méthodes de paiement supportées


Méthode
Code
Statut
Orange Money
ORANGE_MONEY
✅
Moov Money
MOOV_MONEY
✅
Telecel Money
TELECEL_MONEY
✅
Coris Money
CORIS_MONEY
✅
Sank Money
SANK_MONEY
✅
MTN
MTN
✅
Tests réalisés
#
Montant
Méthode
Provider Ref
Statut
1
500 XOF
ORANGE_MONEY
YPCO20260626.2056.65687.8675
✅ Réussi
2
200 XOF
MOOV_MONEY
YPCO20260626.2103.65687.7311
✅ Réussi
Total retiré : 700 XOF
Taux de succès : 100%


4. Machine à états
Paiements

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
                  


Retraits

     ┌──────────┐      ┌────────────┐      ┌──────────┐
     │ PENDING  │─────▶│ PROCESSING │─────▶│ SUCCESS  │
     └──────────┘      └─────┬──────┘      └──────────┘
                             │
                             ▼
                       ┌──────────┐
                       │  FAILED  │
                       └──────────┘



5. Base de données
Table payments

CREATE TABLE payments (
  id UUID PRIMARY KEY,
  shop_id UUID NOT NULL REFERENCES shops(id),
  order_id UUID NOT NULL REFERENCES orders(id),
  provider VARCHAR(50) NOT NULL,
  provider_ref VARCHAR(255),
  amount_cents BIGINT NOT NULL,
  currency VARCHAR(3) NOT NULL DEFAULT 'XOF',
  status VARCHAR(20) NOT NULL,
  customer_phone VARCHAR(20),
  customer_email VARCHAR(255),
  description TEXT,
  metadata JSONB,
  initiated_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_shop_id ON payments(shop_id);
CREATE INDEX idx_payments_order_id ON payments(order_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_provider_ref ON payments(provider, provider_ref);

Table shop_payment_settings

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
    updated_at TIMESTAMPTZ DEFAULT NOW()
);


Table withdrawals

CREATE TABLE withdrawals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL DEFAULT 'yenga_pay',
    provider_ref VARCHAR(255),
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'XOF',
    fees BIGINT DEFAULT 0,
    net_amount BIGINT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' 
        CHECK (status IN ('pending', 'processing', 'success', 'failed', 'cancelled')),
    payment_method VARCHAR(50) NOT NULL
        CHECK (payment_method IN (
            'ORANGE_MONEY', 'MOOV_MONEY', 'TELECEL_MONEY', 
            'CORIS_MONEY', 'SANK_MONEY', 'MTN'
        )),
    destination_number VARCHAR(20) NOT NULL,
    destination_name VARCHAR(255),
    destination_email VARCHAR(255),
    description TEXT,
    error_message TEXT,
    operator_transaction_id VARCHAR(255),
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_withdrawals_shop_id ON withdrawals(shop_id);
CREATE INDEX idx_withdrawals_status ON withdrawals(status);
CREATE INDEX idx_withdrawals_provider_ref ON withdrawals(provider_ref);

Table payment_webhooks

CREATE TABLE payment_webhooks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider VARCHAR(50) NOT NULL,
  event_type VARCHAR(50),
  external_id VARCHAR(255),
  payload JSONB NOT NULL,
  signature TEXT,
  signature_validated BOOLEAN NOT NULL,
  processing_error TEXT,
  processed BOOLEAN NOT NULL DEFAULT false,
  received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

6. Sécurité
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
Isolation par shop_id
Double remboursement
Vérification d'état
Timing attacks
hmac.Equal() (timing-safe)
Fuite de clés API
Chiffrement AES-256-GCM en base
Règles
✅ Clés API chiffrées en base (AES-256-GCM)
✅ Validation HMAC pour tous les webhooks
✅ Idempotence : chaque transaction a un ID unique
✅ Audit trail : toutes les transactions sont loguées
✅ Seuls les propriétaires de boutique peuvent configurer
✅ Réponse API ne contient que des booléens (has_api_key, etc.)
Conformité
BCEAO : Respect des réglementations UEMOA
PCI DSS : Pas de stockage de données bancaires
RGPD : Consentement explicite pour les paiements
7. Monitoring
Métriques Prometheus

payments_initiated_total{provider="yenga_pay"}
payments_success_total{provider="yenga_pay"}
webhooks_received_total{provider="yenga_pay"}
payment_success_rate{provider="yenga_pay"}
payment_duration_seconds{provider="yenga_pay"}
withdrawals_created_total{method="ORANGE_MONEY"}
withdrawals_success_total{method="ORANGE_MONEY"}


Alertes
Taux d'échec > 5% sur 5 min
Webhook non reçu après 10 min
Provider indisponible > 1 min
Retrait échoué > 3 tentatives
8. Tests
Tests réalisés

# Test 1 : Paiement indirect (checkout page)
$payBody = @{
    provider = "yenga_pay"
    phone_number = "+22670123456"
    metadata = @{ flow = "indirect" }
} | ConvertTo-Json -Depth 3

# ✅ Résultat : redirect_url retournée

# Test 2 : Paiement ONE_STEP (Orange Money)
$payBody = @{
    provider = "yenga_pay"
    phone_number = "+22670123456"
    metadata = @{ flow = "direct"; operator = "orange_money" }
} | ConvertTo-Json -Depth 3

# ✅ Résultat : USSD code *144*4*6*500#

# Test 3 : Paiement TWO_STEP (Moov Money)
$payBody = @{
    provider = "yenga_pay"
    phone_number = "+22670123456"
    metadata = @{ flow = "direct"; operator = "moov_money" }
} | ConvertTo-Json -Depth 3

# ✅ Résultat : OTP envoyé par SMS

# Test 4 : Compléter paiement TWO_STEP
$completeBody = @{ otp = "123456" } | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8081/api/payments/$paymentId/complete" ...

# ✅ Résultat : status = success

# Test 5 : Configuration par boutique
Invoke-RestMethod -Uri "http://localhost:8081/api/shops/$shopId/payment-settings" ...

# ✅ Résultat : config sauvegardée avec chiffrement

# Test 6 : Cash-out
$withdrawalBody = @{
    amount_cents = 50000
    payment_method = "ORANGE_MONEY"
    destination_number = "+22670123456"
} | ConvertTo-Json

# ✅ Résultat : retrait créé, 500 XOF transférés

Résultats
Test
Statut
Paiement indirect
✅
Paiement ONE_STEP (Orange)
✅
Paiement TWO_STEP (Moov)
✅
Complétion OTP
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
Total retiré : 700 XOF
Taux de succès : 100%
9. Dépannage
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
🚧 MTN MoMo
🚧 Cash à la livraison

