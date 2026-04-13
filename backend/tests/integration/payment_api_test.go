package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// TestHoldEstimateListingNotFound verifies GET /listings/:id/hold-estimate returns 404
// for a non-existent listing.
func TestHoldEstimateListingNotFound(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/listings/01HNOTEXISTENT00000000000X/hold-estimate", nil, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// TestHoldEstimateTiers verifies that the tiered hold calculation is correct for
// listings with different declared values. The hold-estimate endpoint is public (no auth).
//
// Tiers (PRD §7):
//   - itemValue ≤ $500 (50 000 cents)  → hold = 100% of itemValue
//   - $501–$2000                        → $500 + 25% of (value − $500)
//   - $2001–$5000                       → $875 + 15% of (value − $2000)
//   - $5001+                            → $1,325 hard ceiling
func TestHoldEstimateTiers(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	cases := []struct {
		name              string
		hostDeclaredValue float64 // dollars
		wantHoldCents     int64
		wantGapCents      int64
	}{
		{
			name:              "tier1 $200 item",
			hostDeclaredValue: 200.00,        // 20 000 cents
			wantHoldCents:     20_000,        // 100% hold
			wantGapCents:      0,
		},
		{
			name:              "tier1 boundary $500 item",
			hostDeclaredValue: 500.00,        // 50 000 cents
			wantHoldCents:     50_000,        // 100% hold at the boundary
			wantGapCents:      0,
		},
		{
			name:              "tier2 $1000 item",
			hostDeclaredValue: 1000.00,       // 100 000 cents
			wantHoldCents:     62_500,        // 50 000 + 25% of 50 000
			wantGapCents:      37_500,        // 100 000 − 62 500
		},
		{
			name:              "tier3 $3000 item",
			hostDeclaredValue: 3000.00,       // 300 000 cents
			wantHoldCents:     102_500,       // 87 500 + 15% of 100 000
			wantGapCents:      197_500,       // 300 000 − 102 500
		},
		{
			name:              "tier4 ceiling $10000 item",
			hostDeclaredValue: 10_000.00,     // 1 000 000 cents
			wantHoldCents:     132_500,       // hard ceiling
			wantGapCents:      867_500,       // 1 000 000 − 132 500
		},
	}

	u := CreateTestUser(t, pool)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Insert a listing with the declared value directly via SQL.
			ctx := context.Background()
			listingID := ulid.New()
			_, err := pool.Exec(ctx, `
				INSERT INTO listings (
					id, host_id, title, description,
					price_per_day, min_duration, max_duration,
					host_declared_value,
					location, availability, status, created_at
				) VALUES (
					$1, $2, $3, 'test',
					25.0, '1 hour'::interval, '24 hours'::interval,
					$4,
					ST_SetSRID(ST_MakePoint(-122.4194, 37.7749), 4326)::geography,
					'{}', 'ACTIVE', NOW()
				)`,
				listingID,
				u.ID,
				fmt.Sprintf("Hold Test Listing %s", tc.name),
				tc.hostDeclaredValue,
			)
			if err != nil {
				t.Fatalf("insert listing: %v", err)
			}

			resp := DoJSON(t, client, http.MethodGet,
				ts.URL+"/api/v1/listings/"+listingID+"/hold-estimate", nil, "")
			defer DrainBody(resp)

			if resp.StatusCode != http.StatusOK {
				body, _ := readBody(resp)
				t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
			}

			var body struct {
				ItemValue    int64 `json:"itemValue"`
				HoldAmount   int64 `json:"holdAmount"`
				GuaranteeGap int64 `json:"guaranteeGap"`
			}
			MustDecodeJSON(t, resp, &body)

			if body.HoldAmount != tc.wantHoldCents {
				t.Errorf("holdAmount = %d, want %d", body.HoldAmount, tc.wantHoldCents)
			}
			if body.GuaranteeGap != tc.wantGapCents {
				t.Errorf("guaranteeGap = %d, want %d", body.GuaranteeGap, tc.wantGapCents)
			}
		})
	}
}

// TestHoldEstimatePublic verifies that the hold-estimate endpoint does not require auth.
func TestHoldEstimatePublic(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, u.ID)

	// No token — endpoint should still return 200.
	resp := DoJSON(t, client, http.MethodGet,
		ts.URL+"/api/v1/listings/"+l.ID+"/hold-estimate", nil, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("expected 200 without auth, got %d: %s", resp.StatusCode, body)
	}
}
