//go:build !windows

package hostagent

import (
	"context"
	"os/exec"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
)

func TestIssue1516PermissionFailureRemainsUnknownAndKeepsSysfsIdentity(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	stubLinuxSysfs(t,
		[]string{"sda"},
		map[string]string{
			"/sys/block/sda/size":         "937703088\n",
			"/sys/block/sda/device/model": "Permission-limited disk\n",
			"/sys/block/sda/device/wwid":  "naa.5000c500abcdef01\n",
		},
	)

	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})
	execLookPath = func(string) (string, error) { return "smartctl", nil }
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "--scan-open" {
			return nil, nil
		}
		return exec.CommandContext(ctx, "sh", "-c", "exit 2").Output()
	}

	results, err := CollectSMARTLocal(context.Background(), nil)
	if err != nil {
		t.Fatalf("CollectSMARTLocal() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("disk count = %d, want 1: %+v", len(results), results)
	}
	disk := results[0]
	if disk.Device != "sda" || disk.Model != "Permission-limited disk" ||
		disk.WWN != "naa.5000c500abcdef01" || disk.SizeBytes != 480103981056 {
		t.Fatalf("sysfs identity was not preserved: %+v", disk)
	}
	if disk.Standby || disk.Health != "UNKNOWN" || disk.Type != "" || disk.Temperature != 0 {
		t.Fatalf("permission failure fabricated disk state: %+v", disk)
	}
	if disk.Collection == nil ||
		disk.Collection.Serial.State != diskinventory.FieldMissing ||
		disk.Collection.Temperature.State != diskinventory.FieldUnavailable {
		t.Fatalf("collection status = %+v, want missing serial and unavailable temperature", disk.Collection)
	}
}

func TestIssue1516StandbyDiskKeepsStableIdentityWithoutSMARTClaims(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	stubLinuxSysfs(t,
		[]string{"sdb"},
		map[string]string{
			"/sys/block/sdb/size":             "35156656128\n",
			"/sys/block/sdb/device/model":     "ST18000NM000J\n",
			"/sys/block/sdb/device/serial":    "ZR5STANDBY\n",
			"/sys/block/sdb/device/wwid":      "naa.5000c500standby1\n",
			"/sys/block/sdb/device/protocol":  "SAS\n",
			"/sys/block/sdb/queue/rotational": "1\n",
		},
	)

	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})
	execLookPath = func(string) (string, error) { return "smartctl", nil }
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "--scan-open" {
			return []byte("/dev/sdb -d scsi # sleeping SAS disk\n"), nil
		}
		return exec.CommandContext(ctx, "sh", "-c", "exit 3").Output()
	}

	results, err := CollectSMARTLocal(context.Background(), nil)
	if err != nil {
		t.Fatalf("CollectSMARTLocal() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("disk count = %d, want 1: %+v", len(results), results)
	}
	disk := results[0]
	if !disk.Standby || disk.Device != "sdb" || disk.Model != "ST18000NM000J" ||
		disk.Serial != "ZR5STANDBY" || disk.WWN != "naa.5000c500standby1" ||
		disk.Type != "sas" || disk.SizeBytes != 18000207937536 {
		t.Fatalf("standby disk identity was lost: %+v", disk)
	}
	if disk.Health != "" || disk.Temperature != 0 || disk.Attributes != nil {
		t.Fatalf("standby disk fabricated SMART evidence: %+v", disk)
	}
	if disk.Collection == nil ||
		disk.Collection.Serial.State != diskinventory.FieldAvailable ||
		disk.Collection.Temperature.State != diskinventory.FieldUnavailable {
		t.Fatalf("standby collection status = %+v", disk.Collection)
	}
}

func TestIssue1516SATRetryMergesTemperatureWithoutDiscardingIdentity(t *testing.T) {
	stubLinuxSysfs(t,
		[]string{"sdc"},
		map[string]string{
			"/sys/block/sdc/size":             "937703088\n",
			"/sys/block/sdc/queue/rotational": "0\n",
		},
	)

	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})
	execLookPath = func(string) (string, error) { return "smartctl", nil }

	var attempts int
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		attempts++
		if len(args) >= 2 && args[0] == "-d" && args[1] == "sat" {
			return []byte(`{
				"device":{"name":"/dev/sdc","type":"sat","protocol":"ATA"},
				"temperature":{"current":54}
			}`), nil
		}
		return []byte(`{
			"device":{"name":"/dev/sdc","type":"scsi","protocol":"SCSI"},
			"model_name":"Crucial BX500 480GB",
			"serial_number":"2110E5A8FEE7",
			"smart_status":{"passed":true}
		}`), nil
	}

	result, err := collectSMARTTarget(context.Background(), smartctlTarget{Path: "/dev/sdc"})
	if err != nil {
		t.Fatalf("collectSMARTTarget() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempt count = %d, want auto then SAT", attempts)
	}
	if result.Model != "Crucial BX500 480GB" || result.Serial != "2110E5A8FEE7" ||
		result.Health != "PASSED" || result.Type != "sata" || result.Temperature != 54 {
		t.Fatalf("retry evidence was not merged: %+v", result)
	}
}

func TestIssue1516SMARTAttemptMergeKeepsFailureEvidence(t *testing.T) {
	firstPassed := mergeSMARTAttemptEvidence(
		&DiskSMART{Health: "PASSED"},
		&DiskSMART{Health: "FAILED"},
	)
	if firstPassed.Health != "FAILED" {
		t.Fatalf("later SMART failure was discarded: %+v", firstPassed)
	}
	firstFailed := mergeSMARTAttemptEvidence(
		&DiskSMART{Health: "FAILED"},
		&DiskSMART{Health: "PASSED"},
	)
	if firstFailed.Health != "FAILED" {
		t.Fatalf("later PASSED result erased SMART failure: %+v", firstFailed)
	}
}

func TestIssue1516TemperaturePrecedenceAndPartialNVMeRemainNeutral(t *testing.T) {
	result, err := parseSMARTOutput([]byte(`{
		"device":{"name":"/dev/sda","type":"sat","protocol":"ATA"},
		"model_name":"Crucial BX500 480GB",
		"serial_number":"2110E5A8FEE7",
		"smart_status":{"passed":true},
		"temperature":{"current":400},
		"nvme_smart_health_information_log":{"temperature":350},
		"ata_sct_status":{"current":{"value":250}},
		"ata_smart_attributes":{"table":[
			{"id":190,"raw":{"value":39,"string":"39"}},
			{"id":194,"raw":{"value":0,"string":"54 (Min/Max 19/57)"}}
		]}
	}`), smartctlTarget{Path: "/dev/sda", DeviceType: "sat"})
	if err != nil {
		t.Fatalf("parse SATA SMART output: %v", err)
	}
	if result.Temperature != 54 {
		t.Fatalf("temperature = %d, want valid ATA 194 value 54", result.Temperature)
	}

	nvme, err := parseSMARTOutput([]byte(`{
		"device":{"name":"/dev/nvme0","type":"nvme","protocol":"NVMe"},
		"model_name":"ShiJi 128GB",
		"serial_number":"NVME-BOOT-1",
		"smart_status":{"passed":true},
		"nvme_smart_health_information_log":{"temperature":37}
	}`), smartctlTarget{Path: "/dev/nvme0", DeviceType: "nvme"})
	if err != nil {
		t.Fatalf("parse partial NVMe SMART output: %v", err)
	}
	if nvme.Temperature != 37 || nvme.Attributes != nil {
		t.Fatalf("partial NVMe log fabricated counters: %+v", nvme)
	}
}

func TestIssue1516FirstNVMeNamespaceUsesNumericOrder(t *testing.T) {
	stubLinuxSysfs(t, []string{"nvme0n10", "nvme0n2", "nvme0n1p1"}, nil)
	if got := firstNVMeNamespace("nvme0"); got != "nvme0n2" {
		t.Fatalf("firstNVMeNamespace() = %q, want nvme0n2", got)
	}
}
