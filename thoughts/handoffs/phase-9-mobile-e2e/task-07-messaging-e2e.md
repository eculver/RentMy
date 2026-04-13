# Task 9.7 Handoff — E2E: Messaging Flows

**Status:** completed  
**Branch:** task-9.7-messaging-e2e  
**Commit:** d8b0ffc  
**Date:** 2026-04-12

---

## What was done

Implemented Maestro E2E flows for the two messaging user flows defined in the phase-9 plan:

1. **View conversations** — confirms the conversation list renders and tapping a row opens the thread
2. **Send message** — confirms typing and sending a message produces a visible sent bubble and clears the input

### Files changed

| File | Change |
|------|--------|
| `mobile/app/(tabs)/(messages)/index.tsx` | Added `testID="screen-messages"` |
| `mobile/app/(tabs)/(messages)/conversation.tsx` | Added `testID="screen-conversation"`, `testID="message-list"` |
| `mobile/components/messaging/ConversationList.tsx` | Added `testID="conversation-list"`, `testID="conversation-row"` |
| `mobile/components/messaging/MessageInput.tsx` | Added `testID="input-message"`, `testID="btn-send-message"` |
| `mobile/components/messaging/MessageBubble.tsx` | Added `testID="message-bubble-own"` / `"message-bubble-received"` |
| `backend/internal/testonly/handler.go` | Added `POST /test/conversation` + helpers |
| `mobile/e2e/scripts/seed-conversation.js` | New — calls seed endpoint |
| `mobile/e2e/helpers/seed-conversation.yaml` | New — wraps script for Maestro |
| `mobile/e2e/flows/messaging/view-conversations.yaml` | New — view conversations flow |
| `mobile/e2e/flows/messaging/send-message.yaml` | New — send message flow |

### testIDs added

No messaging components had any testIDs before this task. The following were added:

- `screen-messages` — conversation list screen wrapper
- `screen-conversation` — chat thread screen wrapper
- `conversation-list` — FlatList of conversations
- `conversation-row` — each Pressable row in the list
- `message-list` — FlatList of messages in the thread
- `input-message` — the TextInput in MessageInput
- `btn-send-message` — the send Pressable in MessageInput
- `message-bubble-own` — outgoing message bubble (sent by current user)
- `message-bubble-received` — incoming message bubble (sent by other party)

### Backend seed endpoint

`POST /api/v1/test/conversation` (E2E_MODE=true only):
- Creates a REQUESTED booking between alice@test.com (host) and the specified renter (default bob@test.com)
- Inserts two seed messages: one from the host, one from the renter
- Returns `{ transactionId, otherPartyName, listingTitle }`

Pattern mirrors the existing `POST /test/booking` endpoint from task 9.5.

---

## How to verify

```bash
# Backend + simulator must be running
# Backend: E2E_MODE=true make dev  (from backend/)
# App: EXPO_PUBLIC_E2E_MODE=true npx expo run:ios  (from mobile/)

cd mobile
maestro test e2e/flows/messaging/view-conversations.yaml
maestro test e2e/flows/messaging/send-message.yaml

# Or run the full messaging suite:
maestro test e2e/flows/messaging/
```

---

## Dependencies used

- Task 9.1 (auth helpers) — `login-as-renter.yaml` helper
- Task 9.0 (infrastructure) — Maestro install, directory structure, dev.env

---

## Graphite mode

Branch created with `/opt/homebrew/bin/gt create` — Graphite mode active this session.

---

## Next tasks

- **9.8** — E2E: Dispute & rating flows (depends on 9.5)
