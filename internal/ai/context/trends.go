package context

import (
	"math"
	"sort"
	"time"
)

// ComputeTrend calculates trend from historical data points.
// This is the core function that transforms raw metrics into meaningful insights.
func ComputeTrend(points []MetricPoint, metricName string, period time.Duration) Trend {
	trend := Trend{
		Metric:     metricName,
		Direction:  TrendStable,
		Period:     period,
		DataPoints: len(points),
	}

	if len(points) < 2 {
		trend.Confidence = 0
		return trend
	}

	// Sort by timestamp to ensure correct order
	sorted := make([]MetricPoint, len(points))
	copy(sorted, points)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	// Calculate basic statistics
	stats := computeStats(sorted)
	trend.Average = stats.Mean
	trend.Min = stats.Min
	trend.Max = stats.Max
	trend.StdDev = stats.StdDev
	trend.Current = sorted[len(sorted)-1].Value

	// Perform linear regression to get slope and fit quality
	regression := linearRegression(sorted)
	trend.Confidence = regression.R2

	// Convert slope from "per second" to "per hour" and "per day"
	// Slope is in units/second
	trend.RatePerHour = regression.Slope * 3600
	trend.RatePerDay = regression.Slope * 86400

	// Classify the trend direction
	trend.Direction = classifyTrend(regression.Slope, stats.Mean, stats.StdDev)

	return trend
}

// computeStats calculates basic statistics for a set of metric points
func computeStats(points []MetricPoint) Stats {
	if len(points) == 0 {
		return Stats{}
	}

	stats := Stats{
		Count: len(points),
		Min:   points[0].Value,
		Max:   points[0].Value,
	}

	for _, p := range points {
		stats.Sum += p.Value
		if p.Value < stats.Min {
			stats.Min = p.Value
		}
		if p.Value > stats.Max {
			stats.Max = p.Value
		}
	}

	stats.Mean = stats.Sum / float64(stats.Count)

	// Calculate standard deviation
	var sumSquares float64
	for _, p := range points {
		diff := p.Value - stats.Mean
		sumSquares += diff * diff
	}
	stats.StdDev = math.Sqrt(sumSquares / float64(stats.Count))

	return stats
}

// linearRegression performs simple linear regression on time-series data.
// Returns slope (change per second), intercept, and R² (goodness of fit).
func linearRegression(points []MetricPoint) LinearRegressionResult {
	if len(points) < 2 {
		return LinearRegressionResult{}
	}

	n := float64(len(points))
	
	// Use time relative to first point for numerical stability
	baseTime := points[0].Timestamp

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for _, p := range points {
		x := p.Timestamp.Sub(baseTime).Seconds() // seconds since start
		y := p.Value

		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}

	// Calculate slope and intercept using least squares
	denominator := n*sumX2 - sumX*sumX
	if math.Abs(denominator) < 1e-10 {
		// All x values are the same (no time span)
		return LinearRegressionResult{R2: 0}
	}

	slope := (n*sumXY - sumX*sumY) / denominator
	intercept := (sumY - slope*sumX) / n

	// Calculate R² (coefficient of determination)
	meanY := sumY / n
	var ssRes, ssTot float64 // Sum of squares residual and total
	for _, p := range points {
		x := p.Timestamp.Sub(baseTime).Seconds()
		yPred := slope*x + intercept
		ssRes += (p.Value - yPred) * (p.Value - yPred)
		ssTot += (p.Value - meanY) * (p.Value - meanY)
	}

	r2 := 0.0
	if ssTot > 0 {
		r2 = 1 - (ssRes / ssTot)
	}
	// Clamp R² to [0, 1] (can be negative for very bad fits)
	if r2 < 0 {
		r2 = 0
	}

	return LinearRegressionResult{
		Slope:     slope,
		Intercept: intercept,
		R2:        r2,
	}
}

// classifyTrend determines the trend direction based on slope and statistics.
// We normalize the slope relative to the metric's magnitude to avoid
// false positives on high-value metrics.
func classifyTrend(slopePerSecond, mean, stdDev float64) TrendDirection {
	// If there's no significant variation, it's stable
	if stdDev < 0.01 && math.Abs(slopePerSecond) < 1e-10 {
		return TrendStable
	}

	// If standard deviation is high relative to mean, it's volatile
	if mean > 0 && stdDev/mean > 0.3 {
		return TrendVolatile
	}

	// Convert slope to hourly rate for easier reasoning
	hourlyRate := slopePerSecond * 3600

	// Determine significance threshold based on the metric's scale
	// For percentage metrics (0-100), we care about ~0.1% per hour (~2.4% per day)
	// This catches slow-growing issues before they become critical
	// For absolute metrics, we care about ~0.5% of mean per hour
	threshold := 0.1 // Default threshold for percentage metrics
	if mean > 100 {
		// For larger absolute values, use relative threshold
		threshold = mean * 0.005
	}

	// Check if the hourly change is significant
	if hourlyRate > threshold {
		return TrendGrowing
	}
	if hourlyRate < -threshold {
		return TrendDeclining
	}

	return TrendStable
}

// ComputePercentiles calculates percentile values from a sorted slice of points
func ComputePercentiles(points []MetricPoint, percentiles ...int) map[int]float64 {
	result := make(map[int]float64)
	if len(points) == 0 {
		return result
	}

	// Extract values and sort
	values := make([]float64, len(points))
	for i, p := range points {
		values[i] = p.Value
	}
	sort.Float64s(values)

	for _, p := range percentiles {
		if p < 0 || p > 100 {
			continue
		}
		
		// Calculate index for percentile
		idx := float64(p) / 100.0 * float64(len(values)-1)
		lower := int(math.Floor(idx))
		upper := int(math.Ceil(idx))
		
		if lower >= len(values) {
			lower = len(values) - 1
		}
		if upper >= len(values) {
			upper = len(values) - 1
		}
		
		if lower == upper {
			result[p] = values[lower]
		} else {
			// Linear interpolation between adjacent values
			frac := idx - float64(lower)
			result[p] = values[lower]*(1-frac) + values[upper]*frac
		}
	}

	return result
}

// TrendSummary generates a human-readable summary of a trend
func TrendSummary(t Trend) string {
	if t.DataPoints < 2 {
		return "insufficient data"
	}

	directionStr := ""
	switch t.Direction {
	case TrendGrowing:
		directionStr = "growing"
	case TrendDeclining:
		directionStr = "declining"
	case TrendVolatile:
		directionStr = "volatile"
	case TrendStable:
		directionStr = "stable"
	}

	// Format rate based on magnitude
	rateStr := ""
	if t.Direction == TrendGrowing || t.Direction == TrendDeclining {
		absRate := math.Abs(t.RatePerDay)
		if absRate > 1 {
			rateStr = formatFloat(absRate, 1) + "/day"
		} else {
			rateStr = formatFloat(math.Abs(t.RatePerHour), 2) + "/hr"
		}
	}

	if rateStr != "" {
		return directionStr + " " + rateStr
	}
	return directionStr
}

// formatFloat formats a float with the given precision, trimming trailing zeros
func formatFloat(v float64, precision int) string {
	return trimTrailingZeros(floatToString(v, precision))
}

func floatToString(v float64, precision int) string {
	switch precision {
	case 0:
		return intToString(int(math.Round(v)))
	case 1:
		return intToString(int(v)) + "." + intToString(int(math.Round((v-float64(int(v)))*10)))
	case 2:
		return intToString(int(v)) + "." + padLeft(intToString(int(math.Round((v-float64(int(v)))*100))), 2, '0')
	default:
		mult := math.Pow(10, float64(precision))
		return intToString(int(v)) + "." + padLeft(intToString(int(math.Round((v-float64(int(v)))*mult))), precision, '0')
	}
}

func intToString(i int) string {
	if i < 0 {
		return "-" + intToString(-i)
	}
	if i < 10 {
		return string(rune('0' + i))
	}
	return intToString(i/10) + string(rune('0'+i%10))
}

func padLeft(s string, length int, pad rune) string {
	for len(s) < length {
		s = string(pad) + s
	}
	return s
}

func trimTrailingZeros(s string) string {
	if s == "" {
		return s
	}
	// Find decimal point
	dotIdx := -1
	for i, c := range s {
		if c == '.' {
			dotIdx = i
			break
		}
	}
	if dotIdx == -1 {
		return s // No decimal point
	}
	
	// Trim trailing zeros after decimal
	end := len(s)
	for end > dotIdx+1 && s[end-1] == '0' {
		end--
	}
	// Also trim decimal if nothing after it
	if end == dotIdx+1 {
		end = dotIdx
	}
	return s[:end]
}
