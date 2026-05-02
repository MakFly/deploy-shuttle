# Check Catalog

DeployShuttle ships **21 production-readiness checks** out of the box.
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
| `system.unattended_upgrades_inactive` | medium | `unattended-upgrades` package is installed and the service is enabled. |
| `system.fail2ban_inactive` | medium | `fail2ban` is installed and active to throttle SSH brute-force attempts. |
| `system.swap_missing` | low | At least one swap device or file is configured. |
| `system.time_sync_inactive` | medium | A time synchronization daemon (`systemd-timesyncd`, `chrony`, `ntp`) is active. |

## SSH

| ID | Severity | What it verifies |
| --- | --- | --- |
| `ssh.root_login_enabled` | high | `PermitRootLogin` is `no` or `prohibit-password` in `/etc/ssh/sshd_config`. |
| `ssh.password_auth_enabled` | high | `PasswordAuthentication` is `no` so only key-based logins are accepted. |

## Docker

| ID | Severity | What it verifies |
| --- | --- | --- |
| `docker.not_installed` | high | Docker Engine binary is reachable. |
| `docker.service_not_enabled` | high | `docker.service` is enabled at boot and active. |
| `docker.containers_without_restart_policy` | high | Every workload (classic + Swarm) has a restart policy. |
| `docker.containers_without_healthcheck` | medium | Every workload defines a `HEALTHCHECK` (or compose/swarm `healthcheck`). |
| `docker.containers_running_as_root` | medium | Workloads do not run as root unless allow-listed via `docker.allowRoot`. |
| `docker.sock_exposed` | high | Workloads do not mount `/var/run/docker.sock` read-write unless allow-listed via `docker.allowDockerSocket`. |

## Firewall

| ID | Severity | What it verifies |
| --- | --- | --- |
| `firewall.ufw_inactive` | high | UFW is installed and active with deny-by-default. |
| `firewall.database_port_public` | high / critical | Sensitive DB ports (Postgres, MySQL, MongoDB, Redis) are not publicly reachable. Downgraded when UFW restricts the port. |

## Secrets

| ID | Severity | What it verifies |
| --- | --- | --- |
| `secrets.env_world_readable` | critical | `.env` is not world-readable. Auto-fix available via `harden --apply`. |
| `secrets.env_in_git` | critical | `.env` is not tracked by Git. |

## Reverse proxy / database

| ID | Severity | What it verifies |
| --- | --- | --- |
| `caddy.not_installed` | medium | Caddy binary or container is present. |
| `caddy.admin_exposed` | critical | Caddy admin API (`:2019`) is not reachable from the public network. |
| `adminer.ip_restriction_missing` | high | A running Adminer container is protected by IP allow-list, deny-by-default rule, and basic auth. |

## Roadmap

Planned additions tracked in [`plans/03-check-catalog.md`](../plans/03-check-catalog.md):
SSH hardening (PermitRootLogin, password auth), automatic security upgrades, fail2ban,
swap, time sync, log rotation, backups detection, monitoring presence.
