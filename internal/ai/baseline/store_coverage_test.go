package baseline

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewStore_LoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "baselines.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0600); err != nil {
		t.Fatalf("write invalid json: %v", err)
	}

	store := NewStore(StoreConfig{DataDir: tmpDir})
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if store.ResourceCount() != 0 {
		t.Errorf("expected empty store, got %d resources", store.ResourceCount())
	}
}

func TestIsAnomaly_EdgeCases(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 5})

	store.mu.Lock()
	store.baselines["res"] = &ResourceBaseline{
		ResourceID:   "res",
		ResourceType: "vm",
		Metrics: map[string]*MetricBaseline{
			"zero":  {Mean: 50, StdDev: 0, SampleCount: 10},
			"small": {Mean: 50, StdDev: 0.5, SampleCount: 10},
			"wide":  {Mean: 50, StdDev: 5, SampleCount: 10},
			"few":   {Mean: 50, StdDev: 2, SampleCount: 2},
		},
	}
	store.mu.Unlock()

	isAnomaly, zScore := store.IsAnomaly("res", "zero", 54)
	if isAnomaly || zScore != 0 {
		t.Errorf("expected no anomaly for 4 point change with zero stddev, got anomaly=%v z=%.2f", isAnomaly, zScore)
	}

	isAnomaly, zScore = store.IsAnomaly("res", "zero", 56)
	if !isAnomaly || zScore != 0 {
		t.Errorf("expected anomaly for 6 point change with zero stddev, got anomaly=%v z=%.2f", isAnomaly, zScore)
	}

	isAnomaly, zScore = store.IsAnomaly("res", "wide", 53)
	if isAnomaly || math.Abs(zScore-0.6) > 0.01 {
		t.Errorf("expected non-anomaly with small z-score, got anomaly=%v z=%.2f", isAnomaly, zScore)
	}

	isAnomaly, zScore = store.IsAnomaly("res", "small", 53)
	if !isAnomaly || math.Abs(zScore-3.0) > 0.01 {
		t.Errorf("expected anomaly after stddev floor, got anomaly=%v z=%.2f", isAnomaly, zScore)
	}

	isAnomaly, zScore = store.IsAnomaly("res", "few", 60)
	if isAnomaly || zScore != 0 {
		t.Errorf("expected no anomaly with insufficient samples, got anomaly=%v z=%.2f", isAnomaly, zScore)
	}
}

func TestCheckAnomaly_StdDevFloorAndMedium(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 5})

	store.mu.Lock()
	store.baselines["res"] = &ResourceBaseline{
		ResourceID:   "res",
		ResourceType: "vm",
		Metrics: map[string]*MetricBaseline{
			"medium": {Mean: 50, StdDev: 1.2, SampleCount: 10},
			"floor":  {Mean: 50, StdDev: 0.5, SampleCount: 10},
		},
	}
	store.mu.Unlock()

	severity, zScore, _ := store.CheckAnomaly("res", "medium", 53.24)
	if severity != AnomalyMedium || math.Abs(zScore-2.7) > 0.01 {
		t.Errorf("expected medium anomaly with z~2.7, got severity=%s z=%.2f", severity, zScore)
	}

	severity, zScore, _ = store.CheckAnomaly("res", "floor", 54)
	if severity != AnomalyCritical || math.Abs(zScore-4.0) > 0.01 {
		t.Errorf("expected critical anomaly after stddev floor, got severity=%s z=%.2f", severity, zScore)
	}
}

func TestCalculateTrend_CapacityReached(t *testing.T) {
	samples := make([]float64, 48)
	for i := range samples {
		samples[i] = 50 + float64(i)
	}

	result := CalculateTrend(samples, 100)
	if result == nil {
		t.Fatal("expected non-nil result for increasing trend")
	}
	if result.DaysToFull != 0 {
		t.Errorf("expected DaysToFull=0 at capacity, got %d", result.DaysToFull)
	}
	if result.Severity != "critical" {
		t.Errorf("expected critical severity at capacity, got %s", result.Severity)
	}
	if result.Description != "Resource at capacity" {
		t.Errorf("unexpected description: %q", result.Description)
	}
}

func TestCalculateTrend_WarningAndInfo(t *testing.T) {
	samplesWarning := make([]float64, 48)
	for i := range samplesWarning {
		samplesWarning[i] = 70 + float64(i)*0.0833333
	}

	result := CalculateTrend(samplesWarning, 80)
	if result == nil {
		t.Fatal("expected non-nil result for warning trend")
	}
	if result.Severity != "warning" {
		t.Errorf("expected warning severity, got %s", result.Severity)
	}
	if !strings.Contains(result.Description, "Resource approaching capacity") {
		t.Errorf("unexpected warning description: %q", result.Description)
	}

	samplesInfo := make([]float64, 48)
	for i := range samplesInfo {
		samplesInfo[i] = 50 + float64(i)*0.01
	}

	result = CalculateTrend(samplesInfo, 20)
	if result == nil {
		t.Fatal("expected non-nil result for info trend")
	}
	if result.Severity != "info" {
		t.Errorf("expected info severity, got %s", result.Severity)
	}
	if !strings.Contains(result.Description, "Trending toward full in") {
		t.Errorf("unexpected info description: %q", result.Description)
	}
}

func TestSaveAndLoad_ErrorPaths(t *testing.T) {
	store := NewStore(StoreConfig{})
	if err := store.Save(); err != nil {
		t.Fatalf("expected Save to succeed with empty data dir: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "baseline-datafile")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	defer os.Remove(tmpPath)

	store.dataDir = tmpPath
	if err := store.Save(); err == nil {
		t.Error("expected Save to fail when data dir is a file")
	}

	store.dataDir = ""
	if err := store.saveToDisk(); err != nil {
		t.Fatalf("expected saveToDisk to succeed with empty data dir: %v", err)
	}

	tmpDir := t.TempDir()
	store.dataDir = tmpDir
	baselinesPath := filepath.Join(tmpDir, "baselines.json")
	if err := os.Mkdir(baselinesPath, 0700); err != nil {
		t.Fatalf("create baselines dir: %v", err)
	}
	if err := store.saveToDisk(); err == nil {
		t.Error("expected saveToDisk to fail when baselines path is a directory")
	}

	store.dataDir = t.TempDir()
	if err := store.loadFromDisk(); err != nil {
		t.Fatalf("expected loadFromDisk to succeed when file missing: %v", err)
	}

	invalidDir := t.TempDir()
	invalidPath := filepath.Join(invalidDir, "baselines.json")
	if err := os.WriteFile(invalidPath, []byte("{invalid"), 0600); err != nil {
		t.Fatalf("write invalid json: %v", err)
	}
	store.dataDir = invalidDir
	if err := store.loadFromDisk(); err == nil {
		t.Error("expected loadFromDisk to fail on invalid json")
	}

	store.dataDir = tmpPath
	if err := store.loadFromDisk(); err == nil {
		t.Error("expected loadFromDisk to fail when data dir is a file")
	}
}

func TestSaveToDisk_MarshalError(t *testing.T) {
	store := NewStore(StoreConfig{})
	store.dataDir = t.TempDir()

	store.mu.Lock()
	store.baselines["res"] = &ResourceBaseline{
		ResourceID: "res",
		Metrics: map[string]*MetricBaseline{
			"cpu": {Mean: math.NaN(), StdDev: 1, SampleCount: 1},
		},
	}
	store.mu.Unlock()

	if err := store.saveToDisk(); err == nil {
		t.Error("expected saveToDisk to fail on invalid json value")
	}
}

func TestPercentile_Empty(t *testing.T) {
	if got := percentile([]float64{}, 50); got != 0 {
		t.Errorf("expected 0 for empty percentile, got %f", got)
	}
}
