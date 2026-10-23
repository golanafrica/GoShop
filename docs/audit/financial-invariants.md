# 🛡️ Invariants Financiers de GoShop

**Version** : 2.0.0 (Audit Closure)  
**Dernière mise à jour** : Octobre 2026  
**Statut** : ✅ Validé par les tests E2E et l'audit de sécurité v5.2.0  

## 🎯 Objectif

Ce document définit les **règles non négociables (invariants)** qui régissent le système comptable et financier de GoShop. Toute modification du code (nouvelle fonctionnalité, refactoring, correction de bug) **DOIT** respecter ces invariants. Si une Pull Request viole l'un de ces principes, elle doit être rejetée.

---

## 🏛️ 1. Principes Comptables Fondamentaux

1. **Source de vérité unique** : Le solde d'un portefeuille (`merchant_wallets`) est une valeur matérialisée pour la performance, mais la **vérité absolue** réside dans la somme des écritures du grand livre (`wallet_transactions`).
2. **Immutabilité du Ledger** : Une ligne dans `wallet_transactions` ayant le statut `completed` ne doit **JAMAIS** être modifiée ou supprimée. Les corrections se font uniquement par de nouvelles écritures compensatoires (ex: `clawback`, `withdrawal_reversal`).
3. **Atomicité** : Toute opération financière impactant l'escrow, le wallet et le ledger doit être encapsulée dans une **seule transaction SQL** (`BEGIN` ... `COMMIT`). En cas d'erreur, un `ROLLBACK` total est obligatoire.
4. **Isolation Multi-tenant** : Aucune requête financière ne peut être exécutée sans un contexte de tenant (`shop_id` ou `user_id`) validé côté serveur. Les identifiants fournis dans le corps de la requête (JSON) ne doivent jamais outrepasser le contexte d'authentification.

---

## 💳 2. Portefeuilles (Wallets) & Dettes

5. **Formule du disponible** : Le solde disponible d'un marchand est strictement défini par :  
   `Available = BalanceCents - HeldCents - DebtCents`
6. **Interdiction de retrait sous dette** : Une demande de retrait (`POST /api/withdrawals`) **DOIT** être rejetée immédiatement si `DebtCents > 0`, indépendamment du solde `BalanceCents`.
7. **Debt Sweep Prioritaire** : Lors de tout crédit (`CreditWithDebtSweep`), le montant est d'abord appliqué à la réduction de `DebtCents`. Seul le reliquat est ajouté à `BalanceCents`. Ce comportement s'applique uniformément aux ventes, aux libérations d'escrow, aux gains de litiges et aux redistributions de tontine.
8. **Pas de solde négatif caché** : Si un débit (ex: commission, clawback) rend le solde inférieur à `MaxNegativeBalanceCents`, l'opération échoue ou déclenche un gel (`Freeze`), mais ne doit jamais créer de dette silencieuse hors du champ `DebtCents`.

---

## 🌐 3. Paiements & Webhooks

9. **Idempotence stricte des Webhooks (FOR UPDATE SKIP LOCKED)** : Un événement webhook traité avec le même `provider_ref` ne doit **JAMAIS** déclencher un second crédit ou une seconde mise à jour de statut. Le système utilise un claim atomique (`FOR UPDATE SKIP LOCKED`) sur l'inbox pour garantir qu'une seule instance multi-thread peut traiter l'événement.
10. **Confirmation post-succès uniquement** : Le statut `processed = true` d'un webhook n'est enregistré en base **QU'APRÈS** la réussite de toutes les écritures métier (création/mise à jour du paiement, crédit escrow, écriture ledger).
11. **Gestion des Timeouts et Erreurs Ambiguës** : Un timeout ou une erreur 5xx du prestataire **N'EST PAS** un échec définitif. Le système utilise une logique `isDefinitiveRejection` stricte. Seuls les rejets métier explicites (ex: "invalid", "rejected") déclenchent un reverse. Les erreurs ambiguës passent le retrait en statut `reconciliation_required` pour vérification manuelle, empêchant tout reverse intempestif.
12. **ONE_STEP Resilience** : Si le prestataire confirme le succès mais qu'une étape interne échoue (ex: création d'escrow), le paiement est **toujours** marqué `success` localement avec un flag `escrow_pending`. Il ne doit **JAMAIS** être marqué `failed`, afin d'éviter toute incitation au double prélèvement.

---

## 🔒 4. Escrow & Libération de Fonds

13. **Libération unique et idempotente** : Un compte séquestre (`escrow_account`) ne peut être libéré (`released`) qu'**une seule fois**. Le système crée systématiquement un marqueur idempotent dans le ledger (`reference_type = 'escrow_auto_release'`), **même lorsque le debt sweep absorbe 100 % du montant** (`netToBalance == 0`). Cela empêche toute boucle de double libération en cas de redémarrage du scheduler.
14. **Blocage par litige** : Si un litige (`dispute`) est ouvert sur une commande ou un voucher, le statut de l'escrow passe à `disputed`. **Aucune** libération automatique ou manuelle ne peut être effectuée tant que le litige n'est pas résolu.
15. **Atomicité de la libération** : La libération d'escrow doit atomiquement :
    - Changer le statut de l'escrow en `released`.
    - Créditer le wallet du marchand (avec Debt Sweep).
    - Créer les lignes de ledger (`sale_credit`, `debt_sweep` si applicable).
    - Créditer la commission plateforme (si configuré).

---

## 🔄 5. Retraits (Withdrawals) & Compensation

16. **Débit unique** : Le montant d'un retrait ne peut être débité du wallet qu'**une seule fois** pour un `withdrawal_id` donné.
17. **Machine à états robuste** : En cas d'échec confirmé du prestataire *après* le débit du wallet, le statut doit passer à `compensation_pending`. En cas d'incertitude, il passe à `reconciliation_required` via une transition atomique (`TransitionToReconciliationRequired`).
18. **Reprise automatique (Retry)** : Un retrait en `compensation_pending` doit être repris automatiquement par le `CompensationPendingScheduler`. Si le reverse réussit, le statut passe à `failed` et une ligne `withdrawal_reversal` est créée dans le ledger.
19. **Alerte après échecs max** : Si le nombre de tentatives de compensation atteint `max_attempts`, le système doit **ALERTER** un administrateur et ne doit **JAMAIS** marquer le retrait comme réussi de manière fictive.

---

## ⚖️ 6. Litiges (Disputes) & Clawback

20. **Résolution unique** : Un litige ne peut être résolu (`customer_wins` ou `merchant_wins`) qu'**une seule fois**.
21. **Clawback en deux temps** : En cas de `customer_wins` sur un escrow déjà libéré :
    - Le système tente d'abord de débiter le `BalanceCents` disponible.
    - Si le solde est insuffisant, le montant manquant est ajouté à `DebtCents` via une écriture `debt_add`.
    - Le portefeuille n'est **PAS** gelé automatiquement par le clawback (le gel est géré par le Freeze Job si la dette persiste au-delà de la période de grâce).

---

## 🤝 7. Tontines, Remboursements & Paiements en Tranches

22. **Unicité des Vouchers** : Un code de voucher de tontine (`voucher_code`) ne peut être utilisé (redeemed) qu'**une seule fois**.
23. **Cohérence des tranches** : La somme des montants des tranches payées (`installment_payments`) ne doit **JAMAIS** dépasser le montant total dû de la commande.
24. **Commission Tontine** : La commission plateforme sur une tontine est prélevée **une seule fois** sur le montant total du cycle (modèle net), et non à chaque cotisation individuelle.
25. **Remboursements Partiels Atomiques** : La mise à jour du cumul remboursé (`refunded_amount_cents`) doit se faire via une requête SQL atomique avec contrainte (`WHERE (amount_cents - refunded_amount_cents) >= $2`). Il est mathématiquement impossible de rembourser plus que le montant initial, même sous forte concurrence.

---

## 🧪 8. Vérification et Preuves (Tests E2E)

Ces invariants ne sont pas théoriques. Ils sont validés en continu par la suite de tests E2E suivante :

| Invariant testé | Script de preuve E2E |
| :--- | :--- |
| Idempotence Webhooks & Atomicité | `e2e-yengapay-full-flow.ps1` |
| Retrait échoué -> Compensation -> Reverse réussi | `e2e-withdrawal-compensation-pending.ps1` |
| Job de reprise de compensation (Retry) | `e2e-compensation-pending-job.ps1` |
| Clawback post-release + Debt Sweep | `e2e-clawback-debt-sweep-chain.ps1` |
| Blocage retrait si dette > 0 | `e2e-debt-sweep-fraud.ps1` |
| Freeze Job : Pagination, Idempotence, Notifications | `e2e-wallet-grace-expiry-job.ps1` |
| Isolation Multi-tenant (IDOR) | Tests unitaires `TestTenantResolver_AccessDenied` |
| Remboursements partiels atomiques | `TestRefundPaymentUsecase` (Integration) |

---

## 🚨 9. Protocole en cas de Violation d'Invariant

Si un écart est détecté en production (via les alertes Prometheus ou la réconciliation) :
1. **Ne pas modifier la base de données manuellement** sans traçabilité.
2. Ouvrir un ticket d'incident critique (P0).
3. Utiliser les outils de réconciliation administrateur pour identifier l'écriture ledger manquante ou en double.
4. Appliquer une correction via une transaction compensatoire explicite (ex: script de réconciliation validé par un Super Admin), qui sera elle-même enregistrée dans le ledger avec `reference_type = 'manual_reconciliation'`.

---

*Ce document fait partie intégrante des critères d'acceptation de toute fonctionnalité financière dans GoShop.*