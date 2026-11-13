package tools

import (
	"context"
	"errors"
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
		assert.Contains(t, text, "du -xh -d 3 \\u003cdirectory\\u003e", command)
	}

	// Other refusals have no wildcard to replace and get no hint.
	text := readExecResult(t, "cat /var/lib/pulse-agent/token")
	assert.Contains(t, text, "SENSITIVE_COMMAND")
	assert.NotContains(t, text, "\"hint\"")
}

// The hint is only worth giving if what it suggests runs. That means three
// gates: the sensitive-path check, the read-only classifier, and the agent's
// own command policy, which never auto-approves a compound command. A first
// version suggested a pipe, passed the first two, and sent Patrol to
// "command requires approval" four times.
func TestWildcardRefusalHintPassesThePolicy(t *testing.T) {
	policy := agentexec.DefaultPolicy()
	forms := regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(WildcardRefusalHint+" "+SingleCommandHint, -1)
	require.Len(t, forms, 3)
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
				if !policy.IsAutoApproved(command) {
					t.Fatalf("%q needs approval under the agent's command policy (%s)", command, policy.Evaluate(command))
				}
			}
		}
	}
}

// pulse_read never sends an approval, so a command the agent will not
// auto-approve can never run through it. The refusal says why and what to run
// instead; a bare "command requires approval" cost Patrol three of its ten
// evidence calls on one investigation.
func TestApprovalRefusalNamesASingleCommand(t *testing.T) {
	t.Setenv("PULSE_STRICT_RESOLUTION", "false")
	run := func(t *testing.T, err error) string {
		t.Helper()
		agentSrv := &mockAgentServer{}
		agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{{AgentID: "agent1", Hostname: "docker01", Platform: "linux"}})
		agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.Anything).Return((*agentexec.CommandResultPayload)(nil), err)
		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
		result, callErr := exec.executeReadExec(context.Background(), map[string]interface{}{
			"command":     "du -xh -d 3 /mnt/nvr | sort -rh | head -20",
			"target_host": "docker01",
		})
		require.NoError(t, callErr)
		require.NotEmpty(t, result.Content)
		return result.Content[0].Text
	}

	text := run(t, agentexec.ErrCommandRequiresApproval)
	assert.Contains(t, text, "APPROVAL_REQUIRED")
	assert.Contains(t, text, "Run each part as its own command")
	assert.Contains(t, text, "du -xh -d 3 \\u003cdirectory\\u003e")

	// Any other failure is reported as it is, with no hint about approval.
	text = run(t, errors.New("agent disconnected"))
	assert.Contains(t, text, "command execution failed: agent disconnected")
	assert.NotContains(t, text, "APPROVAL_REQUIRED")
}
