package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const operationalTrustActionOriginSurface = "operational_trust_attention"

func isAttentionActionPlanPath(path string) bool {
	_, capability, ok := parseAttentionActionPlanPath(path)
	return ok && capability != ""
}

func parseAttentionActionPlanPath(path string) (string, string, bool) {
	path = strings.Trim(path, "/")
	actionBoundary := strings.LastIndex(path, "/actions/")
	if actionBoundary <= 0 {
		return "", "", false
	}
	itemPart := path[:actionBoundary]
	actionPart := path[actionBoundary+len("/actions/"):]
	actionSegments := strings.Split(actionPart, "/")
	if len(actionSegments) != 2 || actionSegments[1] != "plan" {
		return "", "", false
	}
	itemID, err := url.PathUnescape(itemPart)
	if err != nil {
		return "", "", false
	}
	capability, err := url.PathUnescape(actionSegments[0])
	if err != nil {
		return "", "", false
	}
	itemID = strings.TrimSpace(itemID)
	capability = strings.TrimSpace(capability)
	return itemID, capability, itemID != "" && capability != ""
}

func (h *AttentionHandlers) projectAttentionActions(
	r *http.Request,
	details []ai.AttentionItemDetail,
) {
	if h == nil || h.resources == nil || len(details) == 0 {
		return
	}
	orgID := GetOrgID(r.Context())
	registry, registryErr := h.resources.buildRegistry(orgID)
	store, storeErr := h.resources.getStore(orgID)
	actor, actorErr := actionActorForRequest(h.resources.cfg, r, orgID)
	authorized := actorErr == nil &&
		h.hasAutoFixLicense != nil &&
		h.hasAutoFixLicense(r.Context()) &&
		h.actionAuthority.authorizeActor(r.Context(), orgID, actor, auth.ActionPlan) == nil &&
		h.actionAuthority.authorizeActor(r.Context(), orgID, actor, auth.ActionApprove) == nil &&
		h.actionAuthority.authorizeActor(r.Context(), orgID, actor, auth.ActionExecute) == nil

	actions := make(map[string]unified.ActionAuditRecord, len(details))
	if storeErr == nil {
		if reader, ok := store.(unified.OperationalActionAuditOriginBatchReader); ok {
			recordIDs := make([]string, 0, len(details))
			for index := range details {
				recordIDs = append(recordIDs, details[index].Item.OperationalRecordID)
			}
			if records, err := reader.GetLatestActionAuditsByOperationalRecords(
				operationalTrustActionOriginSurface,
				recordIDs,
			); err == nil {
				actions = records
			}
		}
	}
	now := time.Now().UTC()
	for index := range details {
		detail := &details[index]
		candidate := ai.AttentionActionCandidate{Authorized: authorized}
		if record, found := actions[detail.Item.OperationalRecordID]; found &&
			ai.AttentionActionMatchesItem(detail.Item, &record) {
			candidate.Action = &record
			detail.Item.VerificationState = ai.AttentionActionVerificationState(&record)
		}
		if registryErr == nil {
			if resource, found := registry.Get(detail.Item.SubjectResourceID); found {
				candidate.Resource = resource
				candidate.Readiness = unified.ResourceActionReadiness{
					Name:       ai.AttentionDockerRestartCapability,
					Available:  false,
					ReasonCode: "executor_readiness_unavailable",
					Reason:     "Current action readiness could not be verified.",
				}
				if checker, ok := h.resources.actionExecutor.(ActionAvailabilityChecker); ok {
					candidate.Readiness = checker.CheckActionAvailable(
						r.Context(),
						unified.ActionRequest{
							RequestID:      attentionActionRequestID(detail.Item.OperationalRecordID),
							ResourceID:     detail.Item.SubjectResourceID,
							CapabilityName: ai.AttentionDockerRestartCapability,
							Params:         map[string]any{},
							Reason:         attentionActionReason(detail.Item),
						},
						*resource,
					)
				}
			}
		}
		offer, reason := ai.ProjectAttentionAction(detail, candidate, now)
		if reason == ai.AttentionActionEligible {
			operationaltrust.GetMetrics().ObserveActionOffer("eligible")
		} else {
			operationaltrust.GetMetrics().ObserveActionOffer("ineligible")
		}
		if reason == ai.AttentionActionEligible {
			detail.Item.AvailableActions = []ai.AttentionActionOffer{offer}
		}
	}
}

func (h *AttentionHandlers) handleAttentionActionPlan(
	w http.ResponseWriter,
	r *http.Request,
	path string,
) {
	if mock.IsMockEnabled() {
		writeJSONError(w, http.StatusForbidden, "attention_action_mock_mode", "Cannot plan actions in mock mode")
		return
	}
	itemID, capability, ok := parseAttentionActionPlanPath(path)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "attention_action_not_found", "Attention action was not found")
		return
	}
	if capability != ai.AttentionDockerRestartCapability {
		writeJSONError(w, http.StatusConflict, "attention_action_ineligible", "This action is not eligible for the selected attention item")
		return
	}
	if h == nil || h.resources == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "attention_actions_unavailable", "Governed actions are unavailable")
		return
	}
	if h.hasAutoFixLicense == nil || !h.hasAutoFixLicense(r.Context()) {
		WriteLicenseRequired(
			w,
			featureAIAutoFixKey,
			"Patrol fix actions require Pulse Pro",
		)
		return
	}

	projection, err := h.project(r, true)
	if err != nil {
		writeAttentionUnavailable(w, err)
		return
	}
	var detail *ai.AttentionItemDetail
	for index := range projection.Details {
		if projection.Details[index].Item.ID == itemID {
			detail = &projection.Details[index]
			break
		}
	}
	if detail == nil {
		writeJSONError(w, http.StatusNotFound, "attention_item_not_found", "Attention item was not found")
		return
	}
	enriched := []ai.AttentionItemDetail{*detail}
	h.projectAttentionActions(r, enriched)
	detail = &enriched[0]
	var offer *ai.AttentionActionOffer
	for index := range detail.Item.AvailableActions {
		if detail.Item.AvailableActions[index].Capability == capability {
			offer = &detail.Item.AvailableActions[index]
			break
		}
	}
	if offer == nil {
		writeJSONError(w, http.StatusConflict, "attention_action_ineligible", "Current evidence, capability state, or operator authority does not permit this action")
		return
	}

	orgID := GetOrgID(r.Context())
	actor, err := actionActorForRequest(h.resources.cfg, r, orgID)
	if err != nil ||
		h.actionAuthority.authorizeActor(r.Context(), orgID, actor, auth.ActionPlan) != nil ||
		h.actionAuthority.authorizeActor(r.Context(), orgID, actor, auth.ActionApprove) != nil ||
		h.actionAuthority.authorizeActor(r.Context(), orgID, actor, auth.ActionExecute) != nil {
		writeJSONError(w, http.StatusForbidden, "attention_action_denied", "You do not have permission to review and run this action")
		return
	}
	if offer.ActionID != "" {
		record, found, getErr := h.resources.ActionLifecycle().Get(orgID, offer.ActionID)
		if getErr != nil || !found {
			writeJSONError(w, http.StatusServiceUnavailable, "attention_action_query_failed", "The governed action record is unavailable")
			return
		}
		writeAttentionActionPlan(w, record.Plan)
		return
	}

	plan, err := h.resources.ActionLifecycle().PlanWithOptions(
		r.Context(),
		orgID,
		unified.ActionRequest{
			RequestID:      attentionActionRequestID(detail.Item.OperationalRecordID),
			ResourceID:     detail.Item.SubjectResourceID,
			CapabilityName: capability,
			Params:         map[string]any{},
			Reason:         attentionActionReason(detail.Item),
		},
		actionlifecycle.PlanOptions{
			Actor: actor,
			Origin: &unified.ActionOrigin{
				Surface:             operationalTrustActionOriginSurface,
				OperationalRecordID: detail.Item.OperationalRecordID,
				EvidenceIDs:         append([]string(nil), offer.EvidenceIDs...),
			},
		},
	)
	if err != nil {
		writeActionPlanError(w, err)
		return
	}
	writeAttentionActionPlan(w, plan)
}

func writeAttentionActionPlan(w http.ResponseWriter, plan unified.ActionPlan) {
	if err := utils.WriteJSONResponse(w, plan); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "attention_action_encode_failed", "Failed to encode action plan")
	}
}

func attentionActionRequestID(operationalRecordID string) string {
	return "operational-trust:" + strings.TrimSpace(operationalRecordID) + ":" +
		ai.AttentionDockerRestartCapability
}

func attentionActionReason(item ai.AttentionItem) string {
	name := strings.TrimSpace(item.SubjectResourceName)
	if name == "" {
		name = strings.TrimSpace(item.SubjectResourceID)
	}
	return fmt.Sprintf(
		"Restart %s after fresh confirmed evidence reported an unhealthy container.",
		name,
	)
}
