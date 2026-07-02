# DeployShuttle license server

Stripe-backed license issuer for the DeployShuttle CLI Pro tier
(**one-time payment, perpetual license** — no subscription).

```
Stripe Payment Link ── webhook ──▶ license-server ── Postgres
                                       │
                                       ├─▶ /activate  (CLI → JWT 14 d)
                                       ├─▶ /deactivate (free machine slot)
                                       ├─▶ /refresh   (CLI → new JWT)
                                       └─▶ /pubkey    (verifying key)
```

## Stack

- **Bun** + **Hono** for the HTTP layer.
- **Postgres** (Neon free tier works) via `postgres.js`.
- **Stripe** (one-time Checkout via Payment Link, signed webhooks).
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
2. Add one **one-time** price: 199 EUR (TTC). No recurring price.
3. Create a **Payment Link** for that price (card payments; enable
   "collect customer email"). This is the Buy button URL for the
   pricing page — no server-side checkout code needed.
4. Create a webhook endpoint pointing at `https://<host>/webhooks/stripe`.
   Subscribe to `checkout.session.completed` and `charge.refunded`.
   Copy the signing secret into `STRIPE_WEBHOOK_SECRET`.
5. On `checkout.session.completed` (mode `payment`, paid), the server
   generates a perpetual license key, stores it, and emails it to the
   customer (Resend). On `charge.refunded`, the license is revoked.

## Deploy on Fly.io

```bash
fly launch --name shuttle-license --copy-config --no-deploy
fly secrets set \
  DATABASE_URL="..." \
  LICENSE_PRIVATE_KEY_B64="..." \
  LICENSE_PUBLIC_KEY_B64="..." \
  STRIPE_SECRET_KEY="..." \
  STRIPE_WEBHOOK_SECRET="..." \
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
license key format/uniqueness. Webhook integration tests run against a real
Postgres when `TEST_DATABASE_URL` is set (skipped otherwise):

```bash
TEST_DATABASE_URL=postgres://test:test@localhost:5432/shuttle_license_test bun test
```

## GitHub community perk (Spin-style)

The Payment Link carries an optional **"GitHub username"** custom field
(`key=github_username`). When the buyer fills it, `checkout.session.completed`
stores the username on the license row and invites it to the private Pro repo
(`GITHUB_PRO_REPO`, e.g. `MakFly/deployshuttle-pro`) via the GitHub API;
`charge.refunded` removes the collaborator. The invite is a **perk, never the
entitlement**: GitHub API failures are logged and the license is issued anyway.
Requires `GITHUB_TOKEN` (collaborators write on the Pro repo); without it the
server logs `[github:dev] would invite <user>` instead.

## Local dev (mock Stripe + Mailpit)

No Stripe account needed: `stripe-mock/` (repo root) serves a fake Payment
Link page and sends HMAC-signed webhooks with a shared `STRIPE_WEBHOOK_SECRET`.
Set `MAILPIT_URL=http://localhost:8025` to deliver license emails to the dev
Mailpit (UI on :8025) instead of Resend — dev only, never in production.

The full chain (purchase → email → CLI activation → refund revocation) runs
with `make e2e-license` from the repo root.

## Real Stripe test-mode run

`make e2e-stripe-test` replays the same chain against **real Stripe** (test
mode): the script creates an idempotent test product/price/Payment Link via
the Stripe CLI, starts `stripe listen` to forward signed webhooks to the local
server, then waits for you to pay in the browser with the test card
`4242 4242 4242 4242` (any future expiry, any CVC, your email). It asserts the
key email in Mailpit (:8025), activates a gated CLI build, then refunds the
test payment (`stripe refunds create`) and verifies the license is revoked.

Prerequisites: `stripe login` done (test mode), infra-postgres + infra-mailpit
running. No real Stripe API key touches the script — webhook verification is
pure HMAC, so the server runs with `STRIPE_SECRET_KEY=sk_test_dummy` and the
ephemeral `whsec_…` printed by `stripe listen`.
