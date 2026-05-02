package license

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to the license server.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient builds a client with sensible defaults. baseURL trailing slash is trimmed.
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://license.deployshuttle.io"
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// ActivateRequest is the body sent to POST /activate.
type ActivateRequest struct {
	Key                string `json:"key"`
	MachineFingerprint string `json:"machineFingerprint"`
	CLIVersion         string `json:"cliVersion,omitempty"`
}

// ActivateResponse is what the server returns on success.
type ActivateResponse struct {
	Token     string    `json:"token"`
	Tier      string    `json:"tier"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Activate exchanges a license key for a signed token.
func (c *Client) Activate(ctx context.Context, key, fingerprint, cliVersion string) (ActivateResponse, error) {
	if key == "" {
		return ActivateResponse{}, errors.New("license key is required")
	}
	body, err := json.Marshal(ActivateRequest{Key: key, MachineFingerprint: fingerprint, CLIVersion: cliVersion})
	if err != nil {
		return ActivateResponse{}, err
	}
	resp, err := c.post(ctx, "/activate", body)
	if err != nil {
		return ActivateResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ActivateResponse{}, decodeServerError(resp, "activate")
	}
	var out ActivateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ActivateResponse{}, fmt.Errorf("decode activate response: %w", err)
	}
	return out, nil
}

// RefreshRequest renews an existing token.
type RefreshRequest struct {
	Token string `json:"token"`
}

// Refresh issues a new token from a still-valid one.
func (c *Client) Refresh(ctx context.Context, token string) (ActivateResponse, error) {
	if token == "" {
		return ActivateResponse{}, errors.New("token is required")
	}
	body, err := json.Marshal(RefreshRequest{Token: token})
	if err != nil {
		return ActivateResponse{}, err
	}
	resp, err := c.post(ctx, "/refresh", body)
	if err != nil {
		return ActivateResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ActivateResponse{}, decodeServerError(resp, "refresh")
	}
	var out ActivateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ActivateResponse{}, fmt.Errorf("decode refresh response: %w", err)
	}
	return out, nil
}

func (c *Client) post(ctx context.Context, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return c.HTTPClient.Do(req)
}

func decodeServerError(resp *http.Response, op string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("license %s failed (HTTP %d): %s", op, resp.StatusCode, msg)
}
