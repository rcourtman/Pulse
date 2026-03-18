package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestTriageThresholdChecks(t *testing.T) {
	state := patrolRuntimeState{
		Nodes: []models.Node{
			{
				ID:     "node/pve1",
				Name:   "pve1",
				CPU:    0.92,                       // CPU is 0-1 scale (92%)
				Memory: models.Memory{Usage: 30.0}, // Memory.Usage is already percent
			},
		},
	}

	flags := triageThresholdChecksState(state, nil, DefaultPatrolThresholds())
	if len(flags) == 0 {
		t.Fatalf("expected threshold flags, got none")
	}

	flag := triageFindFlag(flags, func(f TriageFlag) bool {
		return f.ResourceID == "node/pve1" && f.Metric == "cpu"
	})
	if flag == nil {
		t.Fatalf("expected CPU flag for node/pve1")
	}
	if flag.Severity != "warning" {
		t.Fatalf("expected warning severity, got %q", flag.Severity)
	}
}

func TestTriageThresholdChecksState_UsesReadStateWhenLegacySlicesEmpty(t *testing.T) {
	state := patrolRuntimeState{
		readState: &mockReadState{
			nodes: []*unifiedresources.NodeView{
				func() *unifiedresources.NodeView {
					node := unifiedresources.NewNodeView(&unifiedresources.Resource{
						ID:     "node/pve1",
						Name:   "pve1",
						Type:   unifiedresources.ResourceTypeAgent,
						Status: unifiedresources.StatusOnline,
						Proxmox: &unifiedresources.ProxmoxData{
							NodeName: "pve1",
						},
						Metrics: &unifiedresources.ResourceMetrics{
							CPU:    &unifiedresources.MetricValue{Percent: 92},
							Memory: &unifiedresources.MetricValue{Percent: 30},
						},
					})
					return &node
				}(),
			},
			vms: []*unifiedresources.VMView{
				func() *unifiedresources.VMView {
					vm := unifiedresources.NewVMView(&unifiedresources.Resource{
						ID:     "qemu/100",
						Name:   "vm-1",
						Type:   unifiedresources.ResourceTypeVM,
						Status: "running",
						Proxmox: &unifiedresources.ProxmoxData{
							SourceID: "qemu/100",
							VMID:     100,
							NodeName: "pve1",
						},
						Metrics: &unifiedresources.ResourceMetrics{
							CPU:    &unifiedresources.MetricValue{Percent: 70},
							Memory: &unifiedresources.MetricValue{Percent: 92},
							Disk:   &unifiedresources.MetricValue{Percent: 85},
						},
					})
					return &vm
				}(),
			},
			storage: []*unifiedresources.StoragePoolView{
				func() *unifiedresources.StoragePoolView {
					storage := unifiedresources.NewStoragePoolView(&unifiedresources.Resource{
						ID:     "storage/local",
						Name:   "local",
						Type:   unifiedresources.ResourceTypeStorage,
						Status: unifiedresources.StatusOnline,
						Metrics: &unifiedresources.ResourceMetrics{
							Disk: &unifiedresources.MetricValue{Percent: 96},
						},
					})
					return &storage
				}(),
			},
		},
	}

	flags := triageThresholdChecksState(state, nil, DefaultPatrolThresholds())
	if len(flags) == 0 {
		t.Fatalf("expected threshold flags from readState, got none")
	}

	if triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "node/pve1" && f.Metric == "cpu" }) == nil {
		t.Fatalf("expected node CPU flag from readState")
	}
	if triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "qemu/100" && f.Metric == "memory" }) == nil {
		t.Fatalf("expected VM memory flag from readState")
	}
	if triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "storage/local" && f.Metric == "usage" }) == nil {
		t.Fatalf("expected storage usage flag from readState")
	}
}

func TestTriageAnomalyChecks(t *testing.T) {
	intel := seedIntelligence{
		anomalies: []baseline.AnomalyReport{
			{
				ResourceID:   "qemu/100",
				ResourceName: "web-01",
				ResourceType: "vm",
				Metric:       "cpu",
				CurrentValue: 0.65,
				BaselineMean: 0.22,
				ZScore:       3.2,
				Severity:     baseline.AnomalyHigh,
			},
			{
				ResourceID:   "lxc/200",
				ResourceName: "worker-ct",
				ResourceType: "system-container",
				Metric:       "memory",
				CurrentValue: 0.82,
				BaselineMean: 0.62,
				ZScore:       2.7,
				Severity:     baseline.AnomalyMedium,
			},
			{
				ResourceID:   "node/pve2",
				ResourceName: "pve2",
				ResourceType: "node",
				Metric:       "cpu",
				CurrentValue: 0.51,
				BaselineMean: 0.40,
				ZScore:       2.2,
				Severity:     baseline.AnomalyLow,
			},
		},
		forecasts: []seedForecast{
			{name: "local", resourceID: "storage/local", metric: "disk", daysToFull: 12, dailyChange: 2.1, current: 91},
			{name: "fast", resourceID: "storage/fast", metric: "disk", daysToFull: 5, dailyChange: 4.3, current: 95},
		},
	}

	flags := triageAnomalyChecks(intel)
	if len(flags) != 4 {
		t.Fatalf("expected 4 anomaly/capacity flags, got %d", len(flags))
	}

	high := triageFindFlag(flags, func(f TriageFlag) bool {
		return f.ResourceID == "qemu/100" && f.Category == "anomaly"
	})
	if high == nil || high.Severity != "warning" {
		t.Fatalf("expected high anomaly warning for qemu/100, got %#v", high)
	}

	medium := triageFindFlag(flags, func(f TriageFlag) bool {
		return f.ResourceID == "lxc/200" && f.Category == "anomaly"
	})
	if medium == nil || medium.Severity != "watch" {
		t.Fatalf("expected medium anomaly watch for lxc/200, got %#v", medium)
	}
}

func TestTriageBackupChecks(t *testing.T) {
	oldBackup := time.Now().Add(-72 * time.Hour)
	state := patrolRuntimeState{
		VMs: []models.VM{
			{ID: "qemu/100", Name: "vm-never", Status: "running", Template: false, LastBackup: time.Time{}},
		},
		Containers: []models.Container{
			{ID: "lxc/200", Name: "ct-stale", Status: "running", Template: false, LastBackup: oldBackup},
		},
	}

	flags := triageBackupChecksState(state, nil)
	if len(flags) != 2 {
		t.Fatalf("expected 2 backup flags, got %d", len(flags))
	}

	never := triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "qemu/100" })
	if never == nil || !strings.Contains(never.Reason, "Never backed up") {
		t.Fatalf("expected never-backed-up flag for qemu/100, got %#v", never)
	}

	stale := triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "lxc/200" })
	if stale == nil || !strings.Contains(stale.Reason, "threshold: 48h") {
		t.Fatalf("expected stale-backup flag for lxc/200, got %#v", stale)
	}
}

func TestTriageBackupChecksState_UsesReadStateWhenLegacySlicesEmpty(t *testing.T) {
	oldBackup := time.Now().Add(-72 * time.Hour)
	state := patrolRuntimeState{
		readState: &mockReadState{
			vms: []*unifiedresources.VMView{
				func() *unifiedresources.VMView {
					vm := unifiedresources.NewVMView(&unifiedresources.Resource{
						ID:     "reg-qemu/100",
						Name:   "vm-never",
						Type:   unifiedresources.ResourceTypeVM,
						Status: "running",
						Proxmox: &unifiedresources.ProxmoxData{
							SourceID: "qemu/100",
							VMID:     100,
							NodeName: "pve1",
						},
					})
					return &vm
				}(),
			},
			containers: []*unifiedresources.ContainerView{
				func() *unifiedresources.ContainerView {
					ct := unifiedresources.NewContainerView(&unifiedresources.Resource{
						ID:     "reg-lxc/200",
						Name:   "ct-stale",
						Type:   unifiedresources.ResourceTypeSystemContainer,
						Status: "running",
						Proxmox: &unifiedresources.ProxmoxData{
							SourceID:   "lxc/200",
							VMID:       200,
							NodeName:   "pve1",
							LastBackup: oldBackup,
						},
					})
					return &ct
				}(),
			},
		},
	}

	flags := triageBackupChecksState(state, nil)
	if len(flags) != 2 {
		t.Fatalf("expected 2 backup flags from readState, got %d", len(flags))
	}

	never := triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "reg-qemu/100" })
	if never == nil || !strings.Contains(never.Reason, "Never backed up") {
		t.Fatalf("expected never-backed-up readState flag, got %#v", never)
	}

	stale := triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "reg-lxc/200" })
	if stale == nil || !strings.Contains(stale.Reason, "threshold: 48h") {
		t.Fatalf("expected stale readState backup flag, got %#v", stale)
	}
}

func TestTriageDiskHealthChecks(t *testing.T) {
	state := patrolRuntimeState{
		PhysicalDisks: []models.PhysicalDisk{
			{ID: "disk-1", DevPath: "/dev/sda", Health: "FAILED", Wearout: 50, Temperature: 40},
			{ID: "disk-2", DevPath: "/dev/nvme0n1", Health: "PASSED", Wearout: 15, Temperature: 40},
			{ID: "disk-3", DevPath: "/dev/sdb", Health: "PASSED", Wearout: -1, Temperature: 60},
			{ID: "disk-4", DevPath: "/dev/sdc", Health: "PASSED", Wearout: 85, Temperature: 40},
		},
	}

	flags := triageDiskHealthChecksState(state, nil)
	if len(flags) != 3 {
		t.Fatalf("expected 3 disk-health flags, got %d", len(flags))
	}

	if triageFindFlag(flags, func(f TriageFlag) bool {
		return f.Severity == "critical" && strings.Contains(f.Reason, "Disk health reported FAILED")
	}) == nil {
		t.Fatalf("expected critical health flag for failed disk")
	}
	if triageFindFlag(flags, func(f TriageFlag) bool {
		return f.Severity == "warning" && strings.Contains(f.Reason, "wearout at 15% remaining")
	}) == nil {
		t.Fatalf("expected wearout warning for 15%% remaining")
	}
	if triageFindFlag(flags, func(f TriageFlag) bool {
		return f.Severity == "warning" && strings.Contains(f.Reason, "Disk temperature 60")
	}) == nil {
		t.Fatalf("expected temperature warning for 60C disk")
	}
	if triageFindFlag(flags, func(f TriageFlag) bool { return strings.Contains(f.Reason, "wearout at 85% remaining") }) != nil {
		t.Fatalf("did not expect wearout flag for disk-4 (85%% remaining)")
	}
}

func TestTriageDiskHealthChecksState_PrefersUnifiedProvider(t *testing.T) {
	state := patrolRuntimeState{
		unifiedResourceProvider: &mockUnifiedResourceProvider{
			getByTypeFunc: func(t unifiedresources.ResourceType) []unifiedresources.Resource {
				if t != unifiedresources.ResourceTypePhysicalDisk {
					return nil
				}
				return []unifiedresources.Resource{
					{
						ID:   "disk-1",
						Name: "disk-1",
						Type: unifiedresources.ResourceTypePhysicalDisk,
						PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
							DevPath:     "/dev/sda",
							Model:       "disk-1",
							Health:      "FAILED",
							Wearout:     50,
							Temperature: 40,
						},
					},
				}
			},
		},
	}

	flags := triageDiskHealthChecksState(state, nil)
	if len(flags) != 1 {
		t.Fatalf("expected 1 disk-health flag from unified provider, got %d", len(flags))
	}
	if flags[0].ResourceType != "physical_disk" {
		t.Fatalf("expected unified-provider disk flag to use physical_disk, got %#v", flags[0])
	}
	if !strings.Contains(flags[0].Reason, "Disk health reported FAILED") {
		t.Fatalf("expected unified-provider flag to use disk health reason, got %#v", flags[0])
	}
}

func TestTriageDiskHealthChecksState_UsesPhysicalDiskFallback(t *testing.T) {
	state := patrolRuntimeState{
		PhysicalDisks: []models.PhysicalDisk{
			{ID: "disk-1", DevPath: "/dev/sda", Health: "FAILED", Wearout: 50, Temperature: 40},
			{ID: "disk-2", DevPath: "/dev/nvme0n1", Health: "PASSED", Wearout: 15, Temperature: 40},
			{ID: "disk-3", DevPath: "/dev/sdb", Health: "PASSED", Wearout: -1, Temperature: 60},
		},
	}

	flags := triageDiskHealthChecksState(state, nil)
	if len(flags) != 3 {
		t.Fatalf("expected 3 disk-health flags from physical-disk fallback, got %d", len(flags))
	}
	if triageFindFlag(flags, func(f TriageFlag) bool {
		return f.ResourceID == "disk-1" && f.ResourceType == "physical_disk" && strings.Contains(f.Reason, "Disk health reported FAILED")
	}) == nil {
		t.Fatalf("expected failed health flag from physical-disk fallback, got %#v", flags)
	}
	if triageFindFlag(flags, func(f TriageFlag) bool {
		return f.ResourceID == "disk-2" && f.ResourceType == "physical_disk" && strings.Contains(f.Reason, "wearout at 15% remaining")
	}) == nil {
		t.Fatalf("expected wearout flag from physical-disk fallback, got %#v", flags)
	}
	if triageFindFlag(flags, func(f TriageFlag) bool {
		return f.ResourceID == "disk-3" && f.ResourceType == "physical_disk" && strings.Contains(f.Reason, "Disk temperature 60")
	}) == nil {
		t.Fatalf("expected temperature flag from physical-disk fallback, got %#v", flags)
	}
}

func TestTriageAlertChecks(t *testing.T) {
	state := patrolRuntimeState{
		ActiveAlerts: []models.Alert{
			{ResourceID: "qemu/100", ResourceName: "web-01", Type: "cpu", Level: "critical", Message: "CPU too high"},
			{ResourceID: "storage/local", ResourceName: "local", Type: "backup", Level: "warning", Message: "Backup failed"},
			{ResourceID: "node/pve1", ResourceName: "pve1", Type: "unknown", Level: "info", Message: ""},
		},
	}

	flags := triageAlertChecksState(state, nil)
	if len(flags) != 3 {
		t.Fatalf("expected 3 alert flags, got %d", len(flags))
	}

	cpu := triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "qemu/100" })
	if cpu == nil || cpu.Severity != "critical" || cpu.Category != "performance" {
		t.Fatalf("expected critical performance flag for qemu/100, got %#v", cpu)
	}

	backup := triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "storage/local" })
	if backup == nil || backup.Severity != "warning" || backup.Category != "backup" {
		t.Fatalf("expected warning backup flag for storage/local, got %#v", backup)
	}

	unknown := triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "node/pve1" })
	if unknown == nil || unknown.Severity != "watch" || unknown.Category != "health" {
		t.Fatalf("expected watch health flag for node/pve1, got %#v", unknown)
	}
}

func TestTriageConnectivityChecks(t *testing.T) {
	reachable := false
	state := patrolRuntimeState{
		ConnectionHealth: map[string]bool{
			"pbs-backup1": false,
			"node/pve1":   true,
		},
	}
	guestIntel := map[string]*GuestIntelligence{
		"qemu/100": {Name: "web-01", GuestType: "vm", Reachable: &reachable},
		"lxc/200":  {Name: "worker-ct", GuestType: "lxc", Reachable: &reachable},
	}

	flags := triageConnectivityChecksState(state, guestIntel, nil)
	if len(flags) != 3 {
		t.Fatalf("expected 3 connectivity flags, got %d", len(flags))
	}

	if triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "pbs-backup1" && f.Severity == "critical" }) == nil {
		t.Fatalf("expected critical disconnected flag for pbs-backup1")
	}
	if triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "qemu/100" && f.Severity == "warning" }) == nil {
		t.Fatalf("expected warning unreachable flag for qemu/100")
	}
	ctFlag := triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "lxc/200" && f.Severity == "warning" })
	if ctFlag == nil || ctFlag.ResourceType != "system-container" {
		t.Fatalf("expected system-container unreachable flag for lxc/200, got %#v", ctFlag)
	}
}

func TestTriageResourceTypeRejectsLegacyKnownTypeAliases(t *testing.T) {
	if got := triageResourceType("host", "qemu/100"); got != "vm" {
		t.Fatalf("expected canonical vm type from resource ID when host alias is provided, got %q", got)
	}
	if got := triageResourceType("container", "qemu/100"); got != "vm" {
		t.Fatalf("expected canonical vm type from resource ID, got %q", got)
	}
	if got := triageResourceType("lxc", "qemu/100"); got != "vm" {
		t.Fatalf("expected canonical vm type from resource ID, got %q", got)
	}
	if got := triageResourceType("docker", "qemu/100"); got != "vm" {
		t.Fatalf("expected canonical vm type from resource ID when docker alias is provided, got %q", got)
	}
	if got := triageResourceType("physical-disk", "disk-1"); got != "physical_disk" {
		t.Fatalf("expected physical-disk alias to normalize to canonical physical_disk, got %q", got)
	}
}

func TestTriageResourceTypeCanonicalResourceIDs(t *testing.T) {
	testCases := []struct {
		name       string
		resourceID string
		want       string
	}{
		{name: "vm canonical id", resourceID: "vm:pve1:100", want: "vm"},
		{name: "system container canonical id", resourceID: "system-container:pve1:200", want: "system-container"},
		{name: "oci container canonical id", resourceID: "oci-container:pve1:201", want: "system-container"},
		{name: "node canonical id", resourceID: "node:pve1", want: "node"},
		{name: "app container canonical id", resourceID: "app-container:docker-host-1:abc123", want: "docker-host"},
		{name: "docker host canonical id", resourceID: "docker-host:edge-1", want: "docker-host"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := triageResourceType("", tc.resourceID); got != tc.want {
				t.Fatalf("triageResourceType(%q) = %q, want %q", tc.resourceID, got, tc.want)
			}
		})
	}
}

func TestTriageQuietInfra(t *testing.T) {
	p := &PatrolService{
		findings:   NewFindingsStore(),
		thresholds: DefaultPatrolThresholds(),
	}

	triage := p.runDeterministicTriageState(context.Background(), patrolRuntimeStateForTest(p, models.StateSnapshot{}), nil, nil)
	if triage == nil {
		t.Fatalf("expected triage result, got nil")
	}
	if !triage.IsQuiet {
		t.Fatalf("expected quiet infrastructure state")
	}
	if len(triage.Flags) != 0 {
		t.Fatalf("expected no flags, got %d", len(triage.Flags))
	}
}

func TestTriageNotQuietWithActiveFindings(t *testing.T) {
	store := NewFindingsStore()
	store.Add(&Finding{
		ID:           "finding-1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryGeneral,
		ResourceID:   "node/pve1",
		ResourceName: "pve1",
		ResourceType: "node",
		Title:        "Test finding",
		Description:  "test",
	})

	p := &PatrolService{
		findings:   store,
		thresholds: DefaultPatrolThresholds(),
	}

	triage := p.runDeterministicTriageState(context.Background(), patrolRuntimeStateForTest(p, models.StateSnapshot{}), nil, nil)
	if triage == nil {
		t.Fatalf("expected triage result, got nil")
	}
	if triage.IsQuiet {
		t.Fatalf("expected non-quiet state when active findings exist")
	}
	if len(triage.Flags) != 0 {
		t.Fatalf("expected no deterministic flags, got %d", len(triage.Flags))
	}
}

func TestTriageDeduplication(t *testing.T) {
	p := &PatrolService{
		findings:   NewFindingsStore(),
		thresholds: DefaultPatrolThresholds(),
	}

	state := models.StateSnapshot{
		VMs: []models.VM{
			{
				ID:         "qemu/100",
				Name:       "web-01",
				Status:     "running",
				Template:   false,
				Disk:       models.Disk{Usage: 95.0}, // already percent
				LastBackup: time.Now(),
			},
		},
		ActiveAlerts: []models.Alert{
			{
				ResourceID:   "qemu/100",
				ResourceName: "web-01",
				Type:         "disk",
				Level:        "warning",
				Message:      "Disk alert",
			},
		},
	}

	triage := p.runDeterministicTriageState(context.Background(), patrolRuntimeStateForTest(p, state), nil, nil)
	if triage == nil {
		t.Fatalf("expected triage result, got nil")
	}

	capacityFlags := triageFilterFlags(triage.Flags, func(f TriageFlag) bool {
		return f.ResourceID == "qemu/100" && f.Category == "capacity"
	})
	if len(capacityFlags) != 1 {
		t.Fatalf("expected 1 deduplicated capacity flag for qemu/100, got %d", len(capacityFlags))
	}
	if capacityFlags[0].Severity != "critical" {
		t.Fatalf("expected deduplicated severity critical, got %q", capacityFlags[0].Severity)
	}
}

func TestTriageDeduplicationDistinctMetrics(t *testing.T) {
	p := &PatrolService{
		findings:   NewFindingsStore(),
		thresholds: DefaultPatrolThresholds(),
	}

	// Node with both CPU and memory above thresholds — should produce TWO performance flags.
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node/pve1",
				Name:   "pve1",
				CPU:    0.92,                       // CPU is 0-1 scale (92%)
				Memory: models.Memory{Usage: 92.0}, // already percent
			},
		},
	}

	triage := p.runDeterministicTriageState(context.Background(), patrolRuntimeStateForTest(p, state), nil, nil)
	if triage == nil {
		t.Fatalf("expected triage result, got nil")
	}

	perfFlags := triageFilterFlags(triage.Flags, func(f TriageFlag) bool {
		return f.ResourceID == "node/pve1" && f.Category == "performance"
	})
	if len(perfFlags) != 2 {
		t.Fatalf("expected 2 distinct performance flags (cpu + memory), got %d", len(perfFlags))
	}

	cpuFlag := triageFindFlag(perfFlags, func(f TriageFlag) bool { return f.Metric == "cpu" })
	memFlag := triageFindFlag(perfFlags, func(f TriageFlag) bool { return f.Metric == "memory" })
	if cpuFlag == nil {
		t.Fatal("expected CPU performance flag")
	}
	if memFlag == nil {
		t.Fatal("expected memory performance flag")
	}
}

func TestFormatTriageBriefing(t *testing.T) {
	triage := &TriageResult{
		Flags: []TriageFlag{
			{
				ResourceID:   "qemu/100",
				ResourceName: "web-01",
				ResourceType: "vm",
				Category:     "performance",
				Severity:     "warning",
				Reason:       "Memory at 92% (threshold: 88%)",
				Metric:       "memory",
			},
		},
		Summary: TriageSummary{
			TotalNodes:         1,
			TotalGuests:        2,
			RunningGuests:      2,
			StoppedGuests:      0,
			TotalStoragePools:  1,
			TotalPhysicalDisks: 0,
			TotalStorage:       1,
			TotalDocker:        1,
			TotalPBS:           1,
			TotalPMG:           0,
			FlaggedCount:       1,
		},
		FlaggedIDs: map[string]bool{"qemu/100": true},
	}

	out := FormatTriageBriefing(triage)
	if !strings.Contains(out, "# Deterministic Triage Results") {
		t.Fatalf("expected title in output, got:\n%s", out)
	}
	if !strings.Contains(out, "| Resource | Type | Flag | Severity | Detail |") {
		t.Fatalf("expected table header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "web-01") || !strings.Contains(out, "High Memory") {
		t.Fatalf("expected resource row in output, got:\n%s", out)
	}
	if !strings.Contains(out, "## Healthy Resources") {
		t.Fatalf("expected healthy summary section, got:\n%s", out)
	}
	if !strings.Contains(out, "Scanned 6 resources: 1 nodes, 2 guests, 1 storage resources (1 pools, 0 physical disks), 1 docker hosts, 1 PBS, 0 PMG.") {
		t.Fatalf("expected explicit storage breakdown in scanned summary, got:\n%s", out)
	}
	if !strings.Contains(out, "Storage: 1 resources monitored (1 pools, 0 physical disks)") {
		t.Fatalf("expected storage resources wording in output, got:\n%s", out)
	}
}

func TestTriageBuildSummaryState_UsesReadStateWhenLegacySlicesEmpty(t *testing.T) {
	state := patrolRuntimeState{
		readState: &mockReadState{
			nodes: []*unifiedresources.NodeView{
				func() *unifiedresources.NodeView {
					node := unifiedresources.NewNodeView(&unifiedresources.Resource{
						ID:     "node/pve1",
						Name:   "pve1",
						Type:   unifiedresources.ResourceTypeAgent,
						Status: unifiedresources.StatusOnline,
					})
					return &node
				}(),
			},
			vms: []*unifiedresources.VMView{
				func() *unifiedresources.VMView {
					vm := unifiedresources.NewVMView(&unifiedresources.Resource{
						ID:     "qemu/100",
						Name:   "vm-run",
						Type:   unifiedresources.ResourceTypeVM,
						Status: "running",
						Proxmox: &unifiedresources.ProxmoxData{
							SourceID: "qemu/100",
							VMID:     100,
							NodeName: "pve1",
						},
					})
					return &vm
				}(),
				func() *unifiedresources.VMView {
					vm := unifiedresources.NewVMView(&unifiedresources.Resource{
						ID:     "qemu/101",
						Name:   "vm-stop",
						Type:   unifiedresources.ResourceTypeVM,
						Status: "stopped",
						Proxmox: &unifiedresources.ProxmoxData{
							SourceID: "qemu/101",
							VMID:     101,
							NodeName: "pve1",
						},
					})
					return &vm
				}(),
			},
			containers: []*unifiedresources.ContainerView{
				func() *unifiedresources.ContainerView {
					ct := unifiedresources.NewContainerView(&unifiedresources.Resource{
						ID:     "lxc/200",
						Name:   "ct-run",
						Type:   unifiedresources.ResourceTypeSystemContainer,
						Status: "running",
						Proxmox: &unifiedresources.ProxmoxData{
							SourceID: "lxc/200",
							VMID:     200,
							NodeName: "pve1",
						},
					})
					return &ct
				}(),
			},
			storage: []*unifiedresources.StoragePoolView{
				func() *unifiedresources.StoragePoolView {
					storage := unifiedresources.NewStoragePoolView(&unifiedresources.Resource{
						ID:     "storage/local",
						Name:   "local",
						Type:   unifiedresources.ResourceTypeStorage,
						Status: unifiedresources.StatusOnline,
					})
					return &storage
				}(),
			},
			dockerHosts: []*unifiedresources.DockerHostView{
				func() *unifiedresources.DockerHostView {
					host := unifiedresources.NewDockerHostView(&unifiedresources.Resource{
						ID:     "docker-host-1",
						Name:   "docker-a",
						Type:   unifiedresources.ResourceTypeAppContainer,
						Status: unifiedresources.StatusOnline,
					})
					return &host
				}(),
			},
			pbs: []*unifiedresources.PBSInstanceView{
				func() *unifiedresources.PBSInstanceView {
					pbs := unifiedresources.NewPBSInstanceView(&unifiedresources.Resource{
						ID:     "pbs-1",
						Name:   "pbs-a",
						Type:   unifiedresources.ResourceTypePBS,
						Status: unifiedresources.StatusOnline,
					})
					return &pbs
				}(),
			},
			pmg: []*unifiedresources.PMGInstanceView{
				func() *unifiedresources.PMGInstanceView {
					pmg := unifiedresources.NewPMGInstanceView(&unifiedresources.Resource{
						ID:     "pmg-1",
						Name:   "pmg-a",
						Type:   unifiedresources.ResourceTypePMG,
						Status: unifiedresources.StatusOnline,
					})
					return &pmg
				}(),
			},
		},
		unifiedResourceProvider: &mockUnifiedResourceProvider{
			getByTypeFunc: func(t unifiedresources.ResourceType) []unifiedresources.Resource {
				if t != unifiedresources.ResourceTypePhysicalDisk {
					return nil
				}
				return []unifiedresources.Resource{
					{
						ID:   "disk-1",
						Name: "sda",
						Type: unifiedresources.ResourceTypePhysicalDisk,
						PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
							DevPath: "/dev/sda",
							Model:   "sda",
							Health:  "PASSED",
						},
					},
				}
			},
		},
	}

	summary := triageBuildSummaryState(state, map[string]bool{"qemu/100": true})
	if summary.TotalNodes != 1 || summary.TotalStoragePools != 1 || summary.TotalPhysicalDisks != 1 || summary.TotalStorage != 2 || summary.TotalDocker != 1 || summary.TotalPBS != 1 || summary.TotalPMG != 1 {
		t.Fatalf("unexpected non-guest summary counts from readState: %#v", summary)
	}
	if summary.TotalGuests != 3 || summary.RunningGuests != 2 || summary.StoppedGuests != 1 {
		t.Fatalf("unexpected guest summary counts from readState: %#v", summary)
	}
	if summary.FlaggedCount != 1 {
		t.Fatalf("expected flagged count to be preserved, got %#v", summary)
	}
}

func triageFindFlag(flags []TriageFlag, predicate func(TriageFlag) bool) *TriageFlag {
	for _, flag := range flags {
		if predicate(flag) {
			flagCopy := flag
			return &flagCopy
		}
	}
	return nil
}

func triageFilterFlags(flags []TriageFlag, predicate func(TriageFlag) bool) []TriageFlag {
	filtered := make([]TriageFlag, 0)
	for _, flag := range flags {
		if predicate(flag) {
			filtered = append(filtered, flag)
		}
	}
	return filtered
}
