import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { useStoragePageSummary } from '@/components/Storage/useStoragePageSummary';

describe('useStoragePageSummary', () => {
  it('derives pool and visible disk counts canonically for the selected node', () => {
    const [filteredRecordCount] = createSignal(4);
    const [selectedNodeId] = createSignal('node-1');
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
        filteredRecordCount,
        selectedNodeId,
        nodeOptions,
        physicalDisks,
      }),
    );

    expect(result.poolCount()).toBe(4);
    expect(result.diskCount()).toBe(1);
    expect(result.summaryTimeRange()).toBe('1h');
  });
});
