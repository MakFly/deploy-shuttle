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

Implement the first `deploy-shuttle doctor` foundation:

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
- `.deployshuttle.yml` support;
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
- product hook: `deploy-shuttle doctor --target root@server`

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

- Add `.deployshuttle.yml` readiness config loading.
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

- Add `deploy-shuttle report`.
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
- Let `report` default to `.deployshuttle/latest-report.json`.
- Keep `report --input <path>` for explicit JSON input.
- Create parent directories for doctor output.

### Completion Checklist

- [x] `doctor --output .deployshuttle/latest-report.json` writes JSON.
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

- Add `deploy-shuttle harden --dry-run`.
- Read latest doctor JSON from `.deployshuttle/latest-report.json` by default.
- Accept `--input <doctor.json>`, `--target user@host`, and `--format console|json`.
- Convert failed, non-ignored findings into concrete proposed actions.
- Group actions by category and surface source check ID + severity.
- Do not mutate any local or remote system in this slice.

### Completion Checklist

- [x] `harden/planner.go` maps known finding IDs to actions with rationale, commands, and notes.
- [x] `harden/render.go` prints a grouped, dry-run-labelled console plan.
- [x] CLI command requires `--dry-run` and refuses any execution path.
- [x] Default input falls back to `.deployshuttle/latest-report.json`.
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

- [x] `report --format html --output deployshuttle-report.html` writes HTML.
- [x] HTML defaults output path to `deployshuttle-report.html`.
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
- New `internal/license` package: Ed25519 token format, store at `~/.deployshuttle/license.json` (0600), HTTP client for activate/refresh, machine fingerprint, `Require(feature)` helper.
- Refactor `license` CLI command: `activate`, `status`, `refresh`, `deactivate` against the new client.
- Gate Pro features: `doctor --target`, `doctor --config`, `report --format html|pdf`, `harden --apply`.
- `scripts/build-go.sh` accepts `LICENSE_PUBKEY_B64`, `LICENSE_SERVER`, `DEPLOY_SHUTTLE_VERSION` and injects via ldflags.

### Completion Checklist

- [x] `internal/license/token.go` implements Ed25519-signed JWT-compatible tokens with sign/verify pair.
- [x] `internal/license/store.go` reads/writes JSON license state under `DEPLOY_SHUTTLE_HOME` (defaults to `~/.deployshuttle`).
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

## Stop Note - 2026-05-02

Paused here intentionally.

Current repository state:

- Go CLI is the active implementation.
- TypeScript CLI is archived under `legacy/ts-cli/`.
- Dashboard and Astro docs were removed.
- `doctor` supports local and remote SSH scans.
- Remote SSH target tested against `root@<prod-vps>:7022`.
- Docker classic, Docker Swarm, and mixed single-VPS runtime detection are implemented.
- `.deployshuttle.yml` readiness config supports check ignores and Docker allowlists.
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

- Add `deploy-shuttle harden --dry-run`.
- Do not mutate the server.
- Read latest doctor report by default from `.deployshuttle/latest-report.json`.
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
