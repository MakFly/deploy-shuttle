import { Hono } from "hono";
import { findLicense, recordActivation } from "../lib/db";
import { signToken, type Claims } from "../lib/jwt";
import { env } from "../lib/env";

export const activateRoute = new Hono();

activateRoute.post("/", async (c) => {
  const body = await c.req.json().catch(() => null);
  if (!body || typeof body.key !== "string" || typeof body.machineFingerprint !== "string") {
    return c.json({ error: "key and machineFingerprint are required" }, 400);
  }
  const cliVersion = typeof body.cliVersion === "string" ? body.cliVersion : null;

  const license = await findLicense(body.key);
  if (!license) return c.json({ error: "unknown license key" }, 404);
  if (license.status !== "active") return c.json({ error: `license ${license.status}` }, 403);
  if (license.tier !== "pro") return c.json({ error: "unsupported tier" }, 403);

  await recordActivation(license.key, body.machineFingerprint, cliVersion);

  const now = Math.floor(Date.now() / 1000);
  const grace = env.tokenGraceDays * 24 * 60 * 60;
  const claims: Claims = {
    key: license.key,
    tier: "pro",
    fp: body.machineFingerprint,
    iat: now,
    exp: now + grace,
  };
  const token = await signToken(claims, env.licensePrivateKeyB64);
  return c.json({
    token,
    tier: claims.tier,
    expiresAt: new Date(claims.exp * 1000).toISOString(),
  });
});
