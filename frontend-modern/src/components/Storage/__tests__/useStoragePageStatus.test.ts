import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import { useStoragePageStatus } from '@/components/Storage/useStoragePageStatus';

describe('useStoragePageStatus', () => {
  it('derives reconnecting banner and pool loading state canonically', () => {
    const [loading] = createSignal(true);
    const [error] = createSignal<unknown>(null);
    const [filteredRecordCount] = createSignal(0);
    const [connected] = createSignal(true);
    const [initialDataReceived] = createSignal(true);
    const [reconnecting] = createSignal(true);
    const [view] = createSignal<'pools' | 'disks'>('pools');

    const { result } = renderHook(() =>
      useStoragePageStatus({
        loading,
        error,
        filteredRecordCount,
        connected,
        initialDataReceived,
        reconnecting,
        view,
      }),
    );

    expect(result.activeBannerKind()).toBe('reconnecting');
    expect(result.isLoadingPools()).toBe(true);
  });

  it('treats disk view loading separately from pool-table loading', () => {
    const [loading] = createSignal(true);
    const [error] = createSignal<unknown>(null);
    const [filteredRecordCount] = createSignal(0);
    const [connected] = createSignal(true);
    const [initialDataReceived] = createSignal(true);
    const [reconnecting] = createSignal(false);
    const [view] = createSignal<'pools' | 'disks'>('disks');

    const { result } = renderHook(() =>
      useStoragePageStatus({
        loading,
        error,
        filteredRecordCount,
        connected,
        initialDataReceived,
        reconnecting,
        view,
      }),
    );

    expect(result.activeBannerKind()).toBeNull();
    expect(result.isLoadingPools()).toBe(false);
  });
});
