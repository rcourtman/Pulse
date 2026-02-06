/**
 * Disk Metrics History Store
 *
 * Tracks time-series data for disk I/O (throughput/IOPS) using a ring buffer.
 * Separated from main metrics history because the data shape is different
 * (rates instead of 0-100% usage).
 */

import { createSignal } from 'solid-js';

export interface DiskMetricSnapshot {
    timestamp: number;
    readBps: number;    // Bytes per second
    writeBps: number;   // Bytes per second
    readIops: number;   // Operations per second
    writeIops: number;  // Operations per second
    ioTime: number;     // % utilization (0-100) based on ioTimeMs
}

interface RingBuffer {
    buffer: DiskMetricSnapshot[];
    head: number;  // Index of oldest item
    size: number;  // Number of items currently stored
}

// Configuration
const MAX_POINTS = 2000; // ~16 hours at 30s samples

// Store - map of resourceId (e.g. "host:node1:sda") to ring buffer
const diskMetricsHistoryMap = new Map<string, RingBuffer>();

// Reactive version signal
const [diskMetricsVersion, setDiskMetricsVersion] = createSignal(0);

export function getDiskMetricsVersion(): number {
    return diskMetricsVersion();
}

function createRingBuffer(): RingBuffer {
    return {
        buffer: new Array(MAX_POINTS),
        head: 0,
        size: 0,
    };
}

function pushToRingBuffer(ring: RingBuffer, snapshot: DiskMetricSnapshot): void {
    const index = (ring.head + ring.size) % MAX_POINTS;
    ring.buffer[index] = snapshot;

    if (ring.size < MAX_POINTS) {
        ring.size++;
    } else {
        // Buffer is full, advance head (overwrite oldest)
        ring.head = (ring.head + 1) % MAX_POINTS;
    }
}

function getRingBufferData(ring: RingBuffer, cutoffTime: number): DiskMetricSnapshot[] {
    const result: DiskMetricSnapshot[] = [];

    for (let i = 0; i < ring.size; i++) {
        const index = (ring.head + i) % MAX_POINTS;
        const snapshot = ring.buffer[index];

        if (snapshot && snapshot.timestamp >= cutoffTime) {
            result.push(snapshot);
        }
    }

    return result;
}

/**
 * Record a new disk metric snapshot
 */
export function recordDiskMetric(
    resourceId: string, // format: "nodeId:device" or "hostId:device"
    readBps: number,
    writeBps: number,
    readIops: number,
    writeIops: number,
    ioTime: number
): void {
    const now = Date.now();
    const snapshot: DiskMetricSnapshot = {
        timestamp: now,
        readBps: Math.round(readBps),
        writeBps: Math.round(writeBps),
        readIops: Math.round(readIops),
        writeIops: Math.round(writeIops),
        ioTime: Math.round(ioTime * 10) / 10
    };

    let ring = diskMetricsHistoryMap.get(resourceId);
    if (!ring) {
        ring = createRingBuffer();
        diskMetricsHistoryMap.set(resourceId, ring);
    }

    pushToRingBuffer(ring, snapshot);

    // Trigger reactivity
    setDiskMetricsVersion(v => v + 1);
}

/**
 * Get metric history for a disk
 */
export function getDiskMetricHistory(resourceId: string, rangeMs: number = 3600000): DiskMetricSnapshot[] {
    const ring = diskMetricsHistoryMap.get(resourceId);
    if (!ring) return [];

    const cutoffTime = Date.now() - rangeMs;
    return getRingBufferData(ring, cutoffTime);
}

/**
 * Get the latest snapshot
 */
export function getLatestDiskMetric(resourceId: string): DiskMetricSnapshot | null {
    const ring = diskMetricsHistoryMap.get(resourceId);
    if (!ring || ring.size === 0) return null;

    const index = (ring.head + ring.size - 1) % MAX_POINTS;
    return ring.buffer[index];
}

// Stats for debugging
export function getDiskMetricsStats() {
    return {
        resourceCount: diskMetricsHistoryMap.size,
        bufferSize: MAX_POINTS
    };
}

if (import.meta.env.DEV) {
    (window as any).__diskMetricsHistory = {
        getStats: getDiskMetricsStats,
        getHistory: getDiskMetricHistory
    };
}

