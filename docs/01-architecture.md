

```markdown
# Architecture de GoShop

**Version** : v5.1.0  
**Dernière mise à jour** : 2026-10-05  
**Paradigme** : Clean Architecture / DDD / ports & adapters (hexagonale)

---

## 1. Principes

- **Domain au centre** : `domain/` ne dépend pas de Chi, Postgres, Redis ni des providers de paiement.
- **Inversion de dépendances** : les usecases dépendent d’**interfaces** (`domain/repository`, services) ; les implémentations sont dans `infrastructure/`.
- **Application = orchestration** : règles de coordination, transactions, appels multi-repos — pas d’accès SQL direct.
- **Interfaces = entrée** : HTTP (Chi), middlewares (auth, tenant), WebSocket.
- **Testabilité** : mocks d’interfaces ; E2E API + scripts PowerShell finance.
- **Multi-tenant Phase 1** : isolation par `shop_id` + `TenantResolver` (voir [04-multi-tenant.md](04-multi-tenant.md)).
- **Finance** : wallet (`balance` / `held` / `debt`), escrow, clawback, debt sweep (voir [12-wallet-debt-sweep.md](12-wallet-debt-sweep.md)).

---

## 2. Structure du dépôt (alignée `main`)

```text
├── cmd/api/                    # Point d'entrée HTTP
├── internal/app/               # Bootstrap + injection de dépendances (manuel)
├── domain/                     # Cœur métier
│   ├── entity/                 # Order, Payment, MerchantWallet, Dispute, Shop, …
│   ├── repository/             # Ports (interfaces) de persistance
│   ├── service/                # Ports métier (notification, rate limit, …)
│   ├── tenant/                 # Contexte boutique (WithTenant / FromContext)
│   └── …                       # value objects / auth selon modules
├── application/
│   ├── usecase/                # Cas d'usage par domaine
│   │   ├── wallet_usecase/
│   │   ├── dispute_usecase/
│   │   ├── withdrawal_usecase/
│   │   ├── installment_usecase/
│   │   ├── payment_usecase/
│   │   ├── order_usecase/
│   │   ├── tontine_usecase/
│   │   └── …
│   ├── dto/                    # Contrats entrée/sortie API
│   ├── mapper/                 # Entité ↔ DTO
│   ├── scheduler/              # Jobs périodiques
│   │   ├── escrow_auto_release_scheduler.go
│   │   ├── installment_auto_release_scheduler.go
│   │   ├── tontine_scheduler.go
│   │   ├── commission_scheduler.go
│   │   └── online_payment_scheduler.go
│   └── metrics/                # Métriques Prometheus applicatives
├── interfaces/
│   ├── handler/                # Handlers Chi (wallet, payment, admin, …)
│   ├── middl/                  # Auth, TenantResolver, headers, recovery
│   └── utils/                  # JSON, erreurs, JWT helpers
├── infrastructure/
│   ├── postgres/               # Repositories SQL
│   ├── payment/                # Registry + providers (Yenga, mocks MM, …)
│   ├── websocket/              # Hub + Redis Pub/Sub
│   └── notification/           # Dispatcher WS / email
├── migrations/                 # SQL versionnés (001 → 058+)
├── config/                     # Env, logging
├── tests/                      # unit / integration / e2e Go / load (k6)
├── mocks/                      # Mocks générés
├── docs/                       # Documentation technique
├── e2e-*.ps1                   # Smoke finance / pay-in (racine)
└── k8s/                        # Manifests (si présents)
```

> L’injection se fait surtout dans **`internal/app/app.go`** (construction des usecases, handlers, schedulers). Si Google Wire est introduit plus tard, documenter `wire.go` explicitement.

---

## 3. Flux de dépendances

```text
HTTP / WS / Cron
       │
       ▼
interfaces (handler, middl)
       │
       ▼
application (usecase, scheduler)
       │
       ├──► domain/entity  (règles : CreditWithDebtSweep, ApplyClawbackToDebt, …)
       │
       └──► domain/repository + domain/service  (interfaces)
                    ▲
                    │ implémente
       infrastructure (postgres, payment, notification, …)
```

**Règle :** une couche intérieure **ne doit pas** importer une couche extérieure (`domain` ↛ `interfaces` / `infrastructure`).

---

## 4. Couches — rôles

| Couche | Responsabilité | Exemples |
|--------|----------------|----------|
| **domain** | Invariants métier, agrégats | `MerchantWallet`, statuts escrow/dispute |
| **application** | Orchestration, TX, idempotence | `CreditWallet`, `ResolveDispute`, auto-release |
| **interfaces** | HTTP/WS, auth, tenant | `WalletHandler`, `TenantResolver` |
| **infrastructure** | I/O | Postgres, YengaPay, Redis, SMTP |

---

## 5. Domaines métier principaux

| Domaine | Emplacement typique |
|---------|---------------------|
| Auth / RBAC / 2FA / sessions / API keys | usecase + handlers auth |
| Shop / multi-tenant | `domain/tenant`, middl tenant |
| Catalogue / commandes / COD | order, product, cod |
| Paiements & webhooks | payment_usecase + infrastructure/payment |
| Wallet / retraits / debt-sweep | wallet_usecase, withdrawal_usecase |
| Litiges / clawback | dispute_usecase |
| Installment & zones | installment + delivery_zone + schedulers |
| Tontine | tontine_usecase + scheduler |
| Notifications | domain/service + websocket + email |

---

## 6. Chemins transverses

### 6.1 Requête marchand (tenant)

```text
Request → Auth JWT → TenantResolver (slug/host + owner/collab)
        → Handler → Usecase → Repo (WHERE shop_id = …) → Response
```

### 6.2 Webhook paiement

```text
Provider → /webhooks/{provider} (HMAC) → Usecase statut payment
         → (éventuel) effets commande / escrow — hors X-Shop-Slug
```

### 6.3 Auto-release escrow

```text
Scheduler → candidats éligibles (délai zone, pas de dispute active)
         → crédit wallet (CreditWithDebtSweep si dette)
         → ledger + statut escrow released
```

### 6.4 Litige post-release `customer_wins`

```text
Admin resolve → clawback balance puis debt_cents → ledger clawback/debt_add
             → refund canal selon pay-in — voir doc wallet
```

---

## 7. Persistence & migrations

- Schéma versionné sous `migrations/` (`golang-migrate` ou runner projet).
- Tables finance critiques : `merchant_wallets`, `wallet_transactions`, escrow, `disputes`, `delivery_zones`.
- Plan : [06-migration-plan.md](06-migration-plan.md).

---

## 8. Observabilité & runtime

- Logs structurés (**zerolog**)
- Métriques **Prometheus** (`/metrics`)
- Health : `/health/live`, `/health/ready`
- Jobs : schedulers démarrés depuis le bootstrap app

---

## 9. Tests

| Niveau | Où |
|--------|-----|
| Unitaires | `*_test.go` près du code / `tests/unit` |
| Intégration | tags `integration` + Postgres |
| E2E Go | `tests/e2e` |
| E2E finance PS1 | `e2e-dispute-merchant-wins.ps1`, `e2e-debt-sweep-fraud.ps1`, `e2e-clawback-*.ps1` |
| Charge | k6 sous `tests/loadtest` |

Détail : [08-testing-guide.md](08-testing-guide.md).

---

## 10. Références

- [Multi-tenant](04-multi-tenant.md)
- [Modèle de données](02-domain-model.md)
- [Wallet debt / clawback / sweep](12-wallet-debt-sweep.md)
- [Plan de migrations](06-migration-plan.md)
- [Glossaire](09-glossary.md)
- [API](03-api-reference.md)

---

**Dernière mise à jour** : 2026-10-05
```

