-- +goose Up
-- Add signup_metadata to users for storing device/network context at registration.
-- Used by FraudAgent signal detectors (WiFi BSSID, payment fingerprint).
ALTER TABLE users ADD COLUMN IF NOT EXISTS signup_metadata JSONB NOT NULL DEFAULT '{}';

-- fraud_flags: per-user fraud investigations created by FraudAgent.
CREATE TABLE fraud_flags (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL REFERENCES users(id),
    signals           JSONB NOT NULL DEFAULT '[]',
    total_score       INTEGER NOT NULL DEFAULT 0,
    action            TEXT NOT NULL DEFAULT 'MONITOR'
                      CHECK (action IN ('MONITOR', 'FLAG', 'SUSPEND')),
    agent_decision_id TEXT REFERENCES agent_decisions(id),
    resolved_at       TIMESTAMPTZ,
    resolved_by       TEXT REFERENCES users(id),
    resolution_notes  TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fraud_flags_user_id ON fraud_flags(user_id);
CREATE INDEX idx_fraud_flags_action ON fraud_flags(action);
CREATE INDEX idx_fraud_flags_created_at ON fraud_flags(created_at DESC);
CREATE INDEX idx_fraud_flags_unresolved ON fraud_flags(created_at DESC) WHERE resolved_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_fraud_flags_unresolved;
DROP INDEX IF EXISTS idx_fraud_flags_created_at;
DROP INDEX IF EXISTS idx_fraud_flags_action;
DROP INDEX IF EXISTS idx_fraud_flags_user_id;
DROP TABLE IF EXISTS fraud_flags;
ALTER TABLE users DROP COLUMN IF EXISTS signup_metadata;
