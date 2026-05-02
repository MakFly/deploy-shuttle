package readiness

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeCFServer returns an httptest.Server that mocks Cloudflare endpoints.
// `routes` maps method+path to a handler.
type cfRoute struct {
	status int
	body   string
}

func newFakeCFServer(t *testing.T, routes map[string]cfRoute) (*httptest.Server, *CloudflareClient) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path
		if r.URL.RawQuery != "" {
			key += "?" + r.URL.RawQuery
		}
		route, ok := routes[key]
		if !ok {
			http.Error(w, `{"errors":[{"message":"unmocked: `+key+`"}]}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(route.status)
		_, _ = w.Write([]byte(route.body))
	}))
	t.Cleanup(srv.Close)
	client := &CloudflareClient{
		BaseURL: srv.URL,
		Token:   "test-token",
		HTTP:    srv.Client(),
	}
	return srv, client
}

func TestResolveCloudflareTokenDefaultsToCanonicalEnv(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "abc")
	if got := ResolveCloudflareToken(CloudflareConfig{}); got != "abc" {
		t.Fatalf("expected abc, got %q", got)
	}
}

func TestResolveCloudflareTokenHonorsCustomEnv(t *testing.T) {
	t.Setenv("MY_CF", "xyz")
	if got := ResolveCloudflareToken(CloudflareConfig{TokenEnv: "MY_CF"}); got != "xyz" {
		t.Fatalf("expected xyz, got %q", got)
	}
}

func TestCloudflareChecksSkippedWhenDisabled(t *testing.T) {
	checks := cloudflareChecks(CloudflareConfig{}, "app.example.com", nil)
	if len(checks) != 5 {
		t.Fatalf("expected 5 placeholder checks, got %d", len(checks))
	}
	for _, c := range checks {
		res := c(nil)
		if res.Status != Skipped {
			t.Fatalf("expected skipped, got %+v", res)
		}
	}
}

func TestCloudflareChecksSkippedWithoutZone(t *testing.T) {
	checks := cloudflareChecks(CloudflareConfig{Enabled: true}, "app.example.com", nil)
	for _, c := range checks {
		res := c(nil)
		if res.Status != Skipped {
			t.Fatalf("expected skipped, got %+v", res)
		}
		if !strings.Contains(res.Summary, "cloudflare.zone") {
			t.Fatalf("expected zone-missing summary, got %q", res.Summary)
		}
	}
}

func TestCloudflareChecksSkippedWithoutClient(t *testing.T) {
	checks := cloudflareChecks(CloudflareConfig{Enabled: true, Zone: "example.com"}, "app.example.com", nil)
	for _, c := range checks {
		res := c(nil)
		if res.Status != Skipped {
			t.Fatalf("expected skipped, got %+v", res)
		}
		if !strings.Contains(res.Summary, "API token") {
			t.Fatalf("expected token-missing summary, got %q", res.Summary)
		}
	}
}

func TestCloudflareChecksHappyPath(t *testing.T) {
	routes := map[string]cfRoute{
		"/zones?name=example.com":                       {200, `{"result":[{"id":"ZONE1","name":"example.com"}]}`},
		"/zones/ZONE1/settings/ssl":                     {200, `{"result":{"id":"ssl","value":"strict"}}`},
		"/zones/ZONE1/settings/always_use_https":        {200, `{"result":{"id":"always_use_https","value":"on"}}`},
		"/zones/ZONE1/settings/waf":                     {200, `{"result":{"id":"waf","value":"on"}}`},
		"/zones/ZONE1/dns_records?name=app.example.com": {200, `{"result":[{"id":"R1","type":"A","name":"app.example.com","content":"1.2.3.4","proxied":true}]}`},
	}
	_, client := newFakeCFServer(t, routes)
	checks := cloudflareChecks(CloudflareConfig{Enabled: true, Zone: "example.com"}, "app.example.com", client)
	if len(checks) != 5 {
		t.Fatalf("expected 5 checks, got %d", len(checks))
	}
	for _, c := range checks {
		res := c(nil)
		if res.Status != Passed {
			t.Fatalf("%s: expected passed, got %+v", res.ID, res)
		}
	}
}

func TestCloudflareSSLFlexibleFails(t *testing.T) {
	routes := map[string]cfRoute{
		"/zones?name=example.com":                       {200, `{"result":[{"id":"ZONE1","name":"example.com"}]}`},
		"/zones/ZONE1/settings/ssl":                     {200, `{"result":{"id":"ssl","value":"flexible"}}`},
		"/zones/ZONE1/settings/always_use_https":        {200, `{"result":{"id":"always_use_https","value":"on"}}`},
		"/zones/ZONE1/settings/waf":                     {200, `{"result":{"id":"waf","value":"on"}}`},
		"/zones/ZONE1/dns_records?name=app.example.com": {200, `{"result":[{"id":"R1","type":"A","name":"app.example.com","content":"1.2.3.4","proxied":true}]}`},
	}
	_, client := newFakeCFServer(t, routes)
	checks := cloudflareChecks(CloudflareConfig{Enabled: true, Zone: "example.com"}, "app.example.com", client)
	for _, c := range checks {
		res := c(nil)
		if res.ID != "cloudflare.ssl_flexible" {
			continue
		}
		if res.Status != Failed {
			t.Fatalf("expected failed, got %+v", res)
		}
		if res.Severity != Critical {
			t.Fatalf("expected critical, got %s", res.Severity)
		}
	}
}

func TestCloudflareProxyDisabledFlagsUnproxiedRecord(t *testing.T) {
	routes := map[string]cfRoute{
		"/zones?name=example.com":                       {200, `{"result":[{"id":"ZONE1","name":"example.com"}]}`},
		"/zones/ZONE1/settings/ssl":                     {200, `{"result":{"id":"ssl","value":"strict"}}`},
		"/zones/ZONE1/settings/always_use_https":        {200, `{"result":{"id":"always_use_https","value":"on"}}`},
		"/zones/ZONE1/settings/waf":                     {200, `{"result":{"id":"waf","value":"on"}}`},
		"/zones/ZONE1/dns_records?name=app.example.com": {200, `{"result":[{"id":"R1","type":"A","name":"app.example.com","content":"1.2.3.4","proxied":false}]}`},
	}
	_, client := newFakeCFServer(t, routes)
	for _, c := range cloudflareChecks(CloudflareConfig{Enabled: true, Zone: "example.com"}, "app.example.com", client) {
		res := c(nil)
		if res.ID == "cloudflare.proxy_disabled" && res.Status != Failed {
			t.Fatalf("expected proxy_disabled to fail, got %+v", res)
		}
	}
}

func TestCloudflareDNSMissingFails(t *testing.T) {
	routes := map[string]cfRoute{
		"/zones?name=example.com":                       {200, `{"result":[{"id":"ZONE1","name":"example.com"}]}`},
		"/zones/ZONE1/settings/ssl":                     {200, `{"result":{"id":"ssl","value":"strict"}}`},
		"/zones/ZONE1/settings/always_use_https":        {200, `{"result":{"id":"always_use_https","value":"on"}}`},
		"/zones/ZONE1/settings/waf":                     {200, `{"result":{"id":"waf","value":"on"}}`},
		"/zones/ZONE1/dns_records?name=app.example.com": {200, `{"result":[]}`},
	}
	_, client := newFakeCFServer(t, routes)
	for _, c := range cloudflareChecks(CloudflareConfig{Enabled: true, Zone: "example.com"}, "app.example.com", client) {
		res := c(nil)
		if res.ID == "cloudflare.dns_missing" && res.Status != Failed {
			t.Fatalf("expected dns_missing to fail, got %+v", res)
		}
	}
}

func TestCloudflareWAFSkippedOn404(t *testing.T) {
	routes := map[string]cfRoute{
		"/zones?name=example.com":                       {200, `{"result":[{"id":"ZONE1","name":"example.com"}]}`},
		"/zones/ZONE1/settings/ssl":                     {200, `{"result":{"id":"ssl","value":"strict"}}`},
		"/zones/ZONE1/settings/always_use_https":        {200, `{"result":{"id":"always_use_https","value":"on"}}`},
		"/zones/ZONE1/settings/waf":                     {404, `{"errors":[{"code":1006,"message":"not available on this plan"}]}`},
		"/zones/ZONE1/dns_records?name=app.example.com": {200, `{"result":[{"id":"R1","type":"A","name":"app.example.com","content":"1.2.3.4","proxied":true}]}`},
	}
	_, client := newFakeCFServer(t, routes)
	for _, c := range cloudflareChecks(CloudflareConfig{Enabled: true, Zone: "example.com"}, "app.example.com", client) {
		res := c(nil)
		if res.ID != "cloudflare.waf_disabled" {
			continue
		}
		if res.Status != Skipped {
			t.Fatalf("expected waf to skip on 404, got %+v", res)
		}
	}
}

func TestCloudflareChecksSkipOnAuthError(t *testing.T) {
	routes := map[string]cfRoute{
		"/zones?name=example.com": {403, `{"errors":[{"code":10000,"message":"Authentication error"}]}`},
	}
	_, client := newFakeCFServer(t, routes)
	checks := cloudflareChecks(CloudflareConfig{Enabled: true, Zone: "example.com"}, "app.example.com", client)
	for _, c := range checks {
		res := c(nil)
		if res.Status != Skipped {
			t.Fatalf("expected skipped on auth error, got %+v", res)
		}
		if !strings.Contains(res.Summary, "rejected") {
			t.Fatalf("expected rejected token summary, got %q", res.Summary)
		}
	}
}
