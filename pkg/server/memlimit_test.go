package server

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestParseCgroupMemoryValue(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int64
		ok   bool
	}{
		{"plain number", "838860800\n", 838860800, true},
		{"max marker", "max\n", 0, false},
		{"empty", "", 0, false},
		{"whitespace", "  \n", 0, false},
		{"garbage", "not-a-number", 0, false},
		{"zero", "0", 0, false},
		{"negative", "-5", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseCgroupMemoryValue(tc.raw)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("parseCgroupMemoryValue(%q) = (%d, %v), want (%d, %v)", tc.raw, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestCgroupV2PathFrom(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"systemd service", "0::/system.slice/pulse.service\n", "/system.slice/pulse.service"},
		{"container root", "0::/\n", "/"},
		{"hybrid picks unified line", "12:memory:/legacy\n0::/system.slice/pulse.service\n", "/system.slice/pulse.service"},
		{"outside namespace", "0::/../system.slice/pulse.service\n", "/../system.slice/pulse.service"},
		{"not absolute", "0::system.slice/pulse.service\n", ""},
		{"v1 only", "12:memory:/legacy\n", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cgroupV2PathFrom(tc.content); got != tc.want {
				t.Fatalf("cgroupV2PathFrom(%q) = %q, want %q", tc.content, got, tc.want)
			}
		})
	}
}

func TestReadCgroupV2MemoryLimitAtNamespacedRoot(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc-self-cgroup")
	writeCgroupFixture(t, proc, "0::/\n")
	writeCgroupFixture(t, filepath.Join(root, "memory.max"), "536870912\n")

	got, ok := readCgroupMemoryLimitFrom(memoryLimitPaths{cgroup: proc, legacyV2Root: root})
	if !ok || got != 536870912 {
		t.Fatalf("got (%d, %v), want (536870912, true)", got, ok)
	}
}

func writeCgroupFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadCgroupV2MemoryLimitWalksAncestors(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc-self-cgroup")
	writeCgroupFixture(t, proc, "0::/system.slice/pulse.service\n")

	// Limit set on the service, "max" at the root: service limit wins.
	writeCgroupFixture(t, filepath.Join(root, "system.slice/pulse.service/memory.max"), "838860800\n")
	writeCgroupFixture(t, filepath.Join(root, "system.slice/memory.max"), "max\n")
	writeCgroupFixture(t, filepath.Join(root, "memory.max"), "max\n")

	got, ok := readCgroupMemoryLimitFrom(memoryLimitPaths{cgroup: proc, legacyV2Root: root})
	if !ok || got != 838860800 {
		t.Fatalf("got (%d, %v), want (838860800, true)", got, ok)
	}
}

func TestReadCgroupV2MemoryLimitTakesSmallestOnPath(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc-self-cgroup")
	writeCgroupFixture(t, proc, "0::/a/b\n")

	// Ancestor holds the tighter limit.
	writeCgroupFixture(t, filepath.Join(root, "a/b/memory.max"), "2147483648\n")
	writeCgroupFixture(t, filepath.Join(root, "a/memory.max"), "1073741824\n")

	got, ok := readCgroupMemoryLimitFrom(memoryLimitPaths{cgroup: proc, legacyV2Root: root})
	if !ok || got != 1073741824 {
		t.Fatalf("got (%d, %v), want (1073741824, true)", got, ok)
	}
}

func TestReadCgroupV2MemoryLimitNoLimitAnywhere(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc-self-cgroup")
	writeCgroupFixture(t, proc, "0::/a\n")
	writeCgroupFixture(t, filepath.Join(root, "a/memory.max"), "max\n")
	writeCgroupFixture(t, filepath.Join(root, "memory.max"), "max\n")

	if got, ok := readCgroupMemoryLimitFrom(memoryLimitPaths{cgroup: proc, legacyV2Root: root}); ok {
		t.Fatalf("expected no limit, got %d", got)
	}
}

func TestCgroupV1MemoryPathFrom(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"memory controller", "5:memory:/system.slice/pulse.service\n", "/system.slice/pulse.service"},
		{"combined controllers", "7:cpu,cpuacct,memory:/docker/abc\n", "/docker/abc"},
		{"namespaced root", "5:memory:/\n", "/"},
		{"colon in path", "5:memory:/system.slice/pulse:worker.service\n", "/system.slice/pulse:worker.service"},
		{"outside namespace", "5:memory:/../system.slice/pulse.service\n", "/../system.slice/pulse.service"},
		{"other controllers only", "2:cpu,cpuacct:/system.slice/pulse.service\n", ""},
		{"v2 only", "0::/system.slice/pulse.service\n", ""},
		{"malformed", "memory:/system.slice/pulse.service\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cgroupV1MemoryPathFrom(tc.content); got != tc.want {
				t.Fatalf("cgroupV1MemoryPathFrom(%q) = %q, want %q", tc.content, got, tc.want)
			}
		})
	}
}

func TestReadCgroupV1HierarchyMemoryLimitAtNamespacedRoot(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc-self-cgroup")
	writeCgroupFixture(t, proc, "5:memory:/\n")
	writeCgroupFixture(t, filepath.Join(root, "memory.limit_in_bytes"), "536870912\n")

	got, ok := readCgroupMemoryLimitFrom(memoryLimitPaths{cgroup: proc, legacyV1Root: root})
	if !ok || got != 536870912 {
		t.Fatalf("got (%d, %v), want (536870912, true)", got, ok)
	}
}

func TestReadCgroupV1HierarchyMemoryLimitWalksAncestors(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc-self-cgroup")
	writeCgroupFixture(t, proc, "5:memory:/system.slice/pulse.service\n")

	// The service limit must win over the unlimited controller root. This is
	// the layout used by systemd MemoryLimit/MemoryMax on cgroup v1.
	writeCgroupFixture(t, filepath.Join(root, "system.slice/pulse.service/memory.limit_in_bytes"), "536870912\n")
	writeCgroupFixture(t, filepath.Join(root, "system.slice/memory.limit_in_bytes"), "1073741824\n")
	writeCgroupFixture(t, filepath.Join(root, "memory.limit_in_bytes"), strconv.FormatInt(cgroupV1NoLimit, 10)+"\n")

	got, ok := readCgroupMemoryLimitFrom(memoryLimitPaths{cgroup: proc, legacyV1Root: root})
	if !ok || got != 536870912 {
		t.Fatalf("got (%d, %v), want (536870912, true)", got, ok)
	}
}

func TestReadCgroupV1HierarchyMemoryLimitTakesTightestAncestor(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc-self-cgroup")
	writeCgroupFixture(t, proc, "8:cpu,memory:/parent/child\n")
	writeCgroupFixture(t, filepath.Join(root, "parent/child/memory.limit_in_bytes"), "2147483648\n")
	writeCgroupFixture(t, filepath.Join(root, "parent/memory.limit_in_bytes"), "805306368\n")

	got, ok := readCgroupMemoryLimitFrom(memoryLimitPaths{cgroup: proc, legacyV1Root: root})
	if !ok || got != 805306368 {
		t.Fatalf("got (%d, %v), want (805306368, true)", got, ok)
	}
}

func TestReadCgroupV1HierarchyMemoryLimitNoMemoryController(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc-self-cgroup")
	writeCgroupFixture(t, proc, "2:cpu,cpuacct:/system.slice/pulse.service\n")

	if got, ok := readCgroupMemoryLimitFrom(memoryLimitPaths{cgroup: proc, legacyV1Root: root}); ok {
		t.Fatalf("expected no memory limit, got %d", got)
	}
}

func TestReadLegacyCgroupV1RootWhenMembershipUnavailable(t *testing.T) {
	dir := t.TempDir()
	limitFile := filepath.Join(dir, "memory.limit_in_bytes")
	paths := memoryLimitPaths{legacyV1Root: dir}

	if _, ok := readCgroupMemoryLimitFrom(paths); ok {
		t.Fatal("expected missing file to mean no limit")
	}

	// The v1 no-limit sentinel reports no limit.
	writeCgroupFixture(t, limitFile, strconv.FormatInt(cgroupV1NoLimit, 10)+"\n")
	if got, ok := readCgroupMemoryLimitFrom(paths); ok {
		t.Fatalf("expected sentinel to mean no limit, got %d", got)
	}

	writeCgroupFixture(t, limitFile, "536870912\n")
	if got, ok := readCgroupMemoryLimitFrom(paths); !ok || got != 536870912 {
		t.Fatalf("got (%d, %v), want (536870912, true)", got, ok)
	}
}

func TestParseProcMeminfoTotal(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    int64
		ok      bool
	}{
		{
			name:    "typical lxcfs 2 GiB",
			content: "MemTotal:        2097152 kB\nMemFree:         1500000 kB\n",
			want:    2147483648,
			ok:      true,
		},
		{
			name:    "small 256 MiB container",
			content: "MemTotal: 262144 KiB\n",
			want:    268435456,
			ok:      true,
		},
		{
			name:    "megabytes unit",
			content: "MemTotal: 512 MB\n",
			want:    536870912,
			ok:      true,
		},
		{
			name:    "gigabytes unit",
			content: "MemTotal: 4 GiB\n",
			want:    4294967296,
			ok:      true,
		},
		{
			name:    "bytes unit",
			content: "MemTotal: 536870912 B\n",
			want:    536870912,
			ok:      true,
		},
		{
			name:    "missing MemTotal",
			content: "MemFree: 1024 kB\n",
			want:    0,
			ok:      false,
		},
		{
			name:    "missing unit",
			content: "MemTotal: 2097152\n",
			want:    0,
			ok:      false,
		},
		{
			name:    "unknown unit",
			content: "MemTotal: 2097152 TB\n",
			want:    0,
			ok:      false,
		},
		{
			name:    "zero value",
			content: "MemTotal: 0 kB\n",
			want:    0,
			ok:      false,
		},
		{
			name:    "negative value",
			content: "MemTotal: -1024 kB\n",
			want:    0,
			ok:      false,
		},
		{
			name:    "non-numeric",
			content: "MemTotal: nope kB\n",
			want:    0,
			ok:      false,
		},
		{
			name:    "overflow",
			content: "MemTotal: 9223372036854775807 kB\n",
			want:    0,
			ok:      false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseProcMeminfoTotal(tc.content)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("parseProcMeminfoTotal(%q) = (%d, %v), want (%d, %v)", tc.content, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestSelectRuntimeMemoryLimit(t *testing.T) {
	const (
		mib512 = 512 << 20
		gib2   = 2 << 30
		gib16  = 16 << 30
	)
	cases := []struct {
		name         string
		cgroupLimit  int64
		hasCgroup    bool
		meminfoTotal int64
		hasMeminfo   bool
		wantLimit    int64
		wantSource   string
		wantOK       bool
	}{
		{
			name:         "Proxmox LXC cgroup v2 max with lxcfs MemTotal",
			hasCgroup:    false,
			meminfoTotal: gib2,
			hasMeminfo:   true,
			wantLimit:    gib2,
			wantSource:   "proc_meminfo",
			wantOK:       true,
		},
		{
			name:         "tighter cgroup limit wins over host MemTotal",
			cgroupLimit:  mib512,
			hasCgroup:    true,
			meminfoTotal: gib16,
			hasMeminfo:   true,
			wantLimit:    mib512,
			wantSource:   "cgroup",
			wantOK:       true,
		},
		{
			name:         "tighter MemTotal wins over larger cgroup limit",
			cgroupLimit:  gib16,
			hasCgroup:    true,
			meminfoTotal: gib2,
			hasMeminfo:   true,
			wantLimit:    gib2,
			wantSource:   "proc_meminfo",
			wantOK:       true,
		},
		{
			name:         "equal limits prefer cgroup",
			cgroupLimit:  gib2,
			hasCgroup:    true,
			meminfoTotal: gib2,
			hasMeminfo:   true,
			wantLimit:    gib2,
			wantSource:   "cgroup",
			wantOK:       true,
		},
		{
			name:        "cgroup only",
			cgroupLimit: mib512,
			hasCgroup:   true,
			wantLimit:   mib512,
			wantSource:  "cgroup",
			wantOK:      true,
		},
		{
			name:   "neither available",
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limit, source, ok := selectRuntimeMemoryLimit(tc.cgroupLimit, tc.hasCgroup, tc.meminfoTotal, tc.hasMeminfo)
			if ok != tc.wantOK || limit != tc.wantLimit || source != tc.wantSource {
				t.Fatalf("selectRuntimeMemoryLimit(...) = (%d, %q, %v), want (%d, %q, %v)",
					limit, source, ok, tc.wantLimit, tc.wantSource, tc.wantOK)
			}
		})
	}
}

func TestReadRuntimeMemoryLimitProxmoxLXCCgroupV2(t *testing.T) {
	// Proxmox VE LXC on cgroup v2 applies memory.max on /lxc/<vmid>, outside
	// the container's /lxc/<vmid>/ns namespace root, so every memory.max visible
	// inside the container reads "max" while lxcfs reports the cap in /proc/meminfo.
	root := t.TempDir()
	v2Root := filepath.Join(root, "cgroup")
	v1Root := filepath.Join(root, "cgroup-v1-missing")
	procCgroup := filepath.Join(root, "proc-self-cgroup")
	procMeminfo := filepath.Join(root, "proc-meminfo")

	writeCgroupFixture(t, procCgroup, "0::/system.slice/pulse.service\n")
	writeCgroupFixture(t, filepath.Join(v2Root, "system.slice/pulse.service/memory.max"), "max\n")
	writeCgroupFixture(t, filepath.Join(v2Root, "system.slice/memory.max"), "max\n")
	writeCgroupFixture(t, filepath.Join(v2Root, "memory.max"), "max\n")
	writeCgroupFixture(t, procMeminfo, "MemTotal:        2097152 kB\nMemFree:         1400000 kB\n")

	paths := memoryLimitPaths{cgroup: procCgroup, meminfo: procMeminfo, legacyV2Root: v2Root, legacyV1Root: v1Root}
	limit, source, ok := readRuntimeMemoryLimitFrom(paths)
	if !ok || limit != 2147483648 || source != "proc_meminfo" {
		t.Fatalf("readRuntimeMemoryLimitFrom(...) = (%d, %q, %v), want (2147483648, %q, true)", limit, source, ok, "proc_meminfo")
	}

	// When a tighter systemd MemoryMax is also set inside the container, it must win.
	writeCgroupFixture(t, filepath.Join(v2Root, "system.slice/pulse.service/memory.max"), "536870912\n")
	limit, source, ok = readRuntimeMemoryLimitFrom(paths)
	if !ok || limit != 536870912 || source != "cgroup" {
		t.Fatalf("readRuntimeMemoryLimitFrom(...) with tighter service limit = (%d, %q, %v), want (536870912, %q, true)", limit, source, ok, "cgroup")
	}
}

func TestReadRuntimeMemoryLimitMountedCgroups(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux /proc cgroup and mountinfo layout")
	}
	const mib, gib = 1 << 20, 1 << 30
	cases := []struct {
		name, membership, mountinfo string
		files                       map[string]string
		wantBytes                   int64
		wantSource                  string
	}{
		{
			name:       "delegated v2 mount resolves the service below its mount root",
			membership: "0::/system.slice/pulse.service\n",
			mountinfo:  "31 21 0:28 /system.slice $ROOT/visible/cgroup ro,nosuid,nodev - cgroup2 cgroup rw\n",
			files: map[string]string{
				"visible/cgroup/pulse.service/memory.max": "536870912\n",
				"visible/cgroup/memory.max":               "1073741824\n",
			},
			wantBytes: 512 * mib, wantSource: "cgroup",
		},
		{
			name:       "another v2 view exposes a tighter ancestor",
			membership: "0::/tenant/pulse.service\n",
			mountinfo: "31 21 0:28 /tenant/pulse.service $ROOT/delegated ro - cgroup2 cgroup rw\n" +
				"32 21 0:28 /tenant $ROOT/full/tenant ro - cgroup2 cgroup rw\n",
			files: map[string]string{
				"delegated/memory.max":                 "1073741824\n",
				"full/tenant/pulse.service/memory.max": "1073741824\n",
				"full/tenant/memory.max":               "536870912\n",
				"full/memory.max":                      "134217728\n", // above the mount point
			},
			wantBytes: 512 * mib, wantSource: "cgroup",
		},
		{
			name:       "combined v1 controller in a hybrid layout with escaped mount paths",
			membership: "0::/system.slice/pulse.service\n7:cpu,cpuacct,memory:/team name/pulse:worker.service\n",
			mountinfo: "31 21 0:28 / $ROOT/unified ro - cgroup2 cgroup rw\n" +
				"32 21 0:29 / $ROOT/other rw - cgroup cgroup rw,cpu,memory_extra\n" +
				"33 21 0:30 /team\\040name $ROOT/cpu\\040\\134memory ro shared:2 - cgroup cgroup rw,cpu,cpuacct,memory\n",
			files: map[string]string{
				"unified/system.slice/pulse.service/memory.max":              "max\n",
				"other/team name/pulse:worker.service/memory.limit_in_bytes": "134217728\n",
				"cpu \\memory/pulse:worker.service/memory.limit_in_bytes":    "268435456\n",
				"cpu \\memory/memory.limit_in_bytes":                         strconv.FormatInt(cgroupV1NoLimit, 10),
			},
			wantBytes: 256 * mib, wantSource: "cgroup",
		},
		{
			name:       "namespaced v2 root with a visible finite cap",
			membership: "0::/\n",
			mountinfo:  "31 21 0:28 / $ROOT/visible ro - cgroup2 cgroup rw\n",
			files:      map[string]string{"visible/memory.max": "536870912\n"},
			wantBytes:  512 * mib, wantSource: "cgroup",
		},
		{
			name:       "inherited v2 mount preserves its visible parent cap",
			membership: "0::/\n",
			mountinfo:  "31 21 0:28 /.. $ROOT/sys/fs/cgroup ro - cgroup2 cgroup rw\n",
			files:      map[string]string{"sys/fs/cgroup/memory.max": "536870912\n"},
			wantBytes:  512 * mib, wantSource: "cgroup",
		},
		{
			name:       "higher inherited root applies without guessing a member directory",
			membership: "0::/../peer/pulse.service\n",
			mountinfo:  "31 21 0:28 /../.. $ROOT/ancestor ro - cgroup2 cgroup rw\n",
			files: map[string]string{
				"ancestor/memory.max":                    "536870912\n",
				"ancestor/peer/pulse.service/memory.max": "134217728\n", // not under the unnamed child
			},
			wantBytes: 512 * mib, wantSource: "cgroup",
		},
		{
			name:       "inherited combined v1 memory mount preserves its parent cap",
			membership: "7:cpu,memory:/system.slice/pulse.service\n",
			mountinfo:  "31 21 0:30 /.. $ROOT/combined ro - cgroup cgroup rw,cpu,memory\n",
			files:      map[string]string{"combined/memory.limit_in_bytes": "536870912\n"},
			wantBytes:  512 * mib, wantSource: "cgroup",
		},
		{
			name:       "a namespace-relative ancestor is used only through its matching mount",
			membership: "0::/../../pulse.service\n",
			mountinfo: "31 21 0:28 / $ROOT/namespace ro - cgroup2 cgroup rw\n" +
				"32 21 0:28 /../.. $ROOT/ancestor ro - cgroup2 cgroup rw\n" +
				"33 21 0:28 /.. $ROOT/closer ro - cgroup2 cgroup rw\n",
			files: map[string]string{
				"namespace/memory.max":              "134217728\n",
				"ancestor/pulse.service/memory.max": "max\n",
				"ancestor/memory.max":               "536870912\n",
				"closer/memory.max":                 "134217728\n",
				"pulse.service/memory.max":          "134217728\n",
			},
			wantBytes: 512 * mib, wantSource: "cgroup",
		},
		{
			name:       "LXC uses MemTotal if no finite cgroup cap is visible",
			membership: "0::/system.slice/pulse.service\n",
			mountinfo:  "31 21 0:28 / $ROOT/visible ro - cgroup2 cgroup rw\n",
			files: map[string]string{
				"visible/system.slice/pulse.service/memory.max": "max\n",
				"visible/system.slice/memory.max":               "max\n",
				"visible/memory.max":                            "max\n",
			},
			wantBytes: 2 * gib, wantSource: "proc_meminfo",
		},
		{
			name:       "sibling mounts and guessed descendants do not supply a cap",
			membership: "0::/system.slice/pulse.service\n",
			mountinfo: "31 21 0:28 /system $ROOT/sibling ro - cgroup2 cgroup rw\n" +
				"32 21 0:28 /.. $ROOT/outside ro - cgroup2 cgroup rw\n" +
				"33 21 0:28 /../sibling $ROOT/outside-sibling ro - cgroup2 cgroup rw\n",
			files: map[string]string{
				"sibling/memory.max":                                  "134217728\n",
				"outside/memory.max":                                  "max\n",
				"outside/system.slice/pulse.service/memory.max":       "134217728\n",
				"outside-sibling/memory.max":                          "134217728\n",
				"sys/fs/cgroup/system.slice/pulse.service/memory.max": "134217728\n",
			},
			wantBytes: 2 * gib, wantSource: "proc_meminfo",
		},
		{
			name:       "a non-leading parent component does not establish ancestry",
			membership: "0::/system.slice/../pulse.service\n",
			mountinfo:  "31 21 0:28 /.. $ROOT/ancestor ro - cgroup2 cgroup rw\n",
			files:      map[string]string{"ancestor/memory.max": "134217728\n"},
			wantBytes:  2 * gib, wantSource: "proc_meminfo",
		},
		{
			name: "when membership is unavailable only the whole v1 controller can supply a fallback",
			mountinfo: "31 21 0:28 /unrelated $ROOT/delegated ro - cgroup cgroup rw,memory\n" +
				"32 21 0:28 / $ROOT/whole ro - cgroup cgroup rw,memory\n",
			files: map[string]string{
				"delegated/memory.limit_in_bytes": "134217728\n",
				"whole/memory.limit_in_bytes":     "536870912\n",
			},
			wantBytes: 512 * mib, wantSource: "cgroup",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			paths := memoryLimitPaths{
				cgroup:       filepath.Join(root, "proc/self/cgroup"),
				mountInfo:    filepath.Join(root, "proc/self/mountinfo"),
				meminfo:      filepath.Join(root, "proc/meminfo"),
				legacyV2Root: filepath.Join(root, "sys/fs/cgroup"),
				legacyV1Root: filepath.Join(root, "sys/fs/cgroup/memory"),
			}
			escape := strings.NewReplacer("\\", `\134`, " ", `\040`, "\t", `\011`, "\n", `\012`)
			writeCgroupFixture(t, paths.mountInfo, strings.ReplaceAll(tc.mountinfo, "$ROOT", escape.Replace(root)))
			if tc.membership != "" {
				writeCgroupFixture(t, paths.cgroup, tc.membership)
			}
			writeCgroupFixture(t, paths.meminfo, "MemTotal: 2097152 kB\n")
			for name, content := range tc.files {
				writeCgroupFixture(t, filepath.Join(root, name), content)
			}

			bytes, source, ok := readRuntimeMemoryLimitFrom(paths)
			if !ok || bytes != tc.wantBytes || source != tc.wantSource {
				t.Fatalf("selected (%d, %q, %v), want (%d, %q, true)", bytes, source, ok, tc.wantBytes, tc.wantSource)
			}
		})
	}
}
