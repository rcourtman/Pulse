package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
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

// TestAvailabilityHTTPOutboundOptionsUsesSharedPeerCertificateCapture moved to
// internal/availabilityprobe alongside the outbound options it asserts on.

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
	if resource.Availability.Evidence == nil {
		t.Fatal("availability evidence = nil")
	}
	if err := resource.Availability.Evidence.Validate(); err != nil {
		t.Fatalf("availability evidence validation error = %v", err)
	}
	if resource.Availability.Evidence.Subject.ProviderRef != target.ID ||
		resource.Availability.Evidence.Subject.ProviderScope != "availability-target" {
		t.Fatalf("availability evidence subject = %+v", resource.Availability.Evidence.Subject)
	}
	if resource.Availability.Evidence.ObservedAt != checkedAt {
		t.Fatalf("evidence observed at = %v, want %v", resource.Availability.Evidence.ObservedAt, checkedAt)
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
	records := availabilityPollProvider{}.SupplementalRecords(monitor, "org-a")
	if len(records) != 2 {
		t.Fatalf("SupplementalRecords() length = %d, want every configured target", len(records))
	}
	foundPaused := false
	for _, record := range records {
		if record.SourceID == "paused" {
			foundPaused = true
			if record.Resource.Availability == nil || record.Resource.Availability.Enabled {
				t.Fatalf("paused target projection = %+v, want disabled check row", record.Resource.Availability)
			}
		}
	}
	if !foundPaused {
		t.Fatal("disabled configured target is missing from supplemental records")
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
	if resource.Availability.Evidence == nil {
		t.Fatal("availability evidence = nil before first probe")
	}
	if resource.Availability.Evidence.Completeness != operationaltrust.EvidencePartial ||
		resource.Availability.Evidence.Confidence != operationaltrust.EvidenceUnknown {
		t.Fatalf("initial availability evidence = %+v, want partial/unknown", resource.Availability.Evidence)
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

func TestAvailabilityResourceProjectsCertificateAndDefaultExpiryIncident(t *testing.T) {
	checkedAt := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:       "pulse-ui",
		Name:     "Pulse UI",
		Address:  "https://pulse.example.test",
		Protocol: config.AvailabilityProbeHTTPS,
		Enabled:  true,
	})
	status := availabilityStatusFromTarget(target)
	status.Available = true
	status.LastChecked = checkedAt
	status.CertificateCurrent = true
	status.Certificate = &tlsutil.CertificateObservation{
		Subject:           "pulse.example.test",
		Issuer:            "Example CA",
		NotBefore:         checkedAt.Add(-300 * 24 * time.Hour),
		NotAfter:          checkedAt.Add(20 * 24 * time.Hour),
		ObservedAt:        checkedAt,
		FingerprintSHA256: "abcdef",
		ChainValid:        true,
		HostnameValid:     true,
		TrustStatus:       tlsutil.CertificateTrustTrusted,
	}

	resource, _ := availabilityResourceFromTarget(target, status, "org-a", checkedAt.Add(time.Second))
	if resource.Availability == nil || resource.Availability.Certificate == nil {
		t.Fatalf("availability certificate = %+v", resource.Availability)
	}
	if !resource.Availability.CertificateMonitoring || resource.Availability.CertificateExpiryWarningDays != 30 {
		t.Fatalf("certificate monitoring projection = %+v", resource.Availability)
	}
	if len(resource.Incidents) != 1 || resource.Incidents[0].Code != "certificate_expiring" {
		t.Fatalf("incidents = %+v, want certificate_expiring", resource.Incidents)
	}
	if resource.Incidents[0].Summary != "Pulse UI certificate expires in 20 days on 26 Aug 2026" {
		t.Fatalf("summary = %q", resource.Incidents[0].Summary)
	}

	resource.Availability.Certificate.DNSNames = append(resource.Availability.Certificate.DNSNames, "changed")
	if len(status.Certificate.DNSNames) != 0 {
		t.Fatal("resource projection shares certificate slices with status")
	}
}

func TestAvailabilityCertificateTrustIncidentsRespectSelfSignedExemptionAndOptOut(t *testing.T) {
	checkedAt := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	target := config.AvailabilityTarget{
		ID:       "endpoint",
		Name:     "Endpoint",
		Address:  "https://endpoint.example.test",
		Protocol: config.AvailabilityProbeHTTPS,
		Enabled:  true,
	}
	status := AvailabilityProbeStatus{
		CertificateCurrent: true,
		Certificate: &tlsutil.CertificateObservation{
			ObservedAt:  checkedAt,
			NotAfter:    checkedAt.Add(180 * 24 * time.Hour),
			TrustStatus: tlsutil.CertificateTrustSelfSigned,
			SelfSigned:  true,
		},
	}
	if incidents := availabilityCertificateIncidents(target, status); len(incidents) != 0 {
		t.Fatalf("self-signed incidents = %+v, want none", incidents)
	}

	status.Certificate.TrustStatus = tlsutil.CertificateTrustUntrusted
	status.Certificate.SelfSigned = false
	if incidents := availabilityCertificateIncidents(target, status); len(incidents) != 1 || incidents[0].Code != "certificate_untrusted" {
		t.Fatalf("untrusted incidents = %+v", incidents)
	}

	target.CertificateMonitoringDisabled = true
	if incidents := availabilityCertificateIncidents(target, status); len(incidents) != 0 {
		t.Fatalf("disabled monitoring incidents = %+v, want none", incidents)
	}
}
