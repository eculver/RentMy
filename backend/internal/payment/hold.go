package payment

import "time"

// TieredHold calculates the security hold amount based on item value.
// All amounts are in cents (int64). The tiers follow PRD section 7:
//
//	itemValue <= $500      → 100% of item value
//	$501 – $2,000          → $500 + 25% of (value − $500)
//	$2,001 – $5,000        → $875 + 15% of (value − $2,000)
//	$5,001+                → $1,325 hard ceiling
func TieredHold(itemValueCents int64) int64 {
	switch {
	case itemValueCents <= 50_000: // $500
		return itemValueCents
	case itemValueCents <= 200_000: // $2,000
		return 50_000 + (itemValueCents-50_000)*25/100
	case itemValueCents <= 500_000: // $5,000
		return 87_500 + (itemValueCents-200_000)*15/100
	default:
		return 132_500 // $1,325 hard ceiling
	}
}

// GuaranteeGap returns the portion of item value not covered by the hold.
// This amount is covered by the RentMy guarantee fund.
func GuaranteeGap(itemValueCents, holdAmountCents int64) int64 {
	gap := itemValueCents - holdAmountCents
	if gap < 0 {
		return 0
	}
	return gap
}

// RentalFee calculates the rental fee for a given duration.
// Uses daily pricing when duration is ≥24h and a daily rate exists;
// otherwise falls back to hourly pricing.
// All price arguments are in cents. Returns 0 if no applicable price is set.
func RentalFee(pricePerHourCents, pricePerDayCents int64, duration time.Duration) int64 {
	const day = 24 * time.Hour
	if duration >= day && pricePerDayCents > 0 {
		days := int64(duration / day)
		if days == 0 {
			days = 1
		}
		return pricePerDayCents * days
	}
	if pricePerHourCents > 0 {
		hours := int64(duration / time.Hour)
		if hours == 0 {
			hours = 1 // minimum 1 hour
		}
		return pricePerHourCents * hours
	}
	return 0
}

// PlatformFee calculates the platform fee as a fraction of the rental fee.
// takeRateBPS is in basis points (e.g. 2000 = 20%).
func PlatformFee(rentalFeeCents int64, takeRateBPS int) int64 {
	return rentalFeeCents * int64(takeRateBPS) / 10_000
}

// HostPayout is the amount paid to the host: rentalFee minus the platform fee.
func HostPayout(rentalFeeCents, platformFeeCents int64) int64 {
	return rentalFeeCents - platformFeeCents
}

// GuaranteeFundContribution is the portion of the platform fee reserved for the guarantee fund.
// guaranteeRateBPS is in basis points (e.g. 1000 = 10% of the platform fee).
func GuaranteeFundContribution(platformFeeCents int64, guaranteeRateBPS int) int64 {
	return platformFeeCents * int64(guaranteeRateBPS) / 10_000
}

// itemValue returns the best available item value from the listing snapshot, in cents.
// Prefers host_declared_value over estimated_value. Returns 0 if neither is set.
func itemValue(snap ListingSnapshot) int64 {
	if snap.HostDeclaredValue != nil && *snap.HostDeclaredValue > 0 {
		return int64(*snap.HostDeclaredValue * 100)
	}
	if snap.EstimatedValue != nil && *snap.EstimatedValue > 0 {
		return int64(*snap.EstimatedValue * 100)
	}
	return 0
}

// listingPriceCents returns the pricePerHour and pricePerDay in cents from the listing snapshot.
func listingPriceCents(snap ListingSnapshot) (perHour, perDay int64) {
	if snap.PricePerHour != nil {
		perHour = int64(*snap.PricePerHour * 100)
	}
	if snap.PricePerDay != nil {
		perDay = int64(*snap.PricePerDay * 100)
	}
	return
}
