package payment

import (
	"context"
	"fmt"

	stripe "github.com/stripe/stripe-go/v81"
	stripeclient "github.com/stripe/stripe-go/v81/client"
)

// StripeAdapter implements PaymentAdapter using Stripe Express Connect.
type StripeAdapter struct {
	sc *stripeclient.API
}

// Option is a functional option for configuring a StripeAdapter.
type Option func(*StripeAdapter)

// NewStripeAdapter creates a new StripeAdapter initialised with the given Stripe secret key.
func NewStripeAdapter(apiKey string, opts ...Option) *StripeAdapter {
	sc := &stripeclient.API{}
	sc.Init(apiKey, nil)
	a := &StripeAdapter{sc: sc}
	for _, o := range opts {
		o(a)
	}
	return a
}

// AuthorizeHold creates an uncaptured PaymentIntent (manual capture) as a security hold.
// The renter's payment method is confirmed immediately; funds are held but not captured.
func (a *StripeAdapter) AuthorizeHold(ctx context.Context, amount int64, currency string, paymentMethodID string, customerID string) (string, error) {
	params := &stripe.PaymentIntentParams{
		Amount:        stripe.Int64(amount),
		Currency:      stripe.String(currency),
		PaymentMethod: stripe.String(paymentMethodID),
		Customer:      stripe.String(customerID),
		CaptureMethod: stripe.String(string(stripe.PaymentIntentCaptureMethodManual)),
		Confirm:       stripe.Bool(true),
	}
	params.Context = ctx
	pi, err := a.sc.PaymentIntents.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe authorize hold: %w", err)
	}
	return pi.ID, nil
}

// CaptureHold captures a portion of an existing PaymentIntent hold.
// Returns the Charge ID associated with the captured amount.
func (a *StripeAdapter) CaptureHold(ctx context.Context, holdID string, amount int64) (string, error) {
	params := &stripe.PaymentIntentCaptureParams{
		AmountToCapture: stripe.Int64(amount),
	}
	params.Context = ctx
	pi, err := a.sc.PaymentIntents.Capture(holdID, params)
	if err != nil {
		return "", fmt.Errorf("stripe capture hold: %w", err)
	}
	chargeID := ""
	if pi.LatestCharge != nil {
		chargeID = pi.LatestCharge.ID
	}
	return chargeID, nil
}

// ReleaseHold cancels an existing PaymentIntent, releasing the hold.
func (a *StripeAdapter) ReleaseHold(ctx context.Context, holdID string) error {
	params := &stripe.PaymentIntentCancelParams{}
	params.Context = ctx
	_, err := a.sc.PaymentIntents.Cancel(holdID, params)
	if err != nil {
		return fmt.Errorf("stripe release hold: %w", err)
	}
	return nil
}

// ChargeRentalFee creates an automatically captured PaymentIntent for the rental fee.
// Returns the Charge ID.
func (a *StripeAdapter) ChargeRentalFee(ctx context.Context, amount int64, currency string, paymentMethodID string, customerID string) (string, error) {
	params := &stripe.PaymentIntentParams{
		Amount:        stripe.Int64(amount),
		Currency:      stripe.String(currency),
		PaymentMethod: stripe.String(paymentMethodID),
		Customer:      stripe.String(customerID),
		CaptureMethod: stripe.String(string(stripe.PaymentIntentCaptureMethodAutomatic)),
		Confirm:       stripe.Bool(true),
	}
	params.Context = ctx
	pi, err := a.sc.PaymentIntents.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe charge rental fee: %w", err)
	}
	chargeID := ""
	if pi.LatestCharge != nil {
		chargeID = pi.LatestCharge.ID
	}
	return chargeID, nil
}

// PayoutHost transfers the host payout amount to a Stripe Express connected account.
// Returns the Transfer ID.
func (a *StripeAdapter) PayoutHost(ctx context.Context, amount int64, currency string, hostAccountID string) (string, error) {
	params := &stripe.TransferParams{
		Amount:      stripe.Int64(amount),
		Currency:    stripe.String(currency),
		Destination: stripe.String(hostAccountID),
	}
	params.Context = ctx
	t, err := a.sc.Transfers.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe payout host: %w", err)
	}
	return t.ID, nil
}

// Refund issues a refund for a previous charge.
// Returns the Refund ID.
func (a *StripeAdapter) Refund(ctx context.Context, chargeID string, amount int64) (string, error) {
	params := &stripe.RefundParams{
		Charge: stripe.String(chargeID),
		Amount: stripe.Int64(amount),
	}
	params.Context = ctx
	ref, err := a.sc.Refunds.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe refund: %w", err)
	}
	return ref.ID, nil
}

// CreateConnectedAccount creates a Stripe Express connected account for a host and
// returns the Stripe account ID and the onboarding URL.
func (a *StripeAdapter) CreateConnectedAccount(ctx context.Context, email string) (string, string, error) {
	acctParams := &stripe.AccountParams{
		Type:  stripe.String(string(stripe.AccountTypeExpress)),
		Email: stripe.String(email),
	}
	acctParams.Context = ctx
	acct, err := a.sc.Accounts.New(acctParams)
	if err != nil {
		return "", "", fmt.Errorf("stripe create connected account: %w", err)
	}

	linkParams := &stripe.AccountLinkParams{
		Account:    stripe.String(acct.ID),
		RefreshURL: stripe.String("https://rentmy.app/onboard/refresh"),
		ReturnURL:  stripe.String("https://rentmy.app/onboard/complete"),
		Type:       stripe.String("account_onboarding"),
	}
	linkParams.Context = ctx
	link, err := a.sc.AccountLinks.New(linkParams)
	if err != nil {
		return acct.ID, "", fmt.Errorf("stripe create account link: %w", err)
	}
	return acct.ID, link.URL, nil
}

// CreateCustomer creates a Stripe customer for a renter.
// Returns the Stripe customer ID.
func (a *StripeAdapter) CreateCustomer(ctx context.Context, email string, name string) (string, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
	}
	params.Context = ctx
	c, err := a.sc.Customers.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe create customer: %w", err)
	}
	return c.ID, nil
}

// CreateSetupIntent creates a Stripe SetupIntent for saving a payment method on file.
// The returned client secret is passed to the Stripe mobile SDK to collect card details.
func (a *StripeAdapter) CreateSetupIntent(ctx context.Context, customerID string) (string, error) {
	params := &stripe.SetupIntentParams{
		Customer: stripe.String(customerID),
		PaymentMethodTypes: []*string{
			stripe.String("card"),
		},
	}
	params.Context = ctx
	si, err := a.sc.SetupIntents.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe create setup intent: %w", err)
	}
	return si.ClientSecret, nil
}
