-- ============================================================
-- Migration 061: Index pour le job compensation_pending
-- Objectif: accelerer FindCompensationPending (FIFO + filtre attempts)
-- ============================================================

BEGIN;

CREATE INDEX IF NOT EXISTS idx_withdrawals_compensation_pending
  ON withdrawals (status, last_compensation_at NULLS FIRST, created_at)
  WHERE status = 'compensation_pending';

COMMENT ON INDEX idx_withdrawals_compensation_pending IS
  'Partial index for CompensationPendingScheduler retry queue (FIFO)';

COMMIT;