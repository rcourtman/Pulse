package alerts

import (
	"testing"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func intPointer(value int) *int    { return &value }
func boolPointer(value bool) *bool { return &value }

func TestAlertIntentPolicyResolutionPrecedenceIsFieldByField(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)
	document := NewAlertIntentPolicyDocument()
	document.Defaults[string(AlertIntentSignalOffline)] = AlertIntentRule{
		GraceSeconds:       intPointer(30),
		HonorOperatorState: boolPointer(true),
		BackupOffline: &BackupOfflineIntentPolicy{
			Enabled: true, PostGraceSeconds: 45, MaxDeferralSeconds: 600,
		},
	}
	document.ResourceTypes["vm"] = map[string]AlertIntentRule{
		string(AlertIntentSignalOffline): {GraceSeconds: intPointer(60)},
	}
	document.Resources["vm:101"] = map[string]AlertIntentRule{
		string(AlertIntentSignalOffline): {GraceSeconds: intPointer(0)},
	}
	if err := m.LoadIntentPolicies(document); err != nil {
		t.Fatalf("LoadIntentPolicies() error = %v", err)
	}

	effective := m.ResolveEffectiveIntentPolicy("vm:101", "vm", string(AlertIntentSignalOffline))
	if effective.GraceSeconds != 0 || !effective.HonorOperatorState || effective.BackupOffline == nil || !effective.BackupOffline.Enabled {
		t.Fatalf("effective policy = %+v", effective)
	}
	if got := effective.Sources["graceSeconds"]; got != "resources.vm:101.state.offline" {
		t.Fatalf("grace source = %q", got)
	}
	if got := effective.Sources["honorOperatorState"]; got != "defaults.state.offline" {
		t.Fatalf("operator source = %q", got)
	}
}

func TestAlertIntentPolicyResourceTypeInheritanceIsGeneralThenSpecific(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)
	document := NewAlertIntentPolicyDocument()
	document.ResourceTypes["guest"] = map[string]AlertIntentRule{
		string(AlertIntentSignalOffline): {
			GraceSeconds:       intPointer(300),
			HonorOperatorState: boolPointer(true),
		},
	}
	document.ResourceTypes["vm"] = map[string]AlertIntentRule{
		string(AlertIntentSignalOffline): {HonorOperatorState: boolPointer(false)},
	}
	if err := m.LoadIntentPolicies(document); err != nil {
		t.Fatal(err)
	}

	vm := m.ResolveEffectiveIntentPolicy("vm:101", "vm", string(AlertIntentSignalOffline))
	if vm.GraceSeconds != 300 || vm.HonorOperatorState {
		t.Fatalf("vm effective policy = %+v", vm)
	}
	if vm.Sources["graceSeconds"] != "resourceTypes.guest.state.offline" ||
		vm.Sources["honorOperatorState"] != "resourceTypes.vm.state.offline" {
		t.Fatalf("vm policy sources = %+v", vm.Sources)
	}

	node := m.ResolveEffectiveIntentPolicy("node:a", "node", string(AlertIntentSignalOffline))
	if node.Explicit || node.Sources["graceSeconds"] != "factory" {
		t.Fatalf("guest powered-off default leaked into node connectivity policy: %+v", node)
	}
}

func TestAlertIntentPolicyResolvesCanonicalResourceFromSourceID(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)
	document := NewAlertIntentPolicyDocument()
	document.Resources["vm-canonical"] = map[string]AlertIntentRule{
		string(AlertIntentSignalOffline): {GraceSeconds: intPointer(90)},
	}
	if err := m.LoadIntentPolicies(document); err != nil {
		t.Fatal(err)
	}
	m.SetResourceIntentIdentityResolver(func(resourceID string) (string, bool) {
		if resourceID == "cluster-a:node-a:101" {
			return "vm-canonical", true
		}
		return "", false
	})

	effective := m.ResolveEffectiveIntentPolicy("cluster-a:node-a:101", "vm", string(AlertIntentSignalOffline))
	if effective.GraceSeconds != 90 || effective.Sources["graceSeconds"] != "resources.vm-canonical.state.offline" {
		t.Fatalf("effective policy = %+v", effective)
	}
}

func TestAlertIntentPolicyValidationRejectsAmbiguousNormalizedKeys(t *testing.T) {
	document := NewAlertIntentPolicyDocument()
	document.ResourceTypes["VM"] = map[string]AlertIntentRule{"default": {GraceSeconds: intPointer(10)}}
	document.ResourceTypes[" vm "] = map[string]AlertIntentRule{"*": {GraceSeconds: intPointer(20)}}
	if err := ValidateAlertIntentPolicyDocument(document); err == nil {
		t.Fatal("expected normalized resource type collision to fail validation")
	}

	document = NewAlertIntentPolicyDocument()
	document.Defaults["default"] = AlertIntentRule{GraceSeconds: intPointer(10)}
	document.Defaults["*"] = AlertIntentRule{GraceSeconds: intPointer(20)}
	if err := ValidateAlertIntentPolicyDocument(document); err == nil {
		t.Fatal("expected normalized signal collision to fail validation")
	}
}

func TestAlertIntentBackupDeferralEndsWithPostGraceAndHardCap(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)
	start := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	now := start
	var tick time.Duration
	m.now = func() time.Time { return now }
	m.intentClock = func() time.Duration { return tick }
	document := NewAlertIntentPolicyDocument()
	document.Resources["vm:101"] = map[string]AlertIntentRule{
		string(AlertIntentSignalOffline): {
			GraceSeconds: intPointer(30),
			BackupOffline: &BackupOfflineIntentPolicy{
				Enabled: true, PostGraceSeconds: 60, MaxDeferralSeconds: 300,
			},
		},
	}
	if err := m.LoadIntentPolicies(document); err != nil {
		t.Fatal(err)
	}

	m.mu.Lock()
	active := m.evaluateIntentNoLock("vm:101", "vm", string(AlertIntentSignalOffline), "offline:vm:101", start, true, BackupIntentContext{Active: true, Evidence: "guest_lock"})
	now, tick = start.Add(100*time.Second), 100*time.Second
	ended := m.evaluateIntentNoLock("vm:101", "vm", string(AlertIntentSignalOffline), "offline:vm:101", start.Add(100*time.Second), true, BackupIntentContext{})
	now, tick = start.Add(159*time.Second), 159*time.Second
	pending := m.evaluateIntentNoLock("vm:101", "vm", string(AlertIntentSignalOffline), "offline:vm:101", start.Add(159*time.Second), true, BackupIntentContext{})
	now, tick = start.Add(160*time.Second), 160*time.Second
	eligible := m.evaluateIntentNoLock("vm:101", "vm", string(AlertIntentSignalOffline), "offline:vm:101", start.Add(160*time.Second), true, BackupIntentContext{})
	m.mu.Unlock()

	if !active.Pending || active.Reason != "backup_active" {
		t.Fatalf("active backup decision = %+v", active)
	}
	if !ended.Pending || ended.EligibleAt != start.Add(160*time.Second) {
		t.Fatalf("ended backup decision = %+v", ended)
	}
	if !pending.Pending || pending.ShouldActivate {
		t.Fatalf("post-backup decision = %+v", pending)
	}
	if !eligible.ShouldActivate || eligible.HardCapAt != start.Add(300*time.Second) {
		t.Fatalf("eligible decision = %+v", eligible)
	}
}

func TestAlertIntentPreviewHonorsOperatorStateWithoutMutatingRuntime(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)
	document := NewAlertIntentPolicyDocument()
	document.Defaults[string(AlertIntentSignalOffline)] = AlertIntentRule{
		GraceSeconds: intPointer(10), HonorOperatorState: boolPointer(true),
	}
	if err := m.LoadIntentPolicies(document); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	end := now.Add(time.Hour)
	m.SetOperatorIntentContextResolver(func(resourceID string, observedAt time.Time) (OperatorIntentContext, bool) {
		return OperatorIntentContext{MaintenanceStartAt: &now, MaintenanceEndAt: &end, MaintenanceReason: "upgrade"}, true
	})

	preview, err := m.PreviewIntentPolicy(AlertIntentPolicyPreviewRequest{
		ResourceID: "vm:101", ResourceType: "vm", Signal: string(AlertIntentSignalOffline), ConditionActive: true,
	})
	if err != nil {
		t.Fatalf("PreviewIntentPolicy() error = %v", err)
	}
	if preview.Status != "expected_transient" || preview.Reason != "operator_maintenance" {
		t.Fatalf("preview = %+v", preview)
	}
	if len(preview.Contexts) != 1 || preview.Contexts[0].Kind != "operator_state" || !preview.Contexts[0].Active {
		t.Fatalf("preview contexts = %+v", preview.Contexts)
	}
	if len(m.intentPending) != 0 {
		t.Fatalf("preview leaked runtime state: %+v", m.intentPending)
	}
}

func TestLifecycleAlertStartsAtFirstIntentMatch(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)
	start := time.Date(2026, 7, 13, 11, 0, 0, 0, time.UTC)
	now := start
	var tick time.Duration
	m.now = func() time.Time { return now }
	m.intentClock = func() time.Duration { return tick }
	document := NewAlertIntentPolicyDocument()
	document.Resources["vm:101"] = map[string]AlertIntentRule{
		string(AlertIntentSignalOffline): {GraceSeconds: intPointer(60)},
	}
	if err := m.LoadIntentPolicies(document); err != nil {
		t.Fatal(err)
	}
	spec, err := buildCanonicalPoweredStateSpec("vm:101", "database", unifiedresources.ResourceTypeVM, AlertLevelWarning, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	tracking := make(map[string]int)
	params := canonicalLifecycleAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: start,
			PoweredState: &alertspecs.PoweredStateEvidence{
				Expected: alertspecs.PowerStateOn,
				Observed: alertspecs.PowerStateOff,
			},
		},
		Tracking: tracking, TrackingKey: "vm:101", AlertID: "guest-powered-off-vm:101",
		AlertType: "powered-off", ResourceID: "vm:101", ResourceName: "database",
	}
	if result, _ := m.evaluateCanonicalLifecycleAlert(params); result.State.State != alertspecs.AlertStatePending {
		t.Fatalf("initial state = %s, want pending", result.State.State)
	}
	now, tick = start.Add(60*time.Second), 60*time.Second
	params.Evidence.ObservedAt = start.Add(60 * time.Second)
	if result, _ := m.evaluateCanonicalLifecycleAlert(params); result.State.State != alertspecs.AlertStateFiring {
		t.Fatalf("eligible state = %s, want firing", result.State.State)
	}

	m.mu.RLock()
	alert, ok := m.getActiveAlertNoLock(canonicalPoweredStateStateID("vm:101"))
	m.mu.RUnlock()
	if !ok || alert == nil {
		t.Fatal("expected powered-state alert")
	}
	if !alert.StartTime.Equal(start) {
		t.Fatalf("alert start = %v, want first match %v", alert.StartTime, start)
	}
}

func TestIntentPendingStatePersistsAcrossRestart(t *testing.T) {
	dataDir := t.TempDir()
	first := NewManagerWithDataDir(dataDir)
	t.Cleanup(first.Stop)
	now := time.Now().UTC().Truncate(time.Second)
	first.mu.Lock()
	first.intentPending["state:vm:101"] = IntentPendingState{
		TrackingKey: "state:vm:101", ResourceID: "vm:101", ResourceType: "vm",
		Signal: string(AlertIntentSignalOffline), FirstMatchedAt: now.Add(-time.Minute), LastObservedAt: now,
	}
	first.mu.Unlock()
	if err := first.SaveActiveAlerts(); err != nil {
		t.Fatalf("SaveActiveAlerts: %v", err)
	}

	second := NewManagerWithDataDir(dataDir)
	t.Cleanup(second.Stop)
	if err := second.LoadActiveAlerts(); err != nil {
		t.Fatalf("LoadActiveAlerts: %v", err)
	}
	second.mu.RLock()
	restored, ok := second.intentPending["state:vm:101"]
	second.mu.RUnlock()
	if !ok || !restored.FirstMatchedAt.Equal(now.Add(-time.Minute)) {
		t.Fatalf("restored intent state = %+v, found %v", restored, ok)
	}
	if restored.ElapsedNanos != int64(time.Minute) {
		t.Fatalf("restored elapsed = %s, want 1m", time.Duration(restored.ElapsedNanos))
	}
}

func TestIntentPendingElapsedProgressSurvivesRestartConservatively(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	first := NewManagerWithDataDir(dataDir)
	first.mu.Lock()
	first.intentPending["state:vm:restart"] = IntentPendingState{
		TrackingKey: "state:vm:restart", ResourceID: "vm:restart", ResourceType: "vm",
		Signal: string(AlertIntentSignalOffline), FirstMatchedAt: now.Add(-2 * time.Minute),
		LastObservedAt: now, ElapsedNanos: int64(120 * time.Second),
	}
	first.mu.Unlock()
	if err := first.SaveActiveAlerts(); err != nil {
		first.Stop()
		t.Fatalf("SaveActiveAlerts: %v", err)
	}
	first.Stop()

	second := NewManagerWithDataDir(dataDir)
	t.Cleanup(second.Stop)
	document := NewAlertIntentPolicyDocument()
	document.Resources["vm:restart"] = map[string]AlertIntentRule{
		string(AlertIntentSignalOffline): {GraceSeconds: intPointer(300)},
	}
	if err := second.LoadIntentPolicies(document); err != nil {
		t.Fatal(err)
	}
	wall := now.Add(time.Hour)
	var tick time.Duration
	second.now = func() time.Time { return wall }
	second.intentClock = func() time.Duration { return tick }

	second.mu.Lock()
	afterRestart := second.evaluateIntentNoLock("vm:restart", "vm", string(AlertIntentSignalOffline), "state:vm:restart", wall, true, BackupIntentContext{})
	tick = 180 * time.Second
	wall = wall.Add(180 * time.Second)
	eligible := second.evaluateIntentNoLock("vm:restart", "vm", string(AlertIntentSignalOffline), "state:vm:restart", wall, true, BackupIntentContext{})
	second.mu.Unlock()

	if !afterRestart.Pending || afterRestart.ShouldActivate {
		t.Fatalf("restart counted unobserved process downtime: %+v", afterRestart)
	}
	if !eligible.ShouldActivate {
		t.Fatalf("persisted plus post-restart elapsed time did not reach tolerance: %+v", eligible)
	}
}
