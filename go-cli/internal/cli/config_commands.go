package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/detect"
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

func newInitCommand() *cobra.Command {
	var force bool
	var app string
	var domain string
	var host string
	var user string
	var port int
	var preset string
	var skipDetect bool
	var withCI bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Detect your stack and generate configuration",
		Long: "Analyze the current project to detect the technology stack, then generate:\n" +
			"  - shuttle.yml (deploy config)\n" +
			"  - .shuttle.yml (readiness config with the right preset)\n" +
			"  - optionally, a GitHub Actions workflow (--ci)\n\n" +
			"Use --preset to override auto-detection. Supported presets: " +
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
				stack = detect.Analyze(dir)
				if stack.Preset != "" {
					fmt.Printf("Detected stack: %s\n", stack.Framework)
					for _, sig := range stack.Signals {
						fmt.Printf("  • %s\n", sig)
					}
					fmt.Printf("  → preset: %s\n\n", stack.Preset)
				} else {
					fmt.Println("Could not auto-detect stack. Using generic config.")
					fmt.Println("Tip: use --preset to specify manually.")
				}
			}

			// --- Defaults ---
			if app == "" {
				app = filepath.Base(dir)
			}
			if domain == "" {
				domain = app + ".example.com"
			}
			if host == "" {
				host = "203.0.113.10"
			}
			if user == "" {
				user = "deploy"
			}
			if stack.HealthPath != "" && domain == app+".example.com" {
				// keep domain placeholder
			}

			// --- shuttle.yml ---
			if _, err := os.Stat("shuttle.yml"); err == nil && !force {
				fmt.Println("shuttle.yml already exists (use --force to overwrite)")
			} else {
				if err := os.MkdirAll(".shuttle", 0o755); err != nil {
					return err
				}
				_ = os.WriteFile(filepath.Join(".shuttle", ".gitkeep"), []byte{}, 0o644)
				if err := os.WriteFile("shuttle.yml", []byte(templates.ShuttleYML(app, domain, host, user, port)), 0o644); err != nil {
					return err
				}
				fmt.Println("✓ shuttle.yml created")
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
					workflow := generateCIWorkflow()
					if err := os.WriteFile(ciFile, []byte(workflow), 0o644); err != nil {
						return err
					}
					fmt.Println("✓ .github/workflows/shuttle.yml created")
				}
			}

			// --- Summary ---
			fmt.Println("\nNext steps:")
			fmt.Println("  1. Edit shuttle.yml with your server details")
			if effectivePreset != "" {
				fmt.Println("  2. Review .shuttle.yml (adjust ignore rules if needed)")
				fmt.Println("  3. Run: shuttle doctor")
			} else {
				fmt.Println("  2. Run: shuttle doctor")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")
	cmd.Flags().StringVar(&app, "app", "", "application name (default: directory name)")
	cmd.Flags().StringVar(&domain, "domain", "", "application domain")
	cmd.Flags().StringVar(&host, "host", "", "server host")
	cmd.Flags().StringVar(&user, "user", "", "server user")
	cmd.Flags().IntVar(&port, "port", 22, "server SSH port")
	cmd.Flags().StringVar(&preset, "preset", "", "override auto-detection with a specific preset ("+strings.Join(templates.ReadinessPresets, "|")+")")
	cmd.Flags().BoolVar(&skipDetect, "no-detect", false, "skip stack detection")
	cmd.Flags().BoolVar(&withCI, "ci", false, "also generate a GitHub Actions workflow")
	return cmd
}

func generateCIWorkflow() string {
	return `name: DeployShuttle Readiness
on:
  push:
    branches: [main]
  pull_request:
  workflow_dispatch:

jobs:
  doctor:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install shuttle
        run: curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh

      - name: Run readiness scan
        run: shuttle doctor --fail-below 75
        env:
          SSH_PRIVATE_KEY: ${{ secrets.SSH_PRIVATE_KEY }}
`
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
