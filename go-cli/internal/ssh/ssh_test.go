package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type fakeAddr string

func (f fakeAddr) Network() string { return "tcp" }
func (f fakeAddr) String() string  { return string(f) }

func TestResolveSSHConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SSH_AUTH_SOCK", "")
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	config := `Host prod
  HostName 203.0.113.10
  User deploy
  Port 7022
  IdentityFile ~/.ssh/id_shuttle
`
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := resolveSSHConfig("prod")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HostName != "203.0.113.10" || cfg.User != "deploy" || cfg.Port != 7022 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if len(cfg.IdentityFiles) != 1 || cfg.IdentityFiles[0] != filepath.Join(sshDir, "id_shuttle") {
		t.Fatalf("unexpected identity files: %#v", cfg.IdentityFiles)
	}
}

func TestAuthMethodsUsesDeployShuttleKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SSH_AUTH_SOCK", "")
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "id_shuttle"), testPrivateKey(t), 0o600); err != nil {
		t.Fatal(err)
	}

	methods, err := authMethods(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected one auth method, got %d", len(methods))
	}
}

func TestHostKeyCallbackUsesKnownHosts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	line := knownhosts.Line([]string{"example.com"}, signer.PublicKey())
	if err := os.WriteFile(filepath.Join(sshDir, "known_hosts"), []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	callback, err := hostKeyCallback()
	if err != nil {
		t.Fatal(err)
	}
	if err := callback("example.com:22", fakeAddr("203.0.113.10:22"), signer.PublicKey()); err != nil {
		t.Fatal(err)
	}

	otherPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := gossh.NewPublicKey(otherPub)
	if err != nil {
		t.Fatal(err)
	}
	if err := callback("example.com:22", fakeAddr("203.0.113.10:22"), otherKey); err == nil {
		t.Fatal("expected changed host key to be rejected")
	}
}

func testPrivateKey(t *testing.T) []byte {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

var _ net.Addr = fakeAddr("")
