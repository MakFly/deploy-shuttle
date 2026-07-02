package cli

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// All deploy-config flags are passed so no pre-wizard prompt consumes stdin;
// the fed lines answer the Pro wizard only.
func proWizardArgs() []string {
	return []string{
		"--preset", "laravel", "--pro", "--force",
		"--app", "demo", "--domain", "demo.example.com",
		"--host", "203.0.113.10", "--port", "22", "--user", "root",
		"--email", "ops@demo.example.com",
	}
}

func TestInitProWizardAnswers(t *testing.T) {
	withTempDir(t, func(dir string) {
		// mysql, no redis, no queue, scheduler, mailpit, ci
		stdinReader = bufio.NewReader(strings.NewReader("2\nn\nn\ny\ny\ny\n"))
		defer func() { stdinReader = nil }()

		cmd := newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs(proWizardArgs())
		if err := cmd.Execute(); err != nil {
			t.Fatalf("init --pro wizard run failed: %v", err)
		}

		body, err := os.ReadFile(filepath.Join(dir, "docker-compose.yml"))
		if err != nil {
			t.Fatalf("read compose: %v", err)
		}
		content := string(body)
		if !strings.Contains(content, "mysql") {
			t.Error("wizard answer 'mysql' should produce a mysql service")
		}
		if strings.Contains(content, "image: redis") {
			t.Error("wizard answer 'no redis' should not produce a redis service")
		}
		if !strings.Contains(content, "mailpit") {
			t.Error("wizard answer 'yes mailpit' should produce a mailpit service")
		}
	})
}

func TestInitProWizardEOFKeepsDefaultSet(t *testing.T) {
	withTempDir(t, func(dir string) {
		// EOF stdin = non-TTY/scripted run: every prompt returns its default,
		// which must match the historical --pro auto-enable set.
		stdinReader = bufio.NewReader(strings.NewReader(""))
		defer func() { stdinReader = nil }()

		cmd := newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs(proWizardArgs())
		if err := cmd.Execute(); err != nil {
			t.Fatalf("init --pro with EOF stdin failed: %v", err)
		}

		body, err := os.ReadFile(filepath.Join(dir, "docker-compose.yml"))
		if err != nil {
			t.Fatalf("read compose: %v", err)
		}
		content := string(body)
		for _, svc := range []string{"postgres", "redis", "mailpit"} {
			if !strings.Contains(content, svc) {
				t.Errorf("EOF defaults should include %s (historical --pro set)", svc)
			}
		}
	})
}

func TestInitProWizardExplicitFlagsSkipPrompts(t *testing.T) {
	withTempDir(t, func(dir string) {
		// Every --with-* flag is explicit, so the wizard must not read stdin
		// at all: an empty reader would turn unanswered prompts into defaults,
		// which would re-enable redis here.
		stdinReader = bufio.NewReader(strings.NewReader(""))
		defer func() { stdinReader = nil }()

		cmd := newInitCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs(append(proWizardArgs(),
			"--with-db", "postgres", "--with-redis=false", "--with-queue=false",
			"--with-scheduler=false", "--with-mailpit=false", "--ci=false"))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("init --pro with explicit flags failed: %v", err)
		}

		body, err := os.ReadFile(filepath.Join(dir, "docker-compose.yml"))
		if err != nil {
			t.Fatalf("read compose: %v", err)
		}
		content := string(body)
		if !strings.Contains(content, "postgres") {
			t.Error("explicit --with-db postgres should produce a postgres service")
		}
		if strings.Contains(content, "image: redis") {
			t.Error("explicit --with-redis=false must not be overridden by wizard defaults")
		}
	})
}
