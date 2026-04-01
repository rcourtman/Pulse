import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { useDiskDetailModel } from '@/components/Storage/useDiskDetailModel';

const buildDisk = (overrides: Partial<Resource> = {}): Resource =>
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
    metricsTarget: { resourceType: 'disk', resourceId: 'agent-tower:sda' },
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
    ...overrides,
  }) as unknown as Resource;

describe('useDiskDetailModel', () => {
  it('builds canonical disk detail model state', () => {
    const [disk] = createSignal(buildDisk());

    const { result } = renderHook(() =>
      useDiskDetailModel({
        disk,
      }),
    );

    expect(result.chartRange()).toBe('24h');
    expect(result.historyResourceId()).toBe('agent-tower:sda');
    expect(result.metricResourceId()).toBe('agent-tower:sda');
    expect(result.attributeCards().length).toBeGreaterThan(0);
    expect(result.historyCharts().map((chart) => chart.metric)).toEqual([
      'smart_temp',
      'smart_reallocated_sectors',
    ]);
  });

  it('keeps historical charts available for api-backed disks without serial or wwn when a canonical disk target exists', () => {
    const [disk] = createSignal(
      buildDisk({
        metricsTarget: { resourceType: 'disk', resourceId: 'disk:truenas-main:sda' },
        physicalDisk: {
          devPath: '/dev/sda',
          model: 'Seagate IronWolf',
          serial: '',
          wwn: '',
          diskType: 'hdd',
          temperature: 39,
        },
      } as Partial<Resource>),
    );
    const { result } = renderHook(() =>
      useDiskDetailModel({
        disk,
      }),
    );

    expect(result.historyResourceId()).toBe('disk:truenas-main:sda');
  });
});
