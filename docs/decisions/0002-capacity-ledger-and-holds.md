# 0002 — Capacity ledger with cart holds

**Status:** accepted 2026-10-10

## Context
Bakes have a total loaf limit plus optional per-product limits. On non-Plus plans, custom checkout validation (Functions in custom apps) is unavailable, so Shopify can only enforce limits through inventory. Per-date variants and hidden $0 "reservation" lines would enforce limits natively but would show implementation details to Jakki and her customers, who are largely older and non-technical.

## Decision
Own a capacity ledger in Postgres (Bakehouse). Adding to cart places a ~30-minute hold; the storefront only reaches checkout through a `/checkout` action that re-validates the hold; `orders/create` converts holds to allocations. The bake day shows as a visible line-item property; machine data travels in signed hidden properties. Shopify inventory is a system-managed backstop (cap − sold across open bakes; 0 when nothing is open).

## Consequences
- No customer- or admin-visible artifacts.
- With several bakes open, checkout cannot enforce a single bake's limit; rare oversells go to a "Needs attention" queue with plain-language fixes. Future option: an unlisted public app with a cart-validation Function.
- More code to own: holds, webhooks, reconciliation, inventory echo suppression.
