package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// duTableAgent answers du commands from a fixed table and records them.
type duTableAgent struct {
	t        *testing.T
	outputs  map[string]agentexec.CommandResultPayload
	commands []string
}

func (a *duTableAgent) GetConnectedAgents() []agentexec.ConnectedAgent {
	return []agentexec.ConnectedAgent{{AgentID: "agent1", Hostname: "docker01", Platform: "linux"}}
}

func (a *duTableAgent) ExecuteCommand(_ context.Context, _ string, payload agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
	a.commands = append(a.commands, payload.Command)
	out, ok := a.outputs[payload.Command]
	if !ok {
		a.t.Errorf("unexpected command %q", payload.Command)
		return &agentexec.CommandResultPayload{ExitCode: 1}, nil
	}
	return &out, nil
}

func diskUsageAgent(t *testing.T, outputs map[string]agentexec.CommandResultPayload) (*PulseToolExecutor, *[]string) {
	t.Helper()
	t.Setenv("PULSE_STRICT_RESOLUTION", "false")
	agent := &duTableAgent{t: t, outputs: outputs}
	return NewPulseToolExecutor(ExecutorConfig{AgentServer: agent}), &agent.commands
}

func runDiskUsage(t *testing.T, exec *PulseToolExecutor, dir string) CallToolResult {
	t.Helper()
	result, err := exec.executeRead(context.Background(), map[string]interface{}{
		"action":      "disk_usage",
		"target_host": "docker01",
		"path":        dir,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	return result
}

// The fault set's D6 disk, as du saw it on 15 November 2026. Patrol drilled it
// one level per call and concluded at .../2026-11-15/02, one folder above the
// camera folder. disk_usage reaches driveway in one tool call.
func TestDiskUsageFollowsTheLargestFolderToTheOneThatHoldsTheSpace(t *testing.T) {
	exec, commands := diskUsageAgent(t, map[string]agentexec.CommandResultPayload{
		"du -x -k -d 1 /mnt/nvr": {
			Stdout:   "31744\t/mnt/nvr/photos\n262144\t/mnt/nvr/frigate\n16\t/mnt/nvr/lost+found\n21504\t/mnt/nvr/music\n313344\t/mnt/nvr\n",
			Stderr:   "du: cannot read directory '/mnt/nvr/lost+found': Permission denied\n",
			ExitCode: 1,
		},
		"du -x -k -d 1 /mnt/nvr/frigate": {
			Stdout: "4\t/mnt/nvr/frigate/clips\n262140\t/mnt/nvr/frigate/recordings\n262144\t/mnt/nvr/frigate\n",
		},
		"du -x -k -d 1 /mnt/nvr/frigate/recordings": {
			Stdout: "246784\t/mnt/nvr/frigate/recordings/2026-11-15\n16384\t/mnt/nvr/frigate/recordings/2026-11-08\n262140\t/mnt/nvr/frigate/recordings\n",
		},
		"du -x -k -d 1 /mnt/nvr/frigate/recordings/2026-11-15": {
			Stdout: "246780\t/mnt/nvr/frigate/recordings/2026-11-15/02\n246784\t/mnt/nvr/frigate/recordings/2026-11-15\n",
		},
		"du -x -k -d 1 /mnt/nvr/frigate/recordings/2026-11-15/02": {
			Stdout: "246776\t/mnt/nvr/frigate/recordings/2026-11-15/02/driveway\n246780\t/mnt/nvr/frigate/recordings/2026-11-15/02\n",
		},
		"du -x -k -d 1 /mnt/nvr/frigate/recordings/2026-11-15/02/driveway": {
			Stdout: "246776\t/mnt/nvr/frigate/recordings/2026-11-15/02/driveway\n",
		},
	})

	result := runDiskUsage(t, exec, "/mnt/nvr/")
	text := result.Content[0].Text
	assert.False(t, result.IsError)
	assert.Contains(t, text, "Largest folder: /mnt/nvr/frigate/recordings/2026-11-15/02/driveway holds 241M, 79% of /mnt/nvr.")
	assert.Contains(t, text, "It has no subfolders: its own files hold the space.")
	assert.Contains(t, text, "/mnt/nvr/frigate 256M, 84% of nvr (beside it: photos 31M, music 21M, lost+found 16K)")
	assert.Contains(t, text, "du could not read 1 path(s)")
	assert.Len(t, *commands, 6)

	// Every level runs without approval under the agent's own policy.
	policy := agentexec.DefaultPolicy()
	for _, command := range *commands {
		assert.True(t, policy.IsAutoApproved(command), "%s: %s", command, policy.Evaluate(command))
	}
}

// When the space is spread out, disk_usage stops at the folder above the
// spread and lists what is below it rather than picking one.
func TestDiskUsageStopsWhereNoSingleFolderHoldsMost(t *testing.T) {
	exec, commands := diskUsageAgent(t, map[string]agentexec.CommandResultPayload{
		"du -x -k -d 1 /srv": {
			Stdout: "1048576\t/srv/media\n10240\t/srv/backup\n1058816\t/srv\n",
		},
		"du -x -k -d 1 /srv/media": {
			Stdout: "409600\t/srv/media/films\n307200\t/srv/media/series\n204800\t/srv/media/music\n1048576\t/srv/media\n",
		},
	})

	text := runDiskUsage(t, exec, "/srv").Content[0].Text
	assert.Contains(t, text, "Largest folder: /srv/media holds 1.0G, 99% of /srv. No single folder below it holds most of its space.")
	assert.Contains(t, text, "Largest below it: films 400M, series 300M, music 200M; files directly in it 124M.")
	assert.Len(t, *commands, 2)
}

// A folder name with a space is quoted, and the quoted form still runs
// without approval.
func TestDiskUsageQuotesPathsTheAgentPolicyStillApproves(t *testing.T) {
	exec, commands := diskUsageAgent(t, map[string]agentexec.CommandResultPayload{
		"du -x -k -d 1 /data": {
			Stdout: "900\t/data/Camera Uploads\n1000\t/data\n",
		},
		"du -x -k -d 1 '/data/Camera Uploads'": {
			Stdout: "900\t/data/Camera Uploads\n",
		},
	})

	text := runDiskUsage(t, exec, "/data").Content[0].Text
	assert.Contains(t, text, "Largest folder: /data/Camera Uploads holds 900K, 90% of /data.")
	require.Len(t, *commands, 2)
	assert.True(t, agentexec.DefaultPolicy().IsAutoApproved((*commands)[1]))
}

// disk_usage goes through the same gates as exec: a sensitive directory is
// refused before the agent is asked, and a pattern is not a directory.
func TestDiskUsageKeepsTheReadGates(t *testing.T) {
	exec, commands := diskUsageAgent(t, nil)

	text := runDiskUsage(t, exec, "/etc/pve/priv").Content[0].Text
	assert.Contains(t, text, "SENSITIVE_COMMAND")

	result := runDiskUsage(t, exec, "/mnt/nvr/*")
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "not a pattern")

	result = runDiskUsage(t, exec, "mnt/nvr")
	assert.True(t, result.IsError)
	assert.Empty(t, *commands)
}

func TestFormatKiBReadsLikeDuH(t *testing.T) {
	for kb, want := range map[int64]string{
		16:         "16K",
		4:          "4K",
		21504:      "21M",
		9216:       "9.0M",
		1048576:    "1.0G",
		246776:     "241M",
		5368709120: "5.0T",
	} {
		assert.Equal(t, want, formatKiB(kb), kb)
	}
	assert.False(t, strings.Contains(formatKiB(0), "."))
}
