package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	recoverymodel "github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

const (
	attentionHistoryReadLimit = 500
	attentionPostureBatchSize = 200
)

type attentionAlertSnapshot func(context.Context) ([]alerts.Alert, []alerts.Alert, error)

type AttentionHandlers struct {
	readAlerts      attentionAlertSnapshot
	recoveryManager *recoverymanager.Manager
}

func NewAttentionHandlers(
	getMonitor func(context.Context) *monitoring.Monitor,
	recoveryManager *recoverymanager.Manager,
) *AttentionHandlers {
	return &AttentionHandlers{
		readAlerts: func(ctx context.Context) ([]alerts.Alert, []alerts.Alert, error) {
			if getMonitor == nil {
				return nil, nil, fmt.Errorf("monitor is not configured")
			}
			monitor := getMonitor(ctx)
			if monitor == nil || monitor.GetAlertManager() == nil {
				return nil, nil, fmt.Errorf("alert lifecycle is not available")
			}
			manager := monitor.GetAlertManager()
			return manager.GetActiveAlerts(), manager.GetAlertHistory(attentionHistoryReadLimit), nil
		},
		recoveryManager: recoveryManager,
	}
}

type attentionListResponse struct {
	Data    []ai.AttentionItem  `json:"data"`
	Summary ai.AttentionSummary `json:"summary"`
	Meta    struct {
		Page       int `json:"page"`
		Limit      int `json:"limit"`
		Total      int `json:"total"`
		TotalPages int `json:"totalPages"`
	} `json:"meta"`
}

func (h *AttentionHandlers) HandleAttention(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/ai/patrol/attention")
	switch {
	case path == "" || path == "/":
		h.handleAttentionList(w, r)
	case path == "/summary":
		h.handleAttentionSummary(w, r)
	case strings.HasPrefix(path, "/"):
		h.handleAttentionDetail(w, r, strings.TrimPrefix(path, "/"))
	default:
		http.NotFound(w, r)
	}
}

func (h *AttentionHandlers) handleAttentionList(w http.ResponseWriter, r *http.Request) {
	filter, page, limit, ok := parseAttentionListQuery(w, r.URL.Query())
	if !ok {
		return
	}
	projection, err := h.project(r.Context(), true)
	if err != nil {
		writeAttentionUnavailable(w, err)
		return
	}
	filtered, err := ai.FilterAttentionDetails(projection.Details, filter)
	if err != nil {
		writeErrorResponse(
			w,
			http.StatusBadRequest,
			"invalid_attention_filter",
			err.Error(),
			map[string]string{"filter": string(filter)},
		)
		return
	}
	paged, err := ai.PaginateAttentionDetails(filtered, page, limit)
	if err != nil {
		writeErrorResponse(
			w,
			http.StatusBadRequest,
			"invalid_attention_page",
			err.Error(),
			nil,
		)
		return
	}

	response := attentionListResponse{
		Data:    make([]ai.AttentionItem, 0, len(paged)),
		Summary: projection.Summary,
	}
	for _, detail := range paged {
		response.Data = append(response.Data, detail.Item)
	}
	response.Meta.Page = page
	response.Meta.Limit = limit
	response.Meta.Total = len(filtered)
	if len(filtered) > 0 {
		response.Meta.TotalPages = (len(filtered) + limit - 1) / limit
	}
	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to serialize Patrol attention list")
	}
}

func (h *AttentionHandlers) handleAttentionSummary(w http.ResponseWriter, r *http.Request) {
	projection, err := h.project(r.Context(), false)
	if err != nil {
		writeAttentionUnavailable(w, err)
		return
	}
	if err := utils.WriteJSONResponse(w, projection.Summary); err != nil {
		log.Error().Err(err).Msg("Failed to serialize Patrol attention summary")
	}
}

func (h *AttentionHandlers) handleAttentionDetail(
	w http.ResponseWriter,
	r *http.Request,
	rawID string,
) {
	itemID, err := url.PathUnescape(rawID)
	if err != nil || strings.TrimSpace(itemID) == "" {
		writeErrorResponse(
			w,
			http.StatusBadRequest,
			"invalid_attention_id",
			"Attention item ID is invalid.",
			nil,
		)
		return
	}
	projection, err := h.project(r.Context(), true)
	if err != nil {
		writeAttentionUnavailable(w, err)
		return
	}
	for _, detail := range projection.Details {
		if detail.Item.ID == itemID {
			if err := utils.WriteJSONResponse(w, detail); err != nil {
				log.Error().Err(err).Msg("Failed to serialize Patrol attention detail")
			}
			return
		}
	}
	writeErrorResponse(
		w,
		http.StatusNotFound,
		"attention_item_not_found",
		"Attention item was not found.",
		nil,
	)
}

func (h *AttentionHandlers) project(
	ctx context.Context,
	includeProtectionPosture bool,
) (ai.AttentionProjection, error) {
	if h == nil || h.readAlerts == nil {
		return ai.AttentionProjection{}, fmt.Errorf("attention lifecycle source is not configured")
	}
	active, history, err := h.readAlerts(ctx)
	if err != nil {
		return ai.AttentionProjection{}, err
	}
	postures := map[string]recoverymodel.ProtectionPosture{}
	postureComplete := true
	if includeProtectionPosture {
		resourceIDs := attentionResourceIDs(active, history)
		postures, postureComplete = h.loadProtectionPostures(
			ctx,
			GetOrgID(ctx),
			resourceIDs,
		)
	}
	projection := ai.ProjectAttentionItems(active, history, postures, time.Now().UTC())
	if !postureComplete {
		projection.Summary.CoverageState = "partial"
	}
	ai.GetPatrolMetrics().ObserveAttentionProjection(projection, projection.Summary.EvaluatedAt)
	return projection, nil
}

func (h *AttentionHandlers) loadProtectionPostures(
	ctx context.Context,
	orgID string,
	resourceIDs []string,
) (map[string]recoverymodel.ProtectionPosture, bool) {
	result := make(map[string]recoverymodel.ProtectionPosture, len(resourceIDs))
	if len(resourceIDs) == 0 {
		return result, true
	}
	if h == nil || h.recoveryManager == nil {
		return result, false
	}
	store, err := h.recoveryManager.StoreForOrg(orgID)
	if err != nil {
		log.Debug().Err(err).Msg("Patrol attention protection posture is unavailable")
		return result, false
	}

	complete := true
	for start := 0; start < len(resourceIDs); start += attentionPostureBatchSize {
		end := start + attentionPostureBatchSize
		if end > len(resourceIDs) {
			end = len(resourceIDs)
		}
		batch := resourceIDs[start:end]
		postures, _, listErr := store.ListProtectionPostures(ctx, recovery.ProtectionPostureQuery{
			SubjectResourceIDs: batch,
			Page:               1,
			Limit:              len(batch),
		})
		if listErr != nil {
			log.Debug().Err(listErr).Msg("Patrol attention posture batch is unavailable")
			complete = false
			continue
		}
		for _, posture := range postures {
			result[posture.SubjectResourceID] = posture
		}
	}
	return result, complete
}

func parseAttentionListQuery(
	w http.ResponseWriter,
	query url.Values,
) (ai.AttentionFilter, int, int, bool) {
	filter := ai.AttentionFilter(strings.TrimSpace(query.Get("filter")))
	if filter == "" {
		filter = ai.AttentionFilterActive
	}
	if !filter.Valid() {
		writeErrorResponse(
			w,
			http.StatusBadRequest,
			"invalid_attention_filter",
			"Filter must be active, open, acknowledged, suppressed, stale_unknown, resolved, or all.",
			map[string]string{"filter": string(filter)},
		)
		return "", 0, 0, false
	}
	page, valid := parsePositiveAttentionInt(w, query.Get("page"), 1, "page")
	if !valid {
		return "", 0, 0, false
	}
	limit, valid := parsePositiveAttentionInt(
		w,
		query.Get("limit"),
		ai.DefaultAttentionPageSize,
		"limit",
	)
	if !valid {
		return "", 0, 0, false
	}
	if limit > ai.MaxAttentionPageSize {
		writeErrorResponse(
			w,
			http.StatusBadRequest,
			"attention_limit_exceeded",
			fmt.Sprintf("Attention limit must not exceed %d.", ai.MaxAttentionPageSize),
			map[string]string{"limit": strconv.Itoa(ai.MaxAttentionPageSize)},
		)
		return "", 0, 0, false
	}
	return filter, page, limit, true
}

func parsePositiveAttentionInt(
	w http.ResponseWriter,
	raw string,
	fallback int,
	field string,
) (int, bool) {
	if strings.TrimSpace(raw) == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		writeErrorResponse(
			w,
			http.StatusBadRequest,
			"invalid_attention_"+field,
			"Attention "+field+" must be a positive integer.",
			map[string]string{field: raw},
		)
		return 0, false
	}
	return value, true
}

func attentionResourceIDs(active, history []alerts.Alert) []string {
	unique := make(map[string]struct{}, len(active)+len(history))
	add := func(values []alerts.Alert, history bool) {
		for _, alert := range values {
			if alert.OperationalRecord == nil {
				continue
			}
			if history && alert.OperationalRecord.State != operationaltrust.OperationalResolved {
				continue
			}
			if resourceID := strings.TrimSpace(alert.OperationalRecord.SubjectResourceID); resourceID != "" {
				unique[resourceID] = struct{}{}
			}
		}
	}
	add(active, false)
	add(history, true)
	result := make([]string, 0, len(unique))
	for resourceID := range unique {
		result = append(result, resourceID)
	}
	sort.Strings(result)
	return result
}

func writeAttentionUnavailable(w http.ResponseWriter, err error) {
	log.Warn().Err(err).Msg("Patrol attention read model is unavailable")
	writeErrorResponse(
		w,
		http.StatusServiceUnavailable,
		"attention_read_model_unavailable",
		"Patrol attention is temporarily unavailable. No calm or healthy state has been inferred.",
		nil,
	)
}
