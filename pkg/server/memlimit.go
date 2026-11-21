package server

import (
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

const (
	cgroupV2Root    = "/sys/fs/cgroup"
	procMeminfoPath = "/proc/meminfo"
	// cgroupV1NoLimit is the value cgroup v1 reports when no memory limit is
	// set (PAGE_SIZE-rounded max int64).
	cgroupV1NoLimit = int64(9223372036854771712)
	// memoryLimitHeadroomPercent is how much of the detected memory limit the
	// Go runtime may use. The remainder absorbs non-heap memory the GC cannot
	// control: goroutine stacks, mmap'd files, and CGO allocations.
	memoryLimitHeadroomPercent = 90
	// minimumUsableMemoryLimit guards against nonsense limit values; below
	// this a soft limit would just make the GC thrash.
	minimumUsableMemoryLimit = int64(64 << 20)
)

// applyRuntimeMemoryLimit aligns the Go GC with the enclosing container or
// host memory limit so a capped service (Proxmox LXC, systemd MemoryMax,
// docker --memory, Kubernetes limits, small VMs) tightens garbage collection
// as it approaches the cap instead of growing until the kernel OOM-kills it.
// Without this the GC paces itself purely off GOGC and is blind to the limit.
//
// In Proxmox VE LXC containers on cgroup v2, memory.max is enforced on the
// parent /lxc/<vmid> cgroup above the container's /lxc/<vmid>/ns namespace
// root, so every visible memory.max inside the container reads "max" while
// lxcfs virtualizes /proc/meminfo MemTotal to the container limit (#1803).
// Taking the tightest bound across cgroup limits and /proc/meminfo MemTotal
// covers both cgroup-capped services and namespaced LXC containers.
// Best effort: any failure leaves the runtime untouched.
func applyRuntimeMemoryLimit() {
	if v := strings.TrimSpace(os.Getenv("GOMEMLIMIT")); v != "" {
		// The runtime already honors the explicit operator setting.
		log.Debug().Str("GOMEMLIMIT", v).Msg("runtime memory limit set from environment")
		return
	}

	limit, source, ok := readRuntimeMemoryLimit()
	if !ok {
		log.Debug().Msg("no cgroup or /proc/meminfo memory limit detected; leaving GC defaults")
		return
	}

	soft := limit / 100 * memoryLimitHeadroomPercent
	if soft < minimumUsableMemoryLimit {
		ev := log.Warn().
			Int64("memoryLimitBytes", limit).
			Str("limitSource", source)
		if source == "cgroup" {
			ev = ev.Int64("cgroupLimitBytes", limit)
		}
		ev.Msg("detected memory limit too small for a GC soft limit; leaving GC defaults")
		return
	}

	debug.SetMemoryLimit(soft)
	ev := log.Info().
		Int64("memoryLimitBytes", limit).
		Str("limitSource", source).
		Int64("goMemLimitBytes", soft)
	if source == "cgroup" {
		ev.Int64("cgroupLimitBytes", limit).Msg("aligned Go memory limit with cgroup memory limit")
		return
	}
	ev.Msg("aligned Go memory limit with /proc/meminfo MemTotal")
}

func readRuntimeMemoryLimit() (int64, string, bool) {
	return readRuntimeMemoryLimitFrom(
		cgroupV2Root,
		"/sys/fs/cgroup/memory",
		"/sys/fs/cgroup/memory/memory.limit_in_bytes",
		"/proc/self/cgroup",
		procMeminfoPath,
	)
}

func readRuntimeMemoryLimitFrom(v2Root, v1Root, v1FallbackFile, procSelfCgroup, meminfoPath string) (int64, string, bool) {
	cgroupLimit, hasCgroup := readCgroupMemoryLimitFrom(v2Root, v1Root, v1FallbackFile, procSelfCgroup)
	meminfoTotal, hasMeminfo := readProcMeminfoTotal(meminfoPath)
	return selectRuntimeMemoryLimit(cgroupLimit, hasCgroup, meminfoTotal, hasMeminfo)
}

func selectRuntimeMemoryLimit(cgroupLimit int64, hasCgroup bool, meminfoTotal int64, hasMeminfo bool) (int64, string, bool) {
	if hasCgroup && cgroupLimit <= 0 {
		hasCgroup = false
	}
	if hasMeminfo && meminfoTotal <= 0 {
		hasMeminfo = false
	}
	switch {
	case hasCgroup && hasMeminfo:
		if meminfoTotal < cgroupLimit {
			return meminfoTotal, "proc_meminfo", true
		}
		return cgroupLimit, "cgroup", true
	case hasCgroup:
		return cgroupLimit, "cgroup", true
	case hasMeminfo:
		return meminfoTotal, "proc_meminfo", true
	default:
		return 0, "", false
	}
}

func readCgroupMemoryLimitFrom(v2Root, v1Root, v1FallbackFile, procSelfCgroup string) (int64, bool) {
	if limit, ok := readCgroupV2MemoryLimit(v2Root, procSelfCgroup); ok {
		return limit, true
	}
	if limit, ok := readCgroupV1HierarchyMemoryLimit(v1Root, procSelfCgroup); ok {
		return limit, true
	}
	// Preserve the original best-effort fallback for unusual v1 mounts that
	// expose the controller limit but not a readable /proc/self/cgroup.
	return readCgroupV1MemoryLimit(v1FallbackFile)
}

func readProcMeminfoTotal(path string) (int64, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	return parseProcMeminfoTotal(string(raw))
}

// parseProcMeminfoTotal extracts MemTotal in bytes from Linux/lxcfs
// /proc/meminfo content (e.g. "MemTotal:        2097152 kB").
func parseProcMeminfoTotal(content string) (int64, bool) {
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, "MemTotal:"))
		if len(fields) < 2 {
			return 0, false
		}
		v, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil || v <= 0 {
			return 0, false
		}
		var mult int64
		switch strings.ToLower(fields[1]) {
		case "b":
			mult = 1
		case "kb", "kib":
			mult = 1 << 10
		case "mb", "mib":
			mult = 1 << 20
		case "gb", "gib":
			mult = 1 << 30
		default:
			return 0, false
		}
		if v > math.MaxInt64/mult {
			return 0, false
		}
		return v * mult, true
	}
	return 0, false
}

// readCgroupV2MemoryLimit resolves the process's own cgroup from
// procSelfCgroup and walks from that directory up to the cgroup root, taking
// the smallest memory.max on the path. The limit may sit on any ancestor
// (systemd applies MemoryMax to the service cgroup; container runtimes to the
// container root).
func readCgroupV2MemoryLimit(root, procSelfCgroup string) (int64, bool) {
	data, err := os.ReadFile(procSelfCgroup)
	if err != nil {
		return 0, false
	}
	rel := cgroupV2PathFrom(string(data))
	if rel == "" {
		return 0, false
	}

	lowest := int64(math.MaxInt64)
	dir := filepath.Join(root, rel)
	for {
		if raw, err := os.ReadFile(filepath.Join(dir, "memory.max")); err == nil {
			if v, ok := parseCgroupMemoryValue(string(raw)); ok && v < lowest {
				lowest = v
			}
		}
		if dir == root {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if lowest == math.MaxInt64 {
		return 0, false
	}
	return lowest, true
}

// cgroupV2PathFrom extracts the unified-hierarchy path from /proc/self/cgroup
// content ("0::/system.slice/pulse.service" -> "system.slice/pulse.service").
// A process in a cgroup namespace commonly sees its own cgroup as "0::/";
// filepath's "." preserves that valid root path without conflating it with a
// missing unified-hierarchy entry.
func cgroupV2PathFrom(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, "0::") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "0::"))
		if path == "/" {
			return "."
		}
		return strings.TrimPrefix(path, "/")
	}
	return ""
}

// readCgroupV1HierarchyMemoryLimit resolves the process's memory-controller
// path and walks its ancestors just as the v2 reader does. Reading only the
// controller root misses a tighter systemd MemoryLimit/MemoryMax applied to a
// service cgroup on v1 hosts.
func readCgroupV1HierarchyMemoryLimit(root, procSelfCgroup string) (int64, bool) {
	data, err := os.ReadFile(procSelfCgroup)
	if err != nil {
		return 0, false
	}
	rel := cgroupV1MemoryPathFrom(string(data))
	if rel == "" {
		return 0, false
	}

	lowest := int64(math.MaxInt64)
	dir := filepath.Join(root, rel)
	for {
		if raw, err := os.ReadFile(filepath.Join(dir, "memory.limit_in_bytes")); err == nil {
			if v, ok := parseCgroupMemoryValue(string(raw)); ok && v < cgroupV1NoLimit && v < lowest {
				lowest = v
			}
		}
		if dir == root {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if lowest == math.MaxInt64 {
		return 0, false
	}
	return lowest, true
}

// cgroupV1MemoryPathFrom extracts the path for the v1 memory controller from
// /proc/self/cgroup. Controllers may be mounted together, so "memory" can be
// one item in a comma-separated controller field.
func cgroupV1MemoryPathFrom(content string) string {
	for _, line := range strings.Split(content, "\n") {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) != 3 {
			continue
		}
		hasMemory := false
		for _, controller := range strings.Split(fields[1], ",") {
			if strings.TrimSpace(controller) == "memory" {
				hasMemory = true
				break
			}
		}
		if !hasMemory {
			continue
		}
		path := strings.TrimSpace(fields[2])
		if path == "/" {
			return "."
		}
		return strings.TrimPrefix(path, "/")
	}
	return ""
}

func readCgroupV1MemoryLimit(limitFile string) (int64, bool) {
	raw, err := os.ReadFile(limitFile)
	if err != nil {
		return 0, false
	}
	v, ok := parseCgroupMemoryValue(string(raw))
	if !ok || v >= cgroupV1NoLimit {
		return 0, false
	}
	return v, true
}

// parseCgroupMemoryValue parses a cgroup memory file value. "max" (v2's
// explicit no-limit marker) and non-numeric content report no limit.
func parseCgroupMemoryValue(raw string) (int64, bool) {
	s := strings.TrimSpace(raw)
	if s == "" || s == "max" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}
