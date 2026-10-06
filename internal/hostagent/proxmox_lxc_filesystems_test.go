package hostagent

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/filesystemprobe"
	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/filesystem"
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
	observations := map[int]map[string]lxcObservation{
		4242: {
			"/":         lxcUsage(8<<30, 2<<30, 6<<30),
			"/srv/data": lxcUsage(1<<40, 512<<30, 512<<30),
			// mp1 is configured but not mounted in the running container, so
			// the prober rejects it instead of reporting the parent's numbers.
			"/mnt/missing": {notMounted: true},
		},
		5252: {"/": lxcUsage(4<<30, 1<<30, 3<<30)},
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
		observeLXCFn: fakeLXCObserve(t, observations),
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
		observeLXCFn: fakeLXCObserve(t, map[int]map[string]lxcObservation{
			4242: {"/": lxcUsage(8<<30, 2<<30, 6<<30)},
		}),
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
				observeLXCFn: fakeLXCObserve(t, map[int]map[string]lxcObservation{
					4242: {
						"/":         lxcUsage(8<<30, 2<<30, 6<<30),
						"/srv/data": {err: "read filesystem counters: permission denied"},
					},
				}),
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

type fakeLXCDirEntry struct {
	name string
	dir  bool
}

func (e fakeLXCDirEntry) Name() string { return e.name }
func (e fakeLXCDirEntry) IsDir() bool  { return e.dir }
func (e fakeLXCDirEntry) Type() fs.FileMode {
	if e.dir {
		return fs.ModeDir
	}
	return 0
}
func (e fakeLXCDirEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrNotExist }

// fakeProxmoxLXCHost models what socket-free discovery reads on a cgroup v2
// Proxmox node. The layout follows a real PVE 9.2 node: configs under
// /etc/pve/lxc, the payload under /sys/fs/cgroup/lxc/<vmid>/ns, and the
// container init identified by an NSpid ending in 1.
type fakeProxmoxLXCHost struct {
	files        map[string]string
	dirs         map[string]map[string]bool
	observations map[int]map[string]lxcObservation
}

func newFakeProxmoxLXCHost() *fakeProxmoxLXCHost {
	h := &fakeProxmoxLXCHost{
		files:        map[string]string{proxmoxLXCCgroupV2Marker: "cpuset cpu io memory pids\n"},
		dirs:         map[string]map[string]bool{},
		observations: map[int]map[string]lxcObservation{},
	}
	h.dir(proxmoxLXCConfigDir)
	return h
}

func (h *fakeProxmoxLXCHost) dir(name string) {
	if _, ok := h.dirs[name]; !ok {
		h.dirs[name] = map[string]bool{}
	}
}

func (h *fakeProxmoxLXCHost) config(vmid int, content string) {
	h.dirs[proxmoxLXCConfigDir][strconv.Itoa(vmid)+".conf"] = false
	h.files[proxmoxLXCConfigPath(vmid)] = content
}

// cgroup registers a cgroup directory (relative to lxc/) and its processes.
func (h *fakeProxmoxLXCHost) cgroup(rel string, pids ...int) {
	full := path.Join(proxmoxLXCCgroupRoot, rel)
	for child := full; child != proxmoxLXCCgroupRoot; child = path.Dir(child) {
		h.dir(child)
		// Every cgroup directory has a cgroup.procs, empty when its
		// processes live in child cgroups.
		if _, ok := h.files[path.Join(child, "cgroup.procs")]; !ok {
			h.files[path.Join(child, "cgroup.procs")] = ""
		}
		parent := path.Dir(child)
		h.dir(parent)
		h.dirs[parent][path.Base(child)] = true
	}
	fields := make([]string, 0, len(pids))
	for _, pid := range pids {
		fields = append(fields, strconv.Itoa(pid))
	}
	h.files[path.Join(full, "cgroup.procs")] = strings.Join(fields, "\n") + "\n"
}

func (h *fakeProxmoxLXCHost) proc(pid int, nspid, cgroup string) {
	h.files["/proc/"+strconv.Itoa(pid)+"/status"] = "Name:\tinit\nNSpid:\t" + nspid + "\n"
	h.files["/proc/"+strconv.Itoa(pid)+"/cgroup"] = "0::" + cgroup + "\n"
}

// collector serves the fake host. pct and lxc-info exist but every call fails
// the way they fail inside the PrivateNetwork helper.
func (h *fakeProxmoxLXCHost) collector(t *testing.T) (*mockCollector, *int) {
	t.Helper()
	commands := 0
	return &mockCollector{
		goos: "linux",
		lookPathFn: func(file string) (string, error) {
			switch file {
			case "pct":
				return "/usr/sbin/pct", nil
			case "lxc-info":
				return "/usr/bin/lxc-info", nil
			}
			return "", os.ErrNotExist
		},
		statFn: func(name string) (os.FileInfo, error) {
			if _, ok := h.files[name]; ok {
				return nil, nil
			}
			if _, ok := h.dirs[name]; ok {
				return nil, nil
			}
			return nil, os.ErrNotExist
		},
		readFileFn: func(name string) ([]byte, error) {
			content, ok := h.files[name]
			if !ok {
				return nil, os.ErrNotExist
			}
			return []byte(content), nil
		},
		readDirFn: func(name string) ([]os.DirEntry, error) {
			children, ok := h.dirs[name]
			if !ok {
				return nil, os.ErrNotExist
			}
			entries := make([]os.DirEntry, 0, len(children))
			for child, isDir := range children {
				entries = append(entries, fakeLXCDirEntry{name: child, dir: isDir})
			}
			return entries, nil
		},
		observeLXCFn: fakeLXCObserve(t, h.observations),
		commandCombinedOutputLimitedFn: func(_ context.Context, _ int, name string, args ...string) (string, error) {
			commands++
			return "ipcc_send_rec[1] failed: Connection refused", errors.New("exit status 2")
		},
	}, &commands
}

func TestProxmoxLXCConfigNameMatchesPctList(t *testing.T) {
	cases := []struct {
		config string
		want   string
	}{
		{"#managed\narch: amd64\nhostname: web\nrootfs: x,size=1G\n", "web"},
		{"arch: amd64\nrootfs: x,size=1G\n", "CT126"},
		{"hostname: \n", "CT126"},
		{"arch: amd64\n\n[snap]\nhostname: old\n", "CT126"},
	}
	for _, tc := range cases {
		if got := proxmoxLXCConfigName(tc.config, 126); got != tc.want {
			t.Errorf("proxmoxLXCConfigName(%q) = %q, want %q", tc.config, got, tc.want)
		}
	}
}

func TestDiscoverRunningProxmoxLXCContainersFindsInitWithoutSockets(t *testing.T) {
	h := newFakeProxmoxLXCHost()
	// Alpine guest running Docker: a nested container's PID 1 shares the
	// guest's cgroup listing and comes first, but is not the guest's init.
	h.config(126, "hostname: qual2511\nrootfs: lvm:vm-126-disk-0,size=1G\n")
	h.cgroup("126")
	h.cgroup("126/ns", 3019900, 3019673, 3019253)
	h.cgroup("126/ns/openrc.syslog", 3019638)
	h.proc(3019900, "3019900\t420\t1", "/lxc/126/ns")
	h.proc(3019673, "3019673\t413", "/lxc/126/ns")
	h.proc(3019253, "3019253\t1", "/lxc/126/ns")
	// systemd guest: init in ns/init.scope, services elsewhere.
	h.config(127, "rootfs: lvm:vm-127-disk-0,size=1G\n")
	h.cgroup("127/ns/system.slice/cron.service", 4100)
	h.cgroup("127/ns/init.scope", 4000)
	h.proc(4100, "4100\t88", "/lxc/127/ns/system.slice/cron.service")
	h.proc(4000, "4000\t1", "/lxc/127/ns/init.scope")
	// Stopped: config only, no cgroup.
	h.config(128, "hostname: stopped\nrootfs: lvm:vm-128-disk-0,size=1G\n")

	collector, commands := h.collector(t)
	agent := &Agent{logger: zerolog.Nop(), collector: collector}
	got, ok := agent.discoverRunningProxmoxLXCContainers(context.Background())
	if !ok {
		t.Fatal("discovery was not applicable on a cgroup v2 Proxmox host")
	}
	want := []proxmoxLXCRunningContainer{
		{VMID: 126, Name: "qual2511", PID: 3019253},
		{VMID: 127, Name: "CT127", PID: 4000},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("discovered %+v, want %+v", got, want)
	}
	if *commands != 0 {
		t.Fatalf("discovery ran %d commands, want none", *commands)
	}
}

// Discovery must never turn a running guest it could not read into a smaller,
// healthy-looking inventory. Whenever a guest's state is not established it
// hands the whole node back to pct, which works for root installs and is
// reported as degraded inside the helper.
func TestDiscoverRunningProxmoxLXCContainersLeavesUnestablishedStateToPct(t *testing.T) {
	running := func() *fakeProxmoxLXCHost {
		h := newFakeProxmoxLXCHost()
		h.config(126, "hostname: web\n")
		h.cgroup("126/ns", 10)
		h.proc(10, "10\t1", "/lxc/126/ns")
		return h
	}
	cases := map[string]func(h *fakeProxmoxLXCHost) context.Context{
		"cgroup v1 host": func(h *fakeProxmoxLXCHost) context.Context {
			delete(h.files, proxmoxLXCCgroupV2Marker)
			return context.Background()
		},
		"unreadable config of a running guest": func(h *fakeProxmoxLXCHost) context.Context {
			delete(h.files, proxmoxLXCConfigPath(126))
			return context.Background()
		},
		"guest cgroup without an init (a reused PID from another guest)": func(h *fakeProxmoxLXCHost) context.Context {
			h.proc(10, "10\t1", "/lxc/130/ns")
			return context.Background()
		},
		"unreadable cgroup.procs": func(h *fakeProxmoxLXCHost) context.Context {
			delete(h.files, path.Join(proxmoxLXCCgroupRoot, "126/ns/cgroup.procs"))
			return context.Background()
		},
		"guest-created cgroup tree beyond the search limit": func(h *fakeProxmoxLXCHost) context.Context {
			h.proc(10, "10\t2", "/lxc/126/ns")
			for i := 0; i < proxmoxLXCMaxCgroupDirs; i++ {
				h.cgroup("126/ns/c" + strconv.Itoa(i))
			}
			return context.Background()
		},
		"cancelled collection": func(*fakeProxmoxLXCHost) context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			h := running()
			ctx := mutate(h)
			collector, _ := h.collector(t)
			got, ok := (&Agent{logger: zerolog.Nop(), collector: collector}).discoverRunningProxmoxLXCContainers(ctx)
			if ok {
				t.Fatalf("discovery reported %+v as complete", got)
			}
		})
	}
}

// lxcObservation is one mount's answer from the fake filesystem prober.
type lxcObservation struct {
	usage      *filesystem.Usage
	notMounted bool
	err        string
}

func lxcUsage(total, used, avail uint64) lxcObservation {
	return lxcObservation{usage: &filesystem.Usage{CapacityBytes: total, FreeBytes: total - used, AvailableBytes: avail}}
}

// fakeLXCObserve answers prober requests from per-PID mount tables. A PID the
// table does not know is not the guest's init, as the real prober would find.
func fakeLXCObserve(t *testing.T, byPID map[int]map[string]lxcObservation) func(context.Context, filesystemprobe.ContainerRequest) ([]filesystem.Observation, error) {
	t.Helper()
	return func(_ context.Context, req filesystemprobe.ContainerRequest) ([]filesystem.Observation, error) {
		if req.Runtime != "lxc" || !req.MountsOnly || req.ContainerID == "" {
			t.Errorf("prober request = %+v, want an exact lxc request with MountsOnly", req)
		}
		mounts, ok := byPID[req.PID]
		if !ok {
			return nil, errors.New("process is not the container's init")
		}
		out := make([]filesystem.Observation, 0, len(req.Mountpoints))
		for _, mountpoint := range req.Mountpoints {
			answer, known := mounts[mountpoint]
			if !known {
				t.Errorf("unexpected mount %q for pid %d", mountpoint, req.PID)
				continue
			}
			observation := filesystem.Observation{Mountpoint: mountpoint, Source: filesystemprobe.Source}
			switch {
			case answer.notMounted:
				observation.Error = filesystemprobe.ErrNotMounted.Error()
			case answer.err != "":
				observation.Error = answer.err
			default:
				observation.Usage = answer.usage
			}
			out = append(out, observation)
		}
		return out, nil
	}
}
