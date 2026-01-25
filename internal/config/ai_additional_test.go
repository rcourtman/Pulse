package config

import "testing"

func TestAIConfig_DiscoveryAndControl(t *testing.T) {
	cfg := &AIConfig{}
	// Default discovery model should fall back to GetModel()
	if got := cfg.GetDiscoveryModel(); got != cfg.GetModel() {
		t.Fatalf("default discovery model should fall back to GetModel(), got %q", got)
	}

	cfg.DiscoveryModel = "custom:discovery"
	if got := cfg.GetDiscoveryModel(); got != "custom:discovery" {
		t.Fatalf("custom discovery model = %q", got)
	}

	cfg = &AIConfig{}
	if got := cfg.GetControlLevel(); got != ControlLevelReadOnly {
		t.Fatalf("default control level = %q", got)
	}

	cfg.AutonomousMode = true
	if got := cfg.GetControlLevel(); got != ControlLevelAutonomous {
		t.Fatalf("legacy autonomous mode = %q", got)
	}

	cfg.ControlLevel = "suggest"
	if got := cfg.GetControlLevel(); got != ControlLevelControlled {
		t.Fatalf("suggest control level = %q", got)
	}

	cfg.ControlLevel = "invalid"
	if got := cfg.GetControlLevel(); got != ControlLevelReadOnly {
		t.Fatalf("invalid control level = %q", got)
	}

	cfg.ControlLevel = ControlLevelControlled
	if !cfg.IsControlEnabled() {
		t.Fatalf("control should be enabled for controlled level")
	}
	if cfg.IsAutonomous() {
		t.Fatalf("autonomous should be false for controlled level")
	}
}

func TestAIConfig_PatrolSettings(t *testing.T) {
	cfg := &AIConfig{}
	if got := cfg.GetPatrolAutonomyLevel(); got != PatrolAutonomyMonitor {
		t.Fatalf("default patrol autonomy = %q", got)
	}
	if cfg.IsPatrolAutonomyEnabled() {
		t.Fatalf("patrol autonomy should be disabled by default")
	}

	cfg.PatrolAutonomyLevel = PatrolAutonomyFull
	if got := cfg.GetPatrolAutonomyLevel(); got != PatrolAutonomyFull {
		t.Fatalf("patrol autonomy = %q", got)
	}
	if !cfg.IsPatrolAutonomyEnabled() {
		t.Fatalf("patrol autonomy should be enabled for full mode")
	}

	cfg.PatrolAutonomyLevel = "invalid"
	if got := cfg.GetPatrolAutonomyLevel(); got != PatrolAutonomyMonitor {
		t.Fatalf("invalid autonomy should fallback to monitor, got %q", got)
	}

	cfg.PatrolInvestigationBudget = 2
	if got := cfg.GetPatrolInvestigationBudget(); got != 5 {
		t.Fatalf("budget should clamp to 5, got %d", got)
	}

	cfg.PatrolInvestigationBudget = 40
	if got := cfg.GetPatrolInvestigationBudget(); got != 30 {
		t.Fatalf("budget should clamp to 30, got %d", got)
	}

	cfg.PatrolInvestigationBudget = 10
	if got := cfg.GetPatrolInvestigationBudget(); got != 10 {
		t.Fatalf("budget should be 10, got %d", got)
	}

	cfg.PatrolInvestigationTimeoutSec = 30
	if got := cfg.GetPatrolInvestigationTimeout(); got.Seconds() != 60 {
		t.Fatalf("timeout should clamp to 60s, got %s", got)
	}

	cfg.PatrolInvestigationTimeoutSec = 1900
	if got := cfg.GetPatrolInvestigationTimeout(); got.Seconds() != 1800 {
		t.Fatalf("timeout should clamp to 1800s, got %s", got)
	}

	cfg.PatrolInvestigationTimeoutSec = 120
	if got := cfg.GetPatrolInvestigationTimeout(); got.Seconds() != 120 {
		t.Fatalf("timeout should be 120s, got %s", got)
	}

	cfg.PatrolAutonomyLevel = ""
	cfg.PatrolCriticalRequireApproval = false
	if !cfg.ShouldCriticalRequireApproval() {
		t.Fatalf("critical approval should default to true when level unset")
	}

	cfg.PatrolAutonomyLevel = PatrolAutonomyMonitor
	if cfg.ShouldCriticalRequireApproval() {
		t.Fatalf("critical approval should be false when explicitly disabled")
	}
}

func TestAIConfig_ProtectedGuestsAndValidation(t *testing.T) {
	cfg := &AIConfig{}
	if guests := cfg.GetProtectedGuests(); len(guests) != 0 {
		t.Fatalf("expected empty protected guests, got %v", guests)
	}

	cfg.ProtectedGuests = []string{"vm-100", "vm-200"}
	guests := cfg.GetProtectedGuests()
	if len(guests) != 2 || guests[0] != "vm-100" {
		t.Fatalf("unexpected protected guests: %v", guests)
	}

	if IsValidControlLevel("bad") {
		t.Fatalf("expected invalid control level to be false")
	}
	if !IsValidControlLevel(ControlLevelAutonomous) {
		t.Fatalf("expected autonomous to be valid")
	}
	if IsValidPatrolAutonomyLevel("bad") {
		t.Fatalf("expected invalid patrol autonomy to be false")
	}
	if !IsValidPatrolAutonomyLevel(PatrolAutonomyApproval) {
		t.Fatalf("expected patrol approval to be valid")
	}
}
