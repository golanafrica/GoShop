
# 🗄️ Plan de Migration de Base de Données (GoShop)

**Version** : v5.2.0  
**Dernière mise à jour** : 2026-10-06  
**SGBD** : PostgreSQL 16  
**État schéma finance** : jusqu’à **`058_merchant_wallet_debt_cents.sql`**

**Runner réel du projet** :
- Script : `tests/loadtest/scripts/migrate.sh`
- Table de suivi : `schema_migrations (version text PRIMARY KEY, applied_at timestamptz)`
- Docker : service one-shot `migrate` (prod/dev) et `migrate-test` (tests)

> Ce n’est **pas** `golang-migrate` en production locale. Les fichiers sont des `.sql` monofichier numérotés, appliqués dans l’ordre lexicographique.

---

## 📋 Table des matières

1. [Vue d'ensemble](#1-vue-densemble)
2. [Convention de nommage](#2-convention-de-nommage)
3. [Comment les migrations sont appliquées](#3-comment-les-migrations-sont-appliquées)
4. [Workflow de développement (Local)](#4-workflow-de-développement-local)
5. [Règles d'Or (Best Practices)](#5-règles-dor-best-practices)
6. [Docker Compose](#6-docker-compose)
7. [Déploiement / réinstall Docker](#7-déploiement--réinstall-docker)
8. [Stratégie de Rollback](#8-stratégie-de-rollback)
9. [Historique des migrations (extrait critique)](#9-historique-des-migrations-extrait-critique)
10. [Wallet / debt — points d’attention](#10-wallet--debt--points-dattention)

---

## 1. Vue d'ensemble

Objectifs :
- **Automatiser** l’application des `.sql` à chaque `docker compose up` (plus de `ALTER` manuel)
- **Traçabilité** via `schema_migrations` + Git
- **Idempotence** SQL (`IF NOT EXISTS`, blocs `DO $$ … $$`)
- **Réversibilité** par **nouvelle** migration corrective (jamais rewrite d’un fichier déjà appliqué)

Dossier : `migrations/` à la racine du repo.

**Important** : le mount historique  
`./migrations:/docker-entrypoint-initdb.d`  
ne s’exécute **qu’à la création du volume**. Il est **remplacé** par le service `migrate` (voir §6).

---

## 2. Convention de nommage

### Format du dépôt

```text
NNN_description_snake_case.sql
```

Exemples :
- `033_add_user_id_to_customers.sql`
- `045_add_wallet_held_balance.sql`
- `048_wallet_credit_idempotency.sql`
- `054_add_name_to_tontine_groups.sql`
- `058_merchant_wallet_debt_cents.sql`

Variants historiques : `019b_…`, `021b_…`, `025b_…`, `026b_…`, `026c_…`.

**Règles** :
1. Numéro **séquentiel unique** — ne jamais réutiliser un `NNN` déjà mergé.
2. Un fichier = une version = une ligne dans `schema_migrations.version` (nom **sans** `.sql`).
3. Pas de paires `.up.sql` / `.down.sql` pour l’instant (le runner shell n’en a pas besoin).

---

## 3. Comment les migrations sont appliquées

### Script `tests/loadtest/scripts/migrate.sh`

1. Attend Postgres (`DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`).
2. `CREATE TABLE IF NOT EXISTS schema_migrations (...)`.
3. Pour chaque `migrations/*.sql` trié :
   - si `version` déjà en table → `SKIP`
   - sinon → `psql -f` puis `INSERT INTO schema_migrations`.

Variables d’environnement :

| Variable         | Défaut (dev)   |
|------------------|----------------|
| `DB_HOST`        | `db`           |
| `DB_PORT`        | `5432`         |
| `DB_USER`        | `postgres`     |
| `DB_PASSWORD`    | `root`         |
| `DB_NAME`        | `goshop_db`    |
| `MIGRATIONS_DIR` | `/migrations`  |

### Vérifier l’état

```bash
docker compose exec -T db psql -U postgres -d goshop_db -c \
  "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 10;"
```

### Logs du runner

```bash
docker compose logs migrate
# ou relancer à la demande :
docker compose run --rm migrate
```

---

## 4. Workflow de développement (Local)

### Étape 1 — Créer le fichier

```text
# Prochain numéro = max(NNN) dans migrations/ + 1
# Après 058 → 059_add_xxx.sql
```

### Étape 2 — SQL idempotent

```sql
ALTER TABLE merchant_wallets
  ADD COLUMN IF NOT EXISTS debt_cents BIGINT NOT NULL DEFAULT 0;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'merchant_wallets_debt_cents_non_negative'
  ) THEN
    ALTER TABLE merchant_wallets
      ADD CONSTRAINT merchant_wallets_debt_cents_non_negative CHECK (debt_cents >= 0);
  END IF;
END $$;
```

### Étape 3 — Appliquer

```bash
# Stack principal : le service migrate s’exécute au up
docker compose up -d

# Forcer uniquement les migrations
docker compose run --rm migrate
```

### Étape 4 — Tests

```bash
go test ./... -v
# + E2E finance / tontine si schéma wallet ou tontine touché
```

### Étape 5 — Commit

**Uniquement** le nouveau fichier SQL (+ doc si besoin).  
**Ne jamais** modifier une migration déjà présente sur `main` / déjà en `schema_migrations`.

---

## 5. Règles d'Or (Best Practices)

1. **Idempotence** : `IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`, `DO $$` pour CHECK / UNIQUE.
2. **Index lourds** : `CREATE INDEX CONCURRENTLY` si possible (attention : hors transaction unique).
3. **Pas de DROP destructif en une étape** : d’abord arrêter d’écrire côté app, suppression plus tard.
4. **NOT NULL sur table peuplée** : toujours un `DEFAULT`.
5. **Data migration massive** : hors du DDL critique (script / batch séparé).
6. **Contraintes nommées** explicitement.
7. **Finance** : `held_cents` / `debt_cents` cohérents avec `wallet_transactions` et les usecases ([12-wallet-debt-sweep.md](12-wallet-debt-sweep.md)).
8. **Fichier sous Windows** : le script shell doit être en **LF** (pas CRLF), sinon Alpine peut échouer.

```powershell
(Get-Content .\tests\loadtest\scripts\migrate.sh -Raw) -replace "`r`n","`n" |
  Set-Content .\tests\loadtest\scripts\migrate.sh -NoNewline -Encoding utf8
```

---

## 6. Docker Compose

### Dev / local (`docker-compose.yml`)

| Service    | Rôle                                      |
|------------|-------------------------------------------|
| `db`       | PostgreSQL 16, volume `postgres_`         |
| `migrate`  | One-shot : applique les `.sql` manquants  |
| `goshop`   | API ; `depends_on: migrate` completed     |

Volumes migrate :
- `./migrations:/migrations:ro`
- `./tests/loadtest/scripts/migrate.sh:/migrate.sh:ro`

**Pas** de `./migrations:/docker-entrypoint-initdb.d` (évite le double chemin et le piège « volume déjà init »).

### Tests (`docker-compose.test.yml`)

Projet isolé : `name: goshop-test`

| Service         | Port hôte | Notes                          |
|-----------------|-----------|--------------------------------|
| `db-test`       | 5434      | DB `goshop_test`               |
| `migrate-test`  | —         | Même script, DB test           |
| `redis-test`    | 6381      | Évite conflit avec Redis 6380  |

```bash
docker compose -f docker-compose.test.yml up -d
docker compose -f docker-compose.test.yml logs migrate-test
```

En cas de conflit de noms de conteneurs :

```bash
docker rm -f goshop-db-test goshop-redis-test goshop-migrate-test
docker compose -f docker-compose.test.yml down -v --remove-orphans
docker compose -f docker-compose.test.yml up -d
```

---

## 7. Déploiement / réinstall Docker

### Volume **existant** (déjà seedé)

Les versions déjà dans `schema_migrations` sont **SKIP**.  
Nouveau fichier `059_….sql` → appliqué au prochain `up` / `run migrate`.

### Volume **vide** (réinstall Docker, `down -v`, machine neuve)

1. `docker compose up -d`
2. `migrate` applique **001 → N** dans l’ordre
3. `goshop` démarre après `service_completed_successfully`

**Plus besoin d’`ALTER` manuel** (ex. `tontine_groups.name`).

### Première bascule depuis l’ancien initdb

Sur un volume déjà à jour **sans** table de suivi :

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
  version    text PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);
-- Puis INSERT de chaque BaseName des fichiers déjà « conceptuellement » appliqués
```

(ou script PowerShell qui boucle sur `migrations\*.sql` avec `ON CONFLICT DO NOTHING`).

### Production

1. Backup avant `up`.
2. Même runner (ou équivalent CI) pointant vers la DB prod.
3. Vérifier `schema_migrations` + health API + smoke finance.

---

## 8. Stratégie de Rollback

1. **Échec pendant** un `.sql` : `ON_ERROR_STOP=1` → le fichier n’est **pas** inséré dans `schema_migrations` (si l’INSERT est après le `-f` réussi). Corriger le SQL, relancer.
2. Migration **déjà applied** mais incorrecte : **nouvelle** migration `059_fix_…` — **jamais** rewrite de `058_…` en prod.
3. Pas de `down` automatisé dans le runner actuel. Rollback = migration corrective ou restore backup.

---

## 9. Historique des migrations (extrait critique)

### v4.5 — fondation

| Version | Fichier                        | Description                  |
| ------- | ------------------------------ | ---------------------------- |
| `032`   | `add_product_full_text_search` | Index GIN recherche produits |
| `033`   | `add_user_id_to_customers`     | Lien Customer ↔ User         |
| `034`   | `fix_users_role_check`         | Rôle `user` à l’inscription  |

### Post v4.5 — perf, webhooks, litiges, wallet, zones

| Version | Fichier                             | Description                                      |
| ------- | ----------------------------------- | ------------------------------------------------ |
| `035`   | `add_missing_performance_indexes`   | Indexes perf                                     |
| `036`–`037` | webhook idempotence             | Anti double-traitement webhooks                  |
| `038`   | `add_provider_fees`                 | Frais provider                                   |
| `039`–`040` | disputes                        | Litiges + `shop_id`                              |
| `045`   | **`add_wallet_held_balance`**       | **`held_cents`**                                 |
| `047`   | **`add_released_status_to_escrow`** | Escrow `released`                                |
| `048`   | **`wallet_credit_idempotency`**     | Idempotence crédits wallet                       |
| `050`   | `installment_escrow_refactor`       | Escrow / installment                             |
| `051`–`052` | delivery_zones                  | Zones + `delivery_zone_id`                       |
| `054`   | **`add_name_to_tontine_groups`**    | Colonne **`name`** sur `tontine_groups`          |
| `055`   | notifications                       | Notifications                                    |
| `056`   | platform_finance_tables             | Finance plateforme                               |
| `057`   | **`wallet_tx_clawback`**            | Ledger clawback                                  |
| `058`   | **`merchant_wallet_debt_cents`**    | **`debt_cents`** + CHECK ≥ 0                     |

Liste exhaustive : contenu de `migrations/` sur le clone.

---

## 10. Wallet / debt — points d’attention

Après **058** (code v5.1+) :

| Colonne / objet               | Rôle                                                   |
| ----------------------------- | ------------------------------------------------------ |
| `merchant_wallets.held_cents` | Fonds non disponibles                                  |
| `merchant_wallets.debt_cents` | Dette résiduelle post-clawback (**≥ 0**)               |
| `wallet_transactions`         | Audit : vente, clawback, debt_add, debt_sweep, payout… |
| Index unique crédit (048)     | Empêche double crédit même référence                   |

**Ne pas** :
- Autoriser `debt_cents < 0` en SQL
- Supprimer les index d’idempotence crédit sans plan de remplacement
- Migrer des soldes « négatifs legacy » vers `debt_cents` sans script data + E2E

Doc métier : [12-wallet-debt-sweep.md](12-wallet-debt-sweep.md)

---

## 📚 Références

- Script : `tests/loadtest/scripts/migrate.sh`
- Compose : `docker-compose.yml` (`migrate`), `docker-compose.test.yml` (`migrate-test`)
- [Wallet debt-sweep](12-wallet-debt-sweep.md)
- [Modèle de données](02-domain-model.md)
- [CHANGELOG](../CHANGELOG.md)

---

**Dernière mise à jour** : 2026-10-06  
**Statut runner** : ✅ validé stack principal + `migrate-test` (APPLY 001… sur volume test vide)
```

---

**Ce qui a changé vs l’ancienne doc**

| Avant | Maintenant |
|--------|------------|
| `golang-migrate` comme outil principal | Script `migrate.sh` + table `schema_migrations` |
| initdb.d / `migrate up` CLI | Service Compose one-shot |
| Pas de chemin test | `docker-compose.test.yml` documenté |
| Date 2026-10-03 | 2026-10-06 + validation runner |

