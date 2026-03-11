import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';
import { useStorageControlsModel } from '@/components/Storage/useStorageControlsModel';

describe('useStorageControlsModel', () => {
  it('centralizes storage controls view and node handlers', () => {
    const [selectedNodeId, setSelectedNodeId] = createSignal('all');
    const onViewChange = vi.fn();

    const { result } = renderHook(() =>
      useStorageControlsModel({
        selectedNodeId,
        setSelectedNodeId,
        onViewChange,
      }),
    );

    expect(result.viewTabs).toEqual([
      { value: 'pools', label: 'Pools' },
      { value: 'disks', label: 'Physical Disks' },
    ]);

    result.handleNodeFilterChange('node-1');
    expect(selectedNodeId()).toBe('node-1');

    result.handleViewChange('disks');
    expect(onViewChange).toHaveBeenCalledWith('disks');
  });
});
