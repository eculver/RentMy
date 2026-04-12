package fraud

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// DetectSharedDeviceFingerprint returns a signal when another user shares the
// same device_fingerprint as userID.  Score: +40.
func DetectSharedDeviceFingerprint(ctx context.Context, repo *Repository, userID string) ([]FraudSignal, error) {
	const q = `
		SELECT DISTINCT u.id
		FROM users u
		JOIN users target ON target.device_fingerprint = u.device_fingerprint
		WHERE target.id = $1
		  AND u.id != $1
		  AND u.device_fingerprint IS NOT NULL`

	rows, err := repo.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("fraud: shared device query: %w", err)
	}
	defer rows.Close()

	var signals []FraudSignal
	for rows.Next() {
		var relatedID string
		if err := rows.Scan(&relatedID); err != nil {
			return nil, fmt.Errorf("fraud: shared device scan: %w", err)
		}
		ev, _ := json.Marshal(map[string]string{"shared_with": relatedID})
		signals = append(signals, FraudSignal{
			Type:          SignalSharedDeviceFingerprint,
			UserID:        userID,
			RelatedUserID: relatedID,
			Score:         40,
			Evidence:      ev,
			DetectedAt:    time.Now().UTC(),
		})
	}
	return signals, rows.Err()
}

// DetectLinkedPaymentInstrument returns a signal when two users share the same
// Stripe payment method fingerprint stored in the user's signup_metadata.
// Score: +50.
func DetectLinkedPaymentInstrument(ctx context.Context, repo *Repository, userID string) ([]FraudSignal, error) {
	const q = `
		SELECT DISTINCT u.id
		FROM users u
		JOIN users target ON
		    target.signup_metadata->>'payment_fingerprint' IS NOT NULL
		    AND target.signup_metadata->>'payment_fingerprint' != ''
		    AND u.signup_metadata->>'payment_fingerprint' = target.signup_metadata->>'payment_fingerprint'
		WHERE target.id = $1
		  AND u.id != $1`

	rows, err := repo.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("fraud: linked payment query: %w", err)
	}
	defer rows.Close()

	var signals []FraudSignal
	for rows.Next() {
		var relatedID string
		if err := rows.Scan(&relatedID); err != nil {
			return nil, fmt.Errorf("fraud: linked payment scan: %w", err)
		}
		ev, _ := json.Marshal(map[string]string{"shared_payment_with": relatedID})
		signals = append(signals, FraudSignal{
			Type:          SignalLinkedPaymentInstrument,
			UserID:        userID,
			RelatedUserID: relatedID,
			Score:         50,
			Evidence:      ev,
			DetectedAt:    time.Now().UTC(),
		})
	}
	return signals, rows.Err()
}

// DetectCarrierBatchPhone checks whether a user's phone number belongs to a
// known carrier batch (sequential numbers sharing a prefix with incrementing
// suffixes within the same batch of accounts created within 48h).
// Score: +30.
func DetectCarrierBatchPhone(ctx context.Context, repo *Repository, userID string) ([]FraudSignal, error) {
	// Fetch the user's phone.
	const phoneQ = `SELECT phone FROM users WHERE id = $1`
	row := repo.pool.QueryRow(ctx, phoneQ, userID)
	var phone *string
	if err := row.Scan(&phone); err != nil || phone == nil || *phone == "" {
		return nil, nil
	}

	p := *phone
	// Require at least 7 characters and isolate the prefix (all but last 3 digits).
	if len(p) < 7 {
		return nil, nil
	}
	prefix := p[:len(p)-3]

	// Find other accounts created within 48h that share the phone prefix.
	const batchQ = `
		SELECT u.id, u.phone
		FROM users u
		JOIN users target ON ABS(EXTRACT(EPOCH FROM u.created_at - target.created_at)) < 172800
		WHERE target.id = $1
		  AND u.id != $1
		  AND u.phone LIKE $2`

	rows, err := repo.pool.Query(ctx, batchQ, userID, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("fraud: carrier batch query: %w", err)
	}
	defer rows.Close()

	var signals []FraudSignal
	for rows.Next() {
		var relatedID, relatedPhone string
		if err := rows.Scan(&relatedID, &relatedPhone); err != nil {
			return nil, fmt.Errorf("fraud: carrier batch scan: %w", err)
		}
		// Verify they are sequential (differ only in last 3 digits as integers).
		if !isSequentialPhone(p, relatedPhone) {
			continue
		}
		ev, _ := json.Marshal(map[string]string{
			"phone_prefix": prefix, "related_phone": relatedPhone, "related_user": relatedID,
		})
		signals = append(signals, FraudSignal{
			Type:          SignalCarrierBatchPhone,
			UserID:        userID,
			RelatedUserID: relatedID,
			Score:         30,
			Evidence:      ev,
			DetectedAt:    time.Now().UTC(),
		})
	}
	return signals, rows.Err()
}

// isSequentialPhone checks if two phones share a prefix and their suffixes are
// numerically sequential (within 10).
func isSequentialPhone(a, b string) bool {
	if len(a) != len(b) || len(a) < 4 {
		return false
	}
	if a[:len(a)-3] != b[:len(b)-3] {
		return false
	}
	var iA, iB int
	fmt.Sscanf(a[len(a)-3:], "%d", &iA)
	fmt.Sscanf(b[len(b)-3:], "%d", &iB)
	diff := iA - iB
	if diff < 0 {
		diff = -diff
	}
	return diff <= 10
}

// DetectSimultaneousCreation returns a signal when another account was created
// within 5 minutes of userID's account.  Score: +35.
func DetectSimultaneousCreation(ctx context.Context, repo *Repository, userID string) ([]FraudSignal, error) {
	const q = `
		SELECT u.id
		FROM users u
		JOIN users target ON ABS(EXTRACT(EPOCH FROM u.created_at - target.created_at)) < 300
		WHERE target.id = $1
		  AND u.id != $1`

	rows, err := repo.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("fraud: simultaneous creation query: %w", err)
	}
	defer rows.Close()

	var signals []FraudSignal
	for rows.Next() {
		var relatedID string
		if err := rows.Scan(&relatedID); err != nil {
			return nil, fmt.Errorf("fraud: simultaneous creation scan: %w", err)
		}
		ev, _ := json.Marshal(map[string]string{"simultaneous_with": relatedID, "window_seconds": "300"})
		signals = append(signals, FraudSignal{
			Type:          SignalSimultaneousCreation,
			UserID:        userID,
			RelatedUserID: relatedID,
			Score:         35,
			Evidence:      ev,
			DetectedAt:    time.Now().UTC(),
		})
	}
	return signals, rows.Err()
}

// DetectWiFiNetwork returns a signal (IsCompoundOnly=true) when another user
// shares the same signup WiFi BSSID.  Score: +30 but only counted when the
// bundle already contains at least one non-compound signal.
func DetectWiFiNetwork(ctx context.Context, repo *Repository, userID string) ([]FraudSignal, error) {
	const q = `
		SELECT DISTINCT u.id, u.signup_metadata->>'wifi_bssid' AS bssid
		FROM users u
		JOIN users target ON
		    target.signup_metadata->>'wifi_bssid' IS NOT NULL
		    AND target.signup_metadata->>'wifi_bssid' != ''
		    AND u.signup_metadata->>'wifi_bssid' = target.signup_metadata->>'wifi_bssid'
		WHERE target.id = $1
		  AND u.id != $1`

	rows, err := repo.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("fraud: wifi network query: %w", err)
	}
	defer rows.Close()

	var signals []FraudSignal
	for rows.Next() {
		var relatedID string
		var bssid *string
		if err := rows.Scan(&relatedID, &bssid); err != nil {
			return nil, fmt.Errorf("fraud: wifi network scan: %w", err)
		}
		bssidStr := ""
		if bssid != nil {
			bssidStr = *bssid
		}
		ev, _ := json.Marshal(map[string]string{"wifi_bssid": bssidStr, "shared_with": relatedID})
		signals = append(signals, FraudSignal{
			Type:           SignalWiFiNetwork,
			UserID:         userID,
			RelatedUserID:  relatedID,
			Score:          30,
			IsCompoundOnly: true,
			Evidence:       ev,
			DetectedAt:     time.Now().UTC(),
		})
	}
	return signals, rows.Err()
}

// RunAllSignals executes every per-user signal detector and assembles a
// SignalBundle.  Errors from individual detectors are logged but do not abort
// the run — a partial bundle is better than no detection.
//
// WiFi compound-only logic:
//   - If the bundle contains at least one signal where IsCompoundOnly == false,
//     the WiFi +30 is included in TotalScore.
//   - WiFi-only bundles produce a TotalScore of 0.
func RunAllSignals(ctx context.Context, repo *Repository, userID string) SignalBundle {
	bundle := SignalBundle{UserID: userID}

	detectors := []func(context.Context, *Repository, string) ([]FraudSignal, error){
		DetectSharedDeviceFingerprint,
		DetectLinkedPaymentInstrument,
		DetectCarrierBatchPhone,
		DetectSimultaneousCreation,
		DetectWiFiNetwork,
	}

	for _, fn := range detectors {
		sigs, err := fn(ctx, repo, userID)
		if err != nil {
			slog.Warn("fraud: signal detector error", "userId", userID, "error", err)
			continue
		}
		bundle.Signals = append(bundle.Signals, sigs...)
	}

	// Compute compound score and track whether any non-compound signal exists.
	for i := range bundle.Signals {
		sig := &bundle.Signals[i]
		if !sig.IsCompoundOnly {
			bundle.HasNonCompoundSignal = true
			bundle.CompoundScore += sig.Score
		} else {
			// WiFi — add its score tentatively; TotalScore() gates it.
			bundle.CompoundScore += sig.Score
		}
	}

	return bundle
}
