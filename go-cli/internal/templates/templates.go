package templates

import (
	"fmt"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
)

func ShuttleYML(app string, domain string, host string, user string) string {
	return fmt.Sprintf(`app: %s
domain: %s
server:
  host: %s
  user: %s

services:
  web:
    port: 3000
    command: bun run start
`, app, domain, host, user)
}

func Caddyfile(cfg *config.Config, upstream string) string {
	domains := Domains(cfg)
	var b strings.Builder
	for _, domain := range domains {
		fmt.Fprintf(&b, "%s {\n", domain)
		if len(cfg.Proxy.Headers) > 0 {
			b.WriteString("  header {\n")
			for key, value := range cfg.Proxy.Headers {
				fmt.Fprintf(&b, "    %s %s\n", key, value)
			}
			b.WriteString("  }\n")
		}
		fmt.Fprintf(&b, "  reverse_proxy %s\n", upstream)
		b.WriteString("}\n\n")
	}
	return b.String()
}

func ComposeDev(cfg *config.Config) string {
	var b strings.Builder
	b.WriteString("services:\n")
	for name, service := range cfg.Services {
		fmt.Fprintf(&b, "  %s:\n", name)
		b.WriteString("    build: .\n")
		if service.Command != "" {
			fmt.Fprintf(&b, "    command: %q\n", service.Command)
		}
		if service.Port != 0 {
			fmt.Fprintf(&b, "    ports:\n      - %q\n", fmt.Sprintf("%d:%d", service.Port, service.Port))
		}
	}
	return b.String()
}

func Domains(cfg *config.Config) []string {
	switch v := cfg.Domain.(type) {
	case string:
		return []string{v}
	case []any:
		out := []string{}
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	default:
		return []string{}
	}
}
