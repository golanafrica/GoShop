-- migrations/030_add_penalty_tracking.sql
-- Description: Ajout du suivi des pénalités de score pour éviter les doublons
-- et préparation du dashboard marchand

-- Ajout du champ penalty_applied_at sur credit_installments
-- Permet de savoir si une pénalité de score a déjà été appliquée pour cette échéance
ALTER TABLE credit_installments 
ADD COLUMN IF NOT EXISTS penalty_applied_at TIMESTAMP WITH TIME ZONE;

-- Index pour les requêtes de scheduler (échéances en retard sans pénalité appliquée)
CREATE INDEX IF NOT EXISTS idx_credit_installments_penalty_tracking 
ON credit_installments(due_date, status, penalty_applied_at)
WHERE status = 'late' AND penalty_applied_at IS NULL;

-- Ajout d'un commentaire pour la documentation
COMMENT ON COLUMN credit_installments.penalty_applied_at IS 'Timestamp when score penalty was applied for this installment (NULL if not yet penalized)';

DO $$
BEGIN
  RAISE NOTICE '✅ Migration 030 appliquée : Champ penalty_applied_at ajouté à credit_installments';
END $$;