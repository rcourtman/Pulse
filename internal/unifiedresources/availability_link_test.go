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

func TestAvailabilityExplicitLinkAttachesFacetToKnownResource(t *testing.T) {
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

	if got := rr.ListByType(ResourceTypeNetworkEndpoint); len(got) != 0 {
		t.Fatalf("expected 0 standalone network endpoints, got %d", len(got))
	}
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
	foundChecksRelationship := false
	for _, relationship := range host.Relationships {
		if relationship.Type == RelChecks &&
			relationship.TargetID == hostID &&
			relationship.Metadata["targetId"] == "probe-1" {
			if relationship.ID == "" {
				t.Fatal("availability relationship is missing its stable ID")
			}
			if relationship.EvidenceID != host.Availability.Evidence.ID {
				t.Fatalf(
					"availability relationship evidence = %q, want %q",
					relationship.EvidenceID,
					host.Availability.Evidence.ID,
				)
			}
			foundChecksRelationship = true
		}
	}
	if !foundChecksRelationship {
		t.Fatalf("relationships = %+v, want availability checks edge", host.Relationships)
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

	if got := rr.ListByType(ResourceTypeNetworkEndpoint); len(got) != 0 {
		t.Fatalf("expected probe to attach (0 endpoints), got %d", len(got))
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

	if got := rr.ListByType(ResourceTypeNetworkEndpoint); len(got) != 0 {
		t.Fatalf("expected hostname probe to attach, got %d standalone endpoints", len(got))
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

func TestAvailabilityKeepsMultipleChecksOnOneCanonicalResource(t *testing.T) {
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
	// A second probe explicitly linked to the same host belongs to the same
	// canonical resource. The singular facet stays as a compatibility summary
	// while the plural facet and relationships retain both checks.
	rr.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-b", "203.0.113.11", &AvailabilityData{
			LinkedResourceID: hostID,
			Address:          "203.0.113.11",
			Protocol:         "tcp",
			Enabled:          true,
			Available:        true,
		}),
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
	if got := rr.ListByType(ResourceTypeNetworkEndpoint); len(got) != 0 {
		t.Fatalf("expected both probes to attach (0 endpoints), got %d", len(got))
	}
	checkRelationships := 0
	for _, relationship := range host.Relationships {
		if relationship.Type == RelChecks {
			checkRelationships++
		}
	}
	if checkRelationships != 2 {
		t.Fatalf("checks relationships = %d, want 2: %+v", checkRelationships, host.Relationships)
	}
}
