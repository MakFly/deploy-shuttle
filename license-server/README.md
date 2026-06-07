# DeployShuttle license server

Stripe-backed license issuer for the DeployShuttle CLI Pro tier.

```
Stripe Checkout ── webhook ──▶ license-server ── Postgres
                                       │
                                       ├─▶ /activate  (CLI → JWT 14 d)
                                       ├─▶ /deactivate (free machine slot)
                                       ├─▶ /refresh   (CLI → new JWT)
                                       └─▶ /pubkey    (verifying key)
```

## Stack

- **Bun** + **Hono** for the HTTP layer.
- **Postgres** (Neon free tier works) via `postgres.js`.
- **Stripe** (subscriptions, signed webhooks).
- **Resend** for transactional email (optional in dev).
- **Ed25519** signing — verifying key embedded in the CLI binary.

## Setup

```bash
cd license-server
bun install
cp .env.example .env  # fill in DATABASE_URL, Stripe keys, Resend, etc.

# 1. Mint your Ed25519 keypair (run once, store somewhere safe):
bun run keygen
# Copy LICENSE_PRIVATE_KEY_B64 to .env (server side, secret).
# Copy LICENSE_PUBLIC_KEY_B64 to .env AND to the CLI build (LICENSE_PUBKEY_B64).

# 2. Bootstrap the schema (idempotent):
bun run migrate

# 3. Run locally:
bun run dev
```

## Stripe wiring

1. Create a product `DeployShuttle Pro` in Stripe Dashboard.
2. Add two prices: monthly and yearly. Copy the IDs into `.env`.
3. Create a webhook endpoint pointing at `https://<host>/webhooks/stripe`.
   Subscribe to `checkout.session.completed`,
   `customer.subscription.updated`, `customer.subscription.deleted`.
   Copy the signing secret into `STRIPE_WEBHOOK_SECRET`.
4. Wire the Checkout button on the landing page to redirect to a Stripe
   Checkout Session that uses the monthly or yearly price ID.
5. On `checkout.session.completed`, the server generates a license key,
   stores it, and emails it to the customer (Resend).

## Deploy on Fly.io

```bash
fly launch --name shuttle-license --copy-config --no-deploy
fly secrets set \
  DATABASE_URL="..." \
  LICENSE_PRIVATE_KEY_B64="..." \
  LICENSE_PUBLIC_KEY_B64="..." \
  STRIPE_SECRET_KEY="..." \
  STRIPE_WEBHOOK_SECRET="..." \
  STRIPE_PRO_PRICE_MONTHLY="..." \
  STRIPE_PRO_PRICE_YEARLY="..." \
  RESEND_API_KEY="..."
fly deploy
```

After the first deploy, point the CLI at the new host:

```bash
SHUTTLE_LICENSE_SERVER=https://shuttle-license.fly.dev \
  shuttle license activate <key>
```

Or bake the URL into the CLI via the `LICENSE_SERVER` build env.

## API

### POST `/activate`

Body:

```json
{ "key": "DS-XXXXXX-XXXXXX-XXXXXX", "machineFingerprint": "<sha256>", "cliVersion": "v0.2.0" }
```

Returns:

```json
{ "token": "<jwt>", "tier": "pro", "expiresAt": "2026-05-16T13:00:00Z" }
```

### POST `/refresh`

Body:

```json
{ "token": "<existing jwt>" }
```

Returns the same shape as `/activate`.

### POST `/deactivate`

Body:

```json
{ "token": "<existing jwt>" }
```

Deletes the activation for the token's license key and machine
fingerprint. Returns `{ "deactivated": true }` when a slot was freed.

### POST `/webhooks/stripe`

Stripe-signed webhook. Idempotent via the `webhook_events` table.

### GET `/pubkey`

```json
{ "algorithm": "Ed25519", "publicKeyB64": "<base64>" }
```

Cached for 1 hour. Public; no auth.

### GET `/healthz`

Liveness probe for Fly.io. Always `200 {ok:true}` if the process is up.

## Tests

```bash
bun test
```

Covers Ed25519 sign/verify round trip, signature tampering rejection, and
license key format/uniqueness.
