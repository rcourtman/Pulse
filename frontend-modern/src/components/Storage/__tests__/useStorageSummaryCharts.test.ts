import { renderHook } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useStorageSummaryCharts } from '@/components/Storage/useStorageSummaryCharts';

const summaryCacheMocks = vi.hoisted(() => ({
  fetchStorageSummaryAndCache: vi.fn(async () => ({ pools: {}, disks: {} })),
  readStorageSummaryCache: vi.fn(() => null),
}));

vi.mock('@/utils/storageSummaryCache', () => summaryCacheMocks);
vi.mock('@/stores/events', () => ({
  eventBus: {
    on: () => vi.fn(),
  },
}));

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe('useStorageSummaryCharts', () => {
  it('defers cache parsing and the first summary request until browser idle', async () => {
    let runWhenIdle: IdleRequestCallback | undefined;
    vi.stubGlobal(
      'requestIdleCallback',
      vi.fn((callback: IdleRequestCallback) => {
        runWhenIdle = callback;
        return 1;
      }),
    );
    vi.stubGlobal('cancelIdleCallback', vi.fn());
    Object.defineProperty(window, 'cancelIdleCallback', {
      configurable: true,
      value: vi.fn(),
    });

    const { cleanup } = renderHook(() =>
      useStorageSummaryCharts({
        timeRange: () => '24h',
        deferInitialLoad: true,
      }),
    );

    expect(summaryCacheMocks.readStorageSummaryCache).not.toHaveBeenCalled();
    expect(summaryCacheMocks.fetchStorageSummaryAndCache).not.toHaveBeenCalled();

    runWhenIdle?.({ didTimeout: false, timeRemaining: () => 50 } as IdleDeadline);
    await Promise.resolve();

    expect(summaryCacheMocks.readStorageSummaryCache).toHaveBeenCalledWith('24h', undefined);
    expect(summaryCacheMocks.fetchStorageSummaryAndCache).toHaveBeenCalledTimes(1);
    cleanup();
    Reflect.deleteProperty(window, 'cancelIdleCallback');
  });
});
