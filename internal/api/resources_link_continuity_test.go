package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// After a restart, an agent an operator linked into a live vSphere VM exists
// only through saved-host continuity until it reports again. The link names
// one identity, so the resources API stores operator state written through the
// agent's references on the VM and Patrol reads it there, on the same store.
// Neither the monitor's read state nor the registry the resources API seeds
// from it merges the saved agent into the VM: the agent keeps its own offline
// row and the VM keeps what vSphere reports.
func TestResourcesAPIAndPatrolAgreeOnContinuityAgentLinkedIntoGuest(t *testing.T) {
	now := time.Now().UTC()
	monitorCfg := &config.Config{DataPath: t.TempDir()}
	seedMonitor, err := monitoring.New(monitorCfg)
	if err != nil {
		t.Fatalf("monitoring.New seed monitor: %v", err)
	}
	t.Cleanup(seedMonitor.Stop)
	if _, err := seedMonitor.ApplyHostReport(agentshost.Report{
		Agent:     agentshost.AgentInfo{ID: "agent-app-guest", Version: "6.0.0-rc.1", IntervalSeconds: 30},
		Host:      agentshost.HostInfo{ID: "host-app-guest", MachineID: "machine-app-guest", Hostname: "app-guest", Platform: "linux"},
		Timestamp: now,
	}, &config.APITokenRecord{ID: "token-1", Name: "Token One"}); err != nil {
		t.Fatalf("ApplyHostReport seed continuity: %v", err)
	}
	monitor, err := monitoring.New(monitorCfg)
	if err != nil {
		t.Fatalf("monitoring.New restarted monitor: %v", err)
	}
	t.Cleanup(monitor.Stop)

	vmRecords := []unifiedresources.IngestRecord{{
		SourceID: "vc-1:vm:vm-42",
		Resource: unifiedresources.Resource{
			Type:       unifiedresources.ResourceTypeVM,
			Technology: "vmware",
			Name:       "app-guest",
			Status:     unifiedresources.StatusOnline,
			LastSeen:   now,
			VMware:     &unifiedresources.VMwareData{ConnectionID: "vc-1", ManagedObjectID: "vm-42", EntityType: "vm"},
		},
		Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"app-guest"}},
	}}
	ids := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	ids.PopulateSnapshotAndSupplemental(models.StateSnapshot{LastUpdate: now}, map[unifiedresources.DataSource][]unifiedresources.IngestRecord{
		unifiedresources.SourceVMware: vmRecords,
	})
	if len(ids.VMs()) != 1 {
		t.Fatalf("vSphere estate has %d VMs, want 1", len(ids.VMs()))
	}
	vmID := ids.VMs()[0].ID()
	agentID := unifiedresources.MachineIdentityCanonicalID(unifiedresources.ResourceTypeAgent, "machine-app-guest")

	handlers := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	t.Cleanup(func() { _ = handlers.CloseStores() })
	store, err := handlers.getStore("default")
	if err != nil {
		t.Fatalf("resource store: %v", err)
	}
	// Linked from the agent's page, so the agent is the stored primary; an
	// agent inside a guest still folds into the guest.
	if err := store.AddLink(unifiedresources.ResourceLink{ResourceA: agentID, ResourceB: vmID, PrimaryID: agentID}); err != nil {
		t.Fatalf("add link: %v", err)
	}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store)))
	vsphere := newTestSupplementalUsageProvider(unifiedresources.SourceVMware)
	vsphere.settleAtWithRecords(now, vmRecords)
	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceVMware, vsphere)
	seed := &countingUnifiedSeedProvider{listing: monitor.UnifiedResourceSnapshot}
	handlers.SetStateProvider(seed)
	router := &Router{monitor: monitor, resourceHandlers: handlers}

	listing, _ := monitor.UnifiedResourceSnapshot()
	readStateRows := map[string]unifiedresources.Resource{}
	for _, resource := range listing {
		readStateRows[resource.ID] = resource
	}
	if agent, ok := readStateRows[agentID]; !ok || agent.Status != unifiedresources.StatusOffline {
		t.Fatalf("read state agent row = %+v (listed=%v), want the saved agent's own offline row", agent, ok)
	}
	if guest, ok := readStateRows[vmID]; !ok || guest.Agent != nil {
		t.Fatalf("read state guest = %+v (listed=%v), want vSphere's VM without the saved agent", guest, ok)
	}

	registry, err := handlers.buildRegistry("default")
	if err != nil {
		t.Fatalf("resources API registry: %v", err)
	}
	agent, listed := registry.Get(agentID)
	if !listed || agent.Status != unifiedresources.StatusOffline {
		t.Fatalf("resources API agent row = %+v (listed=%v), want the saved agent's own offline row", agent, listed)
	}
	if guest, ok := registry.Get(vmID); !ok || guest.Agent != nil || guest.Status != unifiedresources.StatusOnline {
		t.Fatalf("resources API guest = %+v (listed=%v), want vSphere's online VM without the saved agent", guest, ok)
	}

	refs := []string{agentID, "agent:host-app-guest"}
	for _, ref := range refs {
		if got := putOperatorStateThroughAPI(t, handlers, ref, activeMaintenanceBody(now, "guest migration")); got != vmID {
			t.Fatalf("maintenance written through %s landed on %q, want the linked VM %s", ref, got, vmID)
		}
	}
	seed.clones = 0
	provider := router.patrolResourceOperatorStateProvider("default")
	for _, ref := range append(refs, vmID) {
		projection, ok := provider.OperatorStateProjection(ref, now)
		if !ok || projection.MaintenanceWindow == nil || projection.MaintenanceWindow.Reason != "guest migration" {
			t.Fatalf("finding on %s projected %+v (ok=%v), want the linked VM %s's maintenance window", ref, projection, ok, vmID)
		}
	}
	if seed.clones != 0 {
		t.Fatalf("finding projection cloned the unified listing %d times; it must resolve through the monitor's read state", seed.clones)
	}
}
