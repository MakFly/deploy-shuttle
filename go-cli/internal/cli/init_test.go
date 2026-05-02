package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// withTempDir runs fn inside a temporary working directory.
func withTempDir(t *testing.T, fn func(dir string)) {
	t.Helper()
	dir := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(prev)
	fn(dir)
}

func TestInitWithoutPresetSkipsDeployshuttleYAML(t *testing.T) {
	withTempDir(t, func(dir string) {
		cmd := newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs([]string{"--app", "demo", "--domain", "demo.example.com"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("init failed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "shuttle.yml")); err != nil {
			t.Fatalf("shuttle.yml should exist: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, ".deployshuttle.yml")); !os.IsNotExist(err) {
			t.Fatalf(".deployshuttle.yml should not exist without --preset, got err=%v", err)
		}
	})
}

func TestInitWithPresetWritesValidYAML(t *testing.T) {
	withTempDir(t, func(dir string) {
		cmd := newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs([]string{"--preset", "nextjs", "--domain", "demo.example.com"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("init --preset nextjs failed: %v", err)
		}
		body, err := os.ReadFile(filepath.Join(dir, ".deployshuttle.yml"))
		if err != nil {
			t.Fatalf("read .deployshuttle.yml: %v", err)
		}
		var parsed map[string]any
		if err := yaml.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("yaml parse: %v", err)
		}
		app, _ := parsed["app"].(map[string]any)
		if app["domain"] != "demo.example.com" {
			t.Fatalf("expected domain demo.example.com, got %v", app["domain"])
		}
		if !strings.Contains(string(body), "/api/health") {
			t.Fatal("nextjs preset should default healthcheckPath to /api/health")
		}
	})
}

func TestInitRejectsUnknownPreset(t *testing.T) {
	withTempDir(t, func(dir string) {
		cmd := newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"--preset", "rails"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "unknown preset") {
			t.Fatalf("expected unknown preset error, got %v", err)
		}
	})
}

func TestInitForceOverwritesExistingDeployshuttleYAML(t *testing.T) {
	withTempDir(t, func(dir string) {
		if err := os.WriteFile(filepath.Join(dir, ".deployshuttle.yml"), []byte("version: 1\n# stale\n"), 0o644); err != nil {
			t.Fatalf("seed: %v", err)
		}
		// Without --force we must refuse.
		cmd := newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"--preset", "node-api"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error without --force")
		}
		// With --force it overwrites.
		cmd = newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs([]string{"--preset", "node-api", "--force"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("force overwrite failed: %v", err)
		}
		body, _ := os.ReadFile(filepath.Join(dir, ".deployshuttle.yml"))
		if strings.Contains(string(body), "stale") {
			t.Fatal("expected file to be overwritten")
		}
	})
}
