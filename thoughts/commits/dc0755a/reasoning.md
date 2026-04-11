# Commit dc0755a — feat: add search screen with debounced input, filters, and infinite scroll

## Why this commit

Task 2.4 in Phase 2 (Discovery + Payments). The discovery backend (task 2.1) exposes a
`GET /api/v1/discovery/search` endpoint with fulltext + proximity filtering. This commit
wires that endpoint into the mobile UI.

## Key decisions

**Debounced input over controlled query:** Storing the raw input in local component state
and only flushing to the Zustand store after 300ms idle means TanStack Query only refetches
when the user pauses typing. Without the debounce, every keystroke would trigger a new
network request.

**`forwardRef` for FilterSheet:** The parent screen needs to imperatively open and close
the sheet. Forwarding the `BottomSheet` ref eliminates the need for an `isOpen: boolean`
state variable in the parent, which would cause a full re-render of the results list on
every filter sheet toggle.

**`GestureHandlerRootView` at the root layout:** `@gorhom/bottom-sheet` v5 requires this
wrapper for its gesture system. Placing it at `app/_layout.tsx` ensures it covers every
screen (including future screens) without further layout changes.

**Local pending state in FilterSheet:** Drive time pills and price inputs are local to
the sheet component. They are only committed to the Zustand store (and thus trigger a
query) when the user taps "Apply". This avoids firing a search on every toggle.
