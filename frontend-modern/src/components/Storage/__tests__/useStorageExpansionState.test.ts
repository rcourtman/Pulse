import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import { useStorageExpansionState } from '@/components/Storage/useStorageExpansionState';

describe('useStorageExpansionState', () => {
  it('syncs expanded groups from grouped keys and toggles them', () => {
    const [groupedKeys] = createSignal(['alpha', 'beta']);
    const [view] = createSignal<'pools' | 'disks'>('pools');

    const { result } = renderHook(() =>
      useStorageExpansionState({
        groupedKeys,
        view,
      }),
    );

    expect([...result.expandedGroups()]).toEqual(['alpha', 'beta']);

    result.toggleGroup('alpha');
    expect([...result.expandedGroups()]).toEqual(['beta']);
  });

  it('clears expanded pool state when switching away from pools', () => {
    const [groupedKeys] = createSignal(['alpha']);
    const [view, setView] = createSignal<'pools' | 'disks'>('pools');

    const { result } = renderHook(() =>
      useStorageExpansionState({
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
