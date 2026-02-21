package api

import (
	"encoding/json"
	"errors"
	"io"
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
	"github.com/rs/zerolog/log"
)

// ResourceHandlers provides HTTP handlers for the unified resource API.
type ResourceHandlers struct {
	cfg                 *config.Config
	storeMu             sync.Mutex
	stores              map[string]unified.ResourceStore
	cacheMu             sync.Mutex
	registryCache       map[string]registryCacheEntry
	supplementalMu      sync.RWMutex
	supplementalRecords map[unified.DataSource]SupplementalRecordsProvider
	stateProvider       StateProvider
	tenantStateProvider TenantStateProvider
}

// SupplementalRecordsProvider provides out-of-band ingest records for a specific source.
type SupplementalRecordsProvider interface {
	GetCurrentRecords() []unified.IngestRecord
}

// TenantSupplementalRecordsProvider is an optional interface for providers that can scope
// supplemental records to a specific organization. This prevents cross-tenant leakage when
// Pulse runs in multi-tenant mode.
//
// Providers that do not implement this interface will be treated as "global"/legacy and
// their records will be ingested into every tenant registry.
type TenantSupplementalRecordsProvider interface {
	GetCurrentRecordsForOrg(orgID string) []unified.IngestRecord
}

// NewResourceHandlers creates a new ResourceHandlers.
func NewResourceHandlers(cfg *config.Config) *ResourceHandlers {
	return &ResourceHandlers{
		cfg:                 cfg,
		stores:              make(map[string]unified.ResourceStore),
		registryCache:       make(map[string]registryCacheEntry),
		supplementalRecords: make(map[unified.DataSource]SupplementalRecordsProvider),
	}
}

// SetStateProvider sets the state provider for on-demand population.
func (h *ResourceHandlers) SetStateProvider(provider StateProvider) {
	h.stateProvider = provider
}

// SetTenantStateProvider sets the tenant-aware provider.
func (h *ResourceHandlers) SetTenantStateProvider(provider TenantStateProvider) {
	h.tenantStateProvider = provider
}

// SetSupplementalRecordsProvider configures additional records for a source.
func (h *ResourceHandlers) SetSupplementalRecordsProvider(source unified.DataSource, provider SupplementalRecordsProvider) {
	h.supplementalMu.Lock()
	if h.supplementalRecords == nil {
		h.supplementalRecords = make(map[unified.DataSource]SupplementalRecordsProvider)
	}
	if provider == nil {
		delete(h.supplementalRecords, source)
	} else {
		h.supplementalRecords[source] = provider
	}
	h.supplementalMu.Unlock()

	// Provider changes alter ingestion inputs, so clear all cached registries.
	h.cacheMu.Lock()
	h.registryCache = make(map[string]registryCacheEntry)
	h.cacheMu.Unlock()
}

// HandleListResources handles GET /api/resources.
func (h *ResourceHandlers) HandleListResources(w http.ResponseWriter, r *http.Request) {
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
	attachDiscoveryTargets(paged)
	attachMetricsTargets(paged, registry)
	pruneResourcesForListResponse(paged)

	response := ResourcesResponse{
		Data:         paged,
		Meta:         meta,
		Aggregations: registry.Stats(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// pruneResourcesForListResponse removes heavy, platform-specific fields that would bloat
// the list response. Detail drawers can fetch full payloads via GET /api/resources/{id}.
func pruneResourcesForListResponse(resources []unified.Resource) {
	for i := range resources {
		pruneResourceForListResponse(&resources[i])
	}
}

func pruneResourceForListResponse(resource *unified.Resource) {
	if resource == nil {
		return
	}

	// PMG domain stats can be very large; keep summary-only in list.
	if resource.PMG != nil {
		resource.PMG.RelayDomains = nil
		resource.PMG.DomainStats = nil
		resource.PMG.DomainStatsAsOf = time.Time{}
	}
}

// HandleGetResource handles GET /api/resources/{id}.
func (h *ResourceHandlers) HandleGetResource(w http.ResponseWriter, r *http.Request) {
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

	resourceID := strings.TrimPrefix(r.URL.Path, "/api/resources/")
	resourceID = strings.TrimSuffix(resourceID, "/")
	if resourceID == "" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}

	resource, ok := registry.Get(resourceID)
	if !ok {
		http.Error(w, "Resource not found", http.StatusNotFound)
		return
	}

	resourceCopy := *resource
	attachDiscoveryTarget(&resourceCopy)
	attachMetricsTarget(&resourceCopy, registry)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resourceCopy)
}

// HandleResourceRoutes dispatches nested resource routes.
func (h *ResourceHandlers) HandleResourceRoutes(w http.ResponseWriter, r *http.Request) {
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
	if strings.HasSuffix(r.URL.Path, "/report-merge") {
		h.HandleReportMerge(w, r)
		return
	}
	h.HandleGetResource(w, r)
}

// HandleGetChildren handles GET /api/resources/{id}/children.
func (h *ResourceHandlers) HandleGetChildren(w http.ResponseWriter, r *http.Request) {
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

	path := strings.TrimPrefix(r.URL.Path, "/api/resources/")
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

// HandleGetMetrics handles GET /api/resources/{id}/metrics.
func (h *ResourceHandlers) HandleGetMetrics(w http.ResponseWriter, r *http.Request) {
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

	path := strings.TrimPrefix(r.URL.Path, "/api/resources/")
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

// HandleStats handles GET /api/resources/stats.
func (h *ResourceHandlers) HandleStats(w http.ResponseWriter, r *http.Request) {
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

// HandleLink handles POST /api/resources/{id}/link.
func (h *ResourceHandlers) HandleLink(w http.ResponseWriter, r *http.Request) {
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

	path := strings.TrimPrefix(r.URL.Path, "/api/resources/")
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

// HandleUnlink handles POST /api/resources/{id}/unlink.
func (h *ResourceHandlers) HandleUnlink(w http.ResponseWriter, r *http.Request) {
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

	path := strings.TrimPrefix(r.URL.Path, "/api/resources/")
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

// HandleReportMerge handles POST /api/resources/{id}/report-merge.
func (h *ResourceHandlers) HandleReportMerge(w http.ResponseWriter, r *http.Request) {
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

	path := strings.TrimPrefix(r.URL.Path, "/api/resources/")
	path = strings.TrimSuffix(path, "/report-merge")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}

	var payload struct {
		Sources []string `json:"sources"`
		Notes   string   `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	registry, err := h.buildRegistry(orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resource, ok := registry.Get(path)
	if !ok {
		http.Error(w, "Resource not found", http.StatusNotFound)
		return
	}

	if len(resource.Sources) < 2 {
		http.Error(w, "Resource is not merged", http.StatusBadRequest)
		return
	}

	sourceTargets := registry.SourceTargets(path)
	if len(sourceTargets) == 0 {
		http.Error(w, "No source targets found", http.StatusBadRequest)
		return
	}

	filteredSources := make(map[string]struct{})
	for _, source := range payload.Sources {
		filteredSources[strings.ToLower(strings.TrimSpace(source))] = struct{}{}
	}

	reason := strings.TrimSpace(payload.Notes)
	if reason == "" {
		reason = "reported_incorrect_merge"
	}

	exclusionsAdded := 0
	seen := make(map[string]struct{})
	for _, target := range sourceTargets {
		if len(filteredSources) > 0 {
			if _, ok := filteredSources[strings.ToLower(string(target.Source))]; !ok {
				continue
			}
		}
		if target.CandidateID == "" || target.CandidateID == path {
			continue
		}
		key := target.CandidateID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		exclusion := unified.ResourceExclusion{
			ResourceA: path,
			ResourceB: target.CandidateID,
			Reason:    reason,
			CreatedBy: getUserID(r),
			CreatedAt: time.Now().UTC(),
		}
		if err := store.AddExclusion(exclusion); err != nil {
			log.Error().
				Err(err).
				Str("orgID", orgID).
				Str("resourceID", path).
				Str("candidateID", target.CandidateID).
				Msg("Failed to add resource merge exclusion")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		exclusionsAdded += 1
	}

	if exclusionsAdded == 0 {
		http.Error(w, "No exclusions created", http.StatusBadRequest)
		return
	}

	log.Info().
		Str("orgID", orgID).
		Str("resourceID", path).
		Int("exclusionsAdded", exclusionsAdded).
		Str("userID", getUserID(r)).
		Strs("sources", payload.Sources).
		Msg("Reported resource merge issue")
	h.invalidateCache(orgID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":     "ok",
		"message":    "Merge reported",
		"exclusions": exclusionsAdded,
	})
}

// buildRegistry constructs a registry for the current tenant.
func (h *ResourceHandlers) buildRegistry(orgID string) (*unified.ResourceRegistry, error) {
	store, err := h.getStore(orgID)
	if err != nil {
		return nil, err
	}
	key := cacheKey(orgID)

	var snapshot models.StateSnapshot
	if orgID != "" && orgID != "default" {
		// Security: non-default orgs must use tenant-scoped state only.
		if h.tenantStateProvider == nil {
			return nil, errors.New("tenant state provider unavailable")
		}
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
	h.supplementalMu.RLock()
	supplementalProviders := make(map[unified.DataSource]SupplementalRecordsProvider, len(h.supplementalRecords))
	for source, provider := range h.supplementalRecords {
		supplementalProviders[source] = provider
	}
	h.supplementalMu.RUnlock()
	for source, provider := range supplementalProviders {
		if provider == nil {
			continue
		}
		var records []unified.IngestRecord
		if tenantProvider, ok := any(provider).(TenantSupplementalRecordsProvider); ok {
			records = tenantProvider.GetCurrentRecordsForOrg(orgID)
		} else {
			records = provider.GetCurrentRecords()
		}
		if len(records) == 0 {
			continue
		}
		registry.IngestRecords(source, records)
	}

	h.cacheMu.Lock()
	h.registryCache[key] = registryCacheEntry{registry: registry, lastUpdate: snapshot.LastUpdate}
	h.cacheMu.Unlock()

	return registry, nil
}

func (h *ResourceHandlers) getStore(orgID string) (unified.ResourceStore, error) {
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

func (h *ResourceHandlers) invalidateCache(orgID string) {
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

// ResourcesResponse represents the list response for the unified resources API.
type ResourcesResponse struct {
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
	namespace string
	query     string
	tags      map[string]struct{}
	page      int
	limit     int
	sortField string
	sortOrder string
}

func parseListFilters(r *http.Request) listFilters {
	filters := listFilters{
		types:     parseResourceTypes(r.URL.Query().Get("type")),
		sources:   parseSources(r.URL.Query().Get("source")),
		statuses:  parseStatuses(r.URL.Query().Get("status")),
		parent:    strings.TrimSpace(r.URL.Query().Get("parent")),
		cluster:   strings.TrimSpace(r.URL.Query().Get("cluster")),
		namespace: strings.TrimSpace(r.URL.Query().Get("namespace")),
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
			dockerSwarmClusterName := ""
			dockerSwarmClusterID := ""
			if r.Docker != nil && r.Docker.Swarm != nil {
				dockerSwarmClusterName = strings.ToLower(strings.TrimSpace(r.Docker.Swarm.ClusterName))
				dockerSwarmClusterID = strings.ToLower(strings.TrimSpace(r.Docker.Swarm.ClusterID))
			}

			if identityCluster != cluster && proxmoxCluster != cluster && dockerSwarmClusterName != cluster && dockerSwarmClusterID != cluster {
				continue
			}
		}
		if filters.namespace != "" {
			if r.Kubernetes == nil {
				continue
			}
			want := strings.ToLower(filters.namespace)
			got := strings.ToLower(strings.TrimSpace(r.Kubernetes.Namespace))
			if got != want {
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

func parseResourceTypes(raw string) map[unified.ResourceType]struct{} {
	result := make(map[unified.ResourceType]struct{})
	for _, part := range splitCSV(raw) {
		switch part {
		case "host", "hosts":
			result[unified.ResourceTypeHost] = struct{}{}
		case "vm", "vms", "qemu":
			result[unified.ResourceTypeVM] = struct{}{}
		case "lxc", "lxcs":
			result[unified.ResourceTypeLXC] = struct{}{}
		case "container", "containers", "docker_container", "docker-container":
			result[unified.ResourceTypeContainer] = struct{}{}
		case "docker_service", "docker-service", "swarm_service", "swarm-service", "service", "services":
			result[unified.ResourceTypeDockerService] = struct{}{}
		case "pod", "pods", "k8s_pod", "k8s-pod", "kubernetes_pod", "kubernetes-pod":
			result[unified.ResourceTypePod] = struct{}{}
		case "k8s_cluster", "k8s-cluster", "kubernetes_cluster", "kubernetes-cluster":
			result[unified.ResourceTypeK8sCluster] = struct{}{}
		case "k8s_node", "k8s-node", "kubernetes_node", "kubernetes-node":
			result[unified.ResourceTypeK8sNode] = struct{}{}
		case "k8s_deployment", "k8s-deployment", "kubernetes_deployment", "kubernetes-deployment", "deployment", "deployments":
			result[unified.ResourceTypeK8sDeployment] = struct{}{}
		case "k8s", "kubernetes":
			result[unified.ResourceTypeK8sCluster] = struct{}{}
			result[unified.ResourceTypeK8sNode] = struct{}{}
			result[unified.ResourceTypePod] = struct{}{}
			result[unified.ResourceTypeK8sDeployment] = struct{}{}
		case "storage":
			result[unified.ResourceTypeStorage] = struct{}{}
		case "pbs":
			result[unified.ResourceTypePBS] = struct{}{}
		case "pmg":
			result[unified.ResourceTypePMG] = struct{}{}
		case "ceph":
			result[unified.ResourceTypeCeph] = struct{}{}
		case "physical_disk", "physical-disk", "physicaldisk", "disk":
			result[unified.ResourceTypePhysicalDisk] = struct{}{}
		}
	}
	return result
}

func parseSources(raw string) map[unified.DataSource]struct{} {
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
		case "truenas":
			result[unified.SourceTrueNAS] = struct{}{}
		}
	}
	return result
}

func parseStatuses(raw string) map[unified.ResourceStatus]struct{} {
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

func attachDiscoveryTargets(resources []unified.Resource) {
	for i := range resources {
		attachDiscoveryTarget(&resources[i])
	}
}

func attachDiscoveryTarget(resource *unified.Resource) {
	if resource == nil {
		return
	}
	resource.DiscoveryTarget = buildDiscoveryTarget(*resource)
}

func attachMetricsTargets(resources []unified.Resource, registry *unified.ResourceRegistry) {
	for i := range resources {
		attachMetricsTarget(&resources[i], registry)
	}
}

func attachMetricsTarget(resource *unified.Resource, registry *unified.ResourceRegistry) {
	if resource == nil || registry == nil {
		return
	}
	resource.MetricsTarget = buildMetricsTarget(*resource, registry)
}

func buildMetricsTarget(resource unified.Resource, registry *unified.ResourceRegistry) *unified.MetricsTarget {
	sourceTargets := registry.SourceTargets(resource.ID)
	if len(sourceTargets) == 0 {
		return nil
	}

	// Build a map from source to source target for quick lookup.
	bySource := make(map[unified.DataSource]unified.SourceTarget, len(sourceTargets))
	for _, st := range sourceTargets {
		bySource[st.Source] = st
	}

	switch resource.Type {
	case unified.ResourceTypeHost:
		// Infrastructure hosts: prefer Proxmox > Agent > Docker source.
		if st, ok := bySource[unified.SourceProxmox]; ok {
			return &unified.MetricsTarget{ResourceType: "node", ResourceID: st.SourceID}
		}
		if st, ok := bySource[unified.SourceAgent]; ok {
			return &unified.MetricsTarget{ResourceType: "host", ResourceID: st.SourceID}
		}
		if st, ok := bySource[unified.SourceDocker]; ok {
			return &unified.MetricsTarget{ResourceType: "dockerHost", ResourceID: st.SourceID}
		}
	case unified.ResourceTypeVM:
		if st, ok := bySource[unified.SourceProxmox]; ok {
			return &unified.MetricsTarget{ResourceType: "guest", ResourceID: st.SourceID}
		}
	case unified.ResourceTypeLXC:
		if st, ok := bySource[unified.SourceProxmox]; ok {
			return &unified.MetricsTarget{ResourceType: "guest", ResourceID: st.SourceID}
		}
	case unified.ResourceTypeContainer:
		if st, ok := bySource[unified.SourceDocker]; ok {
			return &unified.MetricsTarget{ResourceType: "docker", ResourceID: st.SourceID}
		}
	case unified.ResourceTypeStorage, unified.ResourceTypeCeph:
		if st, ok := bySource[unified.SourceProxmox]; ok {
			return &unified.MetricsTarget{ResourceType: "storage", ResourceID: st.SourceID}
		}
		if st, ok := bySource[unified.SourceTrueNAS]; ok {
			return &unified.MetricsTarget{ResourceType: "storage", ResourceID: st.SourceID}
		}
	case unified.ResourceTypePhysicalDisk:
		if st, ok := bySource[unified.SourceProxmox]; ok {
			return &unified.MetricsTarget{ResourceType: "disk", ResourceID: st.SourceID}
		}
	case unified.ResourceTypePod:
		if st, ok := bySource[unified.SourceK8s]; ok {
			return &unified.MetricsTarget{ResourceType: "k8s", ResourceID: st.SourceID}
		}
	case unified.ResourceTypeK8sCluster, unified.ResourceTypeK8sNode:
		if st, ok := bySource[unified.SourceK8s]; ok {
			return &unified.MetricsTarget{ResourceType: "k8s", ResourceID: st.SourceID}
		}
	case unified.ResourceTypePBS:
		if st, ok := bySource[unified.SourcePBS]; ok {
			return &unified.MetricsTarget{ResourceType: "node", ResourceID: st.SourceID}
		}
	case unified.ResourceTypePMG:
		if st, ok := bySource[unified.SourcePMG]; ok {
			return &unified.MetricsTarget{ResourceType: "node", ResourceID: st.SourceID}
		}
	}

	return nil
}

func buildDiscoveryTarget(resource unified.Resource) *unified.DiscoveryTarget {
	switch resource.Type {
	case unified.ResourceTypeHost:
		return hostDiscoveryTarget(resource)
	case unified.ResourceTypeVM:
		return proxmoxGuestDiscoveryTarget(resource, "vm")
	case unified.ResourceTypeLXC:
		return proxmoxGuestDiscoveryTarget(resource, "lxc")
	case unified.ResourceTypePBS:
		return hostDiscoveryTarget(resource)
	case unified.ResourceTypePMG:
		return hostDiscoveryTarget(resource)
	case unified.ResourceTypeCeph:
		return cephDiscoveryTarget(resource)
	case unified.ResourceTypeK8sCluster:
		return kubernetesDiscoveryTarget(resource, "cluster")
	case unified.ResourceTypeK8sNode:
		return kubernetesDiscoveryTarget(resource, "node")
	case unified.ResourceTypePod:
		return kubernetesDiscoveryTarget(resource, "pod")
	case unified.ResourceTypeK8sDeployment:
		return kubernetesDiscoveryTarget(resource, "deployment")
	case unified.ResourceTypePhysicalDisk:
		return physicalDiskDiscoveryTarget(resource)
	default:
		return nil
	}
}

func cephDiscoveryTarget(resource unified.Resource) *unified.DiscoveryTarget {
	if resource.Ceph == nil {
		return nil
	}
	hostID := firstNonEmptyTrimmed(
		resource.Ceph.FSID,
		resource.Name,
		resource.ID,
	)
	if hostID == "" {
		return nil
	}
	return &unified.DiscoveryTarget{
		ResourceType: "ceph",
		HostID:       hostID,
		ResourceID:   resource.ID,
		Hostname:     resource.Name,
	}
}

func hostDiscoveryTarget(resource unified.Resource) *unified.DiscoveryTarget {
	linkedHostAgentID := ""
	if hasSource(resource.Sources, unified.SourceAgent) || resource.Agent != nil {
		linkedHostAgentID = proxmoxLinkedHostAgentID(resource.Proxmox)
	}
	hostID := firstNonEmptyTrimmed(
		agentID(resource.Agent),
		linkedHostAgentID,
		proxmoxNodeName(resource.Proxmox),
		agentHostname(resource.Agent),
		dockerHostname(resource.Docker),
		pbsHostname(resource.PBS),
		pmgHostname(resource.PMG),
		firstResourceHostname(resource),
		resource.Name,
		resource.ID,
	)
	if hostID == "" {
		return nil
	}
	return &unified.DiscoveryTarget{
		ResourceType: "host",
		HostID:       hostID,
		ResourceID:   hostID,
		Hostname: firstNonEmptyTrimmed(
			agentHostname(resource.Agent),
			proxmoxNodeName(resource.Proxmox),
			dockerHostname(resource.Docker),
			pbsHostname(resource.PBS),
			pmgHostname(resource.PMG),
			firstResourceHostname(resource),
			resource.Name,
			hostID,
		),
	}
}

func proxmoxGuestDiscoveryTarget(resource unified.Resource, resourceType string) *unified.DiscoveryTarget {
	if resource.Proxmox == nil || resource.Proxmox.NodeName == "" || resource.Proxmox.VMID == 0 {
		return nil
	}
	resourceID := strconv.Itoa(resource.Proxmox.VMID)
	hostID := strings.TrimSpace(resource.Proxmox.NodeName)
	if hostID == "" || resourceID == "" {
		return nil
	}
	return &unified.DiscoveryTarget{
		ResourceType: resourceType,
		HostID:       hostID,
		ResourceID:   resourceID,
		Hostname: firstNonEmptyTrimmed(
			firstResourceHostname(resource),
			resource.Name,
			resourceID,
		),
	}
}

func kubernetesDiscoveryTarget(resource unified.Resource, kind string) *unified.DiscoveryTarget {
	if resource.Kubernetes == nil {
		return nil
	}

	hostID := firstNonEmptyTrimmed(
		resource.Kubernetes.AgentID,
		resource.Kubernetes.ClusterID,
		resource.Kubernetes.ClusterName,
	)
	if hostID == "" {
		return nil
	}

	resourceID := ""
	switch kind {
	case "cluster":
		resourceID = firstNonEmptyTrimmed(
			resource.Kubernetes.ClusterID,
			resource.Kubernetes.ClusterName,
			resource.Name,
		)
	case "node":
		resourceID = firstNonEmptyTrimmed(
			resource.Kubernetes.NodeUID,
			resource.Kubernetes.NodeName,
			resource.Name,
		)
	case "deployment":
		resourceID = firstNonEmptyTrimmed(
			resource.Kubernetes.DeploymentUID,
			kubernetesNamespacedName(resource.Kubernetes.Namespace, resource.Name),
			resource.Name,
		)
	default:
		resourceID = firstNonEmptyTrimmed(
			resource.Kubernetes.PodUID,
			kubernetesNamespacedName(resource.Kubernetes.Namespace, resource.Name),
			resource.Name,
		)
	}
	if resourceID == "" {
		return nil
	}

	return &unified.DiscoveryTarget{
		ResourceType: "k8s",
		HostID:       hostID,
		ResourceID:   resourceID,
		Hostname: firstNonEmptyTrimmed(
			resource.Kubernetes.ClusterName,
			resource.Kubernetes.Context,
			resource.Name,
		),
	}
}

func physicalDiskDiscoveryTarget(resource unified.Resource) *unified.DiscoveryTarget {
	if resource.PhysicalDisk == nil {
		return nil
	}
	hostID := ""
	if resource.Proxmox != nil {
		hostID = resource.Proxmox.NodeName
	}
	if hostID == "" {
		hostID = firstNonEmptyTrimmed(firstResourceHostname(resource), resource.Name)
	}
	if hostID == "" {
		return nil
	}
	return &unified.DiscoveryTarget{
		ResourceType: "disk",
		HostID:       hostID,
		ResourceID:   resource.ID,
		Hostname:     hostID,
	}
}

func kubernetesNamespacedName(namespace, name string) string {
	ns := strings.TrimSpace(namespace)
	n := strings.TrimSpace(name)
	if ns == "" {
		return n
	}
	if n == "" {
		return ns
	}
	return ns + "/" + n
}

func firstResourceHostname(resource unified.Resource) string {
	for _, hostname := range resource.Identity.Hostnames {
		trimmed := strings.TrimSpace(hostname)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func hasSource(sources []unified.DataSource, target unified.DataSource) bool {
	for _, source := range sources {
		if source == target {
			return true
		}
	}
	return false
}

func agentID(agent *unified.AgentData) string {
	if agent == nil {
		return ""
	}
	return agent.AgentID
}

func agentHostname(agent *unified.AgentData) string {
	if agent == nil {
		return ""
	}
	return agent.Hostname
}

func proxmoxLinkedHostAgentID(proxmox *unified.ProxmoxData) string {
	if proxmox == nil {
		return ""
	}
	return proxmox.LinkedHostAgentID
}

func proxmoxNodeName(proxmox *unified.ProxmoxData) string {
	if proxmox == nil {
		return ""
	}
	return proxmox.NodeName
}

func dockerHostname(docker *unified.DockerData) string {
	if docker == nil {
		return ""
	}
	return docker.Hostname
}

func pbsHostname(pbs *unified.PBSData) string {
	if pbs == nil {
		return ""
	}
	return pbs.Hostname
}

func pmgHostname(pmg *unified.PMGData) string {
	if pmg == nil {
		return ""
	}
	return pmg.Hostname
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
