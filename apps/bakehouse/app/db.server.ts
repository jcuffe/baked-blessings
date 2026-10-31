import { drizzle, type NodePgDatabase } from "drizzle-orm/node-postgres";
import pg from "pg";

import * as schema from "./db/schema";

declare global {
  // eslint-disable-next-line no-var
  var dbGlobal: NodePgDatabase<typeof schema> | undefined;
}

function createDb() {
  const pool = new pg.Pool({ connectionString: process.env.DATABASE_URL });
  return drizzle(pool, { schema });
}

// Reuse one pool across dev-server reloads.
const db = global.dbGlobal ?? createDb();
if (process.env.NODE_ENV !== "production") global.dbGlobal = db;

export default db;
