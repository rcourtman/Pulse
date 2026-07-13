package unifiedresources

import (
	"testing"
	"time"
)

// Reproduces the v6.0.5 -> v6.1 upgrade shape: a chat-approval audit row
// whose JSON was written by v6.0.5 (no policyDecision, no approvalRequirement,
// no origin) and left in pending_approval. Lifecycle transitions CAS on the
// stored JSON bytes, so without open-time canonicalization the expiry sweep
// matches zero rows and the record is stranded in the open Actions inbox.
func TestExpireActionAuditsClearsLegacyPendingApproval(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}

	plannedAt := time.Now().UTC().Add(-4 * 24 * time.Hour)
	expiresAt := plannedAt.Add(5 * time.Minute)
	requestJSON := `{
		"requestId": "req-legacy-1",
		"resourceId": "pve02:204",
		"capabilityName": "pulse.control",
		"params": {"command": "start", "targetType": "vm", "targetId": "204", "approvalId": "req-legacy-1", "requestedBy": "assistant:pulse"},
		"reason": "start guest XVM-WIN11-DEV",
		"requestedBy": "assistant:pulse"
	}`
	planJSON := `{
		"actionId": "legacy-approval-1",
		"requestId": "req-legacy-1",
		"allowed": true,
		"requiresApproval": true,
		"approvalPolicy": "admin",
		"rollbackAvailable": true,
		"message": "start guest XVM-WIN11-DEV",
		"plannedAt": "` + plannedAt.Format(time.RFC3339Nano) + `",
		"expiresAt": "` + expiresAt.Format(time.RFC3339Nano) + `",
		"resourceVersion": "",
		"policyVersion": "",
		"planHash": "sha256:legacyhash",
		"preflight": {
			"target": "vm pve02:204",
			"currentState": "Resolved approval target: vm / pve02 / pve02:204.",
			"intendedChange": "start guest XVM-WIN11-DEV",
			"dryRunAvailable": false,
			"generatedAt": "` + plannedAt.Format(time.RFC3339Nano) + `"
		}
	}`
	if _, err := store.db.Exec(
		`INSERT INTO action_audits (id, action_id, canonical_id, request_id, created_at, updated_at, state, decision_revision, request_json, plan_json, approvals_json, result_json, verification_outcome_json, origin_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?, NULL, NULL, '', NULL)`,
		"legacy-approval-1", "legacy-approval-1", "pve02:204", "req-legacy-1",
		plannedAt, plannedAt, string(ActionStatePending), requestJSON, planJSON,
	); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	// Reopen: the upgrade path. Open-time canonicalization must rewrite the
	// legacy JSON so lifecycle CAS transitions can find the row again.
	store, err = NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	expired, err := store.ExpireActionAudits(time.Now().UTC(), 500)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(expired) != 1 {
		t.Fatalf("expected legacy pending approval to expire, got %d expired", len(expired))
	}

	current, found, err := store.GetActionAudit("legacy-approval-1")
	if err != nil || !found {
		t.Fatalf("get after sweep: found=%v err=%v", found, err)
	}
	if current.State != ActionStateExpired {
		t.Fatalf("state after sweep = %q, want %q", current.State, ActionStateExpired)
	}
}
