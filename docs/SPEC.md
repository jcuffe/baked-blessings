# Jakki's Baked Blessings — v1 Storefront Specification

> Status: approved 2026-10-10. Decisions are recorded as ADRs in [`decisions/`](decisions/). Compliance notes: [`compliance.md`](compliance.md).

## Context
Jakki bakes sourdough at home in Lincoln, CA (Placer County) under a California Cottage Food registration. We are building an online storefront so customers can order with a credit card for local pickup or delivery. The repo is empty apart from a blank README. Shopify was chosen to balance risk, cost, and quality.

The hardest problem is matching what customers expect from a shop with the physical limits of baking: oven capacity, bake days, and spoilage. Jakki, in her 60s and non-technical, and her many older customers must never see anything that looks like an implementation detail.

Jared (technical) builds and owns the code in the repo. Jakki runs the business from the Shopify admin.

## Guiding principles
1. **Feels like any mainstream shop.** Customers and Jakki never see slots, variants, holds, $0 lines, or jargon. The site uses plain words: "bake day", "ordering closes", "loaf limit". Admin pages use Polaris, so they look native in Shopify admin.
2. **Accessible by default.**
   - WCAG 2.2 AA, with an 18px base font and tap targets of at least 44px.
   - Contrast around 7:1 for body text.
   - Dates spelled out, e.g. "Saturday, October 18". No visible timers, no hover-only controls.
3. **Own the differentiators, rent the commodities.**
   - Shopify provides auth, checkout, payments, catalog, and orders.
   - The bake domain and capacity ledger are ours, in Shopify-free packages (ports and adapters, enforced by lint).
   - Infrastructure is code (Pulumi Go) across the cheapest providers.
4. **Flexible cadence.** Switching between weekly drop, pick-a-date, and made-to-order is a settings change, not a code change.
5. **Compliance is visible.** Every product page and the footer show "Made in a Home Kitchen", Placer County, and her registration number (AB 831). Every product lists ingredients and allergens.

## Decision log
| Area | Decision |
|---|---|
| Platform | Shopify **Basic** plan, Shopify Payments (cards, Apple/Google Pay, Shop Pay). Jakki owns the store; Jared is collaborator/staff. Partner dev store for staging. |
| Storefront | **Hydrogen** (React Router 7, TypeScript), hosted free on **Oxygen**. Shopify-hosted checkout. |
| Bakehouse | Embedded custom-distribution Shopify app on **Shopify's React Router app template** (Node/TypeScript). Polaris web components UI. Shopify libraries handle auth, sessions, and webhook verification. |
| Domain isolation | Ports and adapters. `bake-domain`, `ledger`, and `contracts` are Shopify-free; the apps are thin adapters. dependency-cruiser fails CI on any `@shopify/*` or `react-router` import in the core packages. |
| Languages | **TypeScript** for the apps and packages. **Go** for IaC (Pulumi). Node is required anyway for Shopify CLI, Hydrogen, and the checkout extension. |
| Hosting | **Render**, Oregon region: Starter web service, with the worker running in-process for v1 (`WORKER_MODE=inline`, can be split into its own service later). **Render Postgres**. About $14.50/mo. Docker images in GitHub's container registry (GHCR) for portability. |
| IaC | **Pulumi (Go)** in `infra/`, with stacks `staging` and `prod`. State in Pulumi Cloud Free, using Pulumi-managed secrets. Providers: Cloudflare and GitHub (official); Render via the Terraform bridge (`pulumi package add terraform-provider render-oss/render`). |
| DNS/email | **Cloudflare** registrar (at cost, bought by hand) and DNS. Cloudflare Email Routing forwards `hello@` to her Gmail for free. |
| Ordering cadence | A unified **Bake** model. The cadence mode is a setting: weekly drop, pick-a-date, or made-to-order. |
| Capacity | A total loaf limit per bake, plus optional per-product limits. Products carry a capacity weight. |
| Enforcement | Our own **capacity ledger** with cart holds. Checkout is only reachable through a Hydrogen gate. Shopify inventory serves as a backstop. No per-date variants and no $0 lines. |
| Fulfillment | Local pickup at her home, plus Shopify local delivery (ZIP list or radius). **No shipping in v1.** Customers pick a window; windows have no limits. |
| Policy | Free cancellation before ordering closes (Jakki cancels, and the loaf is released automatically). Sales are final after the cutoff. Unclaimed orders are not refunded. |
| Catalog | A few loaves with at most one real option (e.g., Sliced/Unsliced). Schema is ready for other bake items and for unscheduled items. |
| Accounts | Guest checkout, plus optional new customer accounts. |
| Comms | Branded Shopify notifications. Day-before reminder by email (**Resend**) and by SMS (**Twilio**, opt-in). "Ordering is open" broadcasts. |
| Channels | Website only. Products are published to Hydrogen only; Online Store uses Shopify's redirect theme. |
| Voice | Warm and homey, lightly faith-touched: a verse on the About page and in the confirmation. Built from her existing logo and photos. |
| Tooling | **mise** pins Node, pnpm, Go, and Pulumi. pnpm workspaces + Turborepo for TypeScript; Go module in `infra/`. Strict TS, Vitest + fast-check, Playwright, golangci-lint, GitHub Actions, Oxygen preview per PR. |
| Domain/launch | Buy a domain. No hard date: launch after the compliance checklist and a friends-and-family soft launch. |

## Architecture
```
Customer ──► Hydrogen storefront (Oxygen) ──HMAC-signed──► Bakehouse (Render: web + inline worker) ──► Render Postgres
                │ Storefront API (cart, products)              │ Admin GraphQL (inventory, orders, pickup settings)
                ▼                                              ▲ Webhooks (orders/*, refunds, checkouts/*, inventory)
          Shopify checkout ─────────── orders ─────────────────┘
Jakki ──► Shopify admin ──► Bakehouse embedded pages (Polaris) + native Products/Orders
Pulumi (Go) ──► Cloudflare DNS/Email Routing · Render services/DB/env · GitHub environments & secrets
```

### Repo layout
```
.mise.toml  package.json  pnpm-workspace.yaml  turbo.json  tsconfig.base.json  .dependency-cruiser.cjs
apps/storefront/            Hydrogen
  app/lib/commerce/         ONLY place touching Storefront API (portability seam)
  app/lib/bakehouse/        typed, HMAC-signing client built from packages/contracts (server-side only)
apps/bakehouse/             Shopify React Router app template (shopify.app.toml, shopify.app.production.toml)
  app/routes/app.*          thin admin routes: authenticate.admin → ledger service → Polaris UI
  app/routes/webhooks.*     authenticate.webhook → ledger service
  app/routes/api.storefront.*  our HMAC check → ledger service
  app/adapters/shopify/     ShopifyCommerceAdapter implements CommercePort (Admin GraphQL)
  worker/                   pg-boss jobs (inline in web process for v1)
  extensions/thank-you-bake-day/   checkout UI extension (Preact + Polaris web components)
  Dockerfile
packages/bake-domain/       pure TS: cadence generators, slots, cutoffs, capacity math, tz; defines ports
packages/ledger/            application services + Postgres repositories (Drizzle); no Shopify/React Router
packages/contracts/         zod schemas + types for the Hydrogen↔Bakehouse API
packages/brand/             tokens.css, fonts, logo
infra/                      Pulumi Go: main.go, Pulumi.{staging,prod}.yaml, sdks/render (generated),
                            internal/{dns,app,db,ci}
docs/                       SPEC.md, decisions/ (ADRs), runbooks/ (local development, Jakki-facing, Shopify setup, Resend/Twilio manual setup, ops), compliance.md
```

**Data access.** Drizzle is used for both the ledger and Shopify session storage, through Shopify's Drizzle session adapter (verify in M0; fall back to the Prisma adapter for sessions only). Migrations use drizzle-kit and run on deploy. Ledger writes use `SELECT … FOR UPDATE` row locks.

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
- **SlotMenuItem:** product, optional per-product cap. The capacity weight is snapshotted from the product metafield.
- **Blackout:** date ranges for vacations and holidays.
- **Hold / HoldLine:** cart reservation (cartId, slot, window, lines, `expiresAt`). Status: ACTIVE, CONVERTED, EXPIRED, or RELEASED.
- **Allocation:** an order line assigned to a slot, counted in units = qty × weight. Status: ACTIVE or RELEASED.
- **Issue:** a "Needs attention" item: OVERSOLD, NO_SLOT, METHOD_MISMATCH, LATE_ORDER, BAD_SIGNATURE.
- **Infrastructure tables:** InventoryWrite log (to recognize our own inventory writes), WebhookEvent (dedupe), Notification log, AuditLog.
- **Ports:**
  - `CommercePort`: setInventory, fetchOrder, setPickupTimeBucket, listProducts.
  - `Notifier`: email and SMS.
  - `Clock`.
  - Repositories.

**Capacity math.**
- Remaining total = cap − Σ active allocations − Σ other active holds.
- A per-product limit is computed the same way.
- Quantity a customer can add = min(⌊remaining total ÷ weight⌋, remaining for that product).

**Cadence switching.** Changing the mode only regenerates future slots that have no orders and aren't open yet. Existing orders are never moved.

**Storefront behavior.**
- With one open bake, the bake day is implied. The header shows "Next bake: Saturday, Oct 18 · order by Wednesday 8 pm".
- With more than one open bake, a friendly date chooser appears before the first add-to-cart.
- With none, it shows "Ordering opens Monday at 8 am" and a notify-me signup.

## Order flow and enforcement
1. **Add to cart.** A Hydrogen action calls `POST /api/storefront/holds` (contract in `packages/contracts`), which reserves capacity atomically.
   - On success, the cart line gets visible properties `Bake day: Saturday, Oct 18` and `Pickup: 9–11 am` (or `Delivery: 1–4 pm`), plus hidden `_slot` and `_sig` (HMAC).
   - The cart gets the attribute `_hold`.
   - On failure, plain words: "Only 2 Country Loaves left for Saturday."
2. **Change bake day or window in the cart.** All lines are re-held in one transaction. If capacity isn't there, the cart suggests the next bake that has room.
3. **Checkout gate.** The checkout button posts to Hydrogen `/checkout`. That action:
   - re-validates and extends the hold;
   - sets `buyerIdentity.preferences.delivery` (PICK_UP plus pickupHandle, or SHIPPING plus `cartDeliveryAddressesAdd`);
   - redirects to `checkoutUrl`. The raw checkout URL is never exposed.
   - Hold TTL is 30 minutes. The hold is extended by `checkouts/create` and `checkouts/update`, matched by the `_hold` attribute or the checkout `token`.
   - An expired hold is quietly re-taken if capacity allows. The customer never sees a timer.
4. **Order placed** (`orders/create`).
   - Verify `_sig`, convert the hold into allocations, then recompute the backstop.
   - If there is no valid hold and capacity is exceeded, raise an OVERSOLD Issue.
   - If there is no `_slot`, raise NO_SLOT.
   - If the delivery method doesn't match the chosen window, raise METHOD_MISMATCH.
5. **Order cancelled, refunded, or edited** (`orders/cancelled`, `refunds/create`, `orders/edited`, re-fetched via GraphQL): release or adjust allocations and recompute.
6. **Inventory backstop.**
   - Each scheduled product's inventory at the single location "Jakki's Kitchen" equals Σ over open slots of min(product remaining, ⌊total remaining ÷ weight⌋). Holds are *not* subtracted, so a customer holding the last loaf can still check out.
   - It drops to 0 when no bake is open, which blocks stale checkouts.
   - Writes use `inventorySetQuantities` with `changeFromQuantity` and `@idempotent`.
   - A webhook value that differs from our last write is treated as Jakki editing the limit: with one open bake, the new count becomes that bake's new remaining limit; otherwise Bakehouse raises an Issue.
7. **Pickup-time text.** Shopify checkout only shows fixed "Usually ready in…" buckets. Bakehouse sets the nearest bucket to the next bake through the Admin API, with `TWO_TO_FOUR_DAYS` as the default. The bake day itself shows on every line, on the Thank-you extension, and in emails.
8. **Known limit (non-Plus).** With several bakes open, checkout can't enforce the limit for one specific bake; only the cart hold and gate do. Remaining oversell risk goes to the Issues queue, which offers plain-language fixes: bake extra, move to the next bake (with a message to the customer), or refund. Roadmap: an unlisted public app with a cart-validation Function.

## Storefront (Hydrogen)
**Pages:**
- Home: current bake status, featured loaves, "How it works" in 3 steps, About teaser.
- Menu: this bake's loaves.
- Product page: photos, description, price, ingredients, allergens, storage tips, home-kitchen disclosure.
- Cart: bake day, Pickup/Delivery choice and window, phone number plus "Text me a reminder" opt-in, notes.
- Checkout is Shopify's own. Thank you and Order status pages get an extension card showing bake day and window.
- Account: optional order history.
- Other pages: About Jakki (with a verse), How ordering works (FAQ), Pickup & delivery (address reads "Lincoln, CA — exact address in your confirmation"), Contact, and the refund, privacy, and terms policies.
- Footer: "Made in a Home Kitchen · Registered Cottage Food Operation, Placer County · Reg. #____".

**Other storefront requirements:**
- Analytics: Shopify analytics only, no third-party trackers.
- SEO and local business schema markup.
- Unscheduled products (`bake.scheduled=false`) skip the bake chooser and are sellable anytime.

## Bakehouse admin (embedded, Polaris web components, plain language)
- **Home, "This week":**
  - Next bake status: open, or closes in N days.
  - Orders and loaves sold versus the limit.
  - Needs-attention cards.
- **Bakes:** a calendar of bake cards showing the date, when ordering opens and closes, the loaf limit, per-loaf limits, and windows. Actions: change limits, skip this bake, add an extra bake, and set vacation dates.
- **Bake sheet** (printable): totals per loaf and option for a given bake.
- **Pickups & deliveries** (printable): orders grouped by window, with name, phone, and items. Delivery addresses are listed in route order with a map link.
- **Settings:**
  - "How you take orders": the cadence mode, with plain descriptions and a preview of the next 4 bakes.
  - Default days, times, and limits.
  - Reminder messages on or off.
- Product fields for capacity weight, scheduled flag, ingredients, and allergens. These are editable here and on the product page as app-owned metafields declared in `shopify.app.toml` and set to `merchant_read_write`.

## Shopify configuration (admin, documented in `docs/runbooks/shopify-setup.md`)
- **Location:** one location, "Jakki's Kitchen". Inventory is tracked for scheduled products, and overselling is off.
- **Pickup and delivery:**
  - Local pickup is enabled, with pickup instructions.
  - Local delivery by ZIP list or radius, with a fee and minimum (TBD with Jakki) and delivery-information text.
  - No shipping rates.
- **Taxes:** bread is exempt food in California, so it is configured as non-taxable. Verify seller's-permit needs with CDTFA.
- **Notification templates:** Order confirmation, Order cancelled, Ready for pickup, and Local delivery templates are edited to print the bake day and window from line-item properties. The standard templates don't reliably show these, so this edit is required.
- **Marketing consent:** email marketing consent is collected at checkout. SMS *marketing* stays off; reminder texts use the separate opt-in in our cart.
- **Policies:** refund, privacy, terms, and contact info filled in.
- **Channels:** products are published only to the Hydrogen channel.

## Communications
- **Reminders:** the Bakehouse worker sends the day-before reminder at 3 pm Pacific, by email through Resend and by SMS through Twilio if the customer opted in. Texts include STOP handling.
- **"Ordering is open":** a Resend broadcast at `opensAt` to customers with email-marketing consent, synced from Shopify. Unsubscribes are written back to Shopify. The CAN-SPAM footer uses a PO box rather than her home address.
- **Manual setup:** the Resend domain and the Twilio number and messaging service are set up by hand (no reliable IaC providers), following `docs/runbooks/`. Their DNS records live in Pulumi.

## Infrastructure as code (`infra/`, Pulumi Go)
**Managed by Pulumi:**
- **Cloudflare zone** (looked up; the domain is bought by hand). Records:
  - Shopify: apex A `23.227.38.65`, AAAA `2620:0127:f00f:5::`, and `www` CNAME `shops.myshopify.com`. All DNS-only (not proxied).
  - `bakehouse` CNAME to Render.
  - Resend on the `send` subdomain: MX and SPF, plus a `resend._domainkey` DKIM record whose value comes from config.
  - `_dmarc` at `p=none` to start.
  - Cloudflare Email Routing MX, SPF, and DKIM records, plus the `hello@` → Gmail rule.
- **Render:** web service (Docker image from GHCR), env group (secrets from Pulumi config secrets), custom domain, and Postgres with backups.
- **GitHub:** Actions environments `staging` and `prod`, with their secrets (Shopify CLI tokens, Hydrogen deploy token, Render deploy hook, Pulumi token).
- **Stacks:**
  - `staging`: dev store, `*.staging` subdomains, and a smaller Postgres.
  - `prod`: apex records and production services.

**Manual, with runbooks:**
- Buying the domain.
- Gmail forwarding verification.
- Shopify: creating the app, custom-distribution link, domain to Hydrogen, Oxygen settings.
- Resend domain creation and its DKIM value (which goes into config).
- Twilio number and A2P 10DLC registration.
- Creating provider API tokens.

## Security
- **Hydrogen to Bakehouse:** server-side only. Requests are HMAC-signed over `method|path|ts|nonce|sha256(body)`, with 5-minute clock skew allowed and nonces stored to block replays. The signing key ID allows rotation.
- **Signed slot data:** `_slot` is signed with `_sig`, because buyers can edit cart attributes with the public token.
- **Shopify auth and webhooks:** handled by Shopify's app library. Webhooks are also deduplicated by event ID.
- **Data protection:** API scopes are least-privilege, and we store minimal customer personal data. Secrets live only in Pulumi config, Render env, and GitHub environments. Render Postgres backups are on.

## Environments, CI/CD
- **Staging:** Partner dev store, Oxygen staging plus per-PR previews, Render staging, Pulumi `staging`.
- **Production:** Jakki's store, Oxygen production, Render prod, Pulumi `prod`.
- **GitHub Actions:**
  - TypeScript checks: lint, typecheck, Vitest, and the dependency-cruiser boundary check.
  - Go checks: `golangci-lint` and `go test` in `infra/`.
  - On PRs: `pulumi preview`.
  - On merge: `pulumi up` for staging and Playwright end-to-end tests against staging.
  - Production deploys are approved manually: `pulumi up` prod, `shopify hydrogen deploy --path apps/storefront`, a Docker build pushed to GHCR, the Render deploy hook, and `shopify app deploy`.
  - Don't use Shopify's auto-generated Hydrogen workflow; it assumes npm and the app at the repo root.

## Scaffold status (M0)
Done in the initial scaffold:
- [x] Root workspace: `.mise.toml`, pnpm 10.34.6 workspace (settings in `pnpm-workspace.yaml`), Turborepo, strict `tsconfig.base.json`, `.dependency-cruiser.cjs` (`pnpm boundaries`).
- [x] `apps/storefront` from `shopify hydrogen init` (TypeScript, CSS Modules, mock.shop), loading `@bb/brand/tokens.css`.
- [x] `apps/bakehouse` from Shopify's React Router app template: Prisma replaced by Drizzle + `@shopify/shopify-app-session-storage-drizzle` (Postgres), custom distribution (`AppDistribution.SingleMerchant`), `/healthz`, monorepo Dockerfile (`pnpm deploy`), demo pages removed.
- [x] `packages/{bake-domain,ledger,contracts,brand}`; `bake-domain` ships the capacity math with property tests.
- [x] `infra/` Pulumi Go project: Cloudflare DNS/Email Routing, Render Postgres + web service (Render provider 1.9.1 via the Terraform bridge, SDK in `infra/sdks/render`), GitHub Actions environments; mocked unit tests.
- [x] GitHub Actions CI (`.github/workflows/ci.yml`), local Postgres via `docker compose` (host port 55432).
- [x] Local dev environment ([`docs/runbooks/local-development.md`](runbooks/local-development.md)): `pnpm bootstrap`, `pnpm dev` (no Shopify accounts needed; storefront :3000, Bakehouse :3001), `pnpm dev:shopify` for the connected app, a committed `.env.example` per app, `pnpm check` mirroring CI.

Remaining, needs Jared (account auth):
- [ ] Shopify Partner account + dev store; install the Hydrogen channel; `shopify hydrogen link` + `env pull`.
- [ ] `shopify app config link` in `apps/bakehouse` (fills `client_id`), then `shopify app generate extension` → `thank-you-bake-day`.
- [ ] `pulumi login`, `pulumi stack init staging`, provider tokens and config (see `infra/README.md`).
- [ ] Day-1 spike (verify on the dev store): custom-app Thank-you extension renders on Basic; `checkouts/*` and `orders/create` payloads carry `_hold` and line properties; line-item properties show in checkout; pickup-time bucket settable via API; Drizzle session adapter end-to-end.

**Secrets:**
- **Storefront:** Hydrogen defaults plus `BAKEHOUSE_API_URL`, `BAKEHOUSE_KEY_ID`, and `BAKEHOUSE_HMAC_SECRET`.
- **Bakehouse:** `SHOPIFY_API_KEY`, `SHOPIFY_API_SECRET`, `SHOPIFY_APP_URL`, `SCOPES`, `DATABASE_URL`, `STOREFRONT_HMAC_SECRETS`, `SLOT_SIGNING_SECRET`, `RESEND_API_KEY`, the `TWILIO_*` variables, `SHOP_DOMAIN`, and `WORKER_MODE`.

## Milestones
- **M0** Scaffold, infra skeleton (staging), and the day-1 spike. Write `docs/SPEC.md` (this document) plus ADRs.
- **M1** `bake-domain` and `ledger`: cadence generators, cutoffs, DST handling, capacity math, and holds, with property tests.
- **M2** Bakehouse adapters: storefront API, webhooks, worker jobs, inventory backstop, and the Issues queue.
- **M3** Storefront: catalog, bake chooser, cart holds, checkout gate, compliance UI, and accessibility.
- **M4** Bakehouse admin UI: This week, Bakes, Bake sheet, Pickups & deliveries, and Settings.
- **M5** Comms, notification templates, the Thank-you extension, the Shopify configuration runbook, and the infra `prod` stack.
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
- [ ] Domain purchase on Cloudflare.
- [ ] Shopify Basic plan, Shopify Payments, and payout bank account, all in Jakki's name.

## Open inputs from Jakki (placeholders in config)
- Product list, prices, photos, ingredients, and allergens.
- Starting cadence, bake days, and when ordering opens and closes.
- Loaf limit and per-loaf limits.
- Pickup windows and delivery window.
- Delivery ZIP codes, fee, and minimum.
- Registration number.
- Verse or blessing text.

## Deferred (post-v1 roadmap)
- Shipping within California.
- Subscription "loaf club".
- Gift cards.
- POS and farmers markets.
- Wholesale (needs a Class B permit).
- Per-window limits.
- Self-service cancellation.
- Unlisted public app with a cart-validation Function, for checkout-enforced per-bake limits.
- Splitting the worker into its own Render service.
- Reviews.
- Migration off Shopify: replace `apps/*` adapters and checkout. `bake-domain`, `ledger`, `contracts`, the database, and brand stay.

## Verification
- **Unit and property tests (`bake-domain`, `ledger`):**
  - Under any random interleaving of add, hold, expire, order, and cancel events, allocations never exceed total or per-product limits.
  - Cutoff boundaries are correct across the DST changes (2026-11-01, 2027-03-14).
  - Switching cadence never orphans existing orders.
- **Integration (Bakehouse):** webhook fixtures cover orders/create, cancel, refund, and edit, plus inventory echo suppression and the Issue paths. Concurrent hold requests are tested against a real Postgres (testcontainers).
- **Architecture:** dependency-cruiser keeps the core packages free of Shopify and React Router imports.
- **Infra:** `pulumi preview` is clean on both stacks. After `pulumi up` staging, DNS resolves, Resend and Email Routing verify, and Render is healthy.
- **End to end (Playwright, staging store):**
  - Pickup and delivery checkouts in Shopify's test payment mode.
  - Multiple carts competing for the last loaf.
  - Expired holds, stale checkout links after cutoff, and checkout-method mismatch.
  - axe accessibility scans on every page.
- **Manual:** watch Jakki run a week end to end on staging (set up a bake, take test orders, print sheets, mark ready for pickup). Then do a soft launch with friends and family, including older customers.
