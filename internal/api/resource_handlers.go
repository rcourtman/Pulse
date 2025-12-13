package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/resources"
)

// StateProvider allows ResourceHandlers to fetch current state on demand.
type StateProvider interface {
	GetState() models.StateSnapshot
}

// ResourceHandlers provides HTTP handlers for the unified resource API.
type ResourceHandlers struct {
	store         *resources.Store
	stateProvider StateProvider
}

// NewResourceHandlers creates resource handlers with a new store.
func NewResourceHandlers() *ResourceHandlers {
	return &ResourceHandlers{
		store: resources.NewStore(),
	}
}

// SetStateProvider sets the state provider for on-demand resource population.
func (h *ResourceHandlers) SetStateProvider(provider StateProvider) {
	h.stateProvider = provider
}

// Store returns the underlying resource store for populating from the monitor.
func (h *ResourceHandlers) Store() *resources.Store {
	return h.store
}

// HandleGetResources returns all resources, optionally filtered by query params.
// GET /api/resources
// Query params:
//   - type: filter by resource type (comma-separated)
//   - platform: filter by platform type (comma-separated)
//   - status: filter by status (comma-separated)
//   - parent: filter by parent ID
//   - infrastructure: if "true", only return infrastructure resources
//   - workloads: if "true", only return workload resources
func (h *ResourceHandlers) HandleGetResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Populate from current state if we have a state provider
	// This ensures fresh data even if the store hasn't been populated yet
	if h.stateProvider != nil {
		h.PopulateFromSnapshot(h.stateProvider.GetState())
	}

	query := h.store.Query()

	// Parse type filter
	if typeParam := r.URL.Query().Get("type"); typeParam != "" {
		types := parseResourceTypes(typeParam)
		if len(types) > 0 {
			query = query.OfType(types...)
		}
	}

	// Parse platform filter
	if platformParam := r.URL.Query().Get("platform"); platformParam != "" {
		platforms := parsePlatformTypes(platformParam)
		if len(platforms) > 0 {
			query = query.FromPlatform(platforms...)
		}
	}

	// Parse status filter
	if statusParam := r.URL.Query().Get("status"); statusParam != "" {
		statuses := parseStatuses(statusParam)
		if len(statuses) > 0 {
			query = query.WithStatus(statuses...)
		}
	}

	// Parse parent filter
	if parentID := r.URL.Query().Get("parent"); parentID != "" {
		query = query.WithParent(parentID)
	}

	// Parse alerts filter
	if r.URL.Query().Get("alerts") == "true" {
		query = query.WithAlerts()
	}

	// Execute query
	var result []resources.Resource

	// Handle infrastructure/workloads shortcut
	if r.URL.Query().Get("infrastructure") == "true" {
		result = h.store.GetInfrastructure()
	} else if r.URL.Query().Get("workloads") == "true" {
		result = h.store.GetWorkloads()
	} else {
		result = query.Execute()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ResourcesResponse{
		Resources: result,
		Count:     len(result),
		Stats:     h.store.GetStats(),
	})
}

// HandleGetResource returns a single resource by ID.
// GET /api/resources/{id}
func (h *ResourceHandlers) HandleGetResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /api/resources/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/resources/")
	if path == "" || path == "/" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}

	resource, ok := h.store.Get(path)
	if !ok {
		http.Error(w, "Resource not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resource)
}

// HandleGetResourceStats returns statistics about the resource store.
// GET /api/resources/stats
func (h *ResourceHandlers) HandleGetResourceStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.store.GetStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// ResourcesResponse is the response for /api/resources.
type ResourcesResponse struct {
	Resources []resources.Resource `json:"resources"`
	Count     int                  `json:"count"`
	Stats     resources.StoreStats `json:"stats"`
}

// PopulateFromState converts all resources from a StateSnapshot to the unified store.
// This should be called whenever the state is updated.
func (h *ResourceHandlers) PopulateFromSnapshot(snapshot models.StateSnapshot) {
	// Convert nodes
	for _, node := range snapshot.Nodes {
		r := resources.FromNode(node)
		h.store.Upsert(r)
	}

	// Convert VMs
	for _, vm := range snapshot.VMs {
		r := resources.FromVM(vm)
		h.store.Upsert(r)
	}

	// Convert containers
	for _, ct := range snapshot.Containers {
		r := resources.FromContainer(ct)
		h.store.Upsert(r)
	}

	// Convert hosts
	for _, host := range snapshot.Hosts {
		r := resources.FromHost(host)
		h.store.Upsert(r)
	}

	// Convert docker hosts and their containers
	for _, dh := range snapshot.DockerHosts {
		r := resources.FromDockerHost(dh)
		h.store.Upsert(r)

		// Convert containers within the docker host
		for _, dc := range dh.Containers {
			r := resources.FromDockerContainer(dc, dh.ID, dh.Hostname)
			h.store.Upsert(r)
		}
	}

	// Convert PBS instances
	for _, pbs := range snapshot.PBSInstances {
		r := resources.FromPBSInstance(pbs)
		h.store.Upsert(r)
	}

	// Convert storage
	for _, storage := range snapshot.Storage {
		r := resources.FromStorage(storage)
		h.store.Upsert(r)
	}
}

// Helper functions for parsing query parameters

func parseResourceTypes(s string) []resources.ResourceType {
	parts := strings.Split(s, ",")
	var result []resources.ResourceType
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		result = append(result, resources.ResourceType(p))
	}
	return result
}

func parsePlatformTypes(s string) []resources.PlatformType {
	parts := strings.Split(s, ",")
	var result []resources.PlatformType
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		result = append(result, resources.PlatformType(p))
	}
	return result
}

func parseStatuses(s string) []resources.ResourceStatus {
	parts := strings.Split(s, ",")
	var result []resources.ResourceStatus
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		result = append(result, resources.ResourceStatus(p))
	}
	return result
}
