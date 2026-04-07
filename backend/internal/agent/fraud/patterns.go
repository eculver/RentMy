package fraud

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// DetectExclusivePairs finds user pairs where >80% of their transactions are
// with each other AND they have >3 transactions total.  Score: +45.
func DetectExclusivePairs(ctx context.Context, repo *Repository) ([]FraudSignal, error) {
	const q = `
		WITH pairs AS (
			SELECT
				renter_id AS user_a,
				host_id   AS user_b,
				COUNT(*)  AS shared_count
			FROM transactions
			GROUP BY renter_id, host_id
			HAVING COUNT(*) > 3
		),
		user_totals AS (
			SELECT user_id, COUNT(*) AS total
			FROM (
				SELECT renter_id AS user_id FROM transactions
				UNION ALL
				SELECT host_id   AS user_id FROM transactions
			) t
			GROUP BY user_id
		)
		SELECT p.user_a, p.user_b, p.shared_count
		FROM pairs p
		JOIN user_totals ta ON ta.user_id = p.user_a
		JOIN user_totals tb ON tb.user_id = p.user_b
		WHERE
			p.shared_count::float / NULLIF(ta.total, 0) > 0.8
			AND p.shared_count::float / NULLIF(tb.total, 0) > 0.8`

	rows, err := repo.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("fraud: exclusive pairs query: %w", err)
	}
	defer rows.Close()

	var signals []FraudSignal
	now := time.Now().UTC()
	for rows.Next() {
		var userA, userB string
		var sharedCount int
		if err := rows.Scan(&userA, &userB, &sharedCount); err != nil {
			return nil, fmt.Errorf("fraud: exclusive pairs scan: %w", err)
		}
		ev, _ := json.Marshal(map[string]any{
			"related_user": userB, "shared_transactions": sharedCount,
		})
		signals = append(signals, FraudSignal{
			Type:          SignalExclusivePair,
			UserID:        userA,
			RelatedUserID: userB,
			Score:         45,
			Evidence:      ev,
			DetectedAt:    now,
		})
	}
	return signals, rows.Err()
}

// DetectDamageAmountGaming finds hosts whose dispute charge amounts are within
// 5% of the transaction hold amount on >50% of their disputes.  Score: +50.
func DetectDamageAmountGaming(ctx context.Context, repo *Repository) ([]FraudSignal, error) {
	const q = `
		WITH damage_disputes AS (
			SELECT
				t.host_id,
				t.hold_amount,
				d.charge_amount,
				CASE
					WHEN t.hold_amount > 0
					     AND d.charge_amount > 0
					     AND ABS(d.charge_amount - t.hold_amount) / t.hold_amount <= 0.05
					THEN 1 ELSE 0
				END AS is_near_hold
			FROM disputes d
			JOIN transactions t ON t.id = d.transaction_id
			WHERE d.charge_amount > 0
			  AND t.hold_amount > 0
		),
		host_stats AS (
			SELECT
				host_id,
				COUNT(*) AS total_claims,
				SUM(is_near_hold) AS near_hold_count
			FROM damage_disputes
			GROUP BY host_id
			HAVING COUNT(*) >= 2
		)
		SELECT host_id, total_claims, near_hold_count
		FROM host_stats
		WHERE near_hold_count::float / total_claims > 0.5`

	rows, err := repo.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("fraud: damage gaming query: %w", err)
	}
	defer rows.Close()

	var signals []FraudSignal
	now := time.Now().UTC()
	for rows.Next() {
		var hostID string
		var total, nearHold int
		if err := rows.Scan(&hostID, &total, &nearHold); err != nil {
			return nil, fmt.Errorf("fraud: damage gaming scan: %w", err)
		}
		ev, _ := json.Marshal(map[string]any{
			"total_claims": total, "near_hold_count": nearHold,
		})
		signals = append(signals, FraudSignal{
			Type:       SignalDamagePattern,
			UserID:     hostID,
			Score:      50,
			Evidence:   ev,
			DetectedAt: now,
		})
	}
	return signals, rows.Err()
}

// DetectSerialDamage finds listings where >60% of completed rentals have an
// associated dispute.  The signal is attributed to the host.  Score: +40.
func DetectSerialDamage(ctx context.Context, repo *Repository) ([]FraudSignal, error) {
	const q = `
		WITH item_stats AS (
			SELECT
				t.host_id,
				t.listing_id,
				COUNT(DISTINCT t.id) AS total_rentals,
				COUNT(DISTINCT d.id) AS dispute_count
			FROM transactions t
			LEFT JOIN disputes d ON d.transaction_id = t.id
			WHERE t.status = 'COMPLETED'
			GROUP BY t.host_id, t.listing_id
			HAVING COUNT(DISTINCT t.id) >= 3
		)
		SELECT host_id, listing_id, total_rentals, dispute_count
		FROM item_stats
		WHERE dispute_count::float / total_rentals > 0.6`

	rows, err := repo.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("fraud: serial damage query: %w", err)
	}
	defer rows.Close()

	var signals []FraudSignal
	now := time.Now().UTC()
	for rows.Next() {
		var hostID, listingID string
		var total, disputeCount int
		if err := rows.Scan(&hostID, &listingID, &total, &disputeCount); err != nil {
			return nil, fmt.Errorf("fraud: serial damage scan: %w", err)
		}
		ev, _ := json.Marshal(map[string]any{
			"listing_id": listingID, "total_rentals": total, "dispute_count": disputeCount,
		})
		signals = append(signals, FraudSignal{
			Type:       SignalDamagePattern,
			UserID:     hostID,
			Score:      40,
			Evidence:   ev,
			DetectedAt: now,
		})
	}
	return signals, rows.Err()
}

// DetectNewAccountValueSpike finds accounts <30 days old that have listed more
// than 3 items with estimated_value > $500.  Score: +35.
func DetectNewAccountValueSpike(ctx context.Context, repo *Repository) ([]FraudSignal, error) {
	const q = `
		SELECT l.host_id, COUNT(*) AS high_value_count
		FROM listings l
		JOIN users u ON u.id = l.host_id
		WHERE u.created_at > NOW() - INTERVAL '30 days'
		  AND l.estimated_value > 500
		GROUP BY l.host_id
		HAVING COUNT(*) > 3`

	rows, err := repo.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("fraud: value spike query: %w", err)
	}
	defer rows.Close()

	var signals []FraudSignal
	now := time.Now().UTC()
	for rows.Next() {
		var hostID string
		var count int
		if err := rows.Scan(&hostID, &count); err != nil {
			return nil, fmt.Errorf("fraud: value spike scan: %w", err)
		}
		ev, _ := json.Marshal(map[string]any{"high_value_listings": count, "threshold_usd": 500})
		signals = append(signals, FraudSignal{
			Type:       SignalValueSpike,
			UserID:     hostID,
			Score:      35,
			Evidence:   ev,
			DetectedAt: now,
		})
	}
	return signals, rows.Err()
}

// RunPatternAnalysis runs all cross-transaction pattern detectors.
// Errors from individual detectors are logged; a partial result is returned.
func RunPatternAnalysis(ctx context.Context, repo *Repository) []FraudSignal {
	type detector func(context.Context, *Repository) ([]FraudSignal, error)
	detectors := []detector{
		DetectExclusivePairs,
		DetectDamageAmountGaming,
		DetectSerialDamage,
		DetectNewAccountValueSpike,
	}

	var all []FraudSignal
	for _, fn := range detectors {
		sigs, err := fn(ctx, repo)
		if err != nil {
			slog.Warn("fraud: pattern detector error", "error", err)
			continue
		}
		all = append(all, sigs...)
	}
	return all
}
