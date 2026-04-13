# Commit 5c3b4eb — fix: resolve 9 rentals/disputes/ratings bugs from Phase 8.11 audit

## Why this commit

Task 8.12 fixes 9 bugs found during the 8.11 static audit of the rentals, disputes, and ratings screens. Three were critical protocol mismatches between Go's JSON serialization and TypeScript interfaces.

## Key decisions

**BUG-1**: Added JSON struct tags to `rating.Rating`. Go's default serialization of unexported-prefix fields (ID, FromUserID, etc.) produces PascalCase output. The frontend `Rating` interface expected camelCase. Without tags, `hasRated` was always false — users saw the "Rate" button even after rating.

**BUG-2**: Chose to update the frontend `DisputeStatus` type to match the backend exactly (8 values) rather than renaming backend constants. Added a `STATUS_STEP` lookup table in `DisputeTimeline` to map the 8 backend values onto 4 visual steps, avoiding the previous `indexOf` fallback that showed all unknown statuses as "Resolved."

**BUG-3**: Updated backend JSON tags to match frontend field names (`route`→`escalationRoute`, `chargeAmount`→`damageChargeCents`, `confidence`→`agentConfidence`, `reviewerId`→`resolvedBy`). For the `agentDecisionId` vs `agentDecision` semantic mismatch (ID vs verdict string), the frontend interface was updated to `agentDecisionId` and the `isInconclusive` check correctly moved to use `dispute.status === "INCONCLUSIVE"`.

**BUG-6**: Rather than show misleading $0.00 hold data, the `HoldStatusCard` is hidden until `authorizedCents > 0`. The backend doesn't yet expose per-rental hold allocation fields on the booking endpoint.

**BUG-7**: `PhotoDiffResult` was integrated into `dispute-status.tsx` using the structured `evidence` field (now typed as `DisputeEvidence`) on the Dispute response. Photo pairs are built by index-matching `checkInMedia` and `checkOutMedia` arrays from the evidence package.

**BUG-8**: The INCONCLUSIVE reprompt now navigates to the check-out camera screen — the best available photo capture surface before a dedicated evidence re-upload flow is built.
