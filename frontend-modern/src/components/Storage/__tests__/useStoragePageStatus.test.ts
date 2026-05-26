import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import { useStoragePageStatus } from '@/components/Storage/useStoragePageStatus';

describe('useStoragePageStatus', () => {
  it('flags pool loading when the surface is fetching and has no pools yet on the pools view', () => {
    const [loading] = createSignal(true);
    const [filteredRecordCount] = createSignal(0);
    const [view] = createSignal<'pools' | 'disks'>('pools');

    const { result } = renderHook(() =>
      useStoragePageStatus({
        loading,
        filteredRecordCount,
        view,
      }),
    );

    expect(result.isLoadingPools()).toBe(true);
  });

  it('treats the disks view as not loading even while the surface fetches', () => {
    const [loading] = createSignal(true);
    const [filteredRecordCount] = createSignal(0);
    const [view] = createSignal<'pools' | 'disks'>('disks');

    const { result } = renderHook(() =>
      useStoragePageStatus({
        loading,
        filteredRecordCount,
        view,
      }),
    );

    expect(result.isLoadingPools()).toBe(false);
  });

  it('does not flag pool loading when records are already present', () => {
    const [loading] = createSignal(false);
    const [filteredRecordCount] = createSignal(1);
    const [view] = createSignal<'pools' | 'disks'>('pools');

    const { result } = renderHook(() =>
      useStoragePageStatus({
        loading,
        filteredRecordCount,
        view,
      }),
    );

    expect(result.isLoadingPools()).toBe(false);
  });
});
