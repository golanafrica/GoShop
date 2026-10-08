-- ============================================================
-- Migration 059: Wallet Grace Expiry Job (ADR P2)
-- Tables: platform_settings, wallet_freeze_job_runs, wallet_freeze_job_actions
-- Objectif: Audit + notification des grâces expirées, SANS modifier les états financiers
-- ============================================================

BEGIN;

-- ============================================================
-- 1. TABLE platform_settings (configuration globale clé-valeur)
-- ============================================================
CREATE TABLE IF NOT EXISTS platform_settings (
    key         VARCHAR(100) PRIMARY KEY,
    value       JSONB        NOT NULL,
    updated_by  UUID,
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Commentaire pour documentation
COMMENT ON TABLE platform_settings IS
    'Configuration globale de la plateforme (clé-valeur JSONB). Ex: wallet.freeze_job.enabled';

COMMENT ON COLUMN platform_settings.key IS
    'Clé hiérarchique (ex: wallet.freeze_job.enabled, platform.default_commission_bps)';

COMMENT ON COLUMN platform_settings.value IS
    'Valeur JSON (booléen, objet, tableau, string)';

COMMENT ON COLUMN platform_settings.updated_by IS
    'user_id du Super Admin ayant modifié (NULL = système)';

-- ============================================================
-- 2. TABLE wallet_freeze_job_runs (historique des exécutions)
-- ============================================================
CREATE TABLE IF NOT EXISTS wallet_freeze_job_runs (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Type et mode d'exécution
    run_type          VARCHAR(20)  NOT NULL
                                   CHECK (run_type IN ('scheduled', 'manual')),
    mode              VARCHAR(10)  NOT NULL DEFAULT 'live'
                                   CHECK (mode IN ('live', 'dry_run')),

    -- Compteurs
    wallets_scanned   INT          NOT NULL DEFAULT 0,
    wallets_processed INT          NOT NULL DEFAULT 0,
    wallets_skipped   INT          NOT NULL DEFAULT 0,

    -- Qui a déclenché
    executed_by       UUID,        -- NULL = SYSTEM (cron scheduled)

    -- Timestamps
    started_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    finished_at       TIMESTAMPTZ,
    duration_ms       INT,

    -- Erreur éventuelle
    error_message     TEXT,

    -- Audit
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Index pour lister les runs récents (admin dashboard)
CREATE INDEX IF NOT EXISTS idx_wallet_freeze_job_runs_started
    ON wallet_freeze_job_runs (started_at DESC);

CREATE INDEX IF NOT EXISTS idx_wallet_freeze_job_runs_type_mode
    ON wallet_freeze_job_runs (run_type, mode);

COMMENT ON TABLE wallet_freeze_job_runs IS
    'Historique des exécutions du Wallet Grace Expiry Job (audit global par run)';

COMMENT ON COLUMN wallet_freeze_job_runs.run_type IS
    'scheduled (cron) ou manual (admin a cliqué Run)';

COMMENT ON COLUMN wallet_freeze_job_runs.mode IS
    'live (actions réelles) ou dry_run (preview sans modification)';

COMMENT ON COLUMN wallet_freeze_job_runs.executed_by IS
    'user_id du Super Admin/Admin délégué, ou NULL pour le scheduler système';

-- ============================================================
-- 3. TABLE wallet_freeze_job_actions (détail par wallet traité)
-- ============================================================
CREATE TABLE IF NOT EXISTS wallet_freeze_job_actions (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Lien vers le run parent
    run_id         UUID         NOT NULL
                                REFERENCES wallet_freeze_job_runs(id)
                                ON DELETE CASCADE,

    -- Identité du wallet concerné
    shop_id        UUID         NOT NULL,
    freeze_id      UUID,        -- Référence vers account_freezes.id (si disponible)

    -- Clé d'idempotence: un shop peut être gelé plusieurs fois,
    -- mais pour une MÊME frozen_until, l'action n'est faite qu'une fois
    frozen_until   TIMESTAMPTZ,

    -- Action effectuée
    action         VARCHAR(50)  NOT NULL
                                CHECK (action IN (
                                    'grace_expired_detected',
                                    'notification_sent',
                                    'skipped_already_processed',
                                    'skipped_no_longer_frozen'
                                )),

    -- Politique appliquée
    policy         VARCHAR(50)  NOT NULL DEFAULT 'alert_only',

    -- Détails additionnels (JSONB pour flexibilité)
    details        JSONB,

    -- Audit
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Index pour lister les actions d'un run donné
CREATE INDEX IF NOT EXISTS idx_wallet_freeze_job_actions_run
    ON wallet_freeze_job_actions (run_id);

-- ⚡ INDEX D'IDEMPOTENCE : empêche le traitement en double pour la même grâce expirée
CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_freeze_job_actions_idempotent
    ON wallet_freeze_job_actions (shop_id, frozen_until, action)
    WHERE action = 'grace_expired_detected';

-- Index pour retrouver les actions par shop (pour debug / audit marchand)
CREATE INDEX IF NOT EXISTS idx_wallet_freeze_job_actions_shop
    ON wallet_freeze_job_actions (shop_id, created_at DESC);

COMMENT ON TABLE wallet_freeze_job_actions IS
    'Détail de chaque action du Wallet Grace Expiry Job, avec idempotence par (shop_id, frozen_until, action)';

COMMENT ON COLUMN wallet_freeze_job_actions.frozen_until IS
    'Date d''expiration de la grâce au moment du traitement. Sert de clé d''idempotence.';

COMMENT ON COLUMN wallet_freeze_job_actions.action IS
    'Action effectuée : grace_expired_detected, notification_sent, skipped_already_processed, skipped_no_longer_frozen';

COMMENT ON INDEX uq_wallet_freeze_job_actions_idempotent IS
    'Idempotence: un même shop ne peut avoir qu''une seule détection grace_expired_detected par frozen_until';

-- ============================================================
-- 4. SEED: Configuration par défaut du job (OFF par sécurité)
-- ============================================================
INSERT INTO platform_settings (key, value, updated_by, updated_at)
VALUES
    ('wallet.freeze_job.enabled',       'false'::jsonb,  NULL, NOW()),
    ('wallet.freeze_job.policy',        '"alert_only"'::jsonb, NULL, NOW()),
    ('wallet.freeze_job.max_per_run',   '500'::jsonb,    NULL, NOW()),
    ('wallet.freeze_job.batch_size',    '100'::jsonb,    NULL, NOW())
ON CONFLICT (key) DO NOTHING;

COMMENT ON COLUMN platform_settings.value IS
    'Valeur JSON. wallet.freeze_job.enabled=false par défaut (safety-first). Activation manuelle par Super Admin.';

COMMIT;

-- ============================================================
-- Notes d'utilisation pour les développeurs
-- ============================================================
-- 1. Le scheduler DOIT vérifier dans l'ordre:
--    a) ENV: FREEZE_JOB_HARD_DISABLED=true → STOP immédiat
--    b) DB:  SELECT value->>'enabled' FROM platform_settings WHERE key='wallet.freeze_job.enabled'
--    c) Advisory Lock: pg_try_advisory_lock(99999) → une seule instance
--
-- 2. Le scheduler NE DOIT PAS:
--    - Appeler ResolveEscalated() (cela pose resolved_at et ferme le dossier)
--    - Modifier debt_cents, balance_cents, ou held_cents
--    - Déclencher de clawback ou debt_sweep
--
-- 3. Le scheduler DOIT:
--    - Créer un wallet_freeze_job_runs au début
--    - Pour chaque wallet FindGracePeriodExpired():
--        * Vérifier l'existence de wallet_freeze_job_actions(shop_id, frozen_until, 'grace_expired_detected')
--        * Si existe → skip + incrémenter wallets_skipped
--        * Sinon → insérer action + notifier Super Admin + incrémenter wallets_processed
--    - Mettre à jour finished_at, duration_ms, wallets_scanned/processed/skipped à la fin
--
-- 4. Endpoints admin à implémenter (P2.3):
--    GET  /api/admin/freeze-job              → status / config
--    PUT  /api/admin/freeze-job              → enable/policy (Super Admin only)
--    POST /api/admin/freeze-job/run          → ?mode=dry_run|live (SA ou délégué si enabled)
--    GET  /api/admin/wallets/grace-expired   → preview candidats