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
	if cfg.Deploy.Strategy != "blue-green" {
		t.Fatalf("expected default blue-green strategy")
	}
}

func TestLoadInvalidConfig(t *testing.T) {
	if _, err := Load("testdata/invalid.yml", ""); err == nil {
		t.Fatal("expected invalid config to fail")
	}
}
