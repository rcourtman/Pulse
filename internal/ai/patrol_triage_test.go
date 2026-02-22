package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestTriageThresholdChecks(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node/pve1",
				Name:   "pve1",
				CPU:    0.92,                       // CPU is 0-1 scale (92%)
				Memory: models.Memory{Usage: 30.0}, // Memory.Usage is already percent
			},
		},
	}

	flags := triageThresholdChecks(state, nil, DefaultPatrolThresholds())
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
				ResourceType: "container",
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
	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", Name: "vm-never", Status: "running", Template: false, LastBackup: time.Time{}},
		},
		Containers: []models.Container{
			{ID: "lxc/200", Name: "ct-stale", Status: "running", Template: false, LastBackup: oldBackup},
		},
	}

	flags := triageBackupChecks(state, nil)
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

func TestTriageDiskHealthChecks(t *testing.T) {
	state := models.StateSnapshot{
		PhysicalDisks: []models.PhysicalDisk{
			{ID: "disk-1", DevPath: "/dev/sda", Health: "FAILED", Wearout: 50, Temperature: 40},
			{ID: "disk-2", DevPath: "/dev/nvme0n1", Health: "PASSED", Wearout: 15, Temperature: 40},
			{ID: "disk-3", DevPath: "/dev/sdb", Health: "PASSED", Wearout: -1, Temperature: 60},
			{ID: "disk-4", DevPath: "/dev/sdc", Health: "PASSED", Wearout: 85, Temperature: 40},
		},
	}

	flags := triageDiskHealthChecks(state, nil)
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

func TestTriageAlertChecks(t *testing.T) {
	state := models.StateSnapshot{
		ActiveAlerts: []models.Alert{
			{ResourceID: "qemu/100", ResourceName: "web-01", Type: "cpu", Level: "critical", Message: "CPU too high"},
			{ResourceID: "storage/local", ResourceName: "local", Type: "backup", Level: "warning", Message: "Backup failed"},
			{ResourceID: "node/pve1", ResourceName: "pve1", Type: "unknown", Level: "info", Message: ""},
		},
	}

	flags := triageAlertChecks(state, nil)
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
	state := models.StateSnapshot{
		ConnectionHealth: map[string]bool{
			"pbs-backup1": false,
			"node/pve1":   true,
		},
	}
	guestIntel := map[string]*GuestIntelligence{
		"qemu/100": {Name: "web-01", GuestType: "vm", Reachable: &reachable},
	}

	flags := triageConnectivityChecks(state, guestIntel, nil)
	if len(flags) != 2 {
		t.Fatalf("expected 2 connectivity flags, got %d", len(flags))
	}

	if triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "pbs-backup1" && f.Severity == "critical" }) == nil {
		t.Fatalf("expected critical disconnected flag for pbs-backup1")
	}
	if triageFindFlag(flags, func(f TriageFlag) bool { return f.ResourceID == "qemu/100" && f.Severity == "warning" }) == nil {
		t.Fatalf("expected warning unreachable flag for qemu/100")
	}
}

func TestTriageQuietInfra(t *testing.T) {
	p := &PatrolService{
		findings:   NewFindingsStore(),
		thresholds: DefaultPatrolThresholds(),
	}

	triage := p.RunDeterministicTriage(context.Background(), models.StateSnapshot{}, nil, nil)
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

	triage := p.RunDeterministicTriage(context.Background(), models.StateSnapshot{}, nil, nil)
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

	triage := p.RunDeterministicTriage(context.Background(), state, nil, nil)
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

	triage := p.RunDeterministicTriage(context.Background(), state, nil, nil)
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
			TotalNodes:    1,
			TotalGuests:   2,
			RunningGuests: 2,
			StoppedGuests: 0,
			TotalStorage:  1,
			TotalDocker:   1,
			TotalPBS:      1,
			TotalPMG:      0,
			FlaggedCount:  1,
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
