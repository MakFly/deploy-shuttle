package cli

import (
	"net/http"
	"sync"
	"time"
)

type availabilityProbeResult struct {
	Samples      int
	Failures     int
	FirstFailure string
}

type availabilityProbe struct {
	url      string
	interval time.Duration
	client   *http.Client
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
	mu       sync.Mutex
	result   availabilityProbeResult
}

func startAvailabilityProbe(url string, intervalMS int) *availabilityProbe {
	if url == "" {
		return nil
	}
	if intervalMS <= 0 {
		intervalMS = 250
	}
	probe := &availabilityProbe{
		url:      url,
		interval: time.Duration(intervalMS) * time.Millisecond,
		client:   &http.Client{Timeout: 2 * time.Second},
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go probe.run()
	return probe
}

func (probe *availabilityProbe) run() {
	defer close(probe.done)
	probe.sample()
	ticker := time.NewTicker(probe.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			probe.sample()
		case <-probe.stop:
			return
		}
	}
}

func (probe *availabilityProbe) sample() {
	response, err := probe.client.Get(probe.url)
	failure := ""
	if err != nil {
		failure = err.Error()
	} else {
		response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			failure = response.Status
		}
	}

	probe.mu.Lock()
	defer probe.mu.Unlock()
	probe.result.Samples++
	if failure != "" {
		probe.result.Failures++
		if probe.result.FirstFailure == "" {
			probe.result.FirstFailure = failure
		}
	}
}

func (probe *availabilityProbe) Stop() availabilityProbeResult {
	if probe == nil {
		return availabilityProbeResult{}
	}
	probe.once.Do(func() {
		close(probe.stop)
		<-probe.done
	})
	probe.mu.Lock()
	defer probe.mu.Unlock()
	return probe.result
}
