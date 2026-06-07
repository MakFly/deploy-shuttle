# Cloud, Pricing, Differentiation, and Launch

## 1. Cloud Product Scope

Cloud dashboard is not required for MVP.

When added, it should include:

### Free account

- sync latest report;
- view one server;
- basic history.

### Pro

- scheduled scans;
- email alerts;
- deploy history;
- uptime checks;
- multiple servers;
- report sharing.

### Agency

- client workspaces;
- white-label reports;
- team access;
- audit logs;
- monthly readiness reports;
- multi-server inventory.

## 2. Pricing Hypothesis

### CLI

Free/open-core:

- `doctor`;
- console report;
- basic hardening suggestions;
- local config generation.

### Paid one-shot

```txt
Production Readiness Report - 19 EUR
```

Includes:

- hosted HTML report;
- shareable link;
- 30-day history.

### Subscription

```txt
Solo - 9 EUR/month
1 server
scheduled scans
email alerts
hosted reports

Pro - 29 EUR/month
5 servers
deploy history
backup monitoring
Cloudflare checks
GitHub Action integration

Agency - 79 EUR/month
25 servers
client workspaces
white-label reports
team access
audit logs
```

## 3. Competitive Differentiation

### Coolify / Dokploy / CapRover

They help run apps.

DeployShuttle helps answer:

```txt
Is this VPS production-ready?
```

### Laravel Forge / Ploi

They manage servers for specific ecosystems.

DeployShuttle is CLI-first and Docker-first.

### Generic security scanners

They scan security broadly.

DeployShuttle focuses on practical VPS production readiness for web apps:

- Docker;
- Caddy;
- Cloudflare;
- backups;
- healthchecks;
- deployment safety.

### PaaS providers

They abstract infrastructure.

DeployShuttle keeps ownership on the user's VPS.

## 4. Success Metrics

### MVP metrics

- 100 CLI installs.
- 30 successful doctor scans.
- 10 generated reports.
- 5 users running `doctor` on a real remote VPS.
- 3 users asking for scheduled scans or dashboard.
- 1 paid readiness report.

### Product-market signal

Strong signal:

- users paste/share their report;
- users ask for "fix this automatically";
- agencies ask for white-label PDF;
- users add it to CI;
- users want recurring scan alerts.

Weak signal:

- users only run it once out of curiosity;
- no one uses reports;
- no one wants cloud;
- only hobbyists engage.

## 5. Launch Plan

### Week 1 - CLI Doctor

- Implement local/SSH exec adapter.
- Implement 15 core checks:
  - OS;
  - disk;
  - Docker installed;
  - Docker restart policy;
  - Docker healthcheck;
  - public DB ports;
  - UFW active;
  - `.env` permissions;
  - `.env` in Git;
  - Caddy installed;
  - Caddy admin exposed;
  - TLS cert;
  - HSTS;
  - backup detected;
  - health endpoint.
- Console output.
- JSON output.

### Week 2 - Report + Init

- Markdown report.
- HTML report.
- `.shuttle.yml`.
- `init --preset node-api`.
- `init --preset next`.
- `init --preset laravel`.
- Basic docs site.

### Week 3 - Harden

- `harden --dry-run`.
- Safe fix registry.
- UFW fix.
- `.env` permissions fix.
- Caddy security headers generation.
- Compose restart policy patch.
- Backup script generation.

### Week 4 - Beta

- Landing page.
- Install script.
- GitHub Action example.
- 20 beta users.
- Collect reports.
- Validate paid report or subscription interest.
