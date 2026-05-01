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
- Validate against `root@185.158.107.49:7022`.

### Completion Checklist

- [x] Postgres `0.0.0.0:5432` includes process evidence.
- [x] Firewall-restricted DB listener is no longer treated as critical Internet exposure.
- [x] Remediation explains API/Adminer private-network or allowlist access.
- [x] Adminer check detects Caddy IP restriction + deny-by-default + basic auth.
- [x] Real VPS scan returns `90/100` with Adminer passed and DB warning high.
- [x] Unit tests cover firewall-restricted and publicly allowed DB port cases.
