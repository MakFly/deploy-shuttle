package templates

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProComposeOptions controls which services to include in the Pro compose.
type ProComposeOptions struct {
	App       string
	Preset    string
	Port      string
	DB        string // "postgres" | "mysql" | ""
	Redis     bool
	Queue     bool
	Scheduler bool
	Mailpit   bool
}

// ProComposeTemplate assembles a multi-service compose from the given options.
func ProComposeTemplate(opts ProComposeOptions) string {
	if opts.Port == "" {
		opts.Port = "8080"
	}

	healthPath := "/"
	switch opts.Preset {
	case "laravel":
		healthPath = "/up"
	}

	services := make(map[string]any)
	volumes := make(map[string]any)

	// Web service (base)
	webDependsOn := make(map[string]any)

	web := map[string]any{
		"build": map[string]string{
			"context":    ".",
			"dockerfile": "Dockerfile",
		},
		"expose":   []string{opts.Port},
		"env_file": ".env",
		"restart":  "unless-stopped",
		"healthcheck": map[string]any{
			"test":         []string{"CMD", "wget", "-qO", "/dev/null", fmt.Sprintf("http://127.0.0.1:%s%s", opts.Port, healthPath)},
			"interval":     "10s",
			"timeout":      "3s",
			"start_period": "15s",
			"retries":      3,
		},
		"deploy": map[string]any{
			"resources": map[string]any{
				"limits": map[string]string{
					"memory": "512M",
					"cpus":   "1.0",
				},
			},
		},
		"networks": []string{"app-network"},
	}
	services["web"] = web

	// Database
	if opts.DB != "" {
		var block ComposeServiceBlock
		switch opts.DB {
		case "mysql":
			block = MySQLService(opts.App)
		default:
			block = PostgresService(opts.App)
		}
		services[block.Name] = block.Service
		for k, v := range block.Volumes {
			volumes[k] = v
		}
		webDependsOn[block.Name] = map[string]string{"condition": "service_healthy"}
	}

	// Redis
	if opts.Redis {
		block := RedisService()
		services[block.Name] = block.Service
		for k, v := range block.Volumes {
			volumes[k] = v
		}
		webDependsOn["redis"] = map[string]string{"condition": "service_healthy"}
	}

	// Mailpit
	if opts.Mailpit {
		block := MailpitService()
		services[block.Name] = block.Service
	}

	// Workers (preset-specific)
	if opts.Queue {
		switch opts.Preset {
		case "laravel":
			block := LaravelQueueWorker()
			services[block.Name] = block.Service
		case "symfony":
			block := SymfonyMessengerWorker()
			services[block.Name] = block.Service
		}
	}

	if opts.Scheduler {
		switch opts.Preset {
		case "laravel":
			block := LaravelScheduler()
			services[block.Name] = block.Service
			// Also add Horizon if Redis is enabled
			if opts.Redis {
				block := LaravelHorizon()
				services[block.Name] = block.Service
			}
		case "symfony":
			block := SymfonyScheduler()
			services[block.Name] = block.Service
		}
	}

	// Wire depends_on into web
	if len(webDependsOn) > 0 {
		web["depends_on"] = webDependsOn
	}

	// Build the compose file structure
	compose := map[string]any{
		"services": services,
		"networks": map[string]any{
			"app-network": map[string]string{"driver": "bridge"},
		},
	}

	if len(volumes) > 0 {
		compose["volumes"] = volumes
	}

	out, err := yaml.Marshal(compose)
	if err != nil {
		return fmt.Sprintf("# error marshaling compose: %v\n", err)
	}

	return formatComposeOutput(string(out))
}

// formatComposeOutput ensures a predictable top-level key order.
func formatComposeOutput(raw string) string {
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil {
		return raw
	}

	order := []string{"services", "volumes", "networks"}
	var b strings.Builder

	for _, key := range order {
		val, ok := parsed[key]
		if !ok {
			continue
		}
		section := map[string]any{key: val}
		out, err := yaml.Marshal(section)
		if err != nil {
			continue
		}
		b.Write(out)
		b.WriteString("\n")
	}

	return b.String()
}

// ValidateProFlags checks that the Pro flags are valid for the given preset.
func ValidateProFlags(preset, db string, queue, scheduler bool) error {
	if db != "" && db != "postgres" && db != "mysql" {
		return fmt.Errorf("--with-db must be 'postgres' or 'mysql', got %q", db)
	}

	phpPresets := map[string]bool{"laravel": true, "symfony": true}

	if queue && !phpPresets[preset] {
		return fmt.Errorf("--with-queue is only available for laravel and symfony presets, got %q", preset)
	}
	if scheduler && !phpPresets[preset] {
		return fmt.Errorf("--with-scheduler is only available for laravel and symfony presets, got %q", preset)
	}

	return nil
}

// ServiceNames returns an ordered list of service names for display.
func ServiceNames(opts ProComposeOptions) []string {
	names := []string{"web"}
	if opts.DB != "" {
		if opts.DB == "mysql" {
			names = append(names, "mysql")
		} else {
			names = append(names, "postgres")
		}
	}
	if opts.Redis {
		names = append(names, "redis")
	}
	if opts.Queue {
		switch opts.Preset {
		case "laravel":
			names = append(names, "queue")
		case "symfony":
			names = append(names, "messenger")
		}
	}
	if opts.Scheduler {
		names = append(names, "scheduler")
		if opts.Preset == "laravel" && opts.Redis {
			names = append(names, "horizon")
		}
	}
	if opts.Mailpit {
		names = append(names, "mailpit")
	}
	sort.Strings(names[1:])
	return names
}
