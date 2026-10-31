# 0003 — One Bake model; cadence is a setting

**Status:** accepted 2026-10-10

## Context
Jakki's schedule is undecided: weekly drop, several pick-a-date bake days, or continuous made-to-order could each be right, and she may switch.

## Decision
Model every bake event as a Slot (bake date, ordering window, fulfillment windows, total limit, menu with per-product limits). Each cadence is a generator of slots from a single active Schedule. The storefront hides the date picker when one bake is open. Switching cadence only regenerates future, unopened, order-free slots.

## Consequences
- Cadence changes are configuration, not code.
- Holidays and vacations are blackout ranges and manual slot overrides.
- The domain lives in `packages/bake-domain` with property tests for limits, cutoffs and DST.
