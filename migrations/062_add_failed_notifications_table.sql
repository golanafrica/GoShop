-- Migration 062: Failed Notifications Retry Table
BEGIN;

CREATE TABLE IF NOT EXISTS failed_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_type VARCHAR(100) NOT NULL,
    shop_id VARCHAR(255), -- Peut être null pour les notifications utilisateur
    payload JSONB NOT NULL,
    error_message TEXT,
    retry_count INT DEFAULT 0,
    max_retries INT DEFAULT 3,
    next_retry_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index pour que le scheduler de retry trouve rapidement les notifications à rejouer
CREATE INDEX IF NOT EXISTS idx_failed_notifications_next_retry 
ON failed_notifications(next_retry_at) 
WHERE retry_count < max_retries;

COMMIT;