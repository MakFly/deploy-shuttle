# Scoring, Config, Reports, and CI

## 1. Scoring Model

DeployShuttle should generate a score from 0 to 100.

Suggested scoring:

```txt
critical = -20
high     = -10
medium   = -5
low      = -2
info     = 0
```

Minimum score: 0  
Maximum score: 100

Readiness levels:

```txt
90-100  Production Ready
75-89   Almost Ready
50-74   Risky
0-49    Not Production Ready
```

Example output:

```txt
DeployShuttle Doctor Report

Target: root@203.0.113.10
Profile: Docker + Caddy + Cloudflare
Score: 62/100 - Risky

Critical:
  [x] PostgreSQL exposed on 0.0.0.0:5432
  [x] Cloudflare SSL mode appears to be Flexible
  [x] No database backup detected

High:
  [x] 3 containers without restart policy
  [x] UFW firewall inactive

Medium:
  [!] No healthcheck configured
  [!] No HSTS header detected

Next:
  deploy-shuttle harden --only firewall,docker
  deploy-shuttle report --format html
```

## 2. Configuration File

`.deployshuttle.yml`

```yaml
version: 1

project:
  name: my-app
  type: docker-compose

server:
  host: 203.0.113.10
  user: deploy
  sshPort: 22

app:
  domain: app.example.com
  healthcheckPath: /health
  composeFile: docker-compose.prod.yml
  envFile: .env.production

reverseProxy:
  type: caddy
  caddyfile: Caddyfile

cloudflare:
  enabled: true
  zone: example.com
  proxied: true

checks:
  profile:
    - docker
    - caddy
    - cloudflare
    - postgres
  ignore:
    - ssh.port_default

docker:
  # Workloads allowed to mount /var/run/docker.sock. Use exact names or simple
  # prefix/suffix wildcards such as caddy_* or *_agent.
  allowDockerSocket:
    - caddy_dozzle
    - shared_uptime-kuma

  # Workloads allowed to run as root because they are trusted infrastructure
  # images or cannot safely be changed yet.
  allowRoot:
    - caddy_*
    - shared_redis

  # Worker services where no HTTP healthcheck is expected.
  workerServices:
    - prod_worker-*

harden:
  allow:
    - firewall
    - docker-restart-policy
    - caddy-security-headers
```

## 3. Auto-Fix Rules

Auto-fix must be conservative.

### Safe in MVP

Can be automated:

- enable UFW with SSH/HTTP/HTTPS allowed;
- add Docker restart policies to generated compose;
- generate Caddy security headers;
- set `.env` file permissions to `600`;
- generate backup script;
- generate healthcheck stubs;
- create `.gitignore` entry for `.env`;
- validate Caddy config;
- create Cloudflare DNS record when token provided.

### Not safe by default

Require explicit confirmation:

- disabling root SSH;
- disabling password auth;
- changing SSH port;
- modifying existing compose services;
- closing ports already in use;
- deleting Docker resources;
- touching database data;
- changing Cloudflare SSL mode.

## 4. Report Output

### Console report

Default for terminal.

### JSON report

For CI/CD and automation.

```json
{
  "target": "root@203.0.113.10",
  "score": 62,
  "level": "risky",
  "checks": [
    {
      "id": "firewall.database_port_public",
      "severity": "critical",
      "status": "failed",
      "title": "PostgreSQL is publicly exposed",
      "remediation": "Bind PostgreSQL to internal Docker network or localhost."
    }
  ]
}
```

### Markdown report

For GitHub issues, client handoff, docs.

### HTML report

For paid/agency use.

Should include:

- score;
- target;
- timestamp;
- critical issues;
- recommendations;
- passed checks;
- ignored checks;
- next commands;
- branding placeholder.

## 5. CI/CD Use Case

Developers can use DeployShuttle in CI to prevent unsafe production deploys.

Example:

```yaml
name: Production readiness

on:
  workflow_dispatch:

jobs:
  doctor:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: curl -fsSL https://deployshuttle.dev/install.sh | bash
      - run: deploy-shuttle doctor --target deploy@server --format json --fail-below 75
```

Exit codes:

- `0`: passed;
- `1`: failed threshold;
- `2`: connection/config error;
- `3`: internal error.
