-- +goose Up
-- reputation_signals records every event that affects a user's reputation score.
-- The risk service reads from this table to compute the full reputation score
-- and apply 180-day decay on negative signals.
CREATE TABLE reputation_signals (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL REFERENCES users(id),
    signal_type      TEXT NOT NULL,
    points           INTEGER NOT NULL,      -- positive = add, negative = subtract
    idempotency_key  TEXT,                  -- non-null for one-time signals (KYC, milestones)
    transaction_id   TEXT,                  -- nullable; links signal to a booking
    emitted_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reputation_signals_user_id    ON reputation_signals(user_id);
CREATE INDEX idx_reputation_signals_emitted_at ON reputation_signals(emitted_at);
CREATE INDEX idx_reputation_signals_type       ON reputation_signals(signal_type);
-- Unique index prevents double-counting one-time signals.
CREATE UNIQUE INDEX idx_reputation_signals_idempotency
    ON reputation_signals(user_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- risk_scores stores the last computed per-transaction risk score.
-- One row per transaction; upserted by the risk service.
CREATE TABLE risk_scores (
    transaction_id   TEXT PRIMARY KEY REFERENCES transactions(id),
    risk_score       INTEGER NOT NULL CHECK (risk_score >= 0 AND risk_score <= 100),
    risk_level       TEXT NOT NULL CHECK (risk_level IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    breakdown        JSONB NOT NULL DEFAULT '{}',
    computed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_risk_scores_risk_level ON risk_scores(risk_level);

-- +goose Down
DROP TABLE IF EXISTS risk_scores;
DROP INDEX IF EXISTS idx_reputation_signals_idempotency;
DROP INDEX IF EXISTS idx_reputation_signals_type;
DROP INDEX IF EXISTS idx_reputation_signals_emitted_at;
DROP INDEX IF EXISTS idx_reputation_signals_user_id;
DROP TABLE IF EXISTS reputation_signals;
