package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// raisedNVMeThresholds is a user who raised the NVMe disk temperature trigger
// to 75C (clear 70) in Alerts and left every other type at its default.
func raisedNVMeThresholds() *mockThresholdProvider {
	return &mockThresholdProvider{diskTemperature: map[string][2]float64{"nvme": {75, 70}}}
}

func TestDiskTemperatureLimitsFollowTheAlertPolicy(t *testing.T) {
	for _, tc := range []struct {
		name     string
		provider ThresholdProvider
		diskType string
		want     diskTemperatureLimits
	}{
		{name: "factory nvme", diskType: "nvme", want: diskTemperatureLimits{trigger: 70, clear: 65}},
		{name: "factory sata", diskType: "sata", want: diskTemperatureLimits{trigger: 55, clear: 50}},
		{name: "factory untyped", diskType: "", want: diskTemperatureLimits{trigger: 55, clear: 50}},
		{name: "raised nvme", provider: raisedNVMeThresholds(), diskType: "nvme", want: diskTemperatureLimits{trigger: 75, clear: 70}},
		{name: "raised nvme leaves sata", provider: raisedNVMeThresholds(), diskType: "sata", want: diskTemperatureLimits{trigger: 55, clear: 50}},
		{name: "alerting off", provider: &mockThresholdProvider{diskTemperature: map[string][2]float64{"sata": {0, 0}}}, diskType: "sata", want: diskTemperatureLimits{}},
		{name: "clear above trigger", provider: &mockThresholdProvider{diskTemperature: map[string][2]float64{"sas": {60, 64}}}, diskType: "sas", want: diskTemperatureLimits{trigger: 60, clear: 60}},
	} {
		if got := diskTemperatureLimitsFor(tc.provider, tc.diskType); got != tc.want {
			t.Errorf("%s: limits = %+v, want %+v", tc.name, got, tc.want)
		}
	}

	off := diskTemperatureLimits{}
	if off.hot(95) || !off.cooled(95) {
		t.Fatal("with disk temperature alerting off no reading is heat")
	}
}

func diskTemperatureTriageState(provider ThresholdProvider) patrolRuntimeState {
	state := newPatrolRuntimeState(models.StateSnapshot{PhysicalDisks: []models.PhysicalDisk{
		{ID: "nvme-63", DevPath: "/dev/nvme0n1", Type: "nvme", Health: "PASSED", Wearout: 90, Temperature: 63},
		{ID: "nvme-66", DevPath: "/dev/nvme1n1", Type: "nvme", Health: "PASSED", Wearout: 90, Temperature: 66},
		{ID: "nvme-72", DevPath: "/dev/nvme2n1", Type: "nvme", Health: "PASSED", Wearout: 90, Temperature: 72},
		{ID: "sata-52", DevPath: "/dev/sda", Type: "sata", Health: "PASSED", Wearout: -1, Temperature: 52},
		{ID: "sata-56", DevPath: "/dev/sdb", Type: "sata", Health: "PASSED", Wearout: -1, Temperature: 56},
	}})
	state.thresholdProvider = provider
	return state
}

// Patrol judges disk heat by the alert disk temperature policy: a warning from
// the per-type trigger and nothing below it, the band under the trigger being
// a Temp cell colour rather than a finding. An NVMe at 63C is not hot; a SATA
// disk at 56C is.
func TestTriageDiskTemperatureFollowsTheAlertPolicy(t *testing.T) {
	temperatureFlag := func(flags []TriageFlag, id string) *TriageFlag {
		return triageFindFlag(flags, func(f TriageFlag) bool {
			return f.ResourceID == id && strings.Contains(f.Reason, "Disk temperature")
		})
	}
	expect := func(flags []TriageFlag, id, severity string, threshold float64) {
		t.Helper()
		flag := temperatureFlag(flags, id)
		if severity == "" {
			if flag != nil {
				t.Errorf("%s: unexpected temperature flag %+v", id, *flag)
			}
			return
		}
		if flag == nil || flag.Severity != severity || flag.Threshold != threshold {
			t.Errorf("%s: temperature flag = %+v, want %s at threshold %.0f", id, flag, severity, threshold)
		}
	}

	flags := triageDiskHealthChecksState(diskTemperatureTriageState(nil), nil)
	expect(flags, "nvme-63", "", 0)
	expect(flags, "nvme-66", "", 0)
	expect(flags, "nvme-72", "warning", 70)
	expect(flags, "sata-52", "", 0)
	expect(flags, "sata-56", "warning", 55)
	if flag := temperatureFlag(flags, "sata-56"); flag == nil || flag.Reason != "Disk temperature 56°C (alert threshold: 55°C)" {
		t.Errorf("sata-56 reason = %+v, want the reading and the alert trigger", flag)
	}

	flags = triageDiskHealthChecksState(diskTemperatureTriageState(raisedNVMeThresholds()), nil)
	expect(flags, "nvme-72", "", 0)
	expect(flags, "sata-56", "warning", 55)

	off := &mockThresholdProvider{diskTemperature: map[string][2]float64{"nvme": {0, 0}, "sata": {0, 0}}}
	if flags := triageDiskHealthChecksState(diskTemperatureTriageState(off), nil); len(flags) != 0 {
		t.Fatalf("disk temperature alerting off should raise no heat flags, got %+v", flags)
	}
}

// The seed's disk issue list and finding verification share the policy: a
// disk is an issue from the trigger and recovers below the clear value.
func TestPatrolDiskHeatIssueAndRecoveryFollowTheAlertPolicy(t *testing.T) {
	issues := map[string]bool{}
	for _, row := range patrolPhysicalDiskRows(diskTemperatureTriageState(nil), nil) {
		issues[row.id] = patrolPhysicalDiskHealthIssue(row)
	}
	for id, want := range map[string]bool{"nvme-63": false, "nvme-66": false, "nvme-72": true, "sata-52": false, "sata-56": true} {
		if issues[id] != want {
			t.Errorf("%s: health issue = %v, want %v", id, issues[id], want)
		}
	}
	for _, row := range patrolPhysicalDiskRows(diskTemperatureTriageState(raisedNVMeThresholds()), nil) {
		if row.id == "nvme-72" && patrolPhysicalDiskHealthIssue(row) {
			t.Error("nvme-72 under a raised 75C trigger should not be an issue")
		}
	}

	state := diskTemperatureTriageState(nil)
	for id, want := range map[string]bool{"nvme-63": true, "nvme-66": false, "nvme-72": false, "sata-52": false} {
		recovered, err := verifyMetricRecoveredState(state, PatrolThresholds{}, "disk-high", id, "physical_disk")
		if err != nil {
			t.Fatalf("%s: verification error %v", id, err)
		}
		if recovered != want {
			t.Errorf("%s: recovered = %v, want %v", id, recovered, want)
		}
	}
}

func TestResourceContextListsDisksHotByTheAlertPolicy(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	now := time.Now()
	registry.IngestSnapshot(models.StateSnapshot{PhysicalDisks: []models.PhysicalDisk{
		{ID: "pve-nvme", Node: "pve", Instance: "lab", DevPath: "/dev/nvme0n1", Model: "Samsung 980", Serial: "N63", Type: "nvme", Health: "PASSED", Wearout: 90, Temperature: 63, LastChecked: now},
		{ID: "pve-nvme-72", Node: "pve", Instance: "lab", DevPath: "/dev/nvme1n1", Model: "Samsung 990", Serial: "N72", Type: "nvme", Health: "PASSED", Wearout: 90, Temperature: 72, LastChecked: now},
		{ID: "pve-sata", Node: "pve", Instance: "lab", DevPath: "/dev/sda", Model: "WDC WD80EFAX", Serial: "S56", Type: "sata", Health: "PASSED", Wearout: -1, Temperature: 56, LastChecked: now},
	}})

	s := &Service{}
	s.SetUnifiedResourceProvider(unifiedresources.NewUnifiedAIAdapter(registry))
	attention := func() string {
		got := s.buildUnifiedResourceContext()
		_, section, found := strings.Cut(got, "Physical Disks Needing Attention")
		if !found {
			return ""
		}
		section, _, _ = strings.Cut(section, "\n\n")
		return section
	}

	// Disk names are redacted in this context, so the readings identify them.
	section := attention()
	if !strings.Contains(section, "Temp: 56C") || !strings.Contains(section, "Temp: 72C") || strings.Contains(section, "Temp: 63C") {
		t.Fatalf("factory policy: attention section = %q, want the SATA 56C and NVMe 72C disks only", section)
	}

	s.SetPatrolThresholdProvider(raisedNVMeThresholds())
	section = attention()
	if !strings.Contains(section, "Temp: 56C") || strings.Contains(section, "Temp: 72C") || strings.Contains(section, "Temp: 63C") {
		t.Fatalf("raised NVMe trigger: attention section = %q, want only the SATA 56C disk", section)
	}
}

// A Patrol run judges disk heat by the thresholds the user configured, through
// both ways a run builds its state.
func TestPatrolRunStateCarriesTheConfiguredDiskTemperaturePolicy(t *testing.T) {
	snapshot := models.StateSnapshot{PhysicalDisks: []models.PhysicalDisk{
		{ID: "nvme-72", DevPath: "/dev/nvme0n1", Type: "nvme", Health: "PASSED", Wearout: 90, Temperature: 72},
	}}
	ps := NewPatrolService(nil, &mockStateProvider{state: snapshot})
	if got := ps.currentPatrolRuntimeState().diskTemperatureLimits("nvme"); got.trigger != 70 {
		t.Fatalf("no provider: nvme trigger = %v, want the factory 70", got.trigger)
	}

	ps.SetThresholdProvider(raisedNVMeThresholds())
	for name, state := range map[string]patrolRuntimeState{
		"current":  ps.currentPatrolRuntimeState(),
		"snapshot": ps.patrolRuntimeStateForSnapshot(snapshot),
	} {
		if got := state.diskTemperatureLimits("nvme"); got.trigger != 75 || got.clear != 70 {
			t.Errorf("%s state: nvme limits = %+v, want the configured 75/70", name, got)
		}
		if flags := triageDiskHealthChecksState(state, nil); len(flags) != 0 {
			t.Errorf("%s state: nvme at 72C under a 75C trigger flagged %+v", name, flags)
		}
	}
}

// A scoped Patrol run filters its state; the filter must keep the configured
// thresholds, or a scoped run would judge heat by the factory triggers.
func TestScopedPatrolStateKeepsTheConfiguredDiskTemperaturePolicy(t *testing.T) {
	state := diskTemperatureTriageState(raisedNVMeThresholds())
	scoped := filterPatrolStateByScopeState(state, PatrolScope{ResourceIDs: []string{"nvme-72"}})
	if got := scoped.diskTemperatureLimits("nvme"); got.trigger != 75 || got.clear != 70 {
		t.Fatalf("scoped state nvme limits = %+v, want the configured 75/70", got)
	}
	if flags := triageDiskHealthChecksState(scoped, nil); len(flags) != 0 {
		t.Fatalf("scoped run flagged nvme at 72C under a 75C trigger: %+v", flags)
	}
}

// The alert threshold provider can arrive before Patrol exists; Patrol must
// still start with it.
func TestPatrolStartsWithAThresholdProviderSetBeforeIt(t *testing.T) {
	s := NewService(nil, nil)
	s.SetPatrolThresholdProvider(raisedNVMeThresholds())
	s.SetStateProvider(&mockStateProvider{})
	patrol := s.GetPatrolService()
	if patrol == nil {
		t.Fatal("expected Patrol to start once a state provider exists")
	}
	if got := patrol.currentPatrolRuntimeState().diskTemperatureLimits("nvme"); got.trigger != 75 {
		t.Fatalf("Patrol nvme trigger = %v, want the provider's 75", got.trigger)
	}
}

// A heat finding recovers at the clear value, where its alert recovers.
func TestDiskHeatRecoversAtTheClearValue(t *testing.T) {
	limits := diskTemperatureLimitsFor(nil, "nvme")
	if !limits.cooled(65) || limits.cooled(66) {
		t.Fatalf("nvme limits %+v: want 65C cooled and 66C still warm", limits)
	}
	// With no band below the trigger, a reading at the trigger is still hot.
	noBand := diskTemperatureLimitsFor(&mockThresholdProvider{diskTemperature: map[string][2]float64{"sas": {70, 70}}}, "sas")
	if !noBand.hot(70) || noBand.cooled(70) || !noBand.cooled(69) {
		t.Fatalf("no-band limits %+v: want 70C hot and not cooled, 69C cooled", noBand)
	}
}
