package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMinimalConfig(t *testing.T) {
	cfg, err := Load("testdata/minimal.yml", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App != "myapp" {
		t.Fatalf("expected app myapp, got %s", cfg.App)
	}
	if cfg.Servers["default"].User != "deploy" {
		t.Fatalf("expected deploy user")
	}
	if cfg.Deploy.Strategy != "swarm" {
		t.Fatalf("expected default swarm strategy, got %s", cfg.Deploy.Strategy)
	}
}

func TestLoadInvalidConfig(t *testing.T) {
	if _, err := Load("testdata/invalid.yml", ""); err == nil {
		t.Fatal("expected invalid config to fail")
	}
}

func TestLoadVPNConfigFromServerShorthand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shuttle.yml")
	body := `app: myapp
domain: myapp.example.com
server:
  host: 10.8.0.12
  user: deploy
  port: 7022
  vpn:
    required: true
    interface: wg0
    check_host: 10.8.0.12
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	vpn := cfg.Servers["default"].VPN
	if !vpn.Required {
		t.Fatal("expected vpn.required to be true")
	}
	if vpn.Interface != "wg0" {
		t.Fatalf("expected interface wg0, got %q", vpn.Interface)
	}
	if vpn.CheckPort != 7022 {
		t.Fatalf("expected check port to default to SSH port 7022, got %d", vpn.CheckPort)
	}
}

func TestLoadVPNConfigFromServerGroup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shuttle.yml")
	body := `app: myapp
domain: myapp.example.com
servers:
  staging:
    hosts:
      - 10.8.0.13
    user: deploy
    port: 22
    vpn:
      required: true
      check_port: 2222
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	vpn := cfg.Servers["staging"].VPN
	if !vpn.Required {
		t.Fatal("expected vpn.required to be true")
	}
	if vpn.CheckPort != 2222 {
		t.Fatalf("expected explicit check port 2222, got %d", vpn.CheckPort)
	}
}

func TestLoadDeployPathAndCaddyNetwork(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shuttle.yml")
	body := `app: myapp
domain: myapp.example.com
server:
  host: 203.0.113.10
  user: root
deploy:
  path: /opt/deploy/myapp
caddy:
  network: edge
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Deploy.Path != "/opt/deploy/myapp" {
		t.Fatalf("expected deploy path override, got %q", cfg.Deploy.Path)
	}
	if cfg.Caddy.Network != "edge" {
		t.Fatalf("expected caddy network edge, got %q", cfg.Caddy.Network)
	}
}

func TestLoadDeploymentSLOConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shuttle.yml")
	body := `app: myapp
domain: myapp.example.com
server:
  host: 203.0.113.10
  user: root
deploy:
  promotion_slo_seconds: 30
  total_slo_seconds: 45
  availability_url: https://myapp.example.com/health
  availability_interval_ms: 200
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Deploy.PromotionSLOSeconds != 30 {
		t.Fatalf("expected 30 second promotion SLO, got %d", cfg.Deploy.PromotionSLOSeconds)
	}
	if cfg.Deploy.TotalSLOSeconds != 45 {
		t.Fatalf("expected 45 second total SLO, got %d", cfg.Deploy.TotalSLOSeconds)
	}
	if cfg.Deploy.AvailabilityURL != "https://myapp.example.com/health" {
		t.Fatalf("unexpected availability URL: %q", cfg.Deploy.AvailabilityURL)
	}
	if cfg.Deploy.AvailabilityIntervalMS != 200 {
		t.Fatalf("expected 200ms interval, got %d", cfg.Deploy.AvailabilityIntervalMS)
	}
}

func TestLoadCaddyBasicAuth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shuttle.yml")
	body := `app: myapp
domain: myapp.example.com
server:
  host: 203.0.113.10
  user: root
caddy:
  basic_auth:
    users:
      - username: audit
        hash: $2a$14$abcdef
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Caddy.BasicAuth.Users) != 1 {
		t.Fatalf("expected one basic auth user, got %d", len(cfg.Caddy.BasicAuth.Users))
	}
	user := cfg.Caddy.BasicAuth.Users[0]
	if user.Username != "audit" || user.Hash != "$2a$14$abcdef" {
		t.Fatalf("unexpected basic auth user: %#v", user)
	}
}
