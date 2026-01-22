package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateApprovalRecord(t *testing.T) {
	approval.SetStore(nil)
	assert.Empty(t, createApprovalRecord("ls", "host", "h1", "host1", "ctx"))

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	require.NoError(t, err)
	approval.SetStore(store)
	defer approval.SetStore(nil)

	approvalID := createApprovalRecord("ls", "host", "h1", "host1", "ctx")
	require.NotEmpty(t, approvalID)

	req, ok := store.GetApproval(approvalID)
	require.True(t, ok)
	assert.Equal(t, "ls", req.Command)
	assert.Equal(t, "host", req.TargetType)
	assert.Equal(t, "h1", req.TargetID)
	assert.Equal(t, "host1", req.TargetName)
	assert.Equal(t, "ctx", req.Context)
}

func TestIsPreApproved(t *testing.T) {
	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	require.NoError(t, err)
	approval.SetStore(store)
	defer approval.SetStore(nil)

	assert.False(t, isPreApproved(map[string]interface{}{}))
	assert.False(t, isPreApproved(map[string]interface{}{"_approval_id": "missing"}))

	req := &approval.ApprovalRequest{
		ID:      "app-1",
		Command: "ls",
	}
	require.NoError(t, store.CreateApproval(req))

	assert.False(t, isPreApproved(map[string]interface{}{"_approval_id": "app-1"}))

	_, err = store.Approve("app-1", "tester")
	require.NoError(t, err)
	assert.True(t, isPreApproved(map[string]interface{}{"_approval_id": "app-1"}))
}

func TestFormattingHelpers(t *testing.T) {
	t.Run("formatApprovalNeeded", func(t *testing.T) {
		payload := decodePayload(t, formatApprovalNeeded("ls", "ok", "id-1"), "APPROVAL_REQUIRED: ")
		assert.Equal(t, "approval_required", payload["type"])
		assert.Equal(t, "ls", payload["command"])
		assert.Equal(t, "ok", payload["reason"])
		assert.Equal(t, "id-1", payload["approval_id"])
	})

	t.Run("formatPolicyBlocked", func(t *testing.T) {
		payload := decodePayload(t, formatPolicyBlocked("rm -rf /", "blocked"), "POLICY_BLOCKED: ")
		assert.Equal(t, "policy_blocked", payload["type"])
		assert.Equal(t, "rm -rf /", payload["command"])
		assert.Equal(t, "blocked", payload["reason"])
	})

	t.Run("formatTargetHostRequired", func(t *testing.T) {
		agents := []agentexec.ConnectedAgent{
			{Hostname: "node-1"},
			{AgentID: "agent-2"},
			{Hostname: "node-3"},
			{Hostname: "node-4"},
			{Hostname: "node-5"},
			{Hostname: "node-6"},
			{Hostname: "node-7"},
		}
		msg := formatTargetHostRequired(agents)
		assert.Contains(t, msg, "Available: node-1, agent-2, node-3, node-4, node-5, node-6")
		assert.Contains(t, msg, "(+1 more)")
	})

	t.Run("formatControlApprovalNeeded", func(t *testing.T) {
		payload := decodePayload(t, formatControlApprovalNeeded("vm1", 101, "start", "qm start 101", "id-2"), "APPROVAL_REQUIRED: ")
		assert.Equal(t, "approval_required", payload["type"])
		assert.Equal(t, "vm1", payload["guest_name"])
		assert.Equal(t, float64(101), payload["guest_vmid"])
		assert.Equal(t, "start", payload["action"])
		assert.Equal(t, "qm start 101", payload["command"])
		assert.Equal(t, "id-2", payload["approval_id"])
	})

	t.Run("formatDockerApprovalNeeded", func(t *testing.T) {
		payload := decodePayload(t, formatDockerApprovalNeeded("web", "node-1", "restart", "docker restart web", "id-3"), "APPROVAL_REQUIRED: ")
		assert.Equal(t, "approval_required", payload["type"])
		assert.Equal(t, "web", payload["container_name"])
		assert.Equal(t, "node-1", payload["docker_host"])
		assert.Equal(t, "restart", payload["action"])
		assert.Equal(t, "docker restart web", payload["command"])
		assert.Equal(t, "id-3", payload["approval_id"])
	})
}

func decodePayload(t *testing.T, value, prefix string) map[string]interface{} {
	t.Helper()
	require.True(t, strings.HasPrefix(value, prefix))

	var payload map[string]interface{}
	raw := strings.TrimPrefix(value, prefix)
	err := json.Unmarshal([]byte(raw), &payload)
	require.NoError(t, err)
	return payload
}
