# Task 8.9 Handoff — Audit: Messaging

**Status:** Completed  
**Commit:** a0a94fc  
**Branch:** task-8.9-audit-messaging  
**Date:** 2026-04-11

## What was done

Static code audit of all messaging screens, components, hooks, and backend endpoints. No running simulator (no bookings exist to seed a message thread per Phase 8 Appendix limitations).

## Deliverable

`thoughts/audits/phase-8-visual-qa/audit-messaging.md`

## Bugs found (6 total)

| ID | Severity | Description |
|----|----------|-------------|
| BUG-MSG-1 | Critical | `GET /api/v1/users/me/conversations` endpoint missing in backend |
| BUG-MSG-2 | Critical | `POST /api/v1/pusher/auth` endpoint missing — Pusher private channel auth fails |
| BUG-MSG-3 | Medium | Send message response shape mismatch: backend returns `Message` directly, mobile expects `{ message: Message }` |
| BUG-MSG-4 | Medium | Pagination direction inverted — `fetchNextPage` loads newer messages, not older ones on scroll-to-top |
| BUG-MSG-5 | Low | Tab badge uses notification unread count, not chat message unread count |
| BUG-MSG-6 | Low | MessageInput clears text immediately on send (before mutation confirms success) |

## What works well

- `ConversationList` clean loading/error/empty states with retry CTA
- `MessageBubble` correct own/other-party bubble differentiation
- `MessageInput` correctly disables send while in-flight
- `usePusher` uses ref to avoid stale closure on event handler
- `KeyboardAvoidingView` correct for iOS with manual header
- `useMessages` Pusher handler correctly appends to newest page

## Next task

**Task 8.10 — Fix: Messaging Bugs** — fix all 6 bugs documented in audit-messaging.md.

## Branch mode

Graphite mode — `gt create` succeeded, `gt submit` for push.
