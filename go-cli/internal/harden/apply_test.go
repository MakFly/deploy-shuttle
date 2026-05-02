package harden

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/execx"
)

type fakeAdapter struct {
	run func(cmd string) execx.Result
}

func (f fakeAdapter) Run(cmd string, _ time.Duration) execx.Result {
	return f.run(cmd)
}

func TestApplyChmodEnvLocal(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("SECRET=1\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	plan := Plan{Actions: []Action{{
		ID:             "secrets.tighten-env-perms",
		Title:          "Tighten .env",
		Commands:       []string{"chmod 600 .env"},
		SafeLocalApply: true,
	}}}
	results := Apply(execx.Local{Dir: dir}, plan)
	if len(results) != 1 || results[0].Status != "applied" {
		t.Fatalf("expected applied, got %+v", results)
	}
	info, err := os.Stat(envPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected mode 0600, got %o", info.Mode().Perm())
	}
}

func TestApplySkipsUnsafeAction(t *testing.T) {
	plan := Plan{Actions: []Action{{
		ID:       "ufw.enable-baseline",
		Title:    "UFW",
		Commands: []string{"ufw --force enable"},
	}}}
	results := Apply(execx.Local{}, plan)
	if len(results) != 1 || results[0].Status != "skipped" {
		t.Fatalf("expected skipped, got %+v", results)
	}
}

func TestApplyOverFakeAdapter(t *testing.T) {
	calls := []string{}
	adapter := fakeAdapter{run: func(cmd string) execx.Result {
		calls = append(calls, cmd)
		return execx.Result{ExitCode: 0}
	}}
	plan := Plan{Actions: []Action{{
		ID:             "secrets.tighten-env-perms",
		Title:          "Tighten .env",
		Commands:       []string{"chmod 600 .env"},
		SafeLocalApply: true,
	}}}
	results := Apply(adapter, plan)
	if len(results) != 1 || results[0].Status != "applied" {
		t.Fatalf("expected applied, got %+v", results)
	}
	if len(calls) != 2 {
		t.Fatalf("expected probe + chmod, got %v", calls)
	}
	if !strings.HasPrefix(calls[0], "test -f ") || !strings.HasPrefix(calls[1], "chmod 600 ") {
		t.Fatalf("unexpected adapter calls: %v", calls)
	}
}

func TestApplyReportsAdapterFailure(t *testing.T) {
	adapter := fakeAdapter{run: func(cmd string) execx.Result {
		if strings.HasPrefix(cmd, "test -f") {
			return execx.Result{ExitCode: 0}
		}
		return execx.Result{ExitCode: 1, Stderr: "permission denied"}
	}}
	plan := Plan{Actions: []Action{{
		ID:             "secrets.tighten-env-perms",
		Title:          "Tighten .env",
		Commands:       []string{"chmod 600 .env"},
		SafeLocalApply: true,
	}}}
	results := Apply(adapter, plan)
	if results[0].Status != "failed" || !strings.Contains(results[0].Detail, "permission denied") {
		t.Fatalf("expected failure with stderr, got %+v", results)
	}
}

func TestApplyReportsMissingTarget(t *testing.T) {
	adapter := fakeAdapter{run: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 1}
	}}
	plan := Plan{Actions: []Action{{
		ID:             "secrets.tighten-env-perms",
		Title:          "Tighten .env",
		Commands:       []string{"chmod 600 .env"},
		SafeLocalApply: true,
	}}}
	results := Apply(adapter, plan)
	if results[0].Status != "failed" || !strings.Contains(results[0].Detail, "does not exist") {
		t.Fatalf("expected does-not-exist failure, got %+v", results)
	}
}

func TestRunSafeCommandRejectsNonAllowlisted(t *testing.T) {
	adapter := fakeAdapter{run: func(string) execx.Result { return execx.Result{} }}
	if err := runSafeCommand(adapter, "rm -rf /"); err == nil {
		t.Fatal("expected rejection of rm")
	}
}

func TestRunChmodRejectsAbsolutePath(t *testing.T) {
	adapter := fakeAdapter{run: func(string) execx.Result { return execx.Result{} }}
	if err := runChmod(adapter, []string{"600", "/etc/.env"}); err == nil {
		t.Fatal("expected rejection of absolute path")
	}
}

func TestRunChmodRejectsTraversal(t *testing.T) {
	adapter := fakeAdapter{run: func(string) execx.Result { return execx.Result{} }}
	if err := runChmod(adapter, []string{"600", "../.env"}); err == nil {
		t.Fatal("expected rejection of parent traversal")
	}
}

func TestRunChmodRejectsNonEnvTarget(t *testing.T) {
	adapter := fakeAdapter{run: func(string) execx.Result { return execx.Result{} }}
	if err := runChmod(adapter, []string{"600", "secrets.txt"}); err == nil {
		t.Fatal("expected rejection of non-.env target")
	}
}

func TestRunChmodRejectsOtherMode(t *testing.T) {
	adapter := fakeAdapter{run: func(string) execx.Result { return execx.Result{} }}
	if err := runChmod(adapter, []string{"777", ".env"}); err == nil {
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

func TestShellQuoteEscapesSingleQuote(t *testing.T) {
	got := shellQuote("a'b")
	want := "'a'\"'\"'b'"
	if got != want {
		t.Fatalf("shellQuote mismatch: %s vs %s", got, want)
	}
	_ = fmt.Sprintf // keep fmt referenced if unused elsewhere
}
