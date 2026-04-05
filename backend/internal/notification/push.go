package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	expo "github.com/oliveroneill/exponent-server-sdk-golang/sdk"
)

// PushClient wraps the Expo push notification SDK.
// It handles chunked delivery (100 messages per batch) and interprets
// DeviceNotRegistered errors so callers can remove stale tokens.
type PushClient struct {
	client *expo.PushClient
}

// NewPushClient creates a PushClient using the provided Expo access token.
// If accessToken is empty the client operates without authentication, which is
// acceptable for development but not production.
func NewPushClient(accessToken string) *PushClient {
	var cfg *expo.ClientConfig
	if accessToken != "" {
		cfg = &expo.ClientConfig{AccessToken: accessToken}
	}
	return &PushClient{client: expo.NewPushClient(cfg)}
}

// SendBatch delivers messages to all provided Expo push tokens.
// Tokens that come back as DeviceNotRegistered are returned in the stale slice
// so the caller can remove them from the database.
func (p *PushClient) SendBatch(_ context.Context, tokens []string, title, body string, data map[string]string) (stale []string, err error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	// Build message list — one message per token for per-token stale tracking.
	msgs := make([]expo.PushMessage, 0, len(tokens))
	validTokens := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		pushToken, parseErr := expo.NewExponentPushToken(tok)
		if parseErr != nil {
			slog.Warn("invalid expo push token, skipping", "token", tok, "error", parseErr)
			continue
		}
		msgs = append(msgs, expo.PushMessage{
			To:    []expo.ExponentPushToken{pushToken},
			Title: title,
			Body:  body,
			Data:  data,
			Sound: "default",
		})
		validTokens = append(validTokens, tok)
	}
	if len(msgs) == 0 {
		return nil, nil
	}

	responses, sendErr := p.client.PublishMultiple(msgs)
	if sendErr != nil {
		return nil, fmt.Errorf("expo publish: %w", sendErr)
	}

	// Inspect receipt statuses to find stale tokens.
	for i := range responses {
		resp := &responses[i]
		if validateErr := resp.ValidateResponse(); validateErr != nil {
			var dnrErr *expo.DeviceNotRegisteredError
			if errors.As(validateErr, &dnrErr) && i < len(validTokens) {
				stale = append(stale, validTokens[i])
			}
			slog.Warn("push notification delivery failed",
				"error", validateErr,
				"token", safeToken(validTokens, i),
			)
		}
	}
	return stale, nil
}

func safeToken(tokens []string, i int) string {
	if i < len(tokens) {
		return tokens[i]
	}
	return "unknown"
}
