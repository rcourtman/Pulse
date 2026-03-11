import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import { useStorageContentCardModel } from '@/components/Storage/useStorageContentCardModel';

describe('useStorageContentCardModel', () => {
  it('derives canonical content-card state from page view', () => {
    const [view] = createSignal<'pools' | 'disks'>('disks');
    const [selectedNodeId] = createSignal('all');

    const { result } = renderHook(() =>
      useStorageContentCardModel({
        view,
        selectedNodeId,
      }),
    );

    expect(result.heading()).toBe('Physical Disks');
    expect(result.showDisks()).toBe(true);
    expect(result.showPools()).toBe(false);
    expect(result.selectedDiskNodeId()).toBe(null);
  });
});
