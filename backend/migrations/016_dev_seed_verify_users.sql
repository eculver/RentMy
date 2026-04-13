-- +goose Up
-- Set identity_status to VERIFIED for the Phase 8 dev seed accounts.
-- These accounts (alice@test.com, bob@test.com) are created during local dev bootstrapping
-- and default to PENDING, which blocks the checkout flow under KYCGate.
-- This migration is a no-op when those email addresses do not exist (e.g. production).
UPDATE users
SET identity_status = 'VERIFIED'
WHERE email IN ('alice@test.com', 'bob@test.com')
  AND identity_status = 'PENDING';

-- +goose Down
UPDATE users
SET identity_status = 'PENDING'
WHERE email IN ('alice@test.com', 'bob@test.com')
  AND identity_status = 'VERIFIED';
