// Dev-only fake Stripe: a minimal Payment Link page that emits real,
// HMAC-signed webhook events to the license-server. Never deployed.
//
// Run: bun run stripe-mock/server.ts
// Env: PORT (4242), STRIPE_WEBHOOK_SECRET (must match license-server),
//      WEBHOOK_URL (http://localhost:3000/webhooks/stripe),
//      SUCCESS_URL (http://localhost:4321/thank-you)

const PORT = Number(process.env.PORT ?? "4242");
const SECRET = process.env.STRIPE_WEBHOOK_SECRET ?? "whsec_dev_mock";
const WEBHOOK_URL = process.env.WEBHOOK_URL ?? "http://localhost:3000/webhooks/stripe";
const SUCCESS_URL = process.env.SUCCESS_URL ?? "http://localhost:4321/thank-you";

type Purchase = { email: string; payment_intent: string; at: string };
const purchases: Purchase[] = [];

function rand(): string {
  return crypto.randomUUID().replaceAll("-", "").slice(0, 24);
}

// Stripe webhook signature scheme: t=<unix>,v1=hex(HMAC_SHA256(secret, "<t>.<body>")).
// Sign at send time — constructEventAsync enforces a 300s tolerance on t.
async function stripeSign(secret: string, payload: string): Promise<string> {
  const t = Math.floor(Date.now() / 1000);
  const key = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const mac = await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(`${t}.${payload}`));
  const v1 = [...new Uint8Array(mac)].map((b) => b.toString(16).padStart(2, "0")).join("");
  return `t=${t},v1=${v1}`;
}

async function sendEvent(event: Record<string, unknown>): Promise<Response> {
  const body = JSON.stringify(event); // stringify once: the signed string IS the body
  return fetch(WEBHOOK_URL, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      "stripe-signature": await stripeSign(SECRET, body),
    },
    body,
  });
}

function checkoutCompletedEvent(email: string, paymentIntent: string): Record<string, unknown> {
  return {
    id: `evt_mock_${rand()}`,
    object: "event",
    api_version: "2024-06-20",
    created: Math.floor(Date.now() / 1000),
    livemode: false,
    type: "checkout.session.completed",
    data: {
      object: {
        id: `cs_test_mock_${rand()}`,
        object: "checkout.session",
        mode: "payment",
        payment_status: "paid",
        payment_intent: paymentIntent,
        customer: `cus_mock_${rand()}`,
        customer_details: { email },
        amount_total: 19900,
        currency: "eur",
      },
    },
  };
}

function chargeRefundedEvent(paymentIntent: string): Record<string, unknown> {
  return {
    id: `evt_mock_${rand()}`,
    object: "event",
    api_version: "2024-06-20",
    created: Math.floor(Date.now() / 1000),
    livemode: false,
    type: "charge.refunded",
    data: {
      object: {
        id: `ch_mock_${rand()}`,
        object: "charge",
        payment_intent: paymentIntent,
        refunded: true,
      },
    },
  };
}

function payPage(): string {
  const rows = purchases
    .map(
      (p) => `<tr><td>${p.email}</td><td><code>${p.payment_intent}</code></td><td>${p.at}</td>
        <td><form method="post" action="/refund" style="margin:0">
          <input type="hidden" name="payment_intent" value="${p.payment_intent}">
          <button>Refund</button></form></td></tr>`,
    )
    .join("");
  return `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>stripe-mock — DeployShuttle Pro</title>
<style>
  body{font-family:system-ui,sans-serif;max-width:32rem;margin:3rem auto;padding:0 1rem;color:#222}
  form.pay{display:flex;flex-direction:column;gap:.75rem;padding:1.5rem;border:1px solid #ddd;border-radius:10px}
  input[type=email]{padding:.6rem;border:1px solid #ccc;border-radius:6px;font-size:1rem}
  button{padding:.6rem 1rem;border:0;border-radius:6px;background:#635bff;color:#fff;font-size:1rem;cursor:pointer}
  table{width:100%;margin-top:2rem;border-collapse:collapse;font-size:.85rem}
  td,th{padding:.4rem;border-bottom:1px solid #eee;text-align:left}
</style></head><body>
<h1>stripe-mock</h1>
<p>Fake Stripe Payment Link — DeployShuttle Pro, <strong>€199.00</strong> one-time.</p>
<form class="pay" method="post" action="/pay">
  <input type="email" name="email" placeholder="you@example.com" required>
  <button type="submit">Pay €199.00</button>
</form>
${purchases.length ? `<table><tr><th>Email</th><th>payment_intent</th><th>At</th><th></th></tr>${rows}</table>` : ""}
</body></html>`;
}

const server = Bun.serve({
  port: PORT,
  async fetch(req) {
    const url = new URL(req.url);

    if (req.method === "GET" && url.pathname === "/pay") {
      return new Response(payPage(), { headers: { "content-type": "text/html; charset=utf-8" } });
    }

    if (req.method === "POST" && url.pathname === "/pay") {
      const form = await req.formData();
      const email = String(form.get("email") ?? "").trim();
      if (!email) return new Response("missing email", { status: 400 });
      const paymentIntent = `pi_mock_${rand()}`;
      const res = await sendEvent(checkoutCompletedEvent(email, paymentIntent));
      if (!res.ok) {
        const detail = await res.text().catch(() => "");
        return new Response(`webhook rejected (${res.status}): ${detail}`, { status: 502 });
      }
      purchases.push({ email, payment_intent: paymentIntent, at: new Date().toISOString() });
      return new Response(null, { status: 303, headers: { location: SUCCESS_URL } });
    }

    if (req.method === "POST" && url.pathname === "/refund") {
      let paymentIntent = url.searchParams.get("payment_intent") ?? "";
      if (!paymentIntent) {
        const form = await req.formData().catch(() => null);
        paymentIntent = String(form?.get("payment_intent") ?? "").trim();
      }
      if (!paymentIntent) return new Response("missing payment_intent", { status: 400 });
      const res = await sendEvent(chargeRefundedEvent(paymentIntent));
      const body = JSON.stringify({ refunded: paymentIntent, webhookStatus: res.status });
      if (!res.ok) return new Response(body, { status: 502, headers: { "content-type": "application/json" } });
      return new Response(body, { headers: { "content-type": "application/json" } });
    }

    if (req.method === "GET" && url.pathname === "/purchases") {
      return Response.json(purchases);
    }

    return new Response("stripe-mock: GET /pay, POST /pay, POST /refund, GET /purchases", { status: 404 });
  },
});

console.log(`stripe-mock listening on http://localhost:${server.port}/pay → webhook ${WEBHOOK_URL}`);
