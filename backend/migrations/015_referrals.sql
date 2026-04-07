-- +goose Up

-- referral_codes: one code per user, generated on demand.
CREATE TABLE referral_codes (
    id         TEXT PRIMARY KEY,
    code       TEXT NOT NULL UNIQUE,
    user_id    TEXT NOT NULL UNIQUE REFERENCES users(id),
    expires_at TIMESTAMPTZ,
    max_uses   INTEGER NOT NULL DEFAULT 0, -- 0 = unlimited
    use_count  INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_referral_codes_code    ON referral_codes(code);
CREATE INDEX idx_referral_codes_user_id ON referral_codes(user_id);

-- referrals: tracks each referral relationship (referrer → referee).
CREATE TABLE referrals (
    id               TEXT PRIMARY KEY,
    referral_code_id TEXT NOT NULL REFERENCES referral_codes(id),
    referrer_id      TEXT NOT NULL REFERENCES users(id),
    referee_id       TEXT NOT NULL UNIQUE REFERENCES users(id),  -- one referral per user
    status           TEXT NOT NULL DEFAULT 'SIGNED_UP'
                     CHECK (status IN ('SIGNED_UP','FIRST_RENTAL_COMPLETED','PAID','FRAUDULENT')),
    referrer_payout  INTEGER NOT NULL DEFAULT 0, -- cents
    referee_payout   INTEGER NOT NULL DEFAULT 0, -- cents
    completed_at     TIMESTAMPTZ,
    paid_at          TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_referrals_referrer_id ON referrals(referrer_id);
CREATE INDEX idx_referrals_referee_id  ON referrals(referee_id);
CREATE INDEX idx_referrals_status      ON referrals(status);

-- referral_payouts: individual payout records for each party.
CREATE TABLE referral_payouts (
    id                 TEXT PRIMARY KEY,
    referral_id        TEXT NOT NULL REFERENCES referrals(id),
    user_id            TEXT NOT NULL REFERENCES users(id),
    amount             INTEGER NOT NULL DEFAULT 2000, -- cents ($20.00)
    status             TEXT NOT NULL DEFAULT 'PENDING'
                       CHECK (status IN ('PENDING','PROCESSING','PAID','FAILED','BLOCKED')),
    stripe_transfer_id TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_referral_payouts_referral_id ON referral_payouts(referral_id);
CREATE INDEX idx_referral_payouts_user_id     ON referral_payouts(user_id);
CREATE INDEX idx_referral_payouts_status      ON referral_payouts(status);

-- +goose Down
DROP INDEX IF EXISTS idx_referral_payouts_status;
DROP INDEX IF EXISTS idx_referral_payouts_user_id;
DROP INDEX IF EXISTS idx_referral_payouts_referral_id;
DROP TABLE IF EXISTS referral_payouts;

DROP INDEX IF EXISTS idx_referrals_status;
DROP INDEX IF EXISTS idx_referrals_referee_id;
DROP INDEX IF EXISTS idx_referrals_referrer_id;
DROP TABLE IF EXISTS referrals;

DROP INDEX IF EXISTS idx_referral_codes_user_id;
DROP INDEX IF EXISTS idx_referral_codes_code;
DROP TABLE IF EXISTS referral_codes;
