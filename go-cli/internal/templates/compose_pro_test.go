package templates

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProComposeTemplateIsValidYAML(t *testing.T) {
	opts := ProComposeOptions{
		App:       "myapp",
		Preset:    "laravel",
		Port:      "8000",
		DB:        "postgres",
		Redis:     true,
		Queue:     true,
		Scheduler: true,
		Mailpit:   true,
	}
	out := ProComposeTemplate(opts)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("Pro compose is not valid YAML: %v\n%s", err, out)
	}
}

func TestProComposeHasAllLaravelServices(t *testing.T) {
	opts := ProComposeOptions{
		App:       "myapp",
		Preset:    "laravel",
		Port:      "8000",
		DB:        "postgres",
		Redis:     true,
		Queue:     true,
		Scheduler: true,
		Mailpit:   true,
	}
	out := ProComposeTemplate(opts)
	for _, svc := range []string{"web", "postgres", "redis", "queue", "scheduler", "horizon", "mailpit"} {
		if !strings.Contains(out, svc+":") {
			t.Errorf("laravel pro compose missing service: %s", svc)
		}
	}
}

func TestProComposeHasAllSymfonyServices(t *testing.T) {
	opts := ProComposeOptions{
		App:       "myapp",
		Preset:    "symfony",
		Port:      "8080",
		DB:        "postgres",
		Redis:     true,
		Queue:     true,
		Scheduler: true,
		Mailpit:   true,
	}
	out := ProComposeTemplate(opts)
	for _, svc := range []string{"web", "postgres", "redis", "messenger", "scheduler", "mailpit"} {
		if !strings.Contains(out, svc+":") {
			t.Errorf("symfony pro compose missing service: %s", svc)
		}
	}
	if strings.Contains(out, "horizon:") {
		t.Error("symfony pro compose should not have horizon")
	}
}

func TestProComposeMySQLVariant(t *testing.T) {
	opts := ProComposeOptions{
		App:    "myapp",
		Preset: "laravel",
		Port:   "8000",
		DB:     "mysql",
	}
	out := ProComposeTemplate(opts)
	if !strings.Contains(out, "mysql:") {
		t.Error("mysql variant should contain mysql service")
	}
	if strings.Contains(out, "postgres:") {
		t.Error("mysql variant should not contain postgres service")
	}
}

func TestProComposeDBOnly(t *testing.T) {
	opts := ProComposeOptions{
		App:    "myapp",
		Preset: "laravel",
		Port:   "8000",
		DB:     "postgres",
	}
	out := ProComposeTemplate(opts)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("not valid YAML: %v", err)
	}
	services := parsed["services"].(map[string]any)
	if _, ok := services["postgres"]; !ok {
		t.Error("should have postgres service")
	}
	if _, ok := services["redis"]; ok {
		t.Error("should not have redis when not requested")
	}
}

func TestProComposeWebDependsOnDB(t *testing.T) {
	opts := ProComposeOptions{
		App:    "myapp",
		Preset: "laravel",
		Port:   "8000",
		DB:     "postgres",
		Redis:  true,
	}
	out := ProComposeTemplate(opts)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("not valid YAML: %v", err)
	}
	services := parsed["services"].(map[string]any)
	web := services["web"].(map[string]any)
	deps := web["depends_on"].(map[string]any)
	if _, ok := deps["postgres"]; !ok {
		t.Error("web should depend on postgres")
	}
	if _, ok := deps["redis"]; !ok {
		t.Error("web should depend on redis")
	}
}

func TestProComposeHasVolumes(t *testing.T) {
	opts := ProComposeOptions{
		App:    "myapp",
		Preset: "laravel",
		Port:   "8000",
		DB:     "postgres",
		Redis:  true,
	}
	out := ProComposeTemplate(opts)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("not valid YAML: %v", err)
	}
	volumes := parsed["volumes"].(map[string]any)
	if _, ok := volumes["postgres_data"]; !ok {
		t.Error("should have postgres_data volume")
	}
	if _, ok := volumes["redis_data"]; !ok {
		t.Error("should have redis_data volume")
	}
}

func TestProComposeHasNetwork(t *testing.T) {
	opts := ProComposeOptions{
		App:    "myapp",
		Preset: "laravel",
		Port:   "8000",
	}
	out := ProComposeTemplate(opts)
	if !strings.Contains(out, "app-network") {
		t.Error("compose should have app-network")
	}
}

func TestValidateProFlagsRejectsInvalidDB(t *testing.T) {
	err := ValidateProFlags("laravel", "sqlite", false, false)
	if err == nil {
		t.Error("should reject sqlite")
	}
}

func TestValidateProFlagsRejectsQueueOnNextjs(t *testing.T) {
	err := ValidateProFlags("nextjs", "", true, false)
	if err == nil {
		t.Error("should reject queue on nextjs")
	}
}

func TestValidateProFlagsAcceptsValidCombination(t *testing.T) {
	err := ValidateProFlags("laravel", "postgres", true, true)
	if err != nil {
		t.Errorf("should accept valid combination: %v", err)
	}
}

func TestServiceNamesLaravelFull(t *testing.T) {
	opts := ProComposeOptions{
		Preset:    "laravel",
		DB:        "postgres",
		Redis:     true,
		Queue:     true,
		Scheduler: true,
		Mailpit:   true,
	}
	names := ServiceNames(opts)
	if names[0] != "web" {
		t.Error("first service should be web")
	}
	if len(names) < 6 {
		t.Errorf("expected at least 6 services, got %d: %v", len(names), names)
	}
}
