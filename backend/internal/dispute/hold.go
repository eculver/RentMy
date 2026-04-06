package dispute

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/giits/rentmy/backend/internal/payment"
)

// HoldService manages hold allocation captures for dispute resolution.
// It delegates to the payment service for actual Stripe operations.
type HoldService struct {
	paymentSvc *payment.Service
}

// NewHoldService creates a HoldService.
func NewHoldService(paymentSvc *payment.Service) *HoldService {
	return &HoldService{paymentSvc: paymentSvc}
}

// CaptureForDamage captures funds from the hold for damage charges.
// DisputeAgent has no damage reserve cap — it can capture whatever remains.
func (h *HoldService) CaptureForDamage(ctx context.Context, transactionID string, amount int64) (string, error) {
	chargeID, err := h.paymentSvc.CaptureFromHold(ctx, transactionID, amount, payment.CaptureReasonDamage)
	if err != nil {
		return "", fmt.Errorf("capture for damage: %w", err)
	}
	slog.Info("dispute: captured hold for damage",
		"transactionId", transactionID,
		"amount", amount,
		"chargeId", chargeID,
	)
	return chargeID, nil
}

// ReleaseRemaining releases all unused hold funds back to the renter.
func (h *HoldService) ReleaseRemaining(ctx context.Context, transactionID string) error {
	if err := h.paymentSvc.ReleaseHold(ctx, transactionID); err != nil {
		return fmt.Errorf("release remaining hold: %w", err)
	}
	slog.Info("dispute: released remaining hold", "transactionId", transactionID)
	return nil
}

// CaptureAndEscalate handles the case where damage exceeds the remaining hold.
// It captures all remaining hold, then attempts to charge the renter's card for
// the difference. If the card charge fails, it draws from the guarantee fund.
func (h *HoldService) CaptureAndEscalate(ctx context.Context, transactionID string, totalDamage int64) error {
	txn, err := h.paymentSvc.GetTransaction(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("get transaction: %w", err)
	}

	remaining := txn.HoldAllocation.Remaining
	if remaining > 0 {
		if _, err := h.CaptureForDamage(ctx, transactionID, remaining); err != nil {
			return fmt.Errorf("capture remaining hold: %w", err)
		}
	}

	difference := totalDamage - remaining
	if difference <= 0 {
		return nil
	}

	slog.Warn("dispute: damage exceeds hold, attempting card charge",
		"transactionId", transactionID,
		"totalDamage", totalDamage,
		"holdRemaining", remaining,
		"difference", difference,
	)

	if err := h.paymentSvc.ChargeForDamageOverflow(ctx, transactionID, difference); err != nil {
		slog.Warn("dispute: card charge failed, drawing from guarantee fund",
			"transactionId", transactionID,
			"amount", difference,
			"error", err,
		)
		if fundErr := h.paymentSvc.ClaimGuaranteeFund(ctx, transactionID, difference); fundErr != nil {
			return fmt.Errorf("guarantee fund claim failed: %w (card charge: %w)", fundErr, err)
		}
	}

	return nil
}
