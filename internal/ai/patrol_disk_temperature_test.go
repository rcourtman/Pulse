package ai

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
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
		if got := diskTemperatureLimitsFor(tc.provider, alerts.DiskTemperatureHost{}, tc.diskType); got != tc.want {
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
	if got := ps.currentPatrolRuntimeState().diskTemperatureLimits(alerts.DiskTemperatureHost{}, "nvme"); got.trigger != 70 {
		t.Fatalf("no provider: nvme trigger = %v, want the factory 70", got.trigger)
	}

	ps.SetThresholdProvider(raisedNVMeThresholds())
	for name, state := range map[string]patrolRuntimeState{
		"current":  ps.currentPatrolRuntimeState(),
		"snapshot": ps.patrolRuntimeStateForSnapshot(snapshot),
	} {
		if got := state.diskTemperatureLimits(alerts.DiskTemperatureHost{}, "nvme"); got.trigger != 75 || got.clear != 70 {
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
	if got := scoped.diskTemperatureLimits(alerts.DiskTemperatureHost{}, "nvme"); got.trigger != 75 || got.clear != 70 {
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
	if got := patrol.currentPatrolRuntimeState().diskTemperatureLimits(alerts.DiskTemperatureHost{}, "nvme"); got.trigger != 75 {
		t.Fatalf("Patrol nvme trigger = %v, want the provider's 75", got.trigger)
	}
}

// A heat finding recovers at the clear value, where its alert recovers.
func TestDiskHeatRecoversAtTheClearValue(t *testing.T) {
	limits := diskTemperatureLimitsFor(nil, alerts.DiskTemperatureHost{}, "nvme")
	if !limits.cooled(65) || limits.cooled(66) {
		t.Fatalf("nvme limits %+v: want 65C cooled and 66C still warm", limits)
	}
	// With no band below the trigger, a reading at the trigger is still hot.
	noBand := diskTemperatureLimitsFor(&mockThresholdProvider{diskTemperature: map[string][2]float64{"sas": {70, 70}}}, alerts.DiskTemperatureHost{}, "sas")
	if !noBand.hot(70) || noBand.cooled(70) || !noBand.cooled(69) {
		t.Fatalf("no-band limits %+v: want 70C hot and not cooled, 69C cooled", noBand)
	}
}

// With no current reading (standby, a silent agent) a disk last seen hot has
// not shown that it cooled, so heat recovery is unknown rather than proven.
func TestDiskHeatRecoveryNeedsACurrentReading(t *testing.T) {
	retained := &diskinventory.CollectionStatus{Temperature: diskinventory.Unavailable("smartctl", "disk is in standby")}
	state := newPatrolRuntimeState(models.StateSnapshot{PhysicalDisks: []models.PhysicalDisk{
		{ID: "sata-retained-hot", DevPath: "/dev/sda", Type: "sata", Health: "PASSED", Wearout: -1, Temperature: 56, Collection: retained},
		{ID: "sata-retained-cool", DevPath: "/dev/sdb", Type: "sata", Health: "PASSED", Wearout: -1, Temperature: 40, Collection: retained},
	}})

	if _, err := verifyMetricRecoveredState(state, PatrolThresholds{}, "disk-high", "sata-retained-hot", "physical_disk"); !errors.Is(err, aicontracts.ErrVerificationUnknown) {
		t.Fatalf("retained 56C: err = %v, want ErrVerificationUnknown", err)
	}
	recovered, err := verifyMetricRecoveredState(state, PatrolThresholds{}, "disk-high", "sata-retained-cool", "physical_disk")
	if err != nil || !recovered {
		t.Fatalf("retained 40C: recovered = %v, err = %v, want recovered", recovered, err)
	}
}

// overriddenAgentDisksRegistry is two host agents, each reporting one NVMe
// disk hotter than the NVMe trigger, with a Disk Temp override of 80C (clear
// 75) set on agent-cool only. Alerts judge agent-cool's disk by the override.
func overriddenAgentDisksRegistry(t *testing.T) (*unifiedresources.ResourceRegistry, *AlertThresholdAdapter) {
	t.Helper()
	now := time.Now()
	agent := func(id string, temperature int) models.Host {
		return models.Host{
			ID: id, Hostname: id, DisplayName: id, Status: "online", LastSeen: now, IntervalSeconds: 30,
			Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
				Device: "nvme0n1", Model: "Samsung 990", Serial: id + "-serial", Type: "nvme",
				Health: "PASSED", Temperature: temperature,
			}}},
		}
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{Hosts: []models.Host{agent("agent-cool", 76), agent("agent-plain", 72)}})

	mgr := alerts.NewManager()
	cfg := mgr.GetConfig()
	cfg.Overrides = map[string]alerts.ThresholdConfig{
		"agent-cool": {DiskTemperature: &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}},
	}
	mgr.UpdateConfig(cfg)
	return registry, NewAlertThresholdAdapter(mgr)
}

// A host agent's Disk Temp override replaces the per-type trigger for every
// disk that agent reports, in Patrol exactly as in its alerts: agent-cool's
// NVMe at 76C is under its 80C override, agent-plain's at 72C is over the
// NVMe trigger of 70C.
func TestPatrolJudgesAgentDisksByTheAgentDiskTemperatureOverride(t *testing.T) {
	registry, provider := overriddenAgentDisksRegistry(t)
	state := newPatrolRuntimeStateWithProviders(models.StateSnapshot{}, nil, unifiedresources.NewUnifiedAIAdapter(registry))
	state.thresholdProvider = provider

	rows := patrolPhysicalDiskRows(state, nil)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want one disk per agent", rows)
	}
	for _, row := range rows {
		want, wantIssue := diskTemperatureLimits{trigger: 70, clear: 65}, true
		if row.temperature.Collected == 76 {
			want, wantIssue = diskTemperatureLimits{trigger: 80, clear: 75}, false
		}
		if row.temperatureLimits != want {
			t.Errorf("disk at %dC: limits = %+v, want %+v", row.temperature.Collected, row.temperatureLimits, want)
		}
		if got := patrolPhysicalDiskHealthIssue(row); got != wantIssue {
			t.Errorf("disk at %dC: health issue = %v, want %v", row.temperature.Collected, got, wantIssue)
		}
	}

	s := &Service{}
	s.SetUnifiedResourceProvider(unifiedresources.NewUnifiedAIAdapter(registry))
	s.SetPatrolThresholdProvider(provider)
	_, section, _ := strings.Cut(s.buildUnifiedResourceContext(), "Physical Disks Needing Attention")
	section, _, _ = strings.Cut(section, "\n\n")
	if !strings.Contains(section, "Temp: 72C") || strings.Contains(section, "Temp: 76C") {
		t.Fatalf("attention section = %q, want agent-plain's 72C disk only", section)
	}
}

// A disk reaches its reporting agent through its parent: the agent's machine
// directly, or the storage pool an Unraid array or cache disk hangs off. A
// disk on a machine with no agent gets the hostless policy.
func TestPhysicalDiskTemperatureHostFollowsTheParentChain(t *testing.T) {
	ref := func(id string) *string { return &id }
	owners := map[string]unifiedresources.Resource{
		"agent-1": {ID: "agent-1", Type: unifiedresources.ResourceTypeAgent, Agent: &unifiedresources.AgentData{
			AgentID: "host-1", LinkedNodeID: "lab-pve1",
		}},
		"vm-1": {ID: "vm-1", Type: unifiedresources.ResourceTypeVM, Agent: &unifiedresources.AgentData{
			AgentID: "host-vm", LinkedVMID: "lab-pve1-101",
		}},
		"array-1": {ID: "array-1", Type: unifiedresources.ResourceTypeStorage, ParentID: ref("agent-1")},
		"node-1":  {ID: "node-1", Type: unifiedresources.ResourceTypeAgent},
	}
	for parent, want := range map[string]alerts.DiskTemperatureHost{
		"agent-1": {ID: "host-1", LinkedNodeID: "lab-pve1"},
		"vm-1":    {ID: "host-vm", LinkedVMID: "lab-pve1-101"},
		"array-1": {ID: "host-1", LinkedNodeID: "lab-pve1"},
		"node-1":  {},
		"missing": {},
	} {
		disk := unifiedresources.Resource{ID: "disk-under-" + parent, Type: unifiedresources.ResourceTypePhysicalDisk, ParentID: ref(parent)}
		if got := physicalDiskTemperatureHost(disk, owners); got != want {
			t.Errorf("disk under %s: host = %+v, want %+v", parent, got, want)
		}
	}
	if got := physicalDiskTemperatureHost(unifiedresources.Resource{ID: "orphan"}, owners); got != (alerts.DiskTemperatureHost{}) {
		t.Errorf("parentless disk: host = %+v, want hostless", got)
	}
}

// trueNASHeatRegistry is a TrueNAS system whose pool holds four disks beside
// the two overridden host agents of overriddenAgentDisksRegistry: nvme0n1 at
// 72C, nvme1n1 at 63C, SATA sda at 60C and SATA sdb, whose 80C reading is
// retained from before standby. The user saved a TrueNAS-wide 62C (clear 57),
// raised nvme0n1's own trigger to 75C (clear 70), and left a 50C Disk Temp
// override under the TrueNAS system's synthetic agent ID. It returns the disk
// resources by name.
func trueNASHeatRegistry(t *testing.T) (*unifiedresources.ResourceRegistry, *AlertThresholdAdapter, map[string]unifiedresources.Resource) {
	t.Helper()
	registry, _ := overriddenAgentDisksRegistry(t)
	now := time.Now()
	disk := func(name, transport string, temperature int) truenas.Disk {
		return truenas.Disk{
			ID: "disk-" + name, Name: name, Pool: "tank", Status: "ONLINE", Model: "Model " + name,
			Serial: "SERIAL-" + strings.ToUpper(name), Temperature: temperature, Transport: transport,
		}
	}
	records := truenas.FixtureRecords(truenas.FixtureSnapshot{
		CollectedAt: now,
		System:      truenas.SystemInfo{Hostname: "truenas-main", Healthy: true, CollectedAt: now},
		Pools:       []truenas.Pool{{ID: "pool-tank", Name: "tank", Status: "ONLINE"}},
		Disks: []truenas.Disk{
			disk("nvme0n1", "nvme", 72), disk("nvme1n1", "nvme", 63),
			disk("sda", "sata", 60), disk("sdb", "sata", 80),
		},
	})
	for i := range records {
		if meta := records[i].Resource.PhysicalDisk; meta != nil && records[i].Resource.Name == "sdb" {
			meta.Collection = &diskinventory.CollectionStatus{Temperature: diskinventory.Unavailable("truenas", "disk is in standby")}
		}
	}
	registry.IngestRecords(unifiedresources.SourceTrueNAS, records)

	adapter := unifiedresources.NewUnifiedAIAdapter(registry)
	disks := make(map[string]unifiedresources.Resource)
	for _, r := range adapter.GetByType(unifiedresources.ResourceTypePhysicalDisk) {
		if alerts.IsTrueNASDiskResource(r) {
			disks[r.Name] = r
		}
	}
	systemAgentID := ""
	for _, r := range adapter.GetAll() {
		if r.Agent != nil && r.Agent.Platform == "truenas" {
			systemAgentID = r.Agent.AgentID
		}
	}
	if len(disks) != 4 || systemAgentID == "" {
		t.Fatalf("TrueNAS disks = %v, system agent ID = %q; want four disks under one system", disks, systemAgentID)
	}

	mgr := alerts.NewManager()
	cfg := mgr.GetConfig()
	cfg.TrueNASDiskDefaults.Temperature = &alerts.HysteresisThreshold{Trigger: 62, Clear: 57}
	cfg.Overrides = map[string]alerts.ThresholdConfig{
		"agent-cool":        {DiskTemperature: &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}},
		systemAgentID:       {DiskTemperature: &alerts.HysteresisThreshold{Trigger: 50, Clear: 45}},
		disks["nvme0n1"].ID: {Temperature: &alerts.HysteresisThreshold{Trigger: 75, Clear: 70}},
	}
	mgr.UpdateConfig(cfg)
	return registry, NewAlertThresholdAdapter(mgr), disks
}

// Patrol judges a TrueNAS disk by its own temperature alert's tiers, not as a
// disk of the TrueNAS system's synthetic host agent: nvme0n1's override keeps
// it quiet at 72C, the TrueNAS-wide 62C flags nvme1n1 at 63C (under the NVMe
// trigger of 70C) and spares SATA sda at 60C (over the SATA trigger of 55C),
// and the 50C host override under the system ID reaches none of them. A
// retained reading stays unjudged. Agent disks keep their host's override.
func TestPatrolJudgesTrueNASDisksByTheirAlertTiers(t *testing.T) {
	registry, provider, disks := trueNASHeatRegistry(t)
	state := newPatrolRuntimeStateWithProviders(models.StateSnapshot{}, nil, unifiedresources.NewUnifiedAIAdapter(registry))
	state.thresholdProvider = provider

	type want struct {
		limits diskTemperatureLimits
		issue  bool
	}
	expected := map[string]want{
		disks["nvme0n1"].ID: {limits: diskTemperatureLimits{trigger: 75, clear: 70}},
		disks["nvme1n1"].ID: {limits: diskTemperatureLimits{trigger: 62, clear: 57}, issue: true},
		disks["sda"].ID:     {limits: diskTemperatureLimits{trigger: 62, clear: 57}},
		disks["sdb"].ID:     {limits: diskTemperatureLimits{trigger: 62, clear: 57}},
	}
	rows := patrolPhysicalDiskRows(state, nil)
	if len(rows) != 6 {
		t.Fatalf("rows = %+v, want four TrueNAS disks and two agent disks", rows)
	}
	for _, row := range rows {
		w, trueNAS := expected[row.id]
		if !trueNAS {
			// overriddenAgentDisksRegistry: agent-cool's 76C NVMe under its 80C
			// override, agent-plain's 72C NVMe over the NVMe trigger.
			w = want{limits: diskTemperatureLimits{trigger: 70, clear: 65}, issue: true}
			if row.temperature.Collected == 76 {
				w = want{limits: diskTemperatureLimits{trigger: 80, clear: 75}}
			}
		}
		if row.temperatureLimits != w.limits {
			t.Errorf("disk %s at %dC: limits = %+v, want %+v", row.id, row.temperature.Collected, row.temperatureLimits, w.limits)
		}
		if got := patrolPhysicalDiskHealthIssue(row); got != w.issue {
			t.Errorf("disk %s at %dC: health issue = %v, want %v", row.id, row.temperature.Collected, got, w.issue)
		}
	}

	flags := triageDiskHealthChecksState(state, nil)
	for _, name := range []string{"nvme0n1", "sda", "sdb"} {
		if flag := triageFindFlag(flags, func(f TriageFlag) bool {
			return f.ResourceID == disks[name].ID && strings.Contains(f.Reason, "Disk temperature")
		}); flag != nil {
			t.Errorf("%s: unexpected temperature flag %+v", name, *flag)
		}
	}
	if flag := triageFindFlag(flags, func(f TriageFlag) bool {
		return f.ResourceID == disks["nvme1n1"].ID && strings.Contains(f.Reason, "Disk temperature")
	}); flag == nil || flag.Threshold != 62 {
		t.Errorf("nvme1n1: temperature flag = %+v, want one at the TrueNAS-wide 62C", flag)
	}

	if _, err := verifyMetricRecoveredState(state, PatrolThresholds{}, "disk-high", disks["sdb"].ID, "physical_disk"); !errors.Is(err, aicontracts.ErrVerificationUnknown) {
		t.Errorf("retained 80C: err = %v, want ErrVerificationUnknown", err)
	}

	s := &Service{}
	s.SetUnifiedResourceProvider(unifiedresources.NewUnifiedAIAdapter(registry))
	s.SetPatrolThresholdProvider(provider)
	_, section, _ := strings.Cut(s.buildUnifiedResourceContext(), "Physical Disks Needing Attention")
	section, _, _ = strings.Cut(section, "\n\n")
	for _, reading := range []string{"Temp: 63C", "Temp: 72C"} {
		if !strings.Contains(section, reading) {
			t.Errorf("attention section = %q, want the disk at %s", section, reading)
		}
	}
	for _, reading := range []string{"Temp: 60C", "Temp: 76C", "Temp: 80C"} {
		if strings.Contains(section, reading) {
			t.Errorf("attention section = %q, lists a disk at %s", section, reading)
		}
	}
	if strings.Count(section, "Temp: 72C") != 1 {
		t.Errorf("attention section = %q, want only agent-plain's 72C disk, not TrueNAS nvme0n1 under its 75C override", section)
	}
}
