# DeployShuttle

DeployShuttle is a Go CLI-first VPS production readiness and deployment assistant for Docker apps.

```txt
Audit, harden and deploy Docker apps on VPS.
```

The project was migrated from the original TypeScript/Bun CLI to a Go CLI. The TS implementation is archived in `legacy/ts-cli/` as a migration reference.

The MVP hook is:

```bash
deploy-shuttle doctor
```

`doctor` currently supports local scans with console/JSON output and deterministic scoring.
Remote `doctor --target user@host`, `report`, and `harden` are planned next steps documented in
[`plans/`](./plans/).

## Current State

Implemented today:

- `init`
- `new`
- `dev`
- `provision`
- `deploy`
- `rollback`
- `destroy`
- `logs`
- `ssh`
- `status`
- `exec`
- `lock`
- `secrets`
- `license`
- `validate`
- `ci`
- `monitor`
- `doctor` (local scan)

Compatibility notes:

- `rollback` is exposed but still guarded until the Go port has persisted blue/green state parity.
- `secrets` uses a passphrase-protected envelope: Argon2id KDF + XChaCha20-Poly1305 authenticated encryption. Use `DEPLOY_SHUTTLE_SECRETS_PASSPHRASE` in CI/non-interactive shells.
- `deploy` is intentionally minimal while the product pivot prioritizes `doctor`.

Current readiness flow:

```bash
deploy-shuttle doctor
deploy-shuttle doctor --format json
deploy-shuttle doctor --fail-below 75
deploy-shuttle doctor --target root@server --output .deployshuttle/latest-report.json
deploy-shuttle report --format pdf --output report.pdf
deploy-shuttle report --input doctor.json --format markdown --output report.md
deploy-shuttle report --input doctor.json --format pdf --output report.pdf
```

Planned readiness flow:

```bash
deploy-shuttle doctor --target root@203.0.113.10
deploy-shuttle report --format html --output report.html
deploy-shuttle harden --target root@203.0.113.10 --dry-run
```

## Why

Developers can often get Docker apps running on low-cost VPS providers, but production quality is inconsistent:

- no firewall;
- no restart policy;
- no healthcheck;
- exposed database ports;
- weak `.env` permissions;
- missing backups;
- Caddy or TLS mistakes;
- no readiness score before client handoff.

DeployShuttle should own the gap between:

```txt
It works.
```

and:

```txt
It is production-ready.
```

## Development

```bash
cd go-cli
go test ./...
go vet ./...
go run ./cmd/deploy-shuttle --help
```

Build release binaries from the repository root:

```bash
sh scripts/build-go.sh
```

PDF reports use the optional React PDF renderer:

```bash
cd report-pdf
bun install
bun run check
```

## Product Plan

Read [`plans/README.md`](./plans/README.md) first.

Primary implementation order:

1. Keep the repository lean: CLI, tests, and plans.
2. Add remote SSH support for `deploy-shuttle doctor --target user@host`.
3. Expand the check catalog toward the 15-check MVP.
4. Add report generation.
5. Add conservative `harden --dry-run`.
6. Treat deployment as the continuation after readiness checks.
