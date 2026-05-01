# MVP Scope

## 1. CLI Commands

### `deploy-shuttle doctor`

Runs a production readiness audit against local machine or remote VPS.

Examples:

```bash
deploy-shuttle doctor
deploy-shuttle doctor --target root@1.2.3.4
deploy-shuttle doctor --target deploy@server.example.com --profile docker
deploy-shuttle doctor --format json
```

Responsibilities:

- connect locally or via SSH;
- collect server facts;
- run checks;
- compute readiness score;
- print actionable report;
- return non-zero exit code if critical issues exist.

### `deploy-shuttle report`

Generates an HTML, JSON or Markdown report from the latest scan.

Examples:

```bash
deploy-shuttle report --format html
deploy-shuttle report --format markdown
deploy-shuttle report --output ./deployshuttle-report.html
```

MVP formats:

- console;
- JSON;
- Markdown;
- HTML static.

PDF can wait.

### `deploy-shuttle harden`

Applies safe hardening fixes.

Examples:

```bash
deploy-shuttle harden --target root@1.2.3.4
deploy-shuttle harden --target root@1.2.3.4 --only firewall,ssh,docker
deploy-shuttle harden --dry-run
```

MVP behavior:

- dry-run by default for destructive changes;
- explicit confirmation required;
- each fix must be reversible or clearly documented;
- never modify app data without explicit consent.

### `deploy-shuttle init`

Generates production files for a project.

Examples:

```bash
deploy-shuttle init
deploy-shuttle init --preset next
deploy-shuttle init --preset laravel
deploy-shuttle init --preset symfony
deploy-shuttle init --preset node-api
```

Generated files:

- `docker-compose.prod.yml`;
- `Caddyfile`;
- `.env.production.example`;
- `.deployshuttle.yml`;
- optional backup script;
- optional healthcheck route suggestion.

### `deploy-shuttle deploy`

MVP can be minimal or postponed after `doctor` and `report`.

Initial behavior:

- read `.deployshuttle.yml`;
- upload compose/Caddy/env to VPS;
- run `docker compose pull/build/up -d`;
- run healthcheck;
- display deployment status.

Examples:

```bash
deploy-shuttle deploy --target deploy@server
deploy-shuttle deploy --env production
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
deploy-shuttle doctor

# scan remote server
deploy-shuttle doctor --target root@203.0.113.10

# fail if score too low
deploy-shuttle doctor --target root@203.0.113.10 --fail-below 75

# generate HTML report
deploy-shuttle report --format html --output report.html

# preview fixes
deploy-shuttle harden --target root@203.0.113.10 --dry-run

# apply only firewall fixes
deploy-shuttle harden --target root@203.0.113.10 --only firewall

# initialize production files
deploy-shuttle init --preset next
```

## 4. Acceptance Criteria for MVP

MVP is accepted when:

- `deploy-shuttle doctor` runs locally.
- `deploy-shuttle doctor --target user@host` runs remotely over SSH.
- At least 15 checks are implemented.
- Console report is readable.
- JSON report is valid.
- Score is deterministic.
- `--fail-below` works.
- `.deployshuttle.yml` is supported.
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
