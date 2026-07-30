package hostagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestCollectProxmoxLXCFilesystemsUsesBoundedRunningPctQueries(t *testing.T) {
	const pctPath = "/usr/sbin/pct"
	listOutput := fmt.Sprintf(
		"%-10s %-10s %-12s %-20s\n%-10d %-10s %-12s %-20s\n%-10d %-10s %-12s %-20s\n%-10d %-10s %-12s %-20s\n",
		"VMID", "Status", "Lock", "Name",
		100, "running", "", "web",
		101, "stopped", "", "archive",
		102, "running", "backup", "database",
	)
	now := time.Date(2026, 7, 30, 20, 0, 0, 0, time.UTC)
	var commands [][]string
	collector := &mockCollector{
		goos:  "linux",
		nowFn: func() time.Time { return now },
		lookPathFn: func(file string) (string, error) {
			if file == "pct" {
				return pctPath, nil
			}
			return "", os.ErrNotExist
		},
		commandCombinedOutputLimitedFn: func(
			_ context.Context,
			maxBytes int,
			name string,
			args ...string,
		) (string, error) {
			if name != pctPath {
				t.Fatalf("command path = %q, want %q", name, pctPath)
			}
			commands = append(commands, append([]string(nil), args...))
			switch strings.Join(args, " ") {
			case "list":
				if maxBytes != proxmoxLXCMaxListOutputBytes {
					t.Fatalf("pct list limit = %d", maxBytes)
				}
				return listOutput, nil
			case "df 100":
				if maxBytes != proxmoxLXCMaxDFOutputBytes {
					t.Fatalf("pct df limit = %d", maxBytes)
				}
				return `MP     Volume                         Size   Used  Avail Use% Path
rootfs local-lvm:vm-100-disk-0       8.0G   2.0G   6.0G 25.0 /
mp0    tank:subvol-100-disk-0        1.0T 512.0G 512.0G 50.0 /srv/data
`, nil
			case "df 102":
				return "", errors.New("container migrated")
			default:
				t.Fatalf("unexpected pct command: %v", args)
				return "", nil
			}
		},
	}
	agent := &Agent{logger: zerolog.Nop(), collector: collector}

	got := agent.collectProxmoxLXCFilesystems(context.Background())
	if got == nil || !got.CollectedAt.Equal(now) {
		t.Fatalf("inventory = %+v", got)
	}
	if len(commands) != 3 {
		t.Fatalf("commands = %v, want list plus two running df queries", commands)
	}
	for _, command := range commands {
		if len(command) == 2 && command[1] == "101" {
			t.Fatalf("stopped container was queried: %v", commands)
		}
	}
	if len(got.Containers) != 1 || got.Containers[0].VMID != 100 || got.Containers[0].Name != "web" {
		t.Fatalf("containers = %+v", got.Containers)
	}
	if len(got.Containers[0].Disks) != 2 {
		t.Fatalf("disks = %+v", got.Containers[0].Disks)
	}
	root := got.Containers[0].Disks[0]
	if root.Mountpoint != "/" || root.Device != "local-lvm:vm-100-disk-0" ||
		root.TotalBytes != 8<<30 || root.UsedBytes != 2<<30 || root.Usage != 25 {
		t.Fatalf("root disk = %+v", root)
	}
	data := got.Containers[0].Disks[1]
	if data.Mountpoint != "/srv/data" || data.Type != "mp0" ||
		data.TotalBytes != 1<<40 || data.FreeBytes != 512<<30 {
		t.Fatalf("data disk = %+v", data)
	}
}

func TestParseProxmoxLXCFilesystemsValidatesAndBoundsInput(t *testing.T) {
	list := fmt.Sprintf(
		"%-10s %-10s %-12s %-20s\n%-10d %-10s %-12s %-20s\n%-10d %-10s %-12s %-20s\n",
		"VMID", "Status", "Lock", "Name",
		99, "running", "", "too-small",
		100, "running", "", "valid",
	)
	containers, err := parseProxmoxLXCRunningContainers(list)
	if err != nil {
		t.Fatalf("parse list: %v", err)
	}
	if len(containers) != 1 || containers[0].VMID != 100 || containers[0].Name != "valid" {
		t.Fatalf("running containers = %+v", containers)
	}
	if _, err := parseProxmoxLXCRunningContainers(
		strings.Repeat("x", proxmoxLXCMaxListOutputBytes+1),
	); err == nil {
		t.Fatal("expected oversized pct list error")
	}

	disks, err := parseProxmoxLXCDF(`MP Volume Size Used Avail Use% Path
mp1 pool:valid 10.0G 4.0G 6.0G 40.0 /valid
mp2 pool:duplicate 10.0G 5.0G 5.0G 50.0 /valid
mp256 pool:bad 10.0G 1.0G 9.0G 10.0 /too-many
mp3 pool:relative 10.0G 1.0G 9.0G 10.0 relative
mp4 pool:traversal 10.0G 1.0G 9.0G 10.0 /srv/../etc
mp5 pool:hot 10.0G 1.0G 9.0G 101.0 /hot
`)
	if err != nil {
		t.Fatalf("parse df: %v", err)
	}
	if len(disks) != 1 || disks[0].Mountpoint != "/valid" {
		t.Fatalf("validated disks = %+v", disks)
	}
	if _, err := parseProxmoxLXCDF(
		strings.Repeat("x", proxmoxLXCMaxDFOutputBytes+1),
	); err == nil {
		t.Fatal("expected oversized pct df error")
	}
}
