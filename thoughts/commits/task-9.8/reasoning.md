# Task 9.8 — Commit Reasoning

## What changed
Get all dispute and rating E2E flows passing:
- 2 dispute flows: file-dispute (happy path), view-dispute-status (pre-seeded)
- 1 rating flow: rate-counterparty (happy path)

## Why
Phase 9.8 requires all Maestro E2E flows in `disputes/` and `ratings/` directories to pass against the real app on iOS Simulator. These flows validate the dispute filing, dispute status viewing, and counterparty rating user journeys end-to-end.

## Key decisions
1. **SQL seeding over API seeding** — The JS seed scripts called non-existent backend test endpoints. Extended `setup.sh` with raw SQL to create ACTIVE, DISPUTED+dispute record, and COMPLETED bookings with proximity proofs.
2. **Dedicated Pressable for DISPUTED badge** — iOS accessibility tree doesn't propagate taps from Text elements inside FlatList rows to parent Pressables. Created a tappable badge with its own testID and onPress.
3. **SafeAreaView → View** — SafeAreaView doesn't forward testID to the native accessibility tree on iOS. Changed to View on 3 screens.
4. **Keyboard dismiss pattern** — Multiline TextInput breaks Maestro's `hideKeyboard`. Used `keyboardDismissMode="on-drag"` + tap on static text.
