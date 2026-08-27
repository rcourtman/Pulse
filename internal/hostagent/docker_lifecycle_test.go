package hostagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	"github.com/rs/zerolog"
)

const dockerLifecycleTestContainerID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

type stubDockerLifecycleOperator struct {
	snapshots []agentexec.DockerContainerLifecycleSnapshot
	mutations []string
}

func (s *stubDockerLifecycleOperator) InspectDockerContainerLifecycle(context.Context, string, string) (agentexec.DockerContainerLifecycleSnapshot, error) {
	if len(s.snapshots) == 0 {
		return agentexec.DockerContainerLifecycleSnapshot{}, fmt.Errorf("unexpected inspect")
	}
	snapshot := s.snapshots[0]
	s.snapshots = s.snapshots[1:]
	return snapshot, nil
}

func (s *stubDockerLifecycleOperator) MutateDockerContainerLifecycle(_ context.Context, _, operation, _ string) error {
	s.mutations = append(s.mutations, operation)
	return nil
}

func TestDockerLifecycleManagerUsesConnectedModuleWithoutExternalCLI(t *testing.T) {
	before := time.Now().UTC().Add(-time.Minute).Truncate(time.Nanosecond)
	after := before.Add(time.Minute)
	operator := &stubDockerLifecycleOperator{snapshots: []agentexec.DockerContainerLifecycleSnapshot{
		{ContainerID: dockerLifecycleTestContainerID, State: "running", Running: true, Health: agentexec.DockerContainerHealthUnhealthy, StartedAt: before, ObservedAt: before},
		{ContainerID: dockerLifecycleTestContainerID, State: "running", Running: true, Health: agentexec.DockerContainerHealthHealthy, StartedAt: after, ObservedAt: after},
	}}
	manager := newLocalDockerLifecycleManager(operator)
	manager.run = func(context.Context, string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("external runtime CLI must not be called")
	}
	result := manager.Apply(context.Background(), dockerLifecycleTestRequest(t, before))
	if !result.MutationStarted || !result.MutationCompleted || !result.ReadbackRan {
		t.Fatalf("API-backed lifecycle result = %#v", result)
	}
	if len(operator.mutations) != 1 || operator.mutations[0] != "restart" {
		t.Fatalf("mutations = %v, want one restart", operator.mutations)
	}
}

func TestDockerLifecycleManagerWaitsForHealthRecovery(t *testing.T) {
	before := time.Now().UTC().Add(-time.Minute).Truncate(time.Nanosecond)
	after := before.Add(time.Minute)
	operator := &stubDockerLifecycleOperator{snapshots: []agentexec.DockerContainerLifecycleSnapshot{
		{ContainerID: dockerLifecycleTestContainerID, State: "running", Running: true, Health: agentexec.DockerContainerHealthUnhealthy, StartedAt: before, ObservedAt: before},
		{ContainerID: dockerLifecycleTestContainerID, State: "running", Running: true, Health: agentexec.DockerContainerHealthStarting, StartedAt: after, ObservedAt: after},
		{ContainerID: dockerLifecycleTestContainerID, State: "running", Running: true, Health: agentexec.DockerContainerHealthHealthy, StartedAt: after, ObservedAt: after.Add(time.Millisecond)},
	}}
	manager := newLocalDockerLifecycleManager(operator)
	manager.healthPollInterval = time.Millisecond
	result := manager.Apply(context.Background(), dockerLifecycleTestRequest(t, before))
	if result.ExecutionPhase != agentexec.DockerContainerPhaseComplete || result.After.Health != agentexec.DockerContainerHealthHealthy {
		t.Fatalf("health recovery result = %#v", result)
	}
	if len(operator.mutations) != 1 {
		t.Fatalf("mutations = %v, want one restart", operator.mutations)
	}
}

func TestDockerLifecycleManagerDoesNotVerifyPersistentUnhealthyState(t *testing.T) {
	before := time.Now().UTC().Add(-time.Minute).Truncate(time.Nanosecond)
	after := before.Add(time.Minute)
	operator := &stubDockerLifecycleOperator{snapshots: []agentexec.DockerContainerLifecycleSnapshot{
		{ContainerID: dockerLifecycleTestContainerID, State: "running", Running: true, Health: agentexec.DockerContainerHealthUnhealthy, StartedAt: before, ObservedAt: before},
		{ContainerID: dockerLifecycleTestContainerID, State: "running", Running: true, Health: agentexec.DockerContainerHealthUnhealthy, StartedAt: after, ObservedAt: after},
	}}
	manager := newLocalDockerLifecycleManager(operator)
	manager.healthPollInterval = time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	result := manager.Apply(ctx, dockerLifecycleTestRequest(t, before))
	if result.ExecutionPhase == agentexec.DockerContainerPhaseComplete || result.After.Health != agentexec.DockerContainerHealthUnhealthy {
		t.Fatalf("persistent unhealthy result = %#v", result)
	}
}

func TestDockerLifecycleManagerObservationIsReadOnly(t *testing.T) {
	now := time.Now().UTC()
	operator := &stubDockerLifecycleOperator{snapshots: []agentexec.DockerContainerLifecycleSnapshot{{
		ContainerID: dockerLifecycleTestContainerID, State: "running", Running: true, Health: agentexec.DockerContainerHealthNone, ObservedAt: now,
	}}}
	manager := newLocalDockerLifecycleManager(operator)
	manager.run = func(context.Context, string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("external runtime CLI must not be called")
	}
	snapshot, err := manager.Observe(context.Background(), "docker", dockerLifecycleTestContainerID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ContainerID != dockerLifecycleTestContainerID || !snapshot.Running || len(operator.mutations) != 0 {
		t.Fatalf("snapshot=%#v mutations=%v", snapshot, operator.mutations)
	}
}

func TestDockerLifecycleManagerRestartPerformsOneMutationAndBoundedReadback(t *testing.T) {
	t.Setenv("DOCKER_CONTEXT", "")
	before := time.Now().UTC().Add(-time.Minute).Truncate(time.Nanosecond)
	after := before.Add(time.Minute)
	var calls [][]string
	manager := &localDockerLifecycleManager{now: func() time.Time { return after }, run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		switch len(calls) {
		case 1:
			return []byte(fmt.Sprintf(`{"Status":"running","Running":true,"StartedAt":%q}`, before.Format(time.RFC3339Nano))), nil
		case 2:
			return []byte("0"), nil
		case 3:
			return []byte(dockerLifecycleTestContainerID), nil
		case 4:
			return []byte(fmt.Sprintf(`{"Status":"running","Running":true,"StartedAt":%q}`, after.Format(time.RFC3339Nano))), nil
		case 5:
			return []byte("0"), nil
		default:
			return nil, fmt.Errorf("unexpected command")
		}
	}}
	req := dockerLifecycleTestRequest(t, before)
	result := manager.Apply(context.Background(), req)
	if !result.MutationStarted || !result.MutationCompleted || !result.ReadbackRan {
		t.Fatalf("result facts = %#v", result)
	}
	mutations := 0
	for _, call := range calls {
		if len(call) > 1 && call[1] == "restart" {
			mutations++
		}
	}
	if mutations != 1 {
		t.Fatalf("restart mutations = %d, calls=%#v", mutations, calls)
	}
}

func TestDockerLifecycleManagerStaleBeforeStateIsTripleZero(t *testing.T) {
	t.Setenv("DOCKER_CONTEXT", "")
	before := time.Now().UTC().Add(-time.Minute)
	mutations := 0
	manager := &localDockerLifecycleManager{now: time.Now, run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "restart" {
			mutations++
		}
		if len(args) > 2 && strings.Contains(args[2], ".State") {
			return []byte(fmt.Sprintf(`{"Status":"exited","Running":false,"StartedAt":%q}`, before.Format(time.RFC3339Nano))), nil
		}
		return []byte("0"), nil
	}}
	result := manager.Apply(context.Background(), dockerLifecycleTestRequest(t, before))
	if result.MutationStarted || result.MutationCompleted || result.ReadbackRan || mutations != 0 {
		t.Fatalf("stale preflight was not triple-zero: result=%#v mutations=%d", result, mutations)
	}
}

func TestDockerLifecyclePreflightRefusesStateDriftWithoutMutation(t *testing.T) {
	t.Setenv("DOCKER_CONTEXT", "")
	before := time.Now().UTC().Add(-time.Minute)
	mutations := 0
	manager := &localDockerLifecycleManager{now: time.Now, run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "restart" {
			mutations++
		}
		if len(args) > 2 && strings.Contains(args[2], ".State") {
			return []byte(fmt.Sprintf(`{"Status":"exited","Running":false,"StartedAt":%q}`, before.Format(time.RFC3339Nano))), nil
		}
		return []byte("0"), nil
	}}
	feasible, reason := manager.Preflight(context.Background(), dockerLifecycleTestRequest(t, before))
	if feasible || reason != agentexec.ActionRefusalTargetStateChanged || mutations != 0 {
		t.Fatalf("feasible=%t reason=%q mutations=%d", feasible, reason, mutations)
	}
}

func TestDockerLifecycleManagerFailedInspectIsTripleZero(t *testing.T) {
	t.Setenv("DOCKER_CONTEXT", "")
	manager := &localDockerLifecycleManager{now: time.Now, run: func(context.Context, string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("inspect unavailable")
	}}
	result := manager.Apply(context.Background(), dockerLifecycleTestRequest(t, time.Now().UTC().Add(-time.Minute)))
	if result.MutationStarted || result.MutationCompleted || result.ReadbackRan {
		t.Fatalf("failed inspect was not triple-zero: %#v", result)
	}
}

func dockerLifecycleTestRequest(t *testing.T, startedAt time.Time) agentexec.DockerContainerLifecyclePayload {
	t.Helper()
	req := agentexec.DockerContainerLifecyclePayload{RequestID: "action.dispatch.1", ActionID: "action", Operation: agentexec.DockerContainerOperationRestart, Runtime: "docker", ContainerID: dockerLifecycleTestContainerID, ExpectedState: "running", ExpectedStartedAt: startedAt, Timeout: 10}
	if err := agentexec.BindDockerContainerLifecyclePayload(&req); err != nil {
		t.Fatal(err)
	}
	return req
}

type stubDockerUpdater struct {
	outcome agentexec.DockerContainerUpdateOutcome
	err     error
	calls   *int
}

func (stubDockerUpdater) TypedContainerUpdatePreflight(context.Context, string, string, string) error {
	return nil
}

func (s stubDockerUpdater) TypedContainerUpdate(_ context.Context, _, _, _ string, _ func(string)) (agentexec.DockerContainerUpdateOutcome, error) {
	if s.calls != nil {
		*s.calls++
	}
	return s.outcome, s.err
}

func boundUpdatePayload(t *testing.T) agentexec.DockerContainerUpdatePayload {
	t.Helper()
	payload := agentexec.DockerContainerUpdatePayload{
		RequestID:           "attempt-1",
		ActionID:            "action-1",
		Runtime:             "docker",
		ContainerID:         strings.Repeat("a", 12),
		ExpectedImageDigest: "sha256:" + strings.Repeat("9", 64),
		Timeout:             60,
	}
	if err := agentexec.BindDockerContainerUpdatePayload(&payload); err != nil {
		t.Fatal(err)
	}
	if err := agentexec.ValidateDockerContainerUpdatePayload(&payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func newUpdateTestClient(updater DockerContainerUpdater) *CommandClient {
	return &CommandClient{
		agentID:       "agent-1",
		dockerUpdater: updater,
		dockerLifecycle: &localDockerLifecycleManager{
			run: func(context.Context, string, ...string) ([]byte, error) {
				return []byte(`{"Status":"running","Running":true,"StartedAt":"2026-07-14T12:00:00Z","RestartCount":0}`), nil
			},
			now: func() time.Time { return time.Date(2026, 7, 14, 12, 0, 5, 0, time.UTC) },
		},
	}
}

func TestRunDockerContainerUpdateSuccessMapsCompleteResult(t *testing.T) {
	payload := boundUpdatePayload(t)
	newID := strings.Repeat("b", 12)
	client := newUpdateTestClient(stubDockerUpdater{outcome: agentexec.DockerContainerUpdateOutcome{
		Success: true, ContainerName: "app", OldContainerID: payload.ContainerID, NewContainerID: newID,
		OldImageDigest: "sha256:" + strings.Repeat("1", 64), NewImageDigest: "sha256:" + strings.Repeat("2", 64),
		BackupCreated: true, BackupContainer: "app_pulse_backup_20260714_120000",
	}})

	result := client.runDockerContainerUpdate(context.Background(), payload)
	if err := agentexec.ValidateDockerContainerUpdateResultForRequest(payload, result); err != nil {
		t.Fatalf("success result failed the wire contract: %v (%+v)", err, result)
	}
	if result.ExecutionPhase != agentexec.DockerContainerPhaseComplete || result.NewContainerID != newID || !result.ReadbackRan {
		t.Fatalf("unexpected success mapping: %+v", result)
	}
}

func TestRunDockerContainerUpdateFailureCarriesCompensationTruth(t *testing.T) {
	payload := boundUpdatePayload(t)
	client := newUpdateTestClient(stubDockerUpdater{outcome: agentexec.DockerContainerUpdateOutcome{
		Success: false, ContainerName: "app", Error: "Failed to create new container: boom",
		BackupCreated: true, BackupContainer: "app_pulse_backup_20260714_120000",
		RollbackAttempted: true, RolledBack: true,
	}})

	result := client.runDockerContainerUpdate(context.Background(), payload)
	if err := agentexec.ValidateDockerContainerUpdateResultForRequest(payload, result); err != nil {
		t.Fatalf("failure result failed the wire contract: %v (%+v)", err, result)
	}
	if result.ExecutionPhase == agentexec.DockerContainerPhaseComplete || !result.MutationStarted || !result.RolledBack || result.Error == "" {
		t.Fatalf("unexpected failure mapping: %+v", result)
	}
}

func TestRunDockerContainerUpdateWithoutModuleRefuses(t *testing.T) {
	payload := boundUpdatePayload(t)
	client := newUpdateTestClient(nil)

	result := client.runDockerContainerUpdate(context.Background(), payload)
	if err := agentexec.ValidateDockerContainerUpdateResultForRequest(payload, result); err != nil {
		t.Fatalf("refusal result failed the wire contract: %v (%+v)", err, result)
	}
	if result.MutationStarted || result.Error == "" || result.ExecutionPhase != agentexec.DockerContainerPhasePreflight {
		t.Fatalf("unexpected refusal mapping: %+v", result)
	}
}

func TestHostOperationReceiptConfigAcceptsDockerUpdateReceipts(t *testing.T) {
	cfg := hostOperationReceiptConfig()
	validators, ok := cfg.Validators[agentexec.DockerContainerUpdateReceiptKind]
	if !ok {
		t.Fatal("docker update receipt kind is not registered with the receipt store")
	}
	validator, ok := validators[agentexec.DockerContainerUpdateReceiptVersion]
	if !ok {
		t.Fatal("docker update receipt version is not registered with the receipt store")
	}

	payload := boundUpdatePayload(t)
	newID := strings.Repeat("b", 12)
	client := newUpdateTestClient(stubDockerUpdater{outcome: agentexec.DockerContainerUpdateOutcome{
		Success: true, ContainerName: "app", OldContainerID: payload.ContainerID, NewContainerID: newID,
		BackupCreated: true, BackupContainer: "app_pulse_backup_20260714_120000",
	}})
	result := client.runDockerContainerUpdate(context.Background(), payload)
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := validator(agentexec.DockerContainerUpdateOperationIdentity("agent-1", payload), encoded); err != nil {
		t.Fatalf("terminal receipt for a successful update was rejected: %v", err)
	}
}

func TestDockerContainerUpdateTerminalReceiptReplaysAfterAgentReconnect(t *testing.T) {
	payload := boundUpdatePayload(t)
	newID := strings.Repeat("b", 12)
	updateCalls := 0
	client := newUpdateTestClient(stubDockerUpdater{
		outcome: agentexec.DockerContainerUpdateOutcome{
			Success: true, ContainerName: "app", OldContainerID: payload.ContainerID, NewContainerID: newID,
			OldImageDigest: "sha256:" + strings.Repeat("1", 64), NewImageDigest: "sha256:" + strings.Repeat("2", 64),
			BackupCreated: true, BackupContainer: "app_pulse_backup_20260714_120000",
		},
		calls: &updateCalls,
	})
	client.logger = zerolog.Nop()
	receipts, err := operationreceipt.Open(filepath.Join(t.TempDir(), "receipts.db"), hostOperationReceiptConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer receipts.Close()
	client.operationReceipts = receipts

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	serverConnections := make(chan *websocket.Conn)
	releaseConnections := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, upgradeErr := upgrader.Upgrade(w, r, nil)
		if upgradeErr != nil {
			t.Errorf("upgrade: %v", upgradeErr)
			return
		}
		serverConnections <- conn
		<-releaseConnections
		_ = conn.Close()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	for attempt := 0; attempt < 2; attempt++ {
		remote, _, dialErr := websocket.DefaultDialer.Dial(wsURL, nil)
		if dialErr != nil {
			t.Fatal(dialErr)
		}
		serverConn := <-serverConnections

		client.handleDockerContainerUpdate(context.Background(), serverConn, payload)
		var message wsMessage
		if readErr := remote.ReadJSON(&message); readErr != nil {
			t.Fatalf("attempt %d read result: %v", attempt+1, readErr)
		}
		if message.Type != msgTypeDockerContainerUpdateResult {
			t.Fatalf("attempt %d message type = %q", attempt+1, message.Type)
		}
		var result agentexec.DockerContainerUpdateResultPayload
		if decodeErr := json.Unmarshal(message.Payload, &result); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if result.NewContainerID != newID || result.RequestID != payload.RequestID || !result.MutationCompleted {
			t.Fatalf("attempt %d result = %+v", attempt+1, result)
		}

		_ = remote.Close()
		releaseConnections <- struct{}{}
	}
	if updateCalls != 1 {
		t.Fatalf("docker update executed %d times across reconnect, want 1", updateCalls)
	}
}
