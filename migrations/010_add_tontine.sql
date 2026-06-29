-- Migration 010 : Module Tontine de Biens
-- Date: 2026-06-28
-- Description: Tables pour la tontine de biens physiques (voucher)
-- Dépend de : 001 (products, shops, customers), 002 (shops)

-- ============================================================
-- 1. Configuration tontine par produit (géré par le marchand)
-- ============================================================
CREATE TABLE IF NOT EXISTS product_tontine_settings (
    product_id UUID PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,

    is_tontine_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    allow_commercial_circle BOOLEAN NOT NULL DEFAULT TRUE,
    allow_corporate_circle BOOLEAN NOT NULL DEFAULT TRUE,
    allow_family_circle BOOLEAN NOT NULL DEFAULT TRUE,

    min_participants INT NOT NULL DEFAULT 4 CHECK (min_participants >= 2),
    max_participants INT NOT NULL DEFAULT 12 CHECK (max_participants <= 50),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT product_tontine_participants_check
        CHECK (min_participants <= max_participants)
);

-- ============================================================
-- 2. Groupes de tontine
-- ============================================================
CREATE TABLE IF NOT EXISTS tontine_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    creator_customer_id UUID REFERENCES customers(id),  -- NULL si créé par marchand

    creator_type VARCHAR(20) NOT NULL,  -- 'merchant' ou 'customer'
    circle_type VARCHAR(20) NOT NULL,   -- 'COMMERCIAL', 'CORPORATE', 'FAMILY'

    -- Montants en centimes (BIGINT, jamais float)
    amount_per_cycle_cents BIGINT NOT NULL CHECK (amount_per_cycle_cents > 0),

    total_cycles INT NOT NULL CHECK (total_cycles >= 2),
    current_cycle INT NOT NULL DEFAULT 1,

    -- Accès restreint par code d'invitation
    invite_code VARCHAR(10) UNIQUE NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'PENDING_MEMBERS',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT tontine_groups_circle_check
        CHECK (circle_type IN ('COMMERCIAL', 'CORPORATE', 'FAMILY')),
    CONSTRAINT tontine_groups_status_check
        CHECK (status IN ('PENDING_MEMBERS', 'ACTIVE', 'COMPLETED')),
    CONSTRAINT tontine_groups_creator_check
        CHECK (creator_type IN ('merchant', 'customer'))
);

-- ============================================================
-- 3. Participants du groupe
-- ============================================================
CREATE TABLE IF NOT EXISTS tontine_participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES tontine_groups(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL REFERENCES customers(id),

    payout_position INT NOT NULL CHECK (payout_position >= 1),
    status VARCHAR(20) NOT NULL DEFAULT 'active',

    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(group_id, customer_id),
    UNIQUE(group_id, payout_position),

    CONSTRAINT tontine_participants_status_check
        CHECK (status IN ('active', 'suspended', 'excluded'))
);

-- ============================================================
-- 4. Paiements de cotisation par cycle (via YengaPay)
-- ============================================================
CREATE TABLE IF NOT EXISTS tontine_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES tontine_groups(id) ON DELETE CASCADE,
    participant_id UUID NOT NULL REFERENCES tontine_participants(id),
    customer_id UUID NOT NULL REFERENCES customers(id),

    cycle_number INT NOT NULL CHECK (cycle_number >= 1),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    commission_cents BIGINT NOT NULL DEFAULT 0,  -- Commission GoShop

    -- Référence YengaPay (format: TONTINE:{groupID_short}:{cycle}:{participantID_short})
    yengapay_reference VARCHAR(255),
    yengapay_transaction_id VARCHAR(255),
    payment_provider VARCHAR(50) NOT NULL DEFAULT 'yenga_pay',

    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    due_date TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Idempotence : un seul paiement réussi par participant par cycle
    UNIQUE(group_id, customer_id, cycle_number),

    CONSTRAINT tontine_payments_status_check
        CHECK (status IN ('PENDING', 'PROCESSING', 'DONE', 'FAILED'))
);

-- ============================================================
-- 5. Vouchers de livraison générés après cycle validé
-- ============================================================
CREATE TABLE IF NOT EXISTS tontine_vouchers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES tontine_groups(id) ON DELETE CASCADE,
    participant_id UUID NOT NULL REFERENCES tontine_participants(id),
    customer_id UUID NOT NULL REFERENCES customers(id),
    product_id UUID NOT NULL REFERENCES products(id),
    shop_id UUID NOT NULL REFERENCES shops(id),  -- Sécurité mono-boutique

    -- Code unique que le client présente au marchand
    voucher_code VARCHAR(20) UNIQUE NOT NULL,
    cycle_number INT NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'generated',
    expires_at TIMESTAMPTZ NOT NULL,  -- NOW() + 6 mois
    redeemed_at TIMESTAMPTZ,
    redeemed_by UUID,  -- user_id du marchand qui a validé

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Un seul voucher par participant par cycle
    UNIQUE(group_id, participant_id, cycle_number),

    CONSTRAINT tontine_vouchers_status_check
        CHECK (status IN ('generated', 'redeemed', 'expired', 'cancelled'))
);

-- ============================================================
-- 6. Configuration tontine par boutique (commission)
-- ============================================================
ALTER TABLE shop_payment_settings
    ADD COLUMN IF NOT EXISTS tontine_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS tontine_commission_rate INTEGER NOT NULL DEFAULT 250
        CHECK (tontine_commission_rate >= 0 AND tontine_commission_rate <= 1500);

COMMENT ON COLUMN shop_payment_settings.tontine_enabled IS 'Activation globale de la tontine pour cette boutique';
COMMENT ON COLUMN shop_payment_settings.tontine_commission_rate IS 'Commission GoShop en basis points (250 = 2.50%, max 1500 = 15%)';

-- ============================================================
-- INDEX pour performance
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_product_tontine_shop ON product_tontine_settings(shop_id);
CREATE INDEX IF NOT EXISTS idx_tontine_groups_shop ON tontine_groups(shop_id);
CREATE INDEX IF NOT EXISTS idx_tontine_groups_product ON tontine_groups(product_id);
CREATE INDEX IF NOT EXISTS idx_tontine_groups_invite ON tontine_groups(invite_code);
CREATE INDEX IF NOT EXISTS idx_tontine_groups_status ON tontine_groups(status);
CREATE INDEX IF NOT EXISTS idx_tontine_participants_group ON tontine_participants(group_id);
CREATE INDEX IF NOT EXISTS idx_tontine_participants_customer ON tontine_participants(customer_id);
CREATE INDEX IF NOT EXISTS idx_tontine_payments_group_cycle ON tontine_payments(group_id, cycle_number);
CREATE INDEX IF NOT EXISTS idx_tontine_payments_customer ON tontine_payments(customer_id);
CREATE INDEX IF NOT EXISTS idx_tontine_payments_status ON tontine_payments(status);
CREATE INDEX IF NOT EXISTS idx_tontine_payments_due_date ON tontine_payments(due_date);
CREATE INDEX IF NOT EXISTS idx_tontine_vouchers_code ON tontine_vouchers(voucher_code);
CREATE INDEX IF NOT EXISTS idx_tontine_vouchers_customer ON tontine_vouchers(customer_id);
CREATE INDEX IF NOT EXISTS idx_tontine_vouchers_shop ON tontine_vouchers(shop_id);