package agentexec

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func boundHostUpdatePreflight(t *testing.T) ActionPreflightPayload {
	t.Helper()
	typed := HostUpdatePayload{
		RequestID: "preflight-1", ActionID: "action-1", Operation: HostUpdateOperationInstall,
		ExpectedInventoryHash: "sha256:" + strings.Repeat("a", 64),
	}
	if err := BindHostUpdatePayload(&typed); err != nil {
		t.Fatal(err)
	}
	return ActionPreflightPayload{RequestID: typed.RequestID, ProtocolVersion: ActionPreflightProtocolVersion, HostUpdate: &typed}
}

func TestActionPreflightPayloadRequiresExactlyOneBoundOperation(t *testing.T) {
	payload := boundHostUpdatePreflight(t)
	if err := ValidateActionPreflightPayload(&payload); err != nil {
		t.Fatalf("valid payload: %v", err)
	}
	payload.StorageCleanup = &HostStorageCleanupPayload{}
	if err := ValidateActionPreflightPayload(&payload); err == nil {
		t.Fatal("payload with two operations was accepted")
	}
	payload = boundHostUpdatePreflight(t)
	payload.RequestID = "different-request"
	if err := ValidateActionPreflightPayload(&payload); err == nil {
		t.Fatal("mismatched outer and typed request ids were accepted")
	}
}

func TestActionPreflightStrictDecodeRejectsUnknownAuthority(t *testing.T) {
	payload := boundHostUpdatePreflight(t)
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded[:len(encoded)-1], []byte(`,"command":"apt-get upgrade"}`)...)
	if _, err := DecodeActionPreflightPayload(encoded); err == nil {
		t.Fatal("unknown command field was accepted")
	}
}

func TestActionPreflightResultMustMatchExactDigestAndChronology(t *testing.T) {
	req := boundHostUpdatePreflight(t)
	operation, version, digest := ActionPreflightBinding(req)
	now := time.Now().UTC()
	result := ActionPreflightResultPayload{
		RequestID: req.RequestID, ProtocolVersion: req.ProtocolVersion,
		Operation: operation, OperationVersion: version, RequestDigest: digest,
		Feasible: true, CheckedAt: now,
	}
	if err := ValidateActionPreflightResultForRequest(req, result, now); err != nil {
		t.Fatalf("valid result: %v", err)
	}
	result.RequestDigest = "sha256:" + strings.Repeat("b", 64)
	if err := ValidateActionPreflightResultForRequest(req, result, now); err == nil {
		t.Fatal("mismatched digest was accepted")
	}
}

func TestActionPreflightRefusalRequiresTypedReason(t *testing.T) {
	req := boundHostUpdatePreflight(t)
	operation, version, digest := ActionPreflightBinding(req)
	result := ActionPreflightResultPayload{
		RequestID: req.RequestID, ProtocolVersion: req.ProtocolVersion,
		Operation: operation, OperationVersion: version, RequestDigest: digest,
		CheckedAt: time.Now().UTC(),
	}
	if err := ValidateActionPreflightResultPayload(&result); err == nil {
		t.Fatal("unclassified refusal was accepted")
	}
	result.ReasonCode = ActionRefusalPackageManagerUnhealthy
	if err := ValidateActionPreflightResultPayload(&result); err != nil {
		t.Fatalf("typed refusal rejected: %v", err)
	}
}
