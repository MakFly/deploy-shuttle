package ssh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"time"

	gossh "golang.org/x/crypto/ssh"
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
	if port == 0 {
		port = 22
	}
	if username == "" {
		current, err := user.Current()
		if err == nil {
			username = current.Username
		}
	}

	keyPath := filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519")
	key, err := os.ReadFile(keyPath)
	if err != nil {
		keyPath = filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
		key, err = os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("no SSH private key found at ~/.ssh/id_ed25519 or ~/.ssh/id_rsa")
		}
	}
	signer, err := gossh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	return &Client{
		Host: host,
		User: username,
		Port: port,
		cfg: &gossh.ClientConfig{
			User:            username,
			Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
			HostKeyCallback: gossh.InsecureIgnoreHostKey(),
			Timeout:         20 * time.Second,
		},
	}, nil
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
