
# 📋 Guide des Logs GoShop

**Version** : v5.1.0  
**Stack** : [zerolog](https://github.com/rs/zerolog)  
**Code** : `config/setupLogging/`, `interfaces/middl/request_logger.go`, `interfaces/middl/request_id.go`

---

## 1. Formats

### Développement (`APP_ENV=development`)

- Sortie **console colorée** (stderr)
- Horodatage court `15:04:05.000`
- Niveaux affichés : TRACE | DEBUG | INFO | WARN | ERROR | FATAL | PANIC
- Niveau par défaut : **debug** (sauf si `LOG_LEVEL` est défini)

### Staging / Production (`APP_ENV=staging|production`)

- **JSON structuré sur stdout** (cloud-native : Docker, K8s, CloudWatch, Loki)
- Timestamp : **RFC3339Nano**
- Niveau par défaut : **info**
- Champs globaux injectés à chaque ligne : `service`, `version`, `environment`

> Note : la config expose `LogFilePath` / rotation, mais l’implémentation actuelle écrit **uniquement sur stdout** en non-dev. Brancher un collecteur sur le flux conteneur, pas un fichier local.

---

## 2. Configuration (env)

| Variable | Rôle | Exemple |
|----------|------|---------|
| `APP_ENV` | Format + niveau par défaut | `development`, `production` |
| `LOG_LEVEL` | Force le niveau | `trace`, `debug`, `info`, `warn`, `error` |
| `APP_VERSION` | Champ `version` | `v5.1.0` / commit SHA |
| Service name | Fixé par défaut | `goshop-api` |

---

## 3. Champs standards

| Champ | Description | Exemple |
|-------|-------------|---------|
| `level` | Niveau | `info`, `error` |
| `service` | Service | `goshop-api` |
| `version` | Build | `v5.1.0` ou `1.0.0` |
| `environment` | Env | `production` |
| `time` | Instant | RFC3339Nano |
| `message` | Message | `request_completed` |
| `component` | Sous-système (si `WithComponent`) | `tontine_scheduler` |
| `request_id` | Corrélation HTTP | UUID |
| `user_id` | Utilisateur authentifié | UUID (souvent tronqué via helper) |
| `shop_id` | Tenant boutique | UUID |
| `operation` | Opération métier | `resolve_dispute` |
| `caller` | Fichier:ligne (config caller) | `resolve_dispute.go:120` |

Champs access log HTTP fréquents :

| Champ | Description |
|-------|-------------|
| `method`, `path`, `query` | Requête |
| `remote_ip`, `user_agent` | Client |
| `status` | Code HTTP |
| `duration_ms` | Latence |
| `response_size_bytes` | Taille réponse |
| `warning` | `slow_request` si > 2s |

---

## 4. Middleware HTTP

Ordre typique : **RequestID** → logger dans le contexte → **RequestLogger**.

Messages :

1. `request_started` (info) — method, path, query, request_id, IP, UA  
2. `request_completed` — niveau selon statut / latence :
   - status ≥ 500 → **error**
   - status ≥ 400 → **warn**
   - durée > 1s → **warn**
   - sinon → **info**

Récupération dans un handler / usecase :

```go
logger := zerolog.Ctx(ctx)
logger.Info().
    Str("shop_id", shopID).
    Int64("debt_cents", debt).
    Msg("wallet state after release")
```

Helpers (`setupLogging.Logger`) : `WithRequestID`, `WithComponent`, `WithUserID`, `WithShopID`, `WithOperation`, `FromContext`.

---

## 5. Sécurité des logs

**Ne jamais logger** : mots de passe, JWT complets, secrets webhook, corps brut de paiement, documents KYC.

Helpers :

- `utils.SecureLogEmail` → `jac***@domain.com`
- `utils.SecureLogUserID` → préfixe/suffixe UUID

En debug, éviter de dump le body HTTP entier.

---

## 6. Patterns métier utiles (v5.1)

| Domaine | Motifs message / champs à chercher |
|---------|-------------------------------------|
| Debt sweep | `debt_sweep`, `CreditWithDebtSweep`, `swept_cents` |
| Clawback | `ClawbackToDebt`, `dispute_clawback`, `customer_wins` |
| Escrow | `escrow`, `auto-release`, `Merchant wallet credited` |
| Tontine | `tontine`, `voucher`, `Held funds released` |
| Withdrawal | `debt_cents`, withdrawal blocked |
| Scheduler | `component=tontine_scheduler`, `escrow_auto_release` |

Exemple local (Docker) :

```powershell
docker compose logs goshop 2>&1 | Select-String -Pattern "debt_sweep|CreditWithDebtSweep|ClawbackToDebt"
```

```bash
docker compose logs goshop 2>&1 | grep -E 'debt_sweep|request_completed'
```

---

## 7. Recherche / filtrage

### JSON (prod)

```bash
# Erreurs
docker compose logs goshop 2>&1 | jq -R 'fromjson? | select(.level=="error")'

# Une requête
docker compose logs goshop 2>&1 | jq -R 'fromjson? | select(.request_id=="UUID-ICI")'

# HTTP 5xx
docker compose logs goshop 2>&1 | jq -R 'fromjson? | select(.status >= 500)'

# Lents (>1s déjà en warn côté middleware)
docker compose logs goshop 2>&1 | jq -R 'fromjson? | select(.warning=="slow_request")'
```

### Process local

```bash
LOG_LEVEL=debug APP_ENV=development go run ./cmd/api
```

---

## 8. Bonnes pratiques

1. Toujours propager le **contexte** (`zerolog.Ctx(ctx)`).  
2. Ajouter `shop_id` / `order_id` / `dispute_id` sur les chemins finance.  
3. Préférer des **messages stables** (`request_completed`, pas de phrases aléatoires) pour le grep.  
4. Les montants : **centimes** (`balance_cents`, `debt_cents`) — pas de float.  
5. Erreurs : `.Err(err)` + message court ; stack via `pkgerrors` si configuré.  
6. Schedulers : logger `component` + `batch_id` / compteurs.

---

## 9. Observabilité associée

| Signal | Endpoint / outil |
|--------|------------------|
| Métriques | `GET /metrics` (Prometheus) |
| Santé | `/health/live`, `/health/ready` |
| Logs app | stdout conteneur `goshop` |
| Traces | request_id dans logs (pas d’OpenTelemetry documenté ici) |

---

## 10. Fichiers de référence

| Fichier | Rôle |
|---------|------|
| `config/setupLogging/setupLogging.go` | Init logger, niveaux, writers |
| `interfaces/middl/request_id.go` | ID de requête |
| `interfaces/middl/request_logger.go` | Access log HTTP |
| `interfaces/utils/secure_logging.go` | Masquage email / user_id |
| `interfaces/middl/recovery.go` | Panic → log |

---

## 📚 Liens

- [Architecture](01-architecture.md)
- [Wallet debt-sweep](12-wallet-debt-sweep.md) (messages finance)
- [Guide des tests](08-testing-guide.md) (E2E + `docker compose logs`)

---

**Dernière mise à jour** : 2026-10-05
```
