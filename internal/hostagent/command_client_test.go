package hostagent

import (
	"context"
	"io"
	"testing"

	"github.com/rs/zerolog"
)

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
