# Commit d8b0ffc — feat: add Maestro E2E messaging flows (task 9.7)

## What changed

- **testIDs added** to all messaging screens and components:
  - `screen-messages` on the conversations list SafeAreaView
  - `screen-conversation` on the chat thread SafeAreaView
  - `conversation-list` on the FlatList in ConversationList
  - `conversation-row` on each Pressable row in ConversationList
  - `message-list` on the FlatList in the conversation screen
  - `input-message` on the TextInput in MessageInput
  - `btn-send-message` on the send Pressable in MessageInput
  - `message-bubble-own` / `message-bubble-received` on MessageBubble based on `isOwn`

- **Backend testonly handler** (`backend/internal/testonly/handler.go`):
  - Added `POST /api/v1/test/conversation` endpoint (only available when E2E_MODE=true)
  - Creates a REQUESTED booking between alice (host) and bob (renter), then inserts two seed messages — one from each party — so the conversation list has a visible entry and the thread has bubbles from both sides
  - Returns `{ transactionId, otherPartyName, listingTitle }`
  - Added helpers: `insertMessage`, `getListingTitle`, `getUserName`

- **Maestro scripts/helpers**:
  - `mobile/e2e/scripts/seed-conversation.js` — calls the new backend endpoint, exports `TRANSACTION_ID`, `OTHER_PARTY_NAME`, `LISTING_TITLE`
  - `mobile/e2e/helpers/seed-conversation.yaml` — wraps the script for use in flows via `runFlow`

- **Maestro flows**:
  - `mobile/e2e/flows/messaging/view-conversations.yaml` — seeds conversation → login → Messages tab → asserts list renders → taps row → asserts thread opens with received bubble
  - `mobile/e2e/flows/messaging/send-message.yaml` — seeds conversation → login → Messages tab → opens thread → types message → taps send → asserts sent bubble appears and input is cleared

## Why this approach

The conversations/messages backend already works through real Pusher+Postgres. The blocking issue was zero testIDs — Maestro can't find any element without them. Adding minimal testIDs (screen wrappers, list, row, input, send button, bubbles) provides all the hooks the flows need without cluttering production code.

The seed endpoint is the same pattern used for bookings (task 9.5–9.6): avoid driving through the full Stripe checkout to set up state. Pre-inserting messages via SQL is idempotent and avoids timing issues with Pusher delivery in tests.

## Verification

- `go build ./...` — passes
- `go vet ./...` — passes
- `npx tsc --noEmit` — passes
