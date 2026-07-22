

```markdown
# 🗄️ Plan de Migration de Base de Données (GoShop)

**Version** : v4.5.0  
**Dernière mise à jour** : 2026-07-21  
**Outil** : `golang-migrate/migrate`  
**SGBD** : PostgreSQL 16

---

## 📋 Table des matières

1. [Vue d'ensemble](#1-vue-densemble)
2. [Convention de nommage](#2-convention-de-nommage)
3. [Workflow de développement (Local)](#3-workflow-de-développement-local)
4. [Règles d'Or (Best Practices)](#4-règles-dor-best-practices)
5. [Déploiement en Production](#5-déploiement-en-production)
6. [Stratégie de Rollback](#6-stratégie-de-rollback)
7. [Historique des migrations récentes (v4.x)](#7-historique-des-migrations-récentes-v4x)

---

## 1. Vue d'ensemble

Ce document définit les standards et le processus pour gérer les évolutions du schéma de la base de données PostgreSQL de GoShop. 

L'objectif est de garantir :
- ✅ **Zéro temps d'arrêt** (Zero-downtime deployments).
- ✅ **Réversibilité** (Possibilité de rollback en cas d'échec).
- ✅ **Idempotence** (Exécuter une migration plusieurs fois ne doit pas causer d'erreur).
- ✅ **Traçabilité** (Chaque changement est versionné dans Git).

---

## 2. Convention de nommage

Nous utilisons le format standard de `golang-migrate` avec des fichiers séparés pour la montée (`up`) et la descente (`down`) en version.

**Format** : `VERSION_NUMÉRIQUE_nom_descriptif.{up|down}.sql`

**Exemples** :
- `033_add_user_id_to_customers.up.sql`
- `033_add_user_id_to_customers.down.sql`
- `034_fix_users_role_check.up.sql`
- `034_fix_users_role_check.down.sql`

> ⚠️ **Règle** : La version numérique doit être séquentielle et unique. Ne jamais réutiliser un numéro de version, même si une migration a été annulée.

---

## 3. Workflow de développement (Local)

### Étape 1 : Créer les fichiers de migration
Utiliser la CLI `migrate` pour générer les fichiers vides :

```bash
# Installer la CLI (si ce n'est pas déjà fait)
# go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Créer une nouvelle migration (ex: ajout d'un index)
migrate create -ext sql -dir migrations -seq add_index_on_customer_email
```

### Étape 2 : Rédiger le SQL
Remplir les fichiers `.up.sql` (application) et `.down.sql` (annulation).

**Exemple (`035_add_index.up.sql`)** :
```sql
-- Utilisation de CONCURRENTLY pour éviter de verrouiller la table en production
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_customers_email_lower 
ON customers (LOWER(email));
```

**Exemple (`035_add_index.down.sql`)** :
```sql
DROP INDEX IF EXISTS idx_customers_email_lower;
```

### Étape 3 : Tester localement
Appliquer la migration sur ta base de développement (`goshop_db`) :

```bash
migrate -path migrations -database "postgres://postgres:password@localhost:5432/goshop_db?sslmode=disable" up
```

Vérifier que les tests passent toujours :
```bash
go test ./... -v
```

### Étape 4 : Commit et Push
Une fois validé, commiter les deux fichiers (`.up` et `.down`) dans Git.

---

## 4. Règles d'Or (Best Practices)

Pour garantir la stabilité d'une application Fintech, respectez scrupuleusement ces règles :

1. **Idempotence** : Utilisez toujours `IF NOT EXISTS` pour les `CREATE TABLE`, `CREATE INDEX`, ou `ALTER TABLE ADD COLUMN`.
2. **Pas de verrouillage de table (Locking)** : Pour les grosses tables, utilisez toujours `CREATE INDEX CONCURRENTLY`.
3. **Éviter les opérations destructives** : 
   - Ne jamais faire de `DROP COLUMN` ou `DROP TABLE` dans une première migration. 
   - Procéder en 2 étapes : 1) L'application ignore la colonne. 2) Une migration ultérieure la supprime après vérification.
4. **Valeurs par défaut** : Lors de l'ajout d'une colonne `NOT NULL` sur une table existante, fournir toujours une `DEFAULT VALUE` pour éviter les erreurs sur les lignes existantes.
5. **Pas de logique métier complexe en SQL** : Les migrations de données (Data Migrations) lourdes doivent être faites via des scripts Go externes ou des batchs, pas dans une transaction SQL unique qui pourrait timeout.
6. **Contraintes de vérification (CHECK)** : Toujours nommer explicitement les contraintes (ex: `CONSTRAINT users_role_check CHECK (...)`) pour faciliter leur modification ou suppression future.

---

## 5. Déploiement en Production

Le déploiement des migrations en production (`goshop_db` de prod) doit suivre ce processus strict :

1. **Backup** : Effectuer un snapshot/backup complet de la base de données avant toute opération.
2. **Fenêtre de maintenance** : Si la migration touche à des tables critiques (ex: `orders`, `payments`), la lancer pendant une période de faible trafic.
3. **Exécution via CI/CD ou Job Kubernetes** : 
   ```bash
   migrate -path migrations -database "postgres://USER:PASS@HOST:5432/goshop_db?sslmode=require" up
   ```
4. **Vérification** : Vérifier les logs pour s'assurer que le statut est `SUCCESS` et que la version de la table `schema_migrations` a été incrémentée.

---

## 6. Stratégie de Rollback

Si une migration `.up.sql` échoue en production :

1. **Ne pas paniquer** : `golang-migrate` annule automatiquement la transaction si une erreur survient *pendant* l'exécution du fichier `.up.sql`.
2. **Si la migration est marquée comme "applied" mais est corrompue** :
   - Corriger le fichier `.up.sql` localement.
   - Créer une **nouvelle** migration corrective (ex: `036_fix_previous_migration.sql`). Ne jamais modifier une migration déjà poussée en production.
3. **Rollback manuel (Dernier recours)** :
   ```bash
   migrate -path migrations -database "postgres://..." down 1
   ```
   > ⚠️ **Attention** : La commande `down` n'est fiable que si le fichier `.down.sql` a été rigoureusement testé en local et qu'aucune donnée n'a été corrompue entre-temps.

---

## 7. Historique des migrations récentes (v4.x)

| Version | Fichier | Description | Impact |
|---------|---------|-------------|--------|
| `032` | `add_product_full_text_search` | Ajout d'index GIN pour la recherche plein texte sur les produits. | Performance |
| `033` | `add_user_id_to_customers` | Ajout de la colonne `user_id` (VARCHAR) pour lier l'entité Customer à l'entité User authentifiée. | Architecture / Sécurité |
| `034` | `fix_users_role_check` | Correction de la contrainte `CHECK` sur la table `users` pour autoriser explicitement le rôle `'user'` lors des inscriptions publiques. | Sécurité / Bugfix |

---

## 📚 Références

- [Documentation officielle golang-migrate](https://github.com/golang-migrate/migrate)
- [PostgreSQL: Safe Operations For High Volume PostgreSQL](https://www.braintreepayments.com/blog/safe-operations-for-high-volume-postgresql/)
- [Zero Downtime Migrations](https://github.com/golang-migrate/migrate/blob/master/FAQ.md#how-do-i-migrate-a-database-with-zero-downtime)
```

---

