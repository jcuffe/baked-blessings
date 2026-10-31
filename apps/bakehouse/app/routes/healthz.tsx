import { sql } from "drizzle-orm";
import db from "../db.server";

// Render health check: the process is up and Postgres answers.
export const loader = async () => {
  await db.execute(sql`select 1`);
  return new Response("ok", { headers: { "Cache-Control": "no-store" } });
};
