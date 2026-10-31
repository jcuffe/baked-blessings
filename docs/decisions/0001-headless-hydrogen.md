# 0001 — Headless Shopify: Hydrogen on Oxygen

**Status:** accepted 2026-10-10

## Context
Jared wants to own the executed storefront code; Jakki is non-technical and only needs to manage products, prices and content. Card payments must be PCI-compliant without us handling card data.

## Decision
Build the storefront with Hydrogen (React Router 7) hosted free on Oxygen, on a Shopify Basic plan. Use Shopify's hosted checkout and Shopify Payments. Keep all Storefront API calls in `apps/storefront/app/lib/commerce/`.

## Consequences
- Jakki cannot rearrange layouts herself; she edits products and content in the Shopify admin.
- Checkout cannot be customized before payment on non-Plus plans, which shapes ADR 0002.
- Leaving Shopify means replacing the commerce adapter and checkout; storefront UI and brand carry over.
