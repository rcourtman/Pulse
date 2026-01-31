package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractFacts_UnknownTool(t *testing.T) {
	facts := ExtractFacts("unknown_tool", nil, `{}`)
	assert.Empty(t, facts)
}

func TestExtractFacts_InvalidJSON(t *testing.T) {
	facts := ExtractFacts("pulse_query", map[string]interface{}{"action": "get"}, "not json")
	assert.Empty(t, facts)
}

func TestExtractFacts_QueryGet(t *testing.T) {
	input := map[string]interface{}{"action": "get", "resource_type": "lxc"}
	// Actual format from NewJSONResult(ResourceResponse): direct JSON, no wrapper.
	// CPU/Memory are nested structs.
	result := `{"type":"lxc","name":"postfix-server","status":"running","node":"delly","id":"lxc/106","vmid":106,"cpu":{"percent":2.5,"cores":4},"memory":{"percent":45.0,"used_gb":1.2,"total_gb":4.0}}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Equal(t, FactCategoryResource, f.Category)
	assert.Equal(t, "lxc:delly:lxc/106:status", f.Key)
	assert.Contains(t, f.Value, "running")
	assert.Contains(t, f.Value, "postfix-server")
	assert.Contains(t, f.Value, "CPU=2.5%")
	assert.Contains(t, f.Value, "Mem=45.0%")
}

func TestExtractFacts_QueryGet_NotFound(t *testing.T) {
	input := map[string]interface{}{"action": "get", "resource_type": "vm"}
	result := `{"error":"not_found","resource_id":"999","type":"vm"}`

	facts := ExtractFacts("pulse_query", input, result)
	assert.Empty(t, facts)
}

func TestExtractFacts_QueryGet_NoCPU(t *testing.T) {
	// Some resources may not have CPU data populated
	input := map[string]interface{}{"action": "get", "resource_type": "container"}
	result := `{"type":"container","name":"test-ct","status":"stopped","node":"minipc","id":"lxc/200","cpu":{"percent":0},"memory":{"percent":0}}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 1)
	assert.Contains(t, facts[0].Value, "stopped")
	assert.Contains(t, facts[0].Value, "test-ct")
	assert.NotContains(t, facts[0].Value, "CPU=")
}

func TestExtractFacts_QuerySearch(t *testing.T) {
	input := map[string]interface{}{"action": "search", "query": "postfix"}
	result := `{"query":"postfix","matches":[{"name":"postfix-lxc","status":"running","type":"lxc"},{"name":"mail-server","status":"stopped","type":"vm"}],"total":2}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Equal(t, FactCategoryResource, f.Category)
	assert.Equal(t, "search:postfix:summary", f.Key)
	assert.Contains(t, f.Value, "2 results")
	assert.Contains(t, f.Value, "postfix-lxc (running)")
	assert.Contains(t, f.Value, "mail-server (stopped)")
}

func TestExtractFacts_QuerySearch_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "search", "query": "nonexistent"}
	result := `{"query":"nonexistent","matches":[],"total":0}`

	facts := ExtractFacts("pulse_query", input, result)
	assert.Empty(t, facts)
}

func TestExtractFacts_StoragePools(t *testing.T) {
	input := map[string]interface{}{"action": "pools"}
	result := `{"pools":[{"name":"pbs-minipc","node":"","nodes":["delly","minipc"],"type":"PBS","status":"available","active":true,"usage_percent":42.7,"total_gb":1000,"used_gb":427}]}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Equal(t, FactCategoryStorage, f.Category)
	assert.Contains(t, f.Key, "storage:")
	assert.Contains(t, f.Key, "pbs-minipc")
	assert.Contains(t, f.Value, "PBS")
	assert.Contains(t, f.Value, "42.7% used")
	assert.Contains(t, f.Value, "573GB free")
}

func TestExtractFacts_BackupTasks_OnlyFailures(t *testing.T) {
	input := map[string]interface{}{"action": "backup_tasks"}
	result := `{"tasks":[{"vmid":"106","node":"delly","status":"ok"},{"vmid":"200","node":"minipc","status":"failed","start_time":"2024-01-15T03:00","error":"snapshot failed"}]}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Equal(t, FactCategoryStorage, f.Category)
	assert.Equal(t, "backup:200:minipc", f.Key)
	assert.Contains(t, f.Value, "failed")
	assert.Contains(t, f.Value, "snapshot failed")
}

func TestExtractFacts_Discovery(t *testing.T) {
	input := map[string]interface{}{"host": "delly", "resource_id": "106"}
	result := `{"service_type":"Postfix","hostname":"patrol-signal-test","host_id":"delly","resource_id":"106","ports":[{"port":25},{"port":22}]}`

	facts := ExtractFacts("pulse_discovery", input, result)
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Equal(t, FactCategoryDiscovery, f.Category)
	assert.Equal(t, "discovery:delly:106", f.Key)
	assert.Contains(t, f.Value, "service=Postfix")
	assert.Contains(t, f.Value, "hostname=patrol-signal-test")
	assert.Contains(t, f.Value, "ports=[25,22]")
}

func TestExtractFacts_Exec_JSON(t *testing.T) {
	input := map[string]interface{}{"command": "pvesm status | grep pbs-minipc", "target_host": "delly"}
	result := `{"success":true,"exit_code":0,"output":"pbs-minipc    active   42.68%"}`

	facts := ExtractFacts("pulse_read", input, result)
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Equal(t, FactCategoryExec, f.Category)
	assert.Contains(t, f.Key, "exec:delly:")
	assert.Contains(t, f.Value, "exit=0")
	assert.Contains(t, f.Value, "pbs-minipc")
}

func TestExtractFacts_Exec_FallbackRaw(t *testing.T) {
	input := map[string]interface{}{"command": "some-cmd", "target_host": "host1"}
	result := `not json at all`

	facts := ExtractFacts("pulse_read", input, result)
	require.Len(t, facts, 1)
	assert.Contains(t, facts[0].Value, "not json at all")
}

func TestExtractFacts_Metrics(t *testing.T) {
	input := map[string]interface{}{"action": "performance", "resource_id": "vm101"}
	// Actual format: summary is map[string]ResourceMetricsSummary keyed by resource ID
	result := `{"resource_id":"vm101","period":"7d","summary":{"vm101":{"resource_id":"vm101","avg_cpu":12.3,"max_cpu":78.5,"avg_memory":65.0,"max_memory":89.0,"trend":"growing"}}}`

	facts := ExtractFacts("pulse_metrics", input, result)
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Equal(t, FactCategoryMetrics, f.Category)
	assert.Equal(t, "metrics:vm101", f.Key)
	assert.Contains(t, f.Value, "avg_cpu=12.3%")
	assert.Contains(t, f.Value, "max=78.5%")
	assert.Contains(t, f.Value, "trend=growing")
}

func TestExtractFacts_Metrics_EmptySummary(t *testing.T) {
	input := map[string]interface{}{"action": "performance", "resource_id": "vm101"}
	result := `{"resource_id":"vm101","period":"7d","summary":{}}`

	facts := ExtractFacts("pulse_metrics", input, result)
	assert.Empty(t, facts)
}

func TestExtractFacts_Finding(t *testing.T) {
	input := map[string]interface{}{
		"key":         "high-cpu-vm101",
		"severity":    "warning",
		"title":       "High CPU usage on vm101",
		"resource_id": "vm101",
	}
	result := `{"id":"abc123","status":"created"}`

	facts := ExtractFacts("patrol_report_finding", input, result)
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Equal(t, FactCategoryFinding, f.Category)
	assert.Equal(t, "finding:high-cpu-vm101", f.Key)
	assert.Contains(t, f.Value, "warning")
	assert.Contains(t, f.Value, "High CPU usage on vm101")
	assert.Contains(t, f.Value, "on vm101")
}

func TestExtractFacts_Finding_MissingFields(t *testing.T) {
	// Missing key and title should return empty
	input := map[string]interface{}{"severity": "warning"}
	result := `{"id":"abc123"}`

	facts := ExtractFacts("patrol_report_finding", input, result)
	assert.Empty(t, facts)
}

func TestPredictFactKeys_Discovery(t *testing.T) {
	keys := PredictFactKeys("pulse_discovery", map[string]interface{}{
		"host_id":     "delly",
		"resource_id": "106",
	})
	require.Len(t, keys, 1)
	assert.Equal(t, "discovery:delly:106", keys[0])
}

func TestPredictFactKeys_DiscoveryAltFields(t *testing.T) {
	keys := PredictFactKeys("pulse_discovery", map[string]interface{}{
		"host":        "minipc",
		"resource_id": "pbs-minipc",
	})
	require.Len(t, keys, 1)
	assert.Equal(t, "discovery:minipc:pbs-minipc", keys[0])
}

func TestPredictFactKeys_Exec(t *testing.T) {
	keys := PredictFactKeys("pulse_read", map[string]interface{}{
		"target_host": "delly",
		"command":     "pvesm status",
	})
	require.Len(t, keys, 1)
	assert.Equal(t, "exec:delly:pvesm status", keys[0])
}

func TestPredictFactKeys_Metrics(t *testing.T) {
	keys := PredictFactKeys("pulse_metrics", map[string]interface{}{
		"action":      "performance",
		"resource_id": "vm101",
	})
	require.Len(t, keys, 1)
	assert.Equal(t, "metrics:vm101", keys[0])
}

func TestPredictFactKeys_UnpredictableTools(t *testing.T) {
	// pulse_query and pulse_storage keys depend on result data
	assert.Nil(t, PredictFactKeys("pulse_query", map[string]interface{}{"action": "get"}))
	assert.Nil(t, PredictFactKeys("pulse_storage", map[string]interface{}{"action": "pools"}))
	assert.Nil(t, PredictFactKeys("unknown_tool", nil))
}

func TestPredictFactKeys_MissingFields(t *testing.T) {
	// Discovery without host_id should return nil
	assert.Nil(t, PredictFactKeys("pulse_discovery", map[string]interface{}{"resource_id": "106"}))
	// Exec without command should return nil
	assert.Nil(t, PredictFactKeys("pulse_read", map[string]interface{}{"target_host": "delly"}))
}

func TestPredictFactKeys_FileReadDistinctPaths(t *testing.T) {
	// Different file paths on the same host should produce different keys
	keys1 := PredictFactKeys("pulse_read", map[string]interface{}{
		"target_host": "delly",
		"action":      "file",
		"path":        "/etc/pve/storage.cfg",
	})
	keys2 := PredictFactKeys("pulse_read", map[string]interface{}{
		"target_host": "delly",
		"action":      "file",
		"path":        "/var/log/pve/tasks/some-task-log",
	})
	require.Len(t, keys1, 1)
	require.Len(t, keys2, 1)
	assert.NotEqual(t, keys1[0], keys2[0], "different file paths must produce different keys")
	assert.Contains(t, keys1[0], "storage.cfg")
	assert.Contains(t, keys2[0], "tasks/some-task-log")
}

func TestExtractFacts_ValueTruncation(t *testing.T) {
	input := map[string]interface{}{"command": "long-output-cmd", "target_host": "host1"}
	// Create a result with very long output
	longOutput := `{"success":true,"exit_code":0,"output":"` + bigContent(500) + `"}`

	facts := ExtractFacts("pulse_read", input, longOutput)
	require.Len(t, facts, 1)
	assert.LessOrEqual(t, len(facts[0].Value), maxValueLen)
}
