package ops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackSender sends alert notifications to a Slack webhook URL.
type SlackSender struct {
	webhookURL string
	client     *http.Client
}

// NewSlackSender creates a SlackSender.  webhookURL may be empty; Send will be
// a no-op in that case.
func NewSlackSender(webhookURL string) *SlackSender {
	return &SlackSender{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

type slackBlock struct {
	Type string          `json:"type"`
	Text slackTextObject `json:"text"`
}

type slackTextObject struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackPayload struct {
	Text   string       `json:"text"`
	Blocks []slackBlock `json:"blocks"`
}

// Send posts an alert notification to the configured Slack webhook.
// Retries once on 5xx response.
func (s *SlackSender) Send(a Alert) error {
	if s.webhookURL == "" {
		return nil
	}

	emoji := ":information_source:"
	switch a.Severity {
	case SeverityWarning:
		emoji = ":warning:"
	case SeverityCritical:
		emoji = ":rotating_light:"
	}

	text := fmt.Sprintf("%s *[%s] %s*\nCurrent: `%.4f` | Threshold: `%.4f` | Fired: %s",
		emoji, a.Severity, a.MetricName, a.CurrentValue, a.Threshold,
		a.FiredAt.Format(time.RFC3339),
	)

	payload := slackPayload{
		Text: text,
		Blocks: []slackBlock{
			{
				Type: "section",
				Text: slackTextObject{Type: "mrkdwn", Text: text},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ops: slack marshal: %w", err)
	}

	return s.post(body)
}

func (s *SlackSender) post(body []byte) error {
	resp, err := s.client.Post(s.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ops: slack post: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode >= 500 {
		// Retry once.
		resp2, err2 := s.client.Post(s.webhookURL, "application/json", bytes.NewReader(body))
		if err2 != nil {
			return fmt.Errorf("ops: slack post retry: %w", err2)
		}
		_ = resp2.Body.Close()
		if resp2.StatusCode >= 400 {
			return fmt.Errorf("ops: slack webhook returned %d", resp2.StatusCode)
		}
		return nil
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ops: slack webhook returned %d", resp.StatusCode)
	}
	return nil
}
