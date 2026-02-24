package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/investigation"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/learning"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/metrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
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

func TestAISettingsHandler_IntelligenceServicesAreOrgScoped(t *testing.T) {
	handler := &AISettingsHandler{}

	defaultCorrelator := &proxmox.EventCorrelator{}
	tenantCorrelator := &proxmox.EventCorrelator{}
	handler.SetProxmoxCorrelatorForOrg("default", defaultCorrelator)
	handler.SetProxmoxCorrelatorForOrg("acme", tenantCorrelator)
	if got := handler.GetProxmoxCorrelator(); got != defaultCorrelator {
		t.Fatalf("expected default correlator, got %#v", got)
	}
	if got := handler.GetProxmoxCorrelatorForOrg("acme"); got != tenantCorrelator {
		t.Fatalf("expected tenant correlator, got %#v", got)
	}
	if got := handler.GetProxmoxCorrelatorForOrg("other"); got != nil {
		t.Fatalf("expected nil correlator for unrelated org, got %#v", got)
	}

	defaultRecorder := &metrics.IncidentRecorder{}
	tenantRecorder := &metrics.IncidentRecorder{}
	handler.SetIncidentRecorderForOrg("default", defaultRecorder)
	handler.SetIncidentRecorderForOrg("acme", tenantRecorder)
	if got := handler.GetIncidentRecorder(); got != defaultRecorder {
		t.Fatalf("expected default recorder, got %#v", got)
	}
	if got := handler.GetIncidentRecorderForOrg("acme"); got != tenantRecorder {
		t.Fatalf("expected tenant recorder, got %#v", got)
	}

	defaultLearningStore := learning.NewLearningStore(learning.LearningStoreConfig{})
	tenantLearningStore := learning.NewLearningStore(learning.LearningStoreConfig{})
	handler.SetLearningStoreForOrg("default", defaultLearningStore)
	handler.SetLearningStoreForOrg("acme", tenantLearningStore)
	if got := handler.GetLearningStore(); got != defaultLearningStore {
		t.Fatalf("expected default learning store, got %#v", got)
	}
	if got := handler.GetLearningStoreForOrg("acme"); got != tenantLearningStore {
		t.Fatalf("expected tenant learning store, got %#v", got)
	}

	defaultForecastService := forecast.NewService(forecast.DefaultForecastConfig())
	tenantForecastService := forecast.NewService(forecast.DefaultForecastConfig())
	handler.SetForecastServiceForOrg("default", defaultForecastService)
	handler.SetForecastServiceForOrg("acme", tenantForecastService)
	if got := handler.GetForecastService(); got != defaultForecastService {
		t.Fatalf("expected default forecast service, got %#v", got)
	}
	if got := handler.GetForecastServiceForOrg("acme"); got != tenantForecastService {
		t.Fatalf("expected tenant forecast service, got %#v", got)
	}

	defaultRemediationEngine := remediation.NewEngine(remediation.DefaultEngineConfig())
	tenantRemediationEngine := remediation.NewEngine(remediation.DefaultEngineConfig())
	handler.SetRemediationEngineForOrg("default", defaultRemediationEngine)
	handler.SetRemediationEngineForOrg("acme", tenantRemediationEngine)
	if got := handler.GetRemediationEngine(); got != defaultRemediationEngine {
		t.Fatalf("expected default remediation engine, got %#v", got)
	}
	if got := handler.GetRemediationEngineForOrg("acme"); got != tenantRemediationEngine {
		t.Fatalf("expected tenant remediation engine, got %#v", got)
	}

	defaultIncidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	tenantIncidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	handler.SetIncidentStoreForOrg("default", defaultIncidentStore)
	handler.SetIncidentStoreForOrg("acme", tenantIncidentStore)
	if got := handler.GetIncidentStoreForOrg("default"); got != defaultIncidentStore {
		t.Fatalf("expected default incident store, got %#v", got)
	}
	if got := handler.GetIncidentStoreForOrg("acme"); got != tenantIncidentStore {
		t.Fatalf("expected tenant incident store, got %#v", got)
	}

	defaultBreaker := circuit.NewBreaker("default", circuit.DefaultConfig())
	tenantBreaker := circuit.NewBreaker("acme", circuit.DefaultConfig())
	handler.SetCircuitBreakerForOrg("default", defaultBreaker)
	handler.SetCircuitBreakerForOrg("acme", tenantBreaker)
	if got := handler.GetCircuitBreaker(); got != defaultBreaker {
		t.Fatalf("expected default circuit breaker, got %#v", got)
	}
	if got := handler.GetCircuitBreakerForOrg("acme"); got != tenantBreaker {
		t.Fatalf("expected tenant circuit breaker, got %#v", got)
	}

	defaultDiscoveryStore, err := servicediscovery.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore default: %v", err)
	}
	tenantDiscoveryStore, err := servicediscovery.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore tenant: %v", err)
	}
	handler.SetDiscoveryStoreForOrg("default", defaultDiscoveryStore)
	handler.SetDiscoveryStoreForOrg("acme", tenantDiscoveryStore)
	if got := handler.GetDiscoveryStore(); got != defaultDiscoveryStore {
		t.Fatalf("expected default discovery store, got %#v", got)
	}
	if got := handler.GetDiscoveryStoreForOrg("acme"); got != tenantDiscoveryStore {
		t.Fatalf("expected tenant discovery store, got %#v", got)
	}

	defaultUnifiedStore := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
	tenantUnifiedStore := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
	handler.SetUnifiedStoreForOrg("default", defaultUnifiedStore)
	handler.SetUnifiedStoreForOrg("acme", tenantUnifiedStore)
	if got := handler.GetUnifiedStore(); got != defaultUnifiedStore {
		t.Fatalf("expected default unified store, got %#v", got)
	}
	if got := handler.GetUnifiedStoreForOrg("acme"); got != tenantUnifiedStore {
		t.Fatalf("expected tenant unified store, got %#v", got)
	}
	handler.RemoveTenantIntelligence("acme")
	if got := handler.GetLearningStoreForOrg("acme"); got != nil {
		t.Fatalf("expected tenant learning store removed on tenant cleanup, got %#v", got)
	}
	if got := handler.GetForecastServiceForOrg("acme"); got != nil {
		t.Fatalf("expected tenant forecast service removed on tenant cleanup, got %#v", got)
	}
	if got := handler.GetRemediationEngineForOrg("acme"); got != nil {
		t.Fatalf("expected tenant remediation engine removed on tenant cleanup, got %#v", got)
	}
	if got := handler.GetIncidentStoreForOrg("acme"); got != nil {
		t.Fatalf("expected tenant incident store removed on tenant cleanup, got %#v", got)
	}
	if got := handler.GetCircuitBreakerForOrg("acme"); got != nil {
		t.Fatalf("expected tenant circuit breaker removed on tenant cleanup, got %#v", got)
	}
	if got := handler.GetDiscoveryStoreForOrg("acme"); got != nil {
		t.Fatalf("expected tenant discovery store removed on tenant cleanup, got %#v", got)
	}
	if got := handler.GetUnifiedStoreForOrg("acme"); got != nil {
		t.Fatalf("expected tenant unified store removed on tenant cleanup, got %#v", got)
	}
}

func TestAISettingsHandler_RemoveTenantService_TrimsOrgID(t *testing.T) {
	handler := &AISettingsHandler{
		aiServices:           map[string]*ai.Service{"acme": nil},
		investigationStores:  map[string]*investigation.Store{"acme": nil},
		proxmoxCorrelators:   map[string]*proxmox.EventCorrelator{"acme": nil},
		alertBridges:         map[string]*unified.AlertBridge{"acme": nil},
		triggerManagers:      map[string]*ai.TriggerManager{"acme": nil},
		incidentCoordinators: map[string]*ai.IncidentCoordinator{"acme": nil},
		incidentRecorders:    map[string]*metrics.IncidentRecorder{"acme": nil},
	}

	handler.RemoveTenantService("  acme  ")

	if _, ok := handler.aiServices["acme"]; ok {
		t.Fatalf("expected ai service entry to be removed")
	}
	if _, ok := handler.investigationStores["acme"]; ok {
		t.Fatalf("expected investigation store entry to be removed")
	}
	if _, ok := handler.proxmoxCorrelators["acme"]; ok {
		t.Fatalf("expected correlator entry to be removed")
	}
	if _, ok := handler.alertBridges["acme"]; ok {
		t.Fatalf("expected alert bridge entry to be removed")
	}
	if _, ok := handler.triggerManagers["acme"]; ok {
		t.Fatalf("expected trigger manager entry to be removed")
	}
	if _, ok := handler.incidentCoordinators["acme"]; ok {
		t.Fatalf("expected incident coordinator entry to be removed")
	}
	if _, ok := handler.incidentRecorders["acme"]; ok {
		t.Fatalf("expected incident recorder entry to be removed")
	}
}
