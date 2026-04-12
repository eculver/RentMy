package ops

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MetricsCollector runs SQL aggregations against live tables to produce metric values.
type MetricsCollector struct {
	pool *pgxpool.Pool
}

// NewMetricsCollector creates a MetricsCollector backed by the given pool.
func NewMetricsCollector(pool *pgxpool.Pool) *MetricsCollector {
	return &MetricsCollector{pool: pool}
}

// CollectBusinessMetrics runs queries for marketplace health KPIs.
func (c *MetricsCollector) CollectBusinessMetrics(ctx context.Context) (BusinessMetrics, error) {
	var m BusinessMetrics

	// Active listings.
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM listings WHERE status = 'ACTIVE'`,
	).Scan(&m.ActiveListings.Value); err != nil {
		return m, fmt.Errorf("ops: active listings: %w", err)
	}
	m.ActiveListings.Name = "active_listings"
	m.ActiveListings.Period = "current"

	// Active users (last 7 days).
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT id) FROM users WHERE last_active_at > NOW() - INTERVAL '7 days'`,
	).Scan(&m.ActiveUsers.Value); err != nil {
		return m, fmt.Errorf("ops: active users: %w", err)
	}
	m.ActiveUsers.Name = "active_users"
	m.ActiveUsers.Period = "7d"

	// Booking conversion rate: completed / total (last 30 days).
	var total, completed float64
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE created_at > NOW() - INTERVAL '30 days'`,
	).Scan(&total); err != nil {
		return m, fmt.Errorf("ops: booking total: %w", err)
	}
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE status = 'COMPLETED' AND created_at > NOW() - INTERVAL '30 days'`,
	).Scan(&completed); err != nil {
		return m, fmt.Errorf("ops: booking completed: %w", err)
	}
	if total > 0 {
		m.BookingConversionRate.Value = completed / total
	}
	m.BookingConversionRate.Name = "booking_conversion_rate"
	m.BookingConversionRate.Period = "30d"

	// Gross revenue: rental_fee + guarantee_contribution (last 30 days).
	if err := c.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(rental_fee + guarantee_contribution), 0) FROM transactions WHERE status = 'COMPLETED' AND created_at > NOW() - INTERVAL '30 days'`,
	).Scan(&m.GrossRevenueCents.Value); err != nil {
		return m, fmt.Errorf("ops: gross revenue: %w", err)
	}
	m.GrossRevenueCents.Name = "gross_revenue_cents"
	m.GrossRevenueCents.Period = "30d"

	// Net revenue: platform fee (last 30 days).
	if err := c.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(platform_fee), 0) FROM transactions WHERE status = 'COMPLETED' AND created_at > NOW() - INTERVAL '30 days'`,
	).Scan(&m.NetRevenueCents.Value); err != nil {
		return m, fmt.Errorf("ops: net revenue: %w", err)
	}
	m.NetRevenueCents.Name = "net_revenue_cents"
	m.NetRevenueCents.Period = "30d"

	// Average transaction value (last 30 days).
	if err := c.pool.QueryRow(ctx,
		`SELECT COALESCE(AVG(rental_fee), 0) FROM transactions WHERE status = 'COMPLETED' AND created_at > NOW() - INTERVAL '30 days'`,
	).Scan(&m.AvgTransactionCents.Value); err != nil {
		return m, fmt.Errorf("ops: avg transaction: %w", err)
	}
	m.AvgTransactionCents.Name = "avg_transaction_cents"
	m.AvgTransactionCents.Period = "30d"

	// Host payout velocity: avg hours from rental end to now for completed transactions (last 30 days).
	// Approximation using actual_end since individual payout timestamps are in Stripe, not Postgres.
	if err := c.pool.QueryRow(ctx,
		`SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (NOW() - actual_end))/3600), 0)
		 FROM transactions
		 WHERE status = 'COMPLETED' AND actual_end IS NOT NULL
		   AND created_at > NOW() - INTERVAL '30 days'`,
	).Scan(&m.HostPayoutVelocityH.Value); err != nil {
		return m, fmt.Errorf("ops: payout velocity: %w", err)
	}
	m.HostPayoutVelocityH.Name = "host_payout_velocity_hours"
	m.HostPayoutVelocityH.Period = "30d"

	return m, nil
}

// CollectTrustMetrics runs queries for trust-and-safety KPIs.
func (c *MetricsCollector) CollectTrustMetrics(ctx context.Context) (TrustMetrics, error) {
	var m TrustMetrics

	// Fraud flag rate: users with non-empty risk_flags / total users.
	var totalUsers, flaggedUsers float64
	if err := c.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&totalUsers); err != nil {
		return m, fmt.Errorf("ops: total users: %w", err)
	}
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE risk_flags != '[]'::jsonb`,
	).Scan(&flaggedUsers); err != nil {
		return m, fmt.Errorf("ops: flagged users: %w", err)
	}
	if totalUsers > 0 {
		m.FraudFlagRate.Value = flaggedUsers / totalUsers
	}
	m.FraudFlagRate.Name = "fraud_flag_rate"
	m.FraudFlagRate.Period = "current"

	// Dispute rate: disputed / total transactions (last 30 days).
	var totalTx, disputedTx float64
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE created_at > NOW() - INTERVAL '30 days'`,
	).Scan(&totalTx); err != nil {
		return m, fmt.Errorf("ops: tx count: %w", err)
	}
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE status = 'DISPUTED' AND created_at > NOW() - INTERVAL '30 days'`,
	).Scan(&disputedTx); err != nil {
		return m, fmt.Errorf("ops: disputed tx: %w", err)
	}
	if totalTx > 0 {
		m.DisputeRate.Value = disputedTx / totalTx
	}
	m.DisputeRate.Name = "dispute_rate"
	m.DisputeRate.Period = "30d"

	// Average agent confidence across all recent decisions (last 7 days).
	if err := c.pool.QueryRow(ctx,
		`SELECT COALESCE(AVG(confidence), 0) FROM agent_decisions WHERE confidence IS NOT NULL AND created_at > NOW() - INTERVAL '7 days'`,
	).Scan(&m.AvgAgentConfidence.Value); err != nil {
		return m, fmt.Errorf("ops: avg confidence: %w", err)
	}
	m.AvgAgentConfidence.Name = "avg_agent_confidence"
	m.AvgAgentConfidence.Period = "7d"

	// Collusion alert count: decisions where agent_type='FRAUD' and decision->>'action'='SUSPEND' (last 30 days).
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_decisions WHERE agent_type = 'FRAUD' AND decision->>'action' = 'SUSPEND' AND created_at > NOW() - INTERVAL '30 days'`,
	).Scan(&m.CollusionAlertCount.Value); err != nil {
		return m, fmt.Errorf("ops: collusion alerts: %w", err)
	}
	m.CollusionAlertCount.Name = "collusion_alert_count"
	m.CollusionAlertCount.Period = "30d"

	return m, nil
}

// CollectSupplyMetrics runs queries for supply-side health KPIs.
func (c *MetricsCollector) CollectSupplyMetrics(ctx context.Context) (SupplyMetrics, error) {
	var m SupplyMetrics

	// New host signups in the last 7 days: hosts who have at least one listing.
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT host_id) FROM listings WHERE created_at > NOW() - INTERVAL '7 days'`,
	).Scan(&m.NewHostSignups7d.Value); err != nil {
		return m, fmt.Errorf("ops: new host signups: %w", err)
	}
	m.NewHostSignups7d.Name = "new_host_signups_7d"
	m.NewHostSignups7d.Period = "7d"

	// Host churn rate: hosts with no listing activity in last 30 days / total hosts.
	var totalHosts, churnedHosts float64
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT host_id) FROM listings`,
	).Scan(&totalHosts); err != nil {
		return m, fmt.Errorf("ops: total hosts: %w", err)
	}
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT host_id) FROM listings
		 WHERE host_id NOT IN (
		   SELECT DISTINCT host_id FROM transactions WHERE created_at > NOW() - INTERVAL '30 days'
		 )`,
	).Scan(&churnedHosts); err != nil {
		return m, fmt.Errorf("ops: churned hosts: %w", err)
	}
	if totalHosts > 0 {
		m.HostChurnRate.Value = churnedHosts / totalHosts
	}
	m.HostChurnRate.Name = "host_churn_rate"
	m.HostChurnRate.Period = "30d"

	// Average response rate approximation: accepted / (accepted + declined + auto-declined) last 30 days.
	var accepted, total float64
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE status IN ('ACCEPTED','CHECKED_IN','CHECKED_OUT','COMPLETED') AND created_at > NOW() - INTERVAL '30 days'`,
	).Scan(&accepted); err != nil {
		return m, fmt.Errorf("ops: accepted tx: %w", err)
	}
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE created_at > NOW() - INTERVAL '30 days'`,
	).Scan(&total); err != nil {
		return m, fmt.Errorf("ops: total tx for response rate: %w", err)
	}
	if total > 0 {
		m.AvgResponseRate.Value = accepted / total
	}
	m.AvgResponseRate.Name = "avg_response_rate"
	m.AvgResponseRate.Period = "30d"

	return m, nil
}

// CollectDemandMetrics runs queries for demand-side health KPIs.
func (c *MetricsCollector) CollectDemandMetrics(ctx context.Context) (DemandMetrics, error) {
	var m DemandMetrics

	// Repeat renter rate: renters with >1 completed transaction / total renters (last 90 days).
	var totalRenters, repeatRenters float64
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT renter_id) FROM transactions WHERE status = 'COMPLETED' AND created_at > NOW() - INTERVAL '90 days'`,
	).Scan(&totalRenters); err != nil {
		return m, fmt.Errorf("ops: total renters: %w", err)
	}
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM (
		   SELECT renter_id FROM transactions WHERE status = 'COMPLETED' AND created_at > NOW() - INTERVAL '90 days'
		   GROUP BY renter_id HAVING COUNT(*) > 1
		 ) sub`,
	).Scan(&repeatRenters); err != nil {
		return m, fmt.Errorf("ops: repeat renters: %w", err)
	}
	if totalRenters > 0 {
		m.RepeatRenterRate.Value = repeatRenters / totalRenters
	}
	m.RepeatRenterRate.Name = "repeat_renter_rate"
	m.RepeatRenterRate.Period = "90d"

	// Failed booking rate: (auto-declined + cancelled) / total (last 30 days).
	var failed, totalBookings float64
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE status IN ('AUTO_DECLINED','CANCELLED') AND created_at > NOW() - INTERVAL '30 days'`,
	).Scan(&failed); err != nil {
		return m, fmt.Errorf("ops: failed bookings: %w", err)
	}
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE created_at > NOW() - INTERVAL '30 days'`,
	).Scan(&totalBookings); err != nil {
		return m, fmt.Errorf("ops: total bookings: %w", err)
	}
	if totalBookings > 0 {
		m.FailedBookingRate.Value = failed / totalBookings
	}
	m.FailedBookingRate.Name = "failed_booking_rate"
	m.FailedBookingRate.Period = "30d"

	return m, nil
}

// DetectAnomalies compares current snapshot values against the rolling average of
// historical snapshots.  Any metric deviating by more than 2 standard deviations
// is flagged and returned as a human-readable message.
func DetectAnomalies(current HealthSnapshot, history []HealthSnapshot) []string {
	if len(history) < 3 {
		return nil // not enough history to detect anomalies
	}

	type sample struct {
		name   string
		values []float64
		cur    float64
	}

	collect := func(name string, cur float64, vals []float64) sample {
		return sample{name: name, cur: cur, values: vals}
	}

	// Gather historical values for each metric.
	bizActiveListings := make([]float64, len(history))
	bizConversion := make([]float64, len(history))
	trustFraudRate := make([]float64, len(history))
	trustDisputeRate := make([]float64, len(history))
	trustConfidence := make([]float64, len(history))
	supplyChurn := make([]float64, len(history))
	demandFailed := make([]float64, len(history))

	for i, h := range history {
		bizActiveListings[i] = h.Business.ActiveListings.Value
		bizConversion[i] = h.Business.BookingConversionRate.Value
		trustFraudRate[i] = h.Trust.FraudFlagRate.Value
		trustDisputeRate[i] = h.Trust.DisputeRate.Value
		trustConfidence[i] = h.Trust.AvgAgentConfidence.Value
		supplyChurn[i] = h.Supply.HostChurnRate.Value
		demandFailed[i] = h.Demand.FailedBookingRate.Value
	}

	samples := []sample{
		collect("active_listings", current.Business.ActiveListings.Value, bizActiveListings),
		collect("booking_conversion_rate", current.Business.BookingConversionRate.Value, bizConversion),
		collect("fraud_flag_rate", current.Trust.FraudFlagRate.Value, trustFraudRate),
		collect("dispute_rate", current.Trust.DisputeRate.Value, trustDisputeRate),
		collect("avg_agent_confidence", current.Trust.AvgAgentConfidence.Value, trustConfidence),
		collect("host_churn_rate", current.Supply.HostChurnRate.Value, supplyChurn),
		collect("failed_booking_rate", current.Demand.FailedBookingRate.Value, demandFailed),
	}

	var anomalies []string
	for _, s := range samples {
		mean, std := meanStdDev(s.values)
		if std < 1e-9 {
			continue // effectively constant — no meaningful deviation possible
		}
		z := math.Abs(s.cur-mean) / std
		if z > 2.0 {
			dir := "above"
			if s.cur < mean {
				dir = "below"
			}
			anomalies = append(anomalies, fmt.Sprintf(
				"%s is %.2f std devs %s rolling average (current=%.4f mean=%.4f)",
				s.name, z, dir, s.cur, mean,
			))
		}
	}
	return anomalies
}

func meanStdDev(vals []float64) (mean, std float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	mean = sum / float64(len(vals))
	variance := 0.0
	for _, v := range vals {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(vals))
	std = math.Sqrt(variance)
	return mean, std
}
