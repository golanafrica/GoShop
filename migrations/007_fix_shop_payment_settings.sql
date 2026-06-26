-- Migration 007 : Correction de la table shop_payment_settings
-- Date: 2026-06-26
-- Description: Ajoute les colonnes manquantes pour Orange/Moov/Wave

-- Ajouter les colonnes manquantes pour les autres providers
ALTER TABLE shop_payment_settings
ADD COLUMN IF NOT EXISTS orange_money_enabled BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS moov_money_enabled BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS wave_enabled BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();

-- Vérifier la structure finale
SELECT column_name, data_type, is_nullable
FROM information_schema.columns
WHERE table_name = 'shop_payment_settings'
ORDER BY ordinal_position;