# stripe-mock

Dev-only fake Stripe for the DeployShuttle monetization chain. Serves a minimal
Payment Link page and emits **real, HMAC-signed** webhook events to the
license-server (`stripe.webhooks.constructEventAsync` verifies pure HMAC-SHA256
locally, so sharing `STRIPE_WEBHOOK_SECRET` is enough — no Stripe account needed).

Never deployed; not part of any Docker image.

## Run

```bash
STRIPE_WEBHOOK_SECRET=whsec_dev_mock bun run stripe-mock/server.ts
# or: make stripe-mock
```

## Env

| Var | Default | Purpose |
|---|---|---|
| `PORT` | `4242` | listen port |
| `STRIPE_WEBHOOK_SECRET` | `whsec_dev_mock` | must equal the license-server's |
| `WEBHOOK_URL` | `http://localhost:3000/webhooks/stripe` | license-server endpoint |
| `SUCCESS_URL` | `http://localhost:4321/thank-you` | redirect after "payment" |

## Endpoints

- `GET /pay` — fake checkout page (email + Pay €199.00) with per-purchase Refund buttons
- `POST /pay` (form `email`) — sends signed `checkout.session.completed`, 303 → `SUCCESS_URL`
- `POST /refund` (`payment_intent` in form or query) — sends signed `charge.refunded`
- `GET /purchases` — JSON of in-memory purchases (used by `scripts/e2e-license.sh`)
