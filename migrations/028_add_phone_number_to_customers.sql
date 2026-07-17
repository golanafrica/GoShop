-- migrations/028_add_phone_number_to_customers.sql
-- Description: Ajout du numéro de téléphone pour les relances automatiques de paiement crédit

ALTER TABLE customers 
ADD COLUMN IF NOT EXISTS phone_number VARCHAR(50);

-- Ajout d'un index pour les recherches rapides par numéro de téléphone
CREATE INDEX IF NOT EXISTS idx_customers_phone_number ON customers(phone_number);

DO $$
BEGIN
  RAISE NOTICE '✅ Migration 028 appliquée : Colonne phone_number ajoutée à la table customers';
END $$;