# Commit 31e3fbb — Post-rental flow (task 6.8)

## Why this commit

Task 6.8 is the last planned mobile task in Phase 6. After a rental completes, users need a coherent UI to rate their counterparty, track hold release status, file a dispute, and monitor dispute progress. Without these screens the rating and dispute backend services (tasks 6.4 and 6.2) have no consumer surface.

## What changed and why

- **`(rentals)` tab** — A dedicated tab groups all of a user's bookings (renter + host) in one place with status badges and quick actions. Previously there was no rentals tab; booking-related screens lived under `(feed)` as push-only destinations. The new tab surfaces these bookings proactively.

- **`useDispute.ts`** — TanStack Query hooks wrapping the dispute CRUD API. `refetchInterval` polling is conditional on dispute status so open disputes update automatically without constant polling for resolved ones.

- **`usePostRental.ts`** — Orchestration hook that detects completion, tracks whether the current user has rated, and manages rating prompt visibility. Listens for `hold-released` and `damage-detected` Pusher events to invalidate relevant query caches.

- **`notifications.ts`** — Type-safe push notification payload definitions and a `NOTIFICATION_ROUTES` map from notification type to Expo Router path, enabling a generic notification handler to navigate the user to the right screen.

- **`HoldStatusCard`** — Bar chart visualization of the hold allocation buckets (late fee, damage charge, reserve, released). Provides immediate transparency into where the money went.

- **`PhotoDiffResult`** — Side-by-side check-in/out photo pairs with classification badges. Placeholder for when the CV pipeline result is available; gracefully shows "pending" state when pairs is empty.

- **`DisputeTimeline`** — Vertical timeline component with four fixed steps (filed → evidence → under review → resolved), highlighting the current step.

- **Screen flow**: return-confirmation → rate (or dispute) → dispute-status. Each screen is self-contained and can also be reached via push notification deep links.

## Decisions

- **Photos via `(feed)` check-out**: the return confirmation screen links out to `(feed)/active-rental` and `(feed)/check-out` for the actual photo capture; it does not duplicate that UI.
- **No `(rentals)` tab badge**: unlike messages we have no unrated-count API yet; a badge can be added in a future task once the backend exposes it.
- **Hold allocation zeros**: the backend transaction API does not yet expose hold allocation breakdown per line item. `HoldStatusCard` renders gracefully with zero values and will become live once task 7.x adds that endpoint.
