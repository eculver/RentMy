-- +goose Up
CREATE TABLE disputes (
    id                TEXT PRIMARY KEY,
    transaction_id    TEXT NOT NULL REFERENCES transactions(id),
    reporter_id       TEXT NOT NULL REFERENCES users(id),
    reason            TEXT NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'PENDING'
                      CHECK (status IN ('PENDING', 'GATHERING', 'ANALYZING', 'AUTO_RESOLVED', 'AUDIT_QUEUED', 'HUMAN_REVIEW', 'RESOLVED', 'INCONCLUSIVE')),
    route             TEXT CHECK (route IN ('AUTO_RESOLVE', 'AUTO_RESOLVE_AUDIT', 'HUMAN_REVIEW')),
    charge_amount     NUMERIC(10,2) DEFAULT 0,
    confidence        REAL CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1)),
    agent_decision_id TEXT REFERENCES agent_decisions(id),
    reviewer_id       TEXT REFERENCES users(id),
    reviewer_notes    TEXT,
    sla_deadline      TIMESTAMPTZ,
    evidence          JSONB,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_disputes_transaction_id ON disputes(transaction_id);
CREATE INDEX idx_disputes_status ON disputes(status);
CREATE INDEX idx_disputes_sla_deadline ON disputes(sla_deadline) WHERE status = 'HUMAN_REVIEW';
CREATE INDEX idx_disputes_reporter_id ON disputes(reporter_id);

-- +goose Down
DROP TABLE IF EXISTS disputes;
