-- +goose Up
-- ULIDs stored as text(26) for readability and portability
-- PostGIS geography type for location columns (meters, WGS84)
-- JSONB for flexible fields: hold_allocation, agreement_snapshot, notification_preferences
-- Timestamps as timestamptz (always UTC)
-- Enums as text with CHECK constraints (not Postgres ENUM — easier to evolve)

CREATE EXTENSION IF NOT EXISTS postgis;

-- users
CREATE TABLE users (
    id                       TEXT PRIMARY KEY,
    email                    TEXT UNIQUE,
    phone                    TEXT UNIQUE,
    password_hash            TEXT,
    name                     TEXT NOT NULL DEFAULT '',
    avatar_url               TEXT,
    identity_status          TEXT NOT NULL DEFAULT 'PENDING'
                             CHECK (identity_status IN ('VERIFIED', 'PENDING', 'REJECTED')),
    reputation_score         INTEGER NOT NULL DEFAULT 0
                             CHECK (reputation_score >= 0 AND reputation_score <= 1000),
    device_fingerprint       TEXT,
    risk_flags               JSONB NOT NULL DEFAULT '[]',
    notification_preferences JSONB NOT NULL DEFAULT '{}',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- listings (with PostGIS)
CREATE TABLE listings (
    id                   TEXT PRIMARY KEY,
    host_id              TEXT NOT NULL REFERENCES users(id),
    title                TEXT NOT NULL DEFAULT '',
    description          TEXT NOT NULL DEFAULT '',
    ai_generated_tags    JSONB NOT NULL DEFAULT '[]',
    estimated_value      NUMERIC(10,2),
    host_declared_value  NUMERIC(10,2),
    value_justification  TEXT,
    price_per_hour       NUMERIC(10,2),
    price_per_day        NUMERIC(10,2),
    min_duration         INTERVAL,
    max_duration         INTERVAL,
    location             GEOGRAPHY(POINT, 4326),
    availability         JSONB NOT NULL DEFAULT '[]',
    has_video            BOOLEAN NOT NULL DEFAULT FALSE,
    status               TEXT NOT NULL DEFAULT 'PENDING'
                         CHECK (status IN ('ACTIVE', 'PENDING', 'FLAGGED', 'SUSPENDED')),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_listings_host_id ON listings(host_id);
CREATE INDEX idx_listings_location ON listings USING GIST(location);
CREATE INDEX idx_listings_status ON listings(status);

-- media
CREATE TABLE media (
    id                TEXT PRIMARY KEY,
    listing_id        TEXT REFERENCES listings(id),
    transaction_id    TEXT,
    media_type        TEXT NOT NULL DEFAULT 'LISTING_PHOTO'
                      CHECK (media_type IN ('LISTING_PHOTO', 'CHECK_IN', 'CHECK_OUT', 'KYC_ID', 'KYC_SELFIE')),
    original_url      TEXT NOT NULL,
    thumbnail_url     TEXT,
    orientation_roll  REAL,
    orientation_pitch REAL,
    orientation_yaw   REAL,
    gps_lat           REAL,
    gps_lng           REAL,
    device_id         TEXT,
    captured_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_media_listing_id ON media(listing_id);

-- transactions
CREATE TABLE transactions (
    id                      TEXT PRIMARY KEY,
    renter_id               TEXT NOT NULL REFERENCES users(id),
    host_id                 TEXT NOT NULL REFERENCES users(id),
    listing_id              TEXT NOT NULL REFERENCES listings(id),
    rental_fee              NUMERIC(10,2),
    hold_amount             NUMERIC(10,2),
    item_value              NUMERIC(10,2),
    guarantee_gap           NUMERIC(10,2),
    risk_score              INTEGER CHECK (risk_score >= 0 AND risk_score <= 100),
    escrow_status           TEXT DEFAULT 'HELD'
                            CHECK (escrow_status IN ('HELD', 'RELEASED', 'CHARGED')),
    hold_status             TEXT DEFAULT 'AUTHORIZED'
                            CHECK (hold_status IN ('AUTHORIZED', 'RELEASED', 'CAPTURED', 'PARTIALLY_CAPTURED')),
    hold_allocation         JSONB NOT NULL DEFAULT '{"totalAuthorized":0,"capturedForLateFees":0,"capturedForDamage":0,"damageReserve":0,"released":0,"remaining":0}',
    guarantee_fund_charged  NUMERIC(10,2) NOT NULL DEFAULT 0,
    agreement_snapshot      JSONB,
    photo_diff_result       TEXT CHECK (photo_diff_result IN ('NO_CHANGE', 'COSMETIC_DAMAGE', 'FUNCTIONAL_DAMAGE', 'MISSING_ITEM', 'INCONCLUSIVE')),
    photo_diff_confidence   REAL CHECK (photo_diff_confidence >= 0 AND photo_diff_confidence <= 1),
    scheduled_start         TIMESTAMPTZ,
    scheduled_end           TIMESTAMPTZ,
    actual_start            TIMESTAMPTZ,
    actual_end              TIMESTAMPTZ,
    status                  TEXT NOT NULL DEFAULT 'REQUESTED'
                            CHECK (status IN ('REQUESTED', 'ACCEPTED', 'DECLINED', 'AUTO_DECLINED', 'ACTIVE', 'COMPLETED', 'DISPUTED', 'CANCELLED')),
    cancelled_by            TEXT CHECK (cancelled_by IN ('RENTER', 'HOST')),
    cancellation_fee        NUMERIC(10,2),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_transactions_renter_id ON transactions(renter_id);
CREATE INDEX idx_transactions_host_id ON transactions(host_id);
CREATE INDEX idx_transactions_listing_id ON transactions(listing_id);
CREATE INDEX idx_transactions_status ON transactions(status);

-- Add FK from media to transactions
ALTER TABLE media ADD CONSTRAINT fk_media_transaction
    FOREIGN KEY (transaction_id) REFERENCES transactions(id);
CREATE INDEX idx_media_transaction_id ON media(transaction_id);

-- proximity_proofs
CREATE TABLE proximity_proofs (
    id              TEXT PRIMARY KEY,
    transaction_id  TEXT NOT NULL REFERENCES transactions(id),
    proof_type      TEXT NOT NULL CHECK (proof_type IN ('CHECK_IN', 'CHECK_OUT')),
    gps_distance    REAL,
    pin             TEXT,
    verified        BOOLEAN NOT NULL DEFAULT FALSE,
    method          TEXT NOT NULL DEFAULT 'GPS'
                    CHECK (method IN ('GPS', 'BLE', 'SMS_FALLBACK')),
    device_id       TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_proximity_proofs_transaction_id ON proximity_proofs(transaction_id);

-- messages
CREATE TABLE messages (
    id              TEXT PRIMARY KEY,
    transaction_id  TEXT NOT NULL REFERENCES transactions(id),
    sender_id       TEXT NOT NULL REFERENCES users(id),
    content         TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_messages_transaction_id ON messages(transaction_id);

-- ratings
CREATE TABLE ratings (
    id              TEXT PRIMARY KEY,
    transaction_id  TEXT NOT NULL REFERENCES transactions(id),
    from_user_id    TEXT NOT NULL REFERENCES users(id),
    to_user_id      TEXT NOT NULL REFERENCES users(id),
    bubbles         JSONB NOT NULL DEFAULT '[]',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(transaction_id, from_user_id)
);
CREATE INDEX idx_ratings_to_user_id ON ratings(to_user_id);

-- agent_decisions
CREATE TABLE agent_decisions (
    id                TEXT PRIMARY KEY,
    agent_type        TEXT NOT NULL
                      CHECK (agent_type IN ('RISK', 'VERIFICATION', 'APPRAISAL', 'DISPUTE', 'AGREEMENT', 'LATE_RETURN', 'FRAUD', 'OPS', 'HUMAN_OVERRIDE')),
    transaction_id    TEXT REFERENCES transactions(id),
    user_id           TEXT REFERENCES users(id),
    input             JSONB NOT NULL,
    decision          JSONB NOT NULL,
    model             TEXT,
    prompt_version    TEXT,
    confidence        REAL,
    escalated         BOOLEAN NOT NULL DEFAULT FALSE,
    escalation_reason TEXT,
    reviewed_by       TEXT REFERENCES users(id),
    override_of       TEXT REFERENCES agent_decisions(id),
    outcome_id        TEXT,
    outcome_correct   BOOLEAN,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_agent_decisions_transaction_id ON agent_decisions(transaction_id);
CREATE INDEX idx_agent_decisions_agent_type ON agent_decisions(agent_type);
CREATE INDEX idx_agent_decisions_outcome ON agent_decisions(outcome_correct) WHERE outcome_correct IS NOT NULL;

-- guarantee_fund_entries
CREATE TABLE guarantee_fund_entries (
    id              TEXT PRIMARY KEY,
    transaction_id  TEXT REFERENCES transactions(id),
    entry_type      TEXT NOT NULL
                    CHECK (entry_type IN ('CONTRIBUTION', 'CLAIM', 'CARD_RECOVERY', 'COLLECTIONS_REFERRAL')),
    amount          NUMERIC(10,2) NOT NULL,
    balance_after   NUMERIC(10,2) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_guarantee_fund_entries_transaction_id ON guarantee_fund_entries(transaction_id);

-- fulltext search index for listings (Phase 2 will use this)
ALTER TABLE listings ADD COLUMN search_vector TSVECTOR;
CREATE INDEX idx_listings_search ON listings USING GIN(search_vector);

-- +goose Down
DROP INDEX IF EXISTS idx_listings_search;
ALTER TABLE listings DROP COLUMN IF EXISTS search_vector;

DROP TABLE IF EXISTS guarantee_fund_entries;
DROP TABLE IF EXISTS agent_decisions;
DROP TABLE IF EXISTS ratings;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS proximity_proofs;
ALTER TABLE media DROP CONSTRAINT IF EXISTS fk_media_transaction;
DROP INDEX IF EXISTS idx_media_transaction_id;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS media;
DROP TABLE IF EXISTS listings;
DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS postgis;
