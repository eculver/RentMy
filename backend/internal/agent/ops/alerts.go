package ops

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// AlertRouter evaluates rules against a snapshot and dispatches alerts.
type AlertRouter struct {
	repo      *Repository
	slack     *SlackSender
	pagerduty *PagerDutySender
}

// NewAlertRouter creates an AlertRouter.
func NewAlertRouter(repo *Repository, slack *SlackSender, pd *PagerDutySender) *AlertRouter {
	return &AlertRouter{repo: repo, slack: slack, pagerduty: pd}
}

// EvaluateRules compares each metric in the snapshot against the configured
// alert rules and returns the set of rules that triggered.
func EvaluateRules(snap HealthSnapshot, rules []AlertRule) []AlertRule {
	values := extractMetricValues(snap)
	var triggered []AlertRule
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		cur, ok := values[rule.MetricName]
		if !ok {
			continue
		}
		if ruleTriggered(rule, cur) {
			triggered = append(triggered, rule)
		}
	}
	return triggered
}

func ruleTriggered(rule AlertRule, value float64) bool {
	switch rule.Operator {
	case OperatorGT:
		return value > rule.Threshold
	case OperatorLT:
		return value < rule.Threshold
	default:
		return false // DEVIATION handled by anomaly detection
	}
}

// extractMetricValues builds a flat map of metric name → current value from a snapshot.
func extractMetricValues(snap HealthSnapshot) map[string]float64 {
	return map[string]float64{
		"active_listings":          snap.Business.ActiveListings.Value,
		"active_users":             snap.Business.ActiveUsers.Value,
		"booking_conversion_rate":  snap.Business.BookingConversionRate.Value,
		"gross_revenue_cents":      snap.Business.GrossRevenueCents.Value,
		"net_revenue_cents":        snap.Business.NetRevenueCents.Value,
		"avg_transaction_cents":    snap.Business.AvgTransactionCents.Value,
		"host_payout_velocity_hours": snap.Business.HostPayoutVelocityH.Value,
		"fraud_flag_rate":          snap.Trust.FraudFlagRate.Value,
		"dispute_rate":             snap.Trust.DisputeRate.Value,
		"avg_agent_confidence":     snap.Trust.AvgAgentConfidence.Value,
		"collusion_alert_count":    snap.Trust.CollusionAlertCount.Value,
		"new_host_signups_7d":      snap.Supply.NewHostSignups7d.Value,
		"host_churn_rate":          snap.Supply.HostChurnRate.Value,
		"avg_response_rate":        snap.Supply.AvgResponseRate.Value,
		"repeat_renter_rate":       snap.Demand.RepeatRenterRate.Value,
		"failed_booking_rate":      snap.Demand.FailedBookingRate.Value,
	}
}

// RouteAlert stores the alert in the DB and dispatches it to the appropriate channel.
func (ar *AlertRouter) RouteAlert(ctx context.Context, rule AlertRule, value float64) error {
	a := Alert{
		ID:           ulid.New(),
		RuleID:       rule.ID,
		MetricName:   rule.MetricName,
		CurrentValue: value,
		Threshold:    rule.Threshold,
		Severity:     rule.Severity,
		Channel:      rule.Channel,
		FiredAt:      time.Now().UTC(),
	}

	if err := ar.repo.InsertAlert(ctx, a); err != nil {
		return fmt.Errorf("ops: store alert: %w", err)
	}

	// Dispatch to Slack for all severities.
	if (rule.Channel == ChannelSlack || rule.Channel == ChannelBoth) && ar.slack != nil {
		if err := ar.slack.Send(a); err != nil {
			slog.Warn("ops: slack alert failed", "alert_id", a.ID, "error", err)
		}
	}

	// Dispatch to PagerDuty for CRITICAL only.
	if rule.Severity == SeverityCritical &&
		(rule.Channel == ChannelPagerDuty || rule.Channel == ChannelBoth) &&
		ar.pagerduty != nil {
		if err := ar.pagerduty.Send(a); err != nil {
			slog.Warn("ops: pagerduty alert failed", "alert_id", a.ID, "error", err)
		}
	}

	return nil
}
