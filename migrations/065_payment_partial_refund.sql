-- migrations/065_payment_partial_refund.sql
-- PR5 Audit : remboursements partiels
-- 1) Colonne cumul refunded_amount_cents
-- 2) Statut partially_refunded dans le CHECK

BEGIN;

-- 1. Cumul des montants déjà remboursés
ALTER TABLE payments
  ADD COLUMN IF NOT EXISTS refunded_amount_cents BIGINT NOT NULL DEFAULT 0;

ALTER TABLE payments
  DROP CONSTRAINT IF EXISTS payments_refunded_amount_non_negative;

ALTER TABLE payments
  ADD CONSTRAINT payments_refunded_amount_non_negative
  CHECK (refunded_amount_cents >= 0);

-- 2. Élargir les statuts autorisés
ALTER TABLE payments
  DROP CONSTRAINT IF EXISTS payments_status_check;

ALTER TABLE payments
  ADD CONSTRAINT payments_status_check
  CHECK (status IN (
    'pending',
    'processing',
    'success',
    'failed',
    'partially_refunded',
    'refunded',
    'cancelled',
    'expired'
  ));

COMMENT ON COLUMN payments.refunded_amount_cents IS
  'Cumul des montants remboursés (centimes). partial + full tracking (PR5).';

COMMIT;