package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	"github.com/rs/zerolog"
)

func TestRealServerAndUnifiedAgentWebSocketQueriesFakeTypedOperationWithoutPackageManager(t *testing.T) {
	server := agentexec.NewServer(func(token, agent, host string) bool { return token == "token" })
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()
	logger := zerolog.Nop()
	client := NewCommandClient(Config{PulseURL: httpServer.URL, APIToken: "token", StateDir: t.TempDir(), Logger: &logger}, "agent-real", "host", "linux", "6")
	if client.packageUpdates != nil || client.storageCleanup != nil {
		t.Fatal("test must not install package-manager adapters")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- client.Run(ctx) }()
	deadline := time.Now().Add(3 * time.Second)
	for !server.IsAgentConnected("agent-real") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !server.IsAgentConnected("agent-real") {
		t.Fatal("real command client did not connect")
	}
	digest, _ := operationreceipt.DigestCanonicalJSON(map[string]string{"fake": "typed-read-only-proof"})
	identity := operationreceipt.Identity{AttemptID: "fake.dispatch.1", ActionID: "fake", OperationKind: "fake.typed", OperationVersion: 1, RequestDigest: digest, AgentID: "agent-real"}
	result, err := server.QueryAgentOperation(context.Background(), "agent-real", identity)
	if err != nil || result.Status != operationreceipt.QueryNotFound {
		t.Fatalf("query=%+v err=%v", result, err)
	}
	if _, fresh, err := client.operationReceipts.Admit(identity); err != nil || !fresh {
		t.Fatalf("admit fresh=%v err=%v", fresh, err)
	}
	if _, err := client.operationReceipts.MarkStarted(identity); err != nil {
		t.Fatal(err)
	}
	result, err = server.QueryAgentOperation(context.Background(), "agent-real", identity)
	if err != nil || result.Status != operationreceipt.QueryFoundInterrupted {
		t.Fatalf("interrupted query=%+v err=%v", result, err)
	}
	cancel()
	_ = client.Close()
	select {
	case err := <-done:
		if err != nil && !strings.Contains(err.Error(), "context canceled") {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("client did not stop")
	}
}

func TestRealServerAndUnifiedAgentWebSocketExecutesAPTThroughFakeTypedManagersAndReplaysWithoutMutation(t *testing.T) {
	server := agentexec.NewServer(func(token, agent, host string) bool { return token == "token" })
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()
	logger := zerolog.Nop()
	client := NewCommandClient(Config{PulseURL: httpServer.URL, APIToken: "token", StateDir: t.TempDir(), Logger: &logger}, "agent-apt", "host", "linux", "6")
	lease := newPackageManagerLease()
	updates := newPackageUpdateManager("linux", lease)
	cleanup := newStorageCleanupManager("linux", lease)

	const pendingSimulation = "Inst pulse-safe [1.0] (1.1 stable [amd64])\n"
	var mu sync.Mutex
	upgraded := false
	cleaned := false
	refreshCalls := 0
	upgradeCalls := 0
	cleanCalls := 0
	fakeRun := func(_ context.Context, _ []string, command string, args ...string) packageUpdateCommandResult {
		if command == "dpkg" && strings.Join(args, " ") == "--audit" {
			return packageUpdateCommandResult{}
		}
		if command != "apt-get" {
			return packageUpdateCommandResult{err: errors.New("unexpected fake command")}
		}
		mu.Lock()
		defer mu.Unlock()
		joined := strings.Join(args, " ")
		switch {
		case joined == "update":
			refreshCalls++
		case strings.Contains(joined, "-y --no-remove"):
			upgradeCalls++
			upgraded = true
		case joined == "clean":
			cleanCalls++
			cleaned = true
		case strings.Contains(joined, "-s -o Debug::NoLocking=1 upgrade"):
			if !upgraded {
				return packageUpdateCommandResult{stdout: pendingSimulation}
			}
		default:
			return packageUpdateCommandResult{err: errors.New("unexpected fake apt catalog call")}
		}
		return packageUpdateCommandResult{}
	}
	updates.run = fakeRun
	updates.lookPath = func(string) (string, error) { return "/fake/apt-get", nil }
	updates.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	updates.cacheTTL = 0
	cleanup.run = fakeRun
	cleanup.lookPath = func(string) (string, error) { return "/fake/apt-get", nil }
	cleanup.cacheTTL = 0
	cleanup.scan = func() (agentexec.HostStorageCleanupSnapshot, error) {
		mu.Lock()
		defer mu.Unlock()
		if cleaned {
			return agentexec.HostStorageCleanupSnapshot{Fingerprint: "sha256:" + strings.Repeat("d", 64), ReclaimableBytes: 8 * 1024 * 1024}, nil
		}
		return agentexec.HostStorageCleanupSnapshot{Fingerprint: "sha256:" + strings.Repeat("c", 64), ReclaimableBytes: 512 * 1024 * 1024}, nil
	}
	client.packageUpdates = updates
	client.storageCleanup = cleanup

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- client.Run(ctx) }()
	defer func() {
		cancel()
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("client did not stop")
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for !server.IsAgentConnected("agent-apt") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !server.IsAgentConnected("agent-apt") {
		t.Fatal("real command client did not connect")
	}

	updateReq := agentexec.HostUpdatePayload{RequestID: "update.dispatch.1", ActionID: "update", Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: aptUpgradeInventoryHash(pendingSimulation), Timeout: 5}
	if err := agentexec.BindHostUpdatePayload(&updateReq); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		result, err := server.ExecuteHostUpdate(context.Background(), "agent-apt", updateReq)
		if err != nil || result == nil || !result.Success || result.Verification != agentexec.HostUpdateVerificationVerified {
			t.Fatalf("update replay %d result=%+v err=%v", i, result, err)
		}
	}

	cleanupReq := agentexec.HostStorageCleanupPayload{RequestID: "cleanup.dispatch.1", ActionID: "cleanup", Operation: agentexec.HostStorageCleanupOperationPackageCache, ExpectedFingerprint: "sha256:" + strings.Repeat("c", 64), Timeout: 5}
	if err := agentexec.BindHostStorageCleanupPayload(&cleanupReq); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		result, err := server.ExecuteHostStorageCleanup(context.Background(), "agent-apt", cleanupReq)
		if err != nil || result == nil || !result.Success || result.Verification != agentexec.HostStorageCleanupVerificationVerified {
			t.Fatalf("cleanup replay %d result=%+v err=%v", i, result, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if refreshCalls != 1 || upgradeCalls != 1 || cleanCalls != 1 {
		t.Fatalf("fake mutation catalog calls: refresh=%d upgrade=%d clean=%d", refreshCalls, upgradeCalls, cleanCalls)
	}
}

type countingDockerLifecycleManager struct {
	mu    sync.Mutex
	calls int
}

func (m *countingDockerLifecycleManager) Preflight(context.Context, agentexec.DockerContainerLifecyclePayload) (bool, string) {
	return true, ""
}

func (m *countingDockerLifecycleManager) Apply(_ context.Context, req agentexec.DockerContainerLifecyclePayload) agentexec.DockerContainerLifecycleResultPayload {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	now := time.Now().UTC()
	return agentexec.DockerContainerLifecycleResultPayload{
		RequestID: req.RequestID, ActionID: req.ActionID, Operation: req.Operation, OperationVersion: req.OperationVersion, RequestDigest: req.RequestDigest, ContainerID: req.ContainerID,
		ExecutionPhase: agentexec.DockerContainerPhaseComplete, MutationStarted: true, MutationCompleted: true, ReadbackRan: true,
		Before: agentexec.DockerContainerLifecycleSnapshot{ContainerID: req.ContainerID, State: "running", Running: true, StartedAt: req.ExpectedStartedAt, ObservedAt: now.Add(-time.Second)},
		After:  agentexec.DockerContainerLifecycleSnapshot{ContainerID: req.ContainerID, State: "running", Running: true, StartedAt: now, ObservedAt: now},
	}
}

func TestRealServerAndUnifiedAgentWebSocketDockerDuplicateReplayMutatesOnce(t *testing.T) {
	server := agentexec.NewServer(func(token, agent, host string) bool { return token == "token" })
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()
	logger := zerolog.Nop()
	client := NewCommandClient(Config{PulseURL: httpServer.URL, APIToken: "token", StateDir: t.TempDir(), Logger: &logger}, "agent-docker", "host", "linux", "6")
	manager := &countingDockerLifecycleManager{}
	client.dockerLifecycle = manager
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- client.Run(ctx) }()
	defer func() { cancel(); _ = client.Close(); <-done }()
	deadline := time.Now().Add(3 * time.Second)
	for !server.IsAgentConnected("agent-docker") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !server.IsAgentConnected("agent-docker") {
		t.Fatal("real command client did not connect")
	}
	startedAt := time.Now().UTC().Add(-time.Minute)
	req := agentexec.DockerContainerLifecyclePayload{RequestID: "docker.dispatch.1", ActionID: "docker", Operation: agentexec.DockerContainerOperationRestart, Runtime: "docker", ContainerID: dockerLifecycleTestContainerID, ExpectedState: "running", ExpectedStartedAt: startedAt, Timeout: 5}
	if err := agentexec.BindDockerContainerLifecyclePayload(&req); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		result, err := server.ExecuteDockerContainerLifecycle(context.Background(), "agent-docker", req)
		if err != nil || result == nil || !result.MutationCompleted || !result.ReadbackRan {
			t.Fatalf("replay %d result=%+v err=%v", i, result, err)
		}
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.calls != 1 {
		t.Fatalf("typed docker mutation calls = %d, want one", manager.calls)
	}
}

// Registration publishes a server session before the runner has activated its
// credential, durably written health, and started reading command messages.
// Observe each new client's actual health write, not a file left by its predecessor.
func observeReceiptRunnerActivation(client *CommandClient) <-chan struct{} {
	activated := make(chan struct{}, 1)
	client.actionHealthWriter = func(ready bool) error {
		if err := client.writeActionRunnerHealth(ready); err != nil {
			return err
		}
		if ready {
			select {
			case activated <- struct{}{}:
			default:
			}
		}
		return nil
	}
	return activated
}

func waitForActivatedReceiptRunner(ctx context.Context, server *agentexec.Server, identity operationreceipt.Identity, activated <-chan struct{}, done <-chan error) error {
	select {
	case <-activated:
	case err := <-done:
		return fmt.Errorf("runner exited before activation: %v", err)
	case <-ctx.Done():
		return ctx.Err()
	}
	// An actual receipt-query round trip proves the activated client's reader is
	// serving this session. It admits no mutation and changes no receipt.
	if _, err := server.QueryAgentOperation(ctx, identity.AgentID, identity); err != nil {
		return fmt.Errorf("activated runner receipt query: %w", err)
	}
	return nil
}

func TestReceiptRunnerReadinessWaitsForActivationReconnect(t *testing.T) {
	admission := agentexec.AgentAdmission{
		TokenID: "readiness-token", AgentID: "agent-ready", Hostname: "ready.example.test",
		RuntimeRole: agentexec.RuntimeRoleActionRunner, ActionCapability: agentexec.ActionCapabilityTypedV1,
	}
	server := agentexec.NewServerWithAdmissionValidator(func(token, _, _ string) (agentexec.AgentAdmission, bool) {
		return admission, token == admission.TokenID
	}, func(agentexec.AgentAdmission) bool { return true })
	activationEntered := make(chan struct{})
	releaseActivation := make(chan struct{})
	var releaseOnce sync.Once
	var activationCalls atomic.Int32
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPatch && request.URL.Path == "/api/agents/action-runner/credential" {
			if activationCalls.Add(1) == 1 {
				close(activationEntered)
				select {
				case <-releaseActivation:
					w.WriteHeader(http.StatusServiceUnavailable)
				case <-request.Context().Done():
				}
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		server.HandleWebSocket(w, request)
	}))
	t.Cleanup(httpServer.Close)
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseActivation) }) })
	origReconnectDelay := reconnectDelay
	reconnectDelay = 50 * time.Millisecond
	t.Cleanup(func() { reconnectDelay = origReconnectDelay })
	dir := t.TempDir()
	logger := zerolog.Nop()
	client := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: httpServer.URL, APIToken: admission.TokenID, StateDir: dir,
		HealthPath: filepath.Join(dir, "health.json"), ActivationNonce: strings.Repeat("b", 32), Logger: &logger,
	}, admission.AgentID, admission.Hostname, "test")
	activated := observeReceiptRunnerActivation(client)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- client.Run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("readiness runner did not stop")
		}
	})
	select {
	case <-activationEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("runner did not reach the controlled activation")
	}
	if !server.IsAgentConnected(admission.AgentID) {
		t.Fatal("control must expose the registered but unactivated session")
	}
	digest, err := operationreceipt.DigestCanonicalJSON(map[string]string{"readiness": "query-only"})
	if err != nil {
		t.Fatal(err)
	}
	identity := operationreceipt.Identity{AttemptID: "readiness.1", ActionID: "readiness", OperationKind: "fake.typed", OperationVersion: 1, RequestDigest: digest, AgentID: admission.AgentID}
	blockedCtx, cancelBlocked := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = waitForActivatedReceiptRunner(blockedCtx, server, identity, activated, done)
	cancelBlocked()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unactivated session passed readiness: %v", err)
	}
	releaseOnce.Do(func() { close(releaseActivation) })
	readyCtx, cancelReady := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelReady()
	if err := waitForActivatedReceiptRunner(readyCtx, server, identity, activated, done); err != nil {
		t.Fatalf("reconnected activated runner: %v", err)
	}
	if activationCalls.Load() != 2 {
		t.Fatalf("activation attempts = %d, want failed first session and activated replacement", activationCalls.Load())
	}
	data, err := os.ReadFile(filepath.Join(dir, "health.json"))
	if err != nil {
		t.Fatal(err)
	}
	var health actionRunnerHealth
	if err := json.Unmarshal(data, &health); err != nil || !health.Registered || !health.Activated {
		t.Fatalf("reconnected health = %+v, err=%v", health, err)
	}
}

func TestRealServerActionRunnerCancellationPersistsAndReplaysProxmoxReceiptAfterReconnect(t *testing.T) {
	admission := agentexec.AgentAdmission{
		TokenID: "runner-token", AgentID: "agent-pve", Hostname: "pve.example.test",
		RuntimeRole: agentexec.RuntimeRoleActionRunner, ActionCapability: agentexec.ActionCapabilityTypedV1,
	}
	server := agentexec.NewServerWithAdmissionValidator(func(token, _, _ string) (agentexec.AgentAdmission, bool) {
		return admission, token == admission.TokenID
	}, func(agentexec.AgentAdmission) bool { return true })
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPatch && request.URL.Path == "/api/agents/action-runner/credential" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		server.HandleWebSocket(w, request)
	}))
	t.Cleanup(httpServer.Close)

	// A runner that drops its session must come back well inside the replay
	// window rather than after the production reconnect backoff.
	origReconnectDelay := reconnectDelay
	reconnectDelay = 50 * time.Millisecond
	t.Cleanup(func() { reconnectDelay = origReconnectDelay })

	stateDir := t.TempDir()
	// Keep runner logs so a failure shows why a session dropped. They are only
	// printed from the cleanup below, never from runner goroutines that can
	// outlive the test.
	runnerLogs := &lockedLogBuffer{}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("action runner log:\n%s", runnerLogs.String())
		}
	})
	logger := zerolog.New(runnerLogs).With().Timestamp().Logger()
	mutationStarted := make(chan struct{})
	var startOnce sync.Once
	var mutationMu sync.Mutex
	mutationCalls := 0
	manager := newProxmoxGuestLifecycleManager()
	manager.run = func(ctx context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) == 0 {
			return nil, errors.New("missing Proxmox verb")
		}
		if args[0] == "status" {
			return []byte("status: running"), nil
		}
		if args[0] != "shutdown" || len(args) != 2 || args[1] != "101" {
			return nil, errors.New("unexpected Proxmox mutation")
		}
		mutationMu.Lock()
		mutationCalls++
		mutationMu.Unlock()
		startOnce.Do(func() { close(mutationStarted) })
		<-ctx.Done()
		return nil, ctx.Err()
	}

	request := agentexec.ProxmoxGuestLifecyclePayload{
		RequestID: "pve.cancel-replay.1", ActionID: "pve.cancel-replay", Operation: "shutdown",
		GuestKind: "vm", VMID: 101, ExpectedStatus: "running", Timeout: 30,
	}
	if err := agentexec.BindProxmoxGuestLifecyclePayload(&request); err != nil {
		t.Fatal(err)
	}
	identity := agentexec.ProxmoxGuestLifecycleOperationIdentity(admission.AgentID, request)

	stopRunner := func(t *testing.T, client *CommandClient, cancel context.CancelFunc, done <-chan error) {
		t.Helper()
		cancel()
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("action runner did not stop")
			return
		}
		// The client has exited, but the server observes the socket close on
		// its own reader goroutine. Reconnecting before that lands would let
		// startRunner see the stale session as "connected" and dispatch the
		// replay to a dead socket.
		deadline := time.Now().Add(3 * time.Second)
		for server.IsAgentConnected(admission.AgentID) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if server.IsAgentConnected(admission.AgentID) {
			t.Error("server did not observe the action runner disconnect")
		}
	}

	startRunner := func(t *testing.T) func() {
		t.Helper()
		client := NewActionRunnerClient(ActionRunnerClientConfig{
			PulseURL: httpServer.URL, APIToken: admission.TokenID, StateDir: stateDir,
			HealthPath: filepath.Join(stateDir, "health.json"), ActivationNonce: strings.Repeat("a", 32), Logger: &logger,
		}, admission.AgentID, admission.Hostname, "test")
		client.proxmoxGuestLifecycle = manager
		activated := observeReceiptRunnerActivation(client)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- client.Run(ctx)
			close(done)
		}()
		// Register cleanup before any readiness assertion. A failed start must
		// not leave the runner alive while httptest waits for its handlers.
		var stopOnce sync.Once
		stop := func() { stopOnce.Do(func() { stopRunner(t, client, cancel, done) }) }
		t.Cleanup(stop)
		readyCtx, cancelReady := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelReady()
		if err := waitForActivatedReceiptRunner(readyCtx, server, identity, activated, done); err != nil {
			t.Fatalf("action runner was not ready: %v", err)
		}
		return stop
	}

	stopFirst := startRunner(t)
	dispatchCtx, cancelDispatch := context.WithCancel(context.Background())
	t.Cleanup(cancelDispatch)
	dispatchDone := make(chan error, 1)
	go func() {
		_, err := server.ExecuteProxmoxGuestLifecycle(dispatchCtx, admission.AgentID, request)
		dispatchDone <- err
	}()
	select {
	case <-mutationStarted:
	case err := <-dispatchDone:
		t.Fatalf("Proxmox dispatch returned before mutation: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("Proxmox mutation did not start")
	}
	cancelDispatch()
	if err := <-dispatchDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled dispatch error = %v, want context.Canceled", err)
	}

	var query operationreceipt.QueryResult
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		queryCtx, cancelQuery := context.WithTimeout(context.Background(), time.Second)
		result, err := server.QueryAgentOperation(queryCtx, admission.AgentID, identity)
		cancelQuery()
		if err == nil && result.Status == operationreceipt.QueryFoundTerminal {
			query = result
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if query.Record == nil || query.Status != operationreceipt.QueryFoundTerminal {
		t.Fatalf("terminal receipt not observed after cancellation: %+v", query)
	}
	var canceled agentexec.ProxmoxGuestLifecycleResultPayload
	if err := json.Unmarshal(query.Record.Result, &canceled); err != nil {
		t.Fatal(err)
	}
	if !canceled.MutationStarted || canceled.MutationCompleted || !strings.Contains(canceled.Error, "recovery inspection") {
		t.Fatalf("canceled durable receipt = %+v", canceled)
	}
	stopFirst()

	stopSecond := startRunner(t)
	defer stopSecond()
	// Readiness above proves activation and a live reader. Retain the existing
	// bounded recovery for a genuinely dropped session after that proof: every
	// attempt must replay the durable receipt without a second mutation.
	var replayed *agentexec.ProxmoxGuestLifecycleResultPayload
	var err error
	for attempt := 1; ; attempt++ {
		replayed, err = server.ExecuteProxmoxGuestLifecycle(context.Background(), admission.AgentID, request)
		if err == nil || attempt == 3 || !strings.Contains(err.Error(), "disconnected before") {
			break
		}
		t.Logf("replay attempt %d lost its session (%v), retrying after reconnect", attempt, err)
		deadline := time.Now().Add(3 * time.Second)
		for !server.IsAgentConnected(admission.AgentID) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if err != nil || replayed == nil || !replayed.MutationStarted || replayed.MutationCompleted || replayed.Error != canceled.Error {
		t.Fatalf("replayed result = %+v, err=%v", replayed, err)
	}
	mutationMu.Lock()
	if mutationCalls != 1 {
		t.Fatalf("Proxmox mutations = %d, want one", mutationCalls)
	}
	mutationMu.Unlock()

	for _, mutate := range []func(*operationreceipt.Identity){
		func(conflict *operationreceipt.Identity) { conflict.ActionID = "different-action" },
		func(conflict *operationreceipt.Identity) {
			conflict.RequestDigest = "sha256:" + strings.Repeat("f", 64)
		},
	} {
		conflict := identity
		mutate(&conflict)
		queryCtx, cancelQuery := context.WithTimeout(context.Background(), 250*time.Millisecond)
		_, err := server.QueryAgentOperation(queryCtx, admission.AgentID, conflict)
		cancelQuery()
		if err == nil {
			t.Fatalf("conflicting receipt identity was accepted: %+v", conflict)
		}
	}
	wrongAgent := identity
	wrongAgent.AgentID = "other-agent"
	if _, err := server.QueryAgentOperation(context.Background(), admission.AgentID, wrongAgent); !errors.Is(err, operationreceipt.ErrBindingConflict) {
		t.Fatalf("cross-agent receipt query error = %v", err)
	}
}

// lockedLogBuffer collects zerolog output from concurrent runner goroutines.
type lockedLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
