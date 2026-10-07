package monitoring

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestVMFilesystemKeepsSystemNamedSiblingVolumes(t *testing.T) {
	paths := []string{"/snapshots", "/runtime", "/var/lib/docker-data", "/snap/image", "/run/worker", "/var/lib/docker/overlay2"}
	var filesystems []proxmox.VMFileSystem
	for i, mount := range paths {
		filesystems = append(filesystems, proxmox.VMFileSystem{
			Mountpoint: mount, Type: "ext4", Disk: mount,
			TotalBytes: 1000, UsedBytes: uint64(950 - i*100),
		})
	}
	summary := (&Monitor{}).summarizeVMFSInfo("fixture", proxmox.ClusterResource{}, filesystems)
	if summary.invalidBytes || summary.totalBytes != 3000 || summary.usedBytes != 2550 || len(summary.individualDisks) != 3 {
		t.Fatalf("independent local volumes missing from QGA capacity: %+v", summary)
	}
	for i, disk := range summary.individualDisks {
		if disk.Mountpoint != paths[i] || disk.Device != paths[i] || disk.Total != 1000 || disk.Used != int64(950-i*100) {
			t.Fatalf("volume metadata/counters changed: %+v", disk)
		}
	}
}

func TestAgentReportsKeepSystemNamedSiblingVolumes(t *testing.T) {
	for _, kind := range []string{"host", "docker"} {
		t.Run(kind, func(t *testing.T) {
			m := newTestMonitor(t)
			m.resourceStore = unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
			cfg := m.alertManager.GetConfig()
			cfg.Enabled, cfg.ActivationState = true, alerts.ActivationActive
			cfg.AgentDefaults = alerts.ThresholdConfig{Disk: &alerts.HysteresisThreshold{Trigger: 90, Clear: 80}}
			cfg.MetricTimeThresholds = map[string]map[string]int{"all": {"disk": 0}}
			cfg.TimeThresholds = map[string]int{"all": 0}
			cfg.MinimumDelta, cfg.SuppressionWindow = 0, 0
			m.alertManager.UpdateConfig(cfg)
			var disks []agentshost.Disk
			if err := json.Unmarshal([]byte(`[
 {"device":"/dev/vda1","mountpoint":"/snapshots","type":"ext4","totalBytes":1000,"usedBytes":950,"freeBytes":50,"usage":95},
 {"device":"/dev/vdb1","mountpoint":"/runtime","type":"xfs","totalBytes":1000,"usedBytes":400,"freeBytes":600,"usage":40},
 {"device":"/dev/vdc1","mountpoint":"/var/lib/docker-data","type":"ext4","totalBytes":1000,"usedBytes":700,"freeBytes":300,"usage":70},
 {"device":"image","mountpoint":"/snap/image","type":"squashfs","totalBytes":1000,"usedBytes":1000,"usage":100},
 {"device":"runtime","mountpoint":"/run/worker","type":"ext4","totalBytes":1000,"usedBytes":1000,"usage":100},
 {"device":"remote","mountpoint":"/snapshots-remote","type":"nfs4","totalBytes":1000,"usedBytes":1000,"usage":100}
 ]`), &disks); err != nil {
				t.Fatal(err)
			}
			for phase := 0; phase < 2; phase++ {
				if phase == 1 {
					for i := range disks {
						disks[i].UsedBytes, disks[i].FreeBytes, disks[i].Usage = 100, 900, 10
					}
				}
				now := time.Now().UTC()
				var native []models.Disk
				var canonical []unifiedresources.DiskInfo
				var stored []models.Disk
				if kind == "host" {
					host, err := m.ApplyHostReport(agentshost.Report{
						Agent: agentshost.AgentInfo{ID: "boundary-host", IntervalSeconds: 30},
						Host:  agentshost.HostInfo{ID: "boundary-machine", Hostname: "boundary-host"},
						Disks: disks, Timestamp: now,
					}, nil)
					if err != nil {
						t.Fatal(err)
					}
					native = host.Disks
					stored = m.state.GetSnapshot().Hosts[0].Disks
					canonical = m.GetUnifiedReadState().Hosts()[0].Disks()
					// Actual report admission evaluates the restored near-full
					// volume, then clears its alert on the next healthy report.
					active := m.alertManager.GetActiveAlerts()
					if phase == 0 {
						if len(active) != 1 || active[0].Metadata["mountpoint"] != "/snapshots" || active[0].Value != 95 {
							t.Fatalf("near-full local volume did not reach normal alert evaluation: %+v", active)
						}
					} else if len(active) != 0 {
						t.Fatalf("healthy volume retained an active usage alert: %+v", active)
					}
				} else {
					host, err := m.ApplyDockerReport(agentsdocker.Report{
						Agent:     agentsdocker.AgentInfo{ID: "boundary-docker", IntervalSeconds: 30},
						Host:      agentsdocker.HostInfo{MachineID: "boundary-docker-machine", Hostname: "boundary-docker", Disks: disks},
						Timestamp: now,
					}, nil)
					if err != nil {
						t.Fatal(err)
					}
					native = host.Disks
					stored = m.state.GetSnapshot().DockerHosts[0].Disks
					canonical = m.GetUnifiedReadState().DockerHosts()[0].Disks()
				}
				if len(native) != 3 || len(stored) != 3 || len(canonical) != 3 {
					t.Fatalf("local volumes missing at report/state/read boundaries: %v / %v / %v", native, stored, canonical)
				}
				for i := 0; i < 3; i++ {
					want := disks[i]
					if native[i].Mountpoint != want.Mountpoint || native[i].Total != want.TotalBytes || native[i].Used != want.UsedBytes || native[i].Usage != want.Usage ||
						stored[i] != native[i] || canonical[i].Mountpoint != want.Mountpoint || canonical[i].Total != want.TotalBytes || canonical[i].Used != want.UsedBytes || canonical[i].Usage != want.Usage {
						t.Fatalf("phase %d lost current local volume identity/usage: %+v / %+v / %+v", phase, native[i], stored[i], canonical[i])
					}
				}
			}
		})
	}
}
