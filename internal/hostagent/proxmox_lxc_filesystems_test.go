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

	result := agent.collectProxmoxLXCFilesystemsResult(context.Background())
	got := result.Inventory
	if got == nil || !got.CollectedAt.Equal(now) {
		t.Fatalf("inventory = %+v", got)
	}
	if !result.Applicable || !result.Degraded || result.FailedContainers != 1 {
		t.Fatalf("collection result = %+v, want applicable partial failure", result)
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

func TestCollectProxmoxLXCFilesystemsReportsTotalContainerFailure(t *testing.T) {
	const pctPath = "/usr/sbin/pct"
	listOutput := "VMID Status Lock Name\n100 running - web\n102 running - database\n"
	collector := &mockCollector{
		goos: "linux",
		lookPathFn: func(file string) (string, error) {
			if file == "pct" {
				return pctPath, nil
			}
			return "", os.ErrNotExist
		},
		commandCombinedOutputLimitedFn: func(_ context.Context, _ int, _ string, args ...string) (string, error) {
			if strings.Join(args, " ") == "list" {
				return listOutput, nil
			}
			return "", errors.New("pct df unavailable")
		},
	}

	result := (&Agent{logger: zerolog.Nop(), collector: collector}).collectProxmoxLXCFilesystemsResult(t.Context())
	if !result.Applicable || !result.Degraded || result.FailedContainers != 2 {
		t.Fatalf("collection result = %+v, want two failed containers", result)
	}
	if result.Inventory == nil || len(result.Inventory.Containers) != 0 {
		t.Fatalf("partial inventory = %+v, want empty retained inventory", result.Inventory)
	}
}

// Regression for the #1477 follow-up: pct df costs over a second per guest,
// so serial pct df inside one shared budget starved every container after the
// first handful. With a resolvable init PID the collector must answer from
// statfs through /proc/<pid>/root and never spawn pct df.
func TestCollectProxmoxLXCFilesystemsPrefersProcStatfsOverPctDF(t *testing.T) {
	const pctPath = "/usr/sbin/pct"
	const lxcInfoPath = "/usr/bin/lxc-info"
	listOutput := fmt.Sprintf(
		"%-10s %-10s %-12s %-20s\n%-10d %-10s %-12s %-20s\n%-10d %-10s %-12s %-20s\n",
		"VMID", "Status", "Lock", "Name",
		100, "running", "", "web",
		101, "running", "", "db",
	)
	configs := map[string]string{
		"/etc/pve/lxc/100.conf": `arch: amd64
hostname: web
rootfs: local-lvm:vm-100-disk-0,size=8G
mp0: tank:subvol-100-disk-0,mp=/srv/data,size=1T
mp1: tank:subvol-100-disk-1,mp=/mnt/missing,size=10G

[snap1]
mp2: tank:snap-only,mp=/snap-only,size=5G
`,
		"/etc/pve/lxc/101.conf": "rootfs: local-lvm:vm-101-disk-0,size=4G\n",
	}
	usageByPath := map[string]hostFilesystemUsage{
		"/proc/4242/root":          {TotalBytes: 8 << 30, UsedBytes: 2 << 30, AvailBytes: 6 << 30, Device: 11},
		"/proc/4242/root/srv":      {TotalBytes: 8 << 30, UsedBytes: 2 << 30, AvailBytes: 6 << 30, Device: 11},
		"/proc/4242/root/srv/data": {TotalBytes: 1 << 40, UsedBytes: 512 << 30, AvailBytes: 512 << 30, Device: 12},
		// mp1 is configured but not mounted in the running container, so the
		// path resolves to the rootfs device and must be dropped, not reported
		// with the parent's numbers.
		"/proc/4242/root/mnt":         {TotalBytes: 8 << 30, UsedBytes: 2 << 30, AvailBytes: 6 << 30, Device: 11},
		"/proc/4242/root/mnt/missing": {TotalBytes: 8 << 30, UsedBytes: 2 << 30, AvailBytes: 6 << 30, Device: 11},
		"/proc/5252/root":             {TotalBytes: 4 << 30, UsedBytes: 1 << 30, AvailBytes: 3 << 30, Device: 21},
	}
	collector := &mockCollector{
		goos: "linux",
		lookPathFn: func(file string) (string, error) {
			switch file {
			case "pct":
				return pctPath, nil
			case "lxc-info":
				return lxcInfoPath, nil
			}
			return "", os.ErrNotExist
		},
		readFileFn: func(name string) ([]byte, error) {
			config, ok := configs[name]
			if !ok {
				return nil, os.ErrNotExist
			}
			return []byte(config), nil
		},
		filesystemUsageFn: func(path string) (hostFilesystemUsage, error) {
			usage, ok := usageByPath[path]
			if !ok {
				t.Fatalf("unexpected statfs path %q", path)
			}
			return usage, nil
		},
		commandCombinedOutputLimitedFn: func(
			_ context.Context,
			_ int,
			name string,
			args ...string,
		) (string, error) {
			joined := strings.Join(args, " ")
			switch {
			case name == pctPath && joined == "list":
				return listOutput, nil
			case name == pctPath:
				t.Fatalf("pct df must not run when the statfs path resolves: %v", args)
				return "", nil
			case name == lxcInfoPath && joined == "-n 100 -p":
				return "PID:            4242\n", nil
			case name == lxcInfoPath && joined == "-n 101 -p":
				return "PID:            5252\n", nil
			default:
				t.Fatalf("unexpected command: %s %v", name, args)
				return "", nil
			}
		},
	}
	agent := &Agent{logger: zerolog.Nop(), collector: collector}

	got := agent.collectProxmoxLXCFilesystems(context.Background())
	if got == nil || len(got.Containers) != 2 {
		t.Fatalf("inventory = %+v", got)
	}
	web := got.Containers[0]
	if web.VMID != 100 || len(web.Disks) != 2 {
		t.Fatalf("web container = %+v", web)
	}
	root := web.Disks[0]
	if root.Mountpoint != "/" || root.Type != "rootfs" || root.Device != "local-lvm:vm-100-disk-0" ||
		root.TotalBytes != 8<<30 || root.UsedBytes != 2<<30 || root.FreeBytes != 6<<30 || root.Usage != 25 {
		t.Fatalf("root disk = %+v", root)
	}
	data := web.Disks[1]
	if data.Mountpoint != "/srv/data" || data.Type != "mp0" || data.Device != "tank:subvol-100-disk-0" ||
		data.TotalBytes != 1<<40 || data.Usage != 50 {
		t.Fatalf("data disk = %+v", data)
	}
	db := got.Containers[1]
	if db.VMID != 101 || len(db.Disks) != 1 || db.Disks[0].Mountpoint != "/" {
		t.Fatalf("db container = %+v", db)
	}
}

func TestCollectProxmoxLXCFilesystemsFallsBackToPctDFPerContainer(t *testing.T) {
	const pctPath = "/usr/sbin/pct"
	const lxcInfoPath = "/usr/bin/lxc-info"
	listOutput := fmt.Sprintf(
		"%-10s %-10s %-12s %-20s\n%-10d %-10s %-12s %-20s\n%-10d %-10s %-12s %-20s\n",
		"VMID", "Status", "Lock", "Name",
		100, "running", "", "web",
		102, "running", "", "legacy",
	)
	var dfQueries []string
	collector := &mockCollector{
		goos: "linux",
		lookPathFn: func(file string) (string, error) {
			switch file {
			case "pct":
				return pctPath, nil
			case "lxc-info":
				return lxcInfoPath, nil
			}
			return "", os.ErrNotExist
		},
		readFileFn: func(name string) ([]byte, error) {
			if name == "/etc/pve/lxc/100.conf" {
				return []byte("rootfs: local-lvm:vm-100-disk-0,size=8G\n"), nil
			}
			// 102's config is unreadable, so only that container may fall
			// back to pct df.
			return nil, os.ErrPermission
		},
		filesystemUsageFn: func(path string) (hostFilesystemUsage, error) {
			if path != "/proc/4242/root" {
				t.Fatalf("unexpected statfs path %q", path)
			}
			return hostFilesystemUsage{TotalBytes: 8 << 30, UsedBytes: 2 << 30, AvailBytes: 6 << 30, Device: 11}, nil
		},
		commandCombinedOutputLimitedFn: func(
			_ context.Context,
			_ int,
			name string,
			args ...string,
		) (string, error) {
			joined := strings.Join(args, " ")
			switch {
			case name == pctPath && joined == "list":
				return listOutput, nil
			case name == lxcInfoPath && joined == "-n 100 -p":
				return "PID: 4242\n", nil
			case name == pctPath && joined == "df 102":
				dfQueries = append(dfQueries, joined)
				return `MP     Volume                         Size   Used  Avail Use% Path
rootfs local-lvm:vm-102-disk-0       8.0G   2.0G   6.0G 25.0 /
`, nil
			default:
				t.Fatalf("unexpected command: %s %v", name, args)
				return "", nil
			}
		},
	}
	agent := &Agent{logger: zerolog.Nop(), collector: collector}

	got := agent.collectProxmoxLXCFilesystems(context.Background())
	if got == nil || len(got.Containers) != 2 {
		t.Fatalf("inventory = %+v", got)
	}
	if len(dfQueries) != 1 || dfQueries[0] != "df 102" {
		t.Fatalf("df queries = %v, want exactly one for the fallback container", dfQueries)
	}
	if got.Containers[0].VMID != 100 || got.Containers[1].VMID != 102 {
		t.Fatalf("containers = %+v", got.Containers)
	}
}

func TestCollectProxmoxLXCFilesystemsFallsBackAfterPartialStatfsFailure(t *testing.T) {
	const pctPath = "/usr/sbin/pct"
	const lxcInfoPath = "/usr/bin/lxc-info"
	const listOutput = "VMID Status Lock Name\n100 running - web\n"
	const dfOutput = `MP     Volume                         Size   Used  Avail Use% Path
rootfs local-lvm:vm-100-disk-0       8.0G   2.0G   6.0G 25.0 /
mp0    tank:subvol-100-disk-0        1.0T 512.0G 512.0G 50.0 /srv/data
`

	for _, test := range []struct {
		name             string
		fallbackError    error
		wantDegraded     bool
		wantContainerLen int
	}{
		{name: "complete fallback recovers", wantContainerLen: 1},
		{name: "failed fallback degrades container", fallbackError: errors.New("pct df unavailable"), wantDegraded: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dfCalls := 0
			collector := &mockCollector{
				goos: "linux",
				lookPathFn: func(file string) (string, error) {
					switch file {
					case "pct":
						return pctPath, nil
					case "lxc-info":
						return lxcInfoPath, nil
					default:
						return "", os.ErrNotExist
					}
				},
				readFileFn: func(string) ([]byte, error) {
					return []byte("rootfs: local-lvm:vm-100-disk-0,size=8G\nmp0: tank:subvol-100-disk-0,mp=/srv/data,size=1T\n"), nil
				},
				filesystemUsageFn: func(target string) (hostFilesystemUsage, error) {
					switch target {
					case "/proc/4242/root":
						return hostFilesystemUsage{TotalBytes: 8 << 30, UsedBytes: 2 << 30, AvailBytes: 6 << 30, Device: 11}, nil
					case "/proc/4242/root/srv/data":
						return hostFilesystemUsage{}, errors.New("statfs access failed")
					default:
						t.Fatalf("unexpected statfs path %q", target)
						return hostFilesystemUsage{}, nil
					}
				},
				commandCombinedOutputLimitedFn: func(_ context.Context, _ int, name string, args ...string) (string, error) {
					joined := strings.Join(args, " ")
					switch {
					case name == pctPath && joined == "list":
						return listOutput, nil
					case name == lxcInfoPath && joined == "-n 100 -p":
						return "PID: 4242\n", nil
					case name == pctPath && joined == "df 100":
						dfCalls++
						return dfOutput, test.fallbackError
					default:
						t.Fatalf("unexpected command: %s %v", name, args)
						return "", nil
					}
				},
			}

			result := (&Agent{logger: zerolog.Nop(), collector: collector}).collectProxmoxLXCFilesystemsResult(t.Context())
			if dfCalls != 1 {
				t.Fatalf("pct df calls = %d, want 1 after partial statfs failure", dfCalls)
			}
			if result.Degraded != test.wantDegraded || result.Inventory == nil || len(result.Inventory.Containers) != test.wantContainerLen {
				t.Fatalf("collection result = %+v", result)
			}
			if test.wantDegraded && result.FailedContainers != 1 {
				t.Fatalf("failed containers = %d, want 1", result.FailedContainers)
			}
		})
	}
}

func TestParseProxmoxLXCConfigMountsStopsAtSectionsAndValidates(t *testing.T) {
	mounts := parseProxmoxLXCConfigMounts(`# comment
arch: amd64
rootfs: local-lvm:vm-100-disk-0,size=8G
mp0: tank:subvol-100-disk-0,mp=/srv/data,size=1T
mp1: /host/bind,mp=/shared
mp2: tank:no-mountpoint,size=5G
mp3: tank:bad-path,mp=relative,size=5G
mp4: tank:dupe,mp=/srv/data,size=5G
unused0: tank:vm-100-disk-9
[snap1]
mp5: tank:snap-only,mp=/snap-only,size=5G
`)
	if len(mounts) != 3 {
		t.Fatalf("mounts = %+v", mounts)
	}
	if mounts[0].Key != "rootfs" || mounts[0].Path != "/" || mounts[0].Volume != "local-lvm:vm-100-disk-0" {
		t.Fatalf("rootfs mount = %+v", mounts[0])
	}
	if mounts[1].Key != "mp0" || mounts[1].Path != "/srv/data" {
		t.Fatalf("mp0 mount = %+v", mounts[1])
	}
	if mounts[2].Key != "mp1" || mounts[2].Path != "/shared" || mounts[2].Volume != "/host/bind" {
		t.Fatalf("bind mount = %+v", mounts[2])
	}
}

func TestParseLXCInfoPID(t *testing.T) {
	if pid, err := parseLXCInfoPID("Name:  100\nPID:            4242\n"); err != nil || pid != 4242 {
		t.Fatalf("pid = %d, err = %v", pid, err)
	}
	if _, err := parseLXCInfoPID("PID: 1\n"); err == nil {
		t.Fatal("expected init pid rejection")
	}
	if _, err := parseLXCInfoPID("PID: nope\n"); err == nil {
		t.Fatal("expected invalid pid error")
	}
	if _, err := parseLXCInfoPID("Name: 100\n"); err == nil {
		t.Fatal("expected missing pid error")
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
