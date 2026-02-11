package hostagent

import (
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
			name:     "http becomes ws",
			pulseURL: "http://example.invalid",
			want:     "ws://example.invalid/api/agent/ws",
		},
		{
			name:     "preserves path prefix",
			pulseURL: "https://example.invalid/pulse/",
			want:     "wss://example.invalid/pulse/api/agent/ws",
		},
		{
			name:     "ws preserved",
			pulseURL: "ws://example.invalid",
			want:     "ws://example.invalid/api/agent/ws",
		},
		{
			name:     "wss preserved",
			pulseURL: "wss://example.invalid",
			want:     "wss://example.invalid/api/agent/ws",
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
