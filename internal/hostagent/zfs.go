package hostagent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

var (
	zpoolLookPath = exec.LookPath
	zpoolStat     = os.Stat
	zpoolRun      = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		return cmd.Output()
	}
)

var commonZpoolPaths = []string{
	"/usr/sbin/zpool",
	"/sbin/zpool",
	"/usr/local/sbin/zpool",
	"/usr/bin/zpool",
	"/bin/zpool",
}

// ZFSDiskPoolMap returns a map from a pool-member device identifier (as
// reported by `zpool status -P`) to the name of the pool it belongs to.
// Keys are inserted in multiple normalized forms so callers can match
// against /dev paths, bare device names, by-id paths, or partition names.
// Returns an empty map if zpool is not installed or no pools are present.
func ZFSDiskPoolMap(ctx context.Context) (map[string]string, error) {
	zpoolPath, err := resolveZpoolPath()
	if err != nil {
		return map[string]string{}, nil
	}
	listCtx, cancelList := context.WithTimeout(ctx, 3*time.Second)
	defer cancelList()

	listOut, err := zpoolRun(listCtx, zpoolPath, "list", "-H", "-o", "name")
	if err != nil {
		return nil, fmt.Errorf("zpool list: %w", err)
	}
	result := make(map[string]string)
	for _, line := range strings.Split(string(listOut), "\n") {
		pool := strings.TrimSpace(line)
		if pool == "" {
			continue
		}
		members, err := collectZpoolMembers(ctx, zpoolPath, pool)
		if err != nil {
			continue
		}
		for _, member := range members {
			for _, key := range normalizeZFSMemberKeys(member) {
				if _, ok := result[key]; !ok {
					result[key] = pool
				}
			}
		}
	}
	return result, nil
}

func collectZpoolMembers(ctx context.Context, zpoolPath, pool string) ([]string, error) {
	detailCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := zpoolRun(detailCtx, zpoolPath, "status", "-P", pool)
	if err != nil {
		return nil, fmt.Errorf("zpool status -P %s: %w", pool, err)
	}
	return parseZpoolStatusMembers(pool, string(out)), nil
}

// parseZpoolStatusMembers pulls leaf device names from `zpool status -P`
// output. The config block looks like:
//
//	config:
//	    NAME        STATE     READ WRITE CKSUM
//	    tank        ONLINE       0     0     0
//	      mirror-0  ONLINE       0     0     0
//	        /dev/sda3              ONLINE       0     0     0
//	        /dev/disk/by-id/ata-... ONLINE       0     0     0
//	    logs
//	      /dev/nvme0n1p1           ONLINE       0     0     0
//
// We keep every token that doesn't match a known non-leaf keyword and
// isn't the pool name itself.
func parseZpoolStatusMembers(pool, output string) []string {
	var members []string
	seen := map[string]struct{}{}
	inConfig := false
	for _, raw := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "config:") {
			inConfig = true
			continue
		}
		if strings.HasPrefix(lower, "errors:") ||
			strings.HasPrefix(lower, "pool:") ||
			strings.HasPrefix(lower, "state:") ||
			strings.HasPrefix(lower, "scan:") ||
			strings.HasPrefix(lower, "status:") ||
			strings.HasPrefix(lower, "action:") ||
			strings.HasPrefix(lower, "see:") {
			inConfig = false
			continue
		}
		if !inConfig {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if name == "NAME" || name == pool {
			continue
		}
		if isZFSVdevKeyword(name) {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		members = append(members, name)
	}
	return members
}

// annotateSMARTWithZFSPools stamps each SMART entry's Pool field when a
// matching leaf device is found in the supplied pool map. Entries with a
// non-empty Pool are left untouched so callers can pre-populate from other
// sources (e.g. Unraid topology) without being overwritten.
func annotateSMARTWithZFSPools(smartData []agentshost.DiskSMART, pools map[string]string) {
	if len(pools) == 0 || len(smartData) == 0 {
		return
	}
	for i := range smartData {
		if smartData[i].Pool != "" {
			continue
		}
		if pool := poolForSMARTEntry(pools, smartData[i]); pool != "" {
			smartData[i].Pool = pool
		}
	}
}

func poolForSMARTEntry(pools map[string]string, entry agentshost.DiskSMART) string {
	seen := map[string]struct{}{}
	try := func(key string) string {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			return ""
		}
		if _, ok := seen[key]; ok {
			return ""
		}
		seen[key] = struct{}{}
		if pool, ok := pools[key]; ok {
			return pool
		}
		return ""
	}
	for _, key := range normalizeZFSMemberKeys(entry.Device) {
		if pool := try(key); pool != "" {
			return pool
		}
	}
	if entry.Serial != "" {
		if pool := try(entry.Serial); pool != "" {
			return pool
		}
	}
	if entry.WWN != "" {
		wwn := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(entry.WWN)), "0x")
		wwn = strings.TrimPrefix(wwn, "eui.")
		if pool := try(wwn); pool != "" {
			return pool
		}
	}
	return ""
}

func isZFSVdevKeyword(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "logs", "log", "cache", "spares", "spare", "special", "dedup":
		return true
	}
	if strings.HasPrefix(lower, "mirror") ||
		strings.HasPrefix(lower, "raidz") ||
		strings.HasPrefix(lower, "draid") {
		return true
	}
	return false
}

// normalizeZFSMemberKeys derives candidate map keys from a leaf-device name
// so that a caller with a plain /dev/sda, a by-id path, or a bare "sda"
// can all find the pool.
func normalizeZFSMemberKeys(raw string) []string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" {
		return nil
	}
	keys := map[string]struct{}{name: {}}
	trimmed := strings.TrimPrefix(name, "/dev/")
	trimmed = strings.TrimPrefix(trimmed, "disk/by-id/")
	trimmed = strings.TrimPrefix(trimmed, "disk/by-path/")
	trimmed = strings.TrimPrefix(trimmed, "disk/by-uuid/")
	if trimmed != "" {
		keys[trimmed] = struct{}{}
	}
	if base := stripZFSPartitionSuffix(trimmed); base != "" && base != trimmed {
		keys[base] = struct{}{}
	}
	if strings.HasPrefix(trimmed, "ata-") ||
		strings.HasPrefix(trimmed, "scsi-") ||
		strings.HasPrefix(trimmed, "nvme-") ||
		strings.HasPrefix(trimmed, "wwn-") {
		if serial := zfsSerialFromByID(trimmed); serial != "" {
			keys[serial] = struct{}{}
		}
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	return out
}

func stripZFSPartitionSuffix(name string) string {
	if name == "" {
		return ""
	}
	if strings.HasSuffix(name, "-part") {
		return name
	}
	if idx := strings.LastIndex(name, "-part"); idx > 0 {
		suffix := name[idx+len("-part"):]
		if allZFSDigits(suffix) {
			return name[:idx]
		}
	}
	if idx := strings.LastIndex(name, "p"); idx > 0 {
		suffix := name[idx+1:]
		if suffix != "" && allZFSDigits(suffix) {
			prev := name[:idx]
			if len(prev) > 0 && isZFSDigit(prev[len(prev)-1]) {
				return prev
			}
		}
	}
	i := len(name)
	for i > 0 && isZFSDigit(name[i-1]) {
		i--
	}
	if i == len(name) || i == 0 {
		return name
	}
	prefix := name[:i]
	for j := 0; j < len(prefix); j++ {
		c := prefix[j]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return name
		}
	}
	return prefix
}

func zfsSerialFromByID(name string) string {
	n := strings.TrimSpace(name)
	if idx := strings.LastIndex(n, "-part"); idx > 0 {
		n = n[:idx]
	}
	// nvme-eui.<hex> references carry the device's WWN/EUI rather than a
	// serial; smartctl reports the same value as "eui.<hex>" or "0x<hex>".
	if strings.HasPrefix(n, "nvme-eui.") {
		return strings.TrimPrefix(n, "nvme-eui.")
	}
	// systemd's nvme by-id links can carry a trailing _<n> namespace suffix
	// (nvme-MODEL_SERIAL_1); the serial is the token before it (#1540).
	n = stripZFSNVMeNamespaceSuffix(n)
	if idx := strings.LastIndex(n, "_"); idx > 0 {
		return n[idx+1:]
	}
	if strings.HasPrefix(n, "wwn-0x") {
		return strings.TrimPrefix(n, "wwn-0x")
	}
	if strings.HasPrefix(n, "wwn-") {
		return strings.TrimPrefix(n, "wwn-")
	}
	return ""
}

// stripZFSNVMeNamespaceSuffix removes the trailing _<n> namespace token
// systemd appends to nvme by-id link names (nvme-MODEL_SERIAL_1). Only short
// all-digit tokens are stripped, and only when another underscore token
// remains, so a genuine serial is never truncated.
func stripZFSNVMeNamespaceSuffix(name string) string {
	if !strings.HasPrefix(name, "nvme-") {
		return name
	}
	idx := strings.LastIndex(name, "_")
	if idx <= 0 {
		return name
	}
	suffix := name[idx+1:]
	if len(suffix) == 0 || len(suffix) > 3 || !allZFSDigits(suffix) {
		return name
	}
	if !strings.Contains(name[:idx], "_") {
		return name
	}
	return name[:idx]
}

func allZFSDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isZFSDigit(s[i]) {
			return false
		}
	}
	return len(s) > 0
}

func isZFSDigit(b byte) bool { return b >= '0' && b <= '9' }

// resolveZpoolPath mirrors the mdadm path-resolution pattern: prefer common
// absolute paths, then fall back to PATH.
func resolveZpoolPath() (string, error) {
	for _, candidate := range commonZpoolPaths {
		candidate = filepath.Clean(candidate)
		if !filepath.IsAbs(candidate) {
			continue
		}
		if _, err := zpoolStat(candidate); err == nil {
			return candidate, nil
		}
	}
	path, err := zpoolLookPath("zpool")
	if err != nil {
		return "", fmt.Errorf("zpool binary not found in PATH or common locations")
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("zpool path is not absolute: %q", path)
	}
	if _, err := zpoolStat(path); err != nil {
		return "", fmt.Errorf("zpool path unavailable: %w", err)
	}
	return path, nil
}
