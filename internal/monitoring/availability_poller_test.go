package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestProbeAvailabilityTargetHTTPFallsBackToGETWhenHeadNotAllowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusMethodNotAllowed)
		case http.MethodGet:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		Address:       server.URL,
		Protocol:      config.AvailabilityProbeHTTP,
		Enabled:       true,
		TimeoutMillis: 1000,
	})

	if err := ProbeAvailabilityTarget(context.Background(), target); err != nil {
		t.Fatalf("ProbeAvailabilityTarget() error = %v", err)
	}
}

func TestAvailabilityHTTPOutboundOptionsUsesSharedPeerCertificateCapture(t *testing.T) {
	tlsConfig := availabilityHTTPOutboundOptions().TLSConfig
	if tlsConfig == nil || !tlsConfig.InsecureSkipVerify {
		t.Fatal("availability TLS config must enter explicit peer-certificate capture mode")
	}
	if tlsConfig.VerifyPeerCertificate == nil {
		t.Fatal("availability TLS config must reject missing or malformed peer certificates")
	}
}

func TestProbeAvailabilityTargetHTTPTreatsServerErrorsAsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		Address:       server.URL,
		Protocol:      config.AvailabilityProbeHTTP,
		Enabled:       true,
		TimeoutMillis: 1000,
	})

	if err := ProbeAvailabilityTarget(context.Background(), target); err == nil {
		t.Fatal("ProbeAvailabilityTarget() error = nil, want HTTP 5xx error")
	}
}

func TestProbeAvailabilityTargetHTTPRejectsMetadataService(t *testing.T) {
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		Address:       "http://169.254.169.254/latest/meta-data",
		Protocol:      config.AvailabilityProbeHTTP,
		Enabled:       true,
		TimeoutMillis: 1000,
	})

	err := ProbeAvailabilityTarget(context.Background(), target)
	if err == nil {
		t.Fatal("ProbeAvailabilityTarget() error = nil, want metadata-service rejection")
	}
	if got := err.Error(); !strings.Contains(got, "metadata service") {
		t.Fatalf("error = %q, want metadata-service rejection", got)
	}
}

func TestAvailabilityPollProviderSupplementalRecordsProjectNetworkEndpointIncident(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:               "sensor-1",
		Name:             "Energy monitor",
		TargetKind:       config.AvailabilityTargetDevice,
		Address:          "192.0.2.10",
		Protocol:         config.AvailabilityProbeICMP,
		Enabled:          true,
		FailureThreshold: 2,
	})
	if err := persistence.SaveAvailabilityTargets([]config.AvailabilityTarget{target}); err != nil {
		t.Fatalf("SaveAvailabilityTargets() error = %v", err)
	}

	checkedAt := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	monitor := &Monitor{
		configPersist: persistence,
		availabilityStatuses: map[string]AvailabilityProbeStatus{
			target.ID: {
				TargetID:            target.ID,
				Name:                target.DisplayName(),
				Address:             target.Address,
				Protocol:            string(target.Protocol),
				Enabled:             true,
				Available:           false,
				LastChecked:         checkedAt,
				ConsecutiveFailures: 2,
				LastError:           "timeout",
				FailureThreshold:    2,
			},
		},
	}

	records := availabilityPollProvider{}.SupplementalRecords(monitor, "org-a")
	if len(records) != 1 {
		t.Fatalf("SupplementalRecords() length = %d, want 1", len(records))
	}

	resource := records[0].Resource
	if resource.Type != unifiedresources.ResourceTypeNetworkEndpoint {
		t.Fatalf("resource type = %q, want network-endpoint", resource.Type)
	}
	if resource.Status != unifiedresources.StatusOffline {
		t.Fatalf("resource status = %q, want offline", resource.Status)
	}
	if resource.Availability == nil || resource.Availability.TargetID != target.ID {
		t.Fatalf("availability payload = %+v, want target %q", resource.Availability, target.ID)
	}
	if resource.Availability.TargetKind != string(config.AvailabilityTargetDevice) {
		t.Fatalf("availability target kind = %q, want %q", resource.Availability.TargetKind, config.AvailabilityTargetDevice)
	}
	if len(resource.Incidents) != 1 || resource.Incidents[0].Code != "availability_unreachable" {
		t.Fatalf("incidents = %+v, want availability_unreachable", resource.Incidents)
	}
	if len(records[0].Identity.IPAddresses) != 1 || records[0].Identity.IPAddresses[0] != "192.0.2.10" {
		t.Fatalf("identity IPs = %+v, want 192.0.2.10", records[0].Identity.IPAddresses)
	}
}

func TestAvailabilityPollProviderListsOnlyEnabledTargets(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	targets := []config.AvailabilityTarget{
		{ID: "enabled", Name: "Enabled", Address: "enabled.local", Protocol: config.AvailabilityProbeICMP, Enabled: true},
		{ID: "paused", Name: "Paused", Address: "paused.local", Protocol: config.AvailabilityProbeICMP, Enabled: false},
	}
	if err := persistence.SaveAvailabilityTargets(targets); err != nil {
		t.Fatalf("SaveAvailabilityTargets() error = %v", err)
	}

	monitor := &Monitor{configPersist: persistence}
	got := availabilityPollProvider{}.ListInstances(monitor)
	if len(got) != 1 || got[0] != "enabled" {
		t.Fatalf("ListInstances() = %+v, want [enabled]", got)
	}
}

func TestAvailabilityResourceFromTargetOmitsUnsetProbeTimes(t *testing.T) {
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:       "router",
		Name:     "Router",
		Address:  "192.0.2.1",
		Protocol: config.AvailabilityProbeICMP,
		Enabled:  true,
	})
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)

	resource, _ := availabilityResourceFromTarget(target, availabilityStatusFromTarget(target), "org-a", now)
	if resource.Availability == nil {
		t.Fatal("availability payload = nil")
	}
	if resource.Availability.LastChecked != nil {
		t.Fatalf("last checked = %v, want nil before the first probe", resource.Availability.LastChecked)
	}
	if resource.Availability.LastSuccess != nil {
		t.Fatalf("last success = %v, want nil before the first successful probe", resource.Availability.LastSuccess)
	}
}

func TestAvailabilityResourceFromTargetPreservesProbeTimes(t *testing.T) {
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:       "router",
		Name:     "Router",
		Address:  "192.0.2.1",
		Protocol: config.AvailabilityProbeICMP,
		Enabled:  true,
	})
	checkedAt := time.Date(2026, 7, 9, 11, 59, 55, 0, time.UTC)
	succeededAt := checkedAt.Add(-time.Minute)
	status := availabilityStatusFromTarget(target)
	status.LastChecked = checkedAt
	status.LastSuccess = succeededAt

	resource, _ := availabilityResourceFromTarget(target, status, "org-a", checkedAt.Add(time.Second))
	if resource.Availability == nil {
		t.Fatal("availability payload = nil")
	}
	if resource.Availability.LastChecked == nil || !resource.Availability.LastChecked.Equal(checkedAt) {
		t.Fatalf("last checked = %v, want %v", resource.Availability.LastChecked, checkedAt)
	}
	if resource.Availability.LastSuccess == nil || !resource.Availability.LastSuccess.Equal(succeededAt) {
		t.Fatalf("last success = %v, want %v", resource.Availability.LastSuccess, succeededAt)
	}
}
