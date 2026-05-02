package harden

import (
	"strings"
	"testing"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/readiness"
)

func failed(id string, severity readiness.Severity, evidence map[string]any) readiness.CheckResult {
	return readiness.CheckResult{
		ID:       id,
		Status:   readiness.Failed,
		Severity: severity,
		Evidence: evidence,
	}
}

func TestBuildPlanEmptyWhenNoFailures(t *testing.T) {
	report := readiness.Report{
		Checks: []readiness.CheckResult{
			{ID: "system.os_supported", Status: readiness.Passed},
		},
	}
	plan := BuildPlan(report)
	if len(plan.Actions) != 0 {
		t.Fatalf("expected zero actions, got %d", len(plan.Actions))
	}
}

func TestBuildPlanIgnoresIgnoredFindings(t *testing.T) {
	report := readiness.Report{
		Checks: []readiness.CheckResult{
			{
				ID:       "firewall.ufw_inactive",
				Status:   readiness.Failed,
				Severity: readiness.High,
				Ignored:  true,
			},
		},
	}
	plan := BuildPlan(report)
	if len(plan.Actions) != 0 {
		t.Fatalf("ignored finding must not produce actions, got %d", len(plan.Actions))
	}
}

func TestBuildPlanCoversKnownFindings(t *testing.T) {
	report := readiness.Report{
		Target: "root@example:7022",
		Score:  60,
		Level:  readiness.Level("risky"),
		Checks: []readiness.CheckResult{
			failed("firewall.ufw_inactive", readiness.High, nil),
			failed("firewall.database_port_public", readiness.High, map[string]any{"publicPorts": []string{"5432"}}),
			failed("secrets.env_world_readable", readiness.Critical, nil),
			failed("caddy.admin_exposed", readiness.High, nil),
			failed("docker.containers_without_healthcheck", readiness.Medium, map[string]any{"workloads": []string{"api"}}),
			failed("docker.containers_running_as_root", readiness.Medium, map[string]any{"workloads": []string{"api"}}),
			failed("docker.sock_exposed", readiness.High, map[string]any{"readWriteWorkloads": []string{"runner"}}),
			failed("docker.containers_without_restart_policy", readiness.Medium, map[string]any{"workloads": []string{"api"}}),
			failed("adminer.ip_restriction_missing", readiness.High, nil),
		},
	}
	plan := BuildPlan(report)
	expected := map[string]bool{
		"ufw.enable-baseline":         true,
		"firewall.lock-db-ports":      true,
		"secrets.tighten-env-perms":   true,
		"caddy.lock-admin-api":        true,
		"docker.add-healthchecks":     true,
		"docker.drop-root":            true,
		"docker.review-socket-mounts": true,
		"docker.set-restart-policy":   true,
		"adminer.lock-down":           true,
	}
	for _, action := range plan.Actions {
		delete(expected, action.ID)
	}
	if len(expected) != 0 {
		missing := []string{}
		for id := range expected {
			missing = append(missing, id)
		}
		t.Fatalf("missing actions: %v", missing)
	}
}

func TestBuildPlanFlagsSafeAutoApplyActions(t *testing.T) {
	report := readiness.Report{
		Checks: []readiness.CheckResult{
			failed("secrets.env_world_readable", readiness.Critical, nil),
			failed("firewall.database_port_public", readiness.High, map[string]any{"publicPorts": []string{"5432"}}),
			failed("docker.containers_running_as_root", readiness.Medium, nil),
		},
	}
	plan := BuildPlan(report)
	safe := map[string]bool{}
	for _, action := range plan.Actions {
		if action.SafeAutoApply {
			safe[action.ID] = true
		}
	}
	if !safe["secrets.tighten-env-perms"] || !safe["firewall.lock-db-ports"] {
		t.Fatalf("expected env + db-port actions safe, got: %v", safe)
	}
	if safe["docker.drop-root"] {
		t.Fatal("docker.drop-root must not be safe-auto-apply")
	}
}

func TestBuildPlanEmbedsDatabasePortsInCommands(t *testing.T) {
	report := readiness.Report{
		Checks: []readiness.CheckResult{
			failed("firewall.database_port_public", readiness.High, map[string]any{"publicPorts": []string{"5432", "27017"}}),
		},
	}
	plan := BuildPlan(report)
	if len(plan.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(plan.Actions))
	}
	joined := strings.Join(plan.Actions[0].Commands, " ")
	if !strings.Contains(joined, "5432/tcp") || !strings.Contains(joined, "27017/tcp") {
		t.Fatalf("expected port-specific commands, got %q", joined)
	}
}

func TestBuildPlanCoversP0CheckPack(t *testing.T) {
	report := readiness.Report{
		Checks: []readiness.CheckResult{
			failed("system.updates_pending", readiness.Medium, map[string]any{"pendingCount": 12}),
			failed("system.memory_low", readiness.Medium, map[string]any{"availablePercent": 7}),
			failed("ssh.port_default", readiness.Low, nil),
			failed("firewall.docker_published_sensitive_ports", readiness.High, map[string]any{
				"exposures": []map[string]string{{"workload": "pg_main", "port": "5432"}},
			}),
			failed("caddy.no_security_headers", readiness.Medium, map[string]any{
				"missing":   []string{"Strict-Transport-Security"},
				"caddyfile": "/etc/caddy/Caddyfile",
			}),
			failed("caddy.invalid_config", readiness.High, map[string]any{
				"caddyfile": "/etc/caddy/Caddyfile",
				"output":    "syntax error at line 4",
			}),
			failed("tls.cert_missing", readiness.High, map[string]any{
				"domain":        "example.com",
				"daysRemaining": 5,
			}),
			failed("secrets.weak_file_permissions", readiness.High, map[string]any{
				"files": []string{".env.production", "certs/server.key"},
			}),
			failed("monitoring.no_health_endpoint", readiness.Medium, map[string]any{
				"url":      "https://example.com/health",
				"httpCode": "500",
			}),
		},
	}
	plan := BuildPlan(report)
	expected := map[string]bool{
		"system.apply-pending-updates":       true,
		"system.relieve-memory-pressure":     true,
		"ssh.move-off-default-port":          true,
		"firewall.unpublish-sensitive-ports": true,
		"caddy.add-security-headers":         true,
		"caddy.fix-invalid-config":           true,
		"tls.restore-certificate":            true,
		"secrets.tighten-secret-perms":       true,
		"monitoring.expose-health-endpoint":  true,
	}
	for _, action := range plan.Actions {
		delete(expected, action.ID)
	}
	if len(expected) != 0 {
		missing := []string{}
		for id := range expected {
			missing = append(missing, id)
		}
		t.Fatalf("missing P0 actions: %v", missing)
	}
}

func TestSecretsWeakPermsActionIsSafeAutoApply(t *testing.T) {
	report := readiness.Report{
		Checks: []readiness.CheckResult{
			failed("secrets.weak_file_permissions", readiness.High, map[string]any{
				"files": []string{".env.production"},
			}),
		},
	}
	plan := BuildPlan(report)
	if len(plan.Actions) != 1 {
		t.Fatalf("expected one action, got %d", len(plan.Actions))
	}
	action := plan.Actions[0]
	if !action.SafeAutoApply {
		t.Fatal("secrets.tighten-secret-perms must be safe-auto-apply")
	}
	if !strings.Contains(strings.Join(action.Commands, " "), "chmod 600 .env.production") {
		t.Fatalf("expected chmod command, got %v", action.Commands)
	}
}

func TestBuildPlanCoversP1CheckPack(t *testing.T) {
	report := readiness.Report{
		Checks: []readiness.CheckResult{
			failed("compose.missing_prod_file", readiness.Medium, nil),
			failed("compose.env_file_missing", readiness.Medium, map[string]any{"missing": []string{".env.production"}}),
			failed("compose.latest_tag_used", readiness.Medium, map[string]any{"offenders": []string{"api:latest"}}),
			failed("compose.no_resource_limits", readiness.Low, nil),
			failed("compose.bind_mount_sensitive_paths", readiness.High, map[string]any{"lines": []string{"- /var/run/docker.sock:/var/run/docker.sock"}}),
			failed("tls.hsts_missing", readiness.Medium, map[string]any{"url": "https://example.com"}),
			failed("dns.domain_not_pointing_to_server", readiness.Medium, map[string]any{"domain": "example.com", "dnsIP": "1.2.3.4", "serverIP": "5.6.7.8"}),
			failed("db.no_backup_detected", readiness.High, nil),
		},
	}
	plan := BuildPlan(report)
	expected := map[string]bool{
		"compose.create-prod-file":        true,
		"compose.fix-env-file-references": true,
		"compose.pin-image-tags":          true,
		"compose.add-resource-limits":     true,
		"compose.remove-sensitive-mounts": true,
		"tls.enforce-hsts":                true,
		"dns.point-to-server":             true,
		"db.add-backup-job":               true,
	}
	for _, action := range plan.Actions {
		delete(expected, action.ID)
	}
	if len(expected) != 0 {
		missing := []string{}
		for id := range expected {
			missing = append(missing, id)
		}
		t.Fatalf("missing P1 actions: %v", missing)
	}
	for _, action := range plan.Actions {
		if action.SafeAutoApply {
			t.Fatalf("P1 action %s must not be safe-auto-apply", action.ID)
		}
	}
}

func TestBuildPlanCoversCloudflarePack(t *testing.T) {
	report := readiness.Report{
		Checks: []readiness.CheckResult{
			failed("cloudflare.ssl_flexible", readiness.Critical, map[string]any{"zone": "example.com", "sslMode": "flexible"}),
			failed("cloudflare.always_https_disabled", readiness.Medium, map[string]any{"zone": "example.com"}),
			failed("cloudflare.waf_disabled", readiness.Medium, map[string]any{"zone": "example.com"}),
			failed("cloudflare.dns_missing", readiness.High, map[string]any{"name": "app.example.com"}),
			failed("cloudflare.proxy_disabled", readiness.Medium, map[string]any{"name": "app.example.com"}),
		},
	}
	plan := BuildPlan(report)
	expected := map[string]bool{
		"cloudflare.upgrade-ssl-mode":    true,
		"cloudflare.enable-always-https": true,
		"cloudflare.enable-waf":          true,
		"cloudflare.create-dns-record":   true,
		"cloudflare.enable-proxy":        true,
	}
	for _, action := range plan.Actions {
		delete(expected, action.ID)
		if action.SafeAutoApply {
			t.Fatalf("%s must not be safe-auto-apply", action.ID)
		}
	}
	if len(expected) != 0 {
		missing := []string{}
		for id := range expected {
			missing = append(missing, id)
		}
		t.Fatalf("missing cloudflare actions: %v", missing)
	}
}

func TestExposurePairsHandlesJSONRoundtrip(t *testing.T) {
	// Simulate evidence loaded from JSON: []any of map[string]any.
	loaded := []any{
		map[string]any{"workload": "pg_main", "port": "5432"},
		map[string]any{"workload": "redis", "port": "6379"},
	}
	pairs := exposurePairs(loaded)
	joined := strings.Join(pairs, " ")
	if !strings.Contains(joined, "pg_main:5432") || !strings.Contains(joined, "redis:6379") {
		t.Fatalf("expected both pairs, got %v", pairs)
	}
}

func TestConsoleNoFindings(t *testing.T) {
	plan := BuildPlan(readiness.Report{Target: "local"})
	rendered := Console(plan)
	if !strings.Contains(rendered, "Nothing to harden") {
		t.Fatalf("expected friendly empty plan, got: %s", rendered)
	}
}

func TestConsoleListsCommandsAndNotes(t *testing.T) {
	report := readiness.Report{
		Target: "local",
		Checks: []readiness.CheckResult{
			failed("firewall.ufw_inactive", readiness.High, nil),
		},
	}
	rendered := Console(BuildPlan(report))
	if !strings.Contains(rendered, "ufw --force enable") {
		t.Fatalf("expected ufw command in output: %s", rendered)
	}
	if !strings.Contains(rendered, "dry-run") {
		t.Fatalf("expected dry-run notice in output: %s", rendered)
	}
}
