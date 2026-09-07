package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// executeQueryAction reads the tenant-pinned durable record, independently of
// cached inventory and session prose. It grants no action authority.
func (e *PulseToolExecutor) executeQueryAction(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	id := strings.TrimSpace(stringArg(args, "action_id"))
	if id == "" {
		return NewErrorResult(fmt.Errorf("action_id is required")), nil
	}
	if e.actionAuditStore == nil {
		return NewErrorResult(fmt.Errorf("canonical action records are unavailable")), nil
	}
	record, found, err := e.actionAuditStore.GetActionAudit(id)
	if err != nil {
		return NewErrorResult(fmt.Errorf("canonical action record could not be read")), nil
	}
	if !found || (record.Request.Actor.OrgID != "" && record.Request.Actor.OrgID != e.orgID) {
		return NewErrorResult(fmt.Errorf("action record not found")), nil
	}
	record = unifiedresources.RedactAuditRecord(record)
	decisions := make([]map[string]interface{}, 0, len(record.Approvals))
	for _, decision := range record.Approvals {
		decisions = append(decisions, map[string]interface{}{
			"outcome":   decision.Outcome,
			"actor":     decision.Actor,
			"timestamp": decision.Timestamp,
			"reason":    unifiedresources.RedactAuditText(decision.Reason),
		})
	}
	// Keep canonical plan and outcome semantics, but never expose request
	// parameters, credential bindings or raw execution output through this read.
	response := map[string]interface{}{
		"source":           "canonical_action_audit",
		"queried_at":       time.Now().UTC(),
		"action_id":        record.ID,
		"resource_id":      record.Request.ResourceID,
		"capability_name":  record.Request.CapabilityName,
		"state":            record.State,
		"decisions":        decisions,
		"updated_at":       record.UpdatedAt,
		"plan":             record.Plan,
		"origin":           record.Origin,
		"action_result_v2": unifiedresources.CanonicalActionResultV2(record),
		"action_url":       "/actions?action=" + url.QueryEscape(record.ID),
		"evidence_limit":   "This is the recorded action outcome, not a claim about the resource's current health. Plan preflight describes state at planning time. Independent verification establishes only its recorded postcondition at its observation time.",
	}
	if rs, err := e.readStateForControl(); err == nil {
		metadata := newGovernedQueryMetadataResolver(rs).Resolve(record.Request.ResourceID)
		if metadata.Policy != nil {
			response["policy"] = metadata.Policy
		}
	}
	return NewJSONResult(response), nil
}
