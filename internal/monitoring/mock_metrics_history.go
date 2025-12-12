package monitoring

import (
	"context"
	"hash/fnv"
	"math"
	"math/rand"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

const (
	defaultMockSeedDuration   = time.Hour
	defaultMockSampleInterval = 30 * time.Second
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
	if seedDuration > 12*time.Hour {
		seedDuration = 12 * time.Hour
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

func seedMockMetricsHistory(mh *MetricsHistory, state models.StateSnapshot, now time.Time, seedDuration, interval time.Duration) {
	if mh == nil {
		return
	}
	if seedDuration <= 0 || interval <= 0 {
		return
	}

	points := int(seedDuration/interval) + 1
	if points < 2 {
		points = 2
	}

	start := now.Add(-time.Duration(points-1) * interval)

	recordNode := func(node models.Node) {
		if node.ID == "" {
			return
		}

		cpuSeries := generateSeededSeries(node.CPU*100, points, hashSeed("node", node.ID, "cpu"), 5, 85)
		memSeries := generateSeededSeries(node.Memory.Usage, points, hashSeed("node", node.ID, "memory"), 10, 85)
		diskSeries := generateSeededSeries(node.Disk.Usage, points, hashSeed("node", node.ID, "disk"), 5, 95)

		for i := 0; i < points; i++ {
			ts := start.Add(time.Duration(i) * interval)
			mh.AddNodeMetric(node.ID, "cpu", cpuSeries[i], ts)
			mh.AddNodeMetric(node.ID, "memory", memSeries[i], ts)
			mh.AddNodeMetric(node.ID, "disk", diskSeries[i], ts)
		}
	}

	recordGuest := func(id string, cpuPercent, memPercent, diskPercent float64) {
		if id == "" {
			return
		}
		cpuSeries := generateSeededSeries(cpuPercent, points, hashSeed("guest", id, "cpu"), 0, 100)
		memSeries := generateSeededSeries(memPercent, points, hashSeed("guest", id, "memory"), 0, 100)
		diskSeries := generateSeededSeries(diskPercent, points, hashSeed("guest", id, "disk"), 0, 100)

		for i := 0; i < points; i++ {
			ts := start.Add(time.Duration(i) * interval)
			mh.AddGuestMetric(id, "cpu", cpuSeries[i], ts)
			mh.AddGuestMetric(id, "memory", memSeries[i], ts)
			mh.AddGuestMetric(id, "disk", diskSeries[i], ts)
		}
	}

	for _, node := range state.Nodes {
		recordNode(node)
	}

	for _, vm := range state.VMs {
		if vm.Status != "running" {
			continue
		}
		recordGuest(vm.ID, vm.CPU*100, vm.Memory.Usage, vm.Disk.Usage)
	}

	for _, ct := range state.Containers {
		if ct.Status != "running" {
			continue
		}
		recordGuest(ct.ID, ct.CPU*100, ct.Memory.Usage, ct.Disk.Usage)
	}

	for _, storage := range state.Storage {
		if storage.ID == "" {
			continue
		}
		usageSeries := generateSeededSeries(storage.Usage, points, hashSeed("storage", storage.ID, "usage"), 0, 100)
		for i := 0; i < points; i++ {
			ts := start.Add(time.Duration(i) * interval)
			mh.AddStorageMetric(storage.ID, "usage", usageSeries[i], ts)
			mh.AddStorageMetric(storage.ID, "used", float64(storage.Used), ts)
			mh.AddStorageMetric(storage.ID, "total", float64(storage.Total), ts)
			mh.AddStorageMetric(storage.ID, "avail", float64(storage.Free), ts)
		}
	}

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

		recordGuest("dockerHost:"+host.ID, host.CPUUsage, host.Memory.Usage, diskPercent)

		for _, container := range host.Containers {
			if container.ID == "" || container.State != "running" {
				continue
			}

			var containerDisk float64
			if container.RootFilesystemBytes > 0 && container.WritableLayerBytes > 0 {
				containerDisk = float64(container.WritableLayerBytes) / float64(container.RootFilesystemBytes) * 100
				containerDisk = clampFloat(containerDisk, 0, 100)
			}
			recordGuest("docker:"+container.ID, container.CPUPercent, container.MemoryPercent, containerDisk)
		}
	}
}

func recordMockStateToMetricsHistory(mh *MetricsHistory, state models.StateSnapshot, ts time.Time) {
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
	}

	for _, storage := range state.Storage {
		if storage.ID == "" || storage.Status != "available" {
			continue
		}
		mh.AddStorageMetric(storage.ID, "usage", storage.Usage, ts)
		mh.AddStorageMetric(storage.ID, "used", float64(storage.Used), ts)
		mh.AddStorageMetric(storage.ID, "total", float64(storage.Total), ts)
		mh.AddStorageMetric(storage.ID, "avail", float64(storage.Free), ts)
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
		}
	}
}

func (m *Monitor) startMockMetricsSampler(ctx context.Context) {
	if ctx == nil || m == nil {
		return
	}
	if !mock.IsMockEnabled() {
		return
	}

	cfg := mockMetricsSamplerConfigFromEnv()

	m.mu.Lock()
	if m.mockMetricsCancel != nil {
		m.mu.Unlock()
		return
	}
	samplerCtx, cancel := context.WithCancel(ctx)
	m.mockMetricsCancel = cancel
	m.mu.Unlock()

	m.metricsHistory.Reset()
	state := mock.GetMockState()
	seedMockMetricsHistory(m.metricsHistory, state, time.Now(), cfg.SeedDuration, cfg.SampleInterval)
	recordMockStateToMetricsHistory(m.metricsHistory, state, time.Now())

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
				recordMockStateToMetricsHistory(m.metricsHistory, mock.GetMockState(), time.Now())
			}
		}
	}()

	log.Info().
		Dur("seedDuration", cfg.SeedDuration).
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
