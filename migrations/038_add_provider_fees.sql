-- migrations/038_add_provider_fees.sql
-- Ajoute le champ pour tracer les frais prélevés par le fournisseur de paiement (ex: YengaPay)
-- RENDU IDEMPOTENT pour éviter les erreurs sur DB de test réutilisée

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'payments' AND column_name = 'provider_fees_cents'
    ) THEN
        ALTER TABLE payments ADD COLUMN provider_fees_cents BIGINT DEFAULT 0;
    END IF;
END $$;