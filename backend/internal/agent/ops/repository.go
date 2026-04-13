package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// Repository performs database operations for OpsAgent.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// InsertHealthSnapshot persists a snapshot to the database.
func (r *Repository) InsertHealthSnapshot(ctx context.Context, snap HealthSnapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("ops: marshal snapshot: %w", err)
	}

	const q = `INSERT INTO ops_health_snapshots (id, snapshot, captured_at) VALUES ($1, $2, $3)`
	_, err = r.pool.Exec(ctx, q, snap.ID, data, snap.CapturedAt)
	if err != nil {
		return fmt.Errorf("ops: insert snapshot: %w", err)
	}
	return nil
}

// GetLatestSnapshot returns the most recent health snapshot, or ErrNoSnapshot if
// the table is empty.
func (r *Repository) GetLatestSnapshot(ctx context.Context) (HealthSnapshot, error) {
	const q = `SELECT snapshot FROM ops_health_snapshots ORDER BY captured_at DESC LIMIT 1`
	row := r.pool.QueryRow(ctx, q)

	var data []byte
	if err := row.Scan(&data); err != nil {
		if err == pgx.ErrNoRows {
			return HealthSnapshot{}, ErrNoSnapshot
		}
		return HealthSnapshot{}, fmt.Errorf("ops: get latest snapshot: %w", err)
	}

	var snap HealthSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return HealthSnapshot{}, fmt.Errorf("ops: unmarshal snapshot: %w", err)
	}
	return snap, nil
}

// GetSnapshotHistory returns all snapshots captured within the given duration.
func (r *Repository) GetSnapshotHistory(ctx context.Context, since time.Duration) ([]HealthSnapshot, error) {
	cutoff := time.Now().UTC().Add(-since)
	const q = `SELECT snapshot FROM ops_health_snapshots WHERE captured_at >= $1 ORDER BY captured_at ASC`
	rows, err := r.pool.Query(ctx, q, cutoff)
	if err != nil {
		return nil, fmt.Errorf("ops: query snapshot history: %w", err)
	}
	defer rows.Close()

	var snaps []HealthSnapshot
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("ops: scan snapshot: %w", err)
		}
		var snap HealthSnapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			return nil, fmt.Errorf("ops: unmarshal snapshot: %w", err)
		}
		snaps = append(snaps, snap)
	}
	return snaps, rows.Err()
}

// ListAlertRules returns all configured alert rules.
func (r *Repository) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	const q = `
		SELECT id, metric_name, operator, threshold, severity, channel, enabled, created_at, updated_at
		FROM ops_alert_rules
		ORDER BY metric_name`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("ops: list alert rules: %w", err)
	}
	defer rows.Close()

	var rules []AlertRule
	for rows.Next() {
		var ru AlertRule
		if err := rows.Scan(
			&ru.ID, &ru.MetricName, &ru.Operator, &ru.Threshold,
			&ru.Severity, &ru.Channel, &ru.Enabled, &ru.CreatedAt, &ru.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("ops: scan alert rule: %w", err)
		}
		rules = append(rules, ru)
	}
	return rules, rows.Err()
}

// UpsertAlertRule creates or updates an alert rule by metric_name.
func (r *Repository) UpsertAlertRule(ctx context.Context, ru AlertRule) error {
	if ru.ID == "" {
		ru.ID = ulid.New()
	}
	const q = `
		INSERT INTO ops_alert_rules (id, metric_name, operator, threshold, severity, channel, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		ON CONFLICT (metric_name) DO UPDATE SET
			operator   = EXCLUDED.operator,
			threshold  = EXCLUDED.threshold,
			severity   = EXCLUDED.severity,
			channel    = EXCLUDED.channel,
			enabled    = EXCLUDED.enabled,
			updated_at = NOW()`
	_, err := r.pool.Exec(ctx, q,
		ru.ID, ru.MetricName, ru.Operator, ru.Threshold, ru.Severity, ru.Channel, ru.Enabled,
	)
	if err != nil {
		return fmt.Errorf("ops: upsert alert rule: %w", err)
	}
	return nil
}

// UpdateAlertRule updates an existing alert rule by ID.
func (r *Repository) UpdateAlertRule(ctx context.Context, ru AlertRule) error {
	const q = `
		UPDATE ops_alert_rules
		SET threshold = $2, severity = $3, channel = $4, enabled = $5, updated_at = NOW()
		WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, ru.ID, ru.Threshold, ru.Severity, ru.Channel, ru.Enabled)
	if err != nil {
		return fmt.Errorf("ops: update alert rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRuleNotFound
	}
	return nil
}

// InsertAlert records a fired alert.
func (r *Repository) InsertAlert(ctx context.Context, a Alert) error {
	const q = `
		INSERT INTO ops_alerts (id, rule_id, metric_name, current_value, threshold, severity, channel, fired_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.pool.Exec(ctx, q,
		a.ID, a.RuleID, a.MetricName, a.CurrentValue, a.Threshold, a.Severity, a.Channel, a.FiredAt,
	)
	if err != nil {
		return fmt.Errorf("ops: insert alert: %w", err)
	}
	return nil
}

// ListAlerts returns paginated alerts matching the given filters.
func (r *Repository) ListAlerts(ctx context.Context, f AlertFilters) ([]Alert, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.Limit

	// Build WHERE clauses dynamically.
	where := " WHERE 1=1"
	args := []any{}
	argIdx := 1

	if f.Severity != "" {
		where += fmt.Sprintf(" AND severity = $%d", argIdx)
		args = append(args, string(f.Severity))
		argIdx++
	}
	if f.Acknowledged != nil {
		if *f.Acknowledged {
			where += " AND acknowledged_at IS NOT NULL"
		} else {
			where += " AND acknowledged_at IS NULL"
		}
	}

	args = append(args, f.Limit, offset)
	q := fmt.Sprintf(`
		SELECT id, rule_id, metric_name, current_value, threshold, severity, channel,
		       fired_at, acknowledged_at, acknowledged_by
		FROM ops_alerts%s
		ORDER BY fired_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("ops: list alerts: %w", err)
	}
	defer rows.Close()

	var alerts []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(
			&a.ID, &a.RuleID, &a.MetricName, &a.CurrentValue, &a.Threshold,
			&a.Severity, &a.Channel, &a.FiredAt, &a.AcknowledgedAt, &a.AcknowledgedBy,
		); err != nil {
			return nil, fmt.Errorf("ops: scan alert: %w", err)
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// AcknowledgeAlert marks an alert as acknowledged by the given user.
func (r *Repository) AcknowledgeAlert(ctx context.Context, alertID, userID string) error {
	const q = `
		UPDATE ops_alerts SET acknowledged_at = NOW(), acknowledged_by = $2
		WHERE id = $1 AND acknowledged_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, alertID, userID)
	if err != nil {
		return fmt.Errorf("ops: acknowledge alert: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAlertNotFound
	}
	return nil
}

// GetSnapshotHistoryRaw returns raw JSONB snapshots for rolling average computation.
// Returns at most maxRows rows ordered by captured_at DESC.
func (r *Repository) GetSnapshotHistoryRaw(ctx context.Context, since time.Duration, maxRows int) ([]HealthSnapshot, error) {
	cutoff := time.Now().UTC().Add(-since)
	q := fmt.Sprintf(`
		SELECT snapshot FROM ops_health_snapshots
		WHERE captured_at >= $1
		ORDER BY captured_at DESC
		LIMIT %d`, maxRows)
	rows, err := r.pool.Query(ctx, q, cutoff)
	if err != nil {
		return nil, fmt.Errorf("ops: query raw snapshots: %w", err)
	}
	defer rows.Close()

	var snaps []HealthSnapshot
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("ops: scan snapshot: %w", err)
		}
		var snap HealthSnapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			return nil, fmt.Errorf("ops: unmarshal snapshot: %w", err)
		}
		snaps = append(snaps, snap)
	}
	return snaps, rows.Err()
}
