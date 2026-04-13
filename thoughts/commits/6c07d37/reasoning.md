# Commit 6c07d37 — Fix: resolve 6 messaging bugs from Phase 8.9 audit

## Why these changes

Phase 8.9 produced a static audit of the messaging feature. 6 bugs were found ranging from missing critical backend endpoints to UI polish. This commit fixes all 6 in one pass since they are tightly coupled (fixing the endpoint without fixing the response shape would break the mobile; fixing pagination without updating the display layer would produce garbled ordering).

## Key decisions

### BUG-MSG-1 conversations query

Used two `LEFT JOIN LATERAL` subqueries rather than a single big JOIN: one for the last message (1 row per transaction via `ORDER BY id DESC LIMIT 1`), one for unread count via `notifications` where `type = 'NEW_MESSAGE'` and `read = false`. This avoids inflating rows via multiple joins and keeps the query correct without window functions.

Unread count is derived from notification records rather than a `read_at` field on messages (which doesn't exist in the schema). This is accurate: the messaging service creates a `NEW_MESSAGE` notification per received message, and the notification service marks them read when the user views the notification list.

### BUG-MSG-2 pusher auth endpoint wiring

Extended the `app.PusherClient` interface to include `AuthenticatePrivateChannel` rather than using a type assertion. This keeps the interface complete and means the compiler enforces the contract for any future Pusher mock.

The handler returns 503 when `h.pusher == nil` (test environment, where `deps.Pusher = nil`). This is correct: the mobile should degrade gracefully if Pusher is unavailable.

### BUG-MSG-4 pagination reversal

Changed the SQL from `id > cursor ASC` to `id < cursor DESC` and reversed the slice in Go. This means:
- `pages[0]` = newest 50 messages (the first page fetched)
- `pages[1]` = 50 messages older than page 0 (fetched when user scrolls to top)
- Display: `[...pages].reverse().flatMap(...)` gives oldest-at-top ordering

Both the Pusher real-time handler and the `useSendMessage.onSuccess` cache update needed to target `pages[0]` (not the last page) since that's where new messages land.

### BUG-MSG-6 MessageInput async clear

Changed `onSend` prop to `Promise<void> | void` — backward-compatible since void is still a valid return. The `handleSend` is now async and only calls `setText("")` in the try block. If the promise rejects (network error), the catch block does nothing, preserving the text for retry.
