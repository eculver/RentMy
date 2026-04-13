# Commit Reasoning: 2bb354a

**Task:** 8.4 — Fix: Feed + Listing Detail + Checkout Bugs  
**Date:** 2026-04-11

## Why these changes

Nine bugs were documented in the Phase 8.3 static audit. This commit addresses them all in one pass because they are independent (no cascading dependencies between fixes) and together they bring the checkout flow from completely blocked to testable end-to-end.

**B1 (migration):** Using a goose SQL migration rather than a seed script is the right approach — it's versioned, reversible, and runs automatically on `make dev`. The conditional `WHERE identity_status = 'PENDING'` makes it idempotent.

**B2 (Stripe):** The ephemeral key is a distinct Stripe object from the SetupIntent — passing the SetupIntent client secret as the ephemeral key is documented by Stripe as an error that causes the payment sheet to fail silently. The fix requires both a backend interface change (the adapter must now create two Stripe objects per setup call) and a frontend type fix.

**B3 (routing):** The `confirmation.tsx` screen was already fully implemented with the right UI — wiring it up just required changing the `router.replace` target and passing the booking result fields as params. All required fields (`holdAmount`, `rentalFee`, `platformFee`, `totalImpact`, `scheduledStart`, `scheduledEnd`) are available at the call site.

**B7 (loading state):** Gating `CostBreakdown` on `holdEstimate != null` is simpler and more correct than showing $0.00 — it eliminates a confusing transient state without adding conditional field logic inside the component.

**B9 (photos):** A second query in `FindByID` is the cleanest approach — it avoids GROUP BY complexity on a wide listing row and keeps `scanListing` unchanged. The `[]string{}` default (not nil) ensures the JSON response always contains `"photos":[]` rather than `"photos":null`, which is friendlier for mobile clients.
