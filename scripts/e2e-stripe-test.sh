#!/usr/bin/env bash
# REAL Stripe test-mode run of the monetization chain. Same assertions as
# scripts/e2e-license.sh, but the events come from Stripe itself:
#   real Payment Link → human pays with the 4242 test card → stripe listen
#   forwards signed webhooks to the local license-server → key email (Mailpit)
#   → CLI activation → Pro gate unlock → real refund → revoked refresh.
# Requires: stripe CLI logged in (test mode), infra-postgres + infra-mailpit,
# bun, go, jq, curl. One human step: paying in the browser.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DB=shuttle_license_stripetest
LS_PORT=3999
LOOKUP_KEY=shuttle_pro_test_199
# v2: the link now carries the optional "GitHub username" custom field.
LINK_FILE="$ROOT/.shuttle/stripe-test-link-v2"
PAY_TIMEOUT="${PAY_TIMEOUT:-300}"   # seconds to wait for the human payment
WORK="$(mktemp -d)"

PIDS=()
cleanup() {
  for pid in "${PIDS[@]:-}"; do kill "$pid" 2>/dev/null || true; done
  # the stripe CLI forks a child that survives killing the parent
  pkill -f "stripe listen --events" 2>/dev/null || true
  rm -rf "$WORK"
}
trap cleanup EXIT

fail() { echo "✗ $1" >&2; exit 1; }
step() { echo "── $1"; }

# 1. Preflight.
step "preflight"
command -v stripe >/dev/null || fail "stripe CLI not installed"
stripe products list --limit 1 >/dev/null 2>&1 || fail "stripe CLI not authenticated (run: stripe login)"
docker ps --format '{{.Names}}' | grep -q '^infra-postgres$' || fail "infra-postgres is not running"
docker ps --format '{{.Names}}' | grep -q '^infra-mailpit$' || fail "infra-mailpit is not running"
curl -sf http://localhost:8025/api/v1/info >/dev/null || fail "Mailpit API unreachable on :8025"

# 2. Throwaway database.
step "fresh database ${DB}"
docker exec infra-postgres psql -U test -d postgres \
  -c "DROP DATABASE IF EXISTS ${DB} WITH (FORCE);" -c "CREATE DATABASE ${DB};" >/dev/null

# 3. Idempotent test-mode product / price / Payment Link.
step "stripe test-mode product + payment link"
PRICE_ID=$(stripe prices list --lookup-keys "$LOOKUP_KEY" | jq -r '.data[0].id // empty')
if [ -z "$PRICE_ID" ]; then
  PRODUCT_ID=$(stripe products create --name "DeployShuttle Pro (test)" | jq -r .id)
  PRICE_ID=$(stripe prices create --unit-amount 19900 --currency eur \
    --lookup-key "$LOOKUP_KEY" --product "$PRODUCT_ID" | jq -r .id)
fi
[ -n "$PRICE_ID" ] || fail "could not resolve a test price"
PAY_URL=""
[ -f "$LINK_FILE" ] && PAY_URL="$(cat "$LINK_FILE")"
if [ -z "$PAY_URL" ]; then
  PAY_URL=$(stripe payment_links create \
    -d "line_items[0][price]=${PRICE_ID}" -d "line_items[0][quantity]=1" \
    -d "custom_fields[0][key]=github_username" \
    -d "custom_fields[0][type]=text" \
    -d "custom_fields[0][optional]=true" \
    -d "custom_fields[0][label][type]=custom" \
    -d "custom_fields[0][label][custom]=GitHub username (community access)" | jq -r .url)
  [ -n "$PAY_URL" ] && [ "$PAY_URL" != "null" ] || fail "payment link creation failed"
  mkdir -p "$(dirname "$LINK_FILE")"
  printf '%s\n' "$PAY_URL" >"$LINK_FILE"
fi

# 4. stripe listen (background) — the whsec it prints is never echoed.
step "stripe listen → localhost:${LS_PORT}"
stripe listen --events checkout.session.completed,charge.refunded \
  --forward-to "localhost:${LS_PORT}/webhooks/stripe" >"$WORK/listen.log" 2>&1 &
PIDS+=($!)
WHSEC=""
for _ in $(seq 1 40); do
  WHSEC=$(grep -oE 'whsec_[A-Za-z0-9]+' "$WORK/listen.log" | head -1 || true)
  [ -n "$WHSEC" ] && break
  sleep 0.5
done
[ -n "$WHSEC" ] || fail "stripe listen never printed a signing secret (see $WORK/listen.log)"

# 5. Local license-server against the real webhook stream.
step "license-server on :${LS_PORT}"
cd "$ROOT/license-server"
bun install >/dev/null 2>&1
while IFS= read -r line; do export "$line"; done < <(bun run scripts/keygen.ts | grep '^LICENSE_')
[ -n "${LICENSE_PUBLIC_KEY_B64:-}" ] || fail "keygen did not produce LICENSE_PUBLIC_KEY_B64"
export DATABASE_URL="postgres://test:test@localhost:5432/${DB}"
export STRIPE_SECRET_KEY=sk_test_dummy   # webhook verify is pure HMAC; no API calls
export STRIPE_WEBHOOK_SECRET="$WHSEC"
export MAILPIT_URL=http://localhost:8025
export PORT=$LS_PORT
bun run scripts/migrate.ts >/dev/null
bun run src/index.ts >"$WORK/license-server.log" 2>&1 &
PIDS+=($!)
for _ in $(seq 1 30); do curl -sf "http://localhost:${LS_PORT}/healthz" >/dev/null && break; sleep 0.5; done
curl -sf "http://localhost:${LS_PORT}/healthz" >/dev/null || fail "license-server did not come up (see $WORK/license-server.log)"

# 6. Human step: pay with the Stripe test card.
step "waiting for a real test-mode payment (max ${PAY_TIMEOUT}s)"
echo
echo "  ➤ Open:  $PAY_URL"
echo "    Card 4242 4242 4242 4242 · any future expiry · any CVC · your email"
echo
KEY=""
for _ in $(seq 1 $((PAY_TIMEOUT / 5))); do
  KEY=$(docker exec infra-postgres psql -U test -d "$DB" -tAc \
    "SELECT key FROM licenses ORDER BY created_at DESC LIMIT 1;" 2>/dev/null | tr -d '[:space:]')
  [ -n "$KEY" ] && break
  sleep 5
done
[ -n "$KEY" ] || fail "no license appeared within ${PAY_TIMEOUT}s — was the payment completed?"
echo "   key: $KEY"
GH_STORED=$(docker exec infra-postgres psql -U test -d "$DB" -tAc \
  "SELECT COALESCE(github_username,'') FROM licenses WHERE key='${KEY}';" | tr -d '[:space:]')
if [ -n "$GH_STORED" ]; then
  echo "   github: $GH_STORED (community perk)"
  grep -q "would invite ${GH_STORED}" "$WORK/license-server.log" \
    || fail "github username stored but invite never attempted"
fi

# 7. The key email must have reached Mailpit.
step "license email in Mailpit"
MSG_ID=""
for _ in $(seq 1 20); do
  MSG_ID=$(curl -sf "http://localhost:8025/api/v1/search?query=%22${KEY}%22" | jq -r '.messages[0].ID // empty')
  [ -n "$MSG_ID" ] && break
  sleep 0.5
done
[ -n "$MSG_ID" ] || fail "no Mailpit email contains the key (checkout email missing?)"

# 8. Gated CLI: closed → activate → open.
step "build gated CLI + gate assertions"
BIN="$WORK/shuttle"
LDPKG=github.com/MakFly/deploy-shuttle/go-cli/internal/version
(cd "$ROOT/go-cli" && go build -trimpath -ldflags \
  "-X ${LDPKG}.Version=e2e-stripe -X ${LDPKG}.LicensePubKeyB64=${LICENSE_PUBLIC_KEY_B64} -X ${LDPKG}.LicenseServer=http://localhost:${LS_PORT}" \
  -o "$BIN" ./cmd/shuttle)
export SHUTTLE_HOME="$WORK/home"
mkdir -p "$SHUTTLE_HOME"
echo '{}' >"$WORK/doctor.json"
if "$BIN" report --format html --input "$WORK/doctor.json" --output "$WORK/report.html" 2>"$WORK/gate.err"; then
  fail "report --format html succeeded WITHOUT a license"
fi
grep -q "Pro license" "$WORK/gate.err" || fail "unexpected pre-activation error: $(cat "$WORK/gate.err")"
"$BIN" license activate "$KEY" >/dev/null || fail "license activate failed"
STATUS_OUT="$("$BIN" license status)" || fail "license status errored"
echo "$STATUS_OUT" | grep -qi "pro" || fail "license status does not report pro: $STATUS_OUT"
"$BIN" report --format html --input "$WORK/doctor.json" --output "$WORK/report.html" || fail "report --format html failed AFTER activation"

# 9. Real refund → revocation.
PI=$(docker exec infra-postgres psql -U test -d "$DB" -tAc \
  "SELECT stripe_payment_intent_id FROM licenses WHERE key='${KEY}';" | tr -d '[:space:]')
[ -n "$PI" ] || fail "no payment_intent stored for the license"
if [ -t 0 ]; then
  read -r -p "── refund test payment ${PI} now? [Y/n] " ANSWER
  case "${ANSWER:-y}" in y|Y|"") ;; *) fail "refund declined — leaving the test payment in place";; esac
fi
step "refund ${PI} → refresh must fail"
stripe refunds create --payment-intent "$PI" >/dev/null || fail "stripe refund failed"
REVOKED=""
for _ in $(seq 1 30); do
  REVOKED=$(docker exec infra-postgres psql -U test -d "$DB" -tAc \
    "SELECT status FROM licenses WHERE key='${KEY}';" | tr -d '[:space:]')
  [ "$REVOKED" = "canceled" ] && break
  sleep 2
done
[ "$REVOKED" = "canceled" ] || fail "charge.refunded webhook never canceled the license"
if "$BIN" license refresh >/dev/null 2>&1; then
  fail "license refresh succeeded AFTER refund (revocation broken)"
fi

echo
echo "E2E STRIPE TEST OK — key=${KEY}, payment ${PI} refunded, email at http://localhost:8025"
