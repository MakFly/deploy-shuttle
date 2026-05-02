# Check Catalog

DeployShuttle ships **43 production-readiness checks** out of the box.
Every finding includes a severity, a one-line summary, and a remediation hint.
Checks run identically over a local shell or an SSH session (`doctor --target user@host`).

Severities map to the score deduction model:

| Severity | Default deduction | Behavior |
| --- | --- | --- |
| `critical` | 30 | Causes `doctor` to exit non-zero regardless of `--fail-below`. |
| `high` | 15 | Counted toward score; counts as an "open finding". |
| `medium` | 5 | Counted toward score. |
| `low` | 1 | Counted toward score. |
| `info` | 0 | Surfaced in reports, no score impact. |

Use [`.deployshuttle.yml`](../README.md#configuration) to ignore findings or allow-list workloads.

## System

| ID | Severity | What it verifies |
| --- | --- | --- |
| `system.os_supported` | high | Host runs Ubuntu 22.04, Ubuntu 24.04, or Debian 12. |
| `system.disk_space_low` | medium / high | Root filesystem free space is comfortable. Fails high above 90% usage. |
| `system.updates_pending` | medium / high | Counts pending APT upgrades. Skipped on non-APT hosts. |
| `system.memory_low` | medium / high | Available memory is at least 10% of total. |
| `system.unattended_upgrades_inactive` | medium | `unattended-upgrades` package is installed and the service is enabled. |
| `system.fail2ban_inactive` | medium | `fail2ban` is installed and active to throttle SSH brute-force attempts. |
| `system.swap_missing` | low | At least one swap device or file is configured. |
| `system.time_sync_inactive` | medium | A time synchronization daemon (`systemd-timesyncd`, `chrony`, `ntp`) is active. |

## SSH

| ID | Severity | What it verifies |
| --- | --- | --- |
| `ssh.root_login_enabled` | high | `PermitRootLogin` is `no` or `prohibit-password` in `/etc/ssh/sshd_config`. |
| `ssh.password_auth_enabled` | high | `PasswordAuthentication` is `no` so only key-based logins are accepted. |
| `ssh.port_default` | low | sshd does not listen on port 22 (drive-by-scan reduction). |

## Docker

| ID | Severity | What it verifies |
| --- | --- | --- |
| `docker.not_installed` | high | Docker Engine binary is reachable. |
| `docker.service_not_enabled` | high | `docker.service` is enabled at boot and active. |
| `docker.containers_without_restart_policy` | high | Every workload (classic + Swarm) has a restart policy. |
| `docker.containers_without_healthcheck` | medium | Every workload defines a `HEALTHCHECK` (or compose/swarm `healthcheck`). |
| `docker.containers_running_as_root` | medium | Workloads do not run as root unless allow-listed via `docker.allowRoot`. |
| `docker.sock_exposed` | high / critical | Workloads do not mount `/var/run/docker.sock` read-write unless allow-listed via `docker.allowDockerSocket`. |

## Firewall

| ID | Severity | What it verifies |
| --- | --- | --- |
| `firewall.ufw_inactive` | high | UFW is installed and active with deny-by-default. |
| `firewall.database_port_public` | high / critical | Sensitive DB ports (Postgres, MySQL, MongoDB, Redis, …) are not publicly reachable. Downgraded when UFW restricts the port. |
| `firewall.docker_published_sensitive_ports` | high | No Docker container publishes a sensitive port on `0.0.0.0` (5432, 3306, 6379, 7700, 9200, 27017, 2019). |

## Secrets

| ID | Severity | What it verifies |
| --- | --- | --- |
| `secrets.env_world_readable` | critical | `.env` is not world-readable. Auto-fix available via `harden --apply`. |
| `secrets.env_in_git` | critical | `.env` is not tracked by Git. |
| `secrets.weak_file_permissions` | high | No `.env*`, `*.pem`, `*.key`, or SSH private key is world-readable in the project tree (depth ≤ 3). |

## Reverse proxy

| ID | Severity | What it verifies |
| --- | --- | --- |
| `caddy.not_installed` | medium | Caddy binary or container is present. |
| `caddy.admin_exposed` | critical | Caddy admin API (`:2019`) is not reachable from the public network. |
| `caddy.no_security_headers` | medium | Caddyfile sets `Strict-Transport-Security`, `X-Content-Type-Options`, and `Referrer-Policy`. |
| `caddy.invalid_config` | high | `caddy validate` succeeds against the active Caddyfile. |
| `adminer.ip_restriction_missing` | high | A running Adminer container is protected by IP allow-list, deny-by-default rule, and basic auth. |

## TLS

| ID | Severity | What it verifies |
| --- | --- | --- |
| `tls.cert_missing` | medium / high / critical | Certificate for `app.domain` is reachable, valid, and not expiring within 14 days. Skipped without `app.domain` in `.deployshuttle.yml`. |
| `tls.hsts_missing` | medium | `https://<app.domain>` returns a `Strict-Transport-Security` header. Skipped without `app.domain`. |

## DNS

| ID | Severity | What it verifies |
| --- | --- | --- |
| `dns.domain_not_pointing_to_server` | medium | `app.domain` resolves to the server's public IP. Skipped without `app.domain` or when the public IP cannot be determined. |

## Monitoring

| ID | Severity | What it verifies |
| --- | --- | --- |
| `monitoring.no_health_endpoint` | medium | `https://<app.domain><app.healthcheckPath>` returns a 2xx response. Skipped when either field is missing. |

## Backups

| ID | Severity | What it verifies |
| --- | --- | --- |
| `db.no_backup_detected` | high | Detects recent backup artifacts (`*.sql*`, `*.dump`, `*.tar*` under `/backups`, `/var/backups`, `/opt/backups`) or `pg_dump`/`mysqldump`/`mongodump` cron entries. Skipped when no DB container is running. |

## Compose

Each check first locates a compose file (`docker-compose.prod.yml`, `compose.prod.yml`,
`docker-compose.yml`, `compose.yml`, or `/opt/shuttle/<app>/docker-compose*.yml`).
All compose checks are skipped cleanly when no file is found.

| ID | Severity | What it verifies |
| --- | --- | --- |
| `compose.missing_prod_file` | medium | A production compose file exists in a standard location. |
| `compose.env_file_missing` | medium | Every `env_file:` reference in the compose file resolves to an existing file on the target. |
| `compose.latest_tag_used` | medium | No `image:` directive uses `:latest` or omits its tag. |
| `compose.no_resource_limits` | low | At least one of `mem_limit`, `cpus`, or `deploy.resources.limits` is set somewhere in the file. |
| `compose.bind_mount_sensitive_paths` | high | No service bind-mounts `/var/run/docker.sock`, `/etc`, `/root`, `/proc`, `/sys`, or `/`. |

## Cloudflare

Cloudflare checks require `cloudflare.enabled: true` plus a `cloudflare.zone`
in `.deployshuttle.yml` and an API token (`CLOUDFLARE_API_TOKEN` by default,
or the env name set via `cloudflare.tokenEnv`). Token must have read scope on
Zone / DNS / Zone Settings. All checks skip cleanly when any prerequisite is
missing or the token is rejected.

| ID | Severity | What it verifies |
| --- | --- | --- |
| `cloudflare.ssl_flexible` | critical | SSL/TLS mode is not Off or Flexible (i.e. requests reach the origin over HTTPS). |
| `cloudflare.always_https_disabled` | medium | "Always Use HTTPS" is enabled. |
| `cloudflare.waf_disabled` | medium / info | Managed WAF is enabled (skipped when the plan tier does not expose the setting). |
| `cloudflare.dns_missing` | high | An A / AAAA / CNAME record exists for `app.domain` (or the apex zone). |
| `cloudflare.proxy_disabled` | medium | Host records are proxied (orange cloud) so the origin IP is not exposed. |

## Roadmap

Planned additions tracked in [`plans/03-check-catalog.md`](../plans/03-check-catalog.md)
and prioritized in [`plans/09-critique-and-deltas.md`](../plans/09-critique-and-deltas.md):
`cloudflare.origin_exposed` (direct A records that leak the origin IP),
`db.volume_not_persistent`, backup recency / restore drill / offsite checks,
log rotation, uptime checks.
