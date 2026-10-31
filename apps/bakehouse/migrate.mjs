// Applies pending Drizzle migrations at container start (drizzle-kit is a dev dependency,
// so production uses the runtime migrator).
import { drizzle } from "drizzle-orm/node-postgres";
import { migrate } from "drizzle-orm/node-postgres/migrator";
import pg from "pg";

const pool = new pg.Pool({ connectionString: process.env.DATABASE_URL });
await migrate(drizzle(pool), { migrationsFolder: "./drizzle" });
await pool.end();
