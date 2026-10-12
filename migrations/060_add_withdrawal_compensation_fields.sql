-- ============================================================
-- Migration 060: Withdrawal Compensation Fields (ADR P0)
-- Objectif: Ajouter le suivi des tentatives de compensation en cas d'échec du reverse immédiat
-- ============================================================

BEGIN;

-- 1. Ajout des colonnes de suivi de compensation à la table withdrawals
ALTER TABLE withdrawals
  ADD COLUMN IF NOT EXISTS compensation_attempts INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_compensation_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS failure_reason TEXT;

-- 2. Mise à jour de la contrainte de statut pour inclure le nouvel état 'compensation_pending'
ALTER TABLE withdrawals DROP CONSTRAINT IF EXISTS withdrawals_status_check;

ALTER TABLE withdrawals ADD CONSTRAINT withdrawals_status_check
  CHECK (status IN (
    'pending', 
    'processing', 
    'success', 
    'failed', 
    'cancelled', 
    'compensation_pending'
  ));

COMMIT;