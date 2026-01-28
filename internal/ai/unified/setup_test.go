package unified

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestSetup_Defaults(t *testing.T) {
	result, err := Setup(SetupConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Integration == nil || result.Store == nil || result.Bridge == nil {
		t.Fatalf("expected setup components")
	}
	if result.Adapter != nil {
		t.Fatalf("expected nil adapter when no alert manager provided")
	}
}

func TestQuickSetup(t *testing.T) {
	manager := alerts.NewManager()
	result, err := QuickSetup(manager, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Adapter == nil {
		t.Fatalf("expected adapter with alert manager")
	}
}

func TestSetupWithPatrol(t *testing.T) {
	manager := alerts.NewManager()
	result, err := SetupWithPatrol(manager, t.TempDir(), func(resourceID, resourceType, reason, alertType string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Bridge == nil {
		t.Fatalf("expected bridge")
	}
}
