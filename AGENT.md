# 📋 AUDIT UNIFIÉ GoShop — Plan d'Action Stratégique

**Date :** 13 août 2026
**Projet :** GoShop v4.5.0 (golanafrica/GoShop)
**Type :** Plateforme e-commerce SaaS multi-tenant (fintech)
**Sources :** Audit Qwen + Audit Claude (synthèse croisée)

---

## 🎯 Contexte et Objectifs

GoShop est une plateforme fintech mature manipulant de l'argent réel (Mobile Money, tontines, crédit BCEAO, escrow). Deux audits indépendants ont été menés pour identifier les failles de sécurité, d'intégrité et de qualité. Ce document fusionne leurs conclusions et définit un **plan d'action priorisé** pour amener GoShop au niveau **fintech tier-1** (conformité bancaire, sécurité renforcée, observabilité complète).

**Objectifs du plan :**
- Sécuriser les flux financiers (anti-double-dépense, idempotency)
- Garantir l'intégrité des données KYC et multi-tenant
- Atteindre une couverture de tests ≥ 80% sur les modules critiques
- Implémenter l'observabilité distribuée (tracing, métriques)
- Préparer la conformité réglementaire (APDP, BCEAO)

---

## 📊 Synthèse des Audits Croisés

### ✅ Points Forts Confirmés (À Préserver)

| Domaine | Pratique Validée | Justification |
|---------|------------------|---------------|
| **Authentification** | JWT HS256 figé, tokens courts (15min/7j), révocation sessions temps réel | Protection contre replay et vol de tokens |
| **Hashing** | Bcrypt adaptatif (mais voir contradiction §3.1) | Standard industrie pour mots de passe |
| **Paiements** | Webhooks HMAC temps constant (`hmac.Equal`), validation stricte | Anti-falsification webhooks providers |
| **Wallet** | Verrous pessimistes `FOR UPDATE`, séparation funds held/available | Anti-double-crédit, intégrité financière |
| **Chiffrement** | AES-256-GCM avec nonce aléatoire, clés via env vars | Protection données sensibles au repos |
| **Multi-tenant** | `RequireShopAccess` vérifie propriétaire/collaborateur | Isolation réelle entre boutiques |
| **SQL** | Requêtes paramétrées, whitelist colonnes triables | Anti-injection SQL |
| **Docker** | Build multi-étapes, utilisateur non-root, image Alpine | Sécurité conteneur |
| **Tests E2E** | Scripts PowerShell WebSocket, tests charge k6, scénarios complets | Validation bout-en-bout |
| **Gestion erreurs** | HTTP 409 sur conflits concurrents (disputes), sentinel errors | Maturité exception handling |

### 🔴 Divergences et Contradictions Détectées

| Point | Audit Claude | Mon Audit (Qwen) | Résolution Requise |
|-------|--------------|------------------|---------------------|
| **Bcrypt cost** | Fallback à 4 si `APP_ENV != production` (fail-open) | README v4.5.0 promet "minimum cost 10" | **Vérifier code actuel** : `domain/entity/user_entity/user.go` doit enforcer `min(10)` |
| **Migrations correctives (25%)** | Signe de mauvaise qualité schéma | Reflète itérations post-audit (048, 033, 034) | Nuancer : démarche itérative d'amélioration, pas négligence |
| **Idempotency disputes** | Non détecté | Commit 12 août 2026 : `ClaimRelease + dispute_resolution idempotency` | Excellente pratique implémentée, documenter |
| **Rate Limiting hybride** | Non noté | v4.4.22 : Redis + fallback mémoire thread-safe | Sophistication rare, à valoriser |
| **DB Warm-up** | Non relevé | Commit 30 juillet 2026 : élimination cold start latency | Optimisation K8s/Serverless importante |

### ⚠️ Failles Critiques (Consensus des 2 Audits)

| # | Sévérité | Faille | Risque | Fichier(s) Concerné(s) |
|---|----------|--------|--------|------------------------|
| 1 | 🔴 **Haute** | Documents KYC non vérifiés côté serveur (MIME, magic number, scope) | IDOR stockage, injection MIME, documents falsifiés | `application/usecase/customer_usecase/upload_kyc.go`, `merchant_kyc_usecase/submit_kyc.go` |
| 2 | 🔴 **Haute** | Couverture tests ≈ 22% sur codebase fintech | Régressions silencieuses, bugs non détectés | Global (fichier `coverage` commité) |
| 3 | 🔴 **Haute** | Transaction KYC marchand `WithTX(nil) // TODO` | État base incohérent si échec partiel | `application/usecase/merchant_kyc_usecase/submit_kyc.go` |
| 4 | 🟠 **Moyenne** | `X-Shop-Slug` actif en production sans restriction | Surface d'attaque superflue (atténué par `RequireShopAccess`) | `interfaces/middl/tenant.go` |
| 5 | 🟠 **Moyenne** | Clé API acceptée via query string `?api_key=` | Fuite dans logs serveur/proxy/CDN/browser history | `interfaces/middl/apikey_auth.go` |
| 6 | 🟡 **Basse** | Absence header HSTS | Pas de force HTTPS | `interfaces/middl/secure_headers.go` |
| 7 | 🟡 **Basse** | Image Docker `alpine:latest` non épinglée | Supply chain compromise possible | `Dockerfile` |
| 8 | 🟡 **Basse** | `k8s/secret.yaml` avec valeurs `"root"` réalistes | Déploiement accidentel non modifié | `k8s/secret.yaml` |

### 🎯 Angles Morts Identifiés (Non Couverts par Claude)

| Domaine | Manque Détecté | Impact |
|---------|----------------|--------|
| **Observabilité** | Pas de distributed tracing (OpenTelemetry) | Impossibilité tracer requêtes multi-provider (Yenga Pay, Orange Money) |
| **Scalabilité DB** | Pas de partitionnement tables `wallet_transactions`, `payments` | Requêtes lentes après 1 an de données |
| **Conformité** | Pas de droit à l'oubli (GDPR/APDP) + rétention BCEAO 10 ans | Non-conformité réglementaire |
| **Mobile** | Pas de BFF (Backend for Frontend) GraphQL | Latence élevée, consommation data mobile |

---

## 🗓️ Chronogramme Détaillé — Plan d'Action

### 🚨 SPRINT 0 : Corrections Immédiates (Semaine 1)

**Objectif :** Quick wins sans risque de régression, sécurité basique.
**Durée :** 2-3 jours
**Livrable :** PR "Hardening Rapide" déployable immédiatement.

| # | Action | Fichier Exact | Commandes/Code | Effort | Status |
|---|--------|---------------|----------------|--------|--------|
| 0.1 | **Vérifier et corriger Bcrypt cost enforcement** | `domain/entity/user_entity/user.go` | Modifier `getBcryptCost()` : ajouter `if cost < 10 { cost = 10 }` avant return + log warning si `APP_ENV` vide | 30 min | ☐ |
| 0.2 | Retirer `coverage` de Git | Racine dépôt | `git rm --cached coverage` + vérifier `.gitignore` | 15 min | ☐ |
| 0.3 | Remplacer valeurs `"root"` par placeholders | `k8s/secret.yaml` | Remplacer toutes les valeurs par `CHANGE_ME_BEFORE_DEPLOY` | 10 min | ☐ |
| 0.4 | Compléter `SECURITY.md` | `SECURITY.md` | Ajouter email contact réel : `security@golanafrica.com` | 5 min | ☐ |
| 0.5 | Supprimer support `?api_key=` query string | `interfaces/middl/apikey_auth.go` | Retirer bloc `r.URL.Query().Get("api_key")` sauf pour webhooks entrants | 30 min | ☐ |
| 0.6 | Ajouter header HSTS | `interfaces/middl/secure_headers.go` | Ajouter `Strict-Transport-Security: max-age=63072000; includeSubDomains` | 15 min | ☐ |
| 0.7 | Épingler image Docker par digest | `Dockerfile` | Remplacer `FROM alpine:latest` par `FROM alpine@sha256:<digest>` (récupérer digest officiel) | 10 min | ☐ |
| 0.8 | Corriger placeholders `.env.example` | `.env.example` | Ajouter commentaires explicites : `# OBLIGATOIRE en production, min 32 caractères` | 20 min | ☐ |

**Critères de validation Sprint 0 :**
- [ ] Tous les fichiers modifiés compilent sans erreur (`go build ./...`)
- [ ] Tests existants passent (`go test ./...`)
- [ ] `coverage` n'apparaît plus dans `git ls-files`
- [ ] `go vet ./...` ne remonte aucune nouvelle erreur
- [ ] PR mergée et tagguée `v4.5.1-security-quickwins`

---

### 🔧 SPRINT 1 : Intégrité Financière et KYC (Semaines 2-3)

**Objectif :** Garantir qu'aucun centime ne peut être perdu et que les documents KYC sont intègres.
**Durée :** 7-10 jours
**Livrable :** PR "Intégrité KYC & Idempotency"

| # | Action | Fichier(s) Exact(s) | Détails Techniques | Effort | Status |
|---|--------|---------------------|---------------------|--------|--------|
| 1.1 | **Implémenter Idempotency Keys globales** | Nouveau : `interfaces/middl/idempotency.go`<br>Nouveau : `migrations/049_idempotency_keys.sql` | **Migration SQL :**<br>`CREATE TABLE idempotency_keys (idempotency_key TEXT PRIMARY KEY, user_id UUID, endpoint TEXT, request_hash TEXT, response_body JSONB, created_at TIMESTAMP DEFAULT NOW());`<br>**Middleware :** Intercepter `POST` financiers (paiements, retraits, crédit, tontine), vérifier clé dans Redis (TTL 24h), rejeter si déjà traité avec même `request_hash` SHA-256 | 4h | ☐ |
| 1.2 | **Transaction réelle KYC marchand** | `application/usecase/merchant_kyc_usecase/submit_kyc.go` | Remplacer `WithTX(nil)` par `txManager.BeginTx()` + `defer tx.Rollback()` + `tx.Commit()` (pattern `wallet_usecase`) | 2h | ☐ |
| 1.3 | **Upload KYC sécurisé — Magic Number** | `application/usecase/customer_usecase/upload_kyc.go`<br>`application/usecase/merchant_kyc_usecase/submit_kyc.go` | Après réception fichier, utiliser `http.DetectContentType(data[:512])` pour vérifier MIME réel vs déclaré. Rejeter si mismatch. | 3h | ☐ |
| 1.4 | **Upload KYC sécurisé — Scope serveur** | Même fichiers que 1.3 | Générer `file_path` côté serveur : `kyc/{shop_id}/{customer_id}/{uuid}.{ext}`. Jamais accepter `file_path` fourni par client. | 2h | ☐ |
| 1.5 | **Upload KYC sécurisé — URLs pré-signées** | Nouveau : `interfaces/handler/kyc_handler/upload_presigned.go` | Endpoint `POST /api/merchant/kyc/presigned-url` qui génère URL S3/GCS temporaire (15min) scopée par `shop_id`. Client upload directement vers storage. | 4h | ☐ |
| 1.6 | **Restreindre `X-Shop-Slug` en production** | `interfaces/middl/tenant.go` | Ajouter check : `if APP_ENV == "production" && !hasAPIKeyScope("tenant_override") { ignore X-Shop-Slug }` | 1h | ☐ |
| 1.7 | **Auditer routes `RequireShopAccess`** | `internal/app/app.go` | Grep systématique : toutes les routes `/api/shops/*`, `/api/products/*`, `/api/orders/*` doivent passer par `RequireShopAccess`. Documenter exceptions (routes publiques). | 2h | ☐ |
| 1.8 | **Vérifier verrous FOR UPDATE tontine** | `application/usecase/tontine_usecase/claim_release.go` | Confirmer que `SELECT ... FOR UPDATE` est utilisé sur `wallet_transactions` lors de distribution gains tontine. | 1h | ☐ |

**Tests à ajouter (Sprint 1) :**
- [ ] Test E2E : soumission KYC avec échec partiel → rollback vérifié
- [ ] Test E2E : tentative IDOR cross-tenant via `X-Shop-Slug` sans droits → HTTP 403
- [ ] Test E2E : double paiement avec même idempotency key → HTTP 409 Conflict
- [ ] Test unitaire : upload fichier avec MIME falsifié → HTTP 400
- [ ] Test unitaire : upload fichier hors scope `shop_id` → HTTP 403

**Critères de validation Sprint 1 :**
- [ ] Tous les modules financiers (`wallet`, `payment`, `tontine`, `withdrawal`) ont idempotency
- [ ] KYC marchand utilise transactions DB réelles
- [ ] Upload KYC vérifie magic number et scope serveur
- [ ] Tests E2E couvrent scénarios critiques (IDOR, double-paiement, rollback)
- [ ] PR mergée et tagguée `v4.6.0-financial-integrity`

---

### 🧪 SPRINT 2 : Observabilité et Couverture Tests (Semaines 4-6)

**Objectif :** Détecter les problèmes avant les utilisateurs, garantir qualité code.
**Durée :** 10-15 jours (peut être mené en parallèle Sprint 1)
**Livrable :** PR "Observabilité & Tests Critiques"

| # | Action | Fichier(s) Exact(s) | Détails Techniques | Effort | Status |
|---|--------|---------------------|---------------------|--------|--------|
| 2.1 | **Intégrer OpenTelemetry** | Nouveau : `infrastructure/tracing/otel.go`<br>`cmd/api/main.go` | Initialiser SDK OpenTelemetry, exporter traces vers Jaeger/Tempo. Corréler `Request-ID` avec appels HTTP externes (Yenga Pay, Orange Money). | 6h | ☐ |
| 2.2 | **Instrumenter handlers critiques** | `interfaces/handler/payment_handler/*`<br>`interfaces/handler/wallet_handler/*` | Ajouter `tracer.Start()` sur handlers paiements, webhooks, wallet. Logger `shop_id`, `user_id`, `payment_id` comme span attributes. | 4h | ☐ |
| 2.3 | **Tests unitaires `wallet_usecase`** | `application/usecase/wallet_usecase/*_test.go` | Couvrir : crédit, débit, held funds, rollback transaction, verrous FOR UPDATE. **Objectif : ≥ 90%** | 8h | ☐ |
| 2.4 | **Tests unitaires `payment_usecase`** | `application/usecase/payment_usecase/*_test.go` | Couvrir : initiation, webhook success/failure, idempotency, refund. **Objectif : ≥ 85%** | 10h | ☐ |
| 2.5 | **Tests unitaires commissions/retraits** | `application/usecase/withdrawal_usecase/*_test.go`<br>`application/usecase/commission_usecase/*_test.go` | Couvrir : calcul commissions, batch generation, withdrawal validation. **Objectif : ≥ 85%** | 6h | ☐ |
| 2.6 | **Tests unitaires tontine** | `application/usecase/tontine_usecase/*_test.go` | Couvrir : création groupe, cotisations, distribution gains, conflits concurrents. **Objectif : ≥ 80%** | 6h | ☐ |
| 2.7 | **Seuil couverture CI bloquant** | `.github/workflows/ci-cd.yml` | Ajouter étape : `go test -coverprofile=coverage.out ./application/usecase/wallet_usecase/...` puis script vérifie `≥ 90%`. Échouer build si seuil non atteint. | 2h | ☐ |
| 2.8 | **Partitionnement tables financières** | Nouveau : `migrations/050_partition_wallet_transactions.sql` | `ALTER TABLE wallet_transactions PARTITION BY RANGE (created_at);` + créer partitions mensuelles. Idem pour `payments`. | 4h | ☐ |
| 2.9 | **Archivage sessions/logs expirés** | `application/scheduler/cleanup_scheduler.go` | Job cron quotidien : archiver `user_sessions` expirées > 90 jours vers table `user_sessions_archive`. Idem `payment_webhooks`. | 3h | ☐ |

**Critères de validation Sprint 2 :**
- [ ] Traces OpenTelemetry visibles dans Jaeger/Tempo pour requêtes paiements
- [ ] Couverture `wallet_usecase` ≥ 90%
- [ ] Couverture `payment_usecase` ≥ 85%
- [ ] CI échoue si couverture modules financiers < seuil
- [ ] Tables partitionnées créées sans downtime (migration en ligne)
- [ ] PR mergée et tagguée `v4.7.0-observability-testing`

---

### 🏗️ SPRINT 3 : Conformité Réglementaire et Mobile (Mois 3)

**Objectif :** Conformité APDP/GDPR, préparer intégrations Mobile Money réelles, optimiser pour mobile.
**Durée :** 15-20 jours
**Livrable :** PR "Conformité & Mobile Ready"

| # | Action | Fichier(s) Exact(s) | Détails Techniques | Effort | Status |
|---|--------|---------------------|---------------------|--------|--------|
| 3.1 | **Droit à l'oubli (GDPR/APDP)** | Nouveau : `application/usecase/user_usecase/anonymize.go`<br>Nouveau : `interfaces/handler/admin_handler/anonymize.go` | Endpoint `POST /api/admin/users/{id}/anonymize` (admin only). Masquer email/phone/nom dans `users` et `customers`, mais **conserver historique financier** (obligation BCEAO 10 ans). Logger action dans `admin_audit_log`. | 6h | ☐ |
| 3.2 | **Rétention données financières 10 ans** | `migrations/051_retention_policy.sql` | Ajouter politique rétention : `payments`, `wallet_transactions`, `tontine_payments` conservés 10 ans minimum. `user_sessions`, `api_keys` : 2 ans. | 2h | ☐ |
| 3.3 | **Intégration Orange Money réelle** | `infrastructure/payment/orange_money_provider.go` | Remplacer mock par appels API Orange Developer (endpoint `/api/v1/payments`). Gérer webhooks asynchrones (confirmation 5-10min après initiation). | 8h | ☐ |
| 3.4 | **Intégration Wave réelle** | `infrastructure/payment/wave_provider.go` | Implémenter API Wave (OAuth2, webhooks). | 6h | ☐ |
| 3.5 | **BFF GraphQL pour mobile** | Nouveau : `interfaces/graphql/schema.graphql`<br>Nouveau : `interfaces/graphql/resolvers/*` | Utiliser `gqlgen` pour générer API GraphQL. Endpoint `/graphql` agrège dashboard marchand en 1 requête (économie data/batterie). | 12h | ☐ |
| 3.6 | **Optimisation images produits** | Nouveau : `infrastructure/image/resize_worker.go` | Worker Go qui redimensionne images uploadées vers WebP (3 tailles : thumbnail, medium, large). Stocker sur CDN. | 6h | ☐ |
| 3.7 | **Tests E2E Mobile Money réels** | `tests/e2e/payment_real_providers_test.go` | Tests sur environnement staging avec vrais comptes Orange Money/Wave (montants faibles, refund immédiat). | 4h | ☐ |
| 3.8 | **Documentation modèle menace** | Nouveau : `docs/THREAT_MODEL.md` | Documenter : quelles routes sont publiques par tenant vs protégées, surface d'attaque multi-tenant, mitigations. | 3h | ☐ |

**Critères de validation Sprint 3 :**
- [ ] Endpoint anonymization fonctionne, conserve historique financier
- [ ] Intégrations Orange Money/Wave testées en staging avec transactions réelles
- [ ] API GraphQL retourne dashboard marchand en < 200ms (vs 800ms REST actuel)
- [ ] Images produits servies en WebP, taille réduite de 70%
- [ ] Documentation modèle menace validée par équipe sécurité
- [ ] PR mergée et tagguée `v5.0.0-compliance-mobile`

---

### 🎯 SPRINT 4 : Audit de Pénétration et Hardening Final (Mois 4)

**Objectif :** Validation externe, corrections finales, préparation production.
**Durée :** 10-15 jours
**Livrable :** Rapport audit + PR "Production Hardened"

| # | Action | Détails | Effort | Status |
|---|--------|---------|--------|--------|
| 4.1 | **Audit de pénétration externe** | Engager firme sécurité pour audit boîte grise. Cibler : webhooks paiement, wallet, KYC, multi-tenant, API keys. | Externe | ☐ |
| 4.2 | **Corrections audit pentest** | Prioriser selon sévérité rapport. | Variable | ☐ |
| 4.3 | **Load testing production-like** | `tests/loadtest/scripts/auth_load.js` + `payment_load.js`. Objectif : 1000 req/s sans dégradation. | 4h | ☐ |
| 4.4 | **Chaos engineering** | Simuler pannes : Redis down, DB latency spike, webhook provider timeout. Vérifier fallbacks. | 6h | ☐ |
| 4.5 | **Runbook production** | Documenter : procédures déploiement, rollback, incident response, escalation. | 8h | ☐ |
| 4.6 | **Formation équipe** | Session 2h sur : modèle menace, procédures incident, monitoring alertes. | 2h | ☐ |

**Critères de validation Sprint 4 :**
- [ ] Audit pentest terminé, toutes les failles critiques corrigées
- [ ] Load test valide 1000 req/s avec P95 latency < 200ms
- [ ] Chaos tests passent sans perte données
- [ ] Runbook production validé par ops team
- [ ] Équipe formée sur procédures incident
- [ ] Tag final : `v5.1.0-production-ready`

---

## 📋 Checklist de Suivi Global

### Sécurité
- [ ] Bcrypt cost enforcement vérifié et corrigé
- [ ] Idempotency keys sur tous endpoints financiers
- [ ] KYC upload sécurisé (magic number, scope serveur, URLs pré-signées)
- [ ] Transactions DB réelles sur KYC marchand
- [ ] `X-Shop-Slug` restreint en production
- [ ] `?api_key=` query string supprimé (sauf webhooks)
- [ ] HSTS activé
- [ ] Docker image épinglée par digest
- [ ] Audit pentest externe réalisé

### Qualité
- [ ] Couverture tests `wallet_usecase` ≥ 90%
- [ ] Couverture tests `payment_usecase` ≥ 85%
- [ ] Couverture tests commissions/retraits ≥ 85%
- [ ] Couverture tests tontine ≥ 80%
- [ ] Seuil couverture CI bloquant configuré
- [ ] Fichier `coverage` retiré de Git
- [ ] `go vet` et `staticcheck` dans CI

### Observabilité
- [ ] OpenTelemetry intégré
- [ ] Traces visibles dans Jaeger/Tempo
- [ ] Métriques Prometheus custom pour modules financiers
- [ ] Alertes configurées (erreurs paiements, latency P95, rate limiting)

### Performance
- [ ] Tables `wallet_transactions` et `payments` partitionnées
- [ ] Archivage sessions/logs expirés automatisé
- [ ] BFF GraphQL pour mobile
- [ ] Images produits optimisées (WebP)
- [ ] Load test 1000 req/s validé

### Conformité
- [ ] Droit à l'oubli (anonymization) implémenté
- [ ] Rétention données financières 10 ans configurée
- [ ] Documentation modèle menace rédigée
- [ ] Intégrations Mobile Money réelles testées

### Process
- [ ] Revue migrations SQL obligatoire avant merge
- [ ] Runbook production rédigé
- [ ] Équipe formée sur procédures incident

---

## 🚀 Prochaines Étapes Immédiates

### Semaine Courante (Sprint 0)
1. **Lundi :** Vérifier contradiction bcrypt (`domain/entity/user_entity/user.go`)
2. **Mardi :** Retirer `coverage` de Git + corriger `k8s/secret.yaml`
3. **Mercredi :** Supprimer `?api_key=` query string + ajouter HSTS
4. **Jeudi :** Épingler Docker image + compléter `SECURITY.md`
5. **Vendredi :** Revue PR Sprint 0, merge, tag `v4.5.1-security-quickwins`

### Semaine Prochaine (Début Sprint 1)
1. **Lundi-Mardi :** Implémenter middleware idempotency keys
2. **Mercredi :** Corriger transaction KYC marchand
3. **Jeudi-Vendredi :** Sécuriser upload KYC (magic number, scope, URLs pré-signées)

---

## 📞 Support et Suivi

**Fréquence de revue :** Point hebdomadaire (30 min) pour suivre avancement, débloquer points techniques, ajuster priorités.

**Contact sécurité :** `security@golanafrica.com` (à confirmer)

**Documentation technique :** Maintenir `docs/CHANGELOG.md` à jour avec chaque PR mergée.

**Versioning :** Suivre Semantic Versioning (SemVer) :
- `v4.5.x` : Corrections sécurité (Sprint 0)
- `v4.6.x` : Intégrité financière (Sprint 1)
- `v4.7.x` : Observabilité/tests (Sprint 2)
- `v5.0.x` : Conformité/mobile (Sprint 3)
- `v5.1.x` : Production hardened (Sprint 4)

---

**Document généré le :** 13 août 2026
**Prochaine revue :** 20 août 2026 (fin Sprint 0)

---

## 📝 Notes pour l'Agent IA

Ce document est conçu pour être **lu et exécuté** par un agent IA assistant le développement GoShop. Pour chaque tâche du chronogramme :

1. **Lire la description** et les fichiers concernés
2. **Ouvrir les fichiers** dans l'éditeur
3. **Appliquer les modifications** selon les détails techniques
4. **Écrire les tests** correspondants
5. **Exécuter les tests** pour valider
6. **Cocher la case Status** (☐ → ☑) une fois terminé
7. **Commiter** avec message conventionnel : `feat(security): implémenter idempotency keys pour paiements`
8. **Créer PR** vers branche `main`

**Règles strictes :**
- Ne jamais merger de PR sans tests passants
- Toujours vérifier `go vet ./...` et `go build ./...` avant commit
- Documenter chaque changement dans `CHANGELOG.md`
- Respecter les conventions de commits (feat, fix, docs, test, chore, security, perf, refactor)

**En cas de doute :** Consulter la documentation existante (`docs/`) ou demander clarification avant d'implémenter.