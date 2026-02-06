/**
 * Tests for Charts API types and interface
 */
import { describe, expect, it } from 'vitest';
import type { ChartData, ChartsResponse, TimeRange, MetricPoint, ChartStats } from '@/api/charts';
import { timeRangeToMs } from '@/utils/timeRange';

// Note: We test the types and interfaces here since the actual API calls
// require a running backend. Integration tests should cover the full flow.

describe('Charts API Types', () => {
    describe('TimeRange', () => {
        it('supports all expected time range values', () => {
            const validRanges: TimeRange[] = ['5m', '15m', '30m', '1h', '4h', '12h', '24h', '7d', '30d'];

            validRanges.forEach(range => {
                // This is a compile-time check - if it compiles, the types are correct
                const r: TimeRange = range;
                expect(r).toBe(range);
            });
        });
    });

    describe('MetricPoint', () => {
        it('has required timestamp and value properties', () => {
            const point: MetricPoint = {
                timestamp: 1733700000000,
                value: 45.7,
            };

            expect(point.timestamp).toBe(1733700000000);
            expect(point.value).toBe(45.7);
        });
    });

    describe('ChartData', () => {
        it('supports all metric types', () => {
            const data: ChartData = {
                cpu: [{ timestamp: 1000, value: 50 }],
                memory: [{ timestamp: 1000, value: 60 }],
                disk: [{ timestamp: 1000, value: 70 }],
                diskread: [{ timestamp: 1000, value: 1000000 }],
                diskwrite: [{ timestamp: 1000, value: 500000 }],
                netin: [{ timestamp: 1000, value: 2000000 }],
                netout: [{ timestamp: 1000, value: 1500000 }],
            };

            expect(data.cpu).toHaveLength(1);
            expect(data.memory![0].value).toBe(60);
        });

        it('allows partial data (all fields optional)', () => {
            const cpuOnly: ChartData = {
                cpu: [{ timestamp: 1000, value: 25 }],
            };

            expect(cpuOnly.cpu).toBeDefined();
            expect(cpuOnly.memory).toBeUndefined();
            expect(cpuOnly.disk).toBeUndefined();
        });

        it('allows empty arrays for metrics', () => {
            const emptyData: ChartData = {
                cpu: [],
                memory: [],
            };

            expect(emptyData.cpu).toHaveLength(0);
        });
    });

    describe('ChartsResponse', () => {
        it('includes all required fields', () => {
            const response: ChartsResponse = {
                data: {},
                nodeData: {},
                storageData: {},
                timestamp: Date.now(),
                stats: { oldestDataTimestamp: Date.now() - 3600000 },
            };

            expect(response.data).toBeDefined();
            expect(response.nodeData).toBeDefined();
            expect(response.timestamp).toBeGreaterThan(0);
        });

        it('supports optional dockerData field', () => {
            const response: ChartsResponse = {
                data: {},
                nodeData: {},
                storageData: {},
                dockerData: {
                    'container-abc123': {
                        cpu: [{ timestamp: 1000, value: 10 }],
                        memory: [{ timestamp: 1000, value: 20 }],
                    },
                },
                timestamp: Date.now(),
                stats: { oldestDataTimestamp: Date.now() },
            };

            expect(response.dockerData).toBeDefined();
            expect(response.dockerData!['container-abc123'].cpu).toHaveLength(1);
        });

        it('supports optional dockerHostData field', () => {
            const response: ChartsResponse = {
                data: {},
                nodeData: {},
                storageData: {},
                dockerHostData: {
                    'host-1': {
                        cpu: [{ timestamp: 1000, value: 30 }],
                        memory: [{ timestamp: 1000, value: 40 }],
                    },
                },
                timestamp: Date.now(),
                stats: { oldestDataTimestamp: Date.now() },
            };

            expect(response.dockerHostData).toBeDefined();
            expect(response.dockerHostData!['host-1'].cpu![0].value).toBe(30);
        });

        it('supports optional guestTypes field for VM/container type mapping', () => {
            const response: ChartsResponse = {
                data: {
                    'vm-100': { cpu: [{ timestamp: 1000, value: 50 }] },
                    'ct-200': { cpu: [{ timestamp: 1000, value: 30 }] },
                },
                nodeData: {},
                storageData: {},
                guestTypes: {
                    'vm-100': 'vm',
                    'ct-200': 'container',
                },
                timestamp: Date.now(),
                stats: { oldestDataTimestamp: Date.now() },
            };

            expect(response.guestTypes!['vm-100']).toBe('vm');
            expect(response.guestTypes!['ct-200']).toBe('container');
        });

        it('contains all data sources for comprehensive monitoring', () => {
            const comprehensiveResponse: ChartsResponse = {
                // VM and container metrics (legacy format)
                data: {
                    'pve1/qemu/100': {
                        cpu: [{ timestamp: 1000, value: 45 }],
                        memory: [{ timestamp: 1000, value: 55 }],
                        disk: [{ timestamp: 1000, value: 30 }],
                    },
                    'pve1/lxc/200': {
                        cpu: [{ timestamp: 1000, value: 15 }],
                    },
                },
                // Node metrics
                nodeData: {
                    'pve1': {
                        cpu: [{ timestamp: 1000, value: 35 }],
                        memory: [{ timestamp: 1000, value: 65 }],
                    },
                },
                // Storage metrics
                storageData: {
                    'local-zfs': {
                        disk: [{ timestamp: 1000, value: 50 }],
                    },
                },
                // Docker container metrics
                dockerData: {
                    'abc123': {
                        cpu: [{ timestamp: 1000, value: 5 }],
                        memory: [{ timestamp: 1000, value: 128 }],
                    },
                },
                // Docker host metrics
                dockerHostData: {
                    'docker-host-1': {
                        cpu: [{ timestamp: 1000, value: 25 }],
                    },
                },
                // Guest type mapping
                guestTypes: {
                    'pve1/qemu/100': 'vm',
                    'pve1/lxc/200': 'container',
                },
                timestamp: 1733700000000,
                stats: {
                    oldestDataTimestamp: 1733696400000, // 1 hour ago
                },
            };

            expect(Object.keys(comprehensiveResponse.data)).toHaveLength(2);
            expect(Object.keys(comprehensiveResponse.nodeData)).toHaveLength(1);
            expect(Object.keys(comprehensiveResponse.dockerData!)).toHaveLength(1);
            expect(Object.keys(comprehensiveResponse.dockerHostData!)).toHaveLength(1);
        });
    });

    describe('ChartStats', () => {
        it('contains oldest data timestamp for data availability', () => {
            const stats: ChartStats = {
                oldestDataTimestamp: 1733696400000,
            };

            expect(stats.oldestDataTimestamp).toBe(1733696400000);
        });

        it('can be used to determine available data range', () => {
            const now = Date.now();
            const oneHourAgo = now - 3600000;

            const stats: ChartStats = {
                oldestDataTimestamp: oneHourAgo,
            };

            const availableRangeMs = now - stats.oldestDataTimestamp;
            expect(availableRangeMs).toBeCloseTo(3600000, -2); // Allow 100ms tolerance
        });
    });
});

describe('Time Range to Milliseconds Conversion', () => {
    const expectedValues: [TimeRange, number][] = [
        ['5m', 300000],
        ['15m', 900000],
        ['30m', 1800000],
        ['1h', 3600000],
        ['4h', 14400000],
        ['12h', 43200000],
        ['24h', 86400000],
        ['7d', 604800000],
        ['30d', 2592000000],
    ];

    it.each(expectedValues)('converts %s to %d ms', (range, expectedMs) => {
        expect(timeRangeToMs(range)).toBe(expectedMs);
    });

    it('defaults to 1 hour for unknown range', () => {
        // @ts-expect-error - Testing invalid input
        expect(timeRangeToMs('invalid')).toBe(3600000);
    });
});
