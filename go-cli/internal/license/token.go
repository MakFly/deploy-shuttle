// Package license implements offline-grace license verification.
//
// Token format (JWT-compatible, EdDSA / Ed25519):
//
//	<base64url(header)>.<base64url(payload)>.<base64url(signature)>
//
// header = {"alg":"EdDSA","typ":"JWT"}
//
//	payload = {
//	  "key": "<license key>",        // sub-equivalent
//	  "tier": "pro",
//	  "fp": "<machine fingerprint>",
//	  "iat": <unix>,
//	  "exp": <unix>
//	}
//
// signature = ed25519.Sign(privKey, header || "." || payload)
//
// Verification only requires the embedded public key and the host clock; no
// network access. The server is the only entity able to mint a valid token.
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Token is the verified, deserialized form of a license JWT.
type Token struct {
	Key  string `json:"key"`
	Tier string `json:"tier"`
	FP   string `json:"fp"`
	IAT  int64  `json:"iat"`
	EXP  int64  `json:"exp"`
}

// ExpiresAt returns the token expiry time.
func (t Token) ExpiresAt() time.Time { return time.Unix(t.EXP, 0).UTC() }

// IssuedAt returns the token issue time.
func (t Token) IssuedAt() time.Time { return time.Unix(t.IAT, 0).UTC() }

var (
	// ErrTokenMalformed signals an unparseable token.
	ErrTokenMalformed = errors.New("license token is malformed")
	// ErrSignatureInvalid signals a forged or corrupted token.
	ErrSignatureInvalid = errors.New("license token signature is invalid")
	// ErrTokenExpired signals an expired token (offline grace exceeded).
	ErrTokenExpired = errors.New("license token has expired")
	// ErrFingerprintMismatch signals a token issued for a different machine.
	ErrFingerprintMismatch = errors.New("license token machine fingerprint mismatch")
)

// VerifyToken parses and verifies a token using the supplied Ed25519 public key.
// `now` is taken explicitly to keep the function deterministic in tests.
// `expectFP`, when non-empty, must match the token's fp claim.
func VerifyToken(raw string, pubKey ed25519.PublicKey, now time.Time, expectFP string) (Token, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return Token{}, ErrTokenMalformed
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Token{}, fmt.Errorf("%w: header: %s", ErrTokenMalformed, err)
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return Token{}, fmt.Errorf("%w: header json: %s", ErrTokenMalformed, err)
	}
	if header.Alg != "EdDSA" {
		return Token{}, fmt.Errorf("%w: unsupported alg %q", ErrTokenMalformed, header.Alg)
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Token{}, fmt.Errorf("%w: payload: %s", ErrTokenMalformed, err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Token{}, fmt.Errorf("%w: signature: %s", ErrTokenMalformed, err)
	}
	signed := []byte(parts[0] + "." + parts[1])
	if !ed25519.Verify(pubKey, signed, sig) {
		return Token{}, ErrSignatureInvalid
	}

	var token Token
	if err := json.Unmarshal(payloadBytes, &token); err != nil {
		return Token{}, fmt.Errorf("%w: payload json: %s", ErrTokenMalformed, err)
	}
	if now.Unix() >= token.EXP {
		return Token{}, ErrTokenExpired
	}
	if expectFP != "" && token.FP != expectFP {
		return Token{}, ErrFingerprintMismatch
	}
	return token, nil
}

// SignToken is exposed only for tests and for the (out-of-tree) license server.
// It is not used by the CLI at runtime.
func SignToken(privKey ed25519.PrivateKey, payload Token) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	h := base64.RawURLEncoding.EncodeToString(header)
	p := base64.RawURLEncoding.EncodeToString(body)
	sig := ed25519.Sign(privKey, []byte(h+"."+p))
	return h + "." + p + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
