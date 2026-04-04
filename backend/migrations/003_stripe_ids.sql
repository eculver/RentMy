-- +goose Up
-- Add Stripe IDs to users for customer and connected account tracking.
ALTER TABLE users ADD COLUMN stripe_customer_id TEXT;
ALTER TABLE users ADD COLUMN stripe_account_id TEXT;

-- Add Stripe IDs and payment breakdown columns to transactions.
ALTER TABLE transactions ADD COLUMN stripe_payment_intent_id TEXT;
ALTER TABLE transactions ADD COLUMN stripe_charge_id TEXT;
ALTER TABLE transactions ADD COLUMN stripe_transfer_id TEXT;
ALTER TABLE transactions ADD COLUMN platform_fee NUMERIC(10,2) NOT NULL DEFAULT 0;
ALTER TABLE transactions ADD COLUMN host_payout NUMERIC(10,2) NOT NULL DEFAULT 0;
ALTER TABLE transactions ADD COLUMN guarantee_contribution NUMERIC(10,2) NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE transactions DROP COLUMN IF EXISTS guarantee_contribution;
ALTER TABLE transactions DROP COLUMN IF EXISTS host_payout;
ALTER TABLE transactions DROP COLUMN IF EXISTS platform_fee;
ALTER TABLE transactions DROP COLUMN IF EXISTS stripe_transfer_id;
ALTER TABLE transactions DROP COLUMN IF EXISTS stripe_charge_id;
ALTER TABLE transactions DROP COLUMN IF EXISTS stripe_payment_intent_id;

ALTER TABLE users DROP COLUMN IF EXISTS stripe_account_id;
ALTER TABLE users DROP COLUMN IF EXISTS stripe_customer_id;
