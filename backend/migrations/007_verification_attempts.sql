-- +goose Up
-- verification_attempts: records each Stripe Identity KYC attempt per user (PRD §6 identity_status).
CREATE TABLE verification_attempts (
    id                  TEXT PRIMARY KEY,
    user_id             TEXT NOT NULL REFERENCES users(id),
    stripe_session_id   TEXT UNIQUE NOT NULL,
    status              TEXT NOT NULL DEFAULT 'PENDING', -- PENDING | VERIFIED | REJECTED | ESCALATED | CANCELED
    stripe_status       TEXT,
    stripe_reason       TEXT,
    document_type       TEXT,
    selfie_match_score  FLOAT,
    fraud_indicators    JSONB NOT NULL DEFAULT '[]',
    decision            TEXT,
    confidence          FLOAT,
    escalation_reason   TEXT,
    model               TEXT,
    prompt_version      TEXT,
    retry_count         INT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_verification_attempts_user_id ON verification_attempts(user_id);
CREATE INDEX idx_verification_attempts_stripe_session_id ON verification_attempts(stripe_session_id);
CREATE INDEX idx_verification_attempts_status ON verification_attempts(status) WHERE status = 'PENDING';

-- +goose Down
DROP INDEX IF EXISTS idx_verification_attempts_status;
DROP INDEX IF EXISTS idx_verification_attempts_stripe_session_id;
DROP INDEX IF EXISTS idx_verification_attempts_user_id;
DROP TABLE IF EXISTS verification_attempts;
