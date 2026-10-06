package monitoring

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"

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

	if reason := proxmox.GuestAgentErrorReason(err); reason != "" {
		return reason
	}

	// Non-Proxmox client implementations may return untyped local errors.
	// Never infer HTTP status or a stopped agent from numbers/body substrings.
	switch {
	case err.Error() == "QEMU guest agent is not running":
		return "agent-not-running"
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") || strings.Contains(strings.ToLower(err.Error()), "deadline exceeded"):
		return "agent-timeout"
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
