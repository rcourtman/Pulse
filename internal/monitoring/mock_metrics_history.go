package monitoring

import (
	"context"
	"hash/fnv"
	"math"
	"math/rand"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rs/zerolog/log"
)

const (
	defaultMockSeedDuration   = 90 * 24 * time.Hour
	defaultMockSampleInterval = 1 * time.Minute // 1m for detailed recent charts
)

type mockMetricsSamplerConfig struct {
	SeedDuration   time.Duration
	SampleInterval time.Duration
}

func mockMetricsSamplerConfigFromEnv() mockMetricsSamplerConfig {
	seedDuration := parseDurationEnv("PULSE_MOCK_TRENDS_SEED_DURATION", defaultMockSeedDuration)
	sampleInterval := parseDurationEnv("PULSE_MOCK_TRENDS_SAMPLE_INTERVAL", defaultMockSampleInterval)

	// Guardrails to keep memory and CPU bounded in demo mode.
	if seedDuration < 5*time.Minute {
		seedDuration = 5 * time.Minute
	}
	if seedDuration > 90*24*time.Hour {
		seedDuration = 90 * 24 * time.Hour
	}
	if sampleInterval < 5*time.Second {
		sampleInterval = 5 * time.Second
	}
	if sampleInterval > 5*time.Minute {
		sampleInterval = 5 * time.Minute
	}

	// Ensure we can generate at least 2 points.
	if seedDuration < sampleInterval {
		seedDuration = sampleInterval
	}

	return mockMetricsSamplerConfig{
		SeedDuration:   seedDuration,
		SampleInterval: sampleInterval,
	}
}

func hashSeed(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

type seededTrendClass int

const (
	trendStable seededTrendClass = iota
	trendGrowing
	trendDeclining
	trendVolatile
)

func pickTrendClass(seed uint64) seededTrendClass {
	switch seed % 4 {
	case 0:
		return trendStable
	case 1:
		return trendGrowing
	case 2:
		return trendDeclining
	default:
		return trendVolatile
	}
}

func generateSeededSeries(current float64, points int, seed uint64, min, max float64) []float64 {
	current = clampFloat(current, min, max)
	if points <= 1 {
		return []float64{current}
	}

	class := pickTrendClass(seed)
	rng := rand.New(rand.NewSource(int64(seed))) // Deterministic per resource/metric
	span := math.Max(1, max-min)

	var totalSlope float64
	var amplitude float64
	var volatility float64

	switch class {
	case trendGrowing:
		totalSlope = span * (0.06 + float64(seed%6)*0.01) // 6-11% of span over window
		amplitude = span * 0.02
		volatility = span * 0.01
	case trendDeclining:
		totalSlope = -span * (0.06 + float64(seed%6)*0.01)
		amplitude = span * 0.02
		volatility = span * 0.01
	case trendVolatile:
		totalSlope = 0
		amplitude = span * (0.06 + float64(seed%6)*0.01)
		volatility = span * 0.03
	default:
		totalSlope = 0
		amplitude = span * 0.015
		volatility = span * 0.007
	}

	cycles := int(seed%3) + 1
	lastIdx := float64(points - 1)
	slopePerStep := totalSlope / lastIdx

	raw := make([]float64, points)
	for i := 0; i < points; i++ {
		progress := float64(i) / lastIdx
		// 0 at both ends (progress 0 and 1), so the end value can align cleanly.
		wave := math.Sin(2 * math.Pi * float64(cycles) * progress)
		noiseScale := 1 - progress
		noise := rng.NormFloat64() * volatility * noiseScale
		raw[i] = current + slopePerStep*float64(i-(points-1)) + amplitude*wave + noise
	}

	// Shift so the last point exactly matches current.
	offset := current - raw[points-1]
	for i := range raw {
		raw[i] = clampFloat(raw[i]+offset, min, max)
	}
	raw[points-1] = current
	return raw
}

func seedMockMetricsHistory(mh *MetricsHistory, ms *metrics.Store, state models.StateSnapshot, now time.Time, seedDuration, interval time.Duration) {
	if mh == nil {
		return
	}
	if seedDuration <= 0 || interval <= 0 {
		return
	}

	// Choose a seed interval that respects the requested sample interval
	// while keeping the total number of points bounded.
	seedInterval := interval
	if seedInterval <= 0 {
		seedInterval = 30 * time.Second
	}
	const maxSeedPoints = 2000
	if seedDuration/seedInterval > maxSeedPoints {
		seedInterval = seedDuration / maxSeedPoints
	}
	if seedInterval < 30*time.Second {
		seedInterval = 30 * time.Second
	}
	const seedBatchSize = 5000

	var seedBatch []metrics.WriteMetric
	queueMetric := func(resourceType, resourceID, metricType string, value float64, ts time.Time) {
		if ms == nil {
			return
		}
		seedBatch = append(seedBatch,
			metrics.WriteMetric{
				ResourceType: resourceType,
				ResourceID:   resourceID,
				MetricType:   metricType,
				Value:        value,
				Timestamp:    ts,
				Tier:         metrics.TierHourly,
			},
			metrics.WriteMetric{
				ResourceType: resourceType,
				ResourceID:   resourceID,
				MetricType:   metricType,
				Value:        value,
				Timestamp:    ts,
				Tier:         metrics.TierDaily,
			},
		)

		if len(seedBatch) >= seedBatchSize {
			ms.WriteBatchSync(seedBatch)
			seedBatch = seedBatch[:0]
		}
	}

	recordNode := func(node models.Node) {
		if node.ID == "" {
			return
		}

		numPoints := int(seedDuration / seedInterval)
		cpuSeries := generateSeededSeries(node.CPU*100, numPoints, hashSeed("node", node.ID, "cpu"), 5, 85)
		memSeries := generateSeededSeries(node.Memory.Usage, numPoints, hashSeed("node", node.ID, "memory"), 10, 85)
		diskSeries := generateSeededSeries(node.Disk.Usage, numPoints, hashSeed("node", node.ID, "disk"), 5, 95)

		startTime := now.Add(-seedDuration)
		for i := 0; i < numPoints; i++ {
			ts := startTime.Add(time.Duration(i) * seedInterval)
			mh.AddNodeMetric(node.ID, "cpu", cpuSeries[i], ts)
			mh.AddNodeMetric(node.ID, "memory", memSeries[i], ts)
			mh.AddNodeMetric(node.ID, "disk", diskSeries[i], ts)
			queueMetric("node", node.ID, "cpu", cpuSeries[i], ts)
			queueMetric("node", node.ID, "memory", memSeries[i], ts)
			queueMetric("node", node.ID, "disk", diskSeries[i], ts)
		}

		// Ensure the latest point lands at "now" for full-range charts.
		mh.AddNodeMetric(node.ID, "cpu", node.CPU*100, now)
		mh.AddNodeMetric(node.ID, "memory", node.Memory.Usage, now)
		mh.AddNodeMetric(node.ID, "disk", node.Disk.Usage, now)
		queueMetric("node", node.ID, "cpu", node.CPU*100, now)
		queueMetric("node", node.ID, "memory", node.Memory.Usage, now)
		queueMetric("node", node.ID, "disk", node.Disk.Usage, now)
	}

	recordGuest := func(metricID, storeType, storeID string, cpuPercent, memPercent, diskPercent, diskRead, diskWrite, netIn, netOut float64, includeIO bool) {
		if metricID == "" || storeID == "" {
			return
		}

		numPoints := int(seedDuration / seedInterval)
		cpuSeries := generateSeededSeries(cpuPercent, numPoints, hashSeed(storeType, storeID, "cpu"), 0, 100)
		memSeries := generateSeededSeries(memPercent, numPoints, hashSeed(storeType, storeID, "memory"), 0, 100)
		diskSeries := generateSeededSeries(diskPercent, numPoints, hashSeed(storeType, storeID, "disk"), 0, 100)
		var diskReadSeries, diskWriteSeries, netInSeries, netOutSeries []float64
		if includeIO {
			ioMax := func(value float64) float64 {
				return math.Max(value*1.8, 1)
			}
			diskReadSeries = generateSeededSeries(diskRead, numPoints, hashSeed(storeType, storeID, "diskread"), 0, ioMax(diskRead))
			diskWriteSeries = generateSeededSeries(diskWrite, numPoints, hashSeed(storeType, storeID, "diskwrite"), 0, ioMax(diskWrite))
			netInSeries = generateSeededSeries(netIn, numPoints, hashSeed(storeType, storeID, "netin"), 0, ioMax(netIn))
			netOutSeries = generateSeededSeries(netOut, numPoints, hashSeed(storeType, storeID, "netout"), 0, ioMax(netOut))
		}

		startTime := now.Add(-seedDuration)
		for i := 0; i < numPoints; i++ {
			ts := startTime.Add(time.Duration(i) * seedInterval)
			mh.AddGuestMetric(metricID, "cpu", cpuSeries[i], ts)
			mh.AddGuestMetric(metricID, "memory", memSeries[i], ts)
			mh.AddGuestMetric(metricID, "disk", diskSeries[i], ts)
			queueMetric(storeType, storeID, "cpu", cpuSeries[i], ts)
			queueMetric(storeType, storeID, "memory", memSeries[i], ts)
			queueMetric(storeType, storeID, "disk", diskSeries[i], ts)
			if includeIO {
				mh.AddGuestMetric(metricID, "diskread", diskReadSeries[i], ts)
				mh.AddGuestMetric(metricID, "diskwrite", diskWriteSeries[i], ts)
				mh.AddGuestMetric(metricID, "netin", netInSeries[i], ts)
				mh.AddGuestMetric(metricID, "netout", netOutSeries[i], ts)
				queueMetric(storeType, storeID, "diskread", diskReadSeries[i], ts)
				queueMetric(storeType, storeID, "diskwrite", diskWriteSeries[i], ts)
				queueMetric(storeType, storeID, "netin", netInSeries[i], ts)
				queueMetric(storeType, storeID, "netout", netOutSeries[i], ts)
			}
		}

		// Ensure the latest point lands at "now" for full-range charts.
		mh.AddGuestMetric(metricID, "cpu", cpuPercent, now)
		mh.AddGuestMetric(metricID, "memory", memPercent, now)
		mh.AddGuestMetric(metricID, "disk", diskPercent, now)
		queueMetric(storeType, storeID, "cpu", cpuPercent, now)
		queueMetric(storeType, storeID, "memory", memPercent, now)
		queueMetric(storeType, storeID, "disk", diskPercent, now)
		if includeIO {
			mh.AddGuestMetric(metricID, "diskread", diskRead, now)
			mh.AddGuestMetric(metricID, "diskwrite", diskWrite, now)
			mh.AddGuestMetric(metricID, "netin", netIn, now)
			mh.AddGuestMetric(metricID, "netout", netOut, now)
			queueMetric(storeType, storeID, "diskread", diskRead, now)
			queueMetric(storeType, storeID, "diskwrite", diskWrite, now)
			queueMetric(storeType, storeID, "netin", netIn, now)
			queueMetric(storeType, storeID, "netout", netOut, now)
		}
	}

	log.Debug().Int("count", len(state.Nodes)).Msg("Mock seeding: processing nodes")
	for _, node := range state.Nodes {
		recordNode(node)
		time.Sleep(50 * time.Millisecond) // Reduced from 200ms for faster startup
	}

	runningVMs := 0
	for _, vm := range state.VMs {
		if vm.Status == "running" {
			runningVMs++
		}
	}
	log.Debug().Int("total", len(state.VMs)).Int("running", runningVMs).Msg("Mock seeding: processing VMs")
	for _, vm := range state.VMs {
		if vm.Status != "running" {
			continue
		}
		recordGuest(vm.ID, "vm", vm.ID, vm.CPU*100, vm.Memory.Usage, vm.Disk.Usage, float64(vm.DiskRead), float64(vm.DiskWrite), float64(vm.NetworkIn), float64(vm.NetworkOut), true)
		time.Sleep(50 * time.Millisecond) // Reduced from 200ms for faster startup
	}

	runningContainers := 0
	for _, ct := range state.Containers {
		if ct.Status == "running" {
			runningContainers++
		}
	}
	log.Debug().Int("total", len(state.Containers)).Int("running", runningContainers).Msg("Mock seeding: processing containers")
	for _, ct := range state.Containers {
		if ct.Status != "running" {
			continue
		}
		recordGuest(ct.ID, "container", ct.ID, ct.CPU*100, ct.Memory.Usage, ct.Disk.Usage, float64(ct.DiskRead), float64(ct.DiskWrite), float64(ct.NetworkIn), float64(ct.NetworkOut), true)
		time.Sleep(50 * time.Millisecond) // Reduced from 200ms for faster startup
	}

	log.Debug().Int("count", len(state.Storage)).Msg("Mock seeding: processing storage")
	for _, storage := range state.Storage {
		if storage.ID == "" {
			continue
		}
		numPoints := int(seedDuration / seedInterval)
		usageSeries := generateSeededSeries(storage.Usage, numPoints, hashSeed("storage", storage.ID, "usage"), 0, 100)

		startTime := now.Add(-seedDuration)
		for i := 0; i < numPoints; i++ {
			ts := startTime.Add(time.Duration(i) * seedInterval)
			mh.AddStorageMetric(storage.ID, "usage", usageSeries[i], ts)
			queueMetric("storage", storage.ID, "usage", usageSeries[i], ts)
		}

		// Ensure the latest point lands at "now" for full-range charts.
		mh.AddStorageMetric(storage.ID, "usage", storage.Usage, now)
		queueMetric("storage", storage.ID, "usage", storage.Usage, now)
		time.Sleep(50 * time.Millisecond) // Reduced from 200ms for faster startup
	}

	log.Debug().Int("count", len(state.DockerHosts)).Msg("Mock seeding: processing docker hosts")
	for _, host := range state.DockerHosts {
		if host.ID == "" {
			continue
		}

		var diskPercent float64
		var usedTotal, totalTotal int64
		for _, d := range host.Disks {
			if d.Total > 0 {
				usedTotal += d.Used
				totalTotal += d.Total
			}
		}
		if totalTotal > 0 {
			diskPercent = float64(usedTotal) / float64(totalTotal) * 100
		}

		recordGuest("dockerHost:"+host.ID, "dockerHost", host.ID, host.CPUUsage, host.Memory.Usage, diskPercent, 0, 0, 0, 0, false)

		for _, container := range host.Containers {
			if container.ID == "" || container.State != "running" {
				continue
			}

			var containerDisk float64
			if container.RootFilesystemBytes > 0 && container.WritableLayerBytes > 0 {
				containerDisk = float64(container.WritableLayerBytes) / float64(container.RootFilesystemBytes) * 100
				containerDisk = clampFloat(containerDisk, 0, 100)
			}
			recordGuest("docker:"+container.ID, "docker", container.ID, container.CPUPercent, container.MemoryPercent, containerDisk, 0, 0, 0, 0, false)
		}
		time.Sleep(50 * time.Millisecond) // Add delay for docker hosts
	}

	if ms != nil && len(seedBatch) > 0 {
		ms.WriteBatchSync(seedBatch)
	}
	log.Debug().Msg("Mock seeding: completed")
}

func recordMockStateToMetricsHistory(mh *MetricsHistory, ms *metrics.Store, state models.StateSnapshot, ts time.Time) {
	if mh == nil {
		return
	}

	for _, node := range state.Nodes {
		if node.ID == "" || node.Status != "online" {
			continue
		}
		mh.AddNodeMetric(node.ID, "cpu", node.CPU*100, ts)
		mh.AddNodeMetric(node.ID, "memory", node.Memory.Usage, ts)
		mh.AddNodeMetric(node.ID, "disk", node.Disk.Usage, ts)

		if ms != nil {
			ms.Write("node", node.ID, "cpu", node.CPU*100, ts)
			ms.Write("node", node.ID, "memory", node.Memory.Usage, ts)
			ms.Write("node", node.ID, "disk", node.Disk.Usage, ts)
		}
	}

	for _, vm := range state.VMs {
		if vm.ID == "" || vm.Status != "running" {
			continue
		}
		mh.AddGuestMetric(vm.ID, "cpu", vm.CPU*100, ts)
		mh.AddGuestMetric(vm.ID, "memory", vm.Memory.Usage, ts)
		mh.AddGuestMetric(vm.ID, "disk", vm.Disk.Usage, ts)
		mh.AddGuestMetric(vm.ID, "diskread", float64(vm.DiskRead), ts)
		mh.AddGuestMetric(vm.ID, "diskwrite", float64(vm.DiskWrite), ts)
		mh.AddGuestMetric(vm.ID, "netin", float64(vm.NetworkIn), ts)
		mh.AddGuestMetric(vm.ID, "netout", float64(vm.NetworkOut), ts)

		if ms != nil {
			ms.Write("vm", vm.ID, "cpu", vm.CPU*100, ts)
			ms.Write("vm", vm.ID, "memory", vm.Memory.Usage, ts)
			ms.Write("vm", vm.ID, "disk", vm.Disk.Usage, ts)
			ms.Write("vm", vm.ID, "diskread", float64(vm.DiskRead), ts)
			ms.Write("vm", vm.ID, "diskwrite", float64(vm.DiskWrite), ts)
			ms.Write("vm", vm.ID, "netin", float64(vm.NetworkIn), ts)
			ms.Write("vm", vm.ID, "netout", float64(vm.NetworkOut), ts)
		}
	}

	for _, ct := range state.Containers {
		if ct.ID == "" || ct.Status != "running" {
			continue
		}
		mh.AddGuestMetric(ct.ID, "cpu", ct.CPU*100, ts)
		mh.AddGuestMetric(ct.ID, "memory", ct.Memory.Usage, ts)
		mh.AddGuestMetric(ct.ID, "disk", ct.Disk.Usage, ts)
		mh.AddGuestMetric(ct.ID, "diskread", float64(ct.DiskRead), ts)
		mh.AddGuestMetric(ct.ID, "diskwrite", float64(ct.DiskWrite), ts)
		mh.AddGuestMetric(ct.ID, "netin", float64(ct.NetworkIn), ts)
		mh.AddGuestMetric(ct.ID, "netout", float64(ct.NetworkOut), ts)

		if ms != nil {
			ms.Write("container", ct.ID, "cpu", ct.CPU*100, ts)
			ms.Write("container", ct.ID, "memory", ct.Memory.Usage, ts)
			ms.Write("container", ct.ID, "disk", ct.Disk.Usage, ts)
			ms.Write("container", ct.ID, "diskread", float64(ct.DiskRead), ts)
			ms.Write("container", ct.ID, "diskwrite", float64(ct.DiskWrite), ts)
			ms.Write("container", ct.ID, "netin", float64(ct.NetworkIn), ts)
			ms.Write("container", ct.ID, "netout", float64(ct.NetworkOut), ts)
		}
	}

	for _, storage := range state.Storage {
		if storage.ID == "" || storage.Status != "available" {
			continue
		}
		mh.AddStorageMetric(storage.ID, "usage", storage.Usage, ts)
		mh.AddStorageMetric(storage.ID, "used", float64(storage.Used), ts)
		mh.AddStorageMetric(storage.ID, "total", float64(storage.Total), ts)
		mh.AddStorageMetric(storage.ID, "avail", float64(storage.Free), ts)

		if ms != nil {
			ms.Write("storage", storage.ID, "usage", storage.Usage, ts)
		}
	}

	for _, host := range state.DockerHosts {
		if host.ID == "" || host.Status != "online" {
			continue
		}

		var diskPercent float64
		var usedTotal, totalTotal int64
		for _, d := range host.Disks {
			if d.Total > 0 {
				usedTotal += d.Used
				totalTotal += d.Total
			}
		}
		if totalTotal > 0 {
			diskPercent = float64(usedTotal) / float64(totalTotal) * 100
		}

		hostKey := "dockerHost:" + host.ID
		mh.AddGuestMetric(hostKey, "cpu", host.CPUUsage, ts)
		mh.AddGuestMetric(hostKey, "memory", host.Memory.Usage, ts)
		mh.AddGuestMetric(hostKey, "disk", diskPercent, ts)

		if ms != nil {
			ms.Write("dockerHost", host.ID, "cpu", host.CPUUsage, ts)
			ms.Write("dockerHost", host.ID, "memory", host.Memory.Usage, ts)
			ms.Write("dockerHost", host.ID, "disk", diskPercent, ts)
		}

		for _, container := range host.Containers {
			if container.ID == "" || container.State != "running" {
				continue
			}

			var containerDisk float64
			if container.RootFilesystemBytes > 0 && container.WritableLayerBytes > 0 {
				containerDisk = float64(container.WritableLayerBytes) / float64(container.RootFilesystemBytes) * 100
				containerDisk = clampFloat(containerDisk, 0, 100)
			}

			metricKey := "docker:" + container.ID
			mh.AddGuestMetric(metricKey, "cpu", container.CPUPercent, ts)
			mh.AddGuestMetric(metricKey, "memory", container.MemoryPercent, ts)
			mh.AddGuestMetric(metricKey, "disk", containerDisk, ts)

			if ms != nil {
				ms.Write("docker", container.ID, "cpu", container.CPUPercent, ts)
				ms.Write("docker", container.ID, "memory", container.MemoryPercent, ts)
				ms.Write("docker", container.ID, "disk", containerDisk, ts)
			}
		}
	}
}

func (m *Monitor) startMockMetricsSampler(ctx context.Context) {
	if ctx == nil || m == nil {
		log.Debug().Msg("Mock metrics sampler: nil context or monitor")
		return
	}
	if !mock.IsMockEnabled() {
		log.Debug().Msg("Mock metrics sampler: mock mode not enabled")
		return
	}

	log.Info().Msg("Mock metrics sampler: starting initialization")

	cfg := mockMetricsSamplerConfigFromEnv()
	seedDuration := cfg.SeedDuration
	// Reduced minimum from 7 days to 1 hour for faster startup on resource-constrained systems
	if seedDuration < time.Hour {
		seedDuration = time.Hour
	}
	maxPoints := int(seedDuration / cfg.SampleInterval)

	m.mu.Lock()
	if m.mockMetricsCancel != nil {
		m.mu.Unlock()
		log.Debug().Msg("Mock metrics sampler: already running")
		return
	}
	samplerCtx, cancel := context.WithCancel(ctx)
	m.mockMetricsCancel = cancel
	m.metricsHistory = NewMetricsHistory(maxPoints, seedDuration)
	m.mu.Unlock()

	state := mock.GetMockState()
	log.Info().
		Int("nodes", len(state.Nodes)).
		Int("vms", len(state.VMs)).
		Int("containers", len(state.Containers)).
		Dur("seedDuration", seedDuration).
		Dur("sampleInterval", cfg.SampleInterval).
		Msg("Mock metrics sampler: seeding historical data")

	if m.metricsStore != nil {
		if err := m.metricsStore.Clear(); err != nil {
			log.Warn().Err(err).Msg("Failed to clear metrics store before mock seeding")
		}
	}
	seedMockMetricsHistory(m.metricsHistory, m.metricsStore, state, time.Now(), seedDuration, cfg.SampleInterval)
	recordMockStateToMetricsHistory(m.metricsHistory, m.metricsStore, state, time.Now())

	// Flush metrics store to ensure all seeded data is written to disk
	if m.metricsStore != nil {
		m.metricsStore.Flush()
	}

	m.mockMetricsWg.Add(1)
	go func() {
		defer m.mockMetricsWg.Done()

		ticker := time.NewTicker(cfg.SampleInterval)
		defer ticker.Stop()

		for {
			select {
			case <-samplerCtx.Done():
				return
			case <-ticker.C:
				if !mock.IsMockEnabled() {
					continue
				}
				recordMockStateToMetricsHistory(m.metricsHistory, m.metricsStore, mock.GetMockState(), time.Now())
			}
		}
	}()

	log.Info().
		Dur("seedDuration", seedDuration).
		Dur("sampleInterval", cfg.SampleInterval).
		Msg("Mock metrics history sampler started")
}

func (m *Monitor) stopMockMetricsSampler() {
	if m == nil {
		return
	}

	m.mu.Lock()
	cancel := m.mockMetricsCancel
	m.mockMetricsCancel = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
		m.mockMetricsWg.Wait()
		log.Info().Msg("Mock metrics history sampler stopped")
	}
}
