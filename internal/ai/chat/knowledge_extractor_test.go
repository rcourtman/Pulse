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
	// pulse_query get/search keys depend on result data
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
	result := `{"findings":[{"key":"high-cpu-vm101","severity":"warning","title":"High CPU on vm101","status":"active","resource_id":"vm101"},{"key":"disk-full-ct200","severity":"critical","title":"Disk full on ct200","status":"active","resource_id":"ct200"},{"key":"old-issue","severity":"info","title":"Old issue","status":"dismissed","resource_id":"ct300"}]}`

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
	assert.Contains(t, findingKeys, "finding:high-cpu-vm101")
	assert.Contains(t, findingKeys, "finding:disk-full-ct200")
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
	keys := PredictFactKeys("pulse_metrics", map[string]interface{}{"action": "baselines", "resource_id": "vm101"})
	require.Len(t, keys, 1)
	assert.Equal(t, "baseline:vm101", keys[0])
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
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Equal(t, FactCategoryResource, f.Category)
	assert.Equal(t, "config:delly:106", f.Key)
	assert.Contains(t, f.Value, "lxc")
	assert.Contains(t, f.Value, "hostname=postfix")
	assert.Contains(t, f.Value, "os=ubuntu")
	assert.Contains(t, f.Value, "onboot=yes")
	assert.Contains(t, f.Value, "2 mounts")
	assert.Contains(t, f.Value, "1 disks")
}

func TestExtractFacts_QueryConfig_VM(t *testing.T) {
	input := map[string]interface{}{"action": "config", "resource_id": "200"}
	onbootFalse := false
	_ = onbootFalse
	result := `{"guest_type":"qemu","vmid":200,"name":"win10","node":"minipc","os_type":"win10","onboot":false,"disks":[{"key":"scsi0"},{"key":"scsi1"}]}`

	facts := ExtractFacts("pulse_query", input, result)
	require.Len(t, facts, 1)

	f := facts[0]
	assert.Equal(t, "config:minipc:200", f.Key)
	assert.Contains(t, f.Value, "qemu")
	assert.Contains(t, f.Value, "onboot=no")
	assert.Contains(t, f.Value, "2 disks")
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
	require.Len(t, keys, 1)
	assert.Equal(t, "config:delly:106", keys[0])
}

func TestPredictFactKeys_QueryConfig_NoNode(t *testing.T) {
	// Without node, can't predict exact key
	keys := PredictFactKeys("pulse_query", map[string]interface{}{
		"action":      "config",
		"resource_id": "106",
	})
	assert.Nil(t, keys)
}

// --- Change 4: PredictFactKeys for query:get ---

func TestPredictFactKeys_QueryGet_ReturnsNil(t *testing.T) {
	// get success key depends on response data (type:node:id:status), can't predict
	keys := PredictFactKeys("pulse_query", map[string]interface{}{
		"action":      "get",
		"resource_id": "106",
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
		{"unknown:key", FactCategoryResource}, // default
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.expected, categoryForPredictedKey(tt.key))
		})
	}
}
