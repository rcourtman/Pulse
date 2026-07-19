package unifiedresources

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// ptrTime is a small helper local to this branch-coverage file.
func ptrTime(v time.Time) *time.Time { return &v }

// =====================
// HostView accessors
// =====================

// HostView.TokenLastUsedAt has a triple nil-guard and returns an independent
// *time.Time when populated.
func TestHostViewsBranchcov0719late_HostTokenLastUsedAt(t *testing.T) {
	// (a) nil receiver.
	var zero HostView
	if got := zero.TokenLastUsedAt(); got != nil {
		t.Fatalf("nil receiver: expected nil, got %v", got)
	}

	// (a) nil Agent.
	r := &Resource{ID: "h-tlu-1", Type: ResourceTypeAgent}
	if got := NewHostView(r).TokenLastUsedAt(); got != nil {
		t.Fatalf("nil Agent: expected nil, got %v", got)
	}

	// (a) Agent set but TokenLastUsedAt pointer nil.
	r.Agent = &AgentData{}
	if got := NewHostView(r).TokenLastUsedAt(); got != nil {
		t.Fatalf("nil TokenLastUsedAt: expected nil, got %v", got)
	}

	// (b) Populated: equal value via an independent pointer.
	used := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	r.Agent.TokenLastUsedAt = ptrTime(used)
	got := NewHostView(r).TokenLastUsedAt()
	if got == nil {
		t.Fatal("populated: expected non-nil")
	}
	if !got.Equal(used) {
		t.Fatalf("expected %v, got %v", used, *got)
	}
	// Mutating the returned pointer's pointee must not leak back to the source.
	*got = got.Add(time.Hour)
	if !r.Agent.TokenLastUsedAt.Equal(used) {
		t.Fatalf("mutation leaked to source: source=%v want=%v", *r.Agent.TokenLastUsedAt, used)
	}
}

// HostView.PackageUpdates guards nil receiver / nil Agent / nil PackageUpdates
// and returns a clone with an independent Packages slice when populated.
func TestHostViewsBranchcov0719late_HostPackageUpdates(t *testing.T) {
	// (a) nil receiver.
	var zero HostView
	if got := zero.PackageUpdates(); got != nil {
		t.Fatalf("nil receiver: expected nil, got %+v", got)
	}
	// (a) nil Agent.
	r := &Resource{ID: "h-pkg-1", Type: ResourceTypeAgent}
	if got := NewHostView(r).PackageUpdates(); got != nil {
		t.Fatalf("nil Agent: expected nil, got %+v", got)
	}
	// (a) Agent present but PackageUpdates pointer nil.
	r.Agent = &AgentData{}
	if got := NewHostView(r).PackageUpdates(); got != nil {
		t.Fatalf("nil PackageUpdates: expected nil, got %+v", got)
	}
	// (b) Populated: value-equal copy with an independent Packages slice.
	checked := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	r.Agent.PackageUpdates = &AgentPackageUpdateMeta{
		Supported:     true,
		Manager:       "apt",
		InventoryHash: "abc",
		PendingCount:  2,
		CheckedAt:     checked,
		Packages: []AgentPackageUpdate{
			{Name: "curl", InstalledVersion: "1.0", AvailableVersion: "1.1"},
			{Name: "wget", InstalledVersion: "2.0", AvailableVersion: "2.1"},
		},
		RebootRequired: true,
	}
	got := NewHostView(r).PackageUpdates()
	if got == nil {
		t.Fatal("populated: expected non-nil")
	}
	if !reflect.DeepEqual(*got, *r.Agent.PackageUpdates) {
		t.Fatalf("expected value-equal clone, got %+v want %+v", *got, *r.Agent.PackageUpdates)
	}
	// Mutate scalar on the clone; source unaffected.
	got.Manager = "yum"
	if r.Agent.PackageUpdates.Manager != "apt" {
		t.Fatalf("scalar mutation leaked to source: source.Manager=%q want %q", r.Agent.PackageUpdates.Manager, "apt")
	}
	// Mutate element of Packages; source unaffected.
	got.Packages[0].Name = "mutated"
	if r.Agent.PackageUpdates.Packages[0].Name != "curl" {
		t.Fatalf("Packages element mutation leaked: source[0].Name=%q want %q", r.Agent.PackageUpdates.Packages[0].Name, "curl")
	}
	// Append to Packages; source length unchanged.
	got.Packages = append(got.Packages, AgentPackageUpdate{Name: "extra"})
	if len(r.Agent.PackageUpdates.Packages) != 2 {
		t.Fatalf("Packages append leaked to source: source len=%d want 2", len(r.Agent.PackageUpdates.Packages))
	}
}

// HostView.StorageCleanup guards nil receiver / nil Agent / nil StorageCleanup.
func TestHostViewsBranchcov0719late_HostStorageCleanup(t *testing.T) {
	// (a) nil receiver.
	var zero HostView
	if got := zero.StorageCleanup(); got != nil {
		t.Fatalf("nil receiver: expected nil, got %+v", got)
	}
	// (a) nil Agent.
	r := &Resource{ID: "h-cln-1", Type: ResourceTypeAgent}
	if got := NewHostView(r).StorageCleanup(); got != nil {
		t.Fatalf("nil Agent: expected nil, got %+v", got)
	}
	// (a) Agent present but StorageCleanup pointer nil.
	r.Agent = &AgentData{}
	if got := NewHostView(r).StorageCleanup(); got != nil {
		t.Fatalf("nil StorageCleanup: expected nil, got %+v", got)
	}
	// (b) Populated: value-equal shallow copy.
	checked := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	r.Agent.StorageCleanup = &AgentStorageCleanupMeta{
		Supported:        true,
		Provider:         "journal",
		Fingerprint:      "fp-1",
		ReclaimableBytes: 1024,
		CheckedAt:        checked,
	}
	got := NewHostView(r).StorageCleanup()
	if got == nil {
		t.Fatal("populated: expected non-nil")
	}
	if !reflect.DeepEqual(*got, *r.Agent.StorageCleanup) {
		t.Fatalf("expected value-equal clone, got %+v want %+v", *got, *r.Agent.StorageCleanup)
	}
	got.Provider = "apt"
	if r.Agent.StorageCleanup.Provider != "journal" {
		t.Fatalf("scalar mutation leaked to source: source.Provider=%q want %q", r.Agent.StorageCleanup.Provider, "journal")
	}
}

// HostView.Capabilities appends into a fresh slice from r.Capabilities.
func TestHostViewsBranchcov0719late_HostCapabilities(t *testing.T) {
	// (a) nil receiver.
	var zero HostView
	if got := zero.Capabilities(); got != nil {
		t.Fatalf("nil receiver: expected nil, got %+v", got)
	}
	// (a) Resource present, no capabilities set: append of nil yields nil.
	r := &Resource{ID: "h-cap-1", Type: ResourceTypeAgent}
	if got := NewHostView(r).Capabilities(); got != nil {
		t.Fatalf("empty source capabilities: expected nil, got %+v", got)
	}
	// (b) Populated: equal contents with an independent backing array.
	r.Capabilities = []ResourceCapability{
		{Name: "restart", InternalHandler: "agent.restart"},
		{Name: "shell", InternalHandler: "agent.shell"},
	}
	got := NewHostView(r).Capabilities()
	if len(got) != 2 || got[0].Name != "restart" || got[1].InternalHandler != "agent.shell" {
		t.Fatalf("expected cloned capabilities, got %+v", got)
	}
	// Mutating element value fields on the clone must not affect the source.
	got[0].Name = "mutated"
	got[1].InternalHandler = "leaked"
	if r.Capabilities[0].Name != "restart" || r.Capabilities[1].InternalHandler != "agent.shell" {
		t.Fatalf("mutation leaked to source: %+v", r.Capabilities)
	}
	// Appending must not grow the source's backing array.
	got = append(got, ResourceCapability{Name: "extra"})
	if len(r.Capabilities) != 2 {
		t.Fatalf("append leaked to source: source len=%d want 2", len(r.Capabilities))
	}
}

// HostView.RAID clones a slice of HostRAIDMeta (with nested Devices + Risk).
func TestHostViewsBranchcov0719late_HostRAID(t *testing.T) {
	// (a) nil receiver.
	var zero HostView
	if got := zero.RAID(); got != nil {
		t.Fatalf("nil receiver: expected nil, got %+v", got)
	}
	// (a) nil Agent.
	r := &Resource{ID: "h-raid-1", Type: ResourceTypeAgent}
	if got := NewHostView(r).RAID(); got != nil {
		t.Fatalf("nil Agent: expected nil, got %+v", got)
	}
	// (a) Agent present, RAID nil: cloneHostRAIDMetaSlice(nil) returns nil.
	r.Agent = &AgentData{}
	if got := NewHostView(r).RAID(); got != nil {
		t.Fatalf("nil RAID slice: expected nil, got %+v", got)
	}
	// (b) Populated: deep clone with independent nested Devices/Risk.
	r.Agent.RAID = []HostRAIDMeta{
		{
			Device:        "md0",
			Level:         "raid1",
			State:         "active",
			TotalDevices:  2,
			ActiveDevices: 2,
			Devices: []HostRAIDDeviceMeta{
				{Device: "sda", State: "active", Slot: 0},
				{Device: "sdb", State: "active", Slot: 1},
			},
			Risk: &StorageRisk{
				Level:   storagehealth.RiskWarning,
				Reasons: []StorageRiskReason{{Code: "raid_degraded", Severity: storagehealth.RiskWarning, Summary: "RAID degraded"}},
			},
		},
	}
	got := NewHostView(r).RAID()
	if len(got) != 1 || got[0].Device != "md0" || len(got[0].Devices) != 2 || got[0].Risk == nil {
		t.Fatalf("expected deep-cloned RAID slice, got %+v", got)
	}
	// Mutating a nested Devices element must not leak to source.
	got[0].Devices[0].Device = "mutated"
	if r.Agent.RAID[0].Devices[0].Device != "sda" {
		t.Fatalf("nested Devices mutation leaked: source[0].Devices[0].Device=%q want %q", r.Agent.RAID[0].Devices[0].Device, "sda")
	}
	// Truncating nested Devices on the clone must not affect source length.
	got[0].Devices = got[0].Devices[:0]
	if len(r.Agent.RAID[0].Devices) != 2 {
		t.Fatalf("Devices truncation leaked: source len=%d want 2", len(r.Agent.RAID[0].Devices))
	}
	// Mutating nested Risk.Reasons must not leak to source.
	got[0].Risk.Reasons[0].Code = "leaked"
	if r.Agent.RAID[0].Risk.Reasons[0].Code != "raid_degraded" {
		t.Fatalf("Risk.Reasons mutation leaked: source code=%q want %q", r.Agent.RAID[0].Risk.Reasons[0].Code, "raid_degraded")
	}
	// Re-clone to verify Risk pointer is a fresh allocation (not shared).
	got2 := NewHostView(r).RAID()
	if &got2[0].Risk == &r.Agent.RAID[0].Risk {
		t.Fatal("expected Risk pointer to be a fresh allocation, not shared with source")
	}
}

// HostView.DiskIO clones a slice of HostDiskIOMeta (flat struct).
func TestHostViewsBranchcov0719late_HostDiskIO(t *testing.T) {
	// (a) nil receiver.
	var zero HostView
	if got := zero.DiskIO(); got != nil {
		t.Fatalf("nil receiver: expected nil, got %+v", got)
	}
	// (a) nil Agent.
	r := &Resource{ID: "h-dio-1", Type: ResourceTypeAgent}
	if got := NewHostView(r).DiskIO(); got != nil {
		t.Fatalf("nil Agent: expected nil, got %+v", got)
	}
	// (a) Agent present, DiskIO nil.
	r.Agent = &AgentData{}
	if got := NewHostView(r).DiskIO(); got != nil {
		t.Fatalf("nil DiskIO slice: expected nil, got %+v", got)
	}
	// (b) Populated: fresh slice with equal contents.
	r.Agent.DiskIO = []HostDiskIOMeta{
		{Device: "sda", ReadBytes: 100, WriteBytes: 200, ReadOps: 1, WriteOps: 2},
		{Device: "sdb", ReadBytes: 300, WriteBytes: 400, ReadOps: 3, WriteOps: 4},
	}
	got := NewHostView(r).DiskIO()
	if len(got) != 2 || got[0].Device != "sda" || got[1].WriteOps != 4 {
		t.Fatalf("expected cloned DiskIO slice, got %+v", got)
	}
	// Mutate element fields on clone; source unaffected (fresh backing array).
	got[0].Device = "mutated"
	got[0].ReadBytes = 999
	if r.Agent.DiskIO[0].Device != "sda" || r.Agent.DiskIO[0].ReadBytes != 100 {
		t.Fatalf("mutation leaked to source: %+v", r.Agent.DiskIO[0])
	}
	// Append on clone must not affect source.
	got = append(got, HostDiskIOMeta{Device: "extra"})
	if len(r.Agent.DiskIO) != 2 {
		t.Fatalf("append leaked to source: source len=%d want 2", len(r.Agent.DiskIO))
	}
}

// HostView.Ceph clones *HostCephMeta (with nested maps/slices).
func TestHostViewsBranchcov0719late_HostCeph(t *testing.T) {
	// (a) nil receiver.
	var zero HostView
	if got := zero.Ceph(); got != nil {
		t.Fatalf("nil receiver: expected nil, got %+v", got)
	}
	// (a) nil Agent.
	r := &Resource{ID: "h-ceph-1", Type: ResourceTypeAgent}
	if got := NewHostView(r).Ceph(); got != nil {
		t.Fatalf("nil Agent: expected nil, got %+v", got)
	}
	// (a) Agent present, Ceph nil: cloneHostCephMeta(nil) returns nil.
	r.Agent = &AgentData{}
	if got := NewHostView(r).Ceph(); got != nil {
		t.Fatalf("nil Ceph: expected nil, got %+v", got)
	}
	// (b) Populated: deep clone with independent nested maps/slices.
	r.Agent.Ceph = &HostCephMeta{
		FSID:         "ceph-fsid-1",
		HealthStatus: "HEALTH_WARN",
		Health: HostCephHealthMeta{
			Status: "HEALTH_WARN",
			Checks: map[string]HostCephCheckMeta{
				"POOL_NEAR_FULL": {Severity: "warning", Message: "pool near full", Detail: []string{"pool1"}},
			},
			Summary: []HostCephHealthSummaryMeta{{Severity: "warning", Message: "near full"}},
		},
		Pools: []HostCephPoolMeta{{ID: 1, Name: "pool1", PercentUsed: 0.9}},
		Services: []HostCephServiceMeta{
			{Type: "mon", Running: 1, Total: 1, Daemons: []string{"mon.a"}},
		},
	}
	got := NewHostView(r).Ceph()
	if got == nil || got.FSID != "ceph-fsid-1" || got.HealthStatus != "HEALTH_WARN" {
		t.Fatalf("expected cloned ceph meta, got %+v", got)
	}
	if got.Health.Checks == nil || got.Health.Checks["POOL_NEAR_FULL"].Message != "pool near full" {
		t.Fatalf("expected cloned checks map, got %+v", got.Health.Checks)
	}
	// Mutating the nested map on the clone must not leak to source.
	got.Health.Checks["POOL_NEAR_FULL"] = HostCephCheckMeta{Severity: "error", Message: "mutated"}
	if r.Agent.Ceph.Health.Checks["POOL_NEAR_FULL"].Message != "pool near full" {
		t.Fatalf("Health.Checks mutation leaked: %q", r.Agent.Ceph.Health.Checks["POOL_NEAR_FULL"].Message)
	}
	// Adding a new key on clone must not affect source map.
	got.Health.Checks["NEW"] = HostCephCheckMeta{}
	if len(r.Agent.Ceph.Health.Checks) != 1 {
		t.Fatalf("map addition leaked: source len=%d want 1", len(r.Agent.Ceph.Health.Checks))
	}
	// Mutating nested Detail slice via a fresh clone must not leak to source.
	fresh := NewHostView(r).Ceph()
	fresh.Health.Checks["POOL_NEAR_FULL"].Detail[0] = "leaked"
	if r.Agent.Ceph.Health.Checks["POOL_NEAR_FULL"].Detail[0] != "pool1" {
		t.Fatalf("Detail mutation leaked: %q", r.Agent.Ceph.Health.Checks["POOL_NEAR_FULL"].Detail[0])
	}
	// Mutating Pools slice element on clone must not affect source.
	got.Pools[0].Name = "leaked"
	if r.Agent.Ceph.Pools[0].Name != "pool1" {
		t.Fatalf("Pools mutation leaked: %q", r.Agent.Ceph.Pools[0].Name)
	}
	// Mutating nested Services[].Daemons must not leak to source.
	got.Services[0].Daemons[0] = "leaked"
	if r.Agent.Ceph.Services[0].Daemons[0] != "mon.a" {
		t.Fatalf("Services.Daemons mutation leaked: %q", r.Agent.Ceph.Services[0].Daemons[0])
	}
}

// =====================
// DockerHostView accessors
// =====================

// dockerHostFixture returns a Docker host Resource with fully-populated swarm
// collections plus the security/hidden flags used by the accessors under test.
func dockerHostFixture() *Resource {
	return &Resource{
		ID:   "dh-1",
		Type: ResourceTypeAgent,
		Name: "dh-name",
		Docker: &DockerData{
			Hidden: true,
			Security: &models.DockerHostSecurity{
				AuthorizationPlugins:          []string{"authz1", "authz2"},
				MutatingCommandsBlocked:       true,
				MutatingCommandsBlockedReason: "policy",
			},
			Services: []models.DockerService{{ID: "svc-1", Name: "nginx", Stack: "web"}},
			Tasks:    []models.DockerTask{{ID: "task-1", ServiceID: "svc-1", NodeID: "node-1"}},
			Nodes:    []models.DockerNode{{ID: "node-1", Hostname: "n1", Role: "manager"}},
			Secrets:  []models.DockerSecret{{ID: "sec-1", Name: "tls-cert"}},
			Configs:  []models.DockerConfig{{ID: "cfg-1", Name: "redis-config"}},
		},
	}
}

// DockerHostView.Services returns a fresh slice; nil-guards return nil.
func TestHostViewsBranchcov0719late_DockerServices(t *testing.T) {
	// (a) nil receiver.
	var zero DockerHostView
	if got := zero.Services(); got != nil {
		t.Fatalf("nil receiver: expected nil, got %+v", got)
	}
	// (a) nil Docker.
	r := &Resource{ID: "dh-svc-1", Type: ResourceTypeAgent}
	if got := NewDockerHostView(r).Services(); got != nil {
		t.Fatalf("nil Docker: expected nil, got %+v", got)
	}
	// (a) Docker present, Services nil: append of nil yields nil.
	r.Docker = &DockerData{}
	if got := NewDockerHostView(r).Services(); got != nil {
		t.Fatalf("nil Services slice: expected nil, got %+v", got)
	}
	// (b) Populated: fresh slice with equal contents.
	r = dockerHostFixture()
	got := NewDockerHostView(r).Services()
	if len(got) != 1 || got[0].ID != "svc-1" || got[0].Stack != "web" {
		t.Fatalf("expected cloned services, got %+v", got)
	}
	// Element value-field mutation on clone must not affect source.
	got[0].ID = "mutated"
	got[0].Stack = "leaked"
	if r.Docker.Services[0].ID != "svc-1" || r.Docker.Services[0].Stack != "web" {
		t.Fatalf("mutation leaked to source: %+v", r.Docker.Services[0])
	}
	// Appending to clone must not affect source.
	got = append(got, models.DockerService{ID: "extra"})
	if len(r.Docker.Services) != 1 {
		t.Fatalf("append leaked to source: source len=%d want 1", len(r.Docker.Services))
	}
}

// DockerHostView.Tasks returns a fresh slice (not deep-cloned per source comment).
func TestHostViewsBranchcov0719late_DockerTasks(t *testing.T) {
	// (a) nil receiver.
	var zero DockerHostView
	if got := zero.Tasks(); got != nil {
		t.Fatalf("nil receiver: expected nil, got %+v", got)
	}
	// (a) nil Docker.
	r := &Resource{ID: "dh-task-1", Type: ResourceTypeAgent}
	if got := NewDockerHostView(r).Tasks(); got != nil {
		t.Fatalf("nil Docker: expected nil, got %+v", got)
	}
	// (a) Docker present, Tasks nil.
	r.Docker = &DockerData{}
	if got := NewDockerHostView(r).Tasks(); got != nil {
		t.Fatalf("nil Tasks slice: expected nil, got %+v", got)
	}
	// (b) Populated: fresh slice with equal contents.
	r = dockerHostFixture()
	got := NewDockerHostView(r).Tasks()
	if len(got) != 1 || got[0].ID != "task-1" || got[0].NodeID != "node-1" {
		t.Fatalf("expected cloned tasks, got %+v", got)
	}
	got[0].ID = "mutated"
	got[0].NodeID = "leaked"
	if r.Docker.Tasks[0].ID != "task-1" || r.Docker.Tasks[0].NodeID != "node-1" {
		t.Fatalf("mutation leaked to source: %+v", r.Docker.Tasks[0])
	}
}

// DockerHostView.Nodes returns a fresh slice.
func TestHostViewsBranchcov0719late_DockerNodes(t *testing.T) {
	// (a) nil receiver.
	var zero DockerHostView
	if got := zero.Nodes(); got != nil {
		t.Fatalf("nil receiver: expected nil, got %+v", got)
	}
	// (a) nil Docker.
	r := &Resource{ID: "dh-node-1", Type: ResourceTypeAgent}
	if got := NewDockerHostView(r).Nodes(); got != nil {
		t.Fatalf("nil Docker: expected nil, got %+v", got)
	}
	// (a) Docker present, Nodes nil.
	r.Docker = &DockerData{}
	if got := NewDockerHostView(r).Nodes(); got != nil {
		t.Fatalf("nil Nodes slice: expected nil, got %+v", got)
	}
	// (b) Populated: fresh slice with equal contents.
	r = dockerHostFixture()
	got := NewDockerHostView(r).Nodes()
	if len(got) != 1 || got[0].ID != "node-1" || got[0].Role != "manager" {
		t.Fatalf("expected cloned nodes, got %+v", got)
	}
	got[0].ID = "mutated"
	got[0].Role = "leaked"
	if r.Docker.Nodes[0].ID != "node-1" || r.Docker.Nodes[0].Role != "manager" {
		t.Fatalf("mutation leaked to source: %+v", r.Docker.Nodes[0])
	}
}

// DockerHostView.Secrets returns a fresh slice.
func TestHostViewsBranchcov0719late_DockerSecrets(t *testing.T) {
	// (a) nil receiver.
	var zero DockerHostView
	if got := zero.Secrets(); got != nil {
		t.Fatalf("nil receiver: expected nil, got %+v", got)
	}
	// (a) nil Docker.
	r := &Resource{ID: "dh-sec-1", Type: ResourceTypeAgent}
	if got := NewDockerHostView(r).Secrets(); got != nil {
		t.Fatalf("nil Docker: expected nil, got %+v", got)
	}
	// (a) Docker present, Secrets nil.
	r.Docker = &DockerData{}
	if got := NewDockerHostView(r).Secrets(); got != nil {
		t.Fatalf("nil Secrets slice: expected nil, got %+v", got)
	}
	// (b) Populated: fresh slice with equal contents.
	r = dockerHostFixture()
	got := NewDockerHostView(r).Secrets()
	if len(got) != 1 || got[0].ID != "sec-1" || got[0].Name != "tls-cert" {
		t.Fatalf("expected cloned secrets, got %+v", got)
	}
	got[0].ID = "mutated"
	got[0].Name = "leaked"
	if r.Docker.Secrets[0].ID != "sec-1" || r.Docker.Secrets[0].Name != "tls-cert" {
		t.Fatalf("mutation leaked to source: %+v", r.Docker.Secrets[0])
	}
}

// DockerHostView.Configs returns a fresh slice.
func TestHostViewsBranchcov0719late_DockerConfigs(t *testing.T) {
	// (a) nil receiver.
	var zero DockerHostView
	if got := zero.Configs(); got != nil {
		t.Fatalf("nil receiver: expected nil, got %+v", got)
	}
	// (a) nil Docker.
	r := &Resource{ID: "dh-cfg-1", Type: ResourceTypeAgent}
	if got := NewDockerHostView(r).Configs(); got != nil {
		t.Fatalf("nil Docker: expected nil, got %+v", got)
	}
	// (a) Docker present, Configs nil.
	r.Docker = &DockerData{}
	if got := NewDockerHostView(r).Configs(); got != nil {
		t.Fatalf("nil Configs slice: expected nil, got %+v", got)
	}
	// (b) Populated: fresh slice with equal contents.
	r = dockerHostFixture()
	got := NewDockerHostView(r).Configs()
	if len(got) != 1 || got[0].ID != "cfg-1" || got[0].Name != "redis-config" {
		t.Fatalf("expected cloned configs, got %+v", got)
	}
	got[0].ID = "mutated"
	got[0].Name = "leaked"
	if r.Docker.Configs[0].ID != "cfg-1" || r.Docker.Configs[0].Name != "redis-config" {
		t.Fatalf("mutation leaked to source: %+v", r.Docker.Configs[0])
	}
}

// DockerHostView.Hidden is a three-way short-circuit boolean.
func TestHostViewsBranchcov0719late_DockerHidden(t *testing.T) {
	// (a) nil receiver.
	var zero DockerHostView
	if zero.Hidden() {
		t.Fatal("nil receiver: expected false")
	}
	// (a) nil Docker.
	r := &Resource{ID: "dh-hid-1", Type: ResourceTypeAgent}
	if NewDockerHostView(r).Hidden() {
		t.Fatal("nil Docker: expected false")
	}
	// (b) Docker present, Hidden=false.
	r.Docker = &DockerData{Hidden: false}
	if NewDockerHostView(r).Hidden() {
		t.Fatal("Hidden=false: expected false")
	}
	// (b) Docker present, Hidden=true.
	r.Docker.Hidden = true
	if !NewDockerHostView(r).Hidden() {
		t.Fatal("Hidden=true: expected true")
	}
}

// DockerHostView.Security guards nil receiver / nil Docker / nil Security and
// returns a shallow copy with an independent AuthorizationPlugins slice.
func TestHostViewsBranchcov0719late_DockerSecurity(t *testing.T) {
	// (a) nil receiver.
	var zero DockerHostView
	if got := zero.Security(); got != nil {
		t.Fatalf("nil receiver: expected nil, got %+v", got)
	}
	// (a) nil Docker.
	r := &Resource{ID: "dh-sec-1", Type: ResourceTypeAgent}
	if got := NewDockerHostView(r).Security(); got != nil {
		t.Fatalf("nil Docker: expected nil, got %+v", got)
	}
	// (a) Docker present, Security nil.
	r.Docker = &DockerData{}
	if got := NewDockerHostView(r).Security(); got != nil {
		t.Fatalf("nil Security: expected nil, got %+v", got)
	}
	// (b) Populated: value-equal clone with independent AuthorizationPlugins.
	r = dockerHostFixture()
	got := NewDockerHostView(r).Security()
	if got == nil {
		t.Fatal("populated: expected non-nil")
	}
	if !reflect.DeepEqual(*got, *r.Docker.Security) {
		t.Fatalf("expected value-equal clone, got %+v want %+v", *got, *r.Docker.Security)
	}
	// Mutate scalar on clone; source unaffected.
	got.MutatingCommandsBlocked = false
	if r.Docker.Security.MutatingCommandsBlocked != true {
		t.Fatalf("scalar mutation leaked to source: %+v", r.Docker.Security)
	}
	// Mutate AuthorizationPlugins element; source unaffected.
	got.AuthorizationPlugins[0] = "leaked"
	if r.Docker.Security.AuthorizationPlugins[0] != "authz1" {
		t.Fatalf("AuthorizationPlugins mutation leaked: %+v", r.Docker.Security.AuthorizationPlugins)
	}
	// Append to AuthorizationPlugins; source length unchanged.
	got.AuthorizationPlugins = append(got.AuthorizationPlugins, "authz3")
	if len(r.Docker.Security.AuthorizationPlugins) != 2 {
		t.Fatalf("AuthorizationPlugins append leaked: source len=%d want 2", len(r.Docker.Security.AuthorizationPlugins))
	}
}
