-- +migrate Up
-- ============================================================
-- Migration 039 : Système de gestion des litiges (Disputes)
-- Objectif : Permettre aux clients et marchands d'ouvrir des litiges
--            sur les commandes, et aux administrateurs de les résoudre.
-- ============================================================

-- 1. CRÉATION DU TYPE ENUM POUR LES STATUTS
-- Utilisation d'un bloc DO pour éviter les erreurs si le type existe déjà (idempotence)
DO $$ 
BEGIN
    CREATE TYPE dispute_status_enum AS ENUM (
        'pending',              -- En attente de traitement
        'under_review',         -- En cours d'examen par l'admin
        'resolved_merchant',    -- Résolu en faveur du marchand (fonds libérés)
        'resolved_customer',    -- Résolu en faveur du client (remboursement)
        'cancelled'             -- Litige annulé (ex: preuve fournie par le marchand)
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- 2. CRÉATION DE LA TABLE DES LITIGES
CREATE TABLE IF NOT EXISTS disputes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    payment_id UUID REFERENCES payments(id) ON DELETE SET NULL,
    
    initiator_id UUID NOT NULL, -- ID de l'utilisateur (client ou marchand) qui a ouvert le litige
    initiator_role VARCHAR(50) NOT NULL CHECK (initiator_role IN ('customer', 'merchant', 'admin')),
    
    reason TEXT NOT NULL,       -- Motif du litige
    status dispute_status_enum DEFAULT 'pending',
    
    resolution_notes TEXT,      -- Notes de l'administrateur sur la résolution
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 3. CRÉATION DES INDEX POUR OPTIMISER LES RECHERCHES (Idempotents)
CREATE INDEX IF NOT EXISTS idx_disputes_order_id ON disputes(order_id);
CREATE INDEX IF NOT EXISTS idx_disputes_status ON disputes(status);
CREATE INDEX IF NOT EXISTS idx_disputes_created_at ON disputes(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_disputes_initiator ON disputes(initiator_id, initiator_role);

-- 4. TRIGGER POUR LA MISE À JOUR AUTOMATIQUE DE updated_at (Rendu 100% Idempotent)
CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- On supprime le trigger s'il existe déjà avant de le recréer
DROP TRIGGER IF EXISTS set_timestamp_disputes ON disputes;

CREATE TRIGGER set_timestamp_disputes
    BEFORE UPDATE ON disputes
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

-- ============================================================
-- Mise à jour des statistiques pour le planificateur de requêtes
-- ============================================================
ANALYZE disputes;

-- ============================================================
-- Documentation du schéma
-- ============================================================
COMMENT ON TABLE disputes IS 'Stocke les litiges ouverts sur les commandes pour résolution administrative';
COMMENT ON COLUMN disputes.initiator_role IS 'Rôle de la personne ayant ouvert le litige : customer, merchant, ou admin';
COMMENT ON COLUMN disputes.status IS 'Statut du litige : pending, under_review, resolved_merchant, resolved_customer, cancelled';
COMMENT ON INDEX idx_disputes_order_id IS 'Optimise la recherche de litiges par commande';
COMMENT ON INDEX idx_disputes_status IS 'Optimise le filtrage des litiges par statut';