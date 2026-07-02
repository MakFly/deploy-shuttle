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

  // Stripe. Checkout happens via a Payment Link (one-time payment);
  // the server only consumes webhooks, so no price ID is needed here.
  stripeSecretKey: required("STRIPE_SECRET_KEY"),
  stripeWebhookSecret: required("STRIPE_WEBHOOK_SECRET"),

  // Resend (optional in dev)
  resendApiKey: optional("RESEND_API_KEY"),
  resendFrom: optional("RESEND_FROM", "DeployShuttle <noreply@deployshuttle.io>"),

  // Mailpit (dev only). When set, license emails go to Mailpit's HTTP send
  // API instead of Resend. Must stay unset in production.
  mailpitUrl: optional("MAILPIT_URL"),

  // Token grace (offline window) and refresh lead.
  tokenGraceDays: Number(optional("TOKEN_GRACE_DAYS", "14")),
};
