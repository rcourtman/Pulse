package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestAlertStillFiring_MetricAtOrAboveThresholdSkipsModelReview(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{{ID: "storage-1", Usage: 95}},
	}
	alert := AlertInfo{
		ID:         "alert-usage",
		Type:       "usage",
		ResourceID: "storage-1",
		Value:      92,
		Threshold:  90,
	}
	if !ps.alertStillFiringState(alert, patrolRuntimeStateForTest(ps, state)) {
		t.Fatalf("expected usage alert still above threshold to be treated as still firing")
	}
}

func TestAlertStillFiring_UncertainCasesGoToModel(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{{ID: "storage-1", Usage: 80}},
		Nodes:   []models.Node{{ID: "node-1", Name: "node-1", CPU: 0.10, Status: "online"}},
	}
	snap := patrolRuntimeStateForTest(ps, state)

	cases := []struct {
		name  string
		alert AlertInfo
	}{
		{"metric below threshold", AlertInfo{Type: "usage", ResourceID: "storage-1", Threshold: 90}},
		{"resource missing", AlertInfo{Type: "usage", ResourceID: "storage-gone", Threshold: 90}},
		{"zero threshold", AlertInfo{Type: "cpu", ResourceID: "node-1", ResourceType: "node", Threshold: 0}},
		{"unknown alert type", AlertInfo{Type: "disk-health", ResourceID: "node-1", ResourceType: "node", Threshold: 1}},
		{"offline resource missing", AlertInfo{Type: "offline", ResourceID: "vm-gone", ResourceType: "vm"}},
		{"offline unhandled resource type", AlertInfo{Type: "offline", ResourceID: "storage-1", ResourceType: "storage"}},
	}
	for _, tc := range cases {
		if ps.alertStillFiringState(tc.alert, snap) {
			t.Errorf("%s: expected alert to go to model review, but gate claimed still firing", tc.name)
		}
	}
}

func TestAlertStillFiring_OfflineResourceStillDownSkipsModelReview(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		VMs: []models.VM{{ID: "vm-1", Name: "vm-1", Status: "stopped"}},
	}
	snap := patrolRuntimeStateForTest(ps, state)

	alert := AlertInfo{
		ID:           "alert-offline",
		Type:         "offline",
		ResourceID:   "vm-1",
		ResourceName: "vm-1",
		ResourceType: "vm",
	}
	if !ps.alertStillFiringState(alert, snap) {
		t.Fatalf("expected offline alert for a still-stopped VM to be treated as still firing")
	}

	state.VMs[0].Status = "running"
	snap = patrolRuntimeStateForTest(ps, state)
	if ps.alertStillFiringState(alert, snap) {
		t.Fatalf("expected offline alert for a running VM to go to model review")
	}
}

func TestAlertStillFiring_ReadState(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Storage: []models.Storage{{ID: "storage-1", Usage: 95, Used: 95, Total: 100}},
	})
	snap := patrolRuntimeState{readState: registry}

	alert := AlertInfo{
		ID:         "alert-usage-readstate",
		Type:       "usage",
		ResourceID: "storage-1",
		Value:      92,
		Threshold:  90,
	}
	if !ps.alertStillFiringState(alert, snap) {
		t.Fatalf("expected readState usage alert above threshold to be treated as still firing")
	}
}

// Without a model, nothing resolves — Pulse must not use local heuristics to
// decide an issue has cleared, even when the metric has dropped below its
// threshold.
func TestReviewAndResolveAlerts_NoModelResolvesNothing(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	resolver := &stubAlertResolver{}
	ps.alertResolver = resolver

	now := time.Now()
	resolver.alerts = []AlertInfo{
		{
			ID:         "recent",
			Type:       "usage",
			ResourceID: "storage-1",
			Value:      95,
			Threshold:  90,
			StartTime:  now.Add(-5 * time.Minute),
		},
		{
			ID:         "stale",
			Type:       "usage",
			ResourceID: "storage-1",
			Value:      95,
			Threshold:  90,
			StartTime:  now.Add(-30 * time.Minute),
		},
	}

	// Usage has dropped below threshold — only a model review may resolve.
	state := models.StateSnapshot{Storage: []models.Storage{{ID: "storage-1", Usage: 70}}}
	resolved := ps.reviewAndResolveAlertsState(context.Background(), patrolRuntimeStateForTest(ps, state), false, "")
	if resolved != 0 {
		t.Fatalf("expected no alert resolution without model review, got %d", resolved)
	}
	if len(resolver.clears) != 0 {
		t.Fatalf("expected no alerts to be resolved, got %v", resolver.clears)
	}
}

func TestReviewAndResolveAlerts_StillFiringMakesNoModelCalls(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	resolver := &stubAlertResolver{}
	ps.alertResolver = resolver
	provider := &mockPatrolProvider{response: "ALERT 1: RESOLVE: should never be asked"}
	ps.aiService = &Service{
		provider: provider,
		cfg:      &config.AIConfig{Enabled: true, PatrolModel: "mock:model"},
	}

	now := time.Now()
	resolver.alerts = []AlertInfo{
		{
			ID:         "still-firing",
			Type:       "usage",
			ResourceID: "storage-1",
			Value:      95,
			Threshold:  90,
			StartTime:  now.Add(-2 * time.Hour),
		},
	}

	// Usage still above threshold — the review could only answer KEEP.
	state := models.StateSnapshot{Storage: []models.Storage{{ID: "storage-1", Usage: 95}}}
	resolved := ps.reviewAndResolveAlertsState(context.Background(), patrolRuntimeStateForTest(ps, state), true, "")
	if resolved != 0 {
		t.Fatalf("expected no resolutions, got %d", resolved)
	}
	if provider.calls != 0 {
		t.Fatalf("expected no model calls for a still-firing alert, got %d", provider.calls)
	}
	if len(resolver.clears) != 0 {
		t.Fatalf("expected no alerts to be resolved, got %v", resolver.clears)
	}
}

func TestReviewAndResolveAlerts_BatchedModelReviewResolves(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	resolver := &stubAlertResolver{}
	ps.alertResolver = resolver
	provider := &mockPatrolProvider{response: "ALERT 1: RESOLVE: usage back to normal\nALERT 2: KEEP: still verifying"}
	ps.aiService = &Service{
		provider: provider,
		cfg:      &config.AIConfig{Enabled: true, PatrolModel: "mock:model"},
	}

	now := time.Now()
	resolver.alerts = []AlertInfo{
		{
			ID:         "cleared",
			Type:       "usage",
			ResourceID: "storage-1",
			Value:      95,
			Threshold:  90,
			StartTime:  now.Add(-2 * time.Hour),
		},
		{
			ID:           "ambiguous",
			Type:         "offline",
			ResourceID:   "vm-1",
			ResourceName: "vm-1",
			ResourceType: "vm",
			StartTime:    now.Add(-2 * time.Hour),
		},
		{
			ID:         "too-recent",
			Type:       "usage",
			ResourceID: "storage-1",
			Value:      95,
			Threshold:  90,
			StartTime:  now.Add(-5 * time.Minute),
		},
	}

	state := models.StateSnapshot{
		Storage: []models.Storage{{ID: "storage-1", Usage: 70}},
		VMs:     []models.VM{{ID: "vm-1", Name: "vm-1", Status: "running"}},
	}
	resolved := ps.reviewAndResolveAlertsState(context.Background(), patrolRuntimeStateForTest(ps, state), true, "")
	if provider.calls != 1 {
		t.Fatalf("expected one batched model call for two review candidates, got %d", provider.calls)
	}
	if resolved != 1 {
		t.Fatalf("expected exactly one resolution, got %d", resolved)
	}
	if len(resolver.clears) != 1 || resolver.clears[0] != "cleared" {
		t.Fatalf("expected only the model-resolved alert to clear, got %v", resolver.clears)
	}
}

// dockerAlertReviewStates returns the same Docker hosts as a legacy snapshot
// and as canonical read state, the form a production Patrol run reads.
func dockerAlertReviewStates(hosts ...models.DockerHost) map[string]patrolRuntimeState {
	snapshot := models.StateSnapshot{DockerHosts: hosts}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	return map[string]patrolRuntimeState{
		"snapshot":   newPatrolRuntimeState(snapshot),
		"read state": {readState: registry},
	}
}

// dockerContainerAlert builds an alert the way the alert subsystem raises
// it, keyed by its "docker:<host>/<container>" resource ID.
func dockerContainerAlert(alertType, hostID string, container models.DockerContainer) AlertInfo {
	return AlertInfo{
		ID:           alertType + "-" + hostID + "-" + container.ID,
		Type:         alertType,
		ResourceID:   alerts.DockerContainerResourceID(hostID, container.ID, container.Name),
		ResourceName: container.Name,
		ResourceType: "app-container",
		StartTime:    time.Now().Add(-30 * time.Minute),
	}
}

func TestAlertStillFiring_DockerContainerConditions(t *testing.T) {
	oom, noOOM := true, false
	cases := []struct {
		name      string
		alertType string
		container models.DockerContainer
		want      bool
	}{
		{"unhealthy while running", "docker-container-health", models.DockerContainer{State: "running", Health: "unhealthy"}, true},
		{"unhealthy with no reported state", "docker-container-health", models.DockerContainer{Health: "unhealthy"}, true},
		{"healthy again", "docker-container-health", models.DockerContainer{State: "running", Health: "healthy"}, false},
		{"health check starting", "docker-container-health", models.DockerContainer{State: "running", Health: "starting"}, false},
		{"health check removed", "docker-container-health", models.DockerContainer{State: "running"}, false},
		{"stopped with stale unhealthy result", "docker-container-health", models.DockerContainer{State: "exited", Health: "unhealthy"}, false},
		{"still exited", "docker-container-state", models.DockerContainer{State: "exited"}, true},
		{"still paused", "docker-container-state", models.DockerContainer{State: "paused"}, true},
		{"running again", "docker-container-state", models.DockerContainer{State: "running"}, false},
		{"state not reported", "docker-container-state", models.DockerContainer{}, false},
		{"still OOM-killed", "docker-container-oom-kill", models.DockerContainer{State: "exited", OOMKilled: &oom}, true},
		{"exited without OOM", "docker-container-oom-kill", models.DockerContainer{State: "exited", OOMKilled: &noOOM}, false},
		{"OOM unknown", "docker-container-oom-kill", models.DockerContainer{State: "exited"}, false},
		{"restarted after OOM", "docker-container-oom-kill", models.DockerContainer{State: "running", OOMKilled: &oom}, false},
		{"waiting to restart", "docker-container-restart-loop", models.DockerContainer{State: "restarting"}, true},
		{"running between restarts", "docker-container-restart-loop", models.DockerContainer{State: "running"}, false},
	}

	ps := NewPatrolService(nil, nil)
	for _, tc := range cases {
		tc.container.ID, tc.container.Name = "c0ffee", "web"
		alert := dockerContainerAlert(tc.alertType, "docker-host-a", tc.container)
		states := dockerAlertReviewStates(models.DockerHost{ID: "docker-host-a", Hostname: "dockhost-a", Containers: []models.DockerContainer{tc.container}})
		for form, snap := range states {
			if got := ps.alertStillFiringState(alert, snap); got != tc.want {
				t.Errorf("%s (%s): still firing = %t, want %t", tc.name, form, got, tc.want)
			}
		}
	}

	// A container missing from the snapshot may have been removed: that is
	// the model's call, not the gate's.
	gone := dockerContainerAlert("docker-container-health", "docker-host-a", models.DockerContainer{ID: "gone", Name: "web"})
	for form, snap := range dockerAlertReviewStates(models.DockerHost{ID: "docker-host-a", Hostname: "dockhost-a"}) {
		if ps.alertStillFiringState(gone, snap) {
			t.Errorf("missing container (%s): expected model review, gate claimed still firing", form)
		}
	}
}

// Two hosts run a container named "web". Each alert is judged against its
// own container, never the same-named one on the other host.
func TestAlertStillFiring_DockerContainerUsesAlertIdentity(t *testing.T) {
	sick := models.DockerContainer{ID: "aaa111", Name: "web", State: "running", Health: "unhealthy"}
	fine := models.DockerContainer{ID: "bbb222", Name: "web", State: "running", Health: "healthy"}
	states := dockerAlertReviewStates(
		models.DockerHost{ID: "docker-host-b", Hostname: "dockhost-b", Containers: []models.DockerContainer{fine}},
		models.DockerHost{ID: "docker-host-a", Hostname: "dockhost-a", Containers: []models.DockerContainer{sick}},
	)

	ps := NewPatrolService(nil, nil)
	sickAlert := dockerContainerAlert("docker-container-health", "docker-host-a", sick)
	fineAlert := dockerContainerAlert("docker-container-health", "docker-host-b", fine)
	for form, snap := range states {
		if !ps.alertStillFiringState(sickAlert, snap) {
			t.Errorf("%s: unhealthy container's alert was judged against its healthy namesake", form)
		}
		if ps.alertStillFiringState(fineAlert, snap) {
			t.Errorf("%s: healthy container's alert was judged against its unhealthy namesake", form)
		}
		if got := ps.getResourceCurrentStateState(fineAlert, snap); !strings.Contains(got, "Health: healthy") {
			t.Errorf("%s: review context for the healthy container = %q", form, got)
		}
	}
}

// The live run on 24 Oct 2026: the review resolved a Docker health alert
// from CPU and memory ("no unhealthy indicator present") while Docker still
// reported the container unhealthy, and the alert engine raised it again on
// the agent's next report. An unhealthy container keeps its alert without a
// model call, whatever the model would have answered.
func TestReviewAndResolveAlerts_UnhealthyContainerKeepsHealthAlert(t *testing.T) {
	container := models.DockerContainer{ID: "5eed", Name: "pulse-live-web", State: "running", Health: "unhealthy", CPUPercent: 0.1, MemoryPercent: 1.2}
	for form, snap := range dockerAlertReviewStates(models.DockerHost{ID: "docker-host-a", Hostname: "dockhost-a", Containers: []models.DockerContainer{container}}) {
		t.Run(form, func(t *testing.T) {
			ps := NewPatrolService(nil, nil)
			resolver := &stubAlertResolver{alerts: []AlertInfo{dockerContainerAlert("docker-container-health", "docker-host-a", container)}}
			ps.alertResolver = resolver
			provider := &mockPatrolProvider{response: "ALERT 1: RESOLVE: Container is running with normal CPU/Memory, no unhealthy indicator present"}
			ps.aiService = &Service{provider: provider, cfg: &config.AIConfig{Enabled: true, PatrolModel: "mock:model"}}

			if resolved := ps.reviewAndResolveAlertsState(context.Background(), snap, true, ""); resolved != 0 {
				t.Fatalf("expected the unhealthy container's alert to stay active, resolved %d", resolved)
			}
			if provider.calls != 0 || len(resolver.clears) != 0 {
				t.Fatalf("expected no model call and no clear, got calls=%d clears=%v", provider.calls, resolver.clears)
			}
		})
	}
}

// Once Docker reports the container healthy, the alert goes to the model
// with that health in its context, and a RESOLVE verdict clears it.
func TestReviewAndResolveAlerts_RecoveredContainerHealthAlertResolves(t *testing.T) {
	container := models.DockerContainer{ID: "5eed", Name: "pulse-live-web", State: "running", Health: "healthy"}
	for form, snap := range dockerAlertReviewStates(models.DockerHost{ID: "docker-host-a", Hostname: "dockhost-a", Containers: []models.DockerContainer{container}}) {
		t.Run(form, func(t *testing.T) {
			ps := NewPatrolService(nil, nil)
			alert := dockerContainerAlert("docker-container-health", "docker-host-a", container)
			resolver := &stubAlertResolver{alerts: []AlertInfo{alert}}
			ps.alertResolver = resolver
			provider := &mockPatrolProvider{response: "ALERT 1: RESOLVE: health check passing again"}
			ps.aiService = &Service{provider: provider, cfg: &config.AIConfig{Enabled: true, PatrolModel: "mock:model"}}

			if resolved := ps.reviewAndResolveAlertsState(context.Background(), snap, true, ""); resolved != 1 {
				t.Fatalf("expected the recovered container's alert to resolve, resolved %d", resolved)
			}
			if provider.calls != 1 || len(resolver.clears) != 1 || resolver.clears[0] != alert.ID {
				t.Fatalf("expected one model call clearing %s, got calls=%d clears=%v", alert.ID, provider.calls, resolver.clears)
			}
			prompt := provider.lastReq.Messages[1].Content
			if !strings.Contains(prompt, "Docker container 'pulse-live-web'") || !strings.Contains(prompt, "Health: healthy") {
				t.Fatalf("expected the review context to carry the container's health, got %q", prompt)
			}
			if !strings.Contains(prompt, "If the current state does not cover the condition the alert is about, answer KEEP") {
				t.Fatalf("expected the review prompt to treat missing evidence as KEEP, got %q", prompt)
			}
		})
	}
}

// A ZFS pool alert names "<storage>/zfs-pool:<pool>", which no storage row
// matches. The storage is still in inventory, so the review must not be told
// the resource may have been removed, which invites a RESOLVE while the pool
// still reports its errors. A storage that is really gone keeps that wording.
func TestGetResourceCurrentState_PresentChildSubjectIsNotRemoved(t *testing.T) {
	storage := models.Storage{
		ID: "pve1-node1-local-zfs", Name: "local-zfs", Node: "node1", Instance: "pve1", Type: "zfspool",
		Status: "available", Total: 100, Used: 40, Free: 60, Usage: 40,
		ZFSPool: &models.ZFSPool{Name: "rpool", State: "ONLINE", ChecksumErrors: 12},
	}
	vm := models.VM{ID: "pve1-node1-100", VMID: 100, Name: "web", Node: "node1", Instance: "pve1", Type: "qemu", Status: "running"}
	snapshot := models.StateSnapshot{Storage: []models.Storage{storage}, VMs: []models.VM{vm}}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	ps := NewPatrolService(nil, nil)
	present := AlertInfo{ID: "zfs-errors", Type: "zfs-pool-errors", ResourceID: storage.ID + "/zfs-pool:rpool", ResourceName: "local-zfs (rpool)", ResourceType: "storage"}
	// A backup-age alert names its guest by alerts.BuildGuestKey.
	backup := AlertInfo{ID: "backup-age", Type: "backup-age", ResourceID: alerts.BuildGuestKey("pve1", "node1", 100), ResourceName: "web backup", ResourceType: "vm"}
	gone := present
	gone.ResourceID = "pve1-node1-retired/zfs-pool:rpool"
	for form, snap := range map[string]patrolRuntimeState{"snapshot": newPatrolRuntimeState(snapshot), "read state": {readState: registry}} {
		if got := ps.getResourceCurrentStateState(present, snap); !strings.Contains(got, "Present in current inventory") {
			t.Errorf("%s: present pool's storage described as %q", form, got)
		}
		if got := ps.getResourceCurrentStateState(backup, snap); !strings.Contains(got, "Present in current inventory") {
			t.Errorf("%s: backed-up guest described as %q", form, got)
		}
		if got := ps.getResourceCurrentStateState(gone, snap); got != "Storage not found in current state (may have been removed)" {
			t.Errorf("%s: missing storage described as %q", form, got)
		}
	}
}
