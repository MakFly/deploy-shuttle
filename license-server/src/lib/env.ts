// Strict env loader. Fail fast on missing required keys; fall back on optional ones.

function required(name: string): string {
  const v = process.env[name];
  if (!v) throw new Error(`Missing required env var: ${name}`);
  return v;
}

function optional(name: string, fallback = ""): string {
  return process.env[name] ?? fallback;
}

export const env = {
  port: Number(optional("PORT", "3000")),
  databaseUrl: required("DATABASE_URL"),

  // Ed25519 keypair (raw 32-byte each, base64-encoded).
  licensePrivateKeyB64: required("LICENSE_PRIVATE_KEY_B64"),
  licensePublicKeyB64: required("LICENSE_PUBLIC_KEY_B64"),

  // Stripe
  stripeSecretKey: required("STRIPE_SECRET_KEY"),
  stripeWebhookSecret: required("STRIPE_WEBHOOK_SECRET"),
  stripeProPriceMonthly: optional("STRIPE_PRO_PRICE_MONTHLY"),
  stripeProPriceYearly: optional("STRIPE_PRO_PRICE_YEARLY"),

  // Resend (optional in dev)
  resendApiKey: optional("RESEND_API_KEY"),
  resendFrom: optional("RESEND_FROM", "DeployShuttle <noreply@deployshuttle.io>"),

  // Token grace (offline window) and refresh lead.
  tokenGraceDays: Number(optional("TOKEN_GRACE_DAYS", "14")),
};
