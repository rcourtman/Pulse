package monitoring

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

type deadManRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn deadManRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func deadManResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newDeadManTestRuntime(t *testing.T, now time.Time, transport http.RoundTripper) *deadManRuntime {
	t.Helper()
	runtime := newDeadManRuntime(t.TempDir())
	runtime.startupAt = now
	runtime.now = func() time.Time { return now }
	runtime.retryDelays = nil
	runtime.client = &http.Client{
		Transport: transport,
		Timeout:   time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return runtime
}

func TestDeadManRunCycleSendsHealthySignalAndPersistsProgress(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	var request *http.Request
	runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(got *http.Request) (*http.Response, error) {
		request = got
		return deadManResponse(http.StatusOK, "OK"), nil
	}))

	runtime.runCycle(
		context.Background(),
		func() string { return "https://watchdog.example.com/ping/secret-token" },
		func() time.Time { return now.Add(-5 * time.Second) },
		nil,
	)

	if request == nil || request.Method != http.MethodGet || request.URL.String() != "https://watchdog.example.com/ping/secret-token" {
		t.Fatalf("healthy request = %#v", request)
	}
	status := runtime.statusSnapshot()
	if status.State != "healthy" || status.LastSuccessAt == nil || status.ConsecutiveFailures != 0 {
		t.Fatalf("healthy status = %+v", status)
	}
	persisted, err := loadDeadManState(runtime.statePath)
	if err != nil {
		t.Fatalf("loadDeadManState: %v", err)
	}
	if !persisted.Enabled || persisted.LastHealthyAt.IsZero() || persisted.LastSuccessfulPing.IsZero() {
		t.Fatalf("persisted healthy state = %+v", persisted)
	}
}

func TestDeadManRunCycleSignalsFailureWhenCanonicalMonitorStalls(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	var method, path, body string
	runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		method = request.Method
		path = request.URL.Path
		data, _ := io.ReadAll(request.Body)
		body = string(data)
		return deadManResponse(http.StatusOK, "OK"), nil
	}))

	runtime.runCycle(
		context.Background(),
		func() string { return "https://watchdog.example.com/ping/secret-token" },
		func() time.Time { return now.Add(-2 * time.Minute) },
		nil,
	)

	if method != http.MethodPost || path != "/ping/secret-token/fail" || !strings.Contains(body, "monitoring loop has stopped") {
		t.Fatalf("stalled signal = method %q path %q body %q", method, path, body)
	}
	status := runtime.statusSnapshot()
	if status.State != "monitor_stalled" || status.LastSuccessAt != nil {
		t.Fatalf("stalled status = %+v", status)
	}
}

func TestDeadManRetriesTransientResponsesButNotPermanentRejections(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	t.Run("retries server failures", func(t *testing.T) {
		attempts := 0
		runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return deadManResponse(http.StatusServiceUnavailable, "unavailable"), nil
			}
			return deadManResponse(http.StatusOK, "OK"), nil
		}))
		runtime.retryDelays = []time.Duration{0, 0}
		runtime.runCycle(context.Background(), func() string {
			return "https://watchdog.example.com/ping/retry-token"
		}, func() time.Time { return now }, nil)
		if attempts != 3 || runtime.statusSnapshot().State != "healthy" {
			t.Fatalf("attempts = %d status = %+v", attempts, runtime.statusSnapshot())
		}
	})

	t.Run("does not follow redirects", func(t *testing.T) {
		attempts := 0
		runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			attempts++
			response := deadManResponse(http.StatusFound, "redirect")
			response.Header.Set("Location", "https://collector.example.net/capture")
			return response, nil
		}))
		runtime.retryDelays = []time.Duration{0, 0}
		runtime.runCycle(context.Background(), func() string {
			return "https://watchdog.example.com/ping/never-forward-this-token"
		}, func() time.Time { return now }, nil)
		status := runtime.statusSnapshot()
		if attempts != 1 || status.State != "delivery_failed" || strings.Contains(status.LastError, "never-forward-this-token") {
			t.Fatalf("attempts = %d status = %+v", attempts, status)
		}
	})

	t.Run("rejects deceptive success body", func(t *testing.T) {
		attempts := 0
		runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			attempts++
			return deadManResponse(http.StatusOK, "Ping not found"), nil
		}))
		runtime.retryDelays = []time.Duration{0, 0}
		runtime.runCycle(context.Background(), func() string {
			return "https://watchdog.example.com/ping/missing-token"
		}, func() time.Time { return now }, nil)
		if attempts != 1 || runtime.statusSnapshot().State != "delivery_failed" {
			t.Fatalf("attempts = %d status = %+v", attempts, runtime.statusSnapshot())
		}
	})
}

func TestDeadManRestartGapIsReportedExternallyAndRecordedInAlertHistory(t *testing.T) {
	dir := t.TempDir()
	startup := time.Now().UTC().Truncate(time.Second)
	endpoint := "https://watchdog.example.com/ping/restart-token"
	previous := deadManPersistedState{
		SchemaVersion:       deadManStateSchemaVersion,
		EndpointFingerprint: deadManEndpointFingerprint(endpoint),
		Enabled:             true,
		StartedAt:           startup.Add(-time.Hour),
		LastHealthyAt:       startup.Add(-5 * time.Minute),
	}
	statePath := dir + "/alerts/deadman-state.json"
	if err := persistDeadManState(statePath, previous); err != nil {
		t.Fatalf("persist previous state: %v", err)
	}

	var method, body string
	runtime := newDeadManRuntime(dir)
	runtime.startupAt = startup
	runtime.now = func() time.Time { return startup }
	runtime.retryDelays = nil
	runtime.client = &http.Client{Transport: deadManRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		method = request.Method
		data, _ := io.ReadAll(request.Body)
		body = string(data)
		return deadManResponse(http.StatusOK, "OK"), nil
	})}
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)

	runtime.runCycle(context.Background(), func() string { return endpoint }, func() time.Time { return startup }, manager)

	if method != http.MethodPost || !strings.Contains(body, "unexpected shutdown") || !strings.Contains(body, "5m0s") {
		t.Fatalf("restart report = method %q body %q", method, body)
	}
	status := runtime.statusSnapshot()
	if status.LastInterruption == nil || status.LastInterruption.CleanShutdown || status.LastInterruption.DurationSecs != 300 {
		t.Fatalf("restart status = %+v", status)
	}
	history := manager.GetAlertHistory(0)
	found := false
	for _, alert := range history {
		if alert.ID == alerts.SystemAlertID(alerts.DeadManInterruptionAlertType) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("restart interruption missing from alert history: %+v", history)
	}
	stored, err := loadDeadManState(statePath)
	if err != nil {
		t.Fatalf("load restart state: %v", err)
	}
	if stored.LastInterruption == nil || stored.LastInterruption.DurationSecs != 300 {
		t.Fatalf("stored interruption = %+v", stored.LastInterruption)
	}
}

func TestDeadManStopWinsAgainstLaterHeartbeatWrites(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	requests := 0
	runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
		mu.Lock()
		requests++
		mu.Unlock()
		return deadManResponse(http.StatusOK, "OK"), nil
	}))
	endpoint := "https://watchdog.example.com/ping/stop-token"
	runtime.runCycle(context.Background(), func() string { return endpoint }, func() time.Time { return now }, nil)
	stoppedAt := now.Add(30 * time.Second)
	runtime.stop(stoppedAt, nil)
	runtime.runCycle(context.Background(), func() string { return endpoint }, func() time.Time { return stoppedAt }, nil)

	mu.Lock()
	requestCount := requests
	mu.Unlock()
	if requestCount != 1 {
		t.Fatalf("requests after stop = %d, want 1", requestCount)
	}
	stored, err := loadDeadManState(runtime.statePath)
	if err != nil {
		t.Fatalf("load stopped state: %v", err)
	}
	if !stored.StoppedAt.Equal(stoppedAt) {
		t.Fatalf("stoppedAt = %s, want %s", stored.StoppedAt, stoppedAt)
	}
}

func TestDeadManConfigurationChangeCancelsActiveSignalWithoutRecordingFailure(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	started := make(chan struct{})
	finished := make(chan struct{})
	runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		close(started)
		<-request.Context().Done()
		return nil, request.Context().Err()
	}))

	go func() {
		defer close(finished)
		runtime.runCycle(context.Background(), func() string {
			return "https://watchdog.example.com/ping/replaced-token"
		}, func() time.Time { return now }, nil)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("heartbeat request did not start")
	}
	runtime.notifyConfigChanged()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("configuration change did not cancel heartbeat request")
	}
	status := runtime.statusSnapshot()
	if status.ConsecutiveFailures != 0 || status.State == "delivery_failed" {
		t.Fatalf("cancelled replacement recorded a delivery failure: %+v", status)
	}
}

func TestDeadManDialRejectsLoopbackResolution(t *testing.T) {
	connection, err := deadManDialContext(context.Background(), "tcp", "localhost:80")
	if connection != nil {
		_ = connection.Close()
		t.Fatal("loopback watchdog dial unexpectedly succeeded")
	}
	if err == nil {
		t.Fatal("loopback watchdog dial unexpectedly returned no error")
	}
}

func TestDeadManDialRejectsPulseInterfaceAddress(t *testing.T) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatalf("enumerate local interfaces: %v", err)
	}
	for _, address := range addresses {
		ip, _, err := net.ParseCIDR(address.String())
		if err != nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			continue
		}
		connection, dialErr := deadManDialContext(context.Background(), "tcp", net.JoinHostPort(ip.String(), "80"))
		if connection != nil {
			_ = connection.Close()
			t.Fatalf("same-host watchdog dial to %s unexpectedly succeeded", ip)
		}
		if dialErr == nil {
			t.Fatalf("same-host watchdog dial to %s unexpectedly returned no error", ip)
		}
		return
	}
	t.Skip("no non-loopback interface address available")
}
