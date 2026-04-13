# Audit: Messaging — Task 8.9

**Date:** 2026-04-11  
**Auditor:** Claude (task-8.9-audit-messaging)  
**Method:** Static code audit (no running simulator — no bookings exist to seed a message thread; see Phase 8 Appendix limitations)

---

## Files Reviewed

| File | Notes |
|------|-------|
| `mobile/app/(tabs)/(messages)/index.tsx` | Conversation list screen |
| `mobile/app/(tabs)/(messages)/conversation.tsx` | Single conversation screen |
| `mobile/components/messaging/ConversationList.tsx` | List rendering + row navigation |
| `mobile/components/messaging/MessageBubble.tsx` | Individual message bubble |
| `mobile/components/messaging/MessageInput.tsx` | Text input + send button |
| `mobile/lib/hooks/useConversations.ts` | TanStack Query hook for conversation list + unread count |
| `mobile/lib/hooks/useMessages.ts` | Infinite query hook + Pusher subscription + send mutation |
| `mobile/lib/hooks/usePusher.ts` | Pusher channel subscription hook |
| `mobile/app/(tabs)/_layout.tsx` | Tab bar badge wiring |
| `backend/internal/messaging/handler.go` | REST handlers (send, list) |
| `backend/internal/messaging/service.go` | Business logic |
| `backend/internal/messaging/repository.go` | SQL queries |
| `backend/internal/messaging/pusher.go` | Channel constants |
| `backend/internal/user/handler.go` | User routes (checked for conversations endpoint) |
| `backend/internal/notification/handler.go` | Notification routes |
| `backend/app/server.go` | Router assembly |

---

## Bugs Found

### BUG-MSG-1 — Missing `/users/me/conversations` backend endpoint (Critical)

**File:** `mobile/lib/hooks/useConversations.ts:27`, `backend/internal/user/handler.go`

**Problem:** `useConversations()` calls `GET /api/v1/users/me/conversations`. This route does not exist anywhere in the backend. The user handler only mounts `GET /users/me` and `PUT /users/me`. The Messages tab will always render the error state and conversations list will be perpetually empty.

**Reproduction:** Navigate to the Messages tab → `ConversationList` error branch renders immediately.

**Fix needed:** Add `GET /users/me/conversations` to the user handler. The query should JOIN `transactions` + `messages` for the authenticated user (as renter or host), returning the `Conversation` shape the frontend expects:
```go
type ConversationResponse struct {
    TransactionID  string    `json:"transactionId"`
    OtherPartyID   string    `json:"otherPartyId"`
    OtherPartyName string    `json:"otherPartyName"`
    ListingTitle   string    `json:"listingTitle"`
    LastMessage    *string   `json:"lastMessage,omitempty"`
    LastMessageAt  *string   `json:"lastMessageAt,omitempty"`
    UnreadCount    int       `json:"unreadCount"`
    BookingStatus  string    `json:"bookingStatus"`
}
```

---

### BUG-MSG-2 — Missing `/pusher/auth` backend endpoint (Critical)

**File:** `mobile/lib/hooks/usePusher.ts:51`, `backend/app/server.go`

**Problem:** `usePusher` configures Pusher channel authorization to use `POST /api/v1/pusher/auth`. This endpoint does not exist. Soketi/Pusher requires the server to sign private channel subscription requests. Without this endpoint returning a valid auth signature, all `private-transaction-*` channel subscriptions will fail with a 403 from the auth endpoint (HTTP 404 in this case), causing Pusher to emit an `pusher:subscription_error` event and never deliver real-time messages.

**Reproduction:** Open any conversation screen → Pusher attempts to subscribe to `private-transaction-<id>` → auth call to `/api/v1/pusher/auth` returns 404 → no real-time messages received.

**Fix needed:** Add `POST /api/v1/pusher/auth` to the messaging handler (or a dedicated pusher handler). The handler must call `pusher.Client.AuthenticatePrivateChannel(params, socketID)` using the Pusher HTTP Go SDK and return the resulting auth token. Must be behind auth middleware so only authenticated users can authorize their own channels.

---

### BUG-MSG-3 — `POST /bookings/:id/messages` response shape mismatch (Medium)

**File:** `backend/internal/messaging/handler.go:73`, `mobile/lib/hooks/useMessages.ts:72-84`

**Problem:** The backend sends the `Message` object directly:
```go
writeJSON(w, http.StatusCreated, msg)
// → {"id":"...","transactionId":"...","senderId":"...","content":"...","createdAt":"..."}
```
The mobile hook expects a `{ message: Message }` wrapper:
```typescript
.json<{ message: Message }>()
onSuccess: ({ message }) => { /* appends to cache */ }
```
`message` destructures as `undefined`, so `onSuccess` silently skips the cache update. The sent message does not appear in the chat UI until the next Pusher event or manual refetch.

**Fix options:**
- Wrap backend response: `writeJSON(w, http.StatusCreated, map[string]any{"message": msg})`
- Or unwrap mobile: `.json<Message>()` and adjust `onSuccess` — but this would break consistency with other endpoints

Recommend wrapping in the backend to match the mobile expectation.

---

### BUG-MSG-4 — Pagination direction inverted in `useMessages` (Medium)

**File:** `mobile/lib/hooks/useMessages.ts:57-58`, `backend/internal/messaging/repository.go:61-67`

**Problem:** The conversation screen shows a standard chat layout (oldest at top, newest at bottom). Scrolling to the **top** triggers `onStartReached → fetchNextPage()`. However:

1. `getNextPageParam` returns `nextCursor` = last message ID of the latest page (the **newest** message loaded).
2. `fetchNextPage()` passes this cursor to the backend.
3. The backend executes `WHERE id > $cursor ORDER BY ASC` — returning messages **newer** than the cursor.

Result: scrolling to the top loads even newer messages (forward in time), not older messages. The chat is effectively broken for threads longer than one page — users can never load their earlier message history.

**Fix needed:** Reverse the pagination direction:
- Backend: add a `direction` parameter or expose a backward cursor (`id < $cursor ORDER BY id DESC LIMIT $limit`, then reverse the result slice before returning).
- Mobile: use `getPreviousPageParam` instead of `getNextPageParam`, pass the oldest loaded message ID as cursor, and call `fetchPreviousPage()` in `handleLoadMore`.

---

### BUG-MSG-5 — Tab badge counts all notifications, not unread messages (Low)

**File:** `mobile/lib/hooks/useConversations.ts:33-39`, `mobile/app/(tabs)/_layout.tsx:62`

**Problem:** `useUnreadCount()` fetches `/api/v1/notifications/unread-count`, which returns a count of all unread system notifications (booking confirmations, dispute updates, rating prompts, etc.). The Messages tab badge displays this count, implying it represents unread chat messages. A user with 5 unread booking-status notifications but 0 unread chat messages will see `5` on the Messages tab — misleading.

**Fix options:**
- Add a separate endpoint `/api/v1/users/me/conversations/unread-count` that counts only unread chat messages, OR
- Include the unread message count in the conversations response (BUG-MSG-1 fix) and sum it on the frontend.

---

### BUG-MSG-6 — MessageInput clears text before send confirmation (Low)

**File:** `mobile/components/messaging/MessageInput.tsx:17-20`

**Problem:**
```typescript
const handleSend = () => {
  const trimmed = text.trim();
  if (!trimmed || isSending || disabled) return;
  onSend(trimmed);  // fires mutation
  setText("");       // clears immediately, before mutation settles
};
```
If the send mutation fails (network error, 4xx), the user's typed message is already cleared from the input — it's silently lost. The user must retype.

**Fix:** Either hold the text until `onSuccess` and clear there, or re-populate on `onError`. Clearing optimistically is common UX but should at minimum restore text on failure.

---

## Summary

| ID | Severity | Area | Description |
|----|----------|------|-------------|
| BUG-MSG-1 | Critical | Backend | `GET /users/me/conversations` endpoint missing |
| BUG-MSG-2 | Critical | Backend | `POST /pusher/auth` endpoint missing |
| BUG-MSG-3 | Medium | Backend+Mobile | Send message response shape mismatch |
| BUG-MSG-4 | Medium | Mobile | Pagination direction inverted (loads newer instead of older) |
| BUG-MSG-5 | Low | Mobile | Tab badge counts all notifications, not just unread messages |
| BUG-MSG-6 | Low | Mobile | MessageInput clears text before mutation confirms |

**Total bugs: 6** — 2 critical, 2 medium, 2 low.

---

## What Works Well

- `ConversationList` has clean loading/error/empty states with retry CTA.
- `MessageBubble` correctly differentiates own vs. other-party messages with color + alignment.
- `MessageInput` correctly disables send button while a send is in flight.
- `usePusher` correctly uses a `ref` for the event callback to avoid stale closures.
- `usePusher` correctly skips subscription when `channelName` is null.
- Unread badge count (9+ cap) and per-conversation unread badge in `ConversationList` are correctly implemented (pending BUG-MSG-1 providing the data).
- `KeyboardAvoidingView` uses correct `behavior` for iOS and `keyboardVerticalOffset={0}` is appropriate since the screen manages its own header (no native nav header).
- `FlatList` has correct `onStartReached` + `onStartReachedThreshold` props for triggering older-message loads on scroll-to-top (correct trigger; direction bug is separate).
- `useMessages` Pusher handler correctly appends to the **last** (newest) page rather than prepending to the first page, maintaining chronological order.
