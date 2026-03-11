import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';
import { useStoragePageControlsModel } from '@/components/Storage/useStoragePageControlsModel';

describe('useStoragePageControlsModel', () => {
  it('derives canonical controls wiring from page state', () => {
    const [kioskMode] = createSignal(false);
    const [view] = createSignal<'pools' | 'disks'>('pools');
    const setGroupBy = vi.fn();
    const setSortKey = vi.fn();
    const [storageFilterGroupBy] = createSignal<'none' | 'host'>('host');

    const { result } = renderHook(() =>
      useStoragePageControlsModel({
        kioskMode,
        view,
        setGroupBy,
        setSortKey,
        storageFilterGroupBy,
      }),
    );

    expect(result.showControls()).toBe(true);
    expect(result.sortDisabled()).toBe(false);
    expect(result.groupBy()?.()).toBe('host');

    result.setGroupBy()?.('host');
    expect(setGroupBy).toHaveBeenCalledWith('host');

    result.setNormalizedSortKey('status');
    expect(setSortKey).toHaveBeenCalledWith('priority');
  });
});
