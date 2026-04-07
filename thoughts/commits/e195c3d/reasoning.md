# Commit e195c3d Reasoning

**Task:** 7.4 — Referral System (backend + RN)

Implemented the full referral growth system as specified in the phase-7 plan.

**Key decisions:**

1. **Pre/post River pattern** — Same as other services (reputation, guarantee fund). `referralSvcPre` is created with `nil` riverClient, workers registered, then full service created post-River and copied with `*referralSvcPre = *referralSvc`.

2. **Interface-driven service** — `RepositoryInterface` allows unit tests with fakes without testcontainers. All 10 service unit tests run in <500ms.

3. **Narrow interface for cross-package hooks** — Rather than importing the referral package from user and booking, defined `user.ReferralApplier` (`ApplyCode`) and `booking.referralSvc` (`OnFirstRentalCompleted`) as local interfaces. The `*referral.Service` satisfies both structurally.

4. **Best-effort referral apply on registration** — A bad/expired code does NOT fail registration. Logs and discards the error.

5. **Fraud checks layered** — Shared device at apply time (prevents fake signup) AND at payout time. Velocity check at payout time only (>5 payouts/30 days → manual review, not block).

6. **Mobile copy uses RN `Clipboard`** — `expo-clipboard` not in this project's deps; used the legacy `Clipboard` from `react-native` directly. Test uses state feedback ("Copied!" text) to avoid native module mocking complexity.
