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
	// SafeLocalApply is true when Commands are safe to run automatically on
	// the local machine without further interaction (idempotent, scoped,
	// non-destructive). Only these actions are eligible for `harden --apply`.
	SafeLocalApply bool `json:"safeLocalApply,omitempty"`
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
		return []Action{{
			ID:          "firewall.lock-db-ports",
			Title:       "Restrict public database ports",
			Category:    "firewall",
			SourceCheck: check.ID,
			Severity:    string(check.Severity),
			Rationale:   "Database ports are publicly reachable or publicly allowed; close them or restrict to trusted IPs.",
			Commands:    commands,
			Notes:       notes,
		}}
	case "secrets.env_world_readable":
		return []Action{{
			ID:             "secrets.tighten-env-perms",
			Title:          "Tighten .env file permissions",
			Category:       "secrets",
			SourceCheck:    check.ID,
			Severity:       string(check.Severity),
			Rationale:      ".env contains production secrets and must not be world-readable.",
			Commands:       []string{"chmod 600 .env"},
			SafeLocalApply: true,
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
			"For images that must run as root, document the reason and add the workload to docker.allowRoot in .deployshuttle.yml.",
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
			"For workloads that genuinely orchestrate Docker (CI runners, autoheal), add them to docker.allowDockerSocket in .deployshuttle.yml.",
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
