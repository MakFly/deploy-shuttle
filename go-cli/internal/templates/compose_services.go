package templates

// ComposeServiceBlock represents a single service to add to a compose file.
type ComposeServiceBlock struct {
	Name    string
	Service map[string]any
	Volumes map[string]any
	EnvVars map[string]string
}

func PostgresService(app string) ComposeServiceBlock {
	return ComposeServiceBlock{
		Name: "postgres",
		Service: map[string]any{
			"image":   "postgres:16",
			"restart": "unless-stopped",
			"environment": map[string]string{
				"POSTGRES_DB":       app,
				"POSTGRES_USER":     app,
				"POSTGRES_PASSWORD": "${DB_PASSWORD}",
			},
			"volumes": []string{"postgres_data:/var/lib/postgresql/data"},
			"healthcheck": map[string]any{
				"test":         []string{"CMD-SHELL", "pg_isready -U " + app},
				"interval":     "10s",
				"timeout":      "3s",
				"start_period": "10s",
				"retries":      3,
			},
			"networks": []string{"app-network"},
		},
		Volumes: map[string]any{"postgres_data": nil},
	}
}

func MySQLService(app string) ComposeServiceBlock {
	return ComposeServiceBlock{
		Name: "mysql",
		Service: map[string]any{
			"image":   "mysql:8.4",
			"restart": "unless-stopped",
			"environment": map[string]string{
				"MYSQL_DATABASE":      app,
				"MYSQL_USER":          app,
				"MYSQL_PASSWORD":      "${DB_PASSWORD}",
				"MYSQL_ROOT_PASSWORD": "${DB_ROOT_PASSWORD}",
			},
			"volumes": []string{"mysql_data:/var/lib/mysql"},
			"healthcheck": map[string]any{
				"test":         []string{"CMD", "mysqladmin", "ping", "-h", "127.0.0.1"},
				"interval":     "10s",
				"timeout":      "3s",
				"start_period": "15s",
				"retries":      3,
			},
			"networks": []string{"app-network"},
		},
		Volumes: map[string]any{"mysql_data": nil},
	}
}

func RedisService() ComposeServiceBlock {
	return ComposeServiceBlock{
		Name: "redis",
		Service: map[string]any{
			"image":   "redis:7-alpine",
			"restart": "unless-stopped",
			"command": "redis-server --appendonly yes",
			"volumes": []string{"redis_data:/data"},
			"healthcheck": map[string]any{
				"test":     []string{"CMD", "redis-cli", "ping"},
				"interval": "10s",
				"timeout":  "3s",
				"retries":  3,
			},
			"networks": []string{"app-network"},
		},
		Volumes: map[string]any{"redis_data": nil},
	}
}

func MailpitService() ComposeServiceBlock {
	return ComposeServiceBlock{
		Name: "mailpit",
		Service: map[string]any{
			"image":    "axllent/mailpit:latest",
			"restart":  "unless-stopped",
			"ports":    []string{"1025:1025", "8025:8025"},
			"networks": []string{"app-network"},
		},
	}
}

func LaravelQueueWorker() ComposeServiceBlock {
	return ComposeServiceBlock{
		Name: "queue",
		Service: map[string]any{
			"build": map[string]string{
				"context":    ".",
				"dockerfile": "Dockerfile",
			},
			"restart":  "unless-stopped",
			"command":  "php artisan queue:work --tries=3 --timeout=90",
			"env_file": ".env",
			"depends_on": map[string]any{
				"web": map[string]string{"condition": "service_healthy"},
			},
			"deploy": map[string]any{
				"resources": map[string]any{
					"limits": map[string]string{
						"memory": "256M",
						"cpus":   "0.5",
					},
				},
			},
			"networks": []string{"app-network"},
		},
	}
}

func LaravelScheduler() ComposeServiceBlock {
	return ComposeServiceBlock{
		Name: "scheduler",
		Service: map[string]any{
			"build": map[string]string{
				"context":    ".",
				"dockerfile": "Dockerfile",
			},
			"restart":  "unless-stopped",
			"command":  "php artisan schedule:work",
			"env_file": ".env",
			"depends_on": map[string]any{
				"web": map[string]string{"condition": "service_healthy"},
			},
			"deploy": map[string]any{
				"resources": map[string]any{
					"limits": map[string]string{
						"memory": "128M",
						"cpus":   "0.25",
					},
				},
			},
			"networks": []string{"app-network"},
		},
	}
}

func LaravelHorizon() ComposeServiceBlock {
	return ComposeServiceBlock{
		Name: "horizon",
		Service: map[string]any{
			"build": map[string]string{
				"context":    ".",
				"dockerfile": "Dockerfile",
			},
			"restart":  "unless-stopped",
			"command":  "php artisan horizon",
			"env_file": ".env",
			"depends_on": map[string]any{
				"web":   map[string]string{"condition": "service_healthy"},
				"redis": map[string]string{"condition": "service_healthy"},
			},
			"deploy": map[string]any{
				"resources": map[string]any{
					"limits": map[string]string{
						"memory": "256M",
						"cpus":   "0.5",
					},
				},
			},
			"networks": []string{"app-network"},
		},
	}
}

func SymfonyMessengerWorker() ComposeServiceBlock {
	return ComposeServiceBlock{
		Name: "messenger",
		Service: map[string]any{
			"build": map[string]string{
				"context":    ".",
				"dockerfile": "Dockerfile",
			},
			"restart":  "unless-stopped",
			"command":  "php bin/console messenger:consume async --time-limit=3600 --memory-limit=256M",
			"env_file": ".env",
			"depends_on": map[string]any{
				"web": map[string]string{"condition": "service_healthy"},
			},
			"deploy": map[string]any{
				"resources": map[string]any{
					"limits": map[string]string{
						"memory": "256M",
						"cpus":   "0.5",
					},
				},
			},
			"networks": []string{"app-network"},
		},
	}
}

func SymfonyScheduler() ComposeServiceBlock {
	return ComposeServiceBlock{
		Name: "scheduler",
		Service: map[string]any{
			"build": map[string]string{
				"context":    ".",
				"dockerfile": "Dockerfile",
			},
			"restart":  "unless-stopped",
			"command":  "php bin/console messenger:consume scheduler_default -vv --time-limit=3600",
			"env_file": ".env",
			"depends_on": map[string]any{
				"web": map[string]string{"condition": "service_healthy"},
			},
			"deploy": map[string]any{
				"resources": map[string]any{
					"limits": map[string]string{
						"memory": "128M",
						"cpus":   "0.25",
					},
				},
			},
			"networks": []string{"app-network"},
		},
	}
}
