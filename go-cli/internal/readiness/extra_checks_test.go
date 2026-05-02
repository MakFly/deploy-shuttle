package readiness

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/execx"
)

func TestUpdatesPendingSkippedWithoutApt(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 1}
	}}
	res := checkUpdatesPending(adapter)
	if res.Status != Skipped {
		t.Fatalf("expected skipped, got %+v", res)
	}
}

func TestUpdatesPendingPassesWhenZero(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		if strings.Contains(cmd, "command -v apt") {
			return execx.Result{ExitCode: 0}
		}
		return execx.Result{ExitCode: 0, Stdout: "0\n"}
	}}
	res := checkUpdatesPending(adapter)
	if res.Status != Passed {
		t.Fatalf("expected passed, got %+v", res)
	}
}

func TestUpdatesPendingFailsWithCount(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		if strings.Contains(cmd, "command -v apt") {
			return execx.Result{ExitCode: 0}
		}
		return execx.Result{ExitCode: 0, Stdout: "12\n"}
	}}
	res := checkUpdatesPending(adapter)
	if res.Status != Failed {
		t.Fatalf("expected failed, got %+v", res)
	}
	if res.Severity != Medium {
		t.Fatalf("expected medium severity, got %s", res.Severity)
	}
}

func TestUpdatesPendingHighSeverityWhenLargeBacklog(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		if strings.Contains(cmd, "command -v apt") {
			return execx.Result{ExitCode: 0}
		}
		return execx.Result{ExitCode: 0, Stdout: "120\n"}
	}}
	res := checkUpdatesPending(adapter)
	if res.Severity != High {
		t.Fatalf("expected high severity, got %s", res.Severity)
	}
}

func TestMemoryLowFailsBelow10Percent(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0, Stdout: "8000 400\n"}
	}}
	res := checkMemoryLow(adapter)
	if res.Status != Failed {
		t.Fatalf("expected failed, got %+v", res)
	}
}

func TestMemoryLowCriticalSeverityBelow5Percent(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0, Stdout: "8000 200\n"}
	}}
	res := checkMemoryLow(adapter)
	if res.Severity != High {
		t.Fatalf("expected high severity, got %s", res.Severity)
	}
}

func TestMemoryLowPassesWithHeadroom(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0, Stdout: "8000 4000\n"}
	}}
	res := checkMemoryLow(adapter)
	if res.Status != Passed {
		t.Fatalf("expected passed, got %+v", res)
	}
}

func TestMemoryLowSkippedOnEmptyOutput(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0, Stdout: ""}
	}}
	res := checkMemoryLow(adapter)
	if res.Status != Skipped {
		t.Fatalf("expected skipped, got %+v", res)
	}
}

func TestSSHPortDefaultFailsOnPort22(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0, Stdout: "Port 22\n"}
	}}
	res := checkSSHPortDefault(adapter)
	if res.Status != Failed {
		t.Fatalf("expected failed, got %+v", res)
	}
}

func TestSSHPortDefaultPassesOnCustomPort(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0, Stdout: "Port 7022\n"}
	}}
	res := checkSSHPortDefault(adapter)
	if res.Status != Passed {
		t.Fatalf("expected passed, got %+v", res)
	}
}

func TestSSHPortDefaultFailsWhenDirectiveAbsent(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		// First call greps for Port: returns no match (exit 1).
		// Then probe for sshd_config readability succeeds (exit 0).
		if strings.Contains(cmd, "grep -E '^[[:space:]]*Port") {
			return execx.Result{ExitCode: 1}
		}
		return execx.Result{ExitCode: 0}
	}}
	res := checkSSHPortDefault(adapter)
	if res.Status != Failed {
		t.Fatalf("expected failed (default 22), got %+v", res)
	}
}

func TestDockerPublishedSensitivePortsFlagsBinding(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0, Stdout: "pg_main\t0.0.0.0:5432->5432/tcp\nweb\t0.0.0.0:443->443/tcp\n"}
	}}
	res := checkDockerPublishedSensitivePorts(adapter)
	if res.Status != Failed {
		t.Fatalf("expected failed, got %+v", res)
	}
	if !strings.Contains(res.Summary, "pg_main:5432") {
		t.Fatalf("expected summary to include pg_main:5432, got %q", res.Summary)
	}
}

func TestDockerPublishedSensitivePortsPassesWithLocalhostBinding(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0, Stdout: "pg_main\t127.0.0.1:5432->5432/tcp\n"}
	}}
	res := checkDockerPublishedSensitivePorts(adapter)
	if res.Status != Passed {
		t.Fatalf("expected passed, got %+v", res)
	}
}

func TestDockerPublishedSensitivePortsSkippedWithoutDocker(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 1}
	}}
	res := checkDockerPublishedSensitivePorts(adapter)
	if res.Status != Skipped {
		t.Fatalf("expected skipped, got %+v", res)
	}
}

func TestCaddySecurityHeadersFailsOnMissing(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0, Stdout: "__caddyfile__=/etc/caddy/Caddyfile\nexample.com {\n  reverse_proxy app:3000\n}\n"}
	}}
	res := checkCaddySecurityHeaders(adapter)
	if res.Status != Failed {
		t.Fatalf("expected failed, got %+v", res)
	}
	missing, _ := res.Evidence["missing"].([]string)
	if len(missing) != 3 {
		t.Fatalf("expected 3 missing headers, got %v", missing)
	}
}

func TestCaddySecurityHeadersPassesWithBaseline(t *testing.T) {
	body := "__caddyfile__=/etc/caddy/Caddyfile\nexample.com {\n  header Strict-Transport-Security max-age=31536000\n  header X-Content-Type-Options nosniff\n  header Referrer-Policy strict-origin\n}\n"
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0, Stdout: body}
	}}
	res := checkCaddySecurityHeaders(adapter)
	if res.Status != Passed {
		t.Fatalf("expected passed, got %+v", res)
	}
}

func TestCaddySecurityHeadersSkippedWhenNoCaddyfile(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 1}
	}}
	res := checkCaddySecurityHeaders(adapter)
	if res.Status != Skipped {
		t.Fatalf("expected skipped, got %+v", res)
	}
}

func TestCaddyConfigValidSkippedWithoutBinary(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 1}
	}}
	res := checkCaddyConfigValid(adapter)
	if res.Status != Skipped {
		t.Fatalf("expected skipped, got %+v", res)
	}
}

func TestCaddyConfigValidPassesWhenValidate0(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		switch {
		case strings.Contains(cmd, "command -v caddy"):
			return execx.Result{ExitCode: 0}
		case strings.Contains(cmd, "for f in /etc/caddy/Caddyfile"):
			return execx.Result{ExitCode: 0, Stdout: "/etc/caddy/Caddyfile\n"}
		case strings.HasPrefix(cmd, "caddy validate"):
			return execx.Result{ExitCode: 0}
		}
		return execx.Result{ExitCode: 1}
	}}
	res := checkCaddyConfigValid(adapter)
	if res.Status != Passed {
		t.Fatalf("expected passed, got %+v", res)
	}
}

func TestCaddyConfigValidFailsOnValidateError(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		switch {
		case strings.Contains(cmd, "command -v caddy"):
			return execx.Result{ExitCode: 0}
		case strings.Contains(cmd, "for f in /etc/caddy/Caddyfile"):
			return execx.Result{ExitCode: 0, Stdout: "/etc/caddy/Caddyfile\n"}
		case strings.HasPrefix(cmd, "caddy validate"):
			return execx.Result{ExitCode: 1, Stdout: "syntax error at line 4"}
		}
		return execx.Result{ExitCode: 1}
	}}
	res := checkCaddyConfigValid(adapter)
	if res.Status != Failed {
		t.Fatalf("expected failed, got %+v", res)
	}
}

func TestTLSCertificateSkippedWithoutDomain(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0}
	}}
	res := checkTLSCertificate("")(adapter)
	if res.Status != Skipped {
		t.Fatalf("expected skipped, got %+v", res)
	}
}

func TestTLSCertificateFailsWhenOpenSSLReturnsEmpty(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		if strings.Contains(cmd, "command -v openssl") {
			return execx.Result{ExitCode: 0}
		}
		return execx.Result{ExitCode: 0, Stdout: ""}
	}}
	res := checkTLSCertificate("example.com")(adapter)
	if res.Status != Failed {
		t.Fatalf("expected failed, got %+v", res)
	}
}

func TestTLSCertificateFailsWhenExpiringSoon(t *testing.T) {
	expires := time.Now().Add(7*24*time.Hour).UTC().Format("Jan 2 15:04:05 2006") + " GMT"
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		if strings.Contains(cmd, "command -v openssl") {
			return execx.Result{ExitCode: 0}
		}
		return execx.Result{ExitCode: 0, Stdout: "notAfter=" + expires + "\nsubject=CN=example.com\n"}
	}}
	res := checkTLSCertificate("example.com")(adapter)
	if res.Status != Failed {
		t.Fatalf("expected failed, got %+v", res)
	}
	if res.Severity != High {
		t.Fatalf("expected high severity, got %s", res.Severity)
	}
}

func TestTLSCertificatePassesWhenLongLived(t *testing.T) {
	expires := time.Now().Add(60*24*time.Hour).UTC().Format("Jan 2 15:04:05 2006") + " GMT"
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		if strings.Contains(cmd, "command -v openssl") {
			return execx.Result{ExitCode: 0}
		}
		return execx.Result{ExitCode: 0, Stdout: fmt.Sprintf("notAfter=%s\nsubject=CN=example.com\n", expires)}
	}}
	res := checkTLSCertificate("example.com")(adapter)
	if res.Status != Passed {
		t.Fatalf("expected passed, got %+v", res)
	}
}

func TestSecretsWeakPermissionsPassesOnEmpty(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0, Stdout: ""}
	}}
	res := checkSecretsWeakPermissions(adapter)
	if res.Status != Passed {
		t.Fatalf("expected passed, got %+v", res)
	}
}

func TestSecretsWeakPermissionsFailsWhenFilesFound(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0, Stdout: "./.env.production\n./certs/server.key\n"}
	}}
	res := checkSecretsWeakPermissions(adapter)
	if res.Status != Failed {
		t.Fatalf("expected failed, got %+v", res)
	}
	files, _ := res.Evidence["files"].([]string)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %v", files)
	}
}

func TestHealthEndpointSkippedWithoutDomain(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		return execx.Result{ExitCode: 0}
	}}
	res := checkHealthEndpoint("", "/health")(adapter)
	if res.Status != Skipped {
		t.Fatalf("expected skipped, got %+v", res)
	}
}

func TestHealthEndpointPassesOn200(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		if strings.Contains(cmd, "command -v curl") {
			return execx.Result{ExitCode: 0}
		}
		return execx.Result{ExitCode: 0, Stdout: "200"}
	}}
	res := checkHealthEndpoint("example.com", "/health")(adapter)
	if res.Status != Passed {
		t.Fatalf("expected passed, got %+v", res)
	}
}

func TestHealthEndpointFailsOn500(t *testing.T) {
	adapter := scriptedAdapter{respond: func(cmd string) execx.Result {
		if strings.Contains(cmd, "command -v curl") {
			return execx.Result{ExitCode: 0}
		}
		return execx.Result{ExitCode: 22, Stdout: "500"}
	}}
	res := checkHealthEndpoint("example.com", "/health")(adapter)
	if res.Status != Failed {
		t.Fatalf("expected failed, got %+v", res)
	}
}
