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

// storageUsageCheck is the Proxmox fault set's P3 shape: a node's "local"
// storage at 91% with its usage warning still active, and the sustained check
// the alert bridge asks for, on a node whose agent has commands off.
type storageUsageCheck struct {
	ps       *PatrolService
	scope    PatrolScope
	state    patrolRuntimeState
	alert    models.Alert
	seed     string
	decision string
}

func newStorageUsageCheck(t *testing.T, content string) storageUsageCheck {
	t.Helper()
	now := time.Now().UTC()
	node := models.Node{ID: "pmxlab-pmxlab", Name: "pmxlab", Instance: "pmxlab", Status: "online", LastSeen: now}
	storage := models.Storage{
		ID: "pmxlab-local", Name: "local", Node: "pmxlab", Instance: "pmxlab", Type: "dir", Status: "available",
		Total: 33603379200, Used: 30579601408, Free: 3023777792, Usage: 91.0, Content: content, Enabled: true, Active: true, LastSeen: now,
	}
	alert := models.Alert{
		ID: "pmxlab-local::metric-threshold:usage", Type: "usage", Level: "warning",
		ResourceID: "pmxlab-local", ResourceName: "local", Node: "pmxlab", Instance: "pmxlab",
		Message: "Storage at 91.0%", Value: 91.0, Threshold: 85, StartTime: now.Add(-11 * time.Minute),
	}
	snapshot := models.StateSnapshot{Nodes: []models.Node{node}, Storage: []models.Storage{storage}, ActiveAlerts: []models.Alert{alert}}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model", PatrolAutonomyLevel: config.PatrolAutonomyMonitor}
	svc.provider = &mockProvider{}
	svc.SetChatService(&patrolMockChatService{executor: tools.NewPulseToolExecutor(tools.ExecutorConfig{})})
	ps := NewPatrolService(svc, &patrolTestStateProvider{state: snapshot})
	ps.SetConfig(PatrolConfig{Enabled: true, Interval: time.Hour, AnalyzeNodes: true, AnalyzeGuests: true, AnalyzeStorage: true})
	ps.SetReadState(registry)
	ps.SetUnifiedResourceProvider(unifiedresources.NewUnifiedAIAdapter(registry))
	ps.SetActionCapabilityLookup(func(_ context.Context, _ string) (aicontracts.ActionCapabilityCatalog, error) {
		return aicontracts.ActionCapabilityCatalog{}, nil
	})

	state := ps.currentPatrolRuntimeState()
	resolved, resolution := resolvePatrolScopeState(state, SustainedAlertPatrolScope(alert.ID, alert.ResourceID, "storage", alert.Type))
	if len(resolution.UnmatchedResourceIDs)+len(resolution.AmbiguousResourceIDs) != 0 || len(resolution.EffectiveResourceIDs) == 0 {
		t.Fatalf("scope did not resolve: %+v", resolution)
	}
	seed, _ := ps.seedFindingsAndContextState(&resolved, filterPatrolStateByScopeState(state, resolved))
	return storageUsageCheck{
		ps: ps, scope: resolved, state: state, alert: alert,
		seed: seed, decision: ps.patrolFindingDecisionPrompt(&resolved, state),
	}
}

// On pve-v1 P3 a node's local storage filled with old backups of one guest.
// With commands off, the sustained check on its usage warning was told only
// "Pulse has no action for "local"". The runs that listed the storage's
// backups named the guest and filed; the runs that read only the pool's size
// recorded nothing, and the finding came when the harness asked. The check is
// now told where it can see what fills the storage, and an unanswered check
// opens the alert's finding, whose investigation reads the list.
func TestSustainedStorageCheckIsToldWhereItSeesWhatFillsIt(t *testing.T) {
	check := newStorageUsageCheck(t, "backup,images,iso,rootdir,snippets,vztmpl")
	for _, want := range []string{
		`Pulse lists the backups kept on "local": pulse_storage with type "backups" shows each guest's backups there`,
		"When you cannot tell from here, report a finding: its investigation reads the list",
	} {
		if !strings.Contains(check.seed, want) {
			t.Errorf("seed lacks %q:\n%s", want, check.seed)
		}
	}
	if !strings.Contains(check.decision, `Pulse lists the backups kept on "local", so a storage reading whose cause you could not see is one to record`) {
		t.Errorf("decision question lacks the backup-list rule:\n%s", check.decision)
	}
	if strings.Contains(check.decision, "a reading with nothing known beyond the alert itself") {
		t.Errorf("decision question still offers the alert-only exit:\n%s", check.decision)
	}
	if !check.ps.sustainedAlertCallsForFinding(&check.scope, check.state, &check.alert) {
		t.Error("an unanswered usage warning on a storage that holds backups does not open its finding")
	}
}

// A storage that holds no backups (guest disks only) has no list that says
// what fills it, so the check keeps the alert-only exit and its judgment.
func TestSustainedStorageCheckWithoutBackupsKeepsItsJudgment(t *testing.T) {
	check := newStorageUsageCheck(t, "images,rootdir")
	if strings.Contains(check.seed, "Pulse lists the backups") || strings.Contains(check.decision, "Pulse lists the backups") {
		t.Errorf("a storage without backups was told it has a backup list:\nseed: %s\ndecision: %s", check.seed, check.decision)
	}
	if !strings.Contains(check.decision, "a reading with nothing known beyond the alert itself") {
		t.Errorf("decision question lost the alert-only exit:\n%s", check.decision)
	}
	if check.ps.sustainedAlertCallsForFinding(&check.scope, check.state, &check.alert) {
		t.Error("an unanswered usage warning on a storage without backups opened its finding")
	}
}
