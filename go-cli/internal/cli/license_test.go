package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/license"
)

func TestLicenseDeactivateCallsServerAndClearsLocalState(t *testing.T) {
	t.Setenv("SHUTTLE_HOME", t.TempDir())
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/deactivate" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"deactivated":true}`))
	}))
	defer server.Close()
	if err := license.Save("", license.State{
		Token:       "token",
		ServerURL:   server.URL,
		Tier:        "pro",
		ExpiresAt:   time.Now().Add(time.Hour),
		RefreshAt:   time.Now().Add(30 * time.Minute),
		ActivatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	cmd := newLicenseCommand()
	cmd.SetArgs([]string{"deactivate"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected deactivate endpoint to be called")
	}
	if _, err := license.Load(""); err != license.ErrNoLicense {
		t.Fatalf("expected local state to be cleared, got %v", err)
	}
}

func TestLicenseDeactivateLocalOnlySkipsServer(t *testing.T) {
	t.Setenv("SHUTTLE_HOME", t.TempDir())
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer server.Close()
	if err := license.Save("", license.State{
		Token:       "token",
		ServerURL:   server.URL,
		Tier:        "pro",
		ExpiresAt:   time.Now().Add(time.Hour),
		RefreshAt:   time.Now().Add(30 * time.Minute),
		ActivatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	cmd := newLicenseCommand()
	cmd.SetArgs([]string{"deactivate", "--local-only"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("expected server not to be called")
	}
}
