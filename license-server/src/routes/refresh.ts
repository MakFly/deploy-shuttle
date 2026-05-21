import { Hono } from "hono";
import { findLicense, touchActivation } from "../lib/db";
import { signToken, verifyToken, type Claims } from "../lib/jwt";
import { env } from "../lib/env";

export const refreshRoute = new Hono();

refreshRoute.post("/", async (c) => {
  const body = await c.req.json().catch(() => null);
  if (!body || typeof body.token !== "string") {
    return c.json({ error: "token is required" }, 400);
  }
  const claims = await verifyToken(body.token, env.licensePublicKeyB64);
  if (!claims) return c.json({ error: "token signature invalid" }, 401);

  const license = await findLicense(claims.key);
  if (!license) return c.json({ error: "license revoked" }, 403);
  if (license.status !== "active") return c.json({ error: `license ${license.status}` }, 403);

  const active = await touchActivation(license.key, claims.fp);
  if (!active) return c.json({ error: "activation revoked" }, 403);

  const now = Math.floor(Date.now() / 1000);
  const grace = env.tokenGraceDays * 24 * 60 * 60;
  const next: Claims = {
    key: license.key,
    tier: "pro",
    fp: claims.fp,
    iat: now,
    exp: now + grace,
  };
  const token = await signToken(next, env.licensePrivateKeyB64);
  return c.json({
    token,
    tier: next.tier,
    expiresAt: new Date(next.exp * 1000).toISOString(),
  });
});
