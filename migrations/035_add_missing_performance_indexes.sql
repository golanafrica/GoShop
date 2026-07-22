-- ============================================================
-- Migration 035 : Index composites pour optimiser les requêtes critiques
-- Objectif : Accélérer la pagination clients, le dashboard crédit et les requêtes KYC
-- ============================================================

-- 1. OPTIMISATION PAGINATION CLIENTS
-- Permet un filtrage par boutique ET un tri par date sans opération de "Sort" coûteuse.
-- Critique pour FindAllCustomersWithPagination et FindAllCustomersWithSorting
CREATE INDEX IF NOT EXISTS idx_customers_shop_created_at 
ON customers(shop_id, created_at DESC);

-- 2. OPTIMISATION DASHBOARD CRÉDIT (GetClientDashboard)
-- Accélère la jointure LEFT JOIN entre customers et credit_scores
CREATE INDEX IF NOT EXISTS idx_credit_scores_customer_shop 
ON credit_scores(customer_id, shop_id);

-- Accélère la sous-requête des contrats actifs
CREATE INDEX IF NOT EXISTS idx_credit_contracts_customer_shop_status 
ON credit_contracts(customer_id, shop_id, status);

-- Accélère la sous-requête des échéances (filtrage par contrat/statut + tri par date)
CREATE INDEX IF NOT EXISTS idx_credit_installments_contract_status_due_date 
ON credit_installments(contract_id, status, due_date ASC);

-- 3. OPTIMISATION REQUÊTES KYC (kyc_repository.go)
-- Accélère FindByCustomerID et CountByCustomer
CREATE INDEX IF NOT EXISTS idx_kyc_documents_customer_shop 
ON customer_kyc_documents(customer_id, shop_id);

-- Accélère FindPendingByShop (recherche des documents en attente pour une boutique)
CREATE INDEX IF NOT EXISTS idx_kyc_documents_shop_status 
ON customer_kyc_documents(shop_id, status);

-- ============================================================
-- Mise à jour des statistiques pour le planificateur de requêtes
-- ============================================================
ANALYZE customers;
ANALYZE credit_scores;
ANALYZE credit_contracts;
ANALYZE credit_installments;
ANALYZE customer_kyc_documents;

-- ============================================================
-- Documentation des index
-- ============================================================
COMMENT ON INDEX idx_customers_shop_created_at IS 'Optimise la pagination et le tri des clients par boutique';
COMMENT ON INDEX idx_credit_scores_customer_shop IS 'Optimise la jointure du score de crédit dans le dashboard client';
COMMENT ON INDEX idx_credit_contracts_customer_shop_status IS 'Optimise la récupération des contrats actifs par client et boutique';
COMMENT ON INDEX idx_credit_installments_contract_status_due_date IS 'Optimise le tri et le filtrage des échéances de crédit';
COMMENT ON INDEX idx_kyc_documents_customer_shop IS 'Optimise la recherche de documents KYC par client et boutique';
COMMENT ON INDEX idx_kyc_documents_shop_status IS 'Optimise la liste des documents KYC en attente par boutique';