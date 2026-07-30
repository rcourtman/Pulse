package monitoring

import (
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const maxAgentXCPNGVMs = 1024

func normalizeAgentXCPNGInventory(
	report *agentshost.XCPNGInventory,
	previous *models.Host,
	observedAt time.Time,
) *models.HostXCPNGInventory {
	if report == nil {
		if previous != nil && previous.XCPNG != nil {
			preserved := *previous.XCPNG
			preserved.VMs = append([]models.HostXCPNGVM(nil), previous.XCPNG.VMs...)
			return &preserved
		}
		return nil
	}

	poolUUID := canonicalAgentXCPNGUUID(report.PoolUUID)
	if poolUUID == "" {
		return nil
	}
	result := &models.HostXCPNGInventory{
		PoolUUID:      poolUUID,
		PoolName:      safeAgentXCPNGName(report.PoolName),
		MasterUUID:    canonicalAgentXCPNGUUID(report.MasterUUID),
		LocalHostUUID: canonicalAgentXCPNGUUID(report.LocalHostUUID),
		VMs:           make([]models.HostXCPNGVM, 0, min(len(report.VMs), maxAgentXCPNGVMs)),
		CollectedAt:   observedAt.UTC(),
	}
	seen := make(map[string]struct{}, len(report.VMs))
	for _, incoming := range report.VMs {
		if len(result.VMs) == maxAgentXCPNGVMs {
			break
		}
		uuid := canonicalAgentXCPNGUUID(incoming.UUID)
		name := safeAgentXCPNGName(incoming.Name)
		if uuid == "" || name == "" {
			continue
		}
		if _, exists := seen[uuid]; exists {
			continue
		}
		seen[uuid] = struct{}{}
		result.VMs = append(result.VMs, models.HostXCPNGVM{
			UUID:             uuid,
			Name:             name,
			PowerState:       normalizeAgentXCPNGPowerState(incoming.PowerState),
			VCPUs:            max(0, min(incoming.VCPUs, 4096)),
			MemoryActual:     max(int64(0), incoming.MemoryActual),
			MemoryStaticMax:  max(int64(0), incoming.MemoryStaticMax),
			ResidentHostUUID: canonicalAgentXCPNGUUID(incoming.ResidentHostUUID),
		})
		last := &result.VMs[len(result.VMs)-1]
		if last.MemoryStaticMax > 0 && last.MemoryActual > last.MemoryStaticMax {
			last.MemoryActual = last.MemoryStaticMax
		}
	}
	sort.Slice(result.VMs, func(i, j int) bool {
		return strings.ToLower(result.VMs[i].Name) < strings.ToLower(result.VMs[j].Name)
	})
	return result
}

func canonicalAgentXCPNGUUID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return ""
	}
	for i, r := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return ""
		}
	}
	return value
}

func safeAgentXCPNGName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 256 {
		return ""
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	return value
}

func normalizeAgentXCPNGPowerState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "running", "halted", "paused", "suspended":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}
