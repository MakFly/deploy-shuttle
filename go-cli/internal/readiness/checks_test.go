package readiness

import "testing"

func TestProfileCategoriesExpandsCompatibilityPresets(t *testing.T) {
	categories := profileCategories([]string{"docker", "caddy"})
	for _, category := range []string{"system", "ssh", "docker", "firewall", "secrets", "database", "compose", "backups", "reverse-proxy", "tls", "dns", "monitoring", "cloudflare"} {
		if !categories[category] {
			t.Fatalf("expected category %q to be enabled by docker,caddy profile", category)
		}
	}
}

func TestProfileCategoriesAllowsExplicitCategoryFilter(t *testing.T) {
	categories := profileCategories([]string{"tls"})
	if !categoryEnabled("tls", categories) {
		t.Fatal("expected tls to be enabled")
	}
	if categoryEnabled("docker", categories) {
		t.Fatal("expected docker to be disabled")
	}
}

func TestDatabaseListenersPublicButFirewallRestricted(t *testing.T) {
	output := `LISTEN 0 200 0.0.0.0:5432 0.0.0.0:* users:(("postgres",pid=2701933,fd=7))
LISTEN 0 200 [::]:5432 [::]:* users:(("postgres",pid=2701933,fd=8))`
	listeners := publicDatabaseListeners(output)
	if len(listeners) != 2 {
		t.Fatalf("expected 2 listeners, got %d", len(listeners))
	}
	ports := uniqueListenerPorts(listeners)
	if len(ports) != 1 || ports[0] != "5432" {
		t.Fatalf("expected only 5432, got %#v", ports)
	}
	firewall := `Status: active
Default: deny (incoming), allow (outgoing), deny (routed)

To                         Action      From
--                         ------      ----
5432/tcp                   ALLOW IN    127.0.0.1
5432/tcp                   ALLOW IN    172.20.0.0/16`
	if !databasePortsFirewallRestricted(firewall, ports) {
		t.Fatal("expected UFW rules to restrict public database access")
	}
}

func TestDatabaseFirewallDetectsPublicAllow(t *testing.T) {
	firewall := `Status: active
Default: deny (incoming), allow (outgoing), deny (routed)

To                         Action      From
--                         ------      ----
5432/tcp                   ALLOW IN    Anywhere`
	if databasePortsFirewallRestricted(firewall, []string{"5432"}) {
		t.Fatal("expected public allow to be treated as unrestricted")
	}
}

func TestSplitRuntimeOutput(t *testing.T) {
	mode, body := splitRuntimeOutput("__shuttle_runtime=swarm\nservice-a\ton-failure\n__shuttle_runtime=classic\n/container\tunless-stopped\n")
	if mode != "mixed" {
		t.Fatalf("expected mixed mode, got %q", mode)
	}
	if body != "service-a\ton-failure\n/container\tunless-stopped\n" {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestHealthMissing(t *testing.T) {
	if !healthMissing("null") {
		t.Fatal("expected null healthcheck to be missing")
	}
	if !healthMissing(`{"Test":["NONE"]}`) {
		t.Fatal("expected NONE healthcheck to be missing")
	}
	if healthMissing(`{"Test":["CMD","curl","-f","http://localhost/health"]}`) {
		t.Fatal("expected CMD healthcheck to be present")
	}
}

func TestSplitTab2(t *testing.T) {
	left, right := splitTab2("service\tvalue")
	if left != "service" || right != "value" {
		t.Fatalf("unexpected split: %q %q", left, right)
	}
	left, right = splitTab2("service-only")
	if left != "service-only" || right != "" {
		t.Fatalf("unexpected split without tab: %q %q", left, right)
	}
}

func TestApplyConfigIgnoresCheckID(t *testing.T) {
	result := applyConfig(CheckResult{
		ID:       "docker.sock_exposed",
		Severity: High,
		Status:   Failed,
		Evidence: map[string]any{"workloads": []string{"dozzle"}},
	}, Config{Checks: CheckConfig{Ignore: []string{"docker.sock_exposed"}}})

	if !result.Ignored {
		t.Fatal("expected result to be ignored")
	}
	if result.Status != Skipped {
		t.Fatalf("expected skipped status, got %s", result.Status)
	}
}

func TestApplyConfigAllowsSomeDockerSocketWorkloads(t *testing.T) {
	result := applyConfig(CheckResult{
		ID:       "docker.sock_exposed",
		Severity: High,
		Status:   Failed,
		Summary:  "2 Docker workload(s) mount /var/run/docker.sock.",
		Evidence: map[string]any{"workloads": []string{"caddy_dozzle", "shared_uptime-kuma"}},
	}, Config{Docker: DockerConfig{AllowDockerSocket: []string{"caddy_dozzle"}}})

	if result.Ignored {
		t.Fatal("expected one remaining workload, not full ignore")
	}
	workloads, ok := result.Evidence["workloads"].([]string)
	if !ok {
		t.Fatalf("expected workloads evidence as []string")
	}
	if len(workloads) != 1 || workloads[0] != "shared_uptime-kuma" {
		t.Fatalf("unexpected remaining workloads %#v", workloads)
	}
}

func TestApplyConfigAllowsAllWorkerHealthcheckFindings(t *testing.T) {
	result := applyConfig(CheckResult{
		ID:       "docker.containers_without_healthcheck",
		Severity: Medium,
		Status:   Failed,
		Evidence: map[string]any{"workloads": []string{"prod_worker-high-priority", "prod_worker-low-priority"}},
	}, Config{Docker: DockerConfig{WorkerServices: []string{"prod_worker-*"}}})

	if !result.Ignored {
		t.Fatal("expected worker healthcheck finding to be ignored")
	}
}
