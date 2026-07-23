package unifiedresources

import (
	"testing"
	"time"

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
