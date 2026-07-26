-- migrations/038_add_provider_fees.sql
-- Ajoute le champ pour tracer les frais prélevés par le fournisseur de paiement (ex: YengaPay)
ALTER TABLE payments ADD COLUMN provider_fees_cents BIGINT DEFAULT 0;