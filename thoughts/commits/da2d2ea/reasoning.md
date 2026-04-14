# Commit Reasoning: da2d2ea

## What changed

Task 9.7 — get all messaging E2E flows passing.

### Backend
- **New `GET /api/v1/users/me/conversations` endpoint.** The frontend's `useConversations` hook called this endpoint but it didn't exist. Added handler, service method, and repository query. The query joins transactions/users/listings with a `LEFT JOIN LATERAL` for the last message per conversation, ordered by most recent activity.
- **Updated messaging repo interface** to include `ListConversations`.
- **Updated service test stub** to satisfy the new interface method.

### Frontend
- **Fixed `useSendMessage` response shape.** Backend returns raw `Message` struct, but the hook expected `{ message: Message }`. The destructured `message` was `undefined`, breaking the optimistic cache update.
- **Added testIDs to all messaging components** — 8 testIDs across 5 files (screen-messages, screen-conversation, conversation-list, conversation-row, message-bubble-own/received, input-message, btn-send-message, message-list).
- **Changed SafeAreaView to View** for testID containers on messages list and conversation screens — iOS doesn't propagate `testID` on SafeAreaView to the accessibility tree.

### E2E tests & seeding
- **Rewrote messaging flow YAML files** — fixed `assertVisible` + `timeout` (not valid, use `extendedWaitUntil`), fixed relative `runFlow` paths, fixed tab navigation to use iOS accessibility pattern `"Messages, tab.*"`.
- **SQL-based conversation seeding in setup.sh** — Maestro's `http.post()` only accepts 2 args (no custom headers), so API-based seeding was impossible. Added `seed_conversations()` to setup.sh that inserts messages via SQL.
- **Fixed message timestamp ordering** — seed messages at `NOW() + 1/2 seconds` to sort above bookings created at `NOW()`.

## Why this approach

The messaging feature had multiple disconnected bugs: a missing backend endpoint, a response shape mismatch, missing testIDs, and a broken seed strategy. Each bug was independently blocking the E2E tests. Fixing them all together in one commit keeps the change atomic — the tests can't pass without all fixes applied.
