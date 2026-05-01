package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSSHTarget(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  sshTarget
	}{
		{
			name:  "user and host",
			input: "root@example.com",
			want:  sshTarget{User: "root", Host: "example.com", Port: 22},
		},
		{
			name:  "user host and port",
			input: "deploy@example.com:2222",
			want:  sshTarget{User: "deploy", Host: "example.com", Port: 2222},
		},
		{
			name:  "host only",
			input: "203.0.113.10",
			want:  sshTarget{Host: "203.0.113.10", Port: 22},
		},
		{
			name:  "bracketed ipv6 with port",
			input: "deploy@[2001:db8::1]:2222",
			want:  sshTarget{User: "deploy", Host: "2001:db8::1", Port: 2222},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSSHTarget(tt.input)
			if err != nil {
				t.Fatalf("parse target: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %#v, got %#v", tt.want, got)
			}
		})
	}
}

func TestParseSSHTargetRejectsInvalidPort(t *testing.T) {
	if _, err := parseSSHTarget("deploy@example.com:not-a-port"); err == nil {
		t.Fatal("expected invalid port to fail")
	}
}

func TestWriteDoctorOutputCreatesParentDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".deployshuttle", "latest-report.json")
	if err := writeDoctorOutput(path, `{"score":100}`); err != nil {
		t.Fatalf("write output: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(raw) != "{\"score\":100}\n" {
		t.Fatalf("unexpected output %q", string(raw))
	}
}
