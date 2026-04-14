#!/usr/bin/env bash
# setup.sh — Seeds E2E test accounts and listings via the real API.
#
# Run from the repo root: bash mobile/e2e/seed/setup.sh
#
# Creates two accounts that are required by the E2E test suite:
#   alice@test.com / password123 — host (will own listings)
#   bob@test.com   / password123 — renter
#
# Then creates several listings owned by alice near Los Angeles
# with keywords matched by the discovery E2E tests.
#
# Idempotent: re-running only fails if the accounts already exist (409),
# which is silently ignored. Listings are created only if none exist yet.

set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5433}"
DB_USER="${DB_USER:-rentmy}"
DB_NAME="${DB_NAME:-rentmy}"

export PGPASSWORD="${DB_PASSWORD:-rentmy}"

register_user() {
  local name="$1"
  local email="$2"
  local password="$3"

  status=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "${API_URL}/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"${name}\",\"email\":\"${email}\",\"password\":\"${password}\"}")

  if [ "$status" = "201" ]; then
    echo "  Created: ${email}"
  elif [ "$status" = "409" ]; then
    echo "  Already exists (ok): ${email}"
  else
    echo "  ERROR registering ${email}: HTTP ${status}" >&2
    exit 1
  fi
}

login_user() {
  local email="$1"
  local password="$2"

  response=$(curl -s -X POST "${API_URL}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"${email}\",\"password\":\"${password}\"}")

  echo "$response" | python3 -c "import sys,json; print(json.load(sys.stdin)['accessToken'])" 2>/dev/null
}

create_listing() {
  local token="$1"
  local title="$2"
  local description="$3"
  local price_per_day="$4"
  local price_per_hour="$5"
  local lat="$6"
  local lng="$7"

  status=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "${API_URL}/api/v1/listings" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${token}" \
    -d "{
      \"title\":\"${title}\",
      \"description\":\"${description}\",
      \"pricePerDay\":${price_per_day},
      \"pricePerHour\":${price_per_hour},
      \"location\":{\"lat\":${lat},\"lng\":${lng}}
    }")

  if [ "$status" = "201" ]; then
    echo "  Created listing: ${title}"
  else
    echo "  Listing create returned HTTP ${status} for: ${title} (may already exist)"
  fi
}

run_sql() {
  local sql="$1"
  # Try docker exec first (psql may not be installed on the host)
  local container
  container=$(docker ps --filter "name=postgres" --format "{{.Names}}" 2>/dev/null | head -1)
  if [ -n "$container" ]; then
    docker exec "$container" psql -U "$DB_USER" -d "$DB_NAME" -t -c "$sql" 2>/dev/null
  else
    PGPASSWORD="${DB_PASSWORD:-rentmy}" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "$sql" 2>/dev/null
  fi
}

activate_listings() {
  echo "  Activating all PENDING listings for alice..."
  run_sql "UPDATE listings SET status = 'ACTIVE' WHERE host_id = (SELECT id FROM users WHERE email = 'alice@test.com') AND status = 'PENDING';" \
    || echo "  WARNING: could not activate listings (non-fatal)"
}

echo "=== Seeding E2E test accounts ==="
register_user "Alice Host"  "alice@test.com" "password123"
register_user "Bob Renter"  "bob@test.com"   "password123"

# Backdate alice's account so the fraud velocity "new-to-new" rule doesn't
# block booking creation (threshold is 30 days — at least one party must have
# an established account).
echo "  Backdating alice's account for fraud-check bypass..."
run_sql "UPDATE users SET created_at = NOW() - INTERVAL '60 days' WHERE email = 'alice@test.com';" \
  || echo "  WARNING: could not backdate alice (non-fatal)"

# Check if alice already has listings (skip creation if so)
LISTING_COUNT=$(run_sql "SELECT count(*) FROM listings WHERE host_id = (SELECT id FROM users WHERE email = 'alice@test.com');" 2>/dev/null | tr -d ' ' || echo "0")

if [ "$LISTING_COUNT" -gt "0" ] 2>/dev/null; then
  echo "=== Alice already has ${LISTING_COUNT} listings, skipping creation ==="
else
  echo "=== Creating test listings for alice ==="
  ALICE_TOKEN=$(login_user "alice@test.com" "password123")

  if [ -z "$ALICE_TOKEN" ]; then
    echo "  ERROR: could not get alice's auth token" >&2
    exit 1
  fi

  # Listings near Los Angeles (34.0522, -118.2437) — diverse titles for search tests
  create_listing "$ALICE_TOKEN" \
    "Canon EOS R5 Camera Kit" \
    "Professional mirrorless camera with 24-105mm lens. Perfect for photo shoots, events, and content creation." \
    75 15 34.0530 -118.2450

  create_listing "$ALICE_TOKEN" \
    "4-Person Camping Tent" \
    "Spacious tent with rainfly and vestibule. Great for weekend camping trips and outdoor adventures." \
    25 5 34.0510 -118.2420

  create_listing "$ALICE_TOKEN" \
    "Electric Bike (eBike)" \
    "Pedal-assist electric bicycle with 50-mile range. Ideal for commuting or exploring the city." \
    40 8 34.0540 -118.2460

  create_listing "$ALICE_TOKEN" \
    "Portable PA Speaker System" \
    "JBL PartyBox with wireless mic. Perfect for outdoor events, parties, and presentations." \
    50 10 34.0515 -118.2430

  create_listing "$ALICE_TOKEN" \
    "GoPro Hero 12 Action Camera" \
    "Waterproof action camera with accessories mount kit. Great for hiking, surfing, and adventure sports." \
    30 6 34.0525 -118.2445
fi

ensure_stripe_customers() {
  echo "=== Ensuring Stripe customer IDs for test users ==="
  # The stub payment adapter requires a non-empty stripe_customer_id.
  # Set placeholder IDs so CreateBooking doesn't fail with ErrNoPaymentMethod.
  run_sql "UPDATE users SET stripe_customer_id = 'cus_stub_alice' WHERE email = 'alice@test.com' AND (stripe_customer_id IS NULL OR stripe_customer_id = '');" \
    && echo "  Set stripe_customer_id for alice" \
    || echo "  WARNING: could not set stripe_customer_id for alice"
  run_sql "UPDATE users SET stripe_customer_id = 'cus_stub_bob' WHERE email = 'bob@test.com' AND (stripe_customer_id IS NULL OR stripe_customer_id = '');" \
    && echo "  Set stripe_customer_id for bob" \
    || echo "  WARNING: could not set stripe_customer_id for bob"
}

echo "=== Activating listings ==="
activate_listings

ensure_stripe_customers

seed_bookings() {
  echo "=== Seeding E2E booking data ==="
  # Create REQUESTED bookings from bob → alice's listings for E2E booking flows.
  # Each booking flow needs its own REQUESTED booking so they don't interfere.
  # We create one booking per alice listing, using the real transactions table.
  #
  # Always clear ALL of bob's bookings and re-seed fresh.  Previous test runs
  # may have left bookings in ACCEPTED / CANCELLED / etc. states that block
  # listing availability for new bookings.
  # Child tables have foreign keys to transactions — delete them first.
  run_sql "
    DO \$\$
    DECLARE
      bob_txn_ids text[];
    BEGIN
      SELECT array_agg(t.id) INTO bob_txn_ids
      FROM transactions t
      JOIN users u ON t.renter_id = u.id
      WHERE u.email = 'bob@test.com';

      IF bob_txn_ids IS NOT NULL THEN
        DELETE FROM agent_decisions WHERE transaction_id = ANY(bob_txn_ids);
        DELETE FROM media WHERE transaction_id = ANY(bob_txn_ids);
        DELETE FROM proximity_proofs WHERE transaction_id = ANY(bob_txn_ids);
        DELETE FROM messages WHERE transaction_id = ANY(bob_txn_ids);
        DELETE FROM ratings WHERE transaction_id = ANY(bob_txn_ids);
        DELETE FROM guarantee_fund_entries WHERE transaction_id = ANY(bob_txn_ids);
        DELETE FROM risk_scores WHERE transaction_id = ANY(bob_txn_ids);
        DELETE FROM agreements WHERE transaction_id = ANY(bob_txn_ids);
        DELETE FROM disputes WHERE transaction_id = ANY(bob_txn_ids);
        DELETE FROM late_returns WHERE transaction_id = ANY(bob_txn_ids);
        DELETE FROM transactions WHERE id = ANY(bob_txn_ids);
      END IF;
    END\$\$;
  " || echo "  WARNING: could not clear stale bookings (non-fatal)"

  # Get alice and bob user IDs
  local alice_id bob_id
  alice_id=$(run_sql "SELECT id FROM users WHERE email = 'alice@test.com';" 2>/dev/null | tr -d ' ')
  bob_id=$(run_sql "SELECT id FROM users WHERE email = 'bob@test.com';" 2>/dev/null | tr -d ' ')

  if [ -z "$alice_id" ] || [ -z "$bob_id" ]; then
    echo "  ERROR: could not find alice or bob user IDs" >&2
    return
  fi

  # Get alice's listing IDs (up to 5)
  local listing_ids
  listing_ids=$(run_sql "SELECT id FROM listings WHERE host_id = '${alice_id}' AND status = 'ACTIVE' ORDER BY created_at LIMIT 5;" 2>/dev/null | tr -d ' ')

  local count=0
  while IFS= read -r lid; do
    [ -z "$lid" ] && continue
    # Generate a ULID-like ID (26 chars, alphanumeric uppercase)
    local txn_id
    txn_id=$(python3 -c "import time,random,string; t=int(time.time()*1000); chars='0123456789ABCDEFGHJKMNPQRSTVWXYZ'; enc=''.join(chars[(t>>(45-5*i))&31] for i in range(10)); rand=''.join(random.choices(chars,k=16)); print(enc+rand)")

    # Schedule start = tomorrow + count hours, end = start + 4 hours
    local start_offset=$((24 + count * 6))
    local end_offset=$((start_offset + 4))

    run_sql "INSERT INTO transactions (id, renter_id, host_id, listing_id, rental_fee, hold_amount, item_value, guarantee_gap, scheduled_start, scheduled_end, status, created_at) VALUES ('${txn_id}', '${bob_id}', '${alice_id}', '${lid}', 0, 0, 0, 0, NOW() + INTERVAL '${start_offset} hours', NOW() + INTERVAL '${end_offset} hours', 'REQUESTED', NOW()) ON CONFLICT DO NOTHING;" \
      && echo "  Created REQUESTED booking ${txn_id} for listing ${lid}" \
      || echo "  WARNING: could not create booking for listing ${lid}"

    count=$((count + 1))
  done <<< "$listing_ids"

  echo "  Seeded ${count} REQUESTED bookings"
}

echo "=== Activating listings ==="
activate_listings

seed_bookings

echo "=== Verifying seed data ==="
ACTIVE_COUNT=$(run_sql "SELECT count(*) FROM listings WHERE host_id = (SELECT id FROM users WHERE email = 'alice@test.com') AND status = 'ACTIVE';" 2>/dev/null | tr -d ' ' || echo "?")
echo "  Active listings for alice: ${ACTIVE_COUNT}"
BOOKING_COUNT=$(run_sql "SELECT count(*) FROM transactions WHERE renter_id = (SELECT id FROM users WHERE email = 'bob@test.com') AND status = 'REQUESTED';" 2>/dev/null | tr -d ' ' || echo "?")
echo "  REQUESTED bookings for bob: ${BOOKING_COUNT}"
echo "Done."
