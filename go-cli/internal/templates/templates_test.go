package templates

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestIsReadinessPreset(t *testing.T) {
	for _, p := range ReadinessPresets {
		if !IsReadinessPreset(p) {
			t.Fatalf("%s should be a valid preset", p)
		}
	}
	if IsReadinessPreset("rails") {
		t.Fatal("rails should not be a valid preset")
	}
}

func TestDeployShuttleYMLPresetsParseAsYAML(t *testing.T) {
	for _, preset := range ReadinessPresets {
		body := DeployShuttleYML(preset, "")
		if body == "" {
			t.Fatalf("preset %s produced empty body", preset)
		}
		var parsed map[string]any
		if err := yaml.Unmarshal([]byte(body), &parsed); err != nil {
			t.Fatalf("preset %s did not parse: %v", preset, err)
		}
		if version, _ := parsed["version"].(int); version != 1 {
			t.Fatalf("preset %s: expected version 1, got %v", preset, parsed["version"])
		}
		app, _ := parsed["app"].(map[string]any)
		if app == nil {
			t.Fatalf("preset %s missing app block", preset)
		}
		if app["domain"] != "app.example.com" {
			t.Fatalf("preset %s default domain placeholder missing, got %v", preset, app["domain"])
		}
		if _, ok := app["healthcheckPath"].(string); !ok {
			t.Fatalf("preset %s missing healthcheckPath", preset)
		}
	}
}

func TestDeployShuttleYMLLaravelHasQueueWorkers(t *testing.T) {
	body := DeployShuttleYML("laravel", "myapp.example.com")
	if !strings.Contains(body, "*-queue") {
		t.Fatalf("laravel preset should include queue worker pattern")
	}
	if !strings.Contains(body, "/up") {
		t.Fatalf("laravel preset should default healthcheckPath to /up")
	}
	if !strings.Contains(body, "myapp.example.com") {
		t.Fatalf("laravel preset should use the supplied domain")
	}
}

func TestDeployShuttleYMLNextjsIgnoresAdminer(t *testing.T) {
	body := DeployShuttleYML("nextjs", "")
	if !strings.Contains(body, "adminer.ip_restriction_missing") {
		t.Fatalf("nextjs preset should ignore adminer check by default")
	}
	if !strings.Contains(body, "/api/health") {
		t.Fatalf("nextjs preset should default to /api/health")
	}
}

func TestDeployShuttleYMLDockerSwarmTargetsSwarmNaming(t *testing.T) {
	body := DeployShuttleYML("docker-swarm", "")
	if !strings.Contains(body, "*_worker") {
		t.Fatalf("docker-swarm preset should use *_worker pattern")
	}
}

func TestDeployShuttleYMLUnknownPresetReturnsEmpty(t *testing.T) {
	if DeployShuttleYML("rails", "") != "" {
		t.Fatal("unknown preset should return empty string")
	}
}

func TestShuttleYMLIncludesWireGuardTemplate(t *testing.T) {
	body := ShuttleYML("myapp", "myapp.example.com", "10.8.0.12", "deploy", 7022)
	if !strings.Contains(body, "# vpn:") {
		t.Fatal("shuttle.yml template should include a commented vpn block")
	}
	if !strings.Contains(body, "#   check_host: 10.8.0.12") {
		t.Fatal("vpn template should use the configured host as check_host")
	}
	if !strings.Contains(body, "#   check_port: 7022") {
		t.Fatal("vpn template should use the configured SSH port as check_port")
	}
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("shuttle.yml template did not parse: %v", err)
	}
}
