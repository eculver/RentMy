-- +goose Up
-- agent_decisions: audit log for every AI agent decision. Supports learning loop (§31).
-- Uses IF NOT EXISTS to be idempotent — the table was also present in 001_initial_schema.sql
-- in earlier project iterations; this migration adds the refined schema and indexes.
CREATE TABLE IF NOT EXISTS agent_decisions (
    id                  TEXT PRIMARY KEY,
    agent_type          TEXT NOT NULL, -- RISK | VERIFICATION | APPRAISAL | DISPUTE | AGREEMENT | LATE_RETURN | FRAUD | OPS | HUMAN_OVERRIDE
    transaction_id      TEXT REFERENCES transactions(id),
    user_id             TEXT REFERENCES users(id),
    input               JSONB NOT NULL DEFAULT '{}',
    decision            JSONB NOT NULL DEFAULT '{}',
    model               TEXT,          -- LLM model used (null for HUMAN_OVERRIDE or TierNone)
    prompt_version      TEXT,          -- e.g. "v1", "v2"
    confidence          FLOAT,         -- 0.0-1.0
    escalated           BOOLEAN NOT NULL DEFAULT FALSE,
    escalation_reason   TEXT,
    reviewed_by         TEXT REFERENCES users(id),
    override_of         TEXT REFERENCES agent_decisions(id),
    outcome_id          TEXT,          -- links to outcome for learning loop
    outcome_correct     BOOLEAN,       -- was this decision validated by outcome?
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_decisions_agent_type ON agent_decisions(agent_type);
CREATE INDEX IF NOT EXISTS idx_agent_decisions_transaction_id ON agent_decisions(transaction_id) WHERE transaction_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_agent_decisions_user_id ON agent_decisions(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_agent_decisions_outcome_correct ON agent_decisions(agent_type, outcome_correct) WHERE outcome_correct IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_agent_decisions_outcome_correct;
DROP INDEX IF EXISTS idx_agent_decisions_user_id;
DROP INDEX IF EXISTS idx_agent_decisions_transaction_id;
DROP INDEX IF EXISTS idx_agent_decisions_agent_type;
DROP TABLE IF EXISTS agent_decisions;
