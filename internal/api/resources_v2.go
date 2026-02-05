package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// ResourceV2Handlers provides HTTP handlers for the v2 unified resource API.
type ResourceV2Handlers struct {
	cfg                 *config.Config
	storeMu             sync.Mutex
	stores              map[string]unified.ResourceStore
	cacheMu             sync.Mutex
	registryCache       map[string]registryCacheEntry
	stateProvider       StateProvider
	tenantStateProvider TenantStateProvider
}

// NewResourceV2Handlers creates a new handler.
func NewResourceV2Handlers(cfg *config.Config) *ResourceV2Handlers {
	return &ResourceV2Handlers{
		cfg:           cfg,
		stores:        make(map[string]unified.ResourceStore),
		registryCache: make(map[string]registryCacheEntry),
	}
}

// SetStateProvider sets the state provider for on-demand population.
func (h *ResourceV2Handlers) SetStateProvider(provider StateProvider) {
	h.stateProvider = provider
}

// SetTenantStateProvider sets the tenant-aware provider.
func (h *ResourceV2Handlers) SetTenantStateProvider(provider TenantStateProvider) {
	h.tenantStateProvider = provider
}

// HandleListResources handles GET /api/v2/resources.
func (h *ResourceV2Handlers) HandleListResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	registry, err := h.buildRegistry(orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resources := registry.List()

	filters := parseListFilters(r)
	resources = applyFilters(resources, filters)
	applySorting(resources, filters.sortField, filters.sortOrder)

	paged, meta := paginate(resources, filters.page, filters.limit)

	response := ResourcesV2Response{
		Data:         paged,
		Meta:         meta,
		Aggregations: registry.Stats(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetResource handles GET /api/v2/resources/{id}.
func (h *ResourceV2Handlers) HandleGetResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	registry, err := h.buildRegistry(orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v2/resources/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}

	resource, ok := registry.Get(id)
	if !ok {
		http.Error(w, "Resource not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resource)
}

// HandleResourceRoutes dispatches nested resource routes.
func (h *ResourceV2Handlers) HandleResourceRoutes(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/children") {
		h.HandleGetChildren(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/metrics") {
		h.HandleGetMetrics(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/link") {
		h.HandleLink(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/unlink") {
		h.HandleUnlink(w, r)
		return
	}
	h.HandleGetResource(w, r)
}

// HandleGetChildren handles GET /api/v2/resources/{id}/children.
func (h *ResourceV2Handlers) HandleGetChildren(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	registry, err := h.buildRegistry(orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v2/resources/")
	path = strings.TrimSuffix(path, "/children")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}

	children := registry.GetChildren(path)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":  children,
		"count": len(children),
	})
}

// HandleGetMetrics handles GET /api/v2/resources/{id}/metrics.
func (h *ResourceV2Handlers) HandleGetMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	registry, err := h.buildRegistry(orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v2/resources/")
	path = strings.TrimSuffix(path, "/metrics")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}

	resource, ok := registry.Get(path)
	if !ok {
		http.Error(w, "Resource not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resource.Metrics)
}

// HandleStats handles GET /api/v2/resources/stats.
func (h *ResourceV2Handlers) HandleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	registry, err := h.buildRegistry(orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(registry.Stats())
}

// HandleLink handles POST /api/v2/resources/{id}/link.
func (h *ResourceV2Handlers) HandleLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	store, err := h.getStore(orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v2/resources/")
	path = strings.TrimSuffix(path, "/link")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}

	var payload struct {
		TargetID string `json:"targetId"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if payload.TargetID == "" {
		http.Error(w, "targetId required", http.StatusBadRequest)
		return
	}

	link := unified.ResourceLink{
		ResourceA: path,
		ResourceB: payload.TargetID,
		PrimaryID: path,
		Reason:    payload.Reason,
		CreatedBy: getUserID(r),
		CreatedAt: time.Now().UTC(),
	}

	if err := store.AddLink(link); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.invalidateCache(orgID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Resources linked",
	})
}

// HandleUnlink handles POST /api/v2/resources/{id}/unlink.
func (h *ResourceV2Handlers) HandleUnlink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	store, err := h.getStore(orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v2/resources/")
	path = strings.TrimSuffix(path, "/unlink")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}

	var payload struct {
		TargetID string `json:"targetId"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if payload.TargetID == "" {
		http.Error(w, "targetId required", http.StatusBadRequest)
		return
	}

	exclusion := unified.ResourceExclusion{
		ResourceA: path,
		ResourceB: payload.TargetID,
		Reason:    payload.Reason,
		CreatedBy: getUserID(r),
		CreatedAt: time.Now().UTC(),
	}

	if err := store.AddExclusion(exclusion); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.invalidateCache(orgID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Resources unlinked",
	})
}

// buildRegistry constructs a registry for the current tenant.
func (h *ResourceV2Handlers) buildRegistry(orgID string) (*unified.ResourceRegistry, error) {
	store, err := h.getStore(orgID)
	if err != nil {
		return nil, err
	}
	key := cacheKey(orgID)

	var snapshot models.StateSnapshot
	if h.tenantStateProvider != nil && orgID != "" && orgID != "default" {
		snapshot = h.tenantStateProvider.GetStateForTenant(orgID)
	} else if h.stateProvider != nil {
		snapshot = h.stateProvider.GetState()
	}

	h.cacheMu.Lock()
	entry, ok := h.registryCache[key]
	if ok && entry.registry != nil && entry.lastUpdate.Equal(snapshot.LastUpdate) {
		h.cacheMu.Unlock()
		return entry.registry, nil
	}
	h.cacheMu.Unlock()

	registry := unified.NewRegistry(store)
	registry.IngestSnapshot(snapshot)

	h.cacheMu.Lock()
	h.registryCache[key] = registryCacheEntry{registry: registry, lastUpdate: snapshot.LastUpdate}
	h.cacheMu.Unlock()

	return registry, nil
}

func (h *ResourceV2Handlers) getStore(orgID string) (unified.ResourceStore, error) {
	h.storeMu.Lock()
	defer h.storeMu.Unlock()
	key := cacheKey(orgID)
	if store, ok := h.stores[key]; ok {
		return store, nil
	}
	dataDir := ""
	if h.cfg != nil {
		dataDir = h.cfg.DataPath
	}
	store, err := unified.NewSQLiteResourceStore(dataDir, key)
	if err != nil {
		return nil, err
	}
	h.stores[key] = store
	return store, nil
}

func (h *ResourceV2Handlers) invalidateCache(orgID string) {
	key := cacheKey(orgID)
	h.cacheMu.Lock()
	delete(h.registryCache, key)
	h.cacheMu.Unlock()
}

func cacheKey(orgID string) string {
	key := strings.TrimSpace(orgID)
	if key == "" {
		key = "default"
	}
	return key
}

// ResourcesV2Response represents list response.
type ResourcesV2Response struct {
	Data         []unified.Resource    `json:"data"`
	Meta         ResourcesMeta         `json:"meta"`
	Aggregations unified.ResourceStats `json:"aggregations"`
}

// ResourcesMeta represents pagination metadata.
type ResourcesMeta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

type registryCacheEntry struct {
	registry   *unified.ResourceRegistry
	lastUpdate time.Time
}

// Filtering helpers.
type listFilters struct {
	types     map[unified.ResourceType]struct{}
	sources   map[unified.DataSource]struct{}
	statuses  map[unified.ResourceStatus]struct{}
	parent    string
	cluster   string
	query     string
	tags      map[string]struct{}
	page      int
	limit     int
	sortField string
	sortOrder string
}

func parseListFilters(r *http.Request) listFilters {
	filters := listFilters{
		types:     parseResourceTypesV2(r.URL.Query().Get("type")),
		sources:   parseSourcesV2(r.URL.Query().Get("source")),
		statuses:  parseStatusesV2(r.URL.Query().Get("status")),
		parent:    strings.TrimSpace(r.URL.Query().Get("parent")),
		cluster:   strings.TrimSpace(r.URL.Query().Get("cluster")),
		query:     strings.TrimSpace(strings.ToLower(r.URL.Query().Get("q"))),
		tags:      parseTags(r.URL.Query().Get("tags")),
		page:      parseIntDefault(r.URL.Query().Get("page"), 1),
		limit:     parseIntDefault(r.URL.Query().Get("limit"), 50),
		sortField: strings.TrimSpace(r.URL.Query().Get("sort")),
		sortOrder: strings.TrimSpace(strings.ToLower(r.URL.Query().Get("order"))),
	}
	if filters.page < 1 {
		filters.page = 1
	}
	if filters.limit < 1 {
		filters.limit = 50
	}
	if filters.limit > 100 {
		filters.limit = 100
	}
	if filters.sortField == "" {
		filters.sortField = "name"
	}
	if filters.sortOrder != "desc" {
		filters.sortOrder = "asc"
	}
	return filters
}

func applyFilters(resources []unified.Resource, filters listFilters) []unified.Resource {
	out := make([]unified.Resource, 0, len(resources))
	for _, r := range resources {
		if len(filters.types) > 0 {
			if _, ok := filters.types[r.Type]; !ok {
				continue
			}
		}
		if len(filters.sources) > 0 {
			matched := false
			for _, source := range r.Sources {
				if _, ok := filters.sources[source]; ok {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if len(filters.statuses) > 0 {
			if _, ok := filters.statuses[r.Status]; !ok {
				continue
			}
		}
		if filters.parent != "" {
			if r.ParentID == nil || *r.ParentID != filters.parent {
				continue
			}
		}
		if filters.cluster != "" {
			cluster := strings.ToLower(filters.cluster)
			identityCluster := strings.ToLower(r.Identity.ClusterName)
			proxmoxCluster := ""
			if r.Proxmox != nil {
				proxmoxCluster = strings.ToLower(r.Proxmox.ClusterName)
			}
			if identityCluster != cluster && proxmoxCluster != cluster {
				continue
			}
		}
		if filters.query != "" {
			name := strings.ToLower(r.Name)
			if !strings.Contains(name, filters.query) {
				continue
			}
		}
		if len(filters.tags) > 0 {
			matched := false
			for _, tag := range r.Tags {
				if _, ok := filters.tags[strings.ToLower(tag)]; ok {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

func applySorting(resources []unified.Resource, field, order string) {
	sort.Slice(resources, func(i, j int) bool {
		a := resources[i]
		b := resources[j]
		less := false
		switch field {
		case "status":
			less = a.Status < b.Status
		case "type":
			less = a.Type < b.Type
		case "lastSeen":
			less = a.LastSeen.Before(b.LastSeen)
		default:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		if order == "desc" {
			return !less
		}
		return less
	})
}

func paginate(resources []unified.Resource, page, limit int) ([]unified.Resource, ResourcesMeta) {
	total := len(resources)
	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	paged := resources[start:end]
	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}
	meta := ResourcesMeta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}
	return paged, meta
}

func parseResourceTypesV2(raw string) map[unified.ResourceType]struct{} {
	result := make(map[unified.ResourceType]struct{})
	for _, part := range splitCSV(raw) {
		switch part {
		case "host":
			result[unified.ResourceTypeHost] = struct{}{}
		case "vm":
			result[unified.ResourceTypeVM] = struct{}{}
		case "lxc":
			result[unified.ResourceTypeLXC] = struct{}{}
		case "container":
			result[unified.ResourceTypeContainer] = struct{}{}
		case "storage":
			result[unified.ResourceTypeStorage] = struct{}{}
		case "pbs":
			result[unified.ResourceTypePBS] = struct{}{}
		case "pmg":
			result[unified.ResourceTypePMG] = struct{}{}
		case "ceph":
			result[unified.ResourceTypeCeph] = struct{}{}
		}
	}
	return result
}

func parseSourcesV2(raw string) map[unified.DataSource]struct{} {
	result := make(map[unified.DataSource]struct{})
	for _, part := range splitCSV(raw) {
		switch part {
		case "proxmox":
			result[unified.SourceProxmox] = struct{}{}
		case "agent":
			result[unified.SourceAgent] = struct{}{}
		case "docker":
			result[unified.SourceDocker] = struct{}{}
		case "pbs":
			result[unified.SourcePBS] = struct{}{}
		case "pmg":
			result[unified.SourcePMG] = struct{}{}
		case "kubernetes":
			result[unified.SourceK8s] = struct{}{}
		}
	}
	return result
}

func parseStatusesV2(raw string) map[unified.ResourceStatus]struct{} {
	result := make(map[unified.ResourceStatus]struct{})
	for _, part := range splitCSV(raw) {
		switch part {
		case "online":
			result[unified.StatusOnline] = struct{}{}
		case "offline":
			result[unified.StatusOffline] = struct{}{}
		case "warning":
			result[unified.StatusWarning] = struct{}{}
		case "unknown":
			result[unified.StatusUnknown] = struct{}{}
		}
	}
	return result
}

func parseTags(raw string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, part := range splitCSV(raw) {
		if part == "" {
			continue
		}
		result[strings.ToLower(part)] = struct{}{}
	}
	return result
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func parseIntDefault(raw string, def int) int {
	if raw == "" {
		return def
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return val
}

// getUserID attempts to resolve the user ID for auditing.
func getUserID(r *http.Request) string {
	return auth.GetUser(r.Context())
}
