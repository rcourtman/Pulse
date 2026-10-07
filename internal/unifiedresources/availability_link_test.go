package unifiedresources

import (
	"slices"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// ingestAgentFixture ingests a single agent resource and returns its
// canonical registry id, so availability link tests can reference the exact
// resource a probe should attach to.
func ingestAgentFixture(t *testing.T, rr *ResourceRegistry, sourceID, machineID string, ips ...string) string {
	t.Helper()
	now := time.Now().UTC()
	rr.IngestRecords(SourceAgent, []IngestRecord{{
		SourceID: sourceID,
		Resource: Resource{
			Type:     ResourceTypeAgent,
			Name:     sourceID,
			Status:   StatusOnline,
			LastSeen: now,
		},
		Identity: ResourceIdentity{MachineID: machineID, IPAddresses: ips},
	}})
	agents := rr.ListByType(ResourceTypeAgent)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent ingested, got %d", len(agents))
	}
	return agents[0].ID
}

func availabilityProbeRecord(targetID, address string, facet *AvailabilityData) IngestRecord {
	now := time.Now().UTC()
	if facet == nil {
		facet = &AvailabilityData{TargetID: targetID, Address: address, Protocol: "icmp", Enabled: true, Available: true}
	}
	facet.TargetID = targetID
	return IngestRecord{
		SourceID: targetID,
		Resource: Resource{
			Type:         ResourceTypeNetworkEndpoint,
			Name:         targetID,
			Status:       StatusOnline,
			LastSeen:     now,
			Sources:      []DataSource{SourceAvailability},
			Availability: facet,
		},
		Identity: ResourceIdentity{IPAddresses: []string{address}},
	}
}

func availabilityProbeEvidence(t *testing.T, targetID string, observedAt time.Time) *operationaltrust.EvidenceEnvelope {
	t.Helper()
	source := operationaltrust.EvidenceSource{
		Provider:  string(SourceAvailability),
		Collector: "availability-poller",
	}
	subject := operationaltrust.EvidenceSubject{
		ProviderRef:   targetID,
		ProviderScope: "availability-target",
	}
	id, err := operationaltrust.NewEvidenceID(source, subject, observedAt, targetID)
	if err != nil {
		t.Fatalf("NewEvidenceID() error = %v", err)
	}
	validUntil := observedAt.Add(2 * time.Minute)
	return &operationaltrust.EvidenceEnvelope{
		ID:           id,
		Source:       source,
		Subject:      subject,
		ObservedAt:   observedAt,
		IngestedAt:   observedAt,
		ValidUntil:   &validUntil,
		Completeness: operationaltrust.EvidenceComplete,
		Confidence:   operationaltrust.EvidenceConfirmed,
		Permissions:  operationaltrust.EvidencePermissionsSufficient,
		PayloadRef: &operationaltrust.EvidencePayloadRef{
			Kind: "availability-target",
			ID:   targetID,
		},
	}
}

func availabilityEndpointByTarget(t *testing.T, rr *ResourceRegistry, targetID string) Resource {
	t.Helper()
	for _, endpoint := range rr.ListByType(ResourceTypeNetworkEndpoint) {
		if endpoint.Availability != nil && endpoint.Availability.TargetID == targetID {
			return endpoint
		}
	}
	t.Fatalf("availability endpoint %q missing", targetID)
	return Resource{}
}

func TestAvailabilityExplicitLinkRetainsCheckAndProjectsFacetToKnownResource(t *testing.T) {
	rr := NewRegistry(nil)
	hostID := ingestAgentFixture(t, rr, "host-1", "machine-1")
	observedAt := time.Now().UTC()

	rr.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-1", "192.0.2.10", &AvailabilityData{
			LinkedResourceID: hostID,
			Address:          "192.0.2.10",
			Protocol:         "icmp",
			Enabled:          true,
			Available:        true,
			LastChecked:      &observedAt,
			Evidence:         availabilityProbeEvidence(t, "probe-1", observedAt),
		}),
	})

	if got := rr.ListByType(ResourceTypeNetworkEndpoint); len(got) != 1 {
		t.Fatalf("expected configured check to retain its endpoint row, got %d", len(got))
	}
	check := availabilityEndpointByTarget(t, rr, "probe-1")
	host, ok := rr.Get(hostID)
	if !ok || host == nil {
		t.Fatalf("host %q missing after ingest", hostID)
	}
	if host.Availability == nil || host.Availability.TargetID != "probe-1" {
		t.Fatalf("expected availability facet probe-1 on host, got %+v", host.Availability)
	}
	if !hasDataSource(host.Sources, SourceAvailability) {
		t.Fatalf("expected host sources to include availability, got %v", host.Sources)
	}
	if host.Availability.CorrelationState != AvailabilityCorrelationAttached ||
		host.Availability.CorrelationRule != "explicit_resource_link" {
		t.Fatalf("availability correlation = %+v, want attached explicit link", host.Availability)
	}
	if host.Availability.Evidence == nil ||
		host.Availability.Evidence.Subject.ResourceID != hostID ||
		host.Availability.Evidence.Subject.ProviderRef != "" {
		t.Fatalf("bound evidence = %+v, want canonical subject %q", host.Availability.Evidence, hostID)
	}
	if host.Availability.Evidence.Correlation == nil ||
		host.Availability.Evidence.Correlation.Rule != "explicit_resource_link" {
		t.Fatalf("evidence correlation = %+v, want explicit resource link", host.Availability.Evidence.Correlation)
	}
	if check.Availability == nil ||
		check.Availability.Evidence == nil ||
		check.Availability.Evidence.Subject.ResourceID != check.ID {
		t.Fatalf("check evidence = %+v, want source-owned subject %q", check.Availability, check.ID)
	}
	if host.Status != StatusOnline || len(host.Incidents) != 0 {
		t.Fatalf("host status/incidents were overwritten by check: status=%q incidents=%+v", host.Status, host.Incidents)
	}
	if len(host.Identity.IPAddresses) != 0 {
		t.Fatalf("host identity was polluted by service address: %+v", host.Identity)
	}
	foundChecksRelationship := false
	for _, relationship := range check.Relationships {
		if relationship.Type == RelChecks &&
			relationship.SourceID == check.ID &&
			relationship.TargetID == hostID &&
			relationship.Metadata["targetId"] == "probe-1" {
			if relationship.ID == "" {
				t.Fatal("availability relationship is missing its stable ID")
			}
			if relationship.EvidenceID != check.Availability.Evidence.ID {
				t.Fatalf(
					"availability relationship evidence = %q, want %q",
					relationship.EvidenceID,
					check.Availability.Evidence.ID,
				)
			}
			foundChecksRelationship = true
		}
	}
	if !foundChecksRelationship {
		t.Fatalf("relationships = %+v, want availability checks edge", check.Relationships)
	}
	if len(host.Relationships) != 0 {
		t.Fatalf("host must not own the check relationship, got %+v", host.Relationships)
	}
}

func TestAvailabilityUnlinkedUnmatchedMintsNetworkEndpoint(t *testing.T) {
	rr := NewRegistry(nil)

	rr.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-orphan", "198.51.100.7", nil),
	})

	if got := rr.ListByType(ResourceTypeNetworkEndpoint); len(got) != 1 {
		t.Fatalf("expected 1 standalone network endpoint, got %d", len(got))
	} else if got[0].Availability == nil ||
		got[0].Availability.CorrelationState != AvailabilityCorrelationStandalone {
		t.Fatalf("standalone availability correlation = %+v", got[0].Availability)
	}
}

func TestAvailabilityExactIPMatchAttachesToKnownResource(t *testing.T) {
	rr := NewRegistry(nil)
	hostID := ingestAgentFixture(t, rr, "host-1", "machine-1", "203.0.113.9")

	rr.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-ip", "203.0.113.9", &AvailabilityData{
			Address:   "203.0.113.9",
			Protocol:  "tcp",
			Enabled:   true,
			Available: true,
		}),
	})

	if got := rr.ListByType(ResourceTypeNetworkEndpoint); len(got) != 1 {
		t.Fatalf("expected attached probe to retain one endpoint, got %d", len(got))
	}
	host, ok := rr.Get(hostID)
	if !ok || host == nil || host.Availability == nil || host.Availability.TargetID != "probe-ip" {
		t.Fatalf("expected availability facet probe-ip on host, got %+v", host)
	}
	if host.Availability.CorrelationRule != "normalized_ip" {
		t.Fatalf("correlation rule = %q, want normalized_ip", host.Availability.CorrelationRule)
	}
}

func TestAvailabilityExactFullHostnameMatchAttachesToKnownResource(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Now().UTC()
	rr.IngestRecords(SourceAgent, []IngestRecord{{
		SourceID: "host-1",
		Resource: Resource{
			Type:     ResourceTypeAgent,
			Name:     "host-1",
			Status:   StatusOnline,
			LastSeen: now,
		},
		Identity: ResourceIdentity{
			MachineID: "machine-1",
			Hostnames: []string{"API.Example.Test."},
		},
	}})
	hostID := rr.ListByType(ResourceTypeAgent)[0].ID

	record := availabilityProbeRecord("probe-hostname", "api.example.test", nil)
	record.Identity = ResourceIdentity{Hostnames: []string{"api.example.test"}}
	rr.IngestRecords(SourceAvailability, []IngestRecord{record})

	if got := rr.ListByType(ResourceTypeNetworkEndpoint); len(got) != 1 {
		t.Fatalf("expected attached hostname probe to retain its endpoint, got %d", len(got))
	}
	host, ok := rr.Get(hostID)
	if !ok || host == nil || host.Availability == nil {
		t.Fatalf("host availability = %+v", host)
	}
	if host.Availability.CorrelationRule != "normalized_hostname" {
		t.Fatalf("correlation rule = %q, want normalized_hostname", host.Availability.CorrelationRule)
	}
}

func TestAvailabilityShortHostnameCollisionDoesNotAttach(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Now().UTC()
	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-a",
			Resource: Resource{Type: ResourceTypeAgent, Name: "host-a", Status: StatusOnline, LastSeen: now},
			Identity: ResourceIdentity{MachineID: "machine-a", Hostnames: []string{"api.alpha.test"}},
		},
		{
			SourceID: "host-b",
			Resource: Resource{Type: ResourceTypeAgent, Name: "host-b", Status: StatusOnline, LastSeen: now},
			Identity: ResourceIdentity{MachineID: "machine-b", Hostnames: []string{"api.beta.test"}},
		},
	})

	record := availabilityProbeRecord("probe-hostname", "api.gamma.test", nil)
	record.Identity = ResourceIdentity{Hostnames: []string{"api.gamma.test"}}
	rr.IngestRecords(SourceAvailability, []IngestRecord{record})

	endpoints := rr.ListByType(ResourceTypeNetworkEndpoint)
	if len(endpoints) != 1 || endpoints[0].Availability == nil {
		t.Fatalf("standalone endpoints = %+v, want unresolved hostname endpoint", endpoints)
	}
	if endpoints[0].Availability.CorrelationState != AvailabilityCorrelationStandalone {
		t.Fatalf("correlation state = %q, want standalone", endpoints[0].Availability.CorrelationState)
	}
}

func TestAvailabilityAmbiguousIPDoesNotAttach(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Now().UTC()
	rr.IngestRecords(SourceAgent, []IngestRecord{
		{SourceID: "h1", Resource: Resource{Type: ResourceTypeAgent, Name: "h1", Status: StatusOnline, LastSeen: now}, Identity: ResourceIdentity{MachineID: "m1", IPAddresses: []string{"203.0.113.9"}}},
		{SourceID: "h2", Resource: Resource{Type: ResourceTypeAgent, Name: "h2", Status: StatusOnline, LastSeen: now}, Identity: ResourceIdentity{MachineID: "m2", IPAddresses: []string{"203.0.113.9"}}},
	})

	rr.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-amb", "203.0.113.9", nil),
	})

	if got := rr.ListByType(ResourceTypeNetworkEndpoint); len(got) != 1 {
		t.Fatalf("expected 1 standalone endpoint (ambiguous IP, no attach), got %d", len(got))
	} else if got[0].Availability == nil ||
		got[0].Availability.CorrelationState != AvailabilityCorrelationAmbiguous ||
		got[0].Availability.CorrelationCandidates != 2 {
		t.Fatalf("ambiguous correlation = %+v, want 2 candidates", got[0].Availability)
	}
}

func TestAvailabilityInvalidExplicitLinkFailsClosedBeforeAddressCorrelation(t *testing.T) {
	rr := NewRegistry(nil)
	hostID := ingestAgentFixture(t, rr, "host-1", "machine-1", "203.0.113.20")

	rr.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-explicit-missing", "203.0.113.20", &AvailabilityData{
			LinkedResourceID: "missing-resource",
			Address:          "203.0.113.20",
			Protocol:         "icmp",
			Enabled:          true,
			Available:        true,
		}),
	})

	host, ok := rr.Get(hostID)
	if !ok || host == nil {
		t.Fatalf("host %q missing", hostID)
	}
	if host.Availability != nil {
		t.Fatalf("invalid explicit link must not fall back to IP attachment, got %+v", host.Availability)
	}
	endpoints := rr.ListByType(ResourceTypeNetworkEndpoint)
	if len(endpoints) != 1 || endpoints[0].Availability == nil {
		t.Fatalf("unresolved endpoints = %+v", endpoints)
	}
	if endpoints[0].Availability.CorrelationState != AvailabilityCorrelationUnresolved ||
		endpoints[0].Availability.CorrelationReason != "explicit_resource_link_unresolved" {
		t.Fatalf("correlation = %+v, want explicit unresolved", endpoints[0].Availability)
	}
}

func TestAvailabilityKeepsEveryConfiguredCheckWithMultipleServicesOnOneHost(t *testing.T) {
	rr := NewRegistry(nil)
	hostID := ingestAgentFixture(t, rr, "host-1", "machine-1")

	rr.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-a", "203.0.113.10", &AvailabilityData{
			LinkedResourceID: hostID,
			Address:          "203.0.113.10",
			Protocol:         "icmp",
			Enabled:          true,
			Available:        true,
		}),
	})
	rr.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-b", "203.0.113.11", &AvailabilityData{
			LinkedResourceID: hostID,
			Address:          "203.0.113.11",
			Protocol:         "tcp",
			Enabled:          true,
			Available:        true,
		}),
		availabilityProbeRecord("probe-public-api", "198.51.100.20", nil),
		availabilityProbeRecord("probe-router", "198.51.100.21", nil),
	})

	host, ok := rr.Get(hostID)
	if !ok || host == nil || host.Availability == nil {
		t.Fatalf("expected availability summary on host, got %+v", host)
	}
	checks := AvailabilityChecksForResource(*host)
	if len(checks) != 2 {
		t.Fatalf("availability checks = %+v, want both attached checks", checks)
	}
	targets := map[string]bool{}
	for _, check := range checks {
		targets[check.TargetID] = true
	}
	if !targets["probe-a"] || !targets["probe-b"] {
		t.Fatalf("availability targets = %+v, want probe-a and probe-b", targets)
	}
	endpoints := rr.ListByType(ResourceTypeNetworkEndpoint)
	if len(endpoints) != 4 {
		t.Fatalf("availability endpoint count = %d, want all 4 configured checks", len(endpoints))
	}
	endpointTargets := map[string]bool{}
	for _, endpoint := range endpoints {
		if endpoint.Availability == nil {
			t.Fatalf("endpoint %q missing availability facet", endpoint.ID)
		}
		endpointTargets[endpoint.Availability.TargetID] = true
	}
	for _, targetID := range []string{"probe-a", "probe-b", "probe-public-api", "probe-router"} {
		if !endpointTargets[targetID] {
			t.Fatalf("availability endpoint targets = %+v, missing %q", endpointTargets, targetID)
		}
	}
	stats := rr.Stats()
	if stats.ByType[ResourceTypeNetworkEndpoint] != 4 {
		t.Fatalf("network endpoint stats = %d, want 4", stats.ByType[ResourceTypeNetworkEndpoint])
	}
	checkRelationships := 0
	for _, endpoint := range endpoints {
		for _, relationship := range endpoint.Relationships {
			if relationship.Type == RelChecks {
				checkRelationships++
			}
		}
	}
	if checkRelationships != 2 {
		t.Fatalf("checks relationships = %d, want 2", checkRelationships)
	}
}

func TestAvailabilitySummaryPrefersConfirmedOutageOverEarlierUnconfirmedFailure(t *testing.T) {
	rr := NewRegistry(nil)
	hostID := ingestAgentFixture(t, rr, "host-1", "machine-1")
	now := time.Now().UTC()
	check := func(targetID, address string, facet AvailabilityData) IngestRecord {
		checkedAt := now
		facet.LinkedResourceID = hostID
		facet.Address = address
		facet.Protocol = "https"
		facet.Enabled = true
		facet.LastChecked = &checkedAt
		facet.FailureThreshold = 3
		return availabilityProbeRecord(targetID, address, &facet)
	}

	// probe-a sorts first by target id and has failed once, below its
	// threshold. probe-b is past its threshold and owns the outage incident.
	unconfirmed := check("probe-a", "203.0.113.10", AvailabilityData{
		AggregateState:         "unavailable",
		ConsecutiveFailures:    1,
		ApplicationFailureCode: "status_mismatch",
	})
	unconfirmed.Resource.Status = StatusWarning
	confirmed := check("probe-b", "203.0.113.11", AvailabilityData{
		AggregateState:      "unavailable",
		ConsecutiveFailures: 3,
	})
	confirmed.Resource.Status = StatusOffline
	confirmed.Resource.Incidents = []ResourceIncident{{
		Provider: string(SourceAvailability),
		NativeID: "probe-b",
		Code:     "availability_unreachable",
	}}
	rr.IngestRecords(SourceAvailability, []IngestRecord{unconfirmed, confirmed})

	host, ok := rr.Get(hostID)
	if !ok || host == nil {
		t.Fatalf("host %q missing after ingest", hostID)
	}
	if len(AvailabilityChecksForResource(*host)) != 2 {
		t.Fatalf("availability checks = %+v, want both attached checks", host.AvailabilityChecks)
	}
	if host.Availability == nil || host.Availability.TargetID != "probe-b" {
		t.Fatalf("availability summary = %+v, want the confirmed outage on probe-b", host.Availability)
	}
	health := EvaluateResourceHealth(*host, nil, now)
	if health.Verdict != HealthCritical || len(health.Reasons) == 0 ||
		health.Reasons[0].Code != "availability_failed" || health.Reasons[0].Detail != "" {
		t.Fatalf("host health = %+v, want availability_failed from probe-b", health)
	}

	// Once probe-b recovers, the unconfirmed failure is the worst check left:
	// it is the summary again, and it does not make the host an outage.
	rr.IngestRecords(SourceAvailability, []IngestRecord{
		check("probe-b", "203.0.113.11", AvailabilityData{
			AggregateState: "healthy",
			Available:      true,
		}),
	})
	host, _ = rr.Get(hostID)
	if host.Availability == nil || host.Availability.TargetID != "probe-a" {
		t.Fatalf("availability summary = %+v, want the remaining failure on probe-a", host.Availability)
	}
	health = EvaluateResourceHealth(*host, nil, now)
	for _, reason := range health.Reasons {
		if reason.Code == "availability_failed" {
			t.Fatalf("host health = %+v, want no outage below the failure threshold", health)
		}
	}
}

func TestAvailabilitySummaryDoesNotConfirmOutageFromSilentProbeAgent(t *testing.T) {
	rr := NewRegistry(nil)
	hostID := ingestAgentFixture(t, rr, "host-1", "machine-1")
	now := time.Now().UTC()
	checkedAt := now
	unconfirmed := availabilityProbeRecord("probe-a", "203.0.113.10", &AvailabilityData{
		LinkedResourceID:    hostID,
		Address:             "203.0.113.10",
		Protocol:            "https",
		Enabled:             true,
		AggregateState:      "unavailable",
		ProbeOutcome:        "unreachable",
		LastChecked:         &checkedAt,
		ConsecutiveFailures: 1,
		FailureThreshold:    3,
	})
	unconfirmed.Resource.Status = StatusWarning

	// The poller's read of a single-location probe whose agent stopped
	// reporting: the old failure count survives, but the outcome is
	// indeterminate, the aggregate unknown, and no incident is raised.
	lastReport := now.Add(-time.Hour)
	silent := availabilityProbeRecord("probe-b", "203.0.113.11", &AvailabilityData{
		LinkedResourceID:    hostID,
		Address:             "203.0.113.11",
		Protocol:            "https",
		ProbeAgentID:        "agent-probe-1",
		Enabled:             true,
		AggregateState:      "unknown",
		ProbeOutcome:        "indeterminate",
		ExpectedLocations:   1,
		LastChecked:         &lastReport,
		ConsecutiveFailures: 5,
		FailureThreshold:    3,
		LastError:           "no recent report from probe agent",
	})
	silent.Resource.Status = StatusWarning
	rr.IngestRecords(SourceAvailability, []IngestRecord{unconfirmed, silent})

	host, ok := rr.Get(hostID)
	if !ok || host == nil {
		t.Fatalf("host %q missing after ingest", hostID)
	}
	if host.Availability == nil || host.Availability.TargetID != "probe-a" {
		t.Fatalf("availability summary = %+v, want probe-a: a silent probe confirms no outage", host.Availability)
	}
	for _, resource := range []Resource{*host, availabilityEndpointByTarget(t, rr, "probe-b")} {
		health := EvaluateResourceHealth(resource, nil, now)
		for _, reason := range health.Reasons {
			if reason.Code == "availability_failed" {
				t.Fatalf("%s health = %+v, want no outage from a silent probe agent", resource.ID, health)
			}
		}
	}
}

func TestAvailabilityEditReplacesEndpointAndMovesProjection(t *testing.T) {
	rr := NewRegistry(nil)
	hostA := ingestAgentFixture(t, rr, "host-a", "machine-a")
	now := time.Now().UTC()
	rr.IngestRecords(SourceAgent, []IngestRecord{{
		SourceID: "host-b",
		Resource: Resource{
			Type:     ResourceTypeAgent,
			Name:     "host-b",
			Status:   StatusOnline,
			LastSeen: now,
		},
		Identity: ResourceIdentity{MachineID: "machine-b"},
	}})
	var hostB string
	for _, host := range rr.ListByType(ResourceTypeAgent) {
		if host.ID != hostA {
			hostB = host.ID
		}
	}
	if hostB == "" {
		t.Fatal("second host missing")
	}

	failed := availabilityProbeRecord("probe-edit", "192.0.2.50", &AvailabilityData{
		LinkedResourceID: hostA,
		Address:          "192.0.2.50",
		Protocol:         "http",
		Enabled:          true,
		Available:        false,
	})
	failed.Resource.Status = StatusOffline
	failed.Resource.Incidents = []ResourceIncident{{
		Provider: string(SourceAvailability),
		NativeID: "probe-edit",
		Code:     "availability_unreachable",
	}}
	rr.IngestRecords(SourceAvailability, []IngestRecord{failed})

	recovered := availabilityProbeRecord("probe-edit", "192.0.2.51", &AvailabilityData{
		LinkedResourceID: hostB,
		Address:          "192.0.2.51",
		Protocol:         "https",
		Enabled:          true,
		Available:        true,
	})
	rr.IngestRecords(SourceAvailability, []IngestRecord{recovered})

	oldHost, _ := rr.Get(hostA)
	if len(AvailabilityChecksForResource(*oldHost)) != 0 || hasDataSource(oldHost.Sources, SourceAvailability) {
		t.Fatalf("old host retained moved projection: %+v", oldHost)
	}
	newHost, _ := rr.Get(hostB)
	if checks := AvailabilityChecksForResource(*newHost); len(checks) != 1 ||
		checks[0].Address != "192.0.2.51" {
		t.Fatalf("new host projection = %+v, want edited endpoint", checks)
	}
	check := availabilityEndpointByTarget(t, rr, "probe-edit")
	if check.Status != StatusOnline || len(check.Incidents) != 0 {
		t.Fatalf("edited check retained failed state: status=%q incidents=%+v", check.Status, check.Incidents)
	}
	if check.Availability.Address != "192.0.2.51" || check.Availability.Protocol != "https" {
		t.Fatalf("edited check = %+v, want replacement endpoint", check.Availability)
	}
}

func TestAvailabilityRehydrateKeepsCheckIdentitySeparateFromProjection(t *testing.T) {
	rr := NewRegistry(nil)
	hostID := ingestAgentFixture(t, rr, "host-1", "machine-1")
	rr.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-restart", "192.0.2.60", &AvailabilityData{
			LinkedResourceID: hostID,
			Address:          "192.0.2.60",
			Protocol:         "tcp",
			Enabled:          true,
			Available:        true,
		}),
	})
	checkBefore := availabilityEndpointByTarget(t, rr, "probe-restart")

	restarted := NewRegistry(nil)
	restarted.IngestResources(rr.List())
	restarted.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-restart", "192.0.2.60", &AvailabilityData{
			LinkedResourceID: hostID,
			Address:          "192.0.2.60",
			Protocol:         "tcp",
			Enabled:          true,
			Available:        true,
		}),
	})

	checkAfter := availabilityEndpointByTarget(t, restarted, "probe-restart")
	if checkAfter.ID != checkBefore.ID {
		t.Fatalf("check ID changed across rehydrate: %q -> %q", checkBefore.ID, checkAfter.ID)
	}
	host, _ := restarted.Get(hostID)
	if checks := AvailabilityChecksForResource(*host); len(checks) != 1 ||
		checks[0].TargetID != "probe-restart" {
		t.Fatalf("host projection after rehydrate = %+v", checks)
	}
}

func TestAvailabilityManualIdentityLinkCannotEraseConfiguredCheck(t *testing.T) {
	initial := NewRegistry(nil)
	hostID := ingestAgentFixture(t, initial, "host-1", "machine-1")
	initial.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-linked", "192.0.2.80", &AvailabilityData{
			LinkedResourceID: hostID,
			Address:          "192.0.2.80",
			Protocol:         "https",
			Enabled:          true,
			Available:        true,
		}),
	})
	checkID := availabilityEndpointByTarget(t, initial, "probe-linked").ID

	store := NewMemoryStore()
	if err := store.AddLink(ResourceLink{
		ResourceA: checkID,
		ResourceB: hostID,
		PrimaryID: hostID,
	}); err != nil {
		t.Fatalf("AddLink(): %v", err)
	}
	rehydrated := NewRegistry(store)
	rehydrated.IngestResources(initial.List())

	if _, ok := rehydrated.Get(checkID); !ok {
		t.Fatalf("manual link erased configured check %q", checkID)
	}
	if _, ok := rehydrated.Get(hostID); !ok {
		t.Fatalf("manual link erased monitored host %q", hostID)
	}
	if got := len(rehydrated.ListByType(ResourceTypeNetworkEndpoint)); got != 1 {
		t.Fatalf("availability check count = %d, want 1", got)
	}
}

func TestAvailabilityIdentityRemainsTenantLocal(t *testing.T) {
	buildTenant := func(machineID string) (*ResourceRegistry, string) {
		rr := NewRegistry(nil)
		hostID := ingestAgentFixture(t, rr, "shared-host", machineID)
		rr.IngestRecords(SourceAvailability, []IngestRecord{
			availabilityProbeRecord("shared-check", "192.0.2.90", &AvailabilityData{
				LinkedResourceID: hostID,
				Address:          "192.0.2.90",
				Protocol:         "tcp",
				Enabled:          true,
				Available:        true,
			}),
		})
		return rr, hostID
	}

	tenantA, hostA := buildTenant("tenant-a-machine")
	tenantB, hostB := buildTenant("tenant-b-machine")
	checkA := availabilityEndpointByTarget(t, tenantA, "shared-check")
	checkB := availabilityEndpointByTarget(t, tenantB, "shared-check")
	if checkA.ID != checkB.ID {
		t.Fatalf("tenant-local source identity changed for same target: %q vs %q", checkA.ID, checkB.ID)
	}
	if hostA == hostB {
		t.Fatalf("tenant fixture hosts unexpectedly share canonical ID %q", hostA)
	}
	if checkA.Relationships[0].TargetID != hostA || checkB.Relationships[0].TargetID != hostB {
		t.Fatalf(
			"cross-tenant projection: tenant A=%+v tenant B=%+v",
			checkA.Relationships,
			checkB.Relationships,
		)
	}
}

// TestAvailabilityRepeatedProjectionKeepsHostRelationshipsStable pins the
// registry side of the relationship-aliasing fix. The projection scrub filters
// each resource's `checks` edges before re-projecting; when it compacted the
// slice in place, a resource whose relationships still shared a backing array
// with another copy ended up with a duplicated trailing edge. Re-ingesting the
// same check repeatedly is the ordinary path (every registry rebuild replays
// availability records), so the edge set must stay stable.
func TestAvailabilityRepeatedProjectionKeepsHostRelationshipsStable(t *testing.T) {
	rr := NewRegistry(nil)
	hostID := ingestAgentFixture(t, rr, "host-1", "machine-1")

	var checkEdges int
	for round := range 4 {
		observedAt := time.Now().UTC()
		rr.IngestRecords(SourceAvailability, []IngestRecord{
			availabilityProbeRecord("probe-1", "192.0.2.10", &AvailabilityData{
				LinkedResourceID: hostID,
				Address:          "192.0.2.10",
				Protocol:         "icmp",
				Enabled:          true,
				Available:        true,
				LastChecked:      &observedAt,
				Evidence:         availabilityProbeEvidence(t, "probe-1", observedAt),
			}),
		})

		host, ok := rr.Get(hostID)
		if !ok || host == nil {
			t.Fatalf("host %q missing after round %d", hostID, round)
		}

		round0Edges := 0
		for _, relationship := range host.Relationships {
			if relationship.Type == RelChecks {
				round0Edges++
			}
		}
		if round == 0 {
			checkEdges = round0Edges
			continue
		}
		if round0Edges != checkEdges {
			t.Fatalf("round %d: checks edges = %d, want %d stable across re-projection",
				round, round0Edges, checkEdges)
		}
	}
}

// An explicit availability link saved under the guest's retired node-scoped
// canonical ID (or its raw node-scoped source ID) must keep resolving after
// the guest live-migrates (#1669). The link is fail-closed, so only
// provider-declared persistence keys resolve.
func TestProxmoxGuestAvailabilityLinkFollowsRetiredIDs(t *testing.T) {
	now := time.Now().UTC()

	refs := map[string]string{
		"retired canonical id": SourceSpecificID(ResourceTypeVM, SourceProxmox, "delly:pve1:100"),
		"old-node source id":   "delly:pve1:100",
	}
	for name, ref := range refs {
		t.Run(name, func(t *testing.T) {
			rr := NewRegistry(nil)
			rr.IngestSnapshot(proxmoxGuestMigrationSnapshot(now, "pve2"))

			rr.IngestRecords(SourceAvailability, []IngestRecord{
				availabilityProbeRecord("probe-guest", "192.0.2.50", &AvailabilityData{
					LinkedResourceID: ref,
					Address:          "192.0.2.50",
					Protocol:         "icmp",
					Enabled:          true,
					Available:        true,
					LastChecked:      &now,
					Evidence:         availabilityProbeEvidence(t, "probe-guest", now),
				}),
			})

			guestID := ProxmoxGuestCanonicalID(ResourceTypeVM, "delly", 100)
			guest, ok := rr.Get(guestID)
			if !ok || guest == nil {
				t.Fatalf("guest %q missing", guestID)
			}
			if guest.Availability == nil || guest.Availability.CorrelationState != AvailabilityCorrelationAttached ||
				guest.Availability.CorrelationRule != "explicit_resource_link" {
				t.Fatalf("availability facet = %+v, want attached explicit link for ref %q", guest.Availability, ref)
			}
		})
	}
}

// assertAvailabilityCheckAttachedTo checks that the configured check targetID
// is projected onto resourceID by its explicit link and that the check's own
// endpoint row points there.
func assertAvailabilityCheckAttachedTo(t *testing.T, stage string, rr *ResourceRegistry, targetID, resourceID string) {
	t.Helper()
	target, ok := rr.Get(resourceID)
	if !ok {
		t.Fatalf("%s: check target %s missing from %v", stage, resourceID, resourceIDs(rr.List()))
	}
	attached := false
	for _, check := range AvailabilityChecksForResource(*target) {
		if check.TargetID == targetID {
			attached = check.CorrelationState == AvailabilityCorrelationAttached && check.CorrelationRule == "explicit_resource_link"
		}
	}
	if !attached {
		t.Fatalf("%s: %s checks = %+v, want %s attached by its explicit link", stage, resourceID, AvailabilityChecksForResource(*target), targetID)
	}
	endpoint := availabilityEndpointByTarget(t, rr, targetID)
	for _, relationship := range endpoint.Relationships {
		if relationship.Type == RelChecks && relationship.TargetID == resourceID {
			return
		}
	}
	t.Fatalf("%s: endpoint relationships = %+v, want a checks edge to %s", stage, endpoint.Relationships, resourceID)
}

func linkedGuestVMRecord(now time.Time) IngestRecord {
	return IngestRecord{
		SourceID: "vc-1:vm:vm-42",
		Resource: Resource{
			Type:       ResourceTypeVM,
			Technology: "vmware",
			Name:       "app-guest",
			Status:     StatusOnline,
			LastSeen:   now,
			VMware:     &VMwareData{ConnectionID: "vc-1", ManagedObjectID: "vm-42", EntityType: "vm"},
		},
		Identity: ResourceIdentity{Hostnames: []string{"app-guest"}},
	}
}

func linkedGuestCheck(t *testing.T, targetID, address, linkedResourceID string, now time.Time) IngestRecord {
	t.Helper()
	return availabilityProbeRecord(targetID, address, &AvailabilityData{
		LinkedResourceID: linkedResourceID,
		Address:          address,
		Protocol:         "icmp",
		Enabled:          true,
		Available:        true,
		LastChecked:      &now,
		Evidence:         availabilityProbeEvidence(t, targetID, now),
	})
}

// An operator link folds the agent inside a vSphere VM into the VM, so the
// agent's own canonical ID no longer names a row. A check configured against
// that ID watches the same machine and follows the agent to the merged row in
// every registry that lists it: the monitor's rebuild, a registry seeded from
// the monitor's listing that replays the checks (the resources API) and a
// read-state overlay. Reference reads (alert intent, operator state, API
// lookups) resolve the folded ID the same way. History does not, and the
// folded ID never becomes a superseded ID, which alert and availability
// migrations would turn into a permanent rewrite of the operator's config.
func TestAvailabilityLinkToALinkFoldedResourceFollowsItsPrimary(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		Hosts:      []models.Host{{ID: "host-app-guest", Hostname: "app-guest", MachineID: "machine-app-guest", Status: "online", LastSeen: now}},
		LastUpdate: now,
	}
	agentID := MachineIdentityCanonicalID(ResourceTypeAgent, "machine-app-guest")
	checks := []IngestRecord{linkedGuestCheck(t, "probe-guest", "192.0.2.60", agentID, now)}
	records := map[DataSource][]IngestRecord{
		SourceVMware:       {linkedGuestVMRecord(now)},
		SourceAvailability: checks,
	}

	for _, agentPrimary := range []bool{false, true} {
		name := "vm-primary"
		if agentPrimary {
			name = "agent-primary"
		}
		t.Run(name, func(t *testing.T) {
			store := NewMemoryStore()
			adapter := NewMonitorAdapter(NewRegistry(store))
			adapter.PopulateSnapshotAndSupplemental(snapshot, records)
			assertAvailabilityCheckAttachedTo(t, "unlinked", adapter.currentRegistry(), "probe-guest", agentID)

			var vmID string
			for _, resource := range adapter.GetAll() {
				if resource.VMware != nil {
					vmID = resource.ID
				}
			}
			if vmID == "" {
				t.Fatalf("unlinked estate = %v, want the vSphere VM", resourceIDs(adapter.GetAll()))
			}
			primaryID := vmID
			if agentPrimary {
				primaryID = agentID
			}
			if err := store.AddLink(ResourceLink{ResourceA: vmID, ResourceB: agentID, PrimaryID: primaryID}); err != nil {
				t.Fatalf("add link: %v", err)
			}

			// Pins persist on the first rebuild; damage from them shows on the second.
			for i, stage := range []string{"rebuild", "second rebuild"} {
				next := snapshot
				next.LastUpdate = now.Add(time.Duration(i+1) * time.Second)
				adapter.PopulateSnapshotAndSupplemental(next, records)
				if _, listed := adapter.currentRegistry().Get(agentID); listed {
					t.Fatalf("%s: agent %s still listed beside the VM it is linked into", stage, agentID)
				}

				rest := NewRegistry(store)
				rest.IngestResources(adapter.GetAll())
				rest.IngestRecords(SourceAvailability, checks)
				overlay, ok := ReadStateWithRecords(adapter, SourceAvailability, checks).(*MonitorAdapter)
				if !ok {
					t.Fatalf("%s: overlay is not a monitor adapter", stage)
				}

				for _, view := range []struct {
					name string
					rr   *ResourceRegistry
				}{
					{"monitor", adapter.currentRegistry()},
					{"resources API", rest},
					{"overlay", overlay.currentRegistry()},
				} {
					label := stage + ", " + view.name
					assertAvailabilityCheckAttachedTo(t, label, view.rr, "probe-guest", vmID)
					// The agent's alert reference already reached the merged
					// row; its canonical ID must agree.
					for _, ref := range []string{agentID, "agent:host-app-guest"} {
						if got, ok := view.rr.ResolveReferenceID(ref); !ok || got != vmID {
							t.Fatalf("%s: ResolveReferenceID(%s) = %q, %t, want the VM %s", label, ref, got, ok, vmID)
						}
					}
					// History bindings persist and join both journals in either
					// direction, so they would outlive the link.
					if got, claimed := view.rr.resolveHistoryReference(agentID); got != "" || claimed {
						t.Fatalf("%s: history reference %s resolved to %q (claimed %t), want it left to its own journal", label, agentID, got, claimed)
					}
					vm, _ := view.rr.Get(vmID)
					if slices.Contains(vm.SupersededCanonicalIDs, agentID) ||
						(vm.Canonical != nil && slices.Contains(vm.Canonical.SupersededIDs, agentID)) {
						t.Fatalf("%s: folded agent %s is listed as superseded by the VM", label, agentID)
					}
				}
			}
		})
	}
}

// An old enrollment of a guest linked into its current agent, and that agent
// linked into its VM: every ID folded along the chain resolves to the final
// primary, in the monitor and in a registry seeded from it.
func TestAvailabilityLinkFollowsChainedLinkFolds(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "host-app-guest", Hostname: "app-guest", MachineID: "machine-app-guest", Status: "online", LastSeen: now},
			{ID: "host-app-guest-old", Hostname: "app-guest-old", MachineID: "machine-app-guest-old", Status: "online", LastSeen: now},
		},
		LastUpdate: now,
	}
	agentID := MachineIdentityCanonicalID(ResourceTypeAgent, "machine-app-guest")
	oldAgentID := MachineIdentityCanonicalID(ResourceTypeAgent, "machine-app-guest-old")
	checks := []IngestRecord{linkedGuestCheck(t, "probe-old", "192.0.2.61", oldAgentID, now)}
	records := map[DataSource][]IngestRecord{
		SourceVMware:       {linkedGuestVMRecord(now)},
		SourceAvailability: checks,
	}

	unlinked := NewMonitorAdapter(NewRegistry(nil))
	unlinked.PopulateSnapshotAndSupplemental(snapshot, records)
	var vmID string
	for _, resource := range unlinked.GetAll() {
		if resource.VMware != nil {
			vmID = resource.ID
		}
	}
	listed := resourceIDs(unlinked.GetAll())
	if vmID == "" || !slices.Contains(listed, agentID) || !slices.Contains(listed, oldAgentID) {
		t.Fatalf("unlinked estate = %v, want the VM and both agents", listed)
	}

	store := NewMemoryStore()
	for _, link := range []ResourceLink{
		{ResourceA: agentID, ResourceB: oldAgentID, PrimaryID: agentID},
		{ResourceA: vmID, ResourceB: agentID, PrimaryID: vmID},
	} {
		if err := store.AddLink(link); err != nil {
			t.Fatalf("add link: %v", err)
		}
	}
	adapter := NewMonitorAdapter(NewRegistry(store))
	adapter.PopulateSnapshotAndSupplemental(snapshot, records)
	listed = resourceIDs(adapter.GetAll())
	if slices.Contains(listed, agentID) || slices.Contains(listed, oldAgentID) {
		t.Fatalf("linked estate = %v, want both agents folded into the VM %s", listed, vmID)
	}

	rest := NewRegistry(store)
	rest.IngestResources(adapter.GetAll())
	rest.IngestRecords(SourceAvailability, checks)
	for _, view := range []struct {
		name string
		rr   *ResourceRegistry
	}{{"monitor", adapter.currentRegistry()}, {"resources API", rest}} {
		assertAvailabilityCheckAttachedTo(t, view.name, view.rr, "probe-old", vmID)
		for _, folded := range []string{agentID, oldAgentID} {
			if got, ok := view.rr.ResolveReferenceID(folded); !ok || got != vmID {
				t.Fatalf("%s: ResolveReferenceID(%s) = %q, %t, want the VM %s", view.name, folded, got, ok, vmID)
			}
		}
	}
}

// The fold index follows the rows holding each folded ID. A seeded listing can
// carry one folded ID on two rows, or on a row while a live row has that ID. A
// live row answers for itself; two holders resolve to nothing, without falling
// through to weaker matches such as a hostname alias, so an explicit check link
// stays fail-closed. Holders a link later merges, and a holder re-keyed in
// place, keep resolving.
func TestLinkFoldIndexFollowsItsHolders(t *testing.T) {
	now := time.Now().UTC()
	vm := func(id string, folded ...string) Resource {
		return Resource{
			ID: id, Type: ResourceTypeVM, Name: id, Status: StatusOnline, LastSeen: now,
			Sources: []DataSource{SourceVMware}, linkFoldedIDs: folded,
		}
	}

	t.Run("ambiguous holders fail closed", func(t *testing.T) {
		rr := NewRegistry(nil)
		rr.IngestResources([]Resource{
			vm("vm-a", "agent-shared", "agent-live"),
			vm("vm-b", "agent-shared"),
			{ID: "agent-live", Type: ResourceTypeAgent, Name: "agent-live", Status: StatusOnline, LastSeen: now, Sources: []DataSource{SourceAgent}},
			{
				ID: "vm-c", Type: ResourceTypeVM, Name: "vm-c", Status: StatusOnline, LastSeen: now,
				Sources: []DataSource{SourceVMware}, Identity: ResourceIdentity{Hostnames: []string{"agent-shared"}},
			},
		})
		if got, ok := rr.ResolveReferenceID("agent-shared"); ok {
			t.Fatalf("ResolveReferenceID(agent-shared) = %q, want no answer while two rows hold it", got)
		}
		if got, ok := rr.ResolveReferenceID("agent-live"); !ok || got != "agent-live" {
			t.Fatalf("ResolveReferenceID(agent-live) = %q, %t, want the live row itself", got, ok)
		}
		rr.IngestRecords(SourceAvailability, []IngestRecord{linkedGuestCheck(t, "probe-shared", "192.0.2.62", "agent-shared", now)})
		endpoint := availabilityEndpointByTarget(t, rr, "probe-shared")
		if endpoint.Availability.CorrelationState != AvailabilityCorrelationUnresolved {
			t.Fatalf("ambiguous folded link correlation = %+v, want unresolved", endpoint.Availability)
		}
	})

	t.Run("holders merged by a link resolve to the survivor", func(t *testing.T) {
		store := NewMemoryStore()
		if err := store.AddLink(ResourceLink{ResourceA: "vm-a", ResourceB: "vm-b", PrimaryID: "vm-a"}); err != nil {
			t.Fatalf("add link: %v", err)
		}
		rr := NewRegistry(store)
		rr.IngestResources([]Resource{vm("vm-a", "agent-shared"), vm("vm-b", "agent-shared")})
		for _, ref := range []string{"agent-shared", "vm-b"} {
			if got, ok := rr.ResolveReferenceID(ref); !ok || got != "vm-a" {
				t.Fatalf("ResolveReferenceID(%s) = %q, %t, want vm-a", ref, got, ok)
			}
		}
	})

	t.Run("re-keyed holder keeps its folds", func(t *testing.T) {
		rr := NewRegistry(nil)
		disk := vm("disk-old", "disk-folded")
		disk.Type = ResourceTypePhysicalDisk
		rr.IngestResources([]Resource{disk})
		rr.mu.Lock()
		rr.rekeyPhysicalDiskLocked(rr.resources["disk-old"], "disk-new")
		rr.mu.Unlock()
		if got, ok := rr.ResolveReferenceID("disk-folded"); !ok || got != "disk-new" {
			t.Fatalf("ResolveReferenceID(disk-folded) = %q, %t, want the re-keyed holder disk-new", got, ok)
		}
	})
}
