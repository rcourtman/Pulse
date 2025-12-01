package monitoring

import (
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

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
