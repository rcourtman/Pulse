package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

const (
	defaultPatrolReceiptLimit = 6
	maxPatrolReceiptLimit     = 50
)

var patrolReceiptOriginSurfaces = []string{
	patrolActionOriginSurface,
	operationalTrustActionOriginSurface,
}

type patrolWorkReceipt struct {
	ActionID            string                               `json:"actionId"`
	ResourceID          string                               `json:"resourceId"`
	ResourceName        string                               `json:"resourceName"`
	ResourceType        unifiedresources.ResourceType        `json:"resourceType,omitempty"`
	CapabilityName      string                               `json:"capabilityName"`
	VerifiedAt          time.Time                            `json:"verifiedAt"`
	EvidenceClass       unifiedresources.ActionEvidenceClass `json:"evidenceClass"`
	OriginSurface       string                               `json:"originSurface"`
	FindingID           string                               `json:"findingId,omitempty"`
	OperationalRecordID string                               `json:"operationalRecordId,omitempty"`
}

type patrolWorkReceiptListResponse struct {
	Data  []patrolWorkReceipt `json:"data"`
	Count int                 `json:"count"`
	Limit int                 `json:"limit"`
}

func (h *AttentionHandlers) handleAttentionReceipts(w http.ResponseWriter, r *http.Request) {
	limit, ok := parsePatrolReceiptLimit(w, r)
	if !ok {
		return
	}
	if h == nil || h.resources == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "patrol_receipts_unavailable", "Verified Patrol work is unavailable.", nil)
		return
	}
	orgID := GetOrgID(r.Context())
	store, err := h.resources.getStore(orgID)
	if err != nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "patrol_receipts_unavailable", "Verified Patrol work is unavailable.", nil)
		return
	}
	reader, ok := store.(unifiedresources.VerifiedActionAuditOriginReader)
	if !ok {
		writeErrorResponse(w, http.StatusServiceUnavailable, "patrol_receipts_unavailable", "Verified Patrol work is unavailable.", nil)
		return
	}
	records, err := reader.GetVerifiedActionAuditsByOrigins(patrolReceiptOriginSurfaces, limit)
	if err != nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "patrol_receipts_unavailable", "Verified Patrol work is unavailable.", nil)
		return
	}

	registry, _ := h.resources.buildRegistry(orgID)
	receipts := make([]patrolWorkReceipt, 0, len(records))
	for _, record := range records {
		if receipt, projected := projectPatrolWorkReceipt(record, registry); projected {
			receipts = append(receipts, receipt)
		}
	}
	response := patrolWorkReceiptListResponse{Data: receipts, Count: len(receipts), Limit: limit}
	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to serialize verified Patrol work receipts")
	}
}

func parsePatrolReceiptLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return defaultPatrolReceiptLimit, true
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > maxPatrolReceiptLimit {
		writeErrorResponse(
			w,
			http.StatusBadRequest,
			"invalid_patrol_receipt_limit",
			"Receipt limit must be between 1 and 50.",
			map[string]string{"limit": strconv.Itoa(maxPatrolReceiptLimit)},
		)
		return 0, false
	}
	return limit, true
}

func projectPatrolWorkReceipt(
	record unifiedresources.ActionAuditRecord,
	registry *unifiedresources.ResourceRegistry,
) (patrolWorkReceipt, bool) {
	if record.Origin == nil {
		return patrolWorkReceipt{}, false
	}
	originSurface := strings.TrimSpace(record.Origin.Surface)
	if originSurface != patrolActionOriginSurface &&
		originSurface != operationalTrustActionOriginSurface {
		return patrolWorkReceipt{}, false
	}
	truth := unifiedresources.CanonicalActionResultV2(record)
	if record.State != unifiedresources.ActionStateCompleted ||
		truth.Execution.Status != unifiedresources.ActionExecutionSucceeded ||
		truth.Verification.Status != unifiedresources.ActionVerificationConfirmed {
		return patrolWorkReceipt{}, false
	}
	resourceID := strings.TrimSpace(record.Request.ResourceID)
	resourceName := resourceID
	var resourceType unifiedresources.ResourceType
	if registry != nil {
		if resource, found := registry.Get(resourceID); found {
			resourceName = strings.TrimSpace(resource.Name)
			resourceType = resource.Type
		}
	}
	if resourceName == "" {
		resourceName = resourceID
	}
	return patrolWorkReceipt{
		ActionID:            record.ID,
		ResourceID:          resourceID,
		ResourceName:        resourceName,
		ResourceType:        resourceType,
		CapabilityName:      strings.TrimSpace(record.Request.CapabilityName),
		VerifiedAt:          record.UpdatedAt.UTC(),
		EvidenceClass:       truth.Verification.EvidenceClass,
		OriginSurface:       originSurface,
		FindingID:           strings.TrimSpace(record.Origin.FindingID),
		OperationalRecordID: strings.TrimSpace(record.Origin.OperationalRecordID),
	}, true
}
