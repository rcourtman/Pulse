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
