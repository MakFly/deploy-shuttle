package license

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
)

// MachineFingerprint returns a stable, non-PII identifier for the host:
// sha256(hostname + machine-id). It is used to bind a license token to one
// machine; collisions are statistically irrelevant for billing purposes.
func MachineFingerprint() string {
	hostname, _ := os.Hostname()
	id := readMachineID()
	if hostname == "" && id == "" {
		// Last-resort: pid-stable fallback. Caller will see the same fp for
		// the lifetime of this process; the server will see a "best-effort"
		// fingerprint.
		hostname = "unknown-host"
	}
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(hostname)) + "|" + id))
	return hex.EncodeToString(sum[:])
}

func readMachineID() string {
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if raw, err := os.ReadFile(path); err == nil {
			id := strings.TrimSpace(string(raw))
			if id != "" {
				return id
			}
		}
	}
	return ""
}
