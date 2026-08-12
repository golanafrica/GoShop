-- ============================================================
-- 048 : Idempotence crédits wallet + claim atomique escrow
-- Anti double-crédit (orders auto-release, disputes, tontine)
-- ============================================================

-- 1) reference_type plus large (escrow_auto_release, dispute_resolution, tontine_cycle, …)
ALTER TABLE wallet_transactions
  ALTER COLUMN reference_type TYPE VARCHAR(64);

-- 2) reference_id en TEXT pour accepter UUID ET clés composites (groupID:cycle:N)
ALTER TABLE wallet_transactions
  ALTER COLUMN reference_id TYPE TEXT USING reference_id::text;

-- 3) Index unique partiel : 1 seul crédit completed par (type, id)
--    Les NULL sont ignorés (plusieurs txn sans référence restent possibles)
CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_txn_ref_completed
ON wallet_transactions (reference_type, reference_id)
WHERE reference_type IS NOT NULL
  AND reference_id IS NOT NULL
  AND status = 'completed';

-- 4) Index de lookup (scheduler / admin sans filtre shop)
CREATE INDEX IF NOT EXISTS idx_wallet_txn_ref
ON wallet_transactions (reference_type, reference_id)
WHERE reference_type IS NOT NULL AND reference_id IS NOT NULL;

COMMENT ON INDEX uq_wallet_txn_ref_completed IS
  'Garantit un seul crédit completed par référence métier (anti double-accréditation)';