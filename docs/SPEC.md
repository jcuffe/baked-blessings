# Jakki's Baked Blessings — v1 Storefront Specification

> Status: approved 2026-10-10; revised 2026-11-25 for AWS hosting ([ADR 0009](decisions/0009-aws-hosting.md)) Stripe Checkout with our own catalog and admin ([ADR 0010](decisions/0010-stripe-checkout-own-catalog.md)), and the queued catalog sync to Stripe ([ADR 0011](decisions/0011-catalog-sync-to-stripe-queue.md)). Decisions are recorded as ADRs in [`decisions/`](decisions/). Compliance notes: [`compliance.md`](compliance.md).

## Context
Jakki bakes sourdough at home in Lincoln, CA (Placer County) under a California Cottage Food registration. We are building an online storefront so customers can order with a credit card for local pickup or delivery.

The hardest problem is matching what customers expect from a shop with the physical limits of baking: oven capacity, bake days, and spoilage. Jakki, in her 60s and non-technical, and her many older customers must never see anything that looks like an implementation detail.

Jared (technical) builds and owns the code in the repo. Jakki runs the whole business from the Bakehouse admin: products, bakes, orders, refunds, customers, messages, site content and settings.

## Guiding principles
1. **Feels like any mainstream shop.** Customers and Jakki never see slots, holds, SKUs, or jargon. The site uses plain words: "bake day", "ordering closes", "loaf limit". The admin uses the same plain words.
2. **Accessible by default.**
   - WCAG 2.2 AA, with an 18px base font and tap targets of at least 44px, on the storefront and in the admin.
   - Contrast around 7:1 for body text.
   - Dates spelled out, e.g. "Saturday, October 18". No visible timers, no hover-only controls.
3. **Own the differentiators and the records, rent the commodities.**
   - Our Postgres is the source of truth for the catalog, bakes, orders and customers.
   - Stripe processes payments. It holds a mirror of the catalog (identifier, name, photo, price), kept in step from our database through a queue; our database always wins.
   - The bake domain and capacity ledger live in provider-free packages (ports and adapters, enforced by lint).
   - Infrastructure is code (Pulumi Go) on AWS.
4. **Flexible cadence.** Switching between weekly drop, pick-a-date, and made-to-order is a settings change, not a code change.
5. **Compliance is visible.** Every product page and the footer show "Made in a Home Kitchen", Placer County, and her registration number (AB 831). Every product lists ingredients and allergens.
6. **Pod-ready.** Every app ships as a container built in CI, configured by environment variables, with a `/healthz` endpoint and graceful shutdown.

## Decision log
| Area | Decision |
|---|---|
| Payments | **Stripe Checkout** (hosted) in Jakki's Stripe account: cards, Apple Pay, Google Pay, Link. Stripe's catalog mirrors ours (one Product per SKU, keyed by the SKU), synced through an outbox and an SQS FIFO queue. |
| Catalog and orders | **Our Postgres** is the system of record. Stripe webhooks only advance an order's payment state. |
| Storefront | **React Router 7** (TypeScript, server-rendered), no Hydrogen. Runs as a container on AWS Lambda behind CloudFront. |
| Bakehouse | Standalone **React Router 7** app: the full-service admin for Jakki, the storefront's private API, Stripe webhooks, and scheduled jobs. Its own sign-in. UI built with **React Aria Components** for accessibility. |
| Domain isolation | Ports and adapters. `bake-domain`, `ledger`, and `contracts` are provider-free; the apps are thin adapters. dependency-cruiser fails CI on any `@shopify/*`, `stripe`, or `react-router` import in the core packages. |
| Languages | **TypeScript** for the apps and packages. **Go** for IaC (Pulumi). |
| Hosting | **AWS**: Lambda (container images through the Lambda Web Adapter) behind CloudFront for now; Fargate or EKS later with the same images. Postgres on AWS (instance type chosen in a follow-up ADR with costs). Photos in S3 behind CloudFront. |
| IaC | **Pulumi (Go)** in `infra/`, with stacks `staging` and `prod`. State in Pulumi Cloud Free, using Pulumi-managed secrets. |
| DNS/email | Domain bought by hand. DNS moves to Route 53 with the domain. Inbound `hello@` forwarding and outbound email through **Resend**. |
| Ordering cadence | A unified **Bake** model. The cadence mode is a setting: weekly drop, pick-a-date, or made-to-order. |
| Capacity | A total loaf limit per bake, plus optional per-product limits. Products carry a capacity weight. |
| Enforcement | Our own **capacity ledger** with cart holds. A hold follows the order into Stripe Checkout and is released only when the session expires, so limits are exact. |
| Fulfillment | Local pickup at her home, plus local delivery (ZIP list, fee and minimum set in the admin). **No shipping in v1.** Customers pick a window; windows have no limits. |
| Policy | Free cancellation before ordering closes (Jakki cancels and refunds from the admin, and the loaf is released automatically). Sales are final after the cutoff. Unclaimed orders are not refunded. |
| Catalog | A few loaves with at most one real option (e.g., Sliced/Unsliced). Schema is ready for other bake items and for unscheduled items. |
| Accounts | Guest checkout. Customers get a private order link by email. Customer accounts are deferred. |
| Comms | Our own order emails through **Resend** (confirmation, cancelled/refunded, ready for pickup, out for delivery). Day-before reminder by email and by SMS (**Twilio**, opt-in). "Ordering is open" broadcasts. |
| Tax | Bread is exempt food in California: no sales tax in v1. Stripe Tax per line (`tax_code`) if taxable items are added. |
| Voice | Warm and homey, lightly faith-touched: a verse on the About page and in the confirmation. Built from her existing logo and photos. |
| Tooling | **mise** pins Node, pnpm, Go, and Pulumi. pnpm workspaces + Turborepo for TypeScript; Go module in `infra/`. Strict TS, Vitest + fast-check, Playwright, golangci-lint, GitHub Actions. |
| Domain/launch | Buy a domain. No hard date: launch after the compliance checklist and a friends-and-family soft launch. |

## Architecture
```
Customer ──► Storefront (Lambda + CloudFront) ──HMAC-signed──► Bakehouse API ──► Postgres
                │ redirect with a Checkout Session                ▲      │
                ▼                                                 │      └─► Resend · Twilio · S3 (photos)
          Stripe Checkout ──── webhooks (completed, expired, refunds, disputes) ─┘
Jakki ──► Bakehouse admin (her own sign-in) ──► products, bakes, orders, refunds, customers, content, settings
Scheduler (EventBridge) ──► Bakehouse jobs: reminders, "ordering is open", bake generation, hold sweeper, outbox relay, nightly Stripe reconciliation
Catalog change ──► outbox row (same transaction) ──► SQS FIFO ──► Stripe sync worker ──► Stripe Products/Prices   (5 failures ──► dead-letter queue ──► "Needs attention")
Pulumi (Go) ──► AWS (ECR, Lambda, CloudFront, S3, SQS, Postgres, Scheduler, Route 53) · GitHub environments & variables
```

### Repo layout
```
.mise.toml  package.json  pnpm-workspace.yaml  turbo.json  tsconfig.base.json  .dependency-cruiser.cjs
apps/storefront/            React Router 7 storefront (container: Dockerfile, container/server.mjs)
  app/lib/bakehouse/        typed, HMAC-signing client built from packages/contracts (server-side only)
apps/bakehouse/             React Router 7: admin UI, storefront API, Stripe webhooks, jobs
  app/routes/admin.*        thin admin routes: requireStaff → ledger service → UI
  app/routes/api.storefront.*  our HMAC check → ledger service
  app/routes/webhooks.stripe   Stripe signature check → ledger service
  app/adapters/stripe/      StripePayments implements PaymentsPort; StripeCatalogSync applies a SKU's current state to Stripe
  app/routes/events.sqs     Stripe sync worker (SQS events via the Lambda Web Adapter pass-through)
  app/adapters/{resend,twilio,s3}/
  jobs/                     scheduled jobs (invoked by EventBridge Scheduler, or pg-boss in dev)
  Dockerfile
packages/bake-domain/       pure TS: cadence generators, slots, cutoffs, capacity math, tz; defines ports
packages/ledger/            application services + Postgres repositories (Drizzle); no providers or frameworks
packages/contracts/         zod schemas + types for the storefront↔Bakehouse API
packages/brand/             tokens.css, fonts, logo
infra/                      Pulumi Go: main.go, internal/{storefront,ci,...}
docs/                       SPEC.md, decisions/ (ADRs), runbooks/, compliance.md
```

**Data access.** Drizzle for all tables. Migrations use drizzle-kit and run on deploy. Ledger writes use `SELECT … FOR UPDATE` row locks.

## Catalog (system of record: our Postgres)
- **Product:** name, slug, short and long description, price (cents), photos (S3 keys, ordered, with alt text), ingredients, allergens, storage tips, capacity weight, `scheduled` flag (scheduled products sell per bake; unscheduled sell anytime), active/hidden, sort order, and an internal SKU.
- **ProductOption:** at most one real option per product in v1 (e.g., Sliced/Unsliced), each value with an optional price change and its own SKU.
- **Stripe holds a mirror, synced through a queue** ([ADR 0011](decisions/0011-catalog-sync-to-stripe-queue.md)):
  - One Stripe Product per SKU, whose `id` is the SKU, with only the name, first photo and `active`, plus our product ID and catalog version in metadata. One active Price per SKU with `lookup_key` = SKU; a price change creates a new Price and deactivates the old one. Hidden or deleted products are archived in Stripe, never deleted.
  - Each SKU row has a `version`, plus the Stripe price ID and the version last synced.
  - Every catalog change bumps the version and writes a `stripe_sync_outbox` row in the same transaction. Bakehouse sends new rows to an SQS FIFO queue right after commit (group = SKU, dedup ID = outbox row ID); a relay every minute sends any it missed.
  - The worker reads the SKU's current row and makes Stripe match it, with idempotency keys of SKU, version and step, so duplicate or stale messages change nothing. Failures retry with a growing delay; after 5 tries a message goes to the dead-letter queue and the admin shows "Country Loaf didn't reach Stripe" with a Retry button. Concurrency is capped at 2.
  - Edits made in the Stripe Dashboard are detected through `product.*` and `price.*` webhooks, logged, and overwritten from our database. A nightly reconciliation lists Stripe's products and prices, compares them with ours and queues every mismatch.
- **Order lines snapshot** name, option, unit price, and capacity weight at the time of the order, so later edits never change past orders.

## Bake domain (`packages/bake-domain`, persisted by `packages/ledger`)
These terms are internal only; the UI says "bake".
- **Schedule** (one active):
  - `mode`: `WEEKLY_DROP | PICK_A_DATE | MADE_TO_ORDER`
  - Bake weekdays.
  - Rule for when ordering opens: an offset, or "always" for made-to-order.
  - Rule for when ordering closes: an offset before the bake, or a lead time in hours.
  - Default windows per weekday, a default total limit, and a default menu with per-product limits.
  - A generation horizon (e.g., 21 days).
- **Slot (a bake):**
  - `bakeDate`, `opensAt`, `cutoffAt`, `totalCapacity`.
  - `status`: scheduled → open → closed → baking → fulfilled, or cancelled.
  - `source`: generated or manual. Manual overrides survive regeneration.
- **SlotWindow:** `method` (PICKUP or DELIVERY), start, end.
- **SlotMenuItem:** product, optional per-product cap. The capacity weight is snapshotted from the product.
- **Blackout:** date ranges for vacations and holidays.
- **Cart:** an anonymous cart (signed cookie) with lines, chosen bake, method, window, and delivery address if any.
- **Hold / HoldLine:** the cart's reservation (slot, window, lines, `expiresAt`, optional `checkoutSessionId`). Status: ACTIVE, CHECKING_OUT, CONVERTED, EXPIRED, or RELEASED.
- **Order / OrderLine:** created at checkout. Status: PENDING_PAYMENT → PAID → READY → COMPLETED (picked up or delivered), or ABANDONED, CANCELLED. Refunds are recorded separately (full or partial) and don't change capacity unless the order is cancelled.
- **Allocation:** an order line assigned to a slot, counted in units = qty × weight. Status: ACTIVE or RELEASED.
- **Customer:** email, name, phone, email-marketing consent, SMS reminder opt-in, notes. Created or matched by email when an order is paid.
- **Issue:** a "Needs attention" item: OVERSOLD, LATE_PAYMENT, DISPUTE, REFUND_OUTSIDE_ADMIN, BAD_SIGNATURE.
- **Infrastructure tables:** ProcessedEvent (webhook de-duplication), Notification log, AuditLog, StaffUser, Session.
- **Ports:**
  - `PaymentsPort`: createCheckout(order) → redirect URL and session ID, getCheckoutStatus, refund(order, amount).
  - `Notifier`: email and SMS.
  - `PhotoStore`: signed upload URL, public URL.
  - `Clock`.
  - Repositories.

**Capacity math.**
- Remaining total = cap − Σ active allocations − Σ other active or checking-out holds.
- A per-product limit is computed the same way.
- Quantity a customer can add = min(⌊remaining total ÷ weight⌋, remaining for that product).

**Cadence switching.** Changing the mode only regenerates future slots that have no orders and aren't open yet. Existing orders are never moved.

**Storefront behavior.**
- With one open bake, the bake day is implied. The header shows "Next bake: Saturday, Oct 18 · order by Wednesday 8 pm".
- With more than one open bake, a friendly date chooser appears before the first add-to-cart.
- With none, it shows "Ordering opens Monday at 8 am" and a notify-me signup.

## Order flow and enforcement
1. **Add to cart.** A storefront action calls `POST /api/storefront/holds` (contract in `packages/contracts`), which reserves capacity atomically. On failure, plain words: "Only 2 Country Loaves left for Saturday." Cart holds last 30 minutes and are quietly renewed while the customer is active; an expired hold is re-taken if capacity allows. The customer never sees a timer.
2. **Change bake day or window in the cart.** All lines are re-held in one transaction. If capacity isn't there, the cart suggests the next bake that has room.
3. **Cart details.** Pickup or delivery, the window, and for delivery the address (checked against the delivery ZIP list, with the fee and minimum applied). Phone number plus "Text me a reminder" opt-in, and order notes.
4. **Checkout.** The checkout button posts to the storefront's `/checkout` action, which calls Bakehouse to:
   - re-validate the hold and move it to CHECKING_OUT;
   - create the Order and its lines (snapshots) as PENDING_PAYMENT;
   - create a Stripe Checkout Session whose lines use each SKU's synced Stripe Price, or inline `price_data` on the Stripe Product when the SKU's latest change is still in the queue (amounts always come from our database), plus a delivery-fee line, `client_reference_id` and metadata set to the order ID, `expires_at` 30 minutes out, email collection, and marketing consent collection;
   - store the session ID on the order and hold, then redirect to Stripe.
5. **Paid** (`checkout.session.completed`). Verify the signature, de-duplicate by event ID, look up the order by ID, check the amount, mark it PAID, convert the hold into allocations, upsert the Customer, and send the confirmation. If the hold was somehow released, allocate anyway when capacity allows; otherwise raise OVERSOLD.
6. **Abandoned** (`checkout.session.expired`). Mark the order ABANDONED and release the hold. A sweeper also checks, through the Stripe API, any session past its expiry without an event, so holds never leak. Holds in CHECKING_OUT are never released by a timer alone.
7. **Cancel or refund** (from the admin). Cancelling refunds the payment through `PaymentsPort.refund` and releases allocations. A partial refund (e.g., a goodwill credit) keeps the allocations. Refund webhooks (`charge.refunded`, `refund.updated`) reconcile; a refund made outside the admin raises REFUND_OUTSIDE_ADMIN so Jakki can decide whether to release the loaf.
8. **Disputes** (`charge.dispute.created`) raise DISPUTE with plain next steps.
9. **Ready and done.** Jakki marks orders ready (sends "ready for pickup" or "out for delivery") and completed from the admin or the printable sheets.

## Storefront
**Pages:**
- Home: current bake status, featured loaves, "How it works" in 3 steps, About teaser.
- Menu: this bake's loaves.
- Product page: photos, description, price, ingredients, allergens, storage tips, home-kitchen disclosure.
- Cart: bake day, Pickup/Delivery choice and window, delivery address, phone number plus "Text me a reminder" opt-in, notes.
- Checkout is Stripe's hosted page, branded with her logo and colors. It returns to our confirmation page, which shows the bake day and window.
- Order page: a private link (signed token) emailed with the confirmation, showing status, bake day, window, and items.
- Content pages, edited in the admin: About Jakki (with a verse), How ordering works (FAQ), Pickup & delivery (address reads "Lincoln, CA — exact address in your confirmation"), Contact, and the refund, privacy, and terms policies.
- Footer: "Made in a Home Kitchen · Registered Cottage Food Operation, Placer County · Reg. #____".

**Other storefront requirements:**
- Analytics: none from third parties. Sales reports come from our own orders; page-view counts, if wanted, from CloudFront logs.
- SEO and local business schema markup.
- Unscheduled products (`scheduled=false`) skip the bake chooser and are sellable anytime.
- Product photos are served from S3 through CloudFront in sizes suited to phones.

## Bakehouse admin (full-service, plain language)
Jakki manages everything here. She never needs the Stripe Dashboard except to change her payout bank account.

- **Sign-in:** passkey (Face ID or fingerprint on her phone or computer), with an emailed sign-in link as a fallback. Roles: owner (Jakki) and admin (Jared). Every change goes to the audit log.
- **Home, "This week":**
  - Next bake status: open, or closes in N days.
  - Orders and loaves sold versus the limit.
  - Needs-attention cards.
- **Products:** (each product shows whether Stripe is up to date, with a Retry button if a change didn't get through) add and edit loaves and other items: name, description, price, photos (upload from phone, reorder, alt text), option (e.g., Sliced/Unsliced) and its price change, ingredients, allergens, storage tips, loaf weight toward the limit, and whether it sells by bake day or anytime. Hide or show a product.
- **Bakes:** a calendar of bake cards showing the date, when ordering opens and closes, the loaf limit, per-loaf limits, and windows. Actions: change limits, skip this bake, add an extra bake, and set vacation dates.
- **Orders:** list by bake, search by name, email or phone. Order detail with items, window, contact info, notes and payment status. Actions: mark ready, mark picked up or delivered, cancel and refund, partial refund, resend confirmation, add a private note.
- **Bake sheet** (printable): totals per loaf and option for a given bake.
- **Pickups & deliveries** (printable): orders grouped by window, with name, phone, and items. Delivery addresses are listed in route order with a map link.
- **Customers:** list and search, order history, email-marketing consent and SMS opt-in, private notes.
- **Messages:** send "Ordering is open" now or on schedule; edit the wording of order emails and reminders (bake day, window and policy lines are filled in automatically); reminders on or off.
- **Site content:** edit the About, FAQ, Pickup & delivery, Contact and policy pages, an announcement banner, the verse, and the registration number.
- **Settings:**
  - "How you take orders": the cadence mode, with plain descriptions and a preview of the next 4 bakes.
  - Default days, times, and limits.
  - Pickup instructions; delivery ZIP codes, fee and minimum.
  - Store name, contact details, and logo.
  - Stripe catalog check: when the nightly comparison last ran and what it fixed.
- **Reports:** sales by bake, week and month; loaves sold per product; refunds; Stripe payouts (read through the API). Export to CSV for taxes and the cottage-food sales cap.

## Communications
- **Order emails (Resend):** confirmation (with bake day, window, pickup address or delivery details, and the verse), cancelled and refunded, ready for pickup, out for delivery. Stripe's own receipts are turned off so customers get one email per event.
- **Reminders:** a scheduled job sends the day-before reminder at 3 pm Pacific, by email and, if the customer opted in, by SMS through Twilio. Texts include STOP handling.
- **"Ordering is open":** a Resend broadcast at `opensAt` to customers with email-marketing consent (collected in Stripe Checkout and stored on the Customer). Unsubscribes are written back to the Customer. The CAN-SPAM footer uses a PO box rather than her home address.
- **Manual setup:** the Resend domain and the Twilio number and messaging service are set up by hand, following `docs/runbooks/`. Their DNS records live in Pulumi.

## Infrastructure as code (`infra/`, Pulumi Go)
**Managed by Pulumi (AWS unless noted):**
- **Storefront:** ECR repository, Lambda (container image), streaming function URL, CloudFront distribution with an origin secret, GitHub OIDC role for CI (built; see the environments runbook).
- **Bakehouse:** the same pattern, plus EventBridge Scheduler rules for jobs (including the outbox relay and nightly reconciliation), and Stripe and Resend webhook endpoints behind CloudFront.
- **Stripe sync:** SQS FIFO queue and its dead-letter queue (5 tries), the worker's event source mapping (partial batch failures, concurrency 2), and a CloudWatch alarm on the dead-letter queue.
- **Data:** Postgres with automated backups (instance type in a follow-up ADR), and an S3 bucket for product photos behind CloudFront.
- **DNS:** Route 53 hosted zone, ACM certificates, and records for the storefront, `admin`, and Resend (MX, SPF, DKIM, DMARC).
- **Secrets:** Stripe keys and webhook signing secret, Resend and Twilio keys, session and HMAC secrets: Pulumi config secrets delivered as Lambda environment variables.
- **GitHub (GitHub provider):** Actions environments `staging` and `prod`, with their variables and secrets.
- **Stacks:**
  - `staging`: Stripe test mode, smaller database.
  - `prod`: Stripe live mode, the domain, and production services.

**Manual, with runbooks:**
- Buying the domain.
- Stripe account in Jakki's name: business details, payout bank account, branding, Apple Pay domain verification, webhook endpoint secrets into config.
- Resend domain creation and its DKIM value (which goes into config).
- Twilio number and A2P 10DLC registration.
- AWS account and Pulumi credentials.

## Security
- **Storefront to Bakehouse:** server-side only. Requests are HMAC-signed over `method|path|ts|nonce|sha256(body)`, with 5-minute clock skew allowed and nonces stored to block replays. The signing key ID allows rotation.
- **Cart cookie:** signed; prices, capacity and totals are always recomputed on the server.
- **Stripe:** webhook signatures verified with the endpoint secret; events de-duplicated by ID; the paid amount is checked against the order. Card data never touches our servers (Stripe-hosted checkout, PCI SAQ A).
- **Admin:** passkeys or single-use emailed links; secure, HTTP-only session cookies; CSRF protection on every form; audit log of changes.
- **Order links:** signed, unguessable tokens.
- **Data protection:** we store the minimum customer data needed to bake and deliver. Secrets live only in Pulumi config, Lambda environment, and GitHub environments. Postgres backups are on.

## Environments, CI/CD
- **Dev:** GitHub Codespaces or a laptop; Stripe test mode with the Stripe CLI forwarding webhooks.
- **Preview:** the `staging` stack on AWS in Stripe test mode; per-PR previews next. (The Oxygen preview on the Shopify dev store stays only until the storefront drops Hydrogen.)
- **Production:** the `prod` stack.
- **GitHub Actions:**
  - TypeScript checks: lint, typecheck, Vitest, and the dependency-cruiser boundary check.
  - Go checks: `golangci-lint` and `go test` in `infra/`.
  - Container images: built and smoke-tested on every change; pushed to ECR and rolled onto Lambda on merge.
  - On PRs: `pulumi preview`.
  - On merge: deploy to staging and run Playwright end-to-end tests there; production deploys go through the `prod` environment's approval.

## Scaffold status (M0)
Done in the initial scaffold:
- [x] Root workspace: `.mise.toml`, pnpm 10.34.6 workspace (settings in `pnpm-workspace.yaml`), Turborepo, strict `tsconfig.base.json`, `.dependency-cruiser.cjs` (`pnpm boundaries`).
- [x] `apps/storefront` from `shopify hydrogen init`, loading `@bb/brand/tokens.css`; now also a container image. To be rewritten as plain React Router 7 (M3).
- [x] `apps/bakehouse` from Shopify's React Router app template with Drizzle (Postgres) and `/healthz`. The Shopify app pieces are removed in M2.
- [x] `packages/{bake-domain,ledger,contracts,brand}`; `bake-domain` ships the capacity math with property tests.
- [x] `infra/` Pulumi Go project with the storefront on AWS; the Render and Cloudflare parts run only when a stack sets `domain` and are replaced with the AWS work.
- [x] GitHub Actions CI, local Postgres via `docker compose` (host port 55432), Codespaces dev container.

## Milestones
- **M0** Scaffold, infra skeleton, dev container and previews. This spec and the ADRs.
- **M1** `bake-domain` and `ledger`: catalog, cadence generators, cutoffs, DST handling, capacity math, holds, and orders, with property tests.
- **M2** Bakehouse core: storefront API, `PaymentsPort` with the Stripe adapter, the catalog outbox, SQS queue, sync worker and nightly reconciliation, Stripe webhooks, hold sweeper, scheduled jobs, the Issues queue. Remove the Shopify app pieces.
- **M3** Storefront on plain React Router 7: catalog, bake chooser, cart and holds, delivery details, checkout, confirmation and order pages, compliance UI, and accessibility.
- **M4** Admin: sign-in, This week, Products (with photo upload), Bakes, Orders and refunds, Customers, Bake sheet, Pickups & deliveries, Settings.
- **M5** Messages and site content in the admin, order emails, reminders, reports, and the AWS `prod` stack (Postgres, Bakehouse, photos, DNS).
- **M6** Brand tokens from her logo and photos, copy, an accessibility audit, then a friends-and-family soft launch and go-live.

## Pre-launch checklist (non-code)
- [ ] Placer County Environmental Health Class A registration. The registration number goes into the site.
- [ ] City of Lincoln business license.
- [ ] CA Food Handler card.
- [ ] Liability insurance.
- [ ] Verify the 2026 gross sales cap (about $88k) and labeling rules with Placer EH.
- [ ] Confirm with CDTFA whether a seller's permit is needed.
- [ ] Twilio A2P 10DLC (sole proprietor) registration. This takes days to weeks.
- [ ] PO box for the CAN-SPAM footer.
- [ ] Domain purchase.
- [ ] Stripe account, identity verification and payout bank account, all in Jakki's name.

## Open inputs from Jakki (entered in the admin)
- Product list, prices, photos, ingredients, and allergens.
- Starting cadence, bake days, and when ordering opens and closes.
- Loaf limit and per-loaf limits.
- Pickup windows and delivery window.
- Delivery ZIP codes, fee, and minimum.
- Registration number.
- Verse or blessing text.

## Deferred (post-v1 roadmap)
- Shipping within California.
- Subscription "loaf club" (Stripe Billing).
- Gift cards and discount codes.
- In-person sales at farmers markets (Stripe Terminal or Tap to Pay, using the same catalog).
- Customer accounts.
- Wholesale (needs a Class B permit).
- Per-window limits.
- Self-service cancellation.
- Reviews.

## Verification
- **Unit and property tests (`bake-domain`, `ledger`):**
  - Under any random interleaving of add, hold, checkout, pay, expire, cancel, and refund events, allocations never exceed total or per-product limits.
  - A hold in checkout is never released before its session is known to be expired.
  - Cutoff boundaries are correct across the DST changes (2027-03-14, 2027-11-07).
  - Switching cadence never orphans existing orders.
  - Editing a product never changes a past order.
  - Every committed catalog change has an outbox row, and none exists for a change that rolled back.
- **Integration (Bakehouse):** Stripe webhook fixtures cover completed, expired, refunded, and disputed, including duplicates and out-of-order delivery. Concurrent hold requests are tested against a real Postgres (testcontainers). The sync worker is tested against Stripe test mode with duplicate, reordered and stale messages, a mid-sync timeout (no second Price), a Dashboard edit that gets overwritten, and checkout falling back to inline prices while a change is queued.
- **Architecture:** dependency-cruiser keeps the core packages free of Shopify, Stripe, and React Router imports.
- **Infra:** `pulumi preview` is clean on both stacks; mocked unit tests for each component.
- **End to end (Playwright, staging, Stripe test mode):**
  - Pickup and delivery orders paid with Stripe test cards.
  - Multiple carts competing for the last loaf, including one left open on Stripe's page until it expires.
  - Cancel and refund from the admin.
  - axe accessibility scans on every storefront and admin page.
- **Manual:** watch Jakki run a week end to end on staging (add a product with photos, set up a bake, take test orders, print sheets, mark ready, refund one). Then do a soft launch with friends and family, including older customers.
