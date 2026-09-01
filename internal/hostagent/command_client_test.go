package hostagent

import (
	"context"
	"io"
	"reflect"
	"testing"

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
	c.registerActiveCommand("req-1", cancel)
	defer c.unregisterActiveCommand("req-1")

	c.handleCancelCommand(cancelCommandPayload{RequestID: "req-1"})

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
	c.registerActiveCommand("other", otherCancel)
	defer c.unregisterActiveCommand("other")

	c.handleCancelCommand(cancelCommandPayload{RequestID: "missing"})

	select {
	case <-otherCtx.Done():
		t.Fatalf("cancel for unknown request canceled an unrelated command")
	default:
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
