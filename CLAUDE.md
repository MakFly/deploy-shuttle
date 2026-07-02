# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

DeployShuttle is a Go CLI moving from a generic VPS deployment tool to a CLI-first VPS
production readiness product.

Current product direction:

```txt
Audit, harden and deploy Docker apps on VPS.
```

Primary hook:

```bash
shuttle doctor --target root@server
```

Existing deploy/provision features must be preserved. Reframe deployment as the natural
continuation after readiness checks, not as the only product promise.

Current config lives in `shuttle.yml`. The readiness config is `.shuttle.yml`.

The historical product plan (`plans/`) was removed from the tree (2026-07-02); it
remains available in git history only, like the previous TypeScript/Bun implementation.
Decided facts that survive it: single Pro tier at 199€ TTC one-time (no Team/Agency
tier, no subscription), perpetual license, checkout via Stripe Payment Link.

## Commands

```bash
cd go-cli
go run ./cmd/shuttle --help
go test ./...
go vet ./...
gofmt -w .
sh ../scripts/build-go.sh
```

CI runs Go only: `gofmt` check → `go vet ./...` → `go test ./...`.

## Search and Project Index

Prefer `ig` over `rg` or `grep` for code search.

```bash
ig index .                     # rebuild project index after structural changes
ig "pattern" .                 # code search
ig read <file> --signatures    # imports and function signatures only
ig smart <path>                # short summaries
```

After adding, moving, deleting, or renaming files, run `ig index .` before finishing the
turn so future agents see the current project shape.

If `.ig/context.md` exists, read it first for the project map. If it does not exist, use
`find`, `ig`, and the repo layout below.

## Structure Sync Rules

Always keep working context aligned with the real repository structure:

- Before non-trivial edits, inspect the relevant files and nearby tests.
- When adding a new top-level directory or changing major ownership boundaries, update
  this `CLAUDE.md` repo layout section in the same turn.
- When product direction, roadmap, command design, or check catalog changes, update the
  relevant doc under `docs/` (the former `plans/` directory is git history only).
- When user-facing behavior changes, update the relevant docs once a docs surface exists,
  or note clearly why docs were not changed.
- Do not document planned features as implemented. Use "planned", "MVP target", or
  "future" when code support does not exist yet.
- Preserve existing deploy-related code unless the task explicitly asks to remove it.

## Architecture

### CLI Layer (`go-cli/internal/cli/`)

The Go CLI uses Cobra. Root command is created by `internal/cli/root.go`.

Current commands include:

- `init` (supports `--pro`, `--with-db`, `--with-redis`, `--with-queue`, `--with-scheduler`, `--with-mailpit` for Pro templates)
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
- `secrets` (`set`, `get`, `list`, `remove`, `push`)
- `license`
- `validate`
- `ci`
- `monitor`
- `doctor` (local readiness scan; remote `--target` is planned)
- `report`
- `harden` (dry-run plan + safe apply, local or over SSH)

Planned readiness commands:

- `harden` action coverage beyond `chmod 600 .env` (UFW baseline, Caddy admin, etc.)

Do not mention these planned commands as implemented until corresponding CLI files,
tests, and docs exist.

### Core Layer (`go-cli/internal/`)
- `config/` — YAML loader, defaults, env overlays, `server` to `servers` normalization
- `readiness/` — `doctor`, check results, scoring, console/JSON reports (43 checks: system, SSH, Docker/compose, firewall, secrets, reverse-proxy/database, Cloudflare)
- `harden/` — dry-run planner mapping doctor findings to proposed actions
- `ssh/` — SSH command execution
- `execx/` — local shell adapter
- `runtime/` — remote path helpers under `/opt/shuttle/<app>/`
- `secrets/` — local secret store for CLI parity

## Key Patterns

- **CLI framework**: Cobra command constructors live in `go-cli/internal/cli/`.
- **Config**: `config.Load(path, env)` resolves `shuttle.yml`, defaults, and optional env overlays.
- **Shell safety**: use `shell.Escape()` for values interpolated into remote shell commands.
- **Secrets**: local secrets use a passphrase-protected envelope with Argon2id and XChaCha20-Poly1305 in `.shuttle/secrets.enc`; CI/non-interactive shells must set `SHUTTLE_SECRETS_PASSPHRASE`.
- **Remote paths**: runtime helpers keep app state under `/opt/shuttle/<app>/`.
- **Readiness checks**: add doctor checks in `go-cli/internal/readiness/` and keep scoring deterministic.
- **Pro templates**: `init --pro` runs an interactive onboarding wizard (DB engine, Redis, queue, scheduler, Mailpit, CI) in `internal/cli/init_pro_wizard.go`; explicit `--with-*` flags act as answers and skip their prompt, non-TTY/EOF stdin falls back to the full default set. Gated by `license.Require("init --pro")` before the questions. Service blocks in `templates/compose_services.go`, assembly in `templates/compose_pro.go`.
- **Dev email**: the license-server delivers license emails to Mailpit when `MAILPIT_URL` is set (dev only — precedence over Resend, never set in prod).
- **Monetization E2E**: `make e2e-license` runs the full purchase→email→activate→refund chain locally against `infra-postgres`/`infra-mailpit` using `stripe-mock/`; the go-live checklist is in git history (`plans/11-go-live-checklist.md`).
- **Pricing**: 199€ TTC one-time, single Pro tier. No Agency tier.
- **Compatibility**: the old TS CLI was removed (git history only); if Go behavior is intentionally partial, document that clearly in `README.md`.

## Style

- Go 1.23
- `gofmt` before final checks
- Keep command behavior compatible with `shuttle.yml`

## Repo Layout

- `go-cli/` — active Go CLI
- `go-cli/cmd/shuttle/` — main package
- `go-cli/internal/` — internal CLI, config, readiness, SSH, templates, runtime, secrets, license packages
- `docs-site/` — Astro landing + docs + pricing site (Bun)
- `license-server/` — Stripe one-time checkout webhook + license issuer (Bun/Hono, Fly.io)
- `stripe-mock/` — dev-only fake Stripe (checkout page + HMAC-signed webhooks); never deployed
- `report-pdf/` — React PDF renderer for `report --format pdf`
- `marketing/` — launch post drafts
- `docs/` — check catalog reference
- `scripts/` — release/build tooling
- `.shuttle/` — local Shuttle workspace/state placeholder
- `Makefile` — dev shortcuts (docs site, tests, build, stripe-mock, e2e-license)

When this layout changes, update this section immediately and rebuild the `ig` index.
