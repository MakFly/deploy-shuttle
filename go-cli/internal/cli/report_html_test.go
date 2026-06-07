package cli

import (
	"strings"
	"testing"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/readiness"
)

func TestHTMLReportContainsKeyMetadata(t *testing.T) {
	report := readiness.Report{
		Target:      "root@example:7022",
		Score:       70,
		Level:       readiness.Level("risky"),
		ConfigPath:  ".shuttle.yml",
		GeneratedAt: "2026-05-02T13:00:00Z",
		Checks: []readiness.CheckResult{
			{
				ID: "firewall.database_port_public", Title: "DB ports", Category: "firewall",
				Severity: readiness.High, Status: readiness.Failed,
				Summary: "5432 publicly bound", Remediation: "ufw deny 5432/tcp",
				Evidence: map[string]any{"publicPorts": []string{"5432"}},
			},
			{
				ID: "docker.containers_running_as_root", Title: "Root containers", Category: "docker",
				Severity: readiness.Medium, Status: readiness.Failed,
				Summary: "5 containers as root",
			},
			{
				ID: "secrets.env_world_readable", Title: ".env perms", Category: "secrets",
				Severity: readiness.Critical, Status: readiness.Failed,
				Ignored: true, IgnoreReason: "dev only",
			},
		},
	}
	rendered, err := htmlReport(report)
	if err != nil {
		t.Fatalf("htmlReport: %v", err)
	}
	mustContain := []string{
		"<!DOCTYPE html>",
		"DeployShuttle Production Readiness Report",
		"root@example:7022",
		"70/100",
		"firewall.database_port_public",
		"docker.containers_running_as_root",
		"Accepted Risks",
		"dev only",
		"Next Actions",
		"ufw deny 5432/tcp",
		"publicPorts=5432",
	}
	for _, needle := range mustContain {
		if !strings.Contains(rendered, needle) {
			t.Fatalf("rendered HTML missing %q\n%s", needle, rendered)
		}
	}
}

func TestHTMLReportEscapesUserContent(t *testing.T) {
	report := readiness.Report{
		Target: "x<script>alert(1)</script>",
		Checks: []readiness.CheckResult{
			{
				ID: "evil", Title: "<img src=x onerror=alert(1)>", Category: "x",
				Severity: readiness.High, Status: readiness.Failed,
				Summary: "<b>oops</b>",
			},
		},
	}
	rendered, err := htmlReport(report)
	if err != nil {
		t.Fatalf("htmlReport: %v", err)
	}
	if strings.Contains(rendered, "<script>alert(1)</script>") {
		t.Fatal("target should be escaped")
	}
	if strings.Contains(rendered, "<img src=x onerror=alert(1)>") {
		t.Fatal("title should be escaped")
	}
	if !strings.Contains(rendered, "&lt;script&gt;") {
		t.Fatal("expected HTML entities in escaped target")
	}
}

func TestHTMLReportNoFindingsHidesSections(t *testing.T) {
	report := readiness.Report{
		Target: "local",
		Checks: []readiness.CheckResult{
			{ID: "system.os_supported", Status: readiness.Passed, Severity: readiness.High},
		},
	}
	rendered, err := htmlReport(report)
	if err != nil {
		t.Fatalf("htmlReport: %v", err)
	}
	if strings.Contains(rendered, "Next Actions") {
		t.Fatal("Next Actions section should be hidden when no failures exist")
	}
	if strings.Contains(rendered, "Accepted Risks") {
		t.Fatal("Accepted Risks section should be hidden when no ignored checks")
	}
}
