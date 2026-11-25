# 0011 — Catalog sync from our database to Stripe through a queue

**Status:** accepted 2026-11-25 (revises 0010's catalog decision)

## Context
ADR 0010 kept Stripe's catalog empty and built each Checkout Session from inline `price_data`. Jared would rather Stripe hold the catalog as a mirror of ours: our Postgres stays the source of truth, Stripe gets as little as possible (ideally an identifier), and the two are kept in step by a queue.

Stripe's posts on database reconciliation (stripe.dev, "Database reconciliation for growing businesses", parts 1 and 2, read 2026-11-25) recommend this shape for one-way sync: changes are stored durably in a queue such as Amazon SQS, a worker applies them to Stripe at a rate Stripe's API limits allow, items leave the queue only after confirmed success, failures retry with backoff, and a dead-letter queue keeps what keeps failing for a person to look at. The posts stay conceptual. They don't cover getting changes into the queue reliably, idempotency, ordering or drift checks, so this ADR fills those in.

## Decision
### What Stripe holds
- One Stripe **Product per sellable SKU** (a product, or one value of its option, such as "Country Loaf, Sliced"). The Stripe product's `id` is our SKU, so Stripe needs no lookup table to find our item and we need none to find theirs.
- The Product carries only what Stripe Checkout shows the customer: name, first photo, `active`. Its metadata holds our product ID and the catalog version it reflects. Descriptions, ingredients, allergens, capacity weights and costs stay in our database only.
- One active **Price** per SKU, in USD, with `lookup_key` set to the SKU. Stripe prices can't change amount, so a price change creates a new Price (moving the lookup key to it) and deactivates the old one.
- Hiding or deleting a product in the admin archives its Stripe Product (`active: false`). Nothing is deleted in Stripe.
- Our rows store the Stripe price ID and the catalog version that last reached Stripe.

### How a change gets to Stripe
1. **Outbox, in the same transaction.** Every admin change to a product, option, price, photo or visibility bumps the SKU's `version` and writes a row to `stripe_sync_outbox` (SKU, version, created at, published at) in the same Postgres transaction. A change is never saved without its sync request, and a sync request never exists for a change that rolled back.
2. **Relay to SQS.** Right after the commit, Bakehouse sends the new outbox rows to an **SQS FIFO queue** and marks them published. A scheduled relay (EventBridge Scheduler, every minute) sends any row still unpublished, so a crash between commit and send only delays the sync. The message group is the SKU, so changes to one item are applied in order, and the deduplication ID is the outbox row ID.
3. **Worker.** A Lambda worker consumes the queue. It treats the message as "SKU X changed", reads the SKU's **current** row from Postgres and makes Stripe match it: create or update the Product, create a new Price if the amount differs, archive if hidden. It then records the synced version and price ID. Because it works from current state, a duplicate, late or out-of-date message changes nothing. Every Stripe write carries an idempotency key of SKU, version and step, so a retry after a timeout can't create a second Price.
4. **Failures.** The worker reports failures per message, so one bad item doesn't hold up the batch. A failed message becomes visible again after a growing delay, and after 5 tries it moves to a **dead-letter queue**. A dead-lettered item raises a "Needs attention" card in the admin ("Country Loaf didn't reach Stripe") with a Retry button, and a CloudWatch alarm fires on the dead-letter queue's depth.
5. **Rate.** The worker's concurrency is capped at 2, far below Stripe's API rate limits, so a bulk edit drains steadily.

### Checkout never waits on the queue
The checkout action prices every line from our database, as before. For each line:
- if the SKU's synced version equals its current version, the line uses the Stripe Price ID;
- otherwise (a change still in the queue, or a dead-lettered one) the line uses inline `price_data` with our amount, attached to the existing Stripe Product, or with inline `product_data` if the item has never reached Stripe.

So customers are always charged the price our database shows, and a slow or stuck sync never blocks a sale.

### Changes made directly in Stripe
Our database wins. Stripe's `product.updated`, `product.deleted`, `price.created` and `price.updated` webhooks are checked against the version in metadata. A change we didn't make (someone edited the product in the Stripe Dashboard) is written to the audit log and a resync is queued, which puts our data back.

### Reconciliation
A nightly job lists every Stripe Product and active Price and compares them with our catalog: missing, extra, wrong name, photo, amount or `active`. Each mismatch is queued for the worker, and the admin's Settings page shows when the last check ran and what it fixed. This catches anything the outbox, the webhooks or a dead-letter retry missed.

### Hosting
The worker runs from the Bakehouse image (pod-ready per 0009): on Lambda, SQS events reach a Bakehouse route through the Lambda Web Adapter's non-HTTP pass-through (to confirm in the M2 spike); on Fargate or EKS later, the same code runs as a long-polling consumer. Pulumi manages the FIFO queue, its dead-letter queue, the event source mapping, the two schedules and the alarm. At this store's volume SQS stays inside its free tier.

## Alternatives considered
- **Inline `price_data` only (0010).** No sync at all, but Stripe's Dashboard, reports and receipts show no catalog, and per-product sales in Stripe can't be grouped. Kept only as checkout's fallback.
- **Sync directly from the admin request.** Simple, but a Stripe outage or timeout loses the change with no retry, which is the failure the Stripe posts warn about.
- **A queue in Postgres (pg-boss or `SKIP LOCKED`).** One less AWS service, but polling Postgres keeps the database busy and the dead-letter, alarm and retry tooling would be ours to build. SQS gives them for free and matches 0009.

## Consequences
- The bake domain stays provider-free: the catalog service only writes outbox rows, and the Stripe calls live in the Bakehouse Stripe adapter.
- Stripe's Dashboard and reports show real products, which also prepares for Stripe Terminal at farmers markets and Stripe Billing for a loaf club later.
- New moving parts to test and watch: the outbox relay, the worker, the dead-letter queue and the nightly reconciliation.
- Customers and orders are not synced to Stripe in v1. Checkout creates Stripe's customer record, and webhooks bring payment state back as 0010 describes.
