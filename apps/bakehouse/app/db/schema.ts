// Bakehouse-owned tables. Shopify session storage lives here (adapter concern);
// capacity-ledger tables are exported from @bb/ledger and share this database.
import {
  bigint,
  boolean,
  pgTable,
  text,
  timestamp,
} from "drizzle-orm/pg-core";

// Column set required by @shopify/shopify-app-session-storage-drizzle (incl. refresh
// tokens for expiring offline access tokens).
export const sessionTable = pgTable("session", {
  id: text("id").primaryKey(),
  shop: text("shop").notNull(),
  state: text("state").notNull(),
  isOnline: boolean("isOnline").default(false).notNull(),
  scope: text("scope"),
  expires: timestamp("expires", { mode: "date" }),
  accessToken: text("accessToken").notNull(),
  userId: bigint("userId", { mode: "number" }),
  firstName: text("firstName"),
  lastName: text("lastName"),
  email: text("email"),
  accountOwner: boolean("accountOwner"),
  locale: text("locale"),
  collaborator: boolean("collaborator"),
  emailVerified: boolean("emailVerified"),
  refreshToken: text("refreshToken"),
  refreshTokenExpires: timestamp("refreshTokenExpires", { mode: "date" }),
});
