package hostagent

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
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
