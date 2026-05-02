package harden

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplySafeLocalChmodEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("SECRET=1\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	plan := Plan{Actions: []Action{{
		ID:             "secrets.tighten-env-perms",
		Title:          "Tighten .env",
		Commands:       []string{"chmod 600 .env"},
		SafeLocalApply: true,
	}}}
	results := ApplySafeLocal(plan)
	if len(results) != 1 || results[0].Status != "applied" {
		t.Fatalf("expected applied result, got %+v", results)
	}
	info, err := os.Stat(envPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected mode 0600, got %o", info.Mode().Perm())
	}
}

func TestApplySafeLocalSkipsUnsafeAction(t *testing.T) {
	plan := Plan{Actions: []Action{{
		ID:       "ufw.enable-baseline",
		Title:    "UFW",
		Commands: []string{"ufw --force enable"},
	}}}
	results := ApplySafeLocal(plan)
	if len(results) != 1 || results[0].Status != "skipped" {
		t.Fatalf("expected skipped result, got %+v", results)
	}
}

func TestRunSafeCommandRejectsNonAllowlisted(t *testing.T) {
	if err := runSafeCommand("rm -rf /"); err == nil {
		t.Fatal("expected rejection of rm")
	}
}

func TestRunChmodRejectsAbsolutePath(t *testing.T) {
	if err := runChmod([]string{"600", "/etc/.env"}); err == nil {
		t.Fatal("expected rejection of absolute path")
	}
}

func TestRunChmodRejectsTraversal(t *testing.T) {
	if err := runChmod([]string{"600", "../.env"}); err == nil {
		t.Fatal("expected rejection of parent traversal")
	}
}

func TestRunChmodRejectsNonEnvTarget(t *testing.T) {
	if err := runChmod([]string{"600", "secrets.txt"}); err == nil {
		t.Fatal("expected rejection of non-.env target")
	}
}

func TestRunChmodRejectsOtherMode(t *testing.T) {
	if err := runChmod([]string{"777", ".env"}); err == nil {
		t.Fatal("expected rejection of mode 777")
	}
}

func TestRenderApplyResultsSummarises(t *testing.T) {
	rendered := RenderApplyResults([]ApplyResult{
		{ActionID: "a", Title: "A", Status: "applied", Detail: "chmod 600 .env"},
		{ActionID: "b", Title: "B", Status: "skipped"},
	})
	if !strings.Contains(rendered, "Applied: 1") || !strings.Contains(rendered, "Skipped: 1") {
		t.Fatalf("expected summary counts, got: %s", rendered)
	}
}
