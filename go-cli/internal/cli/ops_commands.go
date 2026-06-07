package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/runtime"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/shell"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/templates"
	"github.com/spf13/cobra"
)

func newProvisionCommand() *cobra.Command {
	var flags configFlags
	var username string
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Bootstrap a VPS: install Docker, Caddy, create deploy user",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadWithFlags(flags)
			if err != nil {
				return err
			}
			for name, group := range cfg.Servers {
				user := group.User
				if username != "" {
					user = username
				}
				for _, host := range group.Hosts {
					fmt.Printf("Provisioning %s@%s (group: %s)\n", user, host, name)
					provisionGroup := config.ServerGroup{Hosts: group.Hosts, User: "root", Port: group.Port}
					client, err := connectSSH(provisionGroup, host)
					if err != nil {
						return err
					}
					sshPort := group.Port
					commands := []string{
						"apt-get update -y",
						"apt-get install -y ca-certificates curl gnupg ufw fail2ban",
						fmt.Sprintf("id -u %s >/dev/null 2>&1 || useradd -m -s /bin/bash %s", shell.Escape(user), shell.Escape(user)),
						"curl -fsSL https://get.docker.com | sh",
						fmt.Sprintf("usermod -aG docker %s", shell.Escape(user)),
						fmt.Sprintf("mkdir -p %s && chown -R %s:%s %s", shell.Escape(runtime.AppDir(cfg.App)), shell.Escape(user), shell.Escape(user), shell.Escape(runtime.AppDir(cfg.App))),
						fmt.Sprintf("ufw allow %d/tcp && ufw allow 80/tcp && ufw allow 443/tcp && ufw --force enable", sshPort),
					}
					for _, command := range commands {
						res := client.Run(command)
						if res.Code != 0 {
							return fmt.Errorf("provision command failed on %s: %s", host, res.Stderr)
						}
					}
				}
			}
			return nil
		},
	}
	addConfigFlags(cmd, &flags)
	cmd.Flags().StringVar(&username, "user", "", "override SSH user from config")
	return cmd
}

func newDeployCommand() *cobra.Command {
	var flags configFlags
	var skipBuild bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Build and deploy the application to your VPS",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadWithFlags(flags)
			if err != nil {
				return err
			}
			if cfg.Deploy.Strategy == "compose" {
				return deployCompose(cfg, skipBuild, dryRun)
			}
			if cfg.Deploy.Strategy == "blue-green" {
				return deployBlueGreen(cfg, skipBuild, dryRun)
			}
			image := fmt.Sprintf("shuttle/%s:latest", cfg.App)
			if !skipBuild {
				if dryRun {
					fmt.Printf("[dry-run] docker build -t %s -f %s %s\n", image, cfg.Build.Dockerfile, cfg.Build.Context)
				} else {
					build := exec.Command("docker", "build", "-t", image, "-f", cfg.Build.Dockerfile, cfg.Build.Context)
					build.Stdout = os.Stdout
					build.Stderr = os.Stderr
					if err := build.Run(); err != nil {
						return err
					}
				}
			}
			for _, group := range cfg.Servers {
				for _, host := range group.Hosts {
					client, err := connectSSH(group, host)
					if err != nil {
						return err
					}
					for name, service := range cfg.Services {
						port := service.Port
						container := fmt.Sprintf("%s_%s_0", cfg.App, name)
						command := fmt.Sprintf("docker rm -f %s 2>/dev/null; docker run -d --restart always --name %s", shell.Escape(container), shell.Escape(container))
						if port != 0 {
							command += fmt.Sprintf(" -p 127.0.0.1:%d:%d", port, port)
						}
						command += " " + shell.Escape(image)
						if service.Command != "" {
							command += " sh -lc " + shell.Escape(service.Command)
						}
						if dryRun {
							fmt.Printf("[dry-run] %s: %s\n", host, command)
							continue
						}
						res := client.Run(command)
						if res.Code != 0 {
							return fmt.Errorf("deploy failed on %s: %s", host, res.Stderr)
						}
					}
				}
			}
			return nil
		},
	}
	addConfigFlags(cmd, &flags)
	cmd.Flags().BoolVar(&skipBuild, "skip-build", false, "skip Docker build")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show planned deployment without mutating")
	return cmd
}

func newStatusCommand() *cobra.Command {
	var flags configFlags
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show running container status across all servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadWithFlags(flags)
			if err != nil {
				return err
			}
			for _, group := range cfg.Servers {
				for _, host := range group.Hosts {
					client, err := connectSSH(group, host)
					if err != nil {
						return err
					}
					if cfg.Deploy.Strategy == "compose" {
						remoteDir := runtime.AppDir(cfg.App)
						res := client.Run(fmt.Sprintf("cd %s && docker compose ps", shell.Escape(remoteDir)))
						fmt.Printf("%s\n%s\n", host, res.Stdout)
					} else {
						res := client.Run("docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'")
						fmt.Printf("%s\n%s\n", host, res.Stdout)
					}
				}
			}
			return nil
		},
	}
	addConfigFlags(cmd, &flags)
	return cmd
}

func newLogsCommand() *cobra.Command {
	var flags configFlags
	var service string
	var lines int
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Stream remote container logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadWithFlags(flags)
			if err != nil {
				return err
			}
			if service == "" {
				service = "web"
			}
			for _, group := range cfg.Servers {
				for _, host := range group.Hosts {
					client, err := connectSSH(group, host)
					if err != nil {
						return err
					}
					if cfg.Deploy.Strategy == "compose" {
						remoteDir := runtime.AppDir(cfg.App)
						res := client.Run(fmt.Sprintf("cd %s && docker compose logs --tail %d %s", shell.Escape(remoteDir), lines, shell.Escape(service)))
						fmt.Print(res.Stdout)
						if res.Stderr != "" {
							fmt.Fprint(os.Stderr, res.Stderr)
						}
					} else {
						res := client.Run(fmt.Sprintf("docker logs --tail %d %s", lines, shell.Escape(fmt.Sprintf("%s_%s_0", cfg.App, service))))
						fmt.Print(res.Stdout)
						if res.Code != 0 {
							return fmt.Errorf(res.Stderr)
						}
					}
					return nil
				}
			}
			return nil
		},
	}
	addConfigFlags(cmd, &flags)
	cmd.Flags().StringVar(&service, "service", "web", "service name")
	cmd.Flags().IntVar(&lines, "lines", 100, "number of lines")
	return cmd
}

func newExecCommand() *cobra.Command {
	var flags configFlags
	var service string
	cmd := &cobra.Command{
		Use:   "exec -- <command>",
		Short: "Execute a command inside a remote container",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadWithFlags(flags)
			if err != nil {
				return err
			}
			if service == "" {
				service = "web"
			}
			remote := strings.Join(args, " ")
			if remote == "" {
				return fmt.Errorf("no command provided")
			}
			for _, group := range cfg.Servers {
				for _, host := range group.Hosts {
					client, err := connectSSH(group, host)
					if err != nil {
						return err
					}
					if cfg.Deploy.Strategy == "compose" {
						remoteDir := runtime.AppDir(cfg.App)
						res := client.Run(fmt.Sprintf("cd %s && docker compose exec %s %s", shell.Escape(remoteDir), shell.Escape(service), remote))
						fmt.Print(res.Stdout)
						if res.Code != 0 {
							return fmt.Errorf(res.Stderr)
						}
					} else {
						container := fmt.Sprintf("%s_%s_0", cfg.App, service)
						res := client.Run(fmt.Sprintf("docker exec %s sh -lc %s", shell.Escape(container), shell.Escape(remote)))
						fmt.Print(res.Stdout)
						if res.Code != 0 {
							return fmt.Errorf(res.Stderr)
						}
					}
					return nil
				}
			}
			return nil
		},
	}
	addConfigFlags(cmd, &flags)
	cmd.Flags().StringVar(&service, "service", "web", "service name")
	return cmd
}

func newRollbackCommand() *cobra.Command {
	return simpleConfigCommand("rollback", "Rollback the application to a previous version", func(cfg *config.Config) error {
		return fmt.Errorf("rollback needs persisted deployment state parity; Go port command surface is present but rollback execution is not yet safe")
	})
}

func newDestroyCommand() *cobra.Command {
	var flags configFlags
	var yes bool
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Remove an app deployment from remote hosts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadWithFlags(flags)
			if err != nil {
				return err
			}
			if !yes {
				return fmt.Errorf("pass --yes to confirm destroy")
			}
			for _, group := range cfg.Servers {
				for _, host := range group.Hosts {
					client, err := connectSSH(group, host)
					if err != nil {
						return err
					}
					res := client.Run(fmt.Sprintf("rm -rf %s", shell.Escape(runtime.AppDir(cfg.App))))
					if res.Code != 0 {
						return fmt.Errorf(res.Stderr)
					}
				}
			}
			return nil
		},
	}
	addConfigFlags(cmd, &flags)
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

func newLockCommand() *cobra.Command {
	var flags configFlags
	root := &cobra.Command{Use: "lock", Short: "Manage deployment locks"}
	addConfigFlags(root, &flags)
	root.AddCommand(&cobra.Command{Use: "status", RunE: func(cmd *cobra.Command, args []string) error { return lockRun(flags, "status") }})
	root.AddCommand(&cobra.Command{Use: "release", RunE: func(cmd *cobra.Command, args []string) error { return lockRun(flags, "release") }})
	return root
}

func lockRun(flags configFlags, action string) error {
	cfg, err := loadWithFlags(flags)
	if err != nil {
		return err
	}
	for _, group := range cfg.Servers {
		for _, host := range group.Hosts {
			client, err := connectSSH(group, host)
			if err != nil {
				return err
			}
			if action == "release" {
				res := client.Run(fmt.Sprintf("rm -rf %s", shell.Escape(runtime.LockDir(cfg.App))))
				if res.Code != 0 {
					return fmt.Errorf(res.Stderr)
				}
			} else {
				res := client.Run(fmt.Sprintf("test -d %s && echo locked || echo unlocked", shell.Escape(runtime.LockDir(cfg.App))))
				fmt.Printf("%s: %s", host, res.Stdout)
			}
		}
	}
	return nil
}

func newDevCommand() *cobra.Command {
	root := &cobra.Command{Use: "dev", Short: "Local development environment"}
	root.AddCommand(&cobra.Command{Use: "up", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("", "")
		if err != nil {
			return err
		}
		if err := os.WriteFile("docker-compose.dev.yml", []byte(templates.ComposeDev(cfg)), 0o644); err != nil {
			return err
		}
		c := exec.Command("docker", "compose", "-f", "docker-compose.dev.yml", "up", "-d", "--build")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	}})
	root.AddCommand(&cobra.Command{Use: "down", RunE: func(cmd *cobra.Command, args []string) error {
		c := exec.Command("docker", "compose", "-f", "docker-compose.dev.yml", "down")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	}})
	return root
}

func newMonitorCommand() *cobra.Command {
	return simpleConfigCommand("monitor", "Live Docker resource usage across all servers", func(cfg *config.Config) error {
		for _, group := range cfg.Servers {
			for _, host := range group.Hosts {
				client, err := connectSSH(group, host)
				if err != nil {
					return err
				}
				res := client.Run("docker stats --no-stream")
				fmt.Printf("%s\n%s\n", host, res.Stdout)
			}
		}
		return nil
	})
}

func simpleConfigCommand(use string, short string, run func(*config.Config) error) *cobra.Command {
	var flags configFlags
	cmd := &cobra.Command{Use: use, Short: short, RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadWithFlags(flags)
		if err != nil {
			return err
		}
		return run(cfg)
	}}
	addConfigFlags(cmd, &flags)
	return cmd
}

func parsePort(value string) int {
	port, _ := strconv.Atoi(value)
	return port
}
