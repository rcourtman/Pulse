package hostagent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseUnraidStatusOutput(t *testing.T) {
	output := `
mdState=STARTED
mdResyncAction=check
mdResyncPos=500
mdResyncSize=1000
mdResyncCorr=2
mdNumProtected=2
mdNumDisabled=1
diskName.0=parity
diskSize.0=12000
rdevName.0=sdb
rdevStatus.0=DISK_OK
diskName.1=disk1
diskSize.1=12000
rdevName.1=sdc
rdevStatus.1=DISK_OK
rdevSerial.1=SER123
diskFsType.1=xfs
diskName.2=disk2
rdevStatus.2=DISK_DSBL
`

	storage, err := parseUnraidStatusOutput(output)
	if err != nil {
		t.Fatalf("parseUnraidStatusOutput() error = %v", err)
	}
	if !storage.ArrayStarted {
		t.Fatal("expected arrayStarted=true")
	}
	if storage.SyncAction != "check" {
		t.Fatalf("SyncAction = %q, want check", storage.SyncAction)
	}
	if storage.SyncProgress != 50 {
		t.Fatalf("SyncProgress = %v, want 50", storage.SyncProgress)
	}
	if storage.NumProtected != 2 || storage.NumDisabled != 1 {
		t.Fatalf("unexpected counts: %+v", storage)
	}
	if len(storage.Disks) != 3 {
		t.Fatalf("disk count = %d, want 3", len(storage.Disks))
	}
	if got := storage.Disks[0]; got.Role != "parity" || got.Device != "/dev/sdb" || got.Status != "online" {
		t.Fatalf("unexpected parity disk: %+v", got)
	}
	if got := storage.Disks[1]; got.Role != "data" || got.Serial != "SER123" || got.Filesystem != "xfs" {
		t.Fatalf("unexpected data disk: %+v", got)
	}
	if got := storage.Disks[1].SizeBytes; got != 12000*1024 {
		t.Fatalf("data disk size = %d, want %d", got, int64(12000*1024))
	}
	if got := storage.Disks[2]; got.Status != "disabled" {
		t.Fatalf("disabled disk status = %q, want disabled", got.Status)
	}
}

func TestParseUnraidStatusOutputStaleSyncAction(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		wantSyncAction string
		wantProgress   float64
	}{
		{
			name: "sync active reports action",
			output: `
mdState=STARTED
mdResyncAction=check
mdResyncPos=500
mdResyncSize=1000
`,
			wantSyncAction: "check",
			wantProgress:   50,
		},
		{
			name: "stale action cleared when position is zero",
			output: `
mdState=STARTED
mdResyncAction=check
mdResyncPos=0
mdResyncSize=0
mdResyncPct=100
`,
			wantSyncAction: "",
			wantProgress:   0,
		},
		{
			name: "stale action cleared when position fields absent",
			output: `
mdState=STARTED
mdResyncAction=check
`,
			wantSyncAction: "",
			wantProgress:   0,
		},
		{
			name: "idle action always empty",
			output: `
mdState=STARTED
mdResyncAction=idle
mdResyncPos=500
`,
			wantSyncAction: "",
			wantProgress:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := parseUnraidStatusOutput(tt.output)
			if err != nil {
				t.Fatalf("parseUnraidStatusOutput() error = %v", err)
			}
			if storage.SyncAction != tt.wantSyncAction {
				t.Errorf("SyncAction = %q, want %q", storage.SyncAction, tt.wantSyncAction)
			}
			if storage.SyncProgress != tt.wantProgress {
				t.Errorf("SyncProgress = %v, want %v", storage.SyncProgress, tt.wantProgress)
			}
		})
	}
}

func TestParseUnraidStatusOutputSkipsEmptyNoPresentSlots(t *testing.T) {
	output := `
mdState=STARTED
mdNumDisabled=2
mdNumInvalid=2
mdNumMissing=0
diskNumber.0=0
diskName.0=
diskSize.0=0
diskId.0=
rdevStatus.0=DISK_NP_DSBL
rdevName.0=
rdevId.0=
diskNumber.1=1
diskName.1=md1p1
diskSize.1=5860522532
diskId.1=WDC_DATA
rdevStatus.1=DISK_OK
rdevName.1=sde
rdevId.1=WDC_DATA
diskNumber.5=5
diskName.5=
diskSize.5=0
diskId.5=
rdevStatus.5=DISK_NP
rdevName.5=
rdevId.5=
diskNumber.29=29
diskName.29=
diskSize.29=0
diskId.29=
rdevStatus.29=DISK_NP_DSBL
rdevName.29=
rdevId.29=
`

	storage, err := parseUnraidStatusOutput(output)
	if err != nil {
		t.Fatalf("parseUnraidStatusOutput() error = %v", err)
	}
	if len(storage.Disks) != 1 {
		t.Fatalf("disk count = %d, want only assigned disks: %+v", len(storage.Disks), storage.Disks)
	}
	if got := storage.Disks[0]; got.Name != "md1p1" || got.Role != "data" || got.Status != "online" || got.Serial != "WDC_DATA" {
		t.Fatalf("unexpected assigned disk: %+v", got)
	}
}

func TestParseUnraidDisksINIAddsNativeTopologyFields(t *testing.T) {
	input := `
["disk1"]
idx="1"
name="disk1"
device="sde"
id="WDC_WD60EFRX-68L0BN1_WD-WX11D65CUC39"
size="5860522532"
sectors="11721045168"
sector_size="512"
transport="ata"
rotational="1"
spundown="0"
status="DISK_OK"
temp="25"
numReads="494251710"
numWrites="344072698"
numErrors="0"
type="Data"
fsType="luks:xfs"
fsSize="5858433572"
fsFree="1459693324"
fsUsed="4398740248"
["cachepool"]
idx="30"
name="cachepool"
device="sdf"
id="SSD_2000GB_AA202305222000G68551"
size="1953514552"
sectors="3907029168"
sector_size="512"
transport="ata"
rotational="0"
spundown="1"
status="DISK_OK"
temp="*"
numErrors="2"
type="Cache"
fsType="btrfs"
fsFree="1783019588"
fsUsed="166957852"
`

	disks := parseUnraidDisksINI(input)
	if len(disks) != 2 {
		t.Fatalf("disk count = %d, want 2: %+v", len(disks), disks)
	}
	if got := disks[0]; got.Name != "disk1" || got.Role != "data" || got.Device != "/dev/sde" || got.Transport != "sata" {
		t.Fatalf("unexpected disk1 identity: %+v", got)
	}
	if got := disks[0]; got.Model != "WDC WD60EFRX-68L0BN1" || got.Serial != "WD-WX11D65CUC39" {
		t.Fatalf("unexpected disk1 model/serial: %+v", got)
	}
	if got := disks[0].SizeBytes; got != 11721045168*512 {
		t.Fatalf("disk1 size = %d, want %d", got, int64(11721045168*512))
	}
	if got := disks[0].UsedBytes; got != 4398740248*1024 {
		t.Fatalf("disk1 used = %d, want %d", got, int64(4398740248*1024))
	}
	if got := disks[1]; got.Name != "cachepool" || got.Role != "cache" || !got.SpunDown || got.Temperature != 0 || got.ErrorCount != 2 {
		t.Fatalf("unexpected cachepool fields: %+v", got)
	}
}

func TestCollectUnraidStorageSkipsNonUnraid(t *testing.T) {
	t.Parallel()

	mc := &mockCollector{
		goos: "linux",
		statFn: func(name string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
	}

	storage, err := CollectUnraidStorage(context.Background(), mc)
	if err != nil {
		t.Fatalf("CollectUnraidStorage() error = %v", err)
	}
	if storage != nil {
		t.Fatalf("CollectUnraidStorage() = %#v, want nil", storage)
	}
}

func TestCollectUnraidStorageUsesResolvedMdcmd(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mdcmdPath := filepath.Join(tmpDir, "mdcmd")
	if err := os.WriteFile(mdcmdPath, []byte(""), 0600); err != nil {
		t.Fatalf("write mdcmd stub: %v", err)
	}

	mc := &mockCollector{
		goos: "linux",
		statFn: func(name string) (os.FileInfo, error) {
			switch name {
			case hostAgentUnraidVersionPath, mdcmdPath:
				return os.Stat(mdcmdPath)
			default:
				return nil, os.ErrNotExist
			}
		},
		lookPathFn: func(file string) (string, error) {
			if file != "mdcmd" {
				t.Fatalf("unexpected lookPath %q", file)
			}
			return mdcmdPath, nil
		},
		commandCombinedOutputFn: func(_ context.Context, name string, arg ...string) (string, error) {
			if name != mdcmdPath {
				t.Fatalf("command name = %q, want %q", name, mdcmdPath)
			}
			if len(arg) != 1 || arg[0] != "status" {
				t.Fatalf("command args = %v, want [status]", arg)
			}
			return "mdState=STARTED\ndiskName.0=parity\nrdevName.0=sdb\nrdevStatus.0=DISK_OK\n", nil
		},
	}

	storage, err := CollectUnraidStorage(context.Background(), mc)
	if err != nil {
		t.Fatalf("CollectUnraidStorage() error = %v", err)
	}
	if storage == nil || !storage.ArrayStarted || len(storage.Disks) != 1 {
		t.Fatalf("CollectUnraidStorage() = %#v, want populated storage", storage)
	}
}
