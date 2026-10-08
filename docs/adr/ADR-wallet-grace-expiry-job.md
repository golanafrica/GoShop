```markdown
# ADR — Wallet Grace Expiry Job (P2)

**Statut** : Accepté  
**Date** : 2026-10-06  
**Contexte** : GoShop v5.2 — wallet freeze + `debt_cents` / clawback déjà en place ; **aucun** scheduler ne traite la fin de période de grâce  
**Décideurs** : Backend / Ops (Super Admin)

---

## Contexte

Le domaine freeze est déjà riche :

| Couche | Rôle |
|--------|------|
| `merchant_wallets` | État **runtime** : `is_frozen`, `frozen_at`, `frozen_reason`, `frozen_until` |
| `account_freezes` | **Dossier métier** : grâce, montants dus, rappels, `resolved_at` / `resolution` |
| Usecase / API | `FreezeAccount` / `UnfreezeAccount`, freeze-status, gate retrait |
| Repo | `FindGracePeriodExpired`, `FindGracePeriodExpiringSoon` (wallet) + équivalents côté AccountFreeze |

Il **manque** un job périodique piloté par Super Admin pour détecter les grâces expirées, auditer et notifier — **sans** confondre avec debt / clawback / sweep.

---

## Décision

Introduire un **Wallet Grace Expiry Job** (scheduler + API admin) avec la politique **v1** suivante.

### Sources de vérité

1. **`merchant_wallets`** = source de vérité du **gel runtime** (`is_frozen`, `frozen_until`).
2. **`account_freezes`** = **dossier / historique** (grâce, résolution, rappels).  
   Le job **ne refond pas** ce modèle en v1.
3. **Orchestration** = tables dédiées `wallet_freeze_job_runs` + `wallet_freeze_job_actions` (pas de troisième « état gel »).

### Comportement v1 (obligatoire)

4. Job v1 = **détection + audit + notification** uniquement.
5. **Aucune** modification de `debt_cents`.
6. **Aucun** clawback.
7. **Aucun** debt sweep.
8. **Aucun** unfreeze automatique.
9. **Aucun** appel à `AccountFreeze.ResolveEscalated()` (ni autre `Resolve*`) pour « marquer » une expiration — cela pose `resolved_at` et **clôture** le dossier alors que le wallet peut rester gelé.
10. **Pas** de colonne `escalated_at` sur `account_freezes` en P2.1 (prématuré tant qu’il n’y a pas d’escalade métier réelle).

### Activation & sécurité ops

11. **`enabled`** = configuration **métier en DB** (`platform_settings` ou équivalent), défaut **`false`**.
12. **`FREEZE_JOB_HARD_DISABLED`** (ENV) = kill switch d’urgence : si true → **aucune** exécution (scheduled ou manual).
13. Ordre d’évaluation :
    ```text
    HARD_DISABLED ? → STOP
    sinon DB enabled ? → sinon STOP
    sinon advisory lock → run
    ```
14. **Super Admin** : activer / désactiver / configurer la policy.
15. **Admin délégué** (si permission ou rôle accordé) : preview + `run` **uniquement si** `enabled = true`.
16. **Marchand** : uniquement les endpoints freeze existants (status / unfreeze) — **pas** le pilotage du job.

### Exécution & idempotence

17. **Une seule exécution globale** à la fois (PostgreSQL advisory lock nommé ; pas de monolithe transactionnel sur tous les wallets).
18. **Audit** :
    - `wallet_freeze_job_runs` : run global (scheduled | manual, mode live | dry_run, compteurs, déclencheur, horodatages).
    - `wallet_freeze_job_actions` : une ligne par wallet traité (ou candidat en dry-run).
19. **Idempotence** : clé logique **`shop_id` + `frozen_until` + action/policy** (un même shop peut être gelé plusieurs fois dans sa vie).
20. **Batch** : traiter par lots (ex. limit 100) ; plafond de sécurité `max_wallets_per_run` (ex. 500) + alerte si volume anormal.
21. **Dry-run** obligatoire avant activation prod (preview liste + run `mode=dry_run`).

### API (contrat métier, pas « scheduler »)

```text
GET  /api/admin/freeze-job              # status / config
PUT  /api/admin/freeze-job              # enable/policy — Super Admin
POST /api/admin/freeze-job/run          # ?mode=dry_run|live — SA ou délégué si enabled
GET  /api/admin/wallets/grace-expired   # preview candidats
```

### Hors scope P2 v1

- Gel au clawback / dette sans grâce  
- Suspension ou fermeture boutique auto  
- SMS marchand  
- Refonte AccountFreeze / statut `grace_expired` en base  
- Rappels J+1/J+3/J+6 automatisés (domaine déjà prêt ; job séparé ultérieur)

---

## Conséquences

### Positives

- Automatisation contrôlée, **OFF par défaut**, coupures immédiates possibles.  
- Pas de divergence `resolved` vs `is_frozen` via mauvais usage de `ResolveEscalated`.  
- Audit rejouable et reprise après crash (actions partielles).  
- Séparation claire finance (debt) vs conformité ops (grâce).

### Négatives / coûts

- Deux tables d’audit à migrer et maintenir.  
- Advisory lock + batch à tester (multi-instance).  
- RBAC à coller aux middlewares existants (rôles vs permissions fines).

### Suivi

| Phase | Livrable |
|-------|----------|
| P2.1 | Migration settings + runs + actions ; seed `enabled=false` |
| P2.2 | `WalletGraceExpiryScheduler` + wiring cron |
| P2.3 | Handlers admin |
| P2.4 | Tests unitaires + E2E |
| P2.5 | Doc (`12-wallet-debt-sweep` ou `13-wallet-freeze-job`) + CHANGELOG |

---

## Alternatives rejetées

| Alternative | Raison du rejet |
|-------------|-----------------|
| Marquer via `ResolveEscalated()` | Ferme le dossier (`resolved_at`) |
| `escalated_at` dès P2.1 | Pas d’escalade métier v1 ; confusion sémantique |
| ENV seul pour `enabled` | Double vérité / chaos ops |
| Une seule colonne `last_check_at` | Idempotence faible, pas d’historique de run |
| Tout traiter dans une seule transaction | Lock long, risque mémoire / timeout |
| Activer le cron dès le deploy | Risque prod sans preview |

---

## Références code

- `domain/entity/merchant_wallet.go` — `Freeze`, `GracePeriodExpired`, `DefaultGracePeriodDays`
- `domain/entity/account_freeze.go` — dossier, `Resolve*`, rappels
- `domain/repository` / `infrastructure/postgres/wallet` — `FindGracePeriodExpired`
- `application/usecase/wallet_usecase/freeze_account.go`
- Schedulers existants : `application/scheduler/*` (pas de job freeze aujourd’hui)

---

**Dernière mise à jour** : 2026-10-06  
