# Task 8.8 Handoff — Fix: Booking + Handoff Bugs

**Status:** Completed  
**Commit:** c33a4ec  
**Branch:** task-8.8-fix-booking-handoff-bugs  
**Date:** 2026-04-11

## What was done

Resolved all 8 bugs documented in the Task 8.7 audit of booking and handoff screens.

## Bugs fixed

| ID | Severity | Fix summary |
|----|----------|-------------|
| BUG-BH-1 | Medium | `booking-request.tsx`: call `setAmounts` in auto-end branch of `onChangeStart` |
| BUG-BH-2 | Low | Remove hardcoded `mx-4`/`mb-3` from `IncomingRequest` and `BookingCard` |
| BUG-BH-3 | Medium | Maps URL now uses `ll=lat,lng` from backend JOIN; falls back to `maps://` |
| BUG-BH-4 | Medium | "Report an issue" navigates to `/(tabs)/(rentals)/dispute` instead of Alert |
| BUG-BH-5 | Low | Message button navigates to `/(tabs)/(messages)/conversation` with `transactionId` |
| BUG-BH-6 | Low | Backend enriches booking response with `renterName`; `IncomingRequest` receives it |
| BUG-BH-7 | Low | `isRenter` defaults to `true` during data load (more restrictive, same result) |
| BUG-BH-8 | Low | E.164 regex gates PINDisplay Send button (`/^\+[1-9]\d{6,14}$/`) |

## Files changed

**Backend:**
- `backend/internal/booking/model.go` — added `RenterName *string`, `ListingLat *float64`, `ListingLng *float64` to `Booking` struct
- `backend/internal/booking/repository.go` — `FindByID` LEFT JOINs `users` and `listings` to populate enriched fields

**Mobile:**
- `mobile/lib/hooks/useBooking.ts` — added `renterName?`, `listingLat?`, `listingLng?` to `Booking` interface
- `mobile/app/(tabs)/(feed)/booking-request.tsx` — BUG-BH-1
- `mobile/app/(tabs)/(feed)/booking-status.tsx` — BUG-BH-3, BUG-BH-5, BUG-BH-6
- `mobile/app/(tabs)/(feed)/active-rental.tsx` — BUG-BH-3, BUG-BH-4
- `mobile/components/booking/IncomingRequest.tsx` — BUG-BH-2
- `mobile/components/booking/BookingCard.tsx` — BUG-BH-2
- `mobile/components/screens/CheckInScreen.native.tsx` — BUG-BH-7
- `mobile/components/screens/CheckOutScreen.native.tsx` — BUG-BH-7
- `mobile/components/handoff/PINDisplay.tsx` — BUG-BH-8

## Verification

- `cd backend && go vet ./...` — clean
- `cd backend && go build -o /dev/null ./cmd/server` — clean
- `cd mobile && npx tsc --noEmit` — 2 pre-existing profile errors only (not introduced here)
- `cd mobile && npx jest` — 91/91 tests pass, 11 suites

## Backend enrichment details

`FindByID` now LEFT JOINs two tables:
- `LEFT JOIN users u ON u.id = t.renter_id` → `NULLIF(u.name, '')` → `RenterName`
- `LEFT JOIN listings l ON l.id = t.listing_id` → `ST_Y(l.location::geometry)`, `ST_X(l.location::geometry)` → `ListingLat`, `ListingLng`

Both are nullable (`*string`, `*float64`) to handle missing renter name or unlisted location. The list endpoints (`FindByRenterID`, `FindByHostID`) are unchanged — they don't need enriched fields.

## Next task

**Task 8.9 — Audit: Messaging** — static code audit of messaging screens and components.

## Branch mode

Graphite mode — `gt create` succeeded, `gt submit` for push.
