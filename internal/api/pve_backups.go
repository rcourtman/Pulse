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

// PVEBackupsResponse is the canonical payload for
// `GET /api/backups/pve`. It exposes the three Proxmox VE backup
// classes — recent vzdump tasks, vzdump files on storage, and
// qm/pct snapshots — that were previously only carried inside the
// internal `State` snapshot and were never reachable through a v6
// API surface. The Proxmox platform-page Backups tab is the first
// consumer; PBS-resident backups remain on their own platform page.
type PVEBackupsResponse struct {
	Data PVEBackupsPayload `json:"data"`
	Meta PVEBackupsMeta    `json:"meta"`
}

type PVEBackupsPayload struct {
	BackupTasks    []models.BackupTask    `json:"backupTasks"`
	StorageBackups []models.StorageBackup `json:"storageBackups"`
	GuestSnapshots []models.GuestSnapshot `json:"guestSnapshots"`
}

type PVEBackupsMeta struct {
	TotalBackupTasks    int `json:"totalBackupTasks"`
	TotalStorageBackups int `json:"totalStorageBackups"`
	TotalGuestSnapshots int `json:"totalGuestSnapshots"`
}

func (r *Router) handleListPVEBackups(w http.ResponseWriter, req *http.Request) {
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

	backups := monitor.PVEBackupsSnapshot()
	payload := filterPVEBackups(backups, req)

	response := PVEBackupsResponse{
		Data: payload,
		Meta: PVEBackupsMeta{
			TotalBackupTasks:    len(payload.BackupTasks),
			TotalStorageBackups: len(payload.StorageBackups),
			TotalGuestSnapshots: len(payload.GuestSnapshots),
		},
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to encode PVE backups response")
		writeErrorResponse(w, http.StatusInternalServerError, "encoding_error",
			"Failed to encode PVE backups", nil)
	}
}

// filterPVEBackups narrows the three backup classes by optional query
// params:
//
//	instance=<name> — only entries reported by a given PVE instance
//	node=<name>     — only entries scheduled on a given node
//	vmid=<id>       — only entries that match a guest VMID
func filterPVEBackups(backups models.PVEBackups, req *http.Request) PVEBackupsPayload {
	query := req.URL.Query()
	instance := strings.TrimSpace(query.Get("instance"))
	node := strings.TrimSpace(query.Get("node"))
	vmid := strings.TrimSpace(query.Get("vmid"))

	out := PVEBackupsPayload{
		BackupTasks:    backups.BackupTasks,
		StorageBackups: backups.StorageBackups,
		GuestSnapshots: backups.GuestSnapshots,
	}

	if instance == "" && node == "" && vmid == "" {
		return out
	}

	tasks := make([]models.BackupTask, 0, len(out.BackupTasks))
	for _, task := range out.BackupTasks {
		if instance != "" && !strings.EqualFold(task.Instance, instance) {
			continue
		}
		if node != "" && !strings.EqualFold(task.Node, node) {
			continue
		}
		if vmid != "" && strconv.Itoa(task.VMID) != vmid {
			continue
		}
		tasks = append(tasks, task)
	}
	out.BackupTasks = tasks

	stored := make([]models.StorageBackup, 0, len(out.StorageBackups))
	for _, sb := range out.StorageBackups {
		if instance != "" && !strings.EqualFold(sb.Instance, instance) {
			continue
		}
		if node != "" && !strings.EqualFold(sb.Node, node) {
			continue
		}
		if vmid != "" && strconv.Itoa(sb.VMID) != vmid {
			continue
		}
		stored = append(stored, sb)
	}
	out.StorageBackups = stored

	snaps := make([]models.GuestSnapshot, 0, len(out.GuestSnapshots))
	for _, snap := range out.GuestSnapshots {
		if instance != "" && !strings.EqualFold(snap.Instance, instance) {
			continue
		}
		if node != "" && !strings.EqualFold(snap.Node, node) {
			continue
		}
		if vmid != "" && strconv.Itoa(snap.VMID) != vmid {
			continue
		}
		snaps = append(snaps, snap)
	}
	out.GuestSnapshots = snaps

	return out
}

func (r *Router) registerPVEBackupsRoutes() {
	r.mux.HandleFunc("/api/backups/pve", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleListPVEBackups)))
}
