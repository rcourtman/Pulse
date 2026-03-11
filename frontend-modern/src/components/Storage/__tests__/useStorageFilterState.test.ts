import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import { useStorageFilterState } from '@/components/Storage/useStorageFilterState';
import type { StorageNodeOption, StorageGroupKey } from '@/components/Storage/useStorageModel';
import type { NormalizedHealth } from '@/features/storageBackups/models';

describe('useStorageFilterState', () => {
  it('builds source and node filter options canonically', () => {
    const [view] = createSignal<'pools' | 'disks'>('pools');
    const [nodeOptions] = createSignal<StorageNodeOption[]>([
      { id: 'node-1', label: 'pve1' },
    ]);
    const [diskNodeOptions] = createSignal<StorageNodeOption[]>([
      { id: 'node-2', label: 'tower' },
    ]);
    const [selectedNodeId, setSelectedNodeId] = createSignal('node-1');
    const [sourceOptions] = createSignal(['all', 'proxmox-pve', 'agent']);
    const [healthFilter, setHealthFilter] = createSignal<'all' | NormalizedHealth>('all');
    const [groupBy] = createSignal<StorageGroupKey>('host');

    const { result } = renderHook(() =>
      useStorageFilterState({
        view,
        nodeOptions,
        diskNodeOptions,
        selectedNodeId,
        setSelectedNodeId,
        sourceOptions,
        healthFilter,
        setHealthFilter,
        groupBy,
      }),
    );

    expect(result.sourceFilterOptions()).toEqual([
      { key: 'all', label: 'All Sources', tone: 'slate' },
      { key: 'proxmox-pve', label: 'PVE', tone: 'orange' },
      { key: 'agent', label: 'Agent', tone: 'slate' },
    ]);
    expect(result.nodeFilterOptions()).toEqual([
      { value: 'all', label: 'All Nodes' },
      { value: 'node-1', label: 'pve1' },
    ]);
    expect(result.storageFilterGroupBy()).toBe('node');
    expect(result.storageFilterStatus()).toBe('all');
  });

  it('coerces stale selected nodes and maps status setters', () => {
    const [view] = createSignal<'pools' | 'disks'>('disks');
    const [nodeOptions] = createSignal<StorageNodeOption[]>([
      { id: 'all', label: 'All Nodes' },
    ]);
    const [diskNodeOptions] = createSignal<StorageNodeOption[]>([
      { id: 'all', label: 'All Nodes' },
    ]);
    const [selectedNodeId, setSelectedNodeId] = createSignal('missing');
    const [sourceOptions] = createSignal(['all']);
    const [healthFilter, setHealthFilter] = createSignal<'all' | NormalizedHealth>('all');
    const [groupBy] = createSignal<StorageGroupKey>('none');

    const { result } = renderHook(() =>
      useStorageFilterState({
        view,
        nodeOptions,
        diskNodeOptions,
        selectedNodeId,
        setSelectedNodeId,
        sourceOptions,
        healthFilter,
        setHealthFilter,
        groupBy,
      }),
    );

    expect(selectedNodeId()).toBe('all');
    result.setStorageFilterStatus('critical');
    expect(healthFilter()).toBe('critical');
  });
});
