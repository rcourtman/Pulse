package monitoring

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func cloneGuestDisks(src []models.Disk) []models.Disk {
	if len(src) == 0 {
		return nil
	}
	return append([]models.Disk(nil), src...)
}

func classifyGuestAgentDiskStatusError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()
	errStrLower := strings.ToLower(errStr)

	switch {
	case strings.Contains(errStr, "QEMU guest agent is not running"):
		return "agent-not-running"
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		return "agent-timeout"
	case strings.Contains(errStr, "500") && (strings.Contains(errStr, "not running") || strings.Contains(errStr, "not available")):
		return "agent-not-running"
	case (strings.Contains(errStr, "403") || strings.Contains(errStr, "401")) &&
		(strings.Contains(errStrLower, "permission") || strings.Contains(errStrLower, "forbidden") || strings.Contains(errStrLower, "not allowed")):
		return "permission-denied"
	case strings.Contains(errStr, "500"):
		return "agent-not-running"
	default:
		return "agent-error"
	}
}

func shouldCarryForwardQEMUDisk(reason string) bool {
	switch reason {
	case "", "vm-stopped", "agent-disabled", "no-agent":
		return false
	default:
		return true
	}
}

func stabilizeGuestLowTrustDisk(
	prev *models.VM,
	status string,
	diskTotal uint64,
	diskUsed uint64,
	diskFree uint64,
	diskUsage float64,
	individualDisks []models.Disk,
	diskStatusReason string,
	diskFromAgent bool,
	now time.Time,
) (uint64, uint64, uint64, float64, []models.Disk, string) {
	if status != "running" || diskFromAgent || !shouldCarryForwardQEMUDisk(diskStatusReason) {
		return diskTotal, diskUsed, diskFree, diskUsage, individualDisks, diskStatusReason
	}
	if prev == nil || prev.Type != "qemu" || !hasRecentGuestAgentEvidence(prev, now) {
		return diskTotal, diskUsed, diskFree, diskUsage, individualDisks, diskStatusReason
	}
	if strings.HasPrefix(prev.DiskStatusReason, "prev-") {
		return diskTotal, diskUsed, diskFree, diskUsage, individualDisks, diskStatusReason
	}
	if prev.Disk.Total <= 0 || prev.Disk.Used < 0 || prev.Disk.Used > prev.Disk.Total || prev.Disk.Usage < 0 {
		return diskTotal, diskUsed, diskFree, diskUsage, individualDisks, diskStatusReason
	}

	total := uint64(prev.Disk.Total)
	used := uint64(prev.Disk.Used)
	free := total - used
	if prev.Disk.Free >= 0 && prev.Disk.Free <= prev.Disk.Total {
		free = uint64(prev.Disk.Free)
	}

	return total, used, free, prev.Disk.Usage, cloneGuestDisks(prev.Disks), "prev-" + diskStatusReason
}
