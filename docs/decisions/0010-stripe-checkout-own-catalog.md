# 0010 — Stripe Checkout; our database is the system of record

**Status:** accepted 2026-11-25 (supersedes 0001; revises 0002, 0004 and 0008). Catalog decision revised by [0011](0011-catalog-sync-to-stripe-queue.md).

## Context
ADR 0009 moved hosting to AWS and treats Shopify as a vendor to use only where it is the best value. Re-evaluated against that:
- A US Shopify store must take cards through Shopify Payments. Stripe can't be chosen as the gateway except after a review by Shopify, other providers add a 2% transaction fee, and there is no open-source Stripe integration. Development stores can't take real payments, so live checkout needs at least the Basic plan ($39 a month, or $29 billed yearly; shopify.com/pricing, read 2026-11-24).
- Card rates are the same either way: 2.9% + 30¢ online on Shopify Basic and on Stripe's standard pricing. Stripe has no monthly fee, and Stripe Terminal covers in-person sales later at 2.7% + 5¢.
- Nothing Shopify-specific has been built yet (M0 scaffold only), so changing now discards no work.
- Jared wants Jakki's admin to be a full-service interface for every part of the store, with our own database as the source of truth.

## Decision
- **Payments:** Stripe Checkout (hosted), in Jakki's Stripe account. Cards, Apple Pay, Google Pay and Link. No delayed payment methods, so a completed session is a paid order.
- **Catalog of record:** products, options, prices, photos, ingredients, allergens and capacity weights live only in our Postgres, edited in the Bakehouse admin. ~~Nothing is kept in Stripe's catalog; each Checkout Session uses inline `price_data`.~~ Revised by 0011: Stripe holds a mirror of the catalog (one Product per SKU, keyed by our SKU), kept in step through an outbox and an SQS queue. Inline `price_data` remains checkout's fallback while a change is still in the queue.
- **Orders of record:** our Postgres. The checkout action writes the order and its lines (prices snapshotted) before redirecting to Stripe, with the order ID on the session. Stripe webhooks only move that order through payment states; they never create it.
- **Admin:** Bakehouse becomes a standalone, full-service admin with its own sign-in, covering products, bakes, orders, refunds, customers, messages, site content, settings and reports. Jakki should never need the Stripe Dashboard except to manage her payout bank account.
- **Storefront:** React Router 7 without Hydrogen, served by the container from 0009. Hydrogen, Oxygen, the Shopify embedded app, the checkout extension and the Shopify notification templates are dropped.
- **Ports:** Stripe sits behind a `PaymentsPort` adapter, like any provider. dependency-cruiser keeps `stripe` out of the core packages, as it does `@shopify/*`.

## Webhooks (verify the payloads in the first spike)
- `checkout.session.completed`: mark the order paid and convert its hold into allocations.
- `checkout.session.expired`: release the hold and mark the order abandoned.
- `charge.refunded` and `refund.updated`: record refunds, including any made in the Stripe Dashboard.
- `charge.dispute.created`: raise a "Needs attention" item.
Events are verified with the signing secret and de-duplicated by event ID.

## Consequences
- Capacity limits become exact. Once a customer is sent to Stripe, the hold is tied to the Checkout Session (`expires_at` set to 30 minutes, Stripe's minimum), and a session can't be paid after it expires. That hold is released only by `checkout.session.expired`, or by a sweeper that asks Stripe for the session's status, never by a timer alone, so a late `completed` webhook can't oversell. The inventory backstop, pickup-time bucket workaround and multi-bake limitation from 0002 go away.
- We own more UI: the admin, the cart, and customer emails (confirmation, cancellation, refund, ready for pickup), sent through Resend.
- Sales tax: bread is exempt in California, so no tax is collected in v1. Stripe Tax can be added per line through `tax_code` if taxable items are sold later.
- Customer accounts are deferred. Customers get an order link by email instead.
- The Oxygen preview retires when the storefront drops Hydrogen. Previews move to AWS: a `staging` stack in Stripe test mode, then per-PR previews.
- Until the storefront is rewritten, the existing Hydrogen code and Oxygen workflow stay in place as scaffolding.
