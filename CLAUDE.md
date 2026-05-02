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
deploy-shuttle doctor --target root@server
```

Existing deploy/provision features must be preserved. Reframe deployment as the natural
continuation after readiness checks, not as the only product promise.

Current config lives in `shuttle.yml`. The planned readiness config is `.deployshuttle.yml`;
do not treat it as implemented until code and docs explicitly support it.

The product plan lives in `plans/`. Use it as the source of truth for PRD, MVP scope,
check catalog, command design, scoring, architecture, and launch direction. The previous
TypeScript/Bun implementation lives in `legacy/ts-cli/` and is reference-only.

## Commands

```bash
cd go-cli
go run ./cmd/deploy-shuttle --help
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
  relevant file under `plans/`.
- When user-facing behavior changes, update the relevant docs once a docs surface exists,
  or note clearly why docs were not changed.
- Do not document planned features as implemented. Use "planned", "MVP target", or
  "future" when code support does not exist yet.
- Preserve existing deploy-related code unless the task explicitly asks to remove it.

## Architecture

### CLI Layer (`go-cli/internal/cli/`)

The Go CLI uses Cobra. Root command is created by `internal/cli/root.go`.

Current commands include:

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
- `secrets` (`set`, `get`, `list`, `remove`, `push`)
- `license`
- `validate`
- `ci`
- `monitor`
- `doctor` (local readiness scan; remote `--target` is planned)
- `report`
- `harden` (dry-run plan + safe apply, local or over SSH)

Planned readiness commands from `plans/`:

- `harden` action coverage beyond `chmod 600 .env` (UFW baseline, Caddy admin, etc.)

Do not mention these planned commands as implemented until corresponding CLI files,
tests, and docs exist.

### Core Layer (`go-cli/internal/`)
- `config/` — YAML loader, defaults, env overlays, `server` to `servers` normalization
- `readiness/` — `doctor`, check results, scoring, console/JSON reports
- `harden/` — dry-run planner mapping doctor findings to proposed actions
- `ssh/` — SSH command execution
- `execx/` — local shell adapter
- `runtime/` — remote path helpers under `/opt/shuttle/<app>/`
- `secrets/` — local secret store for CLI parity

### Legacy TS (`legacy/ts-cli/`)
Reference implementation only. Do not add new product work there unless explicitly asked.

## Key Patterns

- **CLI framework**: Cobra command constructors live in `go-cli/internal/cli/`.
- **Config**: `config.Load(path, env)` resolves `shuttle.yml`, defaults, and optional env overlays.
- **Shell safety**: use `shell.Escape()` for values interpolated into remote shell commands.
- **Secrets**: local secrets use a passphrase-protected envelope with Argon2id and XChaCha20-Poly1305 in `.shuttle/secrets.enc`; CI/non-interactive shells must set `DEPLOY_SHUTTLE_SECRETS_PASSPHRASE`.
- **Remote paths**: runtime helpers keep app state under `/opt/shuttle/<app>/`.
- **Readiness checks**: add doctor checks in `go-cli/internal/readiness/` and keep scoring deterministic.
- **Compatibility**: old TS behavior is reference material only; if Go behavior is intentionally partial, document that clearly in `README.md` or `plans/08-execution-tracker.md`.

## Style

- Go 1.23
- `gofmt` before final checks
- Keep command behavior compatible with `shuttle.yml`

## Repo Layout

- `go-cli/` — active Go CLI
- `go-cli/cmd/deploy-shuttle/` — main package
- `go-cli/internal/` — internal CLI, config, readiness, SSH, templates, runtime, secrets packages
- `legacy/ts-cli/` — archived TypeScript/Bun implementation
- `scripts/` — release/build tooling
- `plans/` — product pivot plans and PRD split into Markdown parts
- `.shuttle/` — local Shuttle workspace/state placeholder

When this layout changes, update this section immediately and rebuild the `ig` index.
