package secrets

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreEncryptsAndReadsSecrets(t *testing.T) {
	dir := t.TempDir()
	store := Store{
		Path:       filepath.Join(dir, "secrets.enc"),
		Passphrase: "correct horse battery staple",
	}

	if err := store.Set("API_KEY", "test-value"); err != nil {
		t.Fatalf("set secret: %v", err)
	}

	value, ok, err := store.Get("API_KEY")
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if !ok {
		t.Fatal("expected API_KEY to exist")
	}
	if value != "test-value" {
		t.Fatalf("expected test-value, got %q", value)
	}

	raw, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatalf("read encrypted store: %v", err)
	}
	if strings.Contains(string(raw), "test-value") {
		t.Fatal("encrypted store contains plaintext secret value")
	}
	var payload envelope
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("encrypted store should be a JSON envelope: %v", err)
	}
	if payload.KDF != kdfName {
		t.Fatalf("expected kdf %q, got %q", kdfName, payload.KDF)
	}
	if payload.Cipher != cipherName {
		t.Fatalf("expected cipher %q, got %q", cipherName, payload.Cipher)
	}
	if _, err := base64.StdEncoding.DecodeString(payload.Ciphertext); err != nil {
		t.Fatalf("ciphertext should be base64 encoded: %v", err)
	}
}

func TestStoreRejectsWrongPassphrase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.enc")
	store := Store{Path: path, Passphrase: "right-passphrase"}
	if err := store.Set("API_KEY", "test-value"); err != nil {
		t.Fatalf("set secret: %v", err)
	}

	wrong := Store{Path: path, Passphrase: "wrong-passphrase"}
	if _, _, err := wrong.Get("API_KEY"); err == nil {
		t.Fatal("expected wrong passphrase to fail")
	}
}
