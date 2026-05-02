import { Hono } from "hono";
import { env } from "../lib/env";

export const pubkeyRoute = new Hono();

// Public, cacheable. Lets a freshly installed CLI fetch the verifying key
// without baking it into the binary. Production binaries embed it, this
// endpoint exists for tooling parity and rotation drills.
pubkeyRoute.get("/", (c) => {
  c.header("Cache-Control", "public, max-age=3600");
  return c.json({
    algorithm: "Ed25519",
    publicKeyB64: env.licensePublicKeyB64,
  });
});
