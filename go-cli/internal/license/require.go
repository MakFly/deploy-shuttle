package license

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/version"
)

// ErrFeatureLocked is returned when the host has no valid Pro license for a
// given feature. The CLI translates it into a single, consistent UX message.
type ErrFeatureLocked struct {
	Feature string
	Reason  string
}

func (e ErrFeatureLocked) Error() string {
	return fmt.Sprintf("%s requires a DeployShuttle Pro license: %s", e.Feature, e.Reason)
}

// IsFeatureLocked reports whether err indicates a missing/expired license.
func IsFeatureLocked(err error) bool {
	var locked ErrFeatureLocked
	return errors.As(err, &locked)
}

// CheckoutURL is shown in error messages so users can subscribe immediately.
const CheckoutURL = "https://deployshuttle.io/pricing"

// Require returns nil when the host is licensed for `feature`, ErrFeatureLocked otherwise.
//
// It performs a fully offline verification using the embedded Ed25519 public key
// (version.LicensePubKeyB64). When the public key is empty (dev builds), Require
// becomes a no-op so local development is not blocked.
func Require(feature string) error {
	if version.LicensePubKeyB64 == "" {
		return nil
	}
	if devOverride() {
		return nil
	}
	pub, err := decodePubKey(version.LicensePubKeyB64)
	if err != nil {
		// A broken build should not deny service silently; surface a hint.
		return ErrFeatureLocked{Feature: feature, Reason: fmt.Sprintf("embedded public key is invalid (%s); please reinstall", err)}
	}
	state, err := Load("")
	if err != nil {
		if errors.Is(err, ErrNoLicense) {
			return ErrFeatureLocked{Feature: feature, Reason: fmt.Sprintf("no license activated. Run `deploy-shuttle license activate <key>` or buy at %s", CheckoutURL)}
		}
		return ErrFeatureLocked{Feature: feature, Reason: fmt.Sprintf("could not read license state: %s", err)}
	}
	token, err := VerifyToken(state.Token, pub, time.Now().UTC(), MachineFingerprint())
	if err != nil {
		switch {
		case errors.Is(err, ErrTokenExpired):
			return ErrFeatureLocked{Feature: feature, Reason: "license token expired offline. Run `deploy-shuttle license refresh` while online"}
		case errors.Is(err, ErrFingerprintMismatch):
			return ErrFeatureLocked{Feature: feature, Reason: "license token was activated for a different machine. Run `deploy-shuttle license activate <key>` here"}
		case errors.Is(err, ErrSignatureInvalid):
			return ErrFeatureLocked{Feature: feature, Reason: "license token signature is invalid. Re-activate to fetch a fresh token"}
		default:
			return ErrFeatureLocked{Feature: feature, Reason: fmt.Sprintf("license verification failed: %s", err)}
		}
	}
	if token.Tier != "pro" {
		return ErrFeatureLocked{Feature: feature, Reason: fmt.Sprintf("requires tier 'pro', current tier is %q", token.Tier)}
	}
	return nil
}

// RequireOrAttemptRefresh is like Require but tries an online refresh once
// before failing, if the token is close to expiry. Used by long-running
// commands that can afford a network round-trip.
func RequireOrAttemptRefresh(ctx context.Context, feature string) error {
	if err := Require(feature); err != nil {
		return err
	}
	state, err := Load("")
	if err != nil || state.Token == "" {
		return nil
	}
	if time.Now().UTC().Before(state.RefreshAt) {
		return nil
	}
	if os.Getenv("DEPLOY_SHUTTLE_OFFLINE") != "" {
		return nil
	}
	client := NewClient(state.ServerURL)
	resp, refreshErr := client.Refresh(ctx, state.Token)
	if refreshErr != nil {
		return nil // tolerate network failures while the token is still valid
	}
	state.Token = resp.Token
	state.Tier = resp.Tier
	state.ExpiresAt = resp.ExpiresAt
	state.RefreshAt = time.Now().UTC().Add(refreshLead(resp.ExpiresAt))
	_ = Save("", state)
	return nil
}

func refreshLead(exp time.Time) time.Duration {
	d := time.Until(exp)
	if d < 48*time.Hour {
		return 0
	}
	return d - 48*time.Hour
}

func decodePubKey(b64 string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		// also accept URL-encoded
		raw, err = base64.RawURLEncoding.DecodeString(b64)
		if err != nil {
			return nil, err
		}
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("expected %d bytes, got %d", ed25519.PublicKeySize, len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

// devOverride lets developers skip the gate when DEPLOY_SHUTTLE_DEV=1.
// The override is only effective when the binary was built without an
// embedded public key (i.e. local `go build`); it has no effect on official
// release builds because Require returns early when version.LicensePubKeyB64
// is empty.
func devOverride() bool {
	return os.Getenv("DEPLOY_SHUTTLE_DEV") != ""
}
