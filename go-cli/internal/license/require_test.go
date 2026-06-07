package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/version"
)

func TestRequireNoOpInDevBuild(t *testing.T) {
	old := version.LicensePubKeyB64
	version.LicensePubKeyB64 = ""
	t.Cleanup(func() { version.LicensePubKeyB64 = old })
	if err := Require("test"); err != nil {
		t.Fatalf("expected no-op in dev, got %v", err)
	}
}

func TestRequireFailsWhenNoLicense(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	old := version.LicensePubKeyB64
	version.LicensePubKeyB64 = base64.StdEncoding.EncodeToString(pub)
	t.Cleanup(func() { version.LicensePubKeyB64 = old })

	dir := t.TempDir()
	t.Setenv("SHUTTLE_HOME", dir)
	t.Setenv("SHUTTLE_DEV", "")

	err = Require("doctor --target")
	if !IsFeatureLocked(err) {
		t.Fatalf("expected ErrFeatureLocked, got %v", err)
	}
}

func TestRequireSucceedsWithValidToken(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	old := version.LicensePubKeyB64
	version.LicensePubKeyB64 = base64.StdEncoding.EncodeToString(pub)
	t.Cleanup(func() { version.LicensePubKeyB64 = old })

	dir := t.TempDir()
	t.Setenv("SHUTTLE_HOME", dir)
	t.Setenv("SHUTTLE_DEV", "")

	fp := MachineFingerprint()
	tok, err := SignToken(priv, Token{Key: "K", Tier: "pro", FP: fp, IAT: time.Now().Add(-time.Minute).Unix(), EXP: time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := Save("", State{Token: tok, Tier: "pro", ExpiresAt: time.Now().Add(time.Hour).UTC()}); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := Require("doctor --target"); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}

	// Sanity: the saved file lives under the temp SHUTTLE_HOME.
	if _, err := Load(filepath.Join(dir, "license.json")); err != nil {
		t.Fatalf("load explicit path: %v", err)
	}
}

func TestRequireRejectsExpiredToken(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	old := version.LicensePubKeyB64
	version.LicensePubKeyB64 = base64.StdEncoding.EncodeToString(pub)
	t.Cleanup(func() { version.LicensePubKeyB64 = old })

	dir := t.TempDir()
	t.Setenv("SHUTTLE_HOME", dir)
	t.Setenv("SHUTTLE_DEV", "")

	fp := MachineFingerprint()
	tok, _ := SignToken(priv, Token{Tier: "pro", FP: fp, IAT: time.Now().Add(-48 * time.Hour).Unix(), EXP: time.Now().Add(-time.Hour).Unix()})
	_ = Save("", State{Token: tok})

	err = Require("doctor --target")
	if !IsFeatureLocked(err) {
		t.Fatalf("expected feature locked, got %v", err)
	}
	var locked ErrFeatureLocked
	if !errors.As(err, &locked) {
		t.Fatalf("expected ErrFeatureLocked")
	}
}

func TestRequireDevOverride(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	old := version.LicensePubKeyB64
	version.LicensePubKeyB64 = base64.StdEncoding.EncodeToString(pub)
	t.Cleanup(func() { version.LicensePubKeyB64 = old })

	t.Setenv("SHUTTLE_DEV", "1")
	if err := Require("test"); err != nil {
		t.Fatalf("expected dev override, got %v", err)
	}
}
