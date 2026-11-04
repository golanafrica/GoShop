-- 070_add_triggered_by_to_freeze_job_runs.sql
-- 🛡️ A07 FIX : Ajout de la traçabilité de l'acteur qui a déclenché le job (user_id ou 'system_cron')

ALTER TABLE wallet_freeze_job_runs 
ADD COLUMN IF NOT EXISTS triggered_by VARCHAR(255) DEFAULT 'system';

COMMENT ON COLUMN wallet_freeze_job_runs.triggered_by IS 'User ID or system identifier that triggered the job run';