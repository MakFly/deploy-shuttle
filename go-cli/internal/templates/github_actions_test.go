package templates

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCIWorkflowIsValidYAML(t *testing.T) {
	for _, preset := range []string{"", "laravel", "symfony", "nextjs", "node-api"} {
		out := CIWorkflow(preset)
		var parsed map[string]any
		if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
			t.Errorf("CIWorkflow(%q) is not valid YAML: %v", preset, err)
		}
	}
}

func TestCIWorkflowUsesV6Actions(t *testing.T) {
	for _, preset := range []string{"", "laravel", "symfony", "nextjs", "node-api"} {
		out := CIWorkflow(preset)
		if strings.Contains(out, "checkout@v4") {
			t.Errorf("CIWorkflow(%q) still uses checkout@v4", preset)
		}
		if !strings.Contains(out, "checkout@v6") {
			t.Errorf("CIWorkflow(%q) missing checkout@v6", preset)
		}
	}
}

func TestCIWorkflowContainsDoctorStep(t *testing.T) {
	out := CIWorkflow("")
	if !strings.Contains(out, "shuttle doctor") {
		t.Error("CI workflow missing shuttle doctor step")
	}
}

func TestCIWorkflowLaravelHasPHPSetup(t *testing.T) {
	out := CIWorkflow("laravel")
	if !strings.Contains(out, "shivammathur/setup-php") {
		t.Error("laravel CI workflow missing PHP setup")
	}
	if !strings.Contains(out, "composer install") {
		t.Error("laravel CI workflow missing composer install")
	}
}

func TestCIWorkflowNextjsHasNodeSetup(t *testing.T) {
	out := CIWorkflow("nextjs")
	if !strings.Contains(out, "setup-node@v6") {
		t.Error("nextjs CI workflow missing node setup")
	}
}

func TestCIWorkflowProPostgres(t *testing.T) {
	out := CIWorkflowPro("laravel", "postgres")
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("CIWorkflowPro is not valid YAML: %v", err)
	}
	if !strings.Contains(out, "postgres:16") {
		t.Error("pro CI workflow missing postgres service")
	}
	if !strings.Contains(out, "pg_isready") {
		t.Error("pro CI workflow missing postgres healthcheck")
	}
}

func TestCIWorkflowProMySQL(t *testing.T) {
	out := CIWorkflowPro("laravel", "mysql")
	if !strings.Contains(out, "mysql:8.4") {
		t.Error("pro CI workflow missing mysql service")
	}
	if !strings.Contains(out, "mysqladmin ping") {
		t.Error("pro CI workflow missing mysql healthcheck")
	}
}
