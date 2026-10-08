Voici une **doc courte** prête à coller, par ex. en `docs/13-wallet-grace-expiry-job.md` (ou à côté de ton ADR dans `docs/adr/`).

```markdown
# Wallet Grace Expiry Job (P2)

**Version** : v5.2  
**Statut** : ✅ E2E vert (`e2e-wallet-grace-expiry-job.ps1`)  
**ADR** : [docs/adr/](adr/) — *Wallet Grace Expiry Job* (politique v1 : détection + audit + alerte, **sans** mutation financière)

---

## Objectif

Détecter les wallets **gelés** dont la période de grâce (`frozen_until`) est **dépassée**, puis :

- enregistrer une action d’audit `grace_expired_detected` (idempotente) ;
- tracer chaque exécution dans `wallet_freeze_job_runs` ;
- **ne pas** modifier `balance_cents` / `debt_cents` / `held_cents` ;
- **ne pas** déclencher clawback, debt_sweep, ni dégel automatique.

---

## Activation (Super Admin)

### 1. Kill switch ENV (prioritaire)

| Variable | Effet |
|----------|--------|
| `FREEZE_JOB_HARD_DISABLED=true` | Job **coupé** partout (cron + run manuel). Aucune exécution. |
| absent ou `false` | Le job peut tourner si la config DB l’autorise. |

À mettre dans `.env` / secrets de déploiement. En cas d’incident : activer ce flag et redémarrer l’API.

### 2. Config métier (DB — source de vérité)

Table `platform_settings`, clé `wallet.freeze_job.enabled` :

```sql
-- Lire
SELECT key, value FROM platform_settings WHERE key = 'wallet.freeze_job.enabled';

-- Activer (Super Admin / SQL contrôlé)
UPDATE platform_settings
SET value = 'true'::jsonb, updated_at = NOW()
WHERE key = 'wallet.freeze_job.enabled';

-- Désactiver
UPDATE platform_settings
SET value = 'false'::jsonb, updated_at = NOW()
WHERE key = 'wallet.freeze_job.enabled';
```

Seed migration **059** : `enabled = false` par défaut.

Autres clés utiles (si présentes) :

| Clé | Rôle |
|-----|------|
| `wallet.freeze_job.policy` | v1 : `alert_only` |
| `wallet.freeze_job.max_per_run` | plafond de sécurité par run |
| `wallet.freeze_job.batch_size` | taille de lot |

### 3. API Admin (si exposée)

Préfixe typique (Bearer Super Admin) :

```http
GET  /api/admin/wallets/grace-expired
POST /api/admin/freeze-job/run?mode=dry_run
POST /api/admin/freeze-job/run?mode=live
```

- **dry_run** : scan + run audit, **pas** d’insertion d’actions métier.
- **live** : actions `grace_expired_detected` (ON CONFLICT / index partiel → idempotent).

Un marchand reçoit **403** sur ces routes.

---

## Tables (migration 059)

| Table | Rôle |
|-------|------|
| `platform_settings` | Config clé / JSONB |
| `wallet_freeze_job_runs` | Historique scheduled / manual, dry_run / live |
| `wallet_freeze_job_actions` | Actions par wallet ; unique partiel sur `(shop_id, frozen_until, action)` pour `grace_expired_detected` |

État **runtime** du gel : toujours `merchant_wallets` (`is_frozen`, `frozen_until`).  
Historique métier freeze : `account_freezes` (non modifié par ce job en v1).

---

## Multi-instance

Le scheduler prend un **advisory lock** PostgreSQL sur une connexion dédiée pour qu’un seul process exécute le job à la fois.

---

## E2E

```powershell
$env:ADMIN_EMAIL="..."
$env:ADMIN_PASSWORD="..."
.\e2e-wallet-grace-expiry-job.ps1
```

Auth publique API : **`POST /login`** (pas `/api/auth/login`).

Couvre : preview, dry-run, live, idempotence, non-mutation finance, RBAC.

---

## Hors scope v1

- Clawback / `debt_cents` / debt sweep  
- Appel à `ResolveEscalated()` comme simple marqueur  
- Dégel automatique  
- Modification de `merchant_wallets` hors lecture de détection  

---

## Références

- ADR P2 Freeze Job — `docs/adr/`  
- Wallet / dette — [12-wallet-debt-sweep.md](12-wallet-debt-sweep.md)  
- Migrations — [06-migration-plan.md](06-migration-plan.md) (`059_add_wallet_grace_expiry_job_tables`)  

**Dernière mise à jour** : 2026-10-07
```

---

### Commit (après sauvegarde du fichier)

```powershell
git add docs/13-wallet-grace-expiry-job.md
# + le reste P2 si pas encore commité :
git add migrations/059_add_wallet_grace_expiry_job_tables.sql
git add application/scheduler/wallet_grace_expiry_scheduler.go
git add application/usecase/freeze_job_usecase/
git add domain/repository/platform_settings_repository.go
git add infrastructure/postgres/platform_settings/
git add infrastructure/scheduler/cron_scheduler.go
git add interfaces/handler/freeze_job_handler/
git add internal/app/app.go
git add docs/adr/
git add e2e-wallet-grace-expiry-job.ps1

git status

git commit -m @"
feat(wallet): grace expiry job (alert/audit) + admin API + docs + E2E

- Migration 059: platform_settings, job_runs, job_actions
- Scheduler: advisory lock, dry-run/live, no finance mutation
- Config: FREEZE_JOB_HARD_DISABLED + wallet.freeze_job.enabled (DB)
- Docs: 13-wallet-grace-expiry-job.md + ADR
- E2E: preview, idempotence, RBAC, finance safety
"@

git push
```

Si tu as déjà commité une partie du code, ajoute seulement le `.md` + l’E2E restants.