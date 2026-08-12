-- 047_add_released_status_to_escrow.sql
-- Ajoute les statuts 'shipped', 'delivered', 'released' à la table escrow_accounts
-- Et migre les anciens statuts 'partial_release' et 'fully_released' vers 'released'

-- ÉTAPE 1 : Migrer les anciennes valeurs vers les nouvelles (AVANT de changer la contrainte)
UPDATE escrow_accounts 
SET status = 'released'
WHERE status IN ('partial_release', 'fully_released');

-- ÉTAPE 2 : Supprimer l'ancienne contrainte
ALTER TABLE escrow_accounts 
DROP CONSTRAINT IF EXISTS escrow_accounts_status_check;

-- ÉTAPE 3 : Ajouter la nouvelle contrainte avec tous les statuts nécessaires
ALTER TABLE escrow_accounts 
ADD CONSTRAINT escrow_accounts_status_check 
CHECK (status IN (
    'funds_held', 
    'shipped', 
    'delivered', 
    'disputed', 
    'released', 
    'refunded'
));

-- ÉTAPE 4 : Documentation
COMMENT ON COLUMN escrow_accounts.status IS 
'Statut de l''escrow: funds_held (séquestré), shipped (expédié), delivered (livré), disputed (litige), released (libéré au marchand), refunded (remboursé au client)';