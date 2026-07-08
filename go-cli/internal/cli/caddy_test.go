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
