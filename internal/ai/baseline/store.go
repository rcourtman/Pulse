// Package baseline provides learned baseline metrics for anomaly detection.
// It learns what "normal" looks like for each resource by analyzing historical
// metrics and can then flag current values that deviate significantly from the baseline.
package baseline

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// MetricBaseline represents learned "normal" behavior for a single metric
type MetricBaseline struct {
	Mean        float64            `json:"mean"`         // Average value
	StdDev      float64            `json:"stddev"`       // Standard deviation
	Percentiles map[int]float64    `json:"percentiles"`  // 5, 25, 50, 75, 95
	SampleCount int                `json:"sample_count"` // Number of samples used
	
	// Time-of-day patterns (future enhancement)
	HourlyMeans [24]float64 `json:"hourly_means,omitempty"`
}

// ResourceBaseline contains baselines for all metrics of a resource
type ResourceBaseline struct {
	ResourceID   string                     `json:"resource_id"`
	ResourceType string                     `json:"resource_type"` // node, vm, container, storage
	LastUpdated  time.Time                  `json:"last_updated"`
	Metrics      map[string]*MetricBaseline `json:"metrics"` // cpu, memory, disk
}

// Store manages baseline storage and learning
type Store struct {
	mu        sync.RWMutex
	baselines map[string]*ResourceBaseline // resourceID -> baseline
	
	// Configuration
	learningWindow  time.Duration // How far back to learn from (default: 7 days)
	minSamples      int           // Minimum samples needed (default: 50)
	updateInterval  time.Duration // How often to recompute (default: 1 hour)
	
	// Persistence
	dataDir     string
	persistence Persistence
}

// Persistence interface for saving/loading baselines
type Persistence interface {
	Save(baselines map[string]*ResourceBaseline) error
	Load() (map[string]*ResourceBaseline, error)
}

// StoreConfig configures the baseline store
type StoreConfig struct {
	LearningWindow  time.Duration
	MinSamples      int
	UpdateInterval  time.Duration
	DataDir         string
}

// DefaultConfig returns sensible defaults
func DefaultConfig() StoreConfig {
	return StoreConfig{
		LearningWindow:  7 * 24 * time.Hour, // 7 days
		MinSamples:      50,
		UpdateInterval:  1 * time.Hour,
	}
}

// NewStore creates a new baseline store
func NewStore(cfg StoreConfig) *Store {
	if cfg.LearningWindow == 0 {
		cfg.LearningWindow = 7 * 24 * time.Hour
	}
	if cfg.MinSamples == 0 {
		cfg.MinSamples = 50
	}
	if cfg.UpdateInterval == 0 {
		cfg.UpdateInterval = 1 * time.Hour
	}
	
	s := &Store{
		baselines:      make(map[string]*ResourceBaseline),
		learningWindow: cfg.LearningWindow,
		minSamples:     cfg.MinSamples,
		updateInterval: cfg.UpdateInterval,
		dataDir:        cfg.DataDir,
	}
	
	// Try to load existing baselines from disk
	if cfg.DataDir != "" {
		if err := s.loadFromDisk(); err != nil {
			log.Warn().Err(err).Msg("Failed to load baselines from disk, starting fresh")
		} else {
			log.Info().Int("count", len(s.baselines)).Msg("Loaded baselines from disk")
		}
	}
	
	return s
}

// MetricPoint represents a single metric value at a point in time
type MetricPoint struct {
	Value     float64
	Timestamp time.Time
}

// Learn computes baseline from historical data points
func (s *Store) Learn(resourceID, resourceType, metric string, points []MetricPoint) error {
	if len(points) < s.minSamples {
		log.Debug().
			Str("resource", resourceID).
			Str("metric", metric).
			Int("samples", len(points)).
			Int("required", s.minSamples).
			Msg("Insufficient data for baseline learning")
		return nil // Not an error, just not enough data yet
	}
	
	// Extract values
	values := make([]float64, len(points))
	for i, p := range points {
		values[i] = p.Value
	}
	
	// Compute statistics
	baseline := &MetricBaseline{
		Mean:        computeMean(values),
		StdDev:      computeStdDev(values),
		Percentiles: computePercentiles(values),
		SampleCount: len(values),
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Get or create resource baseline
	rb, exists := s.baselines[resourceID]
	if !exists {
		rb = &ResourceBaseline{
			ResourceID:   resourceID,
			ResourceType: resourceType,
			Metrics:      make(map[string]*MetricBaseline),
		}
		s.baselines[resourceID] = rb
	}
	
	rb.Metrics[metric] = baseline
	rb.LastUpdated = time.Now()
	
	log.Debug().
		Str("resource", resourceID).
		Str("metric", metric).
		Float64("mean", baseline.Mean).
		Float64("stddev", baseline.StdDev).
		Int("samples", baseline.SampleCount).
		Msg("Baseline learned")
	
	return nil
}

// GetBaseline returns the baseline for a resource/metric
func (s *Store) GetBaseline(resourceID, metric string) (*MetricBaseline, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	rb, exists := s.baselines[resourceID]
	if !exists {
		return nil, false
	}
	
	mb, exists := rb.Metrics[metric]
	return mb, exists
}

// GetResourceBaseline returns all baselines for a resource
func (s *Store) GetResourceBaseline(resourceID string) (*ResourceBaseline, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	rb, exists := s.baselines[resourceID]
	if !exists {
		return nil, false
	}
	
	// Return a copy to prevent mutation
	copy := &ResourceBaseline{
		ResourceID:   rb.ResourceID,
		ResourceType: rb.ResourceType,
		LastUpdated:  rb.LastUpdated,
		Metrics:      make(map[string]*MetricBaseline),
	}
	for k, v := range rb.Metrics {
		copy.Metrics[k] = v
	}
	return copy, true
}

// IsAnomaly checks if a value is anomalous for the given resource/metric
// Returns: isAnomaly, zScore (number of standard deviations from mean)
func (s *Store) IsAnomaly(resourceID, metric string, value float64) (bool, float64) {
	baseline, ok := s.GetBaseline(resourceID, metric)
	if !ok || baseline.SampleCount < s.minSamples {
		return false, 0 // Not enough data to determine
	}
	
	if baseline.StdDev == 0 {
		// No variance - any different value is anomalous
		if value != baseline.Mean {
			return true, math.Inf(1)
		}
		return false, 0
	}
	
	zScore := (value - baseline.Mean) / baseline.StdDev
	
	// Consider anything > 2 standard deviations as anomalous
	// (covers ~95% of normal distribution)
	isAnomaly := math.Abs(zScore) > 2.0
	
	return isAnomaly, zScore
}

// AnomalySeverity categorizes how severe an anomaly is
type AnomalySeverity string

const (
	AnomalyNone     AnomalySeverity = ""
	AnomalyLow      AnomalySeverity = "low"      // 2-2.5 std devs
	AnomalyMedium   AnomalySeverity = "medium"   // 2.5-3 std devs
	AnomalyHigh     AnomalySeverity = "high"     // 3-4 std devs
	AnomalyCritical AnomalySeverity = "critical" // > 4 std devs
)

// CheckAnomaly performs a detailed anomaly check with severity classification
func (s *Store) CheckAnomaly(resourceID, metric string, value float64) (AnomalySeverity, float64, *MetricBaseline) {
	baseline, ok := s.GetBaseline(resourceID, metric)
	if !ok || baseline.SampleCount < s.minSamples {
		return AnomalyNone, 0, nil
	}
	
	if baseline.StdDev == 0 {
		if value != baseline.Mean {
			return AnomalyCritical, math.Inf(1), baseline
		}
		return AnomalyNone, 0, baseline
	}
	
	zScore := (value - baseline.Mean) / baseline.StdDev
	absZ := math.Abs(zScore)
	
	var severity AnomalySeverity
	switch {
	case absZ < 2.0:
		severity = AnomalyNone
	case absZ < 2.5:
		severity = AnomalyLow
	case absZ < 3.0:
		severity = AnomalyMedium
	case absZ < 4.0:
		severity = AnomalyHigh
	default:
		severity = AnomalyCritical
	}
	
	return severity, zScore, baseline
}

// AnomalyReport represents a detected anomaly for a single metric
type AnomalyReport struct {
	ResourceID   string          `json:"resource_id"`
	ResourceName string          `json:"resource_name,omitempty"`
	ResourceType string          `json:"resource_type,omitempty"`
	Metric       string          `json:"metric"`
	CurrentValue float64         `json:"current_value"`
	BaselineMean float64         `json:"baseline_mean"`
	BaselineStdDev float64       `json:"baseline_std_dev"`
	ZScore       float64         `json:"z_score"`
	Severity     AnomalySeverity `json:"severity"`
	Description  string          `json:"description"`
}

// CheckResourceAnomalies checks multiple metrics for a resource and returns all anomalies
func (s *Store) CheckResourceAnomalies(resourceID string, metrics map[string]float64) []AnomalyReport {
	var anomalies []AnomalyReport
	
	for metric, value := range metrics {
		severity, zScore, baseline := s.CheckAnomaly(resourceID, metric, value)
		if severity != AnomalyNone {
			report := AnomalyReport{
				ResourceID:   resourceID,
				Metric:       metric,
				CurrentValue: value,
				ZScore:       zScore,
				Severity:     severity,
			}
			
			if baseline != nil {
				report.BaselineMean = baseline.Mean
				report.BaselineStdDev = baseline.StdDev
				
				// Generate human-readable description
				ratio := value / baseline.Mean
				direction := "above"
				if zScore < 0 {
					direction = "below"
				}
				report.Description = formatAnomalyDescription(metric, ratio, direction, severity)
			}
			
			anomalies = append(anomalies, report)
		}
	}
	
	return anomalies
}

// formatAnomalyDescription generates a human-readable anomaly description
func formatAnomalyDescription(metric string, ratio float64, direction string, severity AnomalySeverity) string {
	metricLabel := metric
	switch metric {
	case "cpu":
		metricLabel = "CPU usage"
	case "memory":
		metricLabel = "Memory usage"
	case "disk":
		metricLabel = "Disk usage"
	case "network_in":
		metricLabel = "Network inbound"
	case "network_out":
		metricLabel = "Network outbound"
	}
	
	severityLabel := ""
	switch severity {
	case AnomalyCritical:
		severityLabel = "Critical anomaly: "
	case AnomalyHigh:
		severityLabel = "High anomaly: "
	case AnomalyMedium:
		severityLabel = "Moderate anomaly: "
	case AnomalyLow:
		severityLabel = "Minor anomaly: "
	}
	
	return severityLabel + metricLabel + " is " + formatRatio(ratio) + " " + direction + " normal baseline"
}

// formatRatio formats a ratio for display (e.g., 2.5 -> "2.5x")
func formatRatio(ratio float64) string {
	if ratio < 0.01 {
		return "near zero"
	}
	if ratio < 1 {
		return "significantly below"
	}
	if ratio < 1.5 {
		return "slightly above"
	}
	if ratio < 2 {
		return "1.5x"
	}
	if ratio < 3 {
		return "2x"
	}
	if ratio < 5 {
		return "3x"
	}
	return "~" + string([]byte{byte('0' + int(ratio))}) + "x"
}

// GetAllAnomalies checks all resources with current metrics and returns all anomalies
// metricsProvider is a function that returns current metrics for a resource ID
func (s *Store) GetAllAnomalies(metricsProvider func(resourceID string) map[string]float64) []AnomalyReport {
	s.mu.RLock()
	resourceIDs := make([]string, 0, len(s.baselines))
	for id := range s.baselines {
		resourceIDs = append(resourceIDs, id)
	}
	s.mu.RUnlock()
	
	var allAnomalies []AnomalyReport
	for _, resourceID := range resourceIDs {
		metrics := metricsProvider(resourceID)
		if len(metrics) > 0 {
			anomalies := s.CheckResourceAnomalies(resourceID, metrics)
			allAnomalies = append(allAnomalies, anomalies...)
		}
	}
	
	return allAnomalies
}

// ResourceCount returns the number of resources with baselines
func (s *Store) ResourceCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.baselines)
}

// FlatBaseline is a flattened representation of a single metric baseline for API responses
type FlatBaseline struct {
	ResourceID string    `json:"resource_id"`
	Metric     string    `json:"metric"`
	Mean       float64   `json:"mean"`
	StdDev     float64   `json:"std_dev"`
	Min        float64   `json:"min"`
	Max        float64   `json:"max"`
	Samples    int       `json:"samples"`
	LastUpdate time.Time `json:"last_update"`
}

// GetAllBaselines returns all baselines as a flat map for API access
func (s *Store) GetAllBaselines() map[string]*FlatBaseline {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make(map[string]*FlatBaseline)
	for resourceID, rb := range s.baselines {
		for metric, mb := range rb.Metrics {
			key := resourceID + ":" + metric
			fb := &FlatBaseline{
				ResourceID: resourceID,
				Metric:     metric,
				Mean:       mb.Mean,
				StdDev:     mb.StdDev,
				Samples:    mb.SampleCount,
				LastUpdate: rb.LastUpdated,
			}
			// Set min/max from percentiles if available
			if mb.Percentiles != nil {
				if p5, ok := mb.Percentiles[5]; ok {
					fb.Min = p5
				}
				if p95, ok := mb.Percentiles[95]; ok {
					fb.Max = p95
				}
			}
			result[key] = fb
		}
	}
	return result
}

// Save persists baselines to disk
func (s *Store) Save() error {
	if s.dataDir == "" {
		return nil
	}
	
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return s.saveToDisk()
}

// saveToDisk writes baselines to JSON file
func (s *Store) saveToDisk() error {
	if s.dataDir == "" {
		return nil
	}
	
	path := filepath.Join(s.dataDir, "baselines.json")
	
	data, err := json.MarshalIndent(s.baselines, "", "  ")
	if err != nil {
		return err
	}
	
	// Write to temp file first, then rename for atomicity
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	
	return os.Rename(tmpPath, path)
}

// loadFromDisk reads baselines from JSON file
func (s *Store) loadFromDisk() error {
	path := filepath.Join(s.dataDir, "baselines.json")
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No saved baselines yet
		}
		return err
	}
	
	return json.Unmarshal(data, &s.baselines)
}

// Helper functions for statistics

func computeMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func computeStdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := computeMean(values)
	sumSqDiff := 0.0
	for _, v := range values {
		diff := v - mean
		sumSqDiff += diff * diff
	}
	variance := sumSqDiff / float64(len(values)-1) // Sample standard deviation
	return math.Sqrt(variance)
}

func computePercentiles(values []float64) map[int]float64 {
	if len(values) == 0 {
		return nil
	}
	
	// Sort a copy
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	
	percentiles := map[int]float64{
		5:  percentile(sorted, 5),
		25: percentile(sorted, 25),
		50: percentile(sorted, 50),
		75: percentile(sorted, 75),
		95: percentile(sorted, 95),
	}
	
	return percentiles
}

func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	
	// Use linear interpolation
	rank := float64(p) / 100.0 * float64(len(sorted)-1)
	lower := int(rank)
	upper := lower + 1
	
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	
	// Interpolate
	weight := rank - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}
