// Capacity ledger: owns bakes, holds and allocations in Postgres and is the single writer
// of capacity. Adapters (Bakehouse routes, webhooks, worker) call these services; the
// ledger itself never talks to Shopify directly — only through CommercePort.
//
// Schema and services land in milestone M1 (see docs/SPEC.md).

export type { Allocation, Hold, Slot } from "@bb/bake-domain";
