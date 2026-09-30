package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSendServiceHealthEventSendsImmediateStartupFailure(t *testing.T) {
	var received Ping
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode telemetry ping: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	originalEndpoint := pingEndpoint
	pingEndpoint = server.URL
	defer func() { pingEndpoint = originalEndpoint }()

	err := SendServiceHealthEvent(context.Background(), Config{
		Version: "6.5.0",
		DataDir: t.TempDir(),
		Enabled: true,
	}, "startup", ServiceHealthObservation{
		Observed:        true,
		FailureCategory: ServiceHealthFailureListener,
	})
	if err != nil {
		t.Fatalf("send immediate service-health event: %v", err)
	}
	if received.Event != "startup" || !received.ServiceHealthObserved ||
		received.ServiceHealthHealthy || received.ServiceHealthFailureCategory != ServiceHealthFailureListener {
		t.Fatalf("immediate service-health ping = %#v", received)
	}
}

func TestBuildPingAtServiceHealthVersionChangeCohort(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	observation := ServiceHealthObservation{Observed: true, Healthy: true}
	cfg := Config{
		Version:          "6.4.0",
		DataDir:          dir,
		GetServiceHealth: func() ServiceHealthObservation { return observation },
	}

	first, err := buildPingAt(cfg, "startup", now)
	if err != nil {
		t.Fatalf("build first ping: %v", err)
	}
	if !first.ServiceHealthObserved || !first.ServiceHealthHealthy || first.ServiceHealthCohort != ServiceHealthCohortFirstObservation {
		t.Fatalf("first service-health observation = %#v", first)
	}

	observation = ServiceHealthObservation{Observed: true, FailureCategory: ServiceHealthFailureFrontendAssets}
	cfg.Version = "6.5.0"
	upgraded, err := buildPingAt(cfg, "startup", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("build upgraded ping: %v", err)
	}
	if upgraded.ServiceHealthHealthy || upgraded.ServiceHealthFailureCategory != ServiceHealthFailureFrontendAssets {
		t.Fatalf("upgraded service-health observation = %#v", upgraded)
	}
	if upgraded.ServiceHealthCohort != ServiceHealthCohortVersionChange ||
		upgraded.ServiceHealthPreviousVersion != "6.4.0" ||
		!upgraded.ServiceHealthPreviousObserved ||
		!upgraded.ServiceHealthPreviousHealthy {
		t.Fatalf("version-change cohort = %#v", upgraded)
	}

	observation = ServiceHealthObservation{Observed: true, Healthy: true, FailureCategory: "must-not-survive"}
	recovered, err := buildPingAt(cfg, "heartbeat", now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("build recovered ping: %v", err)
	}
	if recovered.ServiceHealthCohort != ServiceHealthCohortSameVersion ||
		!recovered.ServiceHealthHealthy || recovered.ServiceHealthFailureCategory != "" ||
		recovered.ServiceHealthPreviousVersion != "6.4.0" ||
		!recovered.ServiceHealthPreviousHealthy {
		t.Fatalf("same-version recovery = %#v", recovered)
	}
}

func TestServiceHealthObservationRejectsFreeFormFailure(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Version: "6.5.0",
		DataDir: dir,
		GetServiceHealth: func() ServiceHealthObservation {
			return ServiceHealthObservation{
				Observed:        true,
				FailureCategory: "GET http://10.0.0.4:7655/assets/private.js: connection refused",
			}
		},
	}
	ping, err := buildPingAt(cfg, "startup", time.Now().UTC())
	if err != nil {
		t.Fatalf("build ping: %v", err)
	}
	if ping.ServiceHealthFailureCategory != ServiceHealthFailureUnknown {
		t.Fatalf("failure category = %q, want unknown", ping.ServiceHealthFailureCategory)
	}
	for _, forbidden := range []string{"http", "10.0.0.4", "private.js", "connection refused"} {
		if strings.Contains(ping.ServiceHealthFailureCategory, forbidden) {
			t.Fatalf("failure category leaked %q: %q", forbidden, ping.ServiceHealthFailureCategory)
		}
	}

	raw, err := os.ReadFile(filepath.Join(dir, serviceHealthStateFile))
	if err != nil {
		t.Fatalf("read service-health state: %v", err)
	}
	if strings.Contains(string(raw), "10.0.0.4") || strings.Contains(string(raw), "private.js") {
		t.Fatalf("service-health state leaked free-form detail: %s", raw)
	}
}

func TestServiceHealthStateFileIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Version: "6.5.0",
		DataDir: dir,
		GetServiceHealth: func() ServiceHealthObservation {
			return ServiceHealthObservation{Observed: true, Healthy: true}
		},
	}
	if _, err := buildPingAt(cfg, "startup", time.Now().UTC()); err != nil {
		t.Fatalf("build ping: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, serviceHealthStateFile))
	if err != nil {
		t.Fatalf("stat service-health state: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("service-health state mode = %o, want 600", got)
	}
}

func TestServiceHealthStateRejectsTamperedPreviousVersion(t *testing.T) {
	dir := t.TempDir()
	state := `{"schema_version":1,"current_version":"https://customer.example/private","current_observed":true,"current_healthy":true}`
	if err := os.WriteFile(filepath.Join(dir, serviceHealthStateFile), []byte(state), 0o600); err != nil {
		t.Fatalf("write tampered state: %v", err)
	}
	cfg := Config{
		Version: "6.5.0",
		DataDir: dir,
		GetServiceHealth: func() ServiceHealthObservation {
			return ServiceHealthObservation{Observed: true, Healthy: true}
		},
	}
	ping, err := buildPingAt(cfg, "startup", time.Now().UTC())
	if err != nil {
		t.Fatalf("build ping: %v", err)
	}
	if ping.ServiceHealthPreviousVersion != "" || ping.ServiceHealthPreviousObserved || ping.ServiceHealthPreviousHealthy {
		t.Fatalf("tampered previous release escaped local boundary: %#v", ping)
	}
}

// Closed categories survive both the outbound JSON payload and the persisted
// adjacent-version cohort. No transport error text is part of either shape.
func TestServiceHealthFailureCategoriesSerializeAndPersist(t *testing.T) {
	categories := []string{
		ServiceHealthFailureListener, ServiceHealthFailureStartup,
		ServiceHealthFailureRuntime, ServiceHealthFailureAPIConnectivity,
		ServiceHealthFailureTimeout, ServiceHealthFailureAPIStatus,
		ServiceHealthFailureUIStatus, ServiceHealthFailureFrontendAssets,
		ServiceHealthFailureUnknown,
	}
	for _, category := range categories {
		t.Run(category, func(t *testing.T) {
			dir := t.TempDir()
			cfg := Config{Version: "6.4.6", DataDir: dir, GetServiceHealth: func() ServiceHealthObservation {
				return ServiceHealthObservation{Observed: true, FailureCategory: "  " + strings.ToUpper(category) + "  "}
			}}
			ping, err := buildPingAt(cfg, "startup", time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(ping)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]interface{}
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if fields["service_health_failure_category"] != category || fields["service_health_observed"] != true || fields["service_health_healthy"] != false {
				t.Fatalf("serialized service-health fields = %#v", fields)
			}
			record := readServiceHealthRecord(dir)
			if record.CurrentFailureCategory != category || !record.CurrentObserved || record.CurrentHealthy {
				t.Fatalf("persisted observation = %#v", record)
			}
			cfg.Version = "6.4.7"
			cfg.GetServiceHealth = func() ServiceHealthObservation { return ServiceHealthObservation{Observed: true, Healthy: true} }
			recovered, err := buildPingAt(cfg, "startup", time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			record = readServiceHealthRecord(dir)
			if recovered.ServiceHealthFailureCategory != "" || !recovered.ServiceHealthHealthy || record.PreviousFailureCategory != category || record.PreviousHealthy {
				t.Fatalf("recovery lost the previous failure: %#v / %#v", recovered, record)
			}
		})
	}
	for _, category := range []string{"api_timeout: GET http://192.0.2.1/private", "timeout private.js", "timeout\nAuthorization"} {
		if got := canonicalServiceHealthFailureCategory(category); got != ServiceHealthFailureUnknown {
			t.Fatalf("free-form category accepted: %q -> %q", category, got)
		}
	}
}

func TestTelemetryStartDoesNotWaitForServiceHealth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	entered, release, started := make(chan struct{}), make(chan struct{}), make(chan struct{})
	dir := t.TempDir()
	t.Cleanup(func() { close(release); cancel(); Stop() })
	go func() {
		startWithDelay(ctx, Config{Enabled: true, Version: "6.4.6", DataDir: dir, GetServiceHealth: func() ServiceHealthObservation {
			close(entered)
			<-release
			return ServiceHealthObservation{Observed: true, Healthy: true}
		}}, 0)
		close(started)
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("background probe did not begin")
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("telemetry startup blocked on its service-health probe")
	}
}
