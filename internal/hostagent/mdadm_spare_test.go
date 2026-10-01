//go:build !windows

package hostagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// These are synthetic devices, retaining #2369's count tuple but no host
// identity. Both command and kernel reads are replaced; no array is touched.
const healthyMDStatWithSpare = `md1 : active raid5 sda1[0] sdb1[1] sdc1[2] sdd1[3] sde1[4](S)
      1024000 blocks super 1.2 level 5, 512k chunk [4/4] [UUUU]
`

func mdadmSpareDetail(required string, total, active, working, failed, spare int, state string) string {
	var detail strings.Builder
	fmt.Fprintf(&detail, `Raid Level : raid5
Raid Devices : %s
Total Devices : %d
State : %s
Active Devices : %d
Working Devices : %d
Failed Devices : %d
Spare Devices : %d
Number   Major   Minor   RaidDevice State
`, required, total, state, active, working, failed, spare)
	for i := 0; i < active; i++ {
		fmt.Fprintf(&detail, "%d 8 %d %d active sync /dev/sd%c1\n", i, i*16, i, 'a'+i)
	}
	if spare > 0 {
		detail.WriteString("4 8 64 - spare /dev/sde1\n")
	}
	if failed > 0 {
		detail.WriteString("5 8 80 - faulty /dev/sdf1\n")
	}
	return detail.String()
}

func TestIssue2369MdadmRequiredMemberCount(t *testing.T) {
	withReadProcMDStat(t, func() ([]byte, error) { return nil, nil })
	array, err := parseMdadmDetail("/dev/md1", mdadmSpareDetail("4", 5, 4, 5, 0, 1, "clean"))
	if err != nil {
		t.Fatal(err)
	}
	if array.RequiredDevices != 4 || array.TotalDevices != 5 || array.ActiveDevices != 4 || array.WorkingDevices != 5 || array.FailedDevices != 0 || array.SpareDevices != 1 || len(array.Devices) != 5 {
		t.Fatalf("required members must remain separate from attached devices: %+v", array)
	}
	if array.Devices[4].State != "spare" {
		t.Fatalf("spare topology was lost: %+v", array.Devices)
	}
}

func TestRAIDCollectorReportHealthAndAlert(t *testing.T) {
	degradedMDStat := `md1 : active raid5 sda1[0] sdb1[1] sdc1[2] sde1[4](S)
      1024000 blocks super 1.2 level 5, 512k chunk [4/3] [UUU_]
`
	tests := []struct {
		name     string
		mdstat   string
		detail   string
		required int
		total    int
		active   int
		want     storagehealth.RiskLevel
	}{
		{"mdadm clean four plus spare", healthyMDStatWithSpare, mdadmSpareDetail("4", 5, 4, 5, 0, 1, "clean"), 4, 5, 4, storagehealth.RiskHealthy},
		{"mdadm missing member with spare", degradedMDStat, mdadmSpareDetail("4", 4, 3, 4, 0, 1, "clean"), 4, 4, 3, storagehealth.RiskCritical},
		{"mdadm failed member with spare", degradedMDStat, mdadmSpareDetail("4", 5, 3, 4, 1, 1, "clean"), 4, 5, 3, storagehealth.RiskCritical},
		{"known zero active detail cannot borrow kernel members", healthyMDStatWithSpare, mdadmSpareDetail("4", 1, 0, 1, 0, 1, "active"), 4, 1, 0, storagehealth.RiskCritical},
		{"known zero active bitmap cannot borrow device tokens", "md1 : active raid5 sda1[0] sdb1[1] sdc1[2] sdd1[3] sde1[4](S)\n      1024000 blocks super 1.2 [4/0] [____]\n", "", 4, 4, 0, storagehealth.RiskCritical},
		{"mdstat-only clean spare", healthyMDStatWithSpare, "", 4, 4, 4, storagehealth.RiskHealthy},
		{"mdstat-only missing member with spare", degradedMDStat, "", 4, 4, 3, storagehealth.RiskCritical},
		{"malformed detail retains kernel deficit", degradedMDStat, mdadmSpareDetail("invalid", 4, 3, 4, 0, 1, "clean"), 4, 4, 3, storagehealth.RiskCritical},
		{"negative detail retains kernel deficit", degradedMDStat, mdadmSpareDetail("-1", 4, 3, 4, 0, 1, "clean"), 4, 4, 3, storagehealth.RiskCritical},
		{"older detail gets kernel requirement", healthyMDStatWithSpare, strings.ReplaceAll(mdadmSpareDetail("4", 5, 4, 5, 0, 1, "clean"), "Raid Devices : 4\n", ""), 4, 5, 4, storagehealth.RiskHealthy},
		{"scrub with spare", healthyMDStatWithSpare + "      check = 25.0% (...) speed=50000K/sec\n", mdadmSpareDetail("4", 5, 4, 5, 0, 1, "clean"), 4, 5, 4, storagehealth.RiskHealthy},
		{"resync with spare", healthyMDStatWithSpare + "      resync = 25.0% (...) speed=50000K/sec\n", "", 4, 4, 4, storagehealth.RiskHealthy},
		{"recovery with spare remains warning", healthyMDStatWithSpare + "      recovery = 25.0% (...) speed=50000K/sec\n", mdadmSpareDetail("4", 5, 4, 5, 0, 1, "active"), 4, 5, 4, storagehealth.RiskWarning},
		{"degraded scrub remains critical", degradedMDStat + "      check = 25.0% (...) speed=50000K/sec\n", "", 4, 4, 3, storagehealth.RiskCritical},
		{"QNAP sparse role normalization", "md13 : active raid1 sdb4[25] sda4[24]\n      458880 blocks super 1.0 [24/2] [UU______________________]\n", "", 2, 2, 2, storagehealth.RiskHealthy},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withReadProcMDStat(t, func() ([]byte, error) { return []byte(tc.mdstat), nil })
			withResolveMdadmBinary(t, func() (string, error) {
				if tc.detail == "" {
					return "", fmt.Errorf("mdadm unavailable")
				}
				return "/usr/sbin/mdadm", nil
			})
			withRunCommandOutput(t, func(_ context.Context, _ string, args ...string) ([]byte, error) {
				if len(args) == 1 && args[0] == "--version" {
					return []byte("mdadm"), nil
				}
				if len(args) != 2 || args[0] != "--detail" || args[1] != "/dev/md1" {
					t.Fatalf("unexpected command arguments: %v", args)
				}
				return []byte(tc.detail), nil
			})
			arrays, err := CollectRAIDArrays(context.Background())
			if err != nil || len(arrays) != 1 {
				t.Fatalf("CollectRAIDArrays: arrays=%+v err=%v", arrays, err)
			}
			payload, err := json.Marshal(agentshost.Report{RAID: arrays})
			if err != nil {
				t.Fatal(err)
			}
			var report struct {
				RAID []models.HostRAIDArray `json:"raid"`
			}
			if err := json.Unmarshal(payload, &report); err != nil {
				t.Fatal(err)
			}
			array := report.RAID[0]
			if array.RequiredDevices != tc.required || array.TotalDevices != tc.total || array.ActiveDevices != tc.active {
				t.Fatalf("report counts=%+v, want required=%d total=%d active=%d", array, tc.required, tc.total, tc.active)
			}
			assessment := storagehealth.AssessHostRAIDArray(array)
			if assessment.Level != tc.want {
				t.Fatalf("collector/report/health assessment=%+v, want %s", assessment, tc.want)
			}
			manager := alerts.NewManagerWithDataDir(t.TempDir())
			t.Cleanup(manager.Stop)
			cfg := manager.GetConfig()
			cfg.Enabled = true
			cfg.ActivationState = alerts.ActivationActive
			cfg.Schedule.QuietHours.Enabled = false
			cfg.TimeThresholds = map[string]int{}
			manager.UpdateConfig(cfg)
			manager.CheckHost(models.Host{ID: "raid-fixture", Hostname: "linux-fixture", Status: "online", RAID: report.RAID})
			var raidAlerts []alerts.Alert
			for _, alert := range manager.GetActiveAlerts() {
				if alert.Type == "raid" {
					raidAlerts = append(raidAlerts, alert)
				}
			}
			if tc.want == storagehealth.RiskHealthy {
				if len(raidAlerts) != 0 {
					t.Fatalf("healthy collector report activated RAID alert: %+v", raidAlerts)
				}
			} else {
				if len(raidAlerts) != 1 || string(raidAlerts[0].Level) != string(tc.want) || raidAlerts[0].Metadata["raidRequiredDevices"] != tc.required {
					t.Fatalf("RAID alert=%+v, want exactly one %s alert with required count", raidAlerts, tc.want)
				}
			}
		})
	}
}
