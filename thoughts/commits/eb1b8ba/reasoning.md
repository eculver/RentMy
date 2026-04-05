# Commit eb1b8ba — Reasoning

**Task:** 3.7 Messaging screen (RN)  
**Branch:** task-3.7-messaging-screen

## Why this commit

Task 3.7 is the final task in Phase 3. It closes the transaction loop by giving renters and hosts a way to communicate in-app throughout a booking lifecycle.

The MessagingService (3.4) provided REST and Pusher endpoints; this commit wires those into a full React Native UI. The key challenge was making the chat feel real-time without polling: Pusher `new-message` events update the TanStack Query cache directly, so new messages appear immediately for both the sender (via `useSendMessage` mutation `onSuccess`) and the receiver (via the Pusher handler).

Cursor pagination with `useInfiniteQuery` handles long conversation histories — users load older messages by scrolling to the top while the most recent messages always appear at the bottom of the FlatList.

## What changed

- 6 new files (hooks + components + conversation screen)
- 2 modified files (messages index, tab layout)
- 547 net additions, 0 deletions to existing logic

## Trade-offs

The `GET /api/v1/users/me/conversations` endpoint is called by `useConversations` but was not part of task 3.4's scope. The hook will fail gracefully (TanStack Query error state) until the backend implements it. The conversation screen itself only needs a `transactionId` and works independently.
