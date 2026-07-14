package unifiedresources

import (
	"testing"
	"time"
)

// These tests cover the uncovered branches of migrateActionAuditCanonicalization
// (store.go), which rewrites non-terminal action audit rows whose persisted JSON
// predates the current canonical shape. The happy path (one legacy pending row
// gets canonicalized on reopen and then expires) is already covered by
// legacy_expiry_repro_test.go and is deliberately NOT duplicated here.
//
// Branches exercised below:
//   - Idempotency: an already-canonical row is a pure no-op on a second reopen.
//   - Terminal-state exclusion: terminal rows are never selected/rewritten.
//   - Mixed batch: skip vs update partition within a single migration pass.
//   - Un-normalizable WARN branch: a row that fails strict normalization is
//     left as stored and does not poison store open.

// legacyCanonicalizationRequestJSON builds the v6.0.5-era request payload shape
// (no actor block, no policy linkage) that the canonicalization migration is
// responsible for rewriting on reopen. Mirrors the shape used by the repro test.
func legacyCanonicalizationRequestJSON(reqID, label string) string {
	return `{
		"requestId": "` + reqID + `",
		"resourceId": "pve02:204",
		"capabilityName": "pulse.control",
		"params": {"command": "start", "targetType": "vm", "targetId": "204", "approvalId": "` + reqID + `", "requestedBy": "assistant:pulse"},
		"reason": "start guest ` + label + `",
		"requestedBy": "assistant:pulse"
	}`
}

// legacyCanonicalizationPlanJSON builds a v6.0.5-era plan (no policyDecision,
// no approvalRequirement) that normalizes to a different byte form than stored.
func legacyCanonicalizationPlanJSON(actionID, reqID string) string {
	plannedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	expiresAt := plannedAt.Add(5 * time.Minute)
	return `{
		"actionId": "` + actionID + `",
		"requestId": "` + reqID + `",
		"allowed": true,
		"requiresApproval": true,
		"approvalPolicy": "admin",
		"rollbackAvailable": true,
		"message": "start guest ` + actionID + `",
		"plannedAt": "` + plannedAt.Format(time.RFC3339Nano) + `",
		"expiresAt": "` + expiresAt.Format(time.RFC3339Nano) + `",
		"resourceVersion": "",
		"policyVersion": "",
		"planHash": "sha256:` + actionID + `",
		"preflight": {
			"target": "vm pve02:204",
			"currentState": "Resolved approval target: vm / pve02 / pve02:204.",
			"intendedChange": "start guest ` + actionID + `",
			"dryRunAvailable": false,
			"generatedAt": "` + plannedAt.Format(time.RFC3339Nano) + `"
		}
	}`
}

func insertCanonicalizationAuditRow(t *testing.T, store *SQLiteResourceStore, id, reqID, state, requestJSON, planJSON string) {
	t.Helper()
	created := time.Now().UTC().Add(-4 * 24 * time.Hour)
	if _, err := store.db.Exec(
		`INSERT INTO action_audits (id, action_id, canonical_id, request_id, created_at, updated_at, state, decision_revision, request_json, plan_json, approvals_json, result_json, verification_outcome_json, origin_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?, NULL, NULL, '', NULL)`,
		id, id, "pve02:204", reqID, created, created, state, requestJSON, planJSON,
	); err != nil {
		t.Fatalf("insert legacy row %q: %v", id, err)
	}
}

func readCanonicalizationAuditJSON(t *testing.T, store *SQLiteResourceStore, id string) (requestJSON, planJSON, originJSON string) {
	t.Helper()
	if err := store.db.QueryRow(
		`SELECT request_json, plan_json, COALESCE(origin_json, '') FROM action_audits WHERE id = ?`,
		id,
	).Scan(&requestJSON, &planJSON, &originJSON); err != nil {
		t.Fatalf("read audit %q json columns: %v", id, err)
	}
	return requestJSON, planJSON, originJSON
}

// TestMigrateActionAuditCanonicalizationIsIdempotent hits both the per-row
// "already canonical" skip branch (request/plan/origin bytes already equal the
// re-marshaled canonical form) and the len(updates)==0 early return. The robust
// way to reach the skip branch without hand-authoring canonical JSON is to let
// the first reopen produce it, then assert a second reopen changes nothing.
func TestMigrateActionAuditCanonicalizationIsIdempotent(t *testing.T) {
	dir := t.TempDir()

	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	const id = "canonical-idem-1"
	legacyReq := legacyCanonicalizationRequestJSON("req-idem-1", id)
	insertCanonicalizationAuditRow(t, store, id, "req-idem-1", string(ActionStatePending), legacyReq, legacyCanonicalizationPlanJSON(id, "req-idem-1"))
	if err := store.Close(); err != nil {
		t.Fatalf("close initial store: %v", err)
	}

	// First reopen: the legacy JSON must be rewritten to canonical form.
	store, err = NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatalf("first reopen: %v", err)
	}
	reqFirst, planFirst, originFirst := readCanonicalizationAuditJSON(t, store, id)
	if reqFirst == legacyReq {
		t.Fatalf("first reopen did not rewrite request_json; canonicalization is a no-op on legacy bytes")
	}
	if reqFirst == "" || planFirst == "" {
		t.Fatalf("first reopen produced empty canonical json: request=%q plan=%q", reqFirst, planFirst)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close after first reopen: %v", err)
	}

	// Second reopen: row is already canonical, migration must be a pure no-op
	// (hits the byte-equality skip branch and the len(updates)==0 early return).
	store, err = NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatalf("second reopen: %v", err)
	}
	defer store.Close()
	reqSecond, planSecond, originSecond := readCanonicalizationAuditJSON(t, store, id)
	if reqSecond != reqFirst || planSecond != planFirst || originSecond != originFirst {
		t.Fatalf("second reopen mutated an already-canonical row:\n request: %q -> %q\n plan:    %q -> %q\n origin:  %q -> %q",
			reqFirst, reqSecond, planFirst, planSecond, originFirst, originSecond)
	}
}

// TestMigrateActionAuditCanonicalizationSkipsTerminalStates asserts the
// migration's WHERE clause (state IN planned/pending/approved/executing)
// excludes terminal states: an expired row with legacy-shape JSON is never
// selected, so its bytes survive a reopen byte-identical.
func TestMigrateActionAuditCanonicalizationSkipsTerminalStates(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	const id = "canonical-terminal-1"
	legacyReq := legacyCanonicalizationRequestJSON("req-terminal-1", id)
	legacyPlan := legacyCanonicalizationPlanJSON(id, "req-terminal-1")
	// ActionStateExpired is terminal and is NOT in the migration's WHERE list.
	insertCanonicalizationAuditRow(t, store, id, "req-terminal-1", string(ActionStateExpired), legacyReq, legacyPlan)
	if err := store.Close(); err != nil {
		t.Fatalf("close initial store: %v", err)
	}

	store, err = NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store.Close()

	reqAfter, planAfter, originAfter := readCanonicalizationAuditJSON(t, store, id)
	if reqAfter != legacyReq {
		t.Fatalf("terminal row request_json was rewritten despite being excluded by the WHERE clause: got=%q want=%q", reqAfter, legacyReq)
	}
	if planAfter != legacyPlan {
		t.Fatalf("terminal row plan_json was rewritten despite being excluded by the WHERE clause: got=%q want=%q", planAfter, legacyPlan)
	}
	if originAfter != "" {
		t.Fatalf("terminal row origin_json unexpectedly populated: %q", originAfter)
	}
}

// TestMigrateActionAuditCanonicalizationMixedBatchPartitionsRows exercises the
// per-row skip-vs-update partition within a single migration pass plus the
// UPDATE transaction loop. A guaranteed already-canonical row is produced by
// reopening once to canonicalize it; a second legacy row is then added so the
// next reopen sees both: the canonical row is skipped and the legacy row is
// rewritten in the same pass.
func TestMigrateActionAuditCanonicalizationMixedBatchPartitionsRows(t *testing.T) {
	dir := t.TempDir()

	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	const canonicalID = "canonical-mixed-1"
	insertCanonicalizationAuditRow(t, store, canonicalID, "req-mixed-1", string(ActionStatePending), legacyCanonicalizationRequestJSON("req-mixed-1", canonicalID), legacyCanonicalizationPlanJSON(canonicalID, "req-mixed-1"))
	if err := store.Close(); err != nil {
		t.Fatalf("close initial store: %v", err)
	}

	// First reopen canonicalizes the only row on disk, establishing a
	// guaranteed already-canonical baseline without hand-authoring canonical JSON.
	store, err = NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatalf("first reopen: %v", err)
	}
	canonicalReqBefore, canonicalPlanBefore, _ := readCanonicalizationAuditJSON(t, store, canonicalID)
	// While still open, insert a SECOND legacy non-terminal row that will need
	// rewriting on the next reopen. The first row stays canonical on disk.
	const legacyID = "canonical-mixed-2"
	legacyReq := legacyCanonicalizationRequestJSON("req-mixed-2", legacyID)
	legacyPlan := legacyCanonicalizationPlanJSON(legacyID, "req-mixed-2")
	insertCanonicalizationAuditRow(t, store, legacyID, "req-mixed-2", string(ActionStatePending), legacyReq, legacyPlan)
	if err := store.Close(); err != nil {
		t.Fatalf("close after inserting second row: %v", err)
	}

	// Second reopen: the migration scans BOTH non-terminal rows in one pass.
	// The canonical row must hit the skip branch (bytes unchanged); the legacy
	// row must hit the update branch (bytes changed) and be written via the
	// transactional UPDATE loop.
	store, err = NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatalf("second reopen: %v", err)
	}
	defer store.Close()

	canonicalReqAfter, canonicalPlanAfter, _ := readCanonicalizationAuditJSON(t, store, canonicalID)
	legacyReqAfter, legacyPlanAfter, _ := readCanonicalizationAuditJSON(t, store, legacyID)

	if canonicalReqAfter != canonicalReqBefore || canonicalPlanAfter != canonicalPlanBefore {
		t.Fatalf("already-canonical row was rewritten during mixed batch:\n request: %q -> %q\n plan:    %q -> %q",
			canonicalReqBefore, canonicalReqAfter, canonicalPlanBefore, canonicalPlanAfter)
	}
	if legacyReqAfter == legacyReq {
		t.Fatalf("legacy row request_json was not rewritten during mixed batch")
	}
	if legacyPlanAfter == legacyPlan {
		t.Fatalf("legacy row plan_json was not rewritten during mixed batch")
	}
}

// TestMigrateActionAuditCanonicalizationLeavesUnnormalizableRowUntouched reaches
// the strict-error WARN branch: a non-terminal (pending) row whose stored JSON
// parses cleanly but fails strict NormalizeActionAuditRecord (here: a request
// with no resourceId). The migration logs a WARN, leaves the row as stored, and
// must not return an error from store open.
func TestMigrateActionAuditCanonicalizationLeavesUnnormalizableRowUntouched(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	const id = "canonical-warn-1"
	// Valid JSON, but no resourceId: strict normalization returns
	// "action request resource id required". The plan is a normal legacy plan so
	// only the request triggers the strict error.
	warnReq := `{
		"requestId": "req-warn-1",
		"capabilityName": "pulse.control",
		"params": {"command": "start", "targetType": "vm", "targetId": "204", "approvalId": "req-warn-1", "requestedBy": "assistant:pulse"},
		"reason": "start guest ` + id + `",
		"requestedBy": "assistant:pulse"
	}`
	warnPlan := legacyCanonicalizationPlanJSON(id, "req-warn-1")
	insertCanonicalizationAuditRow(t, store, id, "req-warn-1", string(ActionStatePending), warnReq, warnPlan)

	// Honesty guard: prove the row genuinely reaches the strict-error path.
	// getActionAuditFrom returns the read-time fallback (no error); re-running
	// strict NormalizeActionAuditRecord on that fallback must fail. Without this
	// check, "bytes unchanged" could merely mean the row was already canonical.
	readBack, found, err := store.getActionAudit(id)
	if err != nil || !found {
		t.Fatalf("read warn row before reopen: found=%v err=%v", found, err)
	}
	if _, strictErr := NormalizeActionAuditRecord(readBack); strictErr == nil {
		t.Fatalf("warn row does not actually trigger strict normalization error; WARN branch is not reached by this fixture")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close initial store: %v", err)
	}

	// Reopen must succeed even though the row cannot be canonicalized.
	store, err = NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatalf("reopen with un-normalizable row: %v", err)
	}
	defer store.Close()

	// The row must be left exactly as stored.
	reqAfter, planAfter, originAfter := readCanonicalizationAuditJSON(t, store, id)
	if reqAfter != warnReq {
		t.Fatalf("un-normalizable row request_json was rewritten: got=%q want=%q", reqAfter, warnReq)
	}
	if planAfter != warnPlan {
		t.Fatalf("un-normalizable row plan_json was rewritten: got=%q want=%q", planAfter, warnPlan)
	}
	if originAfter != "" {
		t.Fatalf("un-normalizable row origin_json unexpectedly populated: %q", originAfter)
	}
}
