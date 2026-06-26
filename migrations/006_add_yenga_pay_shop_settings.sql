-- Migration 006 : Configuration Yenga Pay par boutique
-- Date: 2026-06-25
-- Description: Ajoute les champs Yenga Pay dans shop_payment_settings
--              Les clés API sont stockées chiffrées (AES-256)

-- Ajouter les colonnes Yenga Pay
ALTER TABLE shop_payment_settings
ADD COLUMN IF NOT EXISTS yenga_pay_enabled BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS yenga_pay_api_key TEXT,
ADD COLUMN IF NOT EXISTS yenga_pay_organization_id TEXT,
ADD COLUMN IF NOT EXISTS yenga_pay_project_id TEXT,
ADD COLUMN IF NOT EXISTS yenga_pay_webhook_secret TEXT,
ADD COLUMN IF NOT EXISTS yenga_pay_operators JSONB DEFAULT '["orange_money", "moov_money", "telecel", "coris_money", "sank_money"]'::jsonb,
ADD COLUMN IF NOT EXISTS yenga_pay_env VARCHAR(10) DEFAULT 'test';

-- Vérifier que la table existe (créée dans migration 002)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'shop_payment_settings') THEN
        CREATE TABLE shop_payment_settings (
            shop_id UUID PRIMARY KEY REFERENCES shops(id) ON DELETE CASCADE,
            orange_money_enabled BOOLEAN DEFAULT false,
            moov_money_enabled BOOLEAN DEFAULT false,
            wave_enabled BOOLEAN DEFAULT false,
            yenga_pay_enabled BOOLEAN DEFAULT false,
            yenga_pay_api_key TEXT,
            yenga_pay_organization_id TEXT,
            yenga_pay_project_id TEXT,
            yenga_pay_webhook_secret TEXT,
            yenga_pay_operators JSONB DEFAULT '["orange_money", "moov_money", "telecel", "coris_money", "sank_money"]'::jsonb,
            yenga_pay_env VARCHAR(10) DEFAULT 'test',
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW()
        );
    END IF;
END $$;

-- Index pour recherche rapide
CREATE INDEX IF NOT EXISTS idx_shop_payment_settings_yenga_enabled 
    ON shop_payment_settings(yenga_pay_enabled);

COMMENT ON TABLE shop_payment_settings IS 'Configuration des moyens de paiement par boutique (clés API chiffrées)';
COMMENT ON COLUMN shop_payment_settings.yenga_pay_api_key IS 'Clé API Yenga Pay chiffrée (AES-256-GCM)';
COMMENT ON COLUMN shop_payment_settings.yenga_pay_organization_id IS 'ID organisation Yenga Pay chiffré';
COMMENT ON COLUMN shop_payment_settings.yenga_pay_project_id IS 'ID projet Yenga Pay chiffré';
COMMENT ON COLUMN shop_payment_settings.yenga_pay_webhook_secret IS 'Secret webhook Yenga Pay chiffré';