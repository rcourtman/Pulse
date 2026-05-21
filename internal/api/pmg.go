package api

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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

	readState := monitor.GetUnifiedReadStateOrSnapshot()
	views := readState.PMGInstances()

	mappedInstances := make([]models.PMGInstance, len(views))
	for i, v := range views {
		mappedInstances[i] = mapPMGInstanceViewToModel(v)
	}

	instances := filterPMGInstances(mappedInstances, req)

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

// mapPMGInstanceViewToModel projects the canonical PMG read-state view onto
// the legacy API payload consumed by existing PMG surfaces.
func mapPMGInstanceViewToModel(v *unifiedresources.PMGInstanceView) models.PMGInstance {
	if v == nil {
		return models.PMGInstance{}
	}

	nodes := v.Nodes()
	modelNodes := make([]models.PMGNodeStatus, len(nodes))
	for i, n := range nodes {
		modelNodes[i] = models.PMGNodeStatus{
			Name:    n.Name,
			Status:  n.Status,
			Role:    n.Role,
			Uptime:  n.Uptime,
			LoadAvg: n.LoadAvg,
		}
		if n.QueueStatus != nil {
			modelNodes[i].QueueStatus = &models.PMGQueueStatus{
				Active:    n.QueueStatus.Active,
				Deferred:  n.QueueStatus.Deferred,
				Hold:      n.QueueStatus.Hold,
				Incoming:  n.QueueStatus.Incoming,
				Total:     n.QueueStatus.Total,
				OldestAge: n.QueueStatus.OldestAge,
				UpdatedAt: n.QueueStatus.UpdatedAt,
			}
		}
	}

	var modelMailStats *models.PMGMailStats
	if stats := v.MailStats(); stats != nil {
		countTotal := stats.CountTotal
		if countTotal == 0 {
			countTotal = v.MailCountTotal()
		}
		updatedAt := stats.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = v.LastUpdated()
		}
		modelMailStats = &models.PMGMailStats{
			Timeframe:            stats.Timeframe,
			CountTotal:           countTotal,
			CountIn:              stats.CountIn,
			CountOut:             stats.CountOut,
			SpamIn:               stats.SpamIn,
			SpamOut:              stats.SpamOut,
			VirusIn:              stats.VirusIn,
			VirusOut:             stats.VirusOut,
			BouncesIn:            stats.BouncesIn,
			BouncesOut:           stats.BouncesOut,
			BytesIn:              stats.BytesIn,
			BytesOut:             stats.BytesOut,
			GreylistCount:        stats.GreylistCount,
			JunkIn:               stats.JunkIn,
			AverageProcessTimeMs: stats.AverageProcessTimeMs,
			RBLRejects:           stats.RBLRejects,
			PregreetRejects:      stats.PregreetRejects,
			UpdatedAt:            updatedAt,
		}
	}

	var modelQuarantine *models.PMGQuarantineTotals
	if q := v.Quarantine(); q != nil {
		modelQuarantine = &models.PMGQuarantineTotals{
			Spam:        q.Spam,
			Virus:       q.Virus,
			Attachment:  q.Attachment,
			Blacklisted: q.Blacklisted,
		}
	}

	spamDist := v.SpamDistribution()
	modelSpamDist := make([]models.PMGSpamBucket, len(spamDist))
	for i, b := range spamDist {
		modelSpamDist[i] = models.PMGSpamBucket{
			Score: b.Bucket,
			Count: b.Count,
		}
	}

	relayDomains := v.RelayDomains()
	modelRelayDomains := make([]models.PMGRelayDomain, len(relayDomains))
	for i, d := range relayDomains {
		modelRelayDomains[i] = models.PMGRelayDomain{
			Domain:  d.Domain,
			Comment: d.Comment,
		}
	}

	domainStats := v.DomainStats()
	modelDomainStats := make([]models.PMGDomainStat, len(domainStats))
	for i, s := range domainStats {
		modelDomainStats[i] = models.PMGDomainStat{
			Domain:     s.Domain,
			MailCount:  s.MailCount,
			SpamCount:  s.SpamCount,
			VirusCount: s.VirusCount,
			Bytes:      s.Bytes,
		}
	}

	host := v.HostURL()
	if strings.TrimSpace(host) == "" {
		host = v.Hostname()
	}
	guestURL := v.GuestURL()
	if strings.TrimSpace(guestURL) == "" {
		guestURL = v.CustomURL()
	}
	instanceID := v.InstanceID()
	if strings.TrimSpace(instanceID) == "" {
		instanceID = v.ID()
	}

	inst := models.PMGInstance{
		ID:               instanceID,
		Name:             v.Name(),
		Host:             host,
		GuestURL:         guestURL,
		Status:           string(v.Status()),
		Version:          v.Version(),
		Nodes:            modelNodes,
		MailStats:        modelMailStats,
		MailCount:        v.MailCount(),
		SpamDistribution: modelSpamDist,
		Quarantine:       modelQuarantine,
		RelayDomains:     modelRelayDomains,
		DomainStats:      modelDomainStats,
		DomainStatsAsOf:  v.DomainStatsAsOf(),
		ConnectionHealth: v.ConnectionHealth(),
		LastSeen:         v.LastSeen(),
		LastUpdated:      v.LastUpdated(),
	}
	return inst.NormalizeCollections()
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
