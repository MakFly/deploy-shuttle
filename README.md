<div align="center">

# Shuttle

**Audit, harden and deploy Docker apps on VPS**

One CLI to go from code to production on any VPS.
Security scoring, zero-downtime deploys, Docker Swarm native.

[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/license-proprietary-blue)](#license)
[![GitHub Action](https://img.shields.io/badge/action-MakFly%2Fdeploy--shuttle-2088FF?logo=githubactions&logoColor=white)](#github-action)

```bash
# Init -> Provision -> Deploy
shuttle init
shuttle provision
shuttle deploy
```

</div>

---

## Table of Contents

- [Why Shuttle](#why-shuttle)
- [Features](#features)
- [Quick Start](#quick-start)
  - [Install](#install)
  - [From Code to Production](#from-code-to-production)
  - [Pro Templates](#pro-templates)
  - [Audit an Existing Server](#audit-an-existing-server)
- [Deploy Strategies](#deploy-strategies)
- [Secrets Management](#secrets-management)
- [Supported Stacks](#supported-stacks)
- [CI / CD Integration](#cicd-integration)
- [Configuration](#configuration)
- [Check Catalog](#check-catalog)
- [Pricing](#pricing)
- [Commands](#commands)
- [Architecture](#architecture)
- [Supported Platforms](#supported-platforms)
- [Development](#development)
- [Versioning](#versioning)
- [License](#license)

---

## Why Shuttle

You can ship a Docker app to a **$5 VPS** in an afternoon -- but the gap between "it works" and "it's production-ready" is a maze of small, easy-to-miss security and reliability issues:

| Problem | Risk |
|---|---|
| No firewall, or a firewall that allows everything | Full server exposure |
| Public database ports (Postgres, Redis, MongoDB) | Data breach |
| Containers running as root or mounting the Docker socket | Privilege escalation |
| `.env` files world-readable or tracked by Git | Credential leak |
| Caddy admin API exposed to the internet | Remote takeover |
| Workloads with no restart policy or healthcheck | Silent downtime |
| Manual deploys with `docker compose up -d` | Downtime on every push |

**Shuttle closes that gap.** It detects your stack, generates production Dockerfiles, provisions a VPS with Docker Swarm and Caddy, deploys with zero-downtime rolling updates, and gives you a 43-check security score you can hand to a client.

---

## Features

### Production Readiness

- **43 automated security and reliability checks** -- system, SSH, Docker, firewall, secrets, reverse proxy, TLS, DNS, monitoring, backups, compose, Cloudflare
- **Deterministic scoring** -- 0-100 readiness score with severity-weighted deductions (critical / high / medium / low / info)
- **Local or remote scan** -- same check suite runs on your machine or over SSH with a single `--target` flag
- **Shareable reports** -- Markdown, self-contained HTML, or PDF output for client handoffs, PRs, or audits
- **Dry-run hardening** -- concrete remediation plan with ready-to-run commands; `--apply` only executes safe, idempotent actions
- **CI gate** -- fail pipelines when the score drops below a threshold; first-class GitHub Action included

### Deployment

- **3 strategies** -- Swarm (rolling updates), Compose, Blue-Green
- **Docker Swarm native** with Caddy reverse proxy and auto-TLS via Let's Encrypt
- **Zero-downtime deploys** via start-first rolling updates
- **FrankenPHP production Dockerfiles** for Laravel (Octane) and Symfony (worker mode)
- **Docker Secrets** -- encrypted at rest in Swarm Raft log, RAM-only in containers
- **Rollback support** for all strategies
- **Caddy SIGUSR1 hot-reload** for instant upstream switching (blue-green)

### Pro Templates

- **Full production stack in one flag** -- `shuttle init --pro` generates a multi-service Docker Compose with database, Redis, queue workers, scheduler, and mail trap
- **Composable** -- pick exactly what you need: `--with-db postgres`, `--with-redis`, `--with-queue`, `--with-scheduler`, `--with-mailpit`
- **Laravel-optimized** -- Queue worker, Scheduler, Horizon (when Redis is enabled), all pre-configured with resource limits and healthchecks
- **Symfony-optimized** -- Messenger consumer, Scheduler via `messenger:consume scheduler_default`, pre-wired Redis transport
- **Pre-wired `.env.example`** -- service hostnames, ports, and connection strings filled in (`DB_HOST=postgres`, `REDIS_HOST=redis`)
- **CI/CD included** -- Pro GitHub Actions workflow with database service containers for integration tests
- **One-time purchase** -- 199 EUR, unlimited projects and servers

### Developer Experience

- **Interactive `shuttle init`** -- detects your stack (Laravel, Symfony, Next.js, Node API), generates Dockerfile + docker-compose.yml + .dockerignore + .env.example
- **`shuttle provision`** -- bootstraps a bare VPS with Docker Swarm + Caddy + UFW in one command
- **Auto-update checks** on startup with `shuttle update`
- **Score badge** for README with `shuttle badge`
- **Stack presets** -- opinionated configs for common stacks, fewer false positives on day one
- **Cloudflare guardrails** -- opt-in SSL mode, WAF, DNS, and proxy checks against your Cloudflare zone
- **Zero dependencies** -- single static Go binary, no runtime, no Docker required on the auditor side

---

## Quick Start

### Install

```bash
curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh
```

Downloads the latest binary for your OS and architecture into `~/.local/bin`.

Pre-built targets: `linux-x64`, `linux-arm64` (Hetzner CAX, AWS Graviton, Raspberry Pi 4/5), `darwin-x64`, `darwin-arm64`.

Or download manually from [Releases](https://github.com/MakFly/deploy-shuttle/releases) and place it on your `$PATH`.

### From Code to Production

```bash
cd my-laravel-app

# 1. Init -- detects Laravel, generates Dockerfile + docker-compose.yml + .dockerignore
shuttle init

# 2. Provision -- bootstraps VPS (Docker Swarm + Caddy + UFW)
shuttle provision

# 3. Deploy -- build, push, rolling update
shuttle deploy

# -> https://my-app.fr is live with TLS
```

### Pro Templates

Generate a full production stack with services, workers, and CI/CD in one command:

```bash
cd my-laravel-app

# Full stack: Postgres + Redis + Queue + Scheduler + Horizon + Mailpit + CI
shuttle init --pro

# Or pick individual services:
shuttle init --with-db postgres --with-redis

# MySQL instead of Postgres:
shuttle init --pro --with-db mysql
```

**What `--pro` generates for Laravel:**

```yaml
services:
  web:        # FrankenPHP + Octane, healthcheck, resource limits
  postgres:   # PostgreSQL 16, named volume, pg_isready healthcheck
  redis:      # Redis 7 with AOF persistence, redis-cli ping healthcheck
  queue:      # php artisan queue:work --tries=3 --timeout=90
  scheduler:  # php artisan schedule:work
  horizon:    # php artisan horizon (when Redis is enabled)
  mailpit:    # SMTP trap on :1025, web UI on :8025
```

**What `--pro` generates for Symfony:**

```yaml
services:
  web:        # FrankenPHP worker mode, healthcheck, resource limits
  postgres:   # PostgreSQL 16, named volume, pg_isready healthcheck
  redis:      # Redis 7 with AOF persistence
  messenger:  # php bin/console messenger:consume async --time-limit=3600
  scheduler:  # php bin/console messenger:consume scheduler_default
  mailpit:    # SMTP trap on :1025, web UI on :8025
```

All Pro services include `depends_on` with health conditions, resource limits, named volumes, and a shared `app-network`. The `.env.example` is pre-filled with service hostnames (`DB_HOST=postgres`, `REDIS_HOST=redis`, `MAIL_HOST=mailpit`).

Pro flags require a [Shuttle Pro license](#pricing). Dev builds (`go build` without ldflags) bypass the gate.

### Audit an Existing Server

```bash
# Remote scan over SSH:
shuttle doctor --target root@203.0.113.10

# Generate a client-facing HTML report:
shuttle report --format html --output report.html

# Plan and apply hardening:
shuttle harden --apply --target root@203.0.113.10 --yes
```

**Example output:**

```text
Shuttle Doctor Report
Target: root@203.0.113.10:7022
Score: 70/100 - Risky

High:
  [x] Sensitive database ports are not public - 5432 publicly bound (UFW restricted).
  [x] Docker socket is not exposed to workloads - 2 workloads mount /var/run/docker.sock.

Medium:
  [x] Docker workloads have healthchecks - 4 workloads lack healthchecks.
  [x] Docker workloads do not run as root - 15 workloads run as root.
```

---

## Deploy Strategies

### Swarm (default)

```yaml
deploy:
  strategy: swarm
```

- Docker stack deploy with rolling updates
- start-first (zero-downtime)
- auto-rollback on failure
- `shuttle rollback --yes` for manual rollback

### Blue-Green

```yaml
deploy:
  strategy: blue-green
```

- Two slots (blue/green) alternating
- Caddy upstream switch via SIGUSR1 hot-reload
- Instant rollback (slot swap)

### Compose

```yaml
deploy:
  strategy: compose
```

- Simple `docker compose up`
- Best for staging/dev environments
- State tracking + rollback

---

## Secrets Management

### Docker Secrets (Swarm)

```bash
shuttle secrets set APP_KEY "base64:xxx"
shuttle secrets set DB_PASSWORD "secret"
shuttle secrets push
# -> Encrypted in Swarm Raft log, RAM-only in containers
```

Secrets are stored encrypted locally with Argon2id + XChaCha20-Poly1305, then pushed as Docker Secrets to the Swarm cluster. Containers read them from `/run/secrets/` -- never written to disk on the host.

### Env File Split (Compose / Blue-Green)

```bash
# .env        -> config values (committed)
# .env.secrets -> secrets (chmod 600, never committed)
```

The `doctor` check suite verifies `.env` permissions and Git tracking automatically.

---

## Supported Stacks

| Stack | Dockerfile | Worker Mode | Health Endpoint |
|---|---|---|---|
| Laravel | FrankenPHP + Octane | Octane workers | `/up` |
| Symfony | FrankenPHP native | `php_server` worker | Caddy metrics |
| Next.js | Node 22 standalone | -- | `/` |
| Node API | Custom | -- | `/health` |

`shuttle init` detects the stack from your project files and generates the appropriate Dockerfile, docker-compose.yml, .dockerignore, and .env.example.

### Pro Services per Stack

| Service | Laravel | Symfony | Next.js / Node |
|---|---|---|---|
| PostgreSQL 16 | `--with-db postgres` | `--with-db postgres` | `--with-db postgres` |
| MySQL 8.4 | `--with-db mysql` | `--with-db mysql` | `--with-db mysql` |
| Redis 7 (AOF) | `--with-redis` | `--with-redis` | `--with-redis` |
| Queue worker | `queue:work` | `messenger:consume` | -- |
| Scheduler | `schedule:work` | `scheduler_default` | -- |
| Horizon | `horizon` (auto with Redis) | -- | -- |
| Mailpit | `:1025` + `:8025` | `:1025` + `:8025` | `:1025` + `:8025` |
| CI/CD workflow | PHP + Composer + DB | PHP + Composer + DB | Node + DB |

Queue and scheduler flags (`--with-queue`, `--with-scheduler`) are only available for Laravel and Symfony. Database, Redis, and Mailpit work with all stacks.

---

## CI/CD Integration

### GitHub Action

```yaml
# .github/workflows/readiness.yml
jobs:
  doctor:
    runs-on: ubuntu-latest
    steps:
      - uses: MakFly/deploy-shuttle@v1
        with:
          target: ${{ secrets.SSH_TARGET }}
          ssh-private-key: ${{ secrets.SSH_PRIVATE_KEY }}
          fail-below: '80'
```

**Action inputs:**

| Input | Required | Default | Description |
|---|---|---|---|
| `target` | no | -- | SSH target (`user@host` or `user@host:port`). Omit to scan the runner. |
| `ssh-private-key` | no | -- | SSH private key (PEM) for remote scans. |
| `ssh-known-hosts` | no | -- | Known hosts content. Auto-detected via `ssh-keyscan` when omitted. |
| `config` | no | -- | Path to `.shuttle.yml`. |
| `fail-below` | no | `75` | Fail the job when the score is strictly below this threshold. |
| `output` | no | `.shuttle/latest-report.json` | JSON report path. |
| `version` | no | `latest` | `shuttle` version to install. |

**Action outputs:** `score`, `level`, `report`

The Action writes a job summary with the readiness level and exits non-zero on any critical finding or when the score drops below `fail-below`.

### CLI in Any CI

```yaml
- run: shuttle doctor --target ${{ secrets.SSH_TARGET }} --fail-below 80
```

Works in GitHub Actions, GitLab CI, CircleCI, Jenkins -- anywhere a Linux runner can run a static binary over SSH.

---

## Configuration

Drop a `.shuttle.yml` at the project root to tune readiness checks, deployment strategy, and server config:

```yaml
app:
  domain: app.example.com
  healthcheckPath: /health

deploy:
  strategy: swarm
  # Reclaim local disk after each successful deploy by pruning the Docker
  # build cache left by `docker build`. off | capped | all (default: capped).
  prune_build_cache: capped
  # Max build-cache size kept when prune_build_cache is "capped" (default: 5GB).
  build_cache_keep: 5GB

checks:
  profile: [docker, caddy]
  ignore:
    - docker.containers_running_as_root

docker:
  allowRoot:
    - shared_redis
  allowDockerSocket:
    - dozzle*
  workerServices:
    - prod_worker-*
```

- `app.domain` + `app.healthcheckPath` unlock TLS and health-endpoint checks; without them those probes are skipped cleanly.
- The config path appears in every report and JSON output so reviewers can verify which exceptions were granted.
- `deploy.prune_build_cache` keeps local disk in check: builds run on your machine/CI and BuildKit caches every layer. `capped` (default) bounds the cache to `build_cache_keep` while keeping recent layers for fast rebuilds, `all` wipes it (slower next build), `off` disables cleanup. Dangling `:latest` images and the local registry's orphaned layers are reclaimed in the same pass (unless `off`). The remote VPS is unaffected — it only pulls.

### Stack Presets

`init --preset` writes an opinionated `.shuttle.yml` for common stacks -- fewer false positives on day one:

```bash
shuttle init --preset nextjs       --domain app.example.com
shuttle init --preset laravel      --domain shop.example.com
shuttle init --preset symfony      --domain api.example.com
shuttle init --preset node-api     --domain api.example.com
shuttle init --preset docker-swarm --domain edge.example.com
```

Each preset pre-fills `app.healthcheckPath`, relevant `docker.workerServices` patterns, and ignores checks that don't apply to the stack.

Combine with `--pro` for the full production stack:

```bash
shuttle init --preset laravel --pro --domain shop.example.com
shuttle init --preset symfony --pro --with-db mysql --domain api.example.com
```

### Cloudflare Integration

Cloudflare guardrails (`cloudflare.*` checks) require explicit opt-in plus an API token:

```yaml
cloudflare:
  enabled: true
  zone: example.com
  tokenEnv: CLOUDFLARE_API_TOKEN  # default; override only if needed
```

The token needs **read scopes** on `Zone`, `DNS`, and `Zone Settings`. When the token is missing, the zone doesn't match, or the token is rejected, every Cloudflare check skips with a clear explanation instead of failing the score.

---

## Check Catalog

43 production-readiness checks ship out of the box across **12 categories**:

| Category | Checks | Key verifications |
|---|---|---|
| **System** | 8 | OS version, disk/memory, unattended upgrades, fail2ban, swap, time sync |
| **SSH** | 3 | Root login, password auth, default port |
| **Docker** | 6 | Engine installed, restart policy, healthcheck, root containers, socket exposure |
| **Firewall** | 3 | UFW active, database ports, Docker published ports |
| **Secrets** | 3 | `.env` permissions, `.env` in Git, weak key/PEM permissions |
| **Reverse Proxy** | 5 | Caddy installed, admin API exposed, security headers, config valid, Adminer protected |
| **TLS** | 2 | Certificate validity, HSTS header |
| **DNS** | 1 | Domain resolves to server IP |
| **Monitoring** | 1 | Health endpoint returns 2xx |
| **Backups** | 1 | Backup artifacts or cron entries detected |
| **Compose** | 5 | Prod file exists, env files present, no `:latest` tags, resource limits, sensitive bind mounts |
| **Cloudflare** | 5 | SSL mode, Always HTTPS, WAF, DNS records, proxy status |

Severity levels: **critical** (-20), **high** (-10), **medium** (-5), **low** (-2), **info** (0).

Full reference with check IDs and remediation hints: [`docs/check-catalog.md`](docs/check-catalog.md)

---

## Pricing

| Feature | Free | Pro (199 EUR one-time) |
|---|---|---|
| `shuttle doctor` (local + `--target`) | yes | yes |
| `.shuttle.yml` readiness config | yes | yes |
| `shuttle init --preset` (basic templates) | yes | yes |
| Console + Markdown report | yes | yes |
| `shuttle harden --dry-run` | yes | yes |
| `shuttle deploy` / `provision` / `rollback` | yes | yes |
| `shuttle secrets` (encrypted store) | yes | yes |
| **`shuttle init --pro` (full stack templates)** | -- | **yes** |
| **`--with-db`, `--with-redis`, `--with-queue`, etc.** | -- | **yes** |
| **`shuttle report --format html`** | -- | **yes** |
| **`shuttle report --format pdf`** | -- | **yes** |
| **`shuttle harden --apply`** | -- | **yes** |

One license, unlimited projects, unlimited servers. No subscription, no per-seat pricing.

```bash
# Activate after purchase:
shuttle license activate <your-key>

# Check status:
shuttle license status
```

---

## Commands

```text
shuttle init        Detect stack, generate Dockerfile + docker-compose.yml + .env.example + config
shuttle provision   Bootstrap VPS: Docker Swarm + Caddy + UFW
shuttle deploy      Build and deploy (swarm / compose / blue-green)
shuttle rollback    Rollback to previous deployment
shuttle doctor      Run production readiness audit (43 checks)
shuttle report      Generate Markdown / HTML / PDF report
shuttle harden      Plan and apply security hardening
shuttle badge       Generate score badge for README
shuttle secrets     Manage encrypted secrets (set, get, list, remove, push)
shuttle status      Show container status across servers
shuttle logs        Stream remote logs
shuttle ssh         Open SSH session to server
shuttle exec        Run command in remote container
shuttle monitor     Live Docker resource usage
shuttle update      Self-update to latest version
shuttle uninstall   Remove shuttle from this machine
shuttle license     Manage Pro license activation
```

### `shuttle init` flags

| Flag | Default | Description |
|---|---|---|
| `--preset` | auto-detect | Force a preset (`laravel`, `symfony`, `nextjs`, `node-api`, `docker-swarm`) |
| `--app` | directory name | Application name |
| `--domain` | `<app>.example.com` | Production domain |
| `--host` | -- | Server IP or hostname |
| `--ci` | false | Generate a GitHub Actions workflow |
| `--force` | false | Overwrite existing files |
| **`--pro`** | false | **[Pro]** Enable all Pro services with sensible defaults |
| **`--with-db`** | -- | **[Pro]** Add database service (`postgres` or `mysql`) |
| **`--with-redis`** | false | **[Pro]** Add Redis 7 with AOF persistence |
| **`--with-queue`** | false | **[Pro]** Add queue worker (Laravel/Symfony only) |
| **`--with-scheduler`** | false | **[Pro]** Add scheduler (Laravel/Symfony only) |
| **`--with-mailpit`** | false | **[Pro]** Add Mailpit SMTP trap + web UI |

`--pro` expands to `--with-db postgres --with-redis --with-queue --with-scheduler --with-mailpit --ci`.

Run `shuttle <command> --help` for usage details.

---

## Architecture

```text
              ┌──────────────────────────────────────────────────────────────────┐
              │                          shuttle CLI                             │
              │                                                                  │
              │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────────┐  │
              │  │ doctor │ │ report │ │ harden │ │ deploy │ │  init [Pro]  │  │
              │  └───┬────┘ └───┬────┘ └───┬────┘ └───┬────┘ └──────┬───────┘  │
              └──────│──────────│──────────│──────────│──────────────│──────────┘
                     │          │          │          │              │
           shell     │  JSON    │  JSON    │  SSH +   │   detect +  │
           calls     │  report  │  report  │  Docker  │   templates │
                     ▼          ▼          ▼          ▼              ▼
              ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
              │  execx   │ │ MD/HTML/ │ │ Planner  │ │ Swarm /  │ │ Compose  │
              │ Local/SSH│ │ PDF      │ │ + apply  │ │ Compose/ │ │ Services │
              └──────────┘ └──────────┘ └──────────┘ │ Blue-Grn │ │ (Pro)    │
                                                      └──────────┘ └──────────┘
```

| Component | Role |
|---|---|
| **doctor** | Runs the 43-check suite over a local shell or SSH session. Outputs a scored JSON report. |
| **report** | Reads the JSON and renders Markdown, HTML, or PDF for sharing. |
| **harden** | Reads the JSON, plans remediation actions, and (with `--apply`) executes only allow-listed commands. |
| **deploy** | Builds the Docker image, pushes to registry, and deploys via the configured strategy. |
| **init** | Detects the stack, generates Dockerfile, compose, .env.example, and config. With `--pro`, assembles a multi-service compose from composable service blocks. |
| **execx** | Unified shell abstraction -- `Local` for the current machine, `SSH` for remote targets. |

Architecture and security details: [`plans/06-architecture-security.md`](plans/06-architecture-security.md)

---

## Supported Platforms

| Platform | Architecture | Notes |
|---|---|---|
| Linux x86_64 | `linux-x64` | Most VPS providers (Hetzner, OVH, DigitalOcean, Scaleway, Contabo) |
| Linux ARM64 | `linux-arm64` | Hetzner CAX, AWS Graviton, Raspberry Pi 4/5 |
| macOS x86_64 | `darwin-x64` | Intel Macs |
| macOS ARM64 | `darwin-arm64` | Apple Silicon (M1/M2/M3/M4) |

**Audited targets:** Ubuntu 22.04, Ubuntu 24.04, Debian 12.

---

## Development

```bash
cd go-cli

# Run tests:
go test ./...

# Lint:
go vet ./...

# Format:
gofmt -w .

# Run locally:
go run ./cmd/shuttle --help
```

Build release binaries:

```bash
sh scripts/build-go.sh
```

CI runs: `gofmt` check -> `go vet ./...` -> `go test ./...`

---

## Versioning

```bash
sh scripts/release.sh patch   # v2.0.0 -> v2.0.1
sh scripts/release.sh minor   # -> v2.1.0
sh scripts/release.sh major   # -> v3.0.0
```

The script validates tests, builds the binary with version injected via ldflags, installs to `~/.local/bin/shuttle`, and creates an annotated git tag. Push triggers the GitHub release workflow that cross-compiles for all platforms.

---

## License

Proprietary. [Contact for evaluation](mailto:kev.aubree@gmail.com).
