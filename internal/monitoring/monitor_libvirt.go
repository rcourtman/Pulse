package monitoring

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const (
	maxAgentLibvirtDomains         = 128
	maxAgentLibvirtDomainNameBytes = 256
)

func (m *Monitor) normalizeAgentLibvirtInventory(
	report *agentshost.LibvirtInventory,
	hostID string,
	previous *models.Host,
	observedAt time.Time,
) *models.HostLibvirtInventory {
	if report == nil {
		if previous != nil && previous.Libvirt != nil {
			preserved := *previous.Libvirt
			preserved.Domains = append([]models.HostLibvirtDomain(nil), previous.Libvirt.Domains...)
			return &preserved
		}
		return nil
	}

	previousByID := make(map[string]models.HostLibvirtDomain)
	previousAt := time.Time{}
	if previous != nil {
		previousAt = previous.LastSeen
		if previous.Libvirt != nil {
			for _, domain := range previous.Libvirt.Domains {
				previousByID[domain.ID] = domain
			}
		}
	}

	result := &models.HostLibvirtInventory{
		Domains:     make([]models.HostLibvirtDomain, 0, min(len(report.Domains), maxAgentLibvirtDomains)),
		CollectedAt: observedAt.UTC(),
	}
	seen := make(map[string]struct{}, len(report.Domains))
	for _, incoming := range report.Domains {
		if len(result.Domains) == maxAgentLibvirtDomains {
			break
		}
		name := strings.TrimSpace(incoming.Name)
		if !validAgentLibvirtDomainName(name) {
			continue
		}
		dedupeKey := strings.ToLower(name)
		if _, exists := seen[dedupeKey]; exists {
			continue
		}
		seen[dedupeKey] = struct{}{}

		domain := models.HostLibvirtDomain{
			ID:                 agentLibvirtDomainID(name),
			Name:               name,
			State:              normalizeAgentLibvirtState(incoming.State),
			VCPUs:              max(0, min(incoming.VCPUs, 4096)),
			CPUTimeNanoseconds: incoming.CPUTimeNanoseconds,
			MemoryCurrentBytes: max(int64(0), incoming.MemoryCurrentBytes),
			MemoryMaximumBytes: max(int64(0), incoming.MemoryMaximumBytes),
			NetworkRXBytes:     incoming.NetworkRXBytes,
			NetworkTXBytes:     incoming.NetworkTXBytes,
			DiskReadBytes:      incoming.DiskReadBytes,
			DiskWriteBytes:     incoming.DiskWriteBytes,
		}
		if domain.MemoryMaximumBytes > 0 && domain.MemoryCurrentBytes > domain.MemoryMaximumBytes {
			domain.MemoryCurrentBytes = domain.MemoryMaximumBytes
		}

		if prior, ok := previousByID[domain.ID]; ok {
			domain.CPUUsagePercent, domain.CPUUsageValid = libvirtCPUUsagePercent(
				prior,
				domain,
				observedAt.Sub(previousAt),
			)
		}

		if m != nil && m.rateTracker != nil {
			diskRead, diskWrite, netIn, netOut := m.rateTracker.CalculateRates(
				"libvirt:"+strings.TrimSpace(hostID)+":"+domain.ID,
				IOMetrics{
					DiskRead:   uint64ToInt64(domain.DiskReadBytes),
					DiskWrite:  uint64ToInt64(domain.DiskWriteBytes),
					NetworkIn:  uint64ToInt64(domain.NetworkRXBytes),
					NetworkOut: uint64ToInt64(domain.NetworkTXBytes),
					Timestamp:  observedAt,
				},
			)
			if diskRead >= 0 && diskWrite >= 0 && netIn >= 0 && netOut >= 0 {
				domain.DiskReadRate = diskRead
				domain.DiskWriteRate = diskWrite
				domain.NetworkInRate = netIn
				domain.NetworkOutRate = netOut
				domain.IORatesValid = true
			}
		}

		result.Domains = append(result.Domains, domain)
	}
	sort.Slice(result.Domains, func(i, j int) bool {
		return strings.ToLower(result.Domains[i].Name) < strings.ToLower(result.Domains[j].Name)
	})
	return result
}

func validAgentLibvirtDomainName(name string) bool {
	if name == "" || len(name) > maxAgentLibvirtDomainNameBytes || !utf8.ValidString(name) {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func normalizeAgentLibvirtState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "running", "blocked", "paused", "shutting-down", "shutoff", "crashed", "suspended":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

func agentLibvirtDomainID(name string) string {
	sum := sha256.Sum256([]byte(name))
	return "domain-" + hex.EncodeToString(sum[:8])
}

func libvirtCPUUsagePercent(previous, current models.HostLibvirtDomain, elapsed time.Duration) (float64, bool) {
	if elapsed <= 0 || current.CPUTimeNanoseconds < previous.CPUTimeNanoseconds {
		return 0, false
	}
	vcpus := current.VCPUs
	if vcpus <= 0 {
		vcpus = 1
	}
	delta := current.CPUTimeNanoseconds - previous.CPUTimeNanoseconds
	percent := (float64(delta) / float64(elapsed.Nanoseconds()) / float64(vcpus)) * 100
	if math.IsNaN(percent) || math.IsInf(percent, 0) {
		return 0, false
	}
	return math.Max(0, math.Min(100, percent)), true
}

func uint64ToInt64(value uint64) int64 {
	if value > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(value)
}

func libvirtDomainMetricID(hostID, domainID string) string {
	return strings.TrimSpace(hostID) + ":libvirt:" + strings.TrimSpace(domainID)
}

func (m *Monitor) writeLibvirtDomainMetrics(host models.Host, observedAt time.Time) {
	if m == nil || host.Libvirt == nil {
		return
	}
	for _, domain := range host.Libvirt.Domains {
		metricID := libvirtDomainMetricID(host.ID, domain.ID)
		if domain.CPUUsageValid {
			if m.metricsHistory != nil {
				m.metricsHistory.AddGuestMetric(metricID, "cpu", domain.CPUUsagePercent, observedAt)
			}
			if m.metricsStore != nil {
				m.metricsStore.Write("vm", metricID, "cpu", domain.CPUUsagePercent, observedAt)
			}
		}
		if domain.MemoryMaximumBytes > 0 {
			memoryPercent := math.Max(0, math.Min(
				100,
				(float64(domain.MemoryCurrentBytes)/float64(domain.MemoryMaximumBytes))*100,
			))
			if m.metricsHistory != nil {
				m.metricsHistory.AddGuestMetric(metricID, "memory", memoryPercent, observedAt)
			}
			if m.metricsStore != nil {
				m.metricsStore.Write("vm", metricID, "memory", memoryPercent, observedAt)
			}
		}
		if !domain.IORatesValid {
			continue
		}
		for metricType, value := range map[string]float64{
			"netin":     domain.NetworkInRate,
			"netout":    domain.NetworkOutRate,
			"diskread":  domain.DiskReadRate,
			"diskwrite": domain.DiskWriteRate,
		} {
			if m.metricsHistory != nil {
				m.metricsHistory.AddGuestMetric(metricID, metricType, value, observedAt)
			}
			if m.metricsStore != nil {
				m.metricsStore.Write("vm", metricID, metricType, value, observedAt)
			}
		}
	}
}
