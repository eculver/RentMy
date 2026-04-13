# Task 8.10 Handoff — Fix: Messaging Bugs

**Status:** Completed  
**Commit:** 6c07d37  
**Branch:** task-8.10-fix-messaging-bugs  
**Date:** 2026-04-11

## What was done

Fixed all 6 bugs identified in the Phase 8.9 messaging audit.

## Bug fixes

### BUG-MSG-1 (Critical) — Add `GET /api/v1/users/me/conversations`

**Files changed:**
- `backend/internal/messaging/model.go` — added `Conversation` struct
- `backend/internal/messaging/repository.go` — added `GetConversations` SQL query (lateral joins for last message and unread count from `NEW_MESSAGE` notifications)
- `backend/internal/messaging/service.go` — added `GetConversations` to `repo` interface and service
- `backend/internal/messaging/handler.go` — added `getConversations` handler, mounted at `GET /users/me/conversations`

### BUG-MSG-2 (Critical) — Add `POST /api/v1/pusher/auth`

**Files changed:**
- `backend/internal/platform/pusher/pusher.go` — added `AuthenticatePrivateChannel` method wrapping the SDK
- `backend/app/server.go` — extended `PusherClient` interface to include `AuthenticatePrivateChannel`; wired pusher to messaging handler via new `WithPusher` method
- `backend/internal/messaging/handler.go` — added `pusherAuthenticator` interface, `WithPusher` method on `Handler`, and `pusherAuth` handler at `POST /pusher/auth`

The auth endpoint returns 503 when Pusher is not configured (test environment). This is the correct behaviour so tests don't require a real Pusher instance.

### BUG-MSG-3 (Medium) — Wrap sendMessage response

**Files changed:**
- `backend/internal/messaging/handler.go` — `writeJSON(w, http.StatusCreated, msg)` changed to `writeJSON(w, http.StatusCreated, map[string]any{"message": msg})`
- `backend/tests/integration/messaging_api_test.go` — `TestSendMessageSuccess` updated to decode from `{ "message": {...} }` envelope

The mobile `useSendMessage` hook already expected the wrapped shape; only the backend needed fixing.

### BUG-MSG-4 (Medium) — Fix pagination direction

**Files changed:**
- `backend/internal/messaging/repository.go` — `FindByTransactionID` now queries `ORDER BY id DESC` and uses `id < cursor` for subsequent pages; results are reversed in Go before returning so each page is still oldest-first. `nextCursor` = ID of the oldest message in the page.
- `mobile/lib/hooks/useMessages.ts` — Pusher and `useSendMessage.onSuccess` cache updates now target `pages[0]` (the newest page) instead of the last page
- `mobile/app/(tabs)/(messages)/conversation.tsx` — flatten pages with `[...data.pages].reverse().flatMap(p => p.messages)` so oldest is at top; use `mutateAsync` so `handleSend` returns a `Promise<void>`

### BUG-MSG-5 (Low) — Tab badge shows message unread count

**Files changed:**
- `mobile/app/(tabs)/_layout.tsx` — replaced `useUnreadCount()` (notification count) with `useConversations()` and sums `unreadCount` across all conversations

### BUG-MSG-6 (Low) — MessageInput clears text only on success

**Files changed:**
- `mobile/components/messaging/MessageInput.tsx` — `onSend` prop type changed to `(content: string) => Promise<void> | void`; `handleSend` is now `async`, calls `await onSend(trimmed)` and only clears `text` in the `try` block after success; failure leaves text intact for retry
- `mobile/app/(tabs)/(messages)/conversation.tsx` — `handleSend` wraps `mutateAsync` to return `Promise<void>`

## Tests added / updated

**Backend integration tests** (`backend/tests/integration/messaging_api_test.go`):
- `TestSendMessageSuccess` — updated to decode from `{ "message": {...} }` envelope
- `TestGetConversationsEmpty` — 200 with empty array when user has no bookings
- `TestGetConversationsWithBooking` — 200 with one conversation after a message is sent
- `TestGetConversationsRequiresAuth` — 401 without token
- `TestPusherAuthRequiresAuth` — 401 without token
- `TestPusherAuthServiceUnavailable` — 503 when Pusher is nil (test environment)

**Backend unit tests** (`backend/internal/messaging/service_test.go`):
- Added `GetConversations` stub method to `stubRepo` to satisfy updated `repo` interface

## Verification

```
cd backend && go vet ./...      # ✓ clean
cd backend && go build ...      # ✓ clean
cd mobile && npx tsc --noEmit   # ✓ same pre-existing errors in profile/index.tsx (unrelated)
cd mobile && npx jest           # ✓ 91 tests pass
```

## Branch mode

Graphite mode — `gt create` succeeded, `gt submit` for push.

## Next task

**Task 8.11** — the next pending task in Phase 8.
