package api

import (
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/resourceapi"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// ResourceHandlers preserves the established router and extension surface
// while composing the resource-query service with API-owned action mutations.
// QueryService owns resource registry construction, durable resource storage,
// filtering, projections, and read handlers. The fields below remain here
// because action lifecycle and operator-state mutations are separate domains.
type ResourceHandlers struct {
	resourceapi.QueryService

	cfg                       *config.Config
	tenantStateProvider       TenantStateProvider
	actionExecutor            ActionExecutor
	actionCompleted           func(unified.ActionAuditRecord)
	actionTransition          func(orgID string, record unified.ActionAuditRecord)
	policyAdmission           *actionlifecycle.PolicyAdmissionCoordinator
	actionEmergencyStop       func(orgID string) (bool, error)
	actionDecisionAuthorizer  actionlifecycle.DecisionAuthorizer
	actionExecutionAuthorizer actionlifecycle.ExecutionAuthorizer
	actionRefreshPlanner      actionlifecycle.RefreshPlanner
	operatorStateChanged      func(orgID, resourceID string)
}

type ResourceDiscoveryReadinessProvider = resourceapi.ResourceDiscoveryReadinessProvider
type SupplementalRecordsProvider = resourceapi.SupplementalRecordsProvider
type TenantSupplementalRecordsProvider = resourceapi.TenantSupplementalRecordsProvider
type SupplementalSnapshotSourceOwner = resourceapi.SupplementalSnapshotSourceOwner
type TenantSupplementalSnapshotSourceOwner = resourceapi.TenantSupplementalSnapshotSourceOwner
type ResourcesResponse = resourceapi.ResourcesResponse
type ResourcesMeta = resourceapi.ResourcesMeta
type StorageSummaryResponse = resourceapi.StorageSummaryResponse
type StorageSummaryIncident = resourceapi.StorageSummaryIncident
type StorageIncidentsResponse = resourceapi.StorageIncidentsResponse
type StorageIncidentSection = resourceapi.StorageIncidentSection

func EmptyResourcesResponse() ResourcesResponse { return resourceapi.EmptyResourcesResponse() }

func EmptyStorageSummaryResponse() StorageSummaryResponse {
	return resourceapi.EmptyStorageSummaryResponse()
}

func EmptyStorageIncidentsResponse() StorageIncidentsResponse {
	return resourceapi.EmptyStorageIncidentsResponse()
}

// NewResourceHandlers builds the compatibility composition used by Router.
func NewResourceHandlers(cfg *config.Config) *ResourceHandlers {
	return &ResourceHandlers{
		QueryService:    *resourceapi.NewQueryService(cfg),
		cfg:             cfg,
		policyAdmission: &actionlifecycle.PolicyAdmissionCoordinator{},
	}
}

func (h *ResourceHandlers) ensureQueryService() *resourceapi.QueryService {
	return &h.QueryService
}

func (h *ResourceHandlers) SetStateProvider(provider SnapshotProvider) {
	h.ensureQueryService().SetStateProvider(provider)
}

func (h *ResourceHandlers) SetTenantStateProvider(provider TenantStateProvider) {
	h.tenantStateProvider = provider
	h.ensureQueryService().SetTenantStateProvider(provider)
}

func (h *ResourceHandlers) SetActionExecutor(executor ActionExecutor) {
	h.actionExecutor = executor
	checker, _ := executor.(actionlifecycle.AvailabilityChecker)
	h.ensureQueryService().SetActionAvailabilityChecker(checker)
}

func (h *ResourceHandlers) SetActionEmergencyStopChecker(checker func(orgID string) (bool, error)) {
	h.actionEmergencyStop = checker
}

func (h *ResourceHandlers) SetActionAuthorizers(decision actionlifecycle.DecisionAuthorizer, execution actionlifecycle.ExecutionAuthorizer) {
	h.actionDecisionAuthorizer = decision
	h.actionExecutionAuthorizer = execution
}

func (h *ResourceHandlers) SetActionRefreshPlanner(planner actionlifecycle.RefreshPlanner) {
	h.actionRefreshPlanner = planner
}

func (h *ResourceHandlers) SetActionCompletedPublisher(publisher func(unified.ActionAuditRecord)) {
	h.actionCompleted = publisher
}

func (h *ResourceHandlers) SetActionTransitionPublisher(publisher func(orgID string, record unified.ActionAuditRecord)) {
	h.actionTransition = publisher
}

func (h *ResourceHandlers) SetOperatorStateChanged(callback func(orgID, resourceID string)) {
	h.operatorStateChanged = callback
}

func (h *ResourceHandlers) buildRegistry(orgID string) (*unified.ResourceRegistry, error) {
	return h.ensureQueryService().BuildRegistry(orgID)
}

func (h *ResourceHandlers) getStore(orgID string) (unified.ResourceStore, error) {
	return h.ensureQueryService().Store(orgID)
}

func (h *ResourceHandlers) invalidateCache(orgID string) {
	h.ensureQueryService().InvalidateCache(orgID)
}

func (h *ResourceHandlers) CloseTenantStore(orgID string) error {
	if h == nil {
		return nil
	}
	return h.QueryService.CloseTenantStore(orgID)
}

func (h *ResourceHandlers) CloseStores() error {
	if h == nil {
		return nil
	}
	return h.QueryService.CloseStores()
}

// firstNonEmptyTrimmed is retained for non-resource connection projections.
func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func getUserID(r *http.Request) string {
	return auth.GetUser(r.Context())
}
