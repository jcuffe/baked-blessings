# 0012 — Medusa as the commerce platform

**Status:** proposed 2026-11-26. Supersedes the Shopify pieces still in place and, once the open questions below are settled, revises 0010 and 0011.

## Context
Jared prefers an open-source commerce platform the store can run anywhere over building on Shopify. Medusa v2 is the leading open-source option: a Node/TypeScript commerce server with catalog, cart, checkout, orders, customers, payments and fulfillment as modules, plus an admin dashboard.

Facts checked 2026-11-26:
- **License.** The repository is MIT except for its Enterprise Edition: "RBAC (role-based access control), SSO (single sign-on), and audit logs are Medusa Enterprise Edition features" that need a commercial agreement (`ENTERPRISE-LICENSE.md` in medusajs/medusa). Latest release on npm is 2.23.0, needing Node 22.22 or later.
- **What it runs.** "At minimum, you'll need server and worker instances, a PostgreSQL database, and a Redis instance" (Medusa docs, Cloud vs self-hosting). The server serves the API and the admin, the worker runs scheduled jobs and subscribers, and both are the same app with `MEDUSA_WORKER_MODE` set to `server` or `worker` (or `shared` for one process). Redis backs the event bus, workflow engine, cache and locking. The docs ask for at least 2 GB of RAM and give no serverless guidance. Health check at `/health`, admin at `/app`, migrations run with `medusa db:migrate` before the server starts.
- **Stripe.** Medusa ships a Stripe payment provider (`@medusajs/medusa/payment-stripe`). It works through Stripe payment intents and webhooks at `/hooks/payment/stripe_stripe`, and the storefront collects payment with Stripe's Payment Element on our own checkout page. Apple Pay and Google Pay come through Stripe's automatic payment methods. Capture is manual by default.
- **Admin.** Extended with widgets in existing pages and new pages ("UI routes", React, with `@medusajs/ui`). The pages read don't describe hiding or renaming built-in pages.

## Decision (the parts with clean answers)
- **Commerce backend:** Medusa v2 replaces the commerce half of Bakehouse: catalog, cart, checkout, orders, customers, payments, refunds and regions. Its Postgres is our database, so "our database is the source of truth" from 0010 still holds; Medusa just owns most of the schema.
- **Bake days and capacity:** a custom Medusa module (bakes, loaf limits, holds, allocations) built on `packages/bake-domain`, which stays provider-free. Workflow hooks check and take holds when items are added to the cart and turn them into allocations when the order is placed. Medusa's inventory module counts stock, not bake-day capacity, so the capacity ledger from 0002 stays ours.
- **Containers:** CI builds one Medusa image (`medusa build`, then `.medusa/server`) and runs it twice: a server deployment and a worker deployment. That fits "pod-ready" from 0009 directly and moves to EKS unchanged.
- **Not on Lambda:** the Medusa server and worker are long-running and need Redis, so they run on Fargate (or a small instance; see question 3). The storefront keeps the Lambda and CloudFront hosting already in `infra/internal/storefront`.
- **Storefront:** start from Medusa's Next.js starter (cart, checkout and Stripe already wired), served by the same container pattern, unless question 2 changes the checkout. The current Hydrogen storefront and Oxygen previews stay until the new one replaces them.
- **AWS and Pulumi:** unchanged. Pulumi adds Postgres (RDS), Redis (ElastiCache or Valkey), the Medusa server and worker, S3 for product photos (Medusa's S3 file provider), and their secrets.
- **Shopify:** dropped entirely, including the Partner dev store, once the new storefront is up.

## Open questions (for Jared)
1. **Admin.** Medusa's admin is complete but generic: it speaks in regions, sales channels, stock locations and fulfillment sets, and roles and audit logs are Enterprise-only, so every admin user has full access. Options: (a) Medusa's admin plus our own pages for bakes and "This week" (least work); (b) our own plain-language admin for Jakki on Medusa's Admin API (matches the spec's principle 1, most work); (c) (a) now, then a small plain-language app for her daily screens later.
2. **Checkout.** Medusa's Stripe provider puts Stripe's card form on our checkout page instead of Stripe's hosted page (0010). Card details still stay inside Stripe's frames. Options: (a) use it as is, and release a held loaf when the cart goes idle after cancelling its payment intent so it can't be paid later; (b) write a Medusa payment provider for Stripe's hosted Checkout, keeping 0010's "a hold ends only when the session expires" guarantee, at the cost of a custom provider to maintain.
3. **Hosting cost.** Shopify Basic was $29 to $39 a month. Monthly on-demand prices in us-east-1 from AWS's public price list (pricing.us-east-1.amazonaws.com, effective 2026-11-01; 730 hours), with the storefront's Lambda and CloudFront still near $0:

   | Piece | (a) managed, split | (b) managed, one task | (c) one instance |
   |---|---|---|---|
   | Medusa compute | 2 Fargate Graviton tasks, 0.5 vCPU / 2 GB: $34.04 | 1 task in `shared` mode: $17.02 | t4g.medium (2 vCPU, 4 GB) running Medusa, Postgres and Redis in containers: $24.53 |
   | Postgres | RDS db.t4g.micro + 20 GB gp3: $13.98 | same: $13.98 | in the instance; 30 GB gp3 disk: $2.40 |
   | Redis | ElastiCache Valkey cache.t4g.micro: $9.34 | same: $9.34 | in the instance |
   | Load balancer | ALB: $16.43 + usage | same: $16.43 + usage | none; CloudFront reaches the instance directly |
   | Public IPv4 | tasks and ALB: about $7 to $15 | about $4 to $11 | one address: $3.65 |
   | **Total** | **about $81 to $89** | **about $61 to $68** | **about $31** |

   The load balancer and the two managed data services are most of (a) and (b). (c) stays containerized for EKS later, but its database backups, patching and restarts are ours (snapshots to S3 cost pennies). Options: (a), (b) or (c).
4. **Stripe catalog mirror (0011).** Medusa's Stripe provider only sends payment amounts; it doesn't put products in Stripe. Options: (a) drop the mirror for v1 and keep 0011's design for when Stripe Terminal or a loaf club needs it; (b) build it now as a Medusa subscriber and workflow (Medusa's workflows retry durably through Redis), or (c) on the SQS queue as 0011 describes.

## Consequences
- Much less of our own code: cart, checkout, orders, refunds, customers and the Stripe integration come from Medusa. What stays ours is the bake module, the storefront's look, order emails and whatever question 1 decides for the admin.
- We follow Medusa's upgrade cadence and its data model.
- Fixed monthly cost goes up from the near-zero Lambda-only plan; question 3 decides by how much.
