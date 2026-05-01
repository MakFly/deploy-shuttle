package secrets

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

type Store struct {
	Path       string
	Passphrase string
}

type envelope struct {
	Version    int    `json:"version"`
	KDF        string `json:"kdf"`
	Salt       string `json:"salt"`
	Time       uint32 `json:"time"`
	MemoryKiB  uint32 `json:"memoryKiB"`
	Threads    uint8  `json:"threads"`
	Cipher     string `json:"cipher"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

const (
	defaultPath     = ".shuttle/secrets.enc"
	envelopeVersion = 1
	kdfName         = "argon2id"
	cipherName      = "xchacha20-poly1305"
	argonTime       = uint32(3)
	argonMemoryKiB  = uint32(64 * 1024)
	argonThreads    = uint8(4)
)

func (s Store) path() string {
	if s.Path == "" {
		return defaultPath
	}
	return s.Path
}

func (s Store) passphrase() ([]byte, error) {
	if s.Passphrase != "" {
		return []byte(s.Passphrase), nil
	}
	if value := os.Getenv("DEPLOY_SHUTTLE_SECRETS_PASSPHRASE"); value != "" {
		return []byte(value), nil
	}
	return nil, fmt.Errorf("missing secrets passphrase; set DEPLOY_SHUTTLE_SECRETS_PASSPHRASE")
}

func deriveKey(passphrase []byte, salt []byte, time uint32, memoryKiB uint32, threads uint8) []byte {
	return argon2.IDKey(passphrase, salt, time, memoryKiB, threads, chacha20poly1305.KeySize)
}

func randomBytes(size int) ([]byte, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return nil, err
	}
	return value, nil
}

func encryptValues(values map[string]string, passphrase []byte) ([]byte, error) {
	plaintext, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return nil, err
	}
	salt, err := randomBytes(32)
	if err != nil {
		return nil, err
	}
	nonce, err := randomBytes(chacha20poly1305.NonceSizeX)
	if err != nil {
		return nil, err
	}
	key := deriveKey(passphrase, salt, argonTime, argonMemoryKiB, argonThreads)
	defer zero(key)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	payload := envelope{
		Version:    envelopeVersion,
		KDF:        kdfName,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Time:       argonTime,
		MemoryKiB:  argonMemoryKiB,
		Threads:    argonThreads,
		Cipher:     cipherName,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, plaintext, nil)),
	}
	return json.MarshalIndent(payload, "", "  ")
}

func decryptValues(raw []byte, passphrase []byte) (map[string]string, error) {
	var payload envelope
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if payload.Version != envelopeVersion {
		return nil, fmt.Errorf("unsupported secrets envelope version %d", payload.Version)
	}
	if subtle.ConstantTimeCompare([]byte(payload.KDF), []byte(kdfName)) != 1 {
		return nil, fmt.Errorf("unsupported secrets kdf %q", payload.KDF)
	}
	if subtle.ConstantTimeCompare([]byte(payload.Cipher), []byte(cipherName)) != 1 {
		return nil, fmt.Errorf("unsupported secrets cipher %q", payload.Cipher)
	}
	salt, err := base64.StdEncoding.DecodeString(payload.Salt)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(payload.Nonce)
	if err != nil {
		return nil, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payload.Ciphertext))
	if err != nil {
		return nil, err
	}
	key := deriveKey(passphrase, salt, payload.Time, payload.MemoryKiB, payload.Threads)
	defer zero(key)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secrets: wrong passphrase or corrupted store")
	}
	var values map[string]string
	if err := json.Unmarshal(plaintext, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func zero(value []byte) {
	for i := range value {
		value[i] = 0
	}
}

func (s Store) load() (map[string]string, error) {
	raw, err := os.ReadFile(s.path())
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	passphrase, err := s.passphrase()
	if err != nil {
		return nil, err
	}
	return decryptValues(raw, passphrase)
}

func (s Store) save(values map[string]string) error {
	path := s.path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	passphrase, err := s.passphrase()
	if err != nil {
		return err
	}
	raw, err := encryptValues(values, passphrase)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func (s Store) Set(key string, value string) error {
	values, err := s.load()
	if err != nil {
		return err
	}
	values[key] = value
	return s.save(values)
}

func (s Store) Get(key string) (string, bool, error) {
	values, err := s.load()
	if err != nil {
		return "", false, err
	}
	value, ok := values[key]
	return value, ok, nil
}

func (s Store) List() ([]string, error) {
	values, err := s.load()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys, nil
}

func (s Store) Remove(key string) error {
	values, err := s.load()
	if err != nil {
		return err
	}
	delete(values, key)
	return s.save(values)
}

func (s Store) LoadAll() (map[string]string, error) {
	return s.load()
}
