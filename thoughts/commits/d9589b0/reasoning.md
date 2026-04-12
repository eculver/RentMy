# Commit d9589b0 — feat: add guarantee fund accounting

## What

Introduced the `guaranteefund` package implementing the platform guarantee fund ledger, reserve ratio monitoring, and loss ratio tracking as specified in PRD section 7.

## Why

The guarantee fund is the platform's financial safety net for damage shortfalls not covered by security holds. Without it, hosts bear 100% of uncovered losses, which erodes trust and platform growth. The tiered hold system (task 3.2) intentionally leaves a "guarantee gap" — the difference between item value and hold amount — that the fund covers.

## Key Decisions

1. **Append-only ledger with running balance**: Each `guarantee_fund_entries` row stores `balance_after` to enable O(1) balance lookups without summing all entries. This trades a small write-time cost for fast read paths.

2. **Four entry types**: CONTRIBUTION (from each booking's platform fee), CLAIM (disbursement for damage), CARD_RECOVERY (renter card charged to replenish), COLLECTIONS_REFERRAL (write-off to collections). These cover the complete fund lifecycle.

3. **Reserve ratio tiers**: NORMAL (>=15%), ALERT (>=10%), RESTRICT_HIGH_VALUE (>=5%), RESTRICT_ALL_GAP (<5%). These map to progressive platform restrictions as the fund health degrades.

4. **River job workers**: FundHealthCheckWorker (hourly) and LossRatioCheckWorker (daily) monitor fund health and log alerts. Workers are registered but periodic scheduling was deferred to task 6.9.

5. **Separate package from payment**: The guarantee fund has its own domain logic, distinct from payment processing. Keeping it separate allows independent testing and avoids bloating the already-large payment package. However, the initial implementation duplicated some types and operations in both packages — this was a known gap addressed in task 6.9.

## Trade-offs

- Amounts stored as floats in the DB (dollars) and converted to int64 (cents) in Go. This matches the existing transaction table convention but introduces float precision risk. Acceptable for the current scale; should migrate to integer cents storage if fund volume grows significantly.
- Tests only covered threshold logic, not database operations — addressed by task 6.9.
