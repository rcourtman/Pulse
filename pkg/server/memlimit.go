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
	cgroupV2Root = "/sys/fs/cgroup"
	// cgroupV1NoLimit is the value cgroup v1 reports when no memory limit is
	// set (PAGE_SIZE-rounded max int64).
	cgroupV1NoLimit = int64(9223372036854771712)
	// memoryLimitHeadroomPercent is how much of the cgroup limit the Go
	// runtime may use. The remainder absorbs non-heap memory the GC cannot
	// control: goroutine stacks, mmap'd files, and CGO allocations.
	memoryLimitHeadroomPercent = 90
	// minimumUsableMemoryLimit guards against nonsense cgroup values; below
	// this a soft limit would just make the GC thrash.
	minimumUsableMemoryLimit = int64(64 << 20)
)

// applyRuntimeMemoryLimit aligns the Go GC with the enclosing cgroup memory
// limit so a capped service (systemd MemoryMax, docker --memory, Kubernetes
// limits) tightens garbage collection as it approaches the cap instead of
// growing until the kernel OOM-kills it. Without this the GC paces itself
// purely off GOGC and is blind to the limit. Best effort: any failure leaves
// the runtime untouched.
func applyRuntimeMemoryLimit() {
	if v := strings.TrimSpace(os.Getenv("GOMEMLIMIT")); v != "" {
		// The runtime already honors the explicit operator setting.
		log.Debug().Str("GOMEMLIMIT", v).Msg("runtime memory limit set from environment")
		return
	}

	limit, ok := readCgroupMemoryLimit()
	if !ok {
		log.Debug().Msg("no cgroup memory limit detected; leaving GC defaults")
		return
	}

	soft := limit / 100 * memoryLimitHeadroomPercent
	if soft < minimumUsableMemoryLimit {
		log.Warn().Int64("cgroupLimitBytes", limit).Msg("cgroup memory limit too small for a GC soft limit; leaving GC defaults")
		return
	}

	debug.SetMemoryLimit(soft)
	log.Info().
		Int64("cgroupLimitBytes", limit).
		Int64("goMemLimitBytes", soft).
		Msg("aligned Go memory limit with cgroup memory limit")
}

func readCgroupMemoryLimit() (int64, bool) {
	if limit, ok := readCgroupV2MemoryLimit(cgroupV2Root, "/proc/self/cgroup"); ok {
		return limit, true
	}
	return readCgroupV1MemoryLimit("/sys/fs/cgroup/memory/memory.limit_in_bytes")
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
