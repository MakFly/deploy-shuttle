package license

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "license.json")
	state := State{
		Token:       "h.p.s",
		ServerURL:   "https://example",
		Tier:        "pro",
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second),
		RefreshAt:   time.Now().UTC().Add(12 * time.Hour).Truncate(time.Second),
		ActivatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := Save(path, state); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Token != state.Token || loaded.Tier != state.Tier {
		t.Fatalf("round trip mismatch: %+v vs %+v", loaded, state)
	}
}

func TestLoadMissingReturnsErrNoLicense(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(filepath.Join(dir, "absent.json"))
	if !errors.Is(err, ErrNoLicense) {
		t.Fatalf("expected ErrNoLicense, got %v", err)
	}
}

func TestClearIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "license.json")
	if err := Clear(path); err != nil {
		t.Fatalf("clear empty: %v", err)
	}
	if err := Save(path, State{Token: "x"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := Clear(path); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, err := Load(path); !errors.Is(err, ErrNoLicense) {
		t.Fatalf("expected ErrNoLicense after clear, got %v", err)
	}
}

func TestFingerprintIsStable(t *testing.T) {
	a := MachineFingerprint()
	b := MachineFingerprint()
	if a == "" || a != b {
		t.Fatalf("expected stable non-empty fingerprint, got %q vs %q", a, b)
	}
}
