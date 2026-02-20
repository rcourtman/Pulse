package api

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/investigation"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/learning"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/conversion"
	"github.com/rcourtman/pulse-go-rewrite/internal/metrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// AISettingsHandler handles AI settings endpoints
type AISettingsHandler struct {
	mtPersistence           *config.MultiTenantPersistence
	mtMonitor               *monitoring.MultiTenantMonitor
	legacyConfig            *config.Config
	legacyPersistence       *config.ConfigPersistence
	legacyAIService         *ai.Service
	aiServices              map[string]*ai.Service
	aiServicesMu            sync.RWMutex
	agentServer             *agentexec.Server
	onModelChange           func() // Called when model or other AI chat-affecting settings change
	onControlSettingsChange func() // Called when control level or protected guests change

	// Providers to be applied to new services
	stateProvider           ai.StateProvider
	readState               unifiedresources.ReadState
	unifiedResourceProvider ai.UnifiedResourceProvider
	metadataProvider        ai.MetadataProvider
	patrolThresholdProvider ai.ThresholdProvider
	metricsHistoryProvider  ai.MetricsHistoryProvider
	baselineStore           *ai.BaselineStore
	changeDetector          *ai.ChangeDetector
	remediationLog          *ai.RemediationLog
	incidentStore           *memory.IncidentStore
	patternDetector         *ai.PatternDetector
	correlationDetector     *ai.CorrelationDetector
	licenseHandlers         *LicenseHandlers

	// New AI intelligence services (Phase 6)
	circuitBreaker    *circuit.Breaker         // Circuit breaker for resilient patrol
	learningStore     *learning.LearningStore  // Feedback learning
	forecastService   *forecast.Service        // Trend forecasting
	proxmoxCorrelator *proxmox.EventCorrelator // Proxmox event correlation
	remediationEngine *remediation.Engine      // AI-guided remediation
	unifiedStore      *unified.UnifiedStore    // Unified alert/finding store
	alertBridge       *unified.AlertBridge     // Bridge between alerts and unified store

	// Event-driven patrol (Phase 7)
	triggerManager      *ai.TriggerManager        // Event-driven patrol trigger manager
	incidentCoordinator *ai.IncidentCoordinator   // Incident recording coordinator
	incidentRecorder    *metrics.IncidentRecorder // High-frequency incident recorder

	// Investigation orchestration (Patrol Autonomy)
	chatHandler         *AIHandler                      // Chat service handler for investigations
	investigationStores map[string]*investigation.Store // Investigation stores per org
	investigationMu     sync.RWMutex

	// Discovery store for deep infrastructure discovery
	discoveryStore *servicediscovery.Store
}

// NewAISettingsHandler creates a new AI settings handler
func NewAISettingsHandler(mtp *config.MultiTenantPersistence, mtm *monitoring.MultiTenantMonitor, agentServer *agentexec.Server) *AISettingsHandler {
	var defaultConfig *config.Config
	var defaultPersistence *config.ConfigPersistence
	var defaultAIService *ai.Service

	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil && m != nil {
			defaultConfig = m.GetConfig()
		}
	}
	if mtp != nil {
		if p, err := mtp.GetPersistence("default"); err == nil {
			defaultPersistence = p
		}
	}

	if defaultPersistence != nil {
		defaultAIService = ai.NewService(defaultPersistence, agentServer)
		if err := defaultAIService.LoadConfig(); err != nil {
			log.Warn().Err(err).Msg("Failed to load AI config on startup")
		}
	}

	return &AISettingsHandler{
		mtPersistence:     mtp,
		mtMonitor:         mtm,
		legacyConfig:      defaultConfig,
		legacyPersistence: defaultPersistence,
		legacyAIService:   defaultAIService,
		aiServices:        make(map[string]*ai.Service),
		agentServer:       agentServer,
	}
}

// GetAIService returns the underlying AI service
func (h *AISettingsHandler) GetAIService(ctx context.Context) *ai.Service {
	orgID := GetOrgID(ctx)
	if orgID == "default" || orgID == "" {
		return h.legacyAIService
	}

	h.aiServicesMu.RLock()
	svc, exists := h.aiServices[orgID]
	h.aiServicesMu.RUnlock()

	if exists {
		return svc
	}

	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()

	// Double check
	if svc, exists = h.aiServices[orgID]; exists {
		return svc
	}

	// Create new service for this tenant
	persistence, err := h.mtPersistence.GetPersistence(orgID)
	if err != nil {
		log.Warn().Str("orgID", orgID).Err(err).Msg("Failed to get persistence for AI service")
		return h.legacyAIService
	}

	svc = ai.NewService(persistence, h.agentServer)
	if err := svc.LoadConfig(); err != nil {
		log.Warn().Str("orgID", orgID).Err(err).Msg("Failed to load AI config for tenant")
	}

	// Set providers on new service
	if h.stateProvider != nil {
		svc.SetStateProvider(h.stateProvider)
	}
	if h.readState != nil {
		svc.SetReadState(h.readState)
	}
	if h.unifiedResourceProvider != nil {
		svc.SetUnifiedResourceProvider(h.unifiedResourceProvider)
	}
	if h.metadataProvider != nil {
		svc.SetMetadataProvider(h.metadataProvider)
	}
	if h.patrolThresholdProvider != nil {
		svc.SetPatrolThresholdProvider(h.patrolThresholdProvider)
	}
	if h.metricsHistoryProvider != nil {
		svc.SetMetricsHistoryProvider(h.metricsHistoryProvider)
	}
	if h.baselineStore != nil {
		svc.SetBaselineStore(h.baselineStore)
	}
	if h.changeDetector != nil {
		svc.SetChangeDetector(h.changeDetector)
	}
	if h.remediationLog != nil {
		svc.SetRemediationLog(h.remediationLog)
	}
	if h.incidentStore != nil {
		svc.SetIncidentStore(h.incidentStore)
	}
	if h.patternDetector != nil {
		svc.SetPatternDetector(h.patternDetector)
	}
	if h.correlationDetector != nil {
		svc.SetCorrelationDetector(h.correlationDetector)
	}
	if h.discoveryStore != nil {
		svc.SetDiscoveryStore(h.discoveryStore)
	}

	// Set license checker if handler available
	if h.licenseHandlers != nil {
		// Used context to resolve tenant license service
		if licSvc, _, err := h.licenseHandlers.getTenantComponents(ctx); err == nil {
			svc.SetLicenseChecker(licSvc)
		}
	}

	// Set up investigation orchestrator if chat handler is available
	if h.chatHandler != nil {
		h.setupInvestigationOrchestrator(orgID, svc)
	}

	h.aiServices[orgID] = svc
	return svc
}

// RemoveTenantService removes the AI settings service for a specific tenant.
func (h *AISettingsHandler) RemoveTenantService(orgID string) {
	if orgID == "default" || orgID == "" {
		return
	}

	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()
	delete(h.aiServices, orgID)
	log.Debug().Str("orgID", orgID).Msg("Removed AI settings service for tenant")
}

// getConfig returns the config for the current context
func (h *AISettingsHandler) getConfig(ctx context.Context) *config.Config {
	orgID := GetOrgID(ctx)
	if h.mtMonitor != nil {
		if m, err := h.mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			return m.GetConfig()
		}
	}
	return h.legacyConfig
}

// GetPersistence returns the persistence for the current context
func (h *AISettingsHandler) getPersistence(ctx context.Context) *config.ConfigPersistence {
	orgID := GetOrgID(ctx)
	if h.mtPersistence != nil {
		if p, err := h.mtPersistence.GetPersistence(orgID); err == nil {
			return p
		}
	}
	return h.legacyPersistence
}

// SetMultiTenantPersistence updates the persistence manager
func (h *AISettingsHandler) SetMultiTenantPersistence(mtp *config.MultiTenantPersistence) {
	h.mtPersistence = mtp
}

// SetMultiTenantMonitor updates the monitor manager
func (h *AISettingsHandler) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	h.mtMonitor = mtm
}

// SetConfig updates the configuration reference used by the handler.
func (h *AISettingsHandler) SetConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	h.legacyConfig = cfg
}

// setSSECORSHeaders validates the request origin against the configured AllowedOrigins
// and sets CORS headers only for allowed origins. This prevents arbitrary origin reflection.
func (h *AISettingsHandler) setSSECORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}

	cfg := h.getConfig(r.Context())
	if cfg == nil {
		return
	}

	allowed := cfg.AllowedOrigins
	if allowed == "" {
		return
	}

	applyConfiguredCORSHeaders(w, origin, allowed, "GET, POST, OPTIONS", "Content-Type, Accept, Cookie")
}

// SetStateProvider sets the state provider for infrastructure context
func (h *AISettingsHandler) SetStateProvider(sp ai.StateProvider) {
	h.stateProvider = sp
	h.legacyAIService.SetStateProvider(sp)

	h.aiServicesMu.Lock()
	for _, svc := range h.aiServices {
		svc.SetStateProvider(sp)
	}
	h.aiServicesMu.Unlock()

	// Now that state provider is set, patrol service should be available.
	// Try to set up the investigation orchestrator if chat handler is ready.
	// Note: This usually fails because chat service isn't started yet.
	// The orchestrator will be wired via WireOrchestratorAfterChatStart() instead.
	if h.chatHandler != nil {
		h.setupInvestigationOrchestrator("default", h.legacyAIService)
		h.aiServicesMu.RLock()
		for orgID, svc := range h.aiServices {
			h.setupInvestigationOrchestrator(orgID, svc)
		}
		h.aiServicesMu.RUnlock()
	}
}

// GetStateProvider returns the state provider for infrastructure context
func (h *AISettingsHandler) GetStateProvider() ai.StateProvider {
	return h.stateProvider
}

// SetReadState injects unified read-state context into AI services (patrol path).
func (h *AISettingsHandler) SetReadState(rs unifiedresources.ReadState) {
	if h == nil {
		return
	}
	h.readState = rs

	if h.legacyAIService != nil {
		h.legacyAIService.SetReadState(rs)
	}

	h.aiServicesMu.Lock()
	for _, svc := range h.aiServices {
		if svc != nil {
			svc.SetReadState(rs)
		}
	}
	h.aiServicesMu.Unlock()
}

// SetMetadataProvider sets the metadata provider for AI URL discovery
func (h *AISettingsHandler) SetMetadataProvider(mp ai.MetadataProvider) {
	h.metadataProvider = mp
	h.legacyAIService.SetMetadataProvider(mp)

	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()
	for _, svc := range h.aiServices {
		svc.SetMetadataProvider(mp)
	}
}

// StartPatrol starts the background AI patrol service
func (h *AISettingsHandler) StartPatrol(ctx context.Context) {
	h.GetAIService(ctx).StartPatrol(ctx)
}

// IsAIEnabled returns true if AI features are enabled
func (h *AISettingsHandler) IsAIEnabled(ctx context.Context) bool {
	return h.GetAIService(ctx).IsEnabled()
}

// SetPatrolThresholdProvider sets the threshold provider for the patrol service
func (h *AISettingsHandler) SetPatrolThresholdProvider(provider ai.ThresholdProvider) {
	h.patrolThresholdProvider = provider
	h.legacyAIService.SetPatrolThresholdProvider(provider)

	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()
	for _, svc := range h.aiServices {
		svc.SetPatrolThresholdProvider(provider)
	}
}

// SetPatrolFindingsPersistence enables findings persistence for the patrol service
func (h *AISettingsHandler) SetPatrolFindingsPersistence(persistence ai.FindingsPersistence) error {
	var firstErr error
	if patrol := h.legacyAIService.GetPatrolService(); patrol != nil {
		if err := patrol.SetFindingsPersistence(persistence); err != nil {
			firstErr = err
		}
	}
	// Also apply to active services
	h.aiServicesMu.RLock()
	defer h.aiServicesMu.RUnlock()
	for orgID, svc := range h.aiServices {
		if patrol := svc.GetPatrolService(); patrol != nil {
			if err := patrol.SetFindingsPersistence(persistence); err != nil {
				log.Warn().Str("orgID", orgID).Err(err).Msg("Failed to set findings persistence for tenant")
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	return firstErr
}

// SetPatrolRunHistoryPersistence enables patrol run history persistence for the patrol service
func (h *AISettingsHandler) SetPatrolRunHistoryPersistence(persistence ai.PatrolHistoryPersistence) error {
	var firstErr error
	if patrol := h.legacyAIService.GetPatrolService(); patrol != nil {
		if err := patrol.SetRunHistoryPersistence(persistence); err != nil {
			firstErr = err
		}
	}
	// Also apply to active services
	h.aiServicesMu.RLock()
	defer h.aiServicesMu.RUnlock()
	for orgID, svc := range h.aiServices {
		if patrol := svc.GetPatrolService(); patrol != nil {
			if err := patrol.SetRunHistoryPersistence(persistence); err != nil {
				log.Warn().Str("orgID", orgID).Err(err).Msg("Failed to set run history persistence for tenant")
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	return firstErr
}

// SetMetricsHistoryProvider sets the metrics history provider for enriched AI context
func (h *AISettingsHandler) SetMetricsHistoryProvider(provider ai.MetricsHistoryProvider) {
	h.metricsHistoryProvider = provider
	h.legacyAIService.SetMetricsHistoryProvider(provider)

	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()
	for _, svc := range h.aiServices {
		svc.SetMetricsHistoryProvider(provider)
	}
}

// SetBaselineStore sets the baseline store for anomaly detection
func (h *AISettingsHandler) SetBaselineStore(store *ai.BaselineStore) {
	h.baselineStore = store
	h.legacyAIService.SetBaselineStore(store)

	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()
	for _, svc := range h.aiServices {
		svc.SetBaselineStore(store)
	}
}

// SetChangeDetector sets the change detector for operational memory
func (h *AISettingsHandler) SetChangeDetector(detector *ai.ChangeDetector) {
	h.changeDetector = detector
	h.legacyAIService.SetChangeDetector(detector)

	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()
	for _, svc := range h.aiServices {
		svc.SetChangeDetector(detector)
	}
}

// SetRemediationLog sets the remediation log for tracking fix attempts
func (h *AISettingsHandler) SetRemediationLog(remLog *ai.RemediationLog) {
	h.remediationLog = remLog
	h.legacyAIService.SetRemediationLog(remLog)

	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()
	for _, svc := range h.aiServices {
		svc.SetRemediationLog(remLog)
	}
}

// SetIncidentStore sets the incident store for alert timelines.
func (h *AISettingsHandler) SetIncidentStore(store *memory.IncidentStore) {
	h.incidentStore = store
	h.legacyAIService.SetIncidentStore(store)

	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()
	for _, svc := range h.aiServices {
		svc.SetIncidentStore(store)
	}
}

// SetPatternDetector sets the pattern detector for failure prediction
func (h *AISettingsHandler) SetPatternDetector(detector *ai.PatternDetector) {
	h.patternDetector = detector
	h.legacyAIService.SetPatternDetector(detector)

	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()
	for _, svc := range h.aiServices {
		svc.SetPatternDetector(detector)
	}
}

// SetCorrelationDetector sets the correlation detector for multi-resource correlation
func (h *AISettingsHandler) SetCorrelationDetector(detector *ai.CorrelationDetector) {
	h.correlationDetector = detector
	h.legacyAIService.SetCorrelationDetector(detector)

	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()
	for _, svc := range h.aiServices {
		svc.SetCorrelationDetector(detector)
	}
}

// SetCircuitBreaker sets the circuit breaker for resilient patrol
func (h *AISettingsHandler) SetCircuitBreaker(breaker *circuit.Breaker) {
	h.circuitBreaker = breaker
}

// GetCircuitBreaker returns the circuit breaker
func (h *AISettingsHandler) GetCircuitBreaker() *circuit.Breaker {
	return h.circuitBreaker
}

// SetLearningStore sets the learning store for feedback learning
func (h *AISettingsHandler) SetLearningStore(store *learning.LearningStore) {
	h.learningStore = store
}

// GetLearningStore returns the learning store
func (h *AISettingsHandler) GetLearningStore() *learning.LearningStore {
	return h.learningStore
}

// SetForecastService sets the forecast service for trend forecasting
func (h *AISettingsHandler) SetForecastService(svc *forecast.Service) {
	h.forecastService = svc
}

// GetForecastService returns the forecast service
func (h *AISettingsHandler) GetForecastService() *forecast.Service {
	return h.forecastService
}

// SetProxmoxCorrelator sets the Proxmox event correlator
func (h *AISettingsHandler) SetProxmoxCorrelator(correlator *proxmox.EventCorrelator) {
	h.proxmoxCorrelator = correlator
}

// GetProxmoxCorrelator returns the Proxmox event correlator
func (h *AISettingsHandler) GetProxmoxCorrelator() *proxmox.EventCorrelator {
	return h.proxmoxCorrelator
}

// SetRemediationEngine sets the remediation engine for AI-guided fixes
func (h *AISettingsHandler) SetRemediationEngine(engine *remediation.Engine) {
	h.remediationEngine = engine
}

// GetRemediationEngine returns the remediation engine
func (h *AISettingsHandler) GetRemediationEngine() *remediation.Engine {
	return h.remediationEngine
}

// SetUnifiedStore sets the unified store
func (h *AISettingsHandler) SetUnifiedStore(store *unified.UnifiedStore) {
	h.unifiedStore = store
}

// GetUnifiedStore returns the unified store
func (h *AISettingsHandler) GetUnifiedStore() *unified.UnifiedStore {
	return h.unifiedStore
}

// SetDiscoveryStore sets the discovery store for deep infrastructure discovery
func (h *AISettingsHandler) SetDiscoveryStore(store *servicediscovery.Store) {
	h.discoveryStore = store
	// Also set on legacy service if it exists
	if h.legacyAIService != nil {
		h.legacyAIService.SetDiscoveryStore(store)
	}
	// Set on all existing tenant services
	h.aiServicesMu.RLock()
	defer h.aiServicesMu.RUnlock()
	for _, svc := range h.aiServices {
		svc.SetDiscoveryStore(store)
	}
}

// GetDiscoveryStore returns the discovery store
func (h *AISettingsHandler) GetDiscoveryStore() *servicediscovery.Store {
	return h.discoveryStore
}

// SetAlertBridge sets the alert bridge
func (h *AISettingsHandler) SetAlertBridge(bridge *unified.AlertBridge) {
	h.alertBridge = bridge
}

// GetAlertBridge returns the alert bridge
func (h *AISettingsHandler) GetAlertBridge() *unified.AlertBridge {
	return h.alertBridge
}

// SetTriggerManager sets the event-driven patrol trigger manager
func (h *AISettingsHandler) SetTriggerManager(tm *ai.TriggerManager) {
	h.triggerManager = tm
}

// GetTriggerManager returns the event-driven patrol trigger manager
func (h *AISettingsHandler) GetTriggerManager() *ai.TriggerManager {
	return h.triggerManager
}

// SetIncidentCoordinator sets the incident recording coordinator
func (h *AISettingsHandler) SetIncidentCoordinator(coordinator *ai.IncidentCoordinator) {
	h.incidentCoordinator = coordinator
}

// GetIncidentCoordinator returns the incident recording coordinator
func (h *AISettingsHandler) GetIncidentCoordinator() *ai.IncidentCoordinator {
	return h.incidentCoordinator
}

// SetIncidentRecorder sets the high-frequency incident recorder
func (h *AISettingsHandler) SetIncidentRecorder(recorder *metrics.IncidentRecorder) {
	h.incidentRecorder = recorder
}

// GetIncidentRecorder returns the high-frequency incident recorder
func (h *AISettingsHandler) GetIncidentRecorder() *metrics.IncidentRecorder {
	return h.incidentRecorder
}

// StopPatrol stops the background AI patrol service
func (h *AISettingsHandler) StopPatrol() {
	h.legacyAIService.StopPatrol()
	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()
	for _, svc := range h.aiServices {
		svc.StopPatrol()
	}
}

// GetAlertTriggeredAnalyzer returns the alert-triggered analyzer for wiring into alert callbacks
func (h *AISettingsHandler) GetAlertTriggeredAnalyzer(ctx context.Context) *ai.AlertTriggeredAnalyzer {
	return h.GetAIService(ctx).GetAlertTriggeredAnalyzer()
}

// SetLicenseHandlers sets the license handlers for Pro feature gating
func (h *AISettingsHandler) SetLicenseHandlers(handlers *LicenseHandlers) {
	h.licenseHandlers = handlers
	// Update legacy service?
	// legacy service needs a legacy/default license checker?
	// We can try to get it using background context (default tenant)
	if svc, _, err := handlers.getTenantComponents(context.Background()); err == nil {
		h.legacyAIService.SetLicenseChecker(svc)
	}
}

// SetOnModelChange sets a callback to be invoked when model settings change
// Used by Router to trigger AI chat service restart
func (h *AISettingsHandler) SetOnModelChange(callback func()) {
	h.onModelChange = callback
}

// SetOnControlSettingsChange sets a callback to be invoked when control settings change
// Used by Router to update MCP tool visibility without restarting AI chat
func (h *AISettingsHandler) SetOnControlSettingsChange(callback func()) {
	h.onControlSettingsChange = callback
}

// SetChatHandler sets the chat handler for investigation orchestration
// This enables the patrol service to spawn chat sessions to investigate findings
func (h *AISettingsHandler) SetChatHandler(chatHandler *AIHandler) {
	h.chatHandler = chatHandler
	h.investigationMu.Lock()
	if h.investigationStores == nil {
		h.investigationStores = make(map[string]*investigation.Store)
	}
	h.investigationMu.Unlock()

	// Wire up orchestrator for the legacy service
	// Note: This usually fails because chat service isn't started yet.
	// The orchestrator will be wired via WireOrchestratorAfterChatStart() instead.
	if h.legacyAIService != nil {
		h.setupInvestigationOrchestrator("default", h.legacyAIService)
	}

	// Wire up orchestrator for any existing services
	h.aiServicesMu.RLock()
	for orgID, svc := range h.aiServices {
		h.setupInvestigationOrchestrator(orgID, svc)
	}
	h.aiServicesMu.RUnlock()
}

// WireOrchestratorAfterChatStart is called after the chat service is started
// to wire up the investigation orchestrator. This must be called after aiHandler.Start()
// because the orchestrator needs an active chat service.
func (h *AISettingsHandler) WireOrchestratorAfterChatStart() {
	if h.chatHandler == nil {
		log.Warn().Msg("WireOrchestratorAfterChatStart called but chatHandler is nil")
		return
	}

	// Wire up orchestrator for the legacy service
	if h.legacyAIService != nil {
		h.setupInvestigationOrchestrator("default", h.legacyAIService)
	}

	// Wire up orchestrator for any existing services
	h.aiServicesMu.RLock()
	for orgID, svc := range h.aiServices {
		h.setupInvestigationOrchestrator(orgID, svc)
	}
	h.aiServicesMu.RUnlock()
}

// setupInvestigationOrchestrator creates and wires the investigation orchestrator for an AI service
func (h *AISettingsHandler) setupInvestigationOrchestrator(orgID string, svc *ai.Service) {
	if h.chatHandler == nil {
		log.Debug().Str("orgID", orgID).Msg("Chat handler not set, skipping orchestrator setup")
		return
	}

	patrol := svc.GetPatrolService()
	if patrol == nil {
		log.Debug().Str("orgID", orgID).Msg("Patrol service not available, skipping orchestrator setup")
		return
	}

	// Get or create investigation store for this org
	h.investigationMu.Lock()
	store, exists := h.investigationStores[orgID]
	if !exists {
		// Get data directory from persistence
		var dataDir string
		if h.legacyPersistence != nil && orgID == "default" {
			dataDir = h.legacyPersistence.DataDir()
		} else if h.mtPersistence != nil {
			if p, err := h.mtPersistence.GetPersistence(orgID); err == nil {
				dataDir = p.DataDir()
			}
		}
		store = investigation.NewStore(dataDir)
		if err := store.LoadFromDisk(); err != nil {
			log.Warn().Err(err).Str("orgID", orgID).Msg("Failed to load investigation store")
		}
		h.investigationStores[orgID] = store
	}
	h.investigationMu.Unlock()

	// Get chat service for this org using org-scoped context
	ctx := context.WithValue(context.Background(), OrgIDContextKey, orgID)
	chatSvc := h.chatHandler.GetService(ctx)
	if chatSvc == nil {
		log.Warn().Str("orgID", orgID).Msg("Chat service not available for orchestrator")
		return
	}

	// Create chat adapter - need to cast to *chat.Service
	chatService, ok := chatSvc.(*chat.Service)
	if !ok {
		log.Warn().Str("orgID", orgID).Msg("Chat service is not *chat.Service, cannot create adapter")
		return
	}

	// Mirror default-org router wiring for per-org services so patrol/investigation
	// executions always use the chat backend path with mid-run budget enforcement.
	svc.SetChatService(&chatServiceAdapter{svc: chatService})
	chatService.SetBudgetChecker(func() error {
		return svc.CheckBudget("patrol")
	})

	chatAdapter := investigation.NewChatServiceAdapter(chatService)

	// Create findings store adapter
	findingsStore := patrol.GetFindings()
	if findingsStore == nil {
		log.Warn().Str("orgID", orgID).Msg("Findings store not available for orchestrator")
		return
	}
	findingsStoreWrapper := &findingsStoreWrapper{store: findingsStore}
	findingsAdapter := investigation.NewFindingsStoreAdapter(findingsStoreWrapper)

	// Create approval adapter from the global approval store
	var approvalAdapter *investigation.ApprovalAdapter
	if approvalStore := approval.GetStore(); approvalStore != nil {
		approvalAdapter = investigation.NewApprovalAdapter(approvalStore)
	}

	// Get config for investigation settings
	cfg := svc.GetConfig()
	invConfig := investigation.DefaultConfig()
	if cfg != nil {
		invConfig.MaxTurns = cfg.GetPatrolInvestigationBudget()
		invConfig.Timeout = cfg.GetPatrolInvestigationTimeout()
	}

	// Create orchestrator
	orchestrator := investigation.NewOrchestrator(
		chatAdapter,
		store,
		findingsAdapter,
		approvalAdapter,
		invConfig,
	)

	// Set command executor for auto-executing fixes in full autonomy mode
	// The chatAdapter implements both ChatService and CommandExecutor interfaces
	orchestrator.SetCommandExecutor(chatAdapter)

	// Set autonomy level provider for re-checking before fix execution
	// This handles cases where user changes autonomy level during an investigation
	orchestrator.SetAutonomyLevelProvider(&autonomyLevelProviderAdapter{svc: svc})

	// Set infrastructure context provider for CLI access information
	// This enables investigations to know where services run (Docker, systemd, native)
	// and propose correct commands (e.g., 'docker exec pbs proxmox-backup-manager ...')
	if knowledgeStore := svc.GetKnowledgeStore(); knowledgeStore != nil {
		// Wire up discovery context to the knowledge store
		// This unifies deep-scanned discovery data with legacy knowledge notes
		if discoveryService := svc.GetDiscoveryService(); discoveryService != nil {
			knowledgeStore.SetDiscoveryContextProvider(func() string {
				discoveries, err := discoveryService.ListDiscoveries()
				if err != nil || len(discoveries) == 0 {
					return ""
				}
				return servicediscovery.FormatForAIContext(discoveries)
			})
			knowledgeStore.SetDiscoveryContextProviderForResources(func(resourceIDs []string) string {
				if len(resourceIDs) == 0 {
					return ""
				}
				discoveries, err := discoveryService.ListDiscoveries()
				if err != nil || len(discoveries) == 0 {
					return ""
				}
				filtered := servicediscovery.FilterDiscoveriesByResourceIDs(discoveries, resourceIDs)
				return servicediscovery.FormatForAIContext(filtered)
			})
		}
		orchestrator.SetInfrastructureContextProvider(knowledgeStore)
	}

	// Create adapter to bridge investigation.Orchestrator to ai.InvestigationOrchestrator interface
	adapter := ai.NewInvestigationOrchestratorAdapter(orchestrator)

	// Set on patrol service
	patrol.SetInvestigationOrchestrator(adapter)

	// Wire up fix verification: patrol re-checks resources after fixes are executed
	adapter.SetFixVerifier(patrol)

	// Wire up Prometheus metrics for investigation outcomes and fix verification
	adapter.SetMetricsCallback()

	// Wire up license checker for defense-in-depth autonomy clamping
	// This prevents auto-fix execution even if autonomy level was somehow set to assisted/full without Pro
	adapter.SetLicenseChecker(&licenseCheckerForOrchestrator{svc: svc})

	log.Info().Str("orgID", orgID).Msg("Investigation orchestrator configured for patrol service")
}

// licenseCheckerForOrchestrator adapts *ai.Service to investigation.LicenseChecker
type licenseCheckerForOrchestrator struct {
	svc *ai.Service
}

func (l *licenseCheckerForOrchestrator) HasFeature(feature string) bool {
	return l.svc.HasLicenseFeature(feature)
}

// findingsStoreWrapper wraps *ai.FindingsStore to implement investigation.AIFindingsStore
type findingsStoreWrapper struct {
	store *ai.FindingsStore
}

func (w *findingsStoreWrapper) Get(id string) investigation.AIFinding {
	if w.store == nil {
		return nil
	}
	f := w.store.Get(id)
	if f == nil {
		return nil
	}
	return f
}

func (w *findingsStoreWrapper) UpdateInvestigation(id, sessionID, status, outcome string, lastInvestigatedAt *time.Time, attempts int) bool {
	if w.store == nil {
		return false
	}
	return w.store.UpdateInvestigation(id, sessionID, status, outcome, lastInvestigatedAt, attempts)
}

// autonomyLevelProviderAdapter provides current autonomy level from config for re-checking before fix execution
type autonomyLevelProviderAdapter struct {
	svc *ai.Service
}

func (a *autonomyLevelProviderAdapter) GetCurrentAutonomyLevel() string {
	if a.svc == nil {
		return config.PatrolAutonomyMonitor
	}
	return a.svc.GetEffectivePatrolAutonomyLevel()
}

func (a *autonomyLevelProviderAdapter) IsFullModeUnlocked() bool {
	if a.svc == nil {
		return false
	}
	cfg := a.svc.GetConfig()
	if cfg == nil {
		return false
	}
	return cfg.PatrolFullModeUnlocked
}

// AISettingsResponse is returned by GET /api/settings/ai
// API keys are masked for security
type AISettingsResponse struct {
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`    // DEPRECATED: legacy single provider
	APIKeySet      bool   `json:"api_key_set"` // DEPRECATED: true if legacy API key is configured
	Model          string `json:"model"`
	ChatModel      string `json:"chat_model,omitempty"`     // Model for interactive chat (empty = use default)
	PatrolModel    string `json:"patrol_model,omitempty"`   // Model for patrol (empty = use default)
	AutoFixModel   string `json:"auto_fix_model,omitempty"` // Model for auto-fix (empty = use patrol model)
	BaseURL        string `json:"base_url,omitempty"`       // DEPRECATED: legacy base URL
	Configured     bool   `json:"configured"`               // true if AI is ready to use
	AutonomousMode bool   `json:"autonomous_mode"`          // true if AI can execute without approval
	CustomContext  string `json:"custom_context"`           // user-provided infrastructure context
	// OAuth fields for Claude Pro/Max subscription authentication
	AuthMethod     string `json:"auth_method"`     // "api_key" or "oauth"
	OAuthConnected bool   `json:"oauth_connected"` // true if OAuth tokens are configured
	// Patrol settings for token efficiency
	PatrolSchedulePreset   string                `json:"patrol_schedule_preset"`   // DEPRECATED: legacy preset
	PatrolIntervalMinutes  int                   `json:"patrol_interval_minutes"`  // Patrol interval in minutes (0 = disabled)
	PatrolEnabled          bool                  `json:"patrol_enabled"`           // true if patrol is enabled
	PatrolAutoFix          bool                  `json:"patrol_auto_fix"`          // true if patrol can auto-fix issues
	AlertTriggeredAnalysis bool                  `json:"alert_triggered_analysis"` // true if AI analyzes when alerts fire
	UseProactiveThresholds bool                  `json:"use_proactive_thresholds"` // true if patrol warns before thresholds (false = use exact thresholds)
	AvailableModels        []providers.ModelInfo `json:"available_models"`         // List of models for current provider
	// Multi-provider credentials - shows which providers are configured
	AnthropicConfigured  bool     `json:"anthropic_configured"`      // true if Anthropic API key or OAuth is set
	OpenAIConfigured     bool     `json:"openai_configured"`         // true if OpenAI API key is set
	OpenRouterConfigured bool     `json:"openrouter_configured"`     // true if OpenRouter API key is set
	DeepSeekConfigured   bool     `json:"deepseek_configured"`       // true if DeepSeek API key is set
	GeminiConfigured     bool     `json:"gemini_configured"`         // true if Gemini API key is set
	OllamaConfigured     bool     `json:"ollama_configured"`         // true (always available for attempt)
	OllamaBaseURL        string   `json:"ollama_base_url"`           // Ollama server URL
	OpenAIBaseURL        string   `json:"openai_base_url,omitempty"` // Custom OpenAI base URL
	ConfiguredProviders  []string `json:"configured_providers"`      // List of provider names with credentials
	// Cost controls
	CostBudgetUSD30d float64 `json:"cost_budget_usd_30d,omitempty"`
	// Request timeout (seconds) - for slow hardware running local models
	RequestTimeoutSeconds int `json:"request_timeout_seconds,omitempty"`
	// Infrastructure control settings
	ControlLevel    string   `json:"control_level"`              // "read_only", "controlled", "autonomous"
	ProtectedGuests []string `json:"protected_guests,omitempty"` // VMIDs/names that AI cannot control
	// Discovery settings
	DiscoveryEnabled       bool `json:"discovery_enabled"`                  // true if discovery is enabled
	DiscoveryIntervalHours int  `json:"discovery_interval_hours,omitempty"` // Hours between auto-scans (0 = manual only)
}

// AISettingsUpdateRequest is the request body for PUT /api/settings/ai
type AISettingsUpdateRequest struct {
	Enabled        *bool   `json:"enabled,omitempty"`
	Provider       *string `json:"provider,omitempty"` // DEPRECATED: use model selection instead
	APIKey         *string `json:"api_key,omitempty"`  // DEPRECATED: use per-provider keys
	Model          *string `json:"model,omitempty"`
	ChatModel      *string `json:"chat_model,omitempty"`     // Model for interactive chat
	PatrolModel    *string `json:"patrol_model,omitempty"`   // Model for background patrol
	AutoFixModel   *string `json:"auto_fix_model,omitempty"` // Model for auto-fix remediation
	BaseURL        *string `json:"base_url,omitempty"`       // DEPRECATED: use per-provider URLs
	AutonomousMode *bool   `json:"autonomous_mode,omitempty"`
	CustomContext  *string `json:"custom_context,omitempty"` // user-provided infrastructure context
	AuthMethod     *string `json:"auth_method,omitempty"`    // "api_key" or "oauth"
	// Patrol settings for token efficiency
	PatrolSchedulePreset   *string `json:"patrol_schedule_preset,omitempty"`   // DEPRECATED: use patrol_interval_minutes
	PatrolIntervalMinutes  *int    `json:"patrol_interval_minutes,omitempty"`  // Custom interval in minutes (0 = disabled, minimum 10)
	PatrolEnabled          *bool   `json:"patrol_enabled,omitempty"`           // true if patrol is enabled
	PatrolAutoFix          *bool   `json:"patrol_auto_fix,omitempty"`          // true if patrol can auto-fix issues
	AlertTriggeredAnalysis *bool   `json:"alert_triggered_analysis,omitempty"` // true if AI analyzes when alerts fire
	UseProactiveThresholds *bool   `json:"use_proactive_thresholds,omitempty"` // true if patrol warns before thresholds (default: false = exact thresholds)
	// Multi-provider credentials
	AnthropicAPIKey  *string `json:"anthropic_api_key,omitempty"`  // Set Anthropic API key
	OpenAIAPIKey     *string `json:"openai_api_key,omitempty"`     // Set OpenAI API key
	OpenRouterAPIKey *string `json:"openrouter_api_key,omitempty"` // Set OpenRouter API key
	DeepSeekAPIKey   *string `json:"deepseek_api_key,omitempty"`   // Set DeepSeek API key
	GeminiAPIKey     *string `json:"gemini_api_key,omitempty"`     // Set Gemini API key
	OllamaBaseURL    *string `json:"ollama_base_url,omitempty"`    // Set Ollama server URL
	OpenAIBaseURL    *string `json:"openai_base_url,omitempty"`    // Set custom OpenAI base URL
	// Clear flags for removing credentials
	ClearAnthropicKey  *bool `json:"clear_anthropic_key,omitempty"`  // Clear Anthropic API key
	ClearOpenAIKey     *bool `json:"clear_openai_key,omitempty"`     // Clear OpenAI API key
	ClearOpenRouterKey *bool `json:"clear_openrouter_key,omitempty"` // Clear OpenRouter API key
	ClearDeepSeekKey   *bool `json:"clear_deepseek_key,omitempty"`   // Clear DeepSeek API key
	ClearGeminiKey     *bool `json:"clear_gemini_key,omitempty"`     // Clear Gemini API key
	ClearOllamaURL     *bool `json:"clear_ollama_url,omitempty"`     // Clear Ollama URL
	// Cost controls
	CostBudgetUSD30d *float64 `json:"cost_budget_usd_30d,omitempty"`
	// Request timeout (seconds) - for slow hardware running local models
	RequestTimeoutSeconds *int `json:"request_timeout_seconds,omitempty"`
	// Infrastructure control settings
	ControlLevel    *string  `json:"control_level,omitempty"`    // "read_only", "controlled", "autonomous"
	ProtectedGuests []string `json:"protected_guests,omitempty"` // VMIDs/names that AI cannot control (nil = don't update, empty = clear)
	// Discovery settings
	DiscoveryEnabled       *bool `json:"discovery_enabled,omitempty"`        // Enable discovery
	DiscoveryIntervalHours *int  `json:"discovery_interval_hours,omitempty"` // Hours between auto-scans (0 = manual only)
}

// HandleGetAISettings returns the current AI settings (GET /api/settings/ai)
func (h *AISettingsHandler) HandleGetAISettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	persistence := h.getPersistence(ctx)
	settings, err := persistence.LoadAIConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load Pulse Assistant settings")
		http.Error(w, "Failed to load Pulse Assistant settings", http.StatusInternalServerError)
		return
	}

	if settings == nil {
		settings = config.NewDefaultAIConfig()
	}

	// Determine auth method string
	authMethod := string(settings.AuthMethod)
	if authMethod == "" {
		authMethod = string(config.AuthMethodAPIKey)
	}

	// Determine if running in demo mode
	isDemo := strings.EqualFold(os.Getenv("PULSE_MOCK_MODE"), "true")

	response := AISettingsResponse{
		Enabled:        settings.Enabled || isDemo,
		Provider:       settings.Provider,
		APIKeySet:      settings.APIKey != "",
		Model:          settings.GetModel(),
		ChatModel:      settings.ChatModel,
		PatrolModel:    settings.PatrolModel,
		AutoFixModel:   settings.AutoFixModel,
		BaseURL:        settings.BaseURL,
		Configured:     settings.IsConfigured() || isDemo,
		AutonomousMode: settings.IsAutonomous(), // Derived from control_level
		CustomContext:  settings.CustomContext,
		AuthMethod:     authMethod,
		OAuthConnected: settings.OAuthAccessToken != "",
		// Patrol settings
		PatrolSchedulePreset:   settings.PatrolSchedulePreset,
		PatrolIntervalMinutes:  settings.PatrolIntervalMinutes,
		PatrolEnabled:          settings.PatrolEnabled,
		PatrolAutoFix:          settings.PatrolAutoFix,
		AlertTriggeredAnalysis: settings.AlertTriggeredAnalysis,
		UseProactiveThresholds: settings.UseProactiveThresholds,
		AvailableModels:        nil, // Now populated via /api/ai/models endpoint
		// Multi-provider configuration
		AnthropicConfigured:    settings.HasProvider(config.AIProviderAnthropic),
		OpenAIConfigured:       settings.HasProvider(config.AIProviderOpenAI),
		OpenRouterConfigured:   settings.HasProvider(config.AIProviderOpenRouter),
		DeepSeekConfigured:     settings.HasProvider(config.AIProviderDeepSeek),
		GeminiConfigured:       settings.HasProvider(config.AIProviderGemini),
		OllamaConfigured:       settings.HasProvider(config.AIProviderOllama),
		OllamaBaseURL:          settings.GetBaseURLForProvider(config.AIProviderOllama),
		OpenAIBaseURL:          settings.OpenAIBaseURL,
		ConfiguredProviders:    settings.GetConfiguredProviders(),
		CostBudgetUSD30d:       settings.CostBudgetUSD30d,
		RequestTimeoutSeconds:  settings.RequestTimeoutSeconds,
		ControlLevel:           settings.GetControlLevel(),
		ProtectedGuests:        settings.GetProtectedGuests(),
		DiscoveryEnabled:       settings.IsDiscoveryEnabled(),
		DiscoveryIntervalHours: settings.DiscoveryIntervalHours,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write AI settings response")
	}
}

// HandleUpdateAISettings updates AI settings (PUT /api/settings/ai)
func (h *AISettingsHandler) HandleUpdateAISettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require admin authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	// Check proxy auth admin status if applicable
	if h.getConfig(r.Context()).ProxyAuthSecret != "" {
		if valid, username, isAdmin := CheckProxyAuth(h.getConfig(r.Context()), r); valid && !isAdmin {
			log.Warn().
				Str("ip", r.RemoteAddr).
				Str("path", r.URL.Path).
				Str("method", r.Method).
				Str("username", username).
				Msg("Non-admin user attempted to update AI settings")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Admin privileges required"})
			return
		}
	}

	// Load existing settings
	settings, err := h.getPersistence(r.Context()).LoadAIConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load existing AI settings")
		settings = config.NewDefaultAIConfig()
	}
	if settings == nil {
		settings = config.NewDefaultAIConfig()
	}

	// Parse request
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var req AISettingsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate and apply updates
	if req.Provider != nil {
		provider := strings.ToLower(strings.TrimSpace(*req.Provider))
		switch provider {
		case config.AIProviderAnthropic, config.AIProviderOpenAI, config.AIProviderOpenRouter, config.AIProviderOllama, config.AIProviderDeepSeek, config.AIProviderGemini:
			settings.Provider = provider
		default:
			http.Error(w, "Invalid provider. Must be 'anthropic', 'openai', 'openrouter', 'ollama', 'deepseek', or 'gemini'", http.StatusBadRequest)
			return
		}
	}

	if req.APIKey != nil {
		// Empty string clears the API key
		settings.APIKey = strings.TrimSpace(*req.APIKey)
	}

	if req.Model != nil {
		settings.Model = strings.TrimSpace(*req.Model)
	}

	if req.ChatModel != nil {
		settings.ChatModel = strings.TrimSpace(*req.ChatModel)
	}

	if req.PatrolModel != nil {
		settings.PatrolModel = strings.TrimSpace(*req.PatrolModel)
	}

	if req.AutoFixModel != nil {
		settings.AutoFixModel = strings.TrimSpace(*req.AutoFixModel)
	}

	if req.PatrolAutoFix != nil {
		// Auto-fix requires Pro license with ai_autofix feature
		if *req.PatrolAutoFix && !h.GetAIService(r.Context()).HasLicenseFeature(ai.FeatureAIAutoFix) {
			WriteLicenseRequired(w, ai.FeatureAIAutoFix, "Pulse Patrol Auto-Fix requires Pulse Pro")
			return
		}
		settings.PatrolAutoFix = *req.PatrolAutoFix
	}

	if req.UseProactiveThresholds != nil {
		settings.UseProactiveThresholds = *req.UseProactiveThresholds
	}

	if req.BaseURL != nil {
		settings.BaseURL = strings.TrimSpace(*req.BaseURL)
	}

	if req.AutonomousMode != nil {
		// Legacy: autonomous_mode now maps to control_level for backwards compatibility
		if *req.AutonomousMode {
			// Autonomous mode requires Pro license with ai_autofix feature
			if !h.GetAIService(r.Context()).HasLicenseFeature(ai.FeatureAIAutoFix) {
				WriteLicenseRequired(w, ai.FeatureAIAutoFix, "Autonomous Mode requires Pulse Pro")
				return
			}
			settings.ControlLevel = config.ControlLevelAutonomous
			settings.AutonomousMode = true
		} else if settings.GetControlLevel() == config.ControlLevelAutonomous {
			// Only downgrade from autonomous to controlled; preserve other levels
			// (e.g., don't change read_only to controlled)
			settings.ControlLevel = config.ControlLevelControlled
			settings.AutonomousMode = false
		}
	}

	if req.CustomContext != nil {
		settings.CustomContext = strings.TrimSpace(*req.CustomContext)
	}

	// Handle multi-provider credentials FIRST - before enabled check
	// This allows the setup flow to send API key + enabled:true together
	// Clear flags take priority over setting new values
	if req.ClearAnthropicKey != nil && *req.ClearAnthropicKey {
		settings.AnthropicAPIKey = ""
	} else if req.AnthropicAPIKey != nil {
		settings.AnthropicAPIKey = strings.TrimSpace(*req.AnthropicAPIKey)
	}
	if req.ClearOpenAIKey != nil && *req.ClearOpenAIKey {
		settings.OpenAIAPIKey = ""
	} else if req.OpenAIAPIKey != nil {
		settings.OpenAIAPIKey = strings.TrimSpace(*req.OpenAIAPIKey)
	}
	if req.ClearOpenRouterKey != nil && *req.ClearOpenRouterKey {
		settings.OpenRouterAPIKey = ""
	} else if req.OpenRouterAPIKey != nil {
		settings.OpenRouterAPIKey = strings.TrimSpace(*req.OpenRouterAPIKey)
	}
	if req.ClearDeepSeekKey != nil && *req.ClearDeepSeekKey {
		settings.DeepSeekAPIKey = ""
	} else if req.DeepSeekAPIKey != nil {
		settings.DeepSeekAPIKey = strings.TrimSpace(*req.DeepSeekAPIKey)
	}
	if req.ClearGeminiKey != nil && *req.ClearGeminiKey {
		settings.GeminiAPIKey = ""
	} else if req.GeminiAPIKey != nil {
		settings.GeminiAPIKey = strings.TrimSpace(*req.GeminiAPIKey)
	}
	if req.ClearOllamaURL != nil && *req.ClearOllamaURL {
		settings.OllamaBaseURL = ""
	} else if req.OllamaBaseURL != nil {
		settings.OllamaBaseURL = strings.TrimSpace(*req.OllamaBaseURL)
	}
	if req.OpenAIBaseURL != nil {
		settings.OpenAIBaseURL = strings.TrimSpace(*req.OpenAIBaseURL)
	}

	if req.Enabled != nil {
		// Only allow enabling if at least one provider is configured
		if *req.Enabled {
			configuredProviders := settings.GetConfiguredProviders()
			if len(configuredProviders) == 0 {
				// No providers configured - give a helpful error
				http.Error(w, "Please configure a provider (API key or Ollama URL) before enabling Pulse Assistant", http.StatusBadRequest)
				return
			}
			// If we have configured providers, we're good to enable
		}
		settings.Enabled = *req.Enabled
	}

	// Handle patrol interval - prefer custom minutes over preset
	if req.PatrolIntervalMinutes != nil {
		minutes := *req.PatrolIntervalMinutes
		if minutes < 0 {
			http.Error(w, "patrol_interval_minutes cannot be negative", http.StatusBadRequest)
			return
		}
		if minutes > 0 && minutes < 10 {
			http.Error(w, "patrol_interval_minutes must be at least 10 minutes (or 0 to disable)", http.StatusBadRequest)
			return
		}
		if minutes > 10080 { // 7 days max
			http.Error(w, "patrol_interval_minutes cannot exceed 10080 (7 days)", http.StatusBadRequest)
			return
		}

		settings.PatrolIntervalMinutes = minutes
		settings.PatrolSchedulePreset = "" // Clear preset when using custom minutes
		if minutes > 0 {
			settings.PatrolEnabled = true // Enable patrol when setting custom interval
		} else {
			settings.PatrolEnabled = false // Disable patrol when setting interval to 0
		}
	} else if req.PatrolSchedulePreset != nil {
		// Legacy preset support
		preset := strings.ToLower(strings.TrimSpace(*req.PatrolSchedulePreset))
		switch preset {
		case "15min", "1hr", "6hr", "12hr", "daily":
			settings.PatrolSchedulePreset = preset
			settings.PatrolIntervalMinutes = config.PresetToMinutes(preset)
			settings.PatrolEnabled = true // Enable patrol when setting schedule preset
		case "disabled":
			settings.PatrolSchedulePreset = preset
			settings.PatrolIntervalMinutes = 0
			settings.PatrolEnabled = false // Disable patrol when using disabled preset
		default:
			http.Error(w, "Invalid patrol_schedule_preset. Must be '15min', '1hr', '6hr', '12hr', 'daily', or 'disabled'", http.StatusBadRequest)
			return
		}
	}

	if req.PatrolEnabled != nil && req.PatrolIntervalMinutes == nil && req.PatrolSchedulePreset == nil {
		settings.PatrolEnabled = *req.PatrolEnabled
		if *req.PatrolEnabled {
			// Re-enable if legacy preset was disabled
			if strings.EqualFold(settings.PatrolSchedulePreset, "disabled") {
				settings.PatrolSchedulePreset = ""
			}
			// Ensure we have a sane default interval when turning on
			if settings.PatrolIntervalMinutes <= 0 {
				settings.PatrolIntervalMinutes = 360
			}
		}
	}

	if req.CostBudgetUSD30d != nil {
		if *req.CostBudgetUSD30d < 0 {
			http.Error(w, "cost_budget_usd_30d cannot be negative", http.StatusBadRequest)
			return
		}
		settings.CostBudgetUSD30d = *req.CostBudgetUSD30d
	}

	// Handle alert-triggered analysis toggle
	if req.AlertTriggeredAnalysis != nil {
		// Alert analysis requires Pro license with ai_alerts feature
		if *req.AlertTriggeredAnalysis && !h.GetAIService(r.Context()).HasLicenseFeature(ai.FeatureAIAlerts) {
			WriteLicenseRequired(w, ai.FeatureAIAlerts, "Pulse Alert Analysis requires Pulse Pro")
			return
		}
		settings.AlertTriggeredAnalysis = *req.AlertTriggeredAnalysis
	}

	// Handle request timeout (for slow hardware)
	if req.RequestTimeoutSeconds != nil {
		if *req.RequestTimeoutSeconds < 0 {
			http.Error(w, "request_timeout_seconds cannot be negative", http.StatusBadRequest)
			return
		}
		if *req.RequestTimeoutSeconds > 3600 {
			http.Error(w, "request_timeout_seconds cannot exceed 3600 (1 hour)", http.StatusBadRequest)
			return
		}
		settings.RequestTimeoutSeconds = *req.RequestTimeoutSeconds
	}

	// Handle infrastructure control settings
	if req.ControlLevel != nil {
		level := strings.TrimSpace(*req.ControlLevel)
		if level == "suggest" {
			level = config.ControlLevelControlled
		}
		if !config.IsValidControlLevel(level) {
			http.Error(w, "invalid control_level: must be read_only, controlled, or autonomous", http.StatusBadRequest)
			return
		}
		// "autonomous" requires Pro license (same as autonomous_mode)
		if level == config.ControlLevelAutonomous {
			if !h.GetAIService(r.Context()).HasLicenseFeature(ai.FeatureAIAutoFix) {
				WriteLicenseRequired(w, ai.FeatureAIAutoFix, "Autonomous control requires Pulse Pro")
				return
			}
		}
		settings.ControlLevel = level
		// Keep legacy AutonomousMode in sync to prevent fallback issues
		settings.AutonomousMode = (level == config.ControlLevelAutonomous)
	}

	// Handle protected guests (nil = don't update)
	if req.ProtectedGuests != nil {
		settings.ProtectedGuests = req.ProtectedGuests
	}

	// Handle discovery settings
	if req.DiscoveryEnabled != nil {
		settings.DiscoveryEnabled = *req.DiscoveryEnabled
	}
	if req.DiscoveryIntervalHours != nil {
		if *req.DiscoveryIntervalHours < 0 {
			http.Error(w, "discovery_interval_hours cannot be negative", http.StatusBadRequest)
			return
		}
		settings.DiscoveryIntervalHours = *req.DiscoveryIntervalHours
	}

	// Auto-default discovery interval to 24h when enabled with no interval set.
	// Without this, enabling discovery with interval=0 silently stays in manual-only mode.
	if settings.DiscoveryEnabled && settings.DiscoveryIntervalHours == 0 {
		settings.DiscoveryIntervalHours = 24
	}

	// Save settings
	if err := h.getPersistence(r.Context()).SaveAIConfig(*settings); err != nil {
		log.Error().Err(err).Msg("Failed to save AI settings")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	// Reload the AI service with new settings
	if err := h.GetAIService(r.Context()).Reload(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload AI service after settings update")
	}

	// Reconfigure patrol service with new settings (applies interval changes immediately)
	h.GetAIService(r.Context()).ReconfigurePatrol()

	// Update alert-triggered analyzer if available
	if analyzer := h.GetAIService(r.Context()).GetAlertTriggeredAnalyzer(); analyzer != nil {
		analyzer.SetEnabled(settings.AlertTriggeredAnalysis)
	}

	// Trigger AI chat service restart if model changed
	// This ensures the new model is picked up by the service
	// Trigger AI chat service restart if model changed or AI enabled
	// This ensures the new model is picked up by the service
	if h.onModelChange != nil && (req.Model != nil || req.ChatModel != nil || req.Enabled != nil) {
		h.onModelChange()
	}

	// Update MCP control settings if control level or protected guests changed
	// This updates tool visibility without restarting AI chat
	// Note: req.AutonomousMode also maps to control_level for backwards compatibility
	if h.onControlSettingsChange != nil && (req.ControlLevel != nil || req.ProtectedGuests != nil || req.AutonomousMode != nil) {
		h.onControlSettingsChange()
	}

	log.Info().
		Bool("enabled", settings.Enabled).
		Str("provider", settings.Provider).
		Str("model", settings.GetModel()).
		Str("chatModel", settings.ChatModel).
		Str("patrolModel", settings.PatrolModel).
		Str("patrolPreset", settings.PatrolSchedulePreset).
		Bool("alertTriggeredAnalysis", settings.AlertTriggeredAnalysis).
		Msg("AI settings updated")

	// Determine auth method for response
	authMethod := string(settings.AuthMethod)
	if authMethod == "" {
		authMethod = string(config.AuthMethodAPIKey)
	}

	// Return updated settings
	response := AISettingsResponse{
		Enabled:                settings.Enabled,
		Provider:               settings.Provider,
		APIKeySet:              settings.APIKey != "",
		Model:                  settings.GetModel(),
		ChatModel:              settings.ChatModel,
		PatrolModel:            settings.PatrolModel,
		AutoFixModel:           settings.AutoFixModel,
		BaseURL:                settings.BaseURL,
		Configured:             settings.IsConfigured(),
		AutonomousMode:         settings.IsAutonomous(), // Derived from control_level
		CustomContext:          settings.CustomContext,
		AuthMethod:             authMethod,
		OAuthConnected:         settings.OAuthAccessToken != "",
		PatrolSchedulePreset:   settings.PatrolSchedulePreset,
		PatrolIntervalMinutes:  settings.PatrolIntervalMinutes,
		PatrolEnabled:          settings.PatrolEnabled,
		PatrolAutoFix:          settings.PatrolAutoFix,
		AlertTriggeredAnalysis: settings.AlertTriggeredAnalysis,
		UseProactiveThresholds: settings.UseProactiveThresholds,
		AvailableModels:        nil, // Now populated via /api/ai/models endpoint
		// Multi-provider configuration
		AnthropicConfigured:    settings.HasProvider(config.AIProviderAnthropic),
		OpenAIConfigured:       settings.HasProvider(config.AIProviderOpenAI),
		OpenRouterConfigured:   settings.HasProvider(config.AIProviderOpenRouter),
		DeepSeekConfigured:     settings.HasProvider(config.AIProviderDeepSeek),
		GeminiConfigured:       settings.HasProvider(config.AIProviderGemini),
		OllamaConfigured:       settings.HasProvider(config.AIProviderOllama),
		OllamaBaseURL:          settings.GetBaseURLForProvider(config.AIProviderOllama),
		OpenAIBaseURL:          settings.OpenAIBaseURL,
		ConfiguredProviders:    settings.GetConfiguredProviders(),
		RequestTimeoutSeconds:  settings.RequestTimeoutSeconds,
		ControlLevel:           settings.GetControlLevel(),
		ProtectedGuests:        settings.GetProtectedGuests(),
		DiscoveryEnabled:       settings.DiscoveryEnabled,
		DiscoveryIntervalHours: settings.DiscoveryIntervalHours,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write AI settings update response")
	}
}

// HandleTestAIConnection tests the AI provider connection (POST /api/ai/test)
func (h *AISettingsHandler) HandleTestAIConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require admin authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	// Check proxy auth admin status if applicable
	if h.getConfig(r.Context()).ProxyAuthSecret != "" {
		if valid, username, isAdmin := CheckProxyAuth(h.getConfig(r.Context()), r); valid && !isAdmin {
			log.Warn().
				Str("ip", r.RemoteAddr).
				Str("path", r.URL.Path).
				Str("method", r.Method).
				Str("username", username).
				Msg("Non-admin user attempted AI connection test")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Admin privileges required"})
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var testResult struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Model   string `json:"model,omitempty"`
	}

	err := h.GetAIService(r.Context()).TestConnection(ctx)
	if err != nil {
		testResult.Success = false
		testResult.Message = err.Error()
	} else {
		cfg := h.GetAIService(r.Context()).GetConfig()
		testResult.Success = true
		testResult.Message = "Connection successful"
		if cfg != nil {
			testResult.Model = cfg.GetModel()
		}
	}

	if err := utils.WriteJSONResponse(w, testResult); err != nil {
		log.Error().Err(err).Msg("Failed to write AI test response")
	}
}

// HandleTestProvider tests a specific AI provider connection (POST /api/ai/test/:provider)
func (h *AISettingsHandler) HandleTestProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require admin authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	// Check proxy auth admin status if applicable
	if h.getConfig(r.Context()).ProxyAuthSecret != "" {
		if valid, username, isAdmin := CheckProxyAuth(h.getConfig(r.Context()), r); valid && !isAdmin {
			log.Warn().
				Str("ip", r.RemoteAddr).
				Str("path", r.URL.Path).
				Str("method", r.Method).
				Str("username", username).
				Msg("Non-admin user attempted AI provider test")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Admin privileges required"})
			return
		}
	}

	// Get provider from URL path (e.g., /api/ai/test/anthropic -> anthropic)
	provider := strings.TrimPrefix(r.URL.Path, "/api/ai/test/")
	if provider == "" || provider == r.URL.Path {
		http.Error(w, `{"error":"Provider is required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var testResult struct {
		Success  bool   `json:"success"`
		Message  string `json:"message"`
		Provider string `json:"provider"`
	}
	testResult.Provider = provider

	// Load config and create provider for testing
	cfg := h.GetAIService(r.Context()).GetConfig()
	if cfg == nil {
		testResult.Success = false
		testResult.Message = "Pulse Assistant not configured"
		if err := utils.WriteJSONResponse(w, testResult); err != nil {
			log.Error().Err(err).Msg("failed to write JSON response")
		}
		return
	}

	// Check if provider is configured
	if !cfg.HasProvider(provider) {
		testResult.Success = false
		testResult.Message = "Provider not configured"
		if err := utils.WriteJSONResponse(w, testResult); err != nil {
			log.Error().Err(err).Msg("failed to write JSON response")
		}
		return
	}

	// Create provider and test connection
	testProvider, err := providers.NewForProvider(cfg, provider, cfg.GetModel())
	if err != nil {
		testResult.Success = false
		testResult.Message = fmt.Sprintf("Failed to create provider: %v", err)
		if err := utils.WriteJSONResponse(w, testResult); err != nil {
			log.Error().Err(err).Msg("failed to write JSON response")
		}
		return
	}

	err = testProvider.TestConnection(ctx)
	if err != nil {
		testResult.Success = false
		testResult.Message = err.Error()
	} else {
		testResult.Success = true
		testResult.Message = "Connection successful"
	}

	if err := utils.WriteJSONResponse(w, testResult); err != nil {
		log.Error().Err(err).Msg("Failed to write provider test response")
	}
}

// HandleListModels fetches available models from the configured AI provider (GET /api/ai/models)
func (h *AISettingsHandler) HandleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	type ModelInfo struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		CreatedAt   int64  `json:"created_at,omitempty"`
		Notable     bool   `json:"notable"`
	}

	type Response struct {
		Models []ModelInfo `json:"models"`
		Error  string      `json:"error,omitempty"`
		Cached bool        `json:"cached"`
	}

	models, cached, err := h.GetAIService(r.Context()).ListModelsWithCache(ctx)
	if err != nil {
		// Return error but don't fail the request - frontend can show a fallback
		resp := Response{
			Models: []ModelInfo{},
			Error:  err.Error(),
		}
		if jsonErr := utils.WriteJSONResponse(w, resp); jsonErr != nil {
			log.Error().Err(jsonErr).Msg("Failed to write AI models response")
		}
		return
	}

	// Convert provider models to response format
	responseModels := make([]ModelInfo, 0, len(models))
	notableCount := 0
	for _, m := range models {
		if m.Notable {
			notableCount++
		}
		responseModels = append(responseModels, ModelInfo{
			ID:          m.ID,
			Name:        m.Name,
			Description: m.Description,
			CreatedAt:   m.CreatedAt,
			Notable:     m.Notable,
		})
	}

	log.Debug().Int("total", len(responseModels)).Int("notable", notableCount).Msg("Returning AI models")

	resp := Response{
		Models: responseModels,
		Cached: cached,
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to write AI models response")
	}
}

// AIExecuteRequest is the request body for POST /api/ai/execute
// AIConversationMessage represents a message in conversation history
type AIConversationMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

type AIExecuteRequest struct {
	Prompt     string                  `json:"prompt"`
	TargetType string                  `json:"target_type,omitempty"` // "host", "container", "vm", "node"
	TargetID   string                  `json:"target_id,omitempty"`
	Context    map[string]interface{}  `json:"context,omitempty"` // Current metrics, state, etc.
	History    []AIConversationMessage `json:"history,omitempty"` // Previous conversation messages
	FindingID  string                  `json:"finding_id,omitempty"`
	Model      string                  `json:"model,omitempty"`
	UseCase    string                  `json:"use_case,omitempty"` // "chat" or "patrol"
}

// AIExecuteResponse is the response from POST /api/ai/execute
type AIExecuteResponse struct {
	Content          string                  `json:"content"`
	Model            string                  `json:"model"`
	InputTokens      int                     `json:"input_tokens"`
	OutputTokens     int                     `json:"output_tokens"`
	ToolCalls        []ai.ToolExecution      `json:"tool_calls,omitempty"`        // Commands that were executed
	PendingApprovals []ai.ApprovalNeededData `json:"pending_approvals,omitempty"` // Commands that require approval (non-streaming)
}

type AIKubernetesAnalyzeRequest struct {
	ClusterID string `json:"cluster_id"`
}

// HandleExecute executes an AI prompt (POST /api/ai/execute)
func (h *AISettingsHandler) HandleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	// Check if AI is enabled
	if !h.GetAIService(r.Context()).IsEnabled() {
		http.Error(w, "Pulse Assistant is not enabled or configured", http.StatusBadRequest)
		return
	}

	// Parse request
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	bodyBytes, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		log.Error().Err(readErr).Msg("Failed to read request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var req AIExecuteRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Fine-grained license checks based on UseCase
	useCase := strings.ToLower(strings.TrimSpace(req.UseCase))
	if useCase == "autofix" || useCase == "remediation" {
		if !h.GetAIService(r.Context()).HasLicenseFeature(ai.FeatureAIAutoFix) {
			WriteLicenseRequired(w, ai.FeatureAIAutoFix, "Pulse Patrol Auto-Fix requires Pulse Pro")
			return
		}
	}

	if strings.TrimSpace(req.Prompt) == "" {
		http.Error(w, "Prompt is required", http.StatusBadRequest)
		return
	}

	// Execute the prompt with a timeout
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	// Convert history from API type to service type
	var history []ai.ConversationMessage
	for _, msg := range req.History {
		history = append(history, ai.ConversationMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	if useCase == "" {
		useCase = "chat"
	}

	resp, err := h.GetAIService(r.Context()).Execute(ctx, ai.ExecuteRequest{
		Prompt:     req.Prompt,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Context:    req.Context,
		History:    history,
		FindingID:  req.FindingID,
		Model:      req.Model,
		UseCase:    useCase,
	})
	if err != nil {
		log.Error().Err(err).Msg("AI execution failed")
		http.Error(w, "Pulse Assistant request failed", http.StatusInternalServerError)
		return
	}

	response := AIExecuteResponse{
		Content:          resp.Content,
		Model:            resp.Model,
		InputTokens:      resp.InputTokens,
		OutputTokens:     resp.OutputTokens,
		ToolCalls:        resp.ToolCalls,
		PendingApprovals: resp.PendingApprovals,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write AI execute response")
	}
}

// HandleAnalyzeKubernetesCluster analyzes a Kubernetes cluster with AI (POST /api/ai/kubernetes/analyze)
func (h *AISettingsHandler) HandleAnalyzeKubernetesCluster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	if !h.GetAIService(r.Context()).IsEnabled() {
		http.Error(w, "Pulse Assistant is not enabled or configured", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var req AIKubernetesAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.ClusterID) == "" {
		http.Error(w, "cluster_id is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	resp, err := h.GetAIService(r.Context()).AnalyzeKubernetesCluster(ctx, req.ClusterID)
	if err != nil {
		switch {
		case errors.Is(err, ai.ErrKubernetesClusterNotFound):
			http.Error(w, "Kubernetes cluster not found", http.StatusNotFound)
			return
		case errors.Is(err, ai.ErrKubernetesStateUnavailable):
			http.Error(w, "Kubernetes state not available", http.StatusServiceUnavailable)
			return
		default:
			log.Error().Err(err).Str("cluster_id", req.ClusterID).Msg("Kubernetes AI analysis failed")
			http.Error(w, "Pulse Assistant request failed", http.StatusInternalServerError)
			return
		}
	}

	response := AIExecuteResponse{
		Content:          resp.Content,
		Model:            resp.Model,
		InputTokens:      resp.InputTokens,
		OutputTokens:     resp.OutputTokens,
		ToolCalls:        resp.ToolCalls,
		PendingApprovals: resp.PendingApprovals,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write Kubernetes AI response")
	}
}

// HandleExecuteStream executes an AI prompt with SSE streaming (POST /api/ai/execute/stream)
func (h *AISettingsHandler) HandleExecuteStream(w http.ResponseWriter, r *http.Request) {
	// Handle CORS for dev mode (frontend on different port)
	h.setSSECORSHeaders(w, r)

	// Handle preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	// Check if AI is enabled
	if !h.GetAIService(r.Context()).IsEnabled() {
		http.Error(w, "Pulse Assistant is not enabled or configured", http.StatusBadRequest)
		return
	}

	// Parse request
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64KB max
	var req AIExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn().Err(err).Msg("Failed to decode AI execute stream request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Fine-grained license checks based on UseCase (before SSE headers)
	useCase := strings.ToLower(strings.TrimSpace(req.UseCase))
	if useCase == "autofix" || useCase == "remediation" {
		if !h.GetAIService(r.Context()).HasLicenseFeature(ai.FeatureAIAutoFix) {
			WriteLicenseRequired(w, ai.FeatureAIAutoFix, "Pulse Patrol Auto-Fix requires Pulse Pro")
			return
		}
	}

	if strings.TrimSpace(req.Prompt) == "" {
		http.Error(w, "Prompt is required", http.StatusBadRequest)
		return
	}

	log.Info().
		Int("prompt_len", len(req.Prompt)).
		Str("target_type", req.TargetType).
		Str("target_id", req.TargetID).
		Msg("AI streaming request started")

	// Set up SSE headers
	// IMPORTANT: Set headers BEFORE any writes to prevent Go from auto-adding Transfer-Encoding: chunked
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
	// Prevent chunked encoding which causes "Invalid character in chunk size" errors in Vite proxy
	w.Header().Set("Transfer-Encoding", "identity")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Disable the server's write deadline for this SSE connection
	// This is critical for long-running AI requests that can take several minutes
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		log.Warn().Err(err).Msg("Failed to disable write deadline for SSE")
	}
	// Also disable read deadline
	if err := rc.SetReadDeadline(time.Time{}); err != nil {
		log.Warn().Err(err).Msg("Failed to disable read deadline for SSE")
	}

	// Flush headers immediately
	flusher.Flush()

	// Create context with timeout (15 minutes for complex analysis with multiple tool calls)
	// Use background context to avoid browser disconnect canceling the request
	// DeepSeek reasoning models + multiple tool executions can easily take 5+ minutes
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Second)
	defer cancel()

	// Set up heartbeat to keep connection alive during long tool executions
	// NOTE: We don't check r.Context().Done() because Vite proxy may close
	// the request context prematurely. We detect real disconnection via write failures.
	heartbeatDone := make(chan struct{})
	var clientDisconnected atomic.Bool
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Extend write deadline before heartbeat
				_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
				// Send SSE comment as heartbeat
				_, err := w.Write([]byte(": heartbeat\n\n"))
				if err != nil {
					log.Debug().Err(err).Msg("Heartbeat write failed, stopping heartbeat (AI continues)")
					clientDisconnected.Store(true)
					// Don't cancel the AI request - let it complete with its own timeout
					// The SSE connection may have issues but the AI work can still finish
					return
				}
				flusher.Flush()
				log.Debug().Msg("Sent SSE heartbeat")
			case <-heartbeatDone:
				return
			}
		}
	}()
	defer close(heartbeatDone)

	// Helper to safely write SSE events, tracking if client disconnected
	safeWrite := func(data []byte) bool {
		if clientDisconnected.Load() {
			return false
		}
		_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, err := w.Write(data)
		if err != nil {
			log.Debug().Err(err).Msg("Failed to write SSE event (client may have disconnected)")
			clientDisconnected.Store(true)
			return false
		}
		flusher.Flush()
		return true
	}

	// Stream callback - write SSE events
	callback := func(event ai.StreamEvent) {
		// Skip the 'done' event from service - we'll send our own at the end
		// This ensures 'complete' comes before 'done'
		if event.Type == "done" {
			log.Debug().Msg("Skipping service 'done' event - will send final 'done' after 'complete'")
			return
		}

		data, err := json.Marshal(event)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal stream event")
			return
		}

		log.Debug().
			Str("event_type", event.Type).
			Msg("Streaming AI event")

		// SSE format: data: <json>\n\n
		safeWrite([]byte("data: " + string(data) + "\n\n"))
	}

	// Convert history from API type to service type
	var history []ai.ConversationMessage
	for _, msg := range req.History {
		history = append(history, ai.ConversationMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	if useCase == "" {
		useCase = "chat"
	}

	// Ensure we always send a final 'done' event
	defer func() {
		if !clientDisconnected.Load() {
			doneEvent := ai.StreamEvent{Type: "done"}
			data, _ := json.Marshal(doneEvent)
			safeWrite([]byte("data: " + string(data) + "\n\n"))
			log.Debug().Msg("Sent final 'done' event")
		}
	}()

	// Execute with streaming
	resp, err := h.GetAIService(r.Context()).ExecuteStream(ctx, ai.ExecuteRequest{
		Prompt:     req.Prompt,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Context:    req.Context,
		History:    history,
		FindingID:  req.FindingID,
		Model:      req.Model,
		UseCase:    useCase,
	}, callback)

	if err != nil {
		log.Error().Err(err).Msg("AI streaming execution failed")
		// Send error event — use generic message to avoid leaking internal details
		errEvent := ai.StreamEvent{Type: "error", Data: map[string]string{"message": "AI request failed. Please try again."}}
		data, _ := json.Marshal(errEvent)
		safeWrite([]byte("data: " + string(data) + "\n\n"))
		return
	}

	log.Info().
		Str("model", resp.Model).
		Int("input_tokens", resp.InputTokens).
		Int("output_tokens", resp.OutputTokens).
		Int("tool_calls", len(resp.ToolCalls)).
		Msg("AI streaming request completed")

	// Send final response with metadata (before 'done')
	finalEvent := struct {
		Type         string             `json:"type"`
		Model        string             `json:"model"`
		InputTokens  int                `json:"input_tokens"`
		OutputTokens int                `json:"output_tokens"`
		ToolCalls    []ai.ToolExecution `json:"tool_calls,omitempty"`
	}{
		Type:         "complete",
		Model:        resp.Model,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		ToolCalls:    resp.ToolCalls,
	}
	data, _ := json.Marshal(finalEvent)
	safeWrite([]byte("data: " + string(data) + "\n\n"))
	// 'done' event is sent by the defer above
}

// AIRunCommandRequest is the request body for POST /api/ai/run-command
type AIRunCommandRequest struct {
	Command    string `json:"command"`
	ApprovalID string `json:"approval_id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	RunOnHost  bool   `json:"run_on_host"`
	VMID       string `json:"vmid,omitempty"`
	TargetHost string `json:"target_host,omitempty"` // Explicit host for routing
}

// HandleRunCommand executes a single approved command (POST /api/ai/run-command)
func (h *AISettingsHandler) HandleRunCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	// Gated for AI Auto-Fix (Pro feature)
	if !h.GetAIService(r.Context()).HasLicenseFeature(ai.FeatureAIAutoFix) {
		WriteLicenseRequired(w, ai.FeatureAIAutoFix, "Pulse Patrol Auto-Fix requires Pulse Pro")
		return
	}

	// Parse request
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	bodyBytes, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		log.Error().Err(readErr).Msg("Failed to read request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Debug().Int("body_len", len(bodyBytes)).Msg("run-command request received")

	var req AIRunCommandRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		log.Error().Err(err).Str("body", string(bodyBytes)).Msg("Failed to decode JSON body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Command) == "" {
		http.Error(w, "Command is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ApprovalID) == "" {
		http.Error(w, "approval_id is required", http.StatusBadRequest)
		return
	}

	approvalTargetType, approvalTargetID, targetErr := normalizeRunCommandApprovalTarget(req)
	if targetErr != nil {
		http.Error(w, targetErr.Error(), http.StatusBadRequest)
		return
	}

	store := approval.GetStore()
	if store == nil {
		http.Error(w, "Approval store not initialized", http.StatusServiceUnavailable)
		return
	}

	_, ok := store.GetApproval(req.ApprovalID)
	if !ok {
		http.Error(w, "Approval request not found", http.StatusNotFound)
		return
	}
	if _, err := store.ConsumeApproval(req.ApprovalID, req.Command, approvalTargetType, approvalTargetID); err != nil {
		http.Error(w, "Failed to consume approval: "+err.Error(), http.StatusConflict)
		return
	}

	log.Info().
		Str("command", req.Command).
		Str("approval_id", req.ApprovalID).
		Str("target_type", req.TargetType).
		Str("target_id", req.TargetID).
		Bool("run_on_host", req.RunOnHost).
		Str("target_host", req.TargetHost).
		Msg("Executing approved command")

	// Execute with timeout (5 minutes for long-running commands)
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Second)
	defer cancel()

	resp, err := h.GetAIService(r.Context()).RunCommand(ctx, ai.RunCommandRequest{
		Command:    req.Command,
		ApprovalID: req.ApprovalID,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		RunOnHost:  req.RunOnHost,
		VMID:       req.VMID,
		TargetHost: req.TargetHost,
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to execute command")
		http.Error(w, "Failed to execute command: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to write run command response")
	}
}

func normalizeRunCommandApprovalTarget(req AIRunCommandRequest) (string, string, error) {
	targetType := strings.ToLower(strings.TrimSpace(req.TargetType))
	targetID := strings.TrimSpace(req.TargetID)
	targetHost := strings.ToLower(strings.TrimSpace(req.TargetHost))

	allowedTargetTypes := map[string]struct{}{
		"host":      {},
		"container": {},
		"vm":        {},
	}

	if req.RunOnHost {
		targetType = "host"
		if targetHost == "" {
			return "", "", errors.New("target_host is required when run_on_host is true")
		}
		targetID = targetHost
	}

	if targetType == "" {
		targetType = "host"
	}
	if _, ok := allowedTargetTypes[targetType]; !ok {
		return "", "", fmt.Errorf("unsupported target_type %q (allowed: host, container, vm)", targetType)
	}

	if (targetType == "container" || targetType == "vm") && strings.TrimSpace(req.VMID) != "" {
		targetID = strings.TrimSpace(req.VMID)
	}

	if targetType == "host" {
		if targetID == "" {
			targetID = targetHost
		}
		if targetID == "" {
			return "", "", errors.New("target_id or target_host is required for host commands")
		}
		targetID = strings.ToLower(targetID)
	}

	if targetID == "" {
		return "", "", fmt.Errorf("target_id is required for target_type '%s'", targetType)
	}

	return targetType, targetID, nil
}

// HandleGetGuestKnowledge returns all notes for a guest
func (h *AISettingsHandler) HandleGetGuestKnowledge(w http.ResponseWriter, r *http.Request) {
	guestID := r.URL.Query().Get("guest_id")
	if guestID == "" {
		http.Error(w, "guest_id is required", http.StatusBadRequest)
		return
	}

	knowledge, err := h.GetAIService(r.Context()).GetGuestKnowledge(guestID)
	if err != nil {
		http.Error(w, "Failed to get knowledge: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := utils.WriteJSONResponse(w, knowledge); err != nil {
		log.Error().Err(err).Msg("Failed to write knowledge response")
	}
}

// HandleSaveGuestNote saves a note for a guest
func (h *AISettingsHandler) HandleSaveGuestNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GuestID   string `json:"guest_id"`
		GuestName string `json:"guest_name"`
		GuestType string `json:"guest_type"`
		Category  string `json:"category"`
		Title     string `json:"title"`
		Content   string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.GuestID == "" || req.Category == "" || req.Title == "" || req.Content == "" {
		http.Error(w, "guest_id, category, title, and content are required", http.StatusBadRequest)
		return
	}

	if err := h.GetAIService(r.Context()).SaveGuestNote(req.GuestID, req.GuestName, req.GuestType, req.Category, req.Title, req.Content); err != nil {
		http.Error(w, "Failed to save note: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true}`))
}

// HandleDeleteGuestNote deletes a note from a guest
func (h *AISettingsHandler) HandleDeleteGuestNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GuestID string `json:"guest_id"`
		NoteID  string `json:"note_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.GuestID == "" || req.NoteID == "" {
		http.Error(w, "guest_id and note_id are required", http.StatusBadRequest)
		return
	}

	if err := h.GetAIService(r.Context()).DeleteGuestNote(req.GuestID, req.NoteID); err != nil {
		http.Error(w, "Failed to delete note: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true}`))
}

// HandleExportGuestKnowledge exports all knowledge for a guest as JSON
func (h *AISettingsHandler) HandleExportGuestKnowledge(w http.ResponseWriter, r *http.Request) {
	guestID := r.URL.Query().Get("guest_id")
	if guestID == "" {
		http.Error(w, "guest_id is required", http.StatusBadRequest)
		return
	}

	knowledge, err := h.GetAIService(r.Context()).GetGuestKnowledge(guestID)
	if err != nil {
		http.Error(w, "Failed to get knowledge: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set headers for file download
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"pulse-notes-"+guestID+".json\"")

	if err := json.NewEncoder(w).Encode(knowledge); err != nil {
		log.Error().Err(err).Msg("Failed to encode knowledge export")
	}
}

// HandleImportGuestKnowledge imports knowledge from a JSON export
func (h *AISettingsHandler) HandleImportGuestKnowledge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body size to 1MB
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var importData struct {
		GuestID   string `json:"guest_id"`
		GuestName string `json:"guest_name"`
		GuestType string `json:"guest_type"`
		Notes     []struct {
			Category string `json:"category"`
			Title    string `json:"title"`
			Content  string `json:"content"`
		} `json:"notes"`
		Merge bool `json:"merge"` // If true, add to existing notes; if false, replace
	}

	if err := json.NewDecoder(r.Body).Decode(&importData); err != nil {
		http.Error(w, "Invalid import data: "+err.Error(), http.StatusBadRequest)
		return
	}

	if importData.GuestID == "" {
		http.Error(w, "guest_id is required in import data", http.StatusBadRequest)
		return
	}

	if len(importData.Notes) == 0 {
		http.Error(w, "No notes to import", http.StatusBadRequest)
		return
	}

	// If not merging, we need to delete existing notes first
	if !importData.Merge {
		existing, err := h.GetAIService(r.Context()).GetGuestKnowledge(importData.GuestID)
		if err == nil && existing != nil {
			for _, note := range existing.Notes {
				_ = h.GetAIService(r.Context()).DeleteGuestNote(importData.GuestID, note.ID)
			}
		}
	}

	// Import each note
	imported := 0
	for _, note := range importData.Notes {
		if note.Category == "" || note.Title == "" || note.Content == "" {
			continue
		}
		if err := h.GetAIService(r.Context()).SaveGuestNote(
			importData.GuestID,
			importData.GuestName,
			importData.GuestType,
			note.Category,
			note.Title,
			note.Content,
		); err != nil {
			log.Warn().Err(err).Str("title", note.Title).Msg("Failed to import note")
			continue
		}
		imported++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"imported": imported,
		"total":    len(importData.Notes),
	})
}

// HandleClearGuestKnowledge deletes all notes for a guest
func (h *AISettingsHandler) HandleClearGuestKnowledge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GuestID string `json:"guest_id"`
		Confirm bool   `json:"confirm"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.GuestID == "" {
		http.Error(w, "guest_id is required", http.StatusBadRequest)
		return
	}

	if !req.Confirm {
		http.Error(w, "confirm must be true to clear all notes", http.StatusBadRequest)
		return
	}

	// Get existing knowledge and delete all notes
	existing, err := h.GetAIService(r.Context()).GetGuestKnowledge(req.GuestID)
	if err != nil {
		http.Error(w, "Failed to get knowledge: "+err.Error(), http.StatusInternalServerError)
		return
	}

	deleted := 0
	for _, note := range existing.Notes {
		if err := h.GetAIService(r.Context()).DeleteGuestNote(req.GuestID, note.ID); err != nil {
			log.Warn().Err(err).Str("note_id", note.ID).Msg("Failed to delete note")
			continue
		}
		deleted++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"deleted": deleted,
	})
}

// HandleDebugContext returns the system prompt and context that would be sent to the AI
// This is useful for debugging when the AI gives incorrect information
func (h *AISettingsHandler) HandleDebugContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Build a sample request to see what context would be sent
	req := ai.ExecuteRequest{
		Prompt:     "Debug context request",
		TargetType: r.URL.Query().Get("target_type"),
		TargetID:   r.URL.Query().Get("target_id"),
	}

	// Get the debug context from the service
	debugInfo := h.GetAIService(r.Context()).GetDebugContext(req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(debugInfo)
}

// HandleGetConnectedAgents returns the list of agents currently connected via WebSocket
// This is useful for debugging when AI can't reach certain hosts
func (h *AISettingsHandler) HandleGetConnectedAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type agentInfo struct {
		AgentID     string `json:"agent_id"`
		Hostname    string `json:"hostname"`
		Version     string `json:"version"`
		Platform    string `json:"platform"`
		ConnectedAt string `json:"connected_at"`
	}

	var agents []agentInfo
	if h.agentServer != nil {
		for _, a := range h.agentServer.GetConnectedAgents() {
			agents = append(agents, agentInfo{
				AgentID:     a.AgentID,
				Hostname:    a.Hostname,
				Version:     a.Version,
				Platform:    a.Platform,
				ConnectedAt: a.ConnectedAt.Format(time.RFC3339),
			})
		}
	}

	response := map[string]interface{}{
		"count":  len(agents),
		"agents": agents,
		"note":   "Agents connect via WebSocket to /api/agent/ws. If a host is missing, check that pulse-agent is installed and can reach the Pulse server.",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AIInvestigateAlertRequest is the request body for POST /api/ai/investigate-alert
type AIInvestigateAlertRequest struct {
	AlertID      string  `json:"alert_id"`
	ResourceID   string  `json:"resource_id"`
	ResourceName string  `json:"resource_name"`
	ResourceType string  `json:"resource_type"` // guest, node, storage, docker
	AlertType    string  `json:"alert_type"`    // cpu, memory, disk, offline, etc.
	Level        string  `json:"level"`         // warning, critical
	Value        float64 `json:"value"`
	Threshold    float64 `json:"threshold"`
	Message      string  `json:"message"`
	Duration     string  `json:"duration"` // How long the alert has been active
	Node         string  `json:"node,omitempty"`
	VMID         int     `json:"vmid,omitempty"`
}

// HandleInvestigateAlert investigates an alert using AI (POST /api/ai/investigate-alert)
// This is a dedicated endpoint for one-click alert investigation from the UI
func (h *AISettingsHandler) HandleInvestigateAlert(w http.ResponseWriter, r *http.Request) {
	// Handle CORS
	h.setSSECORSHeaders(w, r)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	// Check if AI is enabled
	if !h.GetAIService(r.Context()).IsEnabled() {
		http.Error(w, "Pulse Assistant is not enabled or configured", http.StatusBadRequest)
		return
	}

	// Parse request
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var req AIInvestigateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Build investigation prompt
	investigationPrompt := ai.GenerateAlertInvestigationPrompt(ai.AlertInvestigationRequest{
		AlertID:      req.AlertID,
		ResourceID:   req.ResourceID,
		ResourceName: req.ResourceName,
		ResourceType: req.ResourceType,
		AlertType:    req.AlertType,
		Level:        req.Level,
		Value:        req.Value,
		Threshold:    req.Threshold,
		Message:      req.Message,
		Duration:     req.Duration,
		Node:         req.Node,
		VMID:         req.VMID,
	})

	log.Info().
		Str("alert_id", req.AlertID).
		Str("resource", req.ResourceName).
		Str("type", req.AlertType).
		Msg("AI alert investigation started")

	// Set up SSE streaming
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Transfer-Encoding", "identity")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Disable write/read deadlines for SSE
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})
	_ = rc.SetReadDeadline(time.Time{})

	flusher.Flush()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// Heartbeat routine
	heartbeatDone := make(chan struct{})
	var clientDisconnected atomic.Bool
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
				_, err := w.Write([]byte(": heartbeat\n\n"))
				if err != nil {
					clientDisconnected.Store(true)
					return
				}
				flusher.Flush()
			case <-heartbeatDone:
				return
			}
		}
	}()
	defer close(heartbeatDone)

	safeWrite := func(data []byte) bool {
		if clientDisconnected.Load() {
			return false
		}
		_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, err := w.Write(data)
		if err != nil {
			clientDisconnected.Store(true)
			return false
		}
		flusher.Flush()
		return true
	}

	// Determine target type and ID from alert info
	targetType := req.ResourceType
	targetID := req.ResourceID

	// Map resource type to expected target type format
	switch req.ResourceType {
	case "guest":
		// Could be VM or container - try to determine from VMID
		if req.VMID > 0 {
			targetType = "container" // Default to container, AI will figure it out
		}
	case "docker":
		targetType = "docker_container"
	}

	// Stream callback
	callback := func(event ai.StreamEvent) {
		if event.Type == "done" {
			return
		}
		data, err := json.Marshal(event)
		if err != nil {
			return
		}
		safeWrite([]byte("data: " + string(data) + "\n\n"))
	}

	// Execute with streaming
	defer func() {
		if !clientDisconnected.Load() {
			doneEvent := ai.StreamEvent{Type: "done"}
			data, _ := json.Marshal(doneEvent)
			safeWrite([]byte("data: " + string(data) + "\n\n"))
		}
	}()

	resp, err := h.GetAIService(r.Context()).ExecuteStream(ctx, ai.ExecuteRequest{
		Prompt:     investigationPrompt,
		TargetType: targetType,
		TargetID:   targetID,
		Context: map[string]interface{}{
			"alertId":      req.AlertID,
			"alertType":    req.AlertType,
			"alertLevel":   req.Level,
			"alertMessage": req.Message,
			"guestName":    req.ResourceName,
			"node":         req.Node,
		},
	}, callback)

	if err != nil {
		log.Error().Err(err).Msg("AI alert investigation failed")
		errEvent := ai.StreamEvent{Type: "error", Data: map[string]string{"message": "Alert investigation failed. Please try again."}}
		data, _ := json.Marshal(errEvent)
		safeWrite([]byte("data: " + string(data) + "\n\n"))
		return
	}

	// Send completion event
	finalEvent := struct {
		Type         string             `json:"type"`
		Model        string             `json:"model"`
		InputTokens  int                `json:"input_tokens"`
		OutputTokens int                `json:"output_tokens"`
		ToolCalls    []ai.ToolExecution `json:"tool_calls,omitempty"`
	}{
		Type:         "complete",
		Model:        resp.Model,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		ToolCalls:    resp.ToolCalls,
	}
	data, _ := json.Marshal(finalEvent)
	safeWrite([]byte("data: " + string(data) + "\n\n"))

	if req.AlertID != "" {
		h.GetAIService(r.Context()).RecordIncidentAnalysis(req.AlertID, "Pulse Assistant alert investigation completed", map[string]interface{}{
			"model":         resp.Model,
			"tool_calls":    len(resp.ToolCalls),
			"input_tokens":  resp.InputTokens,
			"output_tokens": resp.OutputTokens,
		})
	}

	log.Info().
		Str("alert_id", req.AlertID).
		Str("model", resp.Model).
		Int("tool_calls", len(resp.ToolCalls)).
		Msg("AI alert investigation completed")

	if req.AlertID != "" {
		h.GetAIService(r.Context()).RecordIncidentAnalysis(req.AlertID, "Pulse Assistant investigation completed", map[string]interface{}{
			"model":         resp.Model,
			"input_tokens":  resp.InputTokens,
			"output_tokens": resp.OutputTokens,
			"tool_calls":    len(resp.ToolCalls),
		})
	}
}

// SetAlertProvider sets the alert provider for AI context
// Sets on both the legacy service and all tenant services to ensure multi-tenant support.
func (h *AISettingsHandler) SetAlertProvider(ap ai.AlertProvider) {
	h.legacyAIService.SetAlertProvider(ap)

	h.aiServicesMu.RLock()
	defer h.aiServicesMu.RUnlock()
	for _, svc := range h.aiServices {
		svc.SetAlertProvider(ap)
	}
}

// SetAlertResolver sets the alert resolver for AI Patrol autonomous alert management
// Sets on both the legacy service and all tenant services to ensure multi-tenant support.
func (h *AISettingsHandler) SetAlertResolver(resolver ai.AlertResolver) {
	h.legacyAIService.SetAlertResolver(resolver)

	h.aiServicesMu.RLock()
	defer h.aiServicesMu.RUnlock()
	for _, svc := range h.aiServices {
		svc.SetAlertResolver(resolver)
	}
}

// oauthSessions stores active OAuth sessions (state -> session)
// In production, consider using a more robust session store with expiry
var oauthSessions = make(map[string]*providers.OAuthSession)
var oauthSessionsMu sync.Mutex

// HandleOAuthStart initiates the OAuth flow for Claude Pro/Max subscription (POST /api/ai/oauth/start)
// Returns an authorization URL for the user to visit manually
func (h *AISettingsHandler) HandleOAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate OAuth session (redirect URI is not used since we use Anthropic's callback)
	session, err := providers.GenerateOAuthSession("")
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate OAuth session")
		http.Error(w, "Failed to start OAuth flow", http.StatusInternalServerError)
		return
	}

	// Store session (with cleanup of old sessions)
	oauthSessionsMu.Lock()
	// Clean up sessions older than 15 minutes
	for state, s := range oauthSessions {
		if time.Since(s.CreatedAt) > 15*time.Minute {
			delete(oauthSessions, state)
		}
	}
	oauthSessions[session.State] = session
	oauthSessionsMu.Unlock()

	// Get authorization URL
	authURL := providers.GetAuthorizationURL(session)

	log.Info().
		Str("state", safePrefixForLog(session.State, 8)+"...").
		Str("verifier_len", fmt.Sprintf("%d", len(session.CodeVerifier))).
		Str("auth_url", authURL).
		Msg("Starting Claude OAuth flow - user must visit URL and paste code back")

	// Return the URL for the user to visit
	response := map[string]string{
		"auth_url": authURL,
		"state":    session.State,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write OAuth start response")
	}
}

// HandleOAuthExchange exchanges a manually-pasted authorization code for tokens (POST /api/ai/oauth/exchange)
func (h *AISettingsHandler) HandleOAuthExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Code == "" || req.State == "" {
		http.Error(w, "Missing code or state", http.StatusBadRequest)
		return
	}

	// Trim any whitespace from the code (user might have copied extra spaces)
	code := strings.TrimSpace(req.Code)

	// Anthropic's callback page displays the code as "code#state"
	// We need to extract just the code part before the #
	if idx := strings.Index(code, "#"); idx > 0 {
		code = code[:idx]
	}

	log.Debug().
		Str("code_len", fmt.Sprintf("%d", len(code))).
		Str("code_prefix", code[:min(20, len(code))]).
		Str("state_prefix", req.State[:min(8, len(req.State))]).
		Msg("Processing OAuth code exchange")

	// Look up session
	oauthSessionsMu.Lock()
	session, ok := oauthSessions[req.State]
	if ok {
		delete(oauthSessions, req.State) // One-time use
	}
	oauthSessionsMu.Unlock()

	if !ok {
		log.Error().Str("state", req.State[:min(8, len(req.State))]+"...").Msg("OAuth exchange with unknown state")
		http.Error(w, "Invalid or expired session. Please start the OAuth flow again.", http.StatusBadRequest)
		return
	}

	// Exchange code for tokens
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	tokens, err := providers.ExchangeCodeForTokens(ctx, code, session)
	if err != nil {
		log.Error().Err(err).Msg("Failed to exchange OAuth code for tokens")
		http.Error(w, "Failed to exchange authorization code: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Try to create an API key from the OAuth access token
	// Team/Enterprise users get org:create_api_key scope and can create API keys
	// Pro/Max users don't have this scope and will use OAuth tokens directly
	apiKey, err := providers.CreateAPIKeyFromOAuth(ctx, tokens.AccessToken)
	if err != nil {
		// Check if it's a permission error (Pro/Max users)
		if strings.Contains(err.Error(), "org:create_api_key") || strings.Contains(err.Error(), "403") {
			log.Info().Msg("User doesn't have org:create_api_key permission - will use OAuth tokens directly")
			// This is fine for Pro/Max users - they'll use OAuth tokens
		} else {
			log.Error().Err(err).Msg("Failed to create API key from OAuth token")
			http.Error(w, "Failed to create API key: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	if apiKey != "" {
		log.Info().Msg("Successfully created API key from OAuth - using subscription-based billing")
	}

	// Load existing settings
	settings, err := h.getPersistence(r.Context()).LoadAIConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load Pulse Assistant settings for OAuth")
		settings = config.NewDefaultAIConfig()
	}
	if settings == nil {
		settings = config.NewDefaultAIConfig()
	}

	// Update settings
	settings.Provider = config.AIProviderAnthropic
	settings.AuthMethod = config.AuthMethodOAuth
	settings.OAuthAccessToken = tokens.AccessToken
	settings.OAuthRefreshToken = tokens.RefreshToken
	settings.OAuthExpiresAt = tokens.ExpiresAt
	settings.Enabled = true

	// If we got an API key, use it; otherwise use OAuth tokens directly
	if apiKey != "" {
		settings.APIKey = apiKey
	} else {
		// Pro/Max users: clear any old API key, will use OAuth client
		settings.ClearAPIKey()
	}

	// Save settings
	if err := h.getPersistence(r.Context()).SaveAIConfig(*settings); err != nil {
		log.Error().Err(err).Msg("Failed to save OAuth tokens")
		http.Error(w, "Failed to save OAuth credentials", http.StatusInternalServerError)
		return
	}

	// Reload the AI service with new settings
	if err := h.GetAIService(r.Context()).Reload(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload AI service after OAuth setup")
	}

	log.Info().Msg("Claude OAuth authentication successful")

	response := map[string]interface{}{
		"success": true,
		"message": "Successfully connected to Claude with your subscription",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write OAuth exchange response")
	}
}

// HandleOAuthCallback handles the OAuth callback (GET /api/ai/oauth/callback)
// This is kept for backwards compatibility but mainly serves as a fallback
func (h *AISettingsHandler) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get code and state from query params
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")
	errDesc := r.URL.Query().Get("error_description")

	// Check for OAuth error
	if errParam != "" {
		log.Error().
			Str("error", errParam).
			Str("description", errDesc).
			Msg("OAuth authorization failed")
		// Redirect to settings page with error
		http.Redirect(w, r, "/settings?ai_oauth_error="+errParam, http.StatusTemporaryRedirect)
		return
	}

	if code == "" || state == "" {
		log.Error().Msg("OAuth callback missing code or state")
		http.Redirect(w, r, "/settings?ai_oauth_error=missing_params", http.StatusTemporaryRedirect)
		return
	}

	// Look up session
	oauthSessionsMu.Lock()
	session, ok := oauthSessions[state]
	if ok {
		delete(oauthSessions, state) // One-time use
	}
	oauthSessionsMu.Unlock()

	if !ok {
		log.Error().Str("state", state).Msg("OAuth callback with unknown state")
		http.Redirect(w, r, "/settings?ai_oauth_error=invalid_state", http.StatusTemporaryRedirect)
		return
	}

	// Exchange code for tokens
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	tokens, err := providers.ExchangeCodeForTokens(ctx, code, session)
	if err != nil {
		log.Error().Err(err).Msg("Failed to exchange OAuth code for tokens")
		http.Redirect(w, r, "/settings?ai_oauth_error=token_exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	// Load existing settings
	settings, err := h.getPersistence(r.Context()).LoadAIConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load Pulse Assistant settings for OAuth")
		settings = config.NewDefaultAIConfig()
	}
	if settings == nil {
		settings = config.NewDefaultAIConfig()
	}

	// Update settings with OAuth tokens
	settings.Provider = config.AIProviderAnthropic
	settings.AuthMethod = config.AuthMethodOAuth
	settings.OAuthAccessToken = tokens.AccessToken
	settings.OAuthRefreshToken = tokens.RefreshToken
	settings.OAuthExpiresAt = tokens.ExpiresAt
	settings.Enabled = true
	// Clear API key since we're using OAuth
	settings.ClearAPIKey()

	// Save settings
	if err := h.getPersistence(r.Context()).SaveAIConfig(*settings); err != nil {
		log.Error().Err(err).Msg("Failed to save OAuth tokens")
		http.Redirect(w, r, "/settings?ai_oauth_error=save_failed", http.StatusTemporaryRedirect)
		return
	}

	// Reload the AI service with new settings
	if err := h.GetAIService(r.Context()).Reload(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload AI service after OAuth setup")
	}

	log.Info().Msg("Claude OAuth authentication successful")

	// Redirect to settings page with success
	http.Redirect(w, r, "/settings?ai_oauth_success=true", http.StatusTemporaryRedirect)
}

// HandleOAuthDisconnect disconnects OAuth and clears tokens (POST /api/ai/oauth/disconnect)
func (h *AISettingsHandler) HandleOAuthDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require admin authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	// Load existing settings
	settings, err := h.getPersistence(r.Context()).LoadAIConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load Pulse Assistant settings for OAuth disconnect")
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}
	if settings == nil {
		settings = config.NewDefaultAIConfig()
	}

	// Clear OAuth tokens
	settings.ClearOAuthTokens()
	settings.AuthMethod = config.AuthMethodAPIKey

	// Save settings
	if err := h.getPersistence(r.Context()).SaveAIConfig(*settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings after OAuth disconnect")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	// Reload the AI service
	if err := h.GetAIService(r.Context()).Reload(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload AI service after OAuth disconnect")
	}

	log.Info().Msg("Claude OAuth disconnected")

	response := map[string]interface{}{
		"success": true,
		"message": "OAuth disconnected successfully",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write OAuth disconnect response")
	}
}

// PatrolStatusResponse is the response for GET /api/ai/patrol/status
type PatrolStatusResponse struct {
	Running          bool       `json:"running"`
	Enabled          bool       `json:"enabled"`
	LastPatrolAt     *time.Time `json:"last_patrol_at,omitempty"`
	NextPatrolAt     *time.Time `json:"next_patrol_at,omitempty"`
	LastDurationMs   int64      `json:"last_duration_ms"`
	ResourcesChecked int        `json:"resources_checked"`
	FindingsCount    int        `json:"findings_count"`
	ErrorCount       int        `json:"error_count"`
	Healthy          bool       `json:"healthy"`
	IntervalMs       int64      `json:"interval_ms"` // Patrol interval in milliseconds
	FixedCount       int        `json:"fixed_count"` // Number of issues auto-fixed by Patrol
	BlockedReason    string     `json:"blocked_reason,omitempty"`
	BlockedAt        *time.Time `json:"blocked_at,omitempty"`
	// License status for Pro feature gating
	LicenseRequired bool   `json:"license_required"` // True if Pro license needed for full features
	LicenseStatus   string `json:"license_status"`   // "active", "expired", "grace_period", "none"
	UpgradeURL      string `json:"upgrade_url,omitempty"`
	Summary         struct {
		Critical int `json:"critical"`
		Warning  int `json:"warning"`
		Watch    int `json:"watch"`
		Info     int `json:"info"`
	} `json:"summary"`
}

// HandleGetPatrolStatus returns the current patrol status (GET /api/ai/patrol/status)
func (h *AISettingsHandler) HandleGetPatrolStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		// Service not initialized (e.g. no persistence/config yet). Return safe defaults.
		response := PatrolStatusResponse{
			Running:         false,
			Enabled:         false,
			Healthy:         true,
			LicenseRequired: true,
			LicenseStatus:   "none",
			UpgradeURL:      conversion.UpgradeURLForFeature(license.FeatureAIAutoFix),
		}
		if err := utils.WriteJSONResponse(w, response); err != nil {
			log.Error().Err(err).Msg("Failed to write patrol status response (no AI service)")
		}
		return
	}

	patrol := aiService.GetPatrolService()
	if patrol == nil {
		// Patrol not initialized
		licenseStatus, _ := aiService.GetLicenseState()
		hasAutoFixFeature := aiService.HasLicenseFeature(license.FeatureAIAutoFix)
		response := PatrolStatusResponse{
			Running:         false,
			Enabled:         false,
			Healthy:         true,
			LicenseRequired: !hasAutoFixFeature,
			LicenseStatus:   licenseStatus,
		}
		if !hasAutoFixFeature {
			response.UpgradeURL = conversion.UpgradeURLForFeature(license.FeatureAIAutoFix)
		}
		if err := utils.WriteJSONResponse(w, response); err != nil {
			log.Error().Err(err).Msg("Failed to write patrol status response")
		}
		return
	}

	status := patrol.GetStatus()
	summary := patrol.GetFindingsSummary()

	// Determine license status for Pro feature gating
	// GetLicenseState returns accurate state: none, active, expired, grace_period
	licenseStatus, _ := aiService.GetLicenseState()
	// Check for auto-fix feature - patrol itself is free, auto-fix requires Pro
	hasAutoFixFeature := aiService.HasLicenseFeature(license.FeatureAIAutoFix)

	// Get fixed count from investigation orchestrator
	fixedCount := 0
	if orchestrator := patrol.GetInvestigationOrchestrator(); orchestrator != nil {
		fixedCount = orchestrator.GetFixedCount()
	}

	response := PatrolStatusResponse{
		Running:          status.Running,
		Enabled:          status.Enabled,
		LastPatrolAt:     status.LastPatrolAt,
		NextPatrolAt:     status.NextPatrolAt,
		LastDurationMs:   status.LastDuration.Milliseconds(),
		ResourcesChecked: status.ResourcesChecked,
		FindingsCount:    status.FindingsCount,
		ErrorCount:       status.ErrorCount,
		Healthy:          status.Healthy,
		IntervalMs:       status.IntervalMs,
		FixedCount:       fixedCount,
		BlockedReason:    status.BlockedReason,
		BlockedAt:        status.BlockedAt,
		LicenseRequired:  !hasAutoFixFeature,
		LicenseStatus:    licenseStatus,
	}
	if !hasAutoFixFeature {
		response.UpgradeURL = conversion.UpgradeURLForFeature(license.FeatureAIAutoFix)
	}
	response.Summary.Critical = summary.Critical
	response.Summary.Warning = summary.Warning
	response.Summary.Watch = summary.Watch
	response.Summary.Info = summary.Info

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write patrol status response")
	}
}

// HandleGetIntelligence returns the unified AI intelligence summary (GET /api/ai/intelligence)
// This provides a single endpoint for system-wide AI insights including:
// - Overall health score and grade
// - Active findings summary
// - Upcoming predictions
// - Recent activity
// - Learning progress
// - Resources at risk
func (h *AISettingsHandler) HandleGetIntelligence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	patrol := h.GetAIService(r.Context()).GetPatrolService()
	if patrol == nil {
		// Return empty intelligence when not initialized
		response := map[string]interface{}{
			"error": "Pulse Patrol service not available",
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := utils.WriteJSONResponse(w, response); err != nil {
			log.Error().Err(err).Msg("Failed to write intelligence response")
		}
		return
	}

	// Get unified intelligence facade
	intel := patrol.GetIntelligence()
	if intel == nil {
		response := map[string]interface{}{
			"error": "Intelligence not initialized",
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := utils.WriteJSONResponse(w, response); err != nil {
			log.Error().Err(err).Msg("Failed to write intelligence response")
		}
		return
	}

	// Check for resource_id query parameter for resource-specific intelligence
	resourceID := r.URL.Query().Get("resource_id")
	if resourceID != "" {
		// Return resource-specific intelligence
		resourceIntel := intel.GetResourceIntelligence(resourceID)
		if err := utils.WriteJSONResponse(w, resourceIntel); err != nil {
			log.Error().Err(err).Msg("Failed to write resource intelligence response")
		}
		return
	}

	// Return system-wide intelligence summary
	summary := intel.GetSummary()
	if err := utils.WriteJSONResponse(w, summary); err != nil {
		log.Error().Err(err).Msg("Failed to write intelligence summary response")
	}
}

// HandlePatrolStream streams real-time patrol analysis via SSE (GET /api/ai/patrol/stream)
func (h *AISettingsHandler) HandlePatrolStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	patrol := aiService.GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	// Set SSE headers
	h.setSSECORSHeaders(w, r)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Disable proxy buffering (e.g. nginx) so events reach clients promptly.
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send an SSE comment to flush headers immediately so clients get the
	// HTTP 200 response right away instead of blocking until the first event.
	if _, err := fmt.Fprintf(w, ": connected\n\n"); err != nil {
		log.Debug().Err(err).Msg("Patrol stream: failed to write initial SSE comment")
		return
	}
	flusher.Flush()

	// Subscribe to patrol stream
	// Note: SubscribeToStream already sends the current buffered output to the channel
	lastID := int64(0)
	if raw := strings.TrimSpace(r.Header.Get("Last-Event-ID")); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil && v > 0 {
			lastID = v
		}
	}
	// EventSource doesn't allow setting headers manually. Accept a query param fallback
	// so clients can resume even when they create a new EventSource instance.
	if lastID == 0 {
		if raw := strings.TrimSpace(r.URL.Query().Get("last_event_id")); raw != "" {
			if v, err := strconv.ParseInt(raw, 10, 64); err == nil && v > 0 {
				lastID = v
			}
		}
	}
	ch := patrol.SubscribeToStreamFrom(lastID)
	defer patrol.UnsubscribeFromStream(ch)

	// Stream events until client disconnects
	ctx := r.Context()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			// SSE comment heartbeat keeps intermediaries from timing out the stream.
			if _, err := fmt.Fprintf(w, ": ping %d\n\n", time.Now().Unix()); err != nil {
				log.Debug().Err(err).Msg("Patrol stream: failed to write heartbeat")
				return
			}
			flusher.Flush()
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			// Best-effort event id for Last-Event-ID support.
			if event.Seq != 0 {
				if _, err := fmt.Fprintf(w, "id: %d\n", event.Seq); err != nil {
					log.Debug().Err(err).Msg("Patrol stream: failed to write event id")
					return
				}
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				log.Debug().Err(err).Msg("Patrol stream: failed to write event data")
				return
			}
			flusher.Flush()
		}
	}
}

// HandleGetPatrolFindings returns all active findings (GET /api/ai/patrol/findings)
func (h *AISettingsHandler) HandleGetPatrolFindings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		// Return empty findings
		if err := utils.WriteJSONResponse(w, []interface{}{}); err != nil {
			log.Error().Err(err).Msg("Failed to write patrol findings response (no AI service)")
		}
		return
	}

	patrol := aiService.GetPatrolService()
	if patrol == nil {
		// Return empty findings
		if err := utils.WriteJSONResponse(w, []interface{}{}); err != nil {
			log.Error().Err(err).Msg("Failed to write patrol findings response")
		}
		return
	}

	// Check for resource_id query parameter
	resourceID := r.URL.Query().Get("resource_id")
	var findings []*ai.Finding
	if resourceID != "" {
		findings = patrol.GetFindingsForResource(resourceID)
	} else {
		findings = patrol.GetAllFindings()
	}

	// Optional limit parameter (for relay proxy clients with body size constraints)
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(findings) {
			findings = findings[:limit]
		}
	}

	if err := utils.WriteJSONResponse(w, findings); err != nil {
		log.Error().Err(err).Msg("Failed to write patrol findings response")
	}
}

// HandleForcePatrol triggers an immediate patrol run (POST /api/ai/patrol/run)
func (h *AISettingsHandler) HandleForcePatrol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require admin authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	patrol := aiService.GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	// Cadence cap: Community tier is limited to 1 patrol run per hour.
	// Patrol itself is free (ai_patrol), but higher cadence is gated behind Pro/Cloud.
	if !aiService.HasLicenseFeature(license.FeatureAIAutoFix) {
		if last := patrol.GetStatus().LastPatrolAt; last != nil {
			if since := time.Since(*last); since < 1*time.Hour {
				remaining := (1*time.Hour - since).Round(time.Minute)
				writeErrorResponse(w, http.StatusTooManyRequests, "patrol_rate_limited",
					fmt.Sprintf("Community tier is limited to 1 patrol run per hour. Try again in %s.", remaining), nil)
				return
			}
		}
	}

	// Trigger patrol asynchronously
	patrol.ForcePatrol(r.Context())

	response := map[string]interface{}{
		"success": true,
		"message": "Triggered patrol run",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write force patrol response")
	}
}

// HandleAcknowledgeFinding acknowledges a finding (POST /api/ai/patrol/acknowledge)
// This marks the finding as seen but keeps it visible (dimmed). Auto-resolve removes it when condition clears.
// This matches alert acknowledgement behavior for UI consistency.
func (h *AISettingsHandler) HandleAcknowledgeFinding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	patrol := h.GetAIService(r.Context()).GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		FindingID string `json:"finding_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FindingID == "" {
		http.Error(w, "finding_id is required", http.StatusBadRequest)
		return
	}

	findings := patrol.GetFindings()

	// Try patrol findings store first
	var detectedAt time.Time
	var category, severity, resourceID, findingKey string
	foundInPatrol := false

	finding := findings.Get(req.FindingID)
	if finding != nil {
		foundInPatrol = true
		detectedAt = finding.DetectedAt
		category = string(finding.Category)
		severity = string(finding.Severity)
		resourceID = finding.ResourceID
		findingKey = finding.Key
	}

	// If not in patrol findings, check the unified store (for threshold alerts)
	unifiedStore := h.GetUnifiedStore()
	if !foundInPatrol && unifiedStore != nil {
		unifiedFinding := unifiedStore.Get(req.FindingID)
		if unifiedFinding != nil {
			detectedAt = unifiedFinding.DetectedAt
			category = string(unifiedFinding.Category)
			severity = string(unifiedFinding.Severity)
			resourceID = unifiedFinding.ResourceID
			findingKey = unifiedFinding.ID
		} else {
			http.Error(w, "Finding not found", http.StatusNotFound)
			return
		}
	} else if !foundInPatrol {
		http.Error(w, "Finding not found", http.StatusNotFound)
		return
	}

	// Acknowledge in patrol findings if it exists there
	if foundInPatrol {
		if !findings.Acknowledge(req.FindingID) {
			http.Error(w, "Finding not found", http.StatusNotFound)
			return
		}
	}

	// Acknowledge in unified store (for both patrol and threshold alerts)
	if unifiedStore != nil {
		unifiedStore.Acknowledge(req.FindingID)
	}

	// Record to learning store
	if h.learningStore != nil {
		h.learningStore.RecordFeedback(learning.FeedbackRecord{
			FindingID:    req.FindingID,
			FindingKey:   findingKey,
			ResourceID:   resourceID,
			Category:     category,
			Severity:     severity,
			Action:       learning.ActionAcknowledge,
			TimeToAction: time.Since(detectedAt),
		})
	}

	log.Info().
		Str("finding_id", req.FindingID).
		Msg("AI Patrol: Finding acknowledged by user")

	response := map[string]interface{}{
		"success": true,
		"message": "Finding acknowledged",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write acknowledge response")
	}
}

// HandleSnoozeFinding snoozes a finding for a specified duration (POST /api/ai/patrol/snooze)
// Snoozed findings are hidden from the active list but will reappear if condition persists after snooze expires
func (h *AISettingsHandler) HandleSnoozeFinding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	patrol := h.GetAIService(r.Context()).GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		FindingID     string `json:"finding_id"`
		DurationHours int    `json:"duration_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FindingID == "" {
		http.Error(w, "finding_id is required", http.StatusBadRequest)
		return
	}

	if req.DurationHours <= 0 {
		http.Error(w, "duration_hours must be positive", http.StatusBadRequest)
		return
	}

	// Cap snooze duration at 7 days
	if req.DurationHours > 168 {
		req.DurationHours = 168
	}

	findings := patrol.GetFindings()
	duration := time.Duration(req.DurationHours) * time.Hour

	// Try patrol findings store first
	var detectedAt time.Time
	var category, severity, resourceID, findingKey string
	foundInPatrol := false

	finding := findings.Get(req.FindingID)
	if finding != nil {
		foundInPatrol = true
		detectedAt = finding.DetectedAt
		category = string(finding.Category)
		severity = string(finding.Severity)
		resourceID = finding.ResourceID
		findingKey = finding.Key
	}

	// If not in patrol findings, check the unified store (for threshold alerts)
	unifiedStore := h.GetUnifiedStore()
	if !foundInPatrol && unifiedStore != nil {
		unifiedFinding := unifiedStore.Get(req.FindingID)
		if unifiedFinding != nil {
			detectedAt = unifiedFinding.DetectedAt
			category = string(unifiedFinding.Category)
			severity = string(unifiedFinding.Severity)
			resourceID = unifiedFinding.ResourceID
			findingKey = unifiedFinding.ID
		} else {
			http.Error(w, "Finding not found or already resolved", http.StatusNotFound)
			return
		}
	} else if !foundInPatrol {
		http.Error(w, "Finding not found or already resolved", http.StatusNotFound)
		return
	}

	// Snooze in patrol findings if it exists there
	if foundInPatrol {
		if !findings.Snooze(req.FindingID, duration) {
			http.Error(w, "Finding not found or already resolved", http.StatusNotFound)
			return
		}
	}

	// Snooze in unified store (for both patrol and threshold alerts)
	if unifiedStore != nil {
		unifiedStore.Snooze(req.FindingID, duration)
	}

	// Record to learning store
	if h.learningStore != nil {
		h.learningStore.RecordFeedback(learning.FeedbackRecord{
			FindingID:    req.FindingID,
			FindingKey:   findingKey,
			ResourceID:   resourceID,
			Category:     category,
			Severity:     severity,
			Action:       learning.ActionSnooze,
			TimeToAction: time.Since(detectedAt),
		})
	}

	log.Info().
		Str("finding_id", req.FindingID).
		Int("hours", req.DurationHours).
		Msg("AI Patrol: Finding snoozed by user")

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Finding snoozed for %d hours", req.DurationHours),
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write snooze response")
	}
}

// HandleResolveFinding manually marks a finding as resolved (POST /api/ai/patrol/resolve)
// Use this when the user has fixed the issue and wants to mark it as resolved
func (h *AISettingsHandler) HandleResolveFinding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	patrol := h.GetAIService(r.Context()).GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		FindingID string `json:"finding_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FindingID == "" {
		http.Error(w, "finding_id is required", http.StatusBadRequest)
		return
	}

	findings := patrol.GetFindings()

	// Get finding details before resolving (for learning/analytics)
	finding := findings.Get(req.FindingID)
	if finding == nil {
		http.Error(w, "Finding not found or already resolved", http.StatusNotFound)
		return
	}

	// Capture details before action
	detectedAt := finding.DetectedAt
	category := string(finding.Category)
	severity := string(finding.Severity)
	resourceID := finding.ResourceID

	// Mark as manually resolved (auto=false since user did it)
	if !findings.Resolve(req.FindingID, false) {
		http.Error(w, "Finding not found or already resolved", http.StatusNotFound)
		return
	}

	// Mirror into unified store for consistent UI state
	if store := h.GetUnifiedStore(); store != nil {
		store.Resolve(req.FindingID)
	}

	// Record to learning store - manual resolve = user fixed the issue
	if h.learningStore != nil {
		h.learningStore.RecordFeedback(learning.FeedbackRecord{
			FindingID:    req.FindingID,
			FindingKey:   finding.Key,
			ResourceID:   resourceID,
			Category:     category,
			Severity:     severity,
			Action:       learning.ActionQuickFix, // Manual resolve means user took action to fix
			TimeToAction: time.Since(detectedAt),
		})
	}

	log.Info().
		Str("finding_id", req.FindingID).
		Msg("AI Patrol: Finding manually resolved by user")

	response := map[string]interface{}{
		"success": true,
		"message": "Finding marked as resolved",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write resolve response")
	}
}

// HandleSetFindingNote sets or updates a user note on a finding (POST /api/ai/patrol/findings/note)
// Notes provide context that Patrol sees on future runs (e.g., "PBS server was decommissioned").
func (h *AISettingsHandler) HandleSetFindingNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	patrol := h.GetAIService(r.Context()).GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		FindingID string `json:"finding_id"`
		Note      string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FindingID == "" {
		http.Error(w, "finding_id is required", http.StatusBadRequest)
		return
	}

	findings := patrol.GetFindings()
	ok := findings.SetUserNote(req.FindingID, req.Note)
	if !ok {
		http.Error(w, "Finding not found", http.StatusNotFound)
		return
	}

	// Mirror the note to the unified store immediately so it's visible
	// without waiting for the next patrol sync cycle
	if unifiedStore := h.GetUnifiedStore(); unifiedStore != nil {
		unifiedStore.SetUserNote(req.FindingID, req.Note)
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Note updated",
	}
	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write set-note response")
	}
}

// HandleDismissFinding dismisses a finding with a reason and optional note (POST /api/ai/patrol/dismiss)
// This is part of the LLM memory system - dismissed findings are included in context to prevent re-raising
// Valid reasons: "not_an_issue", "expected_behavior", "will_fix_later"
func (h *AISettingsHandler) HandleDismissFinding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	patrol := h.GetAIService(r.Context()).GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		FindingID string `json:"finding_id"`
		Reason    string `json:"reason"` // "not_an_issue", "expected_behavior", "will_fix_later"
		Note      string `json:"note"`   // Optional freeform note
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FindingID == "" {
		http.Error(w, "finding_id is required", http.StatusBadRequest)
		return
	}

	if req.Reason == "" {
		http.Error(w, "reason is required", http.StatusBadRequest)
		return
	}

	// Validate reason
	validReasons := map[string]bool{
		"not_an_issue":      true,
		"expected_behavior": true,
		"will_fix_later":    true,
	}
	if req.Reason != "" && !validReasons[req.Reason] {
		http.Error(w, "Invalid reason. Valid values: not_an_issue, expected_behavior, will_fix_later", http.StatusBadRequest)
		return
	}

	findings := patrol.GetFindings()

	// Try patrol findings store first
	var detectedAt time.Time
	var category, severity, resourceID, findingKey string
	foundInPatrol := false

	finding := findings.Get(req.FindingID)
	if finding != nil {
		foundInPatrol = true
		detectedAt = finding.DetectedAt
		category = string(finding.Category)
		severity = string(finding.Severity)
		resourceID = finding.ResourceID
		findingKey = finding.Key
	}

	// If not in patrol findings, check the unified store (for threshold alerts)
	unifiedStore := h.GetUnifiedStore()
	if !foundInPatrol && unifiedStore != nil {
		unifiedFinding := unifiedStore.Get(req.FindingID)
		if unifiedFinding != nil {
			detectedAt = unifiedFinding.DetectedAt
			category = string(unifiedFinding.Category)
			severity = string(unifiedFinding.Severity)
			resourceID = unifiedFinding.ResourceID
			findingKey = unifiedFinding.ID // Use ID as key for unified findings
		} else {
			http.Error(w, "Finding not found", http.StatusNotFound)
			return
		}
	} else if !foundInPatrol {
		http.Error(w, "Finding not found", http.StatusNotFound)
		return
	}

	// Dismiss in patrol findings if it exists there
	if foundInPatrol {
		if !findings.Dismiss(req.FindingID, req.Reason, req.Note) {
			http.Error(w, "Finding not found", http.StatusNotFound)
			return
		}
	}

	// Dismiss in unified store (for both patrol and threshold alerts)
	if unifiedStore != nil {
		unifiedStore.Dismiss(req.FindingID, req.Reason, req.Note)
	}

	// Map dismiss reason to learning action
	var learningAction learning.UserAction
	switch req.Reason {
	case "not_an_issue":
		learningAction = learning.ActionDismissNotAnIssue
	case "expected_behavior":
		learningAction = learning.ActionDismissExpected
	case "will_fix_later":
		learningAction = learning.ActionDismissWillFixLater
	default:
		learningAction = learning.ActionDismissNotAnIssue // Default
	}

	// Record to learning store
	if h.learningStore != nil {
		h.learningStore.RecordFeedback(learning.FeedbackRecord{
			FindingID:    req.FindingID,
			FindingKey:   findingKey,
			ResourceID:   resourceID,
			Category:     category,
			Severity:     severity,
			Action:       learningAction,
			UserNote:     req.Note,
			TimeToAction: time.Since(detectedAt),
		})
	}

	log.Info().
		Str("finding_id", req.FindingID).
		Str("reason", req.Reason).
		Bool("has_note", req.Note != "").
		Msg("AI Patrol: Finding dismissed by user with reason")

	response := map[string]interface{}{
		"success": true,
		"message": "Finding dismissed",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write dismiss response")
	}
}

// HandleSuppressFinding permanently suppresses similar findings for a resource (POST /api/ai/patrol/suppress)
// The LLM will be told never to re-raise findings of this type for this resource
func (h *AISettingsHandler) HandleSuppressFinding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	patrol := h.GetAIService(r.Context()).GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		FindingID string `json:"finding_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FindingID == "" {
		http.Error(w, "finding_id is required", http.StatusBadRequest)
		return
	}

	findings := patrol.GetFindings()

	// Get finding details before suppressing (for learning/analytics)
	finding := findings.Get(req.FindingID)
	if finding == nil {
		http.Error(w, "Finding not found", http.StatusNotFound)
		return
	}

	// Capture details before action
	detectedAt := finding.DetectedAt
	category := string(finding.Category)
	severity := string(finding.Severity)
	resourceID := finding.ResourceID

	if !findings.Suppress(req.FindingID) {
		http.Error(w, "Finding not found", http.StatusNotFound)
		return
	}

	// Mirror into unified store for consistent UI state
	if store := h.GetUnifiedStore(); store != nil {
		store.Dismiss(req.FindingID, "not_an_issue", "Permanently suppressed by user")
	}

	// Record to learning store - suppress is a strong "not an issue" signal
	if h.learningStore != nil {
		h.learningStore.RecordFeedback(learning.FeedbackRecord{
			FindingID:    req.FindingID,
			FindingKey:   finding.Key,
			ResourceID:   resourceID,
			Category:     category,
			Severity:     severity,
			Action:       learning.ActionDismissNotAnIssue, // Suppress = permanent dismissal
			UserNote:     "Permanently suppressed by user",
			TimeToAction: time.Since(detectedAt),
		})
	}

	log.Info().
		Str("finding_id", req.FindingID).
		Msg("AI Patrol: Finding type permanently suppressed by user")

	response := map[string]interface{}{
		"success": true,
		"message": "Finding type suppressed - similar issues will not be raised again",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write suppress response")
	}
}

// HandleClearAllFindings clears all AI findings (DELETE /api/ai/patrol/findings)
// This allows users to clear accumulated findings, especially useful for users who
// accumulated findings before the patrol-without-AI bug was fixed.
func (h *AISettingsHandler) HandleClearAllFindings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication (already wrapped by RequireAuth in router)
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	// Check for confirm parameter
	if r.URL.Query().Get("confirm") != "true" {
		http.Error(w, "confirm=true query parameter required", http.StatusBadRequest)
		return
	}

	patrol := h.GetAIService(r.Context()).GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	findings := patrol.GetFindings()
	count := findings.ClearAll()

	log.Info().Int("count", count).Msg("Cleared all AI findings")

	response := map[string]interface{}{
		"success": true,
		"cleared": count,
		"message": fmt.Sprintf("Cleared %d findings", count),
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write clear findings response")
	}
}

// HandleGetFindingsHistory returns all findings including resolved for history (GET /api/ai/patrol/history)
func (h *AISettingsHandler) HandleGetFindingsHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	patrol := h.GetAIService(r.Context()).GetPatrolService()
	if patrol == nil {
		// Return empty history
		if err := utils.WriteJSONResponse(w, []interface{}{}); err != nil {
			log.Error().Err(err).Msg("Failed to write findings history response")
		}
		return
	}

	// Parse optional startTime query parameter
	var startTime *time.Time
	if startTimeStr := r.URL.Query().Get("start_time"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = &t
		}
	}

	findings := patrol.GetFindingsHistory(startTime)

	if err := utils.WriteJSONResponse(w, findings); err != nil {
		log.Error().Err(err).Msg("Failed to write findings history response")
	}
}

// HandleGetPatrolRunHistory returns the history of patrol check runs (GET /api/ai/patrol/runs)
func (h *AISettingsHandler) HandleGetPatrolRunHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		// Return empty history
		if err := utils.WriteJSONResponse(w, []interface{}{}); err != nil {
			log.Error().Err(err).Msg("Failed to write patrol run history response (no AI service)")
		}
		return
	}

	patrol := aiService.GetPatrolService()
	if patrol == nil {
		// Return empty history
		if err := utils.WriteJSONResponse(w, []interface{}{}); err != nil {
			log.Error().Err(err).Msg("Failed to write patrol run history response")
		}
		return
	}

	// Parse optional limit query parameter (default: 50)
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && l > 0 {
			if limit > 100 {
				limit = 100 // Cap at MaxPatrolRunHistory
			}
		}
	}

	runs := patrol.GetRunHistory(limit)

	// By default, omit full tool call arrays to keep payloads lean.
	// Use ?include=tool_calls to get the full array.
	includeToolCalls := r.URL.Query().Get("include") == "tool_calls"
	if !includeToolCalls {
		for i := range runs {
			runs[i].ToolCalls = nil
		}
	}

	if err := utils.WriteJSONResponse(w, runs); err != nil {
		log.Error().Err(err).Msg("Failed to write patrol run history response")
	}
}

// HandleGetAICostSummary returns AI usage rollups (GET /api/ai/cost/summary?days=N).
func (h *AISettingsHandler) HandleGetAICostSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse optional days query parameter (default: 30, max: 365)
	days := 30
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if _, err := fmt.Sscanf(daysStr, "%d", &days); err == nil && days > 0 {
			if days > 365 {
				days = 365
			}
		}
	}

	var summary cost.Summary
	if h.GetAIService(r.Context()) != nil {
		summary = h.GetAIService(r.Context()).GetCostSummary(days)
	} else {
		summary = cost.Summary{
			Days:           days,
			PricingAsOf:    cost.PricingAsOf(),
			ProviderModels: []cost.ProviderModelSummary{},
			DailyTotals:    []cost.DailySummary{},
			Totals:         cost.ProviderModelSummary{Provider: "all"},
		}
	}

	if err := utils.WriteJSONResponse(w, summary); err != nil {
		log.Error().Err(err).Msg("Failed to write AI cost summary response")
	}
}

// HandleResetAICostHistory deletes retained AI usage events (POST /api/ai/cost/reset).
func (h *AISettingsHandler) HandleResetAICostHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.GetAIService(r.Context()) == nil {
		http.Error(w, "Pulse Assistant service unavailable", http.StatusServiceUnavailable)
		return
	}

	backupFile := ""
	if h.getPersistence(r.Context()) != nil {
		configDir := h.getPersistence(r.Context()).DataDir()
		if strings.TrimSpace(configDir) != "" {
			usagePath := filepath.Join(configDir, "ai_usage_history.json")
			if _, err := os.Stat(usagePath); err == nil {
				ts := time.Now().UTC().Format("20060102-150405")
				backupFile = fmt.Sprintf("ai_usage_history.json.bak-%s", ts)
				backupPath := filepath.Join(configDir, backupFile)
				if err := os.Rename(usagePath, backupPath); err != nil {
					log.Error().Err(err).Str("path", usagePath).Msg("Failed to backup Pulse Assistant usage history before reset")
					http.Error(w, "Failed to backup Pulse Assistant usage history", http.StatusInternalServerError)
					return
				}
			}
		}
	}

	if err := h.GetAIService(r.Context()).ClearCostHistory(); err != nil {
		log.Error().Err(err).Msg("Failed to clear Pulse Assistant usage history")
		http.Error(w, "Failed to clear Pulse Assistant usage history", http.StatusInternalServerError)
		return
	}

	resp := map[string]any{"ok": true}
	if backupFile != "" {
		resp["backup_file"] = backupFile
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to write clear cost history response")
	}
}

// HandleExportAICostHistory exports recent AI usage history as JSON or CSV (GET /api/ai/cost/export?days=N&format=csv|json).
func (h *AISettingsHandler) HandleExportAICostHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.GetAIService(r.Context()) == nil {
		http.Error(w, "Pulse Assistant service unavailable", http.StatusServiceUnavailable)
		return
	}

	days := 30
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if v, err := strconv.Atoi(daysStr); err == nil && v > 0 {
			if v > 365 {
				v = 365
			}
			days = v
		}
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" {
		http.Error(w, "format must be 'json' or 'csv'", http.StatusBadRequest)
		return
	}

	events := h.GetAIService(r.Context()).ListCostEvents(days)

	filename := fmt.Sprintf("pulse-ai-usage-%s-%dd.%s", time.Now().UTC().Format("20060102"), days, format)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		type exportEvent struct {
			cost.UsageEvent
			EstimatedUSD float64 `json:"estimated_usd,omitempty"`
			PricingKnown bool    `json:"pricing_known"`
		}
		exported := make([]exportEvent, 0, len(events))
		for _, e := range events {
			provider, model := cost.ResolveProviderAndModel(e.Provider, e.RequestModel, e.ResponseModel)
			usd, ok, _ := cost.EstimateUSD(provider, model, int64(e.InputTokens), int64(e.OutputTokens))
			exported = append(exported, exportEvent{
				UsageEvent:   e,
				EstimatedUSD: usd,
				PricingKnown: ok,
			})
		}
		resp := map[string]any{
			"days":   days,
			"events": exported,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error().Err(err).Msg("Failed to write AI cost export JSON")
		}
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"timestamp",
		"provider",
		"request_model",
		"response_model",
		"use_case",
		"input_tokens",
		"output_tokens",
		"estimated_usd",
		"pricing_known",
		"target_type",
		"target_id",
		"finding_id",
	})
	for _, e := range events {
		provider, model := cost.ResolveProviderAndModel(e.Provider, e.RequestModel, e.ResponseModel)
		usd, ok, _ := cost.EstimateUSD(provider, model, int64(e.InputTokens), int64(e.OutputTokens))

		_ = cw.Write([]string{
			e.Timestamp.UTC().Format(time.RFC3339Nano),
			e.Provider,
			e.RequestModel,
			e.ResponseModel,
			e.UseCase,
			strconv.Itoa(e.InputTokens),
			strconv.Itoa(e.OutputTokens),
			strconv.FormatFloat(usd, 'f', 6, 64),
			strconv.FormatBool(ok),
			e.TargetType,
			e.TargetID,
			e.FindingID,
		})
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		log.Error().Err(err).Msg("Failed to write AI cost export CSV")
	}
}

// HandleGetSuppressionRules returns all suppression rules (GET /api/ai/patrol/suppressions)
func (h *AISettingsHandler) HandleGetSuppressionRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	patrol := h.GetAIService(r.Context()).GetPatrolService()
	if patrol == nil {
		if err := utils.WriteJSONResponse(w, []interface{}{}); err != nil {
			log.Error().Err(err).Msg("Failed to write suppression rules response")
		}
		return
	}

	findings := patrol.GetFindings()
	rules := findings.GetSuppressionRules()

	if err := utils.WriteJSONResponse(w, rules); err != nil {
		log.Error().Err(err).Msg("Failed to write suppression rules response")
	}
}

// HandleAddSuppressionRule creates a new suppression rule (POST /api/ai/patrol/suppressions)
func (h *AISettingsHandler) HandleAddSuppressionRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	patrol := h.GetAIService(r.Context()).GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ResourceID   string `json:"resource_id"`   // Can be empty for "any resource"
		ResourceName string `json:"resource_name"` // Human-readable name
		Category     string `json:"category"`      // Can be empty for "any category"
		Description  string `json:"description"`   // Required - user's reason
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Description == "" {
		http.Error(w, "description is required", http.StatusBadRequest)
		return
	}

	// Convert category string to FindingCategory
	var category ai.FindingCategory
	switch req.Category {
	case "performance":
		category = ai.FindingCategoryPerformance
	case "capacity":
		category = ai.FindingCategoryCapacity
	case "reliability":
		category = ai.FindingCategoryReliability
	case "backup":
		category = ai.FindingCategoryBackup
	case "security":
		category = ai.FindingCategorySecurity
	case "general":
		category = ai.FindingCategoryGeneral
	case "":
		category = "" // Any category
	default:
		http.Error(w, "Invalid category", http.StatusBadRequest)
		return
	}

	findings := patrol.GetFindings()
	rule := findings.AddSuppressionRule(req.ResourceID, req.ResourceName, category, req.Description)

	log.Info().
		Str("rule_id", rule.ID).
		Str("resource_id", req.ResourceID).
		Str("category", req.Category).
		Str("description", req.Description).
		Msg("AI Patrol: Manual suppression rule created")

	response := map[string]interface{}{
		"success": true,
		"message": "Suppression rule created",
		"rule":    rule,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write add suppression rule response")
	}
}

// HandleDeleteSuppressionRule removes a suppression rule (DELETE /api/ai/patrol/suppressions/:id)
func (h *AISettingsHandler) HandleDeleteSuppressionRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	patrol := h.GetAIService(r.Context()).GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	// Get rule ID from URL path
	ruleID := strings.TrimPrefix(r.URL.Path, "/api/ai/patrol/suppressions/")
	if ruleID == "" {
		http.Error(w, "rule_id is required", http.StatusBadRequest)
		return
	}

	findings := patrol.GetFindings()

	if !findings.DeleteSuppressionRule(ruleID) {
		http.Error(w, "Rule not found", http.StatusNotFound)
		return
	}

	log.Info().
		Str("rule_id", ruleID).
		Msg("AI Patrol: Suppression rule deleted")

	response := map[string]interface{}{
		"success": true,
		"message": "Suppression rule deleted",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write delete suppression rule response")
	}
}

// HandleGetDismissedFindings returns all dismissed/suppressed findings (GET /api/ai/patrol/dismissed)
func (h *AISettingsHandler) HandleGetDismissedFindings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	patrol := h.GetAIService(r.Context()).GetPatrolService()
	if patrol == nil {
		if err := utils.WriteJSONResponse(w, []interface{}{}); err != nil {
			log.Error().Err(err).Msg("Failed to write dismissed findings response")
		}
		return
	}

	findings := patrol.GetFindings()
	dismissed := findings.GetDismissedFindings()

	if err := utils.WriteJSONResponse(w, dismissed); err != nil {
		log.Error().Err(err).Msg("Failed to write dismissed findings response")
	}
}

// ============================================
// AI Chat Sessions API
// ============================================

// HandleListAIChatSessions lists all chat sessions for the current user (GET /api/ai/chat/sessions)
func (h *AISettingsHandler) HandleListAIChatSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	// Get username from auth context
	username := getAuthUsername(h.getConfig(r.Context()), r)

	sessions, err := h.getPersistence(r.Context()).GetAIChatSessionsForUser(username)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load chat sessions")
		http.Error(w, "Failed to load chat sessions", http.StatusInternalServerError)
		return
	}

	// Return summary (without full messages) for list view
	type sessionSummary struct {
		ID           string    `json:"id"`
		Title        string    `json:"title"`
		MessageCount int       `json:"message_count"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
	}

	summaries := make([]sessionSummary, 0, len(sessions))
	for _, s := range sessions {
		summaries = append(summaries, sessionSummary{
			ID:           s.ID,
			Title:        s.Title,
			MessageCount: len(s.Messages),
			CreatedAt:    s.CreatedAt,
			UpdatedAt:    s.UpdatedAt,
		})
	}

	if err := utils.WriteJSONResponse(w, summaries); err != nil {
		log.Error().Err(err).Msg("Failed to write chat sessions response")
	}
}

// HandleGetAIChatSession returns a specific chat session (GET /api/ai/chat/sessions/{id})
func (h *AISettingsHandler) HandleGetAIChatSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	// Extract session ID from URL
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/ai/chat/sessions/")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	username := getAuthUsername(h.getConfig(r.Context()), r)

	sessionsData, err := h.getPersistence(r.Context()).LoadAIChatSessions()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load chat sessions")
		http.Error(w, "Failed to load chat sessions", http.StatusInternalServerError)
		return
	}

	session, exists := sessionsData.Sessions[sessionID]
	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Check ownership (allow access if single-user or username matches)
	if session.Username != "" && session.Username != username {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	if err := utils.WriteJSONResponse(w, session); err != nil {
		log.Error().Err(err).Msg("Failed to write chat session response")
	}
}

// HandleSaveAIChatSession creates or updates a chat session (PUT /api/ai/chat/sessions/{id})
func (h *AISettingsHandler) HandleSaveAIChatSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	// Extract session ID from URL
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/ai/chat/sessions/")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	username := getAuthUsername(h.getConfig(r.Context()), r)

	// Parse request body
	var session config.AIChatSession
	if err := json.NewDecoder(r.Body).Decode(&session); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure session ID matches URL
	session.ID = sessionID

	// Check ownership if session exists
	existingData, err := h.getPersistence(r.Context()).LoadAIChatSessions()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load chat sessions")
		http.Error(w, "Failed to load chat sessions", http.StatusInternalServerError)
		return
	}

	if existing, exists := existingData.Sessions[sessionID]; exists {
		// Check ownership
		if existing.Username != "" && existing.Username != username {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
		// Preserve original creation time and username
		session.CreatedAt = existing.CreatedAt
		session.Username = existing.Username
	} else {
		// New session - set creation time and username
		session.CreatedAt = time.Now()
		session.Username = username
	}

	// Auto-generate title from first user message if not set
	if session.Title == "" && len(session.Messages) > 0 {
		for _, msg := range session.Messages {
			if msg.Role == "user" {
				title := msg.Content
				if len(title) > 50 {
					title = title[:47] + "..."
				}
				session.Title = title
				break
			}
		}
	}
	if session.Title == "" {
		session.Title = "New conversation"
	}

	if err := h.getPersistence(r.Context()).SaveAIChatSession(&session); err != nil {
		log.Error().Err(err).Msg("Failed to save chat session")
		http.Error(w, "Failed to save chat session", http.StatusInternalServerError)
		return
	}

	log.Debug().
		Str("session_id", sessionID).
		Str("username", username).
		Int("messages", len(session.Messages)).
		Msg("Chat session saved")

	if err := utils.WriteJSONResponse(w, session); err != nil {
		log.Error().Err(err).Msg("Failed to write save chat session response")
	}
}

// HandleDeleteAIChatSession deletes a chat session (DELETE /api/ai/chat/sessions/{id})
func (h *AISettingsHandler) HandleDeleteAIChatSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !CheckAuth(h.getConfig(r.Context()), w, r) {
		return
	}

	// Extract session ID from URL
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/ai/chat/sessions/")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	username := getAuthUsername(h.getConfig(r.Context()), r)

	// Check ownership
	existingData, err := h.getPersistence(r.Context()).LoadAIChatSessions()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load chat sessions")
		http.Error(w, "Failed to load chat sessions", http.StatusInternalServerError)
		return
	}

	if existing, exists := existingData.Sessions[sessionID]; exists {
		if existing.Username != "" && existing.Username != username {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	if err := h.getPersistence(r.Context()).DeleteAIChatSession(sessionID); err != nil {
		log.Error().Err(err).Msg("Failed to delete chat session")
		http.Error(w, "Failed to delete chat session", http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("session_id", sessionID).
		Str("username", username).
		Msg("Chat session deleted")

	w.WriteHeader(http.StatusNoContent)
}

// getAuthUsername extracts the username from the current auth context
func getAuthUsername(cfg *config.Config, r *http.Request) string {
	// Check OIDC session first
	if cookie, err := r.Cookie("pulse_session"); err == nil && cookie.Value != "" {
		if username := GetSessionUsername(cookie.Value); username != "" {
			return username
		}
	}

	// Check proxy auth
	if cfg.ProxyAuthSecret != "" {
		if valid, username, _ := CheckProxyAuth(cfg, r); valid && username != "" {
			return username
		}
	}

	// Fall back to basic auth username
	if cfg.AuthUser != "" {
		return cfg.AuthUser
	}

	// Single-user mode without auth
	return ""
}

// ============================================================================
// Approval Workflow Handlers (Pro Feature)
// ============================================================================

// HandleListApprovals returns all pending approval requests.
func (h *AISettingsHandler) HandleListApprovals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check license
	if !h.GetAIService(r.Context()).HasLicenseFeature(license.FeatureAIAutoFix) {
		WriteLicenseRequired(w, license.FeatureAIAutoFix, "Pulse Patrol Auto-Fix feature requires Pro license")
		return
	}

	store := approval.GetStore()
	if store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Approval store not initialized", nil)
		return
	}

	approvals := store.GetPendingApprovals()
	if approvals == nil {
		approvals = []*approval.ApprovalRequest{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"approvals": approvals,
		"stats":     store.GetStats(),
	})
}

// HandleGetApproval returns a specific approval request.
func (h *AISettingsHandler) HandleGetApproval(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /api/ai/approvals/{id}
	approvalID := strings.TrimPrefix(r.URL.Path, "/api/ai/approvals/")
	approvalID = strings.TrimSuffix(approvalID, "/")
	approvalID = strings.Split(approvalID, "/")[0] // Handle /approve or /deny suffixes

	if approvalID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Approval ID is required", nil)
		return
	}

	store := approval.GetStore()
	if store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Approval store not initialized", nil)
		return
	}

	req, ok := store.GetApproval(approvalID)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Approval request not found", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}

// HandleApproveCommand approves a pending command and executes it.
func (h *AISettingsHandler) HandleApproveCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check license
	if !h.GetAIService(r.Context()).HasLicenseFeature(license.FeatureAIAutoFix) {
		WriteLicenseRequired(w, license.FeatureAIAutoFix, "Pulse Patrol Auto-Fix feature requires Pro license")
		return
	}

	// SECURITY: Validating command execution scope
	if !ensureScope(w, r, config.ScopeAIExecute) {
		return
	}

	// Extract ID from path: /api/ai/approvals/{id}/approve
	path := strings.TrimPrefix(r.URL.Path, "/api/ai/approvals/")
	path = strings.TrimSuffix(path, "/approve")
	approvalID := strings.TrimSuffix(path, "/")

	if approvalID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Approval ID is required", nil)
		return
	}

	store := approval.GetStore()
	if store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Approval store not initialized", nil)
		return
	}

	username := getAuthUsername(h.getConfig(r.Context()), r)
	if username == "" {
		username = "anonymous"
	}

	req, err := store.Approve(approvalID, username)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "approval_failed", err.Error(), nil)
		return
	}

	// Log audit event
	LogAuditEvent("ai_command_approved", username, GetClientIP(r), r.URL.Path, true,
		fmt.Sprintf("Approved command: %s", truncateForLog(req.Command, 100)))

	// For investigation fixes, execute the command directly since there's no active agentic loop
	if req.ToolID == "investigation_fix" {
		h.executeInvestigationFix(w, r, req)
		return
	}

	// For chat sidebar approvals, the agentic loop will detect approval and execute
	response := map[string]interface{}{
		"approved":    true,
		"request":     req,
		"approval_id": req.ID,
		"message":     "Command approved. Pulse Assistant will now execute it.",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// executeInvestigationFix executes an approved investigation fix directly.
// This is needed because Patrol investigations complete before approval, so there's no
// active agentic loop to execute the command when approved.
func (h *AISettingsHandler) executeInvestigationFix(w http.ResponseWriter, r *http.Request, req *approval.ApprovalRequest) {
	ctx := r.Context()
	findingID := req.TargetID

	// Get the investigation store for this org
	orgID := GetOrgID(ctx)
	h.investigationMu.RLock()
	invStore := h.investigationStores[orgID]
	h.investigationMu.RUnlock()

	if invStore == nil {
		log.Warn().Str("orgID", orgID).Msg("Investigation store not found for org")
		writeErrorResponse(w, http.StatusServiceUnavailable, "no_investigation_store", "Investigation store not available", nil)
		return
	}

	// Get the latest investigation for this finding to get the target host
	session := invStore.GetLatestByFinding(findingID)
	if session == nil {
		log.Warn().Str("findingID", findingID).Msg("No investigation found for finding")
		writeErrorResponse(w, http.StatusNotFound, "no_investigation", "No investigation found for this finding", nil)
		return
	}

	// Resolve target host. Prefer the approved target host stored with the approval request.
	// Legacy approvals may have TargetName as a free-text description; ignore those and
	// fall back to the investigation's proposed fix target.
	approvedTargetHost := ""
	if raw := strings.TrimSpace(req.TargetName); raw != "" && !strings.ContainsAny(raw, " \t\r\n") {
		approvedTargetHost = h.cleanTargetHost(raw)
	}

	sessionTargetHost := ""
	if session.ProposedFix != nil {
		sessionTargetHost = h.cleanTargetHost(session.ProposedFix.TargetHost)
	}

	// Fail closed on drift: if the approved target and latest investigation target differ,
	// do not execute until a fresh approval is requested.
	if approvedTargetHost != "" && sessionTargetHost != "" && !strings.EqualFold(approvedTargetHost, sessionTargetHost) {
		log.Warn().
			Str("findingID", findingID).
			Str("approved_target", approvedTargetHost).
			Str("current_target", sessionTargetHost).
			Msg("Investigation fix target drift detected; blocking execution")
		writeErrorResponse(
			w,
			http.StatusConflict,
			"target_mismatch",
			fmt.Sprintf("Approved target '%s' no longer matches current investigation target '%s'. Re-run investigation and request approval again.", approvedTargetHost, sessionTargetHost),
			nil,
		)
		return
	}

	targetHost := approvedTargetHost
	if targetHost == "" {
		targetHost = sessionTargetHost
	}

	var output string
	var exitCode int
	var execErr string
	var err error

	// Check if this is an MCP tool call or a shell command
	if h.isMCPToolCall(req.Command) {
		// Execute via chat service tool executor
		// Pass the approval ID so the executor knows this is pre-approved
		output, exitCode, err = h.executeMCPToolFix(ctx, req.Command, req.ID)
		if err != nil {
			execErr = err.Error()
			log.Error().Err(err).Str("findingID", findingID).Str("command", req.Command).Msg("Failed to execute MCP tool fix")
		}
	} else {
		// Execute via agent server (shell command)
		if h.agentServer == nil {
			log.Warn().Msg("No agent server available for fix execution")
			writeErrorResponse(w, http.StatusServiceUnavailable, "no_agent_server", "No agent server available", nil)
			return
		}

		// Find the appropriate agent
		agentID := h.findAgentForTarget(targetHost)
		if agentID == "" {
			log.Warn().Str("targetHost", targetHost).Msg("No agent found for target host")
			writeErrorResponse(w, http.StatusServiceUnavailable, "no_agent", "No agent available for target host", nil)
			return
		}

		log.Info().
			Str("findingID", findingID).
			Str("command", req.Command).
			Str("targetHost", targetHost).
			Str("agentID", agentID).
			Msg("Executing approved investigation fix via agent")

		// Execute the command
		var result *agentexec.CommandResultPayload
		result, err = h.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
			Command:    req.Command,
			TargetType: "host",
			TargetID:   "",
		})

		if err != nil {
			execErr = err.Error()
			log.Error().Err(err).Str("findingID", findingID).Msg("Failed to execute investigation fix")
		} else {
			output = result.Stdout
			if result.Stderr != "" {
				output += "\n" + result.Stderr
			}
			exitCode = result.ExitCode
		}
	}

	// Update the investigation outcome
	newOutcome := investigation.OutcomeFixExecuted
	if err != nil || exitCode != 0 {
		newOutcome = investigation.OutcomeFixFailed
	}

	invStore.Complete(session.ID, newOutcome, fmt.Sprintf("Fix executed with exit code %d", exitCode), session.ProposedFix)

	// Update the finding outcome
	h.updateFindingOutcome(ctx, orgID, findingID, string(newOutcome))

	// Log audit event for execution
	success := err == nil && exitCode == 0
	LogAuditEvent("ai_fix_executed", getAuthUsername(h.getConfig(ctx), r), GetClientIP(r), r.URL.Path, success,
		fmt.Sprintf("Executed fix for finding %s: %s (exit code: %d)", findingID, truncateForLog(req.Command, 100), exitCode))

	// Launch background verification if fix executed successfully
	if success {
		aiSvc := h.GetAIService(ctx)
		go func() {
			time.Sleep(30 * time.Second)

			patrol := aiSvc.GetPatrolService()
			if patrol == nil {
				log.Warn().Str("findingID", findingID).Msg("Post-fix verification skipped: no patrol service")
				return
			}

			finding := patrol.GetFindings().Get(findingID)
			if finding == nil {
				log.Warn().Str("findingID", findingID).Msg("Post-fix verification skipped: finding not found")
				return
			}

			bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			verified, verifyErr := patrol.VerifyFixResolved(bgCtx, finding.ResourceID, finding.ResourceType, finding.Key, finding.ID)
			if verifyErr != nil {
				log.Error().Err(verifyErr).Str("findingID", findingID).Msg("Post-fix verification failed with error")
				invStore.Complete(session.ID, investigation.OutcomeFixVerificationFailed, fmt.Sprintf("Fix executed but verification error: %v", verifyErr), session.ProposedFix)
			} else if !verified {
				log.Warn().Str("findingID", findingID).Msg("Post-fix verification: issue persists")
				invStore.Complete(session.ID, investigation.OutcomeFixVerificationFailed, "Fix executed but issue persists after verification.", session.ProposedFix)
			} else {
				log.Info().Str("findingID", findingID).Msg("Post-fix verification: issue resolved")
				invStore.Complete(session.ID, investigation.OutcomeFixVerified, "Fix executed and verified - issue resolved.", session.ProposedFix)
			}
			h.updateFindingOutcome(bgCtx, orgID, findingID, string(invStore.GetLatestByFinding(findingID).Outcome))
		}()
	}

	// Return response
	response := map[string]interface{}{
		"approved":   true,
		"executed":   true,
		"success":    success,
		"output":     output,
		"exit_code":  exitCode,
		"error":      execErr,
		"finding_id": findingID,
		"message":    "Fix executed.",
	}

	if !success {
		response["message"] = fmt.Sprintf("Fix execution failed (exit code %d)", exitCode)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// isMCPToolCall checks if a command is an MCP tool call (vs a shell command)
func (h *AISettingsHandler) isMCPToolCall(command string) bool {
	// MCP tool calls look like: pulse_control_guest(...) or default_api:pulse_control_guest(...)
	return strings.HasPrefix(command, "pulse_") ||
		strings.HasPrefix(command, "default_api:") ||
		strings.Contains(command, "pulse_control_guest") ||
		strings.Contains(command, "pulse_run_command") ||
		strings.Contains(command, "pulse_get_resource")
}

// cleanTargetHost extracts just the hostname from a target host string
// Handles cases like "delly (The container's host is 'delly')" -> "delly"
func (h *AISettingsHandler) cleanTargetHost(targetHost string) string {
	if targetHost == "" {
		return ""
	}
	// If it contains a space and parenthesis, extract the first word
	if idx := strings.Index(targetHost, " ("); idx > 0 {
		return strings.TrimSpace(targetHost[:idx])
	}
	// If it contains a space, take the first word
	if idx := strings.Index(targetHost, " "); idx > 0 {
		return strings.TrimSpace(targetHost[:idx])
	}
	return strings.TrimSpace(targetHost)
}

// executeMCPToolFix executes an MCP tool call via the chat service
// The approvalID is passed to mark this execution as pre-approved
func (h *AISettingsHandler) executeMCPToolFix(ctx context.Context, command string, approvalID string) (output string, exitCode int, err error) {
	// Get the chat service
	if h.chatHandler == nil {
		return "", -1, fmt.Errorf("chat handler not available")
	}

	chatSvc := h.chatHandler.GetService(ctx)
	if chatSvc == nil {
		return "", -1, fmt.Errorf("chat service not available")
	}

	// Cast to *chat.Service to access ExecuteMCPTool
	chatService, ok := chatSvc.(*chat.Service)
	if !ok {
		return "", -1, fmt.Errorf("chat service type mismatch")
	}

	// Parse the tool call
	toolName, args, parseErr := h.parseMCPToolCall(command)
	if parseErr != nil {
		return "", -1, fmt.Errorf("failed to parse tool call: %w", parseErr)
	}

	// Add the approval ID to mark this as pre-approved
	// The tool executor will check this and skip the approval flow
	if approvalID != "" {
		args["_approval_id"] = approvalID
	}

	log.Info().
		Str("tool", toolName).
		Str("approvalID", approvalID).
		Interface("args", args).
		Msg("Executing MCP tool fix with pre-approval")

	// Execute the tool
	result, toolErr := chatService.ExecuteMCPTool(ctx, toolName, args)
	if toolErr != nil {
		return result, 1, toolErr // Return partial result if any
	}

	return result, 0, nil
}

// parseMCPToolCall parses an MCP tool call string into tool name and arguments
// Handles formats like:
// - pulse_control_guest(action='start', guest_id='102')
// - default_api:pulse_control_guest(guest_id="102", action="start")
func (h *AISettingsHandler) parseMCPToolCall(command string) (string, map[string]interface{}, error) {
	// Remove default_api: prefix if present
	command = strings.TrimPrefix(command, "default_api:")

	// Find the opening parenthesis
	openParen := strings.Index(command, "(")
	if openParen == -1 {
		return "", nil, fmt.Errorf("no opening parenthesis in tool call")
	}

	toolName := strings.TrimSpace(command[:openParen])

	// Find the closing parenthesis
	closeParen := strings.LastIndex(command, ")")
	if closeParen == -1 || closeParen <= openParen {
		return "", nil, fmt.Errorf("no closing parenthesis in tool call")
	}

	argsStr := command[openParen+1 : closeParen]
	args := make(map[string]interface{})

	if strings.TrimSpace(argsStr) == "" {
		return toolName, args, nil
	}

	// Parse key=value pairs (handles both 'value' and "value" formats)
	// This is a simple parser - for complex cases we might need something more robust
	pairs := h.splitToolArgs(argsStr)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Remove quotes from value
		value = strings.Trim(value, "'\"")
		args[key] = value
	}

	return toolName, args, nil
}

// splitToolArgs splits tool arguments respecting quoted strings
func (h *AISettingsHandler) splitToolArgs(argsStr string) []string {
	var result []string
	var current strings.Builder
	var inQuote rune
	var escaped bool

	for _, r := range argsStr {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			current.WriteRune(r)
			continue
		}
		if inQuote != 0 {
			current.WriteRune(r)
			if r == inQuote {
				inQuote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			inQuote = r
			current.WriteRune(r)
			continue
		}
		if r == ',' {
			if s := strings.TrimSpace(current.String()); s != "" {
				result = append(result, s)
			}
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		result = append(result, s)
	}
	return result
}

// findAgentForTarget finds an agent that can execute commands on the target host
func (h *AISettingsHandler) findAgentForTarget(targetHost string) string {
	if h.agentServer == nil {
		return ""
	}

	agents := h.agentServer.GetConnectedAgents()
	if len(agents) == 0 {
		return ""
	}

	// If a specific target host is requested, find that agent
	if targetHost != "" {
		for _, agent := range agents {
			if agent.Hostname == targetHost || agent.AgentID == targetHost {
				return agent.AgentID
			}
		}
	}

	// If only one agent is connected, use it
	if len(agents) == 1 {
		return agents[0].AgentID
	}

	// Multiple agents and no specific target - return empty (caller should handle)
	return ""
}

// updateFindingOutcome updates the investigation outcome on a finding
func (h *AISettingsHandler) updateFindingOutcome(ctx context.Context, orgID, findingID, outcome string) {
	// Get AI service for this org
	svc := h.GetAIService(ctx)
	if svc == nil {
		log.Warn().Str("orgID", orgID).Msg("AI service not available for finding update")
		return
	}

	patrol := svc.GetPatrolService()
	if patrol == nil {
		log.Warn().Str("orgID", orgID).Msg("Patrol service not available for finding update")
		return
	}

	findingsStore := patrol.GetFindings()
	if findingsStore == nil {
		log.Warn().Str("orgID", orgID).Msg("Findings store not available for finding update")
		return
	}

	if !findingsStore.UpdateInvestigationOutcome(findingID, outcome) {
		log.Warn().Str("findingID", findingID).Msg("Finding not found for outcome update")
		return
	}

	log.Info().Str("findingID", findingID).Str("outcome", outcome).Msg("Updated finding investigation outcome")
}

// HandleDenyCommand denies a pending command.
func (h *AISettingsHandler) HandleDenyCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /api/ai/approvals/{id}/deny
	path := strings.TrimPrefix(r.URL.Path, "/api/ai/approvals/")
	path = strings.TrimSuffix(path, "/deny")
	approvalID := strings.TrimSuffix(path, "/")

	if approvalID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Approval ID is required", nil)
		return
	}

	// Parse optional reason from body
	var body struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			log.Warn().Err(err).Msg("Failed to decode deny request body")
		}
	}

	store := approval.GetStore()
	if store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Approval store not initialized", nil)
		return
	}

	username := getAuthUsername(h.getConfig(r.Context()), r)
	if username == "" {
		username = "anonymous"
	}

	req, err := store.Deny(approvalID, username, body.Reason)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "denial_failed", err.Error(), nil)
		return
	}

	// Log audit event
	LogAuditEvent("ai_command_denied", username, GetClientIP(r), r.URL.Path, true,
		fmt.Sprintf("Denied command: %s (reason: %s)", truncateForLog(req.Command, 100), body.Reason))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"denied":  true,
		"request": req,
		"message": "Command denied.",
	})
}

// truncateForLog truncates a string for logging purposes.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// PatrolAutonomySettings represents the patrol autonomy configuration for API requests
// Uses pointer for FullModeUnlocked to distinguish "not sent" from "sent as false"
type PatrolAutonomySettings struct {
	AutonomyLevel           string `json:"autonomy_level"`               // "monitor", "approval", "assisted", "full"
	FullModeUnlocked        *bool  `json:"full_mode_unlocked,omitempty"` // User has acknowledged Full mode risks (nil = preserve existing)
	InvestigationBudget     int    `json:"investigation_budget"`         // Max turns per investigation (5-30)
	InvestigationTimeoutSec int    `json:"investigation_timeout_sec"`    // Max seconds per investigation (60-1800)
}

// PatrolAutonomyResponse represents the patrol autonomy configuration for API responses
// Uses plain bool for FullModeUnlocked since responses always include the actual value
type PatrolAutonomyResponse struct {
	AutonomyLevel           string `json:"autonomy_level"`
	FullModeUnlocked        bool   `json:"full_mode_unlocked"`
	InvestigationBudget     int    `json:"investigation_budget"`
	InvestigationTimeoutSec int    `json:"investigation_timeout_sec"`
}

// HandleGetPatrolAutonomy returns the current patrol autonomy settings (GET /api/ai/patrol/autonomy)
func (h *AISettingsHandler) HandleGetPatrolAutonomy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "service_unavailable", "Pulse Patrol service not available", nil)
		return
	}
	cfg := aiService.GetConfig()
	if cfg == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_configured", "Pulse Patrol not configured", nil)
		return
	}

	autonomyLevel := cfg.GetPatrolAutonomyLevel()
	// Community tier lock: without ai_autofix, patrol autonomy is findings-only ("monitor").
	// If config contains a higher level from a previous Pro/trial period, clamp the effective
	// value at read time so the UI reflects runtime enforcement.
	hasAutoFix := aiService.HasLicenseFeature(license.FeatureAIAutoFix)
	if !hasAutoFix && autonomyLevel != config.PatrolAutonomyMonitor {
		autonomyLevel = config.PatrolAutonomyMonitor
	}

	settings := PatrolAutonomyResponse{
		AutonomyLevel:           autonomyLevel,
		FullModeUnlocked:        cfg.PatrolFullModeUnlocked,
		InvestigationBudget:     cfg.GetPatrolInvestigationBudget(),
		InvestigationTimeoutSec: int(cfg.GetPatrolInvestigationTimeout().Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// HandleUpdatePatrolAutonomy updates the patrol autonomy settings (PUT /api/ai/patrol/autonomy)
func (h *AISettingsHandler) HandleUpdatePatrolAutonomy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "service_unavailable", "Pulse Patrol service not available", nil)
		return
	}

	var req PatrolAutonomySettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	// Validate autonomy level
	if !config.IsValidPatrolAutonomyLevel(req.AutonomyLevel) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_autonomy_level",
			fmt.Sprintf("Invalid autonomy level: %s. Must be 'monitor', 'approval', 'assisted', or 'full'", req.AutonomyLevel), nil)
		return
	}

	// Community tier lock: ANY autonomy above "monitor" implies investigation and requires Pro.
	if !aiService.HasLicenseFeature(license.FeatureAIAutoFix) && req.AutonomyLevel != config.PatrolAutonomyMonitor {
		WriteLicenseRequired(w, license.FeatureAIAutoFix, "Investigation and auto-fix require Pulse Pro. Community tier is limited to Monitor (findings-only) autonomy.")
		return
	}

	// Validate budget (5-30)
	if req.InvestigationBudget < 5 {
		req.InvestigationBudget = 5
	}
	if req.InvestigationBudget > 30 {
		req.InvestigationBudget = 30
	}

	// Validate timeout (60-1800 seconds / 30 minutes)
	if req.InvestigationTimeoutSec < 60 {
		req.InvestigationTimeoutSec = 60
	}
	if req.InvestigationTimeoutSec > 1800 {
		req.InvestigationTimeoutSec = 1800
	}

	cfg := aiService.GetConfig()
	if cfg == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_configured", "Pulse Patrol not configured", nil)
		return
	}

	// Determine effective unlock value: use request value if provided, else preserve existing
	effectiveUnlocked := cfg.PatrolFullModeUnlocked
	if req.FullModeUnlocked != nil {
		effectiveUnlocked = *req.FullModeUnlocked
	}

	// Handle auto-downgrade FIRST: if turning off unlock while currently in full mode AND
	// request still asks for "full", downgrade to assisted. If user explicitly requested a
	// lower level (monitor, approval, assisted), honor that instead.
	finalAutonomyLevel := req.AutonomyLevel
	currentLevel := cfg.GetPatrolAutonomyLevel() // Use getter to handle legacy "autonomous" migration
	if !effectiveUnlocked && currentLevel == config.PatrolAutonomyFull && req.AutonomyLevel == config.PatrolAutonomyFull {
		finalAutonomyLevel = config.PatrolAutonomyAssisted
	}

	// Validate the FINAL level: can't use "full" mode unless unlocked
	if finalAutonomyLevel == config.PatrolAutonomyFull && !effectiveUnlocked {
		writeErrorResponse(w, http.StatusForbidden, "full_mode_locked",
			"Auto-fixing critical issues requires acknowledgment. Enable 'Auto-fix critical issues' in settings first.", nil)
		return
	}

	// Now safe to update config (all validation passed)
	cfg.PatrolFullModeUnlocked = effectiveUnlocked
	cfg.PatrolAutonomyLevel = finalAutonomyLevel
	cfg.PatrolInvestigationBudget = req.InvestigationBudget
	cfg.PatrolInvestigationTimeoutSec = req.InvestigationTimeoutSec

	// Save config via persistence layer
	if err := h.getPersistence(r.Context()).SaveAIConfig(*cfg); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "save_failed", "Failed to save Pulse Assistant config", nil)
		return
	}

	// Reload config to update in-memory state
	if err := aiService.LoadConfig(); err != nil {
		// Log but don't fail - config was saved successfully
		LogAuditEvent("patrol_autonomy_reload_warning", "", "", r.URL.Path, false,
			fmt.Sprintf("Config saved but failed to reload: %v", err))
	}

	// Log audit event with actual saved values
	username := getAuthUsername(h.getConfig(r.Context()), r)
	LogAuditEvent("patrol_autonomy_updated", username, GetClientIP(r), r.URL.Path, true,
		fmt.Sprintf("Updated patrol autonomy: level=%s, unlocked=%v, budget=%d, timeout=%ds",
			finalAutonomyLevel, effectiveUnlocked, req.InvestigationBudget, req.InvestigationTimeoutSec))

	// Return actual saved values (may differ from request due to auto-downgrade)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"settings": map[string]interface{}{
			"autonomy_level":            finalAutonomyLevel,
			"full_mode_unlocked":        effectiveUnlocked,
			"investigation_budget":      req.InvestigationBudget,
			"investigation_timeout_sec": req.InvestigationTimeoutSec,
		},
	})
}

// HandleGetInvestigation returns investigation details for a finding (GET /api/ai/findings/{id}/investigation)
func (h *AISettingsHandler) HandleGetInvestigation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract finding ID from path
	findingID := strings.TrimPrefix(r.URL.Path, "/api/ai/findings/")
	findingID = strings.TrimSuffix(findingID, "/investigation")
	if findingID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Finding ID is required", nil)
		return
	}

	aiService := h.GetAIService(r.Context())
	patrol := aiService.GetPatrolService()
	if patrol == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Patrol service not initialized", nil)
		return
	}

	// Get investigation from orchestrator
	orchestrator := patrol.GetInvestigationOrchestrator()
	if orchestrator == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Investigation orchestrator not initialized", nil)
		return
	}

	investigation := orchestrator.GetInvestigationByFinding(findingID)
	if investigation == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "No investigation found for this finding", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(investigation)
}

// HandleReapproveInvestigationFix creates a new approval from an investigation's proposed fix (POST /api/ai/findings/{id}/reapprove)
// This is useful when the original approval has expired but the user still wants to execute the fix.
func (h *AISettingsHandler) HandleReapproveInvestigationFix(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check license
	if !h.GetAIService(r.Context()).HasLicenseFeature(license.FeatureAIAutoFix) {
		WriteLicenseRequired(w, license.FeatureAIAutoFix, "Pulse Patrol Auto-Fix feature requires Pro license")
		return
	}

	// Extract finding ID from path
	findingID := strings.TrimPrefix(r.URL.Path, "/api/ai/findings/")
	findingID = strings.TrimSuffix(findingID, "/reapprove")
	if findingID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Finding ID is required", nil)
		return
	}

	aiService := h.GetAIService(r.Context())
	patrol := aiService.GetPatrolService()
	if patrol == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Patrol service not initialized", nil)
		return
	}

	// Get investigation from orchestrator
	orchestrator := patrol.GetInvestigationOrchestrator()
	if orchestrator == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Investigation orchestrator not initialized", nil)
		return
	}

	inv := orchestrator.GetInvestigationByFinding(findingID)
	if inv == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "No investigation found for this finding", nil)
		return
	}

	// Check if investigation has a proposed fix
	if inv.ProposedFix == nil || len(inv.ProposedFix.Commands) == 0 {
		writeErrorResponse(w, http.StatusBadRequest, "no_fix", "Investigation has no proposed fix", nil)
		return
	}

	// Check approval store
	store := approval.GetStore()
	if store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Approval store not initialized", nil)
		return
	}

	// Create new approval request
	targetName := h.cleanTargetHost(inv.ProposedFix.TargetHost)
	if targetName == "" {
		// Legacy fallback for fixes created before target_host was populated.
		targetName = inv.ProposedFix.Description
	}
	req := &approval.ApprovalRequest{
		ToolID:     "investigation_fix",
		Command:    inv.ProposedFix.Commands[0],
		TargetType: "investigation",
		TargetID:   findingID,
		TargetName: targetName,
		Context:    fmt.Sprintf("Re-approval of fix from investigation: %s", inv.ProposedFix.Description),
		RiskLevel:  approval.AssessRiskLevel(inv.ProposedFix.Commands[0], "investigation"),
	}

	if err := store.CreateApproval(req); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "create_failed", "Failed to create approval: "+err.Error(), nil)
		return
	}

	log.Info().
		Str("finding_id", findingID).
		Str("approval_id", req.ID).
		Str("command", truncateForLog(req.Command, 100)).
		Msg("Re-created approval for investigation fix")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"approval_id": req.ID,
		"message":     "Approval created. You can now approve and execute the fix.",
	})
}

// HandleGetInvestigationMessages returns chat messages for an investigation (GET /api/ai/findings/{id}/investigation/messages)
func (h *AISettingsHandler) HandleGetInvestigationMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract finding ID from path
	findingID := strings.TrimPrefix(r.URL.Path, "/api/ai/findings/")
	findingID = strings.TrimSuffix(findingID, "/investigation/messages")
	if findingID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Finding ID is required", nil)
		return
	}

	aiService := h.GetAIService(r.Context())
	patrol := aiService.GetPatrolService()
	if patrol == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Patrol service not initialized", nil)
		return
	}

	// Get investigation from orchestrator
	orchestrator := patrol.GetInvestigationOrchestrator()
	if orchestrator == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Investigation orchestrator not initialized", nil)
		return
	}

	investigation := orchestrator.GetInvestigationByFinding(findingID)
	if investigation == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "No investigation found for this finding", nil)
		return
	}

	// Get chat messages for the investigation session
	chatService := aiService.GetChatService()
	if chatService == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Chat service not initialized", nil)
		return
	}

	messages, err := chatService.GetMessages(r.Context(), investigation.SessionID)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "fetch_failed", "Failed to get investigation messages", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"investigation_id": investigation.ID,
		"session_id":       investigation.SessionID,
		"messages":         messages,
	})
}

// HandleReinvestigateFinding triggers a re-investigation of a finding (POST /api/ai/findings/{id}/reinvestigate)
func (h *AISettingsHandler) HandleReinvestigateFinding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract finding ID from path
	findingID := strings.TrimPrefix(r.URL.Path, "/api/ai/findings/")
	findingID = strings.TrimSuffix(findingID, "/reinvestigate")
	if findingID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Finding ID is required", nil)
		return
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "service_unavailable", "Pulse Patrol service not available", nil)
		return
	}

	// Reinvestigation is investigation and requires Pro (ai_autofix).
	// This is defense-in-depth in addition to the route-level RequireLicenseFeature gate.
	if !aiService.HasLicenseFeature(license.FeatureAIAutoFix) {
		WriteLicenseRequired(w, license.FeatureAIAutoFix, "Reinvestigation requires Pulse Pro (AI Auto-Fix feature).")
		return
	}

	cfg := aiService.GetConfig()
	if cfg == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_configured", "Pulse Patrol not configured", nil)
		return
	}

	autonomyLevel := cfg.GetPatrolAutonomyLevel()
	if autonomyLevel == config.PatrolAutonomyMonitor {
		writeErrorResponse(w, http.StatusBadRequest, "autonomy_disabled",
			"Patrol autonomy is set to 'monitor' mode. Enable 'approval', 'assisted', or 'full' mode to investigate findings.", nil)
		return
	}

	patrol := aiService.GetPatrolService()
	if patrol == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Patrol service not initialized", nil)
		return
	}

	orchestrator := patrol.GetInvestigationOrchestrator()
	if orchestrator == nil {
		// Try lazy initialization if chat handler is available
		if h.chatHandler != nil {
			log.Debug().Msg("Attempting lazy orchestrator initialization for reinvestigation")
			h.setupInvestigationOrchestrator("default", aiService)
			orchestrator = patrol.GetInvestigationOrchestrator()
		}
		if orchestrator == nil {
			writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Investigation orchestrator not initialized", nil)
			return
		}
	}

	// Check for already-running investigation (return 409 Conflict)
	orgID := GetOrgID(r.Context())
	h.investigationMu.RLock()
	invStore := h.investigationStores[orgID]
	h.investigationMu.RUnlock()
	if invStore != nil {
		if latest := invStore.GetLatestByFinding(findingID); latest != nil && latest.Status == investigation.StatusRunning {
			writeErrorResponse(w, http.StatusConflict, "investigation_running",
				"An investigation is already running for this finding", nil)
			return
		}
	}

	// Trigger re-investigation in background
	go func() {
		ctx := context.Background()
		if err := orchestrator.ReinvestigateFinding(ctx, findingID, autonomyLevel); err != nil {
			log.Error().Err(err).Str("finding_id", findingID).Msg("Re-investigation failed")
		}
	}()

	// Log audit event
	username := getAuthUsername(h.getConfig(r.Context()), r)
	LogAuditEvent("finding_reinvestigation", username, GetClientIP(r), r.URL.Path, true,
		fmt.Sprintf("Triggered re-investigation for finding: %s", findingID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"finding_id": findingID,
		"message":    "Re-investigation started",
	})
}
