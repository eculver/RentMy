-- +goose Up
-- agreements stores the immutable agreement snapshot for each transaction.
-- Each transaction has exactly one agreement (generated after host acceptance).
CREATE TABLE agreements (
    id                  TEXT        NOT NULL PRIMARY KEY,
    transaction_id      TEXT        NOT NULL UNIQUE REFERENCES transactions(id) ON DELETE CASCADE,
    version             TEXT        NOT NULL,          -- template version, e.g. "1"
    full_agreement      JSONB       NOT NULL,          -- complete rendered agreement JSON
    custom_clauses      JSONB       NOT NULL DEFAULT '[]', -- AI-generated item-specific clauses
    prompt_version      TEXT,                          -- prompt version used for clause generation
    model               TEXT,                          -- model that generated clauses
    guardrail_warnings  JSONB       NOT NULL DEFAULT '[]', -- log of any stripped clauses
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agreements_transaction_id ON agreements(transaction_id);

-- agreement_acceptances records when each party accepts the agreement.
-- Both the host and renter must accept before the booking can go ACTIVE.
CREATE TABLE agreement_acceptances (
    id              TEXT        NOT NULL PRIMARY KEY,
    agreement_id    TEXT        NOT NULL REFERENCES agreements(id) ON DELETE CASCADE,
    user_id         TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    accepted_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ip_address      TEXT,
    device_id       TEXT,
    UNIQUE (agreement_id, user_id)
);

CREATE INDEX idx_agreement_acceptances_agreement_id ON agreement_acceptances(agreement_id);
CREATE INDEX idx_agreement_acceptances_user_id ON agreement_acceptances(user_id);

-- +goose Down
DROP TABLE IF EXISTS agreement_acceptances;
DROP TABLE IF EXISTS agreements;
