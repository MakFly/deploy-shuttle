package cli

import (
	"fmt"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
)

func writeCaddySecurityHeaders(b *strings.Builder) {
	b.WriteString("    header {\n")
	b.WriteString("        X-Content-Type-Options \"nosniff\"\n")
	b.WriteString("        X-Frame-Options \"DENY\"\n")
	b.WriteString("        Referrer-Policy \"strict-origin-when-cross-origin\"\n")
	b.WriteString("        Strict-Transport-Security \"max-age=31536000; includeSubDomains\"\n")
	b.WriteString("        -Server\n")
	b.WriteString("    }\n\n")
}

func writeCaddyBasicAuth(b *strings.Builder, cfg *config.Config) {
	if len(cfg.Caddy.BasicAuth.Users) == 0 {
		return
	}
	b.WriteString("    basic_auth {\n")
	for _, user := range cfg.Caddy.BasicAuth.Users {
		fmt.Fprintf(b, "        %s %s\n", user.Username, user.Hash)
	}
	b.WriteString("    }\n\n")
}

// writeCaddyReverseProxy writes a reverse_proxy block to upstream at the given indent.
func writeCaddyReverseProxy(b *strings.Builder, upstream, indent string) {
	fmt.Fprintf(b, "%sreverse_proxy %s {\n", indent, upstream)
	fmt.Fprintf(b, "%s    header_up Host {host}\n", indent)
	fmt.Fprintf(b, "%s    header_up X-Real-IP {remote}\n", indent)
	fmt.Fprintf(b, "%s}\n", indent)
}

// writeCaddyAccess writes the access-control + reverse_proxy portion of a site block.
//
// When cfg.Caddy.IPAllowlist is set, it emits a Cloudflare IP allowlist (matched on the
// Cf-Connecting-Ip header; entries are regex fragments) in place of basic auth, and keeps
// cfg.Caddy.HealthPath open so external monitors (e.g. Uptime Kuma) can probe it without
// the allowlisted IP. When the allowlist is empty, the output is byte-for-byte identical to
// the previous basic_auth + reverse_proxy behavior — no regression for existing apps.
func writeCaddyAccess(b *strings.Builder, cfg *config.Config, upstream string) {
	if len(cfg.Caddy.IPAllowlist) == 0 {
		writeCaddyBasicAuth(b, cfg)
		writeCaddyReverseProxy(b, upstream, "    ")
		return
	}

	// Health endpoint stays open — monitors reach it from an IP outside the allowlist.
	if healthPath := strings.TrimSpace(cfg.Caddy.HealthPath); healthPath != "" {
		fmt.Fprintf(b, "    @health path %s\n", healthPath)
		b.WriteString("    handle @health {\n")
		writeCaddyReverseProxy(b, upstream, "        ")
		b.WriteString("    }\n\n")
	}

	// Cloudflare IP allowlist. Everything not from an allowed IP gets a 403.
	regex := "^(" + strings.Join(cfg.Caddy.IPAllowlist, "|") + ")$"
	b.WriteString("    @not_allowed_ip {\n")
	fmt.Fprintf(b, "        not header_regexp Cf-Connecting-Ip \"%s\"\n", regex)
	b.WriteString("    }\n")
	b.WriteString("    handle @not_allowed_ip {\n")
	b.WriteString("        respond \"Access denied\" 403\n")
	b.WriteString("    }\n\n")
	b.WriteString("    handle {\n")
	writeCaddyReverseProxy(b, upstream, "        ")
	b.WriteString("    }\n")
}
