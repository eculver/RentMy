# RentMy — PRD v6

> **Changelog from v5:**
> - §6: Dual-score model — `reputationScore` (earned trust, 0–1000) + per-transaction `riskScore` (0–100). Removed `depositAmount`. Added `GuaranteeFundEntry` model, `HoldAllocation` ledger, agent `outcomeId` for learning loop
> - §7: Added 7-day rental ceiling (Stripe hold expiry). Hold allocation ledger coordinates LateReturnAgent + DisputeAgent. Guarantee fund reserve ratio requirement
> - §8: Rewritten — reputation builds from 0, risk computed per-transaction. Normalized ranking inputs
> - §9: WiFi signal is compound-only (never scores alone)
> - §11.1: CV preprocessing (OpenCV + SAM) separated from LLM reasoning. Angle diversity enforcement via gyroscope
> - §13: Ranking inputs normalized to [0,1]. Removed undefined "conversion likelihood"
> - §17: Complete booking state machine with all valid transitions
> - §19: LateReturnAgent coordinates with hold allocation ledger
> - §22: Fixed stale "full-value hold" copy
> - §5: Removed "photo diffing" from MediaService
> - §31: New — Agent Learning Framework (outcome tracking, confidence calibration, prompt evolution)

## 0. Summary

**Product:** Mobile-only, hyperlocal P2P rental marketplace
**Promise:** Rent anything nearby, fast, from a verified host with enforced payment protection and renter liability
**Core Differentiator:** Proximity-verified handoff + camera-only listing proof + AI-native operations with multi-agent orchestration
**Company Model:** Small AI-native team. Agents run the platform. Humans watch the dashboard and make strategic decisions.

---

## 1. Goals / Non-Goals

### Goals

- Achieve ≥70% successful booking rate on shown inventory
- Enable hyperlocal rentals with real-time drive-time estimates
- Maintain <1% fraud/abuse rate
- Drive host activation ≥60% (publish within first session)
- Full autonomous agent resolution for disputes, verification, and ops

### Non-Goals (v1)

- Web support
- Shipping / non-local rentals
- Pro Host subscription tier (future roadmap)
- Category taxonomy or dropdowns
- Human-in-the-loop for dispute resolution (except via escalation gate for low-confidence or high-dollar decisions — see §20)

---

## 2. Personas

- **Renter (Demand):** Wants fast, trustworthy access to any gear today
- **Host (Supply):** Wants low-effort monetization with protection

---

## 3. Core Flows

### 3.1 Renter: Discovery → Booking → Handoff → Return

1. Open app → three discovery surfaces: feed, search, map
2. Browse "Available near you" feed, search by keyword ("kayak"), or explore map
3. Listing page → trust signals, availability, estimated drive time, hold amount (tiered, not full value) clearly displayed
4. Tap "Rent Now" → checkout screen shows: rental fee, hold amount (tiered per item value — see §7), rental duration, total card impact
5. First-time renter: KYC triggered (ID + selfie via API, AI agent validates)
6. Confirm → booking request sent to host
7. Host accepts (or auto-decline after configurable timeout)
8. Navigate to pickup → proximity verification (GPS ≤100m + host-generated PIN)
9. Both parties capture check-in photos → rental starts
10. Return → proximity verify + both parties capture return photos → rental closes
11. Photo diff pipeline (§11.1) compares check-in/return photos → release hold or route to dispute flow via escalation gate

### 3.2 Host: Onboard → List → Manage

1. Tap "+ List Item" (FAB)
2. Earn screen: value props + local earnings estimate
3. Set pickup location (home or nearby — exact hidden from renters, shown fuzzed)
4. Camera-only capture: 3–5 photos + optional video (in-app only, no uploads)
5. AI detection + autofill: identifies item, estimates value, suggests pricing, generates semantic data for search
6. Host sets: price (hourly/daily), min/max rental duration, availability windows
7. If host overrides AI estimated value by >100%: AI agent prompts justification, evaluates autonomously
8. Verification: ID + selfie (can publish as PENDING while verification processes)
9. Publish → share prompt (referral incentive)

---

## 4. Tech Stack

| Layer | Technology |
|---|---|
| Mobile | React Native |
| Backend | Go |
| Database | PostgreSQL + PostGIS |
| Real-time State | Redis |
| Real-time Events | SSE via Pusher |
| Media Storage | S3-compatible object storage |
| Payments | Stripe Connect (swappable adapter layer) |
| KYC | Third-party API (Stripe Identity or equivalent, swappable) |
| AI Models | Model router — cheap models for simple tasks, Anthropic Claude for complex decisions |
| Deployment | Containerized, cloud-agnostic |

---

## 5. System Architecture

### Modular Monolith (Go)

**Core Services:**

- **UserService** — registration, profiles, reputation scores, identity status
- **ListingService** — creation, AI autofill, media management, semantic indexing
- **BookingService** — request/accept/decline flow, auto-decline timer, duration management
- **PaymentService** — abstraction layer over Stripe (swappable), pre-auth holds, escrow, payouts
- **ProximityService** — GPS verification, PIN generation, BLE (v2)
- **MessagingService** — in-app renter-host communication, stored for dispute evidence
- **NotificationService** — push, in-app, SMS fallback, user-managed settings
- **MediaService** — camera capture, original storage, thumbnail generation
- **DiscoveryService** — search (semantic), map (PostGIS), feed (ranked), drive-time estimation

**Agent Services:**

- **RiskAgent** — per-transaction risk scoring, collusion detection, velocity monitoring
- **VerificationAgent** — KYC review, ID + selfie validation via API
- **AppraisalAgent** — item identification, value estimation, value override review
- **DisputeAgent** — evidence gathering, photo diff analysis, autonomous resolution
- **AgreementAgent** — custom agreement section generation within legal template
- **LateReturnAgent** — monitors active rentals, auto-charges, escalation decisions
- **OpsAgent** — platform health monitoring, fraud spike detection, supply gap analysis, host churn signals
- **FraudAgent** — collusion detection, multi-account detection, pattern analysis across transaction history

**Agent Model Router:**

Each agent is assigned a model tier based on decision complexity:

- **Cheap model (fast, low-cost):** Notifications, summaries, simple classification, search matching
- **Claude (complex reasoning):** Disputes, value appraisal, fraud pattern detection, agreement generation, any decision that touches money

**Complete Task-to-Model-Tier Matrix:**

| Agent | Task | Model Tier |
|---|---|---|
| AppraisalAgent | Item identification from photos | Claude (Sonnet) |
| AppraisalAgent | Tag generation | Cheap model |
| AppraisalAgent | Value override justification review | Claude (Sonnet) |
| DisputeAgent | Evidence analysis + decision | Claude (Sonnet) |
| DisputeAgent | Evidence summary for human review | Cheap model |
| RiskAgent | Per-transaction risk scoring | Cheap model (rule-based with ML signal) |
| VerificationAgent | KYC result interpretation | Cheap model |
| AgreementAgent | Custom clause generation | Claude (Sonnet) |
| AgreementAgent | Template rendering | No model (template engine) |
| LateReturnAgent | Escalation decision | Claude (Sonnet) |
| LateReturnAgent | Late fee calculation | No model (deterministic) |
| FraudAgent | Pattern detection across history | Claude (Sonnet) |
| FraudAgent | Signal aggregation | Cheap model |
| OpsAgent | Anomaly detection | Cheap model |
| OpsAgent | Health report generation | Cheap model |
| NotificationService | Notification text generation | Cheap model |
| DiscoveryService | Semantic search matching | Cheap model (embeddings) |

---

## 6. Data Models

```
User {
  id: ULID
  identityStatus: VERIFIED | PENDING | REJECTED
  reputationScore: number        // 0–1000, higher = more trusted. Starts at 0. Earned over time
  deviceFingerprint: string
  riskFlags: string[]
  paymentMethods: PaymentMethod[]
  notificationPreferences: NotificationSettings
  createdAt: timestamp
  lastActiveAt: timestamp
}

Listing {
  id: ULID
  hostId: UserID
  title: string
  description: string
  aiGeneratedTags: string[]     // semantic data for search
  estimatedValue: number        // AI appraised
  hostDeclaredValue: number     // if overridden
  valueJustification: string    // if override >100%
  pricePerHour: number
  pricePerDay: number
  minDuration: duration
  maxDuration: duration         // platform ceiling: 7 days (see §7)
  location: GeoPoint            // stored exact, shown fuzzed
  availability: TimeSlots[]
  media: Media[]                // original + thumbnails
  hasVideo: boolean
  status: ACTIVE | PENDING | FLAGGED | SUSPENDED
  createdAt: timestamp
}

Transaction {
  id: ULID
  renterId: UserID
  hostId: UserID
  listingId: ListingID
  rentalFee: number
  holdAmount: number            // tiered pre-auth (per hold tier table)
  itemValue: number             // full estimated value (for guarantee fund calc)
  guaranteeGap: number          // itemValue - holdAmount (covered by fund)
  riskScore: number             // per-transaction risk (0–100, computed at booking time)
  escrowStatus: HELD | RELEASED | CHARGED
  holdStatus: AUTHORIZED | RELEASED | CAPTURED | PARTIALLY_CAPTURED
  holdAllocation: HoldAllocation  // tracks what the hold has been spent on
  guaranteeFundCharged: number  // amount drawn from guarantee fund (0 if none)
  agreementSnapshot: JSON       // immutable
  checkInMedia: Media[]
  checkOutMedia: Media[]
  photoDiffResult: ENUM         // NO_CHANGE | COSMETIC | FUNCTIONAL | MISSING | INCONCLUSIVE
  photoDiffConfidence: number   // 0–1.0
  checkInProximity: ProximityProof
  checkOutProximity: ProximityProof
  scheduledStart: timestamp
  scheduledEnd: timestamp
  actualStart: timestamp
  actualEnd: timestamp
  status: REQUESTED | ACCEPTED | DECLINED | AUTO_DECLINED |
          ACTIVE | COMPLETED | DISPUTED | CANCELLED
  cancelledBy: RENTER | HOST | null
  cancellationFee: number
}

HoldAllocation {
  totalAuthorized: number       // original hold amount
  capturedForLateFees: number   // amount taken by LateReturnAgent
  capturedForDamage: number     // amount taken by DisputeAgent
  damageReserve: number         // held back from late fee captures (configurable %)
  released: number              // amount released back to renter
  remaining: number             // totalAuthorized - all captures - all releases
}

Message {
  id: ULID
  transactionId: TransactionID
  senderId: UserID
  content: string
  createdAt: timestamp
}

Rating {
  id: ULID
  transactionId: TransactionID
  fromUserId: UserID
  toUserId: UserID
  bubbles: string[]             // predefined tags: "good communication", "on time", etc.
  createdAt: timestamp
}

ProximityProof {
  gpsDistance: number
  pin: string
  verified: boolean
  method: GPS | BLE | SMS_FALLBACK
  timestamp: timestamp
  deviceId: string
}

AgentDecision {
  id: ULID
  agentType: RISK | VERIFICATION | APPRAISAL | DISPUTE |
             AGREEMENT | LATE_RETURN | FRAUD | OPS | HUMAN_OVERRIDE
  transactionId: TransactionID  // nullable
  userId: UserID                // nullable
  input: JSON
  decision: JSON
  model: string                 // which LLM was used (null for HUMAN_OVERRIDE)
  confidence: number
  escalated: boolean            // true if routed to human review queue
  escalationReason: string      // nullable — why it was escalated
  reviewedBy: UserID            // nullable — human reviewer ID
  overrideOf: AgentDecisionID   // nullable — links to the agent decision this overrides
  outcomeId: ULID               // nullable — links to eventual outcome for learning loop (see §31)
  outcomeCorrect: boolean       // nullable — was this decision validated as correct by outcome?
  createdAt: timestamp
}

GuaranteeFundEntry {
  id: ULID
  transactionId: TransactionID
  type: CONTRIBUTION | CLAIM | CARD_RECOVERY | COLLECTIONS_REFERRAL
  amount: number                // positive for contributions/recoveries, negative for claims
  balanceAfter: number          // running fund balance after this entry
  createdAt: timestamp
}
```

---

## 7. Payments & Financial Logic

### Model: Tiered Hold (Hotel Model) + Escrow + Platform Guarantee

**Design principle:** Renter's card is never blocked by a hold larger than necessary. The platform carries the gap via a guarantee fund backed by take-rate margin. This is the hotel model — hold enough to cover incidentals, insure the rest.

**Hold Tiers:**

| Item Value | Hold Amount | Gap Coverage |
|---|---|---|
| ≤ $500 | 100% of item value | N/A — fully held |
| $501 – $2,000 | $500 + 25% of value above $500 | Platform guarantee fund |
| $2,001 – $5,000 | $875 cap + 15% of value above $2,000 | Platform guarantee fund |
| $5,001+ | $1,325 cap (hard ceiling) | Platform guarantee fund + right to pursue renter via collections |

**Platform Guarantee Fund:**

- Funded by a configurable % of platform take rate (start at 10% of take, tunable)
- Covers the gap between hold amount and item value when damage exceeds the hold
- If fund payout exceeds hold, platform has contractual right to charge renter's card for the difference (agreement terms)
- If charge fails, platform can pursue via collections (terms accepted at KYC)
- Fund health monitored by OpsAgent — alerts if claims trend toward insolvency
- All transactions logged in GuaranteeFundEntry ledger (see §6)

**Reserve ratio requirement:**

The fund must maintain a balance ≥ 15% of total outstanding guarantee gaps across all active rentals. "Outstanding" = sum of `guaranteeGap` on all transactions with status `ACTIVE`.

| Fund health | Action |
|---|---|
| Balance ≥ 15% of outstanding gaps | Normal operation |
| Balance 10–15% of outstanding gaps | OpsAgent alert. Increase `guaranteeRate` by 5% |
| Balance 5–10% of outstanding gaps | Restrict new listings with `estimatedValue` > $2,000 |
| Balance < 5% of outstanding gaps | Restrict all new bookings where `guaranteeGap` > $500 until fund recovers |

**Loss ratio tracking:**

From day one, track: `lossRatio = totalClaims / totalContributions` (rolling 90-day window). Target loss ratio < 0.6. If trending above 0.6 for 30+ days, OpsAgent recommends: increase `guaranteeRate`, raise hold tier thresholds, or tighten high-value listing requirements.

**On booking confirmation:**

1. **Tiered pre-auth hold** placed on renter's card (per table above)
2. **Rental fee** charged to renter
3. Rental fee held in escrow

**The renter sees the hold amount, rental fee, and total card impact clearly on the checkout screen before confirming.**

If the renter's card cannot support the hold, the booking is blocked.

### On successful return:

1. AI agent diffs check-in/return photos (see §11.1 Photo Diff Pipeline)
2. No damage → hold released, escrow released to host
3. Damage detected → DisputeAgent evaluates:
   - Damage ≤ hold remaining → capture from hold via HoldAllocation, release remainder
   - Damage > hold remaining → capture full remaining hold + charge difference to card + guarantee fund covers any shortfall

### Hold Allocation:

The hold is a shared resource. LateReturnAgent and DisputeAgent both draw from it, coordinated via the HoldAllocation ledger on each Transaction.

**Damage reserve:** When LateReturnAgent captures late fees, it must leave a configurable percentage of the original hold untouched as a damage reserve (default: 40%). Late fee captures are capped at `holdAmount * (1 - damageReserveRate)`. This ensures the DisputeAgent always has funds available for damage claims even after late fees.

**Read-before-write:** Both agents read `holdAllocation.remaining` before any capture. All captures are atomic database operations (SELECT FOR UPDATE).

**Concurrency isolation:** Hold allocation operations use `READ COMMITTED` isolation (Postgres default) with explicit `SELECT ... FOR UPDATE` row-level locking on the transaction row. This prevents LateReturnAgent and DisputeAgent from simultaneously modifying the same hold allocation. All capture operations must: (1) lock the transaction row, (2) read current `remaining`, (3) validate capture does not exceed `remaining`, (4) update `hold_allocation` JSON, (5) commit — all within a single pgx transaction.

**Guarantee fund depletion safeguards:** The fund balance cannot go negative. If a claim would exceed remaining balance, only the available amount is disbursed. The remaining damage amount is charged to the renter's card on file. If card charge fails, the amount is referred to collections (per §10 terms). OpsAgent fires a CRITICAL alert when fund balance drops below $100 absolute (in addition to ratio-based thresholds above). All new bookings where `guaranteeGap > $0` are restricted until fund balance recovers above 5% of outstanding gaps.

### Rental Duration Ceiling:

**v1 platform maximum: 7 days.** Stripe pre-auth holds expire after 7 days. Rentals exceeding this would lose hold protection. The platform enforces this ceiling on all listings regardless of host-configured `maxDuration`. Hosts cannot set `maxDuration` above 7 days. Renters cannot select durations above 7 days.

### Deposit Logic:

```
holdAmount = tieredHold(estimatedItemValue)  // per tier table
rentalFee = hostPrice * duration
platformFee = rentalFee * takeRate
guaranteeFundContribution = platformFee * guaranteeRate
hostPayout = rentalFee - platformFee
```

### Payout Rules:

- New/high-risk hosts → 48-hour delayed release after return
- Established low-risk hosts → same-day payout
- First 3 transactions for any account → mandatory 48-hour delay regardless of risk score

### Monetization:

- Take rate: 15–25% (variable, tunable)
- Pro Host (future): monthly fee, reduced take rate, analytics, ranking boost

### Payment Adapter Layer:

```
PaymentAdapter interface {
  AuthorizeHold(amount, paymentMethod) → HoldID
  CaptureHold(holdID, amount) → ChargeID
  ReleaseHold(holdID)
  ChargeRentalFee(amount, paymentMethod) → ChargeID
  PayoutHost(amount, hostAccount) → PayoutID
  Refund(chargeID, amount) → RefundID
}
```

Stripe is the first implementation. Interface allows swapping to any processor.

---

## 8. Reputation & Risk

### Dual-Score Model

Two distinct scores serve two distinct purposes:

| Score | Lives on | Scale | Direction | Purpose |
|---|---|---|---|---|
| `reputationScore` | User | 0–1000 | Higher = more trusted | Long-running earned trust. Used for ranking, payout speed, social proof |
| `riskScore` | Transaction | 0–100 | Lower = safer | Per-transaction gate. Computed fresh for every booking. Used to block, delay, or flag |

These are related but independent. A user with 800 reputation can still get a high risk score on a specific transaction (e.g., renting a $5K item to another brand-new user at 3am).

---

### Reputation Score (User-Level)

**Starts at 0.** Every user begins with no reputation. Trust is earned, never assumed.

**Reputation is ADDITIVE — positive actions build score:**

```
Positive signals (add to reputationScore):
- Completed rental with no dispute              = +15 per event
- Positive rating bubble received                = +5 per bubble
- On-time return (within 15 min of scheduled)    = +10 per event
- Account age > 30 days                          = +25 (one-time)
- Account age > 90 days                          = +25 (one-time, stacks)
- Account age > 365 days                         = +25 (one-time, stacks)
- KYC verified                                   = +50 (one-time)
- 5+ completed rentals, no disputes              = +50 (milestone, one-time)
- 15+ completed rentals, no disputes             = +50 (milestone, one-time)
- 50+ completed rentals, no disputes             = +50 (milestone, one-time)
```

**Negative events SUBTRACT from reputation:**

```
Negative signals (subtract from reputationScore):
- Dispute filed against user                     = -30
- Dispute lost (decided against user)            = -50
- Cancellation (as cancelling party)             = -20
- Late return                                    = -15
- Fraud flag                                     = -100
```

**Host-specific signals (recalculated monthly):**

```
- Response rate > 90% over last 30 days          = +25
- Acceptance rate > 80% over last 30 days        = +25
- Zero host-initiated cancellations in 90 days   = +25
- Response rate < 50% over last 30 days          = -40
- Acceptance rate < 30% over last 30 days        = -40
```

**Score floor / ceiling:** 0 minimum, 1000 maximum.

**Negative signal decay:** Reputation reductions from negative events decay by 50% after 180 days. A user who lost 50 points from a dispute regains 25 after 6 months. A fraud flag's -100 becomes -50. Repeated offenses reset the decay clock.

**Meaningful differentiation:** At this scale, a user with 5 clean rentals has ~175 reputation. A user with 50 clean rentals has ~750+. The scale provides useful signal across the entire user lifecycle, not just the first few transactions.

---

### Risk Score (Per-Transaction)

**Computed fresh for every booking request.** Never stored on the User — only on the Transaction.

**Inputs:**

```
riskScore = baseRisk(identityStatus, accountAge, reputationScore)
          + transactionRisk(itemValue, duration, bookingTiming)
          + counterpartyRisk(counterpartyReputation, counterpartyAge)
          + behavioralRisk(recentCancellations, recentDisputes, geoConsistency)
          + fraudSignals(deviceFingerprint, networkSignals, velocityFlags)
```

**V1 implementation: rules engine with explicit weighted inputs. No ML.**

```
Base risk:
- identityStatus == PENDING                              = +20
- accountAge < 7 days                                    = +15
- reputationScore < 100                                  = +15
- reputationScore 100–300                                = +5

Transaction risk:
- itemValue > $1,000                                     = +15
- itemValue > $3,000                                     = +25
- bookingTiming is 12am–5am local                        = +10

Counterparty risk:
- Both users have reputationScore < 50                   = +30
- Counterparty accountAge < 14 days                      = +10

Behavioral risk:
- 2+ cancellations in last 60 days                       = +20
- 1+ disputes in last 60 days                            = +25
- Transaction outside user's usual geo radius            = +10

Fraud signals:
- Same device fingerprint as another account             = +50
- Compound network signals (WiFi + one other — see §9)   = +30
- Velocity flag (exceeds transaction frequency threshold) = +20
```

**Clamped to 0–100.**

### Controls:

| Risk Score | Action |
|---|---|
| 0–30 | Fast payout, standard hold |
| 31–70 | Standard escrow, 48h delayed payout |
| 71+ | Block booking or require additional verification |

All inputs instrumented for future ML model training.

---

## 9. Fraud & Collusion Prevention

### Managed by: FraudAgent

### Signal Detection:

- Shared device fingerprints between users
- Same or linked payment instruments
- Phone numbers from the same carrier batch
- Accounts created within minutes of each other
- Social graph analysis: A only transacts with B

**Compound-only signal (never scores alone):**

- Same WiFi network during sign-up — recorded but contributes zero risk score on its own. Only activates risk impact (+30) when combined with at least one other signal from this list. Rationale: coffee shops, campuses, and co-working spaces share WiFi across unrelated legitimate users

### Behavioral Velocity:

- New accounts cannot transact with other new accounts for 30 days
- First 3 transactions: mandatory 48-hour payout delay
- No account can receive more than $X in damage claims within first 60 days (X is tunable)

### Pattern Analysis:

FraudAgent monitors across transaction history:

- Two users who exclusively transact with each other
- Damage claims that consistently land at exactly the hold amount
- Items that get "damaged" on every rental
- Sudden spikes in high-value listings from new accounts

---

## 10. Legal System

### Managed by: AgreementAgent

### Structure: Hybrid

**Base template (lawyer-reviewed, immutable):**

- Platform terms of use
- General liability framework (renter is liable)
- Arbitration clause
- Late fee structure
- Pre-auth hold disclosure
- Data usage and retention
- AI agent disclosure (users acknowledge AI makes decisions)

**AI-customized section (per item, per transaction):**

- Item-specific condition notes
- Item-specific exclusions (e.g., water damage exclusions for electronics)
- Special handling instructions
- Custom damage thresholds based on item type and value

**AgreementAgent guardrails:**

- Cannot contradict base template
- Cannot remove liability or arbitration clauses
- Cannot modify payment terms
- Can only customize within the item-specific section

### Storage:

- Immutable JSON snapshot per transaction
- Timestamp, IP, device ID
- Both parties must accept before booking completes

---

## 11. Listing Authenticity

### Camera-Only Capture

**Requirements:**

- No uploads — in-app capture only
- 3–5 photos + optional video
- Proof frame required (hand in shot or on-screen verification code)

**Metadata captured:**

- Timestamp
- GPS coordinates
- Device ID

**Anti-Spoof:**

- Emulator detection and block
- Liveness prompts (randomized capture instructions)
- Reverse image search
- AI validation of photo consistency (same item, same location, same session)

### AI Autofill:

On capture, the AppraisalAgent:

1. Identifies the item
2. Estimates value (used for pre-auth hold)
3. Suggests pricing (hourly/daily)
4. Generates semantic tags (used by search)
5. Generates description (editable by host)

### Value Override Flow:

If host sets value >100% of AI estimate:

1. AI agent prompts host for justification (text input)
2. AppraisalAgent evaluates justification autonomously
3. Approved → host value used for hold amount
4. Rejected → host can accept AI estimate or provide more evidence

### 11.1 Photo Diff Pipeline

**Purpose:** Compare check-in and return photos to detect damage. This is the evidentiary backbone of the dispute system.

**Pipeline (two-stage: CV preprocessing → LLM reasoning):**

**Stage 1 — Computer Vision preprocessing (no LLM):**

1. **Normalization** — Both photo sets resized to standard resolution. Histogram equalization (OpenCV) to normalize lighting and white-balance variance across different phones and conditions
2. **Object isolation** — Segment the item from background using a dedicated segmentation model (SAM 2 or equivalent). Background changes are irrelevant to damage assessment. Output: cropped, isolated item images on transparent background
3. **Angle matching** — Using captured gyroscope metadata (see Angle Enforcement below), pair check-in and return photos by closest orientation match. Each return photo is compared against its best-matching check-in photo, not a random one

**Stage 2 — LLM reasoning (Claude via model router):**

4. **Structural comparison** — AI receives paired, preprocessed item crops. Compares item structure, surface condition, and component presence across each pair
5. **Damage classification** — Each detected difference classified:

| Classification | Definition | Example |
|---|---|---|
| NO_CHANGE | Item matches within tolerance | Normal wear, lighting artifacts |
| COSMETIC_DAMAGE | Surface-level, does not affect function | Scratches, scuffs, minor dents |
| FUNCTIONAL_DAMAGE | Affects item usability | Broken parts, missing components, water damage |
| MISSING_ITEM | Item or component not present in return photos | Accessory not returned, item absent |
| INCONCLUSIVE | Diff cannot determine with confidence | Poor photo quality, insufficient angles, heavy lighting difference |

This two-stage architecture cuts LLM cost per diff by ~60% (normalization and segmentation are fast, cheap CV operations) and improves accuracy on steps that don't require reasoning.

**Confidence scoring:**

Every diff produces a confidence score (0–1.0). The score reflects how certain the model is about its classification, NOT about the severity of damage.

**Handling INCONCLUSIVE results:**

When the diff returns INCONCLUSIVE or confidence < 0.7:

1. Both parties are prompted to submit additional photos (within 2-hour window)
2. If new photos resolve → re-run diff
3. If still inconclusive after second round → DisputeAgent evaluates using all available evidence (messages, agreement terms, user history, reputation scores) and flags decision for human review queue regardless of dollar amount
4. Inconclusive diffs are never auto-resolved in either party's favor

**Re-prompt notification flow:** Push notification + in-app alert sent to both parties requesting additional photos. A 2-hour countdown timer starts; a River job is scheduled to fire at expiry. If one party submits and the other does not: proceed with available evidence + flag as "partial re-prompt" in the dispute record. If neither party submits: escalate to human review with existing evidence + note "re-prompt unanswered." Additional photos follow the same quality gate (angle enforcement, blur detection, resolution check).

**Angle Enforcement:**

The app requires photos from meaningfully different angles to ensure the item is documented from multiple perspectives.

**How it works (UX):**

1. Host/renter opens check-in camera. First photo captured freely from any angle
2. After first capture, the app reads the device gyroscope/accelerometer orientation and stores it as metadata on the photo
3. For photos 2+, the app compares current device orientation to all previously captured orientations in real time. A circular indicator on the viewfinder shows the user how far they've rotated from their last shot
4. If the user attempts to capture within < 30° rotation of any existing photo, the shutter is soft-blocked with a prompt: "Move to a different angle." The indicator turns green when they've rotated enough
5. Minimum 3 photos required, each ≥ 30° apart in device orientation (roll or pitch)
6. Orientation metadata (roll, pitch, yaw in degrees) stored per photo in the Media record

**Why this matters for contracts:** The rental agreement specifies that the check-in and check-out photo sets constitute the binding visual record of item condition. Multi-angle documentation reduces the surface area for disputes by establishing a comprehensive baseline that's harder to challenge.

**Photo quality gate:**

At check-in and check-out, the app validates photo quality before accepting:

- Minimum resolution threshold
- Blur detection (reject blurry captures)
- Item must be detected in frame (reject photos of walls, floors, etc.)
- Minimum 3 photos, each ≥ 30° apart (enforced by gyroscope — see above)

**Quality gate enforcement ownership:**

- **Client-side (React Native):** Blur detection (frame processor), angle diversity (gyroscope — already specified), minimum resolution check (image metadata). These are real-time UX feedback loops that must run on-device for responsive capture experience.
- **Server-side (Go MediaService):** Item-in-frame detection (requires ML model that cannot run on-device in v1), resolution re-verification (trust but verify), EXIF metadata validation (timestamp, GPS, device ID consistency).

**Limitations (acknowledged):**

- Internal damage (mechanical, electrical) is not detectable via photos — these disputes rely on renter/host testimony + message history
- Pre-existing wear vs. new damage is a judgment call at the margins — the system biases toward the check-in baseline

---

## 12. Proximity Handoff

### v1: GPS + PIN

- GPS proximity verification (≤100m)
- Host-generated 4-digit PIN
- Both parties capture check-in/check-out photos (mandatory)

### v1.5: GPS + BLE Detection

- BLE proximity as secondary signal

### v2: BLE Handshake

- BLE auto-verification + PIN fallback

### Rule:

**Rental cannot start without proximity verification + check-in photos from both parties.**
**Rental cannot close without proximity verification + return photos from both parties.**

### Outage Fallback:

If the app is unreachable during handoff:

- SMS-based PIN exchange bypasses the app
- Photos captured and uploaded once connectivity restores
- LateReturnAgent reconciles any timing discrepancies after service restores

---

## 13. Discovery

### Three Surfaces:

1. **Feed:** "Available near you" — ranked list of nearby items available now
2. **Search:** Keyword-based, matched against AI-generated semantic tags
3. **Map:** Browse items geographically with fuzzed locations

### Ranking Inputs (all normalized to [0, 1] before weighting):

- `availabilityNow` — 1 if available now, 0 if not (binary)
- `proximityScore` — `1 - (driveTimeMinutes / maxDriveTimeInRadius)`, clamped to [0, 1]
- `reputationScore` — `hostReputationScore / 1000`
- `reliabilityScore` — `(responseRate + onTimeRate + acceptanceRate) / 3`

**Ranking formula:**

```
score = w_availability * availabilityNow
      + w_proximity   * proximityScore
      + w_reputation  * reputationScore
      + w_reliability * reliabilityScore
```

Weights: tunable config vars. Starting values: `w_availability = 0.35, w_proximity = 0.30, w_reputation = 0.20, w_reliability = 0.15`. Availability and proximity dominate because the product promise is "rent anything nearby, fast."

### Filtering:

- Drive time (min/max)
- Price range
- Rental duration

### Hide from results:

- Listings with low availability probability
- Hosts with poor response rates
- Flagged or suspended listings

---

## 14. Messaging

### In-App Messaging

- Available between renter and host once a booking request is created
- Full conversation history stored in Postgres
- Queryable by DisputeAgent as evidence
- Push notification on new message
- No phone number exchange needed (but not blocked)

### Agent Communication:

- AI agents communicate with users through the same messaging interface
- Agent messages are clearly marked as from "RentMy AI"
- Used for: dispute updates, verification requests, value justification prompts, late return warnings, resolution notifications

---

## 15. Ratings

### Structured Ratings (No Freeform)

After each completed rental, both parties rate each other.

**Renter rates Host — available bubbles:**

- Good communication
- On time
- Item as described
- Easy pickup
- Friendly

**Host rates Renter — available bubbles:**

- Good communication
- On time return
- Careful with item
- Easy handoff
- Respectful

Ratings feed into reputation score calculation. No public text reviews. No retaliation risk.

---

## 16. Notifications

### System: Pusher (push) + in-app + SMS (fallback/degraded mode)

### Notification Types:

| Event | Recipient | Channel |
|---|---|---|
| New booking request | Host | Push + in-app |
| Booking accepted | Renter | Push + in-app |
| Booking auto-declined (timeout) | Renter + Host | Push + in-app |
| Cancellation | Both parties | Push + in-app |
| Pickup time approaching | Both parties | Push |
| Proximity verified | Both parties | In-app |
| Return time approaching | Renter | Push |
| Late return warning | Both parties | Push |
| Late return escalation | Both parties | Push + SMS |
| Dispute opened | Both parties | Push + in-app |
| Dispute resolved | Both parties | Push + in-app |
| Payout sent | Host | Push + in-app |
| New message | Recipient | Push |
| KYC status update | User | Push + in-app |
| Listing flagged | Host | Push + in-app |

### User-Managed Settings:

- Per-type toggle (on/off)
- Quiet hours (start/end time)
- Push vs. in-app preference per type
- SMS fallback toggle
- Booking notifications cannot be fully disabled (safety requirement)

---

## 17. Booking Flow

### State Machine

Every booking has exactly one status. Only the transitions below are valid — all others are rejected at the service layer.

```
REQUESTED → ACCEPTED          (host accepts)
REQUESTED → DECLINED          (host declines)
REQUESTED → AUTO_DECLINED     (timeout, no host response)
REQUESTED → CANCELLED         (renter cancels before host responds)

ACCEPTED  → ACTIVE            (both parties complete check-in handoff)
ACCEPTED  → CANCELLED         (either party cancels — fee per §18)

ACTIVE    → COMPLETED         (both parties complete check-out handoff, no dispute)
ACTIVE    → DISPUTED          (either party reports issue during active rental)

DISPUTED  → COMPLETED         (dispute resolved, rental closes)

COMPLETED is terminal.
DECLINED, AUTO_DECLINED, CANCELLED are terminal.
```

**Guard conditions:**

- `ACCEPTED → ACTIVE` requires: GPS proximity verified + PIN accepted + check-in photos from BOTH parties
- `ACTIVE → COMPLETED` requires: GPS proximity verified + return photos from BOTH parties + photo diff pipeline has run
- `ACTIVE → DISPUTED` requires: either party submits a dispute report
- `DISPUTED → COMPLETED` requires: DisputeAgent (or human reviewer) has issued a final decision and all financial operations (hold capture/release, refund) have executed

### Manual Acceptance

1. Renter taps "Rent Now"
2. Booking request sent to host
3. Host receives push notification
4. Host accepts or declines

### Auto-Decline:

- Configurable timeout window (platform-level variable, tunable)
- If host does not respond within window → auto-decline
- Renter notified, shown next-nearest alternatives

### Non-Response Consequences:

- Response rate tracked per host
- Repeated non-response → ranking demotion (progressive)
- Sustained non-response → listing auto-paused
- Thresholds are tunable variables

---

## 18. Cancellation Policy

### Principle: The cancelling party pays.

### Renter Cancels:

| Timing | Penalty |
|---|---|
| >2 hours before pickup | No fee |
| 1–2 hours before pickup | 25% of rental fee |
| <1 hour before pickup | 50% of rental fee |
| After host has confirmed and is waiting | 100% of rental fee |

### Host Cancels:

| Timing | Penalty |
|---|---|
| >2 hours before pickup | Warning (tracked) |
| <2 hours before pickup | Fee (% of rental, tunable) + ranking hit |
| After renter is en route | Higher fee + ranking hit + suspension risk |
| Repeated cancellations | Progressive suspension |

Host cancellation is treated as the most damaging action on the platform.

---

## 19. Late Returns

### Managed by: LateReturnAgent

### Auto-Charging:

- Rental continues at the hourly rate (minimum)
- If the late return causes a conflict with another booking: double rate
- Late charges captured from the pre-auth hold via HoldAllocation ledger
- **Late fee capture cap:** LateReturnAgent cannot capture more than `holdAmount * (1 - damageReserveRate)`. The damage reserve (default 40% of hold) is protected for potential damage claims on return. This prevents late fees from consuming the entire hold, leaving nothing for DisputeAgent

### Escalation:

LateReturnAgent decides autonomously when "late" becomes "potential theft" based on:

- Duration overdue
- Whether the renter is responding to messages
- Renter's history and reputation score
- Item value
- Time of day and context

When escalated:

- DisputeAgent takes over
- Renter's hold is captured
- Host notified of status
- If renter is unresponsive beyond agent-determined threshold: flagged for review, potential law enforcement guidance provided to host

---

## 20. Disputes

### Managed by: DisputeAgent (autonomous with escalation gate)

### Flow:

1. Either party reports an issue
2. DisputeAgent gathers evidence:
   - Agreement snapshot
   - Check-in photos (both parties)
   - Return photos (both parties)
   - Message history
   - Proximity proofs
   - Transaction data
3. Photo diff pipeline runs (see §11.1)
4. AI evaluates evidence against agreement terms
5. AI produces decision + confidence score
6. Decision routed through escalation gate (see below)
7. Approved decisions execute: charge from hold, release hold, partial charge, or refund

### Escalation Gate:

Not all decisions are equal. The DisputeAgent operates autonomously within bounds, and escalates outside them.

| Condition | Action |
|---|---|
| Confidence ≥ 0.85 AND charge ≤ $200 | Auto-resolve, no human review |
| Confidence ≥ 0.85 AND charge $201–$1,000 | Auto-resolve, flagged for async human audit (post-hoc) |
| Confidence ≥ 0.85 AND charge > $1,000 | Queue for human review before execution |
| Confidence < 0.85 (any amount) | Queue for human review before execution |
| Photo diff returned INCONCLUSIVE | Queue for human review before execution |
| Either party has active fraud flags | Queue for human review before execution |

**Human review queue:**

- Staffed by ops team (small — this queue should be low volume if agents are working well)
- SLA: 4 hours for active rentals, 24 hours for post-return
- Reviewer sees: full evidence package, agent's proposed decision, confidence score, reasoning
- Reviewer can: approve agent decision, override with different outcome, or request more evidence
- All human overrides logged as their own AgentDecision record (agentType: HUMAN_OVERRIDE)

**Escalation thresholds are tunable.** As agent accuracy is validated over time, the confidence floor can drop and the dollar ceiling can rise. Goal: shrink the human queue to near-zero, but never remove the gate entirely.

### SLA:

- Active rentals (auto-resolve path): < 2 hours
- Active rentals (human review path): < 4 hours
- Post-return disputes (auto-resolve): < 12 hours
- Post-return disputes (human review): < 24 hours

**SLA breach handling:** A River cron job checks the human review queue every 15 minutes. At 80% of the SLA window, the system fires a warning alert to the ops team. On full SLA breach for active rentals (4h): auto-escalate to next-level reviewer + OpsAgent fires CRITICAL alert. On full SLA breach for post-return (24h): OpsAgent fires WARNING alert + auto-assign to next available reviewer. SLA compliance rate is tracked on the ops dashboard (§25).

### All decisions logged:

Every DisputeAgent decision is stored as an AgentDecision record with full input, output, model used, confidence score, and escalation path taken. This is the audit trail.

---

## 21. Renter Onboarding

### Deferred KYC (triggered at first booking attempt)

1. Download app → create account (email/phone)
2. Browse freely: feed, search, map
3. Tap "Rent Now" on any listing
4. KYC triggered:
   - ID document capture
   - Selfie capture
   - VerificationAgent validates via API
5. Add payment method (must support pre-auth holds)
6. Once verified → booking proceeds
7. KYC never required again

---

## 22. Host Onboarding

### 7-Step Flow (target: publish in under 3 minutes)

1. **Earn Screen**
   - Local earnings estimate
   - Value props: guaranteed payment, damage protection (tiered hold + platform guarantee), hands-free scheduling, referral bonuses

2. **Location**
   - Home or nearby pin drop
   - Exact location stored, fuzzed in discovery

3. **Capture**
   - Camera-only, 3–5 photos + optional video
   - Proof frame (hand or verification code)

4. **AI Autofill**
   - Item identified, value estimated, description generated, tags created
   - All fields editable

5. **Pricing & Availability**
   - Hourly and/or daily rate (AI suggested)
   - Min/max rental duration
   - Availability windows

6. **Verification**
   - ID + selfie
   - Can publish as PENDING (listing goes live, bookings held until verified)

7. **Publish + Share**
   - Referral incentive prompt

---

## 23. Referral System

### Host-Refers-Host

- Referrer gets $20 when their referral completes their first rental as a host
- Referred host gets $20 bonus on first completed rental
- Cash incentive, not fee discount

### Fraud Prevention:

- Shared device detection
- Same network detection
- Velocity limits on referral payouts
- Referral abuse = account flag

### Renter Referrals:

Future roadmap — activate once supply density justifies it.

---

## 24. Data Retention & Privacy

### Managed by: Automated retention policies + agent-handled deletion requests

### On Account Deletion Request:

**Immediately deleted:**

- Profile information
- Notification preferences
- Device fingerprints
- Saved payment methods (via Stripe, not stored locally)

**Retained for compliance window (configurable, default 180 days post-last-transaction):**

- Transaction records
- Agreement snapshots
- Check-in/check-out photos
- Message history
- Dispute records and agent decisions

**After compliance window:**

- Transaction records anonymized (user IDs stripped)
- Photos deleted
- Messages deleted
- Anonymized records retained for platform analytics

**Identity documents:**

- Deleted 30 days after account closure (or after last active transaction compliance window, whichever is later)

### GDPR / CCPA Compliance:

- Data export available on request
- Deletion request processed by agent, respecting retention rules
- Agent explains to user what is retained and why

---

## 25. Internal Ops

### Fully Agent-Managed, Human-Monitored

### OpsAgent Responsibilities:

- Monitor platform health in real time
- Detect anomalies: fraud spikes, supply drops, booking failure clusters
- Track host churn signals (declining response rates, listing pauses)
- Identify supply gaps by geography
- Monitor agent performance (resolution accuracy, confidence scores)

### Dashboard (Real-Time):

**Business Metrics:**

- Active listings (total, by area)
- Active users (hosts, renters)
- Booking conversion rate (request → completed rental)
- Revenue (gross, net, take rate)
- Average transaction value
- Host payout velocity

**Trust & Safety:**

- Fraud flag rate
- Dispute rate and resolution outcomes
- Average agent confidence score
- Collusion alerts

**Supply Health:**

- New host sign-ups
- Host churn rate
- Listings per active area
- Response rate distribution

**Demand Health:**

- Search-to-book conversion
- Repeat renter rate
- Failed booking rate (auto-declines, cancellations)

### Alerts:

OpsAgent pushes alerts to the team when:

- Fraud rate exceeds threshold
- Booking conversion drops below threshold
- Supply density in an active market drops below viable level
- Agent confidence scores trend downward
- Payout failures spike
- Any metric deviates significantly from rolling average

**Alert routing:**

- **Primary channel:** Slack webhook (all alerts posted to #ops-alerts channel)
- **Secondary channel:** Email digest for non-urgent alerts (daily summary)
- **Critical alerts page on-call:** Fraud spike, payout failure, SLA breach, guarantee fund CRITICAL — additionally routed to on-call via PagerDuty (or equivalent)
- **Configuration:** Alert routing rules stored in config (env vars for Slack webhook URL, email recipients, PagerDuty integration key). Per-alert-type routing is configurable.

---

## 26. Market Launch Playbook

### Repeatable Framework (Market-Agnostic)

### Phase 1: Identify Market

- Select geography with high rental demand signals (tourism, outdoor activity, events)
- Define radius for density target
- Set minimum viable supply threshold (tunable)

### Phase 2: Seed Supply

- Direct outreach to potential hosts in target area
- Host-refers-host referral activated
- Targeted marketing positioning (e.g., "rent paddleboards" in a lake town, "rent camera gear" in a creative city — platform adapts, marketing targets)
- Goal: reach minimum supply threshold before activating demand

### Phase 3: Activate Demand

- Local SEO + AI-generated content
- On-site signage in high-traffic areas
- Social proof: early reviews and ratings displayed
- Hyper-targeted ads aligned to seeded supply

### Phase 4: Build Trust Loop

- Structured ratings accumulate
- Host reputation scores rise with successful rentals
- Repeat renter incentives
- OpsAgent monitors market health

### Phase 5: Evaluate & Expand

- Market is "alive" when:
  - Booking conversion ≥ 50%
  - Host response rate ≥ 70%
  - Repeat renter rate ≥ 20%
  - Supply density supports discovery (renters see ≥ 5 relevant results)
- If alive → increase demand spend
- If struggling → diagnose via OpsAgent alerts, adjust supply strategy

---

## 27. Acceptance Criteria

### Booking

- User can complete booking in ≤60 seconds (post-KYC)
- Tiered hold amount and rental fee shown clearly before confirmation
- Agreement snapshot stored immutably
- Hold authorized before booking confirmed

### Handoff

- Cannot start without GPS proximity (≤100m) + PIN + check-in photos from both parties
- Cannot close without GPS proximity + return photos from both parties
- SMS fallback works when app is unreachable

### Listing

- Cannot publish without camera capture + minimum 3 photos, each ≥ 30° apart (gyroscope-enforced)
- AI autofill completes within 5 seconds of capture
- Value override >100% triggers justification flow
- Maximum rental duration capped at 7 days (Stripe hold expiry)

### Risk

- All transactions scored with per-transaction `riskScore` (0–100) before booking confirmation
- User `reputationScore` (0–1000) recalculated after every completed transaction
- High-risk flows blocked or escalated
- Collusion signals detected across transaction history
- Fraud velocity rules enforce new-to-new lockout and first-3-transaction delay

### Disputes

- DisputeAgent resolves autonomously within escalation gate bounds (confidence ≥ 0.85, dollar thresholds)
- Decisions outside bounds queued for human review before execution
- All decisions logged with full evidence chain, confidence score, and escalation path
- SLA: < 2 hours auto-resolve, < 4 hours human review (active rentals)
- Photo diff pipeline returns structured classification + confidence for every comparison
- INCONCLUSIVE diffs never auto-resolve — always escalate

### Agents

- All agents communicate as "RentMy AI" — clearly identified as AI
- All agent decisions stored as AgentDecision records with `outcomeId` linkage for learning
- Model router assigns appropriate model tier per task
- Confidence calibration error < 0.10 target for all agents (measured monthly)
- Prompt versions tracked — every decision records which prompt version was used

---

## 28. Failure Modes

| Failure | Mitigation |
|---|---|
| No density → no bookings | Market launch playbook with minimum supply threshold before demand activation |
| Weak verification → fraud spike | AI-powered KYC via API + FraudAgent monitoring |
| No proximity enforcement → theft risk | Hard rule: rental cannot start/end without proximity proof |
| Inflated item values → renter hold abuse | AppraisalAgent validates, override >100% requires justification. Tiered holds cap renter exposure regardless |
| No dispute rigor → trust collapse | DisputeAgent with photo diff pipeline + escalation gate. Human review for high-dollar or low-confidence decisions |
| Host ghosting → broken booking promise | Auto-decline timer + progressive ranking penalties |
| Collusion → systematic fraud | Multi-signal detection (device, network, payment, behavioral patterns) |
| App outage during active rental | SMS-based PIN fallback, agent reconciliation on restore |
| Late return → host anxiety | LateReturnAgent with auto-charging and autonomous escalation |
| Single payment processor dependency | Swappable adapter layer, Stripe first |

---

## 29. Future Roadmap (Not v1)

- Pro Host subscription (reduced take rate, analytics, ranking boost)
- BLE proximity handshake (v1.5/v2)
- Renter referral program
- Web support
- ML-based risk engine (trained on v1 rules engine data)
- Advanced analytics for hosts
- Multi-language support
- Automated tax document generation

---

## 30. Technical Notes

### Media Pipeline:

- Camera capture → original stored in S3
- Thumbnails generated on upload (served to clients)
- Originals preserved for dispute evidence integrity
- AI photo diff operates on originals

### Geospatial:

- PostGIS for all location queries
- Drive-time estimation via routing API
- Listing locations fuzzed in discovery (exact stored, approximate shown)
- Proximity verification uses exact coordinates

### Real-Time:

- SSE via Pusher for: booking updates, messages, notifications, proximity events
- Redis for: active session cache, proximity tracking, hot read path

### Durable State Strategy:

**Problem:** Redis is volatile. If it crashes mid-rental, active timers (auto-decline countdowns, late return monitoring) and session state are lost.

**Solution: Postgres-primary, Redis-cache architecture.**

All critical state transitions write to Postgres FIRST, then project to Redis for fast reads. Redis is a performance optimization, not the source of truth.

**Durable timers via River (Go-native Postgres-backed job queue):**

| Timer | Behavior on Redis loss |
|---|---|
| Auto-decline countdown | River job fires at deadline regardless of Redis state |
| Late return monitor | River job checks rental status at scheduled end time, escalates if still active |
| Payout delay timer | River job triggers payout release after hold period |
| KYC verification timeout | River job follows up if verification API hasn't responded |

**State recovery on Redis restart:**

1. Redis comes back empty
2. Services lazy-load active state from Postgres on next read
3. No manual intervention required — the system self-heals
4. OpsAgent alerts on Redis downtime so the team knows, but no action is needed

**What stays Redis-only (acceptable to lose):**

- Proximity tracking cache (GPS pings) — re-established on next ping
- Read-through caches (listing data, user profiles) — rebuilt from Postgres on miss
- Rate limiter counters — reset is acceptable (brief window of un-throttled requests)

### Agent Orchestration:

- Each agent is an independent service within the modular monolith
- Agents communicate via internal events
- Model router selects LLM per task complexity
- All agent decisions logged for audit, learning, and future training (see §31)

---

## 31. Agent Learning Framework

### Purpose

Every agent decision is an opportunity to improve. The learning framework connects decisions to outcomes, measures accuracy, and evolves agent prompts — without requiring fine-tuning or ML infrastructure in v1.

### Architecture: Decision → Outcome → Calibration → Evolution

**Step 1 — Decision logging (already in place)**

Every agent writes an AgentDecision record with: input, decision, model, confidence, and reasoning.

**Step 2 — Outcome linking**

When a decision's real-world outcome becomes known, the system links it back to the original decision via `outcomeId` and sets `outcomeCorrect`.

| Agent | Outcome trigger | What constitutes "correct" |
|---|---|---|
| AppraisalAgent | Rental completes or damage claim resolves | Estimated value within 30% of actual damage claim amount (if any). No host override on value |
| DisputeAgent | Final resolution (auto or human) | Decision not overridden by human reviewer. If overridden, `outcomeCorrect = false` |
| RiskAgent | Transaction completes | High-risk blocks: would-have-been-fine if similar users/items transacted without incident. Low-risk passes: no fraud or dispute occurred |
| VerificationAgent | Account lifetime behavior | Verified users who later get fraud-flagged = false positive. Rejected users who re-verify successfully = false negative |
| AgreementAgent | Dispute resolution | Dispute arose from a gap in the agreement's custom section (damage threshold was wrong, exclusion was missing) |
| FraudAgent | Investigation outcome | Flagged accounts confirmed as fraudulent. Unflagged accounts later confirmed as collusion |
| LateReturnAgent | Return completion | Escalation to DisputeAgent was warranted (renter was genuinely non-responsive) vs. premature (renter returned within grace) |

**Step 3 — Confidence calibration**

Track expected vs. actual accuracy at each confidence level (rolling 90-day window):

```
For each confidence bucket (0.5–0.6, 0.6–0.7, 0.7–0.8, 0.8–0.9, 0.9–1.0):
  expectedAccuracy = bucket midpoint
  actualAccuracy   = count(outcomeCorrect=true) / count(all decisions in bucket)
  calibrationError = |expectedAccuracy - actualAccuracy|
```

A well-calibrated agent says 0.9 confidence and is right ~90% of the time. If it says 0.9 but is only right 70% of the time, the escalation gate threshold needs to rise (or the prompt needs work).

OpsAgent reports calibration error per agent monthly. Target: calibration error < 0.10 for all agents.

**Step 4 — Prompt evolution**

Prompts are not static. They evolve based on observed performance.

**Quarterly calibration cycle:**

1. Pull all AgentDecisions with `outcomeCorrect` set (both true and false) from the last 90 days
2. For each agent, identify:
   - **Best decisions:** highest-confidence correct decisions → candidates for few-shot examples in the prompt
   - **Worst decisions:** incorrect decisions, especially high-confidence incorrect ones → candidates for "watch out for this" counter-examples in the prompt
   - **Systematic errors:** patterns in incorrect decisions (e.g., AppraisalAgent consistently overvalues electronics, DisputeAgent under-assesses water damage)
3. Update the agent's prompt:
   - Replace few-shot examples with the best recent real decisions
   - Add counter-examples from the worst recent failures
   - Add explicit guidance addressing systematic error patterns
4. A/B test the updated prompt against the current prompt on a held-out set of decisions before deploying

**Prompt version control:** Every prompt is versioned (stored in the repo, tagged with date). The `model` field on AgentDecision records the prompt version used, so performance can be tracked per prompt iteration.

### Per-Agent Evaluation Metrics

| Agent | Primary metric | Secondary metric | Alert threshold |
|---|---|---|---|
| AppraisalAgent | Value accuracy (estimated vs. actual within 30%) | Host override rate (lower is better) | Override rate > 40% |
| DisputeAgent | Human override rate (lower is better) | Average resolution time | Override rate > 20% |
| RiskAgent | False positive rate (high-risk blocks on safe transactions) | False negative rate (low-risk passes on fraudulent transactions) | False negative > 2% |
| VerificationAgent | False rejection rate | Processing time p95 | False rejection > 5% |
| AgreementAgent | Dispute rate on custom clauses | — | Clause-related disputes > 10% of all disputes |
| FraudAgent | Precision (flagged accounts confirmed fraudulent) | Recall (fraudulent accounts actually flagged) | Precision < 60% |
| LateReturnAgent | Premature escalation rate | Missed escalation rate (theft that wasn't flagged) | Premature > 30% |

### Data Pipeline

```
Transaction completes
  → River job: link_outcome (runs 48h after transaction close)
    → For each AgentDecision on this transaction:
      → Evaluate outcome correctness per agent-specific rules
      → Set outcomeId, outcomeCorrect
      → Update rolling calibration metrics

Monthly:
  → River cron: calibration_report
    → Calculate per-agent calibration error
    → Calculate per-agent evaluation metrics
    → OpsAgent surfaces in dashboard

Quarterly:
  → Manual: prompt_evolution_cycle
    → Pull decision data, identify patterns
    → Draft updated prompts
    → A/B test on held-out set
    → Deploy winning prompt version
```

### Ground Truth for Contracts

The rental agreement is the source of truth for every dispute. The learning framework reinforces this by tracking whether agent decisions align with agreement terms. When a DisputeAgent decision is overridden, the override reason is logged — if the reason is "agreement clause was misinterpreted," that feeds back into the DisputeAgent's prompt as a counter-example. If the reason is "agreement clause was missing," that feeds back into the AgreementAgent's prompt to generate more comprehensive clauses for similar item types.

This creates a closed loop: better agreements → fewer disputes → and when disputes do arise, better resolution → which informs even better agreements.
