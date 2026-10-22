-- Migration 064: Élargir failure_reason pour les messages de reconciliation / compensation
-- Contexte : VARCHAR(20) est trop court pour des raisons comme
-- "Stuck in pending/processing after wallet debit. Provider status unknown. Manual verification required."
-- (utilisé par withdrawal_reconciliation_scheduler).
BEGIN;

ALTER TABLE withdrawals
  ALTER COLUMN failure_reason TYPE VARCHAR(255);

COMMIT;