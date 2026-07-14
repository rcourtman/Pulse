package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// diskPoolAssignment maps a disk's canonical key (lowercase, prefix-stripped) to
// its owning pool. Keys are derived from leaf-device names in ZFS pool trees and
// are consulted with several candidate forms of a physical disk's identity.
type diskPoolAssignment struct {
	keyToPool  map[string]string // normalised leaf name / by-id path → pool name
	serialPool map[string]string // lowercase serial fragment → pool name (for by-id matches)
}

// buildDiskPoolAssignment flattens the ZFS pool trees returned by Proxmox and
// indexes every leaf device under several normalised keys so a physical disk
// can be matched regardless of whether zpool references it by /dev path,
// /dev/disk/by-id path, or partition name.
func buildDiskPoolAssignment(pools []proxmox.ZFSPoolInfo) *diskPoolAssignment {
	assignment := &diskPoolAssignment{
		keyToPool:  make(map[string]string),
		serialPool: make(map[string]string),
	}
	for _, pool := range pools {
		if pool.Name == "" {
			continue
		}
		for _, dev := range pool.Devices {
			assignment.indexLeaves(dev, pool.Name)
		}
	}
	return assignment
}

func (a *diskPoolAssignment) indexLeaves(dev proxmox.ZFSPoolDevice, poolName string) {
	if dev.Leaf == 1 {
		name := strings.TrimSpace(dev.Name)
		if name != "" {
			for _, key := range normaliseLeafKeys(name) {
				if _, exists := a.keyToPool[key]; !exists {
					a.keyToPool[key] = poolName
				}
			}
			if serial := serialFromByID(name); serial != "" {
				if _, exists := a.serialPool[serial]; !exists {
					a.serialPool[serial] = poolName
				}
			}
		}
	}
	for _, child := range dev.Children {
		a.indexLeaves(child, poolName)
	}
}

// lookup returns the pool name a disk belongs to, or "" if no match.
func (a *diskPoolAssignment) lookup(disk models.PhysicalDisk) string {
	if a == nil {
		return ""
	}
	for _, key := range diskLookupKeys(disk) {
		if pool, ok := a.keyToPool[key]; ok {
			return pool
		}
	}
	serial := strings.ToLower(strings.TrimSpace(disk.Serial))
	if serial != "" {
		if pool, ok := a.serialPool[serial]; ok {
			return pool
		}
	}
	wwn := strings.ToLower(strings.TrimSpace(disk.WWN))
	wwn = strings.TrimPrefix(wwn, "0x")
	wwn = strings.TrimPrefix(wwn, "eui.")
	if wwn != "" {
		if pool, ok := a.serialPool[wwn]; ok {
			return pool
		}
	}
	return ""
}

// normaliseLeafKeys derives candidate lookup keys from a zpool leaf-device name.
// Keys are lowercased and stripped of common prefixes so they can match against
// physical-disk DevPath variants.
func normaliseLeafKeys(raw string) []string {
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

	if base := stripPartitionSuffix(trimmed); base != "" {
		keys[base] = struct{}{}
	}

	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	return out
}

// diskLookupKeys derives candidate lookup keys from a physical disk's
// identifying fields. Order matters: the most specific matches come first.
func diskLookupKeys(disk models.PhysicalDisk) []string {
	devPath := strings.ToLower(strings.TrimSpace(disk.DevPath))
	if devPath == "" {
		return nil
	}
	seen := map[string]struct{}{}
	add := func(k string) {
		if k == "" {
			return
		}
		seen[k] = struct{}{}
	}

	add(devPath)
	add(strings.TrimPrefix(devPath, "/dev/"))
	base := strings.TrimPrefix(devPath, "/dev/")
	add(stripPartitionSuffix(base))

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

// stripPartitionSuffix removes a trailing partition identifier. Handles both
// sdX style (trailing digits after an all-alphabetic prefix) and the
// p-separator style used by devices whose name already contains digits
// (nvme0n1p3, mmcblk0p1).
func stripPartitionSuffix(name string) string {
	if name == "" {
		return ""
	}
	// p-separator form: the base name already contains a digit, and the
	// partition is appended after a literal "p". nvme0n1p3 → nvme0n1.
	if idx := strings.LastIndex(name, "p"); idx > 0 {
		suffix := name[idx+1:]
		if suffix != "" && allDigits(suffix) {
			prev := name[:idx]
			if len(prev) > 0 && isDigit(prev[len(prev)-1]) {
				return prev
			}
		}
	}
	// sdX / hdX form: strip trailing digits only when the remaining prefix
	// is all alphabetic. This avoids over-stripping device names that
	// carry digits as part of their identifier (nvme0n1, mmcblk0).
	i := len(name)
	for i > 0 && isDigit(name[i-1]) {
		i--
	}
	if i == len(name) || i == 0 {
		return name
	}
	prefix := name[:i]
	for j := 0; j < len(prefix); j++ {
		if !isAlpha(prefix[j]) {
			return name
		}
	}
	return prefix
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// serialFromByID extracts a lowercase serial-like fragment from a by-id path.
// Proxmox commonly references ZFS members via /dev/disk/by-id/ata-MODEL_SERIAL
// or nvme-MODEL_SERIAL; the trailing token is the disk serial.
func serialFromByID(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	name = strings.TrimPrefix(name, "/dev/")
	name = strings.TrimPrefix(name, "disk/by-id/")
	if !(strings.HasPrefix(name, "ata-") ||
		strings.HasPrefix(name, "scsi-") ||
		strings.HasPrefix(name, "nvme-") ||
		strings.HasPrefix(name, "wwn-")) {
		return ""
	}
	// Strip optional partition suffix like "-part1"
	if idx := strings.LastIndex(name, "-part"); idx > 0 {
		name = name[:idx]
	}
	// NVMe pools are often built from nvme-eui.<hex> references (the
	// installer's pick when identical models share a box); the hex is the
	// device's WWN/EUI, which smartctl reports as "eui.<hex>" or "0x<hex>".
	if strings.HasPrefix(name, "nvme-eui.") {
		return strings.TrimPrefix(name, "nvme-eui.")
	}
	// The last underscore-separated token is typically the serial.
	if idx := strings.LastIndex(name, "_"); idx > 0 {
		return name[idx+1:]
	}
	if strings.HasPrefix(name, "wwn-0x") {
		return strings.TrimPrefix(name, "wwn-0x")
	}
	if strings.HasPrefix(name, "wwn-") {
		return strings.TrimPrefix(name, "wwn-")
	}
	return ""
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return len(s) > 0
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
