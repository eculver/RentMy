package agreement

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// Repository handles Postgres persistence for the agreement domain.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Insert persists a new Agreement row.
func (r *Repository) Insert(ctx context.Context, a *Agreement) (*Agreement, error) {
	const q = `
		INSERT INTO agreements
			(id, transaction_id, version, full_agreement, custom_clauses,
			 prompt_version, model, guardrail_warnings, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW())
		RETURNING created_at`

	err := r.pool.QueryRow(ctx, q,
		a.ID, a.TransactionID, a.Version,
		[]byte(a.FullAgreement), []byte(a.CustomClauses),
		a.PromptVersion, a.Model, []byte(a.GuardrailWarnings),
	).Scan(&a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("agreement: insert: %w", err)
	}
	return a, nil
}

// FindByTransactionID retrieves the agreement for a given transaction.
// Returns ErrAgreementNotFound when no record exists.
func (r *Repository) FindByTransactionID(ctx context.Context, transactionID string) (*Agreement, error) {
	const q = `
		SELECT id, transaction_id, version, full_agreement, custom_clauses,
		       prompt_version, model, guardrail_warnings, created_at
		FROM agreements
		WHERE transaction_id = $1`

	a, err := scanAgreement(r.pool.QueryRow(ctx, q, transactionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAgreementNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("agreement: find by transaction: %w", err)
	}
	return a, nil
}

// FindByID retrieves an agreement by its primary key.
func (r *Repository) FindByID(ctx context.Context, id string) (*Agreement, error) {
	const q = `
		SELECT id, transaction_id, version, full_agreement, custom_clauses,
		       prompt_version, model, guardrail_warnings, created_at
		FROM agreements
		WHERE id = $1`

	a, err := scanAgreement(r.pool.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAgreementNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("agreement: find by id: %w", err)
	}
	return a, nil
}

// InsertAcceptance records a user's acceptance of an agreement.
// Returns ErrAlreadyAccepted if the user has already accepted.
func (r *Repository) InsertAcceptance(ctx context.Context, agreementID, userID string, ipAddress, deviceID *string) (*Acceptance, error) {
	id := ulid.New()
	const q = `
		INSERT INTO agreement_acceptances
			(id, agreement_id, user_id, ip_address, device_id, accepted_at)
		VALUES ($1,$2,$3,$4,$5,NOW())
		ON CONFLICT (agreement_id, user_id) DO NOTHING
		RETURNING id, accepted_at`

	var acc Acceptance
	acc.AgreementID = agreementID
	acc.UserID = userID
	acc.IPAddress = ipAddress
	acc.DeviceID = deviceID

	err := r.pool.QueryRow(ctx, q, id, agreementID, userID, ipAddress, deviceID).
		Scan(&acc.ID, &acc.AcceptedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// ON CONFLICT DO NOTHING produced no row — already accepted.
		return nil, ErrAlreadyAccepted
	}
	if err != nil {
		return nil, fmt.Errorf("agreement: insert acceptance: %w", err)
	}
	return &acc, nil
}

// FindAcceptances returns all acceptances for the given agreement.
func (r *Repository) FindAcceptances(ctx context.Context, agreementID string) ([]*Acceptance, error) {
	const q = `
		SELECT id, agreement_id, user_id, accepted_at, ip_address, device_id
		FROM agreement_acceptances
		WHERE agreement_id = $1`

	rows, err := r.pool.Query(ctx, q, agreementID)
	if err != nil {
		return nil, fmt.Errorf("agreement: find acceptances: %w", err)
	}
	defer rows.Close()

	var accs []*Acceptance
	for rows.Next() {
		var acc Acceptance
		if err := rows.Scan(&acc.ID, &acc.AgreementID, &acc.UserID,
			&acc.AcceptedAt, &acc.IPAddress, &acc.DeviceID); err != nil {
			return nil, fmt.Errorf("agreement: scan acceptance: %w", err)
		}
		accs = append(accs, &acc)
	}
	return accs, rows.Err()
}

// UpdateAgreementSnapshot writes the full agreement JSON to the
// agreement_snapshot JSONB column on the transactions row.
func (r *Repository) UpdateAgreementSnapshot(ctx context.Context, transactionID string, snapshot []byte) error {
	const q = `UPDATE transactions SET agreement_snapshot = $1 WHERE id = $2`
	tag, err := r.pool.Exec(ctx, q, snapshot, transactionID)
	if err != nil {
		return fmt.Errorf("agreement: update snapshot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agreement: transaction %s not found", transactionID)
	}
	return nil
}

// GetTransactionParties returns the renter_id and host_id for a transaction.
func (r *Repository) GetTransactionParties(ctx context.Context, transactionID string) (renterID, hostID string, err error) {
	const q = `
		SELECT t.renter_id, l.host_id
		FROM transactions t
		JOIN listings l ON l.id = t.listing_id
		WHERE t.id = $1`

	if err = r.pool.QueryRow(ctx, q, transactionID).Scan(&renterID, &hostID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", fmt.Errorf("agreement: transaction %s not found", transactionID)
		}
		return "", "", fmt.Errorf("agreement: get transaction parties: %w", err)
	}
	return renterID, hostID, nil
}

// GetListingForTransaction returns listing details needed to generate the agreement.
func (r *Repository) GetListingForTransaction(ctx context.Context, transactionID string) (listingID, title, description string, estimatedValueCents int, err error) {
	const q = `
		SELECT l.id, l.title, l.description,
		       COALESCE(l.host_declared_value, l.estimated_value, 0)::bigint
		FROM transactions t
		JOIN listings l ON l.id = t.listing_id
		WHERE t.id = $1`

	if err = r.pool.QueryRow(ctx, q, transactionID).
		Scan(&listingID, &title, &description, &estimatedValueCents); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", 0, fmt.Errorf("agreement: transaction %s not found", transactionID)
		}
		return "", "", "", 0, fmt.Errorf("agreement: get listing for transaction: %w", err)
	}
	return listingID, title, description, estimatedValueCents, nil
}

// GetAppraisalCategory returns the AI-detected category for a listing (may be empty).
func (r *Repository) GetAppraisalCategory(ctx context.Context, listingID string) (string, error) {
	const q = `SELECT COALESCE(category, '') FROM appraisals WHERE listing_id = $1 LIMIT 1`
	var category string
	if err := r.pool.QueryRow(ctx, q, listingID).Scan(&category); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("agreement: get appraisal category: %w", err)
	}
	return category, nil
}

// GetLastAcceptedAt returns the acceptance timestamp for a specific user+agreement pair.
func (r *Repository) GetLastAcceptedAt(ctx context.Context, agreementID, userID string) (*time.Time, error) {
	const q = `SELECT accepted_at FROM agreement_acceptances WHERE agreement_id=$1 AND user_id=$2`
	var t time.Time
	if err := r.pool.QueryRow(ctx, q, agreementID, userID).Scan(&t); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("agreement: get acceptance: %w", err)
	}
	return &t, nil
}

// scanAgreement reads an Agreement from a single-row scanner.
func scanAgreement(row pgx.Row) (*Agreement, error) {
	var a Agreement
	var fullAgreement, customClauses, guardrailWarnings []byte
	if err := row.Scan(
		&a.ID, &a.TransactionID, &a.Version,
		&fullAgreement, &customClauses,
		&a.PromptVersion, &a.Model, &guardrailWarnings,
		&a.CreatedAt,
	); err != nil {
		return nil, err
	}
	a.FullAgreement = fullAgreement
	a.CustomClauses = customClauses
	a.GuardrailWarnings = guardrailWarnings
	return &a, nil
}
