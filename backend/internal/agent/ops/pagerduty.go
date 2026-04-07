package ops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const pagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"

// PagerDutySender sends critical alert notifications via the PagerDuty Events API v2.
type PagerDutySender struct {
	routingKey string
	client     *http.Client
}

// NewPagerDutySender creates a PagerDutySender.  routingKey may be empty; Send
// will be a no-op in that case.
func NewPagerDutySender(routingKey string) *PagerDutySender {
	return &PagerDutySender{
		routingKey: routingKey,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

type pdPayload struct {
	RoutingKey  string    `json:"routing_key"`
	EventAction string    `json:"event_action"`
	DedupKey    string    `json:"dedup_key"`
	Payload     pdDetails `json:"payload"`
}

type pdDetails struct {
	Summary  string `json:"summary"`
	Severity string `json:"severity"`
	Source   string `json:"source"`
}

// Send triggers a PagerDuty incident for the given alert.
// The dedup key is rule_id + metric_name + truncated-hour to prevent duplicate
// pages within the same hour.
func (s *PagerDutySender) Send(a Alert) error {
	if s.routingKey == "" {
		return nil
	}

	hour := a.FiredAt.UTC().Format("2006010215")
	dedupKey := fmt.Sprintf("%s-%s-%s", a.RuleID, a.MetricName, hour)

	pdSeverity := "info"
	switch a.Severity {
	case SeverityWarning:
		pdSeverity = "warning"
	case SeverityCritical:
		pdSeverity = "critical"
	}

	summary := fmt.Sprintf("[RentMy][%s] %s: current=%.4f threshold=%.4f",
		a.Severity, a.MetricName, a.CurrentValue, a.Threshold)

	p := pdPayload{
		RoutingKey:  s.routingKey,
		EventAction: "trigger",
		DedupKey:    dedupKey,
		Payload: pdDetails{
			Summary:  summary,
			Severity: pdSeverity,
			Source:   "rentmy-ops-agent",
		},
	}

	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("ops: pagerduty marshal: %w", err)
	}

	resp, err := s.client.Post(pagerDutyEventsURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ops: pagerduty post: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ops: pagerduty returned %d", resp.StatusCode)
	}
	return nil
}
