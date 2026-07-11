import { describe, expect, it } from 'vitest';

import type { ChartData, MetricPoint } from '@/api/charts';
import type { WorkloadGuest } from '@/types/workloads';

import type { WorkloadTableMetric } from '../workloadMetricHistoryModel';
import {
  computeMetricMiniSparklineHoverState,
  findChartDataForCandidates,
  getMetricMiniSparklineTimeRange,
  getMetricSparklineSeriesFromChartData,
  getWorkloadChartKeyCandidates,
  normalizeWorkloadChartKey,
} from '../workloadMetricHistoryModel';

const makeGuest = (overrides?: Partial<WorkloadGuest>): WorkloadGuest =>
  ({
    id: 'guest-0',
    vmid: 100,
    name: 'workload-0',
    node: 'pve',
    instance: 'cluster-a',
    status: 'running',
    type: 'qemu',
    workloadType: 'vm',
    ...overrides,
  }) as WorkloadGuest;

const seriesPoint = (timestamp: number, value: number): MetricPoint => ({ timestamp, value });

describe('workloadMetricHistoryModel (branch coverage)', () => {
  describe('normalizeWorkloadChartKey', () => {
    it('returns empty string for blank input', () => {
      expect(normalizeWorkloadChartKey('')).toBe('');
      expect(normalizeWorkloadChartKey('   ')).toBe('');
    });

    it('passes colon-bearing keys through after trimming', () => {
      expect(normalizeWorkloadChartKey('  a:b:c  ')).toBe('a:b:c');
    });

    it('normalizes a two-part node-vmid key', () => {
      expect(normalizeWorkloadChartKey('pve-101')).toBe('pve:pve:101');
    });

    it('returns trimmed input for a two-part key with a non-digit vmid', () => {
      expect(normalizeWorkloadChartKey('node-abc')).toBe('node-abc');
    });

    it('returns trimmed input for a two-part key with an empty node (leading dash)', () => {
      expect(normalizeWorkloadChartKey('-101')).toBe('-101');
    });

    it('returns a single-token input unchanged', () => {
      expect(normalizeWorkloadChartKey('foo')).toBe('foo');
    });

    it('normalizes a three-plus-part instance-node-vmid key', () => {
      expect(normalizeWorkloadChartKey('cluster-a-pve-101')).toBe('cluster-a:pve:101');
    });

    it('returns trimmed input when the instance part is empty', () => {
      expect(normalizeWorkloadChartKey('-pve-101')).toBe('-pve-101');
    });

    it('returns trimmed input when the node part is empty', () => {
      expect(normalizeWorkloadChartKey('cluster-a--101')).toBe('cluster-a--101');
    });

    it('returns trimmed input when the vmid part is non-digit in a 3+ part key', () => {
      expect(normalizeWorkloadChartKey('a-b-c')).toBe('a-b-c');
    });
  });

  describe('getWorkloadChartKeyCandidates', () => {
    it('skips node/vmid candidates when vmid is not positive', () => {
      const guest = makeGuest({ id: 'g1', node: 'pve', instance: 'c', vmid: 0, type: 'qemu' });
      expect(getWorkloadChartKeyCandidates(guest)).toEqual(['g1']);
    });

    it('emits node variants but no instance variants when instance is empty', () => {
      const guest = makeGuest({
        id: 'pve-101',
        node: 'pve',
        instance: '',
        vmid: 101,
        type: 'qemu',
      });
      expect(getWorkloadChartKeyCandidates(guest)).toEqual(['pve-101', 'pve:pve:101']);
    });

    it('emits canonical, raw, node, and instance candidates in deduped order', () => {
      const guest = makeGuest({
        id: 'cluster-a-pve-101',
        instance: 'cluster-a',
        node: 'pve',
        vmid: 101,
        type: 'qemu',
      });
      expect(getWorkloadChartKeyCandidates(guest)).toEqual([
        'cluster-a:pve:101',
        'cluster-a-pve-101',
        'pve-101',
        'pve:pve:101',
      ]);
    });

    it('includes containerId and skips the node block when node is empty', () => {
      const guest = makeGuest({
        id: 'cid-1',
        containerId: 'docker-abc',
        node: '',
        instance: '',
        vmid: 0,
        type: 'app-container',
        workloadType: 'app-container',
      });
      expect(getWorkloadChartKeyCandidates(guest)).toEqual(['cid-1', 'cid:cid:1', 'docker-abc']);
    });
  });

  describe('findChartDataForCandidates / readChartData', () => {
    const data: ChartData = { cpu: [seriesPoint(1, 5)] };

    it('returns undefined for empty candidates', () => {
      expect(findChartDataForCandidates([], [{ x: data }])).toBeUndefined();
    });

    it('returns undefined when no source contains any candidate', () => {
      expect(findChartDataForCandidates(['x', 'y'], [{ z: data }])).toBeUndefined();
    });

    it('returns undefined when the source is undefined', () => {
      expect(findChartDataForCandidates(['x'], [undefined])).toBeUndefined();
    });

    it('returns undefined for a blank candidate key (readChartData !key guard)', () => {
      expect(findChartDataForCandidates([''], [{ '': data }])).toBeUndefined();
    });

    it('finds a direct hit in a record source', () => {
      expect(findChartDataForCandidates(['pve:pve:101'], [{ 'pve:pve:101': data }])).toBe(data);
    });

    it('finds a direct hit in a Map source', () => {
      const map = new Map<string, ChartData>([['pve:pve:101', data]]);
      expect(findChartDataForCandidates(['pve:pve:101'], [map])).toBe(data);
    });

    it('finds a normalized hit in a Map source', () => {
      const map = new Map<string, ChartData>([['pve:pve:101', data]]);
      expect(findChartDataForCandidates(['pve-101'], [map])).toBe(data);
    });

    it('skips the normalized lookup when the candidate already normalizes to itself', () => {
      const source = { 'pve-101': data };
      expect(findChartDataForCandidates(['pve-101'], [source])).toBe(data);
    });

    it('finds a hit across multiple sources, returning the first match', () => {
      const other: ChartData = { memory: [seriesPoint(1, 2)] };
      expect(findChartDataForCandidates(['key'], [{}, { key: other }])).toBe(other);
    });
  });

  describe('getMetricSparklineSeriesFromChartData / sanitizeMetricPoints / clamp', () => {
    it('returns an empty array when chartData is undefined', () => {
      expect(getMetricSparklineSeriesFromChartData(undefined, 'cpu')).toEqual([]);
    });

    it('returns the default empty array for an unrecognized metric', () => {
      expect(
        getMetricSparklineSeriesFromChartData(
          { cpu: [] },
          'bogus' as unknown as WorkloadTableMetric,
        ),
      ).toEqual([]);
    });

    it('clamps cpu values via clampPercent (>100, <0, in-range)', () => {
      const series = getMetricSparklineSeriesFromChartData(
        {
          cpu: [
            seriesPoint(1, 150),
            seriesPoint(2, -5),
            seriesPoint(3, 50),
          ],
        },
        'cpu',
      );
      expect(series).toHaveLength(1);
      expect(series[0].id).toBe('cpu');
      expect(series[0].label).toBe('CPU');
      expect(series[0].points).toEqual([
        { timestamp: 1, value: 100 },
        { timestamp: 2, value: 0 },
        { timestamp: 3, value: 50 },
      ]);
    });

    it('filters non-finite points and sorts by timestamp for memory', () => {
      const series = getMetricSparklineSeriesFromChartData(
        {
          memory: [
            seriesPoint(3, 30),
            seriesPoint(1, 10),
            seriesPoint(2, NaN),
            seriesPoint(NaN, 40),
            seriesPoint(4, 40),
          ],
        },
        'memory',
      );
      expect(series[0].id).toBe('memory');
      expect(series[0].points).toEqual([
        { timestamp: 1, value: 10 },
        { timestamp: 3, value: 30 },
        { timestamp: 4, value: 40 },
      ]);
    });

    it('emits a disk series with the disk color', () => {
      const series = getMetricSparklineSeriesFromChartData(
        { disk: [seriesPoint(1, 99)] },
        'disk',
      );
      expect(series[0].id).toBe('disk');
      expect(series[0].color).toBe('#10b981');
    });

    it('returns empty points when the metric array is undefined/empty', () => {
      const series = getMetricSparklineSeriesFromChartData({}, 'cpu');
      expect(series[0].points).toEqual([]);
    });

    it('clamps netIo values via clampNonNegative and returns paired series', () => {
      const series = getMetricSparklineSeriesFromChartData(
        {
          netin: [seriesPoint(1, -10), seriesPoint(2, 30)],
          netout: [seriesPoint(1, 7)],
        },
        'netIo',
      );
      expect(series.map((s) => s.id)).toEqual(['netin', 'netout']);
      expect(series[0].points).toEqual([
        { timestamp: 1, value: 0 },
        { timestamp: 2, value: 30 },
      ]);
      expect(series[1].points).toEqual([{ timestamp: 1, value: 7 }]);
    });

    it('clamps diskIo values via clampNonNegative and filters non-finite', () => {
      const series = getMetricSparklineSeriesFromChartData(
        {
          diskread: [seriesPoint(1, -10), seriesPoint(2, 30)],
          diskwrite: [seriesPoint(1, NaN), seriesPoint(2, 7)],
        },
        'diskIo',
      );
      expect(series.map((s) => s.id)).toEqual(['diskread', 'diskwrite']);
      expect(series[0].points).toEqual([
        { timestamp: 1, value: 0 },
        { timestamp: 2, value: 30 },
      ]);
      expect(series[1].points).toEqual([{ timestamp: 2, value: 7 }]);
    });
  });

  describe('getMetricMiniSparklineTimeRange', () => {
    it('returns null for an empty series array', () => {
      expect(getMetricMiniSparklineTimeRange([])).toBeNull();
    });

    it('returns null when all points are non-finite', () => {
      expect(
        getMetricMiniSparklineTimeRange([
          {
            id: 'a',
            label: 'A',
            color: '#fff',
            points: [seriesPoint(NaN, 1), seriesPoint(1, NaN)],
          },
        ]),
      ).toBeNull();
    });

    it('returns null when min and max timestamps are equal (single point)', () => {
      expect(
        getMetricMiniSparklineTimeRange([
          {
            id: 'a',
            label: 'A',
            color: '#fff',
            points: [seriesPoint(5, 1)],
          },
        ]),
      ).toBeNull();
    });

    it('ignores invalid points and reports the range of valid points only', () => {
      expect(
        getMetricMiniSparklineTimeRange([
          {
            id: 'a',
            label: 'A',
            color: '#fff',
            points: [
              seriesPoint(2, 5),
              seriesPoint(10, NaN),
              seriesPoint(8, 9),
              seriesPoint(NaN, 1),
            ],
          },
        ]),
      ).toEqual({ minTimestamp: 2, maxTimestamp: 8 });
    });

    it('combines ranges across multiple series', () => {
      expect(
        getMetricMiniSparklineTimeRange([
          {
            id: 'a',
            label: 'A',
            color: '#fff',
            points: [seriesPoint(100, 1), seriesPoint(200, 2)],
          },
          {
            id: 'b',
            label: 'B',
            color: '#000',
            points: [seriesPoint(50, 1), seriesPoint(300, 2)],
          },
        ]),
      ).toEqual({ minTimestamp: 50, maxTimestamp: 300 });
    });
  });

  describe('computeMetricMiniSparklineHoverState / findNearestMetricMiniSparklinePoint', () => {
    const twoPointSeries = [
      {
        id: 'a',
        label: 'A',
        color: '#fff',
        points: [seriesPoint(1000, 10), seriesPoint(2000, 20)],
      },
    ];

    it('returns null when there is no valid time range (empty series)', () => {
      expect(computeMetricMiniSparklineHoverState([], 10, 100)).toBeNull();
    });

    it('returns null for non-positive width', () => {
      expect(computeMetricMiniSparklineHoverState(twoPointSeries, 10, 0)).toBeNull();
      expect(computeMetricMiniSparklineHoverState(twoPointSeries, 10, -5)).toBeNull();
    });

    it('returns null for non-finite width', () => {
      expect(
        computeMetricMiniSparklineHoverState(twoPointSeries, 10, Number.NaN),
      ).toBeNull();
    });

    it('clamps a negative cursorX to zero (left edge)', () => {
      const hover = computeMetricMiniSparklineHoverState(twoPointSeries, -50, 100);
      expect(hover).not.toBeNull();
      expect(hover?.cursorX).toBe(0);
      expect(hover?.cursorRatio).toBe(0);
      expect(hover?.timestamp).toBe(1000);
      expect(hover?.entries[0].value).toBe(10);
    });

    it('clamps a cursorX beyond width to the right edge', () => {
      const hover = computeMetricMiniSparklineHoverState(twoPointSeries, 300, 100);
      expect(hover).not.toBeNull();
      expect(hover?.cursorX).toBe(100);
      expect(hover?.cursorRatio).toBe(1);
      expect(hover?.timestamp).toBe(2000);
      expect(hover?.entries[0].value).toBe(20);
    });

    it('selects the previous point when the cursor is closer to it', () => {
      const series = [
        {
          id: 'a',
          label: 'A',
          color: '#fff',
          points: [seriesPoint(1000, 10), seriesPoint(3000, 30)],
        },
      ];
      const hover = computeMetricMiniSparklineHoverState(series, 50, 200);
      expect(hover?.timestamp).toBe(1000);
      expect(hover?.entries[0].value).toBe(10);
    });

    it('selects the candidate point when the cursor is closer to it', () => {
      const series = [
        {
          id: 'a',
          label: 'A',
          color: '#fff',
          points: [seriesPoint(1000, 10), seriesPoint(3000, 30)],
        },
      ];
      const hover = computeMetricMiniSparklineHoverState(series, 150, 200);
      expect(hover?.timestamp).toBe(3000);
      expect(hover?.entries[0].value).toBe(30);
    });

    it('resolves an exact midpoint to the previous point (<= tiebreak)', () => {
      const series = [
        {
          id: 'a',
          label: 'A',
          color: '#fff',
          points: [seriesPoint(1000, 10), seriesPoint(3000, 30)],
        },
      ];
      const hover = computeMetricMiniSparklineHoverState(series, 100, 200);
      expect(hover?.timestamp).toBe(1000);
    });

    it('finds the exact matching point among three points via binary search', () => {
      const series = [
        {
          id: 'a',
          label: 'A',
          color: '#fff',
          points: [seriesPoint(1000, 1), seriesPoint(2000, 2), seriesPoint(3000, 3)],
        },
      ];
      const hover = computeMetricMiniSparklineHoverState(series, 50, 100);
      expect(hover?.timestamp).toBe(2000);
      expect(hover?.entries[0].value).toBe(2);
    });

    it('only emits entries for series with renderable points', () => {
      const series = [
        {
          id: 'a',
          label: 'A',
          color: '#fff',
          points: [seriesPoint(1000, 1), seriesPoint(2000, 2)],
        },
        {
          id: 'b',
          label: 'B',
          color: '#000',
          points: [] as MetricPoint[],
        },
      ];
      const hover = computeMetricMiniSparklineHoverState(series, 0, 100);
      expect(hover).not.toBeNull();
      expect(hover?.entries.map((e) => e.id)).toEqual(['a']);
    });
  });
});
