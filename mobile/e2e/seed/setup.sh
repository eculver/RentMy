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

  # Get alice's listing IDs (first 2 only — listings 3-5 reserved for handoff flows)
  local listing_ids
  listing_ids=$(run_sql "SELECT id FROM listings WHERE host_id = '${alice_id}' AND status = 'ACTIVE' ORDER BY created_at LIMIT 2;" 2>/dev/null | tr -d ' ')

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

seed_bookings

seed_handoff_bookings() {
  echo "=== Seeding E2E handoff booking data ==="
  # Create bookings in ACCEPTED, ACTIVE, and COMPLETED states for handoff flows.
  # Each uses a different listing to avoid conflicts.
  # These are separate from the REQUESTED bookings created by seed_bookings().

  local alice_id bob_id
  alice_id=$(run_sql "SELECT id FROM users WHERE email = 'alice@test.com';" 2>/dev/null | tr -d ' ')
  bob_id=$(run_sql "SELECT id FROM users WHERE email = 'bob@test.com';" 2>/dev/null | tr -d ' ')

  if [ -z "$alice_id" ] || [ -z "$bob_id" ]; then
    echo "  ERROR: could not find alice or bob user IDs" >&2
    return
  fi

  # Get alice's active listing IDs
  local listing_ids
  listing_ids=$(run_sql "SELECT id FROM listings WHERE host_id = '${alice_id}' AND status = 'ACTIVE' ORDER BY created_at;" 2>/dev/null | tr -d ' ')

  # Convert to array
  local listings=()
  while IFS= read -r lid; do
    [ -z "$lid" ] && continue
    listings+=("$lid")
  done <<< "$listing_ids"

  if [ "${#listings[@]}" -lt 5 ]; then
    echo "  WARNING: need at least 5 listings, found ${#listings[@]}" >&2
    return
  fi

  # Use listings 3, 4, 5 (0-indexed) for handoff flows (1, 2 are used for REQUESTED)
  local accepted_listing="${listings[2]}"
  local active_listing="${listings[3]}"
  local completed_listing="${listings[4]}"

  # Normalize ALL of alice's listing locations to known coordinates so Maestro
  # setLocation works reliably for GPS proximity verification (≤100m threshold).
  # We update all listings (not just handoff ones) because the booking acceptance
  # test converts a REQUESTED booking to ACCEPTED — that booking's listing must
  # also have correct coordinates for the check-in GPS flow.
  local handoff_lat=34.0522
  local handoff_lng=-118.2437
  run_sql "UPDATE listings SET location = ST_SetSRID(ST_MakePoint(${handoff_lng}, ${handoff_lat}), 4326)::geography
    WHERE host_id = '${alice_id}';" \
    && echo "  Set all listing locations to (${handoff_lat}, ${handoff_lng})" \
    || echo "  WARNING: could not update listing locations"

  gen_ulid() {
    python3 -c "import time,random; t=int(time.time()*1000); chars='0123456789ABCDEFGHJKMNPQRSTVWXYZ'; enc=''.join(chars[(t>>(45-5*i))&31] for i in range(10)); rand=''.join(random.choices(chars,k=16)); print(enc+rand)"
  }

  # ── ACCEPTED booking (for check-in flow) ──────────────────────────────────
  # Fixed IDs so the check-in E2E test can target this specific booking by
  # its testID suffix (last 8 chars → "king0001").
  local accepted_txn_id="AAAA0000CHECKINBOOKING0001"
  local accepted_proof_id="AAAA0000CHECKINPROOF000001"

  run_sql "INSERT INTO transactions (id, renter_id, host_id, listing_id, rental_fee, hold_amount, item_value, guarantee_gap, scheduled_start, scheduled_end, status, created_at)
    VALUES ('${accepted_txn_id}', '${bob_id}', '${alice_id}', '${accepted_listing}', 7500, 15000, 30000, 0,
      NOW() - INTERVAL '3 hours', NOW() + INTERVAL '1 hour', 'ACCEPTED', NOW() - INTERVAL '4 hours')
    ON CONFLICT DO NOTHING;" \
    && echo "  Created ACCEPTED booking ${accepted_txn_id}" \
    || echo "  WARNING: could not create ACCEPTED booking"

  # Insert host's CHECK_IN proximity proof with PIN=1234 (verified GPS)
  run_sql "INSERT INTO proximity_proofs (id, transaction_id, user_id, proof_type, gps_distance, verified, method, pin, pin_expires_at, created_at)
    VALUES ('${accepted_proof_id}', '${accepted_txn_id}', '${alice_id}', 'CHECK_IN', 15.0, true, 'GPS', '1234', NOW() + INTERVAL '30 minutes', NOW())
    ON CONFLICT DO NOTHING;" \
    && echo "  Created host CHECK_IN proof with PIN=1234" \
    || echo "  WARNING: could not create host CHECK_IN proof"

  # ── ACTIVE booking (for active-rental + check-out flows) ──────────────────
  local active_txn_id
  active_txn_id=$(gen_ulid)
  local active_proof_host_ci active_proof_renter_ci active_proof_host_co
  active_proof_host_ci=$(gen_ulid)
  active_proof_renter_ci=$(gen_ulid)
  active_proof_host_co=$(gen_ulid)

  run_sql "INSERT INTO transactions (id, renter_id, host_id, listing_id, rental_fee, hold_amount, item_value, guarantee_gap, scheduled_start, scheduled_end, actual_start, status, created_at)
    VALUES ('${active_txn_id}', '${bob_id}', '${alice_id}', '${active_listing}', 5000, 10000, 20000, 0,
      NOW(), NOW() + INTERVAL '24 hours', NOW(), 'ACTIVE', NOW() - INTERVAL '1 minute')
    ON CONFLICT DO NOTHING;" \
    && echo "  Created ACTIVE booking ${active_txn_id}" \
    || echo "  WARNING: could not create ACTIVE booking"

  # Host CHECK_IN proof (verified)
  run_sql "INSERT INTO proximity_proofs (id, transaction_id, user_id, proof_type, gps_distance, verified, method, pin, pin_expires_at, created_at)
    VALUES ('${active_proof_host_ci}', '${active_txn_id}', '${alice_id}', 'CHECK_IN', 10.0, true, 'GPS', '5678', NOW() + INTERVAL '30 minutes', NOW() - INTERVAL '1 hour')
    ON CONFLICT DO NOTHING;" \
    || echo "  WARNING: could not create ACTIVE host CHECK_IN proof"

  # Renter CHECK_IN proof (verified)
  run_sql "INSERT INTO proximity_proofs (id, transaction_id, user_id, proof_type, gps_distance, verified, method, pin, pin_expires_at, created_at)
    VALUES ('${active_proof_renter_ci}', '${active_txn_id}', '${bob_id}', 'CHECK_IN', 12.0, true, 'GPS', '5678', NOW() + INTERVAL '30 minutes', NOW() - INTERVAL '1 hour')
    ON CONFLICT DO NOTHING;" \
    || echo "  WARNING: could not create ACTIVE renter CHECK_IN proof"

  # Host CHECK_OUT proof (pre-verified so renter can complete check-out)
  run_sql "INSERT INTO proximity_proofs (id, transaction_id, user_id, proof_type, gps_distance, verified, method, created_at)
    VALUES ('${active_proof_host_co}', '${active_txn_id}', '${alice_id}', 'CHECK_OUT', 8.0, true, 'GPS', NOW())
    ON CONFLICT DO NOTHING;" \
    && echo "  Created ACTIVE booking proofs (host+renter CHECK_IN, host CHECK_OUT)" \
    || echo "  WARNING: could not create ACTIVE CHECK_OUT proof"

  # ── COMPLETED booking (for return-confirmation flow) ──────────────────────
  local completed_txn_id
  completed_txn_id=$(gen_ulid)
  local completed_proof_hci completed_proof_rci completed_proof_hco completed_proof_rco
  completed_proof_hci=$(gen_ulid)
  completed_proof_rci=$(gen_ulid)
  completed_proof_hco=$(gen_ulid)
  completed_proof_rco=$(gen_ulid)

  run_sql "INSERT INTO transactions (id, renter_id, host_id, listing_id, rental_fee, hold_amount, item_value, guarantee_gap, scheduled_start, scheduled_end, actual_start, actual_end, status, created_at)
    VALUES ('${completed_txn_id}', '${bob_id}', '${alice_id}', '${completed_listing}', 3000, 6000, 12000, 0,
      NOW() - INTERVAL '24 hours', NOW() - INTERVAL '20 hours', NOW() - INTERVAL '24 hours', NOW() - INTERVAL '20 hours', 'COMPLETED', NOW() - INTERVAL '25 hours')
    ON CONFLICT DO NOTHING;" \
    && echo "  Created COMPLETED booking ${completed_txn_id}" \
    || echo "  WARNING: could not create COMPLETED booking"

  # All 4 proximity proofs (verified)
  run_sql "INSERT INTO proximity_proofs (id, transaction_id, user_id, proof_type, gps_distance, verified, method, created_at) VALUES
    ('${completed_proof_hci}', '${completed_txn_id}', '${alice_id}', 'CHECK_IN', 5.0, true, 'GPS', NOW() - INTERVAL '24 hours'),
    ('${completed_proof_rci}', '${completed_txn_id}', '${bob_id}', 'CHECK_IN', 7.0, true, 'GPS', NOW() - INTERVAL '24 hours'),
    ('${completed_proof_hco}', '${completed_txn_id}', '${alice_id}', 'CHECK_OUT', 6.0, true, 'GPS', NOW() - INTERVAL '20 hours'),
    ('${completed_proof_rco}', '${completed_txn_id}', '${bob_id}', 'CHECK_OUT', 9.0, true, 'GPS', NOW() - INTERVAL '20 hours')
    ON CONFLICT DO NOTHING;" \
    && echo "  Created COMPLETED booking proofs (all 4 verified)" \
    || echo "  WARNING: could not create COMPLETED proofs"

  echo "  Handoff bookings seeded: ACCEPTED=${accepted_txn_id}, ACTIVE=${active_txn_id}, COMPLETED=${completed_txn_id}"
}

seed_handoff_bookings

seed_conversations() {
  echo "=== Seeding E2E conversation data (messages) ==="
  # Insert pre-existing messages on bob's first REQUESTED booking so the
  # messaging E2E flows find a conversation with visible content.

  local alice_id bob_id
  alice_id=$(run_sql "SELECT id FROM users WHERE email = 'alice@test.com';" 2>/dev/null | tr -d ' ')
  bob_id=$(run_sql "SELECT id FROM users WHERE email = 'bob@test.com';" 2>/dev/null | tr -d ' ')

  if [ -z "$alice_id" ] || [ -z "$bob_id" ]; then
    echo "  ERROR: could not find alice or bob user IDs" >&2
    return
  fi

  # Pick the first REQUESTED booking between bob and alice
  local txn_id
  txn_id=$(run_sql "SELECT id FROM transactions WHERE renter_id = '${bob_id}' AND host_id = '${alice_id}' AND status = 'REQUESTED' ORDER BY created_at LIMIT 1;" 2>/dev/null | tr -d ' ')

  if [ -z "$txn_id" ]; then
    echo "  WARNING: no REQUESTED booking found for messaging seed (non-fatal)"
    return
  fi

  # Clear any existing messages on this booking (idempotent re-run)
  run_sql "DELETE FROM messages WHERE transaction_id = '${txn_id}';" \
    || echo "  WARNING: could not clear existing messages"

  gen_ulid() {
    python3 -c "import time,random; t=int(time.time()*1000); chars='0123456789ABCDEFGHJKMNPQRSTVWXYZ'; enc=''.join(chars[(t>>(45-5*i))&31] for i in range(10)); rand=''.join(random.choices(chars,k=16)); print(enc+rand)"
  }

  local msg1_id msg2_id
  msg1_id=$(gen_ulid)
  msg2_id=$(gen_ulid)

  run_sql "INSERT INTO messages (id, transaction_id, sender_id, content, created_at) VALUES
    ('${msg1_id}', '${txn_id}', '${bob_id}', 'Hi! Is the item ready for pickup?', NOW() + INTERVAL '1 second'),
    ('${msg2_id}', '${txn_id}', '${alice_id}', 'Yes, come by anytime after 10am!', NOW() + INTERVAL '2 seconds')
    ON CONFLICT DO NOTHING;" \
    && echo "  Seeded 2 messages on booking ${txn_id}" \
    || echo "  WARNING: could not seed messages"
}

seed_conversations

seed_dispute_rating_bookings() {
  echo "=== Seeding E2E dispute & rating booking data ==="
  # Create bookings in ACTIVE (for dispute filing), COMPLETED (for rating),
  # and DISPUTED (for viewing dispute status) states.
  # These are used by the dispute and rating E2E flows.

  local alice_id bob_id
  alice_id=$(run_sql "SELECT id FROM users WHERE email = 'alice@test.com';" 2>/dev/null | tr -d ' ')
  bob_id=$(run_sql "SELECT id FROM users WHERE email = 'bob@test.com';" 2>/dev/null | tr -d ' ')

  if [ -z "$alice_id" ] || [ -z "$bob_id" ]; then
    echo "  ERROR: could not find alice or bob user IDs" >&2
    return
  fi

  # Get alice's active listing IDs
  local listing_ids
  listing_ids=$(run_sql "SELECT id FROM listings WHERE host_id = '${alice_id}' AND status = 'ACTIVE' ORDER BY created_at;" 2>/dev/null | tr -d ' ')

  local listings=()
  while IFS= read -r lid; do
    [ -z "$lid" ] && continue
    listings+=("$lid")
  done <<< "$listing_ids"

  if [ "${#listings[@]}" -lt 3 ]; then
    echo "  WARNING: need at least 3 listings, found ${#listings[@]}" >&2
    return
  fi

  # Reuse listings 1 and 2 (0-indexed) since REQUESTED bookings on them were
  # already created above and won't conflict with different-status bookings.
  local dispute_active_listing="${listings[0]}"
  local dispute_status_listing="${listings[1]}"
  local rating_listing="${listings[2]}"

  gen_ulid() {
    python3 -c "import time,random; t=int(time.time()*1000); chars='0123456789ABCDEFGHJKMNPQRSTVWXYZ'; enc=''.join(chars[(t>>(45-5*i))&31] for i in range(10)); rand=''.join(random.choices(chars,k=16)); print(enc+rand)"
  }

  # ── ACTIVE booking for file-dispute flow ──────────────────────────────────
  # The dispute flow navigates: Rentals → tap ACTIVE row → active-rental →
  # "Report an issue" → dispute screen. This booking must be ACTIVE.
  # Use scheduled_start NOW so it sorts as a recent active rental.
  local dispute_active_txn
  dispute_active_txn=$(gen_ulid)
  local dp_proof_hci dp_proof_rci dp_proof_hco
  dp_proof_hci=$(gen_ulid)
  dp_proof_rci=$(gen_ulid)
  dp_proof_hco=$(gen_ulid)

  run_sql "INSERT INTO transactions (id, renter_id, host_id, listing_id, rental_fee, hold_amount, item_value, guarantee_gap, scheduled_start, scheduled_end, actual_start, status, created_at)
    VALUES ('${dispute_active_txn}', '${bob_id}', '${alice_id}', '${dispute_active_listing}', 4000, 8000, 16000, 0,
      NOW() + INTERVAL '1 second', NOW() + INTERVAL '24 hours', NOW() + INTERVAL '1 second', 'ACTIVE', NOW() + INTERVAL '1 second')
    ON CONFLICT DO NOTHING;" \
    && echo "  Created ACTIVE booking for dispute: ${dispute_active_txn}" \
    || echo "  WARNING: could not create dispute ACTIVE booking"

  # Host CHECK_IN + renter CHECK_IN + host CHECK_OUT proximity proofs (all verified)
  run_sql "INSERT INTO proximity_proofs (id, transaction_id, user_id, proof_type, gps_distance, verified, method, created_at) VALUES
    ('${dp_proof_hci}', '${dispute_active_txn}', '${alice_id}', 'CHECK_IN', 10.0, true, 'GPS', NOW()),
    ('${dp_proof_rci}', '${dispute_active_txn}', '${bob_id}', 'CHECK_IN', 12.0, true, 'GPS', NOW()),
    ('${dp_proof_hco}', '${dispute_active_txn}', '${alice_id}', 'CHECK_OUT', 8.0, true, 'GPS', NOW())
    ON CONFLICT DO NOTHING;" \
    && echo "  Created dispute ACTIVE proofs" \
    || echo "  WARNING: could not create dispute ACTIVE proofs"

  # ── DISPUTED booking for view-dispute-status flow ─────────────────────────
  # A COMPLETED transaction with a PENDING dispute, status set to DISPUTED.
  local dispute_status_txn
  dispute_status_txn=$(gen_ulid)
  local ds_dispute_id
  ds_dispute_id=$(gen_ulid)
  local ds_proof_hci ds_proof_rci ds_proof_hco ds_proof_rco
  ds_proof_hci=$(gen_ulid)
  ds_proof_rci=$(gen_ulid)
  ds_proof_hco=$(gen_ulid)
  ds_proof_rco=$(gen_ulid)

  run_sql "INSERT INTO transactions (id, renter_id, host_id, listing_id, rental_fee, hold_amount, item_value, guarantee_gap, scheduled_start, scheduled_end, actual_start, actual_end, status, created_at)
    VALUES ('${dispute_status_txn}', '${bob_id}', '${alice_id}', '${dispute_status_listing}', 5000, 10000, 20000, 0,
      NOW() - INTERVAL '48 hours', NOW() - INTERVAL '44 hours', NOW() - INTERVAL '48 hours', NOW() - INTERVAL '44 hours', 'DISPUTED', NOW() - INTERVAL '49 hours')
    ON CONFLICT DO NOTHING;" \
    && echo "  Created DISPUTED booking ${dispute_status_txn}" \
    || echo "  WARNING: could not create DISPUTED booking"

  # All 4 proximity proofs (verified)
  run_sql "INSERT INTO proximity_proofs (id, transaction_id, user_id, proof_type, gps_distance, verified, method, created_at) VALUES
    ('${ds_proof_hci}', '${dispute_status_txn}', '${alice_id}', 'CHECK_IN', 5.0, true, 'GPS', NOW() - INTERVAL '48 hours'),
    ('${ds_proof_rci}', '${dispute_status_txn}', '${bob_id}', 'CHECK_IN', 7.0, true, 'GPS', NOW() - INTERVAL '48 hours'),
    ('${ds_proof_hco}', '${dispute_status_txn}', '${alice_id}', 'CHECK_OUT', 6.0, true, 'GPS', NOW() - INTERVAL '44 hours'),
    ('${ds_proof_rco}', '${dispute_status_txn}', '${bob_id}', 'CHECK_OUT', 9.0, true, 'GPS', NOW() - INTERVAL '44 hours')
    ON CONFLICT DO NOTHING;" \
    || echo "  WARNING: could not create DISPUTED proofs"

  # Insert PENDING dispute record
  run_sql "INSERT INTO disputes (id, transaction_id, reporter_id, reason, description, status, created_at)
    VALUES ('${ds_dispute_id}', '${dispute_status_txn}', '${bob_id}', 'DAMAGE', 'The item had a visible dent on the side panel when returned.', 'PENDING', NOW() - INTERVAL '1 hour')
    ON CONFLICT DO NOTHING;" \
    && echo "  Created PENDING dispute ${ds_dispute_id}" \
    || echo "  WARNING: could not create dispute record"

  # ── COMPLETED booking for rating flow ─────────────────────────────────────
  # A fully completed rental that bob can rate (no existing ratings).
  local rating_txn
  rating_txn=$(gen_ulid)
  local rt_proof_hci rt_proof_rci rt_proof_hco rt_proof_rco
  rt_proof_hci=$(gen_ulid)
  rt_proof_rci=$(gen_ulid)
  rt_proof_hco=$(gen_ulid)
  rt_proof_rco=$(gen_ulid)

  run_sql "INSERT INTO transactions (id, renter_id, host_id, listing_id, rental_fee, hold_amount, item_value, guarantee_gap, scheduled_start, scheduled_end, actual_start, actual_end, status, created_at)
    VALUES ('${rating_txn}', '${bob_id}', '${alice_id}', '${rating_listing}', 3000, 6000, 12000, 0,
      NOW() - INTERVAL '36 hours', NOW() - INTERVAL '32 hours', NOW() - INTERVAL '36 hours', NOW() - INTERVAL '32 hours', 'COMPLETED', NOW() - INTERVAL '37 hours')
    ON CONFLICT DO NOTHING;" \
    && echo "  Created COMPLETED booking for rating: ${rating_txn}" \
    || echo "  WARNING: could not create rating COMPLETED booking"

  # All 4 proximity proofs (verified)
  run_sql "INSERT INTO proximity_proofs (id, transaction_id, user_id, proof_type, gps_distance, verified, method, created_at) VALUES
    ('${rt_proof_hci}', '${rating_txn}', '${alice_id}', 'CHECK_IN', 5.0, true, 'GPS', NOW() - INTERVAL '36 hours'),
    ('${rt_proof_rci}', '${rating_txn}', '${bob_id}', 'CHECK_IN', 7.0, true, 'GPS', NOW() - INTERVAL '36 hours'),
    ('${rt_proof_hco}', '${rating_txn}', '${alice_id}', 'CHECK_OUT', 6.0, true, 'GPS', NOW() - INTERVAL '32 hours'),
    ('${rt_proof_rco}', '${rating_txn}', '${bob_id}', 'CHECK_OUT', 9.0, true, 'GPS', NOW() - INTERVAL '32 hours')
    ON CONFLICT DO NOTHING;" \
    && echo "  Created rating COMPLETED proofs" \
    || echo "  WARNING: could not create rating proofs"

  echo "  Dispute/rating bookings seeded: ACTIVE=${dispute_active_txn}, DISPUTED=${dispute_status_txn}, COMPLETED(rating)=${rating_txn}"
}

seed_dispute_rating_bookings

echo "=== Verifying seed data ==="
ACTIVE_COUNT=$(run_sql "SELECT count(*) FROM listings WHERE host_id = (SELECT id FROM users WHERE email = 'alice@test.com') AND status = 'ACTIVE';" 2>/dev/null | tr -d ' ' || echo "?")
echo "  Active listings for alice: ${ACTIVE_COUNT}"
BOOKING_COUNT=$(run_sql "SELECT count(*) FROM transactions WHERE renter_id = (SELECT id FROM users WHERE email = 'bob@test.com') AND status = 'REQUESTED';" 2>/dev/null | tr -d ' ' || echo "?")
echo "  REQUESTED bookings for bob: ${BOOKING_COUNT}"
ACCEPTED_COUNT=$(run_sql "SELECT count(*) FROM transactions WHERE renter_id = (SELECT id FROM users WHERE email = 'bob@test.com') AND status = 'ACCEPTED';" 2>/dev/null | tr -d ' ' || echo "?")
echo "  ACCEPTED bookings for bob: ${ACCEPTED_COUNT}"
ACTIVE_BOOKING_COUNT=$(run_sql "SELECT count(*) FROM transactions WHERE renter_id = (SELECT id FROM users WHERE email = 'bob@test.com') AND status = 'ACTIVE';" 2>/dev/null | tr -d ' ' || echo "?")
echo "  ACTIVE bookings for bob: ${ACTIVE_BOOKING_COUNT}"
COMPLETED_COUNT=$(run_sql "SELECT count(*) FROM transactions WHERE renter_id = (SELECT id FROM users WHERE email = 'bob@test.com') AND status = 'COMPLETED';" 2>/dev/null | tr -d ' ' || echo "?")
echo "  COMPLETED bookings for bob: ${COMPLETED_COUNT}"
DISPUTED_COUNT=$(run_sql "SELECT count(*) FROM transactions WHERE renter_id = (SELECT id FROM users WHERE email = 'bob@test.com') AND status = 'DISPUTED';" 2>/dev/null | tr -d ' ' || echo "?")
echo "  DISPUTED bookings for bob: ${DISPUTED_COUNT}"
DISPUTE_RECORD_COUNT=$(run_sql "SELECT count(*) FROM disputes d JOIN transactions t ON d.transaction_id = t.id WHERE t.renter_id = (SELECT id FROM users WHERE email = 'bob@test.com');" 2>/dev/null | tr -d ' ' || echo "?")
echo "  Dispute records for bob: ${DISPUTE_RECORD_COUNT}"
RATING_COUNT=$(run_sql "SELECT count(*) FROM ratings WHERE from_user_id = (SELECT id FROM users WHERE email = 'bob@test.com');" 2>/dev/null | tr -d ' ' || echo "?")
echo "  Ratings from bob: ${RATING_COUNT}"
echo "Done."
