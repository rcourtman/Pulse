package monitoring

import (
	"math"
)

// lttb performs Largest Triangle Three Buckets downsampling on a slice of
// MetricPoints. It reduces data to targetPoints while preserving the visual
// shape of the data — peaks, valleys and trends are retained.
//
// If len(data) <= targetPoints or targetPoints < 3, data is returned as-is.
func lttb(data []MetricPoint, targetPoints int) []MetricPoint {
	n := len(data)
	if targetPoints >= n || targetPoints < 3 {
		return data
	}

	result := make([]MetricPoint, 0, targetPoints)

	// Always keep the first point.
	result = append(result, data[0])

	bucketSize := float64(n-2) / float64(targetPoints-2)
	prevSelected := 0

	for i := 0; i < targetPoints-2; i++ {
		// Current bucket range.
		bucketStart := int(math.Floor(float64(i)*bucketSize)) + 1
		bucketEnd := int(math.Floor(float64(i+1)*bucketSize)) + 1
		if bucketEnd > n-1 {
			bucketEnd = n - 1
		}

		// Next bucket range — used to compute the "third point" average.
		nextStart := bucketEnd
		nextEnd := int(math.Floor(float64(i+2)*bucketSize)) + 1
		if nextEnd > n-1 {
			nextEnd = n - 1
		}
		if nextStart >= nextEnd {
			nextEnd = nextStart + 1
			if nextEnd > n {
				nextEnd = n
			}
		}

		// Average of next bucket (the "C" vertex of the triangle).
		avgTs := float64(0)
		avgVal := float64(0)
		nextCount := nextEnd - nextStart
		for j := nextStart; j < nextEnd; j++ {
			avgTs += float64(data[j].Timestamp.UnixMilli())
			avgVal += data[j].Value
		}
		avgTs /= float64(nextCount)
		avgVal /= float64(nextCount)

		// Previously selected point (the "A" vertex).
		aTs := float64(data[prevSelected].Timestamp.UnixMilli())
		aVal := data[prevSelected].Value

		// Find the point in the current bucket that maximises the triangle area.
		maxArea := float64(-1)
		bestIdx := bucketStart

		for j := bucketStart; j < bucketEnd; j++ {
			bTs := float64(data[j].Timestamp.UnixMilli())
			bVal := data[j].Value

			// Twice the triangle area (sign doesn't matter, we compare magnitudes).
			area := math.Abs((aTs-avgTs)*(bVal-aVal) - (aTs-bTs)*(avgVal-aVal))
			if area > maxArea {
				maxArea = area
				bestIdx = j
			}
		}

		result = append(result, data[bestIdx])
		prevSelected = bestIdx
	}

	// Always keep the last point.
	result = append(result, data[n-1])

	return result
}
