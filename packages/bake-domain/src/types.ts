// Internal vocabulary only. Customer- and baker-facing copy says "bake", never "slot".

/** IANA zone every bake date and cutoff is interpreted in. */
export const BAKERY_TIME_ZONE = "America/Los_Angeles";

/** Calendar date in the bakery's zone, e.g. "2026-10-18". */
export type IsoDate = string;

export type CadenceMode = "WEEKLY_DROP" | "PICK_A_DATE" | "MADE_TO_ORDER";

/** 0 = Sunday … 6 = Saturday. */
export type Weekday = 0 | 1 | 2 | 3 | 4 | 5 | 6;

export type FulfillmentMethod = "PICKUP" | "DELIVERY";

/** Local wall-clock time "HH:mm" in BAKERY_TIME_ZONE. */
export type LocalTime = string;

export interface WindowTemplate {
  method: FulfillmentMethod;
  start: LocalTime;
  end: LocalTime;
}

export interface MenuTemplateItem {
  productId: string;
  /** Per-product limit for a bake; undefined means only the total limit applies. */
  cap?: number;
}

/** When ordering opens, relative to the bake date. */
export type OpenRule =
  | { kind: "DAYS_BEFORE"; days: number; at: LocalTime }
  | { kind: "ALWAYS" };

/** When ordering closes, relative to the bake date. */
export type CutoffRule =
  | { kind: "DAYS_BEFORE"; days: number; at: LocalTime }
  | { kind: "LEAD_TIME_HOURS"; hours: number };

export interface Schedule {
  mode: CadenceMode;
  bakeWeekdays: Weekday[];
  opens: OpenRule;
  closes: CutoffRule;
  defaultWindows: Partial<Record<Weekday, WindowTemplate[]>>;
  defaultTotalCapacity: number;
  defaultMenu: MenuTemplateItem[];
  /** How many days ahead bakes are generated. */
  horizonDays: number;
}

export type SlotStatus =
  | "SCHEDULED"
  | "OPEN"
  | "CLOSED"
  | "BAKING"
  | "FULFILLED"
  | "CANCELLED";

export interface SlotWindow {
  id: string;
  method: FulfillmentMethod;
  startsAt: Date;
  endsAt: Date;
}

export interface SlotMenuItem {
  productId: string;
  cap?: number;
  /** Capacity weight snapshotted from the product when the bake was created. */
  weight: number;
}

/** One bake event. */
export interface Slot {
  id: string;
  bakeDate: IsoDate;
  opensAt: Date;
  cutoffAt: Date;
  totalCapacity: number;
  status: SlotStatus;
  /** Manual slots and manual edits survive schedule regeneration. */
  source: "GENERATED" | "MANUAL";
  windows: SlotWindow[];
  menu: SlotMenuItem[];
}

export interface Blackout {
  from: IsoDate;
  to: IsoDate;
  reason?: string;
}

export type HoldStatus = "ACTIVE" | "CONVERTED" | "EXPIRED" | "RELEASED";

export interface HoldLine {
  productId: string;
  quantity: number;
}

/** Short-lived capacity reservation for one cart. */
export interface Hold {
  id: string;
  cartId: string;
  slotId: string;
  windowId: string;
  lines: HoldLine[];
  expiresAt: Date;
  status: HoldStatus;
}

/** An order line counted against a bake. */
export interface Allocation {
  orderId: string;
  lineItemId: string;
  slotId: string;
  productId: string;
  quantity: number;
  status: "ACTIVE" | "RELEASED";
}

export type IssueKind =
  | "OVERSOLD"
  | "NO_SLOT"
  | "METHOD_MISMATCH"
  | "LATE_ORDER"
  | "BAD_SIGNATURE";
