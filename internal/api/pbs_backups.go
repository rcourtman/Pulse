package api

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

// PBSBackupsResponse is the source-specific payload for
// `GET /api/backups/pbs`. PBS backups are deduplicated server-side
// recovery artifacts, so their protection and verification state lives
// on the PBS snapshot model rather than on PVE's local vzdump file list.
type PBSBackupsResponse struct {
	Data PBSBackupsPayload `json:"data"`
	Meta PBSBackupsMeta    `json:"meta"`
}

type PBSBackupsPayload struct {
	Backups []models.PBSBackup `json:"backups"`
}

type PBSBackupsMeta struct {
	TotalBackups int `json:"totalBackups"`
}

func (r *Router) handleListPBSBackups(w http.ResponseWriter, req *http.Request) {
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

	backups := monitor.PBSBackupsSnapshot()
	payload := PBSBackupsPayload{Backups: filterPBSBackups(backups, req)}

	response := PBSBackupsResponse{
		Data: payload,
		Meta: PBSBackupsMeta{TotalBackups: len(payload.Backups)},
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to encode PBS backups response")
		writeErrorResponse(w, http.StatusInternalServerError, "encoding_error",
			"Failed to encode PBS backups", nil)
	}
}

// filterPBSBackups narrows PBS recovery artifacts by optional query params:
//
//	instance=<name>   - only backups reported by a given PBS connection
//	datastore=<name>  - only backups in a datastore
//	namespace=<name>  - only backups in a namespace
//	type=<vm|ct|host> - only backups of a backup type
//	vmid=<id>         - only backups for a guest id
func filterPBSBackups(backups []models.PBSBackup, req *http.Request) []models.PBSBackup {
	query := req.URL.Query()
	instance := strings.TrimSpace(query.Get("instance"))
	datastore := strings.TrimSpace(query.Get("datastore"))
	namespace := strings.TrimSpace(query.Get("namespace"))
	backupType := strings.TrimSpace(query.Get("type"))
	vmid := strings.TrimSpace(query.Get("vmid"))

	if instance == "" && datastore == "" && namespace == "" && backupType == "" && vmid == "" {
		return normalizePBSBackups(backups)
	}

	out := make([]models.PBSBackup, 0, len(backups))
	for _, backup := range backups {
		if instance != "" && !strings.EqualFold(backup.Instance, instance) {
			continue
		}
		if datastore != "" && !strings.EqualFold(backup.Datastore, datastore) {
			continue
		}
		if namespace != "" && !strings.EqualFold(backup.Namespace, namespace) {
			continue
		}
		if backupType != "" && !strings.EqualFold(backup.BackupType, backupType) {
			continue
		}
		if vmid != "" && backup.VMID != vmid {
			continue
		}
		out = append(out, backup.NormalizeCollections())
	}
	return out
}

func normalizePBSBackups(backups []models.PBSBackup) []models.PBSBackup {
	if backups == nil {
		return []models.PBSBackup{}
	}
	out := make([]models.PBSBackup, 0, len(backups))
	for _, backup := range backups {
		out = append(out, backup.NormalizeCollections())
	}
	return out
}

func (r *Router) registerPBSBackupsRoutes() {
	r.mux.HandleFunc("/api/backups/pbs", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleListPBSBackups)))
}
