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
 * Downsample time-series data using a temporal LTTB algorithm.
 *
 * Standard LTTB divides the input into equal-count index buckets, which
 * distorts representation when data density is non-uniform (e.g. tiered
 * timestamps with sparse old data and dense recent data). This variant
 * divides the *time range* into equal-duration buckets so each output
 * point covers the same amount of wall-clock time. Within each bucket
 * it still picks the point that maximises the triangle area (the core
 * LTTB visual-fidelity property). Empty buckets are skipped, so the
 * output may contain fewer than targetPoints when data is very sparse.
 *
 * @param data - Array of data points sorted by timestamp
 * @param targetPoints - Desired number of output points
 * @returns Downsampled array preserving visual characteristics
 */
export function downsampleLTTB<T extends TimeSeriesPoint>(data: T[], targetPoints: number): T[] {
  if (data.length <= targetPoints || targetPoints < 3) {
    return data;
  }

  const result: T[] = [];
  result.push(data[0]);

  const numBuckets = targetPoints - 2;
  const timeStart = data[0].timestamp;
  const timeEnd = data[data.length - 1].timestamp;
  const bucketDuration = (timeEnd - timeStart) / numBuckets;

  // Build temporal bucket index ranges. Only interior points (indices
  // 1..length-2) are bucketed; first and last are always kept.
  const bucketRanges: [number, number][] = new Array(numBuckets);
  let idx = 1;
  for (let b = 0; b < numBuckets; b++) {
    const tEnd =
      b === numBuckets - 1
        ? Infinity // last bucket collects all remaining interior points
        : timeStart + (b + 1) * bucketDuration;
    const start = idx;
    while (idx < data.length - 1 && data[idx].timestamp < tEnd) {
      idx++;
    }
    bucketRanges[b] = [start, idx];
  }

  let lastSelectedIndex = 0;

  for (let i = 0; i < numBuckets; i++) {
    const [bucketStart, bucketEnd] = bucketRanges[i];
    if (bucketStart >= bucketEnd) continue; // empty bucket — no data in this time slice

    // Average of the next non-empty bucket (triangle vertex C).
    let avgX = data[data.length - 1].timestamp;
    let avgY = data[data.length - 1].value;
    for (let nb = i + 1; nb < numBuckets; nb++) {
      const [ns, ne] = bucketRanges[nb];
      if (ns >= ne) continue;
      avgX = 0;
      avgY = 0;
      for (let j = ns; j < ne; j++) {
        avgX += data[j].timestamp;
        avgY += data[j].value;
      }
      avgX /= ne - ns;
      avgY /= ne - ns;
      break;
    }

    // Pick the point that creates the largest triangle (LTTB core).
    const pointA = data[lastSelectedIndex];
    let maxArea = -1;
    let maxAreaIndex = bucketStart;

    for (let j = bucketStart; j < bucketEnd; j++) {
      const pointB = data[j];
      const area = Math.abs(
        (pointA.timestamp - avgX) * (pointB.value - pointA.value) -
          (pointA.timestamp - pointB.timestamp) * (avgY - pointA.value),
      );
      if (area > maxArea) {
        maxArea = area;
        maxAreaIndex = j;
      }
    }

    result.push(data[maxAreaIndex]);
    lastSelectedIndex = maxAreaIndex;
  }

  result.push(data[data.length - 1]);
  return result;
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
  chartType: 'sparkline' | 'history' = 'history',
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
