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
- `report`, `harden`, and remote `doctor --target` remain planned readiness work.
