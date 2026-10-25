-- 067_webhook_claim_lease.sql
-- C1 : bail de revendication pour reprendre les webhooks bloqués en PROCESSING

BEGIN;

ALTER TABLE payment_webhooks
  ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMPTZ;

COMMENT ON COLUMN payment_webhooks.claimed_at IS
  'Instant de revendication PROCESSING. Null si non claim / terminé. Reprise si claimed_at + lease < now().';

CREATE INDEX IF NOT EXISTS idx_payment_webhooks_stale_processing
  ON payment_webhooks (provider, claimed_at)
  WHERE processed = false AND processing_error = 'PROCESSING';

COMMIT;