package proximity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TwilioClient sends SMS messages via the Twilio REST API.
// Uses a plain HTTP client — no external SDK dependency.
type TwilioClient struct {
	accountSID string
	authToken  string
	fromNumber string
	httpClient *http.Client
}

// NewTwilioClient creates a TwilioClient with the given credentials.
func NewTwilioClient(accountSID, authToken, fromNumber string) *TwilioClient {
	return &TwilioClient{
		accountSID: accountSID,
		authToken:  authToken,
		fromNumber: fromNumber,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// twilioErrorResponse captures the Twilio API error body for diagnostics.
type twilioErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SendSMS delivers a text message to the given E.164 phone number.
func (c *TwilioClient) SendSMS(ctx context.Context, to, body string) error {
	endpoint := fmt.Sprintf(
		"https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json",
		c.accountSID,
	)

	form := url.Values{}
	form.Set("To", to)
	form.Set("From", c.fromNumber)
	form.Set("Body", body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build twilio request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.accountSID, c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("twilio request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		var twErr twilioErrorResponse
		_ = json.Unmarshal(raw, &twErr)
		return fmt.Errorf("twilio error %d (code %d): %s", resp.StatusCode, twErr.Code, twErr.Message)
	}

	return nil
}
