package unifiedresources

import (
	"database/sql"
	"fmt"
	"strings"
)

// ActionRequestReplayMatches compares intent, not the resource or policy snapshot
// used to produce a plan. A replay never refreshes an immutable accepted plan.
// Compare supplied intent without redacting it into an apparent match. If the
// stored request lost information to redaction, equality is unknown and refuses.
func ActionRequestReplayMatches(record ActionAuditRecord, req ActionRequest, origin *ActionOrigin) bool {
	normalize := func(r ActionRequest) ActionRequest {
		r.RequestID = strings.TrimSpace(r.RequestID)
		r.ResourceID = CanonicalResourceID(r.ResourceID)
		r.CapabilityName = strings.TrimSpace(r.CapabilityName)
		r.Reason = strings.TrimSpace(r.Reason)
		r.Actor = NormalizeActionActor(r.Actor)
		r.RequestedBy = r.Actor.SubjectID
		return r
	}
	existing, replay := normalize(record.Request), normalize(req)
	return canonicalActionIdentityJSONEqual(existing, replay) &&
		canonicalActionIdentityJSONEqual(NormalizeActionOrigin(record.Origin), NormalizeActionOrigin(origin))
}

func hasActionRequestIdentity(req ActionRequest) bool {
	return strings.TrimSpace(req.RequestID) != "" && ValidateActionActor(req.Actor) == nil
}

// GetActionAuditByRequest returns the already accepted action for this trusted
// actor and request ID. Conflicting intent or legacy duplicate records fail
// closed. Unbound legacy requests retain their existing action-ID semantics.
func (s *SQLiteResourceStore) GetActionAuditByRequest(req ActionRequest, origin *ActionOrigin) (ActionAuditRecord, bool, error) {
	return getActionAuditByRequestFrom(s.db, req, origin, "")
}

type actionRequestQuerier interface {
	Query(string, ...any) (*sql.Rows, error)
}

func getActionAuditByRequestFrom(db actionRequestQuerier, req ActionRequest, origin *ActionOrigin, excludeID string) (ActionAuditRecord, bool, error) {
	if !hasActionRequestIdentity(req) {
		return ActionAuditRecord{}, false, nil
	}
	actor := NormalizeActionActor(req.Actor)
	rows, err := db.Query(`SELECT id, action_id, request_id, created_at, updated_at, state, decision_revision, request_json, plan_json, approvals_json, result_json, verification_outcome_json, origin_json
 FROM action_audits WHERE request_id=? AND id<>? AND json_valid(request_json)
 AND json_extract(request_json,'$.actor.subjectId')=?
 AND json_extract(request_json,'$.actor.kind')=?
 AND json_extract(request_json,'$.actor.credentialId')=?
 AND json_extract(request_json,'$.actor.orgId')=? LIMIT 2`, strings.TrimSpace(req.RequestID), excludeID, actor.SubjectID, string(actor.Kind), actor.CredentialID, actor.OrgID)
	if err != nil {
		return ActionAuditRecord{}, false, fmt.Errorf("read action request identity: %w", err)
	}
	defer rows.Close()
	var current ActionAuditRecord
	found := false
	for rows.Next() {
		record, err := scanActionAuditRecord(rows)
		if err != nil {
			return ActionAuditRecord{}, false, err
		}
		if found {
			return ActionAuditRecord{}, false, ErrActionIdentityConflict
		}
		current, found = record, true
	}
	if err := rows.Err(); err != nil {
		return ActionAuditRecord{}, false, err
	}
	if found && !ActionRequestReplayMatches(current, req, origin) {
		return current, true, ErrActionIdentityConflict
	}
	return current, found, nil
}

func (m *MemoryStore) GetActionAuditByRequest(req ActionRequest, origin *ActionOrigin) (ActionAuditRecord, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.actionAuditByRequestLocked(req, origin)
}

func (m *MemoryStore) actionAuditByRequestLocked(req ActionRequest, origin *ActionOrigin) (ActionAuditRecord, bool, error) {
	if !hasActionRequestIdentity(req) {
		return ActionAuditRecord{}, false, nil
	}
	var current ActionAuditRecord
	found := false
	for _, record := range m.actionAudits {
		if strings.TrimSpace(record.Request.RequestID) != strings.TrimSpace(req.RequestID) || !ActionActorsEqual(record.Request.Actor, req.Actor) {
			continue
		}
		if found {
			return ActionAuditRecord{}, false, ErrActionIdentityConflict
		}
		current, found = cloneActionAuditRecordForRead(record), true
	}
	if found && !ActionRequestReplayMatches(current, req, origin) {
		return current, true, ErrActionIdentityConflict
	}
	return current, found, nil
}
