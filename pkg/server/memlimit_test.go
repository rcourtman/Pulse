package server

import (
	"os"
	"path/filepath"
	"strconv"
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
		{"systemd service", "0::/system.slice/pulse.service\n", "system.slice/pulse.service"},
		{"container root", "0::/\n", "."},
		{"hybrid picks unified line", "12:memory:/legacy\n0::/system.slice/pulse.service\n", "system.slice/pulse.service"},
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

	got, ok := readCgroupV2MemoryLimit(root, proc)
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

	got, ok := readCgroupV2MemoryLimit(root, proc)
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

	got, ok := readCgroupV2MemoryLimit(root, proc)
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

	if got, ok := readCgroupV2MemoryLimit(root, proc); ok {
		t.Fatalf("expected no limit, got %d", got)
	}
}

func TestCgroupV1MemoryPathFrom(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"memory controller", "5:memory:/system.slice/pulse.service\n", "system.slice/pulse.service"},
		{"combined controllers", "7:cpu,cpuacct,memory:/docker/abc\n", "docker/abc"},
		{"namespaced root", "5:memory:/\n", "."},
		{"colon in path", "5:memory:/system.slice/pulse:worker.service\n", "system.slice/pulse:worker.service"},
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

	got, ok := readCgroupV1HierarchyMemoryLimit(root, proc)
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

	got, ok := readCgroupV1HierarchyMemoryLimit(root, proc)
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

	got, ok := readCgroupV1HierarchyMemoryLimit(root, proc)
	if !ok || got != 805306368 {
		t.Fatalf("got (%d, %v), want (805306368, true)", got, ok)
	}
}

func TestReadCgroupV1HierarchyMemoryLimitNoMemoryController(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc-self-cgroup")
	writeCgroupFixture(t, proc, "2:cpu,cpuacct:/system.slice/pulse.service\n")

	if got, ok := readCgroupV1HierarchyMemoryLimit(root, proc); ok {
		t.Fatalf("expected no memory limit, got %d", got)
	}
}

func TestReadCgroupV1MemoryLimit(t *testing.T) {
	dir := t.TempDir()
	limitFile := filepath.Join(dir, "memory.limit_in_bytes")

	writeCgroupFixture(t, limitFile, "536870912\n")
	if got, ok := readCgroupV1MemoryLimit(limitFile); !ok || got != 536870912 {
		t.Fatalf("got (%d, %v), want (536870912, true)", got, ok)
	}

	// The v1 no-limit sentinel reports no limit.
	writeCgroupFixture(t, limitFile, strconv.FormatInt(cgroupV1NoLimit, 10)+"\n")
	if got, ok := readCgroupV1MemoryLimit(limitFile); ok {
		t.Fatalf("expected sentinel to mean no limit, got %d", got)
	}

	if _, ok := readCgroupV1MemoryLimit(filepath.Join(dir, "missing")); ok {
		t.Fatal("expected missing file to mean no limit")
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

	limit, source, ok := readRuntimeMemoryLimitFrom(v2Root, v1Root, filepath.Join(v1Root, "memory.limit_in_bytes"), procCgroup, procMeminfo)
	if !ok || limit != 2147483648 || source != "proc_meminfo" {
		t.Fatalf("readRuntimeMemoryLimitFrom(...) = (%d, %q, %v), want (2147483648, %q, true)", limit, source, ok, "proc_meminfo")
	}

	// When a tighter systemd MemoryMax is also set inside the container, it must win.
	writeCgroupFixture(t, filepath.Join(v2Root, "system.slice/pulse.service/memory.max"), "536870912\n")
	limit, source, ok = readRuntimeMemoryLimitFrom(v2Root, v1Root, filepath.Join(v1Root, "memory.limit_in_bytes"), procCgroup, procMeminfo)
	if !ok || limit != 536870912 || source != "cgroup" {
		t.Fatalf("readRuntimeMemoryLimitFrom(...) with tighter service limit = (%d, %q, %v), want (536870912, %q, true)", limit, source, ok, "cgroup")
	}
}
