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
	Mean        float64         `json:"mean"`         // Average value
	StdDev      float64         `json:"stddev"`       // Standard deviation
	Percentiles map[int]float64 `json:"percentiles"`  // 5, 25, 50, 75, 95
	SampleCount int             `json:"sample_count"` // Number of samples used

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

// AnomalyCallback is called when an anomaly is detected
// resourceID: the resource with the anomaly
// metric: the metric that's anomalous (cpu, memory, disk, etc)
// severity: how severe the anomaly is
// value: the current value
// baseline: the expected baseline mean
type AnomalyCallback func(resourceID, resourceType, metric string, severity AnomalySeverity, value, baseline float64)

// Store manages baseline storage and learning
type Store struct {
	mu        sync.RWMutex
	baselines map[string]*ResourceBaseline // resourceID -> baseline

	// Configuration
	learningWindow time.Duration // How far back to learn from (default: 7 days)
	minSamples     int           // Minimum samples needed (default: 50)
	updateInterval time.Duration // How often to recompute (default: 1 hour)

	// Persistence
	dataDir     string
	persistence Persistence

	// Event-driven callbacks
	onAnomaly AnomalyCallback // Called when an anomaly is detected
}

// Persistence interface for saving/loading baselines
type Persistence interface {
	Save(baselines map[string]*ResourceBaseline) error
	Load() (map[string]*ResourceBaseline, error)
}

// StoreConfig configures the baseline store
type StoreConfig struct {
	LearningWindow time.Duration
	MinSamples     int
	UpdateInterval time.Duration
	DataDir        string
}

// DefaultConfig returns sensible defaults
func DefaultConfig() StoreConfig {
	return StoreConfig{
		LearningWindow: 14 * 24 * time.Hour, // 14 days to capture weekly patterns
		MinSamples:     50,
		UpdateInterval: 1 * time.Hour,
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

// SetAnomalyCallback sets the callback function to be called when anomalies are detected.
// This enables event-driven responses to anomalies, such as triggering targeted patrols.
func (s *Store) SetAnomalyCallback(callback AnomalyCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onAnomaly = callback
	if callback != nil {
		log.Info().Msg("Baseline store: Anomaly callback configured")
	}
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

	// Calculate absolute difference
	absDiff := math.Abs(value - baseline.Mean)

	// Don't flag small absolute changes as anomalies
	if absDiff < 3.0 {
		return false, 0
	}

	if baseline.StdDev == 0 {
		// No variance - only flag if change is significant (> 5 percentage points)
		if absDiff > 5.0 {
			return true, 0 // No valid z-score when stddev is 0
		}
		return false, 0
	}

	// Apply minimum stddev floor
	effectiveStdDev := baseline.StdDev
	if effectiveStdDev < 1.0 {
		effectiveStdDev = 1.0
	}

	zScore := (value - baseline.Mean) / effectiveStdDev

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

	// Calculate absolute difference for threshold checks
	absDiff := math.Abs(value - baseline.Mean)

	// Handle zero stddev case more intelligently
	// When values have been completely stable, small variations aren't anomalies
	if baseline.StdDev == 0 {
		// Only flag as anomaly if the absolute difference is significant
		// For percentage metrics (cpu, memory, disk), require > 5 percentage point change
		// This prevents false positives from small floating point variations
		if absDiff < 5.0 {
			return AnomalyNone, 0, baseline
		}
		// Even with large difference, just flag as warning, not critical
		// since we don't have historical variance data to judge severity
		return AnomalyMedium, 0, baseline
	}

	// Apply minimum stddev floor to prevent tiny variations from appearing extreme
	// If historical stddev is < 1%, use 1% as the floor for z-score calculation
	effectiveStdDev := baseline.StdDev
	if effectiveStdDev < 1.0 {
		effectiveStdDev = 1.0
	}

	zScore := (value - baseline.Mean) / effectiveStdDev
	absZ := math.Abs(zScore)

	// Also require a minimum absolute difference for practical significance
	// Don't flag anomalies for changes < 3 percentage points regardless of z-score
	if absDiff < 3.0 {
		return AnomalyNone, zScore, baseline
	}

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
	ResourceID     string          `json:"resource_id"`
	ResourceName   string          `json:"resource_name,omitempty"`
	ResourceType   string          `json:"resource_type,omitempty"`
	Metric         string          `json:"metric"`
	CurrentValue   float64         `json:"current_value"`
	BaselineMean   float64         `json:"baseline_mean"`
	BaselineStdDev float64         `json:"baseline_std_dev"`
	ZScore         float64         `json:"z_score"`
	Severity       AnomalySeverity `json:"severity"`
	Description    string          `json:"description"`
}

// CheckResourceAnomalies checks multiple metrics for a resource and returns all anomalies.
// This function also fires the anomaly callback for event-driven processing.
// For read-only checks (e.g., API endpoints), use CheckResourceAnomaliesReadOnly instead.
func (s *Store) CheckResourceAnomalies(resourceID string, metrics map[string]float64) []AnomalyReport {
	return s.checkResourceAnomaliesInternal(resourceID, metrics, true)
}

// CheckResourceAnomaliesReadOnly checks multiple metrics for a resource and returns all anomalies.
// Unlike CheckResourceAnomalies, this does NOT fire the anomaly callback, making it safe
// for use in read-only contexts like API GET endpoints.
func (s *Store) CheckResourceAnomaliesReadOnly(resourceID string, metrics map[string]float64) []AnomalyReport {
	return s.checkResourceAnomaliesInternal(resourceID, metrics, false)
}

// checkResourceAnomaliesInternal is the internal implementation that optionally fires callbacks
func (s *Store) checkResourceAnomaliesInternal(resourceID string, metrics map[string]float64, fireCallback bool) []AnomalyReport {
	var anomalies []AnomalyReport

	// Get resource type from baseline if available
	s.mu.RLock()
	resourceType := ""
	if rb, exists := s.baselines[resourceID]; exists {
		resourceType = rb.ResourceType
	}
	var onAnomaly AnomalyCallback
	if fireCallback {
		onAnomaly = s.onAnomaly
	}
	s.mu.RUnlock()

	for metric, value := range metrics {
		severity, zScore, baseline := s.CheckAnomaly(resourceID, metric, value)
		if severity != AnomalyNone && baseline != nil {
			// Compute ratio: current value / baseline mean
			ratio := value / baseline.Mean

			// Apply metric-specific filters to reduce noise
			// Different metrics have different thresholds for what's "actionable"
			shouldReport := false

			switch metric {
			case "disk":
				// Disk is critical - report if:
				// 1. Usage is above 85% (absolute threshold), OR
				// 2. Usage increased by more than 15 percentage points from baseline
				if value >= 85.0 || (value-baseline.Mean) >= 15.0 {
					shouldReport = true
				}

			case "cpu":
				// CPU fluctuates a lot - only report if:
				// 1. Current usage is above 70% (actually busy), AND
				// 2. It's at least 2x above baseline
				if value >= 70.0 && ratio >= 2.0 {
					shouldReport = true
				}

			case "memory":
				// Memory is more stable - report if:
				// 1. Current usage is above 80% (getting tight), OR
				// 2. It's at least 1.5x above baseline AND above 60%
				if value >= 80.0 || (ratio >= 1.5 && value >= 60.0) {
					shouldReport = true
				}

			default:
				// For other metrics (network, etc), use 2x threshold
				if ratio >= 2.0 || ratio <= 0.5 {
					shouldReport = true
				}
			}

			if !shouldReport {
				continue
			}

			report := AnomalyReport{
				ResourceID:     resourceID,
				Metric:         metric,
				CurrentValue:   value,
				ZScore:         zScore,
				Severity:       severity,
				BaselineMean:   baseline.Mean,
				BaselineStdDev: baseline.StdDev,
			}

			// Generate human-readable description
			direction := "above"
			if zScore < 0 {
				direction = "below"
			}
			report.Description = formatAnomalyDescription(metric, ratio, direction, severity)
			report.ResourceType = resourceType

			anomalies = append(anomalies, report)

			// Trigger anomaly callback for event-driven processing
			if onAnomaly != nil {
				go onAnomaly(resourceID, resourceType, metric, severity, value, baseline.Mean)
			}
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

// TrendPrediction represents a forecast for when a resource might be exhausted
type TrendPrediction struct {
	ResourceID     string  `json:"resource_id"`
	ResourceName   string  `json:"resource_name,omitempty"`
	ResourceType   string  `json:"resource_type,omitempty"`
	Metric         string  `json:"metric"`
	CurrentValue   float64 `json:"current_value"` // Current % usage
	DailyChange    float64 `json:"daily_change"`  // Average change per day
	DaysToFull     int     `json:"days_to_full"`  // Estimated days until 100% (or -1 if decreasing/stable)
	Severity       string  `json:"severity"`      // "critical", "warning", "info"
	Description    string  `json:"description"`
	ConfidenceNote string  `json:"confidence_note,omitempty"`
}

// CalculateTrend analyzes a time series of values and predicts future exhaustion
// samples should be ordered oldest to newest, with at least 2 days of data
// currentValue is the current percentage usage (0-100)
// capacity represents 100% (for percentage-based predictions)
func CalculateTrend(samples []float64, currentValue float64) *TrendPrediction {
	if len(samples) < 5 {
		return nil // Not enough data for meaningful trend
	}

	// Simple linear regression to find slope
	n := float64(len(samples))

	// Calculate means
	sumX := 0.0
	sumY := 0.0
	for i, v := range samples {
		sumX += float64(i)
		sumY += v
	}
	meanX := sumX / n
	meanY := sumY / n

	// Calculate slope (least squares)
	numerator := 0.0
	denominator := 0.0
	for i, v := range samples {
		x := float64(i)
		numerator += (x - meanX) * (v - meanY)
		denominator += (x - meanX) * (x - meanX)
	}

	slope := numerator / denominator

	// slope is change per sample, convert to daily change
	// Assume samples are taken regularly; if 24 samples per day, divide by 24
	// For now, assume hourly samples = 24 per day
	samplesPerDay := 24.0
	dailyChange := slope * samplesPerDay

	prediction := &TrendPrediction{
		CurrentValue: currentValue,
		DailyChange:  dailyChange,
	}

	// Calculate days to full if trending upward
	if dailyChange > 0.1 { // More than 0.1% increase per day
		remaining := 100.0 - currentValue
		if remaining > 0 {
			daysToFull := remaining / dailyChange
			prediction.DaysToFull = int(math.Ceil(daysToFull))

			// Set severity based on time to full
			if prediction.DaysToFull <= 7 {
				prediction.Severity = "critical"
				prediction.Description = formatTrendDescription(prediction.DaysToFull, dailyChange, "critical")
			} else if prediction.DaysToFull <= 30 {
				prediction.Severity = "warning"
				prediction.Description = formatTrendDescription(prediction.DaysToFull, dailyChange, "warning")
			} else {
				prediction.Severity = "info"
				prediction.Description = formatTrendDescription(prediction.DaysToFull, dailyChange, "info")
			}
		} else {
			prediction.DaysToFull = 0
			prediction.Severity = "critical"
			prediction.Description = "Resource at capacity"
		}
	} else if dailyChange < -0.1 {
		// Decreasing trend
		prediction.DaysToFull = -1
		prediction.Severity = "info"
		daysToEmpty := currentValue / (-dailyChange)
		prediction.Description = "Usage declining - at current rate, will reach 0% in " + formatDays(int(math.Ceil(daysToEmpty)))
	} else {
		// Stable
		prediction.DaysToFull = -1
		prediction.Severity = "info"
		prediction.Description = "Usage stable - no significant trend detected"
	}

	return prediction
}

// formatTrendDescription creates human-readable trend descriptions
func formatTrendDescription(daysToFull int, dailyChange float64, severity string) string {
	timeFrame := formatDays(daysToFull)
	changeDesc := ""
	if dailyChange >= 1 {
		changeDesc = " (+" + floatToStr(dailyChange, 1) + "% per day)"
	} else {
		changeDesc = " (+" + floatToStr(dailyChange, 2) + "% per day)"
	}

	switch severity {
	case "critical":
		return "⚠️ Resource will be full in " + timeFrame + changeDesc
	case "warning":
		return "Resource approaching capacity - full in " + timeFrame + changeDesc
	default:
		return "Trending toward full in " + timeFrame + changeDesc
	}
}

// formatDays converts days to human readable format
func formatDays(days int) string {
	if days <= 0 {
		return "now"
	}
	if days == 1 {
		return "1 day"
	}
	if days < 7 {
		return string([]byte{'0' + byte(days)}) + " days"
	}
	if days < 14 {
		return "~1 week"
	}
	if days < 30 {
		weeks := days / 7
		return "~" + string([]byte{'0' + byte(weeks)}) + " weeks"
	}
	months := days / 30
	if months == 1 {
		return "~1 month"
	}
	if months < 12 {
		return "~" + string([]byte{'0' + byte(months)}) + " months"
	}
	return ">1 year"
}

// floatToStr converts float to string with given precision
func floatToStr(f float64, precision int) string {
	// Simple implementation for small numbers
	intPart := int(f)
	fracPart := f - float64(intPart)

	if precision == 1 {
		fracPart = math.Round(fracPart*10) / 10
		if fracPart < 0.1 {
			return string([]byte{'0' + byte(intPart)})
		}
		d := byte('0' + int(fracPart*10))
		return string([]byte{'0' + byte(intPart), '.', d})
	}

	fracPart = math.Round(fracPart*100) / 100
	if fracPart < 0.01 {
		return string([]byte{'0' + byte(intPart)})
	}
	d1 := byte('0' + int(fracPart*10))
	d2 := byte('0' + int(fracPart*100)%10)
	return string([]byte{'0' + byte(intPart), '.', d1, d2})
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
