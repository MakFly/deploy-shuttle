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
