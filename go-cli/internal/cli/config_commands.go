package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
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
	var preset string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate shuttle.yml (and optionally a readiness preset)",
		Long: "Generate a shuttle.yml deploy config. With --preset <stack>, also " +
			"emit an opinionated .deployshuttle.yml so 'doctor' produces fewer " +
			"false positives on day one. Supported presets: " +
			strings.Join(templates.ReadinessPresets, ", ") + ".",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app == "" {
				app = "myapp"
			}
			if domain == "" {
				domain = "myapp.example.com"
			}
			if host == "" {
				host = "203.0.113.10"
			}
			if user == "" {
				user = "deploy"
			}
			if preset != "" && !templates.IsReadinessPreset(preset) {
				return fmt.Errorf("unknown preset %q; supported: %s", preset, strings.Join(templates.ReadinessPresets, ", "))
			}
			if _, err := os.Stat("shuttle.yml"); err == nil && !force {
				return fmt.Errorf("shuttle.yml already exists; use --force to overwrite")
			}
			if err := os.MkdirAll(".shuttle", 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(".shuttle", ".gitkeep"), []byte{}, 0o644); err != nil {
				return err
			}
			if err := os.WriteFile("shuttle.yml", []byte(templates.ShuttleYML(app, domain, host, user)), 0o644); err != nil {
				return err
			}
			if preset == "" {
				return nil
			}
			if _, err := os.Stat(".deployshuttle.yml"); err == nil && !force {
				return fmt.Errorf(".deployshuttle.yml already exists; use --force to overwrite")
			}
			body := templates.DeployShuttleYML(preset, domain)
			if body == "" {
				return fmt.Errorf("preset %q produced an empty config", preset)
			}
			return os.WriteFile(".deployshuttle.yml", []byte(body), 0o644)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing shuttle.yml or .deployshuttle.yml")
	cmd.Flags().StringVar(&app, "app", "", "application name")
	cmd.Flags().StringVar(&domain, "domain", "", "application domain")
	cmd.Flags().StringVar(&host, "host", "", "server host")
	cmd.Flags().StringVar(&user, "user", "", "server user")
	cmd.Flags().StringVar(&preset, "preset", "", "also generate .deployshuttle.yml for the given stack ("+strings.Join(templates.ReadinessPresets, "|")+")")
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
			if err := os.WriteFile(filepath.Join(dir, "shuttle.yml"), []byte(templates.ShuttleYML(filepath.Base(dir), filepath.Base(dir)+".example.com", "203.0.113.10", "deploy")), 0o644); err != nil {
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
