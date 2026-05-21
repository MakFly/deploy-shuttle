import { Hono } from "hono";
import { logger } from "hono/logger";
import { activateRoute } from "./routes/activate";
import { deactivateRoute } from "./routes/deactivate";
import { refreshRoute } from "./routes/refresh";
import { pubkeyRoute } from "./routes/pubkey";
import { webhookRoute } from "./routes/webhooks";
import { ensureSchema } from "./lib/db";
import { env } from "./lib/env";

const app = new Hono();
app.use("*", logger());

app.get("/", (c) => c.json({ name: "deploy-shuttle license server", status: "ok" }));
app.get("/healthz", (c) => c.json({ ok: true }));

app.route("/activate", activateRoute);
app.route("/deactivate", deactivateRoute);
app.route("/refresh", refreshRoute);
app.route("/pubkey", pubkeyRoute);
app.route("/webhooks", webhookRoute);

await ensureSchema();

export default {
  port: env.port,
  fetch: app.fetch,
};
