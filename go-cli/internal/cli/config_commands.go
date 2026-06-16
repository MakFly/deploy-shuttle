package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/detect"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/license"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/templates"
	"github.com/spf13/cobra"
)

type configFlags struct {
	path string
	env  string
}

func addConfigFlags(cmd *cobra.Command, flags *configFlags) {
	cmd.Flags().StringVar(&flags.path, "config", "", "path to shuttle.yml")
	cmd.Flags().StringVarP(&flags.env, "env", "e", "", "environment overlay")
}

func loadWithFlags(flags configFlags) (*config.Config, error) {
	return config.Load(flags.path, flags.env)
}

func newValidateCommand() *cobra.Command {
	var flags configFlags
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate shuttle.yml without deploying",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadWithFlags(flags)
			if err != nil {
				return err
			}
			if asJSON {
				raw, _ := json.MarshalIndent(cfg, "", "  ")
				fmt.Println(string(raw))
				return nil
			}
			fmt.Printf("Configuration for %q is valid.\n", cfg.App)
			fmt.Printf("Servers: %d\n", len(cfg.Servers))
			fmt.Printf("Services: %d\n", len(cfg.Services))
			return nil
		},
	}
	addConfigFlags(cmd, &flags)
	cmd.Flags().BoolVar(&asJSON, "json", false, "print resolved config as JSON")
	return cmd
}

// servicePortForPreset returns the default container port for a given preset.
func servicePortForPreset(preset string) string {
	switch preset {
	case "laravel":
		return "8000"
	case "symfony":
		return "8080"
	case "nextjs", "node-api":
		return "3000"
	default:
		return "8080"
	}
}

// dockerfileForPreset returns the Dockerfile content for the detected preset.
func dockerfileForPreset(preset string) (string, string) {
	switch preset {
	case "laravel":
		return templates.DockerfileLaravel(), "FrankenPHP + Laravel"
	case "symfony":
		return templates.DockerfileSymfony(), "FrankenPHP + Symfony"
	case "nextjs":
		return templates.DockerfileNextJS(), "Next.js standalone"
	default:
		return "", ""
	}
}

// initShuttleYML generates the shuttle.yml content with Caddy routing.
func initShuttleYML(app, domain, host, user string, port int, email, servicePort, strategy string) string {
	if port == 0 {
		port = 22
	}
	var b strings.Builder
	fmt.Fprintf(&b, "app: %s\n", app)
	fmt.Fprintf(&b, "domain: %s\n", domain)
	b.WriteString("\n")
	b.WriteString("server:\n")
	fmt.Fprintf(&b, "  host: %s\n", host)
	fmt.Fprintf(&b, "  user: %s\n", user)
	fmt.Fprintf(&b, "  port: %d\n", port)
	b.WriteString("\n")
	b.WriteString("deploy:\n")
	fmt.Fprintf(&b, "  strategy: %s\n", strategy)
	b.WriteString("  timeout: 120\n")
	b.WriteString("  retain: 3\n")
	b.WriteString("  compose_files:\n")
	b.WriteString("    - docker-compose.yml\n")
	b.WriteString("  # Reclaim local disk after each deploy: prune the Docker build cache.\n")
	b.WriteString("  # off | capped | all  (capped keeps recent layers, bounded to build_cache_keep).\n")
	b.WriteString("  prune_build_cache: capped\n")
	b.WriteString("  build_cache_keep: 5GB\n")
	b.WriteString("\n")
	b.WriteString("caddy:\n")
	b.WriteString("  conf_dir: /opt/shuttle/caddy/conf.d\n")
	b.WriteString("  reload_command: \"docker kill --signal=SIGUSR1 $(docker ps -qf name=shuttle_caddy)\"\n")
	if email != "" {
		fmt.Fprintf(&b, "  email: %s\n", email)
	}
	b.WriteString("  routes:\n")
	fmt.Fprintf(&b, "    \"%s\": \"web:%s\"\n", domain, servicePort)
	return b.String()
}

func newInitCommand() *cobra.Command {
	var force bool
	var app string
	var domain string
	var host string
	var user string
	var port int
	var preset string
	var email string
	var skipDetect bool
	var withCI bool
	var withDB string
	var withRedis bool
	var withQueue bool
	var withScheduler bool
	var withMailpit bool
	var pro bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Detect your stack and generate configuration",
		Long: "Analyze the current project to detect the technology stack, then generate:\n" +
			"  - shuttle.yml (deploy config)\n" +
			"  - Dockerfile (if a preset matches)\n" +
			"  - docker-compose.yml\n" +
			"  - .dockerignore\n" +
			"  - .shuttle.yml (readiness config with the right preset)\n" +
			"  - optionally, a GitHub Actions workflow (--ci)\n\n" +
			"Run without flags for interactive mode. Use flags to skip prompts (for CI/scripting).\n" +
			"Supported presets: " +
			strings.Join(templates.ReadinessPresets, ", ") + ".",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := os.Getwd()

			// --- Stack detection ---
			var stack detect.Stack
			if preset != "" {
				if !templates.IsReadinessPreset(preset) {
					return fmt.Errorf("unknown preset %q; supported: %s", preset, strings.Join(templates.ReadinessPresets, ", "))
				}
				stack = detect.Stack{Preset: preset, Framework: preset}
			} else if !skipDetect {
				fmt.Println("\nDetecting stack...")
				stack = detect.Analyze(dir)
				if stack.Preset != "" {
					fmt.Printf("  ✓ %s detected", stack.Framework)
					if len(stack.Signals) > 0 {
						fmt.Printf(" (%s)", stack.Signals[0])
					}
					fmt.Println()
				} else {
					fmt.Println("  Could not auto-detect stack. Using generic config.")
					fmt.Println("  Tip: use --preset to specify manually.")
				}
				fmt.Println()
			}

			// --- Compute defaults ---
			defaultApp := filepath.Base(dir)
			defaultDomain := defaultApp + ".example.com"
			defaultUser := "root"
			defaultPort := 22

			// --- Interactive prompts (skip if flag is provided) ---
			// app
			if !cmd.Flags().Changed("app") {
				app = promptString("App name", defaultApp)
			}
			if app == "" {
				app = defaultApp
			}

			// domain
			if !cmd.Flags().Changed("domain") {
				defaultDomain = app + ".example.com"
				domain = promptString("Domain", defaultDomain)
			}
			if domain == "" {
				domain = defaultDomain
			}

			// host
			if !cmd.Flags().Changed("host") {
				host = promptString("Server host", "")
			}
			if host == "" {
				host = "203.0.113.10"
			}

			// port
			if !cmd.Flags().Changed("port") {
				port = promptInt("SSH port", defaultPort)
			}
			if port == 0 {
				port = defaultPort
			}

			// user
			if !cmd.Flags().Changed("user") {
				user = promptString("SSH user", defaultUser)
			}
			if user == "" {
				user = defaultUser
			}

			// deploy strategy
			strategy := "swarm"
			if !cmd.Flags().Changed("host") || !cmd.Flags().Changed("app") {
				strategy = promptChoice("Deploy strategy", []string{"swarm (recommended)", "compose", "blue-green"}, 0)
			}
			if strings.HasPrefix(strategy, "swarm") {
				strategy = "swarm"
			}

			// email
			if !cmd.Flags().Changed("email") {
				email = promptString("Email (for Let's Encrypt)", "")
			}

			fmt.Println()

			// --- Pro flag expansion ---
			if pro {
				if withDB == "" {
					withDB = "postgres"
				}
				withRedis = true
				withQueue = true
				withScheduler = true
				withMailpit = true
				withCI = true
			}

			hasPro := withDB != "" || withRedis || withQueue || withScheduler || withMailpit
			if hasPro {
				if err := templates.ValidateProFlags(stack.Preset, withDB, withQueue, withScheduler); err != nil {
					return err
				}
				if err := license.Require("init --pro"); err != nil {
					return err
				}
				if withQueue && !withRedis {
					fmt.Println("  ⚠ --with-queue works best with --with-redis (queue driver needs a broker)")
				}
			}

			// --- Determine service port ---
			servicePort := servicePortForPreset(stack.Preset)

			// --- shuttle.yml ---
			if _, err := os.Stat("shuttle.yml"); err == nil && !force {
				fmt.Println("shuttle.yml already exists (use --force to overwrite)")
			} else {
				if err := os.MkdirAll(".shuttle", 0o755); err != nil {
					return err
				}
				_ = os.WriteFile(filepath.Join(".shuttle", ".gitkeep"), []byte{}, 0o644)
				content := initShuttleYML(app, domain, host, user, port, email, servicePort, strategy)
				if err := os.WriteFile("shuttle.yml", []byte(content), 0o644); err != nil {
					return err
				}
				fmt.Println("✓ shuttle.yml created")
			}

			// --- Dockerfile ---
			if stack.Preset != "" {
				dockerfileContent, dockerfileLabel := dockerfileForPreset(stack.Preset)
				if dockerfileContent != "" {
					if _, err := os.Stat("Dockerfile"); err == nil && !force {
						fmt.Println("Dockerfile already exists (use --force to overwrite)")
					} else {
						if err := os.WriteFile("Dockerfile", []byte(dockerfileContent), 0o644); err != nil {
							return err
						}
						fmt.Printf("✓ Dockerfile created (%s)\n", dockerfileLabel)
					}
				}
			}

			// --- Framework config files ---
			if stack.Preset == "laravel" {
				os.MkdirAll("docker", 0o755)
				if err := os.WriteFile("docker/opcache.ini", []byte(templates.LaravelOpcacheIni()), 0o644); err != nil {
					return err
				}
				fmt.Println("✓ docker/opcache.ini created (OPcache production tuning)")
			}
			if stack.Preset == "laravel" || stack.Preset == "symfony" {
				os.MkdirAll("docker", 0o755)
				if err := os.WriteFile("docker/docker-secrets-entrypoint.sh", []byte(templates.SecretsEntrypoint()), 0o755); err != nil {
					return err
				}
				fmt.Println("✓ docker/docker-secrets-entrypoint.sh created")
			}
			if stack.Preset == "symfony" {
				os.MkdirAll("docker/conf.d", 0o755)
				if err := os.WriteFile("docker/Caddyfile", []byte(templates.SymfonyCaddyfile()), 0o644); err != nil {
					return err
				}
				if err := os.WriteFile("docker/conf.d/10-app.ini", []byte(templates.SymfonyBaseIni()), 0o644); err != nil {
					return err
				}
				if err := os.WriteFile("docker/conf.d/20-app.prod.ini", []byte(templates.SymfonyProdIni()), 0o644); err != nil {
					return err
				}
				fmt.Println("✓ docker/Caddyfile created (FrankenPHP worker mode)")
				fmt.Println("✓ docker/conf.d/*.ini created (OPcache + PHP production tuning)")
			}

			// --- docker-compose.yml ---
			var composeContent string
			if hasPro {
				opts := templates.ProComposeOptions{
					App:       app,
					Preset:    stack.Preset,
					Port:      servicePort,
					DB:        withDB,
					Redis:     withRedis,
					Queue:     withQueue,
					Scheduler: withScheduler,
					Mailpit:   withMailpit,
				}
				composeContent = templates.ProComposeTemplate(opts)
			} else {
				composeContent = templates.ComposeTemplate(app, stack.Preset, servicePort)
			}
			if composeContent != "" {
				if _, err := os.Stat("docker-compose.yml"); err == nil && !force {
					fmt.Println("docker-compose.yml already exists (use --force to overwrite)")
				} else {
					if err := os.WriteFile("docker-compose.yml", []byte(composeContent), 0o644); err != nil {
						return err
					}
					if hasPro {
						fmt.Printf("✓ docker-compose.yml created (Pro: %s)\n", strings.Join(templates.ServiceNames(templates.ProComposeOptions{
							Preset: stack.Preset, DB: withDB, Redis: withRedis,
							Queue: withQueue, Scheduler: withScheduler, Mailpit: withMailpit,
						}), ", "))
					} else {
						fmt.Println("✓ docker-compose.yml created")
					}
				}
			}

			// --- .dockerignore ---
			ignoreContent := templates.Dockerignore(stack.Preset)
			if ignoreContent != "" {
				if _, err := os.Stat(".dockerignore"); err == nil && !force {
					fmt.Println(".dockerignore already exists (use --force to overwrite)")
				} else {
					if err := os.WriteFile(".dockerignore", []byte(ignoreContent), 0o644); err != nil {
						return err
					}
					fmt.Println("✓ .dockerignore created")
				}
			}

			// --- .shuttle.yml ---
			effectivePreset := stack.Preset
			if effectivePreset != "" {
				if _, err := os.Stat(".shuttle.yml"); err == nil && !force {
					if preset != "" {
						return fmt.Errorf(".shuttle.yml already exists; use --force to overwrite")
					}
					fmt.Println(".shuttle.yml already exists (use --force to overwrite)")
				} else {
					body := templates.DeployShuttleYML(effectivePreset, domain)
					if body != "" {
						if err := os.WriteFile(".shuttle.yml", []byte(body), 0o644); err != nil {
							return err
						}
						fmt.Printf("✓ .shuttle.yml created (preset: %s)\n", effectivePreset)
					}
				}
			}

			// --- .env.example ---
			var envContent string
			if hasPro {
				envContent = templates.EnvExamplePro(stack.Preset, withDB)
			} else {
				envContent = templates.EnvExample(stack.Preset)
			}
			if envContent != "" {
				if _, err := os.Stat(".env.example"); err == nil && !force {
					fmt.Println(".env.example already exists (use --force to overwrite)")
				} else {
					if err := os.WriteFile(".env.example", []byte(envContent), 0o644); err != nil {
						return err
					}
					fmt.Println("✓ .env.example created")
				}
			}

			// --- CI workflow ---
			if withCI {
				ciDir := filepath.Join(".github", "workflows")
				ciFile := filepath.Join(ciDir, "shuttle.yml")
				if _, err := os.Stat(ciFile); err == nil && !force {
					fmt.Println("CI workflow already exists (use --force to overwrite)")
				} else {
					if err := os.MkdirAll(ciDir, 0o755); err != nil {
						return err
					}
					var workflow string
					if hasPro {
						workflow = templates.CIWorkflowPro(stack.Preset, withDB)
					} else {
						workflow = templates.CIWorkflow(stack.Preset)
					}
					if err := os.WriteFile(ciFile, []byte(workflow), 0o644); err != nil {
						return err
					}
					fmt.Println("✓ .github/workflows/shuttle.yml created")
				}
			}

			// --- Summary ---
			fmt.Println("\nNext steps:")
			fmt.Println("  1. shuttle provision    (bootstrap your VPS)")
			fmt.Println("  2. shuttle deploy       (build and ship)")
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")
	cmd.Flags().StringVar(&app, "app", "", "application name (default: directory name)")
	cmd.Flags().StringVar(&domain, "domain", "", "application domain")
	cmd.Flags().StringVar(&host, "host", "", "server host")
	cmd.Flags().StringVar(&user, "user", "", "server user")
	cmd.Flags().IntVar(&port, "port", 22, "server SSH port")
	cmd.Flags().StringVar(&email, "email", "", "email for Let's Encrypt")
	cmd.Flags().StringVar(&preset, "preset", "", "override auto-detection with a specific preset ("+strings.Join(templates.ReadinessPresets, "|")+")")
	cmd.Flags().BoolVar(&skipDetect, "no-detect", false, "skip stack detection")
	cmd.Flags().BoolVar(&withCI, "ci", false, "also generate a GitHub Actions workflow")
	cmd.Flags().StringVar(&withDB, "with-db", "", "add database service (postgres|mysql) [Pro]")
	cmd.Flags().BoolVar(&withRedis, "with-redis", false, "add Redis service [Pro]")
	cmd.Flags().BoolVar(&withQueue, "with-queue", false, "add queue worker service [Pro]")
	cmd.Flags().BoolVar(&withScheduler, "with-scheduler", false, "add scheduler service [Pro]")
	cmd.Flags().BoolVar(&withMailpit, "with-mailpit", false, "add Mailpit dev mail service [Pro]")
	cmd.Flags().BoolVar(&pro, "pro", false, "enable all Pro services with sensible defaults")
	return cmd
}

func newNewCommand() *cobra.Command {
	var framework string
	cmd := &cobra.Command{
		Use:   "new <directory>",
		Short: "Scaffold a new project with shuttle.yml, Dockerfile, and services",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if framework == "" {
				framework = "node"
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dir, "shuttle.yml"), []byte(templates.ShuttleYML(filepath.Base(dir), filepath.Base(dir)+".example.com", "203.0.113.10", "deploy", 22)), 0o644); err != nil {
				return err
			}
			dockerfile := "FROM oven/bun:1\nWORKDIR /app\nCOPY . .\nCMD [\"bun\", \"run\", \"start\"]\n"
			if framework == "node" {
				dockerfile = "FROM node:22-alpine\nWORKDIR /app\nCOPY . .\nCMD [\"node\", \"server.js\"]\n"
			}
			return os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfile), 0o644)
		},
	}
	cmd.Flags().StringVar(&framework, "framework", "node", "framework template")
	return cmd
}
