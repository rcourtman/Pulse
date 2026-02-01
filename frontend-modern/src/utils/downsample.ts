/**
 * LTTB (Largest Triangle Three Buckets) Downsampling Algorithm
 * 
 * This algorithm reduces the number of data points while preserving the visual
 * characteristics of the time-series data. It's the gold standard for chart
 * downsampling because it keeps peaks, valleys, and significant changes.
 * 
 * Reference: Sveinn Steinarsson's 2013 thesis
 * "Downsampling Time Series for Visual Representation"
 */

export interface TimeSeriesPoint {
    timestamp: number;
    value: number;
    // Optional: preserve additional data
    min?: number;
    max?: number;
}

/**
 * Downsample time-series data using the LTTB algorithm
 * 
 * @param data - Array of data points with timestamp and value
 * @param targetPoints - Desired number of output points
 * @returns Downsampled array preserving visual characteristics
 */
export function downsampleLTTB<T extends TimeSeriesPoint>(
    data: T[],
    targetPoints: number
): T[] {
    // If data is already small enough, return as-is
    if (data.length <= targetPoints || targetPoints < 3) {
        return data;
    }

    const result: T[] = [];

    // Always keep the first point
    result.push(data[0]);

    // Calculate bucket size (excluding first and last points)
    const bucketSize = (data.length - 2) / (targetPoints - 2);

    let lastSelectedIndex = 0;

    for (let i = 0; i < targetPoints - 2; i++) {
        // Calculate bucket boundaries
        const bucketStart = Math.floor((i + 0) * bucketSize) + 1;
        const bucketEnd = Math.floor((i + 1) * bucketSize) + 1;

        // Calculate the average point of the next bucket (for triangle calculation)
        const nextBucketStart = Math.floor((i + 1) * bucketSize) + 1;
        const nextBucketEnd = Math.min(
            Math.floor((i + 2) * bucketSize) + 1,
            data.length - 1
        );

        let avgX = 0;
        let avgY = 0;
        let avgCount = 0;

        for (let j = nextBucketStart; j < nextBucketEnd; j++) {
            avgX += data[j].timestamp;
            avgY += data[j].value;
            avgCount++;
        }

        if (avgCount > 0) {
            avgX /= avgCount;
            avgY /= avgCount;
        }

        // Find the point in the current bucket that creates the largest triangle
        const pointA = data[lastSelectedIndex];
        let maxArea = -1;
        let maxAreaIndex = bucketStart;

        for (let j = bucketStart; j < bucketEnd && j < data.length - 1; j++) {
            const pointB = data[j];

            // Calculate triangle area using the cross product formula
            // Area = 0.5 * |x1(y2 - y3) + x2(y3 - y1) + x3(y1 - y2)|
            const area = Math.abs(
                (pointA.timestamp - avgX) * (pointB.value - pointA.value) -
                (pointA.timestamp - pointB.timestamp) * (avgY - pointA.value)
            );

            if (area > maxArea) {
                maxArea = area;
                maxAreaIndex = j;
            }
        }

        result.push(data[maxAreaIndex]);
        lastSelectedIndex = maxAreaIndex;
    }

    // Always keep the last point
    result.push(data[data.length - 1]);

    return result;
}

/**
 * Downsample metric snapshots (for Sparkline component)
 * Works with the MetricSnapshot interface from metricsHistory
 */
export interface MetricSnapshotLike {
    timestamp: number;
    cpu: number;
    memory: number;
    disk: number;
}

/**
 * Downsample metric snapshots for a specific metric
 * 
 * @param data - Array of MetricSnapshot data
 * @param metric - Which metric to use for LTTB selection ('cpu', 'memory', 'disk')
 * @param targetPoints - Desired number of output points
 * @returns Downsampled array
 */
export function downsampleMetricSnapshots<T extends MetricSnapshotLike>(
    data: T[],
    metric: 'cpu' | 'memory' | 'disk',
    targetPoints: number
): T[] {
    if (data.length <= targetPoints || targetPoints < 3) {
        return data;
    }

    // Convert to TimeSeriesPoint format for LTTB
    const points: (TimeSeriesPoint & { originalIndex: number })[] = data.map((d, i) => ({
        timestamp: d.timestamp,
        value: d[metric],
        originalIndex: i
    }));

    // Run LTTB
    const downsampled = downsampleLTTB(points, targetPoints);

    // Return original data at selected indices
    return downsampled.map(p => data[p.originalIndex]);
}

/**
 * Calculate optimal number of points for a given pixel width
 * 
 * Best practices:
 * - For sparklines: ~1 point per 2 pixels (max ~80 points for small sparklines)
 * - For larger charts: ~1 point per 2-3 pixels
 * - Never more than width/2 points (overplotting provides no visual benefit)
 * 
 * @param widthPx - Width of the chart in CSS pixels
 * @param chartType - Type of chart ('sparkline' | 'history')
 * @returns Optimal number of data points
 */
export function calculateOptimalPoints(
    widthPx: number,
    chartType: 'sparkline' | 'history' = 'history'
): number {
    if (chartType === 'sparkline') {
        // Sparklines are tiny - 1 point per 1.5-2 pixels is plenty
        // Min 20 points to show a trend, max 100 for small sparklines
        const points = Math.floor(widthPx / 1.5);
        return Math.min(100, Math.max(20, points));
    }

    // History charts - 1 point per 2 pixels is optimal
    // This provides smooth curves without overplotting
    // Min 60 points for basic trends, max 600 for very wide screens
    const points = Math.floor(widthPx / 2);
    return Math.min(600, Math.max(60, points));
}
