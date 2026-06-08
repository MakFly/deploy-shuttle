package config

import "testing"

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
