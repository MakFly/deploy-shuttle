package readiness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".shuttle.yml")
	if err := os.WriteFile(path, []byte(`version: 1
checks:
  profile:
    - docker
  ignore:
    - ssh.port_default
docker:
  allowDockerSocket:
    - caddy_dozzle
  allowRoot:
    - caddy_caddy
  workerServices:
    - prod_worker-*
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, resolved, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if resolved != path {
		t.Fatalf("expected resolved path %q, got %q", path, resolved)
	}
	if cfg.Version != 1 {
		t.Fatalf("expected version 1, got %d", cfg.Version)
	}
	if len(cfg.Docker.WorkerServices) != 1 || cfg.Docker.WorkerServices[0] != "prod_worker-*" {
		t.Fatalf("unexpected worker services %#v", cfg.Docker.WorkerServices)
	}
}
