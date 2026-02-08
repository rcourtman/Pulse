package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// StateProvider allows ResourceHandlers to fetch current state on demand.
type StateProvider interface {
	GetState() models.StateSnapshot
}

// TenantStateProvider allows ResourceHandlers to fetch current state for a specific tenant.
type TenantStateProvider interface {
	GetStateForTenant(orgID string) models.StateSnapshot
}

// ResourceHandlers provides HTTP handlers for the legacy /api/resources API backed by V2 registry data.
type ResourceHandlers struct {
	store               *unifiedresources.LegacyAdapter
	storesByTenant      map[string]*unifiedresources.LegacyAdapter
	storeMu             sync.RWMutex
	stateProvider       StateProvider
	tenantStateProvider TenantStateProvider
}

// NewResourceHandlers creates resource handlers with a V2-backed legacy adapter.
func NewResourceHandlers() *ResourceHandlers {
	return &ResourceHandlers{
		store:          unifiedresources.NewLegacyAdapter(nil),
		storesByTenant: make(map[string]*unifiedresources.LegacyAdapter),
	}
}

// SetStateProvider sets the state provider for on-demand resource population.
func (h *ResourceHandlers) SetStateProvider(provider StateProvider) {
	h.stateProvider = provider
}

// SetTenantStateProvider sets the tenant-aware state provider.
func (h *ResourceHandlers) SetTenantStateProvider(provider TenantStateProvider) {
	h.tenantStateProvider = provider
}

// Store returns the underlying adapter for monitor/AI injection.
func (h *ResourceHandlers) Store() *unifiedresources.LegacyAdapter {
	return h.store
}

// getStoreForTenant returns the resource adapter for a specific tenant.
func (h *ResourceHandlers) getStoreForTenant(orgID string) *unifiedresources.LegacyAdapter {
	if orgID == "" || orgID == "default" {
		return h.store
	}

	h.storeMu.RLock()
	store, exists := h.storesByTenant[orgID]
	h.storeMu.RUnlock()
	if exists {
		return store
	}

	h.storeMu.Lock()
	defer h.storeMu.Unlock()
	if store, exists = h.storesByTenant[orgID]; exists {
		return store
	}

	store = unifiedresources.NewLegacyAdapter(nil)
	h.storesByTenant[orgID] = store
	return store
}

// GetStoreStats returns stats for all tenant stores.
func (h *ResourceHandlers) GetStoreStats() map[string]unifiedresources.LegacyStoreStats {
	h.storeMu.RLock()
	defer h.storeMu.RUnlock()

	stats := make(map[string]unifiedresources.LegacyStoreStats)
	stats["default"] = h.store.GetStats()
	for orgID, store := range h.storesByTenant {
		stats[orgID] = store.GetStats()
	}
	return stats
}

// HandleGetResources returns all resources, optionally filtered by query params.
// GET /api/resources
func (h *ResourceHandlers) HandleGetResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	store := h.getStoreForTenant(orgID)

	if h.tenantStateProvider != nil && orgID != "" && orgID != "default" {
		h.PopulateFromSnapshotForTenant(orgID, h.tenantStateProvider.GetStateForTenant(orgID))
	} else if h.stateProvider != nil {
		h.PopulateFromSnapshot(h.stateProvider.GetState())
	}

	var result []unifiedresources.LegacyResource
	if r.URL.Query().Get("infrastructure") == "true" {
		result = store.GetInfrastructure()
	} else if r.URL.Query().Get("workloads") == "true" {
		result = store.GetWorkloads()
	} else {
		result = store.GetAll()
	}

	if typeParam := r.URL.Query().Get("type"); typeParam != "" {
		types := parseResourceTypes(typeParam)
		if len(types) > 0 {
			result = filterByTypes(result, types)
		}
	}

	if platformParam := r.URL.Query().Get("platform"); platformParam != "" {
		platforms := parsePlatformTypes(platformParam)
		if len(platforms) > 0 {
			result = filterByPlatforms(result, platforms)
		}
	}

	if statusParam := r.URL.Query().Get("status"); statusParam != "" {
		statuses := parseStatuses(statusParam)
		if len(statuses) > 0 {
			result = filterByStatuses(result, statuses)
		}
	}

	if parentID := r.URL.Query().Get("parent"); parentID != "" {
		result = filterByParent(result, parentID)
	}

	if r.URL.Query().Get("alerts") == "true" {
		result = filterByAlerts(result)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ResourcesResponse{
		Resources: result,
		Count:     len(result),
		Stats:     store.GetStats(),
	})
}

// HandleGetResource returns a single resource by ID.
// GET /api/resources/{id}
func (h *ResourceHandlers) HandleGetResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	store := h.getStoreForTenant(orgID)

	path := strings.TrimPrefix(r.URL.Path, "/api/resources/")
	if path == "" || path == "/" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}

	resource, ok := store.Get(path)
	if !ok {
		http.Error(w, "Resource not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resource)
}

// HandleGetResourceStats returns statistics about the resource store.
// GET /api/resources/stats
func (h *ResourceHandlers) HandleGetResourceStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	store := h.getStoreForTenant(orgID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(store.GetStats())
}

// ResourcesResponse is the response for /api/resources.
type ResourcesResponse struct {
	Resources []unifiedresources.LegacyResource `json:"resources"`
	Count     int                               `json:"count"`
	Stats     unifiedresources.LegacyStoreStats `json:"stats"`
}

// PopulateFromSnapshot converts all resources from a StateSnapshot to the unified store.
func (h *ResourceHandlers) PopulateFromSnapshot(snapshot models.StateSnapshot) {
	h.store.PopulateFromSnapshot(snapshot)
}

// PopulateFromSnapshotForTenant converts all resources from a StateSnapshot to a tenant-specific store.
func (h *ResourceHandlers) PopulateFromSnapshotForTenant(orgID string, snapshot models.StateSnapshot) {
	store := h.getStoreForTenant(orgID)
	store.PopulateFromSnapshot(snapshot)
}

func filterByTypes(resources []unifiedresources.LegacyResource, allowed []unifiedresources.LegacyResourceType) []unifiedresources.LegacyResource {
	allow := make(map[unifiedresources.LegacyResourceType]struct{}, len(allowed))
	for _, t := range allowed {
		allow[t] = struct{}{}
	}
	out := make([]unifiedresources.LegacyResource, 0, len(resources))
	for _, r := range resources {
		if _, ok := allow[r.Type]; ok {
			out = append(out, r)
		}
	}
	return out
}

func filterByPlatforms(resources []unifiedresources.LegacyResource, allowed []unifiedresources.LegacyPlatformType) []unifiedresources.LegacyResource {
	allow := make(map[unifiedresources.LegacyPlatformType]struct{}, len(allowed))
	for _, p := range allowed {
		allow[p] = struct{}{}
	}
	out := make([]unifiedresources.LegacyResource, 0, len(resources))
	for _, r := range resources {
		if _, ok := allow[r.PlatformType]; ok {
			out = append(out, r)
		}
	}
	return out
}

func filterByStatuses(resources []unifiedresources.LegacyResource, allowed []unifiedresources.LegacyResourceStatus) []unifiedresources.LegacyResource {
	allow := make(map[unifiedresources.LegacyResourceStatus]struct{}, len(allowed))
	for _, s := range allowed {
		allow[s] = struct{}{}
	}
	out := make([]unifiedresources.LegacyResource, 0, len(resources))
	for _, r := range resources {
		if _, ok := allow[r.Status]; ok {
			out = append(out, r)
		}
	}
	return out
}

func filterByParent(resources []unifiedresources.LegacyResource, parentID string) []unifiedresources.LegacyResource {
	out := make([]unifiedresources.LegacyResource, 0, len(resources))
	for _, r := range resources {
		if r.ParentID == parentID {
			out = append(out, r)
		}
	}
	return out
}

func filterByAlerts(resources []unifiedresources.LegacyResource) []unifiedresources.LegacyResource {
	out := make([]unifiedresources.LegacyResource, 0, len(resources))
	for _, r := range resources {
		if len(r.Alerts) > 0 {
			out = append(out, r)
		}
	}
	return out
}

// Helper functions for parsing query parameters

func parseResourceTypes(s string) []unifiedresources.LegacyResourceType {
	parts := strings.Split(s, ",")
	result := make([]unifiedresources.LegacyResourceType, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		result = append(result, unifiedresources.LegacyResourceType(p))
	}
	return result
}

func parsePlatformTypes(s string) []unifiedresources.LegacyPlatformType {
	parts := strings.Split(s, ",")
	result := make([]unifiedresources.LegacyPlatformType, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		result = append(result, unifiedresources.LegacyPlatformType(p))
	}
	return result
}

func parseStatuses(s string) []unifiedresources.LegacyResourceStatus {
	parts := strings.Split(s, ",")
	result := make([]unifiedresources.LegacyResourceStatus, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		result = append(result, unifiedresources.LegacyResourceStatus(p))
	}
	return result
}
