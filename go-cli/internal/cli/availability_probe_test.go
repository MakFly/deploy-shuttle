package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAvailabilityProbeRecordsContinuousSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	probe := startAvailabilityProbe(server.URL, 10)
	time.Sleep(35 * time.Millisecond)
	result := probe.Stop()
	if result.Samples < 2 || result.Failures != 0 {
		t.Fatalf("unexpected probe result: %+v", result)
	}
}

func TestAvailabilityProbeRecordsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	probe := startAvailabilityProbe(server.URL, 10)
	time.Sleep(25 * time.Millisecond)
	result := probe.Stop()
	if result.Failures == 0 || result.FirstFailure != "502 Bad Gateway" {
		t.Fatalf("expected HTTP failure, got %+v", result)
	}
}
