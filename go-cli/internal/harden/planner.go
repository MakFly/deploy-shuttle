package harden

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/readiness"
)

type Action struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	SourceCheck string   `json:"sourceCheck"`
	Severity    string   `json:"severity"`
	Rationale   string   `json:"rationale"`
	Commands    []string `json:"commands,omitempty"`
	Notes       []string `json:"notes,omitempty"`
	// SafeAutoApply is true when Commands are safe to run automatically
	// (idempotent, scoped, reversible). Only these actions are eligible
	// for `harden --apply`. The same flag governs local and SSH execution.
	SafeAutoApply bool `json:"safeAutoApply,omitempty"`
}

type Plan struct {
	Target      string   `json:"target"`
	Score       int      `json:"score"`
	Level       string   `json:"level"`
	GeneratedAt string   `json:"generatedAt"`
	Actions     []Action `json:"actions"`
}

func BuildPlan(report readiness.Report) Plan {
	actions := []Action{}
	for _, check := range report.Checks {
		if check.Status != readiness.Failed || check.Ignored {
			continue
		}
		actions = append(actions, actionsFor(check)...)
	}
	sort.SliceStable(actions, func(i, j int) bool {
		if actions[i].Category != actions[j].Category {
			return actions[i].Category < actions[j].Category
		}
		return actions[i].ID < actions[j].ID
	})
	return Plan{
		Target:      report.Target,
		Score:       report.Score,
		Level:       string(report.Level),
		GeneratedAt: report.GeneratedAt,
		Actions:     actions,
	}
}

func actionsFor(check readiness.CheckResult) []Action {
	switch check.ID {
	case "firewall.ufw_inactive":
		return []Action{{
			ID:          "ufw.enable-baseline",
			Title:       "Enable UFW with SSH/HTTP/HTTPS baseline",
			Category:    "firewall",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "UFW is missing or inactive; without a deny-by-default firewall, every bound service is reachable from the internet.",
			Commands: []string{
				"ufw default deny incoming",
				"ufw default allow outgoing",
				"ufw allow OpenSSH",
				"ufw allow 80/tcp",
				"ufw allow 443/tcp",
				"ufw --force enable",
			},
			Notes: []string{"Confirm the SSH port allow rule matches the actual sshd port before enabling UFW."},
		}}
	case "firewall.database_port_public":
		ports := stringSlice(check.Evidence["publicPorts"])
		commands := []string{}
		for _, port := range ports {
			commands = append(commands, fmt.Sprintf("ufw deny %s/tcp", port))
		}
		notes := []string{
			"Prefer binding databases to 127.0.0.1 or a private Docker network instead of 0.0.0.0.",
			"If remote DB access is required, allow only known admin IPs explicitly.",
		}
		safe := len(commands) > 0
		return []Action{{
			ID:            "firewall.lock-db-ports",
			Title:         "Restrict public database ports",
			Category:      "firewall",
			SourceCheck:   check.ID,
			Severity:      string(check.Severity),
			Rationale:     "Database ports are publicly reachable or publicly allowed; close them or restrict to trusted IPs.",
			Commands:      commands,
			Notes:         notes,
			SafeAutoApply: safe,
		}}
	case "secrets.env_world_readable":
		return []Action{{
			ID:            "secrets.tighten-env-perms",
			Title:         "Tighten .env file permissions",
			Category:      "secrets",
			SourceCheck:   check.ID,
			Severity:      string(check.Severity),
			Rationale:     ".env contains production secrets and must not be world-readable.",
			Commands:      []string{"chmod 600 .env"},
			SafeAutoApply: true,
		}}
	case "caddy.admin_exposed":
		return []Action{{
			ID:          "caddy.lock-admin-api",
			Title:       "Restrict the Caddy admin API to localhost",
			Category:    "reverse-proxy",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "The Caddy admin API allows full reconfiguration; it must not be reachable from the internet.",
			Notes: []string{
				"Set 'admin 127.0.0.1:2019' (or 'admin off') in the global block of Caddyfile.",
				"Reload Caddy after the change: caddy reload --config /etc/caddy/Caddyfile.",
			},
		}}
	case "docker.containers_without_healthcheck":
		workloads := stringSlice(check.Evidence["workloads"])
		notes := []string{
			"Add a HEALTHCHECK directive in each image, or 'healthcheck:' in the compose/swarm spec.",
			"For long-running workers without an HTTP surface, expose a /healthz endpoint or a CLI probe.",
		}
		if len(workloads) > 0 {
			notes = append(notes, "Workloads missing a healthcheck: "+strings.Join(workloads, ", "))
		}
		return []Action{{
			ID:          "docker.add-healthchecks",
			Title:       "Add Docker healthchecks for production workloads",
			Category:    "docker",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Workloads without healthchecks make rolling restarts and supervision unreliable.",
			Notes:       notes,
		}}
	case "docker.containers_running_as_root":
		workloads := stringSlice(check.Evidence["workloads"])
		notes := []string{
			"Add 'USER appuser' to the Dockerfile or 'user:' in the compose/swarm spec.",
			"For images that must run as root, document the reason and add the workload to docker.allowRoot in .shuttle.yml.",
		}
		if len(workloads) > 0 {
			notes = append(notes, "Workloads running as root: "+strings.Join(workloads, ", "))
		}
		return []Action{{
			ID:          "docker.drop-root",
			Title:       "Run Docker workloads as a non-root user",
			Category:    "docker",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Containers running as root expand the blast radius of a container escape.",
			Notes:       notes,
		}}
	case "docker.sock_exposed":
		readWrite := stringSlice(check.Evidence["readWriteWorkloads"])
		notes := []string{
			"Mount /var/run/docker.sock read-only when possible (':ro').",
			"For workloads that genuinely orchestrate Docker (CI runners, autoheal), add them to docker.allowDockerSocket in .shuttle.yml.",
		}
		if len(readWrite) > 0 {
			notes = append(notes, "Workloads with read-write socket mounts: "+strings.Join(readWrite, ", "))
		}
		return []Action{{
			ID:          "docker.review-socket-mounts",
			Title:       "Review workloads mounting /var/run/docker.sock",
			Category:    "docker",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Read-write access to the Docker socket is equivalent to root on the host.",
			Notes:       notes,
		}}
	case "docker.containers_without_restart_policy":
		workloads := stringSlice(check.Evidence["workloads"])
		notes := []string{
			"Set 'restart: unless-stopped' in compose, or RestartPolicy { Condition: any } for swarm services.",
		}
		if len(workloads) > 0 {
			notes = append(notes, "Workloads without restart policy: "+strings.Join(workloads, ", "))
		}
		return []Action{{
			ID:          "docker.set-restart-policy",
			Title:       "Set a restart policy on production workloads",
			Category:    "docker",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Workloads without a restart policy will not recover from crashes or reboots.",
			Notes:       notes,
		}}
	case "ssh.root_login_enabled":
		return []Action{{
			ID:          "ssh.disable-root-login",
			Title:       "Disable SSH root password login",
			Category:    "ssh",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "PermitRootLogin yes accepts root password authentication, the most common credential-stuffing target.",
			Notes: []string{
				"Edit /etc/ssh/sshd_config to set 'PermitRootLogin prohibit-password' (or 'no' if you do not need root over SSH at all).",
				"Validate the new config with 'sshd -t' before reloading: 'systemctl reload sshd'.",
				"Make sure key-based admin access works in another terminal before disconnecting the current session.",
			},
		}}
	case "ssh.password_auth_enabled":
		return []Action{{
			ID:          "ssh.disable-password-auth",
			Title:       "Require SSH key authentication",
			Category:    "ssh",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "PasswordAuthentication yes lets attackers brute-force any account; key-based logins eliminate that surface.",
			Notes: []string{
				"Confirm every operator's authorized_keys entry works before disabling passwords.",
				"Set 'PasswordAuthentication no' (and check 'KbdInteractiveAuthentication no') in /etc/ssh/sshd_config.",
				"Run 'sshd -t' then 'systemctl reload sshd'.",
			},
		}}
	case "system.unattended_upgrades_inactive":
		return []Action{{
			ID:          "system.enable-unattended-upgrades",
			Title:       "Enable automatic security updates",
			Category:    "system",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Without unattended-upgrades, the host accumulates known security CVEs between manual maintenance windows.",
			Notes: []string{
				"Debian/Ubuntu: 'apt install unattended-upgrades' then 'dpkg-reconfigure -plow unattended-upgrades'.",
				"Verify '/etc/apt/apt.conf.d/20auto-upgrades' has 'Unattended-Upgrade::Allowed-Origins' covering the security suite.",
			},
		}}
	case "system.fail2ban_inactive":
		return []Action{{
			ID:          "system.enable-fail2ban",
			Title:       "Enable fail2ban",
			Category:    "system",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "fail2ban throttles SSH brute-force attacks and can be tuned to protect Caddy/Nginx logins as well.",
			Notes: []string{
				"Install and enable: 'apt install fail2ban && systemctl enable --now fail2ban'.",
				"Confirm the sshd jail is active: 'fail2ban-client status sshd'.",
			},
		}}
	case "system.swap_missing":
		return []Action{{
			ID:          "system.add-swap",
			Title:       "Add a swap file",
			Category:    "system",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "On small VPS, a swap file lets the kernel reclaim memory before OOM-killing production workloads.",
			Notes: []string{
				"Create a 1-2 GB swap file: 'fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile'.",
				"Persist by adding '/swapfile none swap sw 0 0' to /etc/fstab.",
			},
		}}
	case "system.time_sync_inactive":
		return []Action{{
			ID:          "system.enable-time-sync",
			Title:       "Enable time synchronization",
			Category:    "system",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Drifting clocks break TLS handshakes, log correlation, and rate-limited APIs.",
			Notes: []string{
				"Easiest path on Debian/Ubuntu: 'systemctl enable --now systemd-timesyncd'.",
				"Verify with 'timedatectl status' (System clock synchronized: yes).",
			},
		}}
	case "system.updates_pending":
		count := intEvidence(check.Evidence["pendingCount"])
		notes := []string{
			"Schedule a maintenance window: 'apt update && apt -y upgrade'.",
			"Pair this with 'system.unattended_upgrades_inactive' so future security CVEs land automatically.",
		}
		if count > 0 {
			notes = append(notes, fmt.Sprintf("Pending package updates: %d.", count))
		}
		return []Action{{
			ID:          "system.apply-pending-updates",
			Title:       "Apply pending package updates",
			Category:    "system",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Pending APT updates leave known CVEs unpatched until the next manual maintenance window.",
			Notes:       notes,
		}}
	case "system.memory_low":
		pct := intEvidence(check.Evidence["availablePercent"])
		notes := []string{
			"Add or grow a swap file (2-4 GB on small VPS).",
			"Audit container memory ceilings; long-tail RSS growth is the usual culprit.",
			"If memory pressure is structural, scale the VPS plan rather than hide it with swap.",
		}
		if pct > 0 {
			notes = append(notes, fmt.Sprintf("Current available memory: %d%% of total.", pct))
		}
		return []Action{{
			ID:          "system.relieve-memory-pressure",
			Title:       "Relieve memory pressure",
			Category:    "system",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Sustained low free memory triggers OOM kills that take random workloads down.",
			Notes:       notes,
		}}
	case "ssh.port_default":
		return []Action{{
			ID:          "ssh.move-off-default-port",
			Title:       "Move sshd off the default port 22",
			Category:    "ssh",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Port 22 absorbs constant credential-stuffing scans; moving sshd is a free signal-to-noise reduction.",
			Notes: []string{
				"Pick a port in 1024-65535, set 'Port <port>' in /etc/ssh/sshd_config.",
				"Open the new port in UFW BEFORE reloading sshd: 'ufw allow <port>/tcp'.",
				"Update SSH client configs and CI runners (~/.ssh/config, deploy keys).",
				"Reload with 'sshd -t && systemctl reload sshd'; keep the existing session open until you confirm the new port works.",
			},
		}}
	case "firewall.docker_published_sensitive_ports":
		pairs := exposurePairs(check.Evidence["exposures"])
		notes := []string{
			"Replace '0.0.0.0:<port>:<port>' with '127.0.0.1:<port>:<port>' in the compose file (or attach the service to an internal Docker network instead of publishing).",
			"Recreate the container after editing: 'docker compose up -d <service>'.",
		}
		if len(pairs) > 0 {
			notes = append(notes, "Containers exposing sensitive ports: "+strings.Join(pairs, ", "))
		}
		return []Action{{
			ID:          "firewall.unpublish-sensitive-ports",
			Title:       "Stop publishing sensitive Docker ports on 0.0.0.0",
			Category:    "firewall",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Docker port publishes bypass UFW; binding to 0.0.0.0 puts the database directly on the public internet.",
			Notes:       notes,
		}}
	case "caddy.no_security_headers":
		missing := stringSlice(check.Evidence["missing"])
		caddyfile := stringEvidence(check.Evidence["caddyfile"])
		notes := []string{
			"Add a global 'header' block in the Caddyfile, scoped to the public site:",
			"  Strict-Transport-Security \"max-age=31536000; includeSubDomains\"",
			"  X-Content-Type-Options \"nosniff\"",
			"  Referrer-Policy \"strict-origin-when-cross-origin\"",
			"Reload: 'caddy reload --config " + fallback(caddyfile, "/etc/caddy/Caddyfile") + "'.",
		}
		if len(missing) > 0 {
			notes = append(notes, "Missing headers: "+strings.Join(missing, ", "))
		}
		return []Action{{
			ID:          "caddy.add-security-headers",
			Title:       "Add baseline security headers to Caddyfile",
			Category:    "reverse-proxy",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Browsers rely on these headers to enforce HTTPS, block MIME-sniffing attacks, and control referrer leakage.",
			Notes:       notes,
		}}
	case "caddy.invalid_config":
		caddyfile := stringEvidence(check.Evidence["caddyfile"])
		output := stringEvidence(check.Evidence["output"])
		notes := []string{
			"Run 'caddy validate --config " + fallback(caddyfile, "/etc/caddy/Caddyfile") + "' to reproduce the error locally.",
			"Fix the syntax error reported above before reloading; an invalid reload leaves Caddy on the old config but blocks future automated changes.",
		}
		if output != "" {
			notes = append(notes, "Validate output: "+output)
		}
		return []Action{{
			ID:          "caddy.fix-invalid-config",
			Title:       "Fix Caddy configuration errors",
			Category:    "reverse-proxy",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "An invalid Caddyfile blocks future reloads and may already be serving stale config silently.",
			Notes:       notes,
		}}
	case "tls.cert_missing":
		domain := stringEvidence(check.Evidence["domain"])
		days := intEvidence(check.Evidence["daysRemaining"])
		notes := []string{
			"Verify the A/AAAA records for " + fallback(domain, "<domain>") + " point to this server.",
			"Confirm ports 80 and 443 are open in UFW (ACME HTTP-01 challenge needs port 80).",
			"Inspect Caddy logs: 'journalctl -u caddy -n 200' or 'docker logs caddy'.",
			"If using Cloudflare, set SSL/TLS mode to 'Full (strict)', not 'Flexible'.",
		}
		if days < 0 {
			notes = append(notes, fmt.Sprintf("Certificate expired %d day(s) ago; renewal is overdue.", -days))
		} else if days > 0 {
			notes = append(notes, fmt.Sprintf("Certificate expires in %d day(s); fix renewal before cutover.", days))
		}
		return []Action{{
			ID:          "tls.restore-certificate",
			Title:       "Restore a valid TLS certificate",
			Category:    "tls",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Without a valid certificate, every browser shows a security warning and ACME-aware tools will refuse to talk to the host.",
			Notes:       notes,
		}}
	case "secrets.weak_file_permissions":
		files := stringSlice(check.Evidence["files"])
		commands := []string{}
		for _, file := range files {
			commands = append(commands, "chmod 600 "+file)
		}
		notes := []string{
			"Each command tightens a single secret file to mode 600 (owner read/write only).",
			"Audit the parent directory mode too — a 0755 dir on a 0600 file still leaks the filename.",
		}
		if len(files) > 0 {
			notes = append(notes, "Files: "+strings.Join(files, ", "))
		}
		return []Action{{
			ID:            "secrets.tighten-secret-perms",
			Title:         "Tighten permissions on env / key / pem files",
			Category:      "secrets",
			SourceCheck:   check.ID,
			Severity:      string(check.Severity),
			Rationale:     "Secret files must not be world-readable; any user on the host can otherwise dump them.",
			Commands:      commands,
			Notes:         notes,
			SafeAutoApply: len(commands) > 0,
		}}
	case "monitoring.no_health_endpoint":
		url := stringEvidence(check.Evidence["url"])
		code := stringEvidence(check.Evidence["httpCode"])
		notes := []string{
			"Implement a lightweight /health (or /healthz) endpoint that returns 200 only when downstream dependencies are reachable.",
			"Reference it in '.shuttle.yml' under app.healthcheckPath so future scans probe it automatically.",
			"Wire the same endpoint into Docker HEALTHCHECK and uptime monitoring once it exists.",
		}
		if url != "" {
			notes = append(notes, "Probed URL: "+url+" (got "+fallback(code, "n/a")+").")
		}
		return []Action{{
			ID:          "monitoring.expose-health-endpoint",
			Title:       "Expose an application health endpoint",
			Category:    "monitoring",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Without a health endpoint, restart policies and uptime checks have nothing reliable to probe.",
			Notes:       notes,
		}}
	case "compose.missing_prod_file":
		return []Action{{
			ID:          "compose.create-prod-file",
			Title:       "Create a production compose file",
			Category:    "compose",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Without a docker-compose.prod.yml, doctor cannot reason about services and deploys depend on ad-hoc 'docker run' commands.",
			Notes: []string{
				"Start from your dev compose file and pin every image tag, set restart policies, and add healthchecks.",
				"Place the file at the project root or under /opt/shuttle/<app>/.",
			},
		}}
	case "compose.env_file_missing":
		missing := stringSlice(check.Evidence["missing"])
		notes := []string{
			"Create the missing env file(s) or update the env_file: directive(s) to the correct path.",
			"Keep the file out of Git and chmod 600 once it contains real secrets.",
		}
		if len(missing) > 0 {
			notes = append(notes, "Missing references: "+strings.Join(missing, ", "))
		}
		return []Action{{
			ID:          "compose.fix-env-file-references",
			Title:       "Fix missing env_file references",
			Category:    "compose",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "compose 'env_file:' references that point to missing files cause the container to start with empty env vars and fail silently.",
			Notes:       notes,
		}}
	case "compose.latest_tag_used":
		offenders := stringSlice(check.Evidence["offenders"])
		notes := []string{
			"Replace 'image: foo' or 'image: foo:latest' with an immutable tag or digest.",
			"For first-party images, tag releases with a semantic version or short commit SHA.",
		}
		if len(offenders) > 0 {
			notes = append(notes, "Unpinned images: "+strings.Join(offenders, ", "))
		}
		return []Action{{
			ID:          "compose.pin-image-tags",
			Title:       "Pin compose image tags",
			Category:    "compose",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Floating ':latest' tags break reproducibility and silently introduce upstream changes between deploys.",
			Notes:       notes,
		}}
	case "compose.no_resource_limits":
		return []Action{{
			ID:          "compose.add-resource-limits",
			Title:       "Add cpu/memory limits to compose services",
			Category:    "compose",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Without resource ceilings, a single runaway container can starve everything else on the VPS.",
			Notes: []string{
				"Classic compose: 'mem_limit: 512m' / 'cpus: \"0.5\"' per service.",
				"Swarm: 'deploy: { resources: { limits: { memory: 512M, cpus: \"0.5\" } } }'.",
			},
		}}
	case "compose.bind_mount_sensitive_paths":
		lines := stringSlice(check.Evidence["lines"])
		notes := []string{
			"Replace bind mounts of /var/run/docker.sock, /etc, /root, /proc, /sys, or root '/' with named volumes scoped to the service.",
			"If a workload genuinely needs the Docker socket, mount it ':ro' and add the workload to docker.allowDockerSocket in .shuttle.yml.",
		}
		if len(lines) > 0 {
			notes = append(notes, "Offending lines: "+strings.Join(lines, " | "))
		}
		return []Action{{
			ID:          "compose.remove-sensitive-mounts",
			Title:       "Remove sensitive host bind mounts",
			Category:    "compose",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Bind mounts of host system paths give a container escape path to the host filesystem.",
			Notes:       notes,
		}}
	case "tls.hsts_missing":
		url := stringEvidence(check.Evidence["url"])
		notes := []string{
			"Add 'header Strict-Transport-Security \"max-age=31536000; includeSubDomains\"' in the Caddyfile (global block).",
			"Reload Caddy after the change: 'caddy reload --config /etc/caddy/Caddyfile'.",
			"Once HSTS is verified in production, consider preloading via https://hstspreload.org.",
		}
		if url != "" {
			notes = append(notes, "Probed URL: "+url+".")
		}
		return []Action{{
			ID:          "tls.enforce-hsts",
			Title:       "Enforce HSTS via Strict-Transport-Security header",
			Category:    "tls",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Without HSTS, a man-in-the-middle attacker can downgrade the connection to HTTP on the first request.",
			Notes:       notes,
		}}
	case "dns.domain_not_pointing_to_server":
		domain := stringEvidence(check.Evidence["domain"])
		dnsIP := stringEvidence(check.Evidence["dnsIP"])
		serverIP := stringEvidence(check.Evidence["serverIP"])
		notes := []string{
			"Update the A record for " + fallback(domain, "<domain>") + " to point at " + fallback(serverIP, "<server-IP>") + ", or front the origin via Cloudflare and confirm the orange cloud proxy is intentional.",
			"If you use Cloudflare, also lock origin access: only accept HTTPS from Cloudflare IP ranges or use Authenticated Origin Pulls.",
		}
		if dnsIP != "" && serverIP != "" {
			notes = append(notes, fmt.Sprintf("Current resolution: %s -> %s; server public IP: %s.", domain, dnsIP, serverIP))
		}
		return []Action{{
			ID:          "dns.point-to-server",
			Title:       "Align DNS A record with the server",
			Category:    "dns",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "If DNS does not resolve to this server (and is not proxied), TLS issuance and the public app silently break.",
			Notes:       notes,
		}}
	case "db.no_backup_detected":
		return []Action{{
			ID:          "db.add-backup-job",
			Title:       "Schedule database backups",
			Category:    "backups",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Without backups, a single failed migration or volume loss is unrecoverable.",
			Notes: []string{
				"Postgres: 'docker exec <container> pg_dump -U <user> <db> | gzip > /backups/db-$(date +%F).sql.gz'.",
				"Schedule via cron: '0 3 * * * /usr/local/bin/backup-db.sh'.",
				"Ship backups offsite (S3, B2, rsync to a different host) and document the restore procedure.",
				"Add a weekly restore drill so the backup is known to be usable.",
			},
		}}
	case "cloudflare.ssl_flexible":
		zone := stringEvidence(check.Evidence["zone"])
		mode := stringEvidence(check.Evidence["sslMode"])
		notes := []string{
			"In Cloudflare dashboard: SSL/TLS -> Overview -> set mode to 'Full (strict)'.",
			"Verify the origin presents a valid TLS certificate before flipping (Caddy with ACME, or a Cloudflare Origin CA cert).",
		}
		if zone != "" {
			notes = append(notes, "Zone: "+zone+" (current mode: "+fallback(mode, "unknown")+").")
		}
		return []Action{{
			ID:          "cloudflare.upgrade-ssl-mode",
			Title:       "Upgrade Cloudflare SSL/TLS mode to Full (strict)",
			Category:    "cloudflare",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Flexible mode terminates HTTPS at Cloudflare and talks plaintext HTTP to the origin, defeating end-to-end encryption.",
			Notes:       notes,
		}}
	case "cloudflare.always_https_disabled":
		zone := stringEvidence(check.Evidence["zone"])
		notes := []string{
			"In Cloudflare dashboard: SSL/TLS -> Edge Certificates -> enable 'Always Use HTTPS'.",
			"Combine with HSTS (tls.hsts_missing) for end-to-end HTTPS enforcement.",
		}
		if zone != "" {
			notes = append(notes, "Zone: "+zone+".")
		}
		return []Action{{
			ID:          "cloudflare.enable-always-https",
			Title:       "Enable Cloudflare Always Use HTTPS",
			Category:    "cloudflare",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Without 'Always Use HTTPS', any HTTP request hits the origin in clear text before being upgraded.",
			Notes:       notes,
		}}
	case "cloudflare.waf_disabled":
		zone := stringEvidence(check.Evidence["zone"])
		notes := []string{
			"In Cloudflare dashboard: Security -> WAF -> Managed Rules -> enable the Cloudflare Managed Ruleset.",
			"Pro plan or higher is required for managed WAF rules.",
		}
		if zone != "" {
			notes = append(notes, "Zone: "+zone+".")
		}
		return []Action{{
			ID:          "cloudflare.enable-waf",
			Title:       "Enable Cloudflare Managed WAF",
			Category:    "cloudflare",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "The managed WAF blocks known attack patterns (SQLi, XSS, scanners) at the edge before they hit the origin.",
			Notes:       notes,
		}}
	case "cloudflare.dns_missing":
		name := stringEvidence(check.Evidence["name"])
		notes := []string{
			"In Cloudflare dashboard: DNS -> Records -> Add an A or CNAME record for " + fallback(name, "<host>") + ".",
			"Enable the orange cloud (proxied) unless you intentionally serve the origin direct.",
		}
		return []Action{{
			ID:          "cloudflare.create-dns-record",
			Title:       "Create the missing Cloudflare DNS record",
			Category:    "dns",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Without a DNS record at Cloudflare, the host does not resolve and ACME / public access fails.",
			Notes:       notes,
		}}
	case "cloudflare.proxy_disabled":
		name := stringEvidence(check.Evidence["name"])
		notes := []string{
			"In Cloudflare dashboard: DNS -> Records -> toggle the orange cloud on for " + fallback(name, "<host>") + ".",
			"Once proxied, restrict the origin to only accept traffic from Cloudflare IP ranges (UFW or Authenticated Origin Pulls).",
			"Keep mail / sftp / direct subdomains explicit and document why they are unproxied.",
		}
		return []Action{{
			ID:          "cloudflare.enable-proxy",
			Title:       "Enable Cloudflare proxy on host records",
			Category:    "cloudflare",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Unproxied A/AAAA records bypass Cloudflare entirely, leaking the origin IP and forfeiting WAF / cache / DDoS protection.",
			Notes:       notes,
		}}
	case "adminer.ip_restriction_missing":
		return []Action{{
			ID:          "adminer.lock-down",
			Title:       "Lock down Adminer behind IP allowlist + basic auth",
			Category:    "database",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Adminer exposes full database access; it must be restricted by IP and protected by an extra auth layer.",
			Notes: []string{
				"In Caddy: add a @adminer matcher with 'remote_ip <home-ip>' + 'basic_auth' before the reverse_proxy.",
				"Reject all other source IPs with 'respond 403'.",
			},
		}}
	}
	return nil
}

func exposurePairs(value any) []string {
	pairs := []string{}
	switch typed := value.(type) {
	case []map[string]string:
		for _, e := range typed {
			pairs = append(pairs, e["workload"]+":"+e["port"])
		}
	case []any:
		for _, item := range typed {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			workload, _ := m["workload"].(string)
			port, _ := m["port"].(string)
			pairs = append(pairs, workload+":"+port)
		}
	}
	return pairs
}

func intEvidence(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func stringEvidence(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func fallback(value string, def string) string {
	if value == "" {
		return def
	}
	return value
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := []string{}
		for _, item := range typed {
			out = append(out, fmt.Sprint(item))
		}
		return out
	}
	return nil
}
