package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// A full PBS datastore raises its usage warning under the poller's storage ID
// ("pbs-<name>-<datastore>"), which the inventory folds into the canonical
// datastore (#1591). The check that warning starts did not resolve, so Patrol
// recorded "scope could not be resolved" and never looked. It now resolves to
// the datastore and is given the same facts as a Proxmox storage that holds
// backups.
func TestSustainedCheckOnAFullPBSDatastoreResolvesAndIsToldWhereToLook(t *testing.T) {
	now := time.Now().UTC()
	pbs := models.PBSInstance{ID: "pbs-pbs1", Name: "pbs1", Host: "https://pbs1.lan:8007", Status: "online", LastSeen: now,
		Datastores: []models.PBSDatastore{{Name: "store1", Total: 1000, Used: 910, Free: 90, Usage: 91, Status: "available"}}}
	storage := models.Storage{ID: "pbs-pbs1-store1", AliasIDs: []string{"pbs-pbs1/store1"}, Name: "store1", Node: "pbs1",
		Instance: "pbs-pbs1", Type: "pbs", Status: "available", Total: 1000, Used: 910, Free: 90, Usage: 91,
		Content: "backup", Shared: true, Enabled: true, Active: true, LastSeen: now}
	alert := models.Alert{ID: "pbs-pbs1-store1::metric-threshold:usage", Type: "usage", Level: "warning",
		ResourceID: "pbs-pbs1-store1", ResourceName: "store1", Node: "pbs1", Instance: "pbs-pbs1",
		Message: "Storage at 91.0%", Value: 91, Threshold: 85, StartTime: now.Add(-11 * time.Minute)}
	snapshot := models.StateSnapshot{PBSInstances: []models.PBSInstance{pbs}, Storage: []models.Storage{storage}, ActiveAlerts: []models.Alert{alert}}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model", PatrolAutonomyLevel: config.PatrolAutonomyMonitor}
	svc.provider = &mockProvider{}
	svc.SetChatService(&patrolMockChatService{executor: tools.NewPulseToolExecutor(tools.ExecutorConfig{})})
	ps := NewPatrolService(svc, &patrolTestStateProvider{state: snapshot})
	ps.SetConfig(PatrolConfig{Enabled: true, Interval: time.Hour, AnalyzeNodes: true, AnalyzeGuests: true, AnalyzeStorage: true, AnalyzePBS: true})
	ps.SetReadState(registry)
	ps.SetUnifiedResourceProvider(unifiedresources.NewUnifiedAIAdapter(registry))
	ps.SetActionCapabilityLookup(func(_ context.Context, _ string) (aicontracts.ActionCapabilityCatalog, error) {
		return aicontracts.ActionCapabilityCatalog{}, nil
	})

	state := ps.currentPatrolRuntimeState()
	resolved, resolution := resolvePatrolScopeState(state, SustainedAlertPatrolScope(alert.ID, alert.ResourceID, "storage", alert.Type))
	if len(resolution.UnmatchedResourceIDs)+len(resolution.AmbiguousResourceIDs) != 0 || len(resolution.ResolvedResourceIDs) == 0 {
		t.Fatalf("scope did not resolve: %+v", resolution)
	}
	var datastoreID string
	for _, resource := range registry.List() {
		if resource.Type == unifiedresources.ResourceTypeStorage && resource.Name == "store1" {
			datastoreID = resource.ID
		}
	}
	found := false
	for _, id := range resolution.ResolvedResourceIDs {
		found = found || id == datastoreID
	}
	if datastoreID == "" || !found {
		t.Fatalf("scope resolved to %v, want the datastore %q", resolution.ResolvedResourceIDs, datastoreID)
	}
	seed, _ := ps.seedFindingsAndContextState(&resolved, filterPatrolStateByScopeState(state, resolved))
	for _, want := range []string{
		`which has stayed active: usage = 91.0`,
		`Pulse lists the backups kept on "store1"`,
	} {
		if !strings.Contains(seed, want) {
			t.Errorf("seed lacks %q:\n%s", want, seed)
		}
	}
	if !ps.sustainedAlertCallsForFinding(&resolved, state, &alert) {
		t.Error("an unanswered usage warning on a full PBS datastore does not open its finding")
	}
}
