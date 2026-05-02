package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"
)

func mintTestToken(t *testing.T, payload Token) (string, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	tok, err := SignToken(priv, payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return tok, pub
}

func TestVerifyTokenHappyPath(t *testing.T) {
	now := time.Unix(1_000_000_000, 0).UTC()
	tok, pub := mintTestToken(t, Token{Key: "K", Tier: "pro", FP: "abc", IAT: now.Unix(), EXP: now.Add(14 * 24 * time.Hour).Unix()})
	out, err := VerifyToken(tok, pub, now, "abc")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if out.Tier != "pro" || out.Key != "K" {
		t.Fatalf("unexpected token: %+v", out)
	}
}

func TestVerifyTokenExpired(t *testing.T) {
	now := time.Unix(1_000_000_000, 0).UTC()
	tok, pub := mintTestToken(t, Token{Tier: "pro", FP: "abc", IAT: now.Unix(), EXP: now.Add(-time.Hour).Unix()})
	_, err := VerifyToken(tok, pub, now, "abc")
	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestVerifyTokenFingerprintMismatch(t *testing.T) {
	now := time.Unix(1_000_000_000, 0).UTC()
	tok, pub := mintTestToken(t, Token{Tier: "pro", FP: "abc", IAT: now.Unix(), EXP: now.Add(time.Hour).Unix()})
	_, err := VerifyToken(tok, pub, now, "xyz")
	if !errors.Is(err, ErrFingerprintMismatch) {
		t.Fatalf("expected ErrFingerprintMismatch, got %v", err)
	}
}

func TestVerifyTokenWrongKey(t *testing.T) {
	now := time.Unix(1_000_000_000, 0).UTC()
	tok, _ := mintTestToken(t, Token{Tier: "pro", FP: "abc", IAT: now.Unix(), EXP: now.Add(time.Hour).Unix()})
	_, otherPub := mintTestToken(t, Token{})
	_, err := VerifyToken(tok, otherPub, now, "abc")
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestVerifyTokenMalformed(t *testing.T) {
	_, pub := mintTestToken(t, Token{})
	for _, s := range []string{"", "abc", "abc.def", "....", "not.a.token"} {
		if _, err := VerifyToken(s, pub, time.Now(), ""); err == nil {
			t.Fatalf("expected malformed error for %q", s)
		}
	}
}
