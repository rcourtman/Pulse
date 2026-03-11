import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { useDiskDetailModel } from '@/components/Storage/useDiskDetailModel';

vi.mock('@/stores/diskMetricsHistory', () => ({
  getDiskMetricsVersion: () => 1,
  getDiskMetricHistory: () => [
    { timestamp: 1000, readBps: 10, writeBps: 20, ioTime: 30 },
    { timestamp: 2000, readBps: 15, writeBps: 25, ioTime: 35 },
  ],
}));

const buildDisk = (): Resource =>
  ({
    id: 'disk-1',
    type: 'physical_disk',
    name: 'disk-1',
    displayName: 'disk-1',
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    metricsTarget: { resourceId: 'agent-tower:sda' },
    identity: { hostname: 'tower' },
    canonicalIdentity: { hostname: 'tower' },
    platformData: {
      proxmox: { nodeName: 'tower', instance: 'cluster-main' },
    },
    physicalDisk: {
      devPath: '/dev/sda',
      model: 'Archive HDD',
      serial: 'SERIAL-1',
      diskType: 'hdd',
      temperature: 42,
      smart: {
        powerOnHours: 100,
        reallocatedSectors: 0,
      },
    },
  }) as Resource;

describe('useDiskDetailModel', () => {
  it('builds canonical disk detail model state', () => {
    const [disk] = createSignal(buildDisk());
    const [nodes] = createSignal<Resource[]>([]);

    const { result } = renderHook(() =>
      useDiskDetailModel({
        disk,
        nodes,
      }),
    );

    expect(result.chartRange()).toBe('24h');
    expect(result.resId()).toBe('SERIAL-1');
    expect(result.metricResourceId()).toBe('agent-tower:sda');
    expect(result.attributeCards().length).toBeGreaterThan(0);
    expect(result.historyCharts().map((chart) => chart.metric)).toEqual([
      'smart_temp',
      'smart_reallocated_sectors',
    ]);
    expect(result.readData()).toEqual([
      { timestamp: 1000, value: 10, min: 10, max: 10 },
      { timestamp: 2000, value: 15, min: 15, max: 15 },
    ]);
    expect(result.writeData()).toEqual([
      { timestamp: 1000, value: 20, min: 20, max: 20 },
      { timestamp: 2000, value: 25, min: 25, max: 25 },
    ]);
    expect(result.ioData()).toEqual([
      { timestamp: 1000, value: 30, min: 30, max: 30 },
      { timestamp: 2000, value: 35, min: 35, max: 35 },
    ]);
  });
});
