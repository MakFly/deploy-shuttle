# MVP Check Catalog

## 1. System Checks

### `system.os_supported`

Detect if OS is supported.

Supported MVP:

- Ubuntu 22.04;
- Ubuntu 24.04;
- Debian 12.

### `system.updates_pending`

Detect pending security updates.

### `system.disk_space_low`

Warn if disk usage > 80%. Critical if > 90%.

### `system.memory_low`

Warn if available memory is below threshold.

### `system.time_sync`

Verify NTP/time sync is enabled.

## 2. SSH Checks

### `ssh.root_login_enabled`

Warn or fail if root SSH login is enabled.

### `ssh.password_auth_enabled`

High severity if password auth is enabled.

### `ssh.port_default`

Low/medium warning if SSH is on port 22. Not always bad, but should be noted.

### `ssh.fail2ban_missing`

Warn if fail2ban or equivalent is missing.

## 3. Firewall Checks

### `firewall.ufw_inactive`

High severity if no firewall is active.

### `firewall.database_port_public`

Critical if PostgreSQL, MySQL, Redis or Meilisearch are exposed publicly.

Ports:

- 5432;
- 3306;
- 6379;
- 7700;
- 9200;
- 27017.

### `firewall.docker_published_sensitive_ports`

Detect Docker containers publishing sensitive ports to `0.0.0.0`.

## 4. Docker Checks

### `docker.not_installed`

Info/high depending on selected profile.

### `docker.service_not_enabled`

Docker not enabled on boot.

### `docker.containers_without_restart_policy`

High severity for app containers without restart policy.

### `docker.containers_without_healthcheck`

Medium/high severity.

### `docker.containers_running_as_root`

Medium/high depending on image.

### `docker.sock_exposed`

Critical if Docker socket is mounted into a container without clear reason.

### `docker.unused_images_large`

Info/medium cleanup recommendation.

## 5. Docker Compose Checks

### `compose.missing_prod_file`

Detect absence of `docker-compose.prod.yml` or compose file.

### `compose.env_file_missing`

Detect referenced env files that do not exist.

### `compose.latest_tag_used`

Warn if production image uses `latest`.

### `compose.no_resource_limits`

Warn when no memory/cpu limits are set.

### `compose.bind_mount_sensitive_paths`

Warn if host sensitive paths are mounted.

## 6. Reverse Proxy Checks

MVP reverse proxy target: Caddy.

### `caddy.not_installed`

Warn if no Caddy/Nginx/Traefik detected.

### `caddy.admin_exposed`

Critical if Caddy admin API listens publicly.

### `caddy.no_security_headers`

Warn if no HSTS, X-Content-Type-Options, Referrer-Policy.

### `caddy.no_access_logs`

Warn if logs disabled.

### `caddy.invalid_config`

Run `caddy validate` if available.

## 7. TLS / Domain Checks

### `tls.cert_missing`

Critical for public app.

### `tls.cert_expiring_soon`

Warn if cert expires in < 14 days.

### `tls.hsts_missing`

Medium.

### `dns.domain_not_pointing_to_server`

Warn if domain A/AAAA mismatch.

## 8. Cloudflare Checks

Cloudflare checks require API token or manual mode.

### `cloudflare.ssl_flexible`

Critical if Cloudflare SSL mode is Flexible.

### `cloudflare.proxy_disabled`

Warn if orange cloud disabled for public app.

### `cloudflare.always_https_disabled`

Medium.

### `cloudflare.waf_disabled`

Info/medium depending on plan.

### `cloudflare.dns_missing`

Detect missing A/CNAME records.

### `cloudflare.origin_exposed`

Warn if origin IP is exposed through direct DNS records.

## 9. Database Checks

### `db.postgres_public`

Critical.

### `db.redis_public`

Critical.

### `db.no_backup_detected`

High.

### `db.backup_not_recent`

High if last backup > 24h or unknown.

### `db.volume_not_persistent`

Critical if DB container has no persistent volume.

## 10. Secrets Checks

### `secrets.env_world_readable`

Critical if `.env` is world-readable.

### `secrets.env_in_git`

Critical if `.env` tracked by Git.

### `secrets.weak_file_permissions`

High.

### `secrets.example_missing`

Low if no `.env.example`.

## 11. Monitoring Checks

### `monitoring.no_health_endpoint`

Medium.

### `monitoring.no_uptime_check`

Medium.

### `monitoring.no_log_rotation`

Medium.

### `monitoring.no_alerting`

Medium.

## 12. Backup Checks

### `backup.no_strategy`

High.

### `backup.no_restore_test`

Medium.

### `backup.local_only`

Medium if backup exists only on same VPS.

### `backup.no_retention_policy`

Medium.
