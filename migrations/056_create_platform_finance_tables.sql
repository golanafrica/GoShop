-- Migration: 056_create_platform_finance_tables.sql
-- Description: Crée les tables pour la gestion de la trésorerie de la plateforme (Revenus et Fonds de Garantie)

-- 1. Table de la Trésorerie Principale (Revenus GoShop)
CREATE TABLE IF NOT EXISTS platform_revenue_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    balance_cents BIGINT NOT NULL DEFAULT 0,
    total_collected_cents BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insérer un compte de revenu par défaut s'il n'existe pas
INSERT INTO platform_revenue_accounts (id, balance_cents, total_collected_cents)
VALUES (gen_random_uuid(), 0, 0)
ON CONFLICT (id) DO NOTHING;

-- 2. Journal des transactions de la Trésorerie (Audit Trail)
CREATE TABLE IF NOT EXISTS platform_revenue_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_type VARCHAR(50) NOT NULL, -- ex: 'commission_collected', 'refund_adjustment'
    amount_cents BIGINT NOT NULL,
    reference_type VARCHAR(50),            -- ex: 'order', 'dispute'
    reference_id UUID,                     -- ID de la commande ou du litige associé
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Table du Fonds de Garantie (pour couvrir les frais PSP en cas de remboursement)
CREATE TABLE IF NOT EXISTS platform_guarantee_funds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    balance_cents BIGINT NOT NULL DEFAULT 0,
    total_deposited_cents BIGINT NOT NULL DEFAULT 0,
    total_withdrawn_cents BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insérer un fonds de garantie par défaut avec un solde initial de sécurité (ex: 500 000 XOF = 50 000 000 centimes)
-- Tu pourras ajuster ce montant ou le recharger plus tard via un script admin.
INSERT INTO platform_guarantee_funds (id, balance_cents, total_deposited_cents)
VALUES (gen_random_uuid(), 50000000, 50000000)
ON CONFLICT (id) DO NOTHING;

-- 4. Journal des transactions du Fonds de Garantie (Audit Trail)
CREATE TABLE IF NOT EXISTS platform_guarantee_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_type VARCHAR(50) NOT NULL, -- ex: 'initial_deposit', 'withdrawal_for_refund_fee'
    amount_cents BIGINT NOT NULL,
    reference_type VARCHAR(50),            -- ex: 'order', 'dispute'
    reference_id UUID,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index pour les recherches rapides par référence (optimisation des requêtes d'audit)
CREATE INDEX IF NOT EXISTS idx_revenue_txn_reference ON platform_revenue_transactions(reference_type, reference_id);
CREATE INDEX IF NOT EXISTS idx_guarantee_txn_reference ON platform_guarantee_transactions(reference_type, reference_id);