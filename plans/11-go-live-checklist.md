# 11 — Go-live checklist (real Stripe)

Status: **documented, not executed**. The whole chain below is already validated locally
with the mocked Stripe (`make e2e-license`, `stripe-mock/`); this checklist is the
production mirror of it. Work through it top to bottom on launch day.

## 1. Stripe (live mode)

- [ ] Create product **DeployShuttle Pro**
- [ ] Create one-time price: **199 EUR TTC** (tax behavior: inclusive)
- [ ] Create a **Payment Link** for that price: card only, collect customer email,
      promotion codes off
- [ ] Set the Payment Link confirmation to **redirect** to `https://deployshuttle.io/thank-you`
- [ ] Dashboard → Webhooks → add endpoint `https://license.deployshuttle.io/webhooks/stripe`
      with exactly these events: `checkout.session.completed`, `charge.refunded`
- [ ] Copy the signing secret (`whsec_live_…`) — goes to Fly secrets below

## 2. Database (Neon)

- [ ] Create a Neon project (EU region), copy `DATABASE_URL` **with `sslmode=require`**
- [ ] `cd license-server && DATABASE_URL=… bun run migrate`

## 3. Ed25519 keypair

- [ ] `cd license-server && bun run keygen` — run **once**, store both values in a
      password manager
- [ ] Private key → Fly secret only. Public key → Fly secret **and** GitHub Actions
      secret (step 5)

## 4. License server (Fly.io)

- [ ] `fly launch` from `license-server/` (Dockerfile + fly.toml already exist; app
      `shuttle-license`, region `cdg`)
- [ ] `fly secrets set DATABASE_URL=… LICENSE_PRIVATE_KEY_B64=… LICENSE_PUBLIC_KEY_B64=… STRIPE_SECRET_KEY=sk_live_… STRIPE_WEBHOOK_SECRET=whsec_live_… RESEND_API_KEY=…`
- [ ] ⚠ **Never set `MAILPIT_URL` in production** — it takes precedence over Resend and
      would swallow every license email
- [ ] `fly deploy`, then verify `https://license.deployshuttle.io/healthz` and `/pubkey`

## 5. Resend

- [ ] Verify the `deployshuttle.io` domain (SPF + DKIM records)
- [ ] Set `RESEND_FROM="DeployShuttle <noreply@deployshuttle.io>"` (Fly secret or leave default)

## 6. CLI release (armed license gates)

- [ ] GitHub repo secret `LICENSE_PUBKEY_B64` = public key from step 3
- [ ] GitHub repo variable `LICENSE_SERVER` = `https://license.deployshuttle.io`
- [ ] Tag a release via `sh scripts/release.sh …` and push the tag — release binaries
      are then license-gated (dev builds stay no-op)

## 7. Docs site

- [ ] `PUBLIC_STRIPE_PAYMENT_LINK=https://buy.stripe.com/… make site-build`
- [ ] Deploy `docs-site/dist/`; smoke-test `/pricing` buy button (en + fr) and
      `/thank-you` + `/fr/thank-you`

## 8. DNS

- [ ] `license.deployshuttle.io` → Fly app
- [ ] Site host → docs-site deployment

## 9. Live smoke test (with your own card)

- [ ] Buy Pro for real (199 EUR) → redirect lands on `/thank-you`
- [ ] License email arrives (check Resend logs)
- [ ] `shuttle license activate DS-…` on a real machine → `report --format pdf` works
- [ ] Refund yourself from the Stripe Dashboard → `shuttle license refresh` must fail
- [ ] Confirm the seat is freed (`activations` row gone or license `canceled`)
