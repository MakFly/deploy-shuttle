# DeployShuttle

> Audit, harden and deploy Docker apps on VPS.

DeployShuttle is a CLI-first **production readiness** tool for Docker apps running on
low-cost VPS providers (Hetzner, OVH, Scaleway, Contabo, DigitalOcean, …). One command
gives you a deterministic readiness score, a shareable report (Markdown / HTML / PDF),
and a dry-run hardening plan before any client handoff.

```bash
deploy-shuttle doctor --target root@server
```

> [!TIP]
> Copy the repository URL:
>
> ```text
> https://github.com/MakFly/deploy-shuttle
> ```

## Why

You can ship a Docker app to a `$5` VPS in an afternoon — but the gap between
"it works" and "it's production-ready" is a maze of small, easy-to-miss issues:

- no firewall, or a firewall that allows everything;
- public database ports that nobody noticed;
- containers running as root or mounting the Docker socket read-write;
- `.env` files world-readable or tracked by Git;
- Caddy admin API exposed to the internet;
- workloads with no restart policy, no healthcheck, no observability.

DeployShuttle owns that gap. It does not replace your deploy tooling — it produces
the **report you can hand to a client** before you call the project done.

## Quickstart

### Install

```bash
curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh
```

This downloads the latest binary for your OS and architecture into `~/.local/bin`.
Pre-built targets: `linux-x64`, `linux-arm64` (Hetzner CAX, AWS Graviton,
Raspberry Pi 4/5), `darwin-x64`, `darwin-arm64`. Or download manually from
[Releases](https://github.com/MakFly/deploy-shuttle/releases) and put it on
your `$PATH`.

### Audit a server

```bash
# Local scan (current machine):
deploy-shuttle doctor

# Remote scan over SSH (uses your SSH agent):
deploy-shuttle doctor --target root@203.0.113.10

# Custom SSH port:
deploy-shuttle doctor --target root@203.0.113.10:7022

# Persist the JSON report for downstream tooling:
deploy-shuttle doctor --target root@server --output .deployshuttle/latest-report.json
```

You get a console summary like:

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

### Generate a shareable report

```bash
# Markdown (good for PRs and engineering audits):
deploy-shuttle report --format markdown --output report.md

# HTML (good for clients — self-contained, opens in any browser):
deploy-shuttle report --format html --output report.html

# PDF (good for handoff packs — uses the optional React PDF renderer):
deploy-shuttle report --format pdf --output report.pdf
```

By default `report` reads `.deployshuttle/latest-report.json`. Pass
`--input <file>` for any other doctor JSON.

### Plan and apply hardening

```bash
# Dry-run plan, never touches the server:
deploy-shuttle harden --dry-run

# Apply only the actions flagged safe-auto-apply (locally):
deploy-shuttle harden --apply --yes

# Same, over SSH on the audited target:
deploy-shuttle harden --apply --target root@203.0.113.10 --yes
```

The dry-run plan converts each open finding into a concrete proposed action with
the source check ID, the rationale, and either ready-to-run commands or the manual
steps to take. `--apply` only executes commands that are idempotent, scoped, and
reversible. The current safe allow-list is `chmod 600 .env` and `ufw deny <port>/tcp`.

### CI integration

Use the published GitHub Action:

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

Or call the CLI directly:

```yaml
- run: deploy-shuttle doctor --target ${{ secrets.SSH_TARGET }} --fail-below 80
```

`doctor` exits non-zero on any critical finding, or whenever the score falls below
the threshold passed to `--fail-below`. The Action exposes `score`, `level`, and
`report` outputs and writes a job summary with the readiness level.

## Configuration

Drop a `.deployshuttle.yml` at the project root to ignore checks or allow-list workloads:

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

`app.domain` and `app.healthcheckPath` unlock the TLS and health-endpoint
checks; without them those probes are skipped cleanly.

Cloudflare guardrails (`cloudflare.*` checks) are gated by an explicit
opt-in plus an API token:

```yaml
cloudflare:
  enabled: true
  zone: example.com
  # tokenEnv defaults to CLOUDFLARE_API_TOKEN; override only if needed.
  tokenEnv: MY_CF_TOKEN
```

The token needs read scopes on `Zone`, `DNS`, and `Zone Settings`. When the
token is missing, the zone does not match, or the token is rejected, every
Cloudflare check skips with a clear explanation instead of failing the score.

The config path lands in every report and JSON output so reviewers can verify which
exceptions were granted.

### Stack presets

`init --preset` writes an opinionated `.deployshuttle.yml` for common stacks so
`doctor` produces fewer false positives on day one:

```bash
deploy-shuttle init --preset nextjs       --domain app.example.com
deploy-shuttle init --preset laravel      --domain shop.example.com
deploy-shuttle init --preset node-api     --domain api.example.com
deploy-shuttle init --preset docker-swarm --domain edge.example.com
```

Each preset pre-fills `app.healthcheckPath`, the relevant
`docker.workerServices` patterns, and ignores checks that do not apply to the
stack (e.g. `adminer.ip_restriction_missing` for Next.js / Node API).

## Check catalog

43 checks ship today across system, SSH, Docker, firewall, secrets,
reverse proxy, TLS, DNS, monitoring, backups, compose, and Cloudflare
categories. Full reference in [`docs/check-catalog.md`](docs/check-catalog.md).

## How it works

```text
                 ┌──────────────────────────────────────────────┐
                 │              deploy-shuttle CLI              │
                 │ ┌──────────┐  ┌────────┐  ┌────────────────┐ │
                 │ │  doctor  │  │ report │  │     harden     │ │
                 │ └────┬─────┘  └────┬───┘  └────────┬───────┘ │
                 └──────│──────────── │───────────────│─────────┘
                        │ shell calls │ doctor.json   │ doctor.json
                        ▼             ▼               ▼
                ┌──────────────┐  ┌────────────┐  ┌────────────┐
                │ execx.Local  │  │ MD/HTML/PDF│  │  Planner + │
                │ execx.SSH    │  │  renderers │  │ safe apply │
                └──────────────┘  └────────────┘  └────────────┘
```

- `doctor` runs the same check suite over a local shell or an SSH session.
- `report` reads the JSON output and renders Markdown, HTML, or PDF.
- `harden` reads the same JSON, plans actions, and (with `--apply`) executes only
  allow-listed commands.

Architecture and security details: [`plans/06-architecture-security.md`](plans/06-architecture-security.md).

## Other commands

DeployShuttle inherits a deployment toolkit from its earlier life as a generic VPS CLI:

```text
init  new  dev  provision  deploy  rollback  destroy
logs  ssh  status  exec  lock  secrets  license  validate  ci  monitor
```

Run `deploy-shuttle <command> --help` for usage. These are functional but secondary;
the product hook is `doctor` → `report` → `harden`.

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

## Product plan

DeployShuttle is in active development. Roadmap, PRD, scoring model, and launch plan
live under [`plans/`](plans/README.md). The legacy TypeScript implementation is
archived in [`legacy/ts-cli/`](legacy/ts-cli/) for reference only.

## License

Proprietary. Contact for evaluation.
