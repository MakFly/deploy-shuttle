package license

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// State is what we persist locally between CLI invocations.
type State struct {
	// Token is the raw signed JWT-compatible string returned by the server.
	Token string `json:"token"`
	// ServerURL is the license server that issued the token.
	ServerURL string `json:"serverUrl"`
	// Tier is mirrored from the verified token for fast `license status`.
	Tier string `json:"tier"`
	// ExpiresAt is the token expiry, mirrored for human-readable status.
	ExpiresAt time.Time `json:"expiresAt"`
	// RefreshAt is the moment we should attempt an online refresh.
	RefreshAt time.Time `json:"refreshAt"`
	// ActivatedAt is when the user first ran `license activate`.
	ActivatedAt time.Time `json:"activatedAt"`
}

// ErrNoLicense is returned when no license state is on disk.
var ErrNoLicense = errors.New("no license activated")

// DefaultPath returns the per-user license file path.
func DefaultPath() string {
	dir := userConfigDir()
	return filepath.Join(dir, "license.json")
}

func userConfigDir() string {
	if dir := os.Getenv("SHUTTLE_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".shuttle"
	}
	return filepath.Join(home, ".shuttle")
}

// Load reads license state from disk. Returns ErrNoLicense when absent.
func Load(path string) (State, error) {
	if path == "" {
		path = DefaultPath()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, ErrNoLicense
		}
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

// Save writes license state to disk with restrictive permissions (0600).
func Save(path string, state State) error {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// Clear removes any persisted license state. Returns nil when absent.
func Clear(path string) error {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
