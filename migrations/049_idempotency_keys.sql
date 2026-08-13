-- ============================================================
-- Migration 049 : Idempotency Keys pour prévention double-dépense
-- ============================================================
-- Objectif : Empêcher les requêtes dupliquées sur les endpoints financiers
-- Cas d'usage : Paiements, retraits, crédits, tontines, remboursements
-- ============================================================

-- Table principale des clés d'idempotence
CREATE TABLE IF NOT EXISTS idempotency_keys (
    idempotency_key TEXT PRIMARY KEY,
    user_id UUID NOT NULL,
    endpoint TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    response_status INTEGER NOT NULL,
    response_headers JSONB NOT NULL DEFAULT '{}',
    response_body JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    
    -- Contraintes
    CONSTRAINT valid_response_status CHECK (response_status >= 100 AND response_status < 600)
);

-- Index pour nettoyage automatique des clés expirées
CREATE INDEX idx_idempotency_keys_expires_at ON idempotency_keys (expires_at);

-- Index pour recherche rapide par user (utile pour audit)
CREATE INDEX idx_idempotency_keys_user_id ON idempotency_keys (user_id);

-- Index pour recherche par endpoint (utile pour statistiques)
CREATE INDEX idx_idempotency_keys_endpoint ON idempotency_keys (endpoint);

-- Trigger pour nettoyage automatique des clés expirées (optionnel, peut être fait par scheduler)
-- CREATE OR REPLACE FUNCTION cleanup_expired_idempotency_keys()
-- RETURNS trigger AS $$
-- BEGIN
--     DELETE FROM idempotency_keys WHERE expires_at < NOW();
--     RETURN NEW;
-- END;
-- $$ LANGUAGE plpgsql;

-- Commentaires pour documentation
COMMENT ON TABLE idempotency_keys IS 'Stockage des clés d''idempotence pour prévenir les requêtes dupliquées';
COMMENT ON COLUMN idempotency_keys.idempotency_key IS 'Clé unique fournie par le client (header Idempotency-Key)';
COMMENT ON COLUMN idempotency_keys.user_id IS 'ID de l''utilisateur qui a fait la requête';
COMMENT ON COLUMN idempotency_keys.endpoint IS 'Endpoint HTTP (ex: POST /api/orders/{id}/pay)';
COMMENT ON COLUMN idempotency_keys.request_hash IS 'Hash SHA-256 du body de la requête pour détecter les changements';
COMMENT ON COLUMN idempotency_keys.response_status IS 'Code HTTP de la réponse originale';
COMMENT ON COLUMN idempotency_keys.response_headers IS 'Headers de la réponse originale (JSON)';
COMMENT ON COLUMN idempotency_keys.response_body IS 'Body de la réponse originale (JSON)';
COMMENT ON COLUMN idempotency_keys.expires_at IS 'Date d''expiration de la clé (TTL 24h recommandé)';