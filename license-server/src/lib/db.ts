import postgres from "postgres";
import { env } from "./env";

export const sql = postgres(env.databaseUrl, {
  ssl: env.databaseUrl.includes("sslmode=require") ? "require" : undefined,
  max: 10,
  idle_timeout: 20,
  prepare: false,
});

// Bootstrap schema. Idempotent; safe to run on every cold boot. Real
// migrations live under migrations/.
export async function ensureSchema(): Promise<void> {
  await sql`
    CREATE TABLE IF NOT EXISTS licenses (
      key TEXT PRIMARY KEY,
      stripe_customer_id TEXT NOT NULL,
      stripe_subscription_id TEXT NOT NULL,
      tier TEXT NOT NULL DEFAULT 'pro',
      status TEXT NOT NULL,
      seats INT NOT NULL DEFAULT 1,
      expires_at TIMESTAMPTZ,
      created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    );
  `;
  await sql`
    CREATE TABLE IF NOT EXISTS activations (
      id BIGSERIAL PRIMARY KEY,
      license_key TEXT NOT NULL REFERENCES licenses(key) ON DELETE CASCADE,
      machine_fingerprint TEXT NOT NULL,
      cli_version TEXT,
      last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
      created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
      UNIQUE (license_key, machine_fingerprint)
    );
  `;
  await sql`
    CREATE TABLE IF NOT EXISTS webhook_events (
      stripe_event_id TEXT PRIMARY KEY,
      type TEXT NOT NULL,
      payload JSONB NOT NULL,
      processed_at TIMESTAMPTZ
    );
  `;
}

export type LicenseRow = {
  key: string;
  stripe_customer_id: string;
  stripe_subscription_id: string;
  tier: string;
  status: string;
  seats: number;
  expires_at: Date | null;
  created_at: Date;
};

export async function findLicense(key: string): Promise<LicenseRow | null> {
  const rows = await sql<LicenseRow[]>`SELECT * FROM licenses WHERE key = ${key} LIMIT 1`;
  return rows[0] ?? null;
}

export async function recordActivation(
  license: LicenseRow,
  fp: string,
  cliVersion: string | null,
): Promise<"recorded" | "seat_limit"> {
  return await sql.begin(async (tx) => {
    const locked = await tx<{ seats: number }[]>`
      SELECT seats FROM licenses WHERE key = ${license.key} FOR UPDATE
    `;
    const seats = locked[0]?.seats ?? license.seats;
    const existing = await tx<{ id: number }[]>`
      SELECT id FROM activations
      WHERE license_key = ${license.key} AND machine_fingerprint = ${fp}
      LIMIT 1
    `;
    if (existing.length > 0) {
      await tx`
        UPDATE activations
        SET last_seen_at = now(), cli_version = ${cliVersion}
        WHERE license_key = ${license.key} AND machine_fingerprint = ${fp}
      `;
      return "recorded";
    }
    const counts = await tx<{ count: string }[]>`
      SELECT count(*)::text AS count FROM activations WHERE license_key = ${license.key}
    `;
    const count = Number(counts[0]?.count ?? "0");
    if (count >= seats) return "seat_limit";
    await tx`
      INSERT INTO activations (license_key, machine_fingerprint, cli_version)
      VALUES (${license.key}, ${fp}, ${cliVersion})
    `;
    return "recorded";
  });
}

export async function touchActivation(
  licenseKey: string,
  fp: string,
): Promise<boolean> {
  const rows = await sql<{ id: number }[]>`
    UPDATE activations
    SET last_seen_at = now()
    WHERE license_key = ${licenseKey} AND machine_fingerprint = ${fp}
    RETURNING id
  `;
  return rows.length > 0;
}

export async function deactivateActivation(
  licenseKey: string,
  fp: string,
): Promise<boolean> {
  const rows = await sql<{ id: number }[]>`
    DELETE FROM activations
    WHERE license_key = ${licenseKey} AND machine_fingerprint = ${fp}
    RETURNING id
  `;
  return rows.length > 0;
}

export async function upsertLicense(row: {
  key: string;
  stripeCustomerId: string;
  stripeSubscriptionId: string;
  tier: "pro";
  status: "active" | "past_due" | "canceled";
  seats: number;
  expiresAt: Date | null;
}): Promise<void> {
  await sql`
    INSERT INTO licenses (key, stripe_customer_id, stripe_subscription_id, tier, status, seats, expires_at)
    VALUES (${row.key}, ${row.stripeCustomerId}, ${row.stripeSubscriptionId}, ${row.tier}, ${row.status}, ${row.seats}, ${row.expiresAt})
    ON CONFLICT (key) DO UPDATE SET
      stripe_customer_id = EXCLUDED.stripe_customer_id,
      stripe_subscription_id = EXCLUDED.stripe_subscription_id,
      tier = EXCLUDED.tier,
      status = EXCLUDED.status,
      seats = EXCLUDED.seats,
      expires_at = EXCLUDED.expires_at
  `;
}

export async function setLicenseStatusBySubscription(
  subscriptionId: string,
  status: "active" | "past_due" | "canceled",
  expiresAt: Date | null,
): Promise<void> {
  await sql`
    UPDATE licenses SET status = ${status}, expires_at = ${expiresAt}
    WHERE stripe_subscription_id = ${subscriptionId}
  `;
}

export async function recordWebhookEvent(
  id: string,
  type: string,
  payload: unknown,
): Promise<boolean> {
  const rows = await sql<{ id: string }[]>`
    INSERT INTO webhook_events (stripe_event_id, type, payload)
    VALUES (${id}, ${type}, ${sql.json(payload as never)})
    ON CONFLICT (stripe_event_id) DO NOTHING
    RETURNING stripe_event_id AS id
  `;
  return rows.length > 0;
}

export async function markWebhookProcessed(id: string): Promise<void> {
  await sql`UPDATE webhook_events SET processed_at = now() WHERE stripe_event_id = ${id}`;
}
