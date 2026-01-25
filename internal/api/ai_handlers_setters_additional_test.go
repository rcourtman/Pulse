package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/metrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestAISettingsHandler_SettersAndGetters(t *testing.T) {
	handler := &AISettingsHandler{}

	mtp := config.NewMultiTenantPersistence(t.TempDir())
	handler.SetMultiTenantPersistence(mtp)
	if handler.mtPersistence != mtp {
		t.Fatalf("mtPersistence not set")
	}

	mtm := &monitoring.MultiTenantMonitor{}
	handler.SetMultiTenantMonitor(mtm)
	if handler.mtMonitor != mtm {
		t.Fatalf("mtMonitor not set")
	}

	store := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
	handler.SetUnifiedStore(store)
	if handler.GetUnifiedStore() != store {
		t.Fatalf("GetUnifiedStore returned unexpected store")
	}

	bridge := unified.NewAlertBridge(store, unified.DefaultBridgeConfig())
	handler.SetAlertBridge(bridge)
	if handler.GetAlertBridge() != bridge {
		t.Fatalf("GetAlertBridge returned unexpected bridge")
	}

	triggerManager := ai.NewTriggerManager(ai.DefaultTriggerManagerConfig())
	handler.SetTriggerManager(triggerManager)
	if handler.GetTriggerManager() != triggerManager {
		t.Fatalf("GetTriggerManager returned unexpected manager")
	}

	coordinator := ai.NewIncidentCoordinator(ai.IncidentCoordinatorConfig{})
	handler.SetIncidentCoordinator(coordinator)
	if handler.GetIncidentCoordinator() != coordinator {
		t.Fatalf("GetIncidentCoordinator returned unexpected coordinator")
	}

	recorder := &metrics.IncidentRecorder{}
	handler.SetIncidentRecorder(recorder)
	if handler.GetIncidentRecorder() != recorder {
		t.Fatalf("GetIncidentRecorder returned unexpected recorder")
	}

	handler.WireOrchestratorAfterChatStart()
}
