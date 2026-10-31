import fc from "fast-check";
import { describe, expect, it } from "vitest";

import { addableQuantity, committedUnits } from "./capacity.js";
import type { SlotMenuItem } from "./types.js";

const menu: SlotMenuItem[] = [
  { productId: "country", weight: 1, cap: 12 },
  { productId: "rosemary", weight: 1, cap: 8 },
  { productId: "bagels-6", weight: 0.5 },
];

const commitments = (entries: [string, number][]) => ({
  quantityByProduct: new Map(entries),
});

describe("addableQuantity", () => {
  it("is limited by the per-product cap", () => {
    expect(addableQuantity("rosemary", 24, menu, commitments([["rosemary", 6]]))).toBe(2);
  });

  it("is limited by the total, counting weights", () => {
    const c = commitments([
      ["country", 12],
      ["rosemary", 8],
      ["bagels-6", 6],
    ]);
    // 24 - (12 + 8 + 3) = 1 unit left -> two half-weight bagel packs.
    expect(addableQuantity("bagels-6", 24, menu, c)).toBe(2);
    expect(addableQuantity("country", 24, menu, c)).toBe(0);
  });

  it("returns 0 for products not on the menu", () => {
    expect(addableQuantity("focaccia", 24, menu, commitments([]))).toBe(0);
  });

  it("never lets a hold exceed the total or per-product limit", () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 0, max: 60 }),
        fc.array(fc.tuple(fc.constantFrom("country", "rosemary", "bagels-6"), fc.nat(20)), {
          maxLength: 30,
        }),
        (total, requests) => {
          const taken = new Map<string, number>();
          for (const [productId, wanted] of requests) {
            const allowed = Math.min(
              wanted,
              addableQuantity(productId, total, menu, { quantityByProduct: taken }),
            );
            taken.set(productId, (taken.get(productId) ?? 0) + allowed);
          }
          expect(committedUnits(menu, { quantityByProduct: taken })).toBeLessThanOrEqual(total);
          for (const item of menu) {
            if (item.cap !== undefined) {
              expect(taken.get(item.productId) ?? 0).toBeLessThanOrEqual(item.cap);
            }
          }
        },
      ),
    );
  });
});
