package readiness

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/execx"
)

// CloudflareClient is a minimal Cloudflare REST client. The default
// implementation talks to api.cloudflare.com but the BaseURL is overridable
// to make the package testable with httptest.
type CloudflareClient struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// CloudflareError reports a non-2xx response with the upstream message when
// available so check evidence is actionable instead of "request failed".
type CloudflareError struct {
	Status int
	Body   string
}

func (e *CloudflareError) Error() string {
	return fmt.Sprintf("cloudflare api: status %d: %s", e.Status, e.Body)
}

// IsAuthError reports whether the API rejected the token (401 / 403).
func (e *CloudflareError) IsAuthError() bool {
	return e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden
}

// IsNotFound reports whether the endpoint returned 404 (e.g. a setting that
// the current Cloudflare plan tier does not expose).
func (e *CloudflareError) IsNotFound() bool {
	return e.Status == http.StatusNotFound
}

func newCloudflareClient(token string) *CloudflareClient {
	return &CloudflareClient{
		BaseURL: "https://api.cloudflare.com/client/v4",
		Token:   token,
		HTTP:    &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *CloudflareClient) get(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &CloudflareError{Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

type cfZoneListResponse struct {
	Result []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"result"`
}

type cfSettingResponse struct {
	Result struct {
		ID    string `json:"id"`
		Value any    `json:"value"`
	} `json:"result"`
}

type cfDNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

type cfDNSListResponse struct {
	Result []cfDNSRecord `json:"result"`
}

func (c *CloudflareClient) zoneID(zone string) (string, error) {
	var resp cfZoneListResponse
	if err := c.get("/zones?name="+zone, &resp); err != nil {
		return "", err
	}
	if len(resp.Result) == 0 {
		return "", fmt.Errorf("no zone matches %q", zone)
	}
	return resp.Result[0].ID, nil
}

func (c *CloudflareClient) setting(zoneID, name string) (string, error) {
	var resp cfSettingResponse
	if err := c.get("/zones/"+zoneID+"/settings/"+name, &resp); err != nil {
		return "", err
	}
	switch v := resp.Result.Value.(type) {
	case string:
		return v, nil
	case bool:
		if v {
			return "on", nil
		}
		return "off", nil
	}
	if resp.Result.Value == nil {
		return "", nil
	}
	return fmt.Sprint(resp.Result.Value), nil
}

func (c *CloudflareClient) dnsRecords(zoneID, name string) ([]cfDNSRecord, error) {
	var resp cfDNSListResponse
	path := "/zones/" + zoneID + "/dns_records"
	if name != "" {
		path += "?name=" + name
	}
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.Result, nil
}

// ResolveCloudflareToken returns the token to use, or an empty string when
// Cloudflare gating cannot run. It honors the user-provided env var name with
// a sane default of CLOUDFLARE_API_TOKEN.
func ResolveCloudflareToken(cfg CloudflareConfig) string {
	envName := cfg.TokenEnv
	if envName == "" {
		envName = "CLOUDFLARE_API_TOKEN"
	}
	return strings.TrimSpace(os.Getenv(envName))
}

// cloudflareChecks returns the list of Cloudflare-aware checks. They are
// instantiated with a single shared client so we don't re-resolve the zone
// ID per check. When the config does not enable Cloudflare or the token is
// missing, every check skips cleanly with an explanatory summary.
func cloudflareChecks(cfg CloudflareConfig, domain string, client *CloudflareClient) []Check {
	if !cfg.Enabled {
		return cloudflareSkipped("Cloudflare checks disabled (cloudflare.enabled is false in .shuttle.yml).")
	}
	if cfg.Zone == "" {
		return cloudflareSkipped("Cloudflare checks skipped: cloudflare.zone is not set in .shuttle.yml.")
	}
	if client == nil {
		return cloudflareSkipped("Cloudflare API token not found (set CLOUDFLARE_API_TOKEN or cloudflare.tokenEnv).")
	}
	zoneID, err := client.zoneID(cfg.Zone)
	if err != nil {
		summary := "Cloudflare zone lookup failed: " + err.Error() + "."
		if cfErr, ok := err.(*CloudflareError); ok && cfErr.IsAuthError() {
			summary = "Cloudflare API token rejected (401/403). Check token scope (Zone:Read, DNS:Read, Zone Settings:Read)."
		}
		return cloudflareSkipped(summary)
	}
	return []Check{
		checkCloudflareSSLFlexible(client, zoneID, cfg.Zone),
		checkCloudflareAlwaysHTTPS(client, zoneID, cfg.Zone),
		checkCloudflareWAF(client, zoneID, cfg.Zone),
		checkCloudflareDNSPresent(client, zoneID, fallback(domain, cfg.Zone)),
		checkCloudflareProxyEnabled(client, zoneID, fallback(domain, cfg.Zone)),
	}
}

func cloudflareSkipped(summary string) []Check {
	ids := []string{
		"cloudflare.ssl_flexible",
		"cloudflare.always_https_disabled",
		"cloudflare.waf_disabled",
		"cloudflare.dns_missing",
		"cloudflare.proxy_disabled",
	}
	out := make([]Check, 0, len(ids))
	for _, id := range ids {
		id := id
		severity := Medium
		if id == "cloudflare.ssl_flexible" {
			severity = Critical
		}
		out = append(out, func(_ execx.Adapter) CheckResult {
			return CheckResult{
				ID: id, Title: cloudflareTitle(id),
				Category: "cloudflare", Severity: severity, Status: Skipped,
				Summary: summary,
			}
		})
	}
	return out
}

func cloudflareTitle(id string) string {
	switch id {
	case "cloudflare.ssl_flexible":
		return "Cloudflare SSL/TLS mode is not Flexible"
	case "cloudflare.always_https_disabled":
		return "Cloudflare Always Use HTTPS is enabled"
	case "cloudflare.waf_disabled":
		return "Cloudflare WAF is enabled"
	case "cloudflare.dns_missing":
		return "Cloudflare has DNS records for the domain"
	case "cloudflare.proxy_disabled":
		return "Cloudflare proxy (orange cloud) is enabled"
	}
	return id
}

func checkCloudflareSSLFlexible(client *CloudflareClient, zoneID, zone string) Check {
	return func(_ execx.Adapter) CheckResult {
		mode, err := client.setting(zoneID, "ssl")
		if err != nil {
			return cloudflareCheckErr("cloudflare.ssl_flexible", Critical, err)
		}
		status := Passed
		severity := Critical
		summary := "Cloudflare SSL/TLS mode for " + zone + " is " + mode + "."
		remediation := ""
		switch strings.ToLower(mode) {
		case "off", "flexible":
			status = Failed
			summary = "Cloudflare SSL/TLS mode for " + zone + " is " + mode + " (downgrades HTTPS to HTTP toward the origin)."
			remediation = "Switch SSL/TLS mode to 'Full (strict)' in Cloudflare so origin requests use HTTPS with certificate validation."
		}
		return CheckResult{
			ID: "cloudflare.ssl_flexible", Title: cloudflareTitle("cloudflare.ssl_flexible"),
			Category: "cloudflare", Severity: severity, Status: status,
			Summary:     summary,
			Remediation: remediation,
			Evidence:    map[string]any{"zone": zone, "sslMode": mode},
		}
	}
}

func checkCloudflareAlwaysHTTPS(client *CloudflareClient, zoneID, zone string) Check {
	return func(_ execx.Adapter) CheckResult {
		value, err := client.setting(zoneID, "always_use_https")
		if err != nil {
			return cloudflareCheckErr("cloudflare.always_https_disabled", Medium, err)
		}
		status := Passed
		summary := "Cloudflare Always Use HTTPS is on for " + zone + "."
		remediation := ""
		if strings.EqualFold(value, "off") {
			status = Failed
			summary = "Cloudflare Always Use HTTPS is off for " + zone + "."
			remediation = "Enable 'Always Use HTTPS' in Cloudflare so HTTP requests are 308-redirected to HTTPS at the edge."
		}
		return CheckResult{
			ID: "cloudflare.always_https_disabled", Title: cloudflareTitle("cloudflare.always_https_disabled"),
			Category: "cloudflare", Severity: Medium, Status: status,
			Summary:     summary,
			Remediation: remediation,
			Evidence:    map[string]any{"zone": zone, "alwaysUseHTTPS": value},
		}
	}
}

func checkCloudflareWAF(client *CloudflareClient, zoneID, zone string) Check {
	return func(_ execx.Adapter) CheckResult {
		value, err := client.setting(zoneID, "waf")
		if err != nil {
			if cfErr, ok := err.(*CloudflareError); ok && cfErr.IsNotFound() {
				return CheckResult{
					ID: "cloudflare.waf_disabled", Title: cloudflareTitle("cloudflare.waf_disabled"),
					Category: "cloudflare", Severity: Info, Status: Skipped,
					Summary: "WAF setting unavailable for " + zone + " (likely Free plan); upgrade to Pro+ for managed WAF.",
				}
			}
			return cloudflareCheckErr("cloudflare.waf_disabled", Medium, err)
		}
		status := Passed
		summary := "Cloudflare WAF is on for " + zone + "."
		remediation := ""
		if strings.EqualFold(value, "off") {
			status = Failed
			summary = "Cloudflare WAF is off for " + zone + "."
			remediation = "Enable the Cloudflare managed WAF rules to filter common attack patterns at the edge."
		}
		return CheckResult{
			ID: "cloudflare.waf_disabled", Title: cloudflareTitle("cloudflare.waf_disabled"),
			Category: "cloudflare", Severity: Medium, Status: status,
			Summary:     summary,
			Remediation: remediation,
			Evidence:    map[string]any{"zone": zone, "waf": value},
		}
	}
}

func checkCloudflareDNSPresent(client *CloudflareClient, zoneID, name string) Check {
	return func(_ execx.Adapter) CheckResult {
		records, err := client.dnsRecords(zoneID, name)
		if err != nil {
			return cloudflareCheckErr("cloudflare.dns_missing", High, err)
		}
		hostRecords := []cfDNSRecord{}
		for _, r := range records {
			if r.Type == "A" || r.Type == "AAAA" || r.Type == "CNAME" {
				hostRecords = append(hostRecords, r)
			}
		}
		status := Passed
		severity := High
		summary := fmt.Sprintf("%d DNS host record(s) for %s.", len(hostRecords), name)
		remediation := ""
		if len(hostRecords) == 0 {
			status = Failed
			summary = "No A / AAAA / CNAME record found for " + name + " in Cloudflare."
			remediation = "Create an A or CNAME record for " + name + " pointing at the server (or its origin)."
		}
		return CheckResult{
			ID: "cloudflare.dns_missing", Title: cloudflareTitle("cloudflare.dns_missing"),
			Category: "dns", Severity: severity, Status: status,
			Summary:     summary,
			Remediation: remediation,
			Evidence:    map[string]any{"name": name, "records": hostRecords},
		}
	}
}

func checkCloudflareProxyEnabled(client *CloudflareClient, zoneID, name string) Check {
	return func(_ execx.Adapter) CheckResult {
		records, err := client.dnsRecords(zoneID, name)
		if err != nil {
			return cloudflareCheckErr("cloudflare.proxy_disabled", Medium, err)
		}
		var unproxied []cfDNSRecord
		for _, r := range records {
			if (r.Type == "A" || r.Type == "AAAA" || r.Type == "CNAME") && !r.Proxied {
				unproxied = append(unproxied, r)
			}
		}
		if len(unproxied) == 0 {
			return CheckResult{
				ID: "cloudflare.proxy_disabled", Title: cloudflareTitle("cloudflare.proxy_disabled"),
				Category: "cloudflare", Severity: Medium, Status: Passed,
				Summary:  "All host records for " + name + " are proxied through Cloudflare.",
				Evidence: map[string]any{"name": name},
			}
		}
		names := make([]string, 0, len(unproxied))
		for _, r := range unproxied {
			names = append(names, r.Name+"->"+r.Content)
		}
		return CheckResult{
			ID: "cloudflare.proxy_disabled", Title: cloudflareTitle("cloudflare.proxy_disabled"),
			Category: "cloudflare", Severity: Medium, Status: Failed,
			Summary:     fmt.Sprintf("%d DNS record(s) bypass Cloudflare proxy: %s.", len(unproxied), strings.Join(names, ", ")),
			Remediation: "Enable the orange cloud (proxied) on these records, unless you intentionally serve them direct (e.g. mail).",
			Evidence:    map[string]any{"name": name, "unproxied": unproxied},
		}
	}
}

func cloudflareCheckErr(id string, severity Severity, err error) CheckResult {
	return CheckResult{
		ID: id, Title: cloudflareTitle(id),
		Category: "cloudflare", Severity: severity, Status: Unknown,
		Summary:  "Cloudflare API error: " + err.Error() + ".",
		Evidence: map[string]any{"error": err.Error()},
	}
}
