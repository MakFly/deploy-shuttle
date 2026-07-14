package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
	"gopkg.in/yaml.v3"
)

func TestHealthcheckFromConfig(t *testing.T) {
	tests := []struct {
		name string
		svc  config.Service
		want []string // expected test[]; nil means healthcheckFromConfig should return nil
	}{
		{"empty→nil", config.Service{}, nil},
		{"command", config.Service{Healthcheck: config.Healthcheck{Type: "command", Command: "echo ok"}}, []string{"CMD-SHELL", "echo ok"}},
		{"bare command", config.Service{Healthcheck: config.Healthcheck{Command: "true"}}, []string{"CMD-SHELL", "true"}},
		{"none disables", config.Service{Healthcheck: config.Healthcheck{Type: "none"}}, []string{"NONE"}},
		{"http path", config.Service{Port: 9000, Healthcheck: config.Healthcheck{Type: "http", Path: "/healthz"}},
			[]string{"CMD-SHELL", "wget -q -O /dev/null http://127.0.0.1:9000/healthz 2>/dev/null || curl -fsS http://127.0.0.1:9000/healthz >/dev/null 2>&1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := healthcheckFromConfig(tt.svc, "3000")
			if tt.want == nil {
				if got != nil {
					t.Fatalf("want nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("want non-nil healthcheck")
			}
			test, _ := got["test"].([]string)
			if !reflect.DeepEqual(test, tt.want) {
				t.Errorf("test = %v, want %v", test, tt.want)
			}
		})
	}
}

func TestHealthcheckFromConfigHTTPUsesDefaultPort(t *testing.T) {
	// No explicit service port → fall back to the detected web port.
	got := healthcheckFromConfig(config.Service{Healthcheck: config.Healthcheck{Type: "http"}}, "8080")
	test := got["test"].([]string)
	if !strings.Contains(test[1], "127.0.0.1:8080/") {
		t.Errorf("expected default port 8080 in probe, got %q", test[1])
	}
}

// Non-web build services must NOT receive the (previously PHP) default check —
// this is exactly what wedged the searxng deploy.
func TestSwarmDefaultHealthcheckOnlyForWebAndNotPHP(t *testing.T) {
	cf := &composeFile{Services: map[string]composeService{
		"searxng": {Image: "searxng/searxng", Expose: []string{"8080"}, Build: map[string]any{"context": "."}},
		"web":     {Expose: []string{"8080"}, Build: map[string]any{"context": "."}},
	}}
	cfg := &config.Config{App: "demo", Caddy: config.Caddy{Network: "edge"}}

	out := generateSwarmStackYAML(cf, cfg, []string{"searxng", "web"}, "127.0.0.1:5080", "8080")

	if strings.Contains(out, "php") {
		t.Error("default healthcheck must not assume PHP")
	}
	var stack swarmStackFile
	if err := yaml.Unmarshal([]byte(out), &stack); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if hc := stack.Services["searxng"].Healthcheck; hc != nil {
		t.Errorf("non-web build service must get no default healthcheck, got %v", hc)
	}
	if stack.Services["web"].Healthcheck == nil {
		t.Error("web service should still get a default healthcheck")
	}
	if !strings.Contains(out, "wget") || !strings.Contains(out, "curl") {
		t.Error("default web healthcheck should probe over HTTP via wget/curl")
	}
}

// A shuttle.yml services.<name>.healthcheck must be honored (previously ignored).
func TestSwarmHonorsShuttleHealthcheck(t *testing.T) {
	cf := &composeFile{Services: map[string]composeService{
		"searxng": {Image: "searxng/searxng", Expose: []string{"8080"}, Build: map[string]any{"context": "."}},
	}}
	cfg := &config.Config{
		App:   "demo",
		Caddy: config.Caddy{Network: "edge"},
		Services: map[string]config.Service{
			"searxng": {Port: 8080, Healthcheck: config.Healthcheck{Type: "command", Command: "python3 -c 'import sys;sys.exit(0)'", Interval: 30, Retries: 3}},
		},
	}

	out := generateSwarmStackYAML(cf, cfg, []string{"searxng"}, "127.0.0.1:5080", "3000")
	if !strings.Contains(out, "python3 -c 'import sys;sys.exit(0)'") {
		t.Errorf("shuttle.yml services.searxng.healthcheck.command must be honored, got:\n%s", out)
	}
}
