package unifiedresources

import (
	"testing"
	"time"
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

func TestAvailabilityExplicitLinkAttachesFacetToKnownResource(t *testing.T) {
	rr := NewRegistry(nil)
	hostID := ingestAgentFixture(t, rr, "host-1", "machine-1")

	rr.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-1", "192.0.2.10", &AvailabilityData{
			LinkedResourceID: hostID,
			Address:          "192.0.2.10",
			Protocol:         "icmp",
			Enabled:          true,
			Available:        true,
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
}

func TestAvailabilityUnlinkedUnmatchedMintsNetworkEndpoint(t *testing.T) {
	rr := NewRegistry(nil)

	rr.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-orphan", "198.51.100.7", nil),
	})

	if got := rr.ListByType(ResourceTypeNetworkEndpoint); len(got) != 1 {
		t.Fatalf("expected 1 standalone network endpoint, got %d", len(got))
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
	}
}

func TestAvailabilityDoesNotOverwriteDifferentAttachedTarget(t *testing.T) {
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
	// A second probe explicitly linked to the same host must not overwrite the
	// first target's facet; it stays a standalone network-endpoint instead.
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
	if !ok || host == nil || host.Availability == nil || host.Availability.TargetID != "probe-a" {
		t.Fatalf("expected first probe probe-a to remain attached, got %+v", host)
	}
	if got := rr.ListByType(ResourceTypeNetworkEndpoint); len(got) != 1 {
		t.Fatalf("expected second probe to stay standalone (1 endpoint), got %d", len(got))
	}
}
