import type { SlotMenuItem } from "./types.js";

/** Units already spoken for in a bake, by product. Units = quantity × capacity weight. */
export interface Commitments {
  /** Quantity per product from active allocations plus other carts' active holds. */
  quantityByProduct: ReadonlyMap<string, number>;
}

export function committedUnits(
  menu: readonly SlotMenuItem[],
  commitments: Commitments,
): number {
  let units = 0;
  for (const item of menu) {
    units += (commitments.quantityByProduct.get(item.productId) ?? 0) * item.weight;
  }
  return units;
}

/**
 * Largest quantity of `productId` that can still be added to a bake without exceeding
 * either the bake's total limit or the product's own limit. Returns 0 for products not
 * on the menu.
 */
export function addableQuantity(
  productId: string,
  totalCapacity: number,
  menu: readonly SlotMenuItem[],
  commitments: Commitments,
): number {
  const item = menu.find((m) => m.productId === productId);
  if (!item || item.weight <= 0) return 0;

  const remainingUnits = totalCapacity - committedUnits(menu, commitments);
  const byTotal = Math.floor(remainingUnits / item.weight);

  const committedForProduct = commitments.quantityByProduct.get(productId) ?? 0;
  const byProduct =
    item.cap === undefined ? Number.POSITIVE_INFINITY : item.cap - committedForProduct;

  return Math.max(0, Math.min(byTotal, byProduct));
}
