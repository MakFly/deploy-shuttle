# Execution Tracker

This file tracks which parts of the product plan are being implemented.

## Current Slice - Doctor MVP Foundation

**Status:** Implemented  
**Started:** 2026-05-01  
**Plan sources:**

- `plans/02-mvp-scope.md`
- `plans/03-check-catalog.md`
- `plans/04-scoring-config-reports.md`
- `plans/06-architecture-security.md`

### Scope

Implement the first `shuttle doctor` foundation:

- local execution adapter;
- check types and result model;
- deterministic scoring;
- console report;
- JSON report;
- `--fail-below`;
- initial local checks:
  - `system.os_supported`;
  - `system.disk_space_low`;
  - `docker.not_installed`;
  - `firewall.ufw_inactive`;
  - `firewall.database_port_public`;
  - `secrets.env_world_readable`;
  - `secrets.env_in_git`;
  - `caddy.not_installed`.

### Explicitly Not In This Slice

- remote SSH `doctor --target user@host`;
- `.shuttle.yml` support;
- standalone `report` command;
- `harden` command;
- Cloudflare checks;
- HTML/Markdown report generation;
- dashboard or hosted reporting.

### Completion Checklist

- [x] CLI command exists at `src/cli/doctor.ts`.
- [x] Root CLI loads `doctor`.
- [x] Local `doctor` runs without a config file.
- [x] Console report is readable.
- [x] JSON report is valid.
- [x] Score is deterministic.
- [x] `--fail-below` exits non-zero when score is below threshold.
- [x] Unit tests cover scoring and initial checks.
- [x] `bunx biome check .` passes.
- [x] `bunx tsc --noEmit` passes.
- [x] `bun test` passes.
- [x] `ig index .` has been rebuilt.

## Current Slice - Go CLI Migration

**Status:** Implemented  
**Started:** 2026-05-01  
**Completed:** 2026-05-01  
**Plan sources:**

- `plans/02-mvp-scope.md`
- `plans/06-architecture-security.md`
- latest user decision: migrate active CLI to Go, archive TS in `legacy/ts-cli/`

### Scope

- Move TypeScript/Bun CLI to `legacy/ts-cli/`.
- Create active Go CLI in `go-cli/`.
- Use Cobra for maintainable command growth.
- Keep `shuttle.yml` compatibility.
- Recreate current command surface.
- Switch CI/release to Go.

### Completion Checklist

- [x] TS implementation moved to `legacy/ts-cli/`.
- [x] Go module exists under `go-cli/`.
- [x] Root command exposes current command surface.
- [x] `doctor` works in Go with console/JSON/scoring.
- [x] Config loader supports `shuttle.yml`.
- [x] CI uses Go only.
- [x] README and CLAUDE describe Go as active implementation.
- [x] `gofmt`, `go test ./...`, and `go vet ./...` pass.
- [x] `ig index .` has been rebuilt.

### Compatibility Notes

- `doctor` is the active product hook and is implemented for local scans.
- `secrets` uses passphrase-protected Argon2id + XChaCha20-Poly1305 encrypted local storage and can push `.env` to configured servers.
- `rollback` remains guarded until the Go port has persisted blue/green deployment state parity.
- `report` and `harden` remain planned readiness work.

## Current Slice - Remote Doctor SSH

**Status:** Implemented  
**Started:** 2026-05-01  
**Completed:** 2026-05-01  
**Plan sources:**

- `plans/02-mvp-scope.md`
- `plans/06-architecture-security.md`
- product hook: `shuttle doctor --target root@server`

### Scope

- Parse `doctor --target user@host`.
- Support optional SSH port with `user@host:port`.
- Reuse the same readiness check runner over an SSH exec adapter.
- Preserve console/JSON output and `--fail-below` behavior.
- Keep local `doctor` behavior unchanged.

### Completion Checklist

- [x] SSH exec adapter implements the readiness `execx.Adapter`.
- [x] `doctor --target user@host` runs checks over SSH.
- [x] `doctor --target user@host:port` supports custom SSH ports.
- [x] Console and JSON reports show the remote target.
- [x] Failure semantics remain unchanged for critical findings and `--fail-below`.
- [x] Unit tests cover target parsing.
- [x] `gofmt`, `go test ./...`, `go vet ./...`, build, and `ig index .` pass.

## Current Slice - Database/Adminer Exposure Semantics

**Status:** Implemented  
**Started:** 2026-05-01  
**Completed:** 2026-05-01  
**Plan sources:**

- `plans/03-check-catalog.md`
- real VPS validation: API/Adminer may reach DB, Adminer must be home-IP restricted

### Scope

- Keep `firewall.database_port_public` critical when a database port is publicly reachable or publicly allowed.
- Downgrade to high severity when a database binds public interfaces but UFW is active, deny-by-default, and has no public allow for that DB port.
- Include listener process evidence from `ss -ltnp`.
- Add Adminer protection check for IP restriction, deny rule, and basic auth in Caddy config.
- Validate against `root@<prod-vps>:7022`.

### Completion Checklist

- [x] Postgres `0.0.0.0:5432` includes process evidence.
- [x] Firewall-restricted DB listener is no longer treated as critical Internet exposure.
- [x] Remediation explains API/Adminer private-network or allowlist access.
- [x] Adminer check detects Caddy IP restriction + deny-by-default + basic auth.
- [x] Real VPS scan returns `90/100` with Adminer passed and DB warning high.
- [x] Unit tests cover firewall-restricted and publicly allowed DB port cases.

## Current Slice - Docker Runtime Readiness

**Status:** Implemented  
**Started:** 2026-05-01  
**Completed:** 2026-05-01  
**Plan sources:**

- `plans/03-check-catalog.md`
- real VPS validation: single-node Docker Swarm plus Docker classic sidecar workloads

### Scope

- Support Docker classic containers and Docker Swarm services.
- Support mixed single-VPS setups where Swarm services and classic containers coexist.
- Add Docker service enabled/active check.
- Add restart policy check.
- Add healthcheck check.
- Add root-user check.
- Add Docker socket mount check.
- Add Caddy admin API exposure check.

### Completion Checklist

- [x] Swarm service restart policies are read from `TaskTemplate.RestartPolicy`.
- [x] Classic container restart policies are read from `HostConfig.RestartPolicy`.
- [x] Swarm service healthchecks are read from `TaskTemplate.ContainerSpec.Healthcheck`.
- [x] Classic container healthchecks are read from `Config.Healthcheck`.
- [x] Runtime evidence reports `classic`, `swarm`, or `mixed`.
- [x] Mixed VPS scan reports `runtimeMode: mixed`.
- [x] Real VPS scan covers Swarm services and the classic `frontend-iautos` container.
- [x] Unit tests cover runtime output parsing and healthcheck detection.

## Current Slice - Readiness Config Allowlist

**Status:** Implemented  
**Started:** 2026-05-01  
**Completed:** 2026-05-01  
**Plan sources:**

- `plans/04-scoring-config-reports.md`
- real VPS validation: allow expected Docker socket tooling and worker services

### Scope

- Add `.shuttle.yml` readiness config loading.
- Add `doctor --config <path>`.
- Support `checks.profile`.
- Support `checks.ignore`.
- Support `docker.allowDockerSocket`.
- Support `docker.allowRoot`.
- Support `docker.workerServices`.
- Support exact workload names and simple prefix/suffix wildcards.

### Completion Checklist

- [x] Ignored checks do not penalize score.
- [x] Allowlisted Docker socket workloads are removed from findings.
- [x] Worker services can be excluded from healthcheck findings.
- [x] Root allowlist is separate from worker healthcheck allowlist.
- [x] Report includes `configPath`.
- [x] Real VPS config test lifts score from `70` to `80` while keeping remaining risks visible.
- [x] Unit tests cover config loading and allowlist application.

## Current Slice - Local PDF Report Renderer

**Status:** Implemented  
**Started:** 2026-05-02  
**Completed:** 2026-05-02  
**Plan sources:**

- `plans/04-scoring-config-reports.md`
- user decision: use React PDF

### Scope

- Add `shuttle report`.
- Generate Markdown reports from doctor JSON.
- Generate PDF reports from doctor JSON with `@react-pdf/renderer`.
- Keep PDF rendering as an optional local renderer in `report-pdf/`.
- Do not start the cloud dashboard yet.

### Completion Checklist

- [x] `report --format markdown --input doctor.json --output report.md` works.
- [x] `report --format pdf --input doctor.json --output report.pdf` works.
- [x] React PDF renderer lives outside the Go CLI core.
- [x] Renderer typecheck passes with `bun run check`.
- [x] Real VPS doctor JSON renders to Markdown and PDF.
- [x] Cloud dashboard remains explicitly deferred.

## Current Slice - Latest Report Workflow

**Status:** Implemented  
**Started:** 2026-05-02  
**Completed:** 2026-05-02  
**Plan sources:**

- `plans/04-scoring-config-reports.md`
- report CLI ergonomics

### Scope

- Add `doctor --output <path>` to persist doctor JSON.
- Let `report` default to `.shuttle/latest-report.json`.
- Keep `report --input <path>` for explicit JSON input.
- Create parent directories for doctor output.

### Completion Checklist

- [x] `doctor --output .shuttle/latest-report.json` writes JSON.
- [x] `report --format pdf --output report.pdf` reads the default latest report.
- [x] Missing default report returns an actionable error.
- [x] Unit tests cover output directory creation.

## Current Slice - Report v1 Polish

**Status:** Implemented  
**Started:** 2026-05-02  
**Completed:** 2026-05-02  
**Plan sources:**

- `plans/04-scoring-config-reports.md`
- product direction: shareable production readiness report

### Scope

- Add executive summary to Markdown and PDF reports.
- Add next actions derived from remediation text.
- Rename ignored findings to accepted risks in the report surface.
- Add compact evidence summaries for key fields.
- Keep raw evidence out of the client-facing report body.

### Completion Checklist

- [x] Markdown includes executive summary.
- [x] Markdown includes next actions.
- [x] Markdown includes accepted risks.
- [x] Markdown includes compact evidence.
- [x] PDF includes executive summary.
- [x] PDF includes next actions.
- [x] PDF includes accepted risks.
- [x] Real VPS JSON renders to polished Markdown and PDF.

## Current Slice - Harden Dry-Run Planner

**Status:** Implemented  
**Started:** 2026-05-02  
**Completed:** 2026-05-02  
**Plan sources:**

- `plans/02-mvp-scope.md`
- `plans/06-architecture-security.md`
- previous stop note: dry-run hardening planner

### Scope

- Add `shuttle harden --dry-run`.
- Read latest doctor JSON from `.shuttle/latest-report.json` by default.
- Accept `--input <doctor.json>`, `--target user@host`, and `--format console|json`.
- Convert failed, non-ignored findings into concrete proposed actions.
- Group actions by category and surface source check ID + severity.
- Do not mutate any local or remote system in this slice.

### Completion Checklist

- [x] `harden/planner.go` maps known finding IDs to actions with rationale, commands, and notes.
- [x] `harden/render.go` prints a grouped, dry-run-labelled console plan.
- [x] CLI command requires `--dry-run` and refuses any execution path.
- [x] Default input falls back to `.shuttle/latest-report.json`.
- [x] JSON output is supported via `--format json`.
- [x] Unit tests cover empty plans, ignored findings, all known mappings, port-specific commands, and console rendering.
- [x] `gofmt`, `go vet ./...`, `go test ./...` pass.
- [x] Build via `scripts/build-go.sh` and CLI smoke test of `harden --help` and `harden --dry-run --input <sample>`.
- [x] CLAUDE.md command surface updated.

## Current Slice - Harden Safe Local Apply

**Status:** Implemented  
**Started:** 2026-05-02  
**Completed:** 2026-05-02  
**Plan sources:**

- previous slice: `Harden Dry-Run Planner`
- product direction: convert dry-run plan to safe, scoped local execution

### Scope

- Add `--apply` and `--yes` flags to `harden`.
- Make `--dry-run` and `--apply` mutually exclusive and required.
- Tag actions with `SafeLocalApply`; only those run automatically.
- First slice scope: `secrets.env_world_readable` → `chmod 600 .env` only.
- Refuse `--apply` with `--target` (remote execution still pending).
- Hard-allow only specific commands and target paths inside the project tree.

### Completion Checklist

- [x] `Action.SafeLocalApply` flag added; only `secrets.tighten-env-perms` is marked safe.
- [x] `harden/apply.go` runs allow-listed commands (chmod 600 only on local `.env`).
- [x] CLI requires confirmation via `--yes`; preview mode prints planned count.
- [x] CLI rejects `--apply --target` until SSH execution exists.
- [x] CLI rejects unsafe absolute paths, parent traversal, non-`.env` targets, modes other than 600.
- [x] Unit tests cover apply success, skip-unsafe, allow-list rejection, path/mode rejection, summary rendering.
- [x] `gofmt`, `go vet ./...`, `go test ./...` pass.
- [x] End-to-end smoke confirmed `.env` mode flips from 644 to 600.

## Current Slice - Harden Apply Over SSH

**Status:** Implemented  
**Started:** 2026-05-02  
**Completed:** 2026-05-02  
**Plan sources:**

- previous slice: `Harden Safe Local Apply`
- product hook: `harden --apply --target user@host`

### Scope

- Refactor `harden.Apply(adapter, plan)` to take an `execx.Adapter`.
- Reuse the existing SSH adapter when `--apply --target user@host` is set.
- Keep the same allow-list and validation (chmod 600 only, project-local `.env`).
- Drive existence check (`test -f`) and `chmod` through the adapter so the same code path applies locally and remotely.
- Keep `--yes` confirmation requirement.

### Completion Checklist

- [x] `Apply` accepts an `execx.Adapter`.
- [x] CLI builds an SSH adapter when `--target` is set with `--apply`.
- [x] Allow-list rejections still occur before any shell call.
- [x] Adapter shell calls use `shellQuote` to defuse path metacharacters.
- [x] Tests cover local apply, fake-adapter probe + chmod, adapter failure, missing target, and shell-quote escaping.
- [x] `gofmt`, `go vet ./...`, `go test ./...` pass.
- [x] Local smoke confirmed `.env` 644 → 600 unchanged after refactor.

## Current Slice - Harden Allow-List UFW Deny

**Status:** Implemented  
**Started:** 2026-05-02  
**Completed:** 2026-05-02  
**Plan sources:**

- previous slice: `Harden Apply Over SSH`
- real VPS dry-run validation: `firewall.database_port_public` exposes 5432/tcp publicly

### Scope

- Rename `Action.SafeLocalApply` to `SafeAutoApply` (same flag, used locally and over SSH).
- Allow-list `ufw deny <port>/tcp` in `harden --apply` with strict validation.
- Mark `firewall.lock-db-ports` actions as safe-auto-apply when at least one port is present.
- Reject non-`deny` ufw verbs, non-`/tcp` specs, non-numeric ports, and propagate stderr.
- No remote execution on the production VPS in this slice.

### Completion Checklist

- [x] `Action.SafeAutoApply` field replaces the old `SafeLocalApply` everywhere.
- [x] `runUFWDeny` enforces deny-only, `/tcp`-only, numeric port.
- [x] `firewall.lock-db-ports` is `SafeAutoApply: true` when commands exist.
- [x] Existing chmod allow-list behavior unchanged.
- [x] Unit tests cover ufw happy path and four rejection paths.
- [x] Planner test asserts which actions are flagged safe.
- [x] `gofmt`, `go vet ./...`, `go test ./...` pass.
- [x] `harden --dry-run --format json` against real VPS shows `safeAutoApply: true` on the lock-db-ports action.

## Current Slice - HTML Report Format

**Status:** Implemented  
**Started:** 2026-05-02  
**Completed:** 2026-05-02  
**Plan sources:**

- `plans/02-mvp-scope.md` (acceptance: Markdown + HTML reports)
- `plans/04-scoring-config-reports.md`

### Scope

- Add `--format html` to `report`.
- Generate self-contained HTML (inline CSS, no external assets).
- Reuse the same data shape as Markdown (exec summary, next actions, findings by severity, accepted risks, evidence).
- Use Go `html/template` so user-supplied content (target, titles, summaries) is escaped.

### Completion Checklist

- [x] `report --format html --output shuttle-report.html` writes HTML.
- [x] HTML defaults output path to `shuttle-report.html`.
- [x] Template uses `html/template` and escapes injected content.
- [x] Sections collapse cleanly when no findings or no accepted risks exist.
- [x] Unit tests cover key metadata, escaping, and empty-section behavior.
- [x] Smoke test from real VPS doctor report renders score, target, and findings.
- [x] `gofmt`, `go vet ./...`, `go test ./...` pass.

## Current Slice - Sellable Surface (README + Install + Catalog)

**Status:** Implemented  
**Started:** 2026-05-02  
**Completed:** 2026-05-02  
**Plan sources:**

- `plans/02-mvp-scope.md` (acceptance: docs include quickstart and check catalog; install script works on macOS/Linux)
- `plans/07-docs-and-implementation-prompt.md`

### Scope

- Rewrite `README.md` as the marketing surface: 30-second pitch, quickstart, doctor / report / harden flow, CI snippet, configuration, check catalog link, architecture diagram.
- Add user-facing `docs/check-catalog.md` listing the 15 shipped checks with severity and intent.
- Add `scripts/install.sh` for `curl | sh` installation (Linux x64, macOS x64, macOS arm64), with checksum verification when published.

### Completion Checklist

- [x] README pitches the product, shows the doctor → report → harden flow, and removes stale "planned" labels for shipped features.
- [x] README references `docs/check-catalog.md` and links to the architecture plan.
- [x] `docs/check-catalog.md` covers all 15 checks grouped by category with severity and what each verifies.
- [x] `scripts/install.sh` detects OS/arch, supports DEPLOY_SHUTTLE_VERSION and DEPLOY_SHUTTLE_INSTALL_DIR, verifies checksums when present, and warns about $PATH.
- [x] Install script passes `sh -n` syntax check.

## Current Slice - Credibility Checks

**Status:** Implemented  
**Started:** 2026-05-02  
**Completed:** 2026-05-02  
**Plan sources:**

- `plans/03-check-catalog.md` (roadmap entries: SSH hardening, automatic upgrades, fail2ban, swap, time sync)
- product positioning: 15 checks is the MVP minimum; 21 is the credibility floor

### Scope

- Add 6 new checks: `ssh.root_login_enabled`, `ssh.password_auth_enabled`, `system.unattended_upgrades_inactive`, `system.fail2ban_inactive`, `system.swap_missing`, `system.time_sync_inactive`.
- Skip gracefully on non-Linux dev hosts (no `systemctl`) or when sshd_config is not readable.
- Map each new check to an actionable harden plan note (manual, not auto-apply).
- Update the user-facing catalog and README count from 15 → 21.

### Completion Checklist

- [x] `system_checks.go` implements the six new checks with proper Skipped behavior.
- [x] `doctor.go` registers the new checks in the slice (21 total).
- [x] `system_checks_test.go` covers happy path, failure path, and skip path for each check.
- [x] `harden/planner.go` emits actionable notes for every new finding.
- [x] `docs/check-catalog.md` and `README.md` reference the 21-check count and the new SSH/system rows.
- [x] `gofmt`, `go vet ./...`, `go test ./...` pass.
- [x] Local doctor smoke shows the new checks running and contributing to the score.

## Current Slice - License Package + Pro Gates

**Status:** Implemented (server pending - Phase 1a)  
**Started:** 2026-05-02  
**Completed:** 2026-05-02 (code complete, awaits license server)  
**Plan sources:**

- `~/.claude/plans/on-laisse-public-pour-stateful-oasis.md` (Phase 1b)

### Scope

- New `internal/version` package exposing `Version`, `LicensePubKeyB64`, `LicenseServer` injected via `-ldflags`.
- New `internal/license` package: Ed25519 token format, store at `~/.shuttle/license.json` (0600), HTTP client for activate/refresh, machine fingerprint, `Require(feature)` helper.
- Refactor `license` CLI command: `activate`, `status`, `refresh`, `deactivate` against the new client.
- Gate Pro features: `doctor --target`, `doctor --config`, `report --format html|pdf`, `harden --apply`.
- `scripts/build-go.sh` accepts `LICENSE_PUBKEY_B64`, `LICENSE_SERVER`, `DEPLOY_SHUTTLE_VERSION` and injects via ldflags.

### Completion Checklist

- [x] `internal/license/token.go` implements Ed25519-signed JWT-compatible tokens with sign/verify pair.
- [x] `internal/license/store.go` reads/writes JSON license state under `DEPLOY_SHUTTLE_HOME` (defaults to `~/.shuttle`).
- [x] `internal/license/client.go` issues `POST /activate` and `POST /refresh` against the license server.
- [x] `internal/license/fingerprint.go` returns `sha256(hostname|machine-id)`.
- [x] `internal/license/require.go` enforces tier=pro with offline grace; dev builds (no embedded key) and `DEPLOY_SHUTTLE_DEV=1` bypass.
- [x] CLI `license` subcommands: activate / status / refresh / deactivate.
- [x] Pro gates wired in doctor.go (target + config), report.go (html + pdf), harden.go (apply).
- [x] Tests: token (happy/expired/fp/wrong-key/malformed), store (round-trip/missing/clear), require (no-op dev/no-license/valid/expired/dev-override).
- [x] `gofmt`, `go vet ./...`, `go test ./...` pass.
- [x] Build with `LICENSE_PUBKEY_B64=...` produces a binary that refuses `doctor --target` with the correct UX message.
- [x] `DEPLOY_SHUTTLE_DEV=1` bypasses gates on a binary with embedded key (intended for builders/QA).

### Pending (Phase 1a, separate slice)

- License server (Bun + Hono + Drizzle + Postgres + Stripe webhook).
- Stripe product + prices + webhook signature.
- Resend email on activation.
- Release workflow secret `LICENSE_PUBKEY_B64` so prod binaries are gated.

## Current Slice - License Server (Bun + Hono + Stripe)

**Status:** Implemented (code only; awaits Stripe + Postgres + Fly.io provisioning)  
**Started:** 2026-05-02  
**Completed:** 2026-05-02 (code complete)  
**Plan sources:**

- `~/.claude/plans/on-laisse-public-pour-stateful-oasis.md` (Phase 1a)

### Scope

- Bun + Hono server in `license-server/` exposing `/activate`, `/refresh`, `/pubkey`, `/webhooks/stripe`, `/healthz`.
- Postgres schema (licenses, activations, webhook_events) bootstrapped on cold-boot.
- Ed25519 JWT signer using Web Crypto, byte-compatible with the Go CLI verifier.
- Stripe webhook handler (`checkout.session.completed`, subscription updated/deleted) with idempotent storage.
- Resend transactional email for license key delivery (no-op when API key absent).
- License key generator: `DS-XXXXXX-XXXXXX-XXXXXX` Crockford base32, statistically unique.
- Operational: Dockerfile, fly.toml, .env.example, README with deploy steps.
- CI: GitHub Actions runs `bun typecheck` and `bun test` alongside the Go suite.

### Completion Checklist

- [x] `src/index.ts` Hono app with logger, healthz, schema bootstrap.
- [x] `src/routes/{activate,refresh,pubkey,webhooks}.ts` cover the four required endpoints.
- [x] `src/lib/{env,db,jwt,keys,email}.ts` strict env, postgres.js wrapper, Ed25519 sign/verify, key generator, Resend client.
- [x] `tests/jwt.test.ts` covers sign + verify, signature tampering, malformed input.
- [x] `tests/keys.test.ts` covers format and uniqueness over 1000 samples.
- [x] **Cross-compat test**: `go-cli/internal/license/crosscompat_test.go` verifies a Bun-signed token using the matching Bun-generated public key. Locks the wire contract.
- [x] Dockerfile + fly.toml + README deploy guide.
- [x] CI runs both Go and Bun test suites.
- [x] `bun typecheck` and `bun test` pass locally.
- [x] `gofmt`, `go vet ./...`, `go test ./...` still pass.

### Pending (operational, not code)

- Generate the production Ed25519 keypair, store privately.
- Provision Postgres (Neon free tier) and set `DATABASE_URL`.
- Create the Stripe product `DeployShuttle Pro` with monthly + annual prices.
- Create the Stripe webhook → `https://<host>/webhooks/stripe`, store signing secret.
- Configure Resend (or any SMTP-equivalent fronted by the same `sendLicenseKeyEmail` interface).
- `fly deploy` and put the public URL in `LICENSE_SERVER` of the release workflow.
- Release workflow: add `LICENSE_PUBKEY_B64` secret so future tags ship a gated binary.

## Current Slice - Public GitHub Action

**Status:** Implemented (code only; awaits a `v1` tag and Marketplace publish)
**Started:** 2026-05-02
**Plan sources:**

- `plans/09-critique-and-deltas.md` (livrable 4.1: GitHub Action publique)
- distribution priority before any dashboard work

### Scope

- Composite `action.yml` at the repo root, installable as `MakFly/deploy-shuttle@v1`.
- Inputs: `target`, `config`, `fail-below`, `output`, `ssh-private-key`, `ssh-known-hosts`, `version`.
- Outputs: `score`, `level`, `report` (path).
- Auto-installs the CLI via `scripts/install.sh`.
- Configures SSH agent + known_hosts when a remote target is supplied.
- Writes a job summary with score and level.
- README documents the Action as the recommended CI integration.

### Completion Checklist

- [x] `action.yml` validates as YAML and exposes the documented inputs/outputs.
- [x] JSON parsing uses `jq` (pre-installed on GitHub-hosted runners).
- [x] README shows the `uses: MakFly/deploy-shuttle@v1` snippet.
- [ ] Tag `v1` published once the install script URL is reachable on a tagged release.
- [ ] Marketplace listing once Stripe + license server are live.

## Current Slice - P0 Check Pack (21 -> 30)

**Status:** Implemented
**Started:** 2026-05-02
**Plan sources:**

- `plans/09-critique-and-deltas.md` (livrable 4.2: P0 pack lifts the credibility floor)
- `plans/03-check-catalog.md` (catalog rows that were still planned)

### Scope

Add 10 new readiness checks plus an `app:` section in `.shuttle.yml`:

- `system.updates_pending` (apt list --upgradable count, skipped on non-APT).
- `system.memory_low` (`free -m` available vs total, fail < 10%, high < 5%).
- `ssh.port_default` (sshd Port directive; default 22 -> low fail).
- `firewall.docker_published_sensitive_ports` (docker ps mappings on 0.0.0.0).
- `caddy.no_security_headers` (HSTS / X-Content-Type-Options / Referrer-Policy in Caddyfile).
- `caddy.invalid_config` (`caddy validate`).
- `tls.cert_missing` (openssl probe on `app.domain`, severity ramps with expiration).
- `secrets.weak_file_permissions` (find world-readable env/key/pem files).
- `monitoring.no_health_endpoint` (curl `https://<domain><healthcheckPath>`).

`system.fail2ban_inactive` covers the catalog's `ssh.fail2ban_missing` row;
treat them as the same check.

### Completion Checklist

- [x] `internal/readiness/extra_checks.go` implements the 10 checks with
  Skipped behavior on missing tooling, config, or domain.
- [x] `internal/readiness/extra_checks_test.go` covers happy / failed /
  skipped / severity-escalation paths for each check.
- [x] `Config` exposes `App.Domain` and `App.HealthcheckPath`; TLS and
  health checks read from there.
- [x] `doctor.go` runs the new checks (30 total, verified via local smoke).
- [x] `docs/check-catalog.md` and `README.md` reflect 30 checks and the new
  `app:` config block.
- [x] `gofmt`, `go vet ./...`, `go test ./...` pass.
- [x] Local `doctor --format json` enumerates 30 unique IDs.

### Follow-ups (intentionally out of scope)

- Cloudflare and compose checks (P1/P2 in `09-critique-and-deltas.md`).

## Current Slice - Harden Planner Coverage for P0 Pack

**Status:** Implemented
**Started:** 2026-05-02
**Plan sources:**

- previous slice: `P0 Check Pack (21 -> 30)`
- product direction: every doctor finding should map to a concrete action

### Scope

- Extend `harden/planner.go` with one Action per new P0 finding (9 actions;
  `ssh.fail2ban_missing` is already covered by `system.fail2ban_inactive`).
- Make `secrets.tighten-secret-perms` safe-auto-apply with one
  `chmod 600 <file>` per file reported by `secrets.weak_file_permissions`.
- Broaden `runChmod` allow-list from `.env` only to: `.env*`, `*.pem`,
  `*.key`, `id_rsa`, `id_ed25519` (new helper `isAllowedSecretPath`).
- Tolerate JSON-roundtripped evidence (`[]any` of `map[string]any`) for the
  Docker exposures field (`exposurePairs` helper).

### Completion Checklist

- [x] 9 new cases in `actionsFor` covering the P0 findings.
- [x] `secrets.tighten-secret-perms` is `SafeAutoApply: true` with chmod
  commands per file.
- [x] `isAllowedSecretPath` accepts env / pem / key / known SSH key names;
  rejects arbitrary files.
- [x] `exposurePairs` handles both in-memory and JSON-loaded evidence.
- [x] Existing chmod / ufw allow-list semantics unchanged for prior actions.
- [x] Planner tests cover the P0 pack mapping, the safe-auto-apply flag, and
  the JSON-roundtrip path.
- [x] Apply tests cover the broadened chmod allow-list (env variants, key
  files) and still reject arbitrary filenames.
- [x] `gofmt`, `go vet ./...`, `go test ./...` pass.

## Current Slice - Init Stack Presets

**Status:** Implemented
**Started:** 2026-05-02
**Plan sources:**

- `plans/09-critique-and-deltas.md` (livrable 4.3: init --preset)
- adoption signal: a fresh project should flag real issues, not stack noise

### Scope

- Add `--preset` flag to `shuttle init` covering `nextjs`, `laravel`,
  `node-api`, and `docker-swarm`.
- Generate an opinionated `.shuttle.yml` alongside the existing
  `shuttle.yml` when `--preset` is provided.
- Pre-fill `app.healthcheckPath`, `docker.workerServices`, and
  per-stack `checks.ignore` (e.g. drop Adminer noise on Next.js/Node API).
- Reuse the same `--force` flag to overwrite an existing `.shuttle.yml`.
- Reject unknown presets with a clear error listing supported values.

### Completion Checklist

- [x] `templates.ReadinessPresets`, `IsReadinessPreset`, `DeployShuttleYML`
  added; each preset parses as valid YAML with `version: 1`.
- [x] `init --preset <name>` writes `.shuttle.yml` with the supplied
  `--domain` (or a placeholder) baked in.
- [x] Without `--preset`, `init` keeps its previous behavior (only
  `shuttle.yml` written).
- [x] `--force` overrides existing `.shuttle.yml`; absence of `--force`
  refuses to clobber.
- [x] Unknown preset raises an actionable error.
- [x] Tests cover happy-path generation, force semantics, unknown preset, and
  YAML validity for all four presets.
- [x] README documents the four presets and their defaults.
- [x] `gofmt`, `go vet ./...`, `go test ./...` pass.

## Current Slice - P1 Check Pack (30 -> 38)

**Status:** Implemented
**Started:** 2026-05-02
**Plan sources:**

- `plans/09-critique-and-deltas.md` (P1 entries: compose suite + tls.hsts +
  dns.domain_not_pointing_to_server + db.no_backup_detected)

### Scope

- Add 8 new readiness checks:
  - `compose.missing_prod_file`, `compose.env_file_missing`,
    `compose.latest_tag_used`, `compose.no_resource_limits`,
    `compose.bind_mount_sensitive_paths` (single compose lookup,
    `/opt/shuttle/<app>/` aware).
  - `tls.hsts_missing` (HTTP HEAD probe on `app.domain`).
  - `dns.domain_not_pointing_to_server` (dig vs ipify/ifconfig.me).
  - `db.no_backup_detected` (artifact + cron heuristic, skipped without DB).
- Map each new finding to an Action in `harden/planner.go` (no auto-apply;
  every action is a notes-only human task).
- Bump README + check catalog count from 30 to 38, add new categories
  (DNS, Backups, Compose).

### Completion Checklist

- [x] `internal/readiness/compose_checks.go` implements compose lookup +
  5 checks; tag parsing handles `registry.local:5000/team/api:1.2.3`.
- [x] `internal/readiness/extra_checks.go` adds `checkHSTSHeader`,
  `checkDNSPointsToServer`, `checkDatabaseBackup` with proper Skipped paths.
- [x] `doctor.go` registers the 8 new checks (38 total, smoke test
  enumerates 38 IDs and surfaces real findings on the dev box).
- [x] `harden/planner.go` emits 8 new Action entries (compose.create-prod-file,
  compose.fix-env-file-references, compose.pin-image-tags,
  compose.add-resource-limits, compose.remove-sensitive-mounts,
  tls.enforce-hsts, dns.point-to-server, db.add-backup-job); none are
  SafeAutoApply.
- [x] `compose_checks_test.go` covers find / parse / pass / fail / skip
  per check, including the registry-with-port edge case.
- [x] `extra_checks_test.go` covers HSTS, DNS, and backup checks.
- [x] `planner_test.go` asserts the P1 mapping and that every P1 action is
  notes-only.
- [x] README + `docs/check-catalog.md` updated to 38 checks with new
  categories DNS / Backups / Compose.
- [x] `gofmt`, `go vet ./...`, `go test ./...` pass.

## Current Slice - Cloudflare Pack (38 -> 43)

**Status:** Implemented
**Started:** 2026-05-02
**Plan sources:**

- `plans/09-critique-and-deltas.md` (P2 entries: cloudflare.* checks)
- `plans/03-check-catalog.md` (cloudflare section)

### Scope

- Add a `Cloudflare` config block to `.shuttle.yml` (`enabled`, `zone`,
  `tokenEnv`).
- Implement a minimal Cloudflare REST client (`api.cloudflare.com/client/v4`)
  with `zoneID`, `setting`, `dnsRecords` methods and base URL override for
  tests.
- Add 5 Cloudflare-aware checks: ssl_flexible, always_https_disabled,
  waf_disabled (skipped on 404 plan-tier), dns_missing, proxy_disabled.
  `origin_exposed` deferred until we have a reliable origin-IP comparison.
- Resolve the API token from `CLOUDFLARE_API_TOKEN` by default with
  `cloudflare.tokenEnv` override; never log the token.
- Skip every Cloudflare check cleanly when disabled, missing zone, missing
  token, or auth error (401/403). Skip-summaries explain the cause.
- Map the 5 findings to notes-only Actions in `harden/planner.go`.

### Completion Checklist

- [x] `internal/readiness/cloudflare_checks.go` implements client + 5 checks.
- [x] `internal/readiness/cloudflare_checks_test.go` covers happy path,
  ssl_flexible failure, proxy_disabled flag, dns_missing fail, WAF 404
  skip, auth-error skip, env-token resolution.
- [x] `doctor.go` instantiates the client only when `cloudflare.enabled` and
  a token is present; otherwise checks skip with a clear cause.
- [x] `harden/planner.go` adds `cloudflare.upgrade-ssl-mode`,
  `cloudflare.enable-always-https`, `cloudflare.enable-waf`,
  `cloudflare.create-dns-record`, `cloudflare.enable-proxy` (all notes-only).
- [x] README documents the opt-in config block and the token env var.
- [x] `docs/check-catalog.md` has a Cloudflare section.
- [x] Smoke test: `doctor --format json` enumerates 43 unique check IDs and
  all Cloudflare checks skip with the disabled-by-config explanation.
- [x] `gofmt`, `go vet ./...`, `go test ./...` pass.

## Current Slice - Release Distribution Fixes

**Status:** Implemented
**Started:** 2026-05-02
**Plan sources:**

- previous slice: Cloudflare Pack
- adoption blocker found while reviewing the v1 release path

### Scope

Two real bugs that would surface on day one of v1, hidden behind "we'll
tag v1 later":

1. `scripts/build-go.sh` and the release workflow did not produce a
   `linux-arm64` binary. Hetzner CAX, AWS Graviton, Oracle Ampere, and
   Raspberry Pi 4/5 are core targets; without arm64 the install script
   errors out for them.
2. `action.yml` ran `curl install.sh | bash` then immediately invoked
   `shuttle --version`. The default install dir is
   `~/.local/bin`, which is not guaranteed on the runner's `$PATH`,
   especially on self-hosted runners. The next composite step would
   fail with "command not found" the first time anyone used the Action.

### Changes

- `scripts/build-go.sh` builds 4 targets now: linux-x64 + linux-arm64 +
  darwin-x64 + darwin-arm64.
- `scripts/install.sh` accepts linux-arm64 (drop the explicit error).
- `.github/workflows/release.yml` ships the new linux-arm64 binary in the
  GitHub release.
- `action.yml` installs into `/usr/local/bin` (using passwordless sudo on
  GitHub-hosted runners, direct write when running as root). The binary is
  always reachable in subsequent steps and from the caller's later workflow
  steps.
- README documents the four pre-built targets.

### Completion Checklist

- [x] `sh scripts/build-go.sh` produces 4 binaries; arm64 is verified as
  ELF aarch64 statically linked.
- [x] `sha256sum dist/* > dist/checksums.txt` covers all 4 binaries.
- [x] `sh -n scripts/install.sh` and `sh -n scripts/build-go.sh` parse.
- [x] `action.yml` and `release.yml` validate as YAML.
- [x] README mentions linux-arm64 as a supported target.
- [x] No regression on the existing Go test suite.

### Pending (operational, not code)

- Tag `v1` and let `release.yml` produce the first signed release.
- Smoke the Action against a freshly-tagged release in a sandbox repo.

## Current Slice - Re-gate Pro Features for Viral CLI

**Status:** Implemented
**Started:** 2026-05-02
**Plan sources:**

- `plans/09-critique-and-deltas.md` (pricing reframe in section 2.3)
- monetization decision: keep `doctor --target` free (the acquisition hook),
  monetize the deliverables (HTML/PDF reports) and the destructive
  operation (`harden --apply`).

### Scope

- Remove license gates on `doctor --target` (remote SSH scan) and `--config`
  (.shuttle.yml). These are the viral hook + the friction-reduction
  tool; gating them killed the funnel before the product could prove value.
- Keep license gates on:
  - `report --format html` (hosted/shareable client deliverable),
  - `report --format pdf` (white-labeled handoff for Agency tier),
  - `harden --apply` (destructive operation; Pro = "I trust DeployShuttle
    to write to my server").
- No change to free local doctor + Markdown report.

### New tier definition

| Feature | Free | Pro 29 EUR/mo | Agency 99 EUR/mo |
| --- | --- | --- | --- |
| `doctor` (local + --target) | yes | yes | yes |
| `--config .shuttle.yml` | yes | yes | yes |
| Console + Markdown report | yes | yes | yes |
| HTML report (hosted/shareable) | no | yes | yes |
| PDF report (white-label) | no | no | yes |
| `harden --apply` | no | yes | yes |
| Scheduled scans + email alerts | no | future | future |
| Multi-server + workspaces | no | no | future |

### Completion Checklist

- [x] Removed `license.Require` calls in doctor.go for --target and --config.
- [x] Removed unused `license` import in doctor.go.
- [x] Kept gates in report.go (html/pdf) and harden.go (--apply).
- [x] `gofmt`, `go vet ./...`, `go test ./...` pass.

### Pending (operational)

- Tag v1.0.1, force-rebuild release with the new gating.
- Provision license server + Stripe to actually issue Pro tokens
  (see plans/01-product-prd or the license-server README).

## Stop Note - 2026-05-02

Paused here intentionally.

Current repository state:

- Go CLI is the active implementation.
- TypeScript CLI is archived under `legacy/ts-cli/`.
- Dashboard and Astro docs were removed.
- `doctor` supports local and remote SSH scans.
- Remote SSH target tested against `root@<prod-vps>:7022`.
- Docker classic, Docker Swarm, and mixed single-VPS runtime detection are implemented.
- `.shuttle.yml` readiness config supports check ignores and Docker allowlists.
- Markdown and PDF local reports are implemented.
- React PDF renderer lives in `report-pdf/`.
- Latest report workflow is implemented with `doctor --output` and default `report` input.
- Report v1 polish is committed and pushed.

Last pushed commit:

```txt
6905aaa Polish readiness reports
```

Validation before stopping:

```bash
go test ./...
go vet ./...
bun run check
sh scripts/build-go.sh
ig index .
```

Recommended next slice for tomorrow:

```txt
Harden Dry-Run Planner
```

Goal:

- Add `shuttle harden --dry-run`.
- Do not mutate the server.
- Read latest doctor report by default from `.shuttle/latest-report.json`.
- Accept `--input <doctor.json>` and `--target user@host`.
- Convert findings into concrete proposed actions.

Initial dry-run actions:

- UFW proposals only when firewall findings exist.
- `.env` permission proposal for `secrets.env_world_readable`.
- Caddy admin localhost/internal-only proposal for `caddy.admin_exposed`.
- Docker healthcheck suggestions per workload.
- Docker non-root user suggestions per workload.
- Docker socket risk review for workloads mounting `/var/run/docker.sock`.
- Postgres/database bind localhost/private-network suggestion when DB binds public interfaces.

Safety rules:

- Default to dry-run.
- No destructive mutation.
- No automatic SSH changes in the first slice.
- Print commands/suggestions clearly and explain which finding generated each action.

## Current Slice - One-Time Pricing + Stripe One-Time Licensing

**Status:** Implemented
**Started:** 2026-07-02
**Plan sources:**

- `plans/05-cloud-pricing-launch.md` (section 2, decided pricing)
- user decision: single Pro tier, 199 EUR TTC one-time, Stripe checkout

### Scope

Align every pricing surface on the decided model (Free + Pro 199 EUR
one-time, perpetual license) and convert the license server from Stripe
subscriptions to a one-time Payment Link flow.

### Changes

- `docs-site/src/pages/pricing.astro`: removed Pro 29/mo, Agency 99/mo,
  Lifetime Early Bird, and the 99 EUR done-for-you audit blocks. Now two
  tiers only: Free + Pro 199 EUR one-time. Buy CTA reads
  `PUBLIC_STRIPE_PAYMENT_LINK` at build time (falls back to a
  "coming soon" chip when unset). FAQ updated (one-time, refunds link).
- `license-server/`: webhook now handles `checkout.session.completed`
  (mode `payment`, paid → perpetual license, `expires_at = null`) and
  `charge.refunded` (revoke). Removed `customer.subscription.updated` /
  `deleted` handling and the monthly/yearly price env vars. DB column
  `stripe_subscription_id` renamed to `stripe_payment_intent_id`,
  `stripe_customer_id` now nullable (Payment Links may not create a
  customer). README + `.env.example` updated. Activation/refresh JWT
  flow unchanged (14-day offline grace, perpetual entitlement).
- `plans/05`: section 2 rewritten as the decided pricing.
- `plans/10-vault-saas.md`: marked FROZEN, post-traction.
- `CLAUDE.md`: readiness check count corrected 21 -> 43.
- Note: the 2026-05-02 stop note above says "Astro docs were removed";
  `docs-site/` (Astro) has since been re-added and is the active landing
  + docs + pricing surface.

### Completion Checklist

- [x] license-server `bun run typecheck` passes.
- [x] license-server `bun test` passes (5 tests).
- [x] No pricing surface still mentions monthly/Agency tiers.

### Pending (operational, user-side)

- Create the Stripe product + one-time 199 EUR price + Payment Link.
- Create the webhook endpoint (`checkout.session.completed`,
  `charge.refunded`) and set `STRIPE_WEBHOOK_SECRET`.
- Deploy license-server (Fly.io) + set `PUBLIC_STRIPE_PAYMENT_LINK` at
  docs-site build time.
- Deploy docs-site on the production domain (client.go defaults to
  `https://license.deployshuttle.io` — the domain must be owned).

## Current Slice - Monetization Chain E2E (mocked Stripe, iso Spin Pro)

Date: 2026-07-02. Goal: make the whole purchase→activation→revocation funnel
testable locally without a Stripe account, and close the remaining Spin-Pro
UX gaps (interactive Pro onboarding, post-purchase page).

- `license-server`: dev license emails can be delivered to Mailpit via its
  HTTP send API (`MAILPIT_URL`, dev only — precedence over Resend); webhook
  integration tests against a real throwaway Postgres
  (`TEST_DATABASE_URL=… bun test`, skipped otherwise).
- `stripe-mock/` (new top-level, dev-only, never deployed): fake Payment
  Link page + HMAC-signed `checkout.session.completed` / `charge.refunded`
  webhooks. The license-server code is unchanged — the mock shares
  `STRIPE_WEBHOOK_SECRET` and signs with Stripe's real scheme.
- `docs-site`: `/thank-you` + `/fr/thank-you` post-purchase page (noindex);
  Stripe Payment Link confirmation will redirect there.
- `go-cli`: `init --pro` is now an interactive onboarding wizard (DB engine,
  Redis, queue, scheduler, Mailpit, CI). Explicit `--with-*` flags act as
  answers; non-TTY/EOF stdin falls back to the historical auto-enable set,
  so scripted usage is byte-identical. License gate runs before questions.
- `scripts/e2e-license.sh` (`make e2e-license`): full A→Z run against
  infra-postgres/infra-mailpit — purchase → key extracted from the Mailpit
  email → gated CLI build (ldflags pubkey) → gate closed/open assertions →
  refund → revoked refresh. Re-runnable; isolated ports (:3999/:4299),
  throwaway DB, temp `SHUTTLE_HOME`.
- `plans/11-go-live-checklist.md`: production mirror of the mocked chain
  (Stripe live, Neon, Fly.io, Resend, GH secrets, DNS, live smoke test).
  Documented, not executed.

### Completion Checklist

- [x] `make test` green (go + license-server, webhook tests skip without pg).
- [x] Webhook integration tests pass against Postgres (10 pass).
- [x] `make e2e-license` passes twice in a row (idempotent).
- [x] `init --pro` wizard covered by tests (answers, EOF defaults, explicit flags).

### Pending (operational, user-side)

- Everything in `plans/11-go-live-checklist.md` (Stripe live, Neon, Fly,
  Resend, GH secrets, DNS, live smoke test).
