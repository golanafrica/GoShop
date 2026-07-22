
---

```markdown
# 🤝 Contribuer à GoShop

Merci de votre intérêt pour contribuer à **GoShop** ! Ce document fournit les lignes directrices pour garantir la qualité, la sécurité et la cohérence du codebase.

---

## 📋 Table des matières
1. [Environnement de développement](#1-environnement-de-développement)
2. [Workflow de contribution](#2-workflow-de-contribution)
3. [Architecture & Conventions de code](#3-architecture--conventions-de-code)
4. [Règles de Sécurité (CRITIQUE)](#4-règles-de-sécurité-critique)
5. [Stratégie de tests](#5-stratégie-de-tests)
6. [Convention de commits](#6-convention-de-commits)
7. [Processus de Pull Request](#7-processus-de-pull-request)

---

## 1. Environnement de développement

### Prérequis
- **Go** : 1.23+
- **PostgreSQL** : 16+
- **Redis** : 7+
- **Docker & Docker Compose** (recommandé pour un setup local rapide)
- **golangci-lint** (optionnel mais fortement recommandé)

### Installation locale
```bash
# 1. Cloner le dépôt
git clone https://github.com/golanafrica/GoShop.git
cd GoShop

# 2. Configurer les variables d'environnement
cp .env.example .env
# ⚠️ Éditer .env avec vos valeurs. Assurez-vous que JWT_SECRET fait au moins 32 caractères.

# 3. Lancer les services de base (PostgreSQL, Redis)
docker-compose up -d postgres redis

# 4. Appliquer les migrations
migrate -path migrations -database "postgres://postgres:password@localhost:5432/goshop_db?sslmode=disable" up

# 5. Lancer l'API
go run cmd/api/main.go
```

---

## 2. Workflow de contribution

1. **Fork** le projet sur GitHub.
2. **Créer une branche** à partir de `main` : `git checkout -b feature/nom-de-la-fonctionnalite` ou `fix/nom-du-bug`.
3. **Écrire du code** en respectant les conventions d'architecture et de sécurité ci-dessous.
4. **Écrire des tests** (unitaires, intégration ou E2E selon le cas).
5. **Vérifier** que tout passe : `go test ./... -v` et `golangci-lint run`.
6. **Commiter** avec des messages clairs et conventionnels.
7. **Ouvrir une Pull Request** vers la branche `main`.

---

## 3. Architecture & Conventions de code

GoShop suit les principes de la **Clean Architecture** et du **Domain-Driven Design (DDD)** :

- **Domain** : Entités métier pures, interfaces de repositories, Value Objects. Aucune dépendance externe.
- **Application** : Use cases (logique métier), DTOs, orchestration des repositories.
- **Interfaces** : Handlers HTTP, Middlewares, routeurs (Chi).
- **Infrastructure** : Implémentations concrètes des repositories (PostgreSQL), services externes (YengaPay, Redis, WebSocket).

### Règles de codage
- **Formatage** : Toujours exécuter `gofmt -s -w .` et `go vet ./...` avant de commiter.
- **Logs** : Utiliser exclusivement `zerolog` pour des logs structurés en JSON. Inclure systématiquement le `request_id` et le `component`.
- **Monnaie** : Toujours utiliser `int64` pour les montants (en centimes FCFA). **Jamais de `float`**.
- **Erreurs** : Utiliser `fmt.Errorf("contexte: %w", err)` pour envelopper les erreurs et préserver la stack trace.

---

## 4. Règles de Sécurité (CRITIQUE) 🚨

Toute violation de ces règles entraînera le rejet immédiat de la Pull Request :

1. **Isolation Multi-tenant** : 
   - Toutes les requêtes SQL sur les tables métier (`products`, `orders`, `customers`, etc.) **doivent** inclure `WHERE shop_id = $X`.
   - Le `shop_id` doit **toujours** être extrait du contexte via `tenant.FromContext(ctx)`.
   - Les nouvelles routes métier doivent être protégées par le middleware `middl.RequireShopAccess`.
2. **Pas de fuite de données sensibles** : 
   - **NE JAMAIS** logger le `raw_body` brut des requêtes HTTP (surtout `/login` ou `/register`). Logger uniquement les champs décodés et masqués (ex: `decoded_email`).
   - **NE JAMAIS** commiter de fichiers contenant des secrets (mots de passe, clés API, `.env`).
3. **Injection SQL** : Utiliser **exclusivement** des requêtes paramétrées (`$1, $2`). Pour les tris dynamiques (`ORDER BY`), utiliser une whitelist stricte de colonnes autorisées.
4. **Authentification** : Le `JWT_SECRET` doit faire au moins 32 caractères. Le coût de hachage bcrypt ne doit jamais être inférieur à 10.

---

## 5. Stratégie de tests

Tout nouveau code doit être accompagné de tests.

```bash
# 1. Tests unitaires (rapides, mocks)
go test ./... -v

# 2. Tests d'intégration (nécessite DB locale)
go test -tags=integration ./... -v

# 3. Tests End-to-End (E2E)
go test -tags=e2e ./tests/e2e/... -v

# 4. Tests de charge (k6)
k6 run tests/loadtest/scripts/auth_load.js
```
> 💡 **Nouveau** : Pour tester manuellement les notifications WebSocket, utilisez le script PowerShell dédié : `.\Test-WebSocket-Notification.ps1`

---

## 6. Convention de commits

Nous utilisons la spécification [Conventional Commits](https://www.conventionalcommits.org/) :

- `feat:` Nouvelle fonctionnalité (ex: `feat: add 2FA support`)
- `fix:` Correction de bug (ex: `fix: resolve shop_id leak in order handler`)
- `docs:` Mise à jour de la documentation (ex: `docs: update API reference`)
- `test:` Ajout ou correction de tests (ex: `test: add E2E test for tontine workflow`)
- `chore:` Maintenance, mise à jour de dépendances, CI/CD
- `refactor:` Refactoring de code sans changement de comportement
- `perf:` Amélioration des performances
- `security:` Correction d'une vulnérabilité de sécurité

---

## 7. Processus de Pull Request

Pour qu'une PR soit acceptée, elle doit :
1. **Décrire clairement** le problème résolu ou la fonctionnalité ajoutée.
2. **Lier un ticket** (issue) si applicable (ex: `Fixes #123`).
3. **Inclure des tests** couvrant la nouvelle logique.
4. **Mettre à jour la documentation** (`README.md`, `docs/`, ou commentaires Swagger) si l'API ou le comportement change.
5. **Passer tous les checks CI** (Lint, Tests, Build).

---

## 📚 Ressources utiles
- [Architecture GoShop](01-architecture.md)
- [Modèle de données](02-domain-model.md)
- [Stratégie Multi-tenant](04-multi-tenant.md)
- [Plan de Migration DB](06-migration-plan.md)

Merci de contribuer à rendre GoShop plus robuste, sécurisé et accessible pour l'Afrique ! 