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
