package ssh

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Client struct {
	Host string
	User string
	Port int
	cfg  *gossh.ClientConfig
}

type Result struct {
	Stdout string
	Stderr string
	Code   int
}

func NewClient(host string, username string, port int) (*Client, error) {
	resolved, err := resolveSSHConfig(host)
	if err != nil {
		return nil, err
	}
	if resolved.HostName != "" {
		host = resolved.HostName
	}
	if username == "" && resolved.User != "" {
		username = resolved.User
	}
	if port == 0 || port == 22 && resolved.Port != 0 {
		port = resolved.Port
	}
	if username == "" {
		current, err := user.Current()
		if err == nil {
			username = current.Username
		}
	}
	if port == 0 {
		port = 22
	}

	auth, err := authMethods(resolved.IdentityFiles)
	if err != nil {
		return nil, err
	}
	hostKeyCallback, err := hostKeyCallback()
	if err != nil {
		return nil, err
	}

	return &Client{
		Host: host,
		User: username,
		Port: port,
		cfg: &gossh.ClientConfig{
			User:            username,
			Auth:            auth,
			HostKeyCallback: hostKeyCallback,
			Timeout:         20 * time.Second,
		},
	}, nil
}

type sshConfig struct {
	HostName      string
	User          string
	Port          int
	IdentityFiles []string
}

func resolveSSHConfig(host string) (sshConfig, error) {
	path := filepath.Join(homeDir(), ".ssh", "config")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return sshConfig{}, nil
		}
		return sshConfig{}, err
	}
	cfg := sshConfig{}
	applies := false
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, " ")
		if !ok {
			key, value, ok = strings.Cut(line, "\t")
		}
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "host":
			applies = hostMatches(host, strings.Fields(value))
		case "hostname":
			if applies && cfg.HostName == "" {
				cfg.HostName = value
			}
		case "user":
			if applies && cfg.User == "" {
				cfg.User = value
			}
		case "port":
			if applies && cfg.Port == 0 {
				parsed, err := strconv.Atoi(value)
				if err != nil || parsed < 1 || parsed > 65535 {
					return sshConfig{}, fmt.Errorf("invalid Port %q in %s", value, path)
				}
				cfg.Port = parsed
			}
		case "identityfile":
			if applies {
				cfg.IdentityFiles = append(cfg.IdentityFiles, expandPath(value))
			}
		}
	}
	return cfg, nil
}

func hostMatches(host string, patterns []string) bool {
	matched := false
	for _, pattern := range patterns {
		negated := strings.HasPrefix(pattern, "!")
		pattern = strings.TrimPrefix(pattern, "!")
		ok, err := filepath.Match(pattern, host)
		if err != nil {
			ok = pattern == host
		}
		if ok && negated {
			return false
		}
		if ok {
			matched = true
		}
	}
	return matched
}

func authMethods(identityFiles []string) ([]gossh.AuthMethod, error) {
	methods := []gossh.AuthMethod{}
	if socket := os.Getenv("SSH_AUTH_SOCK"); socket != "" {
		conn, err := net.Dial("unix", socket)
		if err == nil {
			methods = append(methods, gossh.PublicKeysCallback(agent.NewClient(conn).Signers))
		}
	}
	seen := map[string]bool{}
	candidates := append([]string{}, identityFiles...)
	candidates = append(candidates,
		filepath.Join(homeDir(), ".ssh", "id_shuttle"),
		filepath.Join(homeDir(), ".ssh", "id_ed25519"),
		filepath.Join(homeDir(), ".ssh", "id_rsa"),
	)
	for _, path := range candidates {
		path = expandPath(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		key, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		signer, err := gossh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse SSH private key %s: %w", path, err)
		}
		methods = append(methods, gossh.PublicKeys(signer))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("no SSH auth method found; load ssh-agent or add ~/.ssh/id_shuttle, ~/.ssh/id_ed25519, or ~/.ssh/id_rsa")
	}
	return methods, nil
}

func hostKeyCallback() (gossh.HostKeyCallback, error) {
	path := filepath.Join(homeDir(), ".ssh", "known_hosts")
	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}
	return callback, nil
}

func expandPath(path string) string {
	if path == "~" {
		return homeDir()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir(), strings.TrimPrefix(path, "~/"))
	}
	return os.ExpandEnv(path)
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func (c *Client) Run(command string) Result {
	conn, err := gossh.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port), c.cfg)
	if err != nil {
		return Result{Code: 255, Stderr: err.Error()}
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return Result{Code: 255, Stderr: err.Error()}
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(command)
	if err != nil {
		if exit, ok := err.(*gossh.ExitError); ok {
			return Result{Code: exit.ExitStatus(), Stdout: stdout.String(), Stderr: stderr.String()}
		}
		return Result{Code: 1, Stdout: stdout.String(), Stderr: err.Error()}
	}

	return Result{Code: 0, Stdout: stdout.String(), Stderr: stderr.String()}
}

func (c *Client) UploadContent(content string, remotePath string, mode os.FileMode) Result {
	conn, err := gossh.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port), c.cfg)
	if err != nil {
		return Result{Code: 255, Stderr: err.Error()}
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return Result{Code: 255, Stderr: err.Error()}
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return Result{Code: 255, Stderr: err.Error()}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	command := fmt.Sprintf("umask 077 && cat > %s && chmod %04o %s", shellQuote(remotePath), mode.Perm(), shellQuote(remotePath))
	if err := session.Start(command); err != nil {
		return Result{Code: 1, Stderr: err.Error()}
	}
	if _, err := io.WriteString(stdin, content); err != nil {
		_ = stdin.Close()
		return Result{Code: 1, Stdout: stdout.String(), Stderr: err.Error()}
	}
	_ = stdin.Close()
	err = session.Wait()
	if err != nil {
		if exit, ok := err.(*gossh.ExitError); ok {
			return Result{Code: exit.ExitStatus(), Stdout: stdout.String(), Stderr: stderr.String()}
		}
		return Result{Code: 1, Stdout: stdout.String(), Stderr: err.Error()}
	}
	return Result{Code: 0, Stdout: stdout.String(), Stderr: stderr.String()}
}

func shellQuote(value string) string {
	var out bytes.Buffer
	out.WriteByte('\'')
	for _, r := range value {
		if r == '\'' {
			out.WriteString("'\\''")
			continue
		}
		out.WriteRune(r)
	}
	out.WriteByte('\'')
	return out.String()
}
