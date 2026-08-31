package api

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

type stateSummaryResponse struct {
	ActiveAlerts int                      `json:"activeAlerts"`
	Nodes        int                      `json:"nodes"`
	VMs          int                      `json:"vms"`
	Containers   int                      `json:"containers"`
	DockerHosts  []stateSummaryDockerHost `json:"dockerHosts"`
	LastUpdate   time.Time                `json:"lastUpdate"`
	Verdicts     stateSummaryVerdicts     `json:"verdicts"`
	Attention    []stateSummaryAttention  `json:"attention"`
}

type stateSummaryVerdicts struct {
	OK        int `json:"ok"`
	Attention int `json:"attention"`
	Critical  int `json:"critical"`
	Stale     int `json:"stale"`
	Off       int `json:"off"`
	Unknown   int `json:"unknown"`
}

type stateSummaryAttention struct {
	ID           string                                 `json:"id"`
	Name         string                                 `json:"name"`
	Type         unifiedresources.ResourceType          `json:"type"`
	PlatformType string                                 `json:"platformType,omitempty"`
	Verdict      unifiedresources.ResourceHealthVerdict `json:"verdict"`
	TopReason    *unifiedresources.ResourceHealthReason `json:"topReason,omitempty"`
}

type stateSummaryDockerHost struct {
	Name            string  `json:"name"`
	Containers      int     `json:"containers"`
	UptimeSeconds   int64   `json:"uptimeSeconds"`
	CPUUsagePercent float64 `json:"cpuUsagePercent"`
}

// Keep the additive summary useful without allowing its attention list to
// approach the size of the full resource response. Thirty representative
// entries stay below the endpoint's 5 KiB target with ordinary identifiers.
const stateSummaryAttentionLimit = 30

func buildStateSummary(
	readState unifiedresources.ReadState,
	resources []unifiedresources.Resource,
	activeAlerts []models.Alert,
	lastUpdate time.Time,
) stateSummaryResponse {
	postureResources := unifiedresources.AttachResourceHealth(
		resources,
		stateSummaryHealthAlerts(activeAlerts),
		time.Now().UTC(),
	)
	verdicts, attention := summarizeResourceHealth(postureResources)
	if readState == nil {
		return stateSummaryResponse{
			ActiveAlerts: len(activeAlerts),
			LastUpdate:   lastUpdate,
			Verdicts:     verdicts,
			Attention:    attention,
		}
	}

	dockerHosts := make([]stateSummaryDockerHost, 0, len(readState.DockerHosts()))
	for _, host := range readState.DockerHosts() {
		if host == nil {
			continue
		}

		name := strings.TrimSpace(host.CustomDisplayName())
		if name == "" {
			name = strings.TrimSpace(host.DisplayName())
		}
		if name == "" {
			name = strings.TrimSpace(host.Hostname())
		}
		if name == "" {
			name = host.ID()
		}

		dockerHosts = append(dockerHosts, stateSummaryDockerHost{
			Name:            name,
			Containers:      len(host.Containers()),
			UptimeSeconds:   host.UptimeSeconds(),
			CPUUsagePercent: host.CPUPercent(),
		})
	}

	return stateSummaryResponse{
		ActiveAlerts: len(activeAlerts),
		Nodes:        len(readState.Nodes()),
		VMs:          len(readState.VMs()),
		Containers:   len(readState.Containers()),
		DockerHosts:  dockerHosts,
		LastUpdate:   lastUpdate,
		Verdicts:     verdicts,
		Attention:    attention,
	}
}

func stateSummaryHealthAlerts(active []models.Alert) []unifiedresources.ResourceHealthAlert {
	out := make([]unifiedresources.ResourceHealthAlert, 0, len(active))
	for _, alert := range active {
		out = append(out, unifiedresources.ResourceHealthAlert{
			ResourceID: alert.ResourceID,
			Level:      alert.Level,
			Type:       alert.Type,
		})
	}
	return out
}

func summarizeResourceHealth(resources []unifiedresources.Resource) (stateSummaryVerdicts, []stateSummaryAttention) {
	counts := stateSummaryVerdicts{}
	attention := make([]stateSummaryAttention, 0)
	for _, resource := range resources {
		if resource.Health == nil {
			continue
		}
		switch resource.Health.Verdict {
		case unifiedresources.HealthOK:
			counts.OK++
		case unifiedresources.HealthAttention:
			counts.Attention++
		case unifiedresources.HealthCritical:
			counts.Critical++
		case unifiedresources.HealthStale:
			counts.Stale++
		case unifiedresources.HealthOff:
			counts.Off++
		case unifiedresources.HealthUnknown:
			counts.Unknown++
		}
		if resource.Health.Verdict == unifiedresources.HealthOK || resource.Health.Verdict == unifiedresources.HealthOff {
			continue
		}
		name := strings.TrimSpace(resource.Name)
		if resource.Canonical != nil && strings.TrimSpace(resource.Canonical.DisplayName) != "" {
			name = strings.TrimSpace(resource.Canonical.DisplayName)
		}
		entry := stateSummaryAttention{
			ID: resource.ID, Name: name, Type: unifiedresources.ContractResourceType(resource),
			PlatformType: stateSummaryPlatformType(resource), Verdict: resource.Health.Verdict,
		}
		if len(resource.Health.Reasons) > 0 {
			reason := resource.Health.Reasons[0]
			entry.TopReason = &reason
		}
		attention = append(attention, entry)
	}
	sort.SliceStable(attention, func(i, j int) bool {
		left, right := summaryVerdictRank(attention[i].Verdict), summaryVerdictRank(attention[j].Verdict)
		if left != right {
			return left > right
		}
		return strings.ToLower(attention[i].Name) < strings.ToLower(attention[j].Name)
	})
	if len(attention) > stateSummaryAttentionLimit {
		attention = attention[:stateSummaryAttentionLimit]
	}
	if attention == nil {
		attention = []stateSummaryAttention{}
	}
	return counts, attention
}

func summaryVerdictRank(verdict unifiedresources.ResourceHealthVerdict) int {
	switch verdict {
	case unifiedresources.HealthCritical:
		return 4
	case unifiedresources.HealthAttention:
		return 3
	case unifiedresources.HealthStale:
		return 2
	case unifiedresources.HealthUnknown:
		return 1
	default:
		return 0
	}
}

func stateSummaryPlatformType(resource unifiedresources.Resource) string {
	if resource.Proxmox != nil || resource.PBS != nil || resource.PMG != nil {
		return "proxmox"
	}
	if resource.Docker != nil {
		return "docker"
	}
	if resource.Kubernetes != nil {
		return "kubernetes"
	}
	if resource.TrueNAS != nil {
		return "truenas"
	}
	if resource.VMware != nil {
		return "vmware"
	}
	if resource.Availability != nil {
		return "availability"
	}
	if len(resource.Sources) > 0 {
		return string(resource.Sources[0])
	}
	return "generic"
}

func (r *Router) handleStateSummary(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
			"Only GET method is allowed", nil)
		return
	}

	authWriter := &responseCapture{ResponseWriter: w}
	if !checkAuth(r.config, authWriter, req, false) {
		if !authWriter.wrote {
			writeErrorResponse(w, http.StatusUnauthorized, "unauthorized",
				"Authentication required", nil)
		}
		return
	}

	if record := getAPITokenRecordFromRequest(req); record != nil && !record.HasScope(config.ScopeMonitoringRead) {
		respondMissingScope(w, config.ScopeMonitoringRead)
		return
	}

	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "no_monitor",
			"Monitor not available", nil)
		return
	}

	readState := monitor.GetUnifiedReadStateOrSnapshot()
	snapshot := monitor.ReadSnapshot()
	resources, _ := monitor.UnifiedResourceSnapshot()
	if err := utils.WriteJSONResponse(w, buildStateSummary(readState, resources, snapshot.ActiveAlerts, snapshot.LastUpdate)); err != nil {
		log.Error().Err(err).Msg("Failed to encode state summary response")
		writeErrorResponse(w, http.StatusInternalServerError, "encoding_error",
			"Failed to encode state summary", nil)
	}
}
