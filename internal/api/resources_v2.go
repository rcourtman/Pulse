package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
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
	supplementalMu      sync.RWMutex
	supplementalRecords map[unified.DataSource]SupplementalRecordsProvider
	stateProvider       StateProvider
	tenantStateProvider TenantStateProvider
}

// SupplementalRecordsProvider provides out-of-band ingest records for a specific source.
type SupplementalRecordsProvider interface {
	GetCurrentRecords() []unified.IngestRecord
}

// NewResourceV2Handlers creates a new handler.
func NewResourceV2Handlers(cfg *config.Config) *ResourceV2Handlers {
	return &ResourceV2Handlers{
		cfg:                 cfg,
		stores:              make(map[string]unified.ResourceStore),
		registryCache:       make(map[string]registryCacheEntry),
		supplementalRecords: make(map[unified.DataSource]SupplementalRecordsProvider),
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

// SetSupplementalRecordsProvider configures additional records for a source.
func (h *ResourceV2Handlers) SetSupplementalRecordsProvider(source unified.DataSource, provider SupplementalRecordsProvider) {
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
	attachDiscoveryTargets(paged)

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

	resourceCopy := *resource
	attachDiscoveryTarget(&resourceCopy)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resourceCopy)
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
	if strings.HasSuffix(r.URL.Path, "/report-merge") {
		h.HandleReportMerge(w, r)
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

// HandleReportMerge handles POST /api/v2/resources/{id}/report-merge.
func (h *ResourceV2Handlers) HandleReportMerge(w http.ResponseWriter, r *http.Request) {
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		exclusionsAdded += 1
	}

	if exclusionsAdded == 0 {
		http.Error(w, "No exclusions created", http.StatusBadRequest)
		return
	}

	log.Printf("v2 report-merge: resource=%s exclusions=%d user=%s sources=%v", path, exclusionsAdded, getUserID(r), payload.Sources)
	h.invalidateCache(orgID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":     "ok",
		"message":    "Merge reported",
		"exclusions": exclusionsAdded,
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
		records := provider.GetCurrentRecords()
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
		case "host", "hosts":
			result[unified.ResourceTypeHost] = struct{}{}
		case "vm", "vms", "qemu":
			result[unified.ResourceTypeVM] = struct{}{}
		case "lxc", "lxcs":
			result[unified.ResourceTypeLXC] = struct{}{}
		case "container", "containers", "docker_container", "docker-container":
			result[unified.ResourceTypeContainer] = struct{}{}
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
		case "truenas":
			result[unified.SourceTrueNAS] = struct{}{}
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
	case unified.ResourceTypeK8sCluster:
		return kubernetesDiscoveryTarget(resource, "cluster")
	case unified.ResourceTypeK8sNode:
		return kubernetesDiscoveryTarget(resource, "node")
	case unified.ResourceTypePod:
		return kubernetesDiscoveryTarget(resource, "pod")
	case unified.ResourceTypeK8sDeployment:
		return kubernetesDiscoveryTarget(resource, "deployment")
	default:
		return nil
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
