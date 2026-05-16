package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

// ReplicationJobsResponse is the canonical payload for
// `GET /api/replication/jobs`. Replication jobs are zfs send/receive
// jobs scheduled between Proxmox VE nodes; they are tracked on the
// monitor's `State.ReplicationJobs` slice but were not previously
// exposed through any v6 API surface. The Proxmox platform-page
// Replication tab is the first consumer.
type ReplicationJobsResponse struct {
	Data []models.ReplicationJob `json:"data"`
	Meta ReplicationJobsMeta     `json:"meta"`
}

type ReplicationJobsMeta struct {
	Total int `json:"total"`
}

func (r *Router) handleListReplicationJobs(w http.ResponseWriter, req *http.Request) {
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

	jobs := monitor.ReplicationJobsSnapshot()
	filtered := filterReplicationJobs(jobs, req)

	response := ReplicationJobsResponse{
		Data: filtered,
		Meta: ReplicationJobsMeta{Total: len(filtered)},
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to encode replication jobs response")
		writeErrorResponse(w, http.StatusInternalServerError, "encoding_error",
			"Failed to encode replication jobs", nil)
	}
}

// filterReplicationJobs narrows the snapshot by optional query params:
//
//	platform=proxmox-pve|proxmox|pve — only PVE jobs (the only emitter today)
//	instance=<name>                  — only jobs from a given PVE instance
//	guest=<id-or-name>               — only jobs scheduled for a guest
//
// Unknown platform tokens return an empty list rather than the whole
// dataset so caller typos don't silently hide a mismatch.
func filterReplicationJobs(jobs []models.ReplicationJob, req *http.Request) []models.ReplicationJob {
	query := req.URL.Query()
	platform := strings.TrimSpace(query.Get("platform"))
	instance := strings.TrimSpace(query.Get("instance"))
	guest := strings.TrimSpace(strings.ToLower(query.Get("guest")))

	if platform == "" && instance == "" && guest == "" {
		out := make([]models.ReplicationJob, len(jobs))
		copy(out, jobs)
		return out
	}

	if platform != "" {
		switch strings.ToLower(platform) {
		case "proxmox", "proxmox-pve", "pve":
		default:
			return []models.ReplicationJob{}
		}
	}

	out := make([]models.ReplicationJob, 0, len(jobs))
	for _, job := range jobs {
		if instance != "" && !strings.EqualFold(strings.TrimSpace(job.Instance), instance) {
			continue
		}
		if guest != "" {
			match := false
			if job.GuestID != 0 && strconv.Itoa(job.GuestID) == guest {
				match = true
			}
			if !match && strings.EqualFold(strings.TrimSpace(job.GuestName), guest) {
				match = true
			}
			if !match && strings.EqualFold(strings.TrimSpace(job.Guest), guest) {
				match = true
			}
			if !match {
				continue
			}
		}
		out = append(out, job)
	}
	return out
}

func (r *Router) registerReplicationRoutes() {
	r.mux.HandleFunc("/api/replication/jobs", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleListReplicationJobs)))
}
