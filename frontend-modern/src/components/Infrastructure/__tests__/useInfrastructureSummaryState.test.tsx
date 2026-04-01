import { renderHook, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';

import {
  __resetInMemoryChartCacheForTests,
  useInfrastructureSummaryState,
} from '../useInfrastructureSummaryState';

const mockFetchInfrastructureSummaryAndCache = vi.fn();
const mockReadInfrastructureSummaryCache = vi.fn(() => null);

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    workloads: () => [],
    resources: () => [],
  }),
}));

vi.mock('@/utils/infrastructureSummaryCache', () => ({
  fetchInfrastructureSummaryAndCache: (...args: unknown[]) =>
    mockFetchInfrastructureSummaryAndCache(...args),
  readInfrastructureSummaryCache: (...args: unknown[]) =>
    mockReadInfrastructureSummaryCache(...args),
}));

vi.mock('@/utils/apiClient', async () => {
  const actual = await vi.importActual<typeof import('@/utils/apiClient')>('@/utils/apiClient');
  return {
    ...actual,
    getOrgID: () => 'default',
  };
});

const now = Date.now();
const twoPointSeries = [
  { timestamp: now - 60_000, value: 12 },
  { timestamp: now, value: 18 },
];

const makeResource = (id: string, name: string): Resource =>
  ({
    id,
    type: 'agent',
    name,
    displayName: name,
    platformId: id,
    platformType: 'agent',
    sourceType: 'hybrid',
    status: 'online',
    lastSeen: now,
    platformData: { sources: ['agent'] },
  }) as Resource;

describe('useInfrastructureSummaryState', () => {
  beforeEach(() => {
    mockFetchInfrastructureSummaryAndCache.mockReset();
    mockReadInfrastructureSummaryCache.mockReset();
    mockReadInfrastructureSummaryCache.mockReturnValue(null);
    __resetInMemoryChartCacheForTests();
    localStorage.clear();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('keeps infrastructure summaries page-scoped when a focused resource is selected', async () => {
    mockFetchInfrastructureSummaryAndCache.mockResolvedValue({
      map: new Map([
        [
          'host-1',
          {
            cpu: twoPointSeries,
            memory: twoPointSeries,
            disk: [],
            diskread: twoPointSeries,
            diskwrite: twoPointSeries,
            netin: twoPointSeries,
            netout: twoPointSeries,
          },
        ],
        [
          'host-2',
          {
            cpu: twoPointSeries,
            memory: twoPointSeries,
            disk: [],
            diskread: twoPointSeries,
            diskwrite: twoPointSeries,
            netin: twoPointSeries,
            netout: twoPointSeries,
          },
        ],
      ]),
      oldestDataTimestamp: now - 60_000,
    });

    const resources = [makeResource('host-1', 'Host 1'), makeResource('host-2', 'Host 2')];

    const { result } = renderHook(() =>
      useInfrastructureSummaryState({
        resources,
        timeRange: '1h',
        focusedResourceId: 'host-1',
      }),
    );

    await waitFor(() => {
      expect(result.effectiveFocusedResourceId()).toBe('host-1');
      expect(result.focusedResourceName()).toBe('Host 1');
      expect(result.seriesFor('cpu')).toHaveLength(2);
      expect(result.networkSeries()).toHaveLength(2);
    });
    expect(result.hasInteractiveResourceId('host-1')).toBe(true);
  });
});
