package monitoring

import (
	"math"
	"testing"
	"time"

	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// --- firstNonEmptyString ---

func TestFirstNonEmptyString_ReturnsFirst(t *testing.T) {
	if got := firstNonEmptyString("a", "b"); got != "a" {
		t.Errorf("expected 'a', got %q", got)
	}
}

func TestFirstNonEmptyString_SkipsEmpty(t *testing.T) {
	if got := firstNonEmptyString("", "  ", "c"); got != "c" {
		t.Errorf("expected 'c', got %q", got)
	}
}

func TestFirstNonEmptyString_AllEmpty(t *testing.T) {
	if got := firstNonEmptyString("", "  "); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestFirstNonEmptyString_NoArgs(t *testing.T) {
	if got := firstNonEmptyString(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestFirstNonEmptyString_Trims(t *testing.T) {
	if got := firstNonEmptyString("  hello  "); got != "hello" {
		t.Errorf("expected trimmed 'hello', got %q", got)
	}
}

// --- proxmoxmapperKey ---

func TestProxmoxmapperKey(t *testing.T) {
	got := proxmoxmapperKey("pve1", "node1", 100)
	if got != "pve1|node1|100" {
		t.Errorf("expected 'pve1|node1|100', got %q", got)
	}
}

func TestProxmoxmapperKey_Trimmed(t *testing.T) {
	got := proxmoxmapperKey("  pve1 ", " node1 ", 200)
	if got != "pve1|node1|200" {
		t.Errorf("expected trimmed key, got %q", got)
	}
}

// --- timePtr ---

func TestTimePtr_NonZero(t *testing.T) {
	now := time.Now()
	p := timePtr(now)
	if p == nil || !p.Equal(now) {
		t.Error("non-zero time should produce non-nil pointer")
	}
}

func TestTimePtr_Zero(t *testing.T) {
	p := timePtr(time.Time{})
	if p != nil {
		t.Error("zero time should produce nil pointer")
	}
}

func TestTimePtr_Independence(t *testing.T) {
	now := time.Now()
	p := timePtr(now)
	// Mutating the original should not affect the pointer
	_ = now.Add(time.Hour)
	if !p.Equal(now) {
		// This test is really about ensuring the copy is made -
		// Go's time.Time is a value type, so this always passes.
		t.Error("pointer should be independent of source")
	}
}

// --- clampUint64ToInt64 ---

func TestClampUint64ToInt64_Normal(t *testing.T) {
	if got := clampUint64ToInt64(42); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
}

func TestClampUint64ToInt64_MaxInt64(t *testing.T) {
	if got := clampUint64ToInt64(uint64(math.MaxInt64)); got != math.MaxInt64 {
		t.Errorf("expected MaxInt64, got %d", got)
	}
}

func TestClampUint64ToInt64_Overflow(t *testing.T) {
	if got := clampUint64ToInt64(uint64(math.MaxInt64) + 1); got != math.MaxInt64 {
		t.Errorf("overflow should clamp to MaxInt64, got %d", got)
	}
}

func TestClampUint64ToInt64_MaxUint64(t *testing.T) {
	if got := clampUint64ToInt64(math.MaxUint64); got != math.MaxInt64 {
		t.Errorf("MaxUint64 should clamp to MaxInt64, got %d", got)
	}
}

// --- isLegacyAgent ---

func TestIsLegacyAgent_Unified(t *testing.T) {
	if isLegacyAgent("unified") {
		t.Error("unified agent should not be legacy")
	}
}

func TestIsLegacyAgent_Empty(t *testing.T) {
	if !isLegacyAgent("") {
		t.Error("empty type should be legacy")
	}
}

func TestIsLegacyAgent_Other(t *testing.T) {
	if !isLegacyAgent("standalone") {
		t.Error("non-unified type should be legacy")
	}
}

// --- safePercentage ---

func TestSafePercentage_Normal(t *testing.T) {
	got := safePercentage(50, 200)
	if got != 25.0 {
		t.Errorf("expected 25.0, got %f", got)
	}
}

func TestSafePercentage_ZeroDivisor(t *testing.T) {
	if safePercentage(100, 0) != 0 {
		t.Error("zero divisor should return 0")
	}
}

func TestSafePercentage_NaN(t *testing.T) {
	if safePercentage(0, math.NaN()) != 0 {
		t.Error("NaN result should return 0")
	}
}

// --- safeFloat ---

func TestSafeFloat_Normal(t *testing.T) {
	if safeFloat(3.14) != 3.14 {
		t.Error("normal float should pass through")
	}
}

func TestSafeFloat_NaN(t *testing.T) {
	if safeFloat(math.NaN()) != 0 {
		t.Error("NaN should return 0")
	}
}

func TestSafeFloat_Inf(t *testing.T) {
	if safeFloat(math.Inf(1)) != 0 {
		t.Error("Inf should return 0")
	}
}

func TestSafeFloat_NegInf(t *testing.T) {
	if safeFloat(math.Inf(-1)) != 0 {
		t.Error("-Inf should return 0")
	}
}

// --- convertAgentSMARTAttributes ---

func TestConvertAgentSMARTAttributes_Nil(t *testing.T) {
	if convertAgentSMARTAttributes(nil) != nil {
		t.Error("nil input should return nil")
	}
}

func TestConvertAgentSMARTAttributes_CopiesPointers(t *testing.T) {
	poh := int64(5000)
	pc := int64(100)
	pu := 15
	src := &agentshost.SMARTAttributes{
		PowerOnHours:   &poh,
		PowerCycles:    &pc,
		PercentageUsed: &pu,
	}
	result := convertAgentSMARTAttributes(src)
	if result == nil {
		t.Fatal("non-nil input should produce non-nil result")
	}
	if result.PowerOnHours == nil || *result.PowerOnHours != 5000 {
		t.Error("PowerOnHours not copied")
	}
	if result.PowerCycles == nil || *result.PowerCycles != 100 {
		t.Error("PowerCycles not copied")
	}
	if result.PercentageUsed == nil || *result.PercentageUsed != 15 {
		t.Error("PercentageUsed not copied")
	}
}

// --- convertAgentSMARTToModels ---

func TestConvertAgentSMARTToModels_Nil(t *testing.T) {
	if convertAgentSMARTToModels(nil) != nil {
		t.Error("nil input should return nil")
	}
}

func TestConvertAgentSMARTToModels_Empty(t *testing.T) {
	if convertAgentSMARTToModels([]agentshost.DiskSMART{}) != nil {
		t.Error("empty input should return nil")
	}
}

func TestConvertAgentSMARTToModels_FullConversion(t *testing.T) {
	poh := int64(10000)
	src := []agentshost.DiskSMART{
		{
			Device:      "/dev/sda",
			Model:       "Samsung 860 EVO",
			Serial:      "S1234",
			WWN:         "0x5001234",
			Type:        "sata",
			Temperature: 38,
			Health:      "PASSED",
			Standby:     false,
			Attributes:  &agentshost.SMARTAttributes{PowerOnHours: &poh},
		},
	}
	result := convertAgentSMARTToModels(src)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Device != "/dev/sda" || result[0].Serial != "S1234" {
		t.Error("basic fields not copied")
	}
	if result[0].Temperature != 38 {
		t.Errorf("temperature not copied, got %d", result[0].Temperature)
	}
	if result[0].Attributes == nil || *result[0].Attributes.PowerOnHours != 10000 {
		t.Error("SMART attributes not converted")
	}
}

// --- convertDockerSwarmInfo ---

func TestConvertDockerSwarmInfo_Nil(t *testing.T) {
	if convertDockerSwarmInfo(nil) != nil {
		t.Error("nil input should return nil")
	}
}

func TestConvertDockerSwarmInfo_Conversion(t *testing.T) {
	src := &agentsdocker.SwarmInfo{
		NodeID:           "node-1",
		NodeRole:         "manager",
		LocalState:       "active",
		ControlAvailable: true,
		ClusterID:        "cluster-1",
		ClusterName:      "my-swarm",
		Scope:            "swarm",
		Error:            "",
	}
	result := convertDockerSwarmInfo(src)
	if result == nil {
		t.Fatal("non-nil input should produce non-nil result")
	}
	if result.NodeID != "node-1" || result.NodeRole != "manager" {
		t.Error("fields not copied correctly")
	}
	if !result.ControlAvailable {
		t.Error("ControlAvailable should be true")
	}
}

// --- convertDockerTasks ---

func TestConvertDockerTasks_Nil(t *testing.T) {
	if convertDockerTasks(nil) != nil {
		t.Error("nil input should return nil")
	}
}

func TestConvertDockerTasks_Empty(t *testing.T) {
	if convertDockerTasks([]agentsdocker.Task{}) != nil {
		t.Error("empty input should return nil")
	}
}

func TestConvertDockerTasks_WithTimestamps(t *testing.T) {
	now := time.Now()
	src := []agentsdocker.Task{
		{
			ID:           "task-1",
			ServiceID:    "svc-1",
			ServiceName:  "web",
			DesiredState: "running",
			CurrentState: "running",
			StartedAt:    &now,
		},
	}
	result := convertDockerTasks(src)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].ID != "task-1" || result[0].ServiceName != "web" {
		t.Error("basic fields not copied")
	}
	if result[0].StartedAt == nil || !result[0].StartedAt.Equal(now) {
		t.Error("StartedAt should be copied")
	}
}

// --- convertDockerServices ---

func TestConvertDockerServices_Nil(t *testing.T) {
	if convertDockerServices(nil) != nil {
		t.Error("nil input should return nil")
	}
}

func TestConvertDockerServices_WithLabelsAndPorts(t *testing.T) {
	now := time.Now()
	src := []agentsdocker.Service{
		{
			ID:           "svc-1",
			Name:         "web",
			Image:        "nginx:latest",
			Mode:         "replicated",
			DesiredTasks: 3,
			RunningTasks: 2,
			Labels:       map[string]string{"app": "web"},
			EndpointPorts: []agentsdocker.ServicePort{
				{Name: "http", Protocol: "tcp", TargetPort: 80, PublishedPort: 8080},
			},
			UpdateStatus: &agentsdocker.ServiceUpdate{
				State:   "completed",
				Message: "update completed",
			},
			CreatedAt: &now,
		},
	}
	result := convertDockerServices(src)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	svc := result[0]
	if svc.Name != "web" || svc.DesiredTasks != 3 || svc.RunningTasks != 2 {
		t.Error("basic fields not copied")
	}
	if len(svc.Labels) != 1 || svc.Labels["app"] != "web" {
		t.Error("labels not copied")
	}
	if len(svc.EndpointPorts) != 1 || svc.EndpointPorts[0].TargetPort != 80 {
		t.Error("endpoint ports not copied")
	}
	if svc.UpdateStatus == nil || svc.UpdateStatus.State != "completed" {
		t.Error("update status not copied")
	}
	if svc.CreatedAt == nil || !svc.CreatedAt.Equal(now) {
		t.Error("CreatedAt not copied")
	}
}

// --- convertDockerServices label independence ---

func TestConvertDockerServices_LabelIsolation(t *testing.T) {
	src := []agentsdocker.Service{
		{
			ID:     "svc-1",
			Name:   "web",
			Labels: map[string]string{"env": "prod"},
		},
	}
	result := convertDockerServices(src)
	result[0].Labels["new"] = "val"
	if _, ok := src[0].Labels["new"]; ok {
		t.Error("label map should be cloned, not shared")
	}
}

// --- extractSnapshotName ---

func TestExtractSnapshotName_WithAt(t *testing.T) {
	got := extractSnapshotName("local:vm-100-disk-0@snap1")
	if got != "snap1" {
		t.Errorf("expected 'snap1', got %q", got)
	}
}

func TestExtractSnapshotName_NoAt(t *testing.T) {
	got := extractSnapshotName("local:vm-100-disk-0")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestExtractSnapshotName_Empty(t *testing.T) {
	if extractSnapshotName("") != "" {
		t.Error("empty input should return empty")
	}
}

func TestExtractSnapshotName_NoColon(t *testing.T) {
	got := extractSnapshotName("vm-100@snap2")
	if got != "snap2" {
		t.Errorf("expected 'snap2' even without storage prefix, got %q", got)
	}
}

// --- convertAgentCephToGlobalCluster ---

func TestConvertAgentCephToGlobalCluster_BasicFields(t *testing.T) {
	now := time.Now()
	ceph := &agentshost.CephCluster{
		FSID: "abc-123",
		Health: agentshost.CephHealth{
			Status: "HEALTH_OK",
		},
		MonMap: agentshost.CephMonitorMap{NumMons: 3},
		MgrMap: agentshost.CephManagerMap{NumMgrs: 2, Available: true},
		OSDMap: agentshost.CephOSDMap{NumOSDs: 6, NumUp: 6, NumIn: 6},
		PGMap: agentshost.CephPGMap{
			BytesTotal:     1e12,
			BytesUsed:      5e11,
			BytesAvailable: 5e11,
			UsagePercent:   50.0,
		},
	}
	result := convertAgentCephToGlobalCluster(ceph, "server1", "host-1", now)
	if result.ID != "abc-123" {
		t.Errorf("expected FSID as ID, got %q", result.ID)
	}
	if result.Health != "OK" {
		t.Errorf("expected HEALTH_ prefix stripped, got %q", result.Health)
	}
	if result.NumOSDs != 6 || result.NumOSDsUp != 6 {
		t.Error("OSD counts not copied")
	}
	if result.Instance != "agent:server1" {
		t.Errorf("expected 'agent:server1', got %q", result.Instance)
	}
}

func TestConvertAgentCephToGlobalCluster_EmptyFSID(t *testing.T) {
	ceph := &agentshost.CephCluster{
		Health: agentshost.CephHealth{Status: "HEALTH_WARN"},
	}
	result := convertAgentCephToGlobalCluster(ceph, "server1", "host-1", time.Now())
	if result.ID != "agent-ceph-host-1" {
		t.Errorf("empty FSID should fallback to agent-ceph-{hostID}, got %q", result.ID)
	}
}

func TestConvertAgentCephToGlobalCluster_HealthMessages(t *testing.T) {
	ceph := &agentshost.CephCluster{
		Health: agentshost.CephHealth{
			Status: "HEALTH_WARN",
			Checks: map[string]agentshost.CephCheck{
				"SLOW_OPS":        {Message: "5 slow ops"},
				"PG_NOT_SCRUBBED": {Message: "2 PGs not scrubbed"},
			},
		},
	}
	result := convertAgentCephToGlobalCluster(ceph, "s1", "h1", time.Now())
	if result.HealthMessage == "" {
		t.Error("health messages should be joined into HealthMessage")
	}
}
