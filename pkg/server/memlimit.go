package server

import (
	"math"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

const (
	// cgroupV1NoLimit is the value cgroup v1 reports when no memory limit is
	// set (PAGE_SIZE-rounded max int64).
	cgroupV1NoLimit = int64(9223372036854771712)
	// memoryLimitHeadroomPercent is how much of the detected memory limit the
	// Go runtime may use. The remainder absorbs non-heap memory the GC cannot
	// control, such as mmap'd files and CGO allocations.
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
	return readRuntimeMemoryLimitFrom(memoryLimitPaths{
		cgroup:       "/proc/self/cgroup",
		mountInfo:    "/proc/self/mountinfo",
		meminfo:      "/proc/meminfo",
		legacyV2Root: "/sys/fs/cgroup",
		legacyV1Root: "/sys/fs/cgroup/memory",
	})
}

type memoryLimitPaths struct {
	cgroup, mountInfo, meminfo string
	legacyV2Root, legacyV1Root string
}

func readRuntimeMemoryLimitFrom(paths memoryLimitPaths) (int64, string, bool) {
	cgroupLimit, hasCgroup := readCgroupMemoryLimitFrom(paths)
	meminfoTotal, hasMeminfo := readProcMeminfoTotal(paths.meminfo)
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

func readCgroupMemoryLimitFrom(paths memoryLimitPaths) (int64, bool) {
	membership, membershipErr := os.ReadFile(paths.cgroup)
	v2Path := cgroupV2PathFrom(string(membership))
	v1Path := cgroupV1MemoryPathFrom(string(membership))

	var lowest int64
	consider := func(mount cgroupMemoryMount) {
		memberPath := v2Path
		if mount.v1 {
			memberPath = v1Path
			if membershipErr != nil && mount.root == "/" {
				// Preserve the best-effort v1 root read if membership is
				// unavailable. A delegated subtree is not a safe substitute.
				memberPath = "/"
			}
		}
		if limit, ok := mount.readLimit(memberPath); ok && (lowest == 0 || limit < lowest) {
			lowest = limit
		}
	}

	if raw, err := os.ReadFile(paths.mountInfo); err == nil {
		for line := range strings.SplitSeq(string(raw), "\n") {
			if mount, ok := parseCgroupMemoryMount(line); ok {
				consider(mount)
			}
		}
	} else {
		// Keep the conventional layout as a fallback for environments
		// without mountinfo. Do not guess a mapping when it is available.
		consider(cgroupMemoryMount{root: "/", point: paths.legacyV2Root})
		consider(cgroupMemoryMount{root: "/", point: paths.legacyV1Root, v1: true})
	}
	return lowest, lowest != 0
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

type cgroupMemoryMount struct {
	root, point string // controller subtree in cgroup space, and its mount point
	v1          bool
}

// mountinfo escapes spaces, tabs, newlines and literal backslashes in paths.
var mountInfoUnescape = strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, "\\")

func parseCgroupMemoryMount(line string) (cgroupMemoryMount, bool) {
	head, tail, ok := strings.Cut(line, " - ")
	if !ok {
		return cgroupMemoryMount{}, false
	}
	fs := strings.SplitN(tail, " ", 3)
	if len(fs) < 3 {
		return cgroupMemoryMount{}, false
	}
	v1 := fs[0] == "cgroup" && slices.Contains(strings.Split(fs[2], ","), "memory")
	if fs[0] != "cgroup2" && !v1 {
		return cgroupMemoryMount{}, false
	}
	fields := strings.SplitN(head, " ", 6)
	if len(fields) < 6 {
		return cgroupMemoryMount{}, false
	}
	root := absoluteCgroupPath(mountInfoUnescape.Replace(fields[3]))
	point := absoluteCgroupPath(mountInfoUnescape.Replace(fields[4]))
	return cgroupMemoryMount{root: root, point: point, v1: v1}, root != "" && point != "" && path.Clean(point) == point
}

// readLimit maps a membership path relative to the mounted subtree, then
// takes the tightest limit visible on that path, including the mount root.
func (m cgroupMemoryMount) readLimit(memberPath string) (int64, bool) {
	if memberPath == "" || m.point == "" {
		return 0, false
	}
	rel := "."
	if memberPath != m.root {
		// Match whole path components before interpreting any ".." from
		// the cgroup namespace. Only the suffix below this mount is local.
		suffix, ok := strings.CutPrefix(memberPath, strings.TrimSuffix(m.root, "/")+"/")
		if ok && filepath.IsLocal(suffix) && path.Clean(suffix) == suffix {
			rel = suffix
		} else if !isInheritedCgroupAncestor(m.root, memberPath) {
			return 0, false
		}
	}

	limitFile, unlimited := "memory.max", int64(math.MaxInt64)
	if m.v1 {
		limitFile, unlimited = "memory.limit_in_bytes", cgroupV1NoLimit
	}
	lowest := unlimited
	point := filepath.Clean(m.point)
	for dir := filepath.Join(point, rel); ; dir = filepath.Dir(dir) {
		if raw, err := os.ReadFile(filepath.Join(dir, limitFile)); err == nil {
			if v, ok := parseCgroupMemoryValue(string(raw)); ok && v < lowest {
				lowest = v
			}
		}
		if dir == point {
			break
		}
	}
	if lowest == unlimited {
		return 0, false
	}
	return lowest, true
}

// An inherited mount can show a root of "/.." (cgroup_namespaces(7)).
// A root made only of parent components encloses the member if it goes up
// farther than the member path does. The missing intermediate directory
// names prevent mapping the member to disk, so only the mount root applies.
func isInheritedCgroupAncestor(root, member string) bool {
	rootUp, rootRest := splitCgroupNamespaceParents(root)
	memberUp, memberRest := splitCgroupNamespaceParents(member)
	if rootRest != "" || rootUp <= memberUp {
		return false
	}
	// Do not infer ancestry from a malformed member suffix. "/" is valid
	// on its own, but not as a trailing separator after parent components.
	switch memberRest {
	case "":
		return memberUp > 0
	case "/":
		return memberUp == 0
	default:
		return path.Clean(memberRest) == memberRest
	}
}

func splitCgroupNamespaceParents(p string) (up int, rest string) {
	for p == "/.." || strings.HasPrefix(p, "/../") {
		up++
		p = p[len("/.."):]
	}
	return up, p
}

// Preserve the kernel's path verbatim: ".." can be part of the namespace
// coordinates, and a literal "/" is a valid membership or mount root.
func absoluteCgroupPath(s string) string {
	if !strings.HasPrefix(s, "/") {
		return ""
	}
	return s
}

// cgroupV2PathFrom extracts the unified hierarchy entry in /proc/self/cgroup.
func cgroupV2PathFrom(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		if memberPath, ok := strings.CutPrefix(line, "0::"); ok {
			return absoluteCgroupPath(memberPath)
		}
	}
	return ""
}

// cgroupV1MemoryPathFrom extracts the path for the v1 memory controller from
// /proc/self/cgroup. Controllers may be mounted together, so "memory" can be
// one item in a comma-separated controller field.
func cgroupV1MemoryPathFrom(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) == 3 && slices.Contains(strings.Split(fields[1], ","), "memory") {
			return absoluteCgroupPath(fields[2])
		}
	}
	return ""
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
