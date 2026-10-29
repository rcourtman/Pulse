package api

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

// handleAgentMetricHistory serves get_metric_history. Agents hold canonical
// resource ids, while the metrics store keys history by each resource's
// MetricsTarget. This resolves one into the other and hands the read to
// handleMetricsHistory, which owns tiers, fallbacks, downsampling and the
// licensed history limit, so an agent and the history charts read the same
// series under the same entitlement.
func (r *Router) handleAgentMetricHistory(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query, problem := parseAgentMetricHistoryQuery(req.URL.Query())
	if problem != "" {
		http.Error(w, problem, http.StatusBadRequest)
		return
	}
	if r.resourceHandlers == nil {
		http.Error(w, "Resource inventory unavailable", http.StatusServiceUnavailable)
		return
	}
	registry, err := r.resourceHandlers.buildRegistry(GetOrgID(req.Context()))
	if err != nil || registry == nil {
		http.Error(w, "Resource inventory unavailable", http.StatusServiceUnavailable)
		return
	}
	_, canonicalID, ok := registry.GetByReference(query.resourceID)
	if !ok || canonicalID == "" {
		http.Error(w, "Unknown resourceId: use a canonical id from get_fleet_context", http.StatusNotFound)
		return
	}
	// The registry derives the target from the resource's source mappings,
	// as it does for the resources API; a stored resource may carry none.
	metricsTarget := registry.MetricsTarget(canonicalID)
	if metricsTarget == nil || metricsTarget.ResourceType == "" || metricsTarget.ResourceID == "" {
		http.Error(w, "Pulse records no metric history for this resource", http.StatusNotFound)
		return
	}

	target := url.Values{}
	target.Set("resourceType", metricsTarget.ResourceType)
	target.Set("resourceId", metricsTarget.ResourceID)
	target.Set("range", query.timeRange)
	target.Set("maxPoints", strconv.Itoa(query.maxPoints))
	if query.metric != "" {
		target.Set("metric", query.metric)
	}
	delegated := req.Clone(req.Context())
	delegated.URL.RawQuery = target.Encode()
	r.handleMetricsHistory(w, delegated)
}

type agentMetricHistoryQuery struct {
	resourceID string
	metric     string
	timeRange  string
	maxPoints  int
}

// parseAgentMetricHistoryQuery reads the get_metric_history inputs. Malformed
// values are a 400 with a reason rather than a silent default, so an agent
// learns what it sent wrong.
func parseAgentMetricHistoryQuery(values url.Values) (agentMetricHistoryQuery, string) {
	query := agentMetricHistoryQuery{
		resourceID: strings.TrimSpace(values.Get(agentcapabilities.ResourceIDArgumentName)),
		metric:     strings.TrimSpace(values.Get("metric")),
		timeRange:  agentcapabilities.MetricHistoryDefaultRange,
		maxPoints:  agentcapabilities.MetricHistoryDefaultMaxPoints,
	}
	if query.resourceID == "" {
		return query, "resourceId is required"
	}
	if raw := strings.ToLower(strings.TrimSpace(values.Get("range"))); raw != "" {
		if !slices.Contains(agentcapabilities.MetricHistoryRanges, raw) {
			return query, "range must be one of " + strings.Join(agentcapabilities.MetricHistoryRanges, ", ")
		}
		query.timeRange = raw
	}
	if raw := strings.TrimSpace(values.Get("maxPoints")); raw != "" {
		maxPoints, err := strconv.Atoi(raw)
		if err != nil || maxPoints < 1 {
			return query, "maxPoints must be a positive integer"
		}
		query.maxPoints = min(maxPoints, agentcapabilities.MetricHistoryMaxPoints)
	}
	return query, ""
}
