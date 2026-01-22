package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatDockerUpdateApprovalNeeded(t *testing.T) {
	value := formatDockerUpdateApprovalNeeded("web", "node-1", "id-1")
	require.True(t, strings.HasPrefix(value, "APPROVAL_REQUIRED: "))

	raw := strings.TrimPrefix(value, "APPROVAL_REQUIRED: ")
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))

	assert.Equal(t, "approval_required", payload["type"])
	assert.Equal(t, "id-1", payload["approval_id"])
	assert.Equal(t, "web", payload["container_name"])
	assert.Equal(t, "node-1", payload["docker_host"])
	assert.Equal(t, "update", payload["action"])
}

func TestTrimLeadingSlash(t *testing.T) {
	assert.Equal(t, "name", trimLeadingSlash("/name"))
	assert.Equal(t, "name", trimLeadingSlash("name"))
	assert.Equal(t, "", trimLeadingSlash(""))
}
