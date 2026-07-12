package api

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/learning"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/metrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/mockmode"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
	"github.com/rs/zerolog/log"
)

// AISettingsHandler handles AI settings endpoints
type AISettingsHandler struct {
	stateMu                 sync.RWMutex
	mtPersistence           *config.MultiTenantPersistence
	mtMonitor               *monitoring.MultiTenantMonitor
	defaultConfig           *config.Config
	defaultPersistence      *config.ConfigPersistence
	hostedMode              bool
	defaultAIService        *ai.Service
	aiServices              map[string]*ai.Service
	aiServicesMu            sync.RWMutex
	agentServer             *agentexec.Server
	onModelChange           func() // Called when model or other AI chat-affecting settings change
	onControlSettingsChange func() // Called when control level or protected guests change

	// Providers to be applied to new services
	stateProvider           ai.StateProvider
	readState               unifiedresources.ReadState
	unifiedResourceProvider ai.UnifiedResourceProvider
	resourceStoreProvider   func(orgID string) (unifiedresources.ResourceStore, error)
	actionBrokerFactory     func(orgID string) aicontracts.OrchestratorActionBroker
	proposalCatalogFactory  func(orgID string) tools.ProposalCatalog
	policyMutation          func(func() error) error
	patrolAutopilotPolicy   func() unifiedresources.PatrolAutopilotServerPolicy
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
	circuitBreaker    *circuit.Breaker              // Circuit breaker for resilient patrol
	learningStore     *learning.LearningStore       // Feedback learning
	forecastService   *forecast.Service             // Trend forecasting
	proxmoxCorrelator *proxmox.EventCorrelator      // Proxmox event correlation
	remediationEngine aicontracts.RemediationEngine // AI-guided remediation
	unifiedStore      *unified.UnifiedStore         // Unified alert/finding store
	alertBridge       *unified.AlertBridge          // Bridge between alerts and unified store

	// Event-driven patrol (Phase 7)
	triggerManager       *ai.TriggerManager        // Event-driven patrol trigger manager
	incidentCoordinator  *ai.IncidentCoordinator   // Incident recording coordinator
	incidentRecorder     *metrics.IncidentRecorder // High-frequency incident recorder
	intelligenceMu       sync.RWMutex
	proxmoxCorrelators   map[string]*proxmox.EventCorrelator
	learningStores       map[string]*learning.LearningStore
	forecastServices     map[string]*forecast.Service
	remediationEngines   map[string]aicontracts.RemediationEngine
	incidentStores       map[string]*memory.IncidentStore
	circuitBreakers      map[string]*circuit.Breaker
	discoveryStores      map[string]*servicediscovery.Store
	unifiedStores        map[string]*unified.UnifiedStore
	alertBridges         map[string]*unified.AlertBridge
	triggerManagers      map[string]*ai.TriggerManager
	incidentCoordinators map[string]*ai.IncidentCoordinator
	incidentRecorders    map[string]*metrics.IncidentRecorder

	// Investigation orchestration (Patrol Autonomy)
	chatHandler         *AIHandler                                // Chat service handler for investigations
	investigationStores map[string]aicontracts.InvestigationStore // Investigation stores per org
	investigationMu     sync.RWMutex

	// Extension endpoints for enterprise feature gating
	aiAutoFixEndpoints extensions.AIAutoFixEndpoints

	// Discovery store for deep infrastructure discovery
	discoveryStore *servicediscovery.Store
}

// SetAIAutoFixEndpoints sets the resolved AI auto-fix extension endpoints.
// Called during route registration so the approval handler can delegate investigation fix approvals.
func (h *AISettingsHandler) SetAIAutoFixEndpoints(ep extensions.AIAutoFixEndpoints) {
	h.aiAutoFixEndpoints = ep
}

// SetResourceStoreProvider installs the resource-store resolver used by
// read-side Patrol response enrichment. The findings store owns durable
// suppression/stamping during Add; this provider lets the HTTP response
// reflect operator priority changes immediately, without waiting for the
// next Patrol detection pass.
func (h *AISettingsHandler) SetResourceStoreProvider(provider func(orgID string) (unifiedresources.ResourceStore, error)) {
	if h == nil {
		return
	}
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.resourceStoreProvider = provider
}

func (h *AISettingsHandler) SetPolicyMutationCoordinator(coordinator func(func() error) error) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.policyMutation = coordinator
}

func (h *AISettingsHandler) SetPatrolAutopilotServerPolicyProvider(provider func() unifiedresources.PatrolAutopilotServerPolicy) {
	h.stateMu.Lock()
	h.patrolAutopilotPolicy = provider
	defaultService := h.defaultAIService
	h.stateMu.Unlock()
	if defaultService != nil {
		defaultService.SetPatrolAutopilotServerPolicyProvider(provider)
	}
	h.aiServicesMu.RLock()
	defer h.aiServicesMu.RUnlock()
	for _, service := range h.aiServices {
		service.SetPatrolAutopilotServerPolicyProvider(provider)
	}
}

func (h *AISettingsHandler) currentPatrolAutopilotServerPolicy() unifiedresources.PatrolAutopilotServerPolicy {
	h.stateMu.RLock()
	provider := h.patrolAutopilotPolicy
	h.stateMu.RUnlock()
	if provider != nil {
		return provider()
	}
	return unifiedresources.CurrentPatrolAutopilotServerPolicy(time.Now().UTC())
}

// SetActionBrokerFactory installs the per-org typed action proposal broker
// used by the investigation orchestrator. Core-owned: it binds the tenant
// and the fixed Patrol actor, so enterprise code can only propose typed
// capabilities, never dispatch commands or claim authority.
func (h *AISettingsHandler) SetActionBrokerFactory(factory func(orgID string) aicontracts.OrchestratorActionBroker) {
	if h == nil {
		return
	}
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.actionBrokerFactory = factory
}

// SetProposalCatalogFactory installs the per-org capability catalog used
// for proposal validation, resolved from the tenant-bound action
// lifecycle service so acceptance and planning share one contract.
func (h *AISettingsHandler) SetProposalCatalogFactory(factory func(orgID string) tools.ProposalCatalog) {
	if h == nil {
		return
	}
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.proposalCatalogFactory = factory
}

func (h *AISettingsHandler) actionBrokerFor(orgID string) aicontracts.OrchestratorActionBroker {
	if h == nil {
		return nil
	}
	h.stateMu.RLock()
	factory := h.actionBrokerFactory
	h.stateMu.RUnlock()
	if factory == nil {
		return nil
	}
	return factory(orgID)
}

func (h *AISettingsHandler) proposalCatalogFor(orgID string) tools.ProposalCatalog {
	if h == nil {
		return nil
	}
	h.stateMu.RLock()
	factory := h.proposalCatalogFactory
	h.stateMu.RUnlock()
	if factory == nil {
		return nil
	}
	return factory(orgID)
}

func (h *AISettingsHandler) stateRefs() (
	*config.MultiTenantPersistence,
	*monitoring.MultiTenantMonitor,
	*config.Config,
	*config.ConfigPersistence,
	unifiedresources.ReadState,
	ai.UnifiedResourceProvider,
) {
	h.stateMu.RLock()
	defer h.stateMu.RUnlock()
	return h.mtPersistence, h.mtMonitor, h.defaultConfig, h.defaultPersistence, h.readState, h.unifiedResourceProvider
}

type aiSettingsProviderSnapshot struct {
	defaultAIService        *ai.Service
	stateProvider           ai.StateProvider
	metadataProvider        ai.MetadataProvider
	patrolThresholdProvider ai.ThresholdProvider
	metricsHistoryProvider  ai.MetricsHistoryProvider
	baselineStore           *ai.BaselineStore
	changeDetector          *ai.ChangeDetector
	remediationLog          *ai.RemediationLog
	incidentStore           *memory.IncidentStore
	patternDetector         *ai.PatternDetector
	correlationDetector     *ai.CorrelationDetector
	discoveryStore          *servicediscovery.Store
	licenseHandlers         *LicenseHandlers
	chatHandler             *AIHandler
}

type failClosedLicenseChecker struct{}

func (failClosedLicenseChecker) HasFeature(string) bool { return false }
func (failClosedLicenseChecker) GetLicenseStateString() (string, bool) {
	return string(ai.LicenseStateNone), false
}

type actionAuditStoreProvider interface {
	GetActionAuditStore() unifiedresources.ResourceStore
}

func (h *AISettingsHandler) providerSnapshot() aiSettingsProviderSnapshot {
	h.stateMu.RLock()
	defer h.stateMu.RUnlock()
	return aiSettingsProviderSnapshot{
		defaultAIService:        h.defaultAIService,
		stateProvider:           h.stateProvider,
		metadataProvider:        h.metadataProvider,
		patrolThresholdProvider: h.patrolThresholdProvider,
		metricsHistoryProvider:  h.metricsHistoryProvider,
		baselineStore:           h.baselineStore,
		changeDetector:          h.changeDetector,
		remediationLog:          h.remediationLog,
		incidentStore:           h.incidentStore,
		patternDetector:         h.patternDetector,
		correlationDetector:     h.correlationDetector,
		discoveryStore:          h.discoveryStore,
		licenseHandlers:         h.licenseHandlers,
		chatHandler:             h.chatHandler,
	}
}

func (h *AISettingsHandler) newFailClosedTenantService(orgID string) *ai.Service {
	svc := ai.NewService(nil, h.agentServer)
	svc.SetOrgID(orgID)
	h.stateMu.RLock()
	patrolAutopilotPolicy := h.patrolAutopilotPolicy
	h.stateMu.RUnlock()
	svc.SetPatrolAutopilotServerPolicyProvider(patrolAutopilotPolicy)
	svc.SetAlertAnalyzerFactory(getCreateAlertAnalyzer())
	svc.SetLicenseChecker(failClosedLicenseChecker{})
	return svc
}

// NewAISettingsHandler creates a new AI settings handler
func NewAISettingsHandler(mtp *config.MultiTenantPersistence, mtm *monitoring.MultiTenantMonitor, agentServer *agentexec.Server) *AISettingsHandler {
	var defaultConfig *config.Config
	var defaultPersistence *config.ConfigPersistence
	var defaultAIService *ai.Service
	hostedMode := hostedModeEnabledFromEnv()

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

	handler := &AISettingsHandler{
		mtPersistence:        mtp,
		mtMonitor:            mtm,
		defaultConfig:        defaultConfig,
		defaultPersistence:   defaultPersistence,
		hostedMode:           hostedMode,
		aiServices:           make(map[string]*ai.Service),
		agentServer:          agentServer,
		proxmoxCorrelators:   make(map[string]*proxmox.EventCorrelator),
		learningStores:       make(map[string]*learning.LearningStore),
		forecastServices:     make(map[string]*forecast.Service),
		remediationEngines:   make(map[string]aicontracts.RemediationEngine),
		incidentStores:       make(map[string]*memory.IncidentStore),
		circuitBreakers:      make(map[string]*circuit.Breaker),
		discoveryStores:      make(map[string]*servicediscovery.Store),
		unifiedStores:        make(map[string]*unified.UnifiedStore),
		alertBridges:         make(map[string]*unified.AlertBridge),
		triggerManagers:      make(map[string]*ai.TriggerManager),
		incidentCoordinators: make(map[string]*ai.IncidentCoordinator),
		incidentRecorders:    make(map[string]*metrics.IncidentRecorder),
	}

	defaultAIService = ai.NewService(defaultPersistence, agentServer)
	defaultAIService.SetOrgID("default")
	defaultAIService.SetAlertAnalyzerFactory(getCreateAlertAnalyzer())
	if defaultPersistence != nil {
		if _, err := handler.loadAIConfigForPersistence(context.Background(), "default", defaultPersistence); err != nil {
			log.Warn().Err(err).Msg("Failed to bootstrap Pulse Assistant config on startup")
		}
		if err := defaultAIService.LoadConfig(); err != nil {
			log.Warn().Err(err).Msg("Failed to load AI config on startup")
		} else if aiSettingsUpdateRequiresPatrolPreflight(nil, defaultAIService.GetConfig()) {
			// Seed the Patrol preflight cache on startup so the UI's
			// "last verified" indicator is populated on first load after
			// a Pulse restart, without forcing operators to save
			// settings or click Verify Patrol just to recover the
			// observability they had before the restart. The dispatch
			// is async with its own timeout, so it cannot block boot.
			log.Info().Msg("Auto-seeding Patrol preflight cache for startup observability")
			defaultAIService.TriggerPatrolPreflightAsync("", "")
		}
	}
	handler.defaultAIService = defaultAIService

	return handler
}

func (h *AISettingsHandler) loadAIConfigForPersistence(_ context.Context, orgID string, persistence *config.ConfigPersistence) (*config.AIConfig, error) {
	if persistence == nil {
		return nil, fmt.Errorf("Pulse Assistant config persistence unavailable")
	}
	billingBaseDir := persistence.DataDir()
	if h != nil && h.mtPersistence != nil {
		billingBaseDir = h.mtPersistence.BaseDataDir()
	}
	return loadHostedAwareAIConfig(h != nil && h.hostedMode, billingBaseDir, orgID, persistence)
}

func (h *AISettingsHandler) loadAIConfig(ctx context.Context) (*config.AIConfig, error) {
	persistence := h.getPersistence(ctx)
	if persistence == nil {
		return nil, fmt.Errorf("Pulse Assistant config persistence unavailable")
	}
	orgID := strings.TrimSpace(GetOrgID(ctx))
	if orgID == "" {
		orgID = "default"
	}
	return h.loadAIConfigForPersistence(ctx, orgID, persistence)
}

// GetAIService returns the underlying AI service
func (h *AISettingsHandler) GetAIService(ctx context.Context) *ai.Service {
	mtPersistence, mtMonitor, _, _, _, _ := h.stateRefs()
	providers := h.providerSnapshot()
	defaultAIService := providers.defaultAIService

	orgID := GetOrgID(ctx)
	if orgID == "default" || orgID == "" {
		return defaultAIService
	}
	if mtPersistence == nil {
		if mtMonitor != nil {
			log.Warn().Str("orgID", orgID).Msg("Failed to get persistence manager for tenant AI service")
			return h.newFailClosedTenantService(orgID)
		}
		return defaultAIService
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
	persistence, err := mtPersistence.GetPersistence(orgID)
	if err != nil {
		log.Warn().Str("orgID", orgID).Err(err).Msg("Failed to get persistence for AI service")
		return h.newFailClosedTenantService(orgID)
	}
	if persistence == nil {
		log.Warn().Str("orgID", orgID).Msg("Tenant persistence unavailable for AI service")
		return h.newFailClosedTenantService(orgID)
	}

	svc = ai.NewService(persistence, h.agentServer)
	svc.SetOrgID(orgID)
	h.stateMu.RLock()
	patrolAutopilotPolicy := h.patrolAutopilotPolicy
	h.stateMu.RUnlock()
	svc.SetPatrolAutopilotServerPolicyProvider(patrolAutopilotPolicy)
	svc.SetAlertAnalyzerFactory(getCreateAlertAnalyzer())
	if _, err := h.loadAIConfigForPersistence(context.Background(), orgID, persistence); err != nil {
		log.Warn().Str("orgID", orgID).Err(err).Msg("Failed to bootstrap Pulse Assistant config for tenant")
	}
	if err := svc.LoadConfig(); err != nil {
		log.Warn().Str("orgID", orgID).Err(err).Msg("Failed to load AI config for tenant")
	}

	// Set providers on new service
	svc.SetStateProvider(h.stateProviderForOrg(orgID, providers.stateProvider))
	if readState := h.readStateForOrg(orgID); readState != nil {
		svc.SetReadState(readState)
	}
	if provider := h.unifiedResourceProviderForOrg(orgID); provider != nil {
		svc.SetUnifiedResourceProvider(provider)
	}
	if providers.metadataProvider != nil {
		svc.SetMetadataProvider(providers.metadataProvider)
	}
	if orgID == "default" {
		if providers.patrolThresholdProvider != nil {
			svc.SetPatrolThresholdProvider(providers.patrolThresholdProvider)
		}
		if providers.metricsHistoryProvider != nil {
			svc.SetMetricsHistoryProvider(providers.metricsHistoryProvider)
		}
		if providers.baselineStore != nil {
			svc.SetBaselineStore(providers.baselineStore)
		}
		if providers.changeDetector != nil {
			svc.SetChangeDetector(providers.changeDetector)
		}
		if providers.remediationLog != nil {
			svc.SetRemediationLog(providers.remediationLog)
		}
	}
	if incidentStore := h.GetIncidentStoreForOrg(orgID); incidentStore != nil {
		svc.SetIncidentStore(incidentStore)
	} else if orgID == "default" && providers.incidentStore != nil {
		svc.SetIncidentStore(providers.incidentStore)
	}
	if orgID == "default" {
		if providers.patternDetector != nil {
			svc.SetPatternDetector(providers.patternDetector)
		}
		if providers.correlationDetector != nil {
			svc.SetCorrelationDetector(providers.correlationDetector)
		}
	}
	if discoveryStore := h.GetDiscoveryStoreForOrg(orgID); discoveryStore != nil {
		svc.SetDiscoveryStore(discoveryStore)
	} else if orgID == "default" && providers.discoveryStore != nil {
		svc.SetDiscoveryStore(providers.discoveryStore)
	}

	// Set license checker if handler available
	if providers.licenseHandlers != nil {
		// Used context to resolve tenant license service
		if licSvc, _, err := providers.licenseHandlers.getTenantComponents(ctx); err == nil {
			svc.SetLicenseChecker(licSvc)
		}
	}

	// Set up investigation orchestrator if chat handler is available
	if providers.chatHandler != nil && isAIInvestigationEnabled() {
		h.setupInvestigationOrchestrator(orgID, svc)
	}

	h.aiServices[orgID] = svc
	return svc
}

// RemoveTenantService removes the AI settings service for a specific tenant.
func (h *AISettingsHandler) RemoveTenantService(orgID string) {
	orgID = strings.TrimSpace(orgID)
	if orgID == "default" || orgID == "" {
		return
	}

	h.aiServicesMu.Lock()
	svc := h.aiServices[orgID]
	delete(h.aiServices, orgID)
	h.aiServicesMu.Unlock()
	if svc != nil {
		svc.StopPatrol()
	}

	h.RemoveTenantIntelligence(orgID)

	h.investigationMu.Lock()
	delete(h.investigationStores, orgID)
	h.investigationMu.Unlock()

	log.Debug().Str("orgID", orgID).Msg("Removed AI settings service for tenant")
}

// getConfig returns the config for the current context
func (h *AISettingsHandler) getConfig(ctx context.Context) *config.Config {
	_, mtMonitor, defaultConfig, _, _, _ := h.stateRefs()
	orgID := strings.TrimSpace(GetOrgID(ctx))
	if orgID == "" || orgID == "default" {
		return defaultConfig
	}
	if mtMonitor != nil {
		if m, err := mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			return m.GetConfig()
		}
		// Security: never fall back to default config for non-default orgs.
		return nil
	}
	return defaultConfig
}

// GetPersistence returns the persistence for the current context
func (h *AISettingsHandler) getPersistence(ctx context.Context) *config.ConfigPersistence {
	mtPersistence, _, _, defaultPersistence, _, _ := h.stateRefs()
	orgID := strings.TrimSpace(GetOrgID(ctx))
	if orgID == "" || orgID == "default" {
		return defaultPersistence
	}
	if mtPersistence != nil {
		if p, err := mtPersistence.GetPersistence(orgID); err == nil && p != nil {
			return p
		}
		// Security: never fall back to default persistence for non-default orgs.
		return nil
	}
	return defaultPersistence
}

// SetMultiTenantPersistence updates the persistence manager
func (h *AISettingsHandler) SetMultiTenantPersistence(mtp *config.MultiTenantPersistence) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.mtPersistence = mtp
}

// SetMultiTenantMonitor updates the monitor manager
func (h *AISettingsHandler) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.mtMonitor = mtm
}

// SetConfig updates the configuration reference used by the handler.
func (h *AISettingsHandler) SetConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.defaultConfig = cfg
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
	h.stateMu.Lock()
	h.stateProvider = sp
	defaultAIService := h.defaultAIService
	h.stateMu.Unlock()
	if defaultAIService != nil {
		defaultAIService.SetStateProvider(sp)
	}

	h.aiServicesMu.Lock()
	for orgID, svc := range h.aiServices {
		svc.SetStateProvider(h.stateProviderForOrg(orgID, sp))
	}
	h.aiServicesMu.Unlock()

	// Now that state provider is set, patrol service should be available.
	// Try to set up the investigation orchestrator if chat handler is ready.
	// Note: This usually fails because chat service isn't started yet.
	// The orchestrator will be wired via WireOrchestratorAfterChatStart() instead.
	if defaultAIService != nil && isAIInvestigationEnabled() {
		h.setupInvestigationOrchestrator("default", defaultAIService)
		h.aiServicesMu.RLock()
		for orgID, svc := range h.aiServices {
			h.setupInvestigationOrchestrator(orgID, svc)
		}
		h.aiServicesMu.RUnlock()
	}
}

// GetStateProvider returns the state provider for infrastructure context
func (h *AISettingsHandler) GetStateProvider() ai.StateProvider {
	h.stateMu.RLock()
	defer h.stateMu.RUnlock()
	return h.stateProvider
}

func (h *AISettingsHandler) stateProviderForOrg(orgID string, fallback ai.StateProvider) ai.StateProvider {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" || orgID == "default" {
		return fallback
	}
	// Security: never fall back to default-org provider for non-default orgs.
	return nil
}

func (h *AISettingsHandler) readStateForOrg(orgID string) unifiedresources.ReadState {
	if h == nil {
		return nil
	}
	_, mtMonitor, _, _, fallbackReadState, _ := h.stateRefs()
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		orgID = "default"
	}

	if mtMonitor != nil {
		if monitor, err := mtMonitor.GetMonitor(orgID); err == nil && monitor != nil {
			if readState := monitor.GetUnifiedReadState(); readState != nil {
				return readState
			}
		}
		if orgID != "default" {
			// Security: never fall back to default-org read state for non-default orgs.
			return nil
		}
	}

	return fallbackReadState
}

func (h *AISettingsHandler) unifiedResourceProviderForOrg(orgID string) ai.UnifiedResourceProvider {
	if h == nil {
		return nil
	}
	_, mtMonitor, _, _, _, fallbackProvider := h.stateRefs()
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		orgID = "default"
	}

	if mtMonitor != nil {
		if monitor, err := mtMonitor.GetMonitor(orgID); err == nil && monitor != nil {
			if readState := monitor.GetUnifiedReadState(); readState != nil {
				if provider, ok := readState.(ai.UnifiedResourceProvider); ok && provider != nil {
					return provider
				}
			}
		}
		if orgID != "default" {
			// Security: never fall back to default-org unified provider for non-default orgs.
			return nil
		}
	}

	return fallbackProvider
}

// SetReadState injects unified read-state context into AI services (patrol path).
func (h *AISettingsHandler) SetReadState(rs unifiedresources.ReadState) {
	if h == nil {
		return
	}
	h.stateMu.Lock()
	h.readState = rs
	defaultAIService := h.defaultAIService
	h.stateMu.Unlock()

	if defaultAIService != nil {
		defaultAIService.SetReadState(h.readStateForOrg("default"))
	}

	h.aiServicesMu.Lock()
	for orgID, svc := range h.aiServices {
		if svc != nil {
			svc.SetReadState(h.readStateForOrg(orgID))
		}
	}
	h.aiServicesMu.Unlock()
}

// SetUnifiedResourceProvider forwards unified-resource-native context to AI services.
func (h *AISettingsHandler) SetUnifiedResourceProvider(urp ai.UnifiedResourceProvider) {
	if h == nil {
		return
	}
	h.stateMu.Lock()
	h.unifiedResourceProvider = urp
	defaultAIService := h.defaultAIService
	h.stateMu.Unlock()

	if defaultAIService != nil {
		defaultAIService.SetUnifiedResourceProvider(h.unifiedResourceProviderForOrg("default"))
	}

	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()
	for orgID, svc := range h.aiServices {
		if svc != nil {
			svc.SetUnifiedResourceProvider(h.unifiedResourceProviderForOrg(orgID))
		}
	}
}

// SetMetadataProvider sets the metadata provider for AI URL discovery
func (h *AISettingsHandler) SetMetadataProvider(mp ai.MetadataProvider) {
	h.stateMu.Lock()
	h.metadataProvider = mp
	defaultAIService := h.defaultAIService
	h.stateMu.Unlock()
	if defaultAIService != nil {
		defaultAIService.SetMetadataProvider(mp)
	}

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
	h.stateMu.Lock()
	h.patrolThresholdProvider = provider
	defaultAIService := h.defaultAIService
	h.stateMu.Unlock()
	if defaultAIService != nil {
		defaultAIService.SetPatrolThresholdProvider(provider)
	}
}

// SetPatrolFindingsPersistence enables findings persistence for the patrol service
func (h *AISettingsHandler) SetPatrolFindingsPersistence(persistence ai.FindingsPersistence) error {
	var firstErr error
	if patrol := h.defaultAIService.GetPatrolService(); patrol != nil {
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
	if patrol := h.defaultAIService.GetPatrolService(); patrol != nil {
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
	h.stateMu.Lock()
	h.metricsHistoryProvider = provider
	defaultAIService := h.defaultAIService
	h.stateMu.Unlock()
	if defaultAIService != nil {
		defaultAIService.SetMetricsHistoryProvider(provider)
	}
}

// SetBaselineStore sets the baseline store for anomaly detection
func (h *AISettingsHandler) SetBaselineStore(store *ai.BaselineStore) {
	h.stateMu.Lock()
	h.baselineStore = store
	defaultAIService := h.defaultAIService
	h.stateMu.Unlock()
	if defaultAIService != nil {
		defaultAIService.SetBaselineStore(store)
	}
}

// SetChangeDetector sets the change detector for operational memory
func (h *AISettingsHandler) SetChangeDetector(detector *ai.ChangeDetector) {
	h.stateMu.Lock()
	h.changeDetector = detector
	defaultAIService := h.defaultAIService
	h.stateMu.Unlock()
	if defaultAIService != nil {
		defaultAIService.SetChangeDetector(detector)
	}
}

// SetRemediationLog sets the remediation log for tracking fix attempts
func (h *AISettingsHandler) SetRemediationLog(remLog *ai.RemediationLog) {
	h.stateMu.Lock()
	h.remediationLog = remLog
	defaultAIService := h.defaultAIService
	h.stateMu.Unlock()
	if defaultAIService != nil {
		defaultAIService.SetRemediationLog(remLog)
	}
}

// SetIncidentStore sets the incident store for the default org.
func (h *AISettingsHandler) SetIncidentStore(store *memory.IncidentStore) {
	h.SetIncidentStoreForOrg("default", store)
}

// SetIncidentStoreForOrg sets the incident store for alert timelines for an org.
func (h *AISettingsHandler) SetIncidentStoreForOrg(orgID string, store *memory.IncidentStore) {
	if h == nil {
		return
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.Lock()
	h.ensureIntelligenceMapsLocked()
	if store == nil {
		delete(h.incidentStores, orgID)
	} else {
		h.incidentStores[orgID] = store
	}
	h.intelligenceMu.Unlock()
	if orgID == "default" {
		h.stateMu.Lock()
		h.incidentStore = store
		defaultAIService := h.defaultAIService
		h.stateMu.Unlock()
		if defaultAIService != nil {
			defaultAIService.SetIncidentStore(store)
		}
	}

	h.aiServicesMu.RLock()
	svc := h.aiServices[orgID]
	h.aiServicesMu.RUnlock()
	if svc != nil {
		svc.SetIncidentStore(store)
	}
}

// GetIncidentStoreForOrg returns the incident store for an org.
func (h *AISettingsHandler) GetIncidentStoreForOrg(orgID string) *memory.IncidentStore {
	if h == nil {
		return nil
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.RLock()
	if store := h.incidentStores[orgID]; store != nil {
		h.intelligenceMu.RUnlock()
		return store
	}
	h.intelligenceMu.RUnlock()
	if orgID == "default" {
		h.stateMu.RLock()
		store := h.incidentStore
		h.stateMu.RUnlock()
		return store
	}
	return nil
}

// SetPatternDetector sets the pattern detector for failure prediction
func (h *AISettingsHandler) SetPatternDetector(detector *ai.PatternDetector) {
	h.stateMu.Lock()
	h.patternDetector = detector
	defaultAIService := h.defaultAIService
	h.stateMu.Unlock()
	if defaultAIService != nil {
		defaultAIService.SetPatternDetector(detector)
	}
}

// SetCorrelationDetector sets the correlation detector for multi-resource correlation
func (h *AISettingsHandler) SetCorrelationDetector(detector *ai.CorrelationDetector) {
	h.stateMu.Lock()
	h.correlationDetector = detector
	defaultAIService := h.defaultAIService
	h.stateMu.Unlock()
	if defaultAIService != nil {
		defaultAIService.SetCorrelationDetector(detector)
	}
}

// SetCircuitBreaker sets the circuit breaker for the default org.
func (h *AISettingsHandler) SetCircuitBreaker(breaker *circuit.Breaker) {
	h.SetCircuitBreakerForOrg("default", breaker)
}

// SetCircuitBreakerForOrg sets the circuit breaker for an org.
func (h *AISettingsHandler) SetCircuitBreakerForOrg(orgID string, breaker *circuit.Breaker) {
	if h == nil {
		return
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.Lock()
	h.ensureIntelligenceMapsLocked()
	if breaker == nil {
		delete(h.circuitBreakers, orgID)
	} else {
		h.circuitBreakers[orgID] = breaker
	}
	h.intelligenceMu.Unlock()
	if orgID == "default" {
		h.circuitBreaker = breaker
	}
}

// GetCircuitBreaker returns the circuit breaker for the default org.
func (h *AISettingsHandler) GetCircuitBreaker() *circuit.Breaker {
	return h.GetCircuitBreakerForOrg("default")
}

// GetCircuitBreakerForOrg returns the circuit breaker for an org.
func (h *AISettingsHandler) GetCircuitBreakerForOrg(orgID string) *circuit.Breaker {
	if h == nil {
		return nil
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.RLock()
	if breaker := h.circuitBreakers[orgID]; breaker != nil {
		h.intelligenceMu.RUnlock()
		return breaker
	}
	h.intelligenceMu.RUnlock()
	if orgID == "default" {
		return h.circuitBreaker
	}
	return nil
}

// SetLearningStore sets the learning store for the default org.
func (h *AISettingsHandler) SetLearningStore(store *learning.LearningStore) {
	h.SetLearningStoreForOrg("default", store)
}

// SetLearningStoreForOrg sets the learning store for an org.
func (h *AISettingsHandler) SetLearningStoreForOrg(orgID string, store *learning.LearningStore) {
	if h == nil {
		return
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.Lock()
	h.ensureIntelligenceMapsLocked()
	if store == nil {
		delete(h.learningStores, orgID)
	} else {
		h.learningStores[orgID] = store
	}
	h.intelligenceMu.Unlock()
	if orgID == "default" {
		h.learningStore = store
	}
}

// GetLearningStore returns the learning store for the default org.
func (h *AISettingsHandler) GetLearningStore() *learning.LearningStore {
	return h.GetLearningStoreForOrg("default")
}

// GetLearningStoreForOrg returns the learning store for an org.
func (h *AISettingsHandler) GetLearningStoreForOrg(orgID string) *learning.LearningStore {
	if h == nil {
		return nil
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.RLock()
	if store := h.learningStores[orgID]; store != nil {
		h.intelligenceMu.RUnlock()
		return store
	}
	h.intelligenceMu.RUnlock()
	if orgID == "default" {
		return h.learningStore
	}
	return nil
}

// SetForecastService sets the forecast service for the default org.
func (h *AISettingsHandler) SetForecastService(svc *forecast.Service) {
	h.SetForecastServiceForOrg("default", svc)
}

// SetForecastServiceForOrg sets the forecast service for an org.
func (h *AISettingsHandler) SetForecastServiceForOrg(orgID string, svc *forecast.Service) {
	if h == nil {
		return
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.Lock()
	h.ensureIntelligenceMapsLocked()
	if svc == nil {
		delete(h.forecastServices, orgID)
	} else {
		h.forecastServices[orgID] = svc
	}
	h.intelligenceMu.Unlock()
	if orgID == "default" {
		h.forecastService = svc
	}
}

// GetForecastService returns the forecast service for the default org.
func (h *AISettingsHandler) GetForecastService() *forecast.Service {
	return h.GetForecastServiceForOrg("default")
}

// GetForecastServiceForOrg returns the forecast service for an org.
func (h *AISettingsHandler) GetForecastServiceForOrg(orgID string) *forecast.Service {
	if h == nil {
		return nil
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.RLock()
	if svc := h.forecastServices[orgID]; svc != nil {
		h.intelligenceMu.RUnlock()
		return svc
	}
	h.intelligenceMu.RUnlock()
	if orgID == "default" {
		return h.forecastService
	}
	return nil
}

func normalizeAIIntelligenceOrgID(orgID string) string {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return "default"
	}
	return orgID
}

func (h *AISettingsHandler) ensureIntelligenceMapsLocked() {
	if h.proxmoxCorrelators == nil {
		h.proxmoxCorrelators = make(map[string]*proxmox.EventCorrelator)
	}
	if h.learningStores == nil {
		h.learningStores = make(map[string]*learning.LearningStore)
	}
	if h.forecastServices == nil {
		h.forecastServices = make(map[string]*forecast.Service)
	}
	if h.remediationEngines == nil {
		h.remediationEngines = make(map[string]aicontracts.RemediationEngine)
	}
	if h.incidentStores == nil {
		h.incidentStores = make(map[string]*memory.IncidentStore)
	}
	if h.circuitBreakers == nil {
		h.circuitBreakers = make(map[string]*circuit.Breaker)
	}
	if h.discoveryStores == nil {
		h.discoveryStores = make(map[string]*servicediscovery.Store)
	}
	if h.alertBridges == nil {
		h.alertBridges = make(map[string]*unified.AlertBridge)
	}
	if h.unifiedStores == nil {
		h.unifiedStores = make(map[string]*unified.UnifiedStore)
	}
	if h.triggerManagers == nil {
		h.triggerManagers = make(map[string]*ai.TriggerManager)
	}
	if h.incidentCoordinators == nil {
		h.incidentCoordinators = make(map[string]*ai.IncidentCoordinator)
	}
	if h.incidentRecorders == nil {
		h.incidentRecorders = make(map[string]*metrics.IncidentRecorder)
	}
}

// SetProxmoxCorrelatorForOrg sets the Proxmox event correlator for an org.
func (h *AISettingsHandler) SetProxmoxCorrelatorForOrg(orgID string, correlator *proxmox.EventCorrelator) {
	if h == nil {
		return
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.Lock()
	h.ensureIntelligenceMapsLocked()
	if correlator == nil {
		delete(h.proxmoxCorrelators, orgID)
	} else {
		h.proxmoxCorrelators[orgID] = correlator
	}
	h.intelligenceMu.Unlock()
	if orgID == "default" {
		h.proxmoxCorrelator = correlator
	}
}

// SetProxmoxCorrelator sets the Proxmox event correlator
func (h *AISettingsHandler) SetProxmoxCorrelator(correlator *proxmox.EventCorrelator) {
	h.SetProxmoxCorrelatorForOrg("default", correlator)
}

// GetProxmoxCorrelatorForOrg returns the Proxmox event correlator for an org.
func (h *AISettingsHandler) GetProxmoxCorrelatorForOrg(orgID string) *proxmox.EventCorrelator {
	if h == nil {
		return nil
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.RLock()
	if correlator := h.proxmoxCorrelators[orgID]; correlator != nil {
		h.intelligenceMu.RUnlock()
		return correlator
	}
	h.intelligenceMu.RUnlock()
	if orgID == "default" {
		return h.proxmoxCorrelator
	}
	return nil
}

// GetProxmoxCorrelator returns the Proxmox event correlator
func (h *AISettingsHandler) GetProxmoxCorrelator() *proxmox.EventCorrelator {
	return h.GetProxmoxCorrelatorForOrg("default")
}

// SetRemediationEngine sets the remediation engine for the default org.
func (h *AISettingsHandler) SetRemediationEngine(engine aicontracts.RemediationEngine) {
	h.SetRemediationEngineForOrg("default", engine)
}

// SetRemediationEngineForOrg sets the remediation engine for an org.
func (h *AISettingsHandler) SetRemediationEngineForOrg(orgID string, engine aicontracts.RemediationEngine) {
	if h == nil {
		return
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.Lock()
	h.ensureIntelligenceMapsLocked()
	if engine == nil {
		delete(h.remediationEngines, orgID)
	} else {
		h.remediationEngines[orgID] = engine
	}
	h.intelligenceMu.Unlock()
	if orgID == "default" {
		h.remediationEngine = engine
	}
}

// GetRemediationEngine returns the remediation engine for the default org.
func (h *AISettingsHandler) GetRemediationEngine() aicontracts.RemediationEngine {
	return h.GetRemediationEngineForOrg("default")
}

// GetRemediationEngineForOrg returns the remediation engine for an org.
func (h *AISettingsHandler) GetRemediationEngineForOrg(orgID string) aicontracts.RemediationEngine {
	if h == nil {
		return nil
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.RLock()
	if engine := h.remediationEngines[orgID]; engine != nil {
		h.intelligenceMu.RUnlock()
		return engine
	}
	h.intelligenceMu.RUnlock()
	if orgID == "default" {
		return h.remediationEngine
	}
	return nil
}

// SetUnifiedStore sets the unified store for the default org.
func (h *AISettingsHandler) SetUnifiedStore(store *unified.UnifiedStore) {
	h.SetUnifiedStoreForOrg("default", store)
}

// SetUnifiedStoreForOrg sets the unified store for an org.
func (h *AISettingsHandler) SetUnifiedStoreForOrg(orgID string, store *unified.UnifiedStore) {
	if h == nil {
		return
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.Lock()
	h.ensureIntelligenceMapsLocked()
	if store == nil {
		delete(h.unifiedStores, orgID)
	} else {
		h.unifiedStores[orgID] = store
	}
	h.intelligenceMu.Unlock()
	if orgID == "default" {
		h.unifiedStore = store
	}
}

// GetUnifiedStore returns the unified store for the default org.
func (h *AISettingsHandler) GetUnifiedStore() *unified.UnifiedStore {
	return h.GetUnifiedStoreForOrg("default")
}

// GetUnifiedStoreForOrg returns the unified store for an org.
func (h *AISettingsHandler) GetUnifiedStoreForOrg(orgID string) *unified.UnifiedStore {
	if h == nil {
		return nil
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.RLock()
	if store := h.unifiedStores[orgID]; store != nil {
		h.intelligenceMu.RUnlock()
		return store
	}
	h.intelligenceMu.RUnlock()
	if orgID == "default" {
		return h.unifiedStore
	}
	return nil
}

// SetDiscoveryStore sets the discovery store for the default org.
func (h *AISettingsHandler) SetDiscoveryStore(store *servicediscovery.Store) {
	h.SetDiscoveryStoreForOrg("default", store)
}

// SetDiscoveryStoreForOrg sets the discovery store for deep infrastructure discovery for an org.
func (h *AISettingsHandler) SetDiscoveryStoreForOrg(orgID string, store *servicediscovery.Store) {
	if h == nil {
		return
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.Lock()
	h.ensureIntelligenceMapsLocked()
	if store == nil {
		delete(h.discoveryStores, orgID)
	} else {
		h.discoveryStores[orgID] = store
	}
	h.intelligenceMu.Unlock()

	if orgID == "default" {
		h.stateMu.Lock()
		h.discoveryStore = store
		defaultAIService := h.defaultAIService
		h.stateMu.Unlock()
		if defaultAIService != nil {
			defaultAIService.SetDiscoveryStore(store)
		}
	}

	h.aiServicesMu.RLock()
	svc := h.aiServices[orgID]
	h.aiServicesMu.RUnlock()
	if svc != nil {
		svc.SetDiscoveryStore(store)
	}
}

// GetDiscoveryStore returns the discovery store for the default org.
func (h *AISettingsHandler) GetDiscoveryStore() *servicediscovery.Store {
	return h.GetDiscoveryStoreForOrg("default")
}

// GetDiscoveryStoreForOrg returns the discovery store for an org.
func (h *AISettingsHandler) GetDiscoveryStoreForOrg(orgID string) *servicediscovery.Store {
	if h == nil {
		return nil
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.RLock()
	if store := h.discoveryStores[orgID]; store != nil {
		h.intelligenceMu.RUnlock()
		return store
	}
	h.intelligenceMu.RUnlock()
	if orgID == "default" {
		h.stateMu.RLock()
		store := h.discoveryStore
		h.stateMu.RUnlock()
		return store
	}
	return nil
}

// SetAlertBridge sets the alert bridge
func (h *AISettingsHandler) SetAlertBridge(bridge *unified.AlertBridge) {
	h.SetAlertBridgeForOrg("default", bridge)
}

// SetAlertBridgeForOrg sets the alert bridge for an org.
func (h *AISettingsHandler) SetAlertBridgeForOrg(orgID string, bridge *unified.AlertBridge) {
	if h == nil {
		return
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.Lock()
	h.ensureIntelligenceMapsLocked()
	if bridge == nil {
		delete(h.alertBridges, orgID)
	} else {
		h.alertBridges[orgID] = bridge
	}
	h.intelligenceMu.Unlock()
	if orgID == "default" {
		h.alertBridge = bridge
	}
}

// GetAlertBridge returns the alert bridge
func (h *AISettingsHandler) GetAlertBridge() *unified.AlertBridge {
	return h.GetAlertBridgeForOrg("default")
}

// GetAlertBridgeForOrg returns the alert bridge for an org.
func (h *AISettingsHandler) GetAlertBridgeForOrg(orgID string) *unified.AlertBridge {
	if h == nil {
		return nil
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.RLock()
	if bridge := h.alertBridges[orgID]; bridge != nil {
		h.intelligenceMu.RUnlock()
		return bridge
	}
	h.intelligenceMu.RUnlock()
	if orgID == "default" {
		return h.alertBridge
	}
	return nil
}

// SetTriggerManager sets the event-driven patrol trigger manager
func (h *AISettingsHandler) SetTriggerManager(tm *ai.TriggerManager) {
	h.SetTriggerManagerForOrg("default", tm)
}

// SetTriggerManagerForOrg sets the event-driven patrol trigger manager for an org.
func (h *AISettingsHandler) SetTriggerManagerForOrg(orgID string, tm *ai.TriggerManager) {
	if h == nil {
		return
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.Lock()
	h.ensureIntelligenceMapsLocked()
	if tm == nil {
		delete(h.triggerManagers, orgID)
	} else {
		h.triggerManagers[orgID] = tm
	}
	h.intelligenceMu.Unlock()
	if orgID == "default" {
		h.triggerManager = tm
	}
}

// GetTriggerManager returns the event-driven patrol trigger manager
func (h *AISettingsHandler) GetTriggerManager() *ai.TriggerManager {
	return h.GetTriggerManagerForOrg("default")
}

// GetTriggerManagerForOrg returns the event-driven patrol trigger manager for an org.
func (h *AISettingsHandler) GetTriggerManagerForOrg(orgID string) *ai.TriggerManager {
	if h == nil {
		return nil
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.RLock()
	if tm := h.triggerManagers[orgID]; tm != nil {
		h.intelligenceMu.RUnlock()
		return tm
	}
	h.intelligenceMu.RUnlock()
	if orgID == "default" {
		return h.triggerManager
	}
	return nil
}

// SetIncidentCoordinator sets the incident recording coordinator
func (h *AISettingsHandler) SetIncidentCoordinator(coordinator *ai.IncidentCoordinator) {
	h.SetIncidentCoordinatorForOrg("default", coordinator)
}

// SetIncidentCoordinatorForOrg sets the incident recording coordinator for an org.
func (h *AISettingsHandler) SetIncidentCoordinatorForOrg(orgID string, coordinator *ai.IncidentCoordinator) {
	if h == nil {
		return
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.Lock()
	h.ensureIntelligenceMapsLocked()
	if coordinator == nil {
		delete(h.incidentCoordinators, orgID)
	} else {
		h.incidentCoordinators[orgID] = coordinator
	}
	h.intelligenceMu.Unlock()
	if orgID == "default" {
		h.incidentCoordinator = coordinator
	}
}

// GetIncidentCoordinator returns the incident recording coordinator
func (h *AISettingsHandler) GetIncidentCoordinator() *ai.IncidentCoordinator {
	return h.GetIncidentCoordinatorForOrg("default")
}

// GetIncidentCoordinatorForOrg returns the incident recording coordinator for an org.
func (h *AISettingsHandler) GetIncidentCoordinatorForOrg(orgID string) *ai.IncidentCoordinator {
	if h == nil {
		return nil
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.RLock()
	if coordinator := h.incidentCoordinators[orgID]; coordinator != nil {
		h.intelligenceMu.RUnlock()
		return coordinator
	}
	h.intelligenceMu.RUnlock()
	if orgID == "default" {
		return h.incidentCoordinator
	}
	return nil
}

// SetIncidentRecorder sets the high-frequency incident recorder
func (h *AISettingsHandler) SetIncidentRecorder(recorder *metrics.IncidentRecorder) {
	h.SetIncidentRecorderForOrg("default", recorder)
}

// SetIncidentRecorderForOrg sets the high-frequency incident recorder for an org.
func (h *AISettingsHandler) SetIncidentRecorderForOrg(orgID string, recorder *metrics.IncidentRecorder) {
	if h == nil {
		return
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.Lock()
	h.ensureIntelligenceMapsLocked()
	if recorder == nil {
		delete(h.incidentRecorders, orgID)
	} else {
		h.incidentRecorders[orgID] = recorder
	}
	h.intelligenceMu.Unlock()
	if orgID == "default" {
		h.incidentRecorder = recorder
	}
}

// GetIncidentRecorder returns the high-frequency incident recorder
func (h *AISettingsHandler) GetIncidentRecorder() *metrics.IncidentRecorder {
	return h.GetIncidentRecorderForOrg("default")
}

// GetIncidentRecorderForOrg returns the high-frequency incident recorder for an org.
func (h *AISettingsHandler) GetIncidentRecorderForOrg(orgID string) *metrics.IncidentRecorder {
	if h == nil {
		return nil
	}
	orgID = normalizeAIIntelligenceOrgID(orgID)
	h.intelligenceMu.RLock()
	if recorder := h.incidentRecorders[orgID]; recorder != nil {
		h.intelligenceMu.RUnlock()
		return recorder
	}
	h.intelligenceMu.RUnlock()
	if orgID == "default" {
		return h.incidentRecorder
	}
	return nil
}

// ListLearningStores returns learning stores keyed by org.
func (h *AISettingsHandler) ListLearningStores() map[string]*learning.LearningStore {
	out := make(map[string]*learning.LearningStore)
	if h == nil {
		return out
	}
	h.intelligenceMu.RLock()
	for orgID, store := range h.learningStores {
		if store != nil {
			out[orgID] = store
		}
	}
	h.intelligenceMu.RUnlock()
	if _, ok := out["default"]; !ok && h.learningStore != nil {
		out["default"] = h.learningStore
	}
	return out
}

// ListAlertBridges returns alert bridges keyed by org for shutdown/cleanup flows.
func (h *AISettingsHandler) ListAlertBridges() map[string]*unified.AlertBridge {
	out := make(map[string]*unified.AlertBridge)
	if h == nil {
		return out
	}
	h.intelligenceMu.RLock()
	for orgID, bridge := range h.alertBridges {
		if bridge != nil {
			out[orgID] = bridge
		}
	}
	h.intelligenceMu.RUnlock()
	if _, ok := out["default"]; !ok && h.alertBridge != nil {
		out["default"] = h.alertBridge
	}
	return out
}

// ListTriggerManagers returns trigger managers keyed by org for shutdown/cleanup flows.
func (h *AISettingsHandler) ListTriggerManagers() map[string]*ai.TriggerManager {
	out := make(map[string]*ai.TriggerManager)
	if h == nil {
		return out
	}
	h.intelligenceMu.RLock()
	for orgID, tm := range h.triggerManagers {
		if tm != nil {
			out[orgID] = tm
		}
	}
	h.intelligenceMu.RUnlock()
	if _, ok := out["default"]; !ok && h.triggerManager != nil {
		out["default"] = h.triggerManager
	}
	return out
}

// ListIncidentCoordinators returns incident coordinators keyed by org.
func (h *AISettingsHandler) ListIncidentCoordinators() map[string]*ai.IncidentCoordinator {
	out := make(map[string]*ai.IncidentCoordinator)
	if h == nil {
		return out
	}
	h.intelligenceMu.RLock()
	for orgID, coordinator := range h.incidentCoordinators {
		if coordinator != nil {
			out[orgID] = coordinator
		}
	}
	h.intelligenceMu.RUnlock()
	if _, ok := out["default"]; !ok && h.incidentCoordinator != nil {
		out["default"] = h.incidentCoordinator
	}
	return out
}

// ListIncidentRecorders returns incident recorders keyed by org.
func (h *AISettingsHandler) ListIncidentRecorders() map[string]*metrics.IncidentRecorder {
	out := make(map[string]*metrics.IncidentRecorder)
	if h == nil {
		return out
	}
	h.intelligenceMu.RLock()
	for orgID, recorder := range h.incidentRecorders {
		if recorder != nil {
			out[orgID] = recorder
		}
	}
	h.intelligenceMu.RUnlock()
	if _, ok := out["default"]; !ok && h.incidentRecorder != nil {
		out["default"] = h.incidentRecorder
	}
	return out
}

// StopPatrol stops the background AI patrol service
func (h *AISettingsHandler) StopPatrol() {
	if h.defaultAIService != nil {
		h.defaultAIService.StopPatrol()
	}
	h.aiServicesMu.RLock()
	services := make([]*ai.Service, 0, len(h.aiServices))
	for _, svc := range h.aiServices {
		if svc != nil {
			services = append(services, svc)
		}
	}
	h.aiServicesMu.RUnlock()
	for _, svc := range services {
		svc.StopPatrol()
	}
}

// StopServices stops all AI services owned by the handler, including their
// debounced persistence timers and background discovery/patrol workers.
func (h *AISettingsHandler) StopServices() {
	if h == nil {
		return
	}
	if h.defaultAIService != nil {
		h.defaultAIService.Stop()
	}
	h.aiServicesMu.RLock()
	services := make([]*ai.Service, 0, len(h.aiServices))
	for _, svc := range h.aiServices {
		if svc != nil {
			services = append(services, svc)
		}
	}
	h.aiServicesMu.RUnlock()
	for _, svc := range services {
		svc.Stop()
	}
}

// StopPatrolForOrg stops patrol for a single org without affecting others.
func (h *AISettingsHandler) StopPatrolForOrg(orgID string) {
	orgID = normalizeAIIntelligenceOrgID(orgID)
	if orgID == "default" {
		if h.defaultAIService != nil {
			h.defaultAIService.StopPatrol()
		}
		return
	}

	h.aiServicesMu.RLock()
	svc := h.aiServices[orgID]
	h.aiServicesMu.RUnlock()
	if svc != nil {
		svc.StopPatrol()
	}
}

// RemoveTenantIntelligence stops and removes org-scoped intelligence components.
func (h *AISettingsHandler) RemoveTenantIntelligence(orgID string) {
	orgID = normalizeAIIntelligenceOrgID(orgID)
	if orgID == "default" {
		return
	}

	var (
		bridge      *unified.AlertBridge
		trigger     *ai.TriggerManager
		coordinator *ai.IncidentCoordinator
		recorder    *metrics.IncidentRecorder
	)

	h.intelligenceMu.Lock()
	delete(h.learningStores, orgID)
	delete(h.forecastServices, orgID)
	delete(h.remediationEngines, orgID)
	delete(h.incidentStores, orgID)
	delete(h.circuitBreakers, orgID)
	delete(h.discoveryStores, orgID)
	bridge = h.alertBridges[orgID]
	trigger = h.triggerManagers[orgID]
	coordinator = h.incidentCoordinators[orgID]
	recorder = h.incidentRecorders[orgID]
	delete(h.unifiedStores, orgID)
	delete(h.alertBridges, orgID)
	delete(h.triggerManagers, orgID)
	delete(h.incidentCoordinators, orgID)
	delete(h.incidentRecorders, orgID)
	delete(h.proxmoxCorrelators, orgID)
	h.intelligenceMu.Unlock()

	if bridge != nil {
		bridge.Stop()
	}
	if trigger != nil {
		trigger.Stop()
	}
	if coordinator != nil {
		coordinator.Stop()
	}
	if recorder != nil {
		recorder.Stop()
	}
}

// GetAlertTriggeredAnalyzer returns the alert-triggered analyzer for wiring into alert callbacks
func (h *AISettingsHandler) GetAlertTriggeredAnalyzer(ctx context.Context) aicontracts.AlertAnalyzer {
	return h.GetAIService(ctx).GetAlertTriggeredAnalyzer()
}

// SetLicenseHandlers sets the license handlers for Pro feature gating
func (h *AISettingsHandler) SetLicenseHandlers(handlers *LicenseHandlers) {
	h.stateMu.Lock()
	h.licenseHandlers = handlers
	defaultAIService := h.defaultAIService
	h.stateMu.Unlock()
	if handlers == nil {
		return
	}
	// Update default-org service with the current license checker.
	// We can try to get it using background context (default tenant).
	if svc, _, err := handlers.getTenantComponents(context.Background()); err == nil {
		if defaultAIService != nil {
			defaultAIService.SetLicenseChecker(svc)
		}
	}
}

// SetOnModelChange sets a callback to be invoked when model settings change
// Used by Router to trigger AI chat service restart
func (h *AISettingsHandler) SetOnModelChange(callback func()) {
	h.onModelChange = callback
}

// aiSettingsUpdateTouchesProviderTransport reports whether the update touches
// the fields shared by the chat-restart and Patrol-readiness predicates:
// enablement, model selection, auth method, provider API keys/base URLs, and
// their clear flags.
func aiSettingsUpdateTouchesProviderTransport(req AISettingsUpdateRequest) bool {
	return req.Enabled != nil ||
		req.Model != nil ||
		req.ChatModel != nil ||
		req.PatrolModel != nil ||
		req.AuthMethod != nil ||
		req.AnthropicAPIKey != nil ||
		req.OpenAIAPIKey != nil ||
		req.OpenRouterAPIKey != nil ||
		req.DeepSeekAPIKey != nil ||
		req.ZaiAPIKey != nil ||
		req.GroqAPIKey != nil ||
		req.MistralAPIKey != nil ||
		req.CerebrasAPIKey != nil ||
		req.TogetherAPIKey != nil ||
		req.FireworksAPIKey != nil ||
		req.GeminiAPIKey != nil ||
		req.OllamaBaseURL != nil ||
		req.OllamaUsername != nil ||
		req.OllamaPassword != nil ||
		req.OllamaKeepAlive != nil ||
		req.OpenAIBaseURL != nil ||
		req.ZaiBaseURL != nil ||
		req.ClearAnthropicKey != nil ||
		req.ClearOpenAIKey != nil ||
		req.ClearOpenRouterKey != nil ||
		req.ClearDeepSeekKey != nil ||
		req.ClearZaiKey != nil ||
		req.ClearGroqKey != nil ||
		req.ClearMistralKey != nil ||
		req.ClearCerebrasKey != nil ||
		req.ClearTogetherKey != nil ||
		req.ClearFireworksKey != nil ||
		req.ClearGeminiKey != nil ||
		req.ClearOllamaURL != nil ||
		req.ClearOllamaUsername != nil ||
		req.ClearOllamaPassword != nil
}

func shouldRestartAIChat(req AISettingsUpdateRequest) bool {
	return req.AutoFixModel != nil || aiSettingsUpdateTouchesProviderTransport(req)
}

// aiSettingsUpdateRequiresPatrolPreflight reports whether the settings
// transition from oldCfg to newCfg warrants an automatic Patrol
// tool-call preflight. We only auto-preflight when the configuration
// that affects Patrol's transport actually moved, so routine settings
// saves (theme, control level, discovery toggles, etc.) don't burn
// provider tokens or add 5-10s of latency to every save.
//
// Triggers preflight when:
//   - assistant is now enabled and a Patrol model is selected, AND
//   - either the Patrol model changed, the new patrol model's provider
//     API key changed, OR there was no prior config (first save).
func aiSettingsUpdateRequiresPatrolPreflight(oldCfg, newCfg *config.AIConfig) bool {
	if newCfg == nil || !newCfg.Enabled {
		return false
	}
	newPatrolModel := strings.TrimSpace(newCfg.GetPatrolModel())
	if newPatrolModel == "" {
		return false
	}
	if oldCfg == nil {
		return true
	}
	if !oldCfg.Enabled {
		return true
	}
	oldPatrolModel := strings.TrimSpace(oldCfg.GetPatrolModel())
	if oldPatrolModel != newPatrolModel {
		return true
	}
	provider, _ := config.ParseModelString(newPatrolModel)
	if provider == "" {
		return false
	}
	return oldCfg.GetAPIKeyForProvider(provider) != newCfg.GetAPIKeyForProvider(provider)
}

func aiSettingsUpdateTouchesPatrolReadiness(req AISettingsUpdateRequest) bool {
	return req.PatrolEnabled != nil ||
		req.PatrolIntervalMinutes != nil ||
		aiSettingsUpdateTouchesProviderTransport(req)
}

// SetOnControlSettingsChange sets a callback to be invoked when control settings change
// Used by Router to update Assistant tool visibility without restarting AI chat.
func (h *AISettingsHandler) SetOnControlSettingsChange(callback func()) {
	h.onControlSettingsChange = callback
}

// EffectiveControlLevel returns the Assistant control level that should be
// exposed or enforced for the current entitlement state. The stored setting can
// remain autonomous so it comes back if the entitlement returns, but runtime
// execution without ai_autofix must stay in approval mode.
func (h *AISettingsHandler) EffectiveControlLevel(ctx context.Context, settings *config.AIConfig) string {
	if settings == nil {
		return config.ControlLevelReadOnly
	}
	return settings.GetEffectiveControlLevel(
		h.GetAIService(ctx).HasLicenseFeature(ai.FeatureAIAutoFix),
	)
}

// SetChatHandler sets the chat handler for investigation orchestration
// This enables the patrol service to spawn chat sessions to investigate findings
func (h *AISettingsHandler) SetChatHandler(chatHandler *AIHandler) {
	h.stateMu.Lock()
	h.chatHandler = chatHandler
	defaultAIService := h.defaultAIService
	h.stateMu.Unlock()
	h.investigationMu.Lock()
	if h.investigationStores == nil {
		h.investigationStores = make(map[string]aicontracts.InvestigationStore)
	}
	h.investigationMu.Unlock()

	// Wire up orchestrator for the default-org service.
	// Note: This usually fails because chat service isn't started yet.
	// The orchestrator will be wired via WireOrchestratorAfterChatStart() instead.
	if defaultAIService != nil && isAIInvestigationEnabled() {
		h.setupInvestigationOrchestrator("default", defaultAIService)
	}

	// Wire up orchestrator for any existing services
	if isAIInvestigationEnabled() {
		h.aiServicesMu.RLock()
		for orgID, svc := range h.aiServices {
			h.setupInvestigationOrchestrator(orgID, svc)
		}
		h.aiServicesMu.RUnlock()
	}
}

// WireOrchestratorAfterChatStart is called after the chat service is started
// to wire up the investigation orchestrator. This must be called after aiHandler.Start()
// because the orchestrator needs an active chat service.
func (h *AISettingsHandler) WireOrchestratorAfterChatStart() {
	if !isAIInvestigationEnabled() {
		log.Info().Msg("WireOrchestratorAfterChatStart skipped (requires Pulse Pro)")
		return
	}

	h.stateMu.RLock()
	hasChatHandler := h.chatHandler != nil
	defaultAIService := h.defaultAIService
	h.stateMu.RUnlock()
	if !hasChatHandler {
		log.Warn().Msg("WireOrchestratorAfterChatStart called but chatHandler is nil")
		return
	}

	// Wire up orchestrator for the default-org service.
	if defaultAIService != nil {
		h.setupInvestigationOrchestrator("default", defaultAIService)
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
	// Check factory exists first — if nil, this is OSS (no orchestrator available).
	// Clear any stale orchestrator from a prior setup so the patrol service
	// doesn't keep using a removed enterprise component.
	factory := getCreateInvestigationOrchestrator()
	if factory == nil {
		if patrol := svc.GetPatrolService(); patrol != nil {
			patrol.SetInvestigationOrchestrator(nil)
		}
		log.Debug().Str("orgID", orgID).Msg("Investigation orchestrator factory not registered (requires Pulse Pro)")
		return
	}

	h.stateMu.RLock()
	chatHandler := h.chatHandler
	h.stateMu.RUnlock()
	if chatHandler == nil {
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
		mtPersistence, _, _, defaultPersistence, _, _ := h.stateRefs()
		if defaultPersistence != nil && orgID == "default" {
			dataDir = defaultPersistence.DataDir()
		} else if mtPersistence != nil {
			if p, err := mtPersistence.GetPersistence(orgID); err == nil {
				dataDir = p.DataDir()
			}
		}
		if storeFactory := getCreateInvestigationStore(); storeFactory != nil {
			store = storeFactory(dataDir)
		}
		if store == nil {
			log.Warn().Str("orgID", orgID).Msg("Investigation store not available (requires Pulse Pro)")
			h.investigationMu.Unlock()
			return
		}
		if err := store.LoadFromDisk(); err != nil {
			log.Warn().Err(err).Str("orgID", orgID).Msg("Failed to load investigation store")
		}
		h.investigationStores[orgID] = store
	}
	h.investigationMu.Unlock()

	// Get chat service for this org using org-scoped context
	ctx := context.WithValue(context.Background(), OrgIDContextKey, orgID)
	chatSvc := chatHandler.GetService(ctx)
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

	// Build local adapters that implement aicontracts.Orchestrator* interfaces
	chatAdapter := &orchestratorChatAdapter{
		svc:     chatService,
		catalog: h.proposalCatalogFor(orgID),
	}

	// Create findings store adapter
	findingsStore := patrol.GetFindings()
	if findingsStore == nil {
		log.Warn().Str("orgID", orgID).Msg("Findings store not available for orchestrator")
		return
	}
	findingsAdapter := &orchestratorFindingsAdapter{store: &findingsStoreWrapper{store: findingsStore}}

	// Get config for investigation settings
	cfg := svc.GetConfig()
	invConfig := aicontracts.DefaultInvestigationConfig()
	if cfg != nil {
		invConfig.MaxTurns = cfg.GetPatrolInvestigationBudget()
		invConfig.Timeout = cfg.GetPatrolInvestigationTimeout()
	}

	// Wire up discovery context to the knowledge store for infrastructure context
	var infraContext aicontracts.OrchestratorInfraContextProvider
	if knowledgeStore := svc.GetKnowledgeStore(); knowledgeStore != nil {
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
		infraContext = knowledgeStore
	}

	// Build deps struct and call factory. The typed action broker is
	// REQUIRED: without it the factory disables the orchestrator - there
	// is no command-execution fallback.
	deps := aicontracts.OrchestratorDeps{
		ChatService:   chatAdapter,
		Store:         store,
		FindingsStore: findingsAdapter,
		ActionBroker:  h.actionBrokerFor(approval.NormalizeOrgID(orgID)),
		Config:        invConfig,
		InfraContext:  infraContext,
		Metrics:       &patrolMetricsCallbackAdapter{},
	}

	orchestrator := factory(deps)
	if orchestrator == nil {
		log.Warn().Str("orgID", orgID).Msg("Investigation orchestrator factory returned nil")
		patrol.SetInvestigationOrchestrator(nil)
		return
	}

	// Set directly on patrol service — factory returns the interface
	patrol.SetInvestigationOrchestrator(orchestrator)

	log.Info().Str("orgID", orgID).Msg("Investigation orchestrator configured for patrol service")
}

// ---------------------------------------------------------------------------
// Local adapters — bridge between OSS singletons and aicontracts interfaces
// ---------------------------------------------------------------------------

// orchestratorChatAdapter wraps *chat.Service to implement
// aicontracts.OrchestratorChatService. It exposes only the
// investigation-specific execution/listing surface: there is no generic
// chat execution, no autonomy control, and no command execution here -
// the typed proposal channel is the only route from an investigation to
// an infrastructure mutation.
type orchestratorChatAdapter struct {
	svc *chat.Service
	// catalog resolves advertised resource capabilities for proposal
	// validation, from the tenant-bound action lifecycle service.
	catalog tools.ProposalCatalog
}

func (a *orchestratorChatAdapter) CreateSession(ctx context.Context) (*aicontracts.OrchestratorChatSession, error) {
	session, err := a.svc.CreateSession(ctx)
	if err != nil {
		return nil, err
	}
	return &aicontracts.OrchestratorChatSession{ID: session.ID}, nil
}

func (a *orchestratorChatAdapter) ExecuteInvestigationStream(ctx context.Context, req aicontracts.OrchestratorInvestigationRequest, callback aicontracts.OrchestratorStreamCallback) (*aicontracts.OrchestratorInvestigationResult, error) {
	if a.svc == nil {
		return nil, fmt.Errorf("chat service is nil")
	}
	if !a.svc.IsRunning() {
		return nil, fmt.Errorf("chat service is not running")
	}
	runResult, err := a.svc.ExecuteInvestigationStream(ctx, chat.InvestigationRunRequest{
		SessionID:    req.SessionID,
		Prompt:       req.Prompt,
		SystemPrompt: req.SystemPrompt,
		MaxTurns:     req.MaxTurns,
		ExecutionID:  req.ExecutionID,
		Identity: tools.ProposalIdentity{
			ProposalID:      req.ProposalID,
			FindingID:       req.FindingID,
			InvestigationID: req.InvestigationID,
			EvidenceIDs:     req.EvidenceIDs,
		},
		Catalog: a.catalog,
	}, func(event chat.StreamEvent) {
		callback(aicontracts.OrchestratorStreamEvent{
			Type: event.Type,
			Data: event.Data,
		})
	})
	if runResult == nil {
		return nil, mapInvestigationProposalError(err)
	}
	result := &aicontracts.OrchestratorInvestigationResult{
		Content:                runResult.Content,
		FailedProposalAttempts: runResult.FailedProposalAttempts,
		InputTokens:            runResult.InputTokens,
		OutputTokens:           runResult.OutputTokens,
	}
	if runResult.Proposal != nil {
		captured := runResult.Proposal
		result.Proposal = &aicontracts.ActionProposal{
			ProposalID:      captured.Identity.ProposalID,
			FindingID:       captured.Identity.FindingID,
			InvestigationID: captured.Identity.InvestigationID,
			ResourceID:      captured.ResourceID,
			CapabilityName:  captured.CapabilityName,
			Params:          captured.Params,
			Reason:          captured.Reason,
			EvidenceIDs:     captured.Identity.EvidenceIDs,
		}
	}
	return result, mapInvestigationProposalError(err)
}

// mapInvestigationProposalError projects the core proposal-channel errors
// onto the public contract sentinels so enterprise outcome mapping can
// key on errors.Is without importing internal packages.
func mapInvestigationProposalError(err error) error {
	var runErr *chat.InvestigationRunError
	if errors.As(err, &runErr) {
		return aicontracts.NewOrchestratorInvestigationError(
			runErr.RunFailure(),
			mapInvestigationProposalSentinel(runErr.ProposalFailure()),
		)
	}
	return mapInvestigationProposalSentinel(err)
}

func mapInvestigationProposalSentinel(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, tools.ErrProposalAmbiguous):
		return aicontracts.ErrInvestigationProposalAmbiguous
	case errors.Is(err, tools.ErrProposalIntegrity):
		return aicontracts.ErrInvestigationProposalIntegrity
	case errors.Is(err, tools.ErrProposalAttemptsFailed):
		return aicontracts.ErrInvestigationProposalAttemptsFailed
	default:
		return err
	}
}

//nolint:dupl // mirrors chatServiceAdapter.GetMessages: same source messages mapped onto a deliberately separate output contract that may diverge
func (a *orchestratorChatAdapter) GetMessages(ctx context.Context, sessionID string) ([]aicontracts.OrchestratorMessage, error) {
	chatMessages, err := a.svc.GetMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	messages := make([]aicontracts.OrchestratorMessage, len(chatMessages))
	for i, msg := range chatMessages {
		m := aicontracts.OrchestratorMessage{
			ID:               msg.ID,
			Role:             msg.Role,
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
			Timestamp:        msg.Timestamp,
		}
		for _, tc := range msg.ToolCalls {
			m.ToolCalls = append(m.ToolCalls, aicontracts.OrchestratorToolCallInfoFromProvider(tc.ProviderToolCall()))
		}
		if msg.ToolResult != nil {
			toolResult := aicontracts.OrchestratorToolResultInfoFromProvider(*msg.ToolResult)
			m.ToolResult = &toolResult
		}
		messages[i] = m.NormalizeCollections()
	}
	return messages, nil
}

func (a *orchestratorChatAdapter) DeleteSession(ctx context.Context, sessionID string) error {
	return a.svc.DeleteSession(ctx, sessionID)
}

func (a *orchestratorChatAdapter) ListInvestigationTools(ctx context.Context) []string {
	_ = ctx
	if a.svc == nil {
		return nil
	}
	return a.svc.ListInvestigationTools()
}

// orchestratorFindingsAdapter wraps findingsStoreWrapper to implement
// aicontracts.OrchestratorFindingsStore.
type orchestratorFindingsAdapter struct {
	store *findingsStoreWrapper
}

func (a *orchestratorFindingsAdapter) Get(id string) *aicontracts.Finding {
	if a.store == nil {
		return nil
	}
	f := a.store.Get(id)
	if f == nil {
		return nil
	}
	return &aicontracts.Finding{
		ID:                     f.GetID(),
		Severity:               f.GetSeverity(),
		Category:               f.GetCategory(),
		ResourceID:             f.GetResourceID(),
		ResourceName:           f.GetResourceName(),
		ResourceType:           f.GetResourceType(),
		Title:                  f.GetTitle(),
		Description:            f.GetDescription(),
		Recommendation:         f.GetRecommendation(),
		Evidence:               f.GetEvidence(),
		InvestigationSessionID: f.GetInvestigationSessionID(),
		InvestigationStatus:    f.GetInvestigationStatus(),
		InvestigationOutcome:   f.GetInvestigationOutcome(),
		LastInvestigatedAt:     f.GetLastInvestigatedAt(),
		InvestigationAttempts:  f.GetInvestigationAttempts(),
	}
}

func (a *orchestratorFindingsAdapter) Update(f *aicontracts.Finding) bool {
	if a.store == nil || f == nil {
		return false
	}
	return a.store.UpdateInvestigation(
		f.ID,
		f.InvestigationSessionID,
		f.InvestigationStatus,
		f.InvestigationOutcome,
		f.LastInvestigatedAt,
		f.InvestigationAttempts,
	)
}

// patrolMetricsCallbackAdapter implements aicontracts.OrchestratorMetricsCallback
// by delegating to the global PatrolMetrics singleton.
type patrolMetricsCallbackAdapter struct{}

func (c *patrolMetricsCallbackAdapter) RecordInvestigationOutcome(outcome string) {
	ai.GetPatrolMetrics().RecordInvestigationOutcome(outcome)
}

func (c *patrolMetricsCallbackAdapter) RecordFixVerification(result string) {
	ai.GetPatrolMetrics().RecordFixVerification(result)
}

// findingsStoreWrapper wraps *ai.FindingsStore to implement aicontracts.OrchestratorAIFindingsStore
type findingsStoreWrapper struct {
	store *ai.FindingsStore
}

func (w *findingsStoreWrapper) Get(id string) aicontracts.OrchestratorAIFinding {
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

// AISettingsResponse is returned by GET /api/settings/ai
// API keys are masked for security
type AISettingsResponse struct {
	Enabled       bool   `json:"enabled"`
	Model         string `json:"model"`
	ChatModel     string `json:"chat_model,omitempty"`     // Model for interactive chat (empty = use default)
	PatrolModel   string `json:"patrol_model,omitempty"`   // Model for patrol (empty = use default)
	AutoFixModel  string `json:"auto_fix_model,omitempty"` // Model for auto-fix (empty = use patrol model)
	Configured    bool   `json:"configured"`               // true if AI is ready to use
	CustomContext string `json:"custom_context"`           // user-provided infrastructure context
	// Legacy OAuth fields are retained for cleanup/migration only.
	AuthMethod     string `json:"auth_method"`     // "api_key" or legacy "oauth"
	OAuthConnected bool   `json:"oauth_connected"` // true if legacy OAuth tokens are stored
	// Patrol settings for token efficiency
	PatrolIntervalMinutes        int  `json:"patrol_interval_minutes"`         // Patrol interval in minutes (0 = disabled)
	PatrolEnabled                bool `json:"patrol_enabled"`                  // true if patrol is enabled
	PatrolAutoFix                bool `json:"patrol_auto_fix"`                 // true if patrol can auto-fix issues
	AlertTriggeredAnalysis       bool `json:"alert_triggered_analysis"`        // true if AI analyzes when alerts fire
	PatrolEventTriggersEnabled   bool `json:"patrol_event_triggers_enabled"`   // Legacy aggregate flag; true when any scoped Patrol trigger source is enabled
	PatrolAlertTriggersEnabled   bool `json:"patrol_alert_triggers_enabled"`   // true if alert-driven scoped Patrol triggers are enabled
	PatrolAnomalyTriggersEnabled bool `json:"patrol_anomaly_triggers_enabled"` // true if anomaly-driven scoped Patrol triggers are enabled
	// Per-rule policy for alert-driven scoped Patrol triggers.
	PatrolAlertTriggerMinSeverity string                `json:"patrol_alert_trigger_min_severity"` // "warning" | "critical"; minimum alert level that warrants investigation
	PatrolAlertTriggerTypes       []string              `json:"patrol_alert_trigger_types"`        // optional allowlist of alert types (empty = all types)
	UseProactiveThresholds        bool                  `json:"use_proactive_thresholds"`          // true if patrol warns before thresholds (false = use exact thresholds)
	AvailableModels               []providers.ModelInfo `json:"available_models"`                  // List of models for current provider
	// Multi-provider credentials - shows which providers are configured
	AnthropicConfigured  bool                           `json:"anthropic_configured"`      // true if Anthropic API key is set
	OpenAIConfigured     bool                           `json:"openai_configured"`         // true if OpenAI API key is set
	OpenRouterConfigured bool                           `json:"openrouter_configured"`     // true if OpenRouter API key is set
	DeepSeekConfigured   bool                           `json:"deepseek_configured"`       // true if DeepSeek API key is set
	GeminiConfigured     bool                           `json:"gemini_configured"`         // true if Gemini API key is set
	ZaiConfigured        bool                           `json:"zai_configured"`            // true if Z.ai (Zhipu) API key is set
	GroqConfigured       bool                           `json:"groq_configured"`           // true if Groq API key is set
	MistralConfigured    bool                           `json:"mistral_configured"`        // true if Mistral API key is set
	CerebrasConfigured   bool                           `json:"cerebras_configured"`       // true if Cerebras API key is set
	TogetherConfigured   bool                           `json:"together_configured"`       // true if Together AI API key is set
	FireworksConfigured  bool                           `json:"fireworks_configured"`      // true if Fireworks AI API key is set
	OllamaConfigured     bool                           `json:"ollama_configured"`         // true (always available for attempt)
	OllamaBaseURL        string                         `json:"ollama_base_url"`           // Ollama server URL
	OllamaUsername       string                         `json:"ollama_username,omitempty"` // Optional Basic Auth username for Ollama
	OllamaPasswordSet    bool                           `json:"ollama_password_set"`       // true if an Ollama password is stored
	OllamaKeepAlive      string                         `json:"ollama_keep_alive"`         // Ollama keep_alive value; empty uses server default
	OpenAIBaseURL        string                         `json:"openai_base_url,omitempty"` // Custom OpenAI base URL
	ZaiBaseURL           string                         `json:"zai_base_url,omitempty"`    // Custom Z.ai base URL (e.g. coding endpoint)
	ConfiguredProviders  []string                       `json:"configured_providers"`      // List of provider names with credentials
	Providers            []AIProviderDefinitionResponse `json:"providers"`                 // Canonical provider registry metadata
	// Cost controls
	CostBudgetUSD30d float64 `json:"cost_budget_usd_30d,omitempty"`
	// Request timeout (seconds) - for slow hardware running local models
	RequestTimeoutSeconds int `json:"request_timeout_seconds,omitempty"`
	// Infrastructure control settings
	ControlLevel    string   `json:"control_level"`    // "read_only", "controlled", "autonomous"
	ProtectedGuests []string `json:"protected_guests"` // VMIDs/names that AI cannot control
	// Discovery settings
	DiscoveryEnabled       bool `json:"discovery_enabled"`                  // true if discovery is enabled
	DiscoveryIntervalHours int  `json:"discovery_interval_hours,omitempty"` // Hours between auto-scans (0 = manual only)
	// Current Patrol runtime readiness after this settings snapshot is applied.
	PatrolReadiness *PatrolReadinessResponse `json:"patrol_readiness,omitempty"`
	// Most recent Patrol tool-call preflight outcome, surfaced so the UI
	// can render a "last verified" indicator without forcing operators
	// to re-click Verify Patrol on every page load. nil when preflight
	// has never run on this service instance.
	PatrolPreflight *PatrolPreflightSnapshot `json:"patrol_preflight,omitempty"`
}

// AIProviderDefinitionResponse exposes provider metadata without credentials.
type AIProviderDefinitionResponse struct {
	ID                  string   `json:"id"`
	DisplayName         string   `json:"display_name"`
	Description         string   `json:"description"`
	Protocol            string   `json:"protocol"`
	DefaultModel        string   `json:"default_model,omitempty"`
	DefaultBaseURL      string   `json:"default_base_url,omitempty"`
	APIKeyField         string   `json:"api_key_field,omitempty"`
	ConfiguredField     string   `json:"configured_field,omitempty"`
	ClearKeyField       string   `json:"clear_key_field,omitempty"`
	BaseURLField        string   `json:"base_url_field,omitempty"`
	RequiresAPIKey      bool     `json:"requires_api_key"`
	UserConfigurable    bool     `json:"user_configurable"`
	Gateway             bool     `json:"gateway"`
	Configured          bool     `json:"configured"`
	ModelsDevProviderID string   `json:"models_dev_provider_id,omitempty"`
	EnvVars             []string `json:"env_vars"`
	DocsURL             string   `json:"docs_url,omitempty"`
	// Patrol-blessed quickstart model for providers where users must pick a
	// model themselves (Ollama). Empty for curated-catalog providers.
	SuggestedModel            string   `json:"suggested_model,omitempty"`
	SuggestedModelNote        string   `json:"suggested_model_note,omitempty"`
	SuggestedModelEquivalents []string `json:"suggested_model_equivalents,omitempty"`
}

// PatrolPreflightSnapshot is the API-shaped projection of the cached
// PatrolPreflightResult plus the wall-clock time it was recorded.
type PatrolPreflightSnapshot struct {
	Success          bool   `json:"success"`
	Provider         string `json:"provider,omitempty"`
	Model            string `json:"model,omitempty"`
	ToolCallObserved bool   `json:"tool_call_observed"`
	DurationMs       int64  `json:"duration_ms"`
	Cause            string `json:"cause,omitempty"`
	Title            string `json:"title,omitempty"`
	Summary          string `json:"summary,omitempty"`
	Recommendation   string `json:"recommendation,omitempty"`
	RecordedAt       string `json:"recorded_at"`
	RecordedAtUnix   int64  `json:"recorded_at_unix"`
}

func EmptyAISettingsResponse() AISettingsResponse {
	return AISettingsResponse{}.NormalizeCollections()
}

func (r AISettingsResponse) NormalizeCollections() AISettingsResponse {
	if r.AvailableModels == nil {
		r.AvailableModels = []providers.ModelInfo{}
	}
	if r.ConfiguredProviders == nil {
		r.ConfiguredProviders = []string{}
	}
	if r.Providers == nil {
		r.Providers = []AIProviderDefinitionResponse{}
	}
	if r.ProtectedGuests == nil {
		r.ProtectedGuests = []string{}
	}
	if r.PatrolAlertTriggerTypes == nil {
		r.PatrolAlertTriggerTypes = []string{}
	}
	return r
}

func aiProviderDefinitionResponses(settings *config.AIConfig) []AIProviderDefinitionResponse {
	defs := config.AIProviderDefinitions()
	responses := make([]AIProviderDefinitionResponse, 0, len(defs))
	for _, def := range defs {
		if !def.UserConfigurable {
			continue
		}
		responses = append(responses, AIProviderDefinitionResponse{
			ID:                  def.ID,
			DisplayName:         def.DisplayName,
			Description:         def.Description,
			Protocol:            string(def.Protocol),
			DefaultModel:        config.DefaultModelForProvider(def.ID),
			DefaultBaseURL:      def.DefaultBaseURL,
			APIKeyField:         def.APIKeyField,
			ConfiguredField:     def.ConfiguredField,
			ClearKeyField:       def.ClearKeyField,
			BaseURLField:        def.BaseURLField,
			RequiresAPIKey:      def.RequiresAPIKey,
			UserConfigurable:    def.UserConfigurable,
			Gateway:             def.Gateway,
			Configured:          settings != nil && settings.HasProvider(def.ID),
			ModelsDevProviderID: def.ModelsDevProviderID,
			EnvVars:             append([]string(nil), def.EnvVars...),
			DocsURL:             def.DocsURL,
			SuggestedModel:      def.SuggestedModel,
			SuggestedModelNote:  def.SuggestedModelNote,
			SuggestedModelEquivalents: append(
				[]string(nil), def.SuggestedModelEquivalents...),
		})
	}
	return responses
}

// AISettingsUpdateRequest is the request body for PUT /api/settings/ai
type AISettingsUpdateRequest struct {
	Enabled       *bool   `json:"enabled,omitempty"`
	Model         *string `json:"model,omitempty"`
	ChatModel     *string `json:"chat_model,omitempty"`     // Model for interactive chat
	PatrolModel   *string `json:"patrol_model,omitempty"`   // Model for background patrol
	AutoFixModel  *string `json:"auto_fix_model,omitempty"` // Model for auto-fix remediation
	CustomContext *string `json:"custom_context,omitempty"` // user-provided infrastructure context
	AuthMethod    *string `json:"auth_method,omitempty"`    // "api_key" or "oauth"
	// Patrol settings for token efficiency
	PatrolIntervalMinutes        *int  `json:"patrol_interval_minutes,omitempty"`         // Custom interval in minutes (0 = disabled, minimum 10)
	PatrolEnabled                *bool `json:"patrol_enabled,omitempty"`                  // true if patrol is enabled
	PatrolAutoFix                *bool `json:"patrol_auto_fix,omitempty"`                 // true if patrol can auto-fix issues
	AlertTriggeredAnalysis       *bool `json:"alert_triggered_analysis,omitempty"`        // true if AI analyzes when alerts fire
	PatrolEventTriggersEnabled   *bool `json:"patrol_event_triggers_enabled,omitempty"`   // Legacy aggregate update; applies to both scoped Patrol trigger sources
	PatrolAlertTriggersEnabled   *bool `json:"patrol_alert_triggers_enabled,omitempty"`   // true if alert-driven scoped Patrol triggers are enabled
	PatrolAnomalyTriggersEnabled *bool `json:"patrol_anomaly_triggers_enabled,omitempty"` // true if anomaly-driven scoped Patrol triggers are enabled
	// Per-rule policy for alert-driven scoped Patrol triggers.
	PatrolAlertTriggerMinSeverity *string   `json:"patrol_alert_trigger_min_severity,omitempty"` // "warning" | "critical"
	PatrolAlertTriggerTypes       *[]string `json:"patrol_alert_trigger_types,omitempty"`        // allowlist of alert types (empty slice = all types)
	UseProactiveThresholds        *bool     `json:"use_proactive_thresholds,omitempty"`          // true if patrol warns before thresholds (default: false = exact thresholds)
	// Multi-provider credentials
	AnthropicAPIKey  *string `json:"anthropic_api_key,omitempty"`  // Set Anthropic API key
	OpenAIAPIKey     *string `json:"openai_api_key,omitempty"`     // Set OpenAI API key
	OpenRouterAPIKey *string `json:"openrouter_api_key,omitempty"` // Set OpenRouter API key
	DeepSeekAPIKey   *string `json:"deepseek_api_key,omitempty"`   // Set DeepSeek API key
	GeminiAPIKey     *string `json:"gemini_api_key,omitempty"`     // Set Gemini API key
	ZaiAPIKey        *string `json:"zai_api_key,omitempty"`        // Set Z.ai (Zhipu) API key
	GroqAPIKey       *string `json:"groq_api_key,omitempty"`       // Set Groq API key
	MistralAPIKey    *string `json:"mistral_api_key,omitempty"`    // Set Mistral API key
	CerebrasAPIKey   *string `json:"cerebras_api_key,omitempty"`   // Set Cerebras API key
	TogetherAPIKey   *string `json:"together_api_key,omitempty"`   // Set Together AI API key
	FireworksAPIKey  *string `json:"fireworks_api_key,omitempty"`  // Set Fireworks AI API key
	OllamaBaseURL    *string `json:"ollama_base_url,omitempty"`    // Set Ollama server URL
	OllamaUsername   *string `json:"ollama_username,omitempty"`    // Set Ollama Basic Auth username
	OllamaPassword   *string `json:"ollama_password,omitempty"`    // Set Ollama Basic Auth password
	OllamaKeepAlive  *string `json:"ollama_keep_alive,omitempty"`  // Set Ollama keep_alive; empty uses server default
	OpenAIBaseURL    *string `json:"openai_base_url,omitempty"`    // Set custom OpenAI base URL
	ZaiBaseURL       *string `json:"zai_base_url,omitempty"`       // Set custom Z.ai base URL (e.g. coding endpoint)
	// Clear flags for removing credentials
	ClearAnthropicKey   *bool `json:"clear_anthropic_key,omitempty"`   // Clear Anthropic API key
	ClearOpenAIKey      *bool `json:"clear_openai_key,omitempty"`      // Clear OpenAI API key
	ClearOpenRouterKey  *bool `json:"clear_openrouter_key,omitempty"`  // Clear OpenRouter API key
	ClearDeepSeekKey    *bool `json:"clear_deepseek_key,omitempty"`    // Clear DeepSeek API key
	ClearGeminiKey      *bool `json:"clear_gemini_key,omitempty"`      // Clear Gemini API key
	ClearZaiKey         *bool `json:"clear_zai_key,omitempty"`         // Clear Z.ai (Zhipu) API key
	ClearGroqKey        *bool `json:"clear_groq_key,omitempty"`        // Clear Groq API key
	ClearMistralKey     *bool `json:"clear_mistral_key,omitempty"`     // Clear Mistral API key
	ClearCerebrasKey    *bool `json:"clear_cerebras_key,omitempty"`    // Clear Cerebras API key
	ClearTogetherKey    *bool `json:"clear_together_key,omitempty"`    // Clear Together AI API key
	ClearFireworksKey   *bool `json:"clear_fireworks_key,omitempty"`   // Clear Fireworks AI API key
	ClearOllamaURL      *bool `json:"clear_ollama_url,omitempty"`      // Clear Ollama URL
	ClearOllamaUsername *bool `json:"clear_ollama_username,omitempty"` // Clear Ollama Basic Auth username
	ClearOllamaPassword *bool `json:"clear_ollama_password,omitempty"` // Clear Ollama Basic Auth password
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

// AssistantEnabled reports whether the Pulse Assistant affordance should be
// shown in the authenticated shell without forcing the browser to probe the
// full AI settings API on every route bootstrap.
func (h *AISettingsHandler) AssistantEnabled(ctx context.Context) bool {
	if h == nil {
		return false
	}

	settings, err := h.loadAIConfig(ctx)
	if err != nil {
		return false
	}
	if settings == nil {
		settings = config.NewDefaultAIConfig()
	}

	enabled := settings.Enabled || mockmode.IsEnabled()
	if !enabled {
		return false
	}

	if settings.IsConfigured() || mockmode.IsEnabled() {
		return true
	}

	return false
}

func aiSettingsRequireModelResolution(settings *config.AIConfig) bool {
	if settings == nil || !settings.IsConfigured() {
		return false
	}
	model := strings.TrimSpace(settings.GetModel())
	if model == "" {
		return true
	}
	providerName, _ := config.ParseModelString(model)
	if providerName == "" || providerName == config.AIProviderQuickstart {
		return false
	}
	return !settings.HasProvider(providerName)
}

// HandleGetAISettings returns the current AI settings (GET /api/settings/ai)
func (h *AISettingsHandler) HandleGetAISettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !ensureSettingsReadScope(h.getConfig(r.Context()), w, r) {
		return
	}

	ctx := r.Context()
	settings, err := h.loadAIConfig(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load Pulse Intelligence settings")
		http.Error(w, "Failed to load Pulse Intelligence settings", http.StatusInternalServerError)
		return
	}

	if settings == nil {
		settings = config.NewDefaultAIConfig()
	}
	settings.NormalizeQuickstartModelAliases()
	if aiSettingsRequireModelResolution(settings) {
		if resolvedModel, resolveErr := ai.ResolveConfiguredModel(ctx, settings); resolveErr == nil {
			settings.Model = resolvedModel
		}
	}

	// Determine auth method string
	authMethod := string(settings.AuthMethod)
	if authMethod == "" {
		authMethod = string(config.AuthMethodAPIKey)
	}

	// Determine if running in demo mode
	isDemo := mockmode.IsEnabled()
	triggerSettings := settings.GetPatrolEventTriggerSettings()
	aiService := h.GetAIService(ctx)
	hasAutoFixFeature := aiService.HasLicenseFeature(ai.FeatureAIAutoFix)
	hasAlertAnalysisFeature := aiService.HasLicenseFeature(ai.FeatureAIAlerts)

	response := AISettingsResponse{
		Enabled:        settings.Enabled || isDemo,
		Model:          settings.GetModel(),
		ChatModel:      config.NormalizeQuickstartModelString(settings.ChatModel),
		PatrolModel:    config.NormalizeQuickstartModelString(settings.PatrolModel),
		AutoFixModel:   config.NormalizeQuickstartModelString(settings.AutoFixModel),
		Configured:     settings.IsConfigured() || isDemo,
		CustomContext:  settings.CustomContext,
		AuthMethod:     authMethod,
		OAuthConnected: settings.OAuthAccessToken != "",
		// Patrol settings
		PatrolIntervalMinutes:         settings.PatrolIntervalMinutes,
		PatrolEnabled:                 settings.PatrolEnabled,
		PatrolAutoFix:                 settings.PatrolAutoFix && hasAutoFixFeature,
		AlertTriggeredAnalysis:        settings.AlertTriggeredAnalysis && hasAlertAnalysisFeature,
		PatrolEventTriggersEnabled:    triggerSettings.AlertTriggersEnabled || triggerSettings.AnomalyTriggersEnabled,
		PatrolAlertTriggersEnabled:    triggerSettings.AlertTriggersEnabled,
		PatrolAnomalyTriggersEnabled:  triggerSettings.AnomalyTriggersEnabled,
		PatrolAlertTriggerMinSeverity: settings.GetPatrolAlertTriggerMinSeverity(),
		PatrolAlertTriggerTypes:       settings.PatrolAlertTriggerTypes,
		UseProactiveThresholds:        settings.UseProactiveThresholds,
		AvailableModels:               nil, // Now populated via /api/ai/models endpoint
		// Multi-provider configuration
		AnthropicConfigured:    settings.HasProvider(config.AIProviderAnthropic),
		OpenAIConfigured:       settings.HasProvider(config.AIProviderOpenAI),
		OpenRouterConfigured:   settings.HasProvider(config.AIProviderOpenRouter),
		DeepSeekConfigured:     settings.HasProvider(config.AIProviderDeepSeek),
		GeminiConfigured:       settings.HasProvider(config.AIProviderGemini),
		ZaiConfigured:          settings.HasProvider(config.AIProviderZai),
		GroqConfigured:         settings.HasProvider(config.AIProviderGroq),
		MistralConfigured:      settings.HasProvider(config.AIProviderMistral),
		CerebrasConfigured:     settings.HasProvider(config.AIProviderCerebras),
		TogetherConfigured:     settings.HasProvider(config.AIProviderTogether),
		FireworksConfigured:    settings.HasProvider(config.AIProviderFireworks),
		OllamaConfigured:       settings.HasProvider(config.AIProviderOllama),
		OllamaBaseURL:          settings.GetBaseURLForProvider(config.AIProviderOllama),
		OllamaUsername:         settings.OllamaUsername,
		OllamaPasswordSet:      settings.OllamaPassword != "",
		OllamaKeepAlive:        settings.GetOllamaKeepAlive(),
		OpenAIBaseURL:          settings.OpenAIBaseURL,
		ZaiBaseURL:             settings.ZaiBaseURL,
		ConfiguredProviders:    settings.GetConfiguredProviders(),
		Providers:              aiProviderDefinitionResponses(settings),
		CostBudgetUSD30d:       settings.CostBudgetUSD30d,
		RequestTimeoutSeconds:  settings.RequestTimeoutSeconds,
		ControlLevel:           settings.GetEffectiveControlLevel(hasAutoFixFeature),
		ProtectedGuests:        settings.GetProtectedGuests(),
		DiscoveryEnabled:       settings.IsDiscoveryEnabled(),
		DiscoveryIntervalHours: settings.DiscoveryIntervalHours,
		PatrolPreflight:        cachedPatrolPreflightSnapshot(aiService),
		PatrolReadiness:        ptrToPatrolReadiness(h.buildPatrolReadiness(ctx, aiService, h.getPatrolService(ctx) != nil)),
	}.NormalizeCollections()

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
	if !ensureSettingsWriteScope(h.getConfig(r.Context()), w, r) {
		return
	}

	// Load existing settings
	settings, err := h.loadAIConfig(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to load existing AI settings")
		settings = config.NewDefaultAIConfig()
	}
	if settings == nil {
		settings = config.NewDefaultAIConfig()
	}

	// Snapshot the pre-mutation config so we can detect Patrol-affecting
	// changes after save and decide whether to auto-trigger preflight.
	// Shallow copy is fine — the detection helper only reads scalar
	// fields (Enabled, PatrolModel, API keys).
	originalSettings := *settings

	// Parse request
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var req AISettingsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate and apply updates
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
		// Patrol fix actions require Pro license with the ai_autofix feature.
		if *req.PatrolAutoFix && !h.GetAIService(r.Context()).HasLicenseFeature(ai.FeatureAIAutoFix) {
			WriteLicenseRequired(w, ai.FeatureAIAutoFix, "Patrol fix actions require Pulse Pro")
			return
		}
		settings.PatrolAutoFix = *req.PatrolAutoFix
	}

	if req.UseProactiveThresholds != nil {
		settings.UseProactiveThresholds = *req.UseProactiveThresholds
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
	if req.ClearZaiKey != nil && *req.ClearZaiKey {
		settings.ZaiAPIKey = ""
	} else if req.ZaiAPIKey != nil {
		settings.ZaiAPIKey = strings.TrimSpace(*req.ZaiAPIKey)
	}
	if req.ClearGroqKey != nil && *req.ClearGroqKey {
		settings.GroqAPIKey = ""
	} else if req.GroqAPIKey != nil {
		settings.GroqAPIKey = strings.TrimSpace(*req.GroqAPIKey)
	}
	if req.ClearMistralKey != nil && *req.ClearMistralKey {
		settings.MistralAPIKey = ""
	} else if req.MistralAPIKey != nil {
		settings.MistralAPIKey = strings.TrimSpace(*req.MistralAPIKey)
	}
	if req.ClearCerebrasKey != nil && *req.ClearCerebrasKey {
		settings.CerebrasAPIKey = ""
	} else if req.CerebrasAPIKey != nil {
		settings.CerebrasAPIKey = strings.TrimSpace(*req.CerebrasAPIKey)
	}
	if req.ClearTogetherKey != nil && *req.ClearTogetherKey {
		settings.TogetherAPIKey = ""
	} else if req.TogetherAPIKey != nil {
		settings.TogetherAPIKey = strings.TrimSpace(*req.TogetherAPIKey)
	}
	if req.ClearFireworksKey != nil && *req.ClearFireworksKey {
		settings.FireworksAPIKey = ""
	} else if req.FireworksAPIKey != nil {
		settings.FireworksAPIKey = strings.TrimSpace(*req.FireworksAPIKey)
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
	if req.ClearOllamaUsername != nil && *req.ClearOllamaUsername {
		settings.OllamaUsername = ""
	} else if req.OllamaUsername != nil {
		settings.OllamaUsername = strings.TrimSpace(*req.OllamaUsername)
	}
	if req.ClearOllamaPassword != nil && *req.ClearOllamaPassword {
		settings.OllamaPassword = ""
	} else if req.OllamaPassword != nil {
		settings.OllamaPassword = *req.OllamaPassword
	}
	if req.OllamaKeepAlive != nil {
		keepAlive, err := config.NormalizeOllamaKeepAlive(*req.OllamaKeepAlive)
		if err != nil {
			http.Error(w, "ollama_keep_alive "+err.Error(), http.StatusBadRequest)
			return
		}
		settings.OllamaKeepAlive = keepAlive
	}
	if req.OpenAIBaseURL != nil {
		settings.OpenAIBaseURL = strings.TrimSpace(*req.OpenAIBaseURL)
	}
	if req.ZaiBaseURL != nil {
		settings.ZaiBaseURL = strings.TrimSpace(*req.ZaiBaseURL)
	}

	if req.Enabled != nil {
		// Only allow enabling if at least one BYOK/local provider is configured.
		if *req.Enabled {
			configuredProviders := settings.GetConfiguredProviders()
			if len(configuredProviders) == 0 {
				http.Error(w, "Please configure a provider (API key or Ollama URL) before enabling Pulse Assistant", http.StatusBadRequest)
				return
			} else {
				if aiSettingsRequireModelResolution(settings) {
					resolvedModel, resolveErr := ai.ResolveConfiguredModel(r.Context(), settings)
					if resolveErr != nil {
						log.Warn().Err(resolveErr).Msg("Provider model resolution failed during enable; saving settings and reporting Patrol readiness instead")
					} else {
						settings.Model = resolvedModel
					}
				}
			}
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
		if minutes > 0 {
			settings.PatrolEnabled = true // Enable patrol when setting custom interval
		} else {
			settings.PatrolEnabled = false // Disable patrol when setting interval to 0
		}
	}

	if req.PatrolEnabled != nil && req.PatrolIntervalMinutes == nil {
		settings.PatrolEnabled = *req.PatrolEnabled
		if *req.PatrolEnabled {
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

	// Handle legacy aggregate event-triggered patrol toggle
	if req.PatrolEventTriggersEnabled != nil {
		settings.SetPatrolEventTriggersEnabled(*req.PatrolEventTriggersEnabled)
	}

	// Handle split scoped patrol trigger toggles
	if req.PatrolAlertTriggersEnabled != nil || req.PatrolAnomalyTriggersEnabled != nil {
		triggerSettings := settings.GetPatrolEventTriggerSettings()
		if req.PatrolAlertTriggersEnabled != nil {
			triggerSettings.AlertTriggersEnabled = *req.PatrolAlertTriggersEnabled
		}
		if req.PatrolAnomalyTriggersEnabled != nil {
			triggerSettings.AnomalyTriggersEnabled = *req.PatrolAnomalyTriggersEnabled
		}
		settings.SetPatrolEventTriggerSettings(
			triggerSettings.AlertTriggersEnabled,
			triggerSettings.AnomalyTriggersEnabled,
		)
	}

	// Handle alert-trigger investigation policy (minimum severity + type allowlist)
	if req.PatrolAlertTriggerMinSeverity != nil {
		sev := strings.ToLower(strings.TrimSpace(*req.PatrolAlertTriggerMinSeverity))
		if sev != config.AlertTriggerSeverityWarning && sev != config.AlertTriggerSeverityCritical {
			http.Error(w, "patrol_alert_trigger_min_severity must be 'warning' or 'critical'", http.StatusBadRequest)
			return
		}
		settings.PatrolAlertTriggerMinSeverity = sev
	}
	if req.PatrolAlertTriggerTypes != nil {
		cleaned := make([]string, 0, len(*req.PatrolAlertTriggerTypes))
		seen := make(map[string]struct{}, len(*req.PatrolAlertTriggerTypes))
		for _, t := range *req.PatrolAlertTriggerTypes {
			t = strings.ToLower(strings.TrimSpace(t))
			if t == "" {
				continue
			}
			if _, dup := seen[t]; dup {
				continue
			}
			seen[t] = struct{}{}
			cleaned = append(cleaned, t)
		}
		settings.PatrolAlertTriggerTypes = cleaned
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
		if !config.IsValidControlLevel(level) {
			http.Error(w, "invalid control_level: must be read_only, controlled, or autonomous", http.StatusBadRequest)
			return
		}
		// "autonomous" requires Pro license
		if level == config.ControlLevelAutonomous {
			if !h.GetAIService(r.Context()).HasLicenseFeature(ai.FeatureAIAutoFix) {
				WriteLicenseRequired(w, ai.FeatureAIAutoFix, "Autonomous control requires Pulse Pro")
				return
			}
		}
		settings.ControlLevel = level
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

	// Auto-default discovery interval only when enabling without an explicit interval.
	// A provided interval of 0 is the canonical manual-only setting and must persist.
	if req.DiscoveryEnabled != nil && *req.DiscoveryEnabled && req.DiscoveryIntervalHours == nil && settings.DiscoveryIntervalHours == 0 {
		settings.DiscoveryIntervalHours = 24
	}

	if aiSettingsRequireModelResolution(settings) {
		resolvedModel, resolveErr := ai.ResolveConfiguredModel(r.Context(), settings)
		if resolveErr != nil {
			log.Warn().Err(resolveErr).Msg("Provider model resolution failed during settings update; saving settings and reporting Patrol readiness instead")
		} else {
			settings.Model = resolvedModel
		}
	}
	settings.NormalizeQuickstartModelAliases()
	if settings.IsPatrolEnabled() && aiSettingsUpdateTouchesPatrolReadiness(req) {
		readiness := ai.EvaluatePatrolConfigReadiness(settings)
		if !readiness.Ready {
			log.Info().
				Str("cause", string(readiness.Cause)).
				Str("provider", readiness.Provider).
				Str("model", config.NormalizeQuickstartModelString(readiness.Model)).
				Msg("AI settings saved with Patrol runtime readiness blocker")
		}
	}

	// Save settings
	if err := h.getPersistence(r.Context()).SaveAIConfig(*settings); err != nil {
		log.Error().Err(err).Msg("Failed to save AI settings")
		writeErrorResponse(w, http.StatusInternalServerError, "ai_settings_save_failed", "Failed to save Pulse Intelligence settings", nil)
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

	// Auto-trigger Patrol tool-call preflight when the change actually
	// moved the Patrol transport (model swap or new key for the model's
	// provider). Runs in the background so the save response isn't
	// blocked; the cached result surfaces on the next /api/settings/ai
	// poll via patrol_preflight. Routine saves that don't touch Patrol
	// transport skip this entirely so they don't burn provider tokens.
	if aiSettingsUpdateRequiresPatrolPreflight(&originalSettings, settings) {
		h.GetAIService(r.Context()).TriggerPatrolPreflightAsync("", "")
	}

	// Trigger AI chat service restart when provider-affecting settings change.
	// The running service keeps provider configuration in memory, so credential,
	// base URL, or model updates must restart the chat runtime to take effect.
	if h.onModelChange != nil && shouldRestartAIChat(req) {
		h.onModelChange()
	}

	// Update Assistant control settings if control level or protected guests changed.
	// This updates tool visibility without restarting AI chat.
	if h.onControlSettingsChange != nil && (req.ControlLevel != nil || req.ProtectedGuests != nil) {
		h.onControlSettingsChange()
	}

	providerName, _ := config.ParseModelString(settings.GetModel())
	LogAuditEventForTenant(GetOrgID(r.Context()), "ai_settings_updated", getAuthUsername(h.getConfig(r.Context()), r), GetClientIP(r), r.URL.Path, true,
		fmt.Sprintf("AI settings updated: enabled=%t provider=%s model=%s", settings.Enabled, providerName, settings.GetModel()))

	log.Info().
		Bool("enabled", settings.Enabled).
		Str("provider", providerName).
		Str("model", settings.GetModel()).
		Str("chatModel", config.NormalizeQuickstartModelString(settings.ChatModel)).
		Str("patrolModel", config.NormalizeQuickstartModelString(settings.PatrolModel)).
		Bool("alertTriggeredAnalysis", settings.AlertTriggeredAnalysis).
		Msg("AI settings updated")

	// Determine auth method for response
	authMethod := string(settings.AuthMethod)
	if authMethod == "" {
		authMethod = string(config.AuthMethodAPIKey)
	}
	triggerSettings := settings.GetPatrolEventTriggerSettings()
	aiService := h.GetAIService(r.Context())
	hasAutoFixFeature := aiService.HasLicenseFeature(ai.FeatureAIAutoFix)
	hasAlertAnalysisFeature := aiService.HasLicenseFeature(ai.FeatureAIAlerts)
	patrolReadiness := patrolReadinessResponseFromConfigReadiness(ai.EvaluatePatrolConfigReadiness(settings))

	// Return updated settings
	response := AISettingsResponse{
		Enabled:                       settings.Enabled,
		Model:                         settings.GetModel(),
		ChatModel:                     config.NormalizeQuickstartModelString(settings.ChatModel),
		PatrolModel:                   config.NormalizeQuickstartModelString(settings.PatrolModel),
		AutoFixModel:                  config.NormalizeQuickstartModelString(settings.AutoFixModel),
		Configured:                    settings.IsConfigured(),
		CustomContext:                 settings.CustomContext,
		AuthMethod:                    authMethod,
		OAuthConnected:                settings.OAuthAccessToken != "",
		PatrolIntervalMinutes:         settings.PatrolIntervalMinutes,
		PatrolEnabled:                 settings.PatrolEnabled,
		PatrolAutoFix:                 settings.PatrolAutoFix && hasAutoFixFeature,
		AlertTriggeredAnalysis:        settings.AlertTriggeredAnalysis && hasAlertAnalysisFeature,
		PatrolEventTriggersEnabled:    triggerSettings.AlertTriggersEnabled || triggerSettings.AnomalyTriggersEnabled,
		PatrolAlertTriggersEnabled:    triggerSettings.AlertTriggersEnabled,
		PatrolAnomalyTriggersEnabled:  triggerSettings.AnomalyTriggersEnabled,
		PatrolAlertTriggerMinSeverity: settings.GetPatrolAlertTriggerMinSeverity(),
		PatrolAlertTriggerTypes:       settings.PatrolAlertTriggerTypes,
		UseProactiveThresholds:        settings.UseProactiveThresholds,
		AvailableModels:               nil, // Now populated via /api/ai/models endpoint
		// Multi-provider configuration
		AnthropicConfigured:    settings.HasProvider(config.AIProviderAnthropic),
		OpenAIConfigured:       settings.HasProvider(config.AIProviderOpenAI),
		OpenRouterConfigured:   settings.HasProvider(config.AIProviderOpenRouter),
		DeepSeekConfigured:     settings.HasProvider(config.AIProviderDeepSeek),
		GeminiConfigured:       settings.HasProvider(config.AIProviderGemini),
		ZaiConfigured:          settings.HasProvider(config.AIProviderZai),
		GroqConfigured:         settings.HasProvider(config.AIProviderGroq),
		MistralConfigured:      settings.HasProvider(config.AIProviderMistral),
		CerebrasConfigured:     settings.HasProvider(config.AIProviderCerebras),
		TogetherConfigured:     settings.HasProvider(config.AIProviderTogether),
		FireworksConfigured:    settings.HasProvider(config.AIProviderFireworks),
		OllamaConfigured:       settings.HasProvider(config.AIProviderOllama),
		OllamaBaseURL:          settings.GetBaseURLForProvider(config.AIProviderOllama),
		OllamaUsername:         settings.OllamaUsername,
		OllamaPasswordSet:      settings.OllamaPassword != "",
		OllamaKeepAlive:        settings.GetOllamaKeepAlive(),
		OpenAIBaseURL:          settings.OpenAIBaseURL,
		ZaiBaseURL:             settings.ZaiBaseURL,
		ConfiguredProviders:    settings.GetConfiguredProviders(),
		Providers:              aiProviderDefinitionResponses(settings),
		RequestTimeoutSeconds:  settings.RequestTimeoutSeconds,
		ControlLevel:           settings.GetEffectiveControlLevel(hasAutoFixFeature),
		ProtectedGuests:        settings.GetProtectedGuests(),
		DiscoveryEnabled:       settings.DiscoveryEnabled,
		DiscoveryIntervalHours: settings.DiscoveryIntervalHours,
		PatrolReadiness:        ptrToPatrolReadiness(patrolReadiness),
		PatrolPreflight:        cachedPatrolPreflightSnapshot(aiService),
	}.NormalizeCollections()

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write AI settings update response")
	}
}

// cachedPatrolPreflightSnapshot projects the AI Service's cached
// preflight result onto the API's wire shape. Returns nil when no
// preflight has run on this service instance.
func cachedPatrolPreflightSnapshot(aiService *ai.Service) *PatrolPreflightSnapshot {
	if aiService == nil {
		return nil
	}
	result, recordedAt := aiService.CachedPatrolPreflight()
	if result == nil || recordedAt.IsZero() {
		return nil
	}
	return &PatrolPreflightSnapshot{
		Success:          result.Success,
		Provider:         result.Provider,
		Model:            result.Model,
		ToolCallObserved: result.ToolCallObserved,
		DurationMs:       result.DurationMs,
		Cause:            string(result.Cause),
		Title:            result.Title,
		Summary:          result.Summary,
		Recommendation:   result.Recommendation,
		RecordedAt:       recordedAt.UTC().Format(time.RFC3339),
		RecordedAtUnix:   recordedAt.Unix(),
	}
}

// HandleTestAIConnection tests the AI provider connection (POST /api/ai/test)
// Auth is enforced by RequirePermission middleware at route registration; with
// default authorizer, non-admin proxy users are hard-denied (with RBAC, deferred
// to authorizer). Token scope is enforced by RequireScope middleware.
// ensureSettingsWriteScope provides the additional admin-session identity guard
// for session-based (non-token) users.
func (h *AISettingsHandler) HandleTestAIConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !ensureSettingsWriteScope(h.getConfig(r.Context()), w, r) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var testResult struct {
		Success        bool   `json:"success"`
		Message        string `json:"message"`
		Model          string `json:"model,omitempty"`
		Cause          string `json:"cause,omitempty"`
		Summary        string `json:"summary,omitempty"`
		Recommendation string `json:"recommendation,omitempty"`
		Action         string `json:"action,omitempty"`
	}

	cfg := h.GetAIService(r.Context()).GetConfig()
	err := h.GetAIService(r.Context()).TestConnection(ctx)
	if err != nil {
		diagnostic := ai.ClassifyProviderConnectionFailure(err)
		testResult.Success = false
		testResult.Message = diagnostic.Summary
		testResult.Cause = string(diagnostic.Cause)
		testResult.Summary = diagnostic.Description
		testResult.Recommendation = diagnostic.Recommendation
		testResult.Action = "open_provider_settings"
		if cfg != nil {
			testResult.Model = config.NormalizeQuickstartModelString(cfg.GetModel())
		}
		log.Error().Err(err).Msg("AI connection test failed")
	} else {
		testResult.Success = true
		testResult.Message = "Connection successful"
		if cfg != nil {
			testResult.Model = config.NormalizeQuickstartModelString(cfg.GetModel())
		}
	}

	if err := utils.WriteJSONResponse(w, testResult); err != nil {
		log.Error().Err(err).Msg("Failed to write AI test response")
	}
}

type aiProviderTestResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	Provider       string `json:"provider"`
	Model          string `json:"model,omitempty"`
	Cause          string `json:"cause,omitempty"`
	Summary        string `json:"summary,omitempty"`
	Recommendation string `json:"recommendation,omitempty"`
	Action         string `json:"action,omitempty"`
}

type aiProviderTestRequest struct {
	Model string `json:"model,omitempty"`
}

func newAIProviderTestResponse(provider string) aiProviderTestResponse {
	return aiProviderTestResponse{Provider: provider}
}

func newAIProviderTestNotConfiguredResponse(provider string) aiProviderTestResponse {
	return aiProviderTestResponse{
		Success:        false,
		Message:        "Provider not configured",
		Provider:       provider,
		Cause:          string(ai.PatrolFailureCauseProviderNotConfigured),
		Summary:        "Pulse cannot test this provider because it is not configured for the current Pulse Intelligence settings.",
		Recommendation: "Open the Provider & Models settings page, configure the provider credentials or base URL, choose a model, and retry.",
		Action:         "open_provider_settings",
	}
}

func newAIProviderTestFailureResponse(provider, model string, err error) aiProviderTestResponse {
	diagnostic := ai.ClassifyProviderConnectionFailure(err)
	return aiProviderTestResponse{
		Success:        false,
		Message:        diagnostic.Summary,
		Provider:       provider,
		Model:          config.NormalizeQuickstartModelString(model),
		Cause:          string(diagnostic.Cause),
		Summary:        diagnostic.Description,
		Recommendation: diagnostic.Recommendation,
		Action:         "open_provider_settings",
	}
}

// HandleTestProvider tests a specific AI provider connection (POST /api/ai/test/:provider)
// Auth is enforced by RequirePermission middleware at route registration; with
// default authorizer, non-admin proxy users are hard-denied (with RBAC, deferred
// to authorizer). Token scope is enforced by RequireScope middleware.
// ensureSettingsWriteScope provides the additional admin-session identity guard
// for session-based (non-token) users.
func (h *AISettingsHandler) HandleTestProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !ensureSettingsWriteScope(h.getConfig(r.Context()), w, r) {
		return
	}

	// Get provider from URL path (e.g., /api/ai/test/anthropic -> anthropic)
	provider := strings.TrimPrefix(r.URL.Path, "/api/ai/test/")
	if provider == "" || provider == r.URL.Path {
		http.Error(w, `{"error":"Provider is required"}`, http.StatusBadRequest)
		return
	}

	// Validate provider name: only allow lowercase alphanumeric and hyphens,
	// max 64 chars. This rejects path traversal, slashes, and injection attempts.
	if len(provider) > 64 || !isValidProviderName(provider) {
		http.Error(w, `{"error":"Invalid provider name"}`, http.StatusBadRequest)
		return
	}

	var body aiProviderTestRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"Invalid JSON body"}`, http.StatusBadRequest)
			return
		}
	}
	requestedModel := config.NormalizeQuickstartModelString(body.Model)
	if len(requestedModel) > 256 {
		http.Error(w, `{"error":"Model id too long"}`, http.StatusBadRequest)
		return
	}
	if requestedModel != "" && strings.Contains(requestedModel, ":") {
		modelProvider, _ := config.ParseModelString(requestedModel)
		if modelProvider != provider {
			http.Error(w, `{"error":"Model provider does not match test provider"}`, http.StatusBadRequest)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	testResult := newAIProviderTestResponse(provider)

	// Load config and create provider for testing
	cfg := h.GetAIService(r.Context()).GetConfig()
	if cfg == nil {
		testResult = newAIProviderTestNotConfiguredResponse(provider)
		testResult.Message = "Pulse Assistant not configured"
		if err := utils.WriteJSONResponse(w, testResult); err != nil {
			log.Error().Err(err).Msg("failed to write JSON response")
		}
		return
	}

	// Check if provider is configured
	if !cfg.HasProvider(provider) {
		testResult = newAIProviderTestNotConfiguredResponse(provider)
		if err := utils.WriteJSONResponse(w, testResult); err != nil {
			log.Error().Err(err).Msg("failed to write JSON response")
		}
		return
	}

	// Create provider and test connection
	model := requestedModel
	if model == "" {
		var err error
		model, err = ai.ResolvePreferredModelForProvider(ctx, cfg, provider)
		if err != nil {
			testResult = newAIProviderTestFailureResponse(provider, "", err)
			log.Error().Err(err).Str("provider", provider).Msg("AI provider model resolution failed")
			if err := utils.WriteJSONResponse(w, testResult); err != nil {
				log.Error().Err(err).Msg("failed to write provider test response")
			}
			return
		}
	}

	testProvider, err := providers.NewForProvider(cfg, provider, model)
	if err != nil {
		testResult = newAIProviderTestFailureResponse(provider, model, err)
		log.Error().Err(err).Str("provider", provider).Msg("AI provider creation failed")
		if err := utils.WriteJSONResponse(w, testResult); err != nil {
			log.Error().Err(err).Msg("failed to write JSON response")
		}
		return
	}

	err = testProvider.TestConnection(ctx)
	if err != nil {
		testResult = newAIProviderTestFailureResponse(provider, model, err)
		log.Error().Err(err).Str("provider", provider).Msg("AI provider connection test failed")
	} else {
		testResult.Success = true
		testResult.Message = "Connection successful"
		testResult.Model = config.NormalizeQuickstartModelString(model)
	}

	if err := utils.WriteJSONResponse(w, testResult); err != nil {
		log.Error().Err(err).Msg("Failed to write provider test response")
	}
}

// isValidProviderName returns true if s contains only lowercase letters, digits,
// and hyphens. This prevents path traversal, URL injection, and log injection.
func isValidProviderName(s string) bool {
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return len(s) > 0
}

// HandleListModels fetches available models from the configured AI provider (GET /api/ai/models)
func (h *AISettingsHandler) HandleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Auth is enforced by RequireAuth + RequireScope middleware at the route level.

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	type ModelInfo struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		CreatedAt   int64  `json:"created_at,omitempty"`
		Notable     bool   `json:"notable"`
		Provider    string `json:"provider,omitempty"`
	}

	type Response struct {
		Models []ModelInfo `json:"models"`
		Error  string      `json:"error,omitempty"`
		Cached bool        `json:"cached"`
	}

	models, cached, err := h.GetAIService(r.Context()).ListModelsWithCache(ctx)
	if err != nil {
		// Return error but don't fail the request - frontend can show a fallback
		log.Error().Err(err).Msg("Failed to list AI models")
		resp := Response{
			Models: []ModelInfo{},
			Error:  "Failed to fetch model list",
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
			Provider:    m.Provider,
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
	TargetType string                  `json:"target_type,omitempty"` // "agent", "system-container", "vm"
	TargetID   string                  `json:"target_id,omitempty"`
	Context    map[string]interface{}  `json:"context,omitempty"` // Current metrics, state, etc.
	History    []AIConversationMessage `json:"history,omitempty"` // Previous conversation messages
	FindingID  string                  `json:"finding_id,omitempty"`
	Model      string                  `json:"model,omitempty"`
	UseCase    string                  `json:"use_case,omitempty"` // "chat" or "patrol"
}

func normalizeAIExecuteTargetType(raw string) string {
	return agentcapabilities.NormalizeActionTargetType(raw)
}

func normalizeAndValidateAIExecuteTargetType(raw string) (string, error) {
	return agentcapabilities.NormalizeAndValidateOptionalActionTargetType(raw)
}

// AIExecuteResponse is the response from POST /api/ai/execute
type AIExecuteResponse struct {
	Content          string                  `json:"content"`
	Model            string                  `json:"model"`
	InputTokens      int                     `json:"input_tokens"`
	OutputTokens     int                     `json:"output_tokens"`
	ToolCalls        []ai.ToolExecution      `json:"tool_calls"`        // Commands that were executed
	PendingApprovals []ai.ApprovalNeededData `json:"pending_approvals"` // Commands that require approval (non-streaming)
}

func EmptyAIExecuteResponse() AIExecuteResponse {
	return AIExecuteResponse{}.NormalizeCollections()
}

func (r AIExecuteResponse) NormalizeCollections() AIExecuteResponse {
	if r.ToolCalls == nil {
		r.ToolCalls = []ai.ToolExecution{}
	}
	if r.PendingApprovals == nil {
		r.PendingApprovals = []ai.ApprovalNeededData{}
	}
	return r
}

type aiExecuteStreamCompleteEvent struct {
	Type         string             `json:"type"`
	Model        string             `json:"model"`
	InputTokens  int                `json:"input_tokens"`
	OutputTokens int                `json:"output_tokens"`
	ToolCalls    []ai.ToolExecution `json:"tool_calls"`
}

func emptyAIExecuteStreamCompleteEvent() aiExecuteStreamCompleteEvent {
	return aiExecuteStreamCompleteEvent{}.NormalizeCollections()
}

func (e aiExecuteStreamCompleteEvent) NormalizeCollections() aiExecuteStreamCompleteEvent {
	if e.ToolCalls == nil {
		e.ToolCalls = []ai.ToolExecution{}
	}
	return e
}

type legacyAssistantSSEWriter struct {
	w                     http.ResponseWriter
	flusher               http.Flusher
	rc                    *http.ResponseController
	cancel                context.CancelFunc
	cancelOnDisconnect    bool
	writeMu               sync.Mutex
	clientDisconnected    atomic.Bool
	lastClientEventUnixMS atomic.Int64
	terminalEventsStarted atomic.Bool
	heartbeatDone         chan struct{}
	heartbeatDoneClosed   atomic.Bool
	executionDone         chan struct{}
	executionDoneClosed   atomic.Bool
}

func newLegacyAssistantSSEWriter(w http.ResponseWriter, flusher http.Flusher, rc *http.ResponseController, cancel context.CancelFunc, cancelOnDisconnect bool) *legacyAssistantSSEWriter {
	writer := &legacyAssistantSSEWriter{
		w:                  w,
		flusher:            flusher,
		rc:                 rc,
		cancel:             cancel,
		cancelOnDisconnect: cancelOnDisconnect,
		heartbeatDone:      make(chan struct{}),
		executionDone:      make(chan struct{}),
	}
	writer.lastClientEventUnixMS.Store(time.Now().UnixMilli())
	return writer
}

func (s *legacyAssistantSSEWriter) start(ctx context.Context) {
	s.startHeartbeat(ctx)
	s.startIdleProgress(ctx)
}

func (s *legacyAssistantSSEWriter) stop() {
	s.closeHeartbeat()
	s.finishExecution()
}

func (s *legacyAssistantSSEWriter) closeHeartbeat() {
	if s.heartbeatDoneClosed.CompareAndSwap(false, true) {
		close(s.heartbeatDone)
	}
}

func (s *legacyAssistantSSEWriter) finishExecution() {
	if s.executionDoneClosed.CompareAndSwap(false, true) {
		close(s.executionDone)
	}
}

func (s *legacyAssistantSSEWriter) disconnected() bool {
	return s.clientDisconnected.Load()
}

func (s *legacyAssistantSSEWriter) startHeartbeat(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.writeMu.Lock()
				if s.clientDisconnected.Load() {
					s.writeMu.Unlock()
					return
				}
				_ = s.rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
				_, err := s.w.Write([]byte(": heartbeat\n\n"))
				if err != nil {
					s.writeMu.Unlock()
					s.markDisconnected()
					return
				}
				s.flusher.Flush()
				s.writeMu.Unlock()
				log.Debug().Msg("Sent SSE heartbeat")
			case <-s.heartbeatDone:
				return
			}
		}
	}()
}

func (s *legacyAssistantSSEWriter) startIdleProgress(ctx context.Context) {
	interval := chatStreamIdleProgressInterval
	if interval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.heartbeatDone:
				return
			case <-s.executionDone:
				return
			case <-ticker.C:
				if s.clientDisconnected.Load() {
					return
				}
				if s.terminalEventsStarted.Load() || s.executionDoneClosed.Load() {
					return
				}
				lastEventAt := time.UnixMilli(s.lastClientEventUnixMS.Load())
				if time.Since(lastEventAt) < interval {
					continue
				}
				progressEvent := ai.StreamEvent{
					Type: "workflow_state",
					Data: chat.WorkflowStateData{
						Phase:   "stream_idle",
						Message: chatStreamIdleProgressMessage,
					},
				}
				s.writePayloadIf(progressEvent, true, func() bool {
					return !s.terminalEventsStarted.Load() && !s.executionDoneClosed.Load()
				})
			}
		}
	}()
}

func (s *legacyAssistantSSEWriter) markDisconnected() {
	s.clientDisconnected.Store(true)
	if s.cancelOnDisconnect && s.cancel != nil {
		s.cancel()
	}
}

func (s *legacyAssistantSSEWriter) writePayload(payload interface{}) bool {
	return s.writePayloadIf(payload, true, nil)
}

func (s *legacyAssistantSSEWriter) writeTerminalPayload(payload interface{}) bool {
	s.terminalEventsStarted.Store(true)
	s.finishExecution()
	return s.writePayloadIf(payload, true, nil)
}

func (s *legacyAssistantSSEWriter) writePayloadIf(payload interface{}, visible bool, shouldWrite func() bool) bool {
	if s.clientDisconnected.Load() {
		return false
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal SSE event")
		return false
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.clientDisconnected.Load() {
		return false
	}
	if shouldWrite != nil && !shouldWrite() {
		return false
	}
	_ = s.rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err = s.w.Write([]byte("data: " + string(data) + "\n\n"))
	if err != nil {
		log.Debug().Err(err).Msg("Failed to write SSE event (client may have disconnected)")
		s.markDisconnected()
		return false
	}
	s.flusher.Flush()
	if visible {
		s.lastClientEventUnixMS.Store(time.Now().UnixMilli())
	}
	return true
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

	// Authentication is enforced by RequireAdmin middleware at route registration.

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
			WriteLicenseRequired(w, ai.FeatureAIAutoFix, "Patrol fix actions require Pulse Pro")
			return
		}
	}

	if strings.TrimSpace(req.Prompt) == "" {
		http.Error(w, "Prompt is required", http.StatusBadRequest)
		return
	}

	// Validate and normalize target_type if provided
	targetType, err := normalizeAndValidateAIExecuteTargetType(req.TargetType)
	if err != nil {
		http.Error(w, "Invalid target_type (allowed: agent, system-container, vm)", http.StatusBadRequest)
		return
	}

	// Validate target_id length if provided
	if len(req.TargetID) > 256 {
		http.Error(w, "target_id exceeds maximum length", http.StatusBadRequest)
		return
	}

	// Validate conversation history roles
	for _, msg := range req.History {
		switch msg.Role {
		case "user", "assistant":
			// valid
		default:
			http.Error(w, "Invalid role in history (allowed: user, assistant)", http.StatusBadRequest)
			return
		}
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
		TargetType: targetType,
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
	}.NormalizeCollections()

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

	// NOTE: Authentication is enforced by RequireAdmin middleware at route
	// registration (router_routes_ai_relay.go). No redundant CheckAuth needed.

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
			WriteLicenseRequired(w, ai.FeatureAIAutoFix, "Patrol fix actions require Pulse Pro")
			return
		}
	}

	if strings.TrimSpace(req.Prompt) == "" {
		http.Error(w, "Prompt is required", http.StatusBadRequest)
		return
	}

	// Validate and normalize target_type if provided
	targetType, err := normalizeAndValidateAIExecuteTargetType(req.TargetType)
	if err != nil {
		http.Error(w, "Invalid target_type (allowed: agent, system-container, vm)", http.StatusBadRequest)
		return
	}

	// Validate target_id length if provided
	if len(req.TargetID) > 256 {
		http.Error(w, "target_id exceeds maximum length", http.StatusBadRequest)
		return
	}

	// Validate conversation history roles
	for _, msg := range req.History {
		switch msg.Role {
		case "user", "assistant":
			// valid
		default:
			http.Error(w, "Invalid role in history (allowed: user, assistant)", http.StatusBadRequest)
			return
		}
	}

	log.Info().
		Int("prompt_len", len(req.Prompt)).
		Str("target_type", targetType).
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

	// Keep the historical background execution behavior for this legacy route,
	// but expose transport progress through the same visible idle event contract
	// as /api/ai/chat.
	streamWriter := newLegacyAssistantSSEWriter(w, flusher, rc, cancel, false)
	streamWriter.start(ctx)
	defer streamWriter.stop()

	// Stream callback - write SSE events
	callback := func(event ai.StreamEvent) {
		// Skip the 'done' event from service - we'll send our own at the end
		// This ensures 'complete' comes before 'done'
		if event.Type == "done" {
			log.Debug().Msg("Skipping service 'done' event - will send final 'done' after 'complete'")
			return
		}

		log.Debug().
			Str("event_type", event.Type).
			Msg("Streaming AI event")

		if event.Type == "error" {
			streamWriter.writeTerminalPayload(event)
			return
		}
		streamWriter.writePayload(event)
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
		if !streamWriter.disconnected() {
			doneEvent := ai.StreamEvent{Type: "done"}
			streamWriter.writeTerminalPayload(doneEvent)
			log.Debug().Msg("Sent final 'done' event")
		}
	}()

	// Execute with streaming
	resp, err := h.GetAIService(r.Context()).ExecuteStream(ctx, ai.ExecuteRequest{
		Prompt:     req.Prompt,
		TargetType: targetType,
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
		streamWriter.writeTerminalPayload(errEvent)
		return
	}

	log.Info().
		Str("model", resp.Model).
		Int("input_tokens", resp.InputTokens).
		Int("output_tokens", resp.OutputTokens).
		Int("tool_calls", len(resp.ToolCalls)).
		Msg("AI streaming request completed")

	// Send final response with metadata (before 'done')
	finalEvent := aiExecuteStreamCompleteEvent{
		Type:         "complete",
		Model:        resp.Model,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		ToolCalls:    resp.ToolCalls,
	}.NormalizeCollections()
	streamWriter.writeTerminalPayload(finalEvent)
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

// HandleRunCommand answers POST /api/ai/run-command. Raw command execution is
// retired (L20 action governance): the endpoint always responds 410 Gone and
// never consumes approvals. Callers must use advertised typed resource actions.
func (h *AISettingsHandler) HandleRunCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSONError(w, http.StatusGone, agentcapabilities.AgentErrCodeRawCommandRetired, "Raw command execution is retired. Use an advertised typed resource action.")
}

func normalizeRunCommandApprovalTarget(req AIRunCommandRequest) (string, string, error) {
	targetType := normalizeAIExecuteTargetType(req.TargetType)
	targetID := strings.TrimSpace(req.TargetID)
	targetHost := strings.ToLower(strings.TrimSpace(req.TargetHost))

	if req.RunOnHost {
		targetType = agentcapabilities.ActionTargetTypeAgent
		if targetHost == "" {
			return "", "", errors.New("target_host is required when run_on_host is true")
		}
		targetID = targetHost
	}

	if targetType == "" {
		targetType = agentcapabilities.ActionTargetTypeAgent
	}
	if !agentcapabilities.IsActionTargetType(targetType) {
		return "", "", fmt.Errorf("unsupported target_type %q (allowed: %s)", targetType, agentcapabilities.ActionTargetTypeAllowedDescription())
	}

	if (targetType == agentcapabilities.ActionTargetTypeSystemContainer || targetType == agentcapabilities.ActionTargetTypeVM) && strings.TrimSpace(req.VMID) != "" {
		targetID = strings.TrimSpace(req.VMID)
	}

	if targetType == agentcapabilities.ActionTargetTypeAgent {
		if targetID == "" {
			targetID = targetHost
		}
		if targetID == "" {
			return "", "", errors.New("target_id or target_host is required for agent commands")
		}
		targetID = strings.ToLower(targetID)
	}

	if targetID == "" {
		return "", "", fmt.Errorf("target_id is required for target_type '%s'", targetType)
	}

	return targetType, targetID, nil
}

// maxGuestIDLength is the maximum allowed length for guest_id across all
// knowledge endpoints, preventing abuse via oversized identifiers.
const maxGuestIDLength = 256

// maxImportNotes is the maximum number of notes allowed in a single import
// request. This prevents abuse even within the 1MB body size limit.
const maxImportNotes = 500

// sanitizeFilenameComponent strips characters unsafe for Content-Disposition
// filenames. Only alphanumeric, hyphens, underscores, and dots are kept.
func sanitizeFilenameComponent(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			b.WriteRune(c)
		}
	}
	result := b.String()
	if result == "" {
		return "export"
	}
	return result
}

// HandleGetGuestKnowledge returns all notes for a guest
func (h *AISettingsHandler) HandleGetGuestKnowledge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	guestID := r.URL.Query().Get("guest_id")
	if guestID == "" {
		http.Error(w, "guest_id is required", http.StatusBadRequest)
		return
	}
	if len(guestID) > maxGuestIDLength {
		http.Error(w, "guest_id too long", http.StatusBadRequest)
		return
	}

	knowledge, err := h.GetAIService(r.Context()).GetGuestKnowledge(guestID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Failed to get knowledge"), http.StatusInternalServerError)
		return
	}

	if err := utils.WriteJSONResponse(w, knowledge); err != nil {
		log.Error().Err(err).Msg("Failed to write knowledge response")
	}
}

// HandleSaveGuestNote saves a note for a guest
func (h *AISettingsHandler) HandleSaveGuestNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64KB max

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
	if len(req.GuestID) > maxGuestIDLength {
		http.Error(w, "guest_id too long", http.StatusBadRequest)
		return
	}
	if len(req.Category) > 128 {
		http.Error(w, "category too long", http.StatusBadRequest)
		return
	}
	if len(req.Title) > 1024 {
		http.Error(w, "title too long", http.StatusBadRequest)
		return
	}
	if len(req.Content) > 32*1024 {
		http.Error(w, "content too long", http.StatusBadRequest)
		return
	}

	if err := h.GetAIService(r.Context()).SaveGuestNote(req.GuestID, req.GuestName, req.GuestType, req.Category, req.Title, req.Content); err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Failed to save note"), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true}`))
}

// HandleDeleteGuestNote deletes a note from a guest
func (h *AISettingsHandler) HandleDeleteGuestNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4*1024) // 4KB max — only needs guest_id + note_id

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
	if len(req.GuestID) > maxGuestIDLength {
		http.Error(w, "guest_id too long", http.StatusBadRequest)
		return
	}
	if len(req.NoteID) > maxGuestIDLength {
		http.Error(w, "note_id too long", http.StatusBadRequest)
		return
	}

	if err := h.GetAIService(r.Context()).DeleteGuestNote(req.GuestID, req.NoteID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Note not found", http.StatusNotFound)
			return
		}
		http.Error(w, sanitizeErrorForClient(err, "Failed to delete note"), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true}`))
}

// HandleExportGuestKnowledge exports all knowledge for a guest as JSON
func (h *AISettingsHandler) HandleExportGuestKnowledge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	guestID := r.URL.Query().Get("guest_id")
	if guestID == "" {
		http.Error(w, "guest_id is required", http.StatusBadRequest)
		return
	}
	if len(guestID) > maxGuestIDLength {
		http.Error(w, "guest_id too long", http.StatusBadRequest)
		return
	}

	knowledge, err := h.GetAIService(r.Context()).GetGuestKnowledge(guestID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Failed to get knowledge"), http.StatusInternalServerError)
		return
	}

	// Sanitize guestID for Content-Disposition header to prevent header injection.
	// Only allow alphanumeric, hyphens, underscores, and dots.
	safeID := sanitizeFilenameComponent(guestID)

	// Set headers for file download
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"pulse-notes-"+safeID+".json\"")

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
		http.Error(w, "Invalid import data", http.StatusBadRequest)
		return
	}

	if importData.GuestID == "" {
		http.Error(w, "guest_id is required in import data", http.StatusBadRequest)
		return
	}
	if len(importData.GuestID) > maxGuestIDLength {
		http.Error(w, "guest_id too long", http.StatusBadRequest)
		return
	}

	if len(importData.Notes) == 0 {
		http.Error(w, "No notes to import", http.StatusBadRequest)
		return
	}
	if len(importData.Notes) > maxImportNotes {
		http.Error(w, fmt.Sprintf("Too many notes: %d exceeds maximum of %d", len(importData.Notes), maxImportNotes), http.StatusBadRequest)
		return
	}

	// Pre-filter valid notes before deleting in replace mode to avoid data loss
	// when all incoming notes fail validation.
	isValidNote := func(n struct {
		Category string `json:"category"`
		Title    string `json:"title"`
		Content  string `json:"content"`
	}) bool {
		if n.Category == "" || n.Title == "" || n.Content == "" {
			return false
		}
		if len(n.Category) > 128 || len(n.Title) > 1024 || len(n.Content) > 32*1024 {
			return false
		}
		return true
	}

	validCount := 0
	for _, note := range importData.Notes {
		if isValidNote(note) {
			validCount++
		}
	}
	if validCount == 0 {
		http.Error(w, "No valid notes to import", http.StatusBadRequest)
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
		if !isValidNote(note) {
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

	r.Body = http.MaxBytesReader(w, r.Body, 4*1024) // 4KB max — only needs guest_id + confirm

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
	if len(req.GuestID) > maxGuestIDLength {
		http.Error(w, "guest_id too long", http.StatusBadRequest)
		return
	}

	if !req.Confirm {
		http.Error(w, "confirm must be true to clear all notes", http.StatusBadRequest)
		return
	}

	// Get existing knowledge and delete all notes
	existing, err := h.GetAIService(r.Context()).GetGuestKnowledge(req.GuestID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Failed to get knowledge"), http.StatusInternalServerError)
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
// This is useful for debugging when AI can't reach certain agents
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
		"note":   "Agents connect via WebSocket to /api/agent/ws. If an agent is missing, check that pulse-agent is installed and can reach the Pulse server.",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AIInvestigateAlertRequest is the request body for POST /api/ai/investigate-alert
type AIInvestigateAlertRequest struct {
	AlertIdentifier string  `json:"alertIdentifier"`
	ResourceID      string  `json:"resource_id"`
	ResourceName    string  `json:"resource_name"`
	ResourceType    string  `json:"resource_type"` // canonical v6 resource type
	AlertType       string  `json:"alert_type"`    // cpu, memory, disk, offline, etc.
	Level           string  `json:"level"`         // warning, critical
	Value           float64 `json:"value"`
	Threshold       float64 `json:"threshold"`
	Message         string  `json:"message"`
	Duration        string  `json:"duration"` // How long the alert has been active
	Node            string  `json:"node,omitempty"`
	VMID            int     `json:"vmid,omitempty"`
}

func (r AIInvestigateAlertRequest) alertIdentifier() string {
	return strings.TrimSpace(r.AlertIdentifier)
}

func normalizeInvestigateAlertTargetType(raw string) (string, error) {
	return agentcapabilities.ActionTargetTypeForResourceType(raw)
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
	alertIdentifier := req.alertIdentifier()
	investigationPrompt := ai.GenerateAlertInvestigationPrompt(ai.AlertInvestigationRequest{
		AlertIdentifier: alertIdentifier,
		ResourceID:      req.ResourceID,
		ResourceName:    req.ResourceName,
		ResourceType:    req.ResourceType,
		AlertType:       req.AlertType,
		Level:           req.Level,
		Value:           req.Value,
		Threshold:       req.Threshold,
		Message:         req.Message,
		Duration:        req.Duration,
		Node:            req.Node,
		VMID:            req.VMID,
	})

	log.Info().
		Str("alert_identifier", alertIdentifier).
		Str("resource", req.ResourceName).
		Str("type", req.AlertType).
		Msg("AI alert investigation started")

	targetType, err := normalizeInvestigateAlertTargetType(req.ResourceType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resourceType := strings.ToLower(strings.TrimSpace(req.ResourceType))
	targetID := strings.TrimSpace(req.ResourceID)
	if targetType == "vm" || targetType == "system-container" {
		if req.VMID > 0 {
			targetID = strconv.Itoa(req.VMID)
		}
	} else if targetType == "agent" {
		if node := strings.ToLower(strings.TrimSpace(req.Node)); node != "" {
			targetID = node
		} else if resourceType != "app-container" {
			// Keep explicit host-like IDs for node/agent resources.
			targetID = strings.ToLower(strings.TrimSpace(req.ResourceID))
		} else {
			// app-container IDs are container-scoped, not host routing IDs.
			targetID = ""
		}
	}

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

	streamWriter := newLegacyAssistantSSEWriter(w, flusher, rc, cancel, false)
	streamWriter.start(ctx)
	defer streamWriter.stop()

	// Stream callback
	callback := func(event ai.StreamEvent) {
		if event.Type == "done" {
			return
		}
		if event.Type == "error" {
			streamWriter.writeTerminalPayload(event)
			return
		}
		streamWriter.writePayload(event)
	}

	// Execute with streaming
	defer func() {
		if !streamWriter.disconnected() {
			doneEvent := ai.StreamEvent{Type: "done"}
			streamWriter.writeTerminalPayload(doneEvent)
		}
	}()

	autonomousMode := false
	resp, err := h.GetAIService(r.Context()).ExecuteStream(ctx, ai.ExecuteRequest{
		Prompt:                 investigationPrompt,
		TargetType:             targetType,
		TargetID:               targetID,
		AutonomousMode:         &autonomousMode,
		RequireCommandApproval: true,
		Context: map[string]interface{}{
			"alertIdentifier": alertIdentifier,
			"alertType":       req.AlertType,
			"alertLevel":      req.Level,
			"alertMessage":    req.Message,
			"guestName":       req.ResourceName,
			"node":            req.Node,
		},
	}, callback)

	if err != nil {
		log.Error().Err(err).Msg("AI alert investigation failed")
		errEvent := ai.StreamEvent{Type: "error", Data: map[string]string{"message": "Alert investigation failed. Please try again."}}
		streamWriter.writeTerminalPayload(errEvent)
		return
	}

	// Send completion event
	finalEvent := aiExecuteStreamCompleteEvent{
		Type:         "complete",
		Model:        resp.Model,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		ToolCalls:    resp.ToolCalls,
	}.NormalizeCollections()
	streamWriter.writeTerminalPayload(finalEvent)

	if alertIdentifier != "" {
		h.GetAIService(r.Context()).RecordIncidentAnalysis(alertIdentifier, "Pulse Assistant alert investigation completed", map[string]interface{}{
			"model":         resp.Model,
			"tool_calls":    len(resp.ToolCalls),
			"input_tokens":  resp.InputTokens,
			"output_tokens": resp.OutputTokens,
		})
	}

	log.Info().
		Str("alert_identifier", alertIdentifier).
		Str("model", resp.Model).
		Int("tool_calls", len(resp.ToolCalls)).
		Msg("AI alert investigation completed")

	if alertIdentifier != "" {
		h.GetAIService(r.Context()).RecordIncidentAnalysis(alertIdentifier, "Pulse Assistant investigation completed", map[string]interface{}{
			"model":         resp.Model,
			"input_tokens":  resp.InputTokens,
			"output_tokens": resp.OutputTokens,
			"tool_calls":    len(resp.ToolCalls),
		})
	}
}

// SetAlertProvider sets the alert provider for AI context
// Sets on both the default-org service and all tenant services to ensure multi-tenant support.
func (h *AISettingsHandler) SetAlertProvider(ap ai.AlertProvider) {
	h.defaultAIService.SetAlertProvider(ap)

	h.aiServicesMu.RLock()
	defer h.aiServicesMu.RUnlock()
	for _, svc := range h.aiServices {
		svc.SetAlertProvider(ap)
	}
}

// SetAlertResolver sets the alert resolver for AI Patrol autonomous alert management
// Sets on both the default-org service and all tenant services to ensure multi-tenant support.
func (h *AISettingsHandler) SetAlertResolver(resolver ai.AlertResolver) {
	h.defaultAIService.SetAlertResolver(resolver)

	h.aiServicesMu.RLock()
	defer h.aiServicesMu.RUnlock()
	for _, svc := range h.aiServices {
		svc.SetAlertResolver(resolver)
	}
}

const unsupportedAnthropicOAuthMessage = "Anthropic subscription OAuth is not supported by Pulse Assistant. Configure an Anthropic API key or another supported provider."

func writeAnthropicOAuthUnsupported(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"success": false,
		"error":   "unsupported_anthropic_oauth",
		"message": unsupportedAnthropicOAuthMessage,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write unsupported Anthropic OAuth response")
	}
}

// HandleOAuthStart rejects legacy Anthropic subscription OAuth setup.
func (h *AISettingsHandler) HandleOAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeAnthropicOAuthUnsupported(w)
}

// HandleOAuthExchange rejects legacy Anthropic subscription OAuth setup.
func (h *AISettingsHandler) HandleOAuthExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeAnthropicOAuthUnsupported(w)
}

// HandleOAuthCallback rejects stale legacy OAuth redirects without saving tokens.
func (h *AISettingsHandler) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	errParam := r.URL.Query().Get("error")
	if errParam != "" {
		log.Info().Str("error", errParam).Msg("Unsupported Anthropic OAuth callback returned provider error")
		http.Redirect(w, r, "/settings?ai_oauth_error="+url.QueryEscape(errParam), http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, "/settings?ai_oauth_error=unsupported", http.StatusTemporaryRedirect)
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
	persistence := h.getPersistence(r.Context())
	if persistence == nil {
		log.Error().Msg("No persistence available for OAuth disconnect")
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}
	settings, err := h.loadAIConfig(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to load Pulse Intelligence settings for OAuth disconnect")
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
	if err := persistence.SaveAIConfig(*settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings after OAuth disconnect")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	// Reload the AI service
	if svc := h.GetAIService(r.Context()); svc != nil {
		if err := svc.Reload(); err != nil {
			log.Warn().Err(err).Msg("Failed to reload AI service after OAuth disconnect")
		}
	}

	log.Info().Msg("Legacy Anthropic OAuth disconnected")

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
	RuntimeState     ai.PatrolRuntimeState `json:"runtime_state"`
	Running          bool                  `json:"running"`
	Enabled          bool                  `json:"enabled"`
	LastPatrolAt     *time.Time            `json:"last_patrol_at,omitempty"`
	LastActivityAt   *time.Time            `json:"last_activity_at,omitempty"`
	TriggerStatus    *ai.TriggerStatus     `json:"trigger_status,omitempty"`
	NextPatrolAt     *time.Time            `json:"next_patrol_at,omitempty"`
	LastDurationMs   int64                 `json:"last_duration_ms"`
	ResourcesChecked int                   `json:"resources_checked"`
	FindingsCount    int                   `json:"findings_count"`
	ErrorCount       int                   `json:"error_count"`
	Healthy          bool                  `json:"healthy"`
	IntervalMs       int64                 `json:"interval_ms"` // Patrol interval in milliseconds
	FixedCount       int                   `json:"fixed_count"` // Number of issues remediated by Patrol
	BlockedReason    string                `json:"blocked_reason,omitempty"`
	BlockedCause     string                `json:"blocked_cause,omitempty"`
	BlockedAt        *time.Time            `json:"blocked_at,omitempty"`
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
	// Trust is a snapshot of how currently-tracked findings have resolved.
	// Operators use this to scan whether Patrol's analysis has been useful:
	// fix-verified and dismissed-as-noise grow as the system gets sharper.
	// Snapshot semantics, not lifetime totals; nil when no patrol service.
	Trust     *ai.FindingsTrustSummary `json:"trust,omitempty"`
	Readiness *PatrolReadinessResponse `json:"readiness,omitempty"`
}

type PatrolReadinessResponse struct {
	Status   string                 `json:"status"`
	Ready    bool                   `json:"ready"`
	Cause    string                 `json:"cause,omitempty"`
	Summary  string                 `json:"summary"`
	Provider string                 `json:"provider,omitempty"`
	Model    string                 `json:"model,omitempty"`
	Checks   []PatrolReadinessCheck `json:"checks"`
}

type PatrolReadinessCheck struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Cause   string `json:"cause,omitempty"`
	Label   string `json:"label"`
	Message string `json:"message"`
	Action  string `json:"action,omitempty"`
}

const (
	patrolReadinessReady    = ai.PatrolReadinessReady
	patrolReadinessWarning  = ai.PatrolReadinessWarning
	patrolReadinessNotReady = ai.PatrolReadinessNotReady

	defaultResolvedPatrolFindingsLimit = 200
	maxPatrolFindingsLimit             = 500
)

func (h *AISettingsHandler) buildPatrolReadiness(ctx context.Context, aiService *ai.Service, patrolAvailable bool) PatrolReadinessResponse {
	checks := make([]PatrolReadinessCheck, 0, 4)
	addCheck := func(id, status string, cause ai.PatrolFailureCause, label, message, action string) {
		checks = append(checks, PatrolReadinessCheck{
			ID:      id,
			Status:  status,
			Cause:   patrolFailureCauseResponse(cause),
			Label:   label,
			Message: message,
			Action:  action,
		})
	}

	if aiService == nil {
		addCheck("service", patrolReadinessNotReady, ai.PatrolFailureCauseServiceUnavailable, "Assistant runtime service", "Pulse Assistant runtime service is not available.", "restart_service")
		return summarizePatrolReadiness("", "", checks)
	}
	if !patrolAvailable {
		addCheck("service", patrolReadinessNotReady, ai.PatrolFailureCauseServiceUnavailable, "Patrol service", "Pulse Patrol service is not available.", "restart_service")
		return summarizePatrolReadiness("", "", checks)
	}
	addCheck("service", patrolReadinessReady, ai.PatrolFailureCauseNone, "Patrol service", "Pulse Patrol service is available.", "")

	if ai.IsDemoMode() {
		// Demo/mock runtimes simulate Patrol's provider path end to end, so
		// the provider-dependent checks report a simulated pass instead of
		// steering visitors into provider setup.
		addCheck("settings", patrolReadinessReady, ai.PatrolFailureCauseNone, "Settings persistence", "Demo mode uses the built-in demo dataset.", "")
		addCheck("enabled", patrolReadinessReady, ai.PatrolFailureCauseNone, "Assistant enabled", "Demo mode simulates Pulse Assistant; no provider key is required.", "")
		addCheck("provider", patrolReadinessReady, ai.PatrolFailureCauseNone, "Provider configured", "Demo mode uses the simulated demo provider.", "")
		addCheck("model", patrolReadinessReady, ai.PatrolFailureCauseNone, "Patrol model", "Demo mode uses the simulated demo model.", "")
		addCheck("tools", patrolReadinessReady, ai.PatrolFailureCauseNone, "Patrol tools", "Demo mode simulates Patrol's tool-backed analysis.", "")
		return summarizePatrolReadiness(ai.DemoPatrolProvider, ai.DemoPatrolModel, checks)
	}

	cfg, err := h.loadAIConfig(ctx)
	if err != nil || cfg == nil {
		addCheck("settings", patrolReadinessNotReady, ai.PatrolFailureCauseSettingsPersistence, "Settings persistence", "Pulse Intelligence settings could not be loaded from persistence.", "open_provider_settings")
		return summarizePatrolReadiness("", "", checks)
	}
	addCheck("settings", patrolReadinessReady, ai.PatrolFailureCauseNone, "Settings persistence", "Pulse Intelligence settings are readable.", "")

	if !cfg.Enabled {
		addCheck("enabled", patrolReadinessNotReady, ai.PatrolFailureCauseAssistantDisabled, "Assistant enabled", "Pulse Intelligence is turned off, so Patrol cannot run.", "open_provider_settings")
	} else {
		addCheck("enabled", patrolReadinessReady, ai.PatrolFailureCauseNone, "Assistant enabled", "Pulse Assistant is enabled for Patrol verification.", "")
	}

	if !cfg.IsConfigured() {
		addCheck("provider", patrolReadinessNotReady, ai.PatrolFailureCauseProviderNotConfigured, "Provider configured", "No AI provider is configured yet. Add a provider API key or an Ollama server on the Provider & Models settings page.", "open_provider_settings")
		return summarizePatrolReadiness("", "", checks)
	}
	addCheck("provider", patrolReadinessReady, ai.PatrolFailureCauseNone, "Provider configured", "At least one AI provider is configured.", "")

	model := strings.TrimSpace(cfg.GetPatrolModel())
	if model == "" {
		model = strings.TrimSpace(cfg.GetChatModel())
	}
	provider, _ := config.ParseModelString(model)
	if model == "" || provider == "" || provider == config.AIProviderQuickstart {
		addCheck("model", patrolReadinessNotReady, ai.PatrolFailureCauseModelNotSelected, "Patrol model", "No concrete Patrol model is selected.", "open_provider_settings")
		return summarizePatrolReadiness(provider, model, checks)
	}
	if !cfg.HasProvider(provider) {
		addCheck("model", patrolReadinessNotReady, ai.PatrolFailureCauseModelProviderUnconfigured, "Patrol model", fmt.Sprintf("The selected Patrol model uses %s, but that provider is not configured.", provider), "open_provider_settings")
		return summarizePatrolReadiness(provider, model, checks)
	}
	addCheck("model", patrolReadinessReady, ai.PatrolFailureCauseNone, "Patrol model", "Patrol has a model selected from a configured provider.", "")

	toolStatus, toolCause, toolMessage, toolAction := resolvePatrolToolsCheck(aiService, provider, model)
	addCheck("tools", toolStatus, toolCause, "Patrol tools", toolMessage, toolAction)

	return summarizePatrolReadiness(provider, model, checks)
}

// resolvePatrolToolsCheck grounds the "Patrol tools" readiness check in
// actual evidence when the preflight cache holds a recent result for
// the configured provider+model. The cache is populated by
// (a) auto-trigger on settings save when transport changes,
// (b) startup-seed when assistant + patrol model are configured, and
// (c) manual Verify Patrol clicks. When no cached evidence is
// available, fall back to the static `PatrolToolReadinessForModel`
// classifier so the check still has a meaningful baseline.
func resolvePatrolToolsCheck(aiService *ai.Service, provider, model string) (status string, cause ai.PatrolFailureCause, message, action string) {
	staticStatus, staticCause, staticMessage := ai.PatrolToolReadinessForModel(provider, model)
	staticAction := ""
	if staticStatus != patrolReadinessReady {
		staticAction = "open_provider_settings"
	}

	if aiService == nil {
		return staticStatus, staticCause, staticMessage, staticAction
	}
	cached, recordedAt := aiService.CachedPatrolPreflight()
	if cached == nil || recordedAt.IsZero() {
		return staticStatus, staticCause, staticMessage, staticAction
	}
	// Cache only carries the most recent result regardless of which
	// model was tested, so a mismatch means the configured model
	// hasn't been preflighted yet. Fall back to static.
	_, bareModel := config.ParseModelString(model)
	if !strings.EqualFold(cached.Provider, provider) || cached.Model != bareModel {
		return staticStatus, staticCause, staticMessage, staticAction
	}

	age := formatPatrolPreflightAge(time.Since(recordedAt))
	if cached.Success && cached.ToolCallObserved {
		return patrolReadinessReady, ai.PatrolFailureCauseNone,
			fmt.Sprintf("Tool calling verified %s against %s.", age, bareModel), ""
	}
	// Cached failure or soft warning — surface the classified summary
	// so the Patrol page reads the same diagnostic the operator would
	// see on the AI settings preflight panel.
	resolvedStatus := patrolReadinessNotReady
	if cached.Cause == ai.PatrolFailureCauseModelToolSupportUnverified {
		resolvedStatus = patrolReadinessWarning
	}
	msg := strings.TrimSpace(cached.Summary)
	if msg == "" {
		msg = strings.TrimSpace(cached.Title)
	}
	if msg == "" {
		msg = staticMessage
	}
	return resolvedStatus, cached.Cause, fmt.Sprintf("%s (last preflight %s).", msg, age), "open_provider_settings"
}

func formatPatrolPreflightAge(age time.Duration) string {
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(age.Hours()/24))
	}
}

func summarizePatrolReadiness(provider, model string, checks []PatrolReadinessCheck) PatrolReadinessResponse {
	status := patrolReadinessReady
	cause := ""
	summary := "Patrol is ready to run tool-backed verification."
	for _, check := range checks {
		if check.Status == patrolReadinessNotReady {
			status = patrolReadinessNotReady
			cause = check.Cause
			summary = check.Message
			break
		}
		if check.Status == patrolReadinessWarning && status == patrolReadinessReady {
			status = patrolReadinessWarning
			cause = check.Cause
			summary = check.Message
		}
	}

	return PatrolReadinessResponse{
		Status:   status,
		Ready:    status != patrolReadinessNotReady,
		Cause:    cause,
		Summary:  summary,
		Provider: provider,
		Model:    model,
		Checks:   checks,
	}
}

func patrolFailureCauseResponse(cause ai.PatrolFailureCause) string {
	if cause == "" || cause == ai.PatrolFailureCauseNone {
		return ""
	}
	return string(cause)
}

func patrolReadinessResponseFromConfigReadiness(readiness ai.PatrolConfigReadiness) PatrolReadinessResponse {
	action := ""
	if !readiness.Ready {
		action = "open_provider_settings"
	}
	cause := patrolFailureCauseResponse(readiness.Cause)
	return PatrolReadinessResponse{
		Status:   readiness.Status,
		Ready:    readiness.Ready,
		Cause:    cause,
		Summary:  readiness.Summary,
		Provider: readiness.Provider,
		Model:    readiness.Model,
		Checks: []PatrolReadinessCheck{
			{
				ID:      "configuration",
				Status:  readiness.Status,
				Cause:   cause,
				Label:   "Patrol control",
				Message: readiness.Summary,
				Action:  action,
			},
		},
	}
}

func ptrToPatrolReadiness(readiness PatrolReadinessResponse) *PatrolReadinessResponse {
	return &readiness
}

func (h *AISettingsHandler) getPatrolService(ctx context.Context) *ai.PatrolService {
	aiService := h.GetAIService(ctx)
	if aiService == nil {
		return nil
	}
	return aiService.GetPatrolService()
}

func writePatrolServiceUnavailableResponse(w http.ResponseWriter) {
	writeErrorResponse(w, http.StatusServiceUnavailable, "service_unavailable", "Pulse Patrol service not available", nil)
}

func writePatrolAgentUnavailableResponse(w http.ResponseWriter) {
	writeJSONError(w, http.StatusServiceUnavailable, agentcapabilities.AgentErrCodePatrolUnavailable, "Pulse Patrol service not available")
}

func writeFindingInvalidRequestResponse(w http.ResponseWriter, message string, details map[string]string) {
	writeJSONErrorWithDetails(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidFindingRequest, message, details)
}

func writeFindingNotFoundResponse(w http.ResponseWriter, message string) {
	writeJSONError(w, http.StatusNotFound, agentcapabilities.AgentErrCodeFindingNotFound, message)
}

func writeFindingActionNotAllowedResponse(w http.ResponseWriter, message string) {
	writeJSONError(w, http.StatusConflict, agentcapabilities.AgentErrCodeFindingActionNotAllowed, message)
}

func writePatrolReadinessNotReadyResponse(w http.ResponseWriter, statusCode int, readiness ai.PatrolConfigReadiness) {
	details := map[string]string{"status": readiness.Status}
	if cause := patrolFailureCauseResponse(readiness.Cause); cause != "" {
		details["cause"] = cause
	}
	if readiness.Provider != "" {
		details["provider"] = readiness.Provider
	}
	if readiness.Model != "" {
		details["model"] = readiness.Model
	}
	writeErrorResponse(w, statusCode, "patrol_readiness_not_ready", readiness.Summary, details)
}

func retireQuickstartPatrolStatus(status ai.PatrolStatus) ai.PatrolStatus {
	if isRetiredQuickstartBlockedReason(status.BlockedReason) {
		status.BlockedReason = ""
		status.BlockedAt = nil
		if status.RuntimeState == ai.PatrolRuntimeStateBlocked {
			if status.Enabled {
				status.RuntimeState = ai.PatrolRuntimeStateActive
			} else {
				status.RuntimeState = ai.PatrolRuntimeStateUnavailable
			}
			status.Healthy = true
		}
	}
	return status
}

func isRetiredQuickstartBlockedReason(reason string) bool {
	normalized := strings.ToLower(strings.TrimSpace(reason))
	return strings.Contains(normalized, "quickstart credits exhausted") ||
		strings.Contains(normalized, "hosted quickstart requires")
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
			RuntimeState:    ai.PatrolRuntimeStateUnavailable,
			Running:         false,
			Enabled:         false,
			Healthy:         true,
			LicenseRequired: true,
			LicenseStatus:   "none",
			UpgradeURL:      upgradeURLForFeatureFromLicensing(featureAIAutoFixValue),
			Readiness:       ptrToPatrolReadiness(h.buildPatrolReadiness(r.Context(), nil, false)),
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
		hasAutoFixFeature := aiService.HasLicenseFeature(featureAIAutoFixValue)
		response := PatrolStatusResponse{
			RuntimeState:    ai.PatrolRuntimeStateUnavailable,
			Running:         false,
			Enabled:         false,
			Healthy:         true,
			LicenseRequired: !hasAutoFixFeature,
			LicenseStatus:   licenseStatus,
			Readiness:       ptrToPatrolReadiness(h.buildPatrolReadiness(r.Context(), aiService, false)),
		}
		if !hasAutoFixFeature {
			response.UpgradeURL = upgradeURLForFeatureFromLicensing(featureAIAutoFixValue)
		}
		if err := utils.WriteJSONResponse(w, response); err != nil {
			log.Error().Err(err).Msg("Failed to write patrol status response")
		}
		return
	}

	status := retireQuickstartPatrolStatus(patrol.GetStatus())
	summary := patrol.GetFindingsSummary()

	// Determine license status for Pro feature gating
	// GetLicenseState returns accurate state: none, active, expired, grace_period
	licenseStatus, _ := aiService.GetLicenseState()
	// Check for auto-fix feature - patrol itself is free, auto-fix requires Pro
	hasAutoFixFeature := aiService.HasLicenseFeature(featureAIAutoFixValue)

	// Get fixed count from investigation orchestrator
	fixedCount := 0
	if orchestrator := patrol.GetInvestigationOrchestrator(); orchestrator != nil {
		fixedCount = orchestrator.GetFixedCount()
	}

	response := PatrolStatusResponse{
		RuntimeState:     status.RuntimeState,
		Running:          status.Running,
		Enabled:          status.Enabled,
		LastPatrolAt:     status.LastPatrolAt,
		LastActivityAt:   status.LastActivityAt,
		TriggerStatus:    status.TriggerStatus,
		NextPatrolAt:     status.NextPatrolAt,
		LastDurationMs:   status.LastDuration.Milliseconds(),
		ResourcesChecked: status.ResourcesChecked,
		FindingsCount:    status.FindingsCount,
		ErrorCount:       status.ErrorCount,
		Healthy:          status.Healthy,
		IntervalMs:       status.IntervalMs,
		FixedCount:       fixedCount,
		BlockedReason:    status.BlockedReason,
		BlockedCause:     patrolFailureCauseResponse(status.BlockedCause),
		BlockedAt:        status.BlockedAt,
		LicenseRequired:  !hasAutoFixFeature,
		LicenseStatus:    licenseStatus,
		Readiness:        ptrToPatrolReadiness(h.buildPatrolReadiness(r.Context(), aiService, true)),
	}
	if !hasAutoFixFeature {
		response.UpgradeURL = upgradeURLForFeatureFromLicensing(featureAIAutoFixValue)
	}
	response.Summary.Critical = summary.Critical
	response.Summary.Warning = summary.Warning
	response.Summary.Watch = summary.Watch
	response.Summary.Info = summary.Info

	trust := patrol.GetFindingsTrustSummary()
	response.Trust = &trust

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

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		response := map[string]interface{}{
			"error": "Pulse Patrol service not available",
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := utils.WriteJSONResponse(w, response); err != nil {
			log.Error().Err(err).Msg("Failed to write intelligence response")
		}
		return
	}

	patrol := aiService.GetPatrolService()
	if patrol == nil {
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
		if len(resourceID) > 500 {
			http.Error(w, "resource_id too long", http.StatusBadRequest)
			return
		}
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
		writePatrolServiceUnavailableResponse(w)
		return
	}

	patrol := aiService.GetPatrolService()
	if patrol == nil {
		writePatrolServiceUnavailableResponse(w)
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
	// include_resolved=1 returns active + resolved + dismissed + snoozed
	// findings, so the Patrol UI's Resolved tab can audit the
	// auto_resolved set credited in the trust strip. Default behaviour
	// remains active-only for clients that just want to render the
	// live findings list.
	includeResolved := r.URL.Query().Get("include_resolved") == "1"
	var findings []*ai.Finding
	if resourceID != "" {
		findings = patrol.GetFindingsForResource(resourceID)
	} else if includeResolved {
		findings = patrol.GetAllFindingsIncludingResolved()
	} else {
		findings = patrol.GetAllFindings()
	}

	limit := patrolFindingsLimitFromQuery(r.URL.Query(), includeResolved)
	if limit > 0 && limit < len(findings) {
		findings = findings[:limit]
	}
	h.applyResourceCriticalityToPatrolFindings(r.Context(), findings)

	if err := utils.WriteJSONResponse(w, findings); err != nil {
		log.Error().Err(err).Msg("Failed to write patrol findings response")
	}
}

func (h *AISettingsHandler) applyResourceCriticalityToPatrolFindings(ctx context.Context, findings []*ai.Finding) {
	if h == nil || len(findings) == 0 {
		return
	}
	h.stateMu.RLock()
	provider := h.resourceStoreProvider
	h.stateMu.RUnlock()
	if provider == nil {
		return
	}
	store, err := provider(GetOrgID(ctx))
	if err != nil || store == nil {
		if err != nil {
			log.Debug().Err(err).Msg("Failed to resolve resource store for Patrol finding priority enrichment")
		}
		return
	}

	criticalityByResourceID := make(map[string]string)
	for _, finding := range findings {
		if finding == nil {
			continue
		}
		resourceID := strings.TrimSpace(finding.ResourceID)
		if resourceID == "" {
			continue
		}
		if _, exists := criticalityByResourceID[resourceID]; exists {
			continue
		}
		state, found, fetchErr := store.GetResourceOperatorState(resourceID)
		if fetchErr != nil {
			log.Debug().Err(fetchErr).Str("resource_id", resourceID).Msg("Failed to load resource operator priority for Patrol finding")
			continue
		}
		if !found {
			criticalityByResourceID[resourceID] = ""
			continue
		}
		criticalityByResourceID[resourceID] = normalizePatrolResourceCriticality(string(state.Criticality))
	}

	for _, finding := range findings {
		if finding == nil {
			continue
		}
		resourceID := strings.TrimSpace(finding.ResourceID)
		if criticality, ok := criticalityByResourceID[resourceID]; ok {
			finding.ResourceCriticality = criticality
		}
	}
}

func normalizePatrolResourceCriticality(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if !unifiedresources.IsValidCriticality(normalized) {
		return ""
	}
	return normalized
}

func patrolFindingsLimitFromQuery(query url.Values, includeResolved bool) int {
	limit := 0
	if includeResolved {
		limit = defaultResolvedPatrolFindingsLimit
	}
	if limitStr := query.Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed <= 0 {
			return limit
		}
		if parsed > maxPatrolFindingsLimit {
			return maxPatrolFindingsLimit
		}
		return parsed
	}
	return limit
}

// manualScopedPatrolRequest is the optional body for POST /api/ai/patrol/run.
// When resource_ids or resource_types are present, the run is scoped to those
// resources as a manual Targeted check over the same scoped engine the
// automatic alert path uses, instead of a fleet-wide Patrol check.
type manualScopedPatrolRequest struct {
	ResourceIDs     []string `json:"resource_ids,omitempty"`
	ResourceTypes   []string `json:"resource_types,omitempty"`
	AlertIdentifier string   `json:"alert_identifier,omitempty"`
	AlertType       string   `json:"alert_type,omitempty"`
	Context         string   `json:"context,omitempty"`
}

// buildManualScopedPatrolScope maps an optional scoped-run request body to a
// manual PatrolScope. It returns hasScope=false when no resource ids or types
// remain after trimming, so the caller falls back to a full fleet run.
func buildManualScopedPatrolScope(req manualScopedPatrolRequest) (ai.PatrolScope, bool) {
	resourceIDs := make([]string, 0, len(req.ResourceIDs))
	for _, id := range req.ResourceIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			resourceIDs = append(resourceIDs, trimmed)
		}
	}
	resourceTypes := make([]string, 0, len(req.ResourceTypes))
	for _, t := range req.ResourceTypes {
		if trimmed := strings.TrimSpace(t); trimmed != "" {
			resourceTypes = append(resourceTypes, trimmed)
		}
	}
	if len(resourceIDs) == 0 && len(resourceTypes) == 0 {
		return ai.PatrolScope{}, false
	}
	scope := ai.PatrolScope{
		ResourceIDs:     resourceIDs,
		ResourceTypes:   resourceTypes,
		Depth:           ai.PatrolDepthQuick,
		Reason:          ai.TriggerReasonManual,
		AlertIdentifier: strings.TrimSpace(req.AlertIdentifier),
	}
	scope.Context = "Manual targeted check"
	if alertType := strings.TrimSpace(req.AlertType); alertType != "" {
		scope.Context = "Manual targeted check for alert: " + alertType
	}
	if ctx := strings.TrimSpace(req.Context); ctx != "" {
		scope.Context = ctx
	}
	return scope, true
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
		writePatrolServiceUnavailableResponse(w)
		return
	}

	patrol := aiService.GetPatrolService()
	if patrol == nil {
		writePatrolServiceUnavailableResponse(w)
		return
	}
	if readiness := aiService.PatrolRuntimeReadiness(); !readiness.Ready {
		writePatrolReadinessNotReadyResponse(w, http.StatusConflict, readiness)
		return
	}

	// Optional scope: resource_ids/resource_types turn this into a manual
	// Targeted check over the same scoped engine the automatic alert path uses,
	// instead of a fleet-wide Patrol check. An empty or absent body keeps the
	// legacy full-run behaviour. Scoped runs bypass the full-run cadence gate
	// (consistent with automatic scoped runs) but still honour readiness above.
	var scopedReq manualScopedPatrolRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&scopedReq)
	}
	if scope, hasScope := buildManualScopedPatrolScope(scopedReq); hasScope {
		go patrol.TriggerScopedPatrol(context.WithoutCancel(r.Context()), scope)
		response := map[string]interface{}{
			"success": true,
			"message": "Triggered targeted Patrol check",
		}
		if err := utils.WriteJSONResponse(w, response); err != nil {
			log.Error().Err(err).Msg("Failed to write scoped patrol response")
		}
		return
	}

	// Cadence cap: Community tier is limited to 1 patrol run per hour.
	// Patrol itself is free (ai_patrol), but higher cadence is gated behind Pro/Cloud.
	if !aiService.HasLicenseFeature(featureAIAutoFixValue) {
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

// aiPatrolPreflightResponse is the JSON shape returned by
// HandlePatrolPreflight. It mirrors aiProviderTestResponse for the
// classified-error fields and adds two preflight-specific signals:
// ToolCallObserved (whether the model emitted a tool call) and
// DurationMs (round-trip latency, useful for spotting slow providers).
type aiPatrolPreflightResponse struct {
	Success          bool   `json:"success"`
	Provider         string `json:"provider,omitempty"`
	Model            string `json:"model,omitempty"`
	ToolCallObserved bool   `json:"tool_call_observed"`
	DurationMs       int64  `json:"duration_ms"`
	Message          string `json:"message"`
	Cause            string `json:"cause,omitempty"`
	Summary          string `json:"summary,omitempty"`
	Description      string `json:"description,omitempty"`
	Recommendation   string `json:"recommendation,omitempty"`
	Action           string `json:"action,omitempty"`
}

type aiPatrolPreflightRequest struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

// HandlePatrolPreflight runs a one-shot tool-call round-trip against the
// configured (or overridden) Patrol provider+model so operators can
// verify tool calling actually works before relying on Patrol's
// scheduled cadence (POST /api/ai/patrol/preflight).
//
// This is distinct from POST /api/ai/test/{provider}, which only lists
// models and therefore can pass while Patrol fails 100% of runs (the
// failure mode that bit Pulse for 33 days before the DeepSeek fix
// landed).
func (h *AISettingsHandler) HandlePatrolPreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !ensureSettingsWriteScope(h.getConfig(r.Context()), w, r) {
		return
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		writePatrolServiceUnavailableResponse(w)
		return
	}

	// Optional body: override the provider and/or model. Empty body is
	// allowed and means "use the configured Patrol provider+model".
	var body aiPatrolPreflightRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"Invalid JSON body"}`, http.StatusBadRequest)
			return
		}
	}
	if body.Provider != "" {
		if len(body.Provider) > 64 || !isValidProviderName(body.Provider) {
			http.Error(w, `{"error":"Invalid provider name"}`, http.StatusBadRequest)
			return
		}
	}
	if len(body.Model) > 256 {
		http.Error(w, `{"error":"Model id too long"}`, http.StatusBadRequest)
		return
	}

	// Tight timeout: preflight is a single round-trip, not a Patrol pass.
	// 30s matches the per-provider connection-test budget; in practice
	// this completes in well under 10s for any healthy provider.
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	preflight := aiService.RunPatrolToolPreflight(ctx, body.Provider, body.Model)

	response := aiPatrolPreflightResponse{
		Success:          preflight.Success && preflight.ToolCallObserved,
		Provider:         preflight.Provider,
		Model:            preflight.Model,
		ToolCallObserved: preflight.ToolCallObserved,
		DurationMs:       preflight.DurationMs,
		Message:          preflight.Summary,
		Cause:            string(preflight.Cause),
		Summary:          preflight.Description,
		Recommendation:   preflight.Recommendation,
	}
	if !response.Success {
		response.Action = "open_provider_settings"
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write Patrol preflight response")
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

	patrol := h.getPatrolService(r.Context())
	if patrol == nil {
		writePatrolAgentUnavailableResponse(w)
		return
	}

	var req struct {
		FindingID string `json:"finding_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFindingInvalidRequestResponse(w, "Invalid request body", nil)
		return
	}

	if req.FindingID == "" {
		writeFindingInvalidRequestResponse(w, "finding_id is required", map[string]string{"finding_id": "required"})
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
	unifiedStore := h.GetUnifiedStoreForOrg(GetOrgID(r.Context()))
	if !foundInPatrol && unifiedStore != nil {
		unifiedFinding := unifiedStore.Get(req.FindingID)
		if unifiedFinding != nil {
			detectedAt = unifiedFinding.DetectedAt
			category = string(unifiedFinding.Category)
			severity = string(unifiedFinding.Severity)
			resourceID = unifiedFinding.ResourceID
			findingKey = unifiedFinding.ID
		} else {
			writeFindingNotFoundResponse(w, "Finding not found")
			return
		}
	} else if !foundInPatrol {
		writeFindingNotFoundResponse(w, "Finding not found")
		return
	}
	if foundInPatrol {
		if err := patrol.RejectManualActionForRuntimeFinding(req.FindingID, "acknowledged"); err != nil {
			writeFindingActionNotAllowedResponse(w, err.Error())
			return
		}
	}

	// Acknowledge in patrol findings if it exists there
	if foundInPatrol {
		if !findings.Acknowledge(req.FindingID) {
			writeFindingNotFoundResponse(w, "Finding not found")
			return
		}
	}

	// Acknowledge in unified store (for both patrol and threshold alerts)
	if unifiedStore != nil {
		unifiedStore.Acknowledge(req.FindingID)
	}

	// Record to learning store
	if learningStore := h.GetLearningStoreForOrg(GetOrgID(r.Context())); learningStore != nil {
		learningStore.RecordFeedback(learning.FeedbackRecord{
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

	patrol := h.getPatrolService(r.Context())
	if patrol == nil {
		writePatrolAgentUnavailableResponse(w)
		return
	}

	var req struct {
		FindingID     string `json:"finding_id"`
		DurationHours int    `json:"duration_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFindingInvalidRequestResponse(w, "Invalid request body", nil)
		return
	}

	if req.FindingID == "" {
		writeFindingInvalidRequestResponse(w, "finding_id is required", map[string]string{"finding_id": "required"})
		return
	}

	if req.DurationHours <= 0 {
		writeFindingInvalidRequestResponse(w, "duration_hours must be positive", map[string]string{"duration_hours": "must be positive"})
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
	unifiedStore := h.GetUnifiedStoreForOrg(GetOrgID(r.Context()))
	if !foundInPatrol && unifiedStore != nil {
		unifiedFinding := unifiedStore.Get(req.FindingID)
		if unifiedFinding != nil {
			detectedAt = unifiedFinding.DetectedAt
			category = string(unifiedFinding.Category)
			severity = string(unifiedFinding.Severity)
			resourceID = unifiedFinding.ResourceID
			findingKey = unifiedFinding.ID
		} else {
			writeFindingNotFoundResponse(w, "Finding not found or already resolved")
			return
		}
	} else if !foundInPatrol {
		writeFindingNotFoundResponse(w, "Finding not found or already resolved")
		return
	}
	if foundInPatrol {
		if err := patrol.RejectManualActionForRuntimeFinding(req.FindingID, "snoozed"); err != nil {
			writeFindingActionNotAllowedResponse(w, err.Error())
			return
		}
	}

	// Snooze in patrol findings if it exists there
	if foundInPatrol {
		if !findings.Snooze(req.FindingID, duration) {
			writeFindingNotFoundResponse(w, "Finding not found or already resolved")
			return
		}
	}

	// Snooze in unified store (for both patrol and threshold alerts)
	if unifiedStore != nil {
		unifiedStore.Snooze(req.FindingID, duration)
	}

	// Record to learning store
	if learningStore := h.GetLearningStoreForOrg(GetOrgID(r.Context())); learningStore != nil {
		learningStore.RecordFeedback(learning.FeedbackRecord{
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

	patrol := h.getPatrolService(r.Context())
	if patrol == nil {
		writePatrolAgentUnavailableResponse(w)
		return
	}

	var req struct {
		FindingID      string `json:"finding_id"`
		ResolutionNote string `json:"resolution_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFindingInvalidRequestResponse(w, "Invalid request body", nil)
		return
	}

	req.ResolutionNote = strings.TrimSpace(req.ResolutionNote)

	if req.FindingID == "" {
		writeFindingInvalidRequestResponse(w, "finding_id is required", map[string]string{"finding_id": "required"})
		return
	}

	findings := patrol.GetFindings()

	// Get finding details before resolving (for learning/analytics)
	finding := findings.Get(req.FindingID)
	if finding == nil {
		writeFindingNotFoundResponse(w, "Finding not found or already resolved")
		return
	}
	if err := patrol.RejectManualActionForRuntimeFinding(req.FindingID, "resolved"); err != nil {
		writeFindingActionNotAllowedResponse(w, err.Error())
		return
	}
	if req.ResolutionNote != "" {
		findings.SetUserNote(req.FindingID, req.ResolutionNote)
	}

	// Capture details before action
	detectedAt := finding.DetectedAt
	category := string(finding.Category)
	severity := string(finding.Severity)
	resourceID := finding.ResourceID

	// Mark as manually resolved (auto=false since user did it)
	if !findings.Resolve(req.FindingID, false) {
		writeFindingNotFoundResponse(w, "Finding not found or already resolved")
		return
	}

	// Mirror into unified store for consistent UI state
	if store := h.GetUnifiedStoreForOrg(GetOrgID(r.Context())); store != nil {
		store.Resolve(req.FindingID)
	}

	// Record to learning store - manual resolve = user fixed the issue
	if learningStore := h.GetLearningStoreForOrg(GetOrgID(r.Context())); learningStore != nil {
		learningStore.RecordFeedback(learning.FeedbackRecord{
			FindingID:    req.FindingID,
			FindingKey:   finding.Key,
			ResourceID:   resourceID,
			Category:     category,
			Severity:     severity,
			Action:       learning.ActionQuickFix, // Manual resolve means user took action to fix
			TimeToAction: time.Since(detectedAt),
			UserNote:     req.ResolutionNote,
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

	patrol := h.getPatrolService(r.Context())
	if patrol == nil {
		writePatrolServiceUnavailableResponse(w)
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
	if unifiedStore := h.GetUnifiedStoreForOrg(GetOrgID(r.Context())); unifiedStore != nil {
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

	patrol := h.getPatrolService(r.Context())
	if patrol == nil {
		writePatrolAgentUnavailableResponse(w)
		return
	}

	var req struct {
		FindingID string `json:"finding_id"`
		Reason    string `json:"reason"` // "not_an_issue", "expected_behavior", "will_fix_later"
		Note      string `json:"note"`   // Optional freeform note
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFindingInvalidRequestResponse(w, "Invalid request body", nil)
		return
	}

	if req.FindingID == "" {
		writeFindingInvalidRequestResponse(w, "finding_id is required", map[string]string{"finding_id": "required"})
		return
	}

	if req.Reason == "" {
		writeFindingInvalidRequestResponse(w, "reason is required", map[string]string{"reason": "required"})
		return
	}

	// Validate reason
	validReasons := map[string]bool{
		"not_an_issue":      true,
		"expected_behavior": true,
		"will_fix_later":    true,
	}
	if req.Reason != "" && !validReasons[req.Reason] {
		writeFindingInvalidRequestResponse(w, "Invalid reason. Valid values: not_an_issue, expected_behavior, will_fix_later", map[string]string{"reason": "must be one of not_an_issue, expected_behavior, will_fix_later"})
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
	unifiedStore := h.GetUnifiedStoreForOrg(GetOrgID(r.Context()))
	if !foundInPatrol && unifiedStore != nil {
		unifiedFinding := unifiedStore.Get(req.FindingID)
		if unifiedFinding != nil {
			detectedAt = unifiedFinding.DetectedAt
			category = string(unifiedFinding.Category)
			severity = string(unifiedFinding.Severity)
			resourceID = unifiedFinding.ResourceID
			findingKey = unifiedFinding.ID // Use ID as key for unified findings
		} else {
			writeFindingNotFoundResponse(w, "Finding not found")
			return
		}
	} else if !foundInPatrol {
		writeFindingNotFoundResponse(w, "Finding not found")
		return
	}
	if foundInPatrol {
		if err := patrol.RejectManualActionForRuntimeFinding(req.FindingID, "dismissed"); err != nil {
			writeFindingActionNotAllowedResponse(w, err.Error())
			return
		}
	}

	// Dismiss in patrol findings if it exists there
	if foundInPatrol {
		if !findings.Dismiss(req.FindingID, req.Reason, req.Note) {
			writeFindingNotFoundResponse(w, "Finding not found")
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
	if learningStore := h.GetLearningStoreForOrg(GetOrgID(r.Context())); learningStore != nil {
		learningStore.RecordFeedback(learning.FeedbackRecord{
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

	patrol := h.getPatrolService(r.Context())
	if patrol == nil {
		writePatrolServiceUnavailableResponse(w)
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
	if err := patrol.RejectManualActionForRuntimeFinding(req.FindingID, "suppressed"); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
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
	if store := h.GetUnifiedStoreForOrg(GetOrgID(r.Context())); store != nil {
		store.Dismiss(req.FindingID, "not_an_issue", "Permanently suppressed by user")
	}

	// Record to learning store - suppress is a strong "not an issue" signal
	if learningStore := h.GetLearningStoreForOrg(GetOrgID(r.Context())); learningStore != nil {
		learningStore.RecordFeedback(learning.FeedbackRecord{
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

	patrol := h.getPatrolService(r.Context())
	if patrol == nil {
		writePatrolServiceUnavailableResponse(w)
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

	patrol := h.getPatrolService(r.Context())
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

	// Parse optional limit query parameter (default: 30)
	limit := 30
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
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

// HandleGetPatrolRun returns a single patrol run by ID (GET /api/ai/patrol/runs/{id})
func (h *AISettingsHandler) HandleGetPatrolRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	const prefix = "/api/ai/patrol/runs/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	runID := strings.TrimPrefix(r.URL.Path, prefix)
	if runID == "" {
		http.Error(w, "run_id is required", http.StatusBadRequest)
		return
	}
	if decoded, err := url.PathUnescape(runID); err == nil {
		runID = decoded
	}
	if strings.TrimSpace(runID) == "" {
		http.Error(w, "run_id is required", http.StatusBadRequest)
		return
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		writeErrorResponse(
			w,
			http.StatusServiceUnavailable,
			"service_unavailable",
			"Pulse Patrol service not available",
			nil,
		)
		return
	}

	patrol := aiService.GetPatrolService()
	if patrol == nil {
		writeErrorResponse(
			w,
			http.StatusServiceUnavailable,
			"service_unavailable",
			"Pulse Patrol service not available",
			nil,
		)
		return
	}

	run, ok := patrol.GetRunByID(runID)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Patrol run not found", nil)
		return
	}

	// By default, omit full tool call arrays to keep payloads lean.
	// Use ?include=tool_calls to get the full array.
	if r.URL.Query().Get("include") != "tool_calls" {
		run.ToolCalls = nil
	}

	if err := utils.WriteJSONResponse(w, run); err != nil {
		log.Error().Err(err).Msg("Failed to write patrol run response")
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
		var parsed int
		if _, err := fmt.Sscanf(daysStr, "%d", &parsed); err == nil && parsed > 0 {
			days = parsed
			if days > 365 {
				days = 365
			}
		}
	}

	var summary cost.Summary
	if h.GetAIService(r.Context()) != nil {
		summary = h.GetAIService(r.Context()).GetCostSummary(days)
	} else {
		summary = cost.EmptySummary(days, cost.DefaultMaxDays, days, false)
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

	// Limit body size — this endpoint takes no body but cap it to prevent abuse.
	r.Body = http.MaxBytesReader(w, r.Body, 1024)

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

	patrol := h.getPatrolService(r.Context())
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

	patrol := h.getPatrolService(r.Context())
	if patrol == nil {
		writePatrolServiceUnavailableResponse(w)
		return
	}

	var req struct {
		ResourceID      string `json:"resource_id"`       // Can be empty for "any resource" when AllowBroadScope is true
		ResourceName    string `json:"resource_name"`     // Human-readable name
		Category        string `json:"category"`          // Can be empty for "any category" when AllowBroadScope is true
		Description     string `json:"description"`       // Required - user's reason
		AllowBroadScope bool   `json:"allow_broad_scope"` // Required when resource or category is intentionally wildcarded
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.ResourceID = strings.TrimSpace(req.ResourceID)
	req.ResourceName = strings.TrimSpace(req.ResourceName)
	req.Category = strings.TrimSpace(req.Category)
	req.Description = strings.TrimSpace(req.Description)

	if req.Description == "" {
		http.Error(w, "description is required", http.StatusBadRequest)
		return
	}
	if (req.ResourceID == "" || req.Category == "") && !req.AllowBroadScope {
		http.Error(w, "broad suppression scope requires allow_broad_scope", http.StatusBadRequest)
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

	patrol := h.getPatrolService(r.Context())
	if patrol == nil {
		writePatrolServiceUnavailableResponse(w)
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

	patrol := h.getPatrolService(r.Context())
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

// getAuthUsername extracts the username from the current auth context
func getAuthUsername(cfg *config.Config, r *http.Request) string {
	if token := getAPITokenRecordFromRequest(r); token != nil {
		if username := apiTokenAuthenticatedUser(token); username != "" {
			return username
		}
	}

	// Check OIDC session first
	if cookie, err := readSessionCookie(r); err == nil && cookie.Value != "" {
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
// Approval Workflow Handlers
// ============================================================================

// HandleListApprovals returns pending approval requests for the current org.
// Pulse Assistant command approvals are a core chat workflow; investigation fix
// approval execution remains gated by the auto-fix endpoint when approved.
func (h *AISettingsHandler) HandleListApprovals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	store := approval.GetStore()
	if store == nil {
		response := map[string]interface{}{
			"approvals": []approval.ApprovalRequest{},
			"stats":     emptyApprovalStats(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	orgID := approval.NormalizeOrgID(GetOrgID(r.Context()))
	response := map[string]interface{}{
		"approvals": store.GetPendingApprovalsForOrg(orgID),
		"stats":     store.GetStatsForOrg(orgID),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func emptyApprovalStats() map[string]int {
	return map[string]int{
		"pending":    0,
		"approved":   0,
		"denied":     0,
		"expired":    0,
		"executions": 0,
	}
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

	orgID := approval.NormalizeOrgID(GetOrgID(r.Context()))
	req, ok := store.GetApproval(approvalID)
	if !ok || !approval.BelongsToOrg(req, orgID) {
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

	// SECURITY: Approval execution accepts the dedicated mobile relay capability
	// for new pairings while remaining backward-compatible with older mobile
	// tokens that still carry ai:execute.
	if !ensureRelayMobileRuntimeRoute(w, r, relayMobileRouteApprovalApprove) {
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
	orgID := approval.NormalizeOrgID(GetOrgID(r.Context()))
	existingReq, ok := store.GetApproval(approvalID)
	if !ok || !approval.BelongsToOrg(existingReq, orgID) {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Approval request not found", nil)
		return
	}

	// Investigation fix approvals are gated by the safe remediation extension point.
	// The free adapter returns 402; the enterprise adapter executes the fix.
	if existingReq.ToolID == "investigation_fix" {
		if h.aiAutoFixEndpoints != nil {
			h.aiAutoFixEndpoints.HandleApproveInvestigationFix(w, r)
		} else {
			WriteLicenseRequired(w, featureAIAutoFixValue, "Patrol fix actions require Pulse Pro")
		}
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

// HandleApproveAndExecuteInvestigationFix has been moved to enterprise.
// The route now delegates to aiAutoFixEndpoints.HandleApproveInvestigationFix.

// executeInvestigationFix has been moved to enterprise.

// cleanTargetHost and the agent command/tool executor adapters have been
// moved to router_routes_ai_relay.go as package-level helpers used by the
// AIAutoFixHandlerDeps adapters.

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
	if existing := findingsStore.Get(findingID); existing != nil && existing.InvestigationOutcome == outcome {
		return
	}

	if !findingsStore.UpdateInvestigationOutcome(findingID, outcome) {
		log.Warn().Str("findingID", findingID).Msg("Finding not found for outcome update")
		return
	}
	patrol.PublishFindingLifecycleUpdate(findingID)

	log.Info().Str("findingID", findingID).Str("outcome", outcome).Msg("Updated finding investigation outcome")
}

func (h *AISettingsHandler) actionAuditStoreForApprovalDecision(ctx context.Context) unifiedresources.ResourceStore {
	if h == nil {
		return nil
	}
	h.stateMu.RLock()
	chatHandler := h.chatHandler
	h.stateMu.RUnlock()
	if chatHandler == nil {
		return nil
	}
	service := chatHandler.GetService(ctx)
	provider, ok := service.(actionAuditStoreProvider)
	if !ok {
		return nil
	}
	return provider.GetActionAuditStore()
}

// HandleDenyCommand denies a pending command.
func (h *AISettingsHandler) HandleDenyCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !ensureRelayMobileRuntimeRoute(w, r, relayMobileRouteApprovalDeny) {
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
	orgID := approval.NormalizeOrgID(GetOrgID(r.Context()))
	existingReq, ok := store.GetApproval(approvalID)
	if !ok || !approval.BelongsToOrg(existingReq, orgID) {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Approval request not found", nil)
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
	reason := strings.TrimSpace(body.Reason)
	if reason == "" {
		reason = "approval denied"
	}
	tools.RecordApprovalDecision(
		h.actionAuditStoreForApprovalDecision(r.Context()),
		req.ID,
		unifiedresources.ActionStateRejected,
		username,
		reason,
	)
	if req.ToolID == "investigation_fix" && strings.EqualFold(strings.TrimSpace(req.TargetType), "investigation") && strings.TrimSpace(req.TargetID) != "" {
		h.updateFindingOutcome(r.Context(), orgID, strings.TrimSpace(req.TargetID), string(ai.InvestigationOutcomeFixRejected))
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

// PatrolAutonomySettings represents the patrol autonomy configuration for API requests.
// FullModeUnlocked is a legacy compatibility field only; it is never acknowledgement authority.
type PatrolAutonomySettings struct {
	AutonomyLevel           string `json:"autonomy_level"`               // "monitor", "approval", "assisted", "full"
	FullModeUnlocked        *bool  `json:"full_mode_unlocked,omitempty"` // Deprecated compatibility label (nil = preserve existing)
	AcknowledgementID       string `json:"acknowledgement_id,omitempty"`
	InvestigationBudget     int    `json:"investigation_budget"`      // Max turns per investigation (5-30)
	InvestigationTimeoutSec int    `json:"investigation_timeout_sec"` // Max seconds per investigation (60-1800)
}

// PatrolAutonomyResponse represents the patrol autonomy configuration for API responses
// Uses plain bool for FullModeUnlocked since responses always include the actual value
type PatrolAutonomyResponse struct {
	AutonomyLevel           string                                 `json:"autonomy_level"`
	RequestedAutonomyLevel  string                                 `json:"requested_autonomy_level"`
	EffectiveAutonomyLevel  string                                 `json:"effective_autonomy_level"`
	FullModeUnlocked        bool                                   `json:"full_mode_unlocked"`
	AutopilotStatus         unifiedresources.PatrolAutopilotStatus `json:"autopilot_acknowledgement"`
	InvestigationBudget     int                                    `json:"investigation_budget"`
	InvestigationTimeoutSec int                                    `json:"investigation_timeout_sec"`
}

type patrolAutopilotAcknowledgementRequest struct {
	AcknowledgementID string `json:"acknowledgement_id"`
}

type patrolAutopilotRevocationRequest struct {
	Reason string `json:"reason,omitempty"`
}

type patrolAutopilotActivationRequest struct {
	AcknowledgementID string
	Actor             unifiedresources.ActionActor
}

type patrolAutopilotActivationContextKey struct{}

func patrolAutopilotActivationFromContext(ctx context.Context) (patrolAutopilotActivationRequest, bool) {
	request, ok := ctx.Value(patrolAutopilotActivationContextKey{}).(patrolAutopilotActivationRequest)
	return request, ok
}

func decodeStrictPatrolAutopilotJSONBody(w http.ResponseWriter, r *http.Request, target any, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return decodeStrictPatrolAutopilotJSONBytes(data, target)
}

func decodeStrictPatrolAutopilotJSONBytes(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func cloneAIConfigForPatrolAutopilot(cfg *config.AIConfig) (*config.AIConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("Pulse Patrol config not configured")
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var cloned config.AIConfig
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func (h *AISettingsHandler) mutatePatrolAutopilotConfig(ctx context.Context, mutate func(*config.AIConfig, unifiedresources.PatrolAutopilotServerPolicy) (bool, error)) error {
	if h == nil {
		return fmt.Errorf("Pulse Patrol service not available")
	}
	persistence := h.getPersistence(ctx)
	service := h.GetAIService(ctx)
	if persistence == nil || service == nil {
		return fmt.Errorf("Pulse Patrol config persistence unavailable")
	}
	write := func() error {
		current, err := h.loadAIConfig(ctx)
		if err != nil {
			return err
		}
		next, err := cloneAIConfigForPatrolAutopilot(current)
		if err != nil {
			return err
		}
		changed, err := mutate(next, h.currentPatrolAutopilotServerPolicy())
		if err != nil || !changed {
			return err
		}
		if err := persistence.SaveAIConfig(*next); err != nil {
			return err
		}
		return service.ApplyPatrolAutonomyConfig(next)
	}
	h.stateMu.RLock()
	coordinator := h.policyMutation
	h.stateMu.RUnlock()
	if coordinator != nil {
		return coordinator(write)
	}
	return write()
}

func patrolAutonomyResponseForConfig(cfg *config.AIConfig, orgID string, policy unifiedresources.PatrolAutopilotServerPolicy, hasAutoFix bool) PatrolAutonomyResponse {
	requested := cfg.GetPatrolAutonomyLevel()
	effective, status := cfg.GetEffectivePatrolAutonomyWithPolicy(orgID, policy)
	if !hasAutoFix {
		effective = config.PatrolAutonomyMonitor
		status.Active = false
		status.Code = unifiedresources.PatrolAutopilotStatusLicenseRequired
	}
	return PatrolAutonomyResponse{
		AutonomyLevel:           effective,
		RequestedAutonomyLevel:  requested,
		EffectiveAutonomyLevel:  effective,
		FullModeUnlocked:        effective == config.PatrolAutonomyFull && status.Active,
		AutopilotStatus:         status,
		InvestigationBudget:     cfg.GetPatrolInvestigationBudget(),
		InvestigationTimeoutSec: int(cfg.GetPatrolInvestigationTimeout().Seconds()),
	}
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

	hasAutoFix := aiService.HasLicenseFeature(featureAIAutoFixValue)
	settings := patrolAutonomyResponseForConfig(cfg, GetOrgID(r.Context()), h.currentPatrolAutopilotServerPolicy(), hasAutoFix)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func (h *AISettingsHandler) HandleCreatePatrolAutopilotAcknowledgement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request patrolAutopilotAcknowledgementRequest
	if err := decodeStrictPatrolAutopilotJSONBody(w, r, &request, 16*1024); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid acknowledgement request", nil)
		return
	}
	orgID := strings.TrimSpace(GetOrgID(r.Context()))
	if orgID == "" {
		orgID = "default"
	}
	actor, err := actionActorForRequest(h.defaultConfig, r, orgID)
	if err != nil || actor.Kind != unifiedresources.ActionActorUser {
		writeErrorResponse(w, http.StatusForbidden, unifiedresources.PatrolAutopilotStatusUserRequired, "Autopilot acknowledgement requires an authenticated human session", nil)
		return
	}
	var acknowledgement unifiedresources.PatrolAutopilotAcknowledgement
	created := false
	err = h.mutatePatrolAutopilotConfig(r.Context(), func(cfg *config.AIConfig, policy unifiedresources.PatrolAutopilotServerPolicy) (bool, error) {
		var issueErr error
		acknowledgement, created, issueErr = unifiedresources.IssuePatrolAutopilotAcknowledgement(cfg.PatrolAutopilotAcknowledgements, request.AcknowledgementID, actor, policy)
		if issueErr != nil || !created {
			return false, issueErr
		}
		cfg.PatrolAutopilotAcknowledgements = append(cfg.PatrolAutopilotAcknowledgements, acknowledgement)
		return true, nil
	})
	if err != nil {
		code := unifiedresources.PatrolAutopilotErrorCode(err)
		if code == "" {
			code = unifiedresources.PatrolAutopilotStatusStoreUnavailable
		}
		status := http.StatusConflict
		if code == unifiedresources.PatrolAutopilotStatusStoreUnavailable {
			status = http.StatusServiceUnavailable
		}
		writeErrorResponse(w, status, code, "Autopilot acknowledgement was not recorded", nil)
		return
	}
	_, acknowledgementStatus := unifiedresources.ValidatePatrolAutopilotAcknowledgement(
		[]unifiedresources.PatrolAutopilotAcknowledgement{acknowledgement}, nil, acknowledgement.ID, orgID, &actor, h.currentPatrolAutopilotServerPolicy(),
	)
	statusCode := http.StatusOK
	if created {
		statusCode = http.StatusCreated
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{"created": created, "acknowledgement": acknowledgementStatus})
}

func (h *AISettingsHandler) HandleRevokePatrolAutopilotAcknowledgement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	acknowledgementID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/ai/patrol/autonomy/acknowledgements/"))
	if acknowledgementID == "" || strings.Contains(acknowledgementID, "/") {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Acknowledgement id is required", nil)
		return
	}
	var request patrolAutopilotRevocationRequest
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid revocation request", nil)
		return
	}
	if len(bytes.TrimSpace(body)) > 0 {
		if err := decodeStrictPatrolAutopilotJSONBytes(body, &request); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid revocation request", nil)
			return
		}
	}
	orgID := strings.TrimSpace(GetOrgID(r.Context()))
	if orgID == "" {
		orgID = "default"
	}
	actor, err := actionActorForRequest(h.defaultConfig, r, orgID)
	if err != nil || actor.Kind != unifiedresources.ActionActorUser {
		writeErrorResponse(w, http.StatusForbidden, unifiedresources.PatrolAutopilotStatusUserRequired, "Autopilot revocation requires an authenticated human session", nil)
		return
	}
	created := false
	err = h.mutatePatrolAutopilotConfig(r.Context(), func(cfg *config.AIConfig, policy unifiedresources.PatrolAutopilotServerPolicy) (bool, error) {
		revocation, wasCreated, revokeErr := unifiedresources.RevokePatrolAutopilotAcknowledgement(cfg.PatrolAutopilotAcknowledgements, cfg.PatrolAutopilotRevocations, acknowledgementID, actor, request.Reason, policy)
		if revokeErr != nil || !wasCreated {
			created = false
			return false, revokeErr
		}
		created = true
		cfg.PatrolAutopilotRevocations = append(cfg.PatrolAutopilotRevocations, revocation)
		cfg.PatrolFullModeUnlocked = false
		return true, nil
	})
	if err != nil {
		code := unifiedresources.PatrolAutopilotErrorCode(err)
		if code == "" {
			code = unifiedresources.PatrolAutopilotStatusStoreUnavailable
		}
		status := http.StatusConflict
		if code == unifiedresources.PatrolAutopilotStatusStoreUnavailable {
			status = http.StatusServiceUnavailable
		}
		writeErrorResponse(w, status, code, "Autopilot acknowledgement was not revoked", nil)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"revoked": true, "created": created, "acknowledgement_id": acknowledgementID})
}

func (h *AISettingsHandler) GatePatrolAutonomyUpdate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			next(w, r)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid Patrol autonomy request", nil)
			return
		}
		var settings PatrolAutonomySettings
		if err := decodeStrictPatrolAutopilotJSONBytes(body, &settings); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid Patrol autonomy request", nil)
			return
		}
		settings.AcknowledgementID = strings.TrimSpace(settings.AcknowledgementID)
		if settings.AutonomyLevel != config.PatrolAutonomyFull {
			if settings.AcknowledgementID != "" {
				writeErrorResponse(w, http.StatusConflict, unifiedresources.PatrolAutopilotStatusNotRequested, "Acknowledgement is only valid for an explicit Autopilot activation", nil)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
			next(w, r)
			return
		}
		if settings.AcknowledgementID == "" {
			writeErrorResponse(w, http.StatusConflict, unifiedresources.PatrolAutopilotStatusAcknowledgementRequired, "Autopilot activation requires a current acknowledgement", nil)
			return
		}
		orgID := strings.TrimSpace(GetOrgID(r.Context()))
		if orgID == "" {
			orgID = "default"
		}
		actor, err := actionActorForRequest(h.defaultConfig, r, orgID)
		if err != nil || actor.Kind != unifiedresources.ActionActorUser {
			writeErrorResponse(w, http.StatusForbidden, unifiedresources.PatrolAutopilotStatusUserRequired, "Autopilot activation requires the human session that owns the acknowledgement", nil)
			return
		}
		cfg, err := h.loadAIConfig(r.Context())
		if err != nil {
			writeErrorResponse(w, http.StatusServiceUnavailable, unifiedresources.PatrolAutopilotStatusStoreUnavailable, "Autopilot acknowledgement store is unavailable", nil)
			return
		}
		_, status := unifiedresources.ValidatePatrolAutopilotAcknowledgement(cfg.PatrolAutopilotAcknowledgements, cfg.PatrolAutopilotRevocations, settings.AcknowledgementID, orgID, &actor, h.currentPatrolAutopilotServerPolicy())
		if !status.Active {
			writeErrorResponse(w, http.StatusConflict, status.Code, "Autopilot acknowledgement is not eligible for activation", nil)
			return
		}
		compatUnlocked := true
		settings.FullModeUnlocked = &compatUnlocked
		normalizedBody, err := json.Marshal(settings)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "encoding_failed", "Failed to prepare Patrol autonomy request", nil)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(normalizedBody))
		r.ContentLength = int64(len(normalizedBody))
		activationRequest := patrolAutopilotActivationRequest{AcknowledgementID: settings.AcknowledgementID, Actor: actor}
		next(w, r.WithContext(context.WithValue(r.Context(), patrolAutopilotActivationContextKey{}, activationRequest)))
	}
}

// HandleUpdatePatrolAutonomyMonitorOnly persists findings-only Patrol autonomy
// settings for builds without the safe remediation extension.
func (h *AISettingsHandler) HandleUpdatePatrolAutonomyMonitorOnly(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
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

	var req PatrolAutonomySettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}
	if !config.IsValidPatrolAutonomyLevel(req.AutonomyLevel) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_autonomy_level",
			fmt.Sprintf("Invalid autonomy level: %s. Must be 'monitor', 'approval', 'assisted', or 'full'", req.AutonomyLevel), nil)
		return
	}
	if req.AutonomyLevel != config.PatrolAutonomyMonitor {
		WriteLicenseRequired(w, featureAIAutoFixKey, "Investigation and auto-fix require Pulse Pro. Community tier is limited to Monitor (findings-only) autonomy.")
		return
	}

	if req.InvestigationBudget < 5 {
		req.InvestigationBudget = 5
	}
	if req.InvestigationBudget > 30 {
		req.InvestigationBudget = 30
	}
	if req.InvestigationTimeoutSec < 60 {
		req.InvestigationTimeoutSec = 60
	}
	if req.InvestigationTimeoutSec > 1800 {
		req.InvestigationTimeoutSec = 1800
	}

	if err := h.mutatePatrolAutopilotConfig(r.Context(), func(current *config.AIConfig, _ unifiedresources.PatrolAutopilotServerPolicy) (bool, error) {
		changed := current.PatrolAutonomyLevel != config.PatrolAutonomyMonitor || current.PatrolFullModeUnlocked || current.PatrolAutopilotActivation != nil ||
			current.PatrolInvestigationBudget != req.InvestigationBudget || current.PatrolInvestigationTimeoutSec != req.InvestigationTimeoutSec
		current.PatrolAutonomyLevel = config.PatrolAutonomyMonitor
		current.PatrolFullModeUnlocked = false
		current.PatrolAutopilotActivation = nil
		current.PatrolInvestigationBudget = req.InvestigationBudget
		current.PatrolInvestigationTimeoutSec = req.InvestigationTimeoutSec
		return changed, nil
	}); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "save_failed", "Failed to save Pulse Patrol autonomy settings", nil)
		return
	}
	settings := patrolAutonomyResponseForConfig(aiService.GetConfig(), GetOrgID(r.Context()), h.currentPatrolAutopilotServerPolicy(), aiService.HasLicenseFeature(featureAIAutoFixValue))
	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"success":  true,
		"settings": settings,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write patrol autonomy update response")
	}
}

// Premium Patrol autonomy updates have been moved to enterprise.
// The route delegates to aiAutoFixEndpoints.HandleUpdatePatrolAutonomy.

// maxFindingIDLength is the maximum allowed length for finding IDs in URL paths.
// Real finding IDs are 16 hex chars (SHA256[:8]), but we accept up to 256 for
// forward compatibility with any future ID scheme.
const maxFindingIDLength = 256

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
	if len(findingID) > maxFindingIDLength {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_id", "Finding ID is too long", nil)
		return
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Patrol service not initialized", nil)
		return
	}
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
	orgID := GetOrgID(r.Context())
	investigation = h.hydratePatrolInvestigationAction(orgID, investigation)
	normalizedInvestigation := investigation.NormalizeCollections()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(normalizedInvestigation)
}

// HandleReapproveInvestigationFix has been moved to enterprise.
// The route now delegates to aiAutoFixEndpoints.HandleReapproveInvestigationFix.

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
	if len(findingID) > maxFindingIDLength {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_id", "Finding ID is too long", nil)
		return
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Patrol service not initialized", nil)
		return
	}
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
	for i := range messages {
		messages[i] = messages[i].NormalizeCollections()
	}
	normalizedInvestigation := investigation.NormalizeCollections()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"investigation_id": normalizedInvestigation.ID,
		"session_id":       normalizedInvestigation.SessionID,
		"messages":         messages,
	})
}

// HandleReinvestigateFinding has been moved to enterprise.
// The route now delegates to aiAutoFixEndpoints.HandleReinvestigateFinding.
