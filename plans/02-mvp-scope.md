# MVP Scope

## 1. CLI Commands

### `shuttle doctor`

Runs a production readiness audit against local machine or remote VPS.

Examples:

```bash
shuttle doctor
shuttle doctor --target root@1.2.3.4
shuttle doctor --target deploy@server.example.com --profile docker
shuttle doctor --format json
```

Responsibilities:

- connect locally or via SSH;
- collect server facts;
- run checks;
- compute readiness score;
- print actionable report;
- return non-zero exit code if critical issues exist.

### `shuttle report`

Generates an HTML, JSON or Markdown report from the latest scan.

Examples:

```bash
shuttle report --format html
shuttle report --format markdown
shuttle report --output ./shuttle-report.html
```

MVP formats:

- console;
- JSON;
- Markdown;
- HTML static.

PDF can wait.

### `shuttle harden`

Applies safe hardening fixes.

Examples:

```bash
shuttle harden --target root@1.2.3.4
shuttle harden --target root@1.2.3.4 --only firewall,ssh,docker
shuttle harden --dry-run
```

MVP behavior:

- dry-run by default for destructive changes;
- explicit confirmation required;
- each fix must be reversible or clearly documented;
- never modify app data without explicit consent.

### `shuttle init`

Generates production files for a project.

Examples:

```bash
shuttle init
shuttle init --preset next
shuttle init --preset laravel
shuttle init --preset symfony
shuttle init --preset node-api
```

Generated files:

- `docker-compose.prod.yml`;
- `Caddyfile`;
- `.env.production.example`;
- `.shuttle.yml`;
- optional backup script;
- optional healthcheck route suggestion.

### `shuttle deploy`

MVP can be minimal or postponed after `doctor` and `report`.

Initial behavior:

- read `.shuttle.yml`;
- upload compose/Caddy/env to VPS;
- run `docker compose pull/build/up -d`;
- run healthcheck;
- display deployment status.

Examples:

```bash
shuttle deploy --target deploy@server
shuttle deploy --env production
```

## 2. Production Readiness Checks

Each check has:

- id;
- title;
- category;
- severity;
- explanation;
- detection command;
- remediation;
- auto-fix availability;
- references/docs link.

Severity:

- `critical`;
- `high`;
- `medium`;
- `low`;
- `info`.

Categories:

- system;
- ssh;
- firewall;
- docker;
- compose;
- reverse-proxy;
- tls;
- cloudflare;
- database;
- backups;
- secrets;
- monitoring;
- app;
- performance.

## 3. MVP Command Examples

```bash
# scan current machine
shuttle doctor

# scan remote server
shuttle doctor --target root@203.0.113.10

# fail if score too low
shuttle doctor --target root@203.0.113.10 --fail-below 75

# generate HTML report
shuttle report --format html --output report.html

# preview fixes
shuttle harden --target root@203.0.113.10 --dry-run

# apply only firewall fixes
shuttle harden --target root@203.0.113.10 --only firewall

# initialize production files
shuttle init --preset next
```

## 4. Acceptance Criteria for MVP

MVP is accepted when:

- `shuttle doctor` runs locally.
- `shuttle doctor --target user@host` runs remotely over SSH.
- At least 15 checks are implemented.
- Console report is readable.
- JSON report is valid.
- Score is deterministic.
- `--fail-below` works.
- `.shuttle.yml` is supported.
- Report generation supports Markdown and HTML.
- No secrets are printed in output.
- `harden --dry-run` lists planned fixes without applying them.
- Documentation includes quickstart and check catalog.
- Install script works on macOS/Linux dev machines.

## 5. MVP Non-Goals

MVP should not include:

- Kubernetes;
- multi-region deployment;
- full observability platform;
- full secret manager;
- full PaaS dashboard;
- complex RBAC;
- automatic destructive hardening;
- generic CIS benchmark clone;
- support for every Linux distro;
- Windows servers;
- complex Terraform provider;
- marketplace/templates store.
