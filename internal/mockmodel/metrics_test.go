package mockmodel

import (
	"math"
	"testing"
	"time"
)

func TestValueAtMetric_CPUTracksActivityWindows(t *testing.T) {
	seed := uint64(0) // office-heavy persona
	day := time.Date(2026, time.March, 31, 10, 30, 0, 0, time.UTC)
	night := time.Date(2026, time.March, 31, 3, 30, 0, 0, time.UTC)

	dayValue := ValueAtMetric(seed, 0, 100, "cpu", 1.0, day)
	nightValue := ValueAtMetric(seed, 0, 100, "cpu", 1.0, night)
	if dayValue <= nightValue+8 {
		t.Fatalf("expected office-hour cpu value to exceed overnight value, got day=%f night=%f", dayValue, nightValue)
	}
}

func TestSeriesForMetricTimestamps_AnchorsTailWithoutRebasingHistory(t *testing.T) {
	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	timestamps := make([]time.Time, 0, 7*24+1)
	for ts := now.Add(-7 * 24 * time.Hour); !ts.After(now); ts = ts.Add(time.Hour) {
		timestamps = append(timestamps, ts)
	}

	seed := uint64(42)
	baseCurrent := ValueAtMetric(seed, 0, 100, "cpu", 1.0, now)
	baseline := SeriesForMetricTimestamps(baseCurrent, timestamps, seed, 0, 100, "cpu", StyleSpiky)
	anchored := SeriesForMetricTimestamps(baseCurrent+24, timestamps, seed, 0, 100, "cpu", StyleSpiky)

	if anchored[len(anchored)-1] != baseCurrent+24 {
		t.Fatalf("expected anchored tail to match current, got %f want %f", anchored[len(anchored)-1], baseCurrent+24)
	}

	farDelta := math.Abs(anchored[0] - baseline[0])
	recentDelta := math.Abs(anchored[len(anchored)-2] - baseline[len(baseline)-2])
	if farDelta >= recentDelta {
		t.Fatalf("expected tail anchoring to affect recent points more than far history, got far=%f recent=%f", farDelta, recentDelta)
	}
	if farDelta > 2.5 {
		t.Fatalf("expected 7d-old history to remain mostly intact, got far delta %f", farDelta)
	}
}

func TestSeriesForMetricTimestamps_MemoryAndCapacityStaySmootherThanCPU(t *testing.T) {
	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	timestamps := make([]time.Time, 0, 7*24+1)
	for ts := now.Add(-7 * 24 * time.Hour); !ts.After(now); ts = ts.Add(time.Hour) {
		timestamps = append(timestamps, ts)
	}

	cpuSeed := uint64(0)
	memSeed := uint64(11)
	diskSeed := uint64(23)

	cpuCurrent := ValueAtMetric(cpuSeed, 0, 100, "cpu", 1.0, now)
	memCurrent := ValueAtMetric(memSeed, 0, 100, "memory", 0.5, now)
	diskCurrent := ValueAtMetric(diskSeed, 0, 100, "usage", 0.12, now)

	cpuSeries := SeriesForMetricTimestamps(cpuCurrent, timestamps, cpuSeed, 0, 100, "cpu", StyleSpiky)
	memSeries := SeriesForMetricTimestamps(memCurrent, timestamps, memSeed, 0, 100, "memory", StylePlateau)
	diskSeries := SeriesForMetricTimestamps(diskCurrent, timestamps, diskSeed, 0, 100, "usage", StyleFlat)

	cpuStep := averageAdjacentDelta(cpuSeries)
	memStep := averageAdjacentDelta(memSeries)
	diskStep := averageAdjacentDelta(diskSeries[len(diskSeries)-7:])
	diskRange := seriesRange(diskSeries)

	if memStep >= cpuStep {
		t.Fatalf("expected memory to move more smoothly than cpu, got mem=%f cpu=%f", memStep, cpuStep)
	}
	if diskStep > 3 {
		t.Fatalf("expected short-term capacity movement to stay gentle, got average 6h delta %f", diskStep)
	}
	if diskRange < 2 {
		t.Fatalf("expected weekly capacity series to show meaningful drift, got range %f", diskRange)
	}
}

func TestStorageCapacitySeriesForTimestamps_KeepsStorageMetricsAligned(t *testing.T) {
	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	timestamps := make([]time.Time, 0, 7*24+1)
	for ts := now.Add(-7 * 24 * time.Hour); !ts.After(now); ts = ts.Add(time.Hour) {
		timestamps = append(timestamps, ts)
	}

	total := 10.0 * 1024 * 1024 * 1024 * 1024
	used := 6.2 * 1024 * 1024 * 1024 * 1024
	series := StorageCapacitySeriesForTimestamps(used, total, timestamps, 77)

	if len(series.Usage) != len(timestamps) || len(series.Used) != len(timestamps) || len(series.Avail) != len(timestamps) {
		t.Fatalf("expected aligned storage series lengths, got usage=%d used=%d avail=%d", len(series.Usage), len(series.Used), len(series.Avail))
	}

	last := len(timestamps) - 1
	if math.Abs(series.Used[last]-used) > 0.5 {
		t.Fatalf("expected tail used bytes to match current value, got %f want %f", series.Used[last], used)
	}
	if math.Abs(series.Avail[last]-(total-used)) > 0.5 {
		t.Fatalf("expected tail available bytes to complement used bytes, got %f want %f", series.Avail[last], total-used)
	}

	for i := range timestamps {
		if math.Abs((series.Used[i]+series.Avail[i])-series.Total[i]) > 0.5 {
			t.Fatalf("expected used+avail to equal total at index %d", i)
		}
		if math.Abs(series.Usage[i]-((series.Used[i]/series.Total[i])*100)) > 0.001 {
			t.Fatalf("expected usage percent to match used/total at index %d", i)
		}
	}
}

func TestSeriesForMetricTimestamps_SmartTempUsesThermalProfile(t *testing.T) {
	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	timestamps := make([]time.Time, 0, 7*24+1)
	for ts := now.Add(-7 * 24 * time.Hour); !ts.After(now); ts = ts.Add(time.Hour) {
		timestamps = append(timestamps, ts)
	}

	series := SeriesForMetricTimestamps(42, timestamps, 19, 25, 95, "smart_temp", StyleFlat)
	if got := seriesRange(series); got > 10 {
		t.Fatalf("expected thermal history to remain gentle, got range %f", got)
	}
}

func TestValueAtMetricWithRole_DatabaseRunsHotterOnMemoryThanWeb(t *testing.T) {
	seed := uint64(17)
	at := time.Date(2026, time.March, 31, 14, 0, 0, 0, time.UTC)

	database := ValueAtMetricWithRole(seed, 0, 100, "memory", 0.5, "database", at)
	web := ValueAtMetricWithRole(seed, 0, 100, "memory", 0.5, "web", at)
	if database <= web+8 {
		t.Fatalf("expected database memory profile to sit materially above web profile, got database=%f web=%f", database, web)
	}
}

func TestSeriesForMetricTimestampsWithRole_WebNetworkVariesMoreThanDatabase(t *testing.T) {
	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	timestamps := make([]time.Time, 0, 48)
	for ts := now.Add(-47 * time.Hour); !ts.After(now); ts = ts.Add(time.Hour) {
		timestamps = append(timestamps, ts)
	}

	seed := uint64(29)
	current := ValueAtMetricWithRole(seed, 0, 100, "netin", 1.0, "web", now)
	web := SeriesForMetricTimestampsWithRole(current, timestamps, seed, 0, 100, "netin", StyleSpiky, "web")
	database := SeriesForMetricTimestampsWithRole(current, timestamps, seed, 0, 100, "netin", StyleSpiky, "database")
	if seriesRange(web) <= seriesRange(database)+4 {
		t.Fatalf("expected web network profile to vary more than database network profile, got web=%f database=%f", seriesRange(web), seriesRange(database))
	}
}

func averageAdjacentDelta(series []float64) float64 {
	if len(series) < 2 {
		return 0
	}
	total := 0.0
	for i := 1; i < len(series); i++ {
		total += math.Abs(series[i] - series[i-1])
	}
	return total / float64(len(series)-1)
}

func seriesRange(series []float64) float64 {
	if len(series) == 0 {
		return 0
	}
	minValue := series[0]
	maxValue := series[0]
	for _, value := range series[1:] {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue - minValue
}
