package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	kubernetesmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/kubernetes"
	recoverystore "github.com/rcourtman/pulse-go-rewrite/internal/recovery/store"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	"github.com/rs/zerolog/log"
)

type RecoveryHandlers struct {
	manager *recoverymanager.Manager
}

func NewRecoveryHandlers(manager *recoverymanager.Manager) *RecoveryHandlers {
	return &RecoveryHandlers{
		manager: manager,
	}
}

func (h *RecoveryHandlers) storeForOrg(orgID string) (*recoverystore.Store, error) {
	if h == nil || h.manager == nil {
		return nil, fmt.Errorf("recovery manager is not configured")
	}
	return h.manager.StoreForOrg(orgID)
}

type recoveryPointsResponse struct {
	Data []recovery.RecoveryPoint `json:"data"`
	Meta struct {
		Page       int `json:"page"`
		Limit      int `json:"limit"`
		Total      int `json:"total"`
		TotalPages int `json:"totalPages"`
	} `json:"meta"`
}

func parseRFC3339QueryTime(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	t := parsed.UTC()
	return &t, nil
}

func parseIntQuery(qs map[string][]string, key string, fallback int) int {
	v := strings.TrimSpace(firstQueryValue(qs, key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func firstQueryValue(qs map[string][]string, key string) string {
	values := qs[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (h *RecoveryHandlers) HandleListPoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	qs := r.URL.Query()
	page := parseIntQuery(qs, "page", 1)
	limit := parseIntQuery(qs, "limit", 100)

	var from, to *time.Time
	if t, err := parseRFC3339QueryTime(qs.Get("from")); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_from", "Invalid from time; must be RFC3339", map[string]string{"error": err.Error()})
		return
	} else {
		from = t
	}
	if t, err := parseRFC3339QueryTime(qs.Get("to")); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_to", "Invalid to time; must be RFC3339", map[string]string{"error": err.Error()})
		return
	} else {
		to = t
	}

	opts := recovery.ListPointsOptions{
		Provider:          recovery.Provider(strings.TrimSpace(qs.Get("provider"))),
		Kind:              recovery.Kind(strings.TrimSpace(qs.Get("kind"))),
		Mode:              recovery.Mode(strings.TrimSpace(qs.Get("mode"))),
		Outcome:           recovery.Outcome(strings.TrimSpace(qs.Get("outcome"))),
		SubjectResourceID: strings.TrimSpace(qs.Get("subjectResourceId")),
		RollupID:          strings.TrimSpace(qs.Get("rollupId")),
		From:              from,
		To:                to,
		Query:             strings.TrimSpace(firstNonEmpty(qs.Get("q"), qs.Get("query"))),
		ClusterLabel:      strings.TrimSpace(firstNonEmpty(qs.Get("cluster"), qs.Get("clusterLabel"))),
		NodeHostLabel:     strings.TrimSpace(firstNonEmpty(qs.Get("node"), qs.Get("nodeHost"), qs.Get("nodeHostLabel"))),
		NamespaceLabel:    strings.TrimSpace(firstNonEmpty(qs.Get("namespace"), qs.Get("namespaceLabel"))),
		WorkloadOnly:      strings.TrimSpace(qs.Get("scope")) == "workload" || strings.TrimSpace(qs.Get("workloadOnly")) == "true",
		Verification:      strings.TrimSpace(firstNonEmpty(qs.Get("verification"), qs.Get("verified"))),
		Page:              page,
		Limit:             limit,
	}

	var (
		points []recovery.RecoveryPoint
		total  int
	)

	if mock.IsMockEnabled() {
		all := mock.GetMockRecoveryPoints()
		filtered := filterRecoveryPoints(all, opts)
		total = len(filtered)
		points = paginateRecoveryPoints(filtered, opts.Page, opts.Limit)
	} else {
		orgID := GetOrgID(r.Context())
		store, err := h.storeForOrg(orgID)
		if err != nil {
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}

		p, t, err := store.ListPoints(r.Context(), opts)
		if err != nil {
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}
		points = p
		total = t
	}

	// Ensure normalized display data is present for UIs (works for both real and mock paths).
	for i := range points {
		if points[i].Display == nil {
			idx := recovery.DeriveIndex(points[i])
			points[i].Display = idx.ToDisplay()
		}
	}

	var resp recoveryPointsResponse
	resp.Data = points
	resp.Meta.Page = page
	resp.Meta.Limit = limit
	resp.Meta.Total = total
	if limit <= 0 {
		resp.Meta.TotalPages = 1
	} else {
		resp.Meta.TotalPages = (total + limit - 1) / limit
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to serialize recovery points response")
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseBoolQuery(qs map[string][]string, key string) bool {
	v := strings.TrimSpace(firstQueryValue(qs, key))
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (h *RecoveryHandlers) HandleListSeries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	qs := r.URL.Query()

	var from, to *time.Time
	if t, err := parseRFC3339QueryTime(qs.Get("from")); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_from", "Invalid from time; must be RFC3339", map[string]string{"error": err.Error()})
		return
	} else {
		from = t
	}
	if t, err := parseRFC3339QueryTime(qs.Get("to")); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_to", "Invalid to time; must be RFC3339", map[string]string{"error": err.Error()})
		return
	} else {
		to = t
	}

	tzOffsetMin := parseIntQuery(qs, "tzOffsetMinutes", 0)

	opts := recovery.ListPointsOptions{
		Provider:          recovery.Provider(strings.TrimSpace(qs.Get("provider"))),
		Kind:              recovery.Kind(strings.TrimSpace(qs.Get("kind"))),
		Mode:              recovery.Mode(strings.TrimSpace(qs.Get("mode"))),
		Outcome:           recovery.Outcome(strings.TrimSpace(qs.Get("outcome"))),
		SubjectResourceID: strings.TrimSpace(qs.Get("subjectResourceId")),
		RollupID:          strings.TrimSpace(qs.Get("rollupId")),
		From:              from,
		To:                to,
		Query:             strings.TrimSpace(firstNonEmpty(qs.Get("q"), qs.Get("query"))),
		ClusterLabel:      strings.TrimSpace(firstNonEmpty(qs.Get("cluster"), qs.Get("clusterLabel"))),
		NodeHostLabel:     strings.TrimSpace(firstNonEmpty(qs.Get("node"), qs.Get("nodeHost"), qs.Get("nodeHostLabel"))),
		NamespaceLabel:    strings.TrimSpace(firstNonEmpty(qs.Get("namespace"), qs.Get("namespaceLabel"))),
		WorkloadOnly:      strings.TrimSpace(qs.Get("scope")) == "workload" || parseBoolQuery(qs, "workloadOnly"),
		Verification:      strings.TrimSpace(firstNonEmpty(qs.Get("verification"), qs.Get("verified"))),
	}

	var series []recovery.PointsSeriesBucket
	if mock.IsMockEnabled() {
		all := mock.GetMockRecoveryPoints()
		filtered := filterRecoveryPoints(all, opts)
		// Count only completed points.
		series = buildSeriesFromPoints(filtered, opts, tzOffsetMin)
	} else {
		orgID := GetOrgID(r.Context())
		store, err := h.storeForOrg(orgID)
		if err != nil {
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}
		s, err := store.ListPointsSeries(r.Context(), opts, tzOffsetMin)
		if err != nil {
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}
		series = s
	}

	if err := utils.WriteJSONResponse(w, map[string]any{"data": series}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize recovery series response")
	}
}

func (h *RecoveryHandlers) HandleListFacets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	qs := r.URL.Query()

	var from, to *time.Time
	if t, err := parseRFC3339QueryTime(qs.Get("from")); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_from", "Invalid from time; must be RFC3339", map[string]string{"error": err.Error()})
		return
	} else {
		from = t
	}
	if t, err := parseRFC3339QueryTime(qs.Get("to")); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_to", "Invalid to time; must be RFC3339", map[string]string{"error": err.Error()})
		return
	} else {
		to = t
	}

	opts := recovery.ListPointsOptions{
		Provider:          recovery.Provider(strings.TrimSpace(qs.Get("provider"))),
		Kind:              recovery.Kind(strings.TrimSpace(qs.Get("kind"))),
		Mode:              recovery.Mode(strings.TrimSpace(qs.Get("mode"))),
		Outcome:           recovery.Outcome(strings.TrimSpace(qs.Get("outcome"))),
		SubjectResourceID: strings.TrimSpace(qs.Get("subjectResourceId")),
		RollupID:          strings.TrimSpace(qs.Get("rollupId")),
		From:              from,
		To:                to,
		Query:             strings.TrimSpace(firstNonEmpty(qs.Get("q"), qs.Get("query"))),
		ClusterLabel:      strings.TrimSpace(firstNonEmpty(qs.Get("cluster"), qs.Get("clusterLabel"))),
		NodeHostLabel:     strings.TrimSpace(firstNonEmpty(qs.Get("node"), qs.Get("nodeHost"), qs.Get("nodeHostLabel"))),
		NamespaceLabel:    strings.TrimSpace(firstNonEmpty(qs.Get("namespace"), qs.Get("namespaceLabel"))),
		WorkloadOnly:      strings.TrimSpace(qs.Get("scope")) == "workload" || parseBoolQuery(qs, "workloadOnly"),
		Verification:      strings.TrimSpace(firstNonEmpty(qs.Get("verification"), qs.Get("verified"))),
	}

	var facets recovery.PointsFacets
	if mock.IsMockEnabled() {
		all := mock.GetMockRecoveryPoints()
		filtered := filterRecoveryPoints(all, opts)
		facets = buildFacetsFromPoints(filtered)
	} else {
		orgID := GetOrgID(r.Context())
		store, err := h.storeForOrg(orgID)
		if err != nil {
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}
		f, err := store.ListPointsFacets(r.Context(), opts)
		if err != nil {
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}
		facets = f
	}

	if err := utils.WriteJSONResponse(w, map[string]any{"data": facets}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize recovery facets response")
	}
}

func filterRecoveryPoints(all []recovery.RecoveryPoint, opts recovery.ListPointsOptions) []recovery.RecoveryPoint {
	if len(all) == 0 {
		return nil
	}

	provider := strings.TrimSpace(string(opts.Provider))
	kind := strings.TrimSpace(string(opts.Kind))
	mode := strings.TrimSpace(string(opts.Mode))
	outcome := strings.TrimSpace(string(opts.Outcome))
	subjectRID := strings.TrimSpace(opts.SubjectResourceID)
	rollupID := strings.TrimSpace(opts.RollupID)
	cluster := strings.TrimSpace(opts.ClusterLabel)
	node := strings.TrimSpace(opts.NodeHostLabel)
	namespace := strings.TrimSpace(opts.NamespaceLabel)
	q := strings.ToLower(strings.TrimSpace(opts.Query))
	workloadOnly := opts.WorkloadOnly
	verification := strings.ToLower(strings.TrimSpace(opts.Verification))
	if rollupID != "" && !strings.HasPrefix(rollupID, "res:") && !strings.HasPrefix(rollupID, "ext:") {
		rollupID = "res:" + rollupID
	}

	out := make([]recovery.RecoveryPoint, 0, len(all))
	for _, p := range all {
		// Ensure display/index data exists for filtering parity with sqlite store.
		if p.Display == nil {
			idx := recovery.DeriveIndex(p)
			p.Display = idx.ToDisplay()
		}
		disp := p.Display

		if provider != "" && strings.TrimSpace(string(p.Provider)) != provider {
			continue
		}
		if kind != "" && strings.TrimSpace(string(p.Kind)) != kind {
			continue
		}
		if mode != "" && strings.TrimSpace(string(p.Mode)) != mode {
			continue
		}
		if outcome != "" && strings.TrimSpace(string(p.Outcome)) != outcome {
			continue
		}
		if rollupID != "" {
			if strings.TrimSpace(recovery.SubjectKeyForPoint(p)) != rollupID {
				continue
			}
		} else if subjectRID != "" && strings.TrimSpace(p.SubjectResourceID) != subjectRID {
			continue
		}
		if opts.From != nil && !opts.From.IsZero() {
			// Match store semantics: completed_at_ms IS NULL OR completed_at_ms >= from
			if p.CompletedAt != nil && !p.CompletedAt.IsZero() && p.CompletedAt.Before(opts.From.UTC()) {
				continue
			}
		}
		if opts.To != nil && !opts.To.IsZero() {
			// Match store semantics: completed_at_ms IS NULL OR completed_at_ms <= to
			if p.CompletedAt != nil && !p.CompletedAt.IsZero() && p.CompletedAt.After(opts.To.UTC()) {
				continue
			}
		}

		if cluster != "" && strings.TrimSpace(getDisplayClusterLabel(p.Display)) != cluster {
			continue
		}
		if node != "" && strings.TrimSpace(getDisplayNodeHostLabel(p.Display)) != node {
			continue
		}
		if namespace != "" && strings.TrimSpace(getDisplayNamespaceLabel(p.Display)) != namespace {
			continue
		}
		if workloadOnly && !(disp != nil && disp.IsWorkload) {
			continue
		}
		if verification != "" {
			v := p.Verified
			switch verification {
			case "verified":
				if v == nil || !*v {
					continue
				}
			case "unverified":
				if v == nil || *v {
					continue
				}
			case "unknown":
				if v != nil {
					continue
				}
			}
		}
		if q != "" {
			// Best-effort match across normalized display fields.
			var subjectLabel, subjectType, clusterLabel, nodeLabel, nsLabel, entityID, repoLabel, detailSummary string
			if disp != nil {
				subjectLabel = disp.SubjectLabel
				subjectType = disp.SubjectType
				clusterLabel = disp.ClusterLabel
				nodeLabel = disp.NodeHostLabel
				nsLabel = disp.NamespaceLabel
				entityID = disp.EntityIDLabel
				repoLabel = disp.RepositoryLabel
				detailSummary = disp.DetailsSummary
			}

			hay := strings.ToLower(strings.Join([]string{
				strings.TrimSpace(p.ID),
				strings.TrimSpace(string(p.Provider)),
				strings.TrimSpace(string(p.Kind)),
				strings.TrimSpace(string(p.Mode)),
				strings.TrimSpace(string(p.Outcome)),
				strings.TrimSpace(subjectLabel),
				strings.TrimSpace(subjectType),
				strings.TrimSpace(clusterLabel),
				strings.TrimSpace(nodeLabel),
				strings.TrimSpace(nsLabel),
				strings.TrimSpace(entityID),
				strings.TrimSpace(repoLabel),
				strings.TrimSpace(detailSummary),
			}, " "))
			if !strings.Contains(hay, q) {
				continue
			}
		}

		out = append(out, p)
	}
	return out
}

func getDisplayClusterLabel(d *recovery.RecoveryPointDisplay) string {
	if d == nil {
		return ""
	}
	return d.ClusterLabel
}

func getDisplayNodeHostLabel(d *recovery.RecoveryPointDisplay) string {
	if d == nil {
		return ""
	}
	return d.NodeHostLabel
}

func getDisplayNamespaceLabel(d *recovery.RecoveryPointDisplay) string {
	if d == nil {
		return ""
	}
	return d.NamespaceLabel
}

func paginateRecoveryPoints(filtered []recovery.RecoveryPoint, page int, limit int) []recovery.RecoveryPoint {
	if len(filtered) == 0 {
		return []recovery.RecoveryPoint{}
	}

	normalizedLimit := limit
	if normalizedLimit <= 0 {
		normalizedLimit = 100
	}
	if normalizedLimit > 500 {
		normalizedLimit = 500
	}
	normalizedPage := page
	if normalizedPage <= 0 {
		normalizedPage = 1
	}

	offset := (normalizedPage - 1) * normalizedLimit
	if offset >= len(filtered) {
		return []recovery.RecoveryPoint{}
	}
	end := offset + normalizedLimit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end]
}

func (h *RecoveryHandlers) HandleListRollups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	qs := r.URL.Query()
	page := parseIntQuery(qs, "page", 1)
	limit := parseIntQuery(qs, "limit", 100)

	var from, to *time.Time
	if t, err := parseRFC3339QueryTime(qs.Get("from")); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_from", "Invalid from time; must be RFC3339", map[string]string{"error": err.Error()})
		return
	} else {
		from = t
	}
	if t, err := parseRFC3339QueryTime(qs.Get("to")); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_to", "Invalid to time; must be RFC3339", map[string]string{"error": err.Error()})
		return
	} else {
		to = t
	}

	opts := recovery.ListPointsOptions{
		Provider:          recovery.Provider(strings.TrimSpace(qs.Get("provider"))),
		Kind:              recovery.Kind(strings.TrimSpace(qs.Get("kind"))),
		Mode:              recovery.Mode(strings.TrimSpace(qs.Get("mode"))),
		Outcome:           recovery.Outcome(strings.TrimSpace(qs.Get("outcome"))),
		SubjectResourceID: strings.TrimSpace(qs.Get("subjectResourceId")),
		RollupID:          strings.TrimSpace(qs.Get("rollupId")),
		From:              from,
		To:                to,
		Page:              page,
		Limit:             limit,
	}

	var (
		rollups []recovery.ProtectionRollup
		total   int
	)

	if mock.IsMockEnabled() {
		all := mock.GetMockRecoveryPoints()
		filtered := filterRecoveryPointsForRollups(all, opts)
		rollups = recovery.BuildRollupsFromPoints(filtered)
		total = len(rollups)
		rollups = paginateRecoveryRollups(rollups, opts.Page, opts.Limit)
	} else {
		orgID := GetOrgID(r.Context())
		store, err := h.storeForOrg(orgID)
		if err != nil {
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}

		r, t, err := store.ListRollups(r.Context(), opts)
		if err != nil {
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}
		rollups = r
		total = t
	}

	meta := map[string]any{
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": 0,
	}
	if limit <= 0 {
		meta["totalPages"] = 1
	} else {
		meta["totalPages"] = (total + limit - 1) / limit
	}

	if err := utils.WriteJSONResponse(w, map[string]any{
		"data": rollups,
		"meta": meta,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize recovery rollups response")
	}
}

func filterRecoveryPointsForRollups(all []recovery.RecoveryPoint, opts recovery.ListPointsOptions) []recovery.RecoveryPoint {
	if len(all) == 0 {
		return nil
	}

	provider := strings.TrimSpace(string(opts.Provider))
	kind := strings.TrimSpace(string(opts.Kind))
	mode := strings.TrimSpace(string(opts.Mode))
	outcome := strings.TrimSpace(string(opts.Outcome))
	subjectRID := strings.TrimSpace(opts.SubjectResourceID)
	rollupID := strings.TrimSpace(opts.RollupID)
	if rollupID != "" && !strings.HasPrefix(rollupID, "res:") && !strings.HasPrefix(rollupID, "ext:") {
		rollupID = "res:" + rollupID
	}

	out := make([]recovery.RecoveryPoint, 0, len(all))
	for _, p := range all {
		if provider != "" && strings.TrimSpace(string(p.Provider)) != provider {
			continue
		}
		if kind != "" && strings.TrimSpace(string(p.Kind)) != kind {
			continue
		}
		if mode != "" && strings.TrimSpace(string(p.Mode)) != mode {
			continue
		}
		if outcome != "" && strings.TrimSpace(string(p.Outcome)) != outcome {
			continue
		}
		if rollupID != "" {
			if strings.TrimSpace(recovery.SubjectKeyForPoint(p)) != rollupID {
				continue
			}
		} else if subjectRID != "" && strings.TrimSpace(p.SubjectResourceID) != subjectRID {
			continue
		}

		ts := rollupTimestamp(p)
		if opts.From != nil && !opts.From.IsZero() && ts != nil && !ts.IsZero() && ts.Before(opts.From.UTC()) {
			continue
		}
		if opts.To != nil && !opts.To.IsZero() && ts != nil && !ts.IsZero() && ts.After(opts.To.UTC()) {
			continue
		}

		out = append(out, p)
	}
	return out
}

func rollupTimestamp(p recovery.RecoveryPoint) *time.Time {
	if p.CompletedAt != nil && !p.CompletedAt.IsZero() {
		t := p.CompletedAt.UTC()
		return &t
	}
	if p.StartedAt != nil && !p.StartedAt.IsZero() {
		t := p.StartedAt.UTC()
		return &t
	}
	return nil
}

func paginateRecoveryRollups(filtered []recovery.ProtectionRollup, page int, limit int) []recovery.ProtectionRollup {
	if len(filtered) == 0 {
		return []recovery.ProtectionRollup{}
	}

	normalizedLimit := limit
	if normalizedLimit <= 0 {
		normalizedLimit = 100
	}
	if normalizedLimit > 500 {
		normalizedLimit = 500
	}
	normalizedPage := page
	if normalizedPage <= 0 {
		normalizedPage = 1
	}

	offset := (normalizedPage - 1) * normalizedLimit
	if offset >= len(filtered) {
		return []recovery.ProtectionRollup{}
	}
	end := offset + normalizedLimit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end]
}

func buildSeriesFromPoints(points []recovery.RecoveryPoint, opts recovery.ListPointsOptions, tzOffsetMinutes int) []recovery.PointsSeriesBucket {
	// Mirror store.ListPointsSeries semantics: completed only; group by day in the requested timezone offset.
	if len(points) == 0 {
		return []recovery.PointsSeriesBucket{}
	}

	offset := time.Duration(tzOffsetMinutes) * time.Minute

	type bucket struct {
		day      string
		total    int
		snapshot int
		local    int
		remote   int
	}

	buckets := map[string]*bucket{}

	// Determine the day window to return.
	start := time.Now().UTC().Add(-29 * 24 * time.Hour)
	end := time.Now().UTC()
	if opts.From != nil && !opts.From.IsZero() {
		start = opts.From.UTC()
	}
	if opts.To != nil && !opts.To.IsZero() {
		end = opts.To.UTC()
	}
	if end.Before(start) {
		start, end = end, start
	}
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	for _, p := range points {
		if p.CompletedAt == nil || p.CompletedAt.IsZero() {
			continue
		}
		dayKey := p.CompletedAt.UTC().Add(offset).Format("2006-01-02")
		b := buckets[dayKey]
		if b == nil {
			b = &bucket{day: dayKey}
			buckets[dayKey] = b
		}
		b.total++
		switch strings.ToLower(strings.TrimSpace(string(p.Mode))) {
		case "snapshot":
			b.snapshot++
		case "remote":
			b.remote++
		default:
			b.local++
		}
	}

	out := make([]recovery.PointsSeriesBucket, 0, int(end.Sub(start).Hours()/24)+1)
	for d := start; !d.After(end); d = d.Add(24 * time.Hour) {
		key := d.Format("2006-01-02")
		if b := buckets[key]; b != nil {
			out = append(out, recovery.PointsSeriesBucket{
				Day:      b.day,
				Total:    b.total,
				Snapshot: b.snapshot,
				Local:    b.local,
				Remote:   b.remote,
			})
		} else {
			out = append(out, recovery.PointsSeriesBucket{Day: key})
		}
	}
	return out
}

func buildFacetsFromPoints(points []recovery.RecoveryPoint) recovery.PointsFacets {
	clusters := map[string]struct{}{}
	nodes := map[string]struct{}{}
	namespaces := map[string]struct{}{}

	var hasSize bool
	var hasVerification bool
	var hasEntityID bool

	for _, p := range points {
		if p.Display == nil {
			idx := recovery.DeriveIndex(p)
			p.Display = idx.ToDisplay()
		}
		if p.Display != nil {
			if v := strings.TrimSpace(p.Display.ClusterLabel); v != "" {
				clusters[v] = struct{}{}
			}
			if v := strings.TrimSpace(p.Display.NodeHostLabel); v != "" {
				nodes[v] = struct{}{}
			}
			if v := strings.TrimSpace(p.Display.NamespaceLabel); v != "" {
				namespaces[v] = struct{}{}
			}
			if v := strings.TrimSpace(p.Display.EntityIDLabel); v != "" {
				hasEntityID = true
			}
		}
		if p.SizeBytes != nil && *p.SizeBytes > 0 {
			hasSize = true
		}
		if p.Verified != nil {
			hasVerification = true
		}
	}

	toSorted := func(m map[string]struct{}) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}

	return recovery.PointsFacets{
		Clusters:        toSorted(clusters),
		NodesHosts:      toSorted(nodes),
		Namespaces:      toSorted(namespaces),
		HasSize:         hasSize,
		HasVerification: hasVerification,
		HasEntityID:     hasEntityID,
	}
}

// IngestKubernetesReport converts Kubernetes recovery artifacts into canonical recovery points and persists them.
// This is called from the Kubernetes agent ingest path; it is intentionally best-effort and must not block
// baseline cluster monitoring ingestion.
func (h *RecoveryHandlers) IngestKubernetesReport(ctx context.Context, orgID string, report agentsk8s.Report) error {
	if report.Recovery == nil {
		return nil
	}

	store, err := h.storeForOrg(orgID)
	if err != nil {
		return err
	}

	points := kubernetesmapper.FromKubernetesRecoveryReport(report.Cluster, report.Recovery)
	if len(points) == 0 {
		return nil
	}

	// Best-effort cap: prevent pathological payloads from creating massive DB writes.
	const maxPointsPerIngest = 2000
	if len(points) > maxPointsPerIngest {
		points = points[:maxPointsPerIngest]
	}

	return store.UpsertPoints(ctx, points)
}
