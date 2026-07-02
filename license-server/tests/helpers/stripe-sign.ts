// Independent implementation of Stripe's webhook signature scheme, used to
// cross-validate the SDK verifier in webhooks.ts (intentionally duplicated
// from stripe-mock/server.ts — two implementations keep each other honest).
// Scheme: t=<unix>,v1=hex(HMAC_SHA256(secret, "<t>.<body>")).
export async function stripeSign(secret: string, payload: string): Promise<string> {
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
