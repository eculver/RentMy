# Commit Reasoning — c33a4ec

**Task:** 8.8 — Fix: Booking + Handoff Bugs  
**Branch:** task-8.8-fix-booking-handoff-bugs

## Why these changes

Eight bugs were documented in the Task 8.7 audit. This commit resolves all eight.

### BUG-BH-1 (Medium)
The `onChangeStart` handler in `booking-request.tsx` auto-set the end date when no end existed but forgot to call `setAmounts`. Result: `CostBreakdown` rendered with correct dates but $0 fees. Fixed by calling `setAmounts(hold, fee)` in the same branch that calls `setSchedule(s, autoEnd)`.

### BUG-BH-2 (Low)
`IncomingRequest` and `BookingCard` both hardcoded `mx-4` (and `IncomingRequest` also had `mb-3`). Their parent containers already apply `paddingHorizontal: 16`, causing 32px double-indentation. Removed the hardcoded classes so parents control spacing.

### BUG-BH-3 (Medium)
Both `booking-status.tsx` and `active-rental.tsx` built Apple Maps URLs using the listing ULID as a search query (`?q=RentMy+pickup+<ulid>`), which returns no results. Fixed by:
1. Adding `RenterName *string`, `ListingLat *float64`, `ListingLng *float64` to the backend `Booking` struct.
2. Updating `FindByID` to LEFT JOIN `users` (renter name) and `listings` (ST_Y/ST_X for coordinates).
3. Adding `renterName?`, `listingLat?`, `listingLng?` to the frontend `Booking` interface.
4. Using `maps://maps.apple.com/?ll={lat},{lng}&q=...` when coords available, `maps://` as fallback.

### BUG-BH-4 (Medium)
`handleReportIssue` in `active-rental.tsx` showed a placeholder Alert. The dispute screen already exists at `/(tabs)/(rentals)/dispute`. Fixed by replacing the Alert with `router.push` to that route with `transactionId` param.

### BUG-BH-5 (Low)
The "Message host/renter" button navigated to `/(tabs)/(messages)` (inbox tab), forcing the user to find the right conversation manually. Fixed to navigate to `/(tabs)/(messages)/conversation` with `transactionId` and `otherPartyName` params, matching what the conversation screen already expects.

### BUG-BH-6 (Low)
`IncomingRequest` received no `renterName`/`renterReputation` props, showing a greyed avatar with no identity. Resolved as part of BUG-BH-3's backend enrichment. `booking-status.tsx` now passes `booking.renterName` to `IncomingRequest`.

### BUG-BH-7 (Low)
`isRenter` was derived as `bookingData !== undefined && user?.id === ...`, which evaluates to `false` during data load. `useProximity` was then initialized with the wrong role. Fixed by defaulting to `true` (the more restrictive role) while loading: `bookingData === undefined ? true : user?.id === ...`.

### BUG-BH-8 (Low)
`PINDisplay` accepted any non-empty string for the phone number. An unformatted number would fail at the API with a cryptic error. Added E.164 regex validation (`/^\+[1-9]\d{6,14}$/`) and tied the Send button's enabled state to `isValidPhone`.
