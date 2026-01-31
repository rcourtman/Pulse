package chat

import (
	"encoding/json"
	"fmt"
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
	input := map[string]interface{}{"action": "get", "resource_type": "lxc", "resource_id": "106"}
	// Actual format from NewJSONResult(ResourceResponse): direct JSON, no wrapper.
	// CPU/Memory are nested structs.
	result := `{"type":"lxc","name":"postfix-server","status":"running","node":"delly","id":"lxc/106","vmid":106,"cpu":{"percent":2.5,"cores":4},"memory":{"percent":45.0,"used_gb":1.2,"total_gb":4.0}}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 2) // primary + cached key

	f := facts[0]
	assert.Equal(t, FactCategoryResource, f.Category)
	assert.Equal(t, "lxc:delly:106:status", f.Key)
	assert.Contains(t, f.Value, "running")
	assert.Contains(t, f.Value, "postfix-server")
	assert.Contains(t, f.Value, "CPU=2.5%")
	assert.Contains(t, f.Value, "Mem=45.0%")

	// Secondary cached fact
	assert.Equal(t, "query:get:106:cached", facts[1].Key)
	assert.Equal(t, FactCategoryResource, facts[1].Category)
}

func TestExtractFacts_QueryGet_NoResourceID(t *testing.T) {
	// Without resource_id in input, only primary fact should be emitted (no cached key)
	input := map[string]interface{}{"action": "get", "resource_type": "lxc"}
	result := `{"type":"lxc","name":"postfix-server","status":"running","node":"delly","id":"lxc/106","vmid":106,"cpu":{"percent":2.5,"cores":4},"memory":{"percent":45.0,"used_gb":1.2,"total_gb":4.0}}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 1) // only primary fact
	assert.Equal(t, "lxc:delly:106:status", facts[0].Key)
}

func TestExtractFacts_QueryGet_NotFound(t *testing.T) {
	input := map[string]interface{}{"action": "get", "resource_type": "vm", "resource_id": "999"}
	result := `{"error":"not_found","resource_id":"999","type":"vm"}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 1)
	assert.Equal(t, "query:get:999:error", facts[0].Key)
	assert.Equal(t, "not found: not_found", facts[0].Value)
	assert.Equal(t, FactCategoryResource, facts[0].Category)
}

func TestExtractFacts_QueryGet_NotFound_NoResourceID(t *testing.T) {
	// Without resource_id in input, negative fact should not be created
	input := map[string]interface{}{"action": "get", "resource_type": "vm"}
	result := `{"error":"not_found","resource_id":"999","type":"vm"}`

	facts := ExtractFacts("pulse_query", input, result)
	assert.Empty(t, facts)
}

func TestNegativeFactCaching_PreventsRetry(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	// Simulate extracting a negative fact from a not-found response
	input := map[string]interface{}{"action": "get", "resource_id": "999"}
	result := `{"error":"not_found","resource_id":"999","type":"vm"}`
	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 1)

	// Store the negative fact
	ka.AddFact(facts[0].Category, facts[0].Key, facts[0].Value)

	// Verify the negative fact is stored and can be looked up
	val, found := ka.Lookup("query:get:999:error")
	assert.True(t, found)
	assert.Contains(t, val, "not found")
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
	require.Len(t, facts, 2) // marker + 1 pool

	// First fact is the marker
	assert.Equal(t, "storage:pools:queried", facts[0].Key)
	assert.Equal(t, "1 pools extracted", facts[0].Value)

	f := facts[1]
	assert.Equal(t, FactCategoryStorage, f.Category)
	assert.Contains(t, f.Key, "storage:")
	assert.Contains(t, f.Key, "pbs-minipc")
	assert.Contains(t, f.Value, "PBS")
	assert.Contains(t, f.Value, "42.7% used")
	assert.Contains(t, f.Value, "573GB free")
}

func TestExtractFacts_BackupTasks_OnlyFailures(t *testing.T) {
	input := map[string]interface{}{"action": "backup_tasks"}
	result := `{"tasks":[{"vmid":106,"node":"delly","status":"OK"},{"vmid":200,"node":"minipc","status":"failed","start_time":"2024-01-15T03:00","error":"snapshot failed"}],"total":2}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 2) // marker + 1 failure

	// Marker fact
	assert.Equal(t, "backup_tasks:queried", facts[0].Key)
	assert.Equal(t, "2 tasks, 1 failed", facts[0].Value)

	// Failure fact
	f := facts[1]
	assert.Equal(t, FactCategoryStorage, f.Category)
	assert.Equal(t, "backup:200:minipc", f.Key)
	assert.Contains(t, f.Value, "failed")
	assert.Contains(t, f.Value, "snapshot failed")
}

func TestExtractFacts_BackupTasks_AllOK(t *testing.T) {
	input := map[string]interface{}{"action": "backup_tasks"}
	result := `{"tasks":[{"vmid":106,"node":"delly","status":"OK"},{"vmid":200,"node":"minipc","status":"success"}],"total":2}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 1) // marker only (no failures)
	assert.Equal(t, "backup_tasks:queried", facts[0].Key)
	assert.Equal(t, "2 tasks, 0 failed", facts[0].Value)
}

func TestPredictFactKeys_BackupTasks(t *testing.T) {
	keys := PredictFactKeys("pulse_storage", map[string]interface{}{"action": "backup_tasks"})
	require.Len(t, keys, 1)
	assert.Equal(t, "backup_tasks:queried", keys[0])
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
	// pulse_query get without resource_id / search without query are unpredictable
	assert.Nil(t, PredictFactKeys("pulse_query", map[string]interface{}{"action": "get"}))
	assert.Nil(t, PredictFactKeys("pulse_query", map[string]interface{}{"action": "search"}))
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

func TestExtractFacts_Exec_LogsDistinctKeys(t *testing.T) {
	// Two different log queries on the same host should produce different keys
	input1 := map[string]interface{}{
		"action":      "logs",
		"target_host": "delly",
		"since":       "1h",
		"grep":        "error",
	}
	input2 := map[string]interface{}{
		"action":      "logs",
		"target_host": "delly",
		"since":       "24h",
		"unit":        "nginx",
	}
	result := `{"success":true,"exit_code":0,"output":"some log lines here"}`

	facts1 := ExtractFacts("pulse_read", input1, result)
	facts2 := ExtractFacts("pulse_read", input2, result)

	require.Len(t, facts1, 1)
	require.Len(t, facts2, 1)
	assert.NotEqual(t, facts1[0].Key, facts2[0].Key, "different log queries must produce different keys")
	assert.Contains(t, facts1[0].Key, "logs")
	assert.Contains(t, facts1[0].Key, "since=1h")
	assert.Contains(t, facts1[0].Key, "grep=error")
	assert.Contains(t, facts2[0].Key, "since=24h")
	assert.Contains(t, facts2[0].Key, "unit=nginx")
}

func TestPredictFactKeys_LogsDistinct(t *testing.T) {
	keys1 := PredictFactKeys("pulse_read", map[string]interface{}{
		"target_host": "delly",
		"action":      "logs",
		"since":       "1h",
		"grep":        "error",
	})
	keys2 := PredictFactKeys("pulse_read", map[string]interface{}{
		"target_host": "delly",
		"action":      "logs",
		"since":       "24h",
		"unit":        "nginx",
	})

	require.Len(t, keys1, 1)
	require.Len(t, keys2, 1)
	assert.NotEqual(t, keys1[0], keys2[0], "different log queries must produce different predicted keys")
}

// --- Topology ---

func TestExtractFacts_QueryTopology(t *testing.T) {
	input := map[string]interface{}{"action": "topology"}
	result := `{"summary":{"total_nodes":3,"total_vms":3,"running_vms":1,"total_lxc_containers":22,"running_lxc":22,"total_docker_hosts":1},"proxmox":{"nodes":[{"name":"delly","status":"online"},{"name":"minipc","status":"online"},{"name":"pi","status":"online"}]}}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Equal(t, FactCategoryResource, f.Category)
	assert.Equal(t, "topology:summary", f.Key)
	assert.Contains(t, f.Value, "3 nodes")
	assert.Contains(t, f.Value, "delly=online")
	assert.Contains(t, f.Value, "minipc=online")
	assert.Contains(t, f.Value, "pi=online")
	assert.Contains(t, f.Value, "3 VMs (1 running)")
	assert.Contains(t, f.Value, "22 LXC (22 running)")
	assert.Contains(t, f.Value, "1 docker host")
}

// --- Health ---

func TestExtractFacts_QueryHealth(t *testing.T) {
	input := map[string]interface{}{"action": "health"}
	result := `{"connections":[{"instance_id":"delly","connected":true},{"instance_id":"minipc","connected":true},{"instance_id":"pi","connected":true}]}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Equal(t, FactCategoryResource, f.Category)
	assert.Equal(t, "health:connections", f.Key)
	assert.Equal(t, "3/3 connected", f.Value)
}

func TestExtractFacts_QueryHealth_Disconnected(t *testing.T) {
	input := map[string]interface{}{"action": "health"}
	result := `{"connections":[{"instance_id":"delly","connected":true},{"instance_id":"minipc","connected":false},{"instance_id":"pi","connected":true}]}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Contains(t, f.Value, "2/3 connected")
	assert.Contains(t, f.Value, "disconnected: minipc")
}

// --- Alerts: Findings ---

func TestExtractFacts_AlertsFindings(t *testing.T) {
	input := map[string]interface{}{"action": "findings"}
	// Real response format: {"active": [...], "counts": {"active": N, "dismissed": N}}
	result := `{"active":[{"key":"high-cpu-vm101","severity":"warning","title":"High CPU on vm101","resource_id":"vm101"},{"key":"disk-full-ct200","severity":"critical","title":"Disk full on ct200","resource_id":"ct200"}],"counts":{"active":2,"dismissed":1}}`

	facts := ExtractFacts("pulse_alerts", input, result)
	require.GreaterOrEqual(t, len(facts), 2) // overview + per-finding

	// First fact should be overview
	assert.Equal(t, "findings:overview", facts[0].Key)
	assert.Equal(t, FactCategoryFinding, facts[0].Category)
	assert.Contains(t, facts[0].Value, "2 active")
	assert.Contains(t, facts[0].Value, "1 dismissed")

	// Per-finding facts
	var findingKeys []string
	for _, f := range facts[1:] {
		findingKeys = append(findingKeys, f.Key)
	}
	assert.Contains(t, findingKeys, "finding:high-cpu-vm101:vm101")
	assert.Contains(t, findingKeys, "finding:disk-full-ct200:ct200")
}

// --- Alerts: List ---

func TestExtractFacts_AlertsList(t *testing.T) {
	input := map[string]interface{}{"action": "list"}
	result := `{"alerts":[{"resource_name":"vm101","type":"cpu","severity":"critical","value":95.2,"threshold":80,"status":"active"},{"resource_name":"ct200","type":"memory","severity":"warning","value":82.0,"threshold":90,"status":"active"}]}`

	facts := ExtractFacts("pulse_alerts", input, result)
	require.GreaterOrEqual(t, len(facts), 2)

	assert.Equal(t, "alerts:overview", facts[0].Key)
	assert.Equal(t, FactCategoryAlert, facts[0].Category)
	assert.Contains(t, facts[0].Value, "2 active alerts")

	// Per-alert facts
	var alertKeys []string
	for _, f := range facts[1:] {
		alertKeys = append(alertKeys, f.Key)
		assert.Equal(t, FactCategoryAlert, f.Category)
	}
	assert.Contains(t, alertKeys, "alert:vm101:cpu")
	assert.Contains(t, alertKeys, "alert:ct200:memory")
}

// --- Metrics: Baselines ---

func TestExtractFacts_MetricsBaselines(t *testing.T) {
	input := map[string]interface{}{"action": "baselines"}
	// Real format: baselines.{nodeName}.{resourceKey:metricType} with mean/std_dev/min/max
	result := `{"baselines":{"delly":{"delly:101:cpu":{"mean":12.3,"std_dev":5.0,"min":0.1,"max":78.5},"delly:101:memory":{"mean":65.0,"std_dev":10.0,"min":20.0,"max":89.0}},"minipc":{"minipc:200:cpu":{"mean":5.0,"std_dev":2.0,"min":0.5,"max":20.0},"minipc:200:memory":{"mean":40.0,"std_dev":8.0,"min":10.0,"max":55.0}}}}`

	facts := ExtractFacts("pulse_metrics", input, result)
	require.Len(t, facts, 3) // marker + 2 nodes

	// First fact is the marker
	assert.Equal(t, "baselines:queried", facts[0].Key)
	assert.Equal(t, "2 nodes extracted", facts[0].Value)

	// Find delly fact
	var dellyFact *FactEntry
	for i := range facts[1:] {
		idx := i + 1
		if facts[idx].Key == "baseline:delly" {
			dellyFact = &facts[idx]
			break
		}
	}
	require.NotNil(t, dellyFact)
	assert.Equal(t, FactCategoryMetrics, dellyFact.Category)
	assert.Contains(t, dellyFact.Value, "cpu: avg=12.3% max=78.5%")
	assert.Contains(t, dellyFact.Value, "memory: avg=65.0% max=89.0%")
}

func TestExtractFacts_MetricsBaselines_Cap(t *testing.T) {
	// Build a response with 15 nodes — should cap at 10
	baselines := make(map[string]interface{})
	for i := 0; i < 15; i++ {
		nodeName := fmt.Sprintf("node%d", i)
		baselines[nodeName] = map[string]interface{}{
			nodeName + ":100:cpu":    map[string]interface{}{"mean": 10.0, "std_dev": 2.0, "min": 1.0, "max": 50.0},
			nodeName + ":100:memory": map[string]interface{}{"mean": 30.0, "std_dev": 5.0, "min": 5.0, "max": 60.0},
		}
	}
	resultBytes, _ := json.Marshal(map[string]interface{}{"baselines": baselines})

	input := map[string]interface{}{"action": "baselines"}
	facts := ExtractFacts("pulse_metrics", input, string(resultBytes))
	assert.LessOrEqual(t, len(facts), 11, "should cap at 10 resources + 1 marker")
	assert.GreaterOrEqual(t, len(facts), 2, "should produce marker + at least some facts")
	assert.Equal(t, "baselines:queried", facts[0].Key)
}

// --- Storage: Disk Health ---

func TestExtractFacts_StorageDiskHealth(t *testing.T) {
	input := map[string]interface{}{"action": "disk_health"}
	result := `{"hosts":[{"hostname":"delly","smart":[{"device":"/dev/sda","model":"WD Blue","health":"PASSED"},{"device":"/dev/sdb","model":"Samsung","health":"FAILED"}]},{"hostname":"minipc","smart":[{"device":"/dev/sda","model":"Crucial","health":"PASSED"}]}]}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 3) // marker + 2 hosts

	// First fact is the marker
	assert.Equal(t, "disk_health:queried", facts[0].Key)
	assert.Equal(t, "2 hosts extracted", facts[0].Value)

	// Find delly fact
	var dellyFact *FactEntry
	var minipcFact *FactEntry
	for i := range facts[1:] {
		idx := i + 1
		switch facts[idx].Key {
		case "disk_health:delly":
			dellyFact = &facts[idx]
		case "disk_health:minipc":
			minipcFact = &facts[idx]
		}
	}

	require.NotNil(t, dellyFact)
	assert.Equal(t, FactCategoryStorage, dellyFact.Category)
	assert.Contains(t, dellyFact.Value, "1 PASSED")
	assert.Contains(t, dellyFact.Value, "1 FAILED")
	assert.Contains(t, dellyFact.Value, "/dev/sdb")

	require.NotNil(t, minipcFact)
	assert.Contains(t, minipcFact.Value, "1 disks all PASSED")
}

// --- Metrics: Physical Disks ---

func TestExtractFacts_MetricsDisks(t *testing.T) {
	input := map[string]interface{}{"action": "disks"}
	result := `{"disks":[{"host":"Tower","device":"/dev/sda","model":"WD Blue","health":"PASSED"},{"host":"Tower","device":"/dev/sdb","model":"Samsung","health":"PASSED"},{"host":"Mini","device":"/dev/sda","model":"Crucial","health":"FAILED"}]}`

	facts := ExtractFacts("pulse_metrics", input, result)
	require.Len(t, facts, 2) // marker + summary

	// First fact is the marker
	assert.Equal(t, "physical_disks:queried", facts[0].Key)
	assert.Equal(t, "summary extracted", facts[0].Value)

	f := facts[1]
	assert.Equal(t, FactCategoryStorage, f.Category)
	assert.Equal(t, "physical_disks:summary", f.Key)
	assert.Contains(t, f.Value, "3 disks")
	assert.Contains(t, f.Value, "1 FAILED")
	assert.Contains(t, f.Value, "Mini /dev/sda Crucial")
}

func TestExtractFacts_MetricsDisks_AllPassed(t *testing.T) {
	input := map[string]interface{}{"action": "disks"}
	result := `{"disks":[{"host":"Tower","device":"/dev/sda","health":"PASSED"},{"host":"Tower","device":"/dev/sdb","health":"PASSED"}]}`

	facts := ExtractFacts("pulse_metrics", input, result)
	require.Len(t, facts, 2) // marker + summary
	assert.Equal(t, "physical_disks:queried", facts[0].Key)
	assert.Contains(t, facts[1].Value, "2 disks total, all PASSED")
}

// --- PredictFactKeys: New entries ---

func TestPredictFactKeys_QueryTopology(t *testing.T) {
	keys := PredictFactKeys("pulse_query", map[string]interface{}{"action": "topology"})
	require.Len(t, keys, 1)
	assert.Equal(t, "topology:summary", keys[0])
}

func TestPredictFactKeys_QueryHealth(t *testing.T) {
	keys := PredictFactKeys("pulse_query", map[string]interface{}{"action": "health"})
	require.Len(t, keys, 1)
	assert.Equal(t, "health:connections", keys[0])
}

func TestPredictFactKeys_AlertsFindings(t *testing.T) {
	keys := PredictFactKeys("pulse_alerts", map[string]interface{}{"action": "findings"})
	require.Len(t, keys, 1)
	assert.Equal(t, "findings:overview", keys[0])
}

func TestPredictFactKeys_AlertsList(t *testing.T) {
	keys := PredictFactKeys("pulse_alerts", map[string]interface{}{"action": "list"})
	require.Len(t, keys, 1)
	assert.Equal(t, "alerts:overview", keys[0])
}

func TestPredictFactKeys_MetricsBaselines(t *testing.T) {
	// Even with resource_id, predict always returns the marker key
	keys := PredictFactKeys("pulse_metrics", map[string]interface{}{"action": "baselines", "resource_id": "vm101"})
	require.Len(t, keys, 1)
	assert.Equal(t, "baselines:queried", keys[0])
}

func TestPredictFactKeys_MetricsBaselinesGlobal(t *testing.T) {
	keys := PredictFactKeys("pulse_metrics", map[string]interface{}{"action": "baselines"})
	require.Len(t, keys, 1)
	assert.Equal(t, "baselines:queried", keys[0])
}

func TestPredictFactKeys_StoragePools(t *testing.T) {
	keys := PredictFactKeys("pulse_storage", map[string]interface{}{"action": "pools"})
	require.Len(t, keys, 1)
	assert.Equal(t, "storage:pools:queried", keys[0])
}

func TestPredictFactKeys_StorageDiskHealth(t *testing.T) {
	keys := PredictFactKeys("pulse_storage", map[string]interface{}{"action": "disk_health"})
	require.Len(t, keys, 1)
	assert.Equal(t, "disk_health:queried", keys[0])
}

func TestPredictFactKeys_MetricsDisks(t *testing.T) {
	keys := PredictFactKeys("pulse_metrics", map[string]interface{}{"action": "disks"})
	require.Len(t, keys, 1)
	assert.Equal(t, "physical_disks:queried", keys[0])
}

// --- Metrics: Temperatures ---

func TestExtractFacts_MetricsTemperatures(t *testing.T) {
	input := map[string]interface{}{"action": "temperatures"}
	result := `[{"hostname":"delly","cpu_temps":{"core0":52,"core1":55},"disk_temps":{"sda":38,"sdb":42}},{"hostname":"minipc","cpu_temps":{"core0":45},"disk_temps":{}}]`

	facts := ExtractFacts("pulse_metrics", input, result)
	require.Len(t, facts, 3) // marker + 2 hosts

	assert.Equal(t, "temperatures:queried", facts[0].Key)
	assert.Equal(t, "2 hosts", facts[0].Value)

	var dellyFact, minipcFact *FactEntry
	for i := range facts[1:] {
		idx := i + 1
		switch facts[idx].Key {
		case "temperatures:delly":
			dellyFact = &facts[idx]
		case "temperatures:minipc":
			minipcFact = &facts[idx]
		}
	}

	require.NotNil(t, dellyFact)
	assert.Equal(t, FactCategoryMetrics, dellyFact.Category)
	assert.Contains(t, dellyFact.Value, "cpu_max=55°C")
	assert.Contains(t, dellyFact.Value, "disk_max=42°C")

	require.NotNil(t, minipcFact)
	assert.Contains(t, minipcFact.Value, "cpu_max=45°C")
	assert.NotContains(t, minipcFact.Value, "disk_max")
}

func TestExtractFacts_MetricsTemperatures_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "temperatures"}
	result := `[]`

	facts := ExtractFacts("pulse_metrics", input, result)
	require.Len(t, facts, 1) // marker only
	assert.Equal(t, "temperatures:queried", facts[0].Key)
	assert.Equal(t, "0 hosts", facts[0].Value)
}

// --- Storage: RAID ---

func TestExtractFacts_StorageRAID(t *testing.T) {
	input := map[string]interface{}{"action": "raid"}
	result := `{"hosts":[{"hostname":"delly","arrays":[{"device":"/dev/md0","level":"raid1","state":"clean","failed_devices":0,"total_devices":2},{"device":"/dev/md1","level":"raid5","state":"degraded","failed_devices":1,"total_devices":4}]}]}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 2) // marker + 1 host

	assert.Equal(t, "raid:queried", facts[0].Key)
	assert.Equal(t, "1 hosts", facts[0].Value)

	assert.Equal(t, "raid:delly", facts[1].Key)
	assert.Equal(t, FactCategoryStorage, facts[1].Category)
	assert.Contains(t, facts[1].Value, "2 arrays")
	assert.Contains(t, facts[1].Value, "1 degraded/failed")
}

func TestExtractFacts_StorageRAID_AllClean(t *testing.T) {
	input := map[string]interface{}{"action": "raid"}
	result := `{"hosts":[{"hostname":"minipc","arrays":[{"device":"/dev/md0","level":"raid1","state":"clean","failed_devices":0,"total_devices":2}]}]}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 2)
	assert.Contains(t, facts[1].Value, "all clean")
}

// --- Storage: Backups ---

func TestExtractFacts_StorageBackups(t *testing.T) {
	input := map[string]interface{}{"action": "backups"}
	result := `{"pbs":[{},{}],"pve":[{}],"pbs_servers":[{"name":"pbs-minipc","status":"connected","datastores":[{"name":"datastore1","usage_percent":42.5}]}]}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.GreaterOrEqual(t, len(facts), 3) // marker + server + summary

	assert.Equal(t, "backups:queried", facts[0].Key)
	assert.Contains(t, facts[0].Value, "2 PBS")
	assert.Contains(t, facts[0].Value, "1 PVE")
	assert.Contains(t, facts[0].Value, "1 PBS servers")

	var serverFact, summaryFact *FactEntry
	for i := range facts[1:] {
		idx := i + 1
		switch facts[idx].Key {
		case "backups:server:pbs-minipc":
			serverFact = &facts[idx]
		case "backups:summary":
			summaryFact = &facts[idx]
		}
	}

	require.NotNil(t, serverFact)
	assert.Contains(t, serverFact.Value, "connected")
	assert.Contains(t, serverFact.Value, "datastore1: 42.5% used")

	require.NotNil(t, summaryFact)
	assert.Contains(t, summaryFact.Value, "2 PBS backups")
	assert.Contains(t, summaryFact.Value, "1 PVE backups")
}

func TestExtractFacts_StorageBackups_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "backups"}
	result := `{"pbs":[],"pve":[],"pbs_servers":[]}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 1) // marker only
	assert.Equal(t, "backups:queried", facts[0].Key)
	assert.Contains(t, facts[0].Value, "0 PBS")
}

// --- PredictFactKeys: New extractors ---

func TestPredictFactKeys_StorageRAID(t *testing.T) {
	keys := PredictFactKeys("pulse_storage", map[string]interface{}{"action": "raid"})
	require.Len(t, keys, 1)
	assert.Equal(t, "raid:queried", keys[0])
}

func TestPredictFactKeys_StorageBackups(t *testing.T) {
	keys := PredictFactKeys("pulse_storage", map[string]interface{}{"action": "backups"})
	require.Len(t, keys, 1)
	assert.Equal(t, "backups:queried", keys[0])
}

func TestPredictFactKeys_MetricsTemperatures(t *testing.T) {
	keys := PredictFactKeys("pulse_metrics", map[string]interface{}{"action": "temperatures"})
	require.Len(t, keys, 1)
	assert.Equal(t, "temperatures:queried", keys[0])
}

func TestExtractFacts_ValueTruncation(t *testing.T) {
	input := map[string]interface{}{"command": "long-output-cmd", "target_host": "host1"}
	// Create a result with very long output
	longOutput := `{"success":true,"exit_code":0,"output":"` + bigContent(500) + `"}`

	facts := ExtractFacts("pulse_read", input, longOutput)
	require.Len(t, facts, 1)
	assert.LessOrEqual(t, len(facts[0].Value), maxValueLen)
}

// --- Change 2: Query List Extractor ---

func TestExtractFacts_QueryList(t *testing.T) {
	input := map[string]interface{}{"action": "list"}
	result := `{"nodes":[{"name":"delly","status":"online"},{"name":"minipc","status":"online"}],"vms":[{"name":"win10","status":"running"},{"name":"ubuntu","status":"stopped"}],"containers":[{"name":"postfix","status":"running"},{"name":"nginx","status":"running"},{"name":"test","status":"stopped"}],"docker_hosts":[{"hostname":"delly","display_name":"Delly Docker","container_count":5}],"total":{"nodes":2,"vms":2,"containers":3,"docker_hosts":1}}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Equal(t, FactCategoryResource, f.Category)
	assert.Equal(t, "inventory:summary", f.Key)
	assert.Contains(t, f.Value, "2 nodes")
	assert.Contains(t, f.Value, "2 VMs (1 running)")
	assert.Contains(t, f.Value, "3 LXC (2 running)")
	assert.Contains(t, f.Value, "1 docker hosts (5 containers)")
}

func TestExtractFacts_QueryList_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "list"}
	result := `{"nodes":[],"vms":[],"containers":[],"docker_hosts":[],"total":{"nodes":0,"vms":0,"containers":0,"docker_hosts":0}}`

	facts := ExtractFacts("pulse_query", input, result)
	assert.Empty(t, facts)
}

func TestExtractFacts_QueryList_TypeFiltered(t *testing.T) {
	// Real-world: model calls pulse_query with action=list AND type=vms
	// The response only has the vms array, but total has all counts
	input := map[string]interface{}{"action": "list", "type": "vms"}
	result := `{"vms":[{"vmid":100,"name":"docker","status":"running","node":"minipc","cpu_percent":0.71,"memory_percent":4593.75},{"vmid":160,"name":"windows-runner","status":"stopped","node":"delly"},{"vmid":250,"name":"tails-anon","status":"stopped","node":"delly"}],"total":{"nodes":3,"vms":3,"containers":22,"docker_hosts":1}}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 1, "type-filtered list should still extract inventory:summary")
	assert.Equal(t, "inventory:summary", facts[0].Key)
	assert.Contains(t, facts[0].Value, "3 VMs")

	// Also verify PredictFactKeys works with type param
	keys := PredictFactKeys("pulse_query", input)
	require.Len(t, keys, 1)
	assert.Equal(t, "inventory:summary", keys[0])

	// End-to-end: store fact in KA, verify gate would fire
	ka := NewKnowledgeAccumulator()
	for _, f := range facts {
		ka.AddFact(f.Category, f.Key, f.Value)
	}
	val, found := ka.Lookup("inventory:summary")
	assert.True(t, found, "inventory:summary should be in KA after extraction")
	assert.Contains(t, val, "3 VMs")
}

func TestPredictFactKeys_QueryList(t *testing.T) {
	keys := PredictFactKeys("pulse_query", map[string]interface{}{"action": "list"})
	require.Len(t, keys, 1)
	assert.Equal(t, "inventory:summary", keys[0])
}

// --- Change 3: Query Config Extractor ---

func TestExtractFacts_QueryConfig_LXC(t *testing.T) {
	input := map[string]interface{}{"action": "config", "resource_id": "106", "node": "delly"}
	result := `{"guest_type":"lxc","vmid":106,"name":"postfix-server","node":"delly","hostname":"postfix","os_type":"ubuntu","onboot":true,"mounts":[{"key":"mp0","mountpoint":"/data"},{"key":"mp1","mountpoint":"/logs"}],"disks":[{"key":"rootfs"}]}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 2) // primary + cached key

	f := facts[0]
	assert.Equal(t, FactCategoryResource, f.Category)
	assert.Equal(t, "config:delly:106", f.Key)
	assert.Contains(t, f.Value, "lxc")
	assert.Contains(t, f.Value, "hostname=postfix")
	assert.Contains(t, f.Value, "os=ubuntu")
	assert.Contains(t, f.Value, "onboot=yes")
	assert.Contains(t, f.Value, "2 mounts")
	assert.Contains(t, f.Value, "1 disks")

	// Secondary cached key for gate matching without node
	assert.Equal(t, "config:106:cached", facts[1].Key)
	assert.Equal(t, f.Value, facts[1].Value)
}

func TestExtractFacts_QueryConfig_VM(t *testing.T) {
	input := map[string]interface{}{"action": "config", "resource_id": "200"}
	result := `{"guest_type":"qemu","vmid":200,"name":"win10","node":"minipc","os_type":"win10","onboot":false,"disks":[{"key":"scsi0"},{"key":"scsi1"}]}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 2) // primary + cached key

	f := facts[0]
	assert.Equal(t, "config:minipc:200", f.Key)
	assert.Contains(t, f.Value, "qemu")
	assert.Contains(t, f.Value, "onboot=no")
	assert.Contains(t, f.Value, "2 disks")

	assert.Equal(t, "config:200:cached", facts[1].Key)
}

func TestExtractFacts_QueryConfig_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "config", "resource_id": "999"}
	result := `{}`

	facts := ExtractFacts("pulse_query", input, result)
	assert.Empty(t, facts)
}

func TestPredictFactKeys_QueryConfig(t *testing.T) {
	keys := PredictFactKeys("pulse_query", map[string]interface{}{
		"action":      "config",
		"resource_id": "106",
		"node":        "delly",
	})
	require.Len(t, keys, 2)
	assert.Equal(t, "config:106:cached", keys[0])
	assert.Equal(t, "config:delly:106", keys[1])
}

func TestPredictFactKeys_QueryConfig_NoNode(t *testing.T) {
	// Without node, still predicts the cached key
	keys := PredictFactKeys("pulse_query", map[string]interface{}{
		"action":      "config",
		"resource_id": "106",
	})
	require.Len(t, keys, 1)
	assert.Equal(t, "config:106:cached", keys[0])
}

func TestPredictFactKeys_QueryConfig_NoResourceID(t *testing.T) {
	// Without resource_id, can't predict anything
	keys := PredictFactKeys("pulse_query", map[string]interface{}{
		"action": "config",
	})
	assert.Nil(t, keys)
}

// --- Change 4: PredictFactKeys for query:get ---

func TestPredictFactKeys_QueryGet_WithResourceID(t *testing.T) {
	keys := PredictFactKeys("pulse_query", map[string]interface{}{
		"action":      "get",
		"resource_id": "106",
	})
	require.Len(t, keys, 2)
	assert.Equal(t, "query:get:106:cached", keys[0])
	assert.Equal(t, "query:get:106:error", keys[1])
}

func TestPredictFactKeys_QueryGet_NoResourceID(t *testing.T) {
	keys := PredictFactKeys("pulse_query", map[string]interface{}{
		"action": "get",
	})
	assert.Nil(t, keys)
}

// --- Change 5: PredictFactKeys for search ---

func TestPredictFactKeys_QuerySearch(t *testing.T) {
	keys := PredictFactKeys("pulse_query", map[string]interface{}{
		"action": "search",
		"query":  "postfix",
	})
	require.Len(t, keys, 1)
	assert.Equal(t, "search:postfix:summary", keys[0])
}

func TestPredictFactKeys_QuerySearch_NoQuery(t *testing.T) {
	keys := PredictFactKeys("pulse_query", map[string]interface{}{
		"action": "search",
	})
	assert.Nil(t, keys)
}

// --- Change 1: Negative Marker Tests ---

func TestNegativeMarker_TextResponse(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	// Simulate: tool returns plain text that doesn't parse as JSON
	// The extractor returns nil, but PredictFactKeys returns a key
	toolName := "pulse_storage"
	toolInput := map[string]interface{}{"action": "raid"}
	resultText := "No RAID arrays found across any hosts"

	// ExtractFacts should return nil for plain text
	facts := ExtractFacts(toolName, toolInput, resultText)
	assert.Empty(t, facts)

	// PredictFactKeys should return the raid marker key
	predictedKeys := PredictFactKeys(toolName, toolInput)
	require.Len(t, predictedKeys, 1)
	assert.Equal(t, "raid:queried", predictedKeys[0])

	// Simulate what agentic.go does: store negative marker
	for _, key := range predictedKeys {
		if _, found := ka.Lookup(key); !found {
			cat := categoryForPredictedKey(key)
			summary := resultText
			if len(summary) > 120 {
				summary = summary[:120]
			}
			ka.AddFact(cat, key, fmt.Sprintf("checked: %s", summary))
		}
	}

	// Verify negative marker is stored
	val, found := ka.Lookup("raid:queried")
	assert.True(t, found)
	assert.Contains(t, val, "checked:")
	assert.Contains(t, val, "No RAID arrays found")
}

func TestNegativeMarker_NotStoredOnSuccess(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	// Simulate: tool returns valid JSON that ExtractFacts can parse
	toolName := "pulse_storage"
	toolInput := map[string]interface{}{"action": "raid"}
	resultText := `{"hosts":[{"hostname":"delly","arrays":[{"device":"/dev/md0","level":"raid1","state":"clean","failed_devices":0,"total_devices":2}]}]}`

	facts := ExtractFacts(toolName, toolInput, resultText)
	require.NotEmpty(t, facts)

	// Store the real facts
	for _, f := range facts {
		ka.AddFact(f.Category, f.Key, f.Value)
	}

	// The negative marker logic only triggers when len(facts) == 0
	// So "raid:queried" should be stored by the extractor itself (as a marker fact), not as a negative marker
	val, found := ka.Lookup("raid:queried")
	assert.True(t, found)
	assert.NotContains(t, val, "checked:") // Real fact, not a negative marker
}

func TestNegativeMarker_GatePreventsRetry(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	// Store a negative marker (as would happen after a text response)
	ka.AddFact(FactCategoryStorage, "raid:queried", "checked: No RAID arrays found across any hosts")

	// Now simulate the gate check: PredictFactKeys returns the key, Lookup finds it
	predictedKeys := PredictFactKeys("pulse_storage", map[string]interface{}{"action": "raid"})
	require.Len(t, predictedKeys, 1)

	val, found := ka.Lookup(predictedKeys[0])
	assert.True(t, found, "gate should find the negative marker")
	assert.Contains(t, val, "checked:")
}

// --- categoryForPredictedKey ---

func TestCategoryForPredictedKey(t *testing.T) {
	tests := []struct {
		key      string
		expected FactCategory
	}{
		{"storage:pools:queried", FactCategoryStorage},
		{"disk_health:queried", FactCategoryStorage},
		{"raid:queried", FactCategoryStorage},
		{"backups:queried", FactCategoryStorage},
		{"physical_disks:queried", FactCategoryStorage},
		{"metrics:vm101", FactCategoryMetrics},
		{"baseline:delly", FactCategoryMetrics},
		{"baselines:queried", FactCategoryMetrics},
		{"temperatures:queried", FactCategoryMetrics},
		{"exec:delly:some-cmd", FactCategoryExec},
		{"discovery:delly:106", FactCategoryDiscovery},
		{"topology:summary", FactCategoryResource},
		{"health:connections", FactCategoryResource},
		{"search:postfix:summary", FactCategoryResource},
		{"inventory:summary", FactCategoryResource},
		{"config:delly:106", FactCategoryResource},
		{"finding:high-cpu", FactCategoryFinding},
		{"findings:overview", FactCategoryFinding},
		{"alert:vm101:cpu", FactCategoryAlert},
		{"alerts:overview", FactCategoryAlert},
		{"ceph:queried", FactCategoryStorage},
		{"ceph:mycluster", FactCategoryStorage},
		{"ceph_details:queried", FactCategoryStorage},
		{"ceph_details:host1", FactCategoryStorage},
		{"snapshots:queried", FactCategoryStorage},
		{"snapshots:summary", FactCategoryStorage},
		{"replication:queried", FactCategoryStorage},
		{"replication:summary", FactCategoryStorage},
		{"pbs_jobs:queried", FactCategoryStorage},
		{"pbs_jobs:summary", FactCategoryStorage},
		{"resource_disks:queried", FactCategoryStorage},
		{"resource_disks:summary", FactCategoryStorage},
		{"backup_tasks:queried", FactCategoryStorage},
		{"backup:200:minipc", FactCategoryStorage},
		{"docker_services:queried", FactCategoryResource},
		{"docker_services:summary", FactCategoryResource},
		{"docker_updates:queried", FactCategoryResource},
		{"docker_swarm:status", FactCategoryResource},
		{"docker_tasks:queried", FactCategoryResource},
		{"k8s_clusters:queried", FactCategoryResource},
		{"k8s_cluster:mycluster", FactCategoryResource},
		{"k8s_nodes:queried", FactCategoryResource},
		{"k8s_pods:queried", FactCategoryResource},
		{"k8s_deployments:queried", FactCategoryResource},
		{"pmg:queried", FactCategoryResource},
		{"pmg:mypmg", FactCategoryResource},
		{"pmg_mail_stats:queried", FactCategoryResource},
		{"pmg_queues:queried", FactCategoryResource},
		{"pmg_spam:queried", FactCategoryResource},
		{"patrol_findings:queried", FactCategoryFinding},
		{"patrol_findings:summary", FactCategoryFinding},
		{"unknown:key", FactCategoryResource}, // default
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.expected, categoryForPredictedKey(tt.key))
		})
	}
}

// --- Storage: Ceph ---

func TestExtractFacts_Ceph(t *testing.T) {
	input := map[string]interface{}{"action": "ceph"}
	result := `[{"name":"ceph-main","health":"HEALTH_OK","details":{"osd_count":6,"osds_up":6,"osds_down":0,"monitors":3,"usage_percent":42.5}}]`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 2) // marker + 1 cluster

	assert.Equal(t, "ceph:queried", facts[0].Key)
	assert.Equal(t, "1 clusters", facts[0].Value)
	assert.Equal(t, FactCategoryStorage, facts[0].Category)

	assert.Equal(t, "ceph:ceph-main", facts[1].Key)
	assert.Contains(t, facts[1].Value, "HEALTH_OK")
	assert.Contains(t, facts[1].Value, "6 OSDs")
	assert.Contains(t, facts[1].Value, "6 up")
	assert.Contains(t, facts[1].Value, "0 down")
	assert.Contains(t, facts[1].Value, "3 monitors")
	assert.Contains(t, facts[1].Value, "42% used")
}

func TestExtractFacts_Ceph_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "ceph"}
	result := `[]`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 1) // marker only
	assert.Equal(t, "ceph:queried", facts[0].Key)
	assert.Equal(t, "0 clusters", facts[0].Value)
}

func TestPredictFactKeys_Ceph(t *testing.T) {
	keys := PredictFactKeys("pulse_storage", map[string]interface{}{"action": "ceph"})
	require.Len(t, keys, 1)
	assert.Equal(t, "ceph:queried", keys[0])
}

// --- Storage: Ceph Details ---

func TestExtractFacts_CephDetails(t *testing.T) {
	input := map[string]interface{}{"action": "ceph_details"}
	result := `{"hosts":[{"hostname":"node1","health":{"status":"HEALTH_OK"},"osd_map":{"num_osds":4,"num_up":4,"num_down":0},"pg_map":{"usage_percent":35.2},"pools":[{},{}]}],"total":1}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 2) // marker + 1 host

	assert.Equal(t, "ceph_details:queried", facts[0].Key)
	assert.Equal(t, "1 hosts", facts[0].Value)
	assert.Equal(t, FactCategoryStorage, facts[0].Category)

	assert.Equal(t, "ceph_details:node1", facts[1].Key)
	assert.Contains(t, facts[1].Value, "HEALTH_OK")
	assert.Contains(t, facts[1].Value, "4 OSDs")
	assert.Contains(t, facts[1].Value, "4 up")
	assert.Contains(t, facts[1].Value, "35% used")
	assert.Contains(t, facts[1].Value, "2 pools")
}

func TestExtractFacts_CephDetails_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "ceph_details"}
	result := `{"hosts":[],"total":0}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 1) // marker only
	assert.Equal(t, "ceph_details:queried", facts[0].Key)
	assert.Equal(t, "0 hosts", facts[0].Value)
}

func TestPredictFactKeys_CephDetails(t *testing.T) {
	keys := PredictFactKeys("pulse_storage", map[string]interface{}{"action": "ceph_details"})
	require.Len(t, keys, 1)
	assert.Equal(t, "ceph_details:queried", keys[0])
}

// --- Storage: Snapshots ---

func TestExtractFacts_Snapshots(t *testing.T) {
	input := map[string]interface{}{"action": "snapshots"}
	result := `{"snapshots":[{"vmid":100,"vm_name":"docker","type":"qemu","node":"minipc","snapshot_name":"snap1"},{"vmid":100,"vm_name":"docker","type":"qemu","node":"minipc","snapshot_name":"snap2"},{"vmid":106,"vm_name":"postfix","type":"lxc","node":"delly","snapshot_name":"backup"}],"total":3}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 2) // marker + summary

	assert.Equal(t, "snapshots:queried", facts[0].Key)
	assert.Equal(t, "3 snapshots", facts[0].Value)

	assert.Equal(t, "snapshots:summary", facts[1].Key)
	assert.Contains(t, facts[1].Value, "3 total")
}

func TestExtractFacts_Snapshots_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "snapshots"}
	result := `{"snapshots":[],"total":0}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 1) // marker only
	assert.Equal(t, "snapshots:queried", facts[0].Key)
	assert.Equal(t, "0 snapshots", facts[0].Value)
}

func TestPredictFactKeys_Snapshots(t *testing.T) {
	keys := PredictFactKeys("pulse_storage", map[string]interface{}{"action": "snapshots"})
	require.Len(t, keys, 1)
	assert.Equal(t, "snapshots:queried", keys[0])
}

// --- Storage: Replication ---

func TestExtractFacts_Replication(t *testing.T) {
	input := map[string]interface{}{"action": "replication"}
	result := `[{"id":"106-0","guest_id":106,"guest_name":"postfix","source_node":"delly","target_node":"minipc","status":"ok","error":""},{"id":"200-0","guest_id":200,"guest_name":"win10","source_node":"delly","target_node":"minipc","status":"error","error":"connection refused"}]`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 2) // marker + summary

	assert.Equal(t, "replication:queried", facts[0].Key)
	assert.Equal(t, "2 jobs", facts[0].Value)

	assert.Equal(t, "replication:summary", facts[1].Key)
	assert.Contains(t, facts[1].Value, "2 jobs")
	assert.Contains(t, facts[1].Value, "1 with errors")
	assert.Contains(t, facts[1].Value, "win10")
}

func TestExtractFacts_Replication_AllOK(t *testing.T) {
	input := map[string]interface{}{"action": "replication"}
	result := `[{"id":"106-0","guest_id":106,"guest_name":"postfix","source_node":"delly","target_node":"minipc","status":"ok","error":""}]`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 2)
	assert.Contains(t, facts[1].Value, "all ok")
}

func TestExtractFacts_Replication_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "replication"}
	result := `[]`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 1) // marker only
	assert.Equal(t, "replication:queried", facts[0].Key)
	assert.Equal(t, "0 jobs", facts[0].Value)
}

func TestPredictFactKeys_Replication(t *testing.T) {
	keys := PredictFactKeys("pulse_storage", map[string]interface{}{"action": "replication"})
	require.Len(t, keys, 1)
	assert.Equal(t, "replication:queried", keys[0])
}

// --- Storage: PBS Jobs ---

func TestExtractFacts_PBSJobs(t *testing.T) {
	input := map[string]interface{}{"action": "pbs_jobs"}
	result := `{"instance":"pbs-minipc","jobs":[{"id":"j1","type":"backup","store":"datastore1","status":"ok","error":""},{"id":"j2","type":"backup","store":"datastore1","status":"error","error":"timeout"},{"id":"j3","type":"sync","store":"datastore2","status":"ok","error":""}],"total":3}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 2) // marker + summary

	assert.Equal(t, "pbs_jobs:queried", facts[0].Key)
	assert.Equal(t, "3 jobs", facts[0].Value)

	assert.Equal(t, "pbs_jobs:summary", facts[1].Key)
	assert.Contains(t, facts[1].Value, "backup")
	assert.Contains(t, facts[1].Value, "sync")
	assert.Contains(t, facts[1].Value, "1 with errors")
}

func TestExtractFacts_PBSJobs_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "pbs_jobs"}
	result := `{"instance":"pbs-minipc","jobs":[],"total":0}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 1) // marker only
	assert.Equal(t, "pbs_jobs:queried", facts[0].Key)
	assert.Equal(t, "0 jobs", facts[0].Value)
}

func TestPredictFactKeys_PBSJobs(t *testing.T) {
	keys := PredictFactKeys("pulse_storage", map[string]interface{}{"action": "pbs_jobs"})
	require.Len(t, keys, 1)
	assert.Equal(t, "pbs_jobs:queried", keys[0])
}

// --- Storage: Resource Disks ---

func TestExtractFacts_ResourceDisks(t *testing.T) {
	input := map[string]interface{}{"action": "resource_disks"}
	result := `{"resources":[{"vmid":106,"name":"postfix","type":"lxc","node":"delly","disks":[{"mountpoint":"/","usage_percent":45.2},{"mountpoint":"/data","usage_percent":85.0}]},{"vmid":200,"name":"win10","type":"qemu","node":"minipc","disks":[{"mountpoint":"C:","usage_percent":92.3}]}],"total":2}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 2) // marker + summary

	assert.Equal(t, "resource_disks:queried", facts[0].Key)
	assert.Equal(t, "2 resources", facts[0].Value)

	assert.Equal(t, "resource_disks:summary", facts[1].Key)
	assert.Contains(t, facts[1].Value, "2 resources")
	assert.Contains(t, facts[1].Value, "3 disks total")
	assert.Contains(t, facts[1].Value, "2 disks over 80%")
}

func TestExtractFacts_ResourceDisks_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "resource_disks"}
	result := `{"resources":[],"total":0}`

	facts := ExtractFacts("pulse_storage", input, result)
	require.Len(t, facts, 1) // marker only
	assert.Equal(t, "resource_disks:queried", facts[0].Key)
	assert.Equal(t, "0 resources", facts[0].Value)
}

func TestPredictFactKeys_ResourceDisks(t *testing.T) {
	keys := PredictFactKeys("pulse_storage", map[string]interface{}{"action": "resource_disks"})
	require.Len(t, keys, 1)
	assert.Equal(t, "resource_disks:queried", keys[0])
}

// --- Docker: Services ---

func TestExtractFacts_DockerServices(t *testing.T) {
	input := map[string]interface{}{"action": "services"}
	result := `{"host":"delly","services":[{"name":"nginx","mode":"replicated","desired_tasks":3,"running_tasks":3},{"name":"redis","mode":"replicated","desired_tasks":2,"running_tasks":1}],"total":2}`

	facts := ExtractFacts("pulse_docker", input, result)
	require.Len(t, facts, 2) // marker + summary

	assert.Equal(t, "docker_services:queried", facts[0].Key)
	assert.Equal(t, "2 services", facts[0].Value)
	assert.Equal(t, FactCategoryResource, facts[0].Category)

	assert.Equal(t, "docker_services:summary", facts[1].Key)
	assert.Contains(t, facts[1].Value, "2 services")
	assert.Contains(t, facts[1].Value, "1 healthy")
	assert.Contains(t, facts[1].Value, "1 degraded")
}

func TestExtractFacts_DockerServices_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "services"}
	result := `{"host":"delly","services":[],"total":0}`

	facts := ExtractFacts("pulse_docker", input, result)
	require.Len(t, facts, 1) // marker only
	assert.Equal(t, "docker_services:queried", facts[0].Key)
	assert.Equal(t, "0 services", facts[0].Value)
}

func TestPredictFactKeys_DockerServices(t *testing.T) {
	keys := PredictFactKeys("pulse_docker", map[string]interface{}{"action": "services"})
	require.Len(t, keys, 2)
	assert.Equal(t, "docker_services:queried", keys[0])
	assert.Equal(t, "docker_services:summary", keys[1])
}

// --- Docker: Updates ---

func TestExtractFacts_DockerUpdates(t *testing.T) {
	input := map[string]interface{}{"action": "updates"}
	result := `{"updates":[{"container_name":"nginx","update_available":true},{"container_name":"redis","update_available":false}],"total":2}`

	facts := ExtractFacts("pulse_docker", input, result)
	require.Len(t, facts, 1)

	assert.Equal(t, "docker_updates:queried", facts[0].Key)
	assert.Contains(t, facts[0].Value, "2 containers")
	assert.Contains(t, facts[0].Value, "1 updates available")
}

func TestPredictFactKeys_DockerUpdates(t *testing.T) {
	keys := PredictFactKeys("pulse_docker", map[string]interface{}{"action": "updates"})
	require.Len(t, keys, 1)
	assert.Equal(t, "docker_updates:queried", keys[0])
}

// --- Docker: Swarm ---

func TestExtractFacts_DockerSwarm(t *testing.T) {
	input := map[string]interface{}{"action": "swarm"}
	result := `{"host":"delly","status":{"node_role":"manager","local_state":"active","control_available":true,"cluster_name":"prod"}}`

	facts := ExtractFacts("pulse_docker", input, result)
	require.Len(t, facts, 1)

	assert.Equal(t, "docker_swarm:status", facts[0].Key)
	assert.Contains(t, facts[0].Value, "role=manager")
	assert.Contains(t, facts[0].Value, "state=active")
	assert.Contains(t, facts[0].Value, "host=delly")
}

func TestPredictFactKeys_DockerSwarm(t *testing.T) {
	keys := PredictFactKeys("pulse_docker", map[string]interface{}{"action": "swarm"})
	require.Len(t, keys, 1)
	assert.Equal(t, "docker_swarm:status", keys[0])
}

// --- Docker: Tasks ---

func TestExtractFacts_DockerTasks(t *testing.T) {
	input := map[string]interface{}{"action": "tasks"}
	result := `{"host":"delly","service":"nginx","tasks":[{"current_state":"running"},{"current_state":"running"},{"current_state":"failed","error":"OOM killed"}],"total":3}`

	facts := ExtractFacts("pulse_docker", input, result)
	require.Len(t, facts, 1)

	assert.Equal(t, "docker_tasks:queried", facts[0].Key)
	assert.Contains(t, facts[0].Value, "service=nginx")
	assert.Contains(t, facts[0].Value, "3 tasks")
	assert.Contains(t, facts[0].Value, "2 running")
	assert.Contains(t, facts[0].Value, "1 failed")
}

func TestPredictFactKeys_DockerTasks(t *testing.T) {
	keys := PredictFactKeys("pulse_docker", map[string]interface{}{"action": "tasks"})
	require.Len(t, keys, 1)
	assert.Equal(t, "docker_tasks:queried", keys[0])
}

// --- Docker: Unknown action ---

func TestExtractFacts_DockerUnknown(t *testing.T) {
	input := map[string]interface{}{"action": "control"}
	result := `some text result`

	facts := ExtractFacts("pulse_docker", input, result)
	assert.Empty(t, facts) // control is a write action, no extractor
}

// --- Kubernetes: Clusters ---

func TestExtractFacts_K8sClusters(t *testing.T) {
	input := map[string]interface{}{"action": "clusters"}
	result := `{"clusters":[{"name":"prod","display_name":"Production","status":"healthy","node_count":3,"pod_count":42,"ready_nodes":3}],"total":1}`

	facts := ExtractFacts("pulse_kubernetes", input, result)
	require.Len(t, facts, 2) // marker + 1 cluster

	assert.Equal(t, "k8s_clusters:queried", facts[0].Key)
	assert.Equal(t, "1 clusters", facts[0].Value)

	assert.Equal(t, "k8s_cluster:Production", facts[1].Key)
	assert.Contains(t, facts[1].Value, "healthy")
	assert.Contains(t, facts[1].Value, "3 nodes")
	assert.Contains(t, facts[1].Value, "3 ready")
	assert.Contains(t, facts[1].Value, "42 pods")
}

func TestExtractFacts_K8sClusters_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "clusters"}
	result := `{"clusters":[],"total":0}`

	facts := ExtractFacts("pulse_kubernetes", input, result)
	require.Len(t, facts, 1)
	assert.Equal(t, "k8s_clusters:queried", facts[0].Key)
	assert.Equal(t, "0 clusters", facts[0].Value)
}

func TestPredictFactKeys_K8sClusters(t *testing.T) {
	keys := PredictFactKeys("pulse_kubernetes", map[string]interface{}{"action": "clusters"})
	require.Len(t, keys, 1)
	assert.Equal(t, "k8s_clusters:queried", keys[0])
}

// --- Kubernetes: Nodes ---

func TestExtractFacts_K8sNodes(t *testing.T) {
	input := map[string]interface{}{"action": "nodes"}
	result := `{"cluster":"prod","nodes":[{"name":"node1","ready":true},{"name":"node2","ready":true},{"name":"node3","ready":false}],"total":3}`

	facts := ExtractFacts("pulse_kubernetes", input, result)
	require.Len(t, facts, 1)

	assert.Equal(t, "k8s_nodes:queried", facts[0].Key)
	assert.Contains(t, facts[0].Value, "cluster=prod")
	assert.Contains(t, facts[0].Value, "3 nodes")
	assert.Contains(t, facts[0].Value, "2 ready")
	assert.Contains(t, facts[0].Value, "1 not ready")
}

func TestPredictFactKeys_K8sNodes(t *testing.T) {
	keys := PredictFactKeys("pulse_kubernetes", map[string]interface{}{"action": "nodes"})
	require.Len(t, keys, 1)
	assert.Equal(t, "k8s_nodes:queried", keys[0])
}

// --- Kubernetes: Pods ---

func TestExtractFacts_K8sPods(t *testing.T) {
	input := map[string]interface{}{"action": "pods"}
	result := `{"cluster":"prod","pods":[{"name":"nginx-1","namespace":"default","phase":"Running","restarts":0},{"name":"redis-1","namespace":"default","phase":"Running","restarts":10},{"name":"broken-1","namespace":"test","phase":"CrashLoopBackOff","restarts":0}],"total":3}`

	facts := ExtractFacts("pulse_kubernetes", input, result)
	require.Len(t, facts, 1)

	assert.Equal(t, "k8s_pods:queried", facts[0].Key)
	assert.Contains(t, facts[0].Value, "cluster=prod")
	assert.Contains(t, facts[0].Value, "3 pods")
	assert.Contains(t, facts[0].Value, "Running")
	assert.Contains(t, facts[0].Value, "1 high-restart")
}

func TestPredictFactKeys_K8sPods(t *testing.T) {
	keys := PredictFactKeys("pulse_kubernetes", map[string]interface{}{"action": "pods"})
	require.Len(t, keys, 1)
	assert.Equal(t, "k8s_pods:queried", keys[0])
}

// --- Kubernetes: Deployments ---

func TestExtractFacts_K8sDeployments(t *testing.T) {
	input := map[string]interface{}{"action": "deployments"}
	result := `{"cluster":"prod","deployments":[{"name":"nginx","namespace":"default","desired_replicas":3,"ready_replicas":3,"available_replicas":3},{"name":"redis","namespace":"default","desired_replicas":2,"ready_replicas":1,"available_replicas":1}],"total":2}`

	facts := ExtractFacts("pulse_kubernetes", input, result)
	require.Len(t, facts, 1)

	assert.Equal(t, "k8s_deployments:queried", facts[0].Key)
	assert.Contains(t, facts[0].Value, "cluster=prod")
	assert.Contains(t, facts[0].Value, "2 deployments")
	assert.Contains(t, facts[0].Value, "1 healthy")
	assert.Contains(t, facts[0].Value, "1 degraded")
}

func TestPredictFactKeys_K8sDeployments(t *testing.T) {
	keys := PredictFactKeys("pulse_kubernetes", map[string]interface{}{"action": "deployments"})
	require.Len(t, keys, 1)
	assert.Equal(t, "k8s_deployments:queried", keys[0])
}

// --- PMG: Status ---

func TestExtractFacts_PMGStatus(t *testing.T) {
	input := map[string]interface{}{"action": "status"}
	result := `{"instances":[{"name":"pmg-main","host":"10.0.0.5","status":"running","version":"8.1.2"}],"total":1}`

	facts := ExtractFacts("pulse_pmg", input, result)
	require.Len(t, facts, 2) // marker + 1 instance

	assert.Equal(t, "pmg:queried", facts[0].Key)
	assert.Equal(t, "1 instances", facts[0].Value)

	assert.Equal(t, "pmg:pmg-main", facts[1].Key)
	assert.Contains(t, facts[1].Value, "running")
	assert.Contains(t, facts[1].Value, "v8.1.2")
}

func TestExtractFacts_PMGStatus_Empty(t *testing.T) {
	input := map[string]interface{}{"action": "status"}
	result := `{"instances":[],"total":0}`

	facts := ExtractFacts("pulse_pmg", input, result)
	require.Len(t, facts, 1)
	assert.Equal(t, "pmg:queried", facts[0].Key)
	assert.Equal(t, "0 instances", facts[0].Value)
}

func TestPredictFactKeys_PMGStatus(t *testing.T) {
	keys := PredictFactKeys("pulse_pmg", map[string]interface{}{"action": "status"})
	require.Len(t, keys, 1)
	assert.Equal(t, "pmg:queried", keys[0])
}

// --- PMG: Mail Stats ---

func TestExtractFacts_PMGMailStats(t *testing.T) {
	input := map[string]interface{}{"action": "mail_stats"}
	result := `{"instance":"pmg-main","stats":{"total_in":1500,"total_out":800,"spam_in":200,"virus_in":5}}`

	facts := ExtractFacts("pulse_pmg", input, result)
	require.Len(t, facts, 1)

	assert.Equal(t, "pmg_mail_stats:queried", facts[0].Key)
	assert.Contains(t, facts[0].Value, "pmg-main")
	assert.Contains(t, facts[0].Value, "in=1500")
	assert.Contains(t, facts[0].Value, "out=800")
	assert.Contains(t, facts[0].Value, "spam=200")
	assert.Contains(t, facts[0].Value, "virus=5")
}

func TestPredictFactKeys_PMGMailStats(t *testing.T) {
	keys := PredictFactKeys("pulse_pmg", map[string]interface{}{"action": "mail_stats"})
	require.Len(t, keys, 1)
	assert.Equal(t, "pmg_mail_stats:queried", keys[0])
}

// --- PMG: Queues ---

func TestExtractFacts_PMGQueues(t *testing.T) {
	input := map[string]interface{}{"action": "queues"}
	result := `{"instance":"pmg-main","queues":[{"node":"pmg1","active":5,"deferred":12,"total":17},{"node":"pmg2","active":2,"deferred":3,"total":5}]}`

	facts := ExtractFacts("pulse_pmg", input, result)
	require.Len(t, facts, 1)

	assert.Equal(t, "pmg_queues:queried", facts[0].Key)
	assert.Contains(t, facts[0].Value, "pmg-main")
	assert.Contains(t, facts[0].Value, "2 nodes")
	assert.Contains(t, facts[0].Value, "22 queued")
	assert.Contains(t, facts[0].Value, "15 deferred")
}

func TestPredictFactKeys_PMGQueues(t *testing.T) {
	keys := PredictFactKeys("pulse_pmg", map[string]interface{}{"action": "queues"})
	require.Len(t, keys, 1)
	assert.Equal(t, "pmg_queues:queried", keys[0])
}

// --- PMG: Spam ---

func TestExtractFacts_PMGSpam(t *testing.T) {
	input := map[string]interface{}{"action": "spam"}
	result := `{"instance":"pmg-main","quarantine":{"spam":150,"virus":3,"total":153}}`

	facts := ExtractFacts("pulse_pmg", input, result)
	require.Len(t, facts, 1)

	assert.Equal(t, "pmg_spam:queried", facts[0].Key)
	assert.Contains(t, facts[0].Value, "pmg-main")
	assert.Contains(t, facts[0].Value, "153 total")
	assert.Contains(t, facts[0].Value, "150 spam")
	assert.Contains(t, facts[0].Value, "3 virus")
}

func TestPredictFactKeys_PMGSpam(t *testing.T) {
	keys := PredictFactKeys("pulse_pmg", map[string]interface{}{"action": "spam"})
	require.Len(t, keys, 1)
	assert.Equal(t, "pmg_spam:queried", keys[0])
}

// --- Patrol: Get Findings ---

func TestExtractFacts_PatrolGetFindings(t *testing.T) {
	result := `{"ok":true,"count":3,"findings":[{"key":"high-cpu","severity":"warning","title":"High CPU","resource_id":"vm101"},{"key":"disk-full","severity":"critical","title":"Disk Full","resource_id":"ct200"},{"key":"low-mem","severity":"warning","title":"Low Memory","resource_id":"vm102"}]}`

	facts := ExtractFacts("patrol_get_findings", nil, result)
	require.Len(t, facts, 2) // marker + summary

	assert.Equal(t, "patrol_findings:queried", facts[0].Key)
	assert.Equal(t, "3 findings", facts[0].Value)
	assert.Equal(t, FactCategoryFinding, facts[0].Category)

	assert.Equal(t, "patrol_findings:summary", facts[1].Key)
	assert.Contains(t, facts[1].Value, "3 total")
	assert.Contains(t, facts[1].Value, "warning")
	assert.Contains(t, facts[1].Value, "critical")
}

func TestExtractFacts_PatrolGetFindings_Empty(t *testing.T) {
	result := `{"ok":true,"count":0,"findings":[]}`

	facts := ExtractFacts("patrol_get_findings", nil, result)
	require.Len(t, facts, 1) // marker only
	assert.Equal(t, "patrol_findings:queried", facts[0].Key)
	assert.Equal(t, "0 findings", facts[0].Value)
}

func TestPredictFactKeys_PatrolGetFindings(t *testing.T) {
	keys := PredictFactKeys("patrol_get_findings", nil)
	require.Len(t, keys, 1)
	assert.Equal(t, "patrol_findings:queried", keys[0])
}

// --- Roundtrip Consistency: Predict keys must match extracted keys ---
// This test prevents the class of bug where PredictFactKeys returns keys that
// don't match what ExtractFacts actually stores — making the gate useless.

func TestPredictExtractRoundtrip(t *testing.T) {
	// Each entry: tool, input, sample result, description
	// PredictFactKeys must return at least one key that appears in ExtractFacts output.
	cases := []struct {
		name   string
		tool   string
		input  map[string]interface{}
		result string
	}{
		{"query:topology", "pulse_query", map[string]interface{}{"action": "topology"},
			`{"summary":{"total_nodes":1},"proxmox":{"nodes":[{"name":"n1","status":"online"}]}}`},
		{"query:health", "pulse_query", map[string]interface{}{"action": "health"},
			`{"connections":[{"instance_id":"n1","connected":true}]}`},
		{"query:get", "pulse_query", map[string]interface{}{"action": "get", "resource_id": "106"},
			`{"type":"lxc","name":"test","status":"running","node":"n1","id":"106","vmid":106,"cpu":{"percent":1},"memory":{"percent":50}}`},
		{"query:get:error", "pulse_query", map[string]interface{}{"action": "get", "resource_id": "999"},
			`{"error":"not_found","resource_id":"999","type":"vm"}`},
		{"query:search", "pulse_query", map[string]interface{}{"action": "search", "query": "test"},
			`{"matches":[{"name":"test","status":"running"}],"total":1}`},
		{"query:list", "pulse_query", map[string]interface{}{"action": "list"},
			`{"nodes":[{"name":"n1","status":"online"}],"total":{"nodes":1}}`},
		{"query:config", "pulse_query", map[string]interface{}{"action": "config", "resource_id": "106", "node": "n1"},
			`{"guest_type":"lxc","vmid":106,"name":"test","node":"n1"}`},
		{"query:config:no_node", "pulse_query", map[string]interface{}{"action": "config", "resource_id": "106"},
			`{"guest_type":"lxc","vmid":106,"name":"test","node":"n1"}`},
		{"storage:pools", "pulse_storage", map[string]interface{}{"action": "pools"},
			`{"pools":[{"name":"local","node":"n1","type":"dir","usage_percent":50,"total_gb":100,"used_gb":50}]}`},
		{"storage:disk_health", "pulse_storage", map[string]interface{}{"action": "disk_health"},
			`{"hosts":[{"hostname":"n1","smart":[{"device":"/dev/sda","health":"PASSED"}]}]}`},
		{"storage:raid", "pulse_storage", map[string]interface{}{"action": "raid"},
			`{"hosts":[{"hostname":"n1","arrays":[{"device":"/dev/md0","state":"clean","failed_devices":0}]}]}`},
		{"storage:backups", "pulse_storage", map[string]interface{}{"action": "backups"},
			`{"pbs":[],"pve":[],"pbs_servers":[]}`},
		{"storage:backup_tasks", "pulse_storage", map[string]interface{}{"action": "backup_tasks"},
			`{"tasks":[{"vmid":100,"node":"n1","status":"OK"}],"total":1}`},
		{"storage:ceph", "pulse_storage", map[string]interface{}{"action": "ceph"},
			`[{"name":"c1","health":"OK","details":{"osd_count":3,"osds_up":3,"osds_down":0,"monitors":1,"usage_percent":30}}]`},
		{"storage:ceph_details", "pulse_storage", map[string]interface{}{"action": "ceph_details"},
			`{"hosts":[{"hostname":"n1","health":{"status":"OK"},"osd_map":{"num_osds":3,"num_up":3},"pg_map":{"usage_percent":30},"pools":[]}]}`},
		{"storage:snapshots", "pulse_storage", map[string]interface{}{"action": "snapshots"},
			`{"snapshots":[{"vmid":100,"vm_name":"test","snapshot_name":"s1"}],"total":1}`},
		{"storage:replication", "pulse_storage", map[string]interface{}{"action": "replication"},
			`[{"id":"1","guest_id":100,"guest_name":"test","status":"ok","error":""}]`},
		{"storage:pbs_jobs", "pulse_storage", map[string]interface{}{"action": "pbs_jobs"},
			`{"jobs":[{"id":"j1","type":"backup","status":"ok"}],"total":1}`},
		{"storage:resource_disks", "pulse_storage", map[string]interface{}{"action": "resource_disks"},
			`{"resources":[{"vmid":100,"name":"test","disks":[{"mountpoint":"/","usage_percent":50}]}],"total":1}`},
		{"metrics:performance", "pulse_metrics", map[string]interface{}{"action": "performance", "resource_id": "vm101"},
			`{"summary":{"vm101":{"avg_cpu":10,"max_cpu":50,"trend":"stable"}}}`},
		{"metrics:baselines", "pulse_metrics", map[string]interface{}{"action": "baselines"},
			`{"baselines":{"n1":{"n1:100:cpu":{"mean":5,"std_dev":2,"min":0,"max":20}}}}`},
		{"metrics:disks", "pulse_metrics", map[string]interface{}{"action": "disks"},
			`{"disks":[{"host":"n1","device":"/dev/sda","health":"PASSED"}]}`},
		{"metrics:temperatures", "pulse_metrics", map[string]interface{}{"action": "temperatures"},
			`[{"hostname":"n1","cpu_temps":{"core0":50},"disk_temps":{}}]`},
		{"alerts:findings", "pulse_alerts", map[string]interface{}{"action": "findings"},
			`{"active":[{"key":"k1","severity":"warning","title":"test"}],"counts":{"active":1,"dismissed":0}}`},
		{"alerts:list", "pulse_alerts", map[string]interface{}{"action": "list"},
			`{"alerts":[{"resource_name":"vm101","type":"cpu","severity":"warning","value":90,"threshold":80,"status":"active"}]}`},
		{"docker:services", "pulse_docker", map[string]interface{}{"action": "services"},
			`{"host":"h1","services":[{"name":"nginx","desired_tasks":2,"running_tasks":2}],"total":1}`},
		{"docker:updates", "pulse_docker", map[string]interface{}{"action": "updates"},
			`{"updates":[{"container_name":"nginx","update_available":false}],"total":1}`},
		{"docker:swarm", "pulse_docker", map[string]interface{}{"action": "swarm"},
			`{"host":"h1","status":{"node_role":"manager","local_state":"active","control_available":true}}`},
		{"docker:tasks", "pulse_docker", map[string]interface{}{"action": "tasks"},
			`{"host":"h1","tasks":[{"current_state":"running"}],"total":1}`},
		{"k8s:clusters", "pulse_kubernetes", map[string]interface{}{"action": "clusters"},
			`{"clusters":[{"name":"prod","status":"healthy","node_count":3,"ready_nodes":3,"pod_count":10}],"total":1}`},
		{"k8s:nodes", "pulse_kubernetes", map[string]interface{}{"action": "nodes"},
			`{"cluster":"prod","nodes":[{"name":"n1","ready":true}],"total":1}`},
		{"k8s:pods", "pulse_kubernetes", map[string]interface{}{"action": "pods"},
			`{"cluster":"prod","pods":[{"name":"p1","phase":"Running"}],"total":1}`},
		{"k8s:deployments", "pulse_kubernetes", map[string]interface{}{"action": "deployments"},
			`{"cluster":"prod","deployments":[{"name":"d1","desired_replicas":2,"ready_replicas":2}],"total":1}`},
		{"pmg:status", "pulse_pmg", map[string]interface{}{"action": "status"},
			`{"instances":[{"name":"pmg1","status":"running"}],"total":1}`},
		{"pmg:mail_stats", "pulse_pmg", map[string]interface{}{"action": "mail_stats"},
			`{"instance":"pmg1","stats":{"total_in":100,"total_out":50,"spam_in":10,"virus_in":1}}`},
		{"pmg:queues", "pulse_pmg", map[string]interface{}{"action": "queues"},
			`{"instance":"pmg1","queues":[{"node":"n1","total":5,"deferred":2}]}`},
		{"pmg:spam", "pulse_pmg", map[string]interface{}{"action": "spam"},
			`{"instance":"pmg1","quarantine":{"spam":10,"virus":1,"total":11}}`},
		{"patrol:get_findings", "patrol_get_findings", nil,
			`{"ok":true,"count":1,"findings":[{"key":"k1","severity":"warning","title":"test"}]}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			predictedKeys := PredictFactKeys(tc.tool, tc.input)
			require.NotNil(t, predictedKeys, "PredictFactKeys should return keys for %s", tc.name)

			extractedFacts := ExtractFacts(tc.tool, tc.input, tc.result)
			require.NotEmpty(t, extractedFacts, "ExtractFacts should return facts for %s", tc.name)

			// Build set of extracted keys
			extractedKeys := make(map[string]bool)
			for _, f := range extractedFacts {
				extractedKeys[f.Key] = true
			}

			// At least one predicted key must appear in extracted keys
			matched := false
			for _, pk := range predictedKeys {
				if extractedKeys[pk] {
					matched = true
					break
				}
			}
			assert.True(t, matched,
				"PredictFactKeys %v must match at least one ExtractFacts key %v for %s",
				predictedKeys, keys(extractedKeys), tc.name)
		})
	}
}

// keys extracts map keys as a slice for readable test output.
func keys(m map[string]bool) []string {
	var result []string
	for k := range m {
		result = append(result, k)
	}
	return result
}

// --- End-to-end gate flow test ---
// Simulates the full cycle: extract facts → store in KA → predict → lookup → gate fires.
// Also validates MarkerExpansion enrichment for marker-based extractors.

func TestGateFlowEndToEnd(t *testing.T) {
	cases := []struct {
		name         string
		tool         string
		input        map[string]interface{}
		result       string
		expectEnrich bool // whether MarkerExpansions should add related facts
	}{
		{"storage:pools", "pulse_storage", map[string]interface{}{"action": "pools"},
			`{"pools":[{"name":"local","node":"n1","type":"dir","usage_percent":50,"total_gb":100,"used_gb":50}]}`,
			true}, // Marker expansion: storage:pools:queried → storage:
		{"storage:ceph", "pulse_storage", map[string]interface{}{"action": "ceph"},
			`[{"name":"c1","health":"OK","details":{"osd_count":3,"osds_up":3,"osds_down":0,"monitors":1,"usage_percent":30}}]`,
			true}, // Marker expansion: ceph:queried → ceph:
		{"k8s:clusters", "pulse_kubernetes", map[string]interface{}{"action": "clusters"},
			`{"clusters":[{"name":"prod","status":"healthy","node_count":3,"ready_nodes":3,"pod_count":10}],"total":1}`,
			true}, // Marker expansion: k8s_clusters:queried → k8s_cluster:
		{"pmg:status", "pulse_pmg", map[string]interface{}{"action": "status"},
			`{"instances":[{"name":"pmg1","status":"running"}],"total":1}`,
			true}, // Marker expansion: pmg:queried → pmg:
		{"query:get", "pulse_query", map[string]interface{}{"action": "get", "resource_id": "106"},
			`{"type":"lxc","name":"test","status":"running","node":"n1","id":"106","vmid":106,"cpu":{"percent":1},"memory":{"percent":50}}`,
			false}, // No marker expansion for query:get:106:cached
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ka := NewKnowledgeAccumulator()

			// Step 1: Extract facts from tool result
			facts := ExtractFacts(tc.tool, tc.input, tc.result)
			require.NotEmpty(t, facts, "should extract facts")

			// Step 2: Store facts in KA (as agentic.go does)
			for _, f := range facts {
				ka.AddFact(f.Category, f.Key, f.Value)
			}

			// Step 3: Predict keys (as gate does on second call)
			predictedKeys := PredictFactKeys(tc.tool, tc.input)
			require.NotNil(t, predictedKeys, "should predict keys")

			// Step 4: Gate lookup — at least one predicted key should be found
			var cachedParts []string
			for _, key := range predictedKeys {
				if value, found := ka.Lookup(key); found {
					cachedParts = append(cachedParts, fmt.Sprintf("%s = %s", key, value))
				}
			}
			require.NotEmpty(t, cachedParts, "gate should fire (cached facts found)")

			// Step 5: Enrichment via MarkerExpansions
			if tc.expectEnrich {
				enriched := false
				for _, key := range predictedKeys {
					if prefix, ok := MarkerExpansions[key]; ok {
						related := ka.RelatedFacts(prefix)
						if related != "" {
							enriched = true
							// RelatedFacts should not include the marker itself
							assert.NotContains(t, related, ":queried",
								"RelatedFacts should exclude marker keys")
						}
					}
				}
				assert.True(t, enriched, "MarkerExpansion should enrich with related facts")
			}
		})
	}
}

// --- Negative marker gate test ---
// Verifies that when ExtractFacts returns nil (text/error response),
// the negative marker prevents re-execution on the next call.

func TestNegativeMarkerGateFlow(t *testing.T) {
	ka := NewKnowledgeAccumulator()

	tool := "pulse_storage"
	input := map[string]interface{}{"action": "ceph"}
	textResult := "No Ceph clusters configured on this system"

	// ExtractFacts should return nil for plain text
	facts := ExtractFacts(tool, input, textResult)
	assert.Empty(t, facts)

	// Simulate negative marker storage (as agentic.go does)
	predictedKeys := PredictFactKeys(tool, input)
	require.NotEmpty(t, predictedKeys)
	for _, key := range predictedKeys {
		if _, found := ka.Lookup(key); !found {
			cat := categoryForPredictedKey(key)
			summary := textResult
			if len(summary) > 120 {
				summary = summary[:120]
			}
			ka.AddFact(cat, key, fmt.Sprintf("checked: %s", summary))
		}
	}

	// Now simulate second call — gate should fire
	predictedKeys2 := PredictFactKeys(tool, input)
	var cachedParts []string
	for _, key := range predictedKeys2 {
		if value, found := ka.Lookup(key); found {
			cachedParts = append(cachedParts, value)
		}
	}
	require.NotEmpty(t, cachedParts, "gate should fire on second call due to negative marker")
	assert.Contains(t, cachedParts[0], "checked:", "negative marker value should start with 'checked:'")
}
