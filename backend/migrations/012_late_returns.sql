-- +goose Up
CREATE TABLE late_returns (
    id              TEXT PRIMARY KEY,
    transaction_id  TEXT NOT NULL REFERENCES transactions(id),
    renter_id       TEXT NOT NULL REFERENCES users(id),
    host_id         TEXT NOT NULL REFERENCES users(id),
    scheduled_end   TIMESTAMPTZ NOT NULL,
    actual_end      TIMESTAMPTZ,
    late_minutes    INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'MONITORING',
    escalation_level TEXT,
    total_fee_charged BIGINT NOT NULL DEFAULT 0,
    confidence      REAL,
    agent_decision_id TEXT REFERENCES agent_decisions(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_late_returns_transaction_id ON late_returns(transaction_id);
CREATE INDEX idx_late_returns_status ON late_returns(status);
CREATE INDEX idx_late_returns_renter_id ON late_returns(renter_id);

-- +goose Down
DROP TABLE IF EXISTS late_returns;
