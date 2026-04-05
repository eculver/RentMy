-- +goose Up
-- Add appraisal_status to listings for quick filtering of un-appraised listings.
ALTER TABLE listings
    ADD COLUMN appraisal_status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (appraisal_status IN ('PENDING', 'COMPLETE', 'FAILED'));

-- appraisals stores the AI appraisal result for each listing.
-- One row per listing (UNIQUE constraint on listing_id).
CREATE TABLE appraisals (
    id                              TEXT PRIMARY KEY,
    listing_id                      TEXT NOT NULL REFERENCES listings(id),
    status                          TEXT NOT NULL DEFAULT 'PENDING'
                                    CHECK (status IN ('PENDING', 'COMPLETE', 'FAILED')),
    item_name                       TEXT,
    category                        TEXT,
    condition                       TEXT,
    estimated_value_cents           INTEGER,
    suggested_price_per_hour_cents  INTEGER,
    suggested_price_per_day_cents   INTEGER,
    description                     TEXT,
    tags                            JSONB NOT NULL DEFAULT '[]',
    confidence                      NUMERIC(5,4),
    model                           TEXT,
    prompt_version                  TEXT,
    override_approved               BOOLEAN,
    override_reasoning              TEXT,
    failure_reason                  TEXT,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_appraisals_listing_id ON appraisals(listing_id);
CREATE INDEX idx_appraisals_status ON appraisals(status);
CREATE INDEX idx_listings_appraisal_status ON listings(appraisal_status);

-- +goose Down
DROP INDEX IF EXISTS idx_listings_appraisal_status;
DROP TABLE IF EXISTS appraisals;
ALTER TABLE listings DROP COLUMN IF EXISTS appraisal_status;
