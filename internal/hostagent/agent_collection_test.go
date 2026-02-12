package hostagent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ceph"
	"github.com/rcourtman/pulse-go-rewrite/internal/sensors"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

func TestAgent_collectTemperatures_MapsPowerFansAndAdditional(t *testing.T) {
	t.Parallel()

	mc := &mockCollector{
		goos:           "linux",
		sensorsLocalFn: func(context.Context) (string, error) { return "{}", nil },
		sensorsParseFn: func(string) (*sensors.TemperatureData, error) {
			return &sensors.TemperatureData{
				Available: true,
				CPUPackage: 60.5,
				Fans: map[string]float64{
					"cpu_fan": 1400,
				},
				Other: map[string]float64{
					"ddr5_dimma2": 48.2,
				},
			}, nil
		},
		sensorsPowerFn: func(context.Context) (*sensors.PowerData, error) {
			return &sensors.PowerData{
				Available:    true,
				PackageWatts: 75.3,
				DRAMWatts:    4.8,
			}, nil
		},
	}

	a := &Agent{logger: zerolog.Nop(), collector: mc}
	got := a.collectTemperatures(context.Background())

	if got.TemperatureCelsius["cpu_package"] != 60.5 {
		t.Fatalf("cpu package temp = %v, want 60.5", got.TemperatureCelsius["cpu_package"])
	}
	if got.FanRPM["cpu_fan"] != 1400 {
		t.Fatalf("fan rpm = %v, want 1400", got.FanRPM["cpu_fan"])
	}
	if got.Additional["ddr5_dimma2"] != 48.2 {
		t.Fatalf("additional temp = %v, want 48.2", got.Additional["ddr5_dimma2"])
	}
	if got.PowerWatts["cpu_package"] != 75.3 {
		t.Fatalf("cpu package watts = %v, want 75.3", got.PowerWatts["cpu_package"])
	}
	if got.PowerWatts["dram"] != 4.8 {
		t.Fatalf("dram watts = %v, want 4.8", got.PowerWatts["dram"])
	}
	if _, ok := got.PowerWatts["cpu_core"]; ok {
		t.Fatal("cpu_core power should be omitted when zero")
	}
}

func TestAgent_collectRAIDArrays_GuardsAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("non-linux skips collection", func(t *testing.T) {
		called := false
		mc := &mockCollector{
			goos: "darwin",
			raidArraysFn: func(context.Context) ([]agentshost.RAIDArray, error) {
				called = true
				return nil, nil
			},
		}
		a := &Agent{logger: zerolog.Nop(), collector: mc}

		if got := a.collectRAIDArrays(context.Background()); got != nil {
			t.Fatalf("collectRAIDArrays() = %#v, want nil", got)
		}
		if called {
			t.Fatal("raidArraysFn should not be called on non-linux hosts")
		}
	})

	t.Run("collector error returns nil", func(t *testing.T) {
		mc := &mockCollector{
			goos:         "linux",
			raidArraysFn: func(context.Context) ([]agentshost.RAIDArray, error) { return nil, errors.New("mdadm missing") },
		}
		a := &Agent{logger: zerolog.Nop(), collector: mc}

		if got := a.collectRAIDArrays(context.Background()); got != nil {
			t.Fatalf("collectRAIDArrays() = %#v, want nil", got)
		}
	})

	t.Run("successful collection returns arrays", func(t *testing.T) {
		want := []agentshost.RAIDArray{
			{Name: "md0", State: "clean", Level: "raid1"},
		}
		mc := &mockCollector{
			goos:         "linux",
			raidArraysFn: func(context.Context) ([]agentshost.RAIDArray, error) { return want, nil },
		}
		a := &Agent{logger: zerolog.Nop(), collector: mc}

		got := a.collectRAIDArrays(context.Background())
		if len(got) != 1 || got[0].Name != "md0" || got[0].State != "clean" {
			t.Fatalf("collectRAIDArrays() = %#v, want %#v", got, want)
		}
	})
}

func TestAgent_collectCephStatus_GuardsAndConversion(t *testing.T) {
	t.Parallel()

	t.Run("disabled ceph skips collection", func(t *testing.T) {
		called := false
		mc := &mockCollector{
			goos: "linux",
			cephStatusFn: func(context.Context) (*ceph.ClusterStatus, error) {
				called = true
				return &ceph.ClusterStatus{}, nil
			},
		}
		a := &Agent{
			cfg:       Config{DisableCeph: true},
			logger:    zerolog.Nop(),
			collector: mc,
		}

		if got := a.collectCephStatus(context.Background()); got != nil {
			t.Fatalf("collectCephStatus() = %#v, want nil", got)
		}
		if called {
			t.Fatal("cephStatusFn should not be called when ceph collection is disabled")
		}
	})

	t.Run("non-linux skips collection", func(t *testing.T) {
		called := false
		mc := &mockCollector{
			goos: "windows",
			cephStatusFn: func(context.Context) (*ceph.ClusterStatus, error) {
				called = true
				return &ceph.ClusterStatus{}, nil
			},
		}
		a := &Agent{logger: zerolog.Nop(), collector: mc}

		if got := a.collectCephStatus(context.Background()); got != nil {
			t.Fatalf("collectCephStatus() = %#v, want nil", got)
		}
		if called {
			t.Fatal("cephStatusFn should not be called on non-linux hosts")
		}
	})

	t.Run("collector error returns nil", func(t *testing.T) {
		mc := &mockCollector{
			goos:         "linux",
			cephStatusFn: func(context.Context) (*ceph.ClusterStatus, error) { return nil, errors.New("ceph not available") },
		}
		a := &Agent{logger: zerolog.Nop(), collector: mc}

		if got := a.collectCephStatus(context.Background()); got != nil {
			t.Fatalf("collectCephStatus() = %#v, want nil", got)
		}
	})

	t.Run("nil collector status returns nil", func(t *testing.T) {
		mc := &mockCollector{
			goos:         "linux",
			cephStatusFn: func(context.Context) (*ceph.ClusterStatus, error) { return nil, nil },
		}
		a := &Agent{logger: zerolog.Nop(), collector: mc}

		if got := a.collectCephStatus(context.Background()); got != nil {
			t.Fatalf("collectCephStatus() = %#v, want nil", got)
		}
	})

	t.Run("maps ceph status fields into report types", func(t *testing.T) {
		collectedAt := time.Date(2026, time.January, 15, 10, 11, 12, 0, time.UTC)

		mc := &mockCollector{
			goos: "linux",
			cephStatusFn: func(context.Context) (*ceph.ClusterStatus, error) {
				return &ceph.ClusterStatus{
					FSID: "ceph-fsid-1",
					Health: ceph.HealthStatus{
						Status: "HEALTH_WARN",
						Checks: map[string]ceph.Check{
							"OSD_DOWN": {
								Severity: "HEALTH_WARN",
								Message:  "1 osd down",
								Detail:   []string{"osd.3 down"},
							},
						},
						Summary: []ceph.HealthSummary{
							{Severity: "HEALTH_WARN", Message: "Reduced data availability"},
						},
					},
					MonMap: ceph.MonitorMap{
						Epoch:   9,
						NumMons: 1,
						Monitors: []ceph.Monitor{
							{Name: "mon.a", Rank: 0, Addr: "10.0.0.11:6789", Status: "leader"},
						},
					},
					MgrMap: ceph.ManagerMap{
						Available: true,
						NumMgrs:   2,
						ActiveMgr: "mgr.x",
						Standbys:  1,
					},
					OSDMap: ceph.OSDMap{
						Epoch:   12,
						NumOSDs: 3,
						NumUp:   2,
						NumIn:   2,
						NumDown: 1,
						NumOut:  1,
					},
					PGMap: ceph.PGMap{
						NumPGs:         128,
						BytesTotal:     1000,
						BytesUsed:      500,
						BytesAvailable: 500,
						UsagePercent:   50,
					},
					Pools: []ceph.Pool{
						{ID: 7, Name: "rbd", BytesUsed: 10, BytesAvailable: 90, Objects: 42, PercentUsed: 10},
					},
					Services: []ceph.ServiceInfo{
						{Type: "mgr", Running: 1, Total: 2, Daemons: []string{"mgr.x"}},
					},
					CollectedAt: collectedAt,
				}, nil
			},
		}
		a := &Agent{logger: zerolog.Nop(), collector: mc}

		got := a.collectCephStatus(context.Background())
		if got == nil {
			t.Fatal("collectCephStatus() returned nil")
		}

		if got.FSID != "ceph-fsid-1" {
			t.Fatalf("FSID = %q, want ceph-fsid-1", got.FSID)
		}
		if got.CollectedAt != collectedAt.Format(time.RFC3339) {
			t.Fatalf("CollectedAt = %q, want %q", got.CollectedAt, collectedAt.Format(time.RFC3339))
		}
		if got.Health.Status != "HEALTH_WARN" {
			t.Fatalf("Health.Status = %q, want HEALTH_WARN", got.Health.Status)
		}
		if check, ok := got.Health.Checks["OSD_DOWN"]; !ok || check.Message != "1 osd down" || len(check.Detail) != 1 || check.Detail[0] != "osd.3 down" {
			t.Fatalf("Health.Checks[OSD_DOWN] = %#v, want mapped check", check)
		}
		if len(got.MonMap.Monitors) != 1 || got.MonMap.Monitors[0].Name != "mon.a" {
			t.Fatalf("MonMap.Monitors = %#v, want mon.a monitor", got.MonMap.Monitors)
		}
		if got.MgrMap.ActiveMgr != "mgr.x" || !got.MgrMap.Available {
			t.Fatalf("MgrMap = %#v, want active mgr.x and available=true", got.MgrMap)
		}
		if len(got.Pools) != 1 || got.Pools[0].Name != "rbd" {
			t.Fatalf("Pools = %#v, want one rbd pool", got.Pools)
		}
		if len(got.Services) != 1 || got.Services[0].Type != "mgr" {
			t.Fatalf("Services = %#v, want one mgr service", got.Services)
		}
	})
}
