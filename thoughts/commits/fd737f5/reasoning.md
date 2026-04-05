# Commit fd737f5 — MessagingService

**Task:** 3.4 MessagingService (backend)  
**Branch:** task-3.4-messaging-service

## Why this commit

Task 3.4 implements the in-app messaging channel between renters and hosts
for active bookings (PRD §6). Real-time delivery via Pusher and push notifications
to the recipient are essential for the UX — renters and hosts coordinate handoffs
in-app without exchanging phone numbers.

## What changed and why

**messaging package created from scratch** because no messaging infrastructure
existed. The messages table was already in the initial schema (migration 001),
so no new migration was needed.

**Authorization via transactions table** avoids an import cycle: importing the
booking package from messaging would create a cycle since booking imports
notification and we want messaging to be a leaf package. Instead, the
repository does a direct `SELECT renter_id, host_id FROM transactions WHERE id = $1`.

**WithPusher() functional option on booking.Service** keeps the constructor
signature unchanged (already 5 params) while enabling Pusher delivery of
booking-status-changed events. Status changes are now announced in real time
on the private-transaction-{id} Pusher channel for Accept, Decline, Cancel,
CheckIn, and CheckOut.

**NewServiceFromParts** is exported to allow test injection of the repo
interface without depending on a live database. The 10-case unit suite covers
all authorization paths, content validation, and direction of push notifications.
