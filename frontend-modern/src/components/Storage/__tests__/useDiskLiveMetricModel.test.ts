import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';
import { useDiskLiveMetricModel } from '@/components/Storage/useDiskLiveMetricModel';

vi.mock('@/stores/diskMetricsHistory', () => ({
  getDiskMetricsVersion: () => 1,
  getLatestDiskMetric: () => ({
    readBps: 1024,
    writeBps: 2048,
    ioTime: 72,
  }),
}));

describe('useDiskLiveMetricModel', () => {
  it('builds canonical live metric state', () => {
    const [resourceId] = createSignal('agent-tower:sda');
    const [type] = createSignal<'read' | 'write' | 'ioTime'>('ioTime');

    const { result } = renderHook(() =>
      useDiskLiveMetricModel({
        resourceId,
        type,
      }),
    );

    expect(result.value()).toBe(72);
    expect(result.formatted()).toBe('72%');
    expect(result.colorClass()).toBe('text-yellow-600 dark:text-yellow-400 font-semibold');
  });
});
