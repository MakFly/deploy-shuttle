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
		if _, err := os.Stat(filepath.Join(dir, ".shuttle.yml")); !os.IsNotExist(err) {
			t.Fatalf(".shuttle.yml should not exist without --preset, got err=%v", err)
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
		body, err := os.ReadFile(filepath.Join(dir, ".shuttle.yml"))
		if err != nil {
			t.Fatalf("read .shuttle.yml: %v", err)
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

func TestInitCreatesEnvExample(t *testing.T) {
	withTempDir(t, func(dir string) {
		cmd := newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs([]string{"--preset", "laravel", "--domain", "demo.example.com", "--force"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("init failed: %v", err)
		}
		body, err := os.ReadFile(filepath.Join(dir, ".env.example"))
		if err != nil {
			t.Fatalf("read .env.example: %v", err)
		}
		if !strings.Contains(string(body), "DB_CONNECTION") {
			t.Fatal("laravel .env.example should contain DB_CONNECTION")
		}
	})
}

func TestInitProBypassesOnDevBuild(t *testing.T) {
	// Dev builds have no embedded pubkey, so license.Require is a no-op.
	// This test verifies --pro succeeds on dev builds (the common case).
	withTempDir(t, func(dir string) {
		cmd := newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs([]string{"--preset", "laravel", "--pro", "--domain", "demo.example.com", "--force"})
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("--pro should succeed on dev build: %v", err)
		}
		body, _ := os.ReadFile(filepath.Join(dir, "docker-compose.yml"))
		if !strings.Contains(string(body), "postgres") {
			t.Error("pro compose should contain postgres")
		}
	})
}

func TestInitProDevBuildBypasses(t *testing.T) {
	withTempDir(t, func(dir string) {
		cmd := newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs([]string{"--preset", "laravel", "--pro", "--domain", "demo.example.com", "--force"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("init --pro should succeed on dev build: %v", err)
		}
		body, err := os.ReadFile(filepath.Join(dir, "docker-compose.yml"))
		if err != nil {
			t.Fatalf("read compose: %v", err)
		}
		content := string(body)
		if !strings.Contains(content, "postgres") {
			t.Error("pro compose should contain postgres service")
		}
		if !strings.Contains(content, "redis") {
			t.Error("pro compose should contain redis service")
		}
		if !strings.Contains(content, "queue") {
			t.Error("pro compose should contain queue service")
		}
	})
}

func TestInitWithDBOnlyPostgres(t *testing.T) {
	withTempDir(t, func(dir string) {
		cmd := newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs([]string{"--preset", "laravel", "--with-db", "postgres", "--domain", "demo.example.com", "--force"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("init --with-db postgres failed: %v", err)
		}
		body, _ := os.ReadFile(filepath.Join(dir, "docker-compose.yml"))
		content := string(body)
		if !strings.Contains(content, "postgres") {
			t.Error("should have postgres service")
		}
		if strings.Contains(content, "redis") {
			t.Error("should not have redis when not requested")
		}
	})
}

func TestInitWithQueueOnNextjsFails(t *testing.T) {
	withTempDir(t, func(dir string) {
		cmd := newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"--preset", "nextjs", "--with-queue", "--domain", "demo.example.com"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error for --with-queue on nextjs")
		}
		if !strings.Contains(err.Error(), "laravel and symfony") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}

func TestInitWithDBInvalidValueFails(t *testing.T) {
	withTempDir(t, func(dir string) {
		cmd := newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"--preset", "laravel", "--with-db", "sqlite", "--domain", "demo.example.com"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error for --with-db sqlite")
		}
		if !strings.Contains(err.Error(), "postgres") {
			t.Fatalf("error should mention valid options: %v", err)
		}
	})
}

func TestInitForceOverwritesExistingDeployshuttleYAML(t *testing.T) {
	withTempDir(t, func(dir string) {
		if err := os.WriteFile(filepath.Join(dir, ".shuttle.yml"), []byte("version: 1\n# stale\n"), 0o644); err != nil {
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
		body, _ := os.ReadFile(filepath.Join(dir, ".shuttle.yml"))
		if strings.Contains(string(body), "stale") {
			t.Fatal("expected file to be overwritten")
		}
	})
}
