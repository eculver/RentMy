# Task 9.7 — E2E: Messaging Flows

**Status: COMPLETED**
**Branch:** `task-9.7-e2e-messaging-flows` (Graphite mode)

---

## Verification Output

```
Waiting for flows to complete...
[Passed] Messaging - View conversations list (happy path) (35s)
[Passed] Messaging - Send a message (happy path) (39s)

2/2 Flows Passed in 1m 14s
```

---

## Bugs Found and Fixed

### Bug 1: Missing `GET /api/v1/users/me/conversations` endpoint (backend)

**Root cause:** The frontend `useConversations` hook called `api/v1/users/me/conversations` but no such endpoint existed in the backend. The conversations list screen would silently fail to load.

**Fix:** Added `ListConversations` to the messaging handler, service, and repository. The query joins `transactions`, `users`, `listings`, and `messages` (via `LEFT JOIN LATERAL`) to return the other party's name, listing title, last message, and booking status. Ordered by most recent activity (`COALESCE(m.created_at, t.created_at) DESC`).

**Files:**
- `backend/internal/messaging/handler.go` — added `listConversations` handler at `GET /users/me/conversations`
- `backend/internal/messaging/service.go` — added `ListConversations` service method, updated `repo` interface
- `backend/internal/messaging/repository.go` — added `ListConversations` SQL query
- `backend/internal/messaging/model.go` — added `Conversation` struct
- `backend/internal/messaging/service_test.go` — updated `stubRepo` to implement new interface

### Bug 2: `useSendMessage` response shape mismatch (frontend)

**Root cause:** The backend's `sendMessage` handler returns the `Message` struct directly (`writeJSON(w, 201, msg)`), but the frontend `useSendMessage` hook expected a wrapped response (`{ message: Message }`). The destructured `message` was `undefined`, causing the optimistic cache update to insert `undefined` into the messages array.

**Fix:** Changed `useSendMessage` from `.json<{ message: Message }>()` and `({ message }) =>` to `.json<Message>()` and `(message) =>`.

**Files:** `mobile/lib/hooks/useMessages.ts`

### Bug 3: Missing testIDs on all messaging components (frontend)

**Root cause:** None of the messaging components had `testID` props, making them invisible to Maestro's element selectors.

**Fix:** Added testIDs to all interactive messaging elements:
- `screen-messages` on the messages list screen (`View`)
- `screen-conversation` on the conversation detail screen (`View`)
- `conversation-list` on the FlatList in ConversationList
- `conversation-row` on each conversation Pressable row
- `message-bubble-own` / `message-bubble-received` on MessageBubble
- `input-message` on the TextInput in MessageInput
- `btn-send-message` on the send Pressable in MessageInput
- `message-list` on the FlatList in the conversation screen

**Files:**
- `mobile/app/(tabs)/(messages)/index.tsx`
- `mobile/app/(tabs)/(messages)/conversation.tsx`
- `mobile/components/messaging/ConversationList.tsx`
- `mobile/components/messaging/MessageBubble.tsx`
- `mobile/components/messaging/MessageInput.tsx`

### Bug 4: `SafeAreaView` testID not propagated to iOS accessibility tree

**Root cause:** On iOS, `SafeAreaView` from React Native does not always propagate `testID` to the accessibility tree, making Maestro unable to find the element.

**Fix:** Changed the root wrapper from `SafeAreaView` to `View` for the messages list and conversation detail screens. The tab navigator already provides safe area handling.

**Files:**
- `mobile/app/(tabs)/(messages)/index.tsx`
- `mobile/app/(tabs)/(messages)/conversation.tsx`

### Bug 5: Non-existent test seed endpoint for conversations

**Root cause:** `seed-conversation.js` called `POST /api/v1/test/conversation` which never existed. The Maestro `http.post()` API also doesn't support the 3-argument form (url, body, headers) used in the script.

**Fix:** Moved conversation seeding to SQL in `setup.sh` — inserts pre-existing messages directly into the `messages` table on an existing REQUESTED booking. Made the `seed-conversation.yaml` helper a no-op (data comes from setup.sh). Removed `runFlow` calls to the seed helper from both messaging flow YAML files.

**Files:**
- `mobile/e2e/seed/setup.sh` — added `seed_conversations()` function
- `mobile/e2e/helpers/seed-conversation.yaml` — converted to no-op
- `mobile/e2e/scripts/seed-conversation.js` — updated (unused, kept for reference)

### Bug 6: Maestro flow YAML issues (formatting, paths, tab navigation)

**Root cause:** Multiple issues in the original messaging flow YAML files:
- `assertVisible` with `timeout` property — Maestro doesn't support `timeout` on `assertVisible`, only on `extendedWaitUntil`
- `runFlow` paths using `e2e/helpers/...` instead of relative `../../helpers/...`
- Tab bar navigation using plain `tapOn: "Messages"` instead of iOS accessibility pattern `tapOn: text: "Messages, tab.*"`

**Fix:** Rewrote both messaging flow YAML files with correct Maestro syntax.

**Files:**
- `mobile/e2e/flows/messaging/view-conversations.yaml`
- `mobile/e2e/flows/messaging/send-message.yaml`

### Bug 7: Conversation ordering put empty bookings above ones with messages

**Root cause:** Message seed timestamps used `NOW() - INTERVAL '5 minutes'` / `NOW() - INTERVAL '10 minutes'`, while new bookings from `seed_bookings()` were created at `NOW()`. The conversations query orders by `COALESCE(m.created_at, t.created_at) DESC`, so bookings without messages but created more recently appeared above the seeded conversation.

**Fix:** Changed seed message timestamps to `NOW() + INTERVAL '1 second'` / `NOW() + INTERVAL '2 seconds'` to ensure they sort above all booking creation times.

**Files:** `mobile/e2e/seed/setup.sh`

---

## Notes for Next Tasks

- `Maestro http.post()` only accepts 2 args (url, body) — cannot set Authorization headers. Use SQL-based seeding in setup.sh for data that requires authenticated API calls.
- `SafeAreaView` testID doesn't propagate on iOS — use `View` with testID instead when the parent navigator provides safe area handling.
- iOS tab bar items must be tapped with `tapOn: text: "TabName, tab.*"` regex pattern.
- `assertVisible` doesn't support `timeout` — use `extendedWaitUntil: visible: ...` instead.
- `runFlow` and `runScript` paths in helper YAMLs resolve relative to the YAML file containing the command, NOT the working directory.
