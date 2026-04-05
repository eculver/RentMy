package verification

import (
	"context"
	"encoding/json"
	"fmt"

	stripe "github.com/stripe/stripe-go/v81"
	stripeclient "github.com/stripe/stripe-go/v81/client"
	"github.com/stripe/stripe-go/v81/webhook"
)

// stripeIdentityAdapter implements StripeIdentityAdapter using the Stripe Go SDK.
type stripeIdentityAdapter struct {
	sc            *stripeclient.API
	webhookSecret string
}

// NewStripeIdentityAdapter creates a StripeIdentityAdapter backed by the Stripe API.
func NewStripeIdentityAdapter(apiKey, webhookSecret string) StripeIdentityAdapter {
	sc := &stripeclient.API{}
	sc.Init(apiKey, nil)
	return &stripeIdentityAdapter{
		sc:            sc,
		webhookSecret: webhookSecret,
	}
}

// CreateVerificationSession creates a Stripe Identity VerificationSession for a user.
func (a *stripeIdentityAdapter) CreateVerificationSession(_ context.Context, userID string) (StripeSessionResult, error) {
	requireSelfie := true
	params := &stripe.IdentityVerificationSessionParams{
		Type: stripe.String("document"),
		Options: &stripe.IdentityVerificationSessionOptionsParams{
			Document: &stripe.IdentityVerificationSessionOptionsDocumentParams{
				RequireMatchingSelfie: &requireSelfie,
			},
		},
		Metadata: map[string]string{
			"user_id": userID,
		},
	}

	session, err := a.sc.IdentityVerificationSessions.New(params)
	if err != nil {
		return StripeSessionResult{}, fmt.Errorf("stripe identity: creating session: %w", err)
	}

	return StripeSessionResult{
		SessionID:  session.ID,
		SessionURL: session.URL,
	}, nil
}

// ConstructWebhookEvent validates the Stripe signature and returns the event type
// and the raw event.data.object JSON blob.
func (a *stripeIdentityAdapter) ConstructWebhookEvent(body []byte, signature string) (string, json.RawMessage, error) {
	event, err := webhook.ConstructEvent(body, signature, a.webhookSecret)
	if err != nil {
		return "", nil, fmt.Errorf("stripe identity: constructing event: %w", err)
	}

	return string(event.Type), event.Data.Raw, nil
}
