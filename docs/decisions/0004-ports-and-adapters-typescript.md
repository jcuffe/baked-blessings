# 0004 — TypeScript apps with Shopify-free core packages

**Status:** accepted 2026-10-10

## Context
Jared preferred Go, but also wanted shared types with the storefront, no tight coupling of differentiating features to Shopify, and no re-implementation of auth Shopify provides. Shopify's React Router app template supplies auth, session storage, webhook verification and the Admin GraphQL client.

## Decision
Build Bakehouse on Shopify's React Router app template (TypeScript) with Polaris web components. Put the bake rules, ledger and API contracts in `packages/{bake-domain,ledger,contracts}`, which must not import `@shopify/*`, React or React Router (enforced by dependency-cruiser in CI). Apps are thin adapters. Go is used for infrastructure (ADR 0006).

## Consequences
- Shopify auth and webhooks come for free; types are shared end to end.
- Migrating off Shopify replaces the adapters and checkout, not the core.
- Session storage uses Drizzle (`@shopify/shopify-app-session-storage-drizzle`) instead of the template's Prisma, so one ORM covers sessions and the ledger.
