# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Shuttle is an open-source Docker deployment CLI (TypeScript/Bun) that provisions VPS servers and deploys containerized apps via SSH. Think Kamal/Dokku but pluggable. Config lives in `shuttle.yml`.

## Commands

```bash
bun run dev -- <command>       # run CLI locally (e.g. bun run dev -- deploy --dry-run)
bun test                       # unit tests (bun native runner)
bun test test/unit/core/deploy-manager.test.ts   # single test file
bun test --watch               # watch mode
SHUTTLE_INTEGRATION=1 bun test test/integration  # integration tests (need live SSH)
bunx biome check .             # lint + format check
bunx biome check --write .     # lint + autofix
bunx tsc --noEmit              # typecheck
```

CI runs: `bun install --frozen-lockfile` → lint → typecheck → `bun test`.

## Architecture

### CLI Layer (`src/cli/`)
Each file exports a `citty` `defineCommand`. Root command (`cli/index.ts`) lazy-imports subcommands. Commands: `init`, `provision`, `deploy`, `rollback`, `destroy`, `logs`, `ssh`, `status`, `exec`, `lock`, `secrets` (set/get/list/remove/push), `license`, `validate`.

### Core Layer (`src/core/`)
Business logic managers injected via `src/container.ts` (constructor DI with module-level singleton fallbacks):
- **DeployManager** — orchestrates blue-green (12 steps) or rolling (8 steps) deploys
- **SSHManager** — connection pool, exec, SFTP upload, interactive shell
- **DockerManager** — local build/save, remote run/stop/tag/prune
- **RuntimeManager** — remote state at `/opt/shuttle/<app>/` (state.json, deploying.json, lock dir)
- **ProvisionManager** — 9-step VPS bootstrap (Debian/Ubuntu)
- **ProxyManager**, **SecretsManager**, **AccessoryManager**, **RollbackManager**, **DestroyManager**, **NotificationsManager**

### Provider Layer (`src/providers/`)
Pluggable drivers behind interfaces in `providers/types.ts`, resolved by `providers/resolver.ts`:
- **ProxyProvider**: caddy
- **RegistryProvider**: local-transfer, ghcr, docker-hub, image-ref
- **SecretsProvider**: aes (AES-256-GCM)
- **DeployStrategy**: blue-green, rolling

Premium providers are gated by `license/gate.ts` → `requirePremium()`.

### Config Layer (`src/config/`)
- `schema.ts` — full Zod schema for `shuttle.yml` (`server:` shorthand normalized to `servers:` map)
- `loader.ts` — walks up from CWD, supports env overlays (`shuttle.<env>.yml` or `SHUTTLE_ENV`), deep merges
- `defaults.ts` — default values (blue-green, 120s timeout, 5 retained, Caddy proxy, AES secrets)

### License System (`src/license/`)
Ed25519 JWT tokens verified client-side. Resolution: `SHUTTLE_LICENSE_KEY` env → `~/.shuttle/license` file.

## Key Patterns

- **Errors**: `ShuttleError` hierarchy (`ConfigError`, `SSHError`, `DeployError`, etc.) with `.wrap(err, msg)` static factory
- **Logging**: `consola`-based `logger` with `step(n, total, msg)` and verbose mode
- **Shell safety**: all remote paths through `shellEscape()` (POSIX single-quote wrapping)
- **Port assignment**: `BASE_PORT(10000) + serviceIndex * 2 + slot_offset` (blue=even, green=odd)
- **Image tags**: `shuttle/<app>:deploy-<YYYYMMDD>-<shortsha>`
- **Remote state**: JSON files at `/opt/shuttle/<app>/`
- **Path alias**: `@/*` → `./src/*`

## Style

- Biome: tabs, 100-char lines, single quotes, no semicolons
- ESM only (`"type": "module"`)
- Strict TypeScript

## Repo Layout

- `src/` — CLI source (the product)
- `test/unit/` — unit tests (mirrors `src/` structure)
- `test/integration/` — integration tests (require live SSH)
- `test/helpers/` — factories and mocks (mock-ssh, mock-docker, config-factory)
- `scripts/` — dev tooling (keypair generation, license signing)
- `docs/` — **separate Astro app** (marketing site + admin backoffice, uses npm, NOT bun)
