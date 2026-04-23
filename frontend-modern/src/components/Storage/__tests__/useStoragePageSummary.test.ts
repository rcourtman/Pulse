import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import type { StorageRecord } from '@/features/storageBackups/models';
import { useStoragePageSummary } from '@/components/Storage/useStoragePageSummary';

const makeRecord = (id: string, health: StorageRecord['health']): StorageRecord => ({
  id,
  name: id,
  category: 'pool',
  health,
  location: { label: 'pve1', scope: 'node' },
  capacity: { totalBytes: null, usedBytes: null, freeBytes: null, usagePercent: null },
  capabilities: [],
  source: {
    platform: 'proxmox-pve',
    family: 'virtualization',
    origin: 'resource',
    adapterId: 'test',
  },
  observedAt: Date.now(),
});

describe('useStoragePageSummary', () => {
  it('derives pool and visible disk counts canonically for the selected node', () => {
    const [filteredRecords] = createSignal<StorageRecord[]>([
      makeRecord('pool-a', 'healthy'),
      makeRecord('pool-b', 'healthy'),
      makeRecord('pool-c', 'healthy'),
      makeRecord('pool-d', 'healthy'),
    ]);
    const [selectedNodeId] = createSignal('node-1');
    const [search] = createSignal('');
    const [sourceFilter] = createSignal('all');
    const [healthFilter] = createSignal('all' as const);
    const [diskRoleFilter] = createSignal('all');
    const [diskGroupFilter] = createSignal('all');
    const [nodeOptions] = createSignal([
      { id: 'node-1', label: 'pve1', aliases: ['pve1.local'] },
      { id: 'node-2', label: 'pve2' },
    ]);
    const [physicalDisks] = createSignal<Resource[]>([
      {
        id: 'disk-1',
        type: 'physical_disk',
        name: '/dev/sda',
        displayName: '/dev/sda',
        platformId: 'cluster-main',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        status: 'online',
        lastSeen: Date.now(),
        parentId: 'pool-1',
        identity: { hostname: 'pve1' },
        canonicalIdentity: { hostname: 'pve1' },
      },
      {
        id: 'disk-2',
        type: 'physical_disk',
        name: '/dev/sdb',
        displayName: '/dev/sdb',
        platformId: 'cluster-main',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        status: 'online',
        lastSeen: Date.now(),
        identity: { hostname: 'pve2' },
        canonicalIdentity: { hostname: 'pve2' },
      },
    ]);

    const { result } = renderHook(() =>
      useStoragePageSummary({
        filteredRecords,
        search,
        sourceFilter,
        healthFilter,
        diskRoleFilter,
        diskGroupFilter,
        selectedNodeId,
        nodeOptions,
        physicalDisks,
      }),
    );

    expect(result.poolCount()).toBe(4);
    expect(result.diskCount()).toBe(1);
    expect(result.poolsDegraded()).toBe(0);
    expect(result.disksFailing()).toBe(0);
  });
});
