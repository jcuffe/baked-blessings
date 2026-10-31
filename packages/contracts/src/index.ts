// Hydrogen <-> Bakehouse API contracts. Both sides validate with these schemas, so the
// storefront and the ledger can never silently disagree about a payload's shape.

import { z } from "zod";

export const FulfillmentMethod = z.enum(["PICKUP", "DELIVERY"]);

export const BakeWindow = z.object({
  id: z.string(),
  method: FulfillmentMethod,
  startsAt: z.iso.datetime({ offset: true }),
  endsAt: z.iso.datetime({ offset: true }),
});

export const BakeMenuItem = z.object({
  productId: z.string(),
  /** How many more the customer could add right now (0 = sold out for this bake). */
  available: z.number().int().nonnegative(),
});

/** An open bake as the storefront sees it. */
export const OpenBake = z.object({
  id: z.string(),
  bakeDate: z.iso.date(),
  cutoffAt: z.iso.datetime({ offset: true }),
  windows: z.array(BakeWindow),
  menu: z.array(BakeMenuItem),
});

/** GET /api/storefront/bakes */
export const OpenBakesResponse = z.object({
  bakes: z.array(OpenBake),
  /** Shown when nothing is open: "Ordering opens Monday at 8 am". */
  nextOpensAt: z.iso.datetime({ offset: true }).nullable(),
});

/** POST /api/storefront/holds — replaces the cart's hold with exactly these lines. */
export const PlaceHoldRequest = z.object({
  cartId: z.string(),
  bakeId: z.string(),
  windowId: z.string(),
  lines: z.array(
    z.object({
      productId: z.string(),
      quantity: z.number().int().positive(),
    }),
  ),
});

export const PlaceHoldResponse = z.discriminatedUnion("ok", [
  z.object({
    ok: z.literal(true),
    holdId: z.string(),
    /** Signed `_slot` / `_sig` values for each cart line's hidden properties. */
    slotToken: z.string(),
    signature: z.string(),
  }),
  z.object({
    ok: z.literal(false),
    shortages: z.array(
      z.object({
        productId: z.string(),
        requested: z.number().int(),
        available: z.number().int().nonnegative(),
      }),
    ),
  }),
]);

export type OpenBake = z.infer<typeof OpenBake>;
export type OpenBakesResponse = z.infer<typeof OpenBakesResponse>;
export type PlaceHoldRequest = z.infer<typeof PlaceHoldRequest>;
export type PlaceHoldResponse = z.infer<typeof PlaceHoldResponse>;
