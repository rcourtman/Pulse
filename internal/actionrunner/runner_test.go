package actionrunner

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

type testPayload struct {
	Value string `json:"value"`
}

type testHandler struct {
	started chan struct{}
}

func (h testHandler) ValidatePayload(data json.RawMessage) error {
	var payload testPayload
	return decodeStrict(data, &payload)
}

func (h testHandler) Execute(ctx context.Context, _ Target, data json.RawMessage) (Result, error) {
	if h.started != nil {
		close(h.started)
		<-ctx.Done()
		return Result{}, ctx.Err()
	}
	return Result{Status: ResultSucceeded, Output: append(json.RawMessage(nil), data...)}, nil
}

func openTestRunner(t *testing.T, handler Handler) *Runner {
	t.Helper()
	runner, err := Open(Config{ReceiptPath: filepath.Join(t.TempDir(), "receipts.db"), Handlers: map[string]map[int]Handler{"host.update": {1: handler}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	return runner
}

func testSession() Session {
	return Session{OrganizationID: "org-1", HostID: "host-1", TokenID: "runner-token-1", Capabilities: map[string]bool{ActionCapability: true}}
}

func testRequest(t *testing.T) Request {
	t.Helper()
	request := Request{ProtocolVersion: ProtocolVersion, OrganizationID: "org-1", HostID: "host-1", AttemptID: "attempt-1", ActionID: "action-1", Operation: "host.update", OperationVersion: 1, Target: Target{Kind: "host", ID: "host-1"}, Deadline: time.Now().UTC().Add(time.Minute), Payload: json.RawMessage(`{"value":"bounded"}`)}
	digest, err := RequestDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	request.RequestDigest = digest
	return request
}

func TestExecutePersistsAndReplaysBoundTerminalReceipt(t *testing.T) {
	runner := openTestRunner(t, testHandler{})
	session := testSession()
	request := testRequest(t)
	record, err := runner.Execute(context.Background(), session, request)
	if err != nil {
		t.Fatal(err)
	}
	if record.State != operationreceipt.StateTerminal || record.Identity != ReceiptIdentity(request, session.TokenID) {
		t.Fatalf("unexpected receipt: %+v", record)
	}
	var result TerminalResult
	if err := json.Unmarshal(record.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != ResultSucceeded || result.Target != request.Target {
		t.Fatalf("unexpected result: %+v", result)
	}
	replayed, err := runner.Execute(context.Background(), session, request)
	if err != nil || replayed.TerminalAt != record.TerminalAt {
		t.Fatalf("replay = %+v, %v", replayed, err)
	}
	query, err := runner.Query(session, record.Identity)
	if err != nil || query.Status != operationreceipt.QueryFoundTerminal {
		t.Fatalf("query = %+v, %v", query, err)
	}
}

func TestExecuteRejectsWrongSessionAndUnknownPayloadFields(t *testing.T) {
	runner := openTestRunner(t, testHandler{})
	request := testRequest(t)
	wrong := testSession()
	wrong.HostID = "host-2"
	if _, err := runner.Execute(context.Background(), wrong, request); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("wrong session error = %v", err)
	}
	request.Payload = json.RawMessage(`{"value":"ok","command":"id"}`)
	request.RequestDigest, _ = RequestDigest(request)
	if _, err := runner.Execute(context.Background(), testSession(), request); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown payload field error = %v", err)
	}
}

func TestDecodeRequestRejectsUnknownVersionShellReadFileAndTrailingJSON(t *testing.T) {
	now := time.Now().UTC()
	base := testRequest(t)
	base.Deadline = now.Add(time.Minute)
	base.RequestDigest, _ = RequestDigest(base)
	encoded, _ := json.Marshal(base)
	var raw map[string]any
	_ = json.Unmarshal(encoded, &raw)
	raw["unexpected"] = true
	unknown, _ := json.Marshal(raw)
	if _, err := DecodeRequest(unknown, now); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown-field error = %v", err)
	}
	if _, err := DecodeRequest(append(encoded, []byte(` {}`)...), now); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing error = %v", err)
	}
	for _, operation := range []string{"shell", "exec", "execute_command", "read_file"} {
		request := base
		request.Operation = operation
		request.RequestDigest, _ = RequestDigest(request)
		data, _ := json.Marshal(request)
		if _, err := DecodeRequest(data, now); !errors.Is(err, ErrUnsupported) {
			t.Errorf("operation %q error = %v", operation, err)
		}
	}
	badVersion := base
	badVersion.OperationVersion = 2
	badVersion.RequestDigest, _ = RequestDigest(badVersion)
	data, _ := json.Marshal(badVersion)
	if _, err := DecodeRequest(data, now); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("version error = %v", err)
	}
}

func TestValidateRequestEnforcesTargetDeadlineAndOutputBounds(t *testing.T) {
	now := time.Now().UTC()
	request := testRequest(t)
	request.Target.ID = "another-host"
	request.RequestDigest, _ = RequestDigest(request)
	if err := ValidateRequest(&request, now); err == nil || !strings.Contains(err.Error(), "bound host") {
		t.Fatalf("target binding error = %v", err)
	}
	request = testRequest(t)
	request.Deadline = now.Add(MaxOperationDeadline + time.Second)
	request.RequestDigest, _ = RequestDigest(request)
	if err := ValidateRequest(&request, now); err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("deadline error = %v", err)
	}
	request = testRequest(t)
	terminal := TerminalResult{ProtocolVersion: ProtocolVersion, AttemptID: request.AttemptID, ActionID: request.ActionID, Operation: request.Operation, OperationVersion: request.OperationVersion, RequestDigest: request.RequestDigest, Target: request.Target, Status: ResultSucceeded, Output: json.RawMessage(`"` + strings.Repeat("x", MaxResultBytes) + `"`)}
	data, _ := json.Marshal(terminal)
	if err := ValidateTerminal(ReceiptIdentity(request, testSession().TokenID), data); err == nil || !strings.Contains(err.Error(), "oversized") {
		t.Fatalf("output bound error = %v", err)
	}
}

func TestCancelRequiresExactCredentialAndProducesDurableCanceledReceipt(t *testing.T) {
	started := make(chan struct{})
	runner := openTestRunner(t, testHandler{started: started})
	session := testSession()
	request := testRequest(t)
	type outcome struct {
		record operationreceipt.Record
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		record, err := runner.Execute(context.Background(), session, request)
		done <- outcome{record, err}
	}()
	<-started
	identity := ReceiptIdentity(request, session.TokenID)
	wrongIdentity := identity
	wrongIdentity.ActionID = "other-action"
	if err := runner.Cancel(session, wrongIdentity); !errors.Is(err, operationreceipt.ErrBindingConflict) {
		t.Fatalf("wrong cancellation binding error = %v", err)
	}
	if err := runner.Cancel(session, identity); err != nil {
		t.Fatal(err)
	}
	result := <-done
	if !errors.Is(result.err, context.Canceled) || result.record.State != operationreceipt.StateTerminal {
		t.Fatalf("canceled outcome = %+v, %v", result.record, result.err)
	}
	var terminal TerminalResult
	if err := json.Unmarshal(result.record.Result, &terminal); err != nil {
		t.Fatal(err)
	}
	if terminal.Status != ResultCanceled || terminal.ReasonCode != "canceled" {
		t.Fatalf("terminal = %+v", terminal)
	}
}

func TestRegistrationIsExplicitActionRunnerRole(t *testing.T) {
	registration, err := testSession().Registration()
	if err != nil {
		t.Fatal(err)
	}
	if registration.RuntimeRole != "action-runner" {
		t.Fatalf("runtime role = %q", registration.RuntimeRole)
	}
	collector := testSession()
	collector.Capabilities = map[string]bool{}
	if _, err := collector.Registration(); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("collector registration error = %v", err)
	}
}
