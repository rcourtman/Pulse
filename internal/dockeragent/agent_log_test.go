package dockeragent

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestCheckForUpdatesLog(t *testing.T) {
	// Create a buffer to capture logs
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	// Create an agent with DisableAutoUpdate set to true
	agent := &Agent{
		cfg: Config{
			DisableAutoUpdate: true,
		},
		logger: logger,
	}

	// Call checkForUpdates
	agent.checkForUpdates(context.Background())

	// Check logs
	output := buf.String()
	expected := "Skipping update check - auto-update disabled"
	if !strings.Contains(output, expected) {
		t.Errorf("expected log message %q not found in output:\n%s", expected, output)
	}

	// Verify log level is INFO (zerolog default level is debug? no, default global is info, but logger.Info() writes regardless)
	// Zerolog JSON output contains "level":"info"
	if !strings.Contains(output, `"level":"info"`) {
		t.Errorf("expected log level info, got output:\n%s", output)
	}
}
