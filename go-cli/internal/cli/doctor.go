package cli

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/execx"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/readiness"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/ssh"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	var target string
	var format string
	var profile string
	var failBelow string
	var configPath string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run a VPS production readiness audit",
		RunE: func(cmd *cobra.Command, args []string) error {
			threshold := -1
			if failBelow != "" {
				value, err := strconv.Atoi(failBelow)
				if err != nil || value < 0 || value > 100 {
					return errors.New("--fail-below must be a number between 0 and 100")
				}
				threshold = value
			}
			readinessConfig, resolvedConfigPath, err := readiness.LoadConfig(configPath)
			if err != nil {
				return err
			}
			adapter := execx.Adapter(execx.Local{})
			reportTarget := "local"
			if target != "" {
				sshTarget, err := parseSSHTarget(target)
				if err != nil {
					return err
				}
				client, err := ssh.NewClient(sshTarget.Host, sshTarget.User, sshTarget.Port)
				if err != nil {
					return err
				}
				adapter = execx.SSH{Client: client}
				reportTarget = sshTarget.String()
			}
			report := readiness.RunWithConfig(adapter, reportTarget, splitCSV(profile, nil), readinessConfig, resolvedConfigPath)
			switch format {
			case "", "console":
				fmt.Print(readiness.Console(report))
			case "json":
				fmt.Println(readiness.JSON(report))
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
			critical := false
			for _, check := range report.Checks {
				if check.Severity == readiness.Critical && check.Status == readiness.Failed {
					critical = true
				}
			}
			if critical || (threshold >= 0 && report.Score < threshold) {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "remote SSH target user@host")
	cmd.Flags().StringVar(&format, "format", "console", "output format: console or json")
	cmd.Flags().StringVar(&profile, "profile", "", "comma-separated profile labels")
	cmd.Flags().StringVar(&failBelow, "fail-below", "", "exit with code 1 when score is below threshold")
	cmd.Flags().StringVar(&configPath, "config", "", "path to .deployshuttle.yml readiness config")
	return cmd
}

type sshTarget struct {
	User string
	Host string
	Port int
}

func (t sshTarget) String() string {
	target := t.Host
	if strings.Contains(target, ":") && !strings.HasPrefix(target, "[") {
		target = "[" + target + "]"
	}
	if t.User != "" {
		target = t.User + "@" + target
	}
	if t.Port != 22 {
		target = fmt.Sprintf("%s:%d", target, t.Port)
	}
	return target
}

func parseSSHTarget(value string) (sshTarget, error) {
	if strings.TrimSpace(value) == "" {
		return sshTarget{}, errors.New("--target cannot be empty")
	}
	user := ""
	hostPort := value
	if strings.Count(value, "@") > 1 {
		return sshTarget{}, fmt.Errorf("invalid SSH target %q", value)
	}
	if before, after, ok := strings.Cut(value, "@"); ok {
		if before == "" || after == "" {
			return sshTarget{}, fmt.Errorf("invalid SSH target %q; expected user@host or user@host:port", value)
		}
		user = before
		hostPort = after
	}
	host := hostPort
	port := 22
	if parsedHost, parsedPort, err := net.SplitHostPort(hostPort); err == nil {
		host = parsedHost
		parsed, err := strconv.Atoi(parsedPort)
		if err != nil || parsed < 1 || parsed > 65535 {
			return sshTarget{}, fmt.Errorf("invalid SSH port %q", parsedPort)
		}
		port = parsed
	} else if strings.Contains(hostPort, ":") && strings.Count(hostPort, ":") == 1 {
		parts := strings.Split(hostPort, ":")
		if parts[0] == "" || parts[1] == "" {
			return sshTarget{}, fmt.Errorf("invalid SSH target %q; expected host:port", value)
		}
		parsed, err := strconv.Atoi(parts[1])
		if err != nil || parsed < 1 || parsed > 65535 {
			return sshTarget{}, fmt.Errorf("invalid SSH port %q", parts[1])
		}
		host = parts[0]
		port = parsed
	} else if strings.Contains(hostPort, ":") && !strings.HasPrefix(hostPort, "[") {
		host = hostPort
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return sshTarget{}, fmt.Errorf("invalid SSH target %q; host is required", value)
	}
	return sshTarget{User: user, Host: host, Port: port}, nil
}

func splitCSV(value string, fallback []string) []string {
	if value == "" {
		return fallback
	}
	parts := []string{}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
