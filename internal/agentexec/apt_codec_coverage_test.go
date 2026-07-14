package agentexec

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

// --- JSON / identity helpers for coverage tests ---

func covHostUpdateDigest(actionID, operation string, opVersion int, hash string) string {
	d, err := hostUpdateRequestDigest(HostUpdatePayload{
		ActionID:              actionID,
		Operation:             operation,
		OperationVersion:      opVersion,
		ExpectedInventoryHash: hash,
	})
	if err != nil {
		panic(err)
	}
	return d
}

func covHostStorageCleanupDigest(actionID, operation string, opVersion int, fingerprint string) string {
	d, err := hostStorageCleanupRequestDigest(HostStorageCleanupPayload{
		ActionID:            actionID,
		Operation:           operation,
		OperationVersion:    opVersion,
		ExpectedFingerprint: fingerprint,
	})
	if err != nil {
		panic(err)
	}
	return d
}

func covBuildHostUpdatePayloadJSON(t *testing.T, requestID, actionID, operation string, opVersion int, hash string, timeout int) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"request_id":              requestID,
		"action_id":               actionID,
		"operation":               operation,
		"operation_version":       opVersion,
		"request_digest":          covHostUpdateDigest(actionID, operation, opVersion, hash),
		"expected_inventory_hash": hash,
		"timeout":                 timeout,
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func covBuildHostStorageCleanupPayloadJSON(t *testing.T, requestID, actionID, operation string, opVersion int, fingerprint string, timeout int) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"request_id":           requestID,
		"action_id":            actionID,
		"operation":            operation,
		"operation_version":    opVersion,
		"request_digest":       covHostStorageCleanupDigest(actionID, operation, opVersion, fingerprint),
		"expected_fingerprint": fingerprint,
		"timeout":              timeout,
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func covWithJSONField(t *testing.T, raw []byte, key string, value any) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	m[key] = value
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func covHostUpdateIdentity(agentID string) operationreceipt.Identity {
	req := HostUpdatePayload{
		RequestID:             "cov.dispatch.1",
		ActionID:              "cov-action",
		Operation:             HostUpdateOperationInstall,
		ExpectedInventoryHash: "sha256:" + strings.Repeat("a", 64),
	}
	if err := BindHostUpdatePayload(&req); err != nil {
		panic(err)
	}
	return HostUpdateOperationIdentity(agentID, req)
}

func covHostCleanupIdentity(agentID string) operationreceipt.Identity {
	req := HostStorageCleanupPayload{
		RequestID:           "cov.dispatch.1",
		ActionID:            "cov-action",
		Operation:           HostStorageCleanupOperationPackageCache,
		ExpectedFingerprint: "sha256:" + strings.Repeat("c", 64),
	}
	if err := BindHostStorageCleanupPayload(&req); err != nil {
		panic(err)
	}
	return HostStorageCleanupOperationIdentity(agentID, req)
}

func covTerminalRecord(identity operationreceipt.Identity, resultKind string, resultVersion int, result json.RawMessage, terminalAt time.Time) *operationreceipt.Record {
	return &operationreceipt.Record{
		Identity:      identity,
		State:         operationreceipt.StateTerminal,
		AcceptedAt:    terminalAt.Add(-10 * time.Minute),
		StartedAt:     terminalAt.Add(-9 * time.Minute),
		TerminalAt:    terminalAt,
		ResultKind:    resultKind,
		ResultVersion: resultVersion,
		Result:        result,
	}
}

func covAcceptedRecord(identity operationreceipt.Identity, acceptedAt time.Time) *operationreceipt.Record {
	return &operationreceipt.Record{
		Identity:   identity,
		State:      operationreceipt.StateAccepted,
		AcceptedAt: acceptedAt,
	}
}

// ---------------------------------------------------------------------------
// DecodeHostUpdatePayload — apt_codec.go:32
// ---------------------------------------------------------------------------

func TestCoverageDecodeHostUpdatePayloadRejectionArms(t *testing.T) {
	hash := "sha256:" + strings.Repeat("a", 64)
	valid := covBuildHostUpdatePayloadJSON(t, "r1", "a1", HostUpdateOperationInstall, HostAPTOperationVersion, hash, 30)

	// Success case.
	t.Run("valid payload decodes", func(t *testing.T) {
		got, err := DecodeHostUpdatePayload(valid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.RequestID != "r1" || got.ActionID != "a1" || got.Operation != HostUpdateOperationInstall {
			t.Fatalf("decoded payload mismatch: %#v", got)
		}
	})

	for _, tc := range []struct {
		name    string
		body    []byte
		wantSub string
	}{
		{"empty payload", []byte(""), "empty"},
		{"whitespace only", []byte("   \n\t "), "empty"},
		{"malformed JSON", []byte(`{"request_id":"incomplete`), ""},
		{"unknown field rejected", covWithJSONField(t, valid, "rogue_field", true), "unknown field"},
		{"trailing JSON", append(append([]byte{}, valid...), '{', '}'), "trailing"},
		{"missing request id", covBuildHostUpdatePayloadJSON(t, "", "a1", HostUpdateOperationInstall, HostAPTOperationVersion, hash, 30), "request id is required"},
		{"empty action id", covBuildHostUpdatePayloadJSON(t, "r1", "", HostUpdateOperationInstall, HostAPTOperationVersion, hash, 30), "action id is required"},
		{"unsupported operation", covBuildHostUpdatePayloadJSON(t, "r1", "a1", "bogus_op", HostAPTOperationVersion, hash, 30), "unsupported host update operation"},
		{"unsupported operation version", covBuildHostUpdatePayloadJSON(t, "r1", "a1", HostUpdateOperationInstall, 99, hash, 30), "unsupported host update operation version"},
		{"request digest mismatch", covWithJSONField(t, valid, "request_digest", "sha256:"+strings.Repeat("0", 64)), "digest mismatch"},
		{"invalid inventory hash", covBuildHostUpdatePayloadJSON(t, "r1", "a1", HostUpdateOperationInstall, HostAPTOperationVersion, "not-a-hash", 30), "expected inventory hash"},
		{"negative timeout", covBuildHostUpdatePayloadJSON(t, "r1", "a1", HostUpdateOperationInstall, HostAPTOperationVersion, hash, -1), "timeout"},
		{"timeout exceeds maximum", covBuildHostUpdatePayloadJSON(t, "r1", "a1", HostUpdateOperationInstall, HostAPTOperationVersion, hash, 9999), "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeHostUpdatePayload(tc.body)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tc.wantSub != "" && !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DecodeHostStorageCleanupPayload — apt_codec.go:54
// ---------------------------------------------------------------------------

func TestCoverageDecodeHostStorageCleanupPayloadRejectionArms(t *testing.T) {
	fp := "sha256:" + strings.Repeat("c", 64)
	valid := covBuildHostStorageCleanupPayloadJSON(t, "r1", "a1", HostStorageCleanupOperationPackageCache, HostAPTOperationVersion, fp, 30)

	t.Run("valid payload decodes", func(t *testing.T) {
		got, err := DecodeHostStorageCleanupPayload(valid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.RequestID != "r1" || got.ActionID != "a1" || got.Operation != HostStorageCleanupOperationPackageCache {
			t.Fatalf("decoded payload mismatch: %#v", got)
		}
	})

	for _, tc := range []struct {
		name    string
		body    []byte
		wantSub string
	}{
		{"empty payload", []byte(""), "empty"},
		{"whitespace only", []byte("\t\n "), "empty"},
		{"malformed JSON", []byte(`{"request_id": invalid`), ""},
		{"unknown field rejected", covWithJSONField(t, valid, "rogue_field", 42), "unknown field"},
		{"trailing JSON", append(append([]byte{}, valid...), '{', '}'), "trailing"},
		{"missing request id", covBuildHostStorageCleanupPayloadJSON(t, "", "a1", HostStorageCleanupOperationPackageCache, HostAPTOperationVersion, fp, 30), "invalid request id"},
		{"empty action id", covBuildHostStorageCleanupPayloadJSON(t, "r1", "", HostStorageCleanupOperationPackageCache, HostAPTOperationVersion, fp, 30), "invalid action id"},
		{"unsupported operation", covBuildHostStorageCleanupPayloadJSON(t, "r1", "a1", "bogus_cleanup", HostAPTOperationVersion, fp, 30), "unsupported host storage cleanup operation"},
		{"unsupported operation version", covBuildHostStorageCleanupPayloadJSON(t, "r1", "a1", HostStorageCleanupOperationPackageCache, 77, fp, 30), "unsupported host storage cleanup operation version"},
		{"request digest mismatch", covWithJSONField(t, valid, "request_digest", "sha256:"+strings.Repeat("9", 64)), "digest mismatch"},
		{"invalid fingerprint", covBuildHostStorageCleanupPayloadJSON(t, "r1", "a1", HostStorageCleanupOperationPackageCache, HostAPTOperationVersion, "garbage-fp", 30), "expected cleanup fingerprint"},
		{"negative timeout", covBuildHostStorageCleanupPayloadJSON(t, "r1", "a1", HostStorageCleanupOperationPackageCache, HostAPTOperationVersion, fp, -1), "timeout"},
		{"timeout exceeds maximum", covBuildHostStorageCleanupPayloadJSON(t, "r1", "a1", HostStorageCleanupOperationPackageCache, HostAPTOperationVersion, fp, 9999), "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeHostStorageCleanupPayload(tc.body)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tc.wantSub != "" && !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateOperationQueryResultForIdentity — apt_codec.go:98
// ---------------------------------------------------------------------------

func TestCoverageValidateOperationQueryResultForIdentity(t *testing.T) {
	now := time.Now().UTC()
	updateIdentity := covHostUpdateIdentity("agent-1")
	cleanupIdentity := covHostCleanupIdentity("agent-1")

	// A mismatched identity for the identity-mismatch arm.
	otherIdentity := updateIdentity
	otherIdentity.ActionID = "different-action"

	// A bare identity with an unsupported operation kind (still normalisable).
	bogusIdentity := operationreceipt.Identity{
		AttemptID:        "cov.dispatch.1",
		ActionID:         "cov-action",
		OperationKind:    "bogus_operation",
		OperationVersion: 1,
		RequestDigest:    "sha256:" + strings.Repeat("a", 64),
		AgentID:          "agent-1",
	}

	// Valid not-found result (success path: nil record, no error).
	notFoundOK := operationreceipt.QueryResult{Version: operationreceipt.ProtocolVersion, Status: operationreceipt.QueryNotFound}

	// Not-found with a stray record.
	notFoundWithRecord := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryNotFound,
		Record:  covAcceptedRecord(updateIdentity, now),
	}

	// Unsupported version.
	badVersion := operationreceipt.QueryResult{Version: 99, Status: operationreceipt.QueryNotFound}

	// Found-terminal but nil record.
	nilRecord := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundTerminal,
	}

	// Identity mismatch (record identity differs).
	mismatchResult := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundInterrupted,
		Record:  covAcceptedRecord(otherIdentity, now),
	}

	// ValidateRecord failure (zero AcceptedAt).
	badRecord := &operationreceipt.Record{
		Identity:   updateIdentity,
		State:      operationreceipt.StateAccepted,
		AcceptedAt: time.Time{},
	}
	validateRecordFail := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundInterrupted,
		Record:  badRecord,
	}

	// Unknown status (neither terminal nor interrupted).
	unknownStatus := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  "bogus_status",
		Record:  covAcceptedRecord(updateIdentity, now),
	}

	// Interrupted status with a terminal-state record (state mismatch).
	interruptedTerminal := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundInterrupted,
		Record:  covTerminalRecord(updateIdentity, "any", 1, json.RawMessage(`{}`), now),
	}

	// Terminal status with a non-terminal-state record.
	terminalNonTerminalRecord := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundTerminal,
		Record:  covAcceptedRecord(updateIdentity, now),
	}

	// Valid interrupted result (success path).
	validInterrupted := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundInterrupted,
		Record:  covAcceptedRecord(updateIdentity, now),
	}

	// Terminal but receivedAt is zero.
	zeroReceivedAt := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundTerminal,
		Record:  covTerminalRecord(updateIdentity, HostUpdateReceiptKind, HostAPTReceiptVersion, json.RawMessage(`{}`), now),
	}

	// Host update envelope mismatch (wrong ResultKind).
	updateEnvelopeMismatch := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundTerminal,
		Record:  covTerminalRecord(updateIdentity, "wrong.kind", HostAPTReceiptVersion, json.RawMessage(`{}`), now),
	}

	// Host update decode error (correct envelope, malformed payload).
	updateDecodeError := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundTerminal,
		Record:  covTerminalRecord(updateIdentity, HostUpdateReceiptKind, HostAPTReceiptVersion, json.RawMessage(`{"rogue":true}`), now),
	}

	// Host storage cleanup envelope mismatch.
	cleanupEnvelopeMismatch := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundTerminal,
		Record:  covTerminalRecord(cleanupIdentity, "wrong.kind", HostAPTReceiptVersion, json.RawMessage(`{}`), now),
	}

	// Host storage cleanup decode error.
	cleanupDecodeError := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundTerminal,
		Record:  covTerminalRecord(cleanupIdentity, HostStorageCleanupReceiptKind, HostAPTReceiptVersion, json.RawMessage(`{"rogue":true}`), now),
	}

	// Unsupported operation kind (default switch arm).
	unsupportedKind := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundTerminal,
		Record:  covTerminalRecord(bogusIdentity, "any.kind", 1, json.RawMessage(`{}`), now),
	}

	for _, tc := range []struct {
		name       string
		result     operationreceipt.QueryResult
		identity   operationreceipt.Identity
		receivedAt time.Time
		wantSub    string
		wantOK     bool
	}{
		{"valid not-found returns nil", notFoundOK, updateIdentity, now, "", true},
		{"valid interrupted returns nil", validInterrupted, updateIdentity, now, "", true},
		{"unsupported version", badVersion, updateIdentity, now, "unsupported operation query result version", false},
		{"not-found with record", notFoundWithRecord, updateIdentity, now, "not-found operation query result contains a record", false},
		{"nil record identity mismatch", nilRecord, updateIdentity, now, "identity mismatch", false},
		{"record identity mismatch", mismatchResult, updateIdentity, now, "identity mismatch", false},
		{"validate record failure", validateRecordFail, updateIdentity, now, "invalid operation receipt bounds", false},
		{"unknown status mismatch", unknownStatus, updateIdentity, now, "status and record state mismatch", false},
		{"interrupted with terminal state", interruptedTerminal, updateIdentity, now, "status and record state mismatch", false},
		{"terminal with non-terminal record", terminalNonTerminalRecord, updateIdentity, now, "terminal operation query result requires terminal record", false},
		{"zero receivedAt rejected", zeroReceivedAt, updateIdentity, time.Time{}, "invalid or implausibly future", false},
		{"host update envelope mismatch", updateEnvelopeMismatch, updateIdentity, now, "host update query result envelope mismatch", false},
		{"host update decode error", updateDecodeError, updateIdentity, now, "", false},
		{"host cleanup envelope mismatch", cleanupEnvelopeMismatch, cleanupIdentity, now, "host cleanup query result envelope mismatch", false},
		{"host cleanup decode error", cleanupDecodeError, cleanupIdentity, now, "", false},
		{"unsupported operation kind", unsupportedKind, bogusIdentity, now, "unsupported operation query kind", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateOperationQueryResultForIdentity(tc.result, tc.identity, tc.receivedAt)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("expected nil error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tc.wantSub != "" && !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BindHostUpdatePayload — apt_codec.go:193
// ---------------------------------------------------------------------------

func TestCoverageBindHostUpdatePayload(t *testing.T) {
	t.Run("nil payload rejected", func(t *testing.T) {
		if err := BindHostUpdatePayload(nil); err == nil || !strings.Contains(err.Error(), "host update payload is required") {
			t.Fatalf("nil payload error=%v", err)
		}
	})

	t.Run("valid payload binds version and digest", func(t *testing.T) {
		hash := "sha256:" + strings.Repeat("a", 64)
		req := &HostUpdatePayload{
			RequestID:             "bind-1",
			ActionID:              "act-1",
			Operation:             HostUpdateOperationInstall,
			ExpectedInventoryHash: hash,
		}
		if err := BindHostUpdatePayload(req); err != nil {
			t.Fatalf("bind error: %v", err)
		}
		if req.OperationVersion != HostAPTOperationVersion {
			t.Fatalf("operation version = %d, want %d", req.OperationVersion, HostAPTOperationVersion)
		}
		wantDigest := covHostUpdateDigest("act-1", HostUpdateOperationInstall, HostAPTOperationVersion, hash)
		if req.RequestDigest != wantDigest {
			t.Fatalf("request digest = %q, want %q", req.RequestDigest, wantDigest)
		}
	})
}

// ---------------------------------------------------------------------------
// BindHostStorageCleanupPayload — apt_codec.go:215
// ---------------------------------------------------------------------------

func TestCoverageBindHostStorageCleanupPayload(t *testing.T) {
	t.Run("nil payload rejected", func(t *testing.T) {
		if err := BindHostStorageCleanupPayload(nil); err == nil || !strings.Contains(err.Error(), "host storage cleanup payload is required") {
			t.Fatalf("nil payload error=%v", err)
		}
	})

	t.Run("valid payload binds version and digest", func(t *testing.T) {
		fp := "sha256:" + strings.Repeat("c", 64)
		req := &HostStorageCleanupPayload{
			RequestID:           "bind-2",
			ActionID:            "act-2",
			Operation:           HostStorageCleanupOperationPackageCache,
			ExpectedFingerprint: fp,
		}
		if err := BindHostStorageCleanupPayload(req); err != nil {
			t.Fatalf("bind error: %v", err)
		}
		if req.OperationVersion != HostAPTOperationVersion {
			t.Fatalf("operation version = %d, want %d", req.OperationVersion, HostAPTOperationVersion)
		}
		wantDigest := covHostStorageCleanupDigest("act-2", HostStorageCleanupOperationPackageCache, HostAPTOperationVersion, fp)
		if req.RequestDigest != wantDigest {
			t.Fatalf("request digest = %q, want %q", req.RequestDigest, wantDigest)
		}
	})
}

// ---------------------------------------------------------------------------
// ValidateHostUpdateResultPayload — apt_codec.go:245
// ---------------------------------------------------------------------------

func TestCoverageValidateHostUpdateResultPayload(t *testing.T) {
	now := time.Now().UTC()
	hashA := "sha256:" + strings.Repeat("a", 64)
	hashB := "sha256:" + strings.Repeat("b", 64)

	// inconclusiveBase passes the internal validator and reaches every public
	// arm when specific fields are tweaked.
	inconclusiveBase := HostUpdateResultPayload{
		RequestID:      "r1",
		ActionID:       "a1",
		ExecutionPhase: HostUpdatePhaseVerify,
		Verification:   HostUpdateVerificationInconclusive,
	}

	// A fully valid verified+complete result for the success arm.
	validVerified := HostUpdateResultPayload{
		RequestID:             "r1",
		ActionID:              "a1",
		Success:               true,
		ExecutionPhase:        HostUpdatePhaseComplete,
		MutationStarted:       true,
		HealthChecked:         true,
		PackageManagerHealthy: true,
		Verification:          HostUpdateVerificationVerified,
		Before:                HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: hashA, PendingCount: 1, CheckedAt: now.Add(-2 * time.Minute)},
		After:                 HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: hashB, PendingCount: 0, CheckedAt: now.Add(-time.Minute)},
	}

	for _, tc := range []struct {
		name    string
		result  HostUpdateResultPayload
		wantSub string
		wantOK  bool
	}{
		{
			name:   "valid verified complete accepted",
			result: validVerified,
			wantOK: true,
		},
		{
			name:    "empty action id",
			result:  func() HostUpdateResultPayload { r := inconclusiveBase; r.ActionID = ""; return r }(),
			wantSub: "invalid action id",
		},
		{
			name: "unsupported execution phase",
			result: func() HostUpdateResultPayload {
				r := inconclusiveBase
				r.ExecutionPhase = "bogus_phase"
				return r
			}(),
			wantSub: "unsupported host update execution phase",
		},
		{
			name: "success in wrong phase",
			result: func() HostUpdateResultPayload {
				r := inconclusiveBase
				r.Success = true
				r.ExecutionPhase = HostUpdatePhasePreflight
				return r
			}(),
			wantSub: "successful host update mutation must be in verify or complete phase",
		},
		{
			name: "evidence timestamps invalid chronology",
			result: func() HostUpdateResultPayload {
				r := inconclusiveBase
				r.Verification = HostUpdateVerificationFailed
				r.Before = HostPackageUpdateSnapshot{CheckedAt: now}
				r.After = HostPackageUpdateSnapshot{CheckedAt: time.Time{}}
				return r
			}(),
			wantSub: "evidence-bearing host update observation timestamps are invalid",
		},
		{
			name: "mutation state conflicts with phase",
			result: func() HostUpdateResultPayload {
				r := inconclusiveBase
				r.MutationStarted = true
				r.ExecutionPhase = HostUpdatePhasePreflight
				return r
			}(),
			wantSub: "host update mutation state conflicts with execution phase",
		},
		{
			name: "recovery required without mutation",
			result: func() HostUpdateResultPayload {
				r := inconclusiveBase
				r.RecoveryRequired = true
				return r
			}(),
			wantSub: "host update recovery requirement conflicts with mutation state",
		},
		{
			name: "healthy package manager without health check",
			result: func() HostUpdateResultPayload {
				r := inconclusiveBase
				r.PackageManagerHealthy = true
				return r
			}(),
			wantSub: "healthy package manager claim requires a completed health check",
		},
		{
			name: "unhealthy package manager after mutation without recovery",
			result: func() HostUpdateResultPayload {
				r := inconclusiveBase
				r.HealthChecked = true
				r.MutationStarted = true
				return r
			}(),
			wantSub: "unhealthy package manager after mutation requires recovery",
		},
		{
			name: "partial install without recovery",
			result: func() HostUpdateResultPayload {
				r := inconclusiveBase
				r.ExecutionPhase = HostUpdatePhaseInstall
				r.MutationStarted = true
				return r
			}(),
			wantSub: "partial host update install requires recovery",
		},
		{
			name: "successful completion cannot require recovery",
			result: func() HostUpdateResultPayload {
				r := inconclusiveBase
				r.Success = true
				r.ExecutionPhase = HostUpdatePhaseComplete
				r.MutationStarted = true
				r.RecoveryRequired = true
				return r
			}(),
			wantSub: "successful host update completion cannot require recovery",
		},
		{
			name: "verified but not complete",
			result: func() HostUpdateResultPayload {
				r := validVerified
				r.ExecutionPhase = HostUpdatePhaseVerify
				return r
			}(),
			wantSub: "verified host update must be complete",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateHostUpdateResultPayload(&tc.result)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("expected nil error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateHostStorageCleanupResultPayload — apt_codec.go:289
// ---------------------------------------------------------------------------

func TestCoverageValidateHostStorageCleanupResultPayload(t *testing.T) {
	now := time.Now().UTC()
	fpA := "sha256:" + strings.Repeat("a", 64)
	fpB := "sha256:" + strings.Repeat("b", 64)

	inconclusiveBase := HostStorageCleanupResultPayload{
		RequestID:      "r1",
		ActionID:       "a1",
		ExecutionPhase: HostStorageCleanupPhaseVerify,
		Verification:   HostStorageCleanupVerificationInconclusive,
	}

	validVerified := HostStorageCleanupResultPayload{
		RequestID:       "r1",
		ActionID:        "a1",
		Success:         true,
		ExecutionPhase:  HostStorageCleanupPhaseComplete,
		MutationStarted: true,
		Verification:    HostStorageCleanupVerificationVerified,
		Before:          HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: fpA, ReclaimableBytes: 100, CheckedAt: now.Add(-2 * time.Minute)},
		After:           HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: fpB, ReclaimableBytes: 10, CheckedAt: now.Add(-time.Minute)},
		ReclaimedBytes:  90,
	}

	for _, tc := range []struct {
		name    string
		result  HostStorageCleanupResultPayload
		wantSub string
		wantOK  bool
	}{
		{
			name:   "valid verified complete accepted",
			result: validVerified,
			wantOK: true,
		},
		{
			name:    "empty action id",
			result:  func() HostStorageCleanupResultPayload { r := inconclusiveBase; r.ActionID = ""; return r }(),
			wantSub: "invalid action id",
		},
		{
			name: "unsupported execution phase",
			result: func() HostStorageCleanupResultPayload {
				r := inconclusiveBase
				r.ExecutionPhase = "bogus_phase"
				return r
			}(),
			wantSub: "unsupported host storage cleanup execution phase",
		},
		{
			name: "success in wrong phase",
			result: func() HostStorageCleanupResultPayload {
				r := inconclusiveBase
				r.Success = true
				r.ExecutionPhase = HostStorageCleanupPhasePreflight
				return r
			}(),
			wantSub: "successful host storage cleanup mutation must be in verify or complete phase",
		},
		{
			name: "verified but not complete",
			result: func() HostStorageCleanupResultPayload {
				r := validVerified
				r.ExecutionPhase = HostStorageCleanupPhaseVerify
				return r
			}(),
			wantSub: "verified host storage cleanup must be complete",
		},
		{
			name: "evidence timestamps invalid chronology",
			result: func() HostStorageCleanupResultPayload {
				r := inconclusiveBase
				r.Verification = HostStorageCleanupVerificationFailed
				r.Before = HostStorageCleanupSnapshot{CheckedAt: now}
				r.After = HostStorageCleanupSnapshot{CheckedAt: time.Time{}}
				return r
			}(),
			wantSub: "evidence-bearing host storage cleanup observation timestamps are invalid",
		},
		{
			name: "mutation state conflicts with phase",
			result: func() HostStorageCleanupResultPayload {
				r := inconclusiveBase
				r.MutationStarted = true
				r.ExecutionPhase = HostStorageCleanupPhasePreflight
				return r
			}(),
			wantSub: "host storage cleanup mutation state conflicts with execution phase",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateHostStorageCleanupResultPayload(&tc.result)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("expected nil error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}
