package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type storageConfigItem struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Instance string   `json:"instance,omitempty"`
	Type     string   `json:"type,omitempty"`
	Content  string   `json:"content,omitempty"`
	Nodes    []string `json:"nodes,omitempty"`
	Path     string   `json:"path,omitempty"`
	Shared   bool     `json:"shared"`
	Enabled  bool     `json:"enabled"`
	Active   bool     `json:"active"`
}

type storageConfigResponse struct {
	Storages []storageConfigItem `json:"storages,omitempty"`
}

func (r *Router) handleStorageConfig(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET method is allowed", nil)
		return
	}

	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "not_initialized", "Monitoring service not initialized", nil)
		return
	}

	query := req.URL.Query()
	instance := strings.TrimSpace(query.Get("instance"))
	storageID := strings.TrimSpace(query.Get("storage_id"))
	node := strings.TrimSpace(query.Get("node"))

	configs, err := monitor.GetStorageConfig(req.Context(), instance)
	if err != nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "storage_config_unavailable", err.Error(), nil)
		return
	}

	response := storageConfigResponse{Storages: []storageConfigItem{}}
	seen := make(map[string]bool)

	for inst, storages := range configs {
		for _, storage := range storages {
			if storageID != "" && !strings.EqualFold(storage.Storage, storageID) {
				continue
			}

			nodes := parseStorageConfigNodes(storage)
			if node != "" && !storageConfigMatchesNode(nodes, node) {
				continue
			}

			key := inst + ":" + storage.Storage
			if seen[key] {
				continue
			}
			seen[key] = true

			response.Storages = append(response.Storages, storageConfigItem{
				ID:       storage.Storage,
				Name:     storage.Storage,
				Instance: inst,
				Type:     storage.Type,
				Content:  storage.Content,
				Nodes:    nodes,
				Path:     storage.Path,
				Shared:   storage.Shared == 1,
				Enabled:  storage.Enabled == 1,
				Active:   storage.Active == 1,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "encoding_error", "Failed to encode response", nil)
	}
}

func parseStorageConfigNodes(storage proxmox.Storage) []string {
	nodes := strings.TrimSpace(storage.Nodes)
	if nodes == "" {
		return nil
	}

	parts := strings.Split(nodes, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		node := strings.TrimSpace(part)
		if node == "" {
			continue
		}
		result = append(result, node)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func storageConfigMatchesNode(nodes []string, filter string) bool {
	if filter == "" {
		return true
	}
	if len(nodes) == 0 {
		return true
	}
	for _, node := range nodes {
		if strings.EqualFold(node, filter) {
			return true
		}
	}
	return false
}
