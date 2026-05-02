import { ensureSchema, sql } from "../src/lib/db";

await ensureSchema();
console.log("schema ensured");
await sql.end();
