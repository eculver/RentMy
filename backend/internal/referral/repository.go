package referral

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrCodeNotFound is returned when a referral code lookup finds no row.
var ErrCodeNotFound = errors.New("referral code not found")

// ErrAlreadyReferred is returned when a user has already been referred.
var ErrAlreadyReferred = errors.New("user has already been referred")

// Repository handles persistence for the referral domain.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository backed by pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// InsertReferralCode persists a new referral code.
func (r *Repository) InsertReferralCode(ctx context.Context, rc *ReferralCode) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO referral_codes (id, code, user_id, expires_at, max_uses, use_count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		rc.ID, rc.Code, rc.UserID, rc.ExpiresAt, rc.MaxUses, rc.UseCount, rc.CreatedAt,
	)
	return err
}

// FindReferralCodeByCode returns the code record for a given code string.
func (r *Repository) FindReferralCodeByCode(ctx context.Context, code string) (*ReferralCode, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, code, user_id, expires_at, max_uses, use_count, created_at
		FROM referral_codes WHERE code = $1`, code)
	rc, err := scanReferralCode(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCodeNotFound
	}
	return rc, err
}

// FindReferralCodeByUser returns the code record owned by a user.
func (r *Repository) FindReferralCodeByUser(ctx context.Context, userID string) (*ReferralCode, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, code, user_id, expires_at, max_uses, use_count, created_at
		FROM referral_codes WHERE user_id = $1`, userID)
	rc, err := scanReferralCode(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCodeNotFound
	}
	return rc, err
}

// IncrementCodeUseCount atomically bumps the use counter for a code.
func (r *Repository) IncrementCodeUseCount(ctx context.Context, codeID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE referral_codes SET use_count = use_count + 1 WHERE id = $1`, codeID)
	return err
}

// InsertReferral persists a new referral record.
func (r *Repository) InsertReferral(ctx context.Context, ref *Referral) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO referrals (id, referral_code_id, referrer_id, referee_id, status,
		                       referrer_payout, referee_payout, completed_at, paid_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		ref.ID, ref.ReferralCodeID, ref.ReferrerID, ref.RefereeID, ref.Status,
		ref.ReferrerPayout, ref.RefereePayout, ref.CompletedAt, ref.PaidAt, ref.CreatedAt,
	)
	return err
}

// UpdateReferralStatus advances a referral to the given status.
func (r *Repository) UpdateReferralStatus(ctx context.Context, id string, status ReferralStatus) error {
	now := time.Now().UTC()
	var err error
	switch status {
	case ReferralStatusFirstRentalCompleted:
		_, err = r.pool.Exec(ctx,
			`UPDATE referrals SET status = $1, completed_at = $2 WHERE id = $3`,
			status, now, id)
	case ReferralStatusPaid:
		_, err = r.pool.Exec(ctx,
			`UPDATE referrals SET status = $1, paid_at = $2 WHERE id = $3`,
			status, now, id)
	default:
		_, err = r.pool.Exec(ctx,
			`UPDATE referrals SET status = $1 WHERE id = $2`, status, id)
	}
	return err
}

// FindReferralByID retrieves a referral by its primary key.
func (r *Repository) FindReferralByID(ctx context.Context, id string) (*Referral, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, referral_code_id, referrer_id, referee_id, status,
		       referrer_payout, referee_payout, completed_at, paid_at, created_at
		FROM referrals WHERE id = $1`, id)
	ref, err := scanReferral(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("referral %s not found", id)
	}
	return ref, err
}

// FindReferralByReferee returns the referral record where the given user is the referee.
func (r *Repository) FindReferralByReferee(ctx context.Context, userID string) (*Referral, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, referral_code_id, referrer_id, referee_id, status,
		       referrer_payout, referee_payout, completed_at, paid_at, created_at
		FROM referrals WHERE referee_id = $1`, userID)
	ref, err := scanReferral(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // not referred — not an error
	}
	return ref, err
}

// ListReferralsByReferrer returns paginated referrals where the given user is the referrer.
func (r *Repository) ListReferralsByReferrer(ctx context.Context, userID string, page, limit int) ([]*Referral, error) {
	offset := (page - 1) * limit
	rows, err := r.pool.Query(ctx, `
		SELECT id, referral_code_id, referrer_id, referee_id, status,
		       referrer_payout, referee_payout, completed_at, paid_at, created_at
		FROM referrals WHERE referrer_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectReferrals(rows)
}

// CountRecentPayouts returns the number of PAID referral payouts for a user in the last 30 days.
func (r *Repository) CountRecentPayouts(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM referral_payouts
		WHERE user_id = $1
		  AND status = 'PAID'
		  AND created_at > NOW() - INTERVAL '30 days'`, userID).Scan(&count)
	return count, err
}

// InsertReferralPayout persists a new payout record.
func (r *Repository) InsertReferralPayout(ctx context.Context, p *ReferralPayout) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO referral_payouts (id, referral_id, user_id, amount, status, stripe_transfer_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		p.ID, p.ReferralID, p.UserID, p.Amount, p.Status, p.StripeTransferID, p.CreatedAt,
	)
	return err
}

// UpdatePayoutStatus sets the status (and optionally the Stripe transfer ID) on a payout.
func (r *Repository) UpdatePayoutStatus(ctx context.Context, payoutID string, status PayoutStatus, stripeID *string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE referral_payouts SET status = $1, stripe_transfer_id = COALESCE($2, stripe_transfer_id) WHERE id = $3`,
		status, stripeID, payoutID)
	return err
}

// FindPayoutByID retrieves a single payout record.
func (r *Repository) FindPayoutByID(ctx context.Context, id string) (*ReferralPayout, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, referral_id, user_id, amount, status, stripe_transfer_id, created_at
		FROM referral_payouts WHERE id = $1`, id)
	return scanPayout(row)
}

// ListReferralPayouts returns all payouts for a referral (used for deduplication checks).
func (r *Repository) ListReferralPayouts(ctx context.Context, referralID string) ([]*ReferralPayout, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, referral_id, user_id, amount, status, stripe_transfer_id, created_at
		FROM referral_payouts WHERE referral_id = $1 ORDER BY created_at`, referralID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectPayouts(rows)
}

// ListAllReferralsPaginated returns a paginated list of all referrals for the ops dashboard.
func (r *Repository) ListAllReferralsPaginated(ctx context.Context, f ListReferralsFilter) ([]*Referral, error) {
	offset := (f.Page - 1) * f.Limit
	rows, err := r.pool.Query(ctx, `
		SELECT id, referral_code_id, referrer_id, referee_id, status,
		       referrer_payout, referee_payout, completed_at, paid_at, created_at
		FROM referrals ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		f.Limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectReferrals(rows)
}

// GetStats returns aggregate referral statistics for the ops dashboard.
func (r *Repository) GetStats(ctx context.Context) (*ReferralStats, error) {
	var s ReferralStats
	err := r.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*)                                                          AS total,
		  COUNT(*) FILTER (WHERE status = 'SIGNED_UP')                     AS signed_up,
		  COUNT(*) FILTER (WHERE status IN ('FIRST_RENTAL_COMPLETED','PAID')) AS converted,
		  COUNT(*) FILTER (WHERE status = 'PAID')                          AS paid,
		  COUNT(*) FILTER (WHERE status = 'FRAUDULENT')                    AS fraudulent,
		  COALESCE(SUM(referrer_payout + referee_payout) FILTER (WHERE status = 'PAID'), 0) AS total_payout
		FROM referrals`).Scan(
		&s.Total, &s.SignedUp, &s.Converted, &s.Paid, &s.Fraudulent, &s.TotalPayoutCents,
	)
	if err != nil {
		return nil, fmt.Errorf("referral stats query: %w", err)
	}
	if s.Total > 0 {
		s.ConversionRate = float64(s.Converted) / float64(s.Total)
	}
	return &s, nil
}

// SharedDeviceFingerprint returns true if userA and userB share the same device_fingerprint.
func (r *Repository) SharedDeviceFingerprint(ctx context.Context, userA, userB string) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users a
		JOIN users b ON a.device_fingerprint = b.device_fingerprint
		WHERE a.id = $1 AND b.id = $2
		  AND a.device_fingerprint IS NOT NULL`,
		userA, userB).Scan(&count)
	return count > 0, err
}

// SharedWiFiBSSID returns true if userA and userB share the same wifi_bssid signup metadata.
func (r *Repository) SharedWiFiBSSID(ctx context.Context, userA, userB string) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users a
		JOIN users b
		  ON a.signup_metadata->>'wifi_bssid' = b.signup_metadata->>'wifi_bssid'
		WHERE a.id = $1 AND b.id = $2
		  AND a.signup_metadata->>'wifi_bssid' IS NOT NULL
		  AND a.signup_metadata->>'wifi_bssid' != ''`,
		userA, userB).Scan(&count)
	return count > 0, err
}

// --- scan helpers ---

type rowScanner interface {
	Scan(dest ...any) error
}

func scanReferralCode(row rowScanner) (*ReferralCode, error) {
	var rc ReferralCode
	err := row.Scan(&rc.ID, &rc.Code, &rc.UserID, &rc.ExpiresAt, &rc.MaxUses, &rc.UseCount, &rc.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &rc, nil
}

func scanReferral(row rowScanner) (*Referral, error) {
	var ref Referral
	err := row.Scan(
		&ref.ID, &ref.ReferralCodeID, &ref.ReferrerID, &ref.RefereeID, &ref.Status,
		&ref.ReferrerPayout, &ref.RefereePayout, &ref.CompletedAt, &ref.PaidAt, &ref.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ref, nil
}

func scanPayout(row rowScanner) (*ReferralPayout, error) {
	var p ReferralPayout
	err := row.Scan(&p.ID, &p.ReferralID, &p.UserID, &p.Amount, &p.Status, &p.StripeTransferID, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func collectReferrals(rows pgx.Rows) ([]*Referral, error) {
	var out []*Referral
	for rows.Next() {
		ref, err := scanReferral(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

func collectPayouts(rows pgx.Rows) ([]*ReferralPayout, error) {
	var out []*ReferralPayout
	for rows.Next() {
		p, err := scanPayout(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
