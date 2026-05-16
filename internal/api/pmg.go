package api

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

// PMGInstancesResponse is the canonical payload for
// `GET /api/pmg/instances`. It exposes the full Proxmox Mail Gateway
// instance snapshot — per-node cluster status with postfix queue
// detail, full mail stats (in/out, bytes, bounces, RBL/pregreet),
// quarantine breakdown by category, spam score distribution, top
// domain stats, and relay-domain configuration. The Mail Gateway
// platform-page drawer is the first consumer; the row table keeps
// reading the slim ResourcePMGMeta projection through /api/resources.
type PMGInstancesResponse struct {
	Data []models.PMGInstance `json:"data"`
	Meta PMGInstancesMeta     `json:"meta"`
}

type PMGInstancesMeta struct {
	Total int `json:"total"`
}

func (r *Router) handleListPMGInstances(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
			"Only GET method is allowed", nil)
		return
	}

	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "no_monitor",
			"Monitor not available", nil)
		return
	}

	state := monitor.GetState()
	instances := filterPMGInstances(state.PMGInstances, req)

	response := PMGInstancesResponse{
		Data: instances,
		Meta: PMGInstancesMeta{Total: len(instances)},
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to encode PMG instances response")
		writeErrorResponse(w, http.StatusInternalServerError, "encoding_error",
			"Failed to encode PMG instances", nil)
	}
}

// filterPMGInstances narrows the snapshot by optional `id=` or
// `name=` query parameters. With neither, returns the full list.
func filterPMGInstances(instances []models.PMGInstance, req *http.Request) []models.PMGInstance {
	query := req.URL.Query()
	idFilter := strings.TrimSpace(query.Get("id"))
	nameFilter := strings.TrimSpace(query.Get("name"))

	if idFilter == "" && nameFilter == "" {
		out := make([]models.PMGInstance, len(instances))
		copy(out, instances)
		return out
	}

	out := make([]models.PMGInstance, 0, len(instances))
	for _, inst := range instances {
		if idFilter != "" && !strings.EqualFold(strings.TrimSpace(inst.ID), idFilter) {
			continue
		}
		if nameFilter != "" && !strings.EqualFold(strings.TrimSpace(inst.Name), nameFilter) {
			continue
		}
		out = append(out, inst)
	}
	return out
}

func (r *Router) registerPMGRoutes() {
	r.mux.HandleFunc("/api/pmg/instances", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleListPMGInstances)))
}
