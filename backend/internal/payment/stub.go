package payment

import (
	"context"
	"fmt"

	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// StubAdapter is a no-op PaymentAdapter for development and testing when Stripe
// is not configured (secret key is "sk_test_placeholder"). It returns fake IDs
// and never calls any external service.
//
// Justification: Maestro E2E tests run against a real backend but cannot
// interact with Stripe payment sheets (native modal). This adapter allows
// booking creation to succeed without a real Stripe connection, similar to the
// __DEV__ camera bypass in AngleEnforcedCamera.native.tsx.
type StubAdapter struct{}

// NewStubAdapter creates a StubAdapter for development environments.
func NewStubAdapter() *StubAdapter {
	return &StubAdapter{}
}

func (s *StubAdapter) AuthorizeHold(_ context.Context, _ int64, _ string, _ string, _ string) (string, error) {
	return fmt.Sprintf("pi_stub_%s", ulid.New()), nil
}

func (s *StubAdapter) CaptureHold(_ context.Context, _ string, _ int64) (string, error) {
	return fmt.Sprintf("ch_stub_%s", ulid.New()), nil
}

func (s *StubAdapter) ReleaseHold(_ context.Context, _ string) error {
	return nil
}

func (s *StubAdapter) ChargeRentalFee(_ context.Context, _ int64, _ string, _ string, _ string) (string, error) {
	return fmt.Sprintf("ch_stub_%s", ulid.New()), nil
}

func (s *StubAdapter) PayoutHost(_ context.Context, _ int64, _ string, _ string) (string, error) {
	return fmt.Sprintf("tr_stub_%s", ulid.New()), nil
}

func (s *StubAdapter) Refund(_ context.Context, _ string, _ int64) (string, error) {
	return fmt.Sprintf("re_stub_%s", ulid.New()), nil
}

func (s *StubAdapter) CreateConnectedAccount(_ context.Context, _ string) (string, string, error) {
	return fmt.Sprintf("acct_stub_%s", ulid.New()), "https://stub.stripe.com/onboard", nil
}

func (s *StubAdapter) CreateCustomer(_ context.Context, _ string, _ string) (string, error) {
	return fmt.Sprintf("cus_stub_%s", ulid.New()), nil
}

func (s *StubAdapter) CreateSetupIntent(_ context.Context, _ string) (string, error) {
	return fmt.Sprintf("seti_stub_%s_secret_stub", ulid.New()), nil
}
