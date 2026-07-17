package cli

import (
	"strings"
	"testing"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
)

func TestGenerateSwarmCaddyConfWithBasicAuth(t *testing.T) {
	cfg := &config.Config{
		App: "myapp",
		Caddy: config.Caddy{
			Routes: map[string]string{
				"myapp.example.com": "web:80",
			},
			BasicAuth: config.CaddyBasicAuth{
				Users: []config.CaddyBasicAuthUser{
					{Username: "audit", Hash: "$2a$14$abcdef"},
				},
			},
		},
	}

	got := generateSwarmCaddyConf(cfg)
	if !strings.Contains(got, "    basic_auth {\n        audit $2a$14$abcdef\n    }\n") {
		t.Fatalf("missing basic auth block:\n%s", got)
	}
	if !strings.Contains(got, "reverse_proxy myapp_web:80") {
		t.Fatalf("missing swarm upstream rewrite:\n%s", got)
	}
}

func TestGenerateSwarmCaddyConfWithIPAllowlist(t *testing.T) {
	cfg := &config.Config{
		App: "myapp",
		Caddy: config.Caddy{
			Routes: map[string]string{
				"myapp.example.com": "web:80",
			},
			HealthPath:  "/healthz",
			IPAllowlist: []string{`203\.0\.113\.4`, `2a01:cb0c:[0-9a-f:]+`},
			// BasicAuth is present but must be ignored when IPAllowlist is set.
			BasicAuth: config.CaddyBasicAuth{
				Users: []config.CaddyBasicAuthUser{{Username: "audit", Hash: "$2a$14$abcdef"}},
			},
		},
	}

	got := generateSwarmCaddyConf(cfg)

	if strings.Contains(got, "basic_auth") {
		t.Fatalf("basic_auth must be replaced by the IP allowlist:\n%s", got)
	}
	if !strings.Contains(got, "@health path /healthz") {
		t.Fatalf("missing health-path exemption:\n%s", got)
	}
	if !strings.Contains(got, `not header_regexp Cf-Connecting-Ip "^(203\.0\.113\.4|2a01:cb0c:[0-9a-f:]+)$"`) {
		t.Fatalf("missing / wrong Cf-Connecting-Ip allowlist regex:\n%s", got)
	}
	if !strings.Contains(got, `respond "Access denied" 403`) {
		t.Fatalf("missing 403 for non-allowed IPs:\n%s", got)
	}
	if !strings.Contains(got, "reverse_proxy myapp_web:80") {
		t.Fatalf("missing swarm upstream rewrite:\n%s", got)
	}
}

func TestGenerateSwarmCaddyConfNoAllowlistUnchanged(t *testing.T) {
	// Without IPAllowlist, output must remain byte-identical to the legacy behavior.
	cfg := &config.Config{
		App:   "myapp",
		Caddy: config.Caddy{Routes: map[string]string{"myapp.example.com": "web:80"}},
	}
	got := generateSwarmCaddyConf(cfg)
	want := "    reverse_proxy myapp_web:80 {\n        header_up Host {host}\n        header_up X-Real-IP {remote}\n    }\n"
	if !strings.Contains(got, want) {
		t.Fatalf("legacy reverse_proxy block changed:\n%s", got)
	}
	if strings.Contains(got, "@not_allowed_ip") {
		t.Fatalf("unexpected allowlist block without IPAllowlist:\n%s", got)
	}
}
