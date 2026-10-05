

## Fichier complet prêt à coller

```markdown
# 🗄️ Plan de Migration de Base de Données (GoShop)

**Version** : v5.1.0  
**Dernière mise à jour** : 2026-10-03  
**Outil** : `golang-migrate/migrate` (ou runner équivalent du projet)  
**SGBD** : PostgreSQL 16  
**État schéma finance** : jusqu’à **`058_merchant_wallet_debt_cents.sql`**

---

## 📋 Table des matières

1. [Vue d'ensemble](#1-vue-densemble)
2. [Convention de nommage](#2-convention-de-nommage)
3. [Workflow de développement (Local)](#3-workflow-de-développement-local)
4. [Règles d'Or (Best Practices)](#4-règles-dor-best-practiques)
5. [Déploiement en Production](#5-déploiement-en-production)
6. [Stratégie de Rollback](#6-stratégie-de-rollback)
7. [Historique des migrations (extrait critique)](#7-historique-des-migrations-extrait-critique)
8. [Wallet / debt — points d’attention](#8-wallet--debt--points-dattention)

---

## 1. Vue d'ensemble

Standards pour faire évoluer le schéma PostgreSQL de GoShop.

Objectifs :
- **Zéro / faible downtime** quand c’est possible
- **Traçabilité** (chaque changement versionné dans Git)
- **Idempotence** (`IF NOT EXISTS`, `DO $$ … $$` pour contraintes)
- **Réversibilité** documentée (script correctif **nouveau**, pas rewrite d’une migration déjà appliquée)

Dossier : `migrations/` à la racine du repo.

---

## 2. Convention de nommage

### Format réel du dépôt (à respecter)

```text
NNN_description_snake_case.sql
```

Exemples présents sur `main` :
- `033_add_user_id_to_customers.sql`
- `045_add_wallet_held_balance.sql`
- `048_wallet_credit_idempotency.sql`
- `058_merchant_wallet_debt_cents.sql`

Variants acceptés historiquement : `019b_…`, `021b_…`, `025b_…`, `026b_…`, `026c_…`.

> **Règle** : numéro **séquentiel unique**. Ne jamais réutiliser un `NNN` déjà mergé, même après rollback.

### Note sur `.up.sql` / `.down.sql`

La doc historique recommandait des paires `golang-migrate` classiques. **Le dépôt utilise majoritairement un seul fichier `.sql` par version.**  
Si vous introduisez des paires `.up` / `.down`, alignez le runner CI et documentez-le ; ne mélangez pas les styles sans raison.

---

## 3. Workflow de développement (Local)

### Étape 1 — Créer le fichier

```bash
# Prochain numéro = max(NNN) dans migrations/ + 1
# Ex. après 058 → 059_add_xxx.sql
```

Ou CLI (si configurée pour ce format) :

```bash
migrate create -ext sql -dir migrations -seq add_xxx
```

### Étape 2 — SQL idempotent

```sql
ALTER TABLE merchant_wallets
  ADD COLUMN IF NOT EXISTS debt_cents BIGINT NOT NULL DEFAULT 0;

-- Contraintes nommées
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

### Étape 3 — Appliquer & tester

```bash
migrate -path migrations -database "postgres://postgres:password@localhost:5432/goshop_db?sslmode=disable" up

go test ./... -v
# + smoke finance si schéma wallet touché
```

### Étape 4 — Commit

Commiter **uniquement** le nouveau fichier (jamais modifier une migration déjà sur `main` / prod).

---

## 4. Règles d'Or (Best Practices)

1. **Idempotence** : `IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`, blocs `DO $$` pour CHECK / UNIQUE.
2. **Index lourds** : préférer `CREATE INDEX CONCURRENTLY` (hors transaction unique si le runner l’exige — adapter).
3. **Pas de DROP destructif en une seule étape** : d’abord arrêter d’écrire la colonne côté app, puis migration de suppression plus tard.
4. **NOT NULL sur table peuplée** : toujours `DEFAULT`.
5. **Pas de data migration massive dans le même fichier** que le DDL critique (scripts Go / batch à part).
6. **Contraintes nommées** explicitement.
7. **Finance** : toute colonne wallet (`held_cents`, `debt_cents`) doit rester cohérente avec le ledger `wallet_transactions` et les usecases (voir [12-wallet-debt-sweep.md](12-wallet-debt-sweep.md)).

---

## 5. Déploiement en Production

1. **Backup** complet avant `up`.
2. Fenêtre de faible trafic si tables chaudes (`orders`, `payments`, `merchant_wallets`, `wallet_transactions`).
3. Exécution :

```bash
migrate -path migrations -database "postgres://USER:PASS@HOST:5432/goshop_db?sslmode=require" up
```

4. Vérifier `schema_migrations` (ou équivalent) + logs app (crédits, withdrawals).

---

## 6. Stratégie de Rollback

1. Échec **pendant** le `.sql` : en général rollback transactionnel si le fichier est une seule transaction.
2. Migration **déjà applied** mais incorrecte : **nouvelle** migration corrective (`059_fix_…`), **jamais** rewrite de `058_…` en prod.
3. `migrate down 1` seulement si un script down fiable existe et a été testé en staging — **dernier recours**.

---

## 7. Historique des migrations (extrait critique)

### v4.5 — fondation doc historique

| Version | Fichier | Description |
| ------- | ------- | ----------- |
| `032` | `add_product_full_text_search` | Index GIN recherche produits |
| `033` | `add_user_id_to_customers` | Lien Customer ↔ User (WS) |
| `034` | `fix_users_role_check` | Rôle `user` à l’inscription |

### Post v4.5 — performance, webhooks, litiges, wallet

| Version | Fichier | Description |
| ------- | ------- | ----------- |
| `035` | `add_missing_performance_indexes` | Indexes perf |
| `036`–`037` | webhook idempotence | Anti double-traitement webhooks |
| `038` | `add_provider_fees` | Frais provider |
| `039`–`040` | disputes | Table litiges + `shop_id` |
| `041` | customer email unique | Contrainte email |
| `042`–`044` | tontine | Intent provider, min participants, commissions |
| `045` | **`add_wallet_held_balance`** | **`held_cents`** sur merchant_wallets |
| `046` | voucher held amount | Hold tontine voucher |
| `047` | **`add_released_status_to_escrow`** | Statut escrow `released` |
| `048` | **`wallet_credit_idempotency`** | Idempotence crédits wallet (anti double-crédit) |
| `049` | `idempotency_keys` | Clés idempotence HTTP / finance |
| `050` | `installment_escrow_refactor` | Escrow / installment |
| `051`–`052` | **delivery_zones** | Zones + `delivery_zone_id` |
| `053` | customer_reliability_scores | Scores fiabilité |
| `054` | tontine group name | Nom groupe |
| `055` | notifications | Table notifications |
| `056` | platform_finance_tables | Tables finance plateforme |
| `057` | **`wallet_tx_clawback`** | Types / support ledger **clawback** |
| `058` | **`merchant_wallet_debt_cents`** | Colonne **`debt_cents`** + contraintes (≥ 0) |

Liste exhaustive : `ls migrations/` sur le clone.

---

## 8. Wallet / debt — points d’attention

Après **058** (et code v5.1) :

| Colonne / objet | Rôle |
|-----------------|------|
| `merchant_wallets.held_cents` | Fonds non disponibles |
| `merchant_wallets.debt_cents` | Dette résiduelle post-clawback (**≥ 0**) |
| `wallet_transactions` | Audit : vente, clawback, debt_add, debt_sweep, payout… |
| Index unique crédit (048) | Empêche double crédit même référence |

**Ne pas** :
- Autoriser `debt_cents < 0` en SQL
- Supprimer les index d’idempotence crédit sans plan de remplacement
- Migrer des soldes « négatifs legacy » vers `debt_cents` sans script de data + validation E2E

Doc métier : [12-wallet-debt-sweep.md](12-wallet-debt-sweep.md)

---

## 📚 Références

- [golang-migrate](https://github.com/golang-migrate/migrate)
- [Wallet debt-sweep](12-wallet-debt-sweep.md)
- [Modèle de données](02-domain-model.md)
- [CHANGELOG](../CHANGELOG.md)
- [Zero Downtime Migrations FAQ](https://github.com/golang-migrate/migrate/blob/master/FAQ.md)

---

**Dernière mise à jour** : 2026-10-03
```

---

