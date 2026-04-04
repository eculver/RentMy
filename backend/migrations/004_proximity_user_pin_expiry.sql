-- +goose Up
-- Add user_id to track which party owns each proximity proof record.
-- Add pin_expires_at to support PIN validity window enforcement.
ALTER TABLE proximity_proofs ADD COLUMN user_id TEXT REFERENCES users(id);
ALTER TABLE proximity_proofs ADD COLUMN pin_expires_at TIMESTAMPTZ;

CREATE INDEX idx_proximity_proofs_user_id ON proximity_proofs(user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_proximity_proofs_user_id;
ALTER TABLE proximity_proofs DROP COLUMN IF EXISTS pin_expires_at;
ALTER TABLE proximity_proofs DROP COLUMN IF EXISTS user_id;
