package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

func TestCommandClientProxmoxRunnerUsesProviderHandoffOnlyForGuestCreation(t *testing.T) {
	tests := []struct {
		name            string
		kind            string
		operation       string
		before          string
		after           string
		mutationCatalog typedActionCatalog
	}{
		{name: "VM start hands off", kind: "vm", operation: "start", before: "stopped", after: "running", mutationCatalog: typedActionCatalogProxmoxHandoff},
		{name: "container start hands off", kind: "ct", operation: "start", before: "stopped", after: "running", mutationCatalog: typedActionCatalogProxmoxHandoff},
		{name: "VM reboot stays contained", kind: "vm", operation: "reboot", before: "running", after: "running", mutationCatalog: typedActionCatalogProxmox},
		{name: "container reboot hands off", kind: "ct", operation: "reboot", before: "running", after: "running", mutationCatalog: typedActionCatalogProxmoxHandoff},
		{name: "graceful shutdown stays contained", kind: "ct", operation: "shutdown", before: "running", after: "stopped", mutationCatalog: typedActionCatalogProxmox},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := agentexec.ProxmoxGuestLifecyclePayload{
				RequestID: "attempt-" + test.kind + "-" + test.operation,
				ActionID:  "action-" + test.kind + "-" + test.operation,
				Operation: test.operation, GuestKind: test.kind, VMID: 141,
				ExpectedStatus: test.before, Timeout: 30,
			}
			if err := agentexec.BindProxmoxGuestLifecyclePayload(&payload); err != nil {
				t.Fatal(err)
			}
			type invocation struct {
				catalog typedActionCatalog
				name    string
				args    []string
			}
			var calls []invocation
			manager := newProxmoxGuestLifecycleManagerWithTypedRunner(func(_ context.Context, _ []string, catalog typedActionCatalog, name string, args ...string) typedActionCommandResult {
				calls = append(calls, invocation{catalog: catalog, name: name, args: append([]string(nil), args...)})
				if args[0] == "status" {
					status := test.before
					if len(calls) == 3 {
						status = test.after
					}
					return typedActionCommandResult{stdout: "status: " + status, exitCode: 0}
				}
				return typedActionCommandResult{exitCode: 0}
			})
			result := manager.Apply(context.Background(), payload)
			if result.ExecutionPhase != agentexec.ProxmoxGuestPhaseComplete || !result.MutationCompleted || !result.ReadbackRan {
				t.Fatalf("result=%+v", result)
			}
			tool := "qm"
			if test.kind == "ct" {
				tool = "pct"
			}
			if len(calls) != 3 || calls[0].catalog != typedActionCatalogProxmox || calls[0].name != tool || !reflect.DeepEqual(calls[0].args, []string{"status", "141"}) ||
				calls[1].catalog != test.mutationCatalog || calls[1].name != tool || !reflect.DeepEqual(calls[1].args, []string{test.operation, "141"}) ||
				calls[2].catalog != typedActionCatalogProxmox || calls[2].name != tool || !reflect.DeepEqual(calls[2].args, []string{"status", "141"}) {
				t.Fatalf("typed action calls=%+v", calls)
			}
		})
	}
}

func TestNew_DefaultPulseURLUsedForCommandClient(t *testing.T) {
	logger := zerolog.New(io.Discard)

	agent, err := New(Config{
		APIToken:       "test-token",
		LogLevel:       zerolog.InfoLevel,
		Logger:         &logger,
		EnableCommands: true, // Commands are disabled by default; enable for this test
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const want = "http://localhost:7655"
	if agent.trimmedPulseURL != want {
		t.Fatalf("trimmedPulseURL = %q, want %q", agent.trimmedPulseURL, want)
	}
	if agent.cfg.PulseURL != want {
		t.Fatalf("cfg.PulseURL = %q, want %q", agent.cfg.PulseURL, want)
	}
	if agent.commandClient == nil {
		t.Fatalf("commandClient should be initialized")
	}
	if agent.commandClient.pulseURL != want {
		t.Fatalf("commandClient.pulseURL = %q, want %q", agent.commandClient.pulseURL, want)
	}
}

func TestNew_TrimsPulseURLForCommandClient(t *testing.T) {
	logger := zerolog.New(io.Discard)

	agent, err := New(Config{
		PulseURL:       "https://example.invalid/",
		APIToken:       "test-token",
		LogLevel:       zerolog.InfoLevel,
		Logger:         &logger,
		EnableCommands: true, // Commands are disabled by default; enable for this test
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const want = "https://example.invalid"
	if agent.trimmedPulseURL != want {
		t.Fatalf("trimmedPulseURL = %q, want %q", agent.trimmedPulseURL, want)
	}
	if agent.cfg.PulseURL != want {
		t.Fatalf("cfg.PulseURL = %q, want %q", agent.cfg.PulseURL, want)
	}
	if agent.commandClient == nil {
		t.Fatalf("commandClient should be initialized")
	}
	if agent.commandClient.pulseURL != want {
		t.Fatalf("commandClient.pulseURL = %q, want %q", agent.commandClient.pulseURL, want)
	}
}

func TestCommandClientBuildWebSocketURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pulseURL string
		want     string
		wantErr  bool
	}{
		{
			name:     "https becomes wss",
			pulseURL: "https://example.invalid",
			want:     "wss://example.invalid/api/agent/ws",
		},
		{
			name:     "loopback http becomes ws",
			pulseURL: "http://localhost:7655",
			want:     "ws://localhost:7655/api/agent/ws",
		},
		{
			name:     "preserves path prefix",
			pulseURL: "https://example.invalid/pulse/",
			want:     "wss://example.invalid/pulse/api/agent/ws",
		},
		{
			name:     "wss preserved",
			pulseURL: "wss://example.invalid",
			want:     "wss://example.invalid/api/agent/ws",
		},
		{
			name:     "non-loopback http rejected",
			pulseURL: "http://example.invalid",
			wantErr:  true,
		},
		{
			name:     "private-network http becomes ws",
			pulseURL: "http://10.0.0.5:7655",
			want:     "ws://10.0.0.5:7655/api/agent/ws",
		},
		{
			name:     "non-loopback ws rejected",
			pulseURL: "ws://example.invalid",
			wantErr:  true,
		},
		{
			name:     "private-network ws preserved",
			pulseURL: "ws://10.0.0.5:7655",
			want:     "ws://10.0.0.5:7655/api/agent/ws",
		},
		{
			name:     "query rejected",
			pulseURL: "https://example.invalid?x=1",
			wantErr:  true,
		},
		{
			name:     "invalid url returns error",
			pulseURL: "http://[::1",
			wantErr:  true,
		},
		{
			name:     "unsupported scheme returns error",
			pulseURL: "ftp://example.invalid",
			wantErr:  true,
		},
		{
			name:     "home domain http becomes ws",
			pulseURL: "http://ct-pulse.home:7655/pulse",
			want:     "ws://ct-pulse.home:7655/pulse/api/agent/ws",
		},
		{
			name:     "missing host returns error",
			pulseURL: "/relative/path",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &CommandClient{pulseURL: tt.pulseURL}
			got, err := client.buildWebSocketURL()
			if (err != nil) != tt.wantErr {
				t.Fatalf("buildWebSocketURL() err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Fatalf("buildWebSocketURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommandClientBuildWebSocketOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pulseURL string
		want     string
		wantErr  bool
	}{
		{
			name:     "https becomes https origin",
			pulseURL: "https://example.invalid/pulse/",
			want:     "https://example.invalid",
		},
		{
			name:     "loopback http stays http origin",
			pulseURL: "http://localhost:7655/pulse",
			want:     "http://localhost:7655",
		},
		{
			name:     "wss becomes https origin",
			pulseURL: "wss://example.invalid",
			want:     "https://example.invalid",
		},
		{
			name:     "non-loopback http rejected",
			pulseURL: "http://example.invalid",
			wantErr:  true,
		},
		{
			name:     "missing host rejected",
			pulseURL: "/relative/path",
			wantErr:  true,
		},
		{
			name:     "private-network http stays http origin",
			pulseURL: "http://10.0.0.5:7655/pulse",
			want:     "http://10.0.0.5:7655",
		},
		{
			name:     "home domain http stays http origin",
			pulseURL: "http://ct-pulse.home:7655/pulse",
			want:     "http://ct-pulse.home:7655",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &CommandClient{pulseURL: tt.pulseURL}
			got, err := client.buildWebSocketOrigin()
			if (err != nil) != tt.wantErr {
				t.Fatalf("buildWebSocketOrigin() err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Fatalf("buildWebSocketOrigin() = %q, want %q", got, tt.want)
			}
		})
	}
}

// A server-issued cancel_command for an in-flight request must cancel exactly
// that request's execution context (minipc probe-storm regression: abandoned
// probes previously ran to their full timeout on the agent).
func TestCommandClient_handleCancelCommand_CancelsRegisteredRequest(t *testing.T) {
	c := &CommandClient{logger: zerolog.Nop()}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	state, _ := c.registerActiveCommand(nil, "req-1", cancel)
	defer c.finishCancellableRequest(nil, "req-1", state)

	c.handleCancelCommand(nil, cancelCommandPayload{RequestID: "req-1"})

	select {
	case <-ctx.Done():
	default:
		t.Fatalf("cancel_command did not cancel the registered request context")
	}
}

// A cancel for a request that already finished (or never existed) must be a
// no-op, not a panic or a cancellation of some other command.
func TestCommandClient_handleCancelCommand_UnknownRequestIsNoOp(t *testing.T) {
	c := &CommandClient{logger: zerolog.Nop()}

	otherCtx, otherCancel := context.WithCancel(context.Background())
	defer otherCancel()
	state, _ := c.registerActiveCommand(nil, "other", otherCancel)
	defer c.finishCancellableRequest(nil, "other", state)

	c.handleCancelCommand(nil, cancelCommandPayload{RequestID: "missing"})

	select {
	case <-otherCtx.Done():
		t.Fatalf("cancel for unknown request canceled an unrelated command")
	default:
	}
}

func TestCommandClient_CancellationBeforeRegistrationIsConsumedAndConnectionScoped(t *testing.T) {
	c := &CommandClient{logger: zerolog.Nop()}
	firstConnection := &websocket.Conn{}
	secondConnection := &websocket.Conn{}
	const requestID = "typed-before-register"

	firstState := c.noteCancellableRequest(firstConnection, requestID)
	if firstState == nil {
		t.Fatal("failed to admit first cancellable request")
	}
	c.handleCancelCommand(firstConnection, cancelCommandPayload{RequestID: requestID})
	firstCtx, firstCancel := context.WithCancel(context.Background())
	defer firstCancel()
	_, firstRegistered := c.registerActiveCommand(firstConnection, requestID, firstCancel)
	if firstRegistered {
		t.Fatal("pre-registration cancellation did not stop provider handoff")
	}
	select {
	case <-firstCtx.Done():
	default:
		t.Fatal("pre-registration cancellation was not consumed by registration")
	}

	secondState := c.noteCancellableRequest(secondConnection, requestID)
	if secondState == nil {
		t.Fatal("connection-scoped request id was incorrectly treated as a duplicate")
	}
	secondCtx, secondCancel := context.WithCancel(context.Background())
	defer secondCancel()
	_, secondRegistered := c.registerActiveCommand(secondConnection, requestID, secondCancel)
	if !secondRegistered {
		t.Fatal("cancellation leaked into a different WebSocket generation")
	}
	c.clearCancellableRequests(firstConnection)
	select {
	case <-secondCtx.Done():
		t.Fatal("first-generation teardown canceled second-generation work")
	default:
	}
	c.clearCancellableRequests(secondConnection)
	select {
	case <-secondCtx.Done():
	default:
		t.Fatal("connection teardown did not cancel its active request")
	}

	thirdConnection := &websocket.Conn{}
	thirdState := c.noteCancellableRequest(thirdConnection, "teardown-before-register")
	if thirdState == nil {
		t.Fatal("failed to admit teardown-race request")
	}
	c.clearCancellableRequests(thirdConnection)
	thirdCtx, thirdCancel := context.WithCancel(context.Background())
	defer thirdCancel()
	_, thirdRegistered := c.registerActiveCommand(thirdConnection, "teardown-before-register", thirdCancel)
	if thirdRegistered {
		t.Fatal("request crossed registration after its connection was torn down")
	}
	select {
	case <-thirdCtx.Done():
	default:
		t.Fatal("teardown tombstone was not consumed at registration")
	}
}

func TestCommandClient_CancellableRequestAdmissionIsBounded(t *testing.T) {
	c := &CommandClient{logger: zerolog.Nop()}
	conn := &websocket.Conn{}
	states := make([]*cancellableRequestState, maxCancellableRequestsPerConnection)
	for i := 0; i < maxCancellableRequestsPerConnection; i++ {
		states[i] = c.noteCancellableRequest(conn, fmt.Sprintf("request-%d", i))
		if states[i] == nil {
			t.Fatalf("request %d was refused below the bound", i)
		}
	}
	if c.noteCancellableRequest(conn, "over-capacity") != nil {
		t.Fatal("over-capacity cancellable request was admitted")
	}
	c.clearCancellableRequests(conn)
	for i := 0; i < maxCancellableRequestsPerConnection; i++ {
		c.finishCancellableRequest(conn, fmt.Sprintf("request-%d", i), states[i])
	}
	if c.noteCancellableRequest(conn, "after-clear") == nil {
		t.Fatal("cancellable request capacity did not recover after teardown")
	}
}

func TestCommandClient_StaleCleanupCannotEraseReusedRequestCancellation(t *testing.T) {
	c := &CommandClient{logger: zerolog.Nop()}
	conn := &websocket.Conn{}
	const requestID = "reused-request"

	firstState := c.noteCancellableRequest(conn, requestID)
	if firstState == nil {
		t.Fatal("failed to admit first request generation")
	}
	firstCtx, firstCancel := context.WithCancel(context.Background())
	registeredState, registered := c.registerActiveCommand(conn, requestID, firstCancel)
	if !registered || registeredState != firstState {
		t.Fatal("first request generation did not register")
	}
	// Model the handler's cleanup before its wrapper defer runs.
	c.finishCancellableRequest(conn, requestID, registeredState)

	secondState := c.noteCancellableRequest(conn, requestID)
	if secondState == nil || secondState == firstState {
		t.Fatal("failed to admit a distinct reused request generation")
	}
	// The stale outer cleanup from generation A must not delete generation B.
	c.finishCancellableRequest(conn, requestID, firstState)
	c.handleCancelCommand(conn, cancelCommandPayload{RequestID: requestID})
	secondCtx, secondCancel := context.WithCancel(context.Background())
	defer secondCancel()
	registeredState, registered = c.registerActiveCommand(conn, requestID, secondCancel)
	if registered || registeredState != secondState {
		t.Fatal("reused request lost its pre-registration cancellation fence")
	}
	select {
	case <-secondCtx.Done():
	default:
		t.Fatal("reused request cancellation was not consumed")
	}
	c.finishCancellableRequest(conn, requestID, secondState)
	firstCancel()
	select {
	case <-firstCtx.Done():
	default:
		t.Fatal("first request cleanup did not retain its own cancel function")
	}
}

func TestCommandClientActionRunnerMessageCatalogRejectsGenericAuthority(t *testing.T) {
	for _, message := range []messageType{msgTypeExecuteCmd, msgTypeReadFile, msgTypeDeployPreflight, msgTypeDeployInstall, msgTypeDeployCancel} {
		if allowedActionRunnerMessage(message) {
			t.Fatalf("action runner unexpectedly admitted generic message %q", message)
		}
	}
	for _, message := range []messageType{msgTypeHostUpdate, msgTypeHostStorageCleanup, msgTypeProxmoxGuestLifecycle, msgTypeDockerContainerLifecycle} {
		if !allowedActionRunnerMessage(message) {
			t.Fatalf("action runner rejected typed message %q", message)
		}
	}
}

func TestCommandClient_ReplayedRequestWaitsForInFlightHandlerInsteadOfDropping(t *testing.T) {
	c := &CommandClient{logger: zerolog.Nop()}
	conn := &websocket.Conn{}
	const requestID = "typed-replay"

	release := make(chan struct{})
	firstRunning := make(chan struct{})
	secondRan := make(chan struct{})
	c.launchCancellableRequest(conn, requestID, "typed", func() {
		close(firstRunning)
		<-release
	})
	select {
	case <-firstRunning:
	case <-time.After(2 * time.Second):
		t.Fatal("first handler did not start")
	}

	// The replay arrives while the first handler still owns the slot. It must
	// not be dropped; it runs once the first handler releases the slot, so it
	// can answer from the durable receipt.
	c.launchCancellableRequest(conn, requestID, "typed", func() { close(secondRan) })
	select {
	case <-secondRan:
		t.Fatal("replay ran while the first handler still owned the slot")
	case <-time.After(50 * time.Millisecond):
	}
	if c.inflightCancellableRequest(conn, requestID) == nil {
		t.Fatal("first handler lost its slot before finishing")
	}

	close(release)
	select {
	case <-secondRan:
	case <-time.After(2 * time.Second):
		t.Fatal("replay was dropped instead of running after the first handler finished")
	}
	deadline := time.Now().Add(2 * time.Second)
	for c.inflightCancellableRequest(conn, requestID) != nil && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if c.inflightCancellableRequest(conn, requestID) != nil {
		t.Fatal("replay handler did not release the slot")
	}
}

func TestCommandClientGuestExecutionAdmissionContract(t *testing.T) {
	testVMGuestExecNoUnverifiedHandoff(t)
}

func TestGuestExecRealServerAgentDiscoveryAdmission(t *testing.T) {
	testGuestExecRealServerAgentDiscoveryAdmission(t)
}

func TestCommandClientGuestCompletionContract(t *testing.T) {
	TestGuestExecZeroCLIExitDoesNotProveGuestCompletion(t)
}

func TestCommandClientGuestOutcomeContract(t *testing.T) {
	t.Run("execution-and-admission", testGuestExecProjectsGuestOutcome)
	t.Run("terminal-dictionary", TestGuestExecTerminalDictionary)
}

// commandIdentityHarness runs an agent with commands enabled against a fake
// Pulse whose report acknowledgement the test scripts, and records the
// identity of every command client that starts.
type commandIdentityHarness struct {
	agent   *Agent
	server  *httptest.Server
	started chan string

	mu      sync.Mutex
	reports int
	built   int
	// refuse makes every started client behave as if Pulse refused its
	// registration; otherwise a started client counts as admitted.
	refuse bool
}

func newCommandIdentityHarness(t *testing.T, presentedID string, interval time.Duration, respond func(report int, w http.ResponseWriter)) *commandIdentityHarness {
	t.Helper()
	h := &commandIdentityHarness{started: make(chan string, 8)}
	h.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.reports++
		report := h.reports
		h.mu.Unlock()
		respond(report, w)
	}))
	t.Cleanup(h.server.Close)

	logger := zerolog.Nop()
	agent, err := New(Config{
		APIToken:       "token",
		PulseURL:       "https://pulse",
		AgentID:        presentedID,
		EnableCommands: true,
		Interval:       interval,
		StateDir:       t.TempDir(),
		Collector:      &mockCollector{},
		Logger:         &logger,
		newCommandClientFn: func(cfg Config, agentID, hostname, platform, version string) *CommandClient {
			h.mu.Lock()
			h.built++
			h.mu.Unlock()
			return NewCommandClient(cfg, agentID, hostname, platform, version)
		},
		runCommandClientFn: func(client *CommandClient, ctx context.Context) error {
			h.mu.Lock()
			refuse := h.refuse
			h.mu.Unlock()
			if !refuse {
				client.markAdmitted()
			}
			h.started <- client.agentID
			<-ctx.Done()
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	agent.httpClient = h.server.Client()
	agent.trimmedPulseURL = h.server.URL
	h.agent = agent
	// Run closes the command client on exit, but several tests never call
	// run. Close it here, before the state TempDir is removed: Windows cannot
	// delete the open operation-receipts.db.
	t.Cleanup(func() { agent.stopCommandClient(true) })
	return h
}

func (h *commandIdentityHarness) run(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = h.agent.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func (h *commandIdentityHarness) waitForReports(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		h.mu.Lock()
		reports := h.reports
		h.mu.Unlock()
		if reports >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d reports were sent, want %d", reports, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (h *commandIdentityHarness) clientIdentity() string {
	h.agent.commandClientMu.Lock()
	defer h.agent.commandClientMu.Unlock()
	if h.agent.commandClient == nil {
		return ""
	}
	return h.agent.commandClient.agentID
}

func (h *commandIdentityHarness) expectStart(t *testing.T, want string) {
	t.Helper()
	select {
	case got := <-h.started:
		if got != want {
			t.Fatalf("command channel registered as %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("command channel never started; want it registered as %q", want)
	}
}

func (h *commandIdentityHarness) expectNoStart(t *testing.T, within time.Duration) {
	t.Helper()
	select {
	case got := <-h.started:
		t.Fatalf("command channel registered as %q before Pulse acknowledged a report", got)
	case <-time.After(within):
	}
}

func (h *commandIdentityHarness) expectNoRestart(t *testing.T) {
	t.Helper()
	select {
	case got := <-h.started:
		t.Fatalf("command channel registered again, as %q", got)
	default:
	}
}

func writeAck(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

// A token binds the first identity its command channel presents and never
// moves. An agent started with a configured ID (or an ID Pulse forked, or
// kept from an earlier enrollment) used to register that ID before its first
// report, while Pulse resolved its reports to another host ID that the agent
// persisted and presented after the next restart: the channel was refused for
// good. The channel now waits for the acknowledgement and registers under the
// identity Pulse names for it.
func TestCommandChannelRegistersUnderTheIdentityPulseAcknowledges(t *testing.T) {
	release := make(chan struct{})
	h := newCommandIdentityHarness(t, "configured-id", time.Hour, func(report int, w http.ResponseWriter) {
		if report == 1 {
			<-release
		}
		writeAck(w, `{"success":true,"agentId":"server-resolved","commandAgentId":"server-resolved"}`)
	})
	h.run(t)

	h.expectNoStart(t, 300*time.Millisecond)
	close(release)
	h.expectStart(t, "server-resolved")
}

// An older Pulse names no command identity, and a current Pulse names none for
// a token already bound to the ID this agent presents (for example a
// configured --agent-id). The channel then registers under the agent's own ID
// exactly as before, once a report is acknowledged.
func TestCommandChannelKeepsItsOwnIdentityWhenPulseNamesNone(t *testing.T) {
	h := newCommandIdentityHarness(t, "configured-id", time.Hour, func(_ int, w http.ResponseWriter) {
		writeAck(w, `{"success":true,"agentId":"server-resolved"}`)
	})
	h.run(t)

	h.expectStart(t, "configured-id")
}

// While Pulse is unreachable or refusing reports the agent has no answer, so
// the channel stays staged and registers once a later report is acknowledged.
func TestCommandChannelWaitsForAnAcknowledgedReport(t *testing.T) {
	h := newCommandIdentityHarness(t, "configured-id", 50*time.Millisecond, func(report int, w http.ResponseWriter) {
		if report < 3 {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		writeAck(w, `{"success":true,"agentId":"server-resolved","commandAgentId":"server-resolved"}`)
	})
	h.run(t)

	h.expectStart(t, "server-resolved")
	h.mu.Lock()
	reports := h.reports
	h.mu.Unlock()
	if reports < 3 {
		t.Fatalf("command channel started after %d report attempts, want it to wait for the accepted third", reports)
	}
}

// Once Pulse has named the channel's identity, a later acknowledgement that
// names none (Pulse now resolves the host elsewhere, and the binding refuses
// that ID) must not drop the channel back to a different ID: the token is
// bound to the acknowledged one.
func TestCommandChannelKeepsTheAcknowledgedIdentityWhenALaterAckNamesNone(t *testing.T) {
	h := newCommandIdentityHarness(t, "configured-id", 50*time.Millisecond, func(report int, w http.ResponseWriter) {
		if report == 1 {
			writeAck(w, `{"success":true,"agentId":"server-resolved","commandAgentId":"server-resolved"}`)
			return
		}
		writeAck(w, `{"success":true,"agentId":"healed-id"}`)
	})
	h.run(t)

	h.expectStart(t, "server-resolved")
	h.waitForReports(t, 4)
	h.expectNoRestart(t)
	if got := h.clientIdentity(); got != "server-resolved" {
		t.Fatalf("command client identity = %q, want the acknowledged %q", got, "server-resolved")
	}

	// A client rebuilt later in this process (commands disabled, then enabled
	// again) still registers under the acknowledged identity.
	h.agent.applyRemoteConfig(false)
	h.agent.applyRemoteConfig(true)
	h.expectStart(t, "server-resolved")
}

// Commands enabled by Pulse after start-up (rather than by --enable-commands)
// stage a client before any report is acknowledged. That client must register
// under the acknowledged identity, not the presented one.
func TestRemotelyEnabledCommandChannelRegistersUnderTheAcknowledgedIdentity(t *testing.T) {
	h := newCommandIdentityHarness(t, "configured-id", time.Hour, func(_ int, w http.ResponseWriter) {
		writeAck(w, `{"success":true,"agentId":"server-resolved","commandAgentId":"server-resolved","config":{"commandsEnabled":true}}`)
	})
	h.agent.stopCommandClient(true)
	h.agent.configMu.Lock()
	h.agent.cfg.EnableCommands = false
	h.agent.configMu.Unlock()
	h.run(t)

	h.expectStart(t, "server-resolved")
}

// An unreadable acknowledgement is no answer. Registering on it could bind a
// fresh token to the presented ID, which the next readable ack would not name,
// so the channel waits for an ack it can read.
func TestCommandChannelIgnoresAnUnreadableAck(t *testing.T) {
	h := newCommandIdentityHarness(t, "configured-id", 50*time.Millisecond, func(report int, w http.ResponseWriter) {
		if report == 1 {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>proxy</html>"))
			return
		}
		writeAck(w, `{"success":true,"agentId":"server-resolved","commandAgentId":"server-resolved"}`)
	})
	h.run(t)

	h.expectStart(t, "server-resolved")
}

// A token Pulse refuses reports for (403: no report scope) can never be named
// a command identity, and it may still run commands. Its channel registers
// under the agent's own ID as it did before registration waited for an ack.
func TestCommandChannelRegistersUnderItsOwnIdentityWhenReportsAreForbidden(t *testing.T) {
	h := newCommandIdentityHarness(t, "configured-id", time.Hour, func(_ int, w http.ResponseWriter) {
		http.Error(w, "missing scope", http.StatusForbidden)
	})
	h.run(t)

	h.expectStart(t, "configured-id")
}

// A running channel was admitted by its token. A later ack naming another
// identity must not tear it down and build a second client: that reopens the
// operation receipt store and marks in-flight operations interrupted.
func TestRunningCommandChannelIsNeverReplacedByALaterIdentity(t *testing.T) {
	h := newCommandIdentityHarness(t, "configured-id", 50*time.Millisecond, func(report int, w http.ResponseWriter) {
		if report == 1 {
			writeAck(w, `{"success":true,"agentId":"server-resolved"}`)
			return
		}
		writeAck(w, `{"success":true,"agentId":"server-resolved","commandAgentId":"server-resolved"}`)
	})
	h.run(t)

	h.expectStart(t, "configured-id")
	h.waitForReports(t, 3)
	h.expectNoRestart(t)
	if got := h.clientIdentity(); got != "configured-id" {
		t.Fatalf("running command client identity = %q, want it left on %q", got, "configured-id")
	}
	h.mu.Lock()
	built := h.built
	h.mu.Unlock()
	if built != 1 {
		t.Fatalf("command clients built = %d, want only the one New staged", built)
	}
}

// Observer destinations may run any Pulse version and never speak for this
// agent's identity: their acknowledgements neither name the command identity
// nor release the channel.
func TestObserverAckNeverReleasesOrNamesTheCommandChannel(t *testing.T) {
	server := newServerVersionAckServer(`{"success":true,"agentId":"observer-id","commandAgentId":"observer-id"}`)
	defer server.Close()

	h := newCommandIdentityHarness(t, "configured-id", time.Hour, func(_ int, w http.ResponseWriter) {
		writeAck(w, `{"success":true,"agentId":"server-resolved"}`)
	})
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.agent.commandClientMu.Lock()
	h.agent.commandClientRunCtx = runCtx
	h.agent.commandClientMu.Unlock()

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "configured-id"},
		Host:  agentshost.HostInfo{Hostname: "test-host"},
	}
	if err := h.agent.sendReportToDestination(context.Background(), report, server.URL, "observer-token", server.Client(), false); err != nil {
		t.Fatalf("sendReportToDestination: %v", err)
	}
	h.agent.commandClientMu.Lock()
	released := h.agent.commandClientParentCtx != nil
	identity := h.agent.commandIdentity
	h.agent.commandClientMu.Unlock()
	if released || identity != "" {
		t.Fatalf("observer ack released=%v identity=%q, want neither", released, identity)
	}
	h.expectNoStart(t, 100*time.Millisecond)
}

// A remote disable can close and clear a client another path is about to
// start. Starting that superseded client must fail without occupying the run
// slot, or the next enabled client never runs.
func TestSupersededCommandClientCannotOccupyTheRunSlot(t *testing.T) {
	h := newCommandIdentityHarness(t, "configured-id", time.Hour, func(_ int, w http.ResponseWriter) {
		writeAck(w, `{"success":true,"agentId":"server-resolved"}`)
	})
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.agent.commandClientMu.Lock()
	h.agent.commandClientParentCtx = runCtx
	superseded := h.agent.commandClient
	h.agent.commandClientMu.Unlock()

	h.agent.applyRemoteConfig(false)
	if h.agent.startCommandClient(superseded) {
		t.Fatal("a cleared command client was started")
	}
	h.agent.commandClientMu.Lock()
	occupied := h.agent.commandClientRunCancel != nil
	h.agent.commandClientMu.Unlock()
	if occupied {
		t.Fatal("a cleared command client occupied the run slot")
	}
	h.expectNoStart(t, 100*time.Millisecond)

	h.agent.applyRemoteConfig(true)
	h.expectStart(t, "configured-id")
}

// A client whose registration Pulse keeps refusing (for example one released
// by a mock-mode acknowledgement that named nothing, for a token bound to the
// resolved identity) has accepted no operation. When a later acknowledgement
// names the identity the token admits, it is retired and rebuilt under that
// identity instead of retrying the refused one for the life of the process.
func TestRefusedCommandChannelMovesToTheIdentityALaterAckNames(t *testing.T) {
	h := newCommandIdentityHarness(t, "configured-id", 50*time.Millisecond, func(report int, w http.ResponseWriter) {
		if report == 1 {
			writeAck(w, `{"success":true,"agentId":"configured-id"}`)
			return
		}
		writeAck(w, `{"success":true,"agentId":"server-resolved","commandAgentId":"server-resolved"}`)
	})
	h.refuse = true
	h.run(t)

	h.expectStart(t, "configured-id")
	h.expectStart(t, "server-resolved")
	if got := h.clientIdentity(); got != "server-resolved" {
		t.Fatalf("command client identity = %q, want %q", got, "server-resolved")
	}
}

// A decodable body that is not a Pulse acknowledgement (an empty object, or a
// refusal) is no answer either: the channel keeps waiting for a real ack.
func TestCommandChannelIgnoresAResponseThatIsNotAnAck(t *testing.T) {
	h := newCommandIdentityHarness(t, "configured-id", 50*time.Millisecond, func(report int, w http.ResponseWriter) {
		switch report {
		case 1:
			writeAck(w, `{}`)
		case 2:
			writeAck(w, `{"success":false}`)
		default:
			writeAck(w, `{"success":true,"agentId":"server-resolved","commandAgentId":"server-resolved"}`)
		}
	})
	h.run(t)

	h.expectStart(t, "server-resolved")
}

// A report buffered by an earlier run may present another agent ID or
// hostname, and Pulse judges the command identity for the report it
// acknowledges. Its acknowledgement must neither name nor release this
// process's channel.
func TestReplayedReportAckNeverNegotiatesTheCurrentChannel(t *testing.T) {
	h := newCommandIdentityHarness(t, "configured-id", time.Hour, func(_ int, w http.ResponseWriter) {
		writeAck(w, `{"success":true,"agentId":"server-resolved","commandAgentId":"server-resolved"}`)
	})
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.agent.commandClientMu.Lock()
	h.agent.commandClientRunCtx = runCtx
	h.agent.commandClientMu.Unlock()

	for _, replayed := range []agentshost.Report{
		{Agent: agentshost.AgentInfo{ID: "earlier-id"}, Host: agentshost.HostInfo{Hostname: h.agent.hostname}},
		{Agent: agentshost.AgentInfo{ID: "configured-id"}, Host: agentshost.HostInfo{Hostname: "earlier-hostname"}},
	} {
		if err := h.agent.sendReport(context.Background(), replayed); err != nil {
			t.Fatalf("sendReport: %v", err)
		}
	}
	h.agent.commandClientMu.Lock()
	released := h.agent.commandClientParentCtx != nil
	identity := h.agent.commandIdentity
	h.agent.commandClientMu.Unlock()
	if released || identity != "" {
		t.Fatalf("replayed acks released=%v identity=%q, want neither", released, identity)
	}
	h.expectNoStart(t, 100*time.Millisecond)

	current := agentshost.Report{Agent: agentshost.AgentInfo{ID: "configured-id"}, Host: agentshost.HostInfo{Hostname: h.agent.hostname}}
	if err := h.agent.sendReport(context.Background(), current); err != nil {
		t.Fatalf("sendReport: %v", err)
	}
	h.expectStart(t, "server-resolved")
}

// Pulse admits a client's registration before sending it anything. A client
// it admitted can no longer be retired, and one the agent retired can never be
// admitted afterwards.
func TestCommandClientAdmissionAndRetirementAreExclusive(t *testing.T) {
	admitted := &CommandClient{}
	if !admitted.markAdmitted() || !admitted.markAdmitted() {
		t.Fatal("an admitted client did not stay admitted across reconnects")
	}
	if admitted.retireIfNeverAdmitted() {
		t.Fatal("an admitted client was retired")
	}
	retired := &CommandClient{}
	if !retired.retireIfNeverAdmitted() {
		t.Fatal("a never-admitted client could not be retired")
	}
	if retired.markAdmitted() {
		t.Fatal("a retired client was admitted")
	}
}

// The admission state is set by the real registration handshake: a client
// Pulse accepts becomes admitted, and a client the agent retired before that
// stops right after registering instead of handling anything.
func TestCommandClientRegistrationAdmitsOnlyAnUnretiredClient(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var registration wsMessage
		if err := conn.ReadJSON(&registration); err != nil {
			return
		}
		payload, _ := json.Marshal(registeredPayload{Success: true, Message: "Registered"})
		if err := conn.WriteJSON(wsMessage{Type: msgTypeRegistered, Timestamp: time.Now(), Payload: payload}); err != nil {
			return
		}
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()

	newClient := func() *CommandClient {
		return &CommandClient{
			pulseURL: strings.TrimRight(server.URL, "/"),
			apiToken: "token",
			agentID:  "agent-1",
			hostname: "host-1",
			platform: "linux",
			version:  "1.2.3",
			logger:   zerolog.Nop(),
			done:     make(chan struct{}),
		}
	}

	admitted := newClient()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- admitted.connectAndHandle(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for admitted.admission.Load() != commandClientAdmitted {
		if time.Now().After(deadline) {
			t.Fatal("an accepted registration did not admit the client")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-errCh

	retired := newClient()
	if !retired.retireIfNeverAdmitted() {
		t.Fatal("precondition: a fresh client could not be retired")
	}
	done := make(chan error, 1)
	go func() { done <- retired.connectAndHandle(context.Background()) }()
	select {
	case err := <-done:
		if !errors.Is(err, errCommandClientSuperseded) {
			t.Fatalf("retired client connectAndHandle = %v, want errCommandClientSuperseded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a retired client kept running after its registration was accepted")
	}
}
