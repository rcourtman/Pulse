package tools

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

const approvalAuditActor = approval.RequesterPulseAssistant

func actionApprovalRecordForExecution(action unifiedresources.ActionAuditRecord, approvalID, actor string, now time.Time, fallbackOutcome unifiedresources.ApprovalOutcome) unifiedresources.ActionApprovalRecord {
	if fallbackOutcome == "" {
		fallbackOutcome = unifiedresources.OutcomeApproved
	}
	records := approvalRecordsForID(approvalID, &action.Plan)
	if len(records) > 0 {
		record := records[len(records)-1]
		if record.Outcome == "" {
			record.Outcome = fallbackOutcome
		}
		if record.Timestamp.IsZero() {
			record.Timestamp = now
		}
		if strings.TrimSpace(record.Actor) == "" {
			record.Actor = actor
		}
		return record
	}
	actorBinding := legacyApprovalActorBinding(approvalID, actor, action.Request.Actor.OrgID)
	evidence := legacyApprovalEvidence(actorBinding, &action.Plan, fallbackOutcome, now)
	return unifiedresources.ActionApprovalRecord{
		Actor:        actorBinding.SubjectID,
		ActorBinding: actorBinding,
		Method:       evidence.Method,
		Timestamp:    now,
		Outcome:      fallbackOutcome,
		Evidence:     &evidence,
	}
}

// RecordApprovalDecision updates the unified action audit for an approval that
// reached a terminal or pre-execution decision state.
func (e *PulseToolExecutor) RecordApprovalDecision(approvalID string, state unifiedresources.ActionState, actor, message string) {
	var store unifiedresources.ResourceStore
	if e != nil {
		store = e.actionAuditStore
	}
	RecordApprovalDecision(store, approvalID, state, actor, message)
}

// RecordApprovalDecision updates the unified action audit for an approval that
// reached a terminal or pre-execution decision state.
func RecordApprovalDecision(store unifiedresources.ResourceStore, approvalID string, state unifiedresources.ActionState, actor, message string) {
	req := approvalRequestForID(approvalID)
	if req == nil || req.Plan == nil {
		return
	}
	if strings.TrimSpace(actor) == "" {
		actor = approvalDecisionActor(req, approvalAuditActor)
	}
	if strings.TrimSpace(message) == "" {
		message = string(state)
	}
	record := actionAuditRecordFromApproval(req, state, actor)
	record.Approvals = approvalRecordsForID(req.ID, req.Plan)
	if recordApprovalDecisionAtomically(store, req.ID, record, actor) {
		return
	}
	event := unifiedresources.ActionLifecycleEvent{ActionID: req.Plan.ActionID, Timestamp: time.Now().UTC(), State: state, Actor: actor, Message: message}
	if _, _, err := store.CreateActionAudit(record, []unifiedresources.ActionLifecycleEvent{event}); err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to create action audit for approval decision")
	}
}

func recordApprovalDecisionAtomically(store unifiedresources.ResourceStore, approvalID string, record unifiedresources.ActionAuditRecord, actor string) bool {
	if store == nil {
		return false
	}
	if record.State != unifiedresources.ActionStateApproved && record.State != unifiedresources.ActionStateRejected {
		return false
	}

	current, ok, err := store.GetActionAudit(record.ID)
	if err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to query action audit before approval decision")
		return false
	}
	if !ok {
		return false
	}
	if current.State != unifiedresources.ActionStatePending {
		if current.State != record.State {
			log.Warn().
				Str("action_id", record.ID).
				Str("current_state", string(current.State)).
				Str("decision_state", string(record.State)).
				Msg("ignoring approval decision for action audit that has already moved past pending")
		}
		return true
	}

	outcome := unifiedresources.OutcomeApproved
	if record.State == unifiedresources.ActionStateRejected {
		outcome = unifiedresources.OutcomeRejected
	}
	approvalRecord := actionApprovalRecordForExecution(current, approvalID, actor, time.Now().UTC(), outcome)
	approvalRecord.Outcome = outcome
	updated, event, err := unifiedresources.ApplyActionDecision(current, approvalRecord, approvalRecord.Timestamp)
	if err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to normalize approval decision")
		return false
	}
	if err := store.RecordActionDecision(updated, event); err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to persist approval decision")
		return false
	}
	return true
}

func actionAuditRecordFromApproval(req *approval.ApprovalRequest, state unifiedresources.ActionState, _ string) unifiedresources.ActionAuditRecord {
	now := time.Now().UTC()
	plan := *req.Plan
	if plan.ApprovalPolicy == "" {
		plan.ApprovalPolicy = unifiedresources.ApprovalAdmin
	}
	if plan.ApprovalRequirement.Version == 0 {
		plan.ApprovalRequirement = unifiedresources.ApprovalRequirementForFloor(plan.ApprovalPolicy)
	}
	createdAt := plan.PlannedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	requestID := strings.TrimSpace(plan.RequestID)
	if requestID == "" {
		requestID = strings.TrimSpace(req.ID)
	}
	params := map[string]any{
		"command":    req.Command,
		"targetType": req.TargetType,
		"targetId":   req.TargetID,
		"targetName": req.TargetName,
		"approvalId": req.ID,
	}
	if orgID := strings.TrimSpace(req.OrgID); orgID != "" {
		params["orgId"] = orgID
	}
	requestedBy := approval.RequesterForRequest(req)
	requestActor := unifiedresources.ActionActor{
		SubjectID:    requestedBy,
		Kind:         unifiedresources.ActionActorService,
		CredentialID: "approval-request:" + strings.TrimSpace(req.ID),
		OrgID:        approval.NormalizeOrgID(req.OrgID),
	}
	return unifiedresources.ActionAuditRecord{
		ID:        plan.ActionID,
		CreatedAt: createdAt,
		UpdatedAt: now,
		State:     state,
		Request: unifiedresources.ActionRequest{
			RequestID:      requestID,
			ResourceID:     approvalAuditResourceID(req.TargetType, req.TargetID, req.TargetName),
			CapabilityName: approvalCapabilityForTargetType(req.TargetType),
			Params:         params,
			Reason:         strings.TrimSpace(req.Context),
			RequestedBy:    requestedBy,
			Actor:          requestActor,
		},
		Plan: plan,
	}
}

func approvalRequestForID(approvalID string) *approval.ApprovalRequest {
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return nil
	}
	store := approval.GetStore()
	if store == nil {
		return nil
	}
	req, ok := store.GetApproval(approvalID)
	if !ok || req == nil {
		return nil
	}
	return req
}

func approvalDecisionActor(req *approval.ApprovalRequest, fallback string) string {
	if req != nil {
		if actor := strings.TrimSpace(req.DecidedBy); actor != "" {
			return actor
		}
	}
	if actor := strings.TrimSpace(fallback); actor != "" {
		return actor
	}
	return approvalAuditActor
}

func approvalCapabilityForTargetType(targetType string) string {
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "docker":
		return agentcapabilities.PulseDockerToolName
	case "file":
		return agentcapabilities.PulseFileEditToolName
	case "kubernetes":
		return agentcapabilities.PulseKubernetesToolName
	default:
		return agentcapabilities.PulseControlToolName
	}
}

func approvalAuditResourceID(targetType, targetID, targetName string) string {
	targetType = strings.TrimSpace(targetType)
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		targetID = strings.TrimSpace(targetName)
	}
	if targetID == "" {
		return strings.TrimSpace(targetType)
	}
	if targetType == "" || strings.Contains(targetID, ":") {
		return targetID
	}
	return targetType + ":" + targetID
}

func approvalRecordsForID(approvalID string, plan *unifiedresources.ActionPlan) []unifiedresources.ActionApprovalRecord {
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return nil
	}
	store := approval.GetStore()
	if store == nil {
		return nil
	}
	req, ok := store.GetApproval(approvalID)
	if !ok || req == nil {
		return nil
	}

	outcome := unifiedresources.OutcomeApproved
	if req.Status == approval.StatusDenied {
		outcome = unifiedresources.OutcomeRejected
	}
	actorBinding := legacyApprovalActorBinding(approvalID, req.DecidedBy, req.OrgID)
	evidence := legacyApprovalEvidence(actorBinding, plan, outcome, approvalTimestamp(req))
	record := unifiedresources.ActionApprovalRecord{
		Actor:        actorBinding.SubjectID,
		ActorBinding: actorBinding,
		Method:       evidence.Method,
		Timestamp:    evidence.IssuedAt,
		Outcome:      outcome,
		Reason:       strings.TrimSpace(req.Context),
		Evidence:     &evidence,
	}
	return []unifiedresources.ActionApprovalRecord{record}
}

func legacyApprovalActorBinding(approvalID, subject, orgID string) unifiedresources.ActionActor {
	return unifiedresources.ActionActor{
		SubjectID:    strings.TrimSpace(subject),
		Kind:         unifiedresources.ActionActorUser,
		CredentialID: "approval-session:" + strings.TrimSpace(approvalID),
		OrgID:        approval.NormalizeOrgID(orgID),
	}
}

func legacyApprovalEvidence(actor unifiedresources.ActionActor, plan *unifiedresources.ActionPlan, outcome unifiedresources.ApprovalOutcome, issuedAt time.Time) unifiedresources.ApprovalEvidence {
	evidence := unifiedresources.ApprovalEvidence{
		Version:  1,
		Method:   unifiedresources.MethodSession,
		Actor:    actor,
		OrgID:    actor.OrgID,
		Outcome:  outcome,
		IssuedAt: issuedAt.UTC(),
	}
	if plan != nil {
		evidence.ActionID = strings.TrimSpace(plan.ActionID)
		evidence.PlanHash = strings.TrimSpace(plan.PlanHash)
	}
	return evidence
}

func approvalTimestamp(req *approval.ApprovalRequest) time.Time {
	if req == nil || req.DecidedAt == nil {
		return time.Now().UTC()
	}
	return req.DecidedAt.UTC()
}
