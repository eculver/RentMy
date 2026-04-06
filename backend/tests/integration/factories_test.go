package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/giits/rentmy/backend/internal/booking"
	"github.com/giits/rentmy/backend/internal/listing"
	"github.com/giits/rentmy/backend/internal/platform/ulid"
	"github.com/giits/rentmy/backend/internal/user"
)

// CreateTestUser inserts a user with a bcrypt-hashed password directly into the
// database and returns the created User model.  The returned User's email and
// phone pointers are set so callers can read them back.
func CreateTestUser(t *testing.T, pool *pgxpool.Pool) *user.User {
	t.Helper()
	ctx := context.Background()

	email := fmt.Sprintf("user-%s@test.example", ulid.New())
	name := "Test User"
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	now := time.Now().UTC()
	id := ulid.New()

	const q = `
		INSERT INTO users (id, email, phone, password_hash, name, identity_status,
		                   reputation_score, notification_preferences, created_at, last_active_at)
		VALUES ($1, $2, NULL, $3, $4, 'PENDING', 0, '{}', $5, $5)
		RETURNING id, email, phone, name, avatar_url, identity_status, reputation_score,
		          notification_preferences, created_at, last_active_at`

	row := pool.QueryRow(ctx, q, id, email, string(hash), name, now)

	u := &user.User{}
	var notifPrefs []byte
	if err := row.Scan(
		&u.ID, &u.Email, &u.Phone, &u.Name, &u.AvatarURL,
		&u.IdentityStatus, &u.ReputationScore, &notifPrefs,
		&u.CreatedAt, &u.LastActiveAt,
	); err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	u.NotificationPreferences = json.RawMessage(notifPrefs)
	return u
}

// CreateTestListing inserts a listing with a default location (San Francisco) and
// returns the created Listing model.
func CreateTestListing(t *testing.T, pool *pgxpool.Pool, ownerID string) *listing.Listing {
	t.Helper()
	ctx := context.Background()

	id := ulid.New()
	pricePerDay := 25.0
	lat := 37.7749
	lng := -122.4194
	minDuration := pgtype.Interval{Microseconds: time.Hour.Microseconds(), Valid: true}
	maxDuration := pgtype.Interval{Microseconds: (24 * time.Hour).Microseconds(), Valid: true}

	const q = `
		INSERT INTO listings (
			id, host_id, title, description,
			price_per_day, min_duration, max_duration,
			location, availability, status, created_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			ST_SetSRID(ST_MakePoint($9::float8, $8::float8), 4326)::geography,
			'{}', 'ACTIVE', NOW()
		)
		RETURNING id, host_id, title, description,
		          ai_generated_tags, estimated_value, host_declared_value, value_justification,
		          price_per_hour, price_per_day,
		          min_duration, max_duration,
		          ST_Y(location::geometry) AS loc_lat, ST_X(location::geometry) AS loc_lng,
		          availability, has_video, status, appraisal_status, created_at`

	row := pool.QueryRow(ctx, q,
		id, ownerID, "Test Listing "+id[:8], "A listing for integration tests",
		pricePerDay, minDuration, maxDuration,
		lat, lng,
	)

	l := &listing.Listing{}
	var (
		aiTags       []byte
		loc          listing.Location
		availability []byte
		minDurIV     pgtype.Interval
		maxDurIV     pgtype.Interval
	)
	if err := row.Scan(
		&l.ID, &l.HostID, &l.Title, &l.Description,
		&aiTags, &l.EstimatedValue, &l.HostDeclaredValue, &l.ValueJustification,
		&l.PricePerHour, &l.PricePerDay,
		&minDurIV, &maxDurIV,
		&loc.Lat, &loc.Lng,
		&availability, &l.HasVideo, &l.Status, &l.AppraisalStatus, &l.CreatedAt,
	); err != nil {
		t.Fatalf("insert test listing: %v", err)
	}
	if aiTags != nil {
		l.AIGeneratedTags = json.RawMessage(aiTags)
	}
	if availability != nil {
		l.Availability = json.RawMessage(availability)
	}
	l.Location = &loc
	if minDurIV.Valid {
		d := listing.Duration(time.Duration(minDurIV.Microseconds) * time.Microsecond)
		l.MinDuration = &d
	}
	if maxDurIV.Valid {
		d := listing.Duration(time.Duration(maxDurIV.Microseconds) * time.Microsecond)
		l.MaxDuration = &d
	}
	return l
}

// CreateTestBooking inserts a booking (transaction) in REQUESTED state and
// returns the booking ID.  Financial fields are zeroed.
func CreateTestBooking(t *testing.T, pool *pgxpool.Pool, renterID, listingID string) *booking.Booking {
	t.Helper()
	ctx := context.Background()

	id := ulid.New()
	start := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Hour)
	end := start.Add(4 * time.Hour)

	// Resolve host_id from the listing.
	var hostID string
	if err := pool.QueryRow(ctx, `SELECT host_id FROM listings WHERE id = $1`, listingID).Scan(&hostID); err != nil {
		t.Fatalf("get listing host: %v", err)
	}

	const q = `
		INSERT INTO transactions (id, renter_id, host_id, listing_id,
		                          rental_fee, hold_amount, item_value, guarantee_gap,
		                          scheduled_start, scheduled_end, status, created_at)
		VALUES ($1, $2, $3, $4,
		        0, 0, 0, 0,
		        $5, $6, 'REQUESTED', NOW())
		RETURNING id, renter_id, host_id, listing_id,
		          scheduled_start, scheduled_end, status, created_at`

	row := pool.QueryRow(ctx, q, id, renterID, hostID, listingID, start, end)

	b := &booking.Booking{}
	if err := row.Scan(
		&b.ID, &b.RenterID, &b.HostID, &b.ListingID,
		&b.ScheduledStart, &b.ScheduledEnd, &b.Status, &b.CreatedAt,
	); err != nil {
		t.Fatalf("insert test booking: %v", err)
	}
	return b
}

// LoginTestUser calls POST /api/v1/auth/login with email "user-<ulid>@test.example"
// and password "password123" for the given user. It returns the JWT access token.
// The user must have been created with CreateTestUser (which uses password "password123").
func LoginTestUser(t *testing.T, client *http.Client, baseURL string, email, password string) string {
	t.Helper()

	resp := DoJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, "")

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("login returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string `json:"accessToken"`
	}
	MustDecodeJSON(t, resp, &result)

	if result.AccessToken == "" {
		t.Fatal("login response missing accessToken")
	}
	return result.AccessToken
}

// AuthHeader returns a map containing the Authorization Bearer header.
func AuthHeader(token string) http.Header {
	h := make(http.Header)
	h.Set("Authorization", "Bearer "+token)
	return h
}

// readBody reads and returns the response body as a string without closing it.
func readBody(resp *http.Response) (string, error) {
	if resp == nil || resp.Body == nil {
		return "", nil
	}
	var buf []byte
	buf = make([]byte, 0, 512)
	tmp := make([]byte, 512)
	for {
		n, err := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return string(buf), nil
}
