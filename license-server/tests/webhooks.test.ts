// Integration tests for POST /webhooks/stripe against a real Postgres.
// Skipped unless TEST_DATABASE_URL is set (CI without pg stays green):
//   TEST_DATABASE_URL=postgres://test:test@localhost:5432/shuttle_license_test bun test
import { beforeAll, beforeEach, describe, expect, test } from "bun:test";
import { stripeSign } from "./helpers/stripe-sign";

const dbUrl = process.env.TEST_DATABASE_URL;
const SECRET = "whsec_test";

let app: { fetch: (req: Request) => Response | Promise<Response> };
let sql: typeof import("../src/lib/db").sql;

function purchaseEvent(email: string, paymentIntent: string, overrides: Record<string, unknown> = {}) {
  return {
    id: `evt_test_${crypto.randomUUID().replaceAll("-", "")}`,
    object: "event",
    api_version: "2024-06-20",
    created: Math.floor(Date.now() / 1000),
    livemode: false,
    type: "checkout.session.completed",
    data: {
      object: {
        id: `cs_test_${crypto.randomUUID().replaceAll("-", "")}`,
        object: "checkout.session",
        mode: "payment",
        payment_status: "paid",
        payment_intent: paymentIntent,
        customer: "cus_test_1",
        customer_details: { email },
        amount_total: 19900,
        currency: "eur",
        ...overrides,
      },
    },
  };
}

function refundEvent(paymentIntent: string) {
  return {
    id: `evt_test_${crypto.randomUUID().replaceAll("-", "")}`,
    object: "event",
    api_version: "2024-06-20",
    created: Math.floor(Date.now() / 1000),
    livemode: false,
    type: "charge.refunded",
    data: {
      object: { id: "ch_test_1", object: "charge", payment_intent: paymentIntent, refunded: true },
    },
  };
}

async function postSigned(event: unknown): Promise<Response> {
  const body = JSON.stringify(event);
  return app.fetch(
    new Request("http://test.local/webhooks/stripe", {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "stripe-signature": await stripeSign(SECRET, body),
      },
      body,
    }),
  );
}

async function generateKeyPairB64(): Promise<{ pub: string; priv: string }> {
  const pair = (await crypto.subtle.generateKey({ name: "Ed25519" }, true, [
    "sign",
    "verify",
  ])) as unknown as CryptoKeyPair;
  const pubRaw = new Uint8Array(await crypto.subtle.exportKey("raw", pair.publicKey));
  const privPkcs8 = new Uint8Array(await crypto.subtle.exportKey("pkcs8", pair.privateKey));
  const privRaw = privPkcs8.slice(privPkcs8.length - 32);
  const toB64 = (b: Uint8Array) => {
    let s = "";
    for (const byte of b) s += String.fromCharCode(byte);
    return btoa(s);
  };
  return { pub: toB64(pubRaw), priv: toB64(privRaw) };
}

describe.skipIf(!dbUrl)("POST /webhooks/stripe", () => {
  beforeAll(async () => {
    // env.ts reads process.env at import time and throws on missing vars —
    // configure everything before the dynamic imports below.
    const { pub, priv } = await generateKeyPairB64();
    process.env.DATABASE_URL = dbUrl!;
    process.env.LICENSE_PRIVATE_KEY_B64 = priv;
    process.env.LICENSE_PUBLIC_KEY_B64 = pub;
    process.env.STRIPE_SECRET_KEY = "sk_test_dummy";
    process.env.STRIPE_WEBHOOK_SECRET = SECRET;
    delete process.env.MAILPIT_URL;
    delete process.env.RESEND_API_KEY;
    app = (await import("../src/index")).default; // also runs ensureSchema()
    sql = (await import("../src/lib/db")).sql;
  });

  beforeEach(async () => {
    await sql`TRUNCATE webhook_events, activations, licenses`;
  });

  test("rejects missing or invalid signature", async () => {
    const body = JSON.stringify(purchaseEvent("a@b.test", "pi_1"));
    const noSig = await app.fetch(
      new Request("http://test.local/webhooks/stripe", { method: "POST", body }),
    );
    expect(noSig.status).toBe(400);

    const badSig = await app.fetch(
      new Request("http://test.local/webhooks/stripe", {
        method: "POST",
        headers: { "stripe-signature": "t=1,v1=deadbeef" },
        body,
      }),
    );
    expect(badSig.status).toBe(400);
  });

  test("valid purchase creates a perpetual pro license", async () => {
    const res = await postSigned(purchaseEvent("buyer@test.dev", "pi_purchase_1"));
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ received: true });

    const rows = await sql`SELECT * FROM licenses`;
    expect(rows).toHaveLength(1);
    expect(rows[0]!.tier).toBe("pro");
    expect(rows[0]!.status).toBe("active");
    expect(rows[0]!.seats).toBe(1);
    expect(rows[0]!.expires_at).toBeNull();
    expect(rows[0]!.stripe_payment_intent_id).toBe("pi_purchase_1");
    expect(rows[0]!.key).toMatch(/^DS-[0-9A-Z]{6}-[0-9A-Z]{6}-[0-9A-Z]{6}$/);
  });

  test("replayed event id is deduplicated", async () => {
    const event = purchaseEvent("replay@test.dev", "pi_replay_1");
    const first = await postSigned(event);
    expect(await first.json()).toEqual({ received: true });

    const second = await postSigned(event); // same id, fresh signature
    expect(second.status).toBe(200);
    expect(await second.json()).toEqual({ received: true, duplicate: true });

    const rows = await sql`SELECT * FROM licenses`;
    expect(rows).toHaveLength(1);
  });

  test("charge.refunded cancels the license", async () => {
    await postSigned(purchaseEvent("refund@test.dev", "pi_refund_1"));
    const res = await postSigned(refundEvent("pi_refund_1"));
    expect(res.status).toBe(200);

    const rows = await sql`SELECT status FROM licenses`;
    expect(rows[0]!.status).toBe("canceled");
  });

  test("unpaid session is acknowledged but creates no license", async () => {
    const res = await postSigned(
      purchaseEvent("unpaid@test.dev", "pi_unpaid_1", { payment_status: "unpaid" }),
    );
    expect(res.status).toBe(200);

    const rows = await sql`SELECT * FROM licenses`;
    expect(rows).toHaveLength(0);
  });
});
