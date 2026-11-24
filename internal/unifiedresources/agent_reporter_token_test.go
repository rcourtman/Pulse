package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func agentRowsByAgentID(t *testing.T, registry *ResourceRegistry) map[string]Resource {
	t.Helper()
	rows := make(map[string]Resource)
	for _, resource := range registry.List() {
		if resource.Type != ResourceTypeAgent || resource.Agent == nil {
			continue
		}
		if _, dup := rows[resource.Agent.AgentID]; dup {
			t.Fatalf("agent %q is on two rows", resource.Agent.AgentID)
		}
		rows[resource.Agent.AgentID] = resource
	}
	return rows
}

// A host agent's hostname and MAC address are its own claims. A report under
// one API token that repeats another host's must not land on the row of the
// agent enrolled under a different token: host actions are dispatched to the
// agent the row names.
func TestResourceRegistry_AgentUnderAnotherTokenDoesNotJoinHostRow(t *testing.T) {
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	nic := []models.HostNetworkInterface{{Name: "eth0", MAC: "52:54:00:00:00:0b"}}
	web01 := models.Host{
		ID: "agent-web01", Hostname: "web01", DisplayName: "web01", MachineID: "machine-web01",
		TokenID: "token-web01", NetworkInterfaces: nic, Status: "online", LastSeen: now,
	}
	other := models.Host{
		ID: "agent-other", Hostname: "web01.zz", DisplayName: "web01", MachineID: "machine-other",
		TokenID: "token-other", NetworkInterfaces: nic, Status: "online", LastSeen: now,
	}
	stopped := other
	stopped.Status = "offline"

	web01ID := MachineIdentityCanonicalID(ResourceTypeAgent, web01.MachineID)
	for name, hosts := range map[string][]models.Host{
		"other host reports last":      {web01, other},
		"other host reports first":     {other, web01},
		"other host stopped reporting": {web01, stopped},
	} {
		t.Run(name, func(t *testing.T) {
			registry := NewRegistry(nil)
			registry.IngestSnapshot(models.StateSnapshot{Hosts: hosts, LastUpdate: now})

			rows := agentRowsByAgentID(t, registry)
			if len(rows) != 2 {
				t.Fatalf("agent rows = %d, want one per token: %+v", len(rows), rows)
			}
			row, ok := rows[web01.ID]
			if !ok || row.ID != web01ID {
				t.Fatalf("web01 row = %q (found %v), want its machine identity %q", row.ID, ok, web01ID)
			}
			if row.Agent.TokenID != web01.TokenID || row.Agent.MachineID != web01.MachineID {
				t.Fatalf("web01 row carries another reporter's agent: %+v", row.Agent)
			}
			if got, want := rows[other.ID].ID, MachineIdentityCanonicalID(ResourceTypeAgent, other.MachineID); got != want {
				t.Fatalf("other host row = %q, want its own machine identity %q", got, want)
			}
		})
	}
}

// A machine ID is a claim too, and the ID derived from it is the row an
// operator approves actions for. While reports under two tokens give one
// machine ID neither holds that ID, whichever is ingested first: hosts arrive
// ordered by the hostname each reports.
func TestResourceRegistry_MachineIDUnderSeveralTokensKeysNoHostRow(t *testing.T) {
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	web01 := models.Host{
		ID: "agent-web01", Hostname: "web01", DisplayName: "web01", MachineID: "machine-web01",
		TokenID:           "token-web01",
		NetworkInterfaces: []models.HostNetworkInterface{{Name: "eth0", MAC: "52:54:00:00:00:0b"}},
		Status:            "online", LastSeen: now,
	}
	claimsMachineID := models.Host{
		ID: "agent-other", Hostname: "db02", DisplayName: "web01", MachineID: web01.MachineID,
		TokenID:           "token-other",
		NetworkInterfaces: []models.HostNetworkInterface{{Name: "eth0", MAC: "52:54:00:00:00:0a"}},
		Status:            "online", LastSeen: now,
	}
	// A report with no machine ID takes one from the pin of the host whose
	// hostname it gives.
	claimsPinnedHostname := claimsMachineID
	claimsPinnedHostname.Hostname, claimsPinnedHostname.MachineID = "web01.zz", ""

	web01ID := MachineIdentityCanonicalID(ResourceTypeAgent, web01.MachineID)
	for name, other := range map[string]models.Host{
		"machine ID":      claimsMachineID,
		"pinned hostname": claimsPinnedHostname,
	} {
		for order, hosts := range map[string][]models.Host{
			"other host first": {other, web01},
			"other host last":  {web01, other},
		} {
			t.Run(name+"/"+order, func(t *testing.T) {
				store := NewMemoryStore()
				alone := NewRegistry(store)
				alone.IngestSnapshot(models.StateSnapshot{Hosts: []models.Host{web01}, LastUpdate: now})
				alone.PersistIdentityPins()
				if rows := agentRowsByAgentID(t, alone); rows[web01.ID].ID != web01ID {
					t.Fatalf("web01 alone = %q, want its machine identity %q", rows[web01.ID].ID, web01ID)
				}

				registry := NewRegistry(store)
				registry.IngestSnapshot(models.StateSnapshot{Hosts: hosts, LastUpdate: now})
				registry.PersistIdentityPins()

				rows := agentRowsByAgentID(t, registry)
				if len(rows) != 2 {
					t.Fatalf("agent rows = %d, want one per token: %+v", len(rows), rows)
				}
				for _, host := range hosts {
					row := rows[host.ID]
					if want := SourceSpecificID(ResourceTypeAgent, SourceAgent, host.ID); row.ID != want {
						t.Fatalf("%s row = %q, want its source-specific ID %q", host.ID, row.ID, want)
					}
					if row.Agent.TokenID != host.TokenID {
						t.Fatalf("%s row carries another reporter's agent: %+v", host.ID, row.Agent)
					}
					if row.Identity.MachineID != "" {
						t.Fatalf("%s row is keyed by the disputed machine ID %q", host.ID, row.Identity.MachineID)
					}
				}
				if _, held := registry.Get(web01ID); held {
					t.Fatalf("a reporter holds the disputed machine identity %q", web01ID)
				}
				pins, err := store.ListResourceIdentityPins()
				if err != nil {
					t.Fatalf("list identity pins: %v", err)
				}
				if len(pins) != 1 || pins[0].CanonicalID != web01ID || pins[0].MachineID != web01.MachineID {
					t.Fatalf("pins = %+v, want only the one web01 held before the dispute", pins)
				}

				settled := NewRegistry(store)
				settled.IngestSnapshot(models.StateSnapshot{Hosts: []models.Host{web01}, LastUpdate: now})
				if rows := agentRowsByAgentID(t, settled); rows[web01.ID].ID != web01ID {
					t.Fatalf("web01 after the dispute = %q, want its machine identity %q", rows[web01.ID].ID, web01ID)
				}
			})
		}
	}
}

// Outside a snapshot the records arrive one at a time, so the reporter that
// holds a machine identity keeps it and a later one under another token takes
// its source-specific ID without the key.
func TestResourceRegistry_RecordUnderAnotherTokenDoesNotTakeHeldMachineIdentity(t *testing.T) {
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	web01 := models.Host{
		ID: "agent-web01", Hostname: "web01", MachineID: "machine-web01",
		TokenID: "token-web01", Status: "online", LastSeen: now,
	}
	other := models.Host{
		ID: "agent-other", Hostname: "db02", MachineID: web01.MachineID,
		TokenID: "token-other", Status: "online", LastSeen: now,
	}

	registry := NewRegistry(nil)
	registry.IngestRecords(SourceAgent, []IngestRecord{HostIngestRecord(web01), HostIngestRecord(other)})

	rows := agentRowsByAgentID(t, registry)
	if len(rows) != 2 {
		t.Fatalf("agent rows = %d, want one per token: %+v", len(rows), rows)
	}
	if got, want := rows[web01.ID].ID, MachineIdentityCanonicalID(ResourceTypeAgent, web01.MachineID); got != want {
		t.Fatalf("web01 row = %q, want its machine identity %q", got, want)
	}
	row := rows[other.ID]
	if want := SourceSpecificID(ResourceTypeAgent, SourceAgent, other.ID); row.ID != want || row.Identity.MachineID != "" {
		t.Fatalf("other host row = %q keyed by %q, want %q without the held machine ID", row.ID, row.Identity.MachineID, want)
	}
}

// One token reporting the same machine under two record IDs is one principal,
// and a Docker report joins the host agent's row whatever token it uses: the
// legacy Docker agent had a token of its own.
func TestResourceRegistry_ReporterTokenSplitLeavesOtherMergesAlone(t *testing.T) {
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	nic := []models.HostNetworkInterface{{Name: "eth0", MAC: "52:54:00:00:00:0b"}}
	first := models.Host{
		ID: "agent-web01", Hostname: "web01", MachineID: "machine-web01",
		TokenID: "token-web01", NetworkInterfaces: nic, Status: "online", LastSeen: now,
	}
	second := first
	second.ID = "agent-web01-reinstalled"

	registry := NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{first, second},
		DockerHosts: []models.DockerHost{{
			ID: "docker-web01", AgentID: "docker-web01", Hostname: "web01", MachineID: "machine-web01",
			TokenID: "token-docker", NetworkInterfaces: nic, Status: "online", LastSeen: now,
		}},
		LastUpdate: now,
	})

	var rows []Resource
	for _, resource := range registry.List() {
		if resource.Type == ResourceTypeAgent {
			rows = append(rows, resource)
		}
	}
	if len(rows) != 1 || rows[0].Agent == nil || rows[0].Docker == nil {
		t.Fatalf("host rows = %+v, want one row with the agent and Docker facets", rows)
	}
	if want := MachineIdentityCanonicalID(ResourceTypeAgent, first.MachineID); rows[0].ID != want {
		t.Fatalf("host row = %q, want its machine identity %q", rows[0].ID, want)
	}
}

func TestResourceRegistry_DockerHostUnderAnotherTokenDoesNotJoinHostRow(t *testing.T) {
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	nic := []models.HostNetworkInterface{{Name: "eth0", MAC: "52:54:00:00:00:0b"}}
	registry := NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{ID: "docker-web01", AgentID: "docker-web01", Hostname: "web01", MachineID: "machine-web01", TokenID: "token-web01", NetworkInterfaces: nic, Status: "online", LastSeen: now},
			{ID: "docker-other", AgentID: "docker-other", Hostname: "web01.zz", MachineID: "machine-other", TokenID: "token-other", NetworkInterfaces: nic, Status: "online", LastSeen: now},
		},
		LastUpdate: now,
	})

	rows := make(map[string]string)
	for _, resource := range registry.List() {
		if resource.Type == ResourceTypeAgent && resource.Docker != nil {
			rows[resource.Docker.HostSourceID] = resource.ID
		}
	}
	if len(rows) != 2 || rows["docker-web01"] != MachineIdentityCanonicalID(ResourceTypeAgent, "machine-web01") {
		t.Fatalf("Docker host rows = %+v, want one per token with docker-web01 on its machine identity", rows)
	}
}

// A saved enrollment only fills an absent machine and is never merged, so the
// retired record of a host that re-enrolled under a new token is still
// recognised as that host and withheld.
func TestResourceRegistry_SavedEnrollmentUnderRetiredTokenStaysWithheld(t *testing.T) {
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	live := models.Host{
		ID: "current-enrollment", Hostname: "web01", MachineID: "machine-web01",
		TokenID: "current-token", Status: "online", LastSeen: now,
	}
	retired := live
	retired.ID, retired.TokenID, retired.Status = "retired-enrollment", "retired-token", "offline"

	registry := NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{Hosts: []models.Host{live}, LastUpdate: now})
	registry.ingestRecords(SourceAgent, []IngestRecord{HostIngestRecord(retired)}, true, nil)

	rows := agentRowsByAgentID(t, registry)
	if len(rows) != 1 || rows[live.ID].Agent == nil {
		t.Fatalf("agent rows = %+v, want only the live enrollment", rows)
	}
}
