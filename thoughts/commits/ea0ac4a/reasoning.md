# Commit ea0ac4a — feat: add Maestro E2E dispute & rating flows (task 9.8)

## Why this commit

Task 9.8 extends the Phase 9 E2E suite with dispute and rating flows — the two
post-rental lifecycle actions that follow a completed or active rental. These
flows complete coverage of all major user journeys in the app.

## Key decisions

### Backend seed endpoint (`POST /api/v1/test/dispute`)

The `view-dispute-status.yaml` flow needs a booking that is already in the
DISPUTED state when the test starts. Filing a dispute via the UI (as
`file-dispute.yaml` does) would make both flows identical. Instead, the
backend seed endpoint:
1. Creates a COMPLETED transaction (yesterday noon → +4h)
2. Inserts a PENDING dispute row directly
3. Updates the transaction status to DISPUTED

This allows `view-dispute-status.yaml` to go straight to the Rentals list and
see a "Dispute open" badge on the row, then tap to see the dispute-status screen.

### Why `file-dispute.yaml` starts from an ACTIVE booking

The dispute filing screen is reached from the active-rental screen via
`btn-report-issue`. The plan explicitly says: "Login as renter → Active rental →
Tap 'Report an issue'". This is the natural user path and exercises the
full filing UI (reason selection, description input, submit, redirect to status).

### TestID placement

- `screen-dispute`, `screen-dispute-status`, `screen-rate` — on SafeAreaView
  wrappers so Maestro can assert the screen is visible even before inner content loads.
- `rating-bubble-{BUBBLE}` — includes the bubble value in the ID so flows can
  tap a specific bubble by name without fragile text matching.
- `dispute-reason-{VALUE}` — same pattern for reason selection.
- `dispute-timeline` on the outer DisputeTimeline `View` — a single stable
  anchor for asserting the timeline section is rendered.
