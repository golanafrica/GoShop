Voici les **deux docs à jour** (P0 reverse cash-out, ledger `payout.failed`, P1-C `processed` après succès métier), prêts à coller.

---

### `docs/payment-system.md`

```markdown
# 💳 Système de paiement GoShop

**Version doc** : v5.2.0  
**Couverture** : Yenga Pay (pay-in / cash-out), COD, paiement à tempérament (installments), escrow online, wallet & retraits, tontine, disputes  
**Statut** : code production multi-modules — valider secrets et env avant prod  
**Dernière MAJ** : 2026-10-08

---

## 1. Vue d’ensemble

| Mode                                                     | Statut | Notes                                                      |
| -------------------------------------------------------- | ------ | ---------------------------------------------------------- |
| **Yenga Pay** (Orange, Moov, Telecel, Coris, Sank, MTN…) | ✅      | API réelle, indirect + ONE_STEP + TWO_STEP                 |
| **COD** (cash à la livraison)                            | ✅      | Accept / reject / deliver + commission boutique            |
| **Paiement à tempérament** (installments)                | ✅      | Tranches Yenga + escrow / release + debt sweep             |
| **Cash-out** (retraits MM)                               | ✅      | Via Yenga ; gates dette / freeze / KYC ; **reverse** si fail |
| **Escrow online** (pay-in comptant)                      | ✅      | Hold après success → release (scheduler / zone)            |
| **Tontine**                                              | ✅      | Webhook dédié + held / redeem voucher                      |
| **Wave**                                                 | 📋     | Non prioritisé                                             |
| **SMS transactionnels**                                  | 📋     | Notif souvent no-op / WebSocket                            |

Base URL dev typique : **`http://localhost:8080`**.

**Important** : le tempérament n’est **pas** un provider séparé. L’encaissement des tranches passe par **Yenga Pay** ; la différence est le **schéma de règlement** (N paiements + règles de libération).

---

## 2. Cycle d’argent (online pay-in comptant) — v5.x

```text
Client paie (Yenga)
  → Payment status = success (webhook HMAC)
  → Fonds en escrow / held marchand
  → Shipping + delivery proofs
  → Délai zone (ex. BF-OUAGA-URB)
  → Scheduler auto-release OU resolve merchant_wins
  → Crédit wallet (+ CreditWithDebtSweep si debt_cents > 0)
  → Retrait cash-out si debt_cents = 0 et available > 0
```

**Dispute post-release (`customer_wins`)** : clawback sur `balance` puis `debt_cents` ; refund client sur le **canal pay-in** (MSISDN / opérateur).  
Doc détaillée : [12-wallet-debt-sweep.md](12-wallet-debt-sweep.md).

---

## 3. Yenga Pay

### Provider

`infrastructure/payment/yenga_pay_provider.go` — `InitiatePayment`, `CheckStatus`, `ValidateWebhook`, `CashOut`, refund via cash-out.

Auth appels API : header **`x-api-key`**.

### Flux client

| Flux         | Opérateurs typiques   | UX                                                   |
| ------------ | --------------------- | ---------------------------------------------------- |
| **Indirect** | Tous                  | Checkout page Yenga (`checkoutPageUrl…`)             |
| **ONE_STEP** | Orange, Telecel, MTN… | USSD / init-and-pay + OTP                            |
| **TWO_STEP** | Moov, Coris, Sank…    | init → send-otp → `POST /api/payments/{id}/complete` |

### Endpoints applicatifs

```http
POST /api/orders/{order_id}/pay
Authorization: Bearer …
X-Shop-Slug: …
Content-Type: application/json

{
  "provider": "yenga_pay",
  "phone_number": "+2267xxxxxxx",
  "metadata": { "flow": "indirect" }
}
```

```http
POST /api/payments/{payment_id}/complete
{ "otp": "123456" }
```

### Webhook pay-in

```http
POST /webhooks/yenga_pay
```

- Signature : header **`x-webhook-hash`** (HMAC-SHA256 du **body brut**), fallback éventuel `X-Signature`
- Secret : env global ou **settings boutique** (déchiffré)
- Idempotence : table `payment_webhooks` + migrations **036 / 037**

#### Cycle de vie `processed` (P1-C — v5.2)

| Étape | Comportement |
| ----- | ------------ |
| 1. Signature OK | `INSERT` webhook avec **`processed = false`** (`ON CONFLICT DO NOTHING`) |
| 2. Métier OK (payment + escrow / tontine / …) | `UPDATE … SET processed = true` |
| 3. Métier en échec | `processed` reste **false** + `processing_error` → **retry** provider possible |
| 4. Déjà `processed = true` | `ErrWebhookAlreadyProcessed` (idempotent, HTTP 200 côté handler typique) |

**Avant v5.2** : `processed=true` dès la validation HMAC → un échec métier bloquait les retries.  
**Ne pas logger** le body webhook ni le secret.

### Canal pay-in (métadonnées)

Après succès, `customer_phone` / `metadata` (ex. `payment_source`, MSISDN) alimentent le **canal de refund** (disputes). Les E2E « real pay-in » utilisent un numéro sandbox réel (pas de force SQL du téléphone).

---

## 4. Configuration boutique (hybride)

```text
Settings shop (chiffrés AES-256-GCM) ──si absents──► Env globale YENGA_PAY_*
```

```http
GET /api/shops/{id}/payment-settings
PUT /api/shops/{id}/payment-settings
```

Réponse **sans secrets** : `has_api_key`, `has_webhook_secret`, `operators`, `env`, flags COD / rates.

```bash
ENCRYPTION_KEY=<32-bytes-secret>
YENGA_PAY_API_KEY=<from-dashboard>
YENGA_PAY_ORGANIZATION_ID=<org>
YENGA_PAY_PROJECT_ID=<project>
YENGA_PAY_WEBHOOK_SECRET=<uuid-secret>
YENGA_PAY_ENV=test   # ou production
```

---

## 5. Cash-out (retraits)

```http
POST /api/withdrawals
X-Shop-Slug: …
{
  "amount_cents": 50000,
  "payment_method": "ORANGE_MONEY",
  "destination_number": "+2267xxxxxxx",
  "destination_name": "…"
}
```

### Gates (v5.1+)

Retrait **refusé** (HTTP 400 typique) si :

- `debt_cents > 0`
- wallet frozen
- `available_cents` insuffisant (`balance - held`)
- éventuellement boutique **KYC** non `verified`

### Séquence et compensation (P0 — v5.2)

```text
1. Gates OK
2. Débit wallet (ledger payout / debit)
3. Appel Yenga CashOut
   ├─ succès → withdrawal status success / processing + provider_ref
   └─ échec  → reversePayoutDebit (crédit compensation + ledger)
                → withdrawal failed + error_message
```

- **P0-A** (`create_withdrawal.go`) : si CashOut échoue **après** débit, compensation via `CreditWalletUsecase` (référence `withdrawal_reversal` / type dépôt ou payout_reversal selon implémentation).
- **P0-B** (`process_payout_webhook.go`) : webhook `payout.failed` → reverse **idempotent** (ledger + re-fetch withdrawal en transaction). Ne pas re-créditer si déjà reversed.

Yenga cash-out : souvent quasi-synchrone (HTTP 200 + ref). Suivi async via `process_payout_webhook` / dashboard opérateur.

---

## 6. Cash à la livraison (COD)

### États

`pending_confirmation` → `confirmed` → `out_for_delivery` → `delivered`  
Branches : `rejected` | `expired` (24h lazy) | `cancelled`

### Endpoints

| Action       | Endpoint                                                     |
| ------------ | ------------------------------------------------------------ |
| Créer        | `POST /api/orders` + `payment_method: cash_on_delivery`      |
| Accept       | `POST /api/orders/{id}/accept`                               |
| Reject       | `POST /api/orders/{id}/reject`                               |
| En livraison | `POST /api/orders/{id}/out-for-delivery`                     |
| Livré + cash | `POST /api/orders/{id}/deliver` + `{ "amount_received": … }` |
| Cancel       | `POST /api/orders/{id}/cancel`                               |

Commission : `cash_commission_rate` en **basis points** (250 = 2,50 %), calcul serveur.  
Stock : réservé à la création ; libéré reject / cancel / expire.

---

## 7. Paiement à tempérament (installments)

**Synonyme métier** : paiement en tranches / crédit à la consommation côté boutique (pas un crédit bancaire BCEAO classique).

### Principe

1. Le marchand configure un **plan** (nombre de tranches, échéances, zone / délai de libération).
2. Le client paie chaque tranche via **Yenga Pay** (mêmes flux pay-in + webhooks).
3. Les fonds restent en **séquestre / held** jusqu’à éligibilité de libération  
   (`DeliveredAt` + délai zone, scheduler installment, ou action manuelle).
4. À la libération : commission plateforme, puis **debt sweep** si `debt_cents > 0`  
   (`ReleaseEscrowFundsUsecase` ; ledger `debt_sweep` / `commission_debit` si `WalletTxnRepo` câblé).

### Différence avec un pay-in comptant

| | Comptant | Tempérament |
|--|----------|-------------|
| Nombre de paiements | 1 | N tranches (`pending` / `paid` / `overdue`) |
| Libération | Auto-release escrow commande | Scheduler installment et/ou `POST …/release-escrow` |
| Wallet | Crédit net après release | Idem, avec sweep de dette résiduelle |

### API (détail frontend / payloads)

Voir **[INSTALLMENT_API_SPECS.md](INSTALLMENT_API_SPECS.md)** :

- dashboard marchand `/api/merchant/installments/…`
- plan commande, liste des tranches, release-escrow
- montants **en centimes** ; champs utiles : `swept_cents`, `debt_cents_after`

### Code de référence

- `application/usecase/installment_usecase/release_escrow_funds.go`
- `application/scheduler/` (auto-release installment)
- Wiring `WalletTxnRepo` dans `internal/app/app.go`

---

## 8. Escrow, tontine, disputes (synthèse)

| Module                  | Rôle paiement                                    |
| ----------------------- | ------------------------------------------------ |
| Escrow online           | Hold après success ; release scheduler / dispute |
| Tempérament             | Tranches + release-escrow + debt sweep           |
| Tontine                 | Cotisations Yenga + held jusqu’au redeem voucher |
| Dispute `customer_wins` | Clawback + refund canal pay-in                   |
| Dispute `merchant_wins` | Claim release + crédit (+ sweep si dette)        |

Docs : [11-tontine-system.md](11-tontine-system.md), [12-wallet-debt-sweep.md](12-wallet-debt-sweep.md), [INSTALLMENT_API_SPECS.md](INSTALLMENT_API_SPECS.md).

---

## 9. Sécurité

| Risque              | Mitigation                                              |
| ------------------- | ------------------------------------------------------- |
| Webhook forgé       | HMAC body brut                                          |
| Replay / double     | Idempotence external_id + `processed` après succès      |
| Fuite clés          | AES-GCM en DB ; jamais dans logs/docs publics           |
| Cross-tenant        | `shop_id` + TenantResolver                              |
| Double crédit       | Idempotence wallet / unique txn ref                     |
| Timing HMAC         | `hmac.Equal`                                            |
| Cash-out échoué     | Reverse débit (synchrone + webhook `payout.failed`)     |

**Alerte** : d’anciennes versions de cette doc contenaient des **vrais secrets sandbox**. Les considérer **compromis** et les **rotationner** s’ils sont encore actifs.

---

## 10. Observabilité

- Logs : `request_id`, `payment_id`, `provider_ref`, `withdrawal_id` — [logging-guide.md](logging-guide.md)
- Métriques : paiements / webhooks / withdrawals / COD (Prometheus `/metrics`)
- E2E : `e2e-clawback-real-payin.ps1`, `e2e-debt-sweep-fraud.ps1`, `e2e-dispute-merchant-wins.ps1`, tests Go payment / withdrawal

---

## 11. Dépannage rapide

| Symptôme                       | Pistes                                                      |
| ------------------------------ | ----------------------------------------------------------- |
| Payment reste `processing`     | Webhook secret, HMAC, tenant, `processed=false` + retry     |
| Webhook « already processed »  | Normal si métier déjà OK ; sinon vérifier mark après succès |
| 401 cash-out                   | Clés shop vs globale ; org/project                          |
| Retrait 400                    | `debt_cents`, held, freeze, KYC                             |
| Cash-out fail, solde débité    | P0-A reverse synchrone ; sinon webhook `payout.failed` (P0-B) |
| Signature invalide             | Body brut, secret, header `x-webhook-hash`                  |
| COD bloqué pending             | `reserved_until` + expiration lazy                          |
| Release installment sans sweep | Wiring `WithTxnRepo` / rebuild app                          |

---

## 12. Références code

| Zone                | Path                                                              |
| ------------------- | ----------------------------------------------------------------- |
| Provider            | `infrastructure/payment/yenga_pay_provider.go`                    |
| Webhook HTTP        | `interfaces/handler/payment_handler/webhook_handler.go`           |
| Process webhook     | `application/usecase/payment_usecase/process_webhook.go`          |
| Tontine webhook     | `application/usecase/payment_usecase/process_tontine_webhook.go`  |
| Create withdrawal   | `application/usecase/withdrawal_usecase/create_withdrawal.go`     |
| Payout webhook      | `application/usecase/withdrawal_usecase/process_payout_webhook.go`|
| Escrow scheduler    | `application/scheduler/escrow_auto_release_scheduler.go`          |
| Release installment | `application/usecase/installment_usecase/release_escrow_funds.go` |

---

## 13. Historique documentaire

| Époque    | Focus                                                                      |
| --------- | -------------------------------------------------------------------------- |
| v2.1–v2.8 | Yenga multi-op, config shop, COD, cash-out basique                         |
| v5.0      | Zones, **paiement à tempérament**, escrow release                          |
| v5.1      | `debt_cents`, sweep, clawback, withdrawal gate, E2E real pay-in            |
| v5.2      | Reverse cash-out (P0-A), ledger `payout.failed` (P0-B), webhook P1-C       |

Les tableaux de **soldes sandbox juin 2026** de l’ancienne doc ne sont **plus** une référence opérationnelle.

---

**Dernière mise à jour** : 2026-10-08
```