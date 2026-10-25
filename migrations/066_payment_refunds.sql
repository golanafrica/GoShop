-- 066_payment_refunds.sql
-- B3 Audit : demandes de remboursement durables + idempotence

BEGIN;

CREATE TABLE IF NOT EXISTS payment_refunds (
  id              UUID PRIMARY KEY,
  payment_id      UUID NOT NULL REFERENCES payments(id),
  shop_id         UUID NOT NULL,
  amount_cents    BIGINT NOT NULL CHECK (amount_cents > 0),
  status          VARCHAR(32) NOT NULL,
  reason          TEXT,
  idempotency_key VARCHAR(128) NOT NULL,
  provider_ref    TEXT,
  provider_error  TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT payment_refunds_status_check CHECK (
    status IN ('pending', 'completed', 'failed', 'reconciliation_required')
  )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_refunds_shop_idempotency
  ON payment_refunds (shop_id, idempotency_key);

CREATE INDEX IF NOT EXISTS idx_payment_refunds_payment_id
  ON payment_refunds (payment_id);

CREATE INDEX IF NOT EXISTS idx_payment_refunds_status_open
  ON payment_refunds (status)
  WHERE status IN ('pending', 'reconciliation_required');

COMMENT ON TABLE payment_refunds IS
  'Demandes de remboursement durables (réservation + résultat provider). B3 audit.';

COMMIT;