<div align="center">

# DeployShuttle

**Production readiness CLI for Docker apps on VPS**

Audit, harden and deploy Docker workloads running on any Linux VPS.
One command gives you a security score, a shareable report, and a hardening plan — before client handoff.

[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/license-proprietary-blue)](#license)
[![GitHub Action](https://img.shields.io/badge/action-MakFly%2Fdeploy--shuttle-2088FF?logo=githubactions&logoColor=white)](#github-action)

```bash
deploy-shuttle doctor --target root@server
```

</div>

---

## Table of Contents

- [Why DeployShuttle](#why-deployshuttle)
- [Features](#features)
- [Quick Start](#quick-start)
  - [Install](#install)
  - [Audit a Server](#audit-a-server)
  - [Generate a Report](#generate-a-shareable-report)
  - [Harden a Server](#plan-and-apply-hardening)
- [CI / CD Integration](#cicd-integration)
  - [GitHub Action](#github-action)
  - [CLI in CI](#cli-in-any-ci)
- [Configuration](#configuration)
  - [Stack Presets](#stack-presets)
  - [Cloudflare Integration](#cloudflare-integration)
- [Check Catalog](#check-catalog)
- [Architecture](#architecture)
- [Supported Platforms](#supported-platforms)
- [Other Commands](#other-commands)
- [Development](#development)
- [Roadmap](#roadmap)
- [License](#license)

---

## Why DeployShuttle

You can ship a Docker app to a **$5 VPS** in an afternoon — but the gap between "it works" and "it's production-ready" is a maze of small, easy-to-miss security and reliability issues:

| Problem | Risk |
|---|---|
| No firewall, or a firewall that allows everything | Full server exposure |
| Public database ports (Postgres, Redis, MongoDB) | Data breach |
| Containers running as root or mounting the Docker socket | Privilege escalation |
| `.env` files world-readable or tracked by Git | Credential leak |
| Caddy admin API exposed to the internet | Remote takeover |
| Workloads with no restart policy or healthcheck | Silent downtime |

**DeployShuttle closes that gap.** It does not replace your deploy tooling — it produces the **production readiness report you hand to a client** before you call the project done.

---

## Features

- **43 automated security and reliability checks** — system, SSH, Docker, firewall, secrets, reverse proxy, TLS, DNS, monitoring, backups, compose, Cloudflare
- **Deterministic scoring** — 0-100 readiness score with severity-weighted deductions (critical / high / medium / low / info)
- **Local or remote scan** — same check suite runs on your machine or over SSH with a single `--target` flag
- **Shareable reports** — Markdown, self-contained HTML, or PDF output for client handoffs, PRs, or audits
- **Dry-run hardening** — concrete remediation plan with ready-to-run commands; `--apply` only executes safe, idempotent actions
- **CI gate** — fail pipelines when the score drops below a threshold; first-class GitHub Action included
- **Stack presets** — opinionated configs for Next.js, Laravel, Node API, Docker Swarm — fewer false positives on day one
- **Cloudflare guardrails** — opt-in SSL mode, WAF, DNS, and proxy checks against your Cloudflare zone
- **Zero dependencies** — single static Go binary, no runtime, no Docker required on the auditor side

---

## Quick Start

### Install

```bash
curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh
```

Downloads the latest binary for your OS and architecture into `~/.local/bin`.

Pre-built targets: `linux-x64`, `linux-arm64` (Hetzner CAX, AWS Graviton, Raspberry Pi 4/5), `darwin-x64`, `darwin-arm64`.

Or download manually from [Releases](https://github.com/MakFly/deploy-shuttle/releases) and place it on your `$PATH`.

### Audit a Server

```bash
# Local scan (current machine):
deploy-shuttle doctor

# Remote scan over SSH (uses your SSH agent):
deploy-shuttle doctor --target root@203.0.113.10

# Custom SSH port:
deploy-shuttle doctor --target root@203.0.113.10:7022

# Save the JSON report for downstream tooling:
deploy-shuttle doctor --target root@server --output .deployshuttle/latest-report.json
```

**Example output:**

```text
DeployShuttle Doctor Report
Target: root@203.0.113.10:7022
Score: 70/100 - Risky

High:
  [x] Sensitive database ports are not public - 5432 publicly bound (UFW restricted).
  [x] Docker socket is not exposed to workloads - 2 workloads mount /var/run/docker.sock.

Medium:
  [x] Docker workloads have healthchecks - 4 workloads lack healthchecks.
  [x] Docker workloads do not run as root - 15 workloads run as root.
```

### Generate a Shareable Report

```bash
# Markdown — good for PRs and engineering audits:
deploy-shuttle report --format markdown --output report.md

# HTML — good for clients, self-contained, opens in any browser:
deploy-shuttle report --format html --output report.html

# PDF — good for handoff packs, uses the optional React PDF renderer:
deploy-shuttle report --format pdf --output report.pdf
```

By default `report` reads `.deployshuttle/latest-report.json`. Pass `--input <file>` for a different doctor JSON.

### Plan and Apply Hardening

```bash
# Dry-run plan — never touches the server:
deploy-shuttle harden --dry-run

# Apply only safe-auto-apply actions (locally):
deploy-shuttle harden --apply --yes

# Apply over SSH on the audited target:
deploy-shuttle harden --apply --target root@203.0.113.10 --yes
```

The dry-run plan converts each open finding into a concrete proposed action with the source check ID, rationale, and either ready-to-run commands or manual steps. `--apply` only executes commands that are **idempotent, scoped, and reversible**.

Current safe allow-list: `chmod 600 .env` and `ufw deny <port>/tcp`.

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
| `target` | no | — | SSH target (`user@host` or `user@host:port`). Omit to scan the runner. |
| `ssh-private-key` | no | — | SSH private key (PEM) for remote scans. |
| `ssh-known-hosts` | no | — | Known hosts content. Auto-detected via `ssh-keyscan` when omitted. |
| `config` | no | — | Path to `.deployshuttle.yml`. |
| `fail-below` | no | `75` | Fail the job when the score is strictly below this threshold. |
| `output` | no | `.deployshuttle/latest-report.json` | JSON report path. |
| `version` | no | `latest` | `deploy-shuttle` version to install. |

**Action outputs:** `score`, `level`, `report`

The Action writes a job summary with the readiness level and exits non-zero on any critical finding or when the score drops below `fail-below`.

### CLI in Any CI

```yaml
- run: deploy-shuttle doctor --target ${{ secrets.SSH_TARGET }} --fail-below 80
```

Works in GitHub Actions, GitLab CI, CircleCI, Jenkins — anywhere a Linux runner can run a static binary over SSH.

---

## Configuration

Drop a `.deployshuttle.yml` at the project root to ignore checks, allow-list workloads, or tune behavior:

```yaml
app:
  domain: app.example.com
  healthcheckPath: /health

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

### Stack Presets

`init --preset` writes an opinionated `.deployshuttle.yml` for common stacks — fewer false positives on day one:

```bash
deploy-shuttle init --preset nextjs       --domain app.example.com
deploy-shuttle init --preset laravel      --domain shop.example.com
deploy-shuttle init --preset node-api     --domain api.example.com
deploy-shuttle init --preset docker-swarm --domain edge.example.com
```

Each preset pre-fills `app.healthcheckPath`, relevant `docker.workerServices` patterns, and ignores checks that don't apply to the stack.

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

Severity levels: **critical** (−20), **high** (−10), **medium** (−5), **low** (−2), **info** (0).

Full reference with check IDs and remediation hints: [`docs/check-catalog.md`](docs/check-catalog.md)

---

## Architecture

```text
              ┌─────────────────────────────────────────────────┐
              │               deploy-shuttle CLI                │
              │                                                 │
              │  ┌──────────┐  ┌──────────┐  ┌──────────────┐  │
              │  │  doctor  │  │  report  │  │    harden    │  │
              │  └────┬─────┘  └────┬─────┘  └──────┬───────┘  │
              └───────│─────────────│───────────────│──────────┘
                      │             │               │
            shell     │   JSON      │     JSON      │
            calls     │   report    │     report    │
                      ▼             ▼               ▼
              ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
              │ execx.Local  │ │  MD / HTML / │ │  Planner +   │
              │ execx.SSH    │ │  PDF render  │ │  safe apply  │
              └──────────────┘ └──────────────┘ └──────────────┘
```

| Component | Role |
|---|---|
| **doctor** | Runs the check suite over a local shell or SSH session. Outputs a scored JSON report. |
| **report** | Reads the JSON and renders Markdown, HTML, or PDF for sharing. |
| **harden** | Reads the JSON, plans remediation actions, and (with `--apply`) executes only allow-listed commands. |
| **execx** | Unified shell abstraction — `Local` for the current machine, `SSH` for remote targets. |

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

## Other Commands

DeployShuttle includes a deployment toolkit alongside its readiness features:

```text
init  new  dev  provision  deploy  rollback  destroy
logs  ssh  status  exec  lock  secrets  license  validate  ci  monitor
```

Run `deploy-shuttle <command> --help` for usage. The primary workflow is `doctor` → `report` → `harden`.

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
go run ./cmd/deploy-shuttle --help
```

Build release binaries:

```bash
sh scripts/build-go.sh
```

CI runs: `gofmt` check → `go vet ./...` → `go test ./...`

---

## Roadmap

DeployShuttle is in active development. Planned additions include:

- Additional `harden` actions (UFW baseline, Caddy admin lockdown)
- `cloudflare.origin_exposed` check (direct A records leaking origin IP)
- Persistent volume checks for database containers
- Backup recency, restore drill, and offsite verification
- Log rotation and uptime checks

Full roadmap, PRD, scoring model, and launch plan: [`plans/`](plans/README.md)

---

## License

Proprietary. [Contact for evaluation](mailto:kev.aubree@gmail.com).
