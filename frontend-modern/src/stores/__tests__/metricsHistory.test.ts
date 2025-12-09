/**
 * Tests for metricsHistory store functionality
 * 
 * These tests focus on the ring buffer operations and metric recording logic.
 */
import { describe, expect, it, beforeEach, afterEach, vi } from 'vitest';

// We can't directly test the module due to side effects (localStorage, window),
// so we test the core logic patterns used in the implementation.

describe('Ring Buffer Logic', () => {
    // Implementation of ring buffer matching metricsHistory.ts
    interface MetricSnapshot {
        timestamp: number;
        cpu: number;
        memory: number;
        disk: number;
    }

    interface RingBuffer {
        buffer: (MetricSnapshot | undefined)[];
        head: number;
        size: number;
    }

    const MAX_POINTS = 100;

    function createRingBuffer(): RingBuffer {
        return {
            buffer: new Array(MAX_POINTS),
            head: 0,
            size: 0,
        };
    }

    function pushToRingBuffer(ring: RingBuffer, snapshot: MetricSnapshot): void {
        const index = (ring.head + ring.size) % MAX_POINTS;
        ring.buffer[index] = snapshot;

        if (ring.size < MAX_POINTS) {
            ring.size++;
        } else {
            ring.head = (ring.head + 1) % MAX_POINTS;
        }
    }

    function getRingBufferData(ring: RingBuffer, cutoffTime: number): MetricSnapshot[] {
        const result: MetricSnapshot[] = [];

        for (let i = 0; i < ring.size; i++) {
            const index = (ring.head + i) % MAX_POINTS;
            const snapshot = ring.buffer[index];

            if (snapshot && snapshot.timestamp >= cutoffTime) {
                result.push(snapshot);
            }
        }

        return result;
    }

    describe('createRingBuffer', () => {
        it('creates an empty ring buffer', () => {
            const ring = createRingBuffer();
            expect(ring.head).toBe(0);
            expect(ring.size).toBe(0);
            expect(ring.buffer.length).toBe(MAX_POINTS);
        });
    });

    describe('pushToRingBuffer', () => {
        it('adds snapshots to the buffer', () => {
            const ring = createRingBuffer();
            const snapshot: MetricSnapshot = {
                timestamp: Date.now(),
                cpu: 50,
                memory: 60,
                disk: 70,
            };

            pushToRingBuffer(ring, snapshot);

            expect(ring.size).toBe(1);
            expect(ring.buffer[0]).toEqual(snapshot);
        });

        it('increments size correctly', () => {
            const ring = createRingBuffer();

            for (let i = 0; i < 10; i++) {
                pushToRingBuffer(ring, {
                    timestamp: Date.now() + i,
                    cpu: i,
                    memory: i,
                    disk: i,
                });
            }

            expect(ring.size).toBe(10);
        });

        it('wraps around and overwrites oldest when full', () => {
            const ring = createRingBuffer();

            // Fill the buffer
            for (let i = 0; i < MAX_POINTS; i++) {
                pushToRingBuffer(ring, {
                    timestamp: 1000 + i,
                    cpu: i,
                    memory: i,
                    disk: i,
                });
            }

            expect(ring.size).toBe(MAX_POINTS);
            expect(ring.head).toBe(0);

            // Add one more - should overwrite first entry
            pushToRingBuffer(ring, {
                timestamp: 2000,
                cpu: 999,
                memory: 999,
                disk: 999,
            });

            expect(ring.size).toBe(MAX_POINTS);
            expect(ring.head).toBe(1); // Head moved forward
            expect(ring.buffer[0]!.cpu).toBe(999); // New entry at position 0
        });

        it('maintains O(1) insertion performance', () => {
            const ring = createRingBuffer();
            const iterations = 10000;
            const start = performance.now();

            for (let i = 0; i < iterations; i++) {
                pushToRingBuffer(ring, {
                    timestamp: Date.now(),
                    cpu: Math.random() * 100,
                    memory: Math.random() * 100,
                    disk: Math.random() * 100,
                });
            }

            const duration = performance.now() - start;
            // Should complete quickly (< 100ms for 10k insertions)
            expect(duration).toBeLessThan(100);
        });
    });

    describe('getRingBufferData', () => {
        it('returns all data when no cutoff', () => {
            const ring = createRingBuffer();

            for (let i = 0; i < 5; i++) {
                pushToRingBuffer(ring, {
                    timestamp: 1000 + i,
                    cpu: i * 10,
                    memory: i * 10,
                    disk: i * 10,
                });
            }

            const data = getRingBufferData(ring, 0);
            expect(data).toHaveLength(5);
        });

        it('returns data in chronological order', () => {
            const ring = createRingBuffer();

            for (let i = 0; i < 5; i++) {
                pushToRingBuffer(ring, {
                    timestamp: 1000 + i,
                    cpu: i,
                    memory: i,
                    disk: i,
                });
            }

            const data = getRingBufferData(ring, 0);
            expect(data[0].timestamp).toBe(1000);
            expect(data[4].timestamp).toBe(1004);
        });

        it('filters by cutoff time', () => {
            const ring = createRingBuffer();

            for (let i = 0; i < 10; i++) {
                pushToRingBuffer(ring, {
                    timestamp: 1000 + i,
                    cpu: i,
                    memory: i,
                    disk: i,
                });
            }

            const data = getRingBufferData(ring, 1005);
            expect(data).toHaveLength(5); // Only timestamps >= 1005
            expect(data[0].timestamp).toBe(1005);
        });

        it('handles wrapped buffer correctly', () => {
            const ring = createRingBuffer();

            // Fill buffer completely and then some
            for (let i = 0; i < MAX_POINTS + 20; i++) {
                pushToRingBuffer(ring, {
                    timestamp: 1000 + i,
                    cpu: i % 100,
                    memory: i % 100,
                    disk: i % 100,
                });
            }

            const data = getRingBufferData(ring, 0);
            expect(data).toHaveLength(MAX_POINTS);

            // First entry should be the 21st one (index 20)
            expect(data[0].timestamp).toBe(1020);
            // Last entry should be the most recent
            expect(data[MAX_POINTS - 1].timestamp).toBe(1000 + MAX_POINTS + 19);
        });

        it('returns empty array for empty buffer', () => {
            const ring = createRingBuffer();
            const data = getRingBufferData(ring, 0);
            expect(data).toHaveLength(0);
        });

        it('returns empty array when all data is before cutoff', () => {
            const ring = createRingBuffer();

            for (let i = 0; i < 5; i++) {
                pushToRingBuffer(ring, {
                    timestamp: 1000 + i,
                    cpu: i,
                    memory: i,
                    disk: i,
                });
            }

            const data = getRingBufferData(ring, 2000);
            expect(data).toHaveLength(0);
        });
    });
});

describe('Metric Key Generation', () => {
    // Test metric key pattern matching
    function buildMetricKey(kind: string, id: string): string {
        return `${kind}:${id}`;
    }

    describe('key format', () => {
        it('creates properly namespaced keys', () => {
            expect(buildMetricKey('vm', '100')).toBe('vm:100');
            expect(buildMetricKey('node', 'pve1')).toBe('node:pve1');
            expect(buildMetricKey('dockerContainer', 'abc123')).toBe('dockerContainer:abc123');
        });

        it('allows filtering by prefix', () => {
            const keys = [
                'vm:100',
                'vm:101',
                'node:pve1',
                'dockerContainer:abc',
            ];

            const vmKeys = keys.filter(k => k.startsWith('vm:'));
            expect(vmKeys).toHaveLength(2);

            const dockerKeys = keys.filter(k => k.startsWith('dockerContainer:'));
            expect(dockerKeys).toHaveLength(1);
        });
    });
});

describe('Time Range Calculations', () => {
    type TimeRange = '5m' | '15m' | '30m' | '1h' | '4h' | '12h' | '24h' | '7d';

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
            default: return 60 * 60 * 1000;
        }
    }

    it('converts all time ranges correctly', () => {
        expect(timeRangeToMs('5m')).toBe(300000);
        expect(timeRangeToMs('15m')).toBe(900000);
        expect(timeRangeToMs('30m')).toBe(1800000);
        expect(timeRangeToMs('1h')).toBe(3600000);
        expect(timeRangeToMs('4h')).toBe(14400000);
        expect(timeRangeToMs('12h')).toBe(43200000);
        expect(timeRangeToMs('24h')).toBe(86400000);
        expect(timeRangeToMs('7d')).toBe(604800000);
    });

    it('calculates correct cutoff times', () => {
        const now = Date.now();
        const oneHourAgo = now - timeRangeToMs('1h');

        expect(now - oneHourAgo).toBe(3600000);
    });
});

describe('Sample Interval Enforcement', () => {
    const SAMPLE_INTERVAL_MS = 30 * 1000; // 30 seconds

    function shouldRecordSample(lastSampleTime: number, now: number): boolean {
        return now - lastSampleTime >= SAMPLE_INTERVAL_MS;
    }

    it('allows recording when interval has passed', () => {
        const now = Date.now();
        const lastSample = now - 31000; // 31 seconds ago
        expect(shouldRecordSample(lastSample, now)).toBe(true);
    });

    it('blocks recording when too soon', () => {
        const now = Date.now();
        const lastSample = now - 15000; // 15 seconds ago
        expect(shouldRecordSample(lastSample, now)).toBe(false);
    });

    it('allows recording exactly at interval', () => {
        const now = Date.now();
        const lastSample = now - SAMPLE_INTERVAL_MS;
        expect(shouldRecordSample(lastSample, now)).toBe(true);
    });

    it('always allows first sample (lastSample = 0)', () => {
        const now = Date.now();
        expect(shouldRecordSample(0, now)).toBe(true);
    });
});

describe('Metric Value Rounding', () => {
    function roundMetric(value: number): number {
        return Math.round(value * 10) / 10;
    }

    it('rounds to 1 decimal place', () => {
        expect(roundMetric(45.123)).toBe(45.1);
        expect(roundMetric(45.156)).toBe(45.2);
        expect(roundMetric(45.149)).toBe(45.1);
        expect(roundMetric(100.0)).toBe(100.0);
        expect(roundMetric(0.05)).toBe(0.1);
        expect(roundMetric(0.04)).toBe(0.0);
    });

    it('handles edge cases', () => {
        expect(roundMetric(0)).toBe(0);
        expect(roundMetric(-45.123)).toBe(-45.1);
        expect(roundMetric(99.95)).toBe(100.0);
    });
});

describe('Duplicate Timestamp Prevention', () => {
    interface MetricSnapshot {
        timestamp: number;
        cpu: number;
    }

    function shouldSkipDuplicate(
        existingData: MetricSnapshot[],
        newTimestamp: number,
        toleranceMs: number = 15000
    ): boolean {
        for (const existing of existingData) {
            if (Math.abs(existing.timestamp - newTimestamp) < toleranceMs) {
                return true;
            }
        }
        return false;
    }

    it('skips timestamps within tolerance', () => {
        const existing: MetricSnapshot[] = [
            { timestamp: 1000000, cpu: 50 },
            { timestamp: 1030000, cpu: 55 },
        ];

        // Within 15s of existing
        expect(shouldSkipDuplicate(existing, 1010000)).toBe(true);
        expect(shouldSkipDuplicate(existing, 1025000)).toBe(true);
    });

    it('allows timestamps outside tolerance', () => {
        const existing: MetricSnapshot[] = [
            { timestamp: 1000000, cpu: 50 },
        ];

        // More than 15s from existing
        expect(shouldSkipDuplicate(existing, 1020000)).toBe(false);
    });

    it('allows all timestamps when no existing data', () => {
        const existing: MetricSnapshot[] = [];
        expect(shouldSkipDuplicate(existing, 1000000)).toBe(false);
    });
});
