package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBuildCephClusterModelServiceRoles(t *testing.T) {
	for _, tc := range []struct {
		service, status string
		wantRunning     int
	}{
		{"mgr", "active", 1},
		{"mgr", "running", 1},
		{"mgr", "standby", 1},
		{"MGR", "STANDBY", 1},
		{"mgr", "stopped", 0},
		{"mgr", "down", 0},
		{"mgr", "unknown", 0},
		{"mgr", "", 0},
		{"mon", "running", 1},
		{"mon", "standby", 0},
		{"osd", "running", 1},
		{"osd", "standby", 0},
	} {
		t.Run(tc.service+"/"+tc.status, func(t *testing.T) {
			status := &proxmox.CephStatus{
				Health: proxmox.CephHealth{Status: "HEALTH_OK"},
				ServiceMap: proxmox.CephServiceMap{Services: map[string]proxmox.CephServiceDefinition{
					tc.service: {Daemons: map[string]proxmox.CephServiceDaemon{
						"daemon": {Host: "node", Status: tc.status},
					}},
				}},
			}
			cluster := buildCephClusterModel("pve", status, nil)
			want := models.CephServiceStatus{Type: tc.service, Running: tc.wantRunning, Total: 1}
			if tc.wantRunning == 0 {
				want.Message = "Offline: daemon@node"
			}
			assert.Equal(t, []models.CephServiceStatus{want}, cluster.Services)
		})
	}
}

// Exercise the API poll and accepted host report, rather than making agent
// data canonical to hide an inconsistent API service row (issue #1626).
func TestPollCephClusterMixedSourceServiceHealth(t *testing.T) {
	for _, scenario := range []string{"healthy", "manager_down", "manager_unavailable", "mon_osd_down", "summary_only"} {
		for _, apiFirst := range []bool{true, false} {
			order := "agent_then_api"
			if apiFirst {
				order = "api_then_agent"
			}
			t.Run(scenario+"/"+order, func(t *testing.T) {
				snapshot := cephMixedSourceServiceHealthSnapshot(t, scenario, apiFirst)
				if !assert.Len(t, snapshot.CephClusters, 1) || !assert.Len(t, snapshot.Hosts, 1) {
					return
				}
				cluster := snapshot.CephClusters[0]
				assert.Equal(t, models.CephClusterSourceProxmoxAPI, cluster.Source)
				assert.Equal(t, "pve-api-ceph-mixed-services", cluster.ID)
				assert.Equal(t, 3, cluster.NumMons)
				assert.Equal(t, 2, cluster.NumMgrs)
				services := make(map[string]models.CephServiceStatus)
				for _, service := range cluster.Services {
					services[service.Type] = service
				}
				mgr := models.CephServiceStatus{Type: "mgr", Running: 2, Total: 2}
				health := "HEALTH_OK"
				switch scenario {
				case "manager_down":
					mgr.Running, mgr.Message = 1, "Offline: spare@pve-b"
					health = "HEALTH_WARN"
				case "manager_unavailable":
					mgr.Running = 0
					assert.Contains(t, []string{"Offline: active@pve-a, spare@pve-b", "Offline: spare@pve-b, active@pve-a"}, services["mgr"].Message)
					mgr.Message = services["mgr"].Message
					health = "HEALTH_ERR"
					if assert.Len(t, cluster.HealthChecks, 1) {
						assert.Equal(t, "MGR_DOWN", cluster.HealthChecks[0].Code)
					}
				case "mon_osd_down":
					health = "HEALTH_WARN"
					assert.Equal(t, models.CephServiceStatus{Type: "mon", Running: 2, Total: 3, Message: "Offline: c@pve-c"}, services["mon"])
					assert.Equal(t, models.CephServiceStatus{Type: "osd", Running: 2, Total: 3, Message: "Offline: 2@pve-c"}, services["osd"])
					assert.Equal(t, 2, cluster.NumOSDsUp)
					assert.Len(t, cluster.HealthChecks, 2)
					codes := make([]string, 0, len(cluster.HealthChecks))
					for _, check := range cluster.HealthChecks {
						codes = append(codes, check.Code)
					}
					assert.ElementsMatch(t, []string{"MON_DOWN", "OSD_DOWN"}, codes)
					assert.Contains(t, cluster.HealthMessage, "monitor down")
					assert.Contains(t, cluster.HealthMessage, "osd down")
				}
				assert.Equal(t, mgr, services["mgr"])
				assert.Equal(t, health, cluster.Health)
				if health == "HEALTH_OK" {
					assert.Empty(t, cluster.HealthChecks)
					assert.Empty(t, cluster.HealthMessage)
				}
				if assert.NotNil(t, snapshot.Hosts[0].Ceph) {
					// The intentionally healthy agent must not erase API down
					// evidence; its own machine projection remains independent.
					for _, service := range snapshot.Hosts[0].Ceph.Services {
						if service.Type == "mgr" {
							assert.Equal(t, 2, service.Running)
							assert.Equal(t, 2, service.Total)
						}
					}
				}
			})
		}
	}
}

func cephMixedSourceServiceHealthSnapshot(t *testing.T, scenario string, apiFirst bool) models.StateSnapshot {
	t.Helper()
	var status proxmox.CephStatus
	err := json.Unmarshal([]byte(`{
		"fsid":"ceph-mixed-services","health":{"status":"HEALTH_OK"},
		"monmap":{"num_mons":3},"mgrmap":{"available":true,"num_standbys":1},
		"osdmap":{"num_osds":3,"num_up_osds":3,"num_in_osds":3},
		"pgmap":{"num_pgs":32,"bytes_total":1000,"bytes_used":100,"bytes_avail":900},
		"servicemap":{"services":{
			"mon":{"daemons":{"a":{"hostname":"pve-a","status":"running"},"b":{"hostname":"pve-b","status":"running"},"c":{"hostname":"pve-c","status":"running"}}},
			"mgr":{"daemons":{"active":{"hostname":"pve-a","status":"active"},"spare":{"hostname":"pve-b","status":"standby"}}},
			"osd":{"daemons":{"0":{"hostname":"pve-a","status":"running"},"1":{"hostname":"pve-b","status":"running"},"2":{"hostname":"pve-c","status":"running"}}}
		}}
	}`), &status)
	if err != nil {
		t.Fatal(err)
	}
	setDaemonStatus := func(service, daemon, state string) {
		d := status.ServiceMap.Services[service].Daemons[daemon]
		d.Status = state
		status.ServiceMap.Services[service].Daemons[daemon] = d
	}
	switch scenario {
	case "healthy":
	case "manager_down":
		setDaemonStatus("mgr", "spare", "stopped")
		status.Health.Status = "HEALTH_WARN"
	case "manager_unavailable":
		setDaemonStatus("mgr", "active", "stopped")
		setDaemonStatus("mgr", "spare", "stopped")
		status.MgrMap = proxmox.CephMgrMap{Available: false, NumMgrs: 2}
		status.Health.Status = "HEALTH_ERR"
		status.Health.Checks = map[string]proxmox.CephHealthCheckRaw{
			"MGR_DOWN": {Severity: "HEALTH_ERR", Summary: json.RawMessage(`{"message":"no active manager"}`)},
		}
	case "mon_osd_down":
		setDaemonStatus("mon", "c", "stopped")
		setDaemonStatus("osd", "2", "down")
		status.OSDMap.NumUpOSDs = 2
		status.Health.Status = "HEALTH_WARN"
		status.Health.Checks = map[string]proxmox.CephHealthCheckRaw{
			"MON_DOWN": {Severity: "HEALTH_WARN", Summary: json.RawMessage(`{"message":"1 monitor down"}`)},
			"OSD_DOWN": {Severity: "HEALTH_WARN", Summary: json.RawMessage(`{"message":"1 osd down"}`)},
		}
	case "summary_only":
		status.ServiceMap.Services = nil
	default:
		t.Fatalf("unknown Ceph service scenario %q", scenario)
	}
	m := &Monitor{state: models.NewState(), alertManager: alerts.NewManager(), config: &config.Config{},
		hostTokenBindings: make(map[string]string), rateTracker: NewRateTracker()}
	t.Cleanup(m.alertManager.Stop)
	client := &mockCephPVEClient{}
	client.On("GetCephStatus", mock.Anything).Return(&status, nil).Once()
	client.On("GetCephDF", mock.Anything).Return(nil, nil).Once()
	t.Cleanup(func() { client.AssertExpectations(t) })
	applyAPI := func() { m.pollCephCluster(context.Background(), "pve-api", client, true) }
	applyAgent := func() {
		_, err := m.ApplyHostReport(agentshost.Report{
			Agent:     agentshost.AgentInfo{ID: "mixed-agent", Version: "6.7.0", IntervalSeconds: 30},
			Host:      agentshost.HostInfo{ID: "mixed-host", Hostname: "pve-a", Platform: "linux"},
			Timestamp: time.Now().UTC(),
			Ceph: &agentshost.CephCluster{
				FSID: "ceph-mixed-services", Health: agentshost.CephHealth{Status: "HEALTH_OK"},
				MonMap: agentshost.CephMonitorMap{NumMons: 3},
				MgrMap: agentshost.CephManagerMap{Available: true, NumMgrs: 2, Standbys: 1},
				OSDMap: agentshost.CephOSDMap{NumOSDs: 3, NumUp: 3, NumIn: 3},
				Services: []agentshost.CephService{
					{Type: "mon", Running: 3, Total: 3}, {Type: "mgr", Running: 1, Total: 2}, {Type: "osd", Running: 3, Total: 3},
				},
			},
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	if apiFirst {
		applyAPI()
		applyAgent()
	} else {
		applyAgent()
		applyAPI()
	}
	return m.state.GetSnapshot()
}

type mockCephPVEClient struct {
	mock.Mock
	PVEClientInterface
}

func (m *mockCephPVEClient) GetCephStatus(ctx context.Context) (*proxmox.CephStatus, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*proxmox.CephStatus), args.Error(1)
}

func (m *mockCephPVEClient) GetCephDF(ctx context.Context) (*proxmox.CephDF, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*proxmox.CephDF), args.Error(1)
}

func TestPollCephCluster(t *testing.T) {
	t.Run("clears state when ceph not detected", func(t *testing.T) {
		m := &Monitor{state: models.NewState()}
		m.state.UpdateCephClustersForInstance("pve1", []models.CephCluster{{ID: "old", Instance: "pve1"}})

		m.pollCephCluster(context.Background(), "pve1", nil, false)

		clusters := m.state.GetSnapshot().CephClusters
		assert.Empty(t, clusters)
	})

	t.Run("handles status error", func(t *testing.T) {
		m := &Monitor{state: models.NewState()}
		client := &mockCephPVEClient{}
		client.On("GetCephStatus", mock.Anything).Return(nil, fmt.Errorf("api error"))

		m.pollCephCluster(context.Background(), "pve1", client, true)

		clusters := m.state.GetSnapshot().CephClusters
		assert.Empty(t, clusters)
	})

	t.Run("handles nil status", func(t *testing.T) {
		m := &Monitor{state: models.NewState()}
		client := &mockCephPVEClient{}
		client.On("GetCephStatus", mock.Anything).Return(nil, nil)

		m.pollCephCluster(context.Background(), "pve1", client, true)

		clusters := m.state.GetSnapshot().CephClusters
		assert.Empty(t, clusters)
	})

	t.Run("successful poll with status only", func(t *testing.T) {
		m := &Monitor{state: models.NewState()}
		client := &mockCephPVEClient{}

		status := &proxmox.CephStatus{
			FSID:   "fsid123",
			Health: proxmox.CephHealth{Status: "HEALTH_OK"},
		}
		client.On("GetCephStatus", mock.Anything).Return(status, nil)
		client.On("GetCephDF", mock.Anything).Return(nil, fmt.Errorf("df error"))

		m.pollCephCluster(context.Background(), "pve1", client, true)

		clusters := m.state.GetSnapshot().CephClusters
		assert.Len(t, clusters, 1)
		assert.Equal(t, "pve1-fsid123", clusters[0].ID)
		assert.Equal(t, "HEALTH_OK", clusters[0].Health)
	})

	t.Run("successful poll with full data", func(t *testing.T) {
		m := &Monitor{state: models.NewState()}
		client := &mockCephPVEClient{}

		status := &proxmox.CephStatus{
			FSID:   "fsid123",
			Health: proxmox.CephHealth{Status: "HEALTH_OK"},
		}
		df := &proxmox.CephDF{
			Data: proxmox.CephDFData{
				Stats: proxmox.CephDFStats{
					TotalBytes:     1000,
					TotalUsedBytes: 200,
				},
			},
		}
		client.On("GetCephStatus", mock.Anything).Return(status, nil)
		client.On("GetCephDF", mock.Anything).Return(df, nil)

		m.pollCephCluster(context.Background(), "pve1", client, true)

		clusters := m.state.GetSnapshot().CephClusters
		assert.Len(t, clusters, 1)
		assert.Equal(t, int64(1000), clusters[0].TotalBytes)
		assert.Equal(t, int64(200), clusters[0].UsedBytes)
	})
}

func TestPollCephClusterChecksPoolStorageThresholds(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	defer manager.Stop()
	manager.UpdateConfig(alerts.AlertConfig{
		Enabled:         true,
		ActivationState: alerts.ActivationActive,
		StorageDefault:  alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
		TimeThresholds:  map[string]int{"storage": 0},
		Overrides: map[string]alerts.ThresholdConfig{
			models.CephPoolStorageID("pve1", "data_replication"): {
				Usage: &alerts.HysteresisThreshold{Trigger: 50, Clear: 45},
			},
		},
	})

	m := &Monitor{
		state:        models.NewState(),
		alertManager: manager,
	}
	client := &mockCephPVEClient{}
	status := &proxmox.CephStatus{
		FSID:   "fsid123",
		Health: proxmox.CephHealth{Status: "HEALTH_OK"},
	}
	df := &proxmox.CephDF{
		Data: proxmox.CephDFData{
			Pools: []proxmox.CephDFPool{
				{
					ID:   1,
					Name: "data_replication",
					Stats: proxmox.CephDFPoolStat{
						BytesUsed:   70,
						MaxAvail:    30,
						PercentUsed: 70,
					},
				},
			},
		},
	}
	client.On("GetCephStatus", mock.Anything).Return(status, nil)
	client.On("GetCephDF", mock.Anything).Return(df, nil)

	m.pollCephCluster(context.Background(), "pve1", client, true)

	active := manager.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("active alerts = %+v, want one Ceph pool usage alert", active)
	}
	if active[0].ResourceID != models.CephPoolStorageID("pve1", "data_replication") {
		t.Fatalf("ResourceID = %q, want %q", active[0].ResourceID, models.CephPoolStorageID("pve1", "data_replication"))
	}
	if active[0].Threshold != 50 {
		t.Fatalf("Threshold = %v, want override threshold 50", active[0].Threshold)
	}
}

func TestIsCephStorageType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		storageType string
		want        bool
	}{
		// Valid Ceph types
		{"rbd lowercase", "rbd", true},
		{"cephfs lowercase", "cephfs", true},
		{"ceph lowercase", "ceph", true},

		// Case variations
		{"RBD uppercase", "RBD", true},
		{"CephFS mixed case", "CephFS", true},
		{"CEPH uppercase", "CEPH", true},

		// Whitespace handling
		{"rbd with leading space", " rbd", true},
		{"rbd with trailing space", "rbd ", true},
		{"rbd with surrounding spaces", "  rbd  ", true},

		// Non-Ceph storage types
		{"local storage", "local", false},
		{"dir storage", "dir", false},
		{"nfs storage", "nfs", false},
		{"lvm storage", "lvm", false},
		{"zfs storage", "zfs", false},
		{"zfspool storage", "zfspool", false},
		{"iscsi storage", "iscsi", false},

		// Edge cases
		{"empty string", "", false},
		{"whitespace only", "   ", false},
		{"partial match rbd", "rbd-pool", false},
		{"partial match ceph", "ceph-pool", false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isCephStorageType(tc.storageType); got != tc.want {
				t.Fatalf("isCephStorageType(%q) = %v, want %v", tc.storageType, got, tc.want)
			}
		})
	}
}

func TestBuildCephClusterModelPreservesNativeHealthCheckCodes(t *testing.T) {
	status := &proxmox.CephStatus{
		FSID: "fsid-native",
		Health: proxmox.CephHealth{
			Status: "HEALTH_WARN",
			Checks: map[string]proxmox.CephHealthCheckRaw{
				"OSD_DOWN": {
					Severity: "HEALTH_WARN",
					Summary:  json.RawMessage(`{"message":"1 osd down"}`),
				},
				"PG_DEGRADED": {
					Severity: "HEALTH_WARN",
					Detail:   []proxmox.CephCheckDetail{{Message: "42 pgs degraded"}},
				},
			},
		},
	}

	cluster := buildCephClusterModel("pve-a", status, nil)
	if len(cluster.HealthChecks) != 2 {
		t.Fatalf("health checks = %+v", cluster.HealthChecks)
	}
	if cluster.HealthChecks[0].Code != "OSD_DOWN" || cluster.HealthChecks[0].Summary != "1 osd down" {
		t.Fatalf("sorted native check 0 = %+v", cluster.HealthChecks[0])
	}
	if cluster.HealthChecks[1].Code != "PG_DEGRADED" || cluster.HealthChecks[1].Summary != "42 pgs degraded" {
		t.Fatalf("sorted native check 1 = %+v", cluster.HealthChecks[1])
	}
}

func TestCountServiceDaemons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		services    map[string]proxmox.CephServiceDefinition
		serviceType string
		want        int
	}{
		{
			name:        "nil services map",
			services:    nil,
			serviceType: "mon",
			want:        0,
		},
		{
			name:        "empty services map",
			services:    map[string]proxmox.CephServiceDefinition{},
			serviceType: "mon",
			want:        0,
		},
		{
			name: "service type not found",
			services: map[string]proxmox.CephServiceDefinition{
				"mon": {
					Daemons: map[string]proxmox.CephServiceDaemon{
						"a": {Host: "node1", Status: "running"},
					},
				},
			},
			serviceType: "osd",
			want:        0,
		},
		{
			name: "single daemon",
			services: map[string]proxmox.CephServiceDefinition{
				"mon": {
					Daemons: map[string]proxmox.CephServiceDaemon{
						"a": {Host: "node1", Status: "running"},
					},
				},
			},
			serviceType: "mon",
			want:        1,
		},
		{
			name: "multiple daemons",
			services: map[string]proxmox.CephServiceDefinition{
				"mon": {
					Daemons: map[string]proxmox.CephServiceDaemon{
						"a": {Host: "node1", Status: "running"},
						"b": {Host: "node2", Status: "running"},
						"c": {Host: "node3", Status: "running"},
					},
				},
			},
			serviceType: "mon",
			want:        3,
		},
		{
			name: "multiple service types",
			services: map[string]proxmox.CephServiceDefinition{
				"mon": {
					Daemons: map[string]proxmox.CephServiceDaemon{
						"a": {Host: "node1", Status: "running"},
						"b": {Host: "node2", Status: "running"},
					},
				},
				"mgr": {
					Daemons: map[string]proxmox.CephServiceDaemon{
						"node1": {Host: "node1", Status: "active"},
					},
				},
			},
			serviceType: "mgr",
			want:        1,
		},
		{
			name: "empty daemons map",
			services: map[string]proxmox.CephServiceDefinition{
				"mon": {
					Daemons: map[string]proxmox.CephServiceDaemon{},
				},
			},
			serviceType: "mon",
			want:        0,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := countServiceDaemons(tc.services, tc.serviceType); got != tc.want {
				t.Fatalf("countServiceDaemons(%v, %q) = %d, want %d", tc.services, tc.serviceType, got, tc.want)
			}
		})
	}
}

func TestCountCephMonitorDaemons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status *proxmox.CephStatus
		want   int
	}{
		{
			name: "uses explicit mon count",
			status: &proxmox.CephStatus{
				MonMap: proxmox.CephMonMap{NumMons: 3},
				ServiceMap: proxmox.CephServiceMap{
					Services: map[string]proxmox.CephServiceDefinition{
						"mon": {
							Daemons: map[string]proxmox.CephServiceDaemon{
								"a": {Host: "node1", Status: "running"},
							},
						},
					},
				},
			},
			want: 3,
		},
		{
			name: "uses mons fallback",
			status: &proxmox.CephStatus{
				MonMap: proxmox.CephMonMap{
					Mons: []proxmox.CephMonitor{{Name: "a"}, {Name: "b"}},
				},
			},
			want: 2,
		},
		{
			name: "uses quorum names fallback",
			status: &proxmox.CephStatus{
				MonMap: proxmox.CephMonMap{
					QuorumNames: []string{"a", "b", "c"},
				},
				ServiceMap: proxmox.CephServiceMap{
					Services: map[string]proxmox.CephServiceDefinition{
						"mon": {
							Daemons: map[string]proxmox.CephServiceDaemon{
								"a": {Host: "node1", Status: "running"},
							},
						},
					},
				},
			},
			want: 3,
		},
		{
			name: "falls back to service map",
			status: &proxmox.CephStatus{
				ServiceMap: proxmox.CephServiceMap{
					Services: map[string]proxmox.CephServiceDefinition{
						"mon": {
							Daemons: map[string]proxmox.CephServiceDaemon{
								"a": {Host: "node1", Status: "running"},
								"b": {Host: "node2", Status: "running"},
							},
						},
					},
				},
			},
			want: 2,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := countCephMonitorDaemons(tc.status); got != tc.want {
				t.Fatalf("countCephMonitorDaemons() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCountCephManagerDaemons(t *testing.T) {
	t.Parallel()

	status := &proxmox.CephStatus{
		MgrMap: proxmox.CephMgrMap{
			ActiveName: "mgr-a",
			Standbys:   []string{"mgr-b", "mgr-c"},
		},
	}
	if got := countCephManagerDaemons(status); got != 3 {
		t.Fatalf("countCephManagerDaemons() = %d, want 3", got)
	}
}

func TestExtractCephCheckSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		// Empty/nil cases
		{
			name: "nil raw message",
			raw:  nil,
			want: "",
		},
		{
			name: "empty raw message",
			raw:  json.RawMessage{},
			want: "",
		},

		// Object format with message field
		{
			name: "object with message",
			raw:  json.RawMessage(`{"message": "1 slow ops"}`),
			want: "1 slow ops",
		},
		{
			name: "object with summary",
			raw:  json.RawMessage(`{"summary": "health warning"}`),
			want: "health warning",
		},
		{
			name: "object with both message and summary prefers message",
			raw:  json.RawMessage(`{"message": "from message", "summary": "from summary"}`),
			want: "from message",
		},
		{
			name: "object with empty message falls back to summary",
			raw:  json.RawMessage(`{"message": "", "summary": "fallback summary"}`),
			want: "fallback summary",
		},

		// Array format
		{
			name: "array with single item message",
			raw:  json.RawMessage(`[{"message": "array message"}]`),
			want: "array message",
		},
		{
			name: "array with single item summary",
			raw:  json.RawMessage(`[{"summary": "array summary"}]`),
			want: "array summary",
		},
		{
			name: "array with multiple items returns first",
			raw:  json.RawMessage(`[{"message": "first"}, {"message": "second"}]`),
			want: "first",
		},
		{
			name: "array skips empty to find message",
			raw:  json.RawMessage(`[{"message": ""}, {"message": "second"}]`),
			want: "second",
		},
		{
			name: "empty array",
			raw:  json.RawMessage(`[]`),
			want: "",
		},

		// Plain string format
		{
			name: "plain string",
			raw:  json.RawMessage(`"simple string message"`),
			want: "simple string message",
		},
		{
			name: "empty string",
			raw:  json.RawMessage(`""`),
			want: "",
		},

		// Invalid JSON
		{
			name: "invalid JSON",
			raw:  json.RawMessage(`{invalid`),
			want: "",
		},
		{
			name: "number value",
			raw:  json.RawMessage(`123`),
			want: "",
		},
		{
			name: "boolean value",
			raw:  json.RawMessage(`true`),
			want: "",
		},
		{
			name: "null value",
			raw:  json.RawMessage(`null`),
			want: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := extractCephCheckSummary(tc.raw); got != tc.want {
				t.Fatalf("extractCephCheckSummary(%s) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestSummarizeCephHealth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status *proxmox.CephStatus
		want   string
	}{
		{
			name:   "nil status",
			status: nil,
			want:   "",
		},
		{
			name:   "empty status",
			status: &proxmox.CephStatus{},
			want:   "",
		},
		{
			name: "single summary message",
			status: &proxmox.CephStatus{
				Health: proxmox.CephHealth{
					Summary: []proxmox.CephHealthSummary{
						{Message: "HEALTH_WARN"},
					},
				},
			},
			want: "HEALTH_WARN",
		},
		{
			name: "summary with summary field instead of message",
			status: &proxmox.CephStatus{
				Health: proxmox.CephHealth{
					Summary: []proxmox.CephHealthSummary{
						{Summary: "1 pool(s) have no replicas configured"},
					},
				},
			},
			want: "1 pool(s) have no replicas configured",
		},
		{
			name: "multiple summary messages",
			status: &proxmox.CephStatus{
				Health: proxmox.CephHealth{
					Summary: []proxmox.CephHealthSummary{
						{Message: "first warning"},
						{Message: "second warning"},
					},
				},
			},
			want: "first warning; second warning",
		},
		{
			name: "health check with summary object",
			status: &proxmox.CephStatus{
				Health: proxmox.CephHealth{
					Checks: map[string]proxmox.CephHealthCheckRaw{
						"SLOW_OPS": {
							Summary: json.RawMessage(`{"message": "1 slow ops"}`),
						},
					},
				},
			},
			want: "SLOW_OPS: 1 slow ops",
		},
		{
			name: "health check with detail fallback",
			status: &proxmox.CephStatus{
				Health: proxmox.CephHealth{
					Checks: map[string]proxmox.CephHealthCheckRaw{
						"OSD_DOWN": {
							Summary: json.RawMessage(`{}`),
							Detail: []proxmox.CephCheckDetail{
								{Message: "osd.0 is down"},
							},
						},
					},
				},
			},
			want: "OSD_DOWN: osd.0 is down",
		},
		{
			name: "combined summary and checks",
			status: &proxmox.CephStatus{
				Health: proxmox.CephHealth{
					Summary: []proxmox.CephHealthSummary{
						{Message: "HEALTH_WARN"},
					},
					Checks: map[string]proxmox.CephHealthCheckRaw{
						"PG_DEGRADED": {
							Summary: json.RawMessage(`{"message": "Degraded data redundancy"}`),
						},
					},
				},
			},
			want: "HEALTH_WARN; PG_DEGRADED: Degraded data redundancy",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := summarizeCephHealth(tc.status); got != tc.want {
				t.Fatalf("summarizeCephHealth() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildCephClusterModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		instanceName string
		status       *proxmox.CephStatus
		df           *proxmox.CephDF
		check        func(t *testing.T, cluster models.CephCluster)
	}{
		{
			name:         "basic case with minimal status data",
			instanceName: "pve-cluster",
			status: &proxmox.CephStatus{
				FSID: "abc123",
				PGMap: proxmox.CephPGMap{
					BytesTotal: 1000000,
					BytesUsed:  400000,
					BytesAvail: 600000,
				},
			},
			df: nil,
			check: func(t *testing.T, cluster models.CephCluster) {
				if cluster.ID != "pve-cluster-abc123" {
					t.Errorf("ID = %q, want %q", cluster.ID, "pve-cluster-abc123")
				}
				if cluster.Instance != "pve-cluster" {
					t.Errorf("Instance = %q, want %q", cluster.Instance, "pve-cluster")
				}
				if cluster.FSID != "abc123" {
					t.Errorf("FSID = %q, want %q", cluster.FSID, "abc123")
				}
				if cluster.TotalBytes != 1000000 {
					t.Errorf("TotalBytes = %d, want %d", cluster.TotalBytes, 1000000)
				}
				if cluster.UsedBytes != 400000 {
					t.Errorf("UsedBytes = %d, want %d", cluster.UsedBytes, 400000)
				}
				if cluster.AvailableBytes != 600000 {
					t.Errorf("AvailableBytes = %d, want %d", cluster.AvailableBytes, 600000)
				}
			},
		},
		{
			name:         "DF data overrides PGMap data",
			instanceName: "test-instance",
			status: &proxmox.CephStatus{
				FSID: "fsid-456",
				PGMap: proxmox.CephPGMap{
					BytesTotal: 1000,
					BytesUsed:  500,
					BytesAvail: 500,
				},
			},
			df: &proxmox.CephDF{
				Data: proxmox.CephDFData{
					Stats: proxmox.CephDFStats{
						TotalBytes:      2000000,
						TotalUsedBytes:  800000,
						TotalAvailBytes: 1200000,
					},
				},
			},
			check: func(t *testing.T, cluster models.CephCluster) {
				if cluster.TotalBytes != 2000000 {
					t.Errorf("TotalBytes = %d, want %d (DF should override PGMap)", cluster.TotalBytes, 2000000)
				}
				if cluster.UsedBytes != 800000 {
					t.Errorf("UsedBytes = %d, want %d (DF should override PGMap)", cluster.UsedBytes, 800000)
				}
				if cluster.AvailableBytes != 1200000 {
					t.Errorf("AvailableBytes = %d, want %d (DF should override PGMap)", cluster.AvailableBytes, 1200000)
				}
			},
		},
		{
			name:         "DF nil uses PGMap values",
			instanceName: "pgmap-test",
			status: &proxmox.CephStatus{
				FSID: "fsid-789",
				PGMap: proxmox.CephPGMap{
					BytesTotal: 5000000,
					BytesUsed:  1500000,
					BytesAvail: 3500000,
				},
			},
			df: nil,
			check: func(t *testing.T, cluster models.CephCluster) {
				if cluster.TotalBytes != 5000000 {
					t.Errorf("TotalBytes = %d, want %d", cluster.TotalBytes, 5000000)
				}
				if cluster.UsedBytes != 1500000 {
					t.Errorf("UsedBytes = %d, want %d", cluster.UsedBytes, 1500000)
				}
				if cluster.AvailableBytes != 3500000 {
					t.Errorf("AvailableBytes = %d, want %d", cluster.AvailableBytes, 3500000)
				}
			},
		},
		{
			name:         "pool parsing from DF",
			instanceName: "pool-test",
			status: &proxmox.CephStatus{
				FSID: "fsid-pools",
			},
			df: &proxmox.CephDF{
				Data: proxmox.CephDFData{
					Stats: proxmox.CephDFStats{
						TotalBytes: 10000000,
					},
					Pools: []proxmox.CephDFPool{
						{
							ID:   1,
							Name: "rbd-pool",
							Stats: proxmox.CephDFPoolStat{
								BytesUsed:   100000,
								MaxAvail:    900000,
								Objects:     50,
								PercentUsed: 10.0,
							},
						},
						{
							ID:   2,
							Name: "cephfs-data",
							Stats: proxmox.CephDFPoolStat{
								BytesUsed:   200000,
								MaxAvail:    800000,
								Objects:     100,
								PercentUsed: 20.0,
							},
						},
					},
				},
			},
			check: func(t *testing.T, cluster models.CephCluster) {
				if len(cluster.Pools) != 2 {
					t.Fatalf("len(Pools) = %d, want 2", len(cluster.Pools))
				}
				pool1 := cluster.Pools[0]
				if pool1.ID != 1 || pool1.Name != "rbd-pool" {
					t.Errorf("Pool[0] = {ID:%d, Name:%q}, want {ID:1, Name:rbd-pool}", pool1.ID, pool1.Name)
				}
				if pool1.StoredBytes != 100000 {
					t.Errorf("Pool[0].StoredBytes = %d, want %d", pool1.StoredBytes, 100000)
				}
				if pool1.AvailableBytes != 900000 {
					t.Errorf("Pool[0].AvailableBytes = %d, want %d", pool1.AvailableBytes, 900000)
				}
				if pool1.Objects != 50 {
					t.Errorf("Pool[0].Objects = %d, want %d", pool1.Objects, 50)
				}
				if pool1.PercentUsed != 10.0 {
					t.Errorf("Pool[0].PercentUsed = %f, want %f", pool1.PercentUsed, 10.0)
				}

				pool2 := cluster.Pools[1]
				if pool2.ID != 2 || pool2.Name != "cephfs-data" {
					t.Errorf("Pool[1] = {ID:%d, Name:%q}, want {ID:2, Name:cephfs-data}", pool2.ID, pool2.Name)
				}
			},
		},
		{
			name:         "service status parsing with running and stopped daemons",
			instanceName: "service-test",
			status: &proxmox.CephStatus{
				FSID: "fsid-services",
				ServiceMap: proxmox.CephServiceMap{
					Services: map[string]proxmox.CephServiceDefinition{
						"mon": {
							Daemons: map[string]proxmox.CephServiceDaemon{
								"a": {Host: "node1", Status: "running"},
								"b": {Host: "node2", Status: "running"},
								"c": {Host: "node3", Status: "stopped"},
							},
						},
						"mgr": {
							Daemons: map[string]proxmox.CephServiceDaemon{
								"node1": {Host: "node1", Status: "active"},
								"node2": {Host: "node2", Status: "standby"},
							},
						},
					},
				},
			},
			df: nil,
			check: func(t *testing.T, cluster models.CephCluster) {
				if cluster.NumMons != 3 {
					t.Errorf("NumMons = %d, want 3", cluster.NumMons)
				}
				if cluster.NumMgrs != 2 {
					t.Errorf("NumMgrs = %d, want 2", cluster.NumMgrs)
				}
				if len(cluster.Services) != 2 {
					t.Fatalf("len(Services) = %d, want 2", len(cluster.Services))
				}
				// Find mon service
				var monService *models.CephServiceStatus
				for i := range cluster.Services {
					if cluster.Services[i].Type == "mon" {
						monService = &cluster.Services[i]
						break
					}
				}
				if monService == nil {
					t.Fatal("mon service not found")
				}
				if monService.Running != 2 {
					t.Errorf("mon.Running = %d, want 2", monService.Running)
				}
				if monService.Total != 3 {
					t.Errorf("mon.Total = %d, want 3", monService.Total)
				}
				if monService.Message != "Offline: c@node3" {
					t.Errorf("mon.Message = %q, want %q", monService.Message, "Offline: c@node3")
				}
			},
		},
		{
			name:         "health message integration",
			instanceName: "health-test",
			status: &proxmox.CephStatus{
				FSID: "fsid-health",
				Health: proxmox.CephHealth{
					Status: "HEALTH_WARN",
					Summary: []proxmox.CephHealthSummary{
						{Message: "1 pool(s) have too few placement groups"},
					},
				},
			},
			df: nil,
			check: func(t *testing.T, cluster models.CephCluster) {
				if cluster.Health != "HEALTH_WARN" {
					t.Errorf("Health = %q, want %q", cluster.Health, "HEALTH_WARN")
				}
				if cluster.HealthMessage != "1 pool(s) have too few placement groups" {
					t.Errorf("HealthMessage = %q, want %q", cluster.HealthMessage, "1 pool(s) have too few placement groups")
				}
			},
		},
		{
			name:         "OSD map values",
			instanceName: "osd-test",
			status: &proxmox.CephStatus{
				FSID: "fsid-osd",
				OSDMap: proxmox.CephOSDMap{
					NumOSDs:   12,
					NumUpOSDs: 10,
					NumInOSDs: 11,
				},
				PGMap: proxmox.CephPGMap{
					NumPGs: 256,
				},
			},
			df: nil,
			check: func(t *testing.T, cluster models.CephCluster) {
				if cluster.NumOSDs != 12 {
					t.Errorf("NumOSDs = %d, want 12", cluster.NumOSDs)
				}
				if cluster.NumOSDsUp != 10 {
					t.Errorf("NumOSDsUp = %d, want 10", cluster.NumOSDsUp)
				}
				if cluster.NumOSDsIn != 11 {
					t.Errorf("NumOSDsIn = %d, want 11", cluster.NumOSDsIn)
				}
				if cluster.NumPGs != 256 {
					t.Errorf("NumPGs = %d, want 256", cluster.NumPGs)
				}
			},
		},
		{
			name:         "empty FSID fallback",
			instanceName: "no-fsid-instance",
			status: &proxmox.CephStatus{
				FSID: "",
			},
			df: nil,
			check: func(t *testing.T, cluster models.CephCluster) {
				if cluster.ID != "no-fsid-instance" {
					t.Errorf("ID = %q, want %q (should equal instanceName when FSID is empty)", cluster.ID, "no-fsid-instance")
				}
				if cluster.FSID != "" {
					t.Errorf("FSID = %q, want empty", cluster.FSID)
				}
			},
		},
		{
			name:         "usage percent calculation",
			instanceName: "usage-test",
			status: &proxmox.CephStatus{
				FSID: "fsid-usage",
				PGMap: proxmox.CephPGMap{
					BytesTotal: 1000,
					BytesUsed:  250,
					BytesAvail: 750,
				},
			},
			df: nil,
			check: func(t *testing.T, cluster models.CephCluster) {
				// 250/1000 * 100 = 25%
				if cluster.UsagePercent != 25.0 {
					t.Errorf("UsagePercent = %f, want 25.0", cluster.UsagePercent)
				}
			},
		},
		{
			name:         "DF with zero TotalBytes uses PGMap",
			instanceName: "zero-df-test",
			status: &proxmox.CephStatus{
				FSID: "fsid-zero",
				PGMap: proxmox.CephPGMap{
					BytesTotal: 3000000,
					BytesUsed:  1000000,
					BytesAvail: 2000000,
				},
			},
			df: &proxmox.CephDF{
				Data: proxmox.CephDFData{
					Stats: proxmox.CephDFStats{
						TotalBytes:      0,
						TotalUsedBytes:  0,
						TotalAvailBytes: 0,
					},
				},
			},
			check: func(t *testing.T, cluster models.CephCluster) {
				// When DF TotalBytes is 0, should use PGMap values
				if cluster.TotalBytes != 3000000 {
					t.Errorf("TotalBytes = %d, want %d (PGMap value when DF TotalBytes=0)", cluster.TotalBytes, 3000000)
				}
				if cluster.UsedBytes != 1000000 {
					t.Errorf("UsedBytes = %d, want %d", cluster.UsedBytes, 1000000)
				}
			},
		},
		{
			name:         "service with daemon without host",
			instanceName: "no-host-test",
			status: &proxmox.CephStatus{
				FSID: "fsid-nohost",
				ServiceMap: proxmox.CephServiceMap{
					Services: map[string]proxmox.CephServiceDefinition{
						"osd": {
							Daemons: map[string]proxmox.CephServiceDaemon{
								"0": {Host: "", Status: "stopped"},
							},
						},
					},
				},
			},
			df: nil,
			check: func(t *testing.T, cluster models.CephCluster) {
				if len(cluster.Services) != 1 {
					t.Fatalf("len(Services) = %d, want 1", len(cluster.Services))
				}
				// When host is empty, message should just use daemon name
				if cluster.Services[0].Message != "Offline: 0" {
					t.Errorf("Service.Message = %q, want %q", cluster.Services[0].Message, "Offline: 0")
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cluster := buildCephClusterModel(tc.instanceName, tc.status, tc.df)
			tc.check(t, cluster)
		})
	}
}
