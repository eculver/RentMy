-- +goose Up
-- notifications: in-app notification records, source of truth for read/unread state.
-- Push is fire-and-forget; this table is what the mobile app queries.
CREATE TABLE notifications (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id),
    type       TEXT NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL,
    data       JSONB NOT NULL DEFAULT '{}',
    read       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notifications_user_id ON notifications(user_id);
CREATE INDEX idx_notifications_user_id_created_at ON notifications(user_id, created_at DESC);
-- Partial index for fast unread-count queries.
CREATE INDEX idx_notifications_unread ON notifications(user_id) WHERE read = FALSE;

-- push_tokens: Expo push tokens registered by a user's device.
-- One user may have multiple devices, so one-to-many.
CREATE TABLE push_tokens (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id),
    token      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_push_tokens_user_id ON push_tokens(user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_push_tokens_user_id;
DROP TABLE IF EXISTS push_tokens;
DROP INDEX IF EXISTS idx_notifications_unread;
DROP INDEX IF EXISTS idx_notifications_user_id_created_at;
DROP INDEX IF EXISTS idx_notifications_user_id;
DROP TABLE IF EXISTS notifications;
