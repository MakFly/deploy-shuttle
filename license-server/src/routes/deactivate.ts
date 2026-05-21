import { Hono } from "hono";
import { deactivateActivation } from "../lib/db";
import { env } from "../lib/env";
import { verifyToken } from "../lib/jwt";

export const deactivateRoute = new Hono();

deactivateRoute.post("/", async (c) => {
  const body = await c.req.json().catch(() => null);
  if (!body || typeof body.token !== "string") {
    return c.json({ error: "token is required" }, 400);
  }
  const claims = await verifyToken(body.token, env.licensePublicKeyB64);
  if (!claims) return c.json({ error: "token signature invalid" }, 401);

  const deactivated = await deactivateActivation(claims.key, claims.fp);
  return c.json({ deactivated });
});
