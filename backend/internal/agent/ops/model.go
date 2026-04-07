// Package ops implements the OpsAgent — platform health monitoring, anomaly
// detection, and alert routing.  It runs as a River periodic job every 15
// minutes and exposes a read-only HTTP API consumed by the internal ops
// dashboard.
package ops

import "time"

// Operator defines the comparison applied when evaluating an alert rule.
type Operator string

const (
	OperatorGT        Operator = "GT"        // current > threshold
	OperatorLT        Operator = "LT"        // current < threshold
	OperatorDeviation Operator = "DEVIATION" // |current - rolling_avg| > threshold * rolling_avg
)

// Severity represents the urgency level of a fired alert.
type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
)

// Channel controls where a fired alert is routed.
type Channel string

const (
	ChannelSlack      Channel = "SLACK"
	ChannelPagerDuty  Channel = "PAGERDUTY"
	ChannelBoth       Channel = "BOTH"
)

// MetricValue holds a single named metric and trend metadata.
type MetricValue struct {
	Name          string  `json:"name"`
	Value         float64 `json:"value"`
	PreviousValue float64 `json:"previousValue"`
	Trend         string  `json:"trend"`   // "up" | "down" | "flat"
	Period        string  `json:"period"`  // e.g. "7d"
}

// BusinessMetrics groups marketplace health KPIs.
type BusinessMetrics struct {
	ActiveListings       MetricValue `json:"activeListings"`
	ActiveUsers          MetricValue `json:"activeUsers"`
	BookingConversionRate MetricValue `json:"bookingConversionRate"`
	GrossRevenueCents    MetricValue `json:"grossRevenueCents"`
	NetRevenueCents      MetricValue `json:"netRevenueCents"`
	AvgTransactionCents  MetricValue `json:"avgTransactionCents"`
	HostPayoutVelocityH  MetricValue `json:"hostPayoutVelocityHours"`
}

// TrustMetrics groups trust-and-safety KPIs.
type TrustMetrics struct {
	FraudFlagRate       MetricValue `json:"fraudFlagRate"`
	DisputeRate         MetricValue `json:"disputeRate"`
	AvgAgentConfidence  MetricValue `json:"avgAgentConfidence"`
	CollusionAlertCount MetricValue `json:"collusionAlertCount"`
}

// SupplyMetrics groups supply-side health KPIs.
type SupplyMetrics struct {
	NewHostSignups7d    MetricValue `json:"newHostSignups7d"`
	HostChurnRate       MetricValue `json:"hostChurnRate"`
	AvgResponseRate     MetricValue `json:"avgResponseRate"`
}

// DemandMetrics groups demand-side health KPIs.
type DemandMetrics struct {
	RepeatRenterRate   MetricValue `json:"repeatRenterRate"`
	FailedBookingRate  MetricValue `json:"failedBookingRate"`
}

// HealthSnapshot is a complete, timestamped collection of all metric categories
// captured during a single OpsAgent run.
type HealthSnapshot struct {
	ID         string          `json:"id"`
	Business   BusinessMetrics `json:"business"`
	Trust      TrustMetrics    `json:"trust"`
	Supply     SupplyMetrics   `json:"supply"`
	Demand     DemandMetrics   `json:"demand"`
	Anomalies  []string        `json:"anomalies"`
	CapturedAt time.Time       `json:"capturedAt"`
}

// AlertRule defines a threshold condition that, when triggered, fires an Alert.
type AlertRule struct {
	ID         string    `json:"id"`
	MetricName string    `json:"metricName"`
	Operator   Operator  `json:"operator"`
	Threshold  float64   `json:"threshold"`
	Severity   Severity  `json:"severity"`
	Channel    Channel   `json:"channel"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Alert records a single fired alert event.
type Alert struct {
	ID             string     `json:"id"`
	RuleID         string     `json:"ruleId"`
	MetricName     string     `json:"metricName"`
	CurrentValue   float64    `json:"currentValue"`
	Threshold      float64    `json:"threshold"`
	Severity       Severity   `json:"severity"`
	Channel        Channel    `json:"channel"`
	FiredAt        time.Time  `json:"firedAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
	AcknowledgedBy *string    `json:"acknowledgedBy,omitempty"`
}

// AlertFilters are query parameters for listing alerts.
type AlertFilters struct {
	Severity     Severity
	Acknowledged *bool // nil = all, true = acknowledged, false = unacknowledged
	Page         int
	Limit        int
}
