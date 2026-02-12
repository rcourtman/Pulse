// Package forecast provides trend extrapolation and forecasting capabilities.
package forecast

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// MetricDataPoint represents a single metric observation
type MetricDataPoint struct {
	Timestamp time.Time
	Value     float64
}

// Forecast represents a prediction about a metric's future value
type Forecast struct {
	ResourceID      string         `json:"resource_id"`
	ResourceName    string         `json:"resource_name,omitempty"`
	Metric          string         `json:"metric"`
	CurrentValue    float64        `json:"current_value"`
	PredictedValue  float64        `json:"predicted_value"`
	PredictedAt     time.Time      `json:"predicted_at"` // When the prediction is for
	Confidence      float64        `json:"confidence"`   // 0-1
	Trend           Trend          `json:"trend"`
	TimeToThreshold *time.Duration `json:"time_to_threshold,omitempty"` // Time until threshold breach
	ThresholdValue  float64        `json:"threshold_value,omitempty"`
	Description     string         `json:"description"`
}

// Trend describes the direction and rate of change
type Trend struct {
	Direction    TrendDirection `json:"direction"`
	RatePerHour  float64        `json:"rate_per_hour"` // Change per hour
	RatePerDay   float64        `json:"rate_per_day"`  // Change per day
	Acceleration float64        `json:"acceleration"`  // Is the rate changing?
	Seasonality  *Seasonality   `json:"seasonality,omitempty"`
}

// TrendDirection indicates trend direction
type TrendDirection string

const (
	TrendStable     TrendDirection = "stable"
	TrendIncreasing TrendDirection = "increasing"
	TrendDecreasing TrendDirection = "decreasing"
	TrendVolatile   TrendDirection = "volatile"
)

// Seasonality captures periodic patterns
type Seasonality struct {
	HasDaily  bool `json:"has_daily"`
	HasWeekly bool `json:"has_weekly"`
	// Peak hours (0-23) for daily pattern
	PeakHours []int `json:"peak_hours,omitempty"`
	// Peak days (0=Sun, 6=Sat) for weekly pattern
	PeakDays []int `json:"peak_days,omitempty"`
}

// ForecastConfig configures the forecast service
type ForecastConfig struct {
	// Analysis windows
	ShortTermWindow  time.Duration // For recent trend (default: 1 hour)
	MediumTermWindow time.Duration // For medium trend (default: 24 hours)
	LongTermWindow   time.Duration // For long-term trend (default: 7 days)

	// Prediction horizons
	DefaultHorizon time.Duration // Default prediction horizon (default: 24 hours)
	MaxHorizon     time.Duration // Maximum prediction horizon (default: 30 days)

	// Thresholds
	StableThreshold   float64 // Change per day below this is "stable" (default: 1%)
	VolatileThreshold float64 // Standard deviation above this is "volatile" (default: 15%)
}

// DefaultForecastConfig returns sensible defaults
func DefaultForecastConfig() ForecastConfig {
	return ForecastConfig{
		ShortTermWindow:   1 * time.Hour,
		MediumTermWindow:  24 * time.Hour,
		LongTermWindow:    7 * 24 * time.Hour,
		DefaultHorizon:    24 * time.Hour,
		MaxHorizon:        30 * 24 * time.Hour,
		StableThreshold:   1.0,
		VolatileThreshold: 15.0,
	}
}

// DataProvider provides historical metric data
type DataProvider interface {
	GetMetricHistory(resourceID, metric string, from, to time.Time) ([]MetricDataPoint, error)
}

// StateProvider provides access to current infrastructure state
type StateProvider interface {
	GetState() StateSnapshot
}

// StateSnapshot contains the current infrastructure state (subset of models.StateSnapshot)
type StateSnapshot struct {
	VMs        []VMInfo
	Containers []ContainerInfo
	Nodes      []NodeInfo
	Storage    []StorageInfo
}

// VMInfo contains VM information
type VMInfo struct {
	ID   string
	Name string
}

// ContainerInfo contains container information
type ContainerInfo struct {
	ID   string
	Name string
}

// NodeInfo contains node information
type NodeInfo struct {
	ID   string
	Name string
}

// StorageInfo contains storage information
type StorageInfo struct {
	ID   string
	Name string
}

// Service provides forecasting capabilities
type Service struct {
	mu sync.RWMutex

	config        ForecastConfig
	provider      DataProvider
	stateProvider StateProvider
}

// NewService creates a new forecast service
func NewService(cfg ForecastConfig) *Service {
	if cfg.ShortTermWindow <= 0 {
		cfg.ShortTermWindow = 1 * time.Hour
	}
	if cfg.MediumTermWindow <= 0 {
		cfg.MediumTermWindow = 24 * time.Hour
	}
	if cfg.LongTermWindow <= 0 {
		cfg.LongTermWindow = 7 * 24 * time.Hour
	}
	if cfg.DefaultHorizon <= 0 {
		cfg.DefaultHorizon = 24 * time.Hour
	}
	if cfg.MaxHorizon <= 0 {
		cfg.MaxHorizon = 30 * 24 * time.Hour
	}
	if cfg.StableThreshold <= 0 {
		cfg.StableThreshold = 1.0
	}
	if cfg.VolatileThreshold <= 0 {
		cfg.VolatileThreshold = 15.0
	}

	return &Service{
		config: cfg,
	}
}

// SetDataProvider sets the data provider for historical metrics
func (s *Service) SetDataProvider(provider DataProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider = provider
}

// SetStateProvider sets the state provider for accessing current infrastructure state
func (s *Service) SetStateProvider(sp StateProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateProvider = sp
}

// Forecast generates a prediction for a metric
func (s *Service) Forecast(resourceID, resourceName, metric string, horizon time.Duration, threshold float64) (*Forecast, error) {
	s.mu.RLock()
	provider := s.provider
	cfg := s.config
	s.mu.RUnlock()

	if provider == nil {
		return nil, fmt.Errorf("no data provider configured")
	}

	if horizon <= 0 {
		horizon = cfg.DefaultHorizon
	}
	if horizon > cfg.MaxHorizon {
		horizon = cfg.MaxHorizon
	}

	now := time.Now()

	// Get historical data
	data, err := provider.GetMetricHistory(resourceID, metric, now.Add(-cfg.LongTermWindow), now)
	if err != nil {
		return nil, fmt.Errorf("failed to get metric history: %w", err)
	}

	if len(data) < 2 {
		return nil, fmt.Errorf("insufficient data points for forecast of %s/%s: got %d points, need at least 2", resourceID, metric, len(data))
	}

	// Calculate trend
	trend := s.calculateTrend(data, cfg)

	// Get current value
	currentValue := data[len(data)-1].Value

	// Calculate predicted value
	hoursAhead := horizon.Hours()
	predictedValue := currentValue + (trend.RatePerHour * hoursAhead)

	// Clamp predictions to valid ranges for percentage metrics
	if isPercentageMetric(metric) {
		if predictedValue < 0 {
			predictedValue = 0
		} else if predictedValue > 100 {
			predictedValue = 100
		}
	}

	// Calculate time to threshold if specified
	var timeToThreshold *time.Duration
	if threshold > 0 && trend.RatePerHour != 0 {
		if (trend.RatePerHour > 0 && currentValue < threshold) ||
			(trend.RatePerHour < 0 && currentValue > threshold) {
			hoursUntil := (threshold - currentValue) / trend.RatePerHour
			if hoursUntil > 0 {
				d := time.Duration(hoursUntil * float64(time.Hour))
				timeToThreshold = &d
			}
		}
	}

	// Calculate confidence based on data quality and trend stability
	confidence := s.calculateConfidence(data, trend)

	// Generate description
	description := s.generateDescription(metric, currentValue, predictedValue, trend, timeToThreshold, threshold)

	forecast := &Forecast{
		ResourceID:      resourceID,
		ResourceName:    resourceName,
		Metric:          metric,
		CurrentValue:    currentValue,
		PredictedValue:  predictedValue,
		PredictedAt:     now.Add(horizon),
		Confidence:      confidence,
		Trend:           trend,
		TimeToThreshold: timeToThreshold,
		ThresholdValue:  threshold,
		Description:     description,
	}

	log.Debug().
		Str("resource", resourceID).
		Str("metric", metric).
		Float64("current", currentValue).
		Float64("predicted", predictedValue).
		Str("direction", string(trend.Direction)).
		Float64("confidence", confidence).
		Msg("Generated forecast")

	return forecast, nil
}

// calculateTrend analyzes trend from data points
func (s *Service) calculateTrend(data []MetricDataPoint, cfg ForecastConfig) Trend {
	if len(data) < 2 {
		return Trend{Direction: TrendStable}
	}

	// Sort by timestamp
	sort.Slice(data, func(i, j int) bool {
		return data[i].Timestamp.Before(data[j].Timestamp)
	})

	now := time.Now()

	// Calculate linear regression for overall trend
	slope, _ := linearRegression(data)

	// Convert slope to rate per hour
	ratePerHour := slope * 3600 // slope is per second

	// Calculate acceleration (change in rate)
	// Compare short-term rate to medium-term rate
	shortTermData := filterByWindow(data, now.Add(-cfg.ShortTermWindow), now)
	mediumTermData := filterByWindow(data, now.Add(-cfg.MediumTermWindow), now)

	var acceleration float64
	if len(shortTermData) >= 2 && len(mediumTermData) >= 2 {
		shortSlope, _ := linearRegression(shortTermData)
		mediumSlope, _ := linearRegression(mediumTermData)
		acceleration = (shortSlope - mediumSlope) * 3600 // Difference in rate per hour
	}

	// Calculate volatility (standard deviation)
	stdDev := standardDeviation(data)
	mean := mean(data)
	volatilityPct := 0.0
	if mean != 0 {
		volatilityPct = (stdDev / math.Abs(mean)) * 100
	}

	// Determine direction
	direction := TrendStable
	ratePerDay := ratePerHour * 24

	if volatilityPct > cfg.VolatileThreshold {
		direction = TrendVolatile
	} else if math.Abs(ratePerDay) < cfg.StableThreshold {
		direction = TrendStable
	} else if ratePerDay > 0 {
		direction = TrendIncreasing
	} else {
		direction = TrendDecreasing
	}

	// Detect seasonality
	seasonality := s.detectSeasonality(data)

	return Trend{
		Direction:    direction,
		RatePerHour:  ratePerHour,
		RatePerDay:   ratePerDay,
		Acceleration: acceleration,
		Seasonality:  seasonality,
	}
}

// detectSeasonality looks for periodic patterns
func (s *Service) detectSeasonality(data []MetricDataPoint) *Seasonality {
	if len(data) < 48 { // Need at least 2 days of hourly data
		return nil
	}

	seasonality := &Seasonality{}

	// Group by hour of day
	hourlyAverages := make(map[int][]float64)
	for _, dp := range data {
		hour := dp.Timestamp.Hour()
		hourlyAverages[hour] = append(hourlyAverages[hour], dp.Value)
	}

	// Calculate average per hour
	hourlyMeans := make(map[int]float64)
	for hour, values := range hourlyAverages {
		if len(values) > 0 {
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			hourlyMeans[hour] = sum / float64(len(values))
		}
	}

	// Find peak hours (above overall mean + 1 std dev)
	overallMean := mean(data)
	overallStdDev := standardDeviation(data)
	threshold := overallMean + overallStdDev

	for hour, avg := range hourlyMeans {
		if avg > threshold {
			seasonality.PeakHours = append(seasonality.PeakHours, hour)
			seasonality.HasDaily = true
		}
	}

	// Group by day of week
	dailyAverages := make(map[int][]float64)
	for _, dp := range data {
		day := int(dp.Timestamp.Weekday())
		dailyAverages[day] = append(dailyAverages[day], dp.Value)
	}

	// Calculate average per day
	dailyMeans := make(map[int]float64)
	for day, values := range dailyAverages {
		if len(values) > 0 {
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			dailyMeans[day] = sum / float64(len(values))
		}
	}

	// Find peak days
	for day, avg := range dailyMeans {
		if avg > threshold {
			seasonality.PeakDays = append(seasonality.PeakDays, day)
			seasonality.HasWeekly = true
		}
	}

	if !seasonality.HasDaily && !seasonality.HasWeekly {
		return nil
	}

	return seasonality
}

// calculateConfidence calculates prediction confidence
func (s *Service) calculateConfidence(data []MetricDataPoint, trend Trend) float64 {
	// Base confidence on:
	// 1. Amount of data (more data = higher confidence)
	// 2. Trend stability (stable = higher confidence)
	// 3. Volatility (lower = higher confidence)

	// Data quantity factor (0.3 to 1.0)
	dataFactor := math.Min(1.0, 0.3+float64(len(data))/500.0)

	// Stability factor (0.5 to 1.0)
	stabilityFactor := 0.5
	switch trend.Direction {
	case TrendStable:
		stabilityFactor = 1.0
	case TrendIncreasing, TrendDecreasing:
		stabilityFactor = 0.8
	case TrendVolatile:
		stabilityFactor = 0.5
	}

	// Acceleration factor (high acceleration = lower confidence)
	accelerationFactor := 1.0
	if math.Abs(trend.Acceleration) > 1.0 {
		accelerationFactor = 0.7
	}

	confidence := dataFactor * stabilityFactor * accelerationFactor
	return math.Max(0.1, math.Min(0.95, confidence))
}

// generateDescription creates a human-readable description
func (s *Service) generateDescription(metric string, current, predicted float64, trend Trend, timeToThreshold *time.Duration, threshold float64) string {
	var desc string

	switch trend.Direction {
	case TrendStable:
		desc = fmt.Sprintf("%s is stable at %.1f", metric, current)
	case TrendIncreasing:
		desc = fmt.Sprintf("%s is increasing (%.1f/day)", metric, trend.RatePerDay)
	case TrendDecreasing:
		desc = fmt.Sprintf("%s is decreasing (%.1f/day)", metric, trend.RatePerDay)
	case TrendVolatile:
		desc = fmt.Sprintf("%s is volatile", metric)
	}

	if timeToThreshold != nil && threshold > 0 {
		days := timeToThreshold.Hours() / 24
		if days < 1 {
			hours := timeToThreshold.Hours()
			desc += fmt.Sprintf(". Will reach %.0f%% in %.0f hours", threshold, hours)
		} else if days < 7 {
			desc += fmt.Sprintf(". Will reach %.0f%% in %.0f days", threshold, days)
		} else {
			desc += fmt.Sprintf(". Will reach %.0f%% in %.0f weeks", threshold, days/7)
		}
	}

	if trend.Acceleration > 0.5 {
		desc += " (accelerating)"
	} else if trend.Acceleration < -0.5 {
		desc += " (decelerating)"
	}

	return desc
}

// FormatForContext formats forecasts for AI prompt injection
func (s *Service) FormatForContext(forecasts []*Forecast) string {
	if len(forecasts) == 0 {
		return ""
	}

	result := "\n## Forecasts\n"
	result += "Predicted trends based on historical data:\n\n"

	for _, f := range forecasts {
		result += "- " + f.Description
		if f.Confidence < 0.5 {
			result += " (low confidence)"
		}
		result += "\n"
	}

	return result
}

// FormatKeyForecasts returns formatted forecasts for resources with concerning trends
// This method is for AI patrol context injection - returns empty if no data provider
func (s *Service) FormatKeyForecasts() string {
	s.mu.RLock()
	provider := s.provider
	stateProvider := s.stateProvider
	cfg := s.config
	s.mu.RUnlock()

	if provider == nil || stateProvider == nil {
		return ""
	}

	var concerns []string
	state := stateProvider.GetState()

	// Check VMs for concerning trends
	for _, vm := range state.VMs {
		s.checkResourceForecasts(vm.ID, vm.Name, "vm", cfg, provider, &concerns)
	}

	// Check containers for concerning trends
	for _, ct := range state.Containers {
		s.checkResourceForecasts(ct.ID, ct.Name, "container", cfg, provider, &concerns)
	}

	// Check nodes for concerning trends
	for _, node := range state.Nodes {
		s.checkResourceForecasts(node.ID, node.Name, "node", cfg, provider, &concerns)
	}

	// Check storage for concerning disk trends
	for _, storage := range state.Storage {
		s.checkResourceForecasts(storage.ID, storage.Name, "storage", cfg, provider, &concerns)
	}

	if len(concerns) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Resource Forecasts\n")
	sb.WriteString("Resources with concerning trends:\n\n")
	for _, c := range concerns {
		sb.WriteString("- " + c + "\n")
	}
	return sb.String()
}

// checkResourceForecasts checks a resource for concerning forecast trends
func (s *Service) checkResourceForecasts(resourceID, resourceName, resourceType string, cfg ForecastConfig, provider DataProvider, concerns *[]string) {
	now := time.Now()
	metricsToCheck := []string{"cpu", "memory", "disk"}

	for _, metric := range metricsToCheck {
		data, err := provider.GetMetricHistory(resourceID, metric, now.Add(-cfg.MediumTermWindow), now)
		if err != nil || len(data) < 2 {
			continue
		}

		trend := s.calculateTrend(data, cfg)

		// Check for concerning trends: high rate of increase
		if trend.Direction == TrendIncreasing && math.Abs(trend.RatePerDay) > 5.0 {
			// Check if current value is already elevated and still growing
			currentValue := data[len(data)-1].Value
			if currentValue > 60 { // Already above 60%, and growing fast
				name := resourceName
				if name == "" {
					name = resourceID
				}
				*concerns = append(*concerns, fmt.Sprintf("%s (%s): %s at %.1f%%, increasing %.1f%%/day",
					name, resourceType, metric, currentValue, trend.RatePerDay))
			}
		}

		// Check for approaching threshold (above 80% and still growing)
		if trend.Direction == TrendIncreasing {
			currentValue := data[len(data)-1].Value
			if currentValue > 80 {
				name := resourceName
				if name == "" {
					name = resourceID
				}
				*concerns = append(*concerns, fmt.Sprintf("%s (%s): %s critical at %.1f%%, still increasing",
					name, resourceType, metric, currentValue))
			}
		}
	}
}

// Helper functions

func linearRegression(data []MetricDataPoint) (slope, intercept float64) {
	if len(data) < 2 {
		return 0, 0
	}

	// Use timestamps as x values (seconds since first point)
	firstTime := data[0].Timestamp
	n := float64(len(data))

	var sumX, sumY, sumXY, sumX2 float64
	for _, dp := range data {
		x := dp.Timestamp.Sub(firstTime).Seconds()
		y := dp.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0, sumY / n
	}

	slope = (n*sumXY - sumX*sumY) / denominator
	intercept = (sumY - slope*sumX) / n

	return slope, intercept
}

func filterByWindow(data []MetricDataPoint, from, to time.Time) []MetricDataPoint {
	var result []MetricDataPoint
	for _, dp := range data {
		if (dp.Timestamp.Equal(from) || dp.Timestamp.After(from)) &&
			(dp.Timestamp.Equal(to) || dp.Timestamp.Before(to)) {
			result = append(result, dp)
		}
	}
	return result
}

func mean(data []MetricDataPoint) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, dp := range data {
		sum += dp.Value
	}
	return sum / float64(len(data))
}

func standardDeviation(data []MetricDataPoint) float64 {
	if len(data) < 2 {
		return 0
	}
	avg := mean(data)
	sumSquares := 0.0
	for _, dp := range data {
		diff := dp.Value - avg
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(data)-1))
}

// isPercentageMetric returns true for metrics that should be bounded 0-100%
func isPercentageMetric(metric string) bool {
	metric = strings.ToLower(metric)
	return metric == "cpu" || metric == "memory" || metric == "mem" || metric == "disk"
}

// ForecastOverviewItem represents a single resource forecast for the overview
type ForecastOverviewItem struct {
	ResourceID      string  `json:"resource_id"`
	ResourceName    string  `json:"resource_name"`
	ResourceType    string  `json:"resource_type"`
	Metric          string  `json:"metric"`
	CurrentValue    float64 `json:"current_value"`
	PredictedValue  float64 `json:"predicted_value"`
	TimeToThreshold *int64  `json:"time_to_threshold"` // seconds, null if won't breach
	Confidence      float64 `json:"confidence"`
	Trend           string  `json:"trend"` // increasing, decreasing, stable, volatile
}

// ForecastOverviewResponse is the response structure for ForecastAll
type ForecastOverviewResponse struct {
	Forecasts    []*ForecastOverviewItem `json:"forecasts"`
	Metric       string                  `json:"metric"`
	Threshold    float64                 `json:"threshold"`
	HorizonHours int                     `json:"horizon_hours"`
}

// MinConfidenceThreshold is the minimum confidence required to include a forecast
const MinConfidenceThreshold = 0.3

// ForecastAll generates forecasts for all resources and returns them sorted by urgency.
// For disk capacity planning, this only returns actionable forecasts:
// - Trend must be "increasing" (disk is actually growing)
// - Time to threshold must not be nil (will actually breach threshold)
// - Confidence must be above minimum threshold
func (s *Service) ForecastAll(metric string, horizon time.Duration, threshold float64) (*ForecastOverviewResponse, error) {
	s.mu.RLock()
	provider := s.provider
	stateProvider := s.stateProvider
	s.mu.RUnlock()

	if provider == nil {
		return nil, fmt.Errorf("no data provider configured")
	}

	if stateProvider == nil {
		return nil, fmt.Errorf("no state provider configured")
	}

	state := stateProvider.GetState()
	var items []*ForecastOverviewItem

	// Helper to add forecast if it's actionable
	addIfActionable := func(forecast *Forecast, resourceType string) {
		// Only include forecasts with increasing trend
		if forecast.Trend.Direction != TrendIncreasing {
			return
		}
		// Only include forecasts that will breach threshold
		if forecast.TimeToThreshold == nil {
			return
		}
		// Only include forecasts with sufficient confidence
		if forecast.Confidence < MinConfidenceThreshold {
			return
		}
		items = append(items, forecastToOverviewItem(forecast, resourceType))
	}

	// Process VMs
	for _, vm := range state.VMs {
		forecast, err := s.Forecast(vm.ID, vm.Name, metric, horizon, threshold)
		if err != nil {
			log.Debug().Str("resource", vm.ID).Str("metric", metric).Err(err).Msg("Skipping VM forecast")
			continue
		}
		addIfActionable(forecast, "vm")
	}

	// Process Containers
	for _, ct := range state.Containers {
		forecast, err := s.Forecast(ct.ID, ct.Name, metric, horizon, threshold)
		if err != nil {
			log.Debug().Str("resource", ct.ID).Str("metric", metric).Err(err).Msg("Skipping container forecast")
			continue
		}
		addIfActionable(forecast, "lxc")
	}

	// Process Nodes
	for _, node := range state.Nodes {
		forecast, err := s.Forecast(node.ID, node.Name, metric, horizon, threshold)
		if err != nil {
			log.Debug().Str("resource", node.ID).Str("metric", metric).Err(err).Msg("Skipping node forecast")
			continue
		}
		addIfActionable(forecast, "node")
	}

	// Sort by TimeToThreshold ascending (most urgent first)
	sort.Slice(items, func(i, j int) bool {
		// Both should have TimeToThreshold at this point (filtered above)
		// but handle nil defensively
		if items[i].TimeToThreshold == nil && items[j].TimeToThreshold == nil {
			return false
		}
		if items[i].TimeToThreshold == nil {
			return false
		}
		if items[j].TimeToThreshold == nil {
			return true
		}
		return *items[i].TimeToThreshold < *items[j].TimeToThreshold
	})

	return &ForecastOverviewResponse{
		Forecasts:    items,
		Metric:       metric,
		Threshold:    threshold,
		HorizonHours: int(horizon.Hours()),
	}, nil
}

// forecastToOverviewItem converts a Forecast to ForecastOverviewItem
func forecastToOverviewItem(f *Forecast, resourceType string) *ForecastOverviewItem {
	var timeToThreshold *int64
	if f.TimeToThreshold != nil {
		secs := int64(f.TimeToThreshold.Seconds())
		timeToThreshold = &secs
	}

	return &ForecastOverviewItem{
		ResourceID:      f.ResourceID,
		ResourceName:    f.ResourceName,
		ResourceType:    resourceType,
		Metric:          f.Metric,
		CurrentValue:    f.CurrentValue,
		PredictedValue:  f.PredictedValue,
		TimeToThreshold: timeToThreshold,
		Confidence:      f.Confidence,
		Trend:           string(f.Trend.Direction),
	}
}
