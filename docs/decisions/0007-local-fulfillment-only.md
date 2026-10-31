# 0007 — Pickup and local delivery only in v1

**Status:** accepted 2026-10-10

## Context
California cottage food law (AB 1144) permits online direct sales fulfilled in person, by the operator, or by mail — within California only. Fresh sourdough stales and ships poorly.

## Decision
Offer local pickup at Jakki's home and Shopify-native local delivery to a ZIP list or radius around Lincoln. Customers choose a pickup or delivery window per bake (no window limits). No shipping rates in v1.

## Consequences
- Checkout shows Shopify's fixed "Usually ready in…" pickup text; Bakehouse sets the nearest bucket and the bake day is shown everywhere else.
- The pickup address is revealed only in order confirmations.
- In-state shipping is a post-v1 option.
