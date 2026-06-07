package readiness

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/execx"
)

// sensitiveDockerPorts lists ports that should never be published on 0.0.0.0
// when used in production. Mirrors firewall.database_port_public + Caddy admin.
var sensitiveDockerPorts = []string{"5432", "3306", "6379", "7700", "9200", "27017", "2019"}

// checkUpdatesPending counts pending APT upgrades. Skipped on non-APT hosts.
func checkUpdatesPending(adapter execx.Adapter) CheckResult {
	if adapter.Run("command -v apt >/dev/null 2>&1", time.Second).ExitCode != 0 {
		return CheckResult{
			ID: "system.updates_pending", Title: "No pending package updates",
			Category: "system", Severity: Medium, Status: Skipped,
			Summary: "apt is not available; cannot evaluate pending updates.",
		}
	}
	res := adapter.Run("apt list --upgradable 2>/dev/null | tail -n +2 | wc -l", 5*time.Second)
	count, _ := strconv.Atoi(strings.TrimSpace(res.Stdout))
	status := Passed
	severity := Medium
	summary := "No pending package updates."
	if count > 0 {
		status = Failed
		summary = fmt.Sprintf("%d pending package update(s).", count)
		if count >= 50 {
			severity = High
		}
	}
	return CheckResult{
		ID: "system.updates_pending", Title: "No pending package updates",
		Category: "system", Severity: severity, Status: status,
		Summary:     summary,
		Remediation: "Run 'apt update && apt upgrade' (or rely on unattended-upgrades) to apply pending package updates.",
		Evidence:    map[string]any{"pendingCount": count},
	}
}

// checkMemoryLow warns when available memory is below 10% of total.
func checkMemoryLow(adapter execx.Adapter) CheckResult {
	res := adapter.Run("free -m | awk '/^Mem:/ {print $2, $7}'", 2*time.Second)
	parts := strings.Fields(strings.TrimSpace(res.Stdout))
	if res.ExitCode != 0 || len(parts) < 2 {
		return CheckResult{
			ID: "system.memory_low", Title: "Available memory has enough headroom",
			Category: "system", Severity: Medium, Status: Skipped,
			Summary: "free is not available or returned no data.",
		}
	}
	total, _ := strconv.Atoi(parts[0])
	available, _ := strconv.Atoi(parts[1])
	if total <= 0 {
		return CheckResult{
			ID: "system.memory_low", Title: "Available memory has enough headroom",
			Category: "system", Severity: Medium, Status: Skipped,
			Summary: "Could not parse total memory.",
		}
	}
	pct := (available * 100) / total
	status := Passed
	severity := Medium
	summary := fmt.Sprintf("Available memory is %d%% of total (%d MB / %d MB).", pct, available, total)
	if pct < 5 {
		status = Failed
		severity = High
	} else if pct < 10 {
		status = Failed
	}
	return CheckResult{
		ID: "system.memory_low", Title: "Available memory has enough headroom",
		Category: "system", Severity: severity, Status: status,
		Summary:     summary,
		Remediation: "Add swap, scale up the VPS, or reduce container memory footprint. Investigate workloads with abnormal RSS growth.",
		Evidence:    map[string]any{"totalMB": total, "availableMB": available, "availablePercent": pct},
	}
}

// checkSSHPortDefault flags ssh listening on the default port 22.
// Low severity: not a vulnerability, but a credibility/triage signal.
func checkSSHPortDefault(adapter execx.Adapter) CheckResult {
	res := adapter.Run("test -r /etc/ssh/sshd_config && grep -E '^[[:space:]]*Port[[:space:]]+' /etc/ssh/sshd_config", 2*time.Second)
	if res.ExitCode != 0 {
		// Default is 22 when no Port directive is set; only skip if config unreadable.
		probe := adapter.Run("test -r /etc/ssh/sshd_config", time.Second)
		if probe.ExitCode != 0 {
			return CheckResult{
				ID: "ssh.port_default", Title: "SSH does not listen on the default port",
				Category: "ssh", Severity: Low, Status: Skipped,
				Summary: "Could not read /etc/ssh/sshd_config.",
			}
		}
		return CheckResult{
			ID: "ssh.port_default", Title: "SSH does not listen on the default port",
			Category: "ssh", Severity: Low, Status: Failed,
			Summary:     "No explicit Port directive; sshd defaults to 22.",
			Remediation: "Set 'Port <non-22>' in /etc/ssh/sshd_config and update firewall + agents accordingly.",
			Evidence:    map[string]any{"port": "22 (default)"},
		}
	}
	ports := []string{}
	for _, line := range nonEmptyLines(res.Stdout) {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			ports = append(ports, fields[1])
		}
	}
	if len(ports) == 0 {
		return CheckResult{
			ID: "ssh.port_default", Title: "SSH does not listen on the default port",
			Category: "ssh", Severity: Low, Status: Skipped,
			Summary: "Port directive present but unparseable.",
		}
	}
	default22 := false
	for _, p := range ports {
		if p == "22" {
			default22 = true
			break
		}
	}
	status := Passed
	summary := "SSH listens on: " + strings.Join(ports, ", ") + "."
	if default22 {
		status = Failed
		summary = "SSH listens on the default port 22 (" + strings.Join(ports, ", ") + ")."
	}
	return CheckResult{
		ID: "ssh.port_default", Title: "SSH does not listen on the default port",
		Category: "ssh", Severity: Low, Status: status,
		Summary:     summary,
		Remediation: "Move sshd to a non-default port to reduce drive-by scan noise. Update UFW and SSH client configs accordingly.",
		Evidence:    map[string]any{"ports": ports},
	}
}

// checkDockerPublishedSensitivePorts inspects docker port mappings and flags
// containers publishing sensitive ports on 0.0.0.0.
func checkDockerPublishedSensitivePorts(adapter execx.Adapter) CheckResult {
	res := adapter.Run("docker ps --format '{{.Names}}\t{{.Ports}}' 2>/dev/null", 5*time.Second)
	if res.ExitCode != 0 {
		return CheckResult{
			ID: "firewall.docker_published_sensitive_ports", Title: "Docker does not publish sensitive ports publicly",
			Category: "firewall", Severity: High, Status: Skipped,
			Summary: "Docker is not available; cannot evaluate published ports.",
		}
	}
	exposures := []map[string]string{}
	for _, line := range nonEmptyLines(res.Stdout) {
		name, ports := splitTab2(line)
		for _, port := range sensitiveDockerPorts {
			pattern := regexp.MustCompile(`(?:0\.0\.0\.0|\[::\]):` + port + `->`)
			if pattern.MatchString(ports) {
				exposures = append(exposures, map[string]string{
					"workload": strings.TrimPrefix(name, "/"),
					"port":     port,
				})
			}
		}
	}
	status := Passed
	summary := "No Docker container publishes a sensitive port on 0.0.0.0."
	if len(exposures) > 0 {
		status = Failed
		names := make([]string, 0, len(exposures))
		for _, e := range exposures {
			names = append(names, e["workload"]+":"+e["port"])
		}
		summary = fmt.Sprintf("%d sensitive Docker port mapping(s) on 0.0.0.0: %s.", len(exposures), strings.Join(names, ", "))
	}
	return CheckResult{
		ID: "firewall.docker_published_sensitive_ports", Title: "Docker does not publish sensitive ports publicly",
		Category: "firewall", Severity: High, Status: status,
		Summary:     summary,
		Remediation: "Bind sensitive container ports to 127.0.0.1 (e.g. '127.0.0.1:5432:5432') or to a private Docker network instead of '0.0.0.0'.",
		Evidence:    map[string]any{"exposures": exposures, "sensitivePorts": sensitiveDockerPorts},
	}
}

// caddyfileSearchCommand reads the first Caddyfile found in standard locations.
const caddyfileSearchCommand = "for f in /etc/caddy/Caddyfile /opt/caddy/Caddyfile ./Caddyfile; do if [ -r \"$f\" ]; then echo __caddyfile__=\"$f\"; cat \"$f\"; exit 0; fi; done; exit 1"

// checkCaddySecurityHeaders looks for HSTS + X-Content-Type-Options in Caddyfile.
func checkCaddySecurityHeaders(adapter execx.Adapter) CheckResult {
	res := adapter.Run(caddyfileSearchCommand, 3*time.Second)
	if res.ExitCode != 0 {
		return CheckResult{
			ID: "caddy.no_security_headers", Title: "Caddy emits baseline security headers",
			Category: "reverse-proxy", Severity: Medium, Status: Skipped,
			Summary: "No readable Caddyfile detected (looked in /etc/caddy, /opt/caddy, and current dir).",
		}
	}
	path := capture(res.Stdout, `__caddyfile__=([^\n]+)`)
	body := res.Stdout
	hasHSTS := regexp.MustCompile(`(?i)Strict-Transport-Security`).MatchString(body)
	hasNoSniff := regexp.MustCompile(`(?i)X-Content-Type-Options`).MatchString(body)
	hasReferrer := regexp.MustCompile(`(?i)Referrer-Policy`).MatchString(body)
	missing := []string{}
	if !hasHSTS {
		missing = append(missing, "Strict-Transport-Security")
	}
	if !hasNoSniff {
		missing = append(missing, "X-Content-Type-Options")
	}
	if !hasReferrer {
		missing = append(missing, "Referrer-Policy")
	}
	status := Passed
	summary := "Caddyfile sets baseline security headers."
	if len(missing) > 0 {
		status = Failed
		summary = "Caddyfile is missing baseline security headers: " + strings.Join(missing, ", ") + "."
	}
	return CheckResult{
		ID: "caddy.no_security_headers", Title: "Caddy emits baseline security headers",
		Category: "reverse-proxy", Severity: Medium, Status: status,
		Summary:     summary,
		Remediation: "Add a 'header' block in Caddyfile setting Strict-Transport-Security, X-Content-Type-Options nosniff, and a Referrer-Policy.",
		Evidence:    map[string]any{"caddyfile": strings.TrimSpace(path), "missing": missing},
	}
}

// checkCaddyConfigValid runs `caddy validate` on the active Caddyfile.
func checkCaddyConfigValid(adapter execx.Adapter) CheckResult {
	if adapter.Run("command -v caddy >/dev/null 2>&1", time.Second).ExitCode != 0 {
		return CheckResult{
			ID: "caddy.invalid_config", Title: "Caddy configuration is valid",
			Category: "reverse-proxy", Severity: High, Status: Skipped,
			Summary: "caddy CLI is not available on PATH; skipping validation.",
		}
	}
	probe := adapter.Run("for f in /etc/caddy/Caddyfile /opt/caddy/Caddyfile ./Caddyfile; do [ -r \"$f\" ] && echo \"$f\" && exit 0; done; exit 1", 2*time.Second)
	if probe.ExitCode != 0 {
		return CheckResult{
			ID: "caddy.invalid_config", Title: "Caddy configuration is valid",
			Category: "reverse-proxy", Severity: High, Status: Skipped,
			Summary: "No readable Caddyfile detected for validation.",
		}
	}
	caddyfile := strings.TrimSpace(probe.Stdout)
	res := adapter.Run("caddy validate --config "+caddyfile+" --adapter caddyfile 2>&1", 5*time.Second)
	if res.ExitCode == 0 {
		return CheckResult{
			ID: "caddy.invalid_config", Title: "Caddy configuration is valid",
			Category: "reverse-proxy", Severity: High, Status: Passed,
			Summary:  "caddy validate succeeded for " + caddyfile + ".",
			Evidence: map[string]any{"caddyfile": caddyfile},
		}
	}
	return CheckResult{
		ID: "caddy.invalid_config", Title: "Caddy configuration is valid",
		Category: "reverse-proxy", Severity: High, Status: Failed,
		Summary:     "caddy validate reported errors for " + caddyfile + ".",
		Remediation: "Fix the Caddyfile errors reported by 'caddy validate' before reloading the proxy.",
		Evidence:    map[string]any{"caddyfile": caddyfile, "output": strings.TrimSpace(res.Stdout)},
	}
}

// checkTLSCert verifies the public TLS certificate of cfg.App.Domain.
// Combined check: missing cert OR expiring soon.
func checkTLSCertificate(domain string) Check {
	return func(adapter execx.Adapter) CheckResult {
		if domain == "" {
			return CheckResult{
				ID: "tls.cert_missing", Title: "TLS certificate is present and valid",
				Category: "tls", Severity: Critical, Status: Skipped,
				Summary: "No app.domain configured in .shuttle.yml; skipping TLS check.",
			}
		}
		if adapter.Run("command -v openssl >/dev/null 2>&1", time.Second).ExitCode != 0 {
			return CheckResult{
				ID: "tls.cert_missing", Title: "TLS certificate is present and valid",
				Category: "tls", Severity: Critical, Status: Skipped,
				Summary: "openssl is not available; cannot evaluate TLS certificate.",
			}
		}
		cmd := fmt.Sprintf("echo | openssl s_client -servername %s -connect %s:443 -verify_return_error 2>/dev/null | openssl x509 -noout -enddate -subject 2>/dev/null", domain, domain)
		res := adapter.Run(cmd, 8*time.Second)
		if res.ExitCode != 0 || strings.TrimSpace(res.Stdout) == "" {
			return CheckResult{
				ID: "tls.cert_missing", Title: "TLS certificate is present and valid",
				Category: "tls", Severity: Critical, Status: Failed,
				Summary:     "Could not retrieve a TLS certificate for " + domain + ".",
				Remediation: "Verify the domain points to this server, that ports 80/443 are open, and that Caddy/your reverse proxy issued a certificate.",
				Evidence:    map[string]any{"domain": domain},
			}
		}
		notAfter := capture(res.Stdout, `notAfter=([^\n]+)`)
		end, err := time.Parse("Jan 2 15:04:05 2006 MST", strings.TrimSpace(notAfter))
		if err != nil {
			return CheckResult{
				ID: "tls.cert_missing", Title: "TLS certificate is present and valid",
				Category: "tls", Severity: Critical, Status: Unknown,
				Summary:  "Could not parse certificate expiration date.",
				Evidence: map[string]any{"domain": domain, "notAfter": notAfter},
			}
		}
		days := int(time.Until(end).Hours() / 24)
		status := Passed
		severity := Medium
		summary := fmt.Sprintf("Certificate for %s is valid; expires in %d day(s).", domain, days)
		remediation := ""
		if days < 0 {
			status = Failed
			severity = Critical
			summary = fmt.Sprintf("Certificate for %s expired %d day(s) ago.", domain, -days)
			remediation = "Renew the TLS certificate immediately; users get a browser warning until then."
		} else if days < 14 {
			status = Failed
			severity = High
			summary = fmt.Sprintf("Certificate for %s expires in %d day(s).", domain, days)
			remediation = "Investigate ACME renewal failure (Caddy logs, Cloudflare DNS, ports 80/443) before the cert expires."
		}
		return CheckResult{
			ID: "tls.cert_missing", Title: "TLS certificate is present and valid",
			Category: "tls", Severity: severity, Status: status,
			Summary:     summary,
			Remediation: remediation,
			Evidence:    map[string]any{"domain": domain, "notAfter": notAfter, "daysRemaining": days},
		}
	}
}

// checkSecretsWeakPermissions finds env/key files in the project tree that are world-readable.
func checkSecretsWeakPermissions(adapter execx.Adapter) CheckResult {
	cmd := "find . -maxdepth 3 -type f \\( -name '.env*' -o -name '*.pem' -o -name '*.key' -o -name 'id_rsa' -o -name 'id_ed25519' \\) -perm /o+r 2>/dev/null"
	res := adapter.Run(cmd, 3*time.Second)
	if res.ExitCode != 0 {
		return CheckResult{
			ID: "secrets.weak_file_permissions", Title: "Secret files are not world-readable",
			Category: "secrets", Severity: High, Status: Skipped,
			Summary: "find is not available or returned an error.",
		}
	}
	files := nonEmptyLines(res.Stdout)
	status := Passed
	summary := "No world-readable secret files detected in the project tree."
	if len(files) > 0 {
		status = Failed
		summary = fmt.Sprintf("%d world-readable secret file(s): %s.", len(files), strings.Join(files, ", "))
	}
	return CheckResult{
		ID: "secrets.weak_file_permissions", Title: "Secret files are not world-readable",
		Category: "secrets", Severity: High, Status: status,
		Summary:     summary,
		Remediation: "chmod 600 the affected secret files (env/keys/PEM) and ensure their containing directory is not world-listable.",
		Evidence:    map[string]any{"files": files},
	}
}

// checkHSTSHeader probes the public URL and verifies a Strict-Transport-Security header.
func checkHSTSHeader(domain string) Check {
	return func(adapter execx.Adapter) CheckResult {
		if domain == "" {
			return CheckResult{
				ID: "tls.hsts_missing", Title: "HSTS header is enforced",
				Category: "tls", Severity: Medium, Status: Skipped,
				Summary: "No app.domain configured; skipping HSTS probe.",
			}
		}
		if adapter.Run("command -v curl >/dev/null 2>&1", time.Second).ExitCode != 0 {
			return CheckResult{
				ID: "tls.hsts_missing", Title: "HSTS header is enforced",
				Category: "tls", Severity: Medium, Status: Skipped,
				Summary: "curl is not available on the target.",
			}
		}
		url := "https://" + domain
		res := adapter.Run("curl -sI --max-time 5 "+url+" | grep -i '^Strict-Transport-Security'", 8*time.Second)
		if res.ExitCode == 0 && strings.TrimSpace(res.Stdout) != "" {
			return CheckResult{
				ID: "tls.hsts_missing", Title: "HSTS header is enforced",
				Category: "tls", Severity: Medium, Status: Passed,
				Summary:  "HSTS header present on " + url + ".",
				Evidence: map[string]any{"url": url, "header": strings.TrimSpace(res.Stdout)},
			}
		}
		return CheckResult{
			ID: "tls.hsts_missing", Title: "HSTS header is enforced",
			Category: "tls", Severity: Medium, Status: Failed,
			Summary:     "No Strict-Transport-Security header on " + url + ".",
			Remediation: "Add 'header Strict-Transport-Security \"max-age=31536000; includeSubDomains\"' in the Caddyfile (or equivalent in your reverse proxy).",
			Evidence:    map[string]any{"url": url},
		}
	}
}

// checkDNSPointsToServer compares the A record for app.domain with the server's
// public IP. Skipped if either side cannot be determined.
func checkDNSPointsToServer(domain string) Check {
	return func(adapter execx.Adapter) CheckResult {
		if domain == "" {
			return CheckResult{
				ID: "dns.domain_not_pointing_to_server", Title: "DNS A record points to this server",
				Category: "dns", Severity: Medium, Status: Skipped,
				Summary: "No app.domain configured; skipping DNS check.",
			}
		}
		if adapter.Run("command -v dig >/dev/null 2>&1", time.Second).ExitCode != 0 {
			return CheckResult{
				ID: "dns.domain_not_pointing_to_server", Title: "DNS A record points to this server",
				Category: "dns", Severity: Medium, Status: Skipped,
				Summary: "dig is not available on the target.",
			}
		}
		dnsRes := adapter.Run("dig +short A "+domain+" | tail -n 1", 5*time.Second)
		dnsIP := strings.TrimSpace(dnsRes.Stdout)
		if dnsIP == "" {
			return CheckResult{
				ID: "dns.domain_not_pointing_to_server", Title: "DNS A record points to this server",
				Category: "dns", Severity: Medium, Status: Failed,
				Summary:     "dig returned no A record for " + domain + ".",
				Remediation: "Create an A record for " + domain + " pointing at this server's public IP.",
				Evidence:    map[string]any{"domain": domain},
			}
		}
		ipRes := adapter.Run("curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || curl -fsS --max-time 5 https://ifconfig.me 2>/dev/null", 8*time.Second)
		serverIP := strings.TrimSpace(ipRes.Stdout)
		if serverIP == "" {
			return CheckResult{
				ID: "dns.domain_not_pointing_to_server", Title: "DNS A record points to this server",
				Category: "dns", Severity: Medium, Status: Skipped,
				Summary:  "Could not determine the server's public IP; DNS A record is " + dnsIP + ".",
				Evidence: map[string]any{"domain": domain, "dnsIP": dnsIP},
			}
		}
		if dnsIP == serverIP {
			return CheckResult{
				ID: "dns.domain_not_pointing_to_server", Title: "DNS A record points to this server",
				Category: "dns", Severity: Medium, Status: Passed,
				Summary:  domain + " resolves to this server (" + serverIP + ").",
				Evidence: map[string]any{"domain": domain, "dnsIP": dnsIP, "serverIP": serverIP},
			}
		}
		// Mismatch may be Cloudflare-proxied; lower severity to medium with a clear note.
		return CheckResult{
			ID: "dns.domain_not_pointing_to_server", Title: "DNS A record points to this server",
			Category: "dns", Severity: Medium, Status: Failed,
			Summary:     fmt.Sprintf("%s resolves to %s but server's public IP is %s.", domain, dnsIP, serverIP),
			Remediation: "Update the A record to point at this server, or — if you front the origin with Cloudflare — confirm the orange-cloud proxy is intentional and origin access is restricted.",
			Evidence:    map[string]any{"domain": domain, "dnsIP": dnsIP, "serverIP": serverIP},
		}
	}
}

// checkDatabaseBackup detects a recent backup artifact or a backup-related cron job.
func checkDatabaseBackup(adapter execx.Adapter) CheckResult {
	hasDocker := adapter.Run("command -v docker >/dev/null 2>&1 && docker ps --format '{{.Image}}' 2>/dev/null | grep -Ei 'postgres|mysql|mariadb|mongo' >/dev/null", 3*time.Second).ExitCode == 0
	if !hasDocker {
		return CheckResult{
			ID: "db.no_backup_detected", Title: "Database backups are configured",
			Category: "backups", Severity: High, Status: Skipped,
			Summary: "No running database container detected; skipping backup heuristic.",
		}
	}
	artifactsRes := adapter.Run("find /backups /var/backups /opt/backups -type f \\( -name '*.sql*' -o -name '*.dump' -o -name '*.tar*' \\) -mtime -7 2>/dev/null | head -n 5", 5*time.Second)
	artifacts := nonEmptyLines(artifactsRes.Stdout)
	cronRes := adapter.Run("(crontab -l 2>/dev/null; cat /etc/crontab 2>/dev/null; cat /etc/cron.d/* 2>/dev/null) | grep -Ei 'pg_dump|mysqldump|mongodump|mariadb-dump|pg_basebackup' | head -n 5", 3*time.Second)
	cronEntries := nonEmptyLines(cronRes.Stdout)
	if len(artifacts) > 0 || len(cronEntries) > 0 {
		return CheckResult{
			ID: "db.no_backup_detected", Title: "Database backups are configured",
			Category: "backups", Severity: High, Status: Passed,
			Summary:  fmt.Sprintf("Found %d recent backup artifact(s) and %d backup-related cron entry/entries.", len(artifacts), len(cronEntries)),
			Evidence: map[string]any{"artifacts": artifacts, "cronEntries": cronEntries},
		}
	}
	return CheckResult{
		ID: "db.no_backup_detected", Title: "Database backups are configured",
		Category: "backups", Severity: High, Status: Failed,
		Summary:     "No recent backup artifacts (in /backups, /var/backups, /opt/backups) and no pg_dump/mysqldump cron entry detected.",
		Remediation: "Schedule a daily 'pg_dump' (or mysqldump) cron that writes to /backups/ and ship the output offsite (S3, B2, rsync). Document the restore procedure.",
		Evidence:    map[string]any{},
	}
}

// checkHealthEndpoint probes cfg.App.Domain + cfg.App.HealthcheckPath for a 2xx response.
func checkHealthEndpoint(domain string, path string) Check {
	return func(adapter execx.Adapter) CheckResult {
		if domain == "" || path == "" {
			return CheckResult{
				ID: "monitoring.no_health_endpoint", Title: "App exposes a health endpoint",
				Category: "monitoring", Severity: Medium, Status: Skipped,
				Summary: "app.domain or app.healthcheckPath is missing; skipping health probe.",
			}
		}
		if adapter.Run("command -v curl >/dev/null 2>&1", time.Second).ExitCode != 0 {
			return CheckResult{
				ID: "monitoring.no_health_endpoint", Title: "App exposes a health endpoint",
				Category: "monitoring", Severity: Medium, Status: Skipped,
				Summary: "curl is not available on the target.",
			}
		}
		url := "https://" + domain + path
		res := adapter.Run("curl -fsS --max-time 5 -o /dev/null -w '%{http_code}' "+url, 8*time.Second)
		code := strings.TrimSpace(res.Stdout)
		if res.ExitCode == 0 && strings.HasPrefix(code, "2") {
			return CheckResult{
				ID: "monitoring.no_health_endpoint", Title: "App exposes a health endpoint",
				Category: "monitoring", Severity: Medium, Status: Passed,
				Summary:  url + " returned HTTP " + code + ".",
				Evidence: map[string]any{"url": url, "httpCode": code},
			}
		}
		return CheckResult{
			ID: "monitoring.no_health_endpoint", Title: "App exposes a health endpoint",
			Category: "monitoring", Severity: Medium, Status: Failed,
			Summary:     url + " did not return a 2xx response (got " + fallback(code, "n/a") + ").",
			Remediation: "Implement a /health endpoint returning 200 OK and reference it via app.healthcheckPath in .shuttle.yml.",
			Evidence:    map[string]any{"url": url, "httpCode": code},
		}
	}
}
