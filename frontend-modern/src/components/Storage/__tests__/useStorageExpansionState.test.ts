import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import { useStorageExpansionState } from '@/components/Storage/useStorageExpansionState';
import type { StorageGroupKey } from '@/components/Storage/useStorageModel';

describe('useStorageExpansionState', () => {
  it('syncs expanded groups from grouped keys and toggles them', () => {
    const [groupedKeys] = createSignal(['alpha', 'beta']);
    const [view] = createSignal<'pools' | 'disks'>('pools');
    const [groupBy] = createSignal<StorageGroupKey>('node');

    const { result } = renderHook(() =>
      useStorageExpansionState({
        groupBy,
        groupedKeys,
        view,
      }),
    );

    expect([...result.expandedGroups()]).toEqual(['alpha', 'beta']);

    result.toggleGroup('alpha');
    expect([...result.expandedGroups()]).toEqual(['beta']);
  });

  it('keeps collapsed groups collapsed when live updates rebuild the group keys', () => {
    const [groupedKeys, setGroupedKeys] = createSignal(['alpha', 'beta']);
    const [view] = createSignal<'pools' | 'disks'>('pools');
    const [groupBy] = createSignal<StorageGroupKey>('node');

    const { result } = renderHook(() =>
      useStorageExpansionState({
        groupBy,
        groupedKeys,
        view,
      }),
    );

    result.toggleGroup('alpha');
    setGroupedKeys(['alpha', 'beta']);
    expect([...result.expandedGroups()]).toEqual(['beta']);

    // A group that appears later opens by default.
    setGroupedKeys(['alpha', 'beta', 'gamma']);
    expect([...result.expandedGroups()]).toEqual(['beta', 'gamma']);

    // Collapsing every group sticks too.
    result.toggleGroup('beta');
    result.toggleGroup('gamma');
    setGroupedKeys(['alpha', 'beta', 'gamma']);
    expect([...result.expandedGroups()]).toEqual([]);

    result.toggleGroup('alpha');
    expect([...result.expandedGroups()]).toEqual(['alpha']);
  });

  it('opens every group when the grouping changes', () => {
    // A node labelled like the headerless ungrouped container must not hide it.
    const [groupedKeys, setGroupedKeys] = createSignal(['All', 'pve1']);
    const [groupBy, setGroupBy] = createSignal<StorageGroupKey>('node');
    const [view] = createSignal<'pools' | 'disks'>('pools');

    const { result } = renderHook(() =>
      useStorageExpansionState({
        groupBy,
        groupedKeys,
        view,
      }),
    );

    result.toggleGroup('All');
    expect([...result.expandedGroups()]).toEqual(['pve1']);

    setGroupBy('none');
    setGroupedKeys(['All']);
    expect([...result.expandedGroups()]).toEqual(['All']);
  });

  it('clears expanded pool state when switching away from pools', () => {
    const [groupedKeys] = createSignal(['alpha']);
    const [view, setView] = createSignal<'pools' | 'disks'>('pools');
    const [groupBy] = createSignal<StorageGroupKey>('node');

    const { result } = renderHook(() =>
      useStorageExpansionState({
        groupBy,
        groupedKeys,
        view,
      }),
    );

    result.setExpandedPoolId('pool-1');
    expect(result.expandedPoolId()).toBe('pool-1');

    setView('disks');
    expect(result.expandedPoolId()).toBeNull();
  });
});
