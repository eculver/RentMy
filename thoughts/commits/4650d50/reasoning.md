# Commit 4650d50 — ListingService

## Why
Task 1.3 of Phase 1 (Supply). UserService (1.1) and MediaService (1.2) are both complete. Listings are the core supply-side entity — hosts need to create and manage items before the demand-side (Discovery, Phase 2) can surface them.

## What
- Full listing CRUD with owner-scoped update/media-attach
- 7-day ceiling enforced at service layer (not just validation tags) so it applies to both create and update
- PostGIS GEOGRAPHY(POINT) for listing location — required for Phase 2 proximity queries
- Custom `Duration` JSON type keeps the API readable ("168h" not 604800000000000)
- 15 unit tests covering all business rules without touching Postgres or S3

## Trade-offs
- `AttachMedia` touches the `media` table from the listing repository — acceptable because it expresses a listing ownership relationship. Avoids a circular import (listing importing media).
- Pagination is offset-based (simple). Will work for Phase 1 scale; cursor-based can be added in Phase 2 if needed.
