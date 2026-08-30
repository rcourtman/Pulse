import { describe, expect, it } from 'vitest';

import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';

import {
  buildMetricMiniSparklinePath,
  computeMetricMiniSparklineHoverState,
  findChartDataForCandidates,
  getMetricMiniSparklineScale,
  getMetricMiniSparklineTimeRange,
  getMetricSparklineSeriesFromChartData,
  getNodeChartKeyCandidates,
  getWorkloadChartKeyCandidates,
  isWorkloadTableMetricHistoryRange,
  normalizeWorkloadChartKey,
  WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE,
  WORKLOAD_TABLE_HISTORY_MAX_POINTS,
  WORKLOAD_TABLE_HISTORY_RANGES,
} from '../workloadMetricHistoryModel';

describe('workloadMetricHistoryModel', () => {
  it('canonicalizes legacy Proxmox chart keys', () => {
    expect(normalizeWorkloadChartKey('pve-101')).toBe('pve:pve:101');
    expect(normalizeWorkloadChartKey('cluster-a-pve-101')).toBe('cluster-a:pve:101');
    expect(normalizeWorkloadChartKey('cluster-a:pve:101')).toBe('cluster-a:pve:101');
  });

  it('keeps table sparkline history ranges bounded to dense table windows', () => {
    expect(WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE).toBe('1h');
    expect(WORKLOAD_TABLE_HISTORY_MAX_POINTS).toBe(36);
    expect(WORKLOAD_TABLE_HISTORY_RANGES).toEqual(['1h', '12h', '24h', '7d']);
    expect(isWorkloadTableMetricHistoryRange('12h')).toBe(true);
    expect(isWorkloadTableMetricHistoryRange('30d')).toBe(false);
  });

  it('builds workload chart lookup candidates around canonical identity', () => {
    const guest = {
      id: 'cluster-a-pve-101',
      instance: 'cluster-a',
      node: 'pve',
      vmid: 101,
      type: 'qemu',
    } as WorkloadGuest;

    expect(getWorkloadChartKeyCandidates(guest)).toEqual(
      expect.arrayContaining(['cluster-a:pve:101', 'cluster-a-pve-101', 'pve-101', 'pve:pve:101']),
    );
  });

  it('builds node chart lookup candidates for agent-linked Proxmox hosts', () => {
    const node = {
      id: 'agent:pve',
      linkedAgentId: 'agent-1',
      name: 'pve',
      instance: 'cluster-a',
    } as Node;

    expect(getNodeChartKeyCandidates(node)).toEqual(
      expect.arrayContaining(['agent:pve', 'agent-1', 'pve', 'cluster-a-pve', 'pve']),
    );
  });

  it('finds chart data by normalized candidate', () => {
    const chartData = { cpu: [{ timestamp: 1, value: 5 }] };
    expect(findChartDataForCandidates(['pve-101'], [{ 'pve:pve:101': chartData }])).toBe(chartData);
  });

  it('returns paired I/O series from chart data', () => {
    const series = getMetricSparklineSeriesFromChartData(
      {
        netin: [
          { timestamp: 1, value: 10 },
          { timestamp: 2, value: 20 },
        ],
        netout: [
          { timestamp: 1, value: 3 },
          { timestamp: 2, value: 4 },
        ],
      },
      'netIo',
    );

    expect(series.map((item) => item.id)).toEqual(['netin', 'netout']);
    expect(series[0].points[1].value).toBe(20);
  });

  it('derives host-relative memory history from raw used bytes', () => {
    const gib = 1024 ** 3;
    const series = getMetricSparklineSeriesFromChartData(
      {
        memory: [
          { timestamp: 1, value: 50 },
          { timestamp: 2, value: 75 },
        ],
        memoryused: [
          { timestamp: 1, value: 2 * gib },
          { timestamp: 2, value: 4 * gib },
        ],
      },
      'memory',
      { memoryDisplayBasis: 'host', parentMemoryTotal: 16 * gib },
    );

    expect(series[0].label).toBe('Host memory share');
    expect(series[0].points).toEqual([
      { timestamp: 1, value: 12.5 },
      { timestamp: 2, value: 25 },
    ]);
  });

  it('does not relabel guest-relative percentages when raw host-relative history is absent', () => {
    const series = getMetricSparklineSeriesFromChartData(
      { memory: [{ timestamp: 1, value: 75 }] },
      'memory',
      { memoryDisplayBasis: 'host', parentMemoryTotal: 16 * 1024 ** 3 },
    );

    expect(series[0].points).toEqual([]);
  });

  it('builds bounded mini sparkline paths', () => {
    const scale = getMetricMiniSparklineScale(
      [
        {
          id: 'cpu',
          label: 'CPU',
          color: '#fff',
          points: [
            { timestamp: 1, value: 0 },
            { timestamp: 2, value: 50 },
            { timestamp: 3, value: 100 },
          ],
        },
      ],
      '%',
    );

    expect(scale).toEqual({ minValue: 0, maxValue: 100 });
    expect(
      buildMetricMiniSparklinePath(
        [
          { timestamp: 1, value: 0 },
          { timestamp: 2, value: 50 },
          { timestamp: 3, value: 100 },
        ],
        scale,
      ),
    ).toBe('M1.00,16.00 L48.00,9.00 L95.00,2.00');
  });

  it('lifts low-domain percent sparklines off the axis instead of pinning them to a 0-100 window', () => {
    const hostMemoryShare = [
      {
        id: 'memory',
        label: 'Host memory share',
        color: '#f59e0b',
        points: [
          { timestamp: 1, value: 1.9 },
          { timestamp: 2, value: 2.4 },
          { timestamp: 3, value: 2.1 },
        ],
      },
    ];
    const scale = getMetricMiniSparklineScale(hostMemoryShare, '%');

    // A guest's share of host memory is single digits by construction; a fixed
    // 0-100 window drew it on top of the axis rule.
    expect(scale).toEqual({ minValue: 0, maxValue: 5 });
    expect(buildMetricMiniSparklinePath(hostMemoryShare[0].points, scale)).toBe(
      'M1.00,10.68 L48.00,9.28 L95.00,10.12',
    );
  });

  it('keeps percent sparklines zero-floored and capped at 100', () => {
    const scale = getMetricMiniSparklineScale(
      [
        {
          id: 'cpu',
          label: 'CPU',
          color: '#8b5cf6',
          points: [
            { timestamp: 1, value: 40 },
            { timestamp: 2, value: 95 },
          ],
        },
      ],
      '%',
    );

    expect(scale).toEqual({ minValue: 0, maxValue: 100 });
  });

  it('uses a shared time range and resolves nearest hover values for paired I/O sparklines', () => {
    const series = [
      {
        id: 'netin',
        label: 'In',
        color: '#10b981',
        points: [
          { timestamp: 1_000, value: 10 },
          { timestamp: 2_000, value: 20 },
          { timestamp: 3_000, value: 30 },
        ],
      },
      {
        id: 'netout',
        label: 'Out',
        color: '#fb923c',
        points: [
          { timestamp: 2_000, value: 200 },
          { timestamp: 3_000, value: 300 },
        ],
      },
    ];
    const timeRange = getMetricMiniSparklineTimeRange(series);
    const scale = getMetricMiniSparklineScale(series, 'B/s');

    expect(timeRange).toEqual({ minTimestamp: 1_000, maxTimestamp: 3_000 });
    expect(buildMetricMiniSparklinePath(series[1].points, scale, 96, 18, timeRange)).toMatch(
      /^M48\.00,/,
    );

    const hover = computeMetricMiniSparklineHoverState(series, 48, 96);
    expect(hover?.timestamp).toBe(2_000);
    expect(hover?.entries.map((entry) => [entry.label, entry.value])).toEqual([
      ['In', 20],
      ['Out', 200],
    ]);
  });
});
