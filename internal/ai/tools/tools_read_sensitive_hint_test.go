package tools

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func readExecResult(t *testing.T, command string) string {
	t.Helper()
	t.Setenv("PULSE_STRICT_RESOLUTION", "false")
	agentSrv := &mockAgentServer{agents: []agentexec.ConnectedAgent{{AgentID: "agent1", Hostname: "docker01", Platform: "linux"}}}
	exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
	result, err := exec.executeReadExec(context.Background(), map[string]interface{}{
		"command":     command,
		"target_host": "docker01",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	agentSrv.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything, mock.Anything)
	return result.Content[0].Text
}

// A refused wildcard says what to ask instead. Without it, Patrol sized three
// folders by name and stopped one level above the folder that filled the disk.
func TestWildcardRefusalNamesAFormThatWorks(t *testing.T) {
	for _, command := range []string{
		"du -sh /mnt/nvr/* 2>/dev/null | sort -rh | head -20",
		"du -sh /mnt/nvr/*",
	} {
		text := readExecResult(t, command)
		assert.Contains(t, text, "SENSITIVE_COMMAND", command)
		assert.Contains(t, text, "du -xh -d 3 \\u003cdirectory\\u003e | sort -rh | head -20", command)
	}

	// Other refusals have no wildcard to replace and get no hint.
	text := readExecResult(t, "cat /var/lib/pulse-agent/token")
	assert.Contains(t, text, "SENSITIVE_COMMAND")
	assert.NotContains(t, text, "\"hint\"")
}

// The hint is only worth giving if what it suggests passes the same policy.
func TestWildcardRefusalHintPassesThePolicy(t *testing.T) {
	forms := regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(WildcardRefusalHint, -1)
	require.Len(t, forms, 2)
	for _, form := range forms {
		for _, dir := range []string{"/mnt/nvr", "/mnt/nvr/frigate/recordings", "/srv/media"} {
			command := strings.ReplaceAll(form[1], "<directory>", dir)
			for _, depth := range []string{"", "6"} {
				if depth != "" {
					if !strings.Contains(command, "-d 3") {
						continue
					}
					command = strings.Replace(command, "-d 3", "-d "+depth, 1)
				}
				if sensitive, reason := safety.CommandTouchesSensitivePath(command); sensitive {
					t.Fatalf("%q is refused as sensitive: %s", command, reason)
				}
				if intent := ClassifyExecutionIntent(command); intent.Intent == IntentWriteOrUnknown {
					t.Fatalf("%q is refused as not read-only: %s", command, intent.Reason)
				}
			}
		}
	}
}
