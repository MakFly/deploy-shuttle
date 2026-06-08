package templates

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func assertValidYAMLService(t *testing.T, name string, block ComposeServiceBlock) {
	t.Helper()
	if block.Name != name {
		t.Errorf("expected name %q, got %q", name, block.Name)
	}
	out, err := yaml.Marshal(block.Service)
	if err != nil {
		t.Fatalf("%s service is not marshalable: %v", name, err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("%s service YAML is not parseable: %v", name, err)
	}
}

func TestPostgresService(t *testing.T) {
	block := PostgresService("myapp")
	assertValidYAMLService(t, "postgres", block)
	if block.Volumes["postgres_data"] != nil {
		t.Error("postgres_data volume should be nil (default driver)")
	}
	hc := block.Service["healthcheck"].(map[string]any)
	test := hc["test"].([]string)
	if test[0] != "CMD-SHELL" {
		t.Error("expected CMD-SHELL healthcheck")
	}
}

func TestMySQLService(t *testing.T) {
	block := MySQLService("myapp")
	assertValidYAMLService(t, "mysql", block)
	if block.Volumes["mysql_data"] != nil {
		t.Error("mysql_data volume should be nil (default driver)")
	}
	hc := block.Service["healthcheck"].(map[string]any)
	test := hc["test"].([]string)
	if test[1] != "mysqladmin" {
		t.Error("expected mysqladmin healthcheck")
	}
}

func TestRedisService(t *testing.T) {
	block := RedisService()
	assertValidYAMLService(t, "redis", block)
	cmd := block.Service["command"].(string)
	if cmd != "redis-server --appendonly yes" {
		t.Errorf("unexpected redis command: %s", cmd)
	}
	hc := block.Service["healthcheck"].(map[string]any)
	test := hc["test"].([]string)
	if test[1] != "redis-cli" {
		t.Error("expected redis-cli healthcheck")
	}
}

func TestMailpitService(t *testing.T) {
	block := MailpitService()
	assertValidYAMLService(t, "mailpit", block)
	ports := block.Service["ports"].([]string)
	if len(ports) != 2 {
		t.Errorf("expected 2 ports, got %d", len(ports))
	}
}

func TestLaravelQueueWorker(t *testing.T) {
	block := LaravelQueueWorker()
	assertValidYAMLService(t, "queue", block)
	cmd := block.Service["command"].(string)
	if cmd != "php artisan queue:work --tries=3 --timeout=90" {
		t.Errorf("unexpected queue command: %s", cmd)
	}
}

func TestLaravelScheduler(t *testing.T) {
	block := LaravelScheduler()
	assertValidYAMLService(t, "scheduler", block)
	cmd := block.Service["command"].(string)
	if cmd != "php artisan schedule:work" {
		t.Errorf("unexpected scheduler command: %s", cmd)
	}
}

func TestLaravelHorizon(t *testing.T) {
	block := LaravelHorizon()
	assertValidYAMLService(t, "horizon", block)
	deps := block.Service["depends_on"].(map[string]any)
	if _, ok := deps["redis"]; !ok {
		t.Error("horizon should depend on redis")
	}
}

func TestSymfonyMessengerWorker(t *testing.T) {
	block := SymfonyMessengerWorker()
	assertValidYAMLService(t, "messenger", block)
	cmd := block.Service["command"].(string)
	if cmd != "php bin/console messenger:consume async --time-limit=3600 --memory-limit=256M" {
		t.Errorf("unexpected messenger command: %s", cmd)
	}
}

func TestSymfonyScheduler(t *testing.T) {
	block := SymfonyScheduler()
	assertValidYAMLService(t, "scheduler", block)
	cmd := block.Service["command"].(string)
	if cmd != "php bin/console messenger:consume scheduler_default -vv --time-limit=3600" {
		t.Errorf("unexpected symfony scheduler command: %s", cmd)
	}
}
