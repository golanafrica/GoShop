-- ============================================
-- Migration 054 : Customer Reliability Scores
-- (fichier: migrations/053_create_customer_reliability_scores.sql)
-- ============================================
-- Score de fiabilité client (0-1000) basé sur l'historique des paiements,
-- KYC, et comportements pour déterminer l'éligibilité aux tranches et tontines.

-- ============================================
-- 0. Fonction partagée auto-update updated_at
--    (absente des migrations antérieures ; utilisée par le trigger ci-dessous)
-- ============================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ============================================
-- 1. Création de la table
-- ============================================

CREATE TABLE IF NOT EXISTS customer_reliability_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    score INT NOT NULL DEFAULT 400 CHECK (score >= 0 AND score <= 1000),
    tier VARCHAR(20) NOT NULL DEFAULT 'BRONZE' CHECK (tier IN ('BRONZE', 'SILVER', 'GOLD')),
    last_calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Un client ne peut avoir qu'un seul score
    UNIQUE(customer_id)
);

-- ============================================
-- 2. Index pour performance
-- ============================================

CREATE INDEX IF NOT EXISTS idx_customer_reliability_scores_customer_id
    ON customer_reliability_scores(customer_id);

CREATE INDEX IF NOT EXISTS idx_customer_reliability_scores_tier
    ON customer_reliability_scores(tier);

CREATE INDEX IF NOT EXISTS idx_customer_reliability_scores_score
    ON customer_reliability_scores(score DESC);

-- ============================================
-- 3. Trigger auto-update updated_at
-- ============================================

DROP TRIGGER IF EXISTS update_customer_reliability_scores_updated_at
    ON customer_reliability_scores;

CREATE TRIGGER update_customer_reliability_scores_updated_at
    BEFORE UPDATE ON customer_reliability_scores
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================
-- 4. Commentaires
-- ============================================

COMMENT ON TABLE customer_reliability_scores IS
    'Score de fiabilité client (0-1000) basé sur l''historique des paiements, KYC, et comportements';

COMMENT ON COLUMN customer_reliability_scores.score IS
    'Score de 0 à 1000 : 0-599 Bronze, 600-799 Silver, 800-1000 Gold';

COMMENT ON COLUMN customer_reliability_scores.tier IS
    'Niveau de fiabilité : BRONZE (nouveau), SILVER (KYC vérifié), GOLD (client fidèle)';

COMMENT ON COLUMN customer_reliability_scores.last_calculated_at IS
    'Date du dernier calcul automatique du score';

-- ============================================
-- 5. Résumé
-- ============================================

DO $$
BEGIN
    RAISE NOTICE '✅ Migration 054 terminée avec succès';
    RAISE NOTICE '   - Fonction update_updated_at_column assurée';
    RAISE NOTICE '   - Table customer_reliability_scores créée';
    RAISE NOTICE '   - 3 index créés pour performance';
    RAISE NOTICE '   - Trigger auto-update activé';
    RAISE NOTICE '   - Règles : Bronze (400-599, max 5 tranches), Silver (600-799, max 5 tranches), Gold (800-1000, illimité)';
END $$;