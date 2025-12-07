/**
 * Metrics History Store (Ring Buffer Implementation)
 *
 * Tracks time-series data for sparkline visualizations using a ring buffer
 * for O(1) insertions and minimal GC pressure.
 */

import { logger } from '@/utils/logger';
import { ChartsAPI, type ChartData, type TimeRange } from '@/api/charts';
import { buildMetricKey } from '@/utils/metricsKeys';

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
const MAX_AGE_MS = 24 * 60 * 60 * 1000; // 24 hours (to support all time ranges)
const SAMPLE_INTERVAL_MS = 30 * 1000;   // 30 seconds
const MAX_POINTS = Math.ceil(MAX_AGE_MS / SAMPLE_INTERVAL_MS); // ~2880 points
const STORAGE_KEY = 'pulse_metrics_history';
const STORAGE_VERSION = 2; // Bumped version due to increased buffer size

/**
 * Convert TimeRange string to milliseconds
 */
function timeRangeToMs(range: TimeRange): number {
  switch (range) {
    case '5m': return 5 * 60 * 1000;
    case '15m': return 15 * 60 * 1000;
    case '30m': return 30 * 60 * 1000;
    case '1h': return 60 * 60 * 1000;
    case '4h': return 4 * 60 * 60 * 1000;
    case '12h': return 12 * 60 * 60 * 1000;
    case '24h': return 24 * 60 * 60 * 1000;
    case '7d': return 7 * 24 * 60 * 60 * 1000;
    default: return 60 * 60 * 1000; // Default 1h
  }
}

// Store - map of resourceId to ring buffer
const metricsHistoryMap = new Map<string, RingBuffer>();

// Track last sample time per resource to enforce sampling interval
const lastSampleTimes = new Map<string, number>();

// Reactive version counter - increments when data is seeded, used to trigger component re-renders
import { createSignal } from 'solid-js';
const [metricsVersion, setMetricsVersion] = createSignal(0);

/**
 * Get the current metrics version (for reactivity)
 */
export function getMetricsVersion(): number {
  return metricsVersion();
}

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
    if (!stored) {
      logger.debug('[MetricsHistory] No stored data found');
      return;
    }

    const payload = JSON.parse(stored);

    if (!payload || typeof payload !== 'object') {
      logger.warn('[MetricsHistory] Invalid payload format, clearing');
      localStorage.removeItem(STORAGE_KEY);
      return;
    }

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

// Track if we've already seeded from backend to avoid redundant fetches
let hasSeededFromBackend = false;
let seedingPromise: Promise<void> | null = null;

/**
 * Seed metrics history from backend historical data.
 * This provides immediate trend data instead of waiting for 30s samples.
 * Called automatically when switching to sparklines/trends view.
 */
export async function seedFromBackend(range: TimeRange = '1h'): Promise<void> {
  // Don't re-fetch if we've already seeded
  if (hasSeededFromBackend) {
    return;
  }

  // If already seeding, wait for that request
  if (seedingPromise) {
    return seedingPromise;
  }

  seedingPromise = (async () => {
    try {
      logger.info('[MetricsHistory] Seeding from backend', { range });
      const response = await ChartsAPI.getCharts(range);

      const now = Date.now();
      const cutoff = now - MAX_AGE_MS;
      let seededCount = 0;

      // Helper to convert backend ChartData to our MetricSnapshot format
      const processChartData = (resourceId: string, chartData: ChartData) => {
        const cpuPoints = chartData.cpu || [];
        const memPoints = chartData.memory || [];
        const diskPoints = chartData.disk || [];

        // If no data, skip
        if (cpuPoints.length === 0 && memPoints.length === 0 && diskPoints.length === 0) {
          return;
        }

        // Get or create ring buffer
        let ring = metricsHistoryMap.get(resourceId);
        if (!ring) {
          ring = createRingBuffer();
          metricsHistoryMap.set(resourceId, ring);
        }

        // Find all unique timestamps across all metrics
        const timestampSet = new Set<number>();
        cpuPoints.forEach(p => timestampSet.add(p.timestamp));
        memPoints.forEach(p => timestampSet.add(p.timestamp));
        diskPoints.forEach(p => timestampSet.add(p.timestamp));

        // Create lookup maps for efficient access
        const cpuMap = new Map(cpuPoints.map(p => [p.timestamp, p.value]));
        const memMap = new Map(memPoints.map(p => [p.timestamp, p.value]));
        const diskMap = new Map(diskPoints.map(p => [p.timestamp, p.value]));

        // Sort timestamps and create snapshots
        const timestamps = Array.from(timestampSet).sort((a, b) => a - b);

        for (const ts of timestamps) {
          // Skip if too old
          if (ts < cutoff) continue;

          // Skip if we already have data around this timestamp (within 15s)
          let skipDuplicate = false;
          for (let i = 0; i < ring.size; i++) {
            const idx = (ring.head + i) % MAX_POINTS;
            const existing = ring.buffer[idx];
            if (existing && Math.abs(existing.timestamp - ts) < 15000) {
              skipDuplicate = true;
              break;
            }
          }
          if (skipDuplicate) continue;

          const snapshot: MetricSnapshot = {
            timestamp: ts,
            cpu: Math.round((cpuMap.get(ts) ?? 0) * 10) / 10,
            memory: Math.round((memMap.get(ts) ?? 0) * 10) / 10,
            disk: Math.round((diskMap.get(ts) ?? 0) * 10) / 10,
          };

          pushToRingBuffer(ring, snapshot);
          seededCount++;
        }
      };

      // Process VMs and containers using backend-provided guest types
      // This avoids race conditions with WebSocket state
      if (response.data) {
        // Use guestTypes from backend response (available since backend update)
        // Falls back to WebSocket state for backwards compatibility
        let guestTypeMap: Map<string, 'vm' | 'container'>;

        if (response.guestTypes) {
          // Preferred: Use backend-provided types (no race condition)
          guestTypeMap = new Map(Object.entries(response.guestTypes) as [string, 'vm' | 'container'][]);
          logger.debug('[MetricsHistory] Using backend-provided guestTypes', { count: guestTypeMap.size });
        } else {
          // Fallback: Try WebSocket state (legacy backends without guestTypes)
          guestTypeMap = new Map<string, 'vm' | 'container'>();
          try {
            const { getGlobalWebSocketStore } = await import('./websocket-global');
            const wsStore = getGlobalWebSocketStore();
            const state = wsStore?.state;

            if (state?.vms) {
              for (const vm of state.vms) {
                if (vm.id) guestTypeMap.set(vm.id, 'vm');
              }
            }
            if (state?.containers) {
              for (const ct of state.containers) {
                if (ct.id) guestTypeMap.set(ct.id, 'container');
              }
            }
          } catch {
            logger.warn('[MetricsHistory] Failed to load WebSocket state for guest types');
          }
          logger.debug('[MetricsHistory] Using WebSocket state for guestTypes', { count: guestTypeMap.size });
        }

        for (const [id, chartData] of Object.entries(response.data)) {
          // Look up the guest type, default to 'vm' if unknown
          const guestType = guestTypeMap.get(id) ?? 'vm';
          const resourceKey = buildMetricKey(guestType, id);
          processChartData(resourceKey, chartData as ChartData);
        }
      }

      // Process nodes
      if (response.nodeData) {
        for (const [id, chartData] of Object.entries(response.nodeData)) {
          const resourceKey = buildMetricKey('node', id);
          processChartData(resourceKey, chartData as ChartData);
        }
      }


      hasSeededFromBackend = true;
      logger.info('[MetricsHistory] Seeded from backend', { seededCount, totalResources: metricsHistoryMap.size });

      // Increment version to trigger reactive component updates
      setMetricsVersion(v => v + 1);

      // Save to localStorage
      saveToLocalStorage();
    } catch (error) {
      logger.error('[MetricsHistory] Failed to seed from backend', { error });
      // Don't throw - gracefully degrade to client-side sampling
    } finally {
      seedingPromise = null;
    }
  })();

  return seedingPromise;
}

/**
 * Force re-seed from backend (useful when range changes)
 */
export function resetSeedingState(): void {
  hasSeededFromBackend = false;
}

/**
 * Check if we have seeded from backend
 */
export function hasSeedData(): boolean {
  return hasSeededFromBackend;
}


/**
 * Get metric history for a resource (full history)
 */
export function getMetricHistory(resourceId: string): MetricSnapshot[] {
  const ring = metricsHistoryMap.get(resourceId);
  if (!ring) return [];

  const cutoffTime = Date.now() - MAX_AGE_MS;
  return getRingBufferData(ring, cutoffTime);
}

/**
 * Get metric history for a resource filtered by time range
 */
export function getMetricHistoryForRange(resourceId: string, range: TimeRange): MetricSnapshot[] {
  const ring = metricsHistoryMap.get(resourceId);
  if (!ring) return [];

  const rangeMs = timeRangeToMs(range);
  const cutoffTime = Date.now() - rangeMs;
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
