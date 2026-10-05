Voici **`docs/12-wallet-debt-sweep.md`**, prêt à coller.

```markdown
# Wallet — Dette résiduelle, clawback & debt sweep

**Version :** v5.1.x (debt-sweep milestone)  
**Statut :** Livré et validé E2E (webhook + pay-in réel)  
**Dernière MAJ :** 2026-10-03

---

## 1. Objectif métier

Après un litige gagné par le client **une fois les fonds déjà libérés** au marchand (escrow `released`), GoShop doit :

1. Récupérer l’argent déjà crédité (clawback).
2. Si le solde ne suffit pas (ex. retrait partiel déjà effectué), enregistrer une **dette résiduelle** explicite (`debt_cents`).
3. Rembourser cette dette automatiquement sur les **prochains crédits** (auto-release, merchant_wins, installment, tontine release_held).
4. **Bloquer tout retrait** tant que `debt_cents > 0`.

Pas de gel automatique au moment du clawback : la dette est silencieuse jusqu’à recovery, puis gate retrait strict.

---

## 2. Modèle de données (`merchant_wallets`)

| Colonne | Rôle |
|---------|------|
| `balance_cents` | Solde ledger (peut baisser via clawback / commission / sweep) |
| `held_cents` | Fonds non disponibles (escrow / tontine hold) — **≥ 0** |
| `debt_cents` | Dette résiduelle post-clawback — **≥ 0** (contrainte SQL) |
| `is_frozen` | Gel admin / legacy (négatif / fraude) — orthogonal à `debt_cents` |

**Disponible pour retrait :**

```text
available_cents = max(0, balance_cents - held_cents)
```

**Règle retrait :**

```text
autorisé seulement si :
  !is_frozen
  AND debt_cents == 0
  AND available_cents > 0
  AND amount <= available_cents
  AND KYC marchand OK
```

API marchand (`GET /api/wallet`) expose notamment :

- `balance_cents`, `held_cents`, `available_cents`
- `debt_cents`, `debt_formatted`
- `withdrawal_blocked` = `is_frozen || debt_cents > 0 || available_cents <= 0`

---

## 3. Clawback (`customer_wins` post-release)

**Usecase :** `application/usecase/dispute_usecase/resolve_dispute.go`  
**Entity :** `MerchantWallet.ApplyClawbackToDebt`

Ordre :

1. Prendre sur `balance_cents` (sans forcer sous 0 côté debt model).
2. Le reste → `debt_cents`.
3. Ledger :
   - `clawback` (montant prélevé sur balance)
   - `debt_add` si dette créée / augmentée
4. Escrow → `refunded` ; dispute → `resolved_customer`.
5. **Pas de freeze** sur ce chemin.

Exemple E2E :

```text
Après release : balance=95000, debt=0
Retrait partiel 50000 → balance=45000
Clawback 95000 → balance=0, debt=50000
```

Legacy : `ApplyClawback` (solde négatif + `MaxNegativeBalanceCents`) — préférer `ApplyClawbackToDebt`.

---

## 4. Debt sweep (recovery)

Dès qu’un flux **rend des fonds disponibles** (ou crédite le wallet), on applique :

```text
swept = min(debt_cents, available_after_operation)
debt_cents    -= swept
balance_cents -= swept   // paie la dette avec le cash libéré / crédité
```

| Chemin | Mécanisme | Fichier principal |
|--------|-----------|-------------------|
| Auto-release escrow (sale) | `CreditWithDebtSweep` | scheduler escrow + `wallet_usecase` crédit |
| Dispute `merchant_wins` | crédit + sweep | `resolve_dispute.go` |
| Installment release | `ReleaseHeld` → commission → sweep | `installment_usecase/release_escrow_funds.go` |
| Tontine redeem / release held | `ReleaseHeld` → sweep | `wallet_usecase/release_held_wallet.go` |
| Dépôt manuel | crédit + sweep | `wallet_handler` / credit deposit |

**Ledger :** type `debt_sweep` (`WalletTxDebtSweep`), `amount_cents` négatif (ex. `-50000`), `reference_type` souvent `debt_sweep`.

Wiring installment : `NewReleaseEscrowFundsUsecase(...).WithTxnRepo(walletTxnRepo)` dans `internal/app/app.go`.

---

## 5. Flux résumé

```text
[Pay-in success] → escrow funds_held
       ↓
[Shipping + delivery + délai zone]
       ↓
[Scheduler auto-release] → CreditWithDebtSweep → balance↑ (net de dette)
       ↓
[Option] Withdrawal si debt=0
       ↓
[Dispute customer_wins POST-release]
       → ApplyClawbackToDebt → balance↓ puis debt↑
       → withdrawal BLOQUÉ
       ↓
[Prochain crédit / release]
       → debt_sweep → debt↓ , balance net = crédit − swept
```

Dispute **PRE-release** (`merchant_wins`) : escrow `disputed` → claim release + crédit marchand (avec sweep si dette déjà présente) — **pas** de clawback client.

---

## 6. API & messages UX

| Situation | Comportement API |
|-----------|------------------|
| `POST /api/withdrawals` avec `debt_cents > 0` | HTTP 400, message dette résiduelle à régulariser |
| `GET /api/wallet` | `withdrawal_blocked: true` si dette / gel / disponible ≤ 0 |
| `GET /api/wallet/freeze-status` | `amount_due_cents` priorise `debt_cents` |

Frontend recommandé :

- Afficher `debt_cents` clairement sur le dashboard wallet.
- Désactiver le CTA « Retirer » si `withdrawal_blocked`.
- Expliquer : « Une dette post-litige sera déduite des prochaines ventes. »

---

## 7. Tests E2E (smoke)

Prérequis : API up, env `YENGA_PAY_WEBHOOK_SECRET`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`.

| Script | Scénario | Attendu |
|--------|----------|---------|
| `e2e-dispute-merchant-wins.ps1` | Litige pré-release, `merchant_wins` (± `INJECT_DEBT_CENTS`) | Escrow `released`, crédit net, sweep si dette |
| `e2e-debt-sweep-fraud.ps1` | Inject SQL `debt_cents` puis 2e auto-release | Debt ↓, ledger `debt_sweep`, retrait bloqué si debt > 0 |
| `e2e-clawback-debt-sweep-chain.ps1` | 2 pay-in **réels**, clawback puis sweep | debt créée par métier, puis swept |
| `e2e-clawback-real-payin.ps1` | Clawback seul (canal pay-in réel) | Wallet ↓, escrow refunded |

Exemple :

```powershell
$env:YENGA_PAY_WEBHOOK_SECRET="..."
$env:ADMIN_EMAIL="..."
$env:ADMIN_PASSWORD="..."

Remove-Item Env:INJECT_DEBT_CENTS -ErrorAction SilentlyContinue
.\e2e-dispute-merchant-wins.ps1

$env:INJECT_DEBT_CENTS="50000"
.\e2e-dispute-merchant-wins.ps1

.\e2e-debt-sweep-fraud.ps1
```

---

## 8. Fichiers code de référence

| Zone | Chemin |
|------|--------|
| Entity | `domain/entity/merchant_wallet.go` (`ApplyClawbackToDebt`, `CreditWithDebtSweep`, `ReleaseHeld`) |
| Dispute | `application/usecase/dispute_usecase/resolve_dispute.go` |
| Crédit wallet | `application/usecase/wallet_usecase/` (credit + release_held) |
| Installment | `application/usecase/installment_usecase/release_escrow_funds.go` |
| Retrait | `application/usecase/withdrawal_usecase/create_withdrawal.go` |
| API | `interfaces/handler/wallet_handler/wallet_handler.go` |
| DI | `internal/app/app.go` (`.WithTxnRepo(walletTxnRepo)`) |

---

## 9. Non-objectifs (v1)

- Table séparée `merchant_debt_transactions` (redondant avec `debt_cents` + ledger).
- Freeze automatique au clawback (entity freeze existe pour autres raisons).
- Autoriser un retrait « net de dette » tant que `debt_cents > 0`.

---

## 10. Liens doc associés

- Domaine : [02-domain-model.md](02-domain-model.md)
- API : [03-api-reference.md](03-api-reference.md)
- Tests : [08-testing-guide.md](08-testing-guide.md)
- Glossaire : [09-glossary.md](09-glossary.md)
- Frontend : [10-frontend-guidelines.md](10-frontend-guidelines.md)
- Changelog : [../CHANGELOG.md](../CHANGELOG.md)
```
