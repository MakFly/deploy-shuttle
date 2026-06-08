package templates

import "fmt"

// ComposeTemplate returns a production docker-compose.yml for the given app
// and preset. port defaults to "8080" for laravel/symfony and "3000" for
// nextjs when left empty.
func ComposeTemplate(app, preset, port string) string {
	if port == "" {
		switch preset {
		case "nextjs":
			port = "3000"
		default:
			port = "8080"
		}
	}

	healthPath := "/"
	switch preset {
	case "laravel":
		healthPath = "/up"
	}

	return fmt.Sprintf(`services:
  web:
    build:
      context: .
      dockerfile: Dockerfile
    expose:
      - "%s"
    env_file: .env
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO", "/dev/null", "http://127.0.0.1:%s%s"]
      interval: 10s
      timeout: 3s
      start_period: 15s
      retries: 3
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: "1.0"
`, port, port, healthPath)
}
