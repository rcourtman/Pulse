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
	if got := storage.Disks[2]; got.Status != "disabled" {
		t.Fatalf("disabled disk status = %q, want disabled", got.Status)
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
