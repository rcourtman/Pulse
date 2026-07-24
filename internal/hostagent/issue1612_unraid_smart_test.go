//go:build !windows

package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

type issue1612Fixture struct {
	SmartctlVersion    string `json:"smartctlVersion"`
	Model              string `json:"model"`
	SizeBytes          int64  `json:"sizeBytes"`
	MdcmdStatus        string `json:"mdcmdStatus"`
	DisksINI           string `json:"disksINI"`
	SmartctlScan       string `json:"smartctlScan"`
	UnsupportedATAJSON string `json:"unsupportedATAJSON"`
}

func loadIssue1612Fixture(t *testing.T) issue1612Fixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "issue1612_unraid_28tb.json"))
	if err != nil {
		t.Fatalf("read issue #1612 fixture: %v", err)
	}
	var fixture issue1612Fixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode issue #1612 fixture: %v", err)
	}
	if fixture.SmartctlVersion != "smartctl 7.5 2025-04-30 r6178" ||
		fixture.Model != "ST28000NT000-4AB103" ||
		fixture.SizeBytes != 28_001_039_286_272 {
		t.Fatalf("fixture lost reporter hardware identity: %+v", fixture)
	}
	return fixture
}

func issue1612NativeStorage(t *testing.T, fixture issue1612Fixture) *agentshost.UnraidStorage {
	t.Helper()
	storage, err := parseUnraidStatusOutput(fixture.MdcmdStatus)
	if err != nil {
		t.Fatalf("parse mdcmd fixture: %v", err)
	}
	return reconcileUnraidDiskCounts(mergeUnraidDiskINI(storage, parseUnraidDisksINI(fixture.DisksINI)))
}

func TestIssue1612NativeInventoryOverridesFalseAggregateMissingAndPreservesRealMissing(t *testing.T) {
	fixture := loadIssue1612Fixture(t)
	mdcmdPath := filepath.Join(t.TempDir(), "mdcmd")
	if err := os.WriteFile(mdcmdPath, nil, 0o600); err != nil {
		t.Fatalf("write mdcmd fixture: %v", err)
	}
	stat, err := os.Stat(mdcmdPath)
	if err != nil {
		t.Fatalf("stat mdcmd fixture: %v", err)
	}

	collector := &mockCollector{
		goos: "linux",
		statFn: func(name string) (os.FileInfo, error) {
			switch name {
			case hostAgentUnraidVersionPath, mdcmdPath:
				return stat, nil
			default:
				return nil, fs.ErrNotExist
			}
		},
		lookPathFn: func(string) (string, error) { return mdcmdPath, nil },
		readFileFn: func(name string) ([]byte, error) {
			if name == hostAgentUnraidDisksINIPath {
				return []byte(fixture.DisksINI), nil
			}
			return nil, fs.ErrNotExist
		},
		commandCombinedOutputFn: func(_ context.Context, name string, args ...string) (string, error) {
			if name != mdcmdPath || len(args) != 1 || args[0] != "status" {
				t.Fatalf("unexpected native command: %s %v", name, args)
			}
			return fixture.MdcmdStatus, nil
		},
	}

	storage, err := CollectUnraidStorage(context.Background(), collector)
	if err != nil {
		t.Fatalf("CollectUnraidStorage() error = %v", err)
	}
	if storage == nil || !storage.ArrayStarted || storage.SyncAction != "check" || storage.SyncProgress != 50 {
		t.Fatalf("native parity-check state was not preserved: %+v", storage)
	}
	if storage.NumMissing != 1 || storage.NumDisabled != 0 || storage.NumInvalid != 0 {
		t.Fatalf("native counts = missing:%d disabled:%d invalid:%d, want only one genuine missing member",
			storage.NumMissing, storage.NumDisabled, storage.NumInvalid)
	}
	if len(storage.Disks) != 7 {
		t.Fatalf("native membership count = %d, want 7: %+v", len(storage.Disks), storage.Disks)
	}
	for _, disk := range storage.Disks {
		if disk.Name == "disk1" && (disk.Model != fixture.Model || disk.SizeBytes != fixture.SizeBytes || disk.Status != "online") {
			t.Fatalf("28 TB native member lost identity: %+v", disk)
		}
	}
}

func TestIssue1612DisksINIRemainsAvailableWhenMdcmdTimesOut(t *testing.T) {
	fixture := loadIssue1612Fixture(t)
	mdcmdPath := filepath.Join(t.TempDir(), "mdcmd")
	if err := os.WriteFile(mdcmdPath, nil, 0o600); err != nil {
		t.Fatalf("write mdcmd fixture: %v", err)
	}
	stat, err := os.Stat(mdcmdPath)
	if err != nil {
		t.Fatalf("stat mdcmd fixture: %v", err)
	}

	collector := &mockCollector{
		goos: "linux",
		statFn: func(name string) (os.FileInfo, error) {
			switch name {
			case hostAgentUnraidVersionPath, mdcmdPath:
				return stat, nil
			default:
				return nil, fs.ErrNotExist
			}
		},
		lookPathFn: func(string) (string, error) { return mdcmdPath, nil },
		readFileFn: func(name string) ([]byte, error) {
			if name == hostAgentUnraidDisksINIPath {
				return []byte(fixture.DisksINI), nil
			}
			return nil, fs.ErrNotExist
		},
		commandCombinedOutputFn: func(context.Context, string, ...string) (string, error) {
			return "", context.DeadlineExceeded
		},
	}

	storage, err := CollectUnraidStorage(context.Background(), collector)
	if err != nil {
		t.Fatalf("native disks.ini fallback returned error: %v", err)
	}
	if storage == nil || len(storage.Disks) != 7 || storage.NumMissing != 1 {
		t.Fatalf("native inventory was lost after mdcmd timeout: %+v", storage)
	}
	if storage.ArrayStarted || storage.SyncAction != "" {
		t.Fatalf("mdcmd-only runtime state was fabricated: %+v", storage)
	}
}

func TestIssue1612SMARTCommandsAreTransportAwareAndSkipNativeStandby(t *testing.T) {
	fixture := loadIssue1612Fixture(t)
	entries := []string{"sda", "sdb", "sdc", "sdd", "sde", "sdf", "sdg", "sdh"}
	files := make(map[string]string)
	for _, block := range entries {
		files["/sys/block/"+block+"/size"] = "54689529856\n"
		files["/sys/block/"+block+"/queue/rotational"] = "1\n"
		files["/sys/block/"+block+"/device/model"] = fixture.Model + "\n"
		files["/sys/block/"+block+"/device/vendor"] = "ATA\n"
		files["/sys/block/"+block+"/device/serial"] = "SERIAL-" + block + "\n"
	}
	files["/sys/block/sdf/device/protocol"] = "SAS\n"
	delete(files, "/sys/block/sdf/device/vendor")
	stubLinuxSysfs(t, entries, files)

	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})
	execLookPath = func(string) (string, error) { return "smartctl", nil }

	var mu sync.Mutex
	var commands [][]string
	smartRunCommandOutput = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		mu.Lock()
		commands = append(commands, append([]string(nil), args...))
		mu.Unlock()
		if len(args) == 1 && args[0] == "--scan" {
			return []byte(fixture.SmartctlScan), nil
		}
		device := args[len(args)-1]
		switch device {
		case "/dev/sdb", "/dev/sdc":
			return []byte(fixture.UnsupportedATAJSON), nil
		case "/dev/sdg":
			return nil, fs.ErrPermission
		case "/dev/sdh":
			return nil, context.DeadlineExceeded
		default:
			deviceType := "sat"
			protocol := "ATA"
			if device == "/dev/sdf" || device == "/dev/bus/0" {
				deviceType = "scsi"
				protocol = "SCSI"
			}
			return []byte(fmt.Sprintf(
				`{"device":{"name":%q,"type":%q,"protocol":%q},"model_name":%q,"serial_number":%q,"user_capacity":{"bytes":%d},"smart_status":{"passed":true},"temperature":{"current":31}}`,
				device, deviceType, protocol, fixture.Model, "SERIAL-"+filepath.Base(device), fixture.SizeBytes,
			)), nil
		}
	}

	native := issue1612NativeStorage(t, fixture)
	results, err := CollectSMARTLocalWithUnraid(context.Background(), nil, native)
	if err != nil {
		t.Fatalf("CollectSMARTLocalWithUnraid() error = %v", err)
	}

	byDevice := make(map[string]DiskSMART, len(results))
	for _, disk := range results {
		byDevice[disk.Device] = disk
	}
	for _, device := range []string{"sda", "sdb", "sdc", "sdd", "sde", "sdf", "sdg", "sdh"} {
		if _, ok := byDevice[device]; !ok {
			t.Fatalf("present disk %s disappeared after unsupported SMART response: %+v", device, results)
		}
	}
	if disk := byDevice["sdd"]; !disk.Standby || disk.Model != fixture.Model || disk.SizeBytes != fixture.SizeBytes {
		t.Fatalf("native standby identity = %+v", disk)
	}
	for _, device := range []string{"sdb", "sdc", "sdg", "sdh"} {
		disk := byDevice[device]
		if disk.SizeBytes != fixture.SizeBytes || disk.Health != "UNKNOWN" {
			t.Fatalf("unsupported/failed disk %s was not retained as unknown identity: %+v", device, disk)
		}
	}
	if byDevice["sdb"].Type != "sata" || byDevice["sdc"].Type != "sata" || byDevice["sde"].Type != "usb" || byDevice["sdf"].Type != "sas" {
		t.Fatalf("native transport evidence was not preserved: sdb=%q sdc=%q sde=%q sdf=%q",
			byDevice["sdb"].Type, byDevice["sdc"].Type, byDevice["sde"].Type, byDevice["sdf"].Type)
	}
	if native.NumMissing != 1 || native.Disks[6].Status != "missing" {
		t.Fatalf("SMART collection mutated genuine native missing evidence: %+v", native)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, args := range commands {
		joined := strings.Join(args, " ")
		if joined == "--scan-open" {
			t.Fatalf("opening discovery command was issued: %v", commands)
		}
		if strings.HasSuffix(joined, " /dev/sdd") || joined == "/dev/sdd" {
			t.Fatalf("spun-down native member received a SMART command: %v", args)
		}
		if (strings.HasSuffix(joined, " /dev/sda") ||
			strings.HasSuffix(joined, " /dev/sdb") ||
			strings.HasSuffix(joined, " /dev/sdc")) &&
			strings.Contains(" "+joined+" ", " -d scsi ") {
			t.Fatalf("direct ATA member was forced through SCSI: %v", args)
		}
	}
	assertIssue1612Command(t, commands, "/dev/sdf", "scsi")
	assertIssue1612Command(t, commands, "/dev/bus/0", "megaraid,0")
}

func TestIssue1612USBPathKeepsExplicitBridgeMode(t *testing.T) {
	stubLinuxSysfs(t, []string{"sde"}, map[string]string{
		"/sys/block/sde/device/vendor": "ATA\n",
	})
	origEval := smartctlEvalSymlinks
	t.Cleanup(func() { smartctlEvalSymlinks = origEval })
	smartctlEvalSymlinks = func(name string) (string, error) {
		if name == "/sys/block/sde/device" {
			return "/sys/devices/pci0000:00/0000:00:14.0/usb1/1-2/1-2:1.0/host8/target8:0:0/8:0:0:0", nil
		}
		return "", fs.ErrNotExist
	}

	attempts := smartctlProbeAttempts(smartctlTarget{Path: "/dev/sde", DeviceType: "scsi"})
	if len(attempts) != 2 || attempts[0][0] != "-d" || attempts[0][1] != "scsi" || attempts[1][0] == "-d" {
		t.Fatalf("USB bridge attempts = %v, want typed scan mode then auto-detection", attempts)
	}
	for _, args := range attempts {
		if strings.Contains(" "+strings.Join(args, " ")+" ", " -d sat ") {
			t.Fatalf("USB path was reclassified as direct ATA: %v", attempts)
		}
	}
}

func assertIssue1612Command(t *testing.T, commands [][]string, device, deviceType string) {
	t.Helper()
	for _, args := range commands {
		if len(args) >= 3 && args[0] == "-d" && args[1] == deviceType && args[len(args)-1] == device {
			return
		}
	}
	t.Fatalf("missing smartctl -d %s command for %s: %v", deviceType, device, commands)
}

func TestIssue1612BuildReportCollectsNativeInventoryBeforeSMARTTimeout(t *testing.T) {
	native := &agentshost.UnraidStorage{
		ArrayStarted: true,
		Disks: []agentshost.UnraidDisk{
			{Name: "parity", Device: "/dev/sda", Role: "parity", Status: "online"},
			{Name: "disk1", Device: "/dev/sdb", Role: "data", Status: "online"},
		},
	}
	nativeCalled := make(chan struct{})
	raidCalled := false
	cephCalled := false
	collector := &mockCollector{
		goos: "linux",
		unraidStorageFn: func(context.Context) (*agentshost.UnraidStorage, error) {
			close(nativeCalled)
			return native, nil
		},
		metricsFn: func(context.Context, []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{}, nil
		},
		raidArraysFn: func(context.Context) ([]agentshost.RAIDArray, error) {
			raidCalled = true
			return nil, nil
		},
		cephStatusFn: func(context.Context) (*CephClusterStatus, error) {
			cephCalled = true
			return nil, nil
		},
		smartLocalFn: func(ctx context.Context, _ []string, got *agentshost.UnraidStorage) ([]DiskSMART, error) {
			select {
			case <-nativeCalled:
			default:
				t.Fatal("SMART collection ran before native Unraid inventory")
			}
			if got != native {
				t.Fatalf("SMART inventory pointer = %p, want %p", got, native)
			}
			if !raidCalled || !cephCalled {
				t.Fatalf("optional SMART ran before topology collectors: raid=%v ceph=%v", raidCalled, cephCalled)
			}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	agent, err := New(Config{
		AgentID:   "issue-1612",
		APIToken:  "redacted-test-token",
		LogLevel:  -1,
		Collector: collector,
		UpdateStatus: func() agentupdate.Status {
			return agentupdate.Status{State: agentupdate.UpdateStateIdle}
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	report, err := agent.buildReport(ctx)
	if err != nil {
		t.Fatalf("buildReport() error = %v", err)
	}
	if report.Unraid == nil || len(report.Unraid.Disks) != 2 || report.Unraid.NumMissing != 0 {
		t.Fatalf("native inventory was lost when SMART timed out: %+v", report.Unraid)
	}
}

func TestIssue1612AggregateFallbackStillPreservesErrorWhenNoNativeInventoryExists(t *testing.T) {
	collector := &mockCollector{
		goos: "linux",
		statFn: func(name string) (os.FileInfo, error) {
			if name == hostAgentUnraidVersionPath {
				return nil, errors.New("permission denied")
			}
			return nil, fs.ErrNotExist
		},
	}
	if storage, err := CollectUnraidStorage(context.Background(), collector); err == nil || storage != nil {
		t.Fatalf("unreadable Unraid identity = (%+v, %v), want explicit error", storage, err)
	}
}
