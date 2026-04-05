# Task 3.7 — Messaging Screen (RN) Handoff

**Status:** Completed  
**Commit:** eb1b8ba  
**Branch:** task-3.7-messaging-screen  
**Date:** 2026-04-04

---

## What Was Built

Messaging screen (React Native / Expo Router) for in-transaction chat between renters and hosts. Connects to the MessagingService (task 3.4) REST API and receives real-time events via Pusher.

### New files

| File | Purpose |
|------|---------|
| `mobile/lib/hooks/useMessages.ts` | `useMessages` — `useInfiniteQuery` with cursor pagination; Pusher `new-message` handler appends to cache. `useSendMessage` — `useMutation` POST with cache update on success. |
| `mobile/lib/hooks/useConversations.ts` | `useConversations` — fetches `GET /api/v1/users/me/conversations`. `useUnreadCount` — polls `GET /api/v1/notifications/unread-count` every 30 s for tab badge. |
| `mobile/components/messaging/MessageBubble.tsx` | Chat bubble: right-aligned (own) / left-aligned (other), sky-600 fill / gray-100 fill, sender name above other-party bubbles, timestamp below. |
| `mobile/components/messaging/MessageInput.tsx` | Multiline `TextInput` (2000-char limit) + circular send button. Button disabled while sending or text is empty; shows `ActivityIndicator` during send. |
| `mobile/components/messaging/ConversationList.tsx` | `FlatList` of conversations with avatar, unread badge dot, last-message preview, relative timestamp. Empty state, error state with retry, and pull-to-refresh. |
| `mobile/app/(tabs)/(messages)/conversation.tsx` | Full-screen chat: `FlatList` (chronological, oldest first), auto-scroll to bottom on mount and new messages, `onStartReached` triggers `fetchNextPage` for older messages, `KeyboardAvoidingView` for iOS/Android. |

### Modified files

| File | Change |
|------|--------|
| `mobile/app/(tabs)/(messages)/index.tsx` | Replaced static placeholder with `ConversationList` wired to `useConversations` + pull-to-refresh that also invalidates unread-count query. |
| `mobile/app/(tabs)/_layout.tsx` | Added `useUnreadCount` import; `tabBarBadge` on the Messages tab shows count when > 0, capped at "99+". |

---

## Key Design Decisions

**Cursor pagination with `useInfiniteQuery`.** Backend returns `{messages, nextCursor}` — `getNextPageParam` threads the cursor forward. "Load more older messages" fires `fetchNextPage` when the user scrolls to the top (`onStartReached`).

**Pusher cache mutation instead of full refetch.** The `new-message` Pusher event appends the incoming `Message` object directly to the last page of the `useInfiniteQuery` cache. This gives instant display without a round-trip. If the Pusher event arrives before the initial query resolves (race), the handler returns early (`if (!old) return old`).

**`useSendMessage` also writes to cache on success.** The mutation `onSuccess` handler appends the server-returned message to the cache. This prevents the sent message from being "dropped" if Pusher delivery is delayed.

**`useUnreadCount` over a dedicated messaging unread count.** The backend tracks all notification types in one table. Polling `GET /api/v1/notifications/unread-count` every 30 s provides a reasonable badge update cadence without websocket overhead for the badge alone.

**`GET /api/v1/users/me/conversations` is a new endpoint.** The MessagingService handoff (3.4) did not list this endpoint. The hook calls it optimistically — the backend will need to implement it to make the list functional. The conversation screen (direct navigation by transactionId) works independently of this endpoint.

**`KeyboardAvoidingView` wraps input.** iOS shifts the layout up when the keyboard appears; `behavior="padding"` on iOS and `"height"` on Android gives consistent push-up behavior.

---

## Navigation

From `BookingStatusScreen` the "Message host/renter" button already navigates to `/(tabs)/(messages)`. With this task, tapping a row in the conversation list pushes to:

```
/(tabs)/(messages)/conversation
  params: { transactionId, otherPartyName }
```

The conversation screen uses `useLocalSearchParams` to read both params.

---

## Verification

```
cd mobile && npx tsc --noEmit   ✓ no errors
```

Manual verification checklist (requires running backend):
- Send message → appears in conversation for both parties in real time
- Scroll to top loads older messages
- Unread badge on messages tab updates correctly
- Push notification tap (future: deep-link) should open correct conversation

---

## Branching

Used vanilla git (Graphite unavailable). Branch `task-3.7-messaging-screen` stacks on `task-3.6-handoff-screens`.

---

## Next Tasks Unblocked

Phase 3 is now fully complete. Phase 4 (AI Agents) is unblocked:
- **4.1** Model router (backend)
