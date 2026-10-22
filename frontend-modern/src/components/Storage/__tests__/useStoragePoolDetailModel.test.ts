import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import { useStoragePoolDetailModel } from '@/components/Storage/useStoragePoolDetailModel';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { Resource } from '@/types/resource';

const buildRecord = (): StorageRecord => ({
  id: 'storage-1',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'truenas01', scope: 'host' },
  capacity: { totalBytes: 1000, usedBytes: 400, freeBytes: 600, usagePercent: 40 },
  capabilities: ['capacity', 'health'],
  source: {
    platform: 'truenas',
    family: 'onprem',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  metricsTarget: { resourceType: 'storage', resourceId: 'pool:tank' },
  details: {
    node: 'truenas01',
    parentId: 'host-truenas01',
    type: 'pool',
    zfsPool: {
      state: 'ONLINE',
      scan: 'scrub in progress',
      readErrors: 0,
      writeErrors: 0,
      checksumErrors: 0,
      devices: [{ name: 'sda' }],
    },
  },
});

const buildDisk = (): Resource =>
  ({
    id: 'disk-1',
    parentId: 'host-truenas01',
    type: 'physical_disk',
    name: 'disk-1',
    displayName: 'disk-1',
    platformId: 'truenas01',
    platformType: 'truenas',
    sourceType: 'agent',
    status: 'online',
    lastSeen: Date.now(),
    platformData: {
      physicalDisk: {
        devPath: '/dev/sda',
        model: 'Disk A',
        temperature: 44,
        smart: { reallocatedSectors: 0 },
      },
    },
  }) as Resource;

describe('useStoragePoolDetailModel', () => {
  it('falls back to the record id when the pool has no metrics target', () => {
    const [record] = createSignal({ ...buildRecord(), metricsTarget: undefined });
    const [physicalDisks] = createSignal<Resource[]>([]);
    const { result } = renderHook(() => useStoragePoolDetailModel({ record, physicalDisks }));
    expect(result.historyTarget()).toEqual({ resourceType: 'storage', resourceId: 'storage-1' });
  });

  it('builds canonical pool detail model state', () => {
    const [record] = createSignal(buildRecord());
    const [physicalDisks] = createSignal<Resource[]>([buildDisk()]);

    const { result } = renderHook(() =>
      useStoragePoolDetailModel({
        record,
        physicalDisks,
      }),
    );

    // The drawer owns its chart range; the model owns what the chart reads.
    expect(result.historyTarget()).toEqual({
      resourceType: 'storage',
      resourceId: 'pool:tank',
    });
    expect(result.configRows()).toEqual(
      expect.arrayContaining([
        { label: 'Node', value: 'truenas01' },
        { label: 'Usage', value: '40%' },
      ]),
    );
    expect(result.zfsSummary()).toEqual({
      state: 'ONLINE',
      scan: 'scrub in progress',
      scanErrors: 0,
      errorSummary: null,
      devices: [{ name: 'sda', type: '', state: '', errorSummary: '', message: '' }],
      datasets: [],
    });
    expect(result.linkedDisks()).toEqual([
      {
        id: 'disk-1',
        devPath: '/dev/sda',
        model: 'Disk A',
        diskType: '',
        alertResourceIds: [],
        temperature: 44,
        hasIssue: false,
        errorCount: 0,
        ioLabel: '',
        role: '',
        sizeLabel: '',
        spunDown: false,
        state: '',
      },
    ]);
  });
});
