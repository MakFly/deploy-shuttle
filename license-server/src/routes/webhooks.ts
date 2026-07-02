import { Hono } from "hono";
import Stripe from "stripe";
import { env } from "../lib/env";
import { findLicenseByPaymentIntent, markWebhookProcessed, recordWebhookEvent, setLicenseStatusByPaymentIntent, upsertLicense } from "../lib/db";
import { generateLicenseKey } from "../lib/keys";
import { sendLicenseKeyEmail } from "../lib/email";
import { inviteToProRepo, removeFromProRepo } from "../lib/github";

export const stripeClient = new Stripe(env.stripeSecretKey, {
  typescript: true,
});

export const webhookRoute = new Hono();

webhookRoute.post("/stripe", async (c) => {
  const sig = c.req.header("stripe-signature");
  if (!sig) return c.json({ error: "missing signature" }, 400);
  const raw = await c.req.text();
  let event: Stripe.Event;
  try {
    event = await stripeClient.webhooks.constructEventAsync(raw, sig, env.stripeWebhookSecret);
  } catch (err) {
    return c.json({ error: `bad signature: ${(err as Error).message}` }, 400);
  }

  const fresh = await recordWebhookEvent(event.id, event.type, event);
  if (!fresh) {
    // Already processed; idempotent ack.
    return c.json({ received: true, duplicate: true });
  }

  try {
    await dispatch(event);
    await markWebhookProcessed(event.id);
  } catch (err) {
    console.error(`webhook ${event.id} (${event.type}) failed:`, err);
    return c.json({ error: "internal" }, 500);
  }
  return c.json({ received: true });
});

async function dispatch(event: Stripe.Event): Promise<void> {
  switch (event.type) {
    case "checkout.session.completed": {
      const session = event.data.object as Stripe.Checkout.Session;
      if (session.mode !== "payment") {
        console.warn(`ignoring checkout.session.completed mode=${session.mode} on session ${session.id}`);
        return;
      }
      if (session.payment_status !== "paid") {
        // Async payment methods fire a later checkout.session.async_payment_succeeded;
        // for the card-only Payment Link this state should not happen.
        console.warn(`checkout.session.completed not paid (${session.payment_status}) on session ${session.id}`);
        return;
      }
      const paymentIntentId = stringId(session.payment_intent);
      if (!paymentIntentId) {
        console.warn(`checkout.session.completed missing payment_intent on session ${session.id}`);
        return;
      }
      const key = generateLicenseKey();
      const githubUsername = githubUsernameFromSession(session);
      await upsertLicense({
        key,
        stripeCustomerId: stringId(session.customer),
        stripePaymentIntentId: paymentIntentId,
        tier: "pro",
        status: "active",
        seats: 1,
        // One-time purchase: the license never expires. Activation JWTs
        // still rotate every TOKEN_GRACE_DAYS; only entitlement is perpetual.
        expiresAt: null,
        githubUsername,
      });
      const email = session.customer_details?.email ?? null;
      if (email) await sendLicenseKeyEmail(email, key);
      else console.warn(`checkout.session.completed has no customer email; key=${key} pi=${paymentIntentId}`);
      // Community perk, never fatal: the license is the entitlement.
      if (githubUsername) {
        try {
          await inviteToProRepo(githubUsername);
        } catch (err) {
          console.warn(`github invite for ${githubUsername} failed:`, err);
        }
      }
      return;
    }
    case "charge.refunded": {
      const charge = event.data.object as Stripe.Charge;
      const paymentIntentId = stringId(charge.payment_intent);
      if (!paymentIntentId) return;
      const license = await findLicenseByPaymentIntent(paymentIntentId);
      await setLicenseStatusByPaymentIntent(paymentIntentId, "canceled");
      if (license?.github_username) {
        try {
          await removeFromProRepo(license.github_username);
        } catch (err) {
          console.warn(`github removal for ${license.github_username} failed:`, err);
        }
      }
      return;
    }
    default:
      // Stored for audit, not actionable here.
      return;
  }
}

function stringId(value: string | { id: string } | null | undefined): string | null {
  if (!value) return null;
  return typeof value === "string" ? value : value.id;
}

// Payment Link custom field "github_username" (optional text field). A "@"
// prefix is tolerated since buyers often paste their handle that way.
function githubUsernameFromSession(session: Stripe.Checkout.Session): string | null {
  const field = session.custom_fields?.find((f) => f.key === "github_username");
  const raw = field?.text?.value?.trim().replace(/^@/, "") ?? "";
  return raw !== "" ? raw : null;
}
