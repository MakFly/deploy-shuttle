#!/usr/bin/env bash
# End-to-end monetization chain against the shared dev infra:
#   purchase (stripe-mock, signed webhook) → license key email (Mailpit)
#   → CLI activation → Pro gate unlock → refund → revoked refresh.
# Requires: infra-postgres + infra-mailpit running, bun, go, jq, curl.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DB=shuttle_license_e2e
LS_PORT=3999
MOCK_PORT=4299
RUN_ID="$(date +%s)"
EMAIL="e2e-${RUN_ID}@deployshuttle.test"
WHSEC="whsec_e2e_${RUN_ID}"
WORK="$(mktemp -d)"

PIDS=()
cleanup() {
  for pid in "${PIDS[@]:-}"; do kill "$pid" 2>/dev/null || true; done
  rm -rf "$WORK"
}
trap cleanup EXIT

fail() { echo "✗ $1" >&2; exit 1; }
step() { echo "── $1"; }

# 1. Preflight: shared dev infra must be up (never redeployed here).
step "preflight"
docker ps --format '{{.Names}}' | grep -q '^infra-postgres$' || fail "infra-postgres is not running (start the dev-infra stack)"
docker ps --format '{{.Names}}' | grep -q '^infra-mailpit$' || fail "infra-mailpit is not running (start the dev-infra stack)"
curl -sf http://localhost:8025/api/v1/info >/dev/null || fail "Mailpit API unreachable on :8025"

# 2. Throwaway database.
step "fresh database ${DB}"
docker exec infra-postgres psql -U test -d postgres \
  -c "DROP DATABASE IF EXISTS ${DB} WITH (FORCE);" -c "CREATE DATABASE ${DB};" >/dev/null

# 3. Keypair + migrate + license-server.
step "license-server on :${LS_PORT}"
cd "$ROOT/license-server"
bun install >/dev/null 2>&1
while IFS= read -r line; do export "$line"; done < <(bun run scripts/keygen.ts | grep '^LICENSE_')
[ -n "${LICENSE_PUBLIC_KEY_B64:-}" ] || fail "keygen did not produce LICENSE_PUBLIC_KEY_B64"
export DATABASE_URL="postgres://test:test@localhost:5432/${DB}"
export STRIPE_SECRET_KEY=sk_test_dummy
export STRIPE_WEBHOOK_SECRET="$WHSEC"
export MAILPIT_URL=http://localhost:8025
export PORT=$LS_PORT
bun run scripts/migrate.ts >/dev/null
bun run src/index.ts >"$WORK/license-server.log" 2>&1 &
PIDS+=($!)

# 4. stripe-mock.
step "stripe-mock on :${MOCK_PORT}"
PORT=$MOCK_PORT WEBHOOK_URL="http://localhost:${LS_PORT}/webhooks/stripe" \
  STRIPE_WEBHOOK_SECRET="$WHSEC" SUCCESS_URL=http://localhost:4321/thank-you \
  bun run "$ROOT/stripe-mock/server.ts" >"$WORK/stripe-mock.log" 2>&1 &
PIDS+=($!)
for _ in $(seq 1 30); do
  curl -sf "http://localhost:${LS_PORT}/healthz" >/dev/null && \
  curl -sf "http://localhost:${MOCK_PORT}/purchases" >/dev/null && break
  sleep 0.5
done
curl -sf "http://localhost:${LS_PORT}/healthz" >/dev/null || fail "license-server did not come up (see $WORK/license-server.log)"

# 5. Purchase (with the optional GitHub community perk field).
GH_USER="e2e-octocat"
step "purchase as ${EMAIL} (github: ${GH_USER})"
curl -sf -X POST "http://localhost:${MOCK_PORT}/pay" \
  -d "email=${EMAIL}" -d "github_username=${GH_USER}" -o /dev/null || fail "mock purchase rejected"

# 6. License key from the Mailpit email (proves the email leg).
step "license key from Mailpit"
MSG_ID=""
for _ in $(seq 1 20); do
  MSG_ID=$(curl -sf "http://localhost:8025/api/v1/search?query=to:${EMAIL}" | jq -r '.messages[0].ID // empty')
  [ -n "$MSG_ID" ] && break
  sleep 0.5
done
[ -n "$MSG_ID" ] || fail "license email never reached Mailpit"
KEY=$(curl -sf "http://localhost:8025/api/v1/message/${MSG_ID}" \
  | jq -r .Text | grep -oE 'DS-[A-Z0-9]{6}-[A-Z0-9]{6}-[A-Z0-9]{6}' | head -1)
[ -n "$KEY" ] || fail "no DS-… key found in the email body"
echo "   key: $KEY"

# 6b. GitHub perk: username stored, invite attempted (dev no-op logged).
STORED_GH=$(docker exec infra-postgres psql -U test -d "$DB" -tAc \
  "SELECT github_username FROM licenses WHERE key='${KEY}';" | tr -d '[:space:]')
[ "$STORED_GH" = "$GH_USER" ] || fail "github_username not stored (got '${STORED_GH}')"
grep -q "would invite ${GH_USER}" "$WORK/license-server.log" || fail "github invite was never attempted"

# 7. Gated CLI build (dev builds skip gates; ldflags arm them).
step "build gated CLI"
BIN="$WORK/shuttle"
LDPKG=github.com/MakFly/deploy-shuttle/go-cli/internal/version
(cd "$ROOT/go-cli" && go build -trimpath -ldflags \
  "-X ${LDPKG}.Version=e2e -X ${LDPKG}.LicensePubKeyB64=${LICENSE_PUBLIC_KEY_B64} -X ${LDPKG}.LicenseServer=http://localhost:${LS_PORT}" \
  -o "$BIN" ./cmd/shuttle)
export SHUTTLE_HOME="$WORK/home"
mkdir -p "$SHUTTLE_HOME"

# 8. Gate closed before activation, open after.
step "gate closed → activate → gate open"
echo '{}' >"$WORK/doctor.json"
if "$BIN" report --format html --input "$WORK/doctor.json" --output "$WORK/report.html" 2>"$WORK/gate.err"; then
  fail "report --format html succeeded WITHOUT a license"
fi
grep -q "Pro license" "$WORK/gate.err" || fail "unexpected pre-activation error: $(cat "$WORK/gate.err")"
"$BIN" license activate "$KEY" >/dev/null || fail "license activate failed"
STATUS_OUT="$("$BIN" license status)" || fail "license status errored"
echo "$STATUS_OUT" | grep -qi "pro" || fail "license status does not report pro: $STATUS_OUT"
"$BIN" report --format html --input "$WORK/doctor.json" --output "$WORK/report.html" || fail "report --format html failed AFTER activation"
[ -s "$WORK/report.html" ] || fail "report.html is empty"

# 9. Refund revokes; refresh must then fail.
step "refund → refresh must fail"
PI=$(curl -sf "http://localhost:${MOCK_PORT}/purchases" | jq -r '.[0].payment_intent')
[ -n "$PI" ] || fail "no payment_intent recorded by the mock"
curl -sf -X POST "http://localhost:${MOCK_PORT}/refund" -d "payment_intent=${PI}" >/dev/null || fail "mock refund rejected"
if "$BIN" license refresh >/dev/null 2>&1; then
  fail "license refresh succeeded AFTER refund (revocation broken)"
fi
grep -q "would remove ${GH_USER}" "$WORK/license-server.log" || fail "github removal was never attempted after refund"

echo
echo "E2E OK — key=${KEY}, email visible at http://localhost:8025 (to: ${EMAIL})"
