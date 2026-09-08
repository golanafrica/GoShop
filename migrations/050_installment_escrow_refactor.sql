-- Migration 050 : Refonte du système de paiement en tranches (Escrow-based Installments)
-- Objectif : Permettre aux marchands de vendre en N tranches sans scoring ni intérêts.
-- L'argent est bloqué en séquestre (escrow) et libéré à la fin, moins la commission admin.

-- 1. Table de configuration du paiement en tranches par produit (Décision du Marchand)
CREATE TABLE IF NOT EXISTS installment_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    nb_tranches INT NOT NULL CHECK (nb_tranches >= 2 AND nb_tranches <= 10),
    delai_jours INT NOT NULL CHECK (delai_jours >= 1),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(product_id) -- Un seul plan actif par produit
);

CREATE INDEX IF NOT EXISTS idx_installment_plans_shop_id ON installment_plans(shop_id);

-- 2. Table des tranches d'une commande (Suivi des paiements)
CREATE TABLE IF NOT EXISTS order_installments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    tranche_number INT NOT NULL,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    due_date TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'overdue')),
    paid_at TIMESTAMPTZ,
    payment_ref VARCHAR(255), -- Référence du paiement Mobile Money (ex: MV-xxxx)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(order_id, tranche_number) -- Une seule tranche N par commande
);

CREATE INDEX IF NOT EXISTS idx_order_installments_order_id ON order_installments(order_id);
CREATE INDEX IF NOT EXISTS idx_order_installments_status ON order_installments(status);
CREATE INDEX IF NOT EXISTS idx_order_installments_due_date ON order_installments(due_date);

-- 3. Liaison de la commande avec le système de séquestre (Escrow)
-- Si les colonnes n'existent pas déjà, on les ajoute.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS escrow_account_id UUID REFERENCES escrow_accounts(id);
ALTER TABLE orders ADD COLUMN IF NOT EXISTS is_fully_paid_in_escrow BOOLEAN NOT NULL DEFAULT false;

-- 4. Index pour retrouver rapidement les commandes en attente de libération de fonds
CREATE INDEX IF NOT EXISTS idx_orders_escrow_pending ON orders(escrow_account_id) WHERE is_fully_paid_in_escrow = false AND escrow_account_id IS NOT NULL;