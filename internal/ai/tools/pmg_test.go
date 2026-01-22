package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteGetPMGStatus(t *testing.T) {
	ctx := context.Background()

	exec := NewPulseToolExecutor(ExecutorConfig{})
	result, err := exec.executeGetPMGStatus(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "State provider not available.", result.Content[0].Text)

	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
	result, err = exec.executeGetPMGStatus(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "No Proxmox Mail Gateway instances found. PMG monitoring may not be configured.", result.Content[0].Text)

	state := models.StateSnapshot{
		PMGInstances: []models.PMGInstance{
			{
				ID:      "pmg1",
				Name:    "gateway-1",
				Host:    "pmg.local",
				Status:  "online",
				Version: "7.4",
				Nodes: []models.PMGNodeStatus{
					{Name: "node1", Status: "online", Role: "master", Uptime: 100, LoadAvg: "0.10"},
				},
			},
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})

	result, err = exec.executeGetPMGStatus(ctx, map[string]interface{}{
		"instance": "missing",
	})
	require.NoError(t, err)
	assert.Equal(t, "PMG instance 'missing' not found.", result.Content[0].Text)

	result, err = exec.executeGetPMGStatus(ctx, map[string]interface{}{})
	require.NoError(t, err)

	var resp PMGStatusResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Instances, 1)
	assert.Equal(t, "gateway-1", resp.Instances[0].Name)
	assert.Equal(t, 1, resp.Total)
}

func TestExecuteGetMailStats(t *testing.T) {
	ctx := context.Background()

	exec := NewPulseToolExecutor(ExecutorConfig{})
	result, err := exec.executeGetMailStats(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "State provider not available.", result.Content[0].Text)

	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
	result, err = exec.executeGetMailStats(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "No Proxmox Mail Gateway instances found. PMG monitoring may not be configured.", result.Content[0].Text)

	state := models.StateSnapshot{
		PMGInstances: []models.PMGInstance{
			{ID: "pmg1", Name: "gateway-1"},
			{
				ID:   "pmg2",
				Name: "gateway-2",
				MailStats: &models.PMGMailStats{
					Timeframe:            "24h",
					CountIn:              5,
					CountOut:             6,
					SpamIn:               1,
					VirusIn:              2,
					BouncesIn:            1,
					BytesIn:              12,
					BytesOut:             34,
					GreylistCount:        2,
					RBLRejects:           3,
					AverageProcessTimeMs: 50,
				},
			},
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})

	result, err = exec.executeGetMailStats(ctx, map[string]interface{}{
		"instance": "pmg1",
	})
	require.NoError(t, err)
	assert.Equal(t, "No mail statistics available for PMG instance 'pmg1'.", result.Content[0].Text)

	result, err = exec.executeGetMailStats(ctx, map[string]interface{}{
		"instance": "missing",
	})
	require.NoError(t, err)
	assert.Equal(t, "PMG instance 'missing' not found.", result.Content[0].Text)

	result, err = exec.executeGetMailStats(ctx, map[string]interface{}{
		"instance": "gateway-2",
	})
	require.NoError(t, err)

	var resp MailStatsResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.Equal(t, "gateway-2", resp.Instance)
	assert.Equal(t, 5.0, resp.Stats.TotalIn)
	assert.Equal(t, 6.0, resp.Stats.TotalOut)
}

func TestExecuteGetMailQueues(t *testing.T) {
	ctx := context.Background()

	exec := NewPulseToolExecutor(ExecutorConfig{})
	result, err := exec.executeGetMailQueues(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "State provider not available.", result.Content[0].Text)

	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
	result, err = exec.executeGetMailQueues(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "No Proxmox Mail Gateway instances found. PMG monitoring may not be configured.", result.Content[0].Text)

	state := models.StateSnapshot{
		PMGInstances: []models.PMGInstance{
			{ID: "pmg1", Name: "gateway-1"},
			{
				ID:   "pmg2",
				Name: "gateway-2",
				Nodes: []models.PMGNodeStatus{
					{
						Name: "node1",
						QueueStatus: &models.PMGQueueStatus{
							Active: 1, Deferred: 2, Hold: 3, Incoming: 4, Total: 10, OldestAge: 20,
						},
					},
				},
			},
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})

	result, err = exec.executeGetMailQueues(ctx, map[string]interface{}{
		"instance": "pmg1",
	})
	require.NoError(t, err)
	assert.Equal(t, "No queue data available for PMG instance 'pmg1'.", result.Content[0].Text)

	result, err = exec.executeGetMailQueues(ctx, map[string]interface{}{
		"instance": "missing",
	})
	require.NoError(t, err)
	assert.Equal(t, "PMG instance 'missing' not found.", result.Content[0].Text)

	result, err = exec.executeGetMailQueues(ctx, map[string]interface{}{
		"instance": "gateway-2",
	})
	require.NoError(t, err)

	var resp MailQueuesResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Queues, 1)
	assert.Equal(t, "node1", resp.Queues[0].Node)
}

func TestExecuteGetSpamStats(t *testing.T) {
	ctx := context.Background()

	exec := NewPulseToolExecutor(ExecutorConfig{})
	result, err := exec.executeGetSpamStats(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "State provider not available.", result.Content[0].Text)

	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
	result, err = exec.executeGetSpamStats(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "No Proxmox Mail Gateway instances found. PMG monitoring may not be configured.", result.Content[0].Text)

	state := models.StateSnapshot{
		PMGInstances: []models.PMGInstance{
			{
				ID:   "pmg1",
				Name: "gateway-1",
				Quarantine: &models.PMGQuarantineTotals{
					Spam: 1, Virus: 2, Attachment: 3, Blacklisted: 4,
				},
				SpamDistribution: []models.PMGSpamBucket{
					{Score: "5-6", Count: 10},
				},
			},
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})

	result, err = exec.executeGetSpamStats(ctx, map[string]interface{}{
		"instance": "missing",
	})
	require.NoError(t, err)
	assert.Equal(t, "PMG instance 'missing' not found.", result.Content[0].Text)

	result, err = exec.executeGetSpamStats(ctx, map[string]interface{}{
		"instance": "gateway-1",
	})
	require.NoError(t, err)

	var resp SpamStatsResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.Equal(t, "gateway-1", resp.Instance)
	assert.Equal(t, 10, resp.Quarantine.Total)
	assert.Len(t, resp.Distribution, 1)
	assert.Equal(t, "5-6", resp.Distribution[0].Score)
}

func TestExecuteGetSpamStatsEmpty(t *testing.T) {
	ctx := context.Background()
	state := models.StateSnapshot{
		PMGInstances: []models.PMGInstance{
			{ID: "pmg1", Name: "gateway-1"},
		},
	}

	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err := exec.executeGetSpamStats(ctx, map[string]interface{}{
		"instance": "gateway-1",
	})
	require.NoError(t, err)

	var resp SpamStatsResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.Equal(t, "gateway-1", resp.Instance)
	assert.Equal(t, 0, resp.Quarantine.Total)
	assert.Len(t, resp.Distribution, 0)
}

func TestExecuteGetMailStatsFallback(t *testing.T) {
	ctx := context.Background()
	state := models.StateSnapshot{
		PMGInstances: []models.PMGInstance{
			{ID: "pmg1", Name: "gateway-1"},
		},
	}

	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err := exec.executeGetMailStats(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "No mail statistics available from any PMG instance.", result.Content[0].Text)
}

func TestExecuteGetMailQueuesFallback(t *testing.T) {
	ctx := context.Background()
	state := models.StateSnapshot{
		PMGInstances: []models.PMGInstance{
			{ID: "pmg1", Name: "gateway-1"},
		},
	}

	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err := exec.executeGetMailQueues(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "No mail queue data available from any PMG instance.", result.Content[0].Text)
}

func TestExecuteGetMailStatsUsesFirstInstance(t *testing.T) {
	ctx := context.Background()
	state := models.StateSnapshot{
		PMGInstances: []models.PMGInstance{
			{
				ID:   "pmg1",
				Name: "gateway-1",
				MailStats: &models.PMGMailStats{
					Timeframe: "24h",
					CountIn:   3,
					CountOut:  4,
				},
			},
			{
				ID:   "pmg2",
				Name: "gateway-2",
				MailStats: &models.PMGMailStats{
					Timeframe: "24h",
					CountIn:   5,
					CountOut:  6,
				},
			},
		},
	}

	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err := exec.executeGetMailStats(ctx, map[string]interface{}{})
	require.NoError(t, err)

	var resp MailStatsResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.Equal(t, "gateway-1", resp.Instance)
	assert.Equal(t, 3.0, resp.Stats.TotalIn)
}

func TestExecuteGetPMGStatusNodesEmpty(t *testing.T) {
	ctx := context.Background()
	state := models.StateSnapshot{
		PMGInstances: []models.PMGInstance{
			{ID: "pmg1", Name: "gateway-1"},
		},
	}

	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err := exec.executeGetPMGStatus(ctx, map[string]interface{}{})
	require.NoError(t, err)

	var resp PMGStatusResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Instances, 1)
	assert.Len(t, resp.Instances[0].Nodes, 0)
}

func TestExecuteGetMailQueuesUsesFirstInstance(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	state := models.StateSnapshot{
		PMGInstances: []models.PMGInstance{
			{
				ID:   "pmg1",
				Name: "gateway-1",
				Nodes: []models.PMGNodeStatus{
					{
						Name: "node1",
						QueueStatus: &models.PMGQueueStatus{
							Active: 1, Deferred: 0, Hold: 0, Incoming: 0, Total: 1, OldestAge: 5, UpdatedAt: now,
						},
					},
				},
			},
		},
	}

	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err := exec.executeGetMailQueues(ctx, map[string]interface{}{})
	require.NoError(t, err)

	var resp MailQueuesResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.Equal(t, "gateway-1", resp.Instance)
	require.Len(t, resp.Queues, 1)
}
