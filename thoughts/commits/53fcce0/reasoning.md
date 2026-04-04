# Commit 53fcce0 — Listing detail screen (Task 2.6)

## Why this commit

Task 2.6 closes the discovery-to-detail navigation loop. Renters can now tap any listing card (feed, search, map) and land on a full detail page before deciding to rent. Without this screen, the "Rent Now" checkout flow in Task 2.7 has no entry point.

## Key decisions

- Host info passed via route params to avoid a missing host profile endpoint — avoids a blocking API gap without adding tech debt (easy to upgrade later).
- No calendar library — read-only time slots don't justify the bundle cost. Plain list view is sufficient.
- Hold explainer hidden for hosts — UX clarity: a host seeing their own hold amount is confusing and irrelevant.
- Single photo from thumbnailUrl — listing API returns no media list in Phase 2; passing the discovery thumbnail keeps the gallery functional without a new endpoint.
