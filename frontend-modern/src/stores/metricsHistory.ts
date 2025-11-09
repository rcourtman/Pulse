/**
 * Metrics History Store (Ring Buffer Implementation)
 *
 * Tracks time-series data for sparkline visualizations using a ring buffer
 * for O(1) insertions and minimal GC pressure.
 */

import { logger } from '@/utils/logger';

export interface MetricSnapshot {
  timestamp: number; // Unix timestamp in ms
  cpu: number;       // 0-100
  memory: number;    // 0-100
  disk: number;      // 0-100
}

interface RingBuffer {
  buffer: MetricSnapshot[];
  head: number;  // Index of oldest item
  size: number;  // Number of items currently stored
}

// Configuration
const MAX_AGE_MS = 2 * 60 * 60 * 1000; // 2 hours
const SAMPLE_INTERVAL_MS = 30 * 1000;   // 30 seconds
const MAX_POINTS = Math.ceil(MAX_AGE_MS / SAMPLE_INTERVAL_MS); // ~240 points
const STORAGE_KEY = 'pulse_metrics_history';
const STORAGE_VERSION = 1;

// Store - map of resourceId to ring buffer
const metricsHistoryMap = new Map<string, RingBuffer>();

// Track last sample time per resource to enforce sampling interval
const lastSampleTimes = new Map<string, number>();

/**
 * Create a new ring buffer
 */
function createRingBuffer(): RingBuffer {
  return {
    buffer: new Array(MAX_POINTS),
    head: 0,
    size: 0,
  };
}

/**
 * Add a metric snapshot to the ring buffer (O(1) operation)
 */
function pushToRingBuffer(ring: RingBuffer, snapshot: MetricSnapshot): void {
  const index = (ring.head + ring.size) % MAX_POINTS;
  ring.buffer[index] = snapshot;

  if (ring.size < MAX_POINTS) {
    ring.size++;
  } else {
    // Buffer is full, advance head (overwrite oldest)
    ring.head = (ring.head + 1) % MAX_POINTS;
  }
}

/**
 * Get all snapshots from ring buffer, ordered oldest to newest
 */
function getRingBufferData(ring: RingBuffer, cutoffTime: number): MetricSnapshot[] {
  const result: MetricSnapshot[] = [];

  for (let i = 0; i < ring.size; i++) {
    const index = (ring.head + i) % MAX_POINTS;
    const snapshot = ring.buffer[index];

    // Filter out old snapshots beyond MAX_AGE_MS
    if (snapshot && snapshot.timestamp >= cutoffTime) {
      result.push(snapshot);
    }
  }

  return result;
}

/**
 * Save metrics history to localStorage
 */
function saveToLocalStorage(): void {
  try {
    const cutoffTime = Date.now() - MAX_AGE_MS;
    const serialized: Record<string, MetricSnapshot[]> = {};

    // Only save non-expired data
    for (const [resourceId, ring] of metricsHistoryMap.entries()) {
      const data = getRingBufferData(ring, cutoffTime);
      if (data.length > 0) {
        serialized[resourceId] = data;
      }
    }

    const payload = {
      version: STORAGE_VERSION,
      timestamp: Date.now(),
      data: serialized,
    };

    localStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
  } catch (error) {
    logger.error('[MetricsHistory] Failed to save to localStorage', { error });
  }
}

/**
 * Load metrics history from localStorage
 */
function loadFromLocalStorage(): void {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (!stored) return;

    const payload = JSON.parse(stored);

    // Check version and age
    if (payload.version !== STORAGE_VERSION) {
      logger.warn('[MetricsHistory] Storage version mismatch, clearing');
      localStorage.removeItem(STORAGE_KEY);
      return;
    }

    // Discard if data is too old (older than 12 hours)
    const maxStorageAge = 12 * 60 * 60 * 1000;
    if (Date.now() - payload.timestamp > maxStorageAge) {
      logger.info('[MetricsHistory] Storage data too old, clearing');
      localStorage.removeItem(STORAGE_KEY);
      return;
    }

    const cutoffTime = Date.now() - MAX_AGE_MS;

    // Restore data
    for (const [resourceId, snapshots] of Object.entries(payload.data as Record<string, MetricSnapshot[]>)) {
      const ring = createRingBuffer();

      // Only restore non-expired snapshots
      for (const snapshot of snapshots) {
        if (snapshot.timestamp >= cutoffTime) {
          pushToRingBuffer(ring, snapshot);
        }
      }

      if (ring.size > 0) {
        metricsHistoryMap.set(resourceId, ring);
      }
    }

    logger.info('[MetricsHistory] Loaded from localStorage', {
      resourceCount: metricsHistoryMap.size,
    });
  } catch (error) {
    logger.error('[MetricsHistory] Failed to load from localStorage', { error });
    localStorage.removeItem(STORAGE_KEY);
  }
}

/**
 * Add a metric snapshot for a resource
 */
export function recordMetric(
  resourceId: string,
  cpu: number,
  memory: number,
  disk: number,
): void {
  const now = Date.now();

  // Enforce sampling interval - don't record if we sampled too recently
  const lastSample = lastSampleTimes.get(resourceId) || 0;
  if (now - lastSample < SAMPLE_INTERVAL_MS) {
    return;
  }

  lastSampleTimes.set(resourceId, now);

  const snapshot: MetricSnapshot = {
    timestamp: now,
    cpu: Math.round(cpu * 10) / 10,      // Round to 1 decimal
    memory: Math.round(memory * 10) / 10,
    disk: Math.round(disk * 10) / 10,
  };

  // Get or create ring buffer
  let ring = metricsHistoryMap.get(resourceId);
  if (!ring) {
    ring = createRingBuffer();
    metricsHistoryMap.set(resourceId, ring);
  }

  pushToRingBuffer(ring, snapshot);

  // Save to localStorage periodically (debounced)
  debouncedSave();
}

// Debounced save to avoid excessive localStorage writes
let saveTimeout: number | null = null;
function debouncedSave() {
  if (saveTimeout !== null) {
    clearTimeout(saveTimeout);
  }
  saveTimeout = window.setTimeout(() => {
    saveToLocalStorage();
    saveTimeout = null;
  }, 5000); // Save 5 seconds after last change
}

/**
 * Get metric history for a resource
 */
export function getMetricHistory(resourceId: string): MetricSnapshot[] {
  const ring = metricsHistoryMap.get(resourceId);
  if (!ring) return [];

  const cutoffTime = Date.now() - MAX_AGE_MS;
  return getRingBufferData(ring, cutoffTime);
}

/**
 * Clear history for a specific resource
 */
export function clearMetricHistory(resourceId: string): void {
  metricsHistoryMap.delete(resourceId);
  lastSampleTimes.delete(resourceId);
}

/**
 * Clear all history
 */
export function clearAllMetricsHistory(): void {
  metricsHistoryMap.clear();
  lastSampleTimes.clear();
}

/**
 * Prune metrics for resources that are no longer present
 * @param prefix - Resource kind prefix (e.g., "vm:", "node:")
 * @param validIds - Set of IDs that should be kept (WITHOUT prefix)
 */
export function pruneMetricsByPrefix(prefix: string, validIds: Set<string>): void {
  const keysToDelete: string[] = [];

  for (const key of metricsHistoryMap.keys()) {
    if (key.startsWith(prefix)) {
      // Extract the ID part (everything after prefix)
      const id = key.slice(prefix.length);

      if (!validIds.has(id)) {
        keysToDelete.push(key);
      }
    }
  }

  // Delete stale entries
  for (const key of keysToDelete) {
    clearMetricHistory(key);
  }

  if (keysToDelete.length > 0) {
    logger.debug('[MetricsHistory] Pruned stale entries', {
      prefix,
      count: keysToDelete.length,
    });
  }
}

/**
 * Cleanup old data across all resources
 * Call this periodically to remove expired snapshots from ring buffers
 */
export function cleanupOldMetrics(): void {
  const now = Date.now();
  const cutoff = now - MAX_AGE_MS;
  const keysToDelete: string[] = [];

  for (const [resourceId, ring] of metricsHistoryMap.entries()) {
    // Count valid (non-expired) snapshots
    let validCount = 0;
    for (let i = 0; i < ring.size; i++) {
      const index = (ring.head + i) % MAX_POINTS;
      const snapshot = ring.buffer[index];
      if (snapshot && snapshot.timestamp >= cutoff) {
        validCount++;
      }
    }

    // If no valid snapshots remain, mark for deletion
    if (validCount === 0) {
      keysToDelete.push(resourceId);
    }
  }

  // Delete empty entries
  for (const key of keysToDelete) {
    clearMetricHistory(key);
  }
}

// Initialize: Load from localStorage on startup
if (typeof window !== 'undefined') {
  loadFromLocalStorage();

  // Auto-cleanup every 5 minutes
  setInterval(cleanupOldMetrics, 5 * 60 * 1000);

  // Save to localStorage before page unload
  window.addEventListener('beforeunload', () => {
    saveToLocalStorage();
  });

  // Periodic save every 2 minutes as backup
  setInterval(saveToLocalStorage, 2 * 60 * 1000);
}

// Expose stats for debugging
export function getMetricsStats() {
  const resourceCount = metricsHistoryMap.size;
  let totalPoints = 0;

  for (const ring of metricsHistoryMap.values()) {
    totalPoints += ring.size;
  }

  const avgPointsPerResource = resourceCount > 0 ? totalPoints / resourceCount : 0;

  return {
    resourceCount,
    totalPoints,
    avgPointsPerResource,
    maxAge: MAX_AGE_MS,
    sampleInterval: SAMPLE_INTERVAL_MS,
    maxPointsPerResource: MAX_POINTS,
  };
}

if (import.meta.env.DEV) {
  // Expose for debugging in dev mode
  (window as any).__metricsHistory = {
    getStats: getMetricsStats,
    clear: clearAllMetricsHistory,
    getHistory: getMetricHistory,
  };
}
