package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// The scopes below are the ones the Patrol fault set recorded as "scope
// resolution failed". Each carried a type that is not a Patrol resource type:
// the unified store's "agent-disk", or an alert kind ("metric-threshold",
// "connectivity") passed where a resource type belongs. The alert's resource
// resolved, and the type filter then removed it.
func TestPatrolAlertScope_ProducerTypeHintYieldsToTheResolvedResource(t *testing.T) {
	const agentID = "66cd990e-2def-40c7-a9e4-c450a9e450de"
	diskAlert := models.Alert{
		ID:         "agent:" + agentID + "/disk:mnt-nvr::metric-threshold:disk",
		ResourceID: "agent:" + agentID + "/disk:mnt-nvr",
		Type:       "disk", Level: "warning", Value: 92.3, Threshold: 90,
		Metadata: map[string]interface{}{"mountpoint": "/mnt/nvr"},
	}
	hostAlert := models.Alert{
		ID:         alerts.DockerHostResourceID(agentID) + "::" + alerts.DockerHostResourceID(agentID) + "-connectivity",
		ResourceID: alerts.DockerHostResourceID(agentID),
		Type:       "docker-host-offline", Level: "critical",
	}
	hosts := []models.Host{{ID: agentID, Hostname: "docker01", MachineID: "machine-docker01"}}
	dockerHosts := []models.DockerHost{{ID: agentID, Hostname: "docker01", Status: "online", LastSeen: time.Now()}}

	for _, tc := range []struct {
		name     string
		snapshot models.StateSnapshot
		scope    PatrolScope
	}{
		{
			name: "cleared host filesystem alert",
			snapshot: models.StateSnapshot{Hosts: hosts,
				RecentlyResolved: []models.ResolvedAlert{{Alert: diskAlert, ResolvedTime: time.Now()}}},
			scope: AlertClearedPatrolScope(diskAlert.ID, diskAlert.ResourceID, "agent-disk"),
		},
		{
			name:     "firing host filesystem alert",
			snapshot: models.StateSnapshot{Hosts: hosts, ActiveAlerts: []models.Alert{diskAlert}},
			scope:    AlertTriggeredPatrolScope(diskAlert.ID, diskAlert.ResourceID, "agent-disk", "disk"),
		},
		{
			name:     "flapping host filesystem alert",
			snapshot: models.StateSnapshot{Hosts: hosts, ActiveAlerts: []models.Alert{diskAlert}},
			scope:    FlappingPostmortemPatrolScope(diskAlert.ID, diskAlert.ResourceID, "metric-threshold", "disk"),
		},
		{
			name:     "flapping docker host connectivity alert",
			snapshot: models.StateSnapshot{DockerHosts: dockerHosts, ActiveAlerts: []models.Alert{hostAlert}},
			scope:    FlappingPostmortemPatrolScope(hostAlert.ID, hostAlert.ResourceID, "connectivity", "docker-host-offline"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := newPatrolRuntimeState(tc.snapshot)
			resolved, resolution := resolvePatrolScopeState(state, tc.scope)
			if len(resolution.UnmatchedResourceIDs)+len(resolution.AmbiguousResourceIDs) != 0 {
				t.Fatalf("scope did not resolve: %+v", resolution)
			}
			if len(resolution.EffectiveResourceIDs) == 0 {
				t.Fatalf("the type hint %v filtered out the resolved resource: %+v", tc.scope.ResourceTypes, resolution)
			}
			// The accepted run resolves a second time against current state.
			if _, again := resolvePatrolScopeState(state, resolved); len(again.EffectiveResourceIDs) == 0 {
				t.Fatalf("second resolution lost the resource: %+v", again)
			}
			filtered := filterPatrolStateByScopeState(state, resolved)
			if len(filtered.Hosts)+len(filtered.DockerHosts) != 1 {
				t.Fatalf("expected exactly the alert's owner in scope, got %d hosts and %d docker hosts",
					len(filtered.Hosts), len(filtered.DockerHosts))
			}
		})
	}
}

func newClearedAlertPatrol(t *testing.T, snapshot models.StateSnapshot) *PatrolService {
	t.Helper()
	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	return NewPatrolService(svc, &patrolTestStateProvider{state: snapshot})
}

// When the operator fixes what an alert reported, the finding that restated
// the alert must close with it, even when its investigation had already ended
// at "needs attention" and nothing would look at it again.
func TestResolveFindingsForClearedAlert_ClosesTheFindingThatRestatedTheAlert(t *testing.T) {
	containerRef := alerts.DockerContainerResourceID("docker01", "nextcloud-1", "nextcloud")
	alertID := containerRef + "::" + containerRef + "-health"
	ps := newClearedAlertPatrol(t, models.StateSnapshot{
		DockerHosts: []models.DockerHost{{ID: "docker01", Hostname: "docker01", Status: "online", LastSeen: time.Now(),
			Containers: []models.DockerContainer{{ID: "nextcloud-1", Name: "nextcloud", State: "running", Health: "healthy"}}}},
	})

	restates := &Finding{
		ID: "restates-alert", Key: "health-check-failed", Severity: FindingSeverityCritical,
		Category: FindingCategoryReliability, ResourceID: containerRef, ResourceName: "nextcloud",
		ResourceType: "app-container", Title: "nextcloud is unhealthy", Source: "ai-analysis",
		InvestigationOutcome: string(InvestigationOutcomeNeedsAttention),
	}
	differentProblem := &Finding{
		ID: "different-problem", Key: "invalid-configuration", Severity: FindingSeverityWarning,
		Category: FindingCategoryReliability, ResourceID: containerRef, ResourceName: "nextcloud",
		ResourceType: "app-container", Title: "DATABASE_URL points at a missing host", Source: "ai-analysis",
	}
	otherResource := &Finding{
		ID: "other-resource", Key: "health-check-failed", Severity: FindingSeverityCritical,
		Category: FindingCategoryReliability, ResourceID: alerts.DockerContainerResourceID("docker01", "db-1", "db"),
		ResourceName: "db", ResourceType: "app-container", Title: "db is unhealthy", Source: "ai-analysis",
	}
	for _, f := range []*Finding{restates, differentProblem, otherResource} {
		ps.findings.Add(f)
	}

	if closed := ps.ResolveFindingsForClearedAlert(alertID, containerRef, "docker-container-health"); closed != 1 {
		t.Fatalf("expected one finding to close, got %d", closed)
	}
	got := ps.findings.Get("restates-alert")
	if got == nil || got.ResolvedAt == nil || got.ResolveReason != clearedAlertResolveReason {
		t.Fatalf("the finding that restated the alert stayed open: %+v", got)
	}
	for _, id := range []string{"different-problem", "other-resource"} {
		if f := ps.findings.Get(id); f == nil || f.ResolvedAt != nil {
			t.Fatalf("finding %s must stay open: %+v", id, f)
		}
	}
	if closed := ps.ResolveFindingsForClearedAlert(alertID, containerRef, "docker-container-health"); closed != 0 {
		t.Fatalf("a second clear closed %d more findings", closed)
	}
	if closed := ps.ResolveFindingsForClearedAlert("", containerRef, "docker-container-health"); closed != 0 {
		t.Fatalf("a clear without an alert identifier closed %d findings", closed)
	}
}

// A recreated container gets a new ID, so its old alert clears and the old
// resource is gone from inventory. The finding stamped as mirroring that alert
// must still close: nothing else will ever match it again.
func TestResolveFindingsForClearedAlert_ClosesAStampedFindingWhoseResourceIsGone(t *testing.T) {
	oldRef := alerts.DockerContainerResourceID("docker01", "old-container-id", "worker")
	alertID := oldRef + "::" + oldRef + "-restart-loop"
	ps := newClearedAlertPatrol(t, models.StateSnapshot{
		DockerHosts: []models.DockerHost{{ID: "docker01", Hostname: "docker01", Status: "online", LastSeen: time.Now(),
			Containers: []models.DockerContainer{{ID: "new-container-id", Name: "worker", State: "running"}}}},
	})
	ps.findings.Add(&Finding{
		ID: "old-worker", Key: "restart-loop", Severity: FindingSeverityCritical,
		Category: FindingCategoryReliability, ResourceID: "app-container:old-unified-id", ResourceName: "worker",
		ResourceType: "app-container", Title: "worker is restarting repeatedly", Source: "ai-analysis",
		InvestigationOutcome: string(InvestigationOutcomeNeedsAttention),
	})
	// Stamped by the run that raised it, while the alert was active.
	ps.findings.StampAlertMirrors(map[string]findingAlertMirror{
		"old-worker": {AlertID: alertID, AlertType: "docker-container-restart-loop"},
	})

	if closed := ps.ResolveFindingsForClearedAlert(alertID, oldRef, "docker-container-restart-loop"); closed != 1 {
		t.Fatalf("expected the stamped finding to close, got %d", closed)
	}
	if got := ps.findings.Get("old-worker"); got == nil || got.ResolvedAt == nil {
		t.Fatalf("the old container's finding stayed open: %+v", got)
	}
}
