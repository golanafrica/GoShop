-- 069_unique_tontine_voucher_group_cycle.sql
-- Un seul bon par (group_id, cycle_number).
-- Avant l'index : dédoublonner si des lignes existent déjà.

-- 1) Inspecter les doublons (manuel en staging) :
-- SELECT group_id, cycle_number, COUNT(*)
-- FROM tontine_vouchers
-- GROUP BY group_id, cycle_number
-- HAVING COUNT(*) > 1;

-- 2) Si doublons : garder le plus ancien id, supprimer les autres (adapter si besoin)
DELETE FROM tontine_vouchers a
USING tontine_vouchers b
WHERE a.group_id = b.group_id
  AND a.cycle_number = b.cycle_number
  AND a.created_at > b.created_at;

-- Variante si created_at égal : garder le plus petit id
DELETE FROM tontine_vouchers a
USING tontine_vouchers b
WHERE a.group_id = b.group_id
  AND a.cycle_number = b.cycle_number
  AND a.id > b.id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_tontine_vouchers_group_cycle
  ON tontine_vouchers (group_id, cycle_number);

COMMENT ON INDEX uq_tontine_vouchers_group_cycle IS
  'Idempotence: at most one voucher per tontine group cycle';