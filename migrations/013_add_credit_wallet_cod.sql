-- Migration 013 : Système complet v3.0.0
-- Date: 2026-06-29
-- Description: Crédit par tempérament + Escrow unifié + Wallet marchand + COD
-- Dépend de : 001-012 (toutes les migrations précédentes)

-- ============================================================
-- PARTIE 1 : CRÉDIT PAR TEMPÉRAMENT
-- ============================================================

-- 1.1 Plans de crédit par produit
CREATE TABLE IF NOT EXISTS credit_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    
    -- Activation
    is_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- Configuration
    min_down_payment_percent INT NOT NULL DEFAULT 20 CHECK (min_down_payment_percent BETWEEN 0 AND 50),
    max_duration_months INT NOT NULL DEFAULT 12 CHECK (max_duration_months BETWEEN 1 AND 36),
    interest_rate_bps INT NOT NULL DEFAULT 0 CHECK (interest_rate_bps BETWEEN 0 AND 1500),
    penalty_rate_bps INT NOT NULL DEFAULT 500 CHECK (penalty_rate_bps BETWEEN 0 AND 2000),
    
    -- Score minimum requis
    min_credit_score INT NOT NULL DEFAULT 300 CHECK (min_credit_score BETWEEN 0 AND 1000),
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(product_id)
);

-- 1.2 Demandes de crédit
CREATE TABLE IF NOT EXISTS credit_applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id),
    product_id UUID NOT NULL REFERENCES products(id),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    
    -- Demande
    requested_duration_months INT NOT NULL,
    credit_score_at_application INT NOT NULL,
    
    -- Calculs
    product_price_cents BIGINT NOT NULL,
    down_payment_cents BIGINT NOT NULL,
    financed_amount_cents BIGINT NOT NULL,
    interest_amount_cents BIGINT NOT NULL,
    total_amount_cents BIGINT NOT NULL,
    monthly_payment_cents BIGINT NOT NULL,
    
    -- Statut
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    rejection_reason TEXT,
    reviewed_by UUID,
    reviewed_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT credit_applications_status_check
        CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled'))
);

-- 1.3 Contrats de crédit (après approbation + down payment)
CREATE TABLE IF NOT EXISTS credit_contracts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES credit_applications(id),
    customer_id UUID NOT NULL REFERENCES customers(id),
    product_id UUID NOT NULL REFERENCES products(id),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    
    -- Montants
    product_price_cents BIGINT NOT NULL,
    down_payment_cents BIGINT NOT NULL,
    financed_amount_cents BIGINT NOT NULL,
    interest_amount_cents BIGINT NOT NULL,
    total_amount_cents BIGINT NOT NULL,
    monthly_payment_cents BIGINT NOT NULL,
    
    -- Durée
    duration_months INT NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    
    -- Statut
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    down_payment_paid_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT credit_contracts_status_check
        CHECK (status IN ('active', 'completed', 'defaulted', 'cancelled'))
);

-- 1.4 Échéances (installments)
CREATE TABLE IF NOT EXISTS credit_installments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contract_id UUID NOT NULL REFERENCES credit_contracts(id) ON DELETE CASCADE,
    
    installment_number INT NOT NULL CHECK (installment_number >= 1),
    due_date DATE NOT NULL,
    amount_cents BIGINT NOT NULL,
    
    -- Paiement
    payment_id UUID REFERENCES payments(id),
    paid_at TIMESTAMPTZ,
    
    -- Statut
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    late_fee_cents BIGINT DEFAULT 0,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(contract_id, installment_number),
    
    CONSTRAINT credit_installments_status_check
        CHECK (status IN ('pending', 'paid', 'late', 'defaulted'))
);

-- 1.5 Scores de crédit
CREATE TABLE IF NOT EXISTS credit_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    
    score INT NOT NULL DEFAULT 500 CHECK (score BETWEEN 0 AND 1000),
    
    -- Statistiques
    total_contracts INT NOT NULL DEFAULT 0,
    completed_contracts INT NOT NULL DEFAULT 0,
    on_time_payments INT NOT NULL DEFAULT 0,
    late_payments INT NOT NULL DEFAULT 0,
    defaults INT NOT NULL DEFAULT 0,
    
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(customer_id, shop_id)
);

-- Index crédit
CREATE INDEX IF NOT EXISTS idx_credit_plans_product ON credit_plans(product_id);
CREATE INDEX IF NOT EXISTS idx_credit_plans_shop ON credit_plans(shop_id);
CREATE INDEX IF NOT EXISTS idx_credit_applications_customer ON credit_applications(customer_id);
CREATE INDEX IF NOT EXISTS idx_credit_applications_status ON credit_applications(status);
CREATE INDEX IF NOT EXISTS idx_credit_contracts_customer ON credit_contracts(customer_id);
CREATE INDEX IF NOT EXISTS idx_credit_contracts_status ON credit_contracts(status);
CREATE INDEX IF NOT EXISTS idx_credit_installments_contract ON credit_installments(contract_id);
CREATE INDEX IF NOT EXISTS idx_credit_installments_due_date ON credit_installments(due_date);
CREATE INDEX IF NOT EXISTS idx_credit_scores_customer ON credit_scores(customer_id);

COMMENT ON TABLE credit_plans IS 'Plans de crédit configurés par produit';
COMMENT ON TABLE credit_applications IS 'Demandes de crédit soumises par les clients';
COMMENT ON TABLE credit_contracts IS 'Contrats de crédit approuvés et actifs';
COMMENT ON TABLE credit_installments IS 'Échéances mensuelles des contrats';
COMMENT ON TABLE credit_scores IS 'Score de fiabilité des clients par boutique';

-- ============================================================
-- PARTIE 2 : ESCROW UNIFIÉ (Tiers de confiance)
-- ============================================================

-- 2.1 Comptes séquestres (unifiés pour tous les modes)
CREATE TABLE IF NOT EXISTS escrow_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Référence selon le type (une seule doit être NOT NULL)
    order_id UUID REFERENCES orders(id),
    credit_contract_id UUID REFERENCES credit_contracts(id),
    tontine_group_id UUID REFERENCES tontine_groups(id),
    
    -- Type de source
    source_type VARCHAR(20) NOT NULL,
    
    -- Montants
    total_amount_cents BIGINT NOT NULL,
    released_amount_cents BIGINT NOT NULL DEFAULT 0,
    commission_cents BIGINT NOT NULL DEFAULT 0,
    
    -- Statut escrow
    status VARCHAR(20) NOT NULL DEFAULT 'funds_held',
    
    -- Timestamps
    funds_held_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    funds_released_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT escrow_accounts_source_type_check
        CHECK (source_type IN ('order', 'credit_contract', 'tontine_group')),
    CONSTRAINT escrow_accounts_status_check
        CHECK (status IN ('funds_held', 'partial_release', 'fully_released', 'refunded', 'disputed')),
    CONSTRAINT escrow_accounts_reference_check
        CHECK (
            (source_type = 'order' AND order_id IS NOT NULL AND credit_contract_id IS NULL AND tontine_group_id IS NULL) OR
            (source_type = 'credit_contract' AND credit_contract_id IS NOT NULL AND order_id IS NULL AND tontine_group_id IS NULL) OR
            (source_type = 'tontine_group' AND tontine_group_id IS NOT NULL AND order_id IS NULL AND credit_contract_id IS NULL)
        )
);

-- 2.2 Preuves de livraison (unifiées)
CREATE TABLE IF NOT EXISTS delivery_proofs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Référence (une seule doit être NOT NULL)
    order_id UUID REFERENCES orders(id),
    credit_contract_id UUID REFERENCES credit_contracts(id),
    tontine_voucher_id UUID REFERENCES tontine_vouchers(id),
    
    -- Preuve d'envoi (marchand)
    shipping_proof_url TEXT,
    shipping_tracking_number VARCHAR(100),
    shipping_carrier VARCHAR(50),
    shipping_date TIMESTAMPTZ,
    shipping_notes TEXT,
    
    -- Preuve de réception (client)
    delivery_proof_url TEXT,
    delivery_signature TEXT,
    delivery_date TIMESTAMPTZ,
    delivery_notes TEXT,
    delivery_rating INT CHECK (delivery_rating BETWEEN 1 AND 5),
    
    -- Statut escrow
    escrow_status VARCHAR(20) NOT NULL DEFAULT 'pending_shipment',
    
    -- Litige
    dispute_raised_at TIMESTAMPTZ,
    dispute_reason TEXT,
    dispute_resolved_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT delivery_proofs_escrow_status_check
        CHECK (escrow_status IN (
            'pending_shipment',
            'shipped',
            'delivered',
            'disputed',
            'released',
            'refunded'
        ))
);

-- 2.3 Historique des événements escrow (audit trail)
CREATE TABLE IF NOT EXISTS delivery_proof_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_proof_id UUID NOT NULL REFERENCES delivery_proofs(id) ON DELETE CASCADE,
    
    event_type VARCHAR(30) NOT NULL,
    event_data JSONB,
    performed_by UUID NOT NULL,
    performed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT delivery_proof_events_type_check
        CHECK (event_type IN (
            'proof_submitted',
            'status_changed',
            'dispute_raised',
            'dispute_resolved',
            'funds_released',
            'funds_refunded'
        ))
);

-- Index escrow
CREATE INDEX IF NOT EXISTS idx_escrow_accounts_order ON escrow_accounts(order_id);
CREATE INDEX IF NOT EXISTS idx_escrow_accounts_credit ON escrow_accounts(credit_contract_id);
CREATE INDEX IF NOT EXISTS idx_escrow_accounts_tontine ON escrow_accounts(tontine_group_id);
CREATE INDEX IF NOT EXISTS idx_escrow_accounts_status ON escrow_accounts(status);
CREATE INDEX IF NOT EXISTS idx_delivery_proofs_order ON delivery_proofs(order_id);
CREATE INDEX IF NOT EXISTS idx_delivery_proofs_credit ON delivery_proofs(credit_contract_id);
CREATE INDEX IF NOT EXISTS idx_delivery_proofs_tontine ON delivery_proofs(tontine_voucher_id);
CREATE INDEX IF NOT EXISTS idx_delivery_proofs_status ON delivery_proofs(escrow_status);
CREATE INDEX IF NOT EXISTS idx_delivery_proof_events_proof ON delivery_proof_events(delivery_proof_id);

COMMENT ON TABLE escrow_accounts IS 'Comptes séquestres unifiés pour tous les modes d''achat';
COMMENT ON TABLE delivery_proofs IS 'Preuves de livraison obligatoires (marchand + client)';
COMMENT ON TABLE delivery_proof_events IS 'Audit trail des événements escrow';

-- ============================================================
-- PARTIE 3 : WALLET MARCHAND + GEL
-- ============================================================

-- 3.1 Portefeuilles marchands
CREATE TABLE IF NOT EXISTS merchant_wallets (
    shop_id UUID PRIMARY KEY REFERENCES shops(id) ON DELETE CASCADE,
    
    -- Solde (peut être NÉGATIF = dette)
    balance_cents BIGINT NOT NULL DEFAULT 0,
    
    -- Gel du compte
    is_frozen BOOLEAN NOT NULL DEFAULT FALSE,
    frozen_at TIMESTAMPTZ,
    frozen_reason TEXT,
    frozen_until TIMESTAMPTZ,
    
    -- Limites
    max_negative_balance_cents BIGINT NOT NULL DEFAULT -100000,
    
    -- Stats
    total_sales_cents BIGINT NOT NULL DEFAULT 0,
    total_commissions_cents BIGINT NOT NULL DEFAULT 0,
    total_payouts_cents BIGINT NOT NULL DEFAULT 0,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3.2 Transactions wallet (audit trail)
CREATE TABLE IF NOT EXISTS wallet_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    
    -- Type de transaction
    transaction_type VARCHAR(30) NOT NULL,
    
    -- Montant (positif = crédit, négatif = débit)
    amount_cents BIGINT NOT NULL,
    
    -- Solde après transaction
    balance_after_cents BIGINT NOT NULL,
    
    -- Référence
    reference_type VARCHAR(20),
    reference_id UUID,
    
    -- Description
    description TEXT,
    
    -- Statut
    status VARCHAR(20) NOT NULL DEFAULT 'completed',
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT wallet_transactions_type_check
        CHECK (transaction_type IN (
            'sale_credit',
            'sale_cod',
            'sale_tontine',
            'sale_credit_plan',
            'commission_debit',
            'payout',
            'deposit',
            'refund',
            'freeze_penalty',
            'unfreeze_deposit'
        )),
    CONSTRAINT wallet_transactions_status_check
        CHECK (status IN ('pending', 'completed', 'failed', 'cancelled'))
);

-- 3.3 Gels de compte
CREATE TABLE IF NOT EXISTS account_freezes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    
    -- Raison
    freeze_reason VARCHAR(30) NOT NULL,
    freeze_details TEXT,
    
    -- Montant dû
    amount_due_cents BIGINT NOT NULL,
    
    -- Timeline
    frozen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    grace_period_days INT NOT NULL DEFAULT 7,
    grace_period_ends_at TIMESTAMPTZ NOT NULL,
    
    -- Résolution
    resolved_at TIMESTAMPTZ,
    resolution VARCHAR(20),
    resolved_by UUID,
    
    -- Rappels
    reminder_1_sent_at TIMESTAMPTZ,
    reminder_2_sent_at TIMESTAMPTZ,
    reminder_3_sent_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT account_freezes_reason_check
        CHECK (freeze_reason IN (
            'negative_balance',
            'unpaid_commission',
            'fraud_suspected',
            'admin_decision'
        )),
    CONSTRAINT account_freezes_resolution_check
        CHECK (resolution IN ('paid', 'suspended', 'waived', 'escalated'))
);

-- Index wallet
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_shop ON wallet_transactions(shop_id);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_type ON wallet_transactions(transaction_type);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_created ON wallet_transactions(created_at);
CREATE INDEX IF NOT EXISTS idx_account_freezes_shop ON account_freezes(shop_id);
CREATE INDEX IF NOT EXISTS idx_account_freezes_grace ON account_freezes(grace_period_ends_at);
CREATE INDEX IF NOT EXISTS idx_merchant_wallets_frozen ON merchant_wallets(is_frozen);

COMMENT ON TABLE merchant_wallets IS 'Portefeuilles virtuels des marchands';
COMMENT ON TABLE wallet_transactions IS 'Historique complet des transactions wallet';
COMMENT ON TABLE account_freezes IS 'Historique des gels de comptes marchands';

-- ============================================================
-- PARTIE 4 : COD (Cash à la livraison) avec preuves
-- ============================================================

-- 4.1 Preuves COD
CREATE TABLE IF NOT EXISTS cod_proofs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL REFERENCES customers(id),
    
    -- Preuve CLIENT
    client_payment_proof_url TEXT,
    client_payment_amount_cents BIGINT,
    client_payment_date TIMESTAMPTZ,
    client_receipt_number VARCHAR(100),
    client_notes TEXT,
    client_submitted_at TIMESTAMPTZ,
    
    -- Preuve MARCHAND
    merchant_receipt_proof_url TEXT,
    merchant_received_amount_cents BIGINT,
    merchant_receipt_date TIMESTAMPTZ,
    merchant_notes TEXT,
    merchant_submitted_at TIMESTAMPTZ,
    
    -- Cohérence
    amounts_match BOOLEAN,
    dates_match BOOLEAN,
    
    -- Commission
    commission_cents BIGINT NOT NULL,
    commission_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    commission_collected_at TIMESTAMPTZ,
    
    -- Statut
    status VARCHAR(20) NOT NULL DEFAULT 'pending_proofs',
    
    -- Litige
    dispute_raised_at TIMESTAMPTZ,
    dispute_reason TEXT,
    dispute_resolved_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT cod_proofs_status_check
        CHECK (status IN (
            'pending_proofs',
            'client_proof_sent',
            'merchant_proof_sent',
            'confirmed',
            'disputed',
            'resolved',
            'completed'
        )),
    CONSTRAINT cod_proofs_commission_status_check
        CHECK (commission_status IN (
            'pending',
            'collected',
            'due',
            'waived'
        ))
);

-- Index COD
CREATE INDEX IF NOT EXISTS idx_cod_proofs_order ON cod_proofs(order_id);
CREATE INDEX IF NOT EXISTS idx_cod_proofs_shop ON cod_proofs(shop_id);
CREATE INDEX IF NOT EXISTS idx_cod_proofs_status ON cod_proofs(status);
CREATE INDEX IF NOT EXISTS idx_cod_proofs_commission ON cod_proofs(commission_status);

COMMENT ON TABLE cod_proofs IS 'Preuves de paiement cash à la livraison';

-- ============================================================
-- PARTIE 5 : Modifications sur tables existantes
-- ============================================================

-- 5.1 Ajout statut COD sur orders
ALTER TABLE orders ADD COLUMN IF NOT EXISTS is_cod BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS cod_proof_id UUID REFERENCES cod_proofs(id);

-- 5.2 Ajout wallet_id sur shops (référence vers merchant_wallets)
-- Pas besoin de FK car merchant_wallets.shop_id est déjà la PK

-- 5.3 Ajout escrow_account_id sur orders
ALTER TABLE orders ADD COLUMN IF NOT EXISTS escrow_account_id UUID REFERENCES escrow_accounts(id);

-- 5.4 Ajout delivery_proof_id sur orders
ALTER TABLE orders ADD COLUMN IF NOT EXISTS delivery_proof_id UUID REFERENCES delivery_proofs(id);

-- ============================================================
-- PARTIE 6 : Données initiales
-- ============================================================

-- Créer un wallet pour chaque shop existant
INSERT INTO merchant_wallets (shop_id, balance_cents, created_at, updated_at)
SELECT id, 0, NOW(), NOW()
FROM shops
WHERE id NOT IN (SELECT shop_id FROM merchant_wallets);

COMMENT ON COLUMN orders.is_cod IS 'Indique si la commande est en cash à la livraison';
COMMENT ON COLUMN orders.cod_proof_id IS 'Référence vers les preuves COD';
COMMENT ON COLUMN orders.escrow_account_id IS 'Référence vers le compte séquestre';
COMMENT ON COLUMN orders.delivery_proof_id IS 'Référence vers les preuves de livraison';