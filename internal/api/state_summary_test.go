package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestStateSummaryPayloadShapeAndSize(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-1", Status: "online"},
		},
		VMs: []models.VM{
			{ID: "vm-1", Name: strings.Repeat("large-vm-name-", 20), Status: "running"},
			{ID: "vm-2", Name: "stopped-vm", Status: "stopped"},
		},
		Containers: []models.Container{
			{ID: "ct-1", Name: "lxc-1", Status: "running"},
		},
		Storage: []models.Storage{
			{ID: "storage-1", Name: "local-zfs", Status: "available", Enabled: true, Active: true, ZFSPool: &models.ZFSPool{State: "ONLINE"}},
			{ID: "storage-2", Name: "truenas-backup", Status: "unknown", Enabled: true, Active: false, ZFSPool: &models.ZFSPool{State: "DEGRADED"}},
		},
		CephClusters: []models.CephCluster{
			{ID: "ceph-1", Health: "HEALTH_WARN"},
		},
		PhysicalDisks: []models.PhysicalDisk{
			{ID: "disk-1", Health: "PASSED"},
			{ID: "disk-2", Health: "FAILED"},
		},
		DockerHosts: []models.DockerHost{
			{
				ID:       "docker-1",
				Hostname: "docker-host",
				Status:   "online",
				Containers: []models.DockerContainer{
					{ID: "container-1", Name: "homepage", State: "running", Labels: map[string]string{"very-large-label": strings.Repeat("x", 512)}},
					{ID: "container-2", Name: "worker", Health: "unhealthy"},
				},
				Services: []models.DockerService{{ID: "svc-1", DesiredTasks: 2, RunningTasks: 1}},
				Tasks:    []models.DockerTask{{ID: "task-1"}},
			},
		},
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:     "k8s-1",
				Name:   "lab",
				Status: "online",
				Nodes: []models.KubernetesNode{
					{Name: "worker-1", Ready: true},
					{Name: "worker-2", Ready: false},
				},
				Pods: []models.KubernetesPod{
					{Name: "api", Namespace: "default", Phase: "Running"},
					{Name: "job", Namespace: "default", Phase: "Failed"},
				},
				Deployments: []models.KubernetesDeployment{
					{Name: "api", Namespace: "default", DesiredReplicas: 3, AvailableReplicas: 3},
				},
			},
		},
		Hosts: []models.Host{
			{
				ID:       "host-1",
				Hostname: "nas-1",
				Status:   "online",
				Sensors: models.HostSensorSummary{
					TemperatureCelsius: map[string]float64{"cpu": 42.5},
					FanRPM:             map[string]float64{"fan1": 1200},
					Additional:         map[string]float64{"custom.sensor": 7},
					SMART:              []models.HostDiskSMART{{Device: "sda", Health: "PASSED"}},
				},
				RAID: []models.HostRAIDArray{{Device: "md0", State: "degraded"}},
			},
		},
		PBSInstances: []models.PBSInstance{
			{ID: "pbs-1", Status: "online", ConnectionHealth: "healthy", Datastores: []models.PBSDatastore{{Name: "backups", Status: "available"}}},
		},
		PMGInstances: []models.PMGInstance{
			{ID: "pmg-1", Status: "online", ConnectionHealth: "healthy"},
		},
		PBSBackups:       []models.PBSBackup{{ID: "pbs-backup-1"}},
		PMGBackups:       []models.PMGBackup{{ID: "pmg-backup-1"}},
		ReplicationJobs:  []models.ReplicationJob{{ID: "replication-1"}},
		PVEBackups:       models.PVEBackups{BackupTasks: []models.BackupTask{{ID: "task-1"}}, StorageBackups: []models.StorageBackup{{ID: "backup-1"}}, GuestSnapshots: []models.GuestSnapshot{{ID: "snap-1"}}},
		ActiveAlerts:     []models.Alert{{ID: "alert-1", Level: "critical"}, {ID: "alert-2", Level: "warning", Acknowledged: true}, {ID: "alert-3", Level: "info"}},
		RecentlyResolved: []models.ResolvedAlert{{Alert: models.Alert{ID: "resolved-1"}}},
		ConnectionHealth: map[string]bool{
			"pve-1": true,
			"pbs-1": false,
		},
		LastUpdate: now,
	}

	summary := buildStateSummary(state)
	body, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	if len(body) > 1600 {
		t.Fatalf("summary payload = %d bytes, want <= 1600: %s", len(body), string(body))
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		t.Fatalf("unmarshal top-level summary: %v", err)
	}
	expectedTopLevel := []string{"version", "lastUpdate", "coverage", "health", "alerts", "connectionHealth"}
	if len(top) != len(expectedTopLevel) {
		t.Fatalf("top-level key count = %d, want %d: %v", len(top), len(expectedTopLevel), top)
	}
	for _, key := range expectedTopLevel {
		if _, ok := top[key]; !ok {
			t.Fatalf("missing top-level key %q in %s", key, string(body))
		}
	}
	for _, fullStateKey := range []string{"nodes", "vms", "containers", "dockerHosts", "kubernetesClusters", "hosts", "activeAlerts"} {
		if _, ok := top[fullStateKey]; ok {
			t.Fatalf("summary leaked full state key %q: %s", fullStateKey, string(body))
		}
	}

	if summary.Version != stateSummaryVersion {
		t.Fatalf("version = %d, want %d", summary.Version, stateSummaryVersion)
	}
	if summary.LastUpdate != now.Unix() {
		t.Fatalf("lastUpdate = %d, want %d", summary.LastUpdate, now.Unix())
	}
	if summary.Coverage.Proxmox.Nodes != 1 || summary.Coverage.Proxmox.VMs != 2 || summary.Coverage.Proxmox.PhysicalDisks != 2 {
		t.Fatalf("unexpected proxmox coverage: %+v", summary.Coverage.Proxmox)
	}
	if summary.Coverage.Docker.Hosts != 1 || summary.Coverage.Docker.Containers != 2 || summary.Coverage.Docker.Services != 1 || summary.Coverage.Docker.Tasks != 1 {
		t.Fatalf("unexpected docker coverage: %+v", summary.Coverage.Docker)
	}
	if summary.Coverage.Kubernetes.Clusters != 1 || summary.Coverage.Kubernetes.Nodes != 2 || summary.Coverage.Kubernetes.Pods != 2 || summary.Coverage.Kubernetes.Deployments != 1 {
		t.Fatalf("unexpected kubernetes coverage: %+v", summary.Coverage.Kubernetes)
	}
	if summary.Coverage.HostAgents.Hosts != 1 || summary.Coverage.HostAgents.Sensors != 3 || summary.Coverage.HostAgents.RAIDArrays != 1 || summary.Coverage.HostAgents.SMARTDisks != 1 {
		t.Fatalf("unexpected host-agent coverage: %+v", summary.Coverage.HostAgents)
	}
	if summary.Coverage.Backup.PBSInstances != 1 || summary.Coverage.Backup.PBSBackups != 1 || summary.Coverage.Backup.PMGInstances != 1 || summary.Coverage.Backup.PMGBackups != 1 || summary.Coverage.Backup.PVEBackupTasks != 1 || summary.Coverage.Backup.PVEStorageBackups != 1 || summary.Coverage.Backup.PVEGuestSnapshots != 1 || summary.Coverage.Backup.ReplicationJobs != 1 {
		t.Fatalf("unexpected backup coverage: %+v", summary.Coverage.Backup)
	}
	if summary.Alerts.Active != 3 || summary.Alerts.Critical != 1 || summary.Alerts.Warning != 1 || summary.Alerts.Info != 1 || summary.Alerts.Acknowledged != 1 || summary.Alerts.RecentlyResolved != 1 {
		t.Fatalf("unexpected alert summary: %+v", summary.Alerts)
	}
	if summary.ConnectionHealth.Total != 2 || summary.ConnectionHealth.Healthy != 1 || summary.ConnectionHealth.Unhealthy != 1 {
		t.Fatalf("unexpected connection summary: %+v", summary.ConnectionHealth)
	}
	if summary.Health.Total == 0 || summary.Health.Up == 0 || summary.Health.Degraded == 0 || summary.Health.Down == 0 {
		t.Fatalf("health summary did not classify expected states: %+v", summary.Health)
	}
}

func TestStateSummaryRouteRequiresAuthInAPIMode(t *testing.T) {
	rawToken := "summary-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	monitor, state, _ := newTestMonitor(t)
	state.LastUpdate = time.Unix(1710000000, 0).UTC()
	state.Nodes = []models.Node{{ID: "node-1", Name: "node-1", Status: "online"}}
	router := NewRouter(cfg, monitor, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/state/summary", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d (%s)", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/state/summary", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d (%s)", rec.Code, rec.Body.String())
	}

	var summary StateSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("unmarshal state summary response: %v", err)
	}
	if summary.Coverage.Proxmox.Nodes != 1 || summary.Health.Up != 1 {
		t.Fatalf("unexpected summary response: %+v", summary)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/state/summary", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with bearer token, got %d (%s)", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/state/summary?token="+rawToken, nil)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for query token on normal HTTP request, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestStateSummaryRouteRequiresMonitoringReadScope(t *testing.T) {
	rawToken := "summary-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	monitor, _, _ := newTestMonitor(t)
	router := NewRouter(cfg, monitor, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/state/summary", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing monitoring:read scope, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), config.ScopeMonitoringRead) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeMonitoringRead, rec.Body.String())
	}
}
