import { Hono } from "hono";
import Stripe from "stripe";
import { env } from "../lib/env";
import { markWebhookProcessed, recordWebhookEvent, setLicenseStatusBySubscription, upsertLicense } from "../lib/db";
import { generateLicenseKey } from "../lib/keys";
import { sendLicenseKeyEmail } from "../lib/email";

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
      const customerId = stringId(session.customer);
      const subscriptionId = stringId(session.subscription);
      if (!customerId || !subscriptionId) {
        console.warn(`checkout.session.completed missing ids on session ${session.id}`);
        return;
      }
      const sub = await stripeClient.subscriptions.retrieve(subscriptionId);
      const key = generateLicenseKey();
      await upsertLicense({
        key,
        stripeCustomerId: customerId,
        stripeSubscriptionId: subscriptionId,
        tier: "pro",
        status: mapSubscriptionStatus(sub.status),
        seats: 1,
        expiresAt: subscriptionEndDate(sub),
      });
      const email = session.customer_details?.email ?? null;
      if (email) await sendLicenseKeyEmail(email, key);
      else console.warn(`checkout.session.completed has no customer email; key=${key} customer=${customerId}`);
      return;
    }
    case "customer.subscription.updated": {
      const sub = event.data.object as Stripe.Subscription;
      await setLicenseStatusBySubscription(sub.id, mapSubscriptionStatus(sub.status), subscriptionEndDate(sub));
      return;
    }
    case "customer.subscription.deleted": {
      const sub = event.data.object as Stripe.Subscription;
      await setLicenseStatusBySubscription(sub.id, "canceled", subscriptionEndDate(sub));
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

function mapSubscriptionStatus(s: Stripe.Subscription.Status): "active" | "past_due" | "canceled" {
  switch (s) {
    case "active":
    case "trialing":
      return "active";
    case "past_due":
    case "unpaid":
    case "incomplete":
      return "past_due";
    default:
      return "canceled";
  }
}

function subscriptionEndDate(sub: Stripe.Subscription): Date | null {
  // Stripe v17 typings expose current_period_end on items in newer API versions.
  // Falls back to null when absent.
  const candidate = (sub as unknown as { current_period_end?: number }).current_period_end;
  if (typeof candidate === "number") return new Date(candidate * 1000);
  const item = sub.items?.data?.[0] as { current_period_end?: number } | undefined;
  if (item && typeof item.current_period_end === "number") return new Date(item.current_period_end * 1000);
  return null;
}
