// Ports implemented by adapters (Shopify, Resend, Twilio, Postgres). The domain never
// imports an implementation, which keeps the bake rules portable off Shopify.

import type { FulfillmentMethod } from "./types.js";

export interface CommerceOrderLine {
  lineItemId: string;
  productId: string;
  quantity: number;
  /** Hidden line-item properties, e.g. `_slot` and `_sig`. */
  properties: Record<string, string>;
}

export interface CommerceOrder {
  orderId: string;
  cancelled: boolean;
  fulfillmentMethod: FulfillmentMethod | "UNKNOWN";
  attributes: Record<string, string>;
  lines: CommerceOrderLine[];
}

export type PickupTimeBucket =
  | "ONE_HOUR"
  | "TWO_HOURS"
  | "FOUR_HOURS"
  | "TWENTY_FOUR_HOURS"
  | "TWO_TO_FOUR_DAYS"
  | "FIVE_OR_MORE_DAYS";

export interface CommercePort {
  /** Set the orderable quantity for a product; `expected` guards against concurrent edits. */
  setInventory(input: {
    productId: string;
    quantity: number;
    expected: number | null;
    idempotencyKey: string;
  }): Promise<void>;
  fetchOrder(orderId: string): Promise<CommerceOrder | null>;
  setPickupTimeBucket(bucket: PickupTimeBucket): Promise<void>;
  listProducts(): Promise<
    { productId: string; title: string; scheduled: boolean; weight: number }[]
  >;
}

export interface Notifier {
  sendEmail(input: { to: string; subject: string; html: string; text: string }): Promise<void>;
  sendSms(input: { to: string; body: string }): Promise<void>;
}

export interface Clock {
  now(): Date;
}
