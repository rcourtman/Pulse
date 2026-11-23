package mock

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

const mockGiB = int64(1) << 30

type mockLibvirtDomainSpec struct {
	id         string
	name       string
	state      string
	vcpus      int
	memoryGiB  int64
	staticMemP float64
}

var (
	mockPrimaryLibvirtDomainSpecs = []mockLibvirtDomainSpec{
		{id: "dom-kvm-app-prod-01", name: "kvm-app-prod-01", state: "running", vcpus: 4, memoryGiB: 8},
		{id: "dom-kvm-postgres-01", name: "kvm-postgres-01", state: "running", vcpus: 8, memoryGiB: 16},
		{id: "dom-kvm-build-runner", name: "kvm-build-runner", state: "paused", vcpus: 4, memoryGiB: 8, staticMemP: 38},
		{id: "dom-kvm-win11-lab", name: "kvm-win11-lab", state: "shutoff", vcpus: 4, memoryGiB: 16},
	}
	mockSecondaryLibvirtDomainSpecs = []mockLibvirtDomainSpec{
		{id: "dom-kvm-media-vault", name: "kvm-media-vault", state: "running", vcpus: 4, memoryGiB: 12},
		{id: "dom-kvm-dns-edge", name: "kvm-dns-edge", state: "running", vcpus: 2, memoryGiB: 4},
		{id: "dom-kvm-legacy-erp", name: "kvm-legacy-erp", state: "crashed", vcpus: 4, memoryGiB: 8, staticMemP: 44},
	}
)

func mockLibvirtDomainMetricID(hostID, domainID string) string {
	return strings.TrimSpace(hostID) + ":libvirt:" + strings.TrimSpace(domainID)
}

func attachMockStandaloneVMInventories(hosts []models.Host, now time.Time) {
	if len(hosts) == 0 {
		return
	}

	linuxIndices := make([]int, 0, len(hosts))
	for i := range hosts {
		if strings.TrimSpace(hosts[i].LinkedNodeID) != "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(hosts[i].Platform), "linux") {
			linuxIndices = append(linuxIndices, i)
		}
	}
	if len(linuxIndices) == 0 {
		return
	}

	populateHostLibvirt := func(host *models.Host, specs []mockLibvirtDomainSpec) {
		collectedAt := host.LastSeen
		if collectedAt.IsZero() {
			collectedAt = now
		}
		domains := make([]models.HostLibvirtDomain, 0, len(specs))
		for _, spec := range specs {
			domains = append(domains, buildMockLibvirtDomain(host.ID, spec, now))
		}
		host.Libvirt = &models.HostLibvirtInventory{
			Domains:     domains,
			CollectedAt: collectedAt,
		}
	}

	populateHostLibvirt(&hosts[linuxIndices[0]], mockPrimaryLibvirtDomainSpecs)

	if len(linuxIndices) >= 2 {
		populateHostLibvirt(&hosts[linuxIndices[1]], mockSecondaryLibvirtDomainSpecs)
	}

	if len(linuxIndices) >= 3 {
		xcpHost := &hosts[linuxIndices[2]]
		collectedAt := xcpHost.LastSeen
		if collectedAt.IsZero() {
			collectedAt = now
		}
		xcpHost.OSName = "XCP-ng"
		xcpHost.OSVersion = "8.3"
		xcpHost.KernelVersion = "4.19.0+1"
		xcpHost.XCPNG = &models.HostXCPNGInventory{
			PoolUUID:      "pool-xcp-lab-01",
			PoolName:      "xcp-lab-pool",
			MasterUUID:    "xcp-host-01",
			LocalHostUUID: "xcp-host-01",
			VMs: []models.HostXCPNGVM{
				{
					UUID:             "xcp-vm-gateway-01",
					Name:             "xcp-gateway-01",
					PowerState:       "Running",
					VCPUs:            4,
					MemoryActual:     6 * mockGiB,
					MemoryStaticMax:  8 * mockGiB,
					ResidentHostUUID: "xcp-host-01",
				},
				{
					UUID:             "xcp-vm-staging-db",
					Name:             "xcp-staging-db",
					PowerState:       "Halted",
					VCPUs:            4,
					MemoryActual:     0,
					MemoryStaticMax:  16 * mockGiB,
					ResidentHostUUID: "xcp-host-01",
				},
			},
			CollectedAt: collectedAt,
		}
	}
}

func buildMockLibvirtDomain(hostID string, spec mockLibvirtDomainSpec, now time.Time) models.HostLibvirtDomain {
	maxBytes := spec.memoryGiB * mockGiB
	domain := models.HostLibvirtDomain{
		ID:                 spec.id,
		Name:               spec.name,
		State:              spec.state,
		VCPUs:              spec.vcpus,
		MemoryMaximumBytes: maxBytes,
	}

	switch strings.ToLower(strings.TrimSpace(spec.state)) {
	case "running":
		domain.MemoryCurrentBytes = maxBytes
		domain.CPUTimeNanoseconds = uint64(3600+mockStableChoice(40000, hostID, spec.id, "cpu-time")) * 1_000_000_000
		domain.NetworkRXBytes = uint64(512+mockStableChoice(8192, hostID, spec.id, "net-rx")) * 1024 * 1024
		domain.NetworkTXBytes = uint64(256+mockStableChoice(4096, hostID, spec.id, "net-tx")) * 1024 * 1024
		domain.DiskReadBytes = uint64(1024+mockStableChoice(16384, hostID, spec.id, "disk-read")) * 1024 * 1024
		domain.DiskWriteBytes = uint64(512+mockStableChoice(8192, hostID, spec.id, "disk-write")) * 1024 * 1024
		refreshMockLibvirtDomainMetrics(hostID, &domain, now, 0)
	case "paused", "crashed":
		availBytes := maxBytes
		memPct := spec.staticMemP
		if memPct <= 0 {
			memPct = 40
		}
		usedBytes := int64(float64(availBytes) * (memPct / 100.0))
		domain.MemoryCurrentBytes = maxBytes
		domain.MemoryAvailableBytes = availBytes
		domain.MemoryUsableBytes = max(int64(0), availBytes-usedBytes)
		domain.CPUUsagePercent = 0
		domain.CPUUsageValid = true
		domain.IORatesValid = true
	case "shutoff":
		domain.MemoryCurrentBytes = 0
		domain.CPUUsageValid = false
		domain.IORatesValid = false
	}

	return domain
}

func refreshMockLibvirtDomainMetrics(hostID string, domain *models.HostLibvirtDomain, now time.Time, stepSeconds int64) {
	if domain == nil || !strings.EqualFold(strings.TrimSpace(domain.State), "running") {
		return
	}
	metricID := mockLibvirtDomainMetricID(hostID, domain.ID)
	cpuPct := SampleMetric("vm", metricID, "cpu", now)
	memPct := SampleMetric("vm", metricID, "memory", now)
	netIn := SampleMetric("vm", metricID, "netin", now)
	netOut := SampleMetric("vm", metricID, "netout", now)
	diskRead := SampleMetric("vm", metricID, "diskread", now)
	diskWrite := SampleMetric("vm", metricID, "diskwrite", now)

	domain.CPUUsagePercent = cpuPct
	domain.CPUUsageValid = true
	domain.NetworkInRate = netIn
	domain.NetworkOutRate = netOut
	domain.DiskReadRate = diskRead
	domain.DiskWriteRate = diskWrite
	domain.IORatesValid = true

	if domain.MemoryMaximumBytes > 0 {
		availBytes := domain.MemoryMaximumBytes
		usedBytes := int64(float64(availBytes) * (memPct / 100.0))
		domain.MemoryCurrentBytes = domain.MemoryMaximumBytes
		domain.MemoryAvailableBytes = availBytes
		domain.MemoryUsableBytes = max(int64(0), availBytes-usedBytes)
	}

	if stepSeconds > 0 {
		vcpus := max(1, domain.VCPUs)
		domain.CPUTimeNanoseconds += uint64((cpuPct / 100.0) * float64(vcpus) * float64(stepSeconds) * 1e9)
		domain.NetworkRXBytes += uint64(max(0, netIn) * float64(stepSeconds))
		domain.NetworkTXBytes += uint64(max(0, netOut) * float64(stepSeconds))
		domain.DiskReadBytes += uint64(max(0, diskRead) * float64(stepSeconds))
		domain.DiskWriteBytes += uint64(max(0, diskWrite) * float64(stepSeconds))
	}
}

func updateMockHostStandaloneVMs(host *models.Host, now time.Time, stepSeconds int64, refreshMetrics bool) {
	if host == nil || strings.EqualFold(strings.TrimSpace(host.Status), "offline") {
		return
	}
	collectedAt := host.LastSeen
	if collectedAt.IsZero() {
		collectedAt = now
	}
	if host.Libvirt != nil {
		host.Libvirt.CollectedAt = collectedAt
		if refreshMetrics {
			for i := range host.Libvirt.Domains {
				refreshMockLibvirtDomainMetrics(host.ID, &host.Libvirt.Domains[i], now, stepSeconds)
			}
		}
	}
	if host.XCPNG != nil {
		host.XCPNG.CollectedAt = collectedAt
	}
}

func rebaseMockHostStandaloneVMTimestamps(host *models.Host, now time.Time) {
	if host == nil || now.IsZero() || strings.EqualFold(strings.TrimSpace(host.Status), "offline") {
		return
	}
	collectedAt := host.LastSeen
	if collectedAt.IsZero() {
		collectedAt = now
	}
	if host.Libvirt != nil {
		host.Libvirt.CollectedAt = collectedAt
	}
	if host.XCPNG != nil {
		host.XCPNG.CollectedAt = collectedAt
	}
}
